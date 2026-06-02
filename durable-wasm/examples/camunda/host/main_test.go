package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nativebpm/connectors/camunda"
	"github.com/nativebpm/connectors/durable-wasm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCamundaWasmOrchestration_Success_With_Retry(t *testing.T) {
	// 1. Cleanup files
	_ = os.Remove(dbFile)
	_ = os.Remove(sqliteDBFile)
	defer func() {
		_ = os.Remove(dbFile)
		_ = os.Remove(sqliteDBFile)
	}()

	// 2. Start mock REST API services
	mockServices := startMockServer(serverAddr)
	defer mockServices.Shutdown(context.Background())

	// 3. Start mock Camunda server
	var fetchCount int32
	var failureCount int32
	var completeCount int32

	camundaServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// Mock Deployment

		if r.URL.Path == "/engine-rest/deployment/create" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":"test-deployment"}`))
			return
		}

		// Mock Start Process Instance
		if r.URL.Path == "/engine-rest/process-definition/key/DurableWasmCamundaProcess/start" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":"test-process-instance"}`))
			return
		}

		// Mock Fetch and Lock
		if r.URL.Path == "/engine-rest/external-task/fetchAndLock" {
			count := atomic.AddInt32(&fetchCount, 1)
			w.WriteHeader(http.StatusOK)

			// We only return task on the first fetch, and on the second fetch (after failure)
			if count == 1 || (count == 2 && atomic.LoadInt32(&failureCount) == 1) {
				task := camunda.ExternalTask{
					ID:          "task-123",
					TopicName:   "durable-wasm-task",
					WorkerID:    "durable-wasm-worker",
					BusinessKey: "test-order-key",
				}
				tasks := []camunda.ExternalTask{task}
				data, _ := json.Marshal(tasks)
				_, _ = w.Write(data)
			} else {
				_, _ = w.Write([]byte(`[]`))
			}
			return
		}

		// Mock Failure reporting
		if r.URL.Path == "/engine-rest/external-task/task-123/failure" {
			atomic.AddInt32(&failureCount, 1)
			w.WriteHeader(http.StatusNoContent)
			return
		}

		// Mock Complete reporting
		if r.URL.Path == "/engine-rest/external-task/task-123/complete" {
			atomic.AddInt32(&completeCount, 1)
			w.WriteHeader(http.StatusNoContent)
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))

	defer camundaServer.Close()

	// 4. Initialize Camunda Client pointed to mock server
	client, err := camunda.NewClient(camundaServer.URL, "durable-wasm-worker")
	require.NoError(t, err)

	// 5. Initialize Durable Engine
	wasmPath := "../worker/worker.wasm"
	store, err := durable.NewSqliteSnapshotStore(sqliteDBFile)
	require.NoError(t, err)
	defer store.Close()

	engine, err := durable.NewEngine(wasmPath, store)
	require.NoError(t, err)

	// 6. Create Camunda Worker
	w := camunda.NewWorker(client, nil)
	w.SetMaxTasks(1)
	w.SetPollInterval(10 * time.Millisecond)

	w.RegisterHandler("durable-wasm-task", camunda.TaskHandlerFunc(
		func(ctx context.Context, c *camunda.Client, task camunda.ExternalTask, complete camunda.CompleteFunc, fail camunda.FailFunc) error {
			businessKey := task.BusinessKey
			_, err := store.Load(businessKey)
			hasSnapshot := err == nil

			shouldCrash := !hasSnapshot

			crashed, err := engine.Execute(businessKey, "run", serverAddr, shouldCrash)
			if err != nil {
				if crashed {
					_ = fail("simulated_host_crash", "WASM state snapshotted", 1, 10)
					return nil
				}
				_ = fail(err.Error(), "execution error", 0, 0)
				return nil
			}

			err = complete().Execute()
			if err != nil {
				return err
			}

			_ = store.Delete(businessKey)
			return nil
		},
	), 60000, []string{})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start worker
	go func() {
		w.Start(ctx)
	}()

	// Start process instance
	_, err = client.StartProcessInstance(ctx, "DurableWasmCamundaProcess", "test-order-key", nil)
	require.NoError(t, err)

	// Wait for successful complete (up to 5 seconds)
	require.Eventually(t, func() bool {
		return atomic.LoadInt32(&completeCount) == 1
	}, 5*time.Second, 50*time.Millisecond)

	// 7. Verify result
	assert.Equal(t, int32(1), atomic.LoadInt32(&failureCount), "Should have failed once")
	assert.Equal(t, int32(1), atomic.LoadInt32(&completeCount), "Should have completed once")

	// Verify persistence file exists and contains correct content
	dbBytes, err := os.ReadFile(dbFile)
	require.NoError(t, err)
	assert.Contains(t, string(dbBytes), `"status":"processed"`)

	// Snapshot should be cleaned up
	_, err = store.Load("test-order-key")
	assert.Error(t, err, "Snapshot should be deleted")
}

