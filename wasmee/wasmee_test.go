package wasmee

import (
	"context"
	"errors"
	"net"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func startRustServer(t *testing.T) func() {
	// First check if a server is already listening on 8081 (e.g. running via docker or manually)
	conn, err := net.DialTimeout("tcp", "localhost:8081", 100*time.Millisecond)
	if err == nil {
		conn.Close()
		return func() {} // Already running, no-op cleanup
	}

	// Try to launch the sibling debug binary if it exists
	_, statErr := os.Stat("../../wasmee/target/debug/wasmee")
	if statErr == nil {
		cmd := exec.Command("../../wasmee/target/debug/wasmee")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Start(); err != nil {
			t.Fatalf("failed to start rust server: %v", err)
		}

		for i := 0; i < 50; i++ {
			conn, err := net.DialTimeout("tcp", "localhost:8081", 50*time.Millisecond)
			if err == nil {
				conn.Close()
				break
			}
			time.Sleep(50 * time.Millisecond)
		}

		return func() {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
	}

	// Otherwise, skip the test
	t.Skip("wasmee server is not running on localhost:8081 and sibling wasmee binary is not found; skipping integration test")
	return func() {}
}


type testInMemoryStore struct {
	snapshots map[string][]byte
	deltas    map[string]map[int][]byte
	oplogs    map[string][]OplogEntry
	metadata  map[string]*InstanceMeta
}

func newTestStore() *testInMemoryStore {
	return &testInMemoryStore{
		snapshots: make(map[string][]byte),
		deltas:    make(map[string]map[int][]byte),
		oplogs:    make(map[string][]OplogEntry),
		metadata:  make(map[string]*InstanceMeta),
	}
}

func (s *testInMemoryStore) SaveSnapshot(ctx context.Context, id string, snapshot []byte) error {
	s.snapshots[id] = snapshot
	return nil
}

func (s *testInMemoryStore) LoadSnapshot(ctx context.Context, id string) ([]byte, error) {
	data, ok := s.snapshots[id]
	if !ok {
		return nil, errors.New("not found")
	}
	return data, nil
}

func (s *testInMemoryStore) DeleteSnapshot(ctx context.Context, id string) error {
	delete(s.snapshots, id)
	return nil
}

func (s *testInMemoryStore) SaveDeltas(ctx context.Context, id string, deltas map[int][]byte) error {
	if s.deltas[id] == nil {
		s.deltas[id] = make(map[int][]byte)
	}
	for k, v := range deltas {
		s.deltas[id][k] = v
	}
	return nil
}

func (s *testInMemoryStore) LoadDeltas(ctx context.Context, id string) (map[int][]byte, error) {
	return s.deltas[id], nil
}

func (s *testInMemoryStore) TruncateDeltas(ctx context.Context, id string) error {
	delete(s.deltas, id)
	return nil
}

func (s *testInMemoryStore) SaveOplog(ctx context.Context, id string, entry OplogEntry) error {
	s.oplogs[id] = append(s.oplogs[id], entry)
	return nil
}

func (s *testInMemoryStore) LoadOplog(ctx context.Context, id string) ([]OplogEntry, error) {
	return s.oplogs[id], nil
}

func (s *testInMemoryStore) TruncateOplog(ctx context.Context, id string, beforeCallIndex int) error {
	var filtered []OplogEntry
	for _, entry := range s.oplogs[id] {
		if entry.CallIndex < beforeCallIndex {
			filtered = append(filtered, entry)
		}
	}
	s.oplogs[id] = filtered
	return nil
}

func (s *testInMemoryStore) SaveMetadata(ctx context.Context, meta *InstanceMeta) (bool, error) {
	prev, exists := s.metadata[meta.InstanceID]
	if exists && prev.Version != meta.Version-1 {
		return false, nil
	}
	metaCopy := *meta
	s.metadata[meta.InstanceID] = &metaCopy
	return true, nil
}

func (s *testInMemoryStore) LoadMetadata(ctx context.Context, id string) (*InstanceMeta, error) {
	meta, ok := s.metadata[id]
	if !ok {
		return nil, errors.New("not found")
	}
	metaCopy := *meta
	return &metaCopy, nil
}

func TestWasmRunnerExecution(t *testing.T) {
	ctx := context.Background()
	instanceID := "test-wasm-execution-instance"

	// 1. Read compiled test WASM file from local testdata
	wasmBytes, err := os.ReadFile("testdata/wasmee_guest.wasm")
	if err != nil {
		t.Fatalf("failed to read test WASM binary: %v", err)
	}

	store := newTestStore()
	meta := &InstanceMeta{
		InstanceID: instanceID,
		WasmHash:   "test_hash",
		Version:    0,
	}
	store.SaveMetadata(ctx, meta)

	cleanup := startRustServer(t)
	defer cleanup()

	// RUN 1: Starts fresh, executes up to checkpoint 1, and finishes successfully.
	crashed, err := NewFluentRunner().
		WithContext(ctx).
		WithServerAddress("http://localhost:8081").
		WithWasmBytes(wasmBytes).
		WithStore(store).
		WithSessionID(instanceID).
		WithEntrypoint("run_test").
		Run()
	if err != nil {
		t.Fatalf("execution failed: %v", err)
	}
	if crashed {
		t.Fatalf("unexpected crash detected")
	}

	// 2. Validate that checkpoint 1 saved a base snapshot (Version = 1)
	snapshot, err := store.LoadSnapshot(ctx, instanceID)
	if err != nil {
		t.Fatalf("failed to load base snapshot: %v", err)
	}
	if len(snapshot) == 0 {
		t.Fatalf("base snapshot is empty")
	}

	// Validate oplog contains 2 calls: test_api("hello") and test_api("world")
	oplogs, err := store.LoadOplog(ctx, instanceID)
	if err != nil {
		t.Fatalf("failed to load oplog: %v", err)
	}
	if len(oplogs) != 2 {
		t.Fatalf("expected exactly 2 oplog entries, got %d", len(oplogs))
	}
	if oplogs[0].ApiName != "test_api" || string(oplogs[0].RequestPayload) != "hello" {
		t.Fatalf("invalid first call entry in oplog")
	}
	if oplogs[1].ApiName != "test_api" || string(oplogs[1].RequestPayload) != "world" {
		t.Fatalf("invalid second call entry in oplog")
	}

	// Validate memory deltas exist (checkpoint 2 saved changes to offset 70000 on page 1)
	deltas, err := store.LoadDeltas(ctx, instanceID)
	if err != nil {
		t.Fatalf("failed to load memory deltas: %v", err)
	}
	if len(deltas) == 0 {
		t.Fatalf("expected memory deltas for dirty pages, got none")
	}
	if _, ok := deltas[1]; !ok {
		t.Fatalf("expected page index 1 to be marked as dirty")
	}
}

func TestWasmRunnerSimulatedCrashRecovery(t *testing.T) {
	ctx := context.Background()
	instanceID := "test-crash-recovery-instance"

	wasmBytes, err := os.ReadFile("testdata/wasmee_guest.wasm")
	if err != nil {
		t.Fatalf("failed to read test WASM binary: %v", err)
	}

	store := newTestStore()
	meta := &InstanceMeta{
		InstanceID: instanceID,
		WasmHash:   "test_hash",
		Version:    0,
	}
	store.SaveMetadata(ctx, meta)

	cleanup := startRustServer(t)
	defer cleanup()

	// RUN 1: Starts fresh, hits checkpoint 1, saves state, and crashes.
	crashed, err := NewFluentRunner().
		WithContext(ctx).
		WithServerAddress("http://localhost:8081").
		WithWasmBytes(wasmBytes).
		WithStore(store).
		WithSessionID(instanceID).
		WithEntrypoint("run_test").
		WithCrashSimulation(true).
		Run()
	if err == nil || !crashed {
		t.Fatalf("expected simulated host crash, got nil or no crash")
	}

	// Verify checkpoint 1 saved version 1 snapshot metadata
	metaReload, err := store.LoadMetadata(ctx, instanceID)
	if err != nil {
		t.Fatalf("failed to load metadata: %v", err)
	}
	if metaReload.Version != 1 {
		t.Fatalf("expected snapshot version 1, got %d", metaReload.Version)
	}

	// Verify oplog contains only 1 call (test_api("hello")) before crash
	oplogs, err := store.LoadOplog(ctx, instanceID)
	if err != nil {
		t.Fatalf("failed to load oplog: %v", err)
	}
	if len(oplogs) != 1 {
		t.Fatalf("expected exactly 1 oplog entry before crash, got %d", len(oplogs))
	}

	// RUN 2: Reload state and resume from crash point (disabling crash simulation)
	crashed2, err := NewFluentRunner().
		WithContext(ctx).
		WithServerAddress("http://localhost:8081").
		WithWasmBytes(wasmBytes).
		WithStore(store).
		WithSessionID(instanceID).
		WithEntrypoint("run_test").
		WithCrashSimulation(false).
		Run()
	if err != nil {
		t.Fatalf("resume execution failed: %v", err)
	}
	if crashed2 {
		t.Fatalf("unexpected crash on resume run")
	}

	// Verify the final oplog has 2 entries and the second entry is the live call for "world"
	oplogs2, err := store.LoadOplog(ctx, instanceID)
	if err != nil {
		t.Fatalf("failed to load final oplog: %v", err)
	}
	t.Logf("Final oplog count: %d", len(oplogs2))
	for i, entry := range oplogs2 {
		t.Logf("Oplog[%d]: CallIndex=%d, ApiName=%s, RequestPayload=%s, ResponsePayload=%s", i, entry.CallIndex, entry.ApiName, string(entry.RequestPayload), string(entry.ResponsePayload))
	}
	if len(oplogs2) != 2 {
		t.Fatalf("expected exactly 2 oplog entries after recovery, got %d", len(oplogs2))
	}
	if oplogs2[1].ApiName != "test_api" || string(oplogs2[1].RequestPayload) != "world" {
		t.Fatalf("invalid second call entry in final oplog: got Api=%s Request=%s", oplogs2[1].ApiName, string(oplogs2[1].RequestPayload))
	}

	// Verify final version is 3
	metaFinal, err := store.LoadMetadata(ctx, instanceID)
	if err != nil {
		t.Fatalf("failed to load final metadata: %v", err)
	}
	if metaFinal.Version != 3 {
		t.Fatalf("expected final version 3, got %d", metaFinal.Version)
	}
}

func TestDynamicWasmModuleExecution(t *testing.T) {
	ctx := context.Background()
	instanceID := "test-dynamic-wasm-instance"

	// 1. Read valid guest WASM file
	wasmBytes, err := os.ReadFile("testdata/wasmee_guest.wasm")
	if err != nil {
		t.Fatalf("failed to read test WASM binary: %v", err)
	}

	cleanup := startRustServer(t)
	defer cleanup()

	store := newTestStore()
	meta := &InstanceMeta{
		InstanceID: instanceID,
		WasmHash:   "test_hash",
		Version:    0,
	}
	store.SaveMetadata(ctx, meta)

	// Case A: Verify successful execution with dynamic WASM bytes (first compile)
	crashed, err := NewFluentRunner().
		WithContext(ctx).
		WithServerAddress("http://localhost:8081").
		WithWasmBytes(wasmBytes).
		WithStore(store).
		WithSessionID(instanceID).
		WithEntrypoint("run_test").
		Run()
	if err != nil {
		t.Fatalf("dynamic execution failed: %v", err)
	}
	if crashed {
		t.Fatalf("unexpected crash detected")
	}

	// Case B: Verify successful execution with cache hit (second run)
	crashed2, err := NewFluentRunner().
		WithContext(ctx).
		WithServerAddress("http://localhost:8081").
		WithWasmBytes(wasmBytes).
		WithStore(store).
		WithSessionID(instanceID).
		WithEntrypoint("run_test").
		Run()
	if err != nil {
		t.Fatalf("cached execution failed: %v", err)
	}
	if crashed2 {
		t.Fatalf("unexpected crash on cached execution")
	}

	// Case C: Verify compile failure with corrupt/invalid WASM bytes
	corruptBytes := []byte("this is not a valid wasm file header")
	crashed3, err := NewFluentRunner().
		WithContext(ctx).
		WithServerAddress("http://localhost:8081").
		WithWasmBytes(corruptBytes).
		WithStore(store).
		WithSessionID(instanceID).
		WithEntrypoint("run_test").
		Run()
	if err == nil {
		t.Fatalf("expected error executing corrupt WASM, got nil")
	}
	if !crashed3 {
		t.Fatalf("expected crashed=true for compile error, got false")
	}
	if !strings.Contains(err.Error(), "Failed to compile") {
		t.Fatalf("expected error message to contain 'Failed to compile', got: %v", err)
	}
}

func TestFluentRunnerExecution(t *testing.T) {
	stop := startRustServer(t)
	defer stop()

	wasmPath := "testdata/wasmee_guest.wasm"
	wasmBytes, err := os.ReadFile(wasmPath)
	if err != nil {
		t.Fatalf("failed to read guest WASM: %v", err)
	}

	ctx := context.Background()
	instanceID := "fluent-session-1"
	store := newTestStore()

	meta := &InstanceMeta{
		InstanceID: instanceID,
		WasmHash:   "test_hash",
		Version:    0,
	}
	_, _ = store.SaveMetadata(ctx, meta)

	crashed, err := NewFluentRunner().
		WithContext(ctx).
		WithServerAddress("http://localhost:8081").
		WithWasmBytes(wasmBytes).
		WithStore(store).
		WithSessionID(instanceID).
		WithEntrypoint("run_test").
		Run()

	if err != nil {
		t.Fatalf("fluent run failed: %v", err)
	}
	if crashed {
		t.Fatalf("expected crashed=false, got true")
	}

	oplogs, err := store.LoadOplog(ctx, instanceID)
	if err != nil {
		t.Fatalf("failed to load oplog: %v", err)
	}
	if len(oplogs) == 0 {
		t.Fatalf("expected oplogs to be recorded, got 0")
	}
}

