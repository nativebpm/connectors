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
	instanceID = "worker-instance-42"
	serverAddr = "localhost:18080"
	dbFile     = "snapshots.db"
)

func main() {
	slog.Info("[HOST] Starting Reusable Durable WASM Execution Orchestrator")

	// 2. Start local Mock HTTP Server to mock external REST calls
	mockServer := startMockServer(serverAddr)
	defer mockServer.Shutdown(context.Background())

	// Give the server a small moment to bind to the port
	time.Sleep(100 * time.Millisecond)

	// 3. Initialize the Reusable Durable WASM Engine with SQLite store
	wasmPath := os.Getenv("WASM_PATH")
	if wasmPath == "" {
		wasmPath = filepath.Join("..", "worker", "worker.wasm")
	}
	store, err := durable.NewSqliteSnapshotStore(dbFile)
	if err != nil {
		slog.Error("[HOST] Failed to initialize SQLite store", "error", err)
		os.Exit(1)
	}
	defer store.Close()

	// Clear any leftover snapshot from previous runs in the database
	_ = store.Delete(instanceID)

	engine, err := durable.NewEngine(wasmPath, store)
	if err != nil {
		slog.Error("[HOST] Failed to initialize engine", "error", err)
		slog.Error("[HOST] Make sure worker.wasm is compiled by running 'make build-worker'")
		os.Exit(1)
	}

	// 4. RUN 1: Execute with simulated crash on the first checkpoint
	slog.Info("[HOST] RUN 1: Executing WASM from scratch with simulated crash")
	crashed, err := engine.Execute(instanceID, "run", serverAddr, true)
	if err != nil {
		if crashed {
			slog.Info("[HOST] Execution successfully suspended/crashed", "error", err)
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
	slog.Info("[HOST] RUN 2: Restoring from snapshot and completing execution")
	crashed, err = engine.Execute(instanceID, "run", serverAddr, false)
	if err != nil {
		slog.Error("[HOST] Resumed execution failed", "error", err)
		os.Exit(1)
	}

	if crashed {
		slog.Error("[HOST] Resumed execution crashed unexpectedly!")
		os.Exit(1)
	}

	// 6. Final Clean up
	// We keep the snapshot in the database to verify replication to S3 and restore capability
	// _ = store.Delete(instanceID)
	slog.Info("[HOST] Durable WASM Execution demonstration complete")
	time.Sleep(5 * time.Second) // Give Litestream ample time to sync database and final WAL frames
	os.Exit(0)
}

func startMockServer(addr string) *http.Server {
	mux := http.NewServeMux()

	mux.HandleFunc("/download", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.WriteHeader(http.StatusOK)

		// Stream 40KB of lowercase text
		line := []byte("durable execution engine base on webassembly and tinygo stream processing test line.\n")
		for i := 0; i < 500; i++ {
			_, _ = w.Write(line)
		}
	})

	mux.HandleFunc("/upload", func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, 4096)
		totalBytes := 0
		allUppercase := true

		for {
			n, err := r.Body.Read(buf)
			if n > 0 {
				totalBytes += n
				for i := 0; i < n; i++ {
					if buf[i] >= 'a' && buf[i] <= 'z' {
						allUppercase = false
					}
				}
			}
			if err == io.EOF {
				break
			}
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}

		slog.Info("[MOCK SERVER] Received payload", "bytes", totalBytes, "all_uppercase", allUppercase)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	server := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	go func() {
		l, err := net.Listen("tcp", addr)
		if err != nil {
			slog.Error("[MOCK SERVER] Failed to listen", "error", err)
			return
		}
		if err := server.Serve(l); err != nil && err != http.ErrServerClosed {
			slog.Error("[MOCK SERVER] Serve error", "error", err)
		}
	}()

	slog.Info("[MOCK SERVER] Listening", "addr", "http://"+addr)
	return server
}
