package main

import (
	"context"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/nativebpm/connectors/durable-wasm"
)

const (
	instanceID   = "temporal-activity-tx"
	serverAddr   = "localhost:18085"
	sqliteDBFile = "snapshots.db"
	dbFile       = "database_temporal.json"
)

func main() {
	slog.Info("[HOST] Starting Temporal Durable Activity Execution Example")

	// 1. Clean up old files
	_ = os.Remove(dbFile)

	// 2. Start local Mock HTTP Server to mock external billing, CRM, and DB API endpoints
	mockServer := startMockServer(serverAddr)
	defer mockServer.Shutdown(context.Background())

	// Give the server a small moment to bind to the port
	time.Sleep(100 * time.Millisecond)

	// 3. Initialize the Reusable Durable WASM Engine with SQLite store
	wasmPath := filepath.Join("..", "worker", "worker.wasm")
	store, err := durable.NewSqliteSnapshotStore(sqliteDBFile)
	if err != nil {
		slog.Error("[HOST] Failed to initialize SQLite store", "error", err)
		os.Exit(1)
	}
	defer store.Close()
	
	engine, err := durable.NewEngine(wasmPath, store)
	if err != nil {
		slog.Error("[HOST] Failed to initialize engine", "error", err)
		os.Exit(1)
	}

	// Clear any leftover snapshot from previous runs in the database
	_ = store.Delete(instanceID)

	// 4. RUN 1: Execute with simulated crash on the first checkpoint (Step 0)
	slog.Info("[HOST] RUN 1: Starting Temporal Activity with Simulated Crash")
	crashed, err := engine.Execute(instanceID, "run", serverAddr, true)
	if err != nil {
		if crashed {
			slog.Info("[HOST] Activity successfully suspended/crashed", "error", err)
		} else {
			slog.Error("[HOST] Execution failed", "error", err)
			os.Exit(1)
		}
	}

	// Verify snapshot exists in SQLite database
	_, err = store.Load(instanceID)
	if err != nil {
		slog.Error("[HOST] Snapshot was not found in SQLite", "error", err)
		os.Exit(1)
	}
	slog.Info("[HOST] Verified that snapshot was successfully written to SQLite database")

	// 5. RUN 2: Restore from checkpoint and resume execution
	slog.Info("[HOST] RUN 2: Restoring Activity State from Snapshot and Resuming execution")
	crashed, err = engine.Execute(instanceID, "run", serverAddr, false)
	if err != nil {
		slog.Error("[HOST] Resumed execution failed", "error", err)
		os.Exit(1)
	}

	if crashed {
		slog.Error("[HOST] Resumed execution crashed unexpectedly!")
		os.Exit(1)
	}

	// 6. Verify Database Persistence (Hybrid approach validation)
	slog.Info("[HOST] HYBRID APPROACH VALIDATION")
	dbBytes, err := os.ReadFile(dbFile)
	if err != nil {
		slog.Error("[HOST] Final database record not found", "error", err)
		os.Exit(1)
	}
	slog.Info("[HOST] Read from persistent DB", "file", dbFile, "content", string(dbBytes))

	// Clean up snapshot since the transaction is completed (we no longer need workflow memory)
	_ = store.Delete(instanceID)
	if _, err := store.Load(instanceID); err != nil {
		slog.Info("[HOST] Workflow memory snapshot successfully cleaned up from store (Transaction Completed)")
	}

	// Clean up database_temporal.json
	_ = os.Remove(dbFile)
	
	slog.Info("[HOST] Temporal Activity example completed successfully")
	os.Exit(0)
}

func startMockServer(addr string) *http.Server {
	mux := http.NewServeMux()

	// Download route (used for reading calculation parameters)
	mux.HandleFunc("/download", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		// Returns base rate and multiplier
		paramsResponse := `{"base_rate":1.5,"multiplier":8.0}`
		_, _ = w.Write([]byte(paramsResponse))
	})

	// Upload route (used for sending request body and final results)
	var uploadCount int
	mux.HandleFunc("/upload", func(w http.ResponseWriter, r *http.Request) {
		uploadCount++
		w.WriteHeader(http.StatusOK)

		body, err := io.ReadAll(r.Body)
		if err != nil {
			slog.Error("[MOCK SERVICES] Failed to read upload", "error", err)
			return
		}

		if uploadCount == 1 {
			slog.Info("[MOCK TEMPORAL SERVICE] Received param request query", "body", string(body))
		} else if uploadCount == 2 {
			slog.Info("[MOCK DATABASE API] Received final calculation result to persist", "body", string(body))
			_ = os.WriteFile(dbFile, body, 0644)
		}
	})

	server := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	go func() {
		l, err := net.Listen("tcp", addr)
		if err != nil {
			slog.Error("[MOCK SERVICES] Failed to listen", "error", err)
			return
		}
		if err := server.Serve(l); err != nil && err != http.ErrServerClosed {
			slog.Error("[MOCK SERVICES] Serve error", "error", err)
		}
	}()

	slog.Info("[MOCK SERVICES] Listening", "addr", "http://"+addr)
	return server
}
