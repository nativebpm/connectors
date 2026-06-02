package durable

import (
	"context"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/bytecodealliance/wasmtime-go/v20"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDurableExecutionLifecycle(t *testing.T) {
	instanceID := "test-worker-instance"
	serverAddr := "localhost:18081"

	// 2. Start mock HTTP server
	mux := http.NewServeMux()
	mux.HandleFunc("/download", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("hello from durable test stream!"))
	})

	var receivedBytes []byte
	var uploadErr error
	mux.HandleFunc("/upload", func(w http.ResponseWriter, r *http.Request) {
		receivedBytes, uploadErr = io.ReadAll(r.Body)
		if uploadErr != nil {
			http.Error(w, uploadErr.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	server := &http.Server{
		Addr:    serverAddr,
		Handler: mux,
	}

	ln, err := net.Listen("tcp", serverAddr)
	require.NoError(t, err)

	go func() {
		_ = server.Serve(ln)
	}()
	defer server.Shutdown(context.Background())

	// Give the server a small moment to start
	time.Sleep(50 * time.Millisecond)

	// 3. Initialize engine
	wasmPath := filepath.Join("examples", "durable-s3", "worker", "worker.wasm")

	// Use an in-memory SQLite store for maximum speed and zero disk cleanup
	store, err := NewSqliteSnapshotStore(":memory:")
	require.NoError(t, err)
	defer store.Close()

	engine, err := NewEngine(wasmPath, store)
	require.NoError(t, err, "Failed to compile WASM module. Make sure worker.wasm is built.")

	// 4. RUN 1: Execute with simulated crash
	crashed, err := engine.Execute(instanceID, "run", serverAddr, true)
	require.Error(t, err)
	assert.True(t, crashed, "Expected run 1 to crash at checkpoint")

	// Verify snapshot exists in SQLite database
	snapshot, err := store.Load(instanceID)
	require.NoError(t, err, "Snapshot should exist in SQLite database")
	assert.NotEmpty(t, snapshot, "Snapshot data should not be empty")

	// 5. RUN 2: Restore from checkpoint and run to completion
	crashed, err = engine.Execute(instanceID, "run", serverAddr, false)
	require.NoError(t, err, "Run 2 should complete without errors")
	assert.False(t, crashed, "Run 2 should not crash")

	// 6. Verify processed output
	expectedOutput := "HELLO FROM DURABLE TEST STREAM!"
	assert.Equal(t, expectedOutput, string(receivedBytes), "Data processed by WASM worker should be converted to uppercase")
}

func TestDirtyPageAndOplog(t *testing.T) {
	instanceID := "test-dirty-oplog-instance"

	// Write simple WebAssembly Text (WAT) module to simulate Oplog and Dirty pages
	wat := `
	(module
	  (import "env" "checkpoint" (func $checkpoint))
	  (import "env" "host_call_api" (func $host_call_api (param i32 i32 i32 i32 i32 i32) (result i32)))
	  (memory (export "memory") 2)
	  (data (i32.const 0) "test_api")
	  (data (i32.const 16) "hello")
	  (data (i32.const 100) "world")
	  (func (export "run_test")
	    ;; Call test_api with payload "hello" -> outputs to offset 32
	    (call $host_call_api
	      (i32.const 0)   ;; apiNamePtr
	      (i32.const 8)   ;; apiNameLen
	      (i32.const 16)  ;; reqPtr
	      (i32.const 5)   ;; reqLen
	      (i32.const 32)  ;; respPtr
	      (i32.const 64)  ;; respMaxLen
	    )
	    drop

	    ;; First checkpoint (Crash point 1)
	    (call $checkpoint)

	    ;; Modify memory in the 2nd page (offset 70000) to trigger dirty-page tracking
	    (i32.store (i32.const 70000) (i32.const 42))

	    ;; Call test_api with payload "world" -> outputs to offset 200
	    (call $host_call_api
	      (i32.const 0)   ;; apiNamePtr
	      (i32.const 8)   ;; apiNameLen
	      (i32.const 100) ;; reqPtr
	      (i32.const 5)   ;; reqLen
	      (i32.const 200) ;; respPtr
	      (i32.const 64)  ;; respMaxLen
	    )
	    drop

	    ;; Second checkpoint
	    (call $checkpoint)
	  )
	)
	`
	wasmBytes, err := wasmtime.Wat2Wasm(wat)
	require.NoError(t, err)

	tempDir := t.TempDir()
	wasmPath := filepath.Join(tempDir, "test.wasm")
	err = os.WriteFile(wasmPath, wasmBytes, 0644)
	require.NoError(t, err)

	store, err := NewSqliteSnapshotStore(":memory:")
	require.NoError(t, err)
	defer store.Close()

	engine, err := NewEngine(wasmPath, store)
	require.NoError(t, err)

	// RUN 1: Run and crash on first checkpoint
	crashed, err := engine.Execute(instanceID, "run_test", "localhost:0", true)
	require.Error(t, err)
	assert.True(t, crashed)

	// Verify deltas and oplog saved
	deltas, err := store.LoadDeltas(instanceID)
	require.NoError(t, err)
	assert.NotEmpty(t, deltas, "Memory deltas should not be empty")

	oplog, err := store.LoadOplog(instanceID)
	require.NoError(t, err)
	require.Len(t, oplog, 1, "Should have exactly 1 oplog entry")
	assert.Equal(t, "test_api", oplog[0].ApiName)
	assert.Equal(t, "hello", string(oplog[0].RequestPayload))
	assert.Equal(t, "resp_for_hello_call_1", string(oplog[0].ResponsePayload))

	// RUN 2: Resume, should replay first api call without crash, modify page 2, and complete second checkpoint without crash
	crashed, err = engine.Execute(instanceID, "run_test", "localhost:0", false)
	require.NoError(t, err)
	assert.False(t, crashed)

	// Verify memory delta has dirty pages saved
	deltas2, err := store.LoadDeltas(instanceID)
	require.NoError(t, err)
	// Block size is 4KB, offset 70000 lies in page index 70000/4096 = 17
	assert.Contains(t, deltas2, 17, "Delta snapshot must contain dirty page index 17 (offset 70000)")

	// Verify oplog contains 2 calls
	oplog2, err := store.LoadOplog(instanceID)
	require.NoError(t, err)
	assert.Len(t, oplog2, 2, "Oplog must contain 2 entries after complete run")
	assert.Equal(t, "world", string(oplog2[1].RequestPayload))
}

func TestPostgresSnapshotStore(t *testing.T) {
	// Try to connect to a local PostgreSQL instance (default credentials or env)
	connStr := os.Getenv("POSTGRES_CONN")
	if connStr == "" {
		connStr = "postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable"
	}

	// Ping PG to see if it is available
	db, err := net.DialTimeout("tcp", "localhost:5432", 1*time.Second)
	if err != nil {
		t.Skip("PostgreSQL is not running on localhost:5432. Skipping Postgres integration test.")
		return
	}
	db.Close()

	store, err := NewPostgresSnapshotStore(connStr)
	if err != nil {
		t.Skipf("PostgreSQL connection failed (credentials or DB might not be configured): %v. Skipping Postgres integration test.", err)
		return
	}
	defer store.Close()

	instanceID := "postgres-test-instance"
	defer store.Delete(instanceID)

	// Test basic save/load
	err = store.Save(instanceID, []byte("postgres-full-snapshot"))
	require.NoError(t, err)

	snapshot, err := store.Load(instanceID)
	require.NoError(t, err)
	assert.Equal(t, "postgres-full-snapshot", string(snapshot))

	// Test deltas
	deltas := map[int][]byte{
		0: []byte("page-0-data"),
		5: []byte("page-5-data"),
	}
	err = store.SaveDeltas(instanceID, deltas)
	require.NoError(t, err)

	loadedDeltas, err := store.LoadDeltas(instanceID)
	require.NoError(t, err)
	assert.Len(t, loadedDeltas, 2)
	assert.Equal(t, "page-0-data", string(loadedDeltas[0]))
	assert.Equal(t, "page-5-data", string(loadedDeltas[5]))

	// Test oplog
	err = store.SaveOplog(instanceID, 1, "test_call", []byte("req"), []byte("resp"))
	require.NoError(t, err)

	oplog, err := store.LoadOplog(instanceID)
	require.NoError(t, err)
	require.Len(t, oplog, 1)
	assert.Equal(t, 1, oplog[0].CallIndex)
	assert.Equal(t, "test_call", oplog[0].ApiName)
	assert.Equal(t, "req", string(oplog[0].RequestPayload))
	assert.Equal(t, "resp", string(oplog[0].ResponsePayload))
}
