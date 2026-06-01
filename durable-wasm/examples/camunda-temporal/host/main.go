package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/nativebpm/connectors/durable-wasm/host/durable"
)

const (
	instanceID   = "camunda-temporal-tx"
	serverAddr   = "localhost:18083"
	snapshotFile = "camunda-temporal-tx.bin"
)

func main() {
	fmt.Println("[HOST] Starting Camunda-Temporal Orchestration Durable Execution Example...")

	// 1. Clean up old snapshots
	_ = os.Remove(snapshotFile)

	// 2. Start local Mock HTTP Server to mock external billing and CRM API endpoints
	mockServer := startMockServer(serverAddr)
	defer mockServer.Shutdown(context.Background())

	// Give the server a small moment to bind to the port
	time.Sleep(100 * time.Millisecond)

	// 3. Initialize the Reusable Durable WASM Engine
	wasmPath := filepath.Join("..", "worker", "worker.wasm")
	store := &durable.FileSnapshotStore{Dir: "."}
	
	engine, err := durable.NewEngine(wasmPath, store)
	if err != nil {
		fmt.Printf("[HOST ERROR] Failed to initialize engine: %v\n", err)
		os.Exit(1)
	}

	// 4. RUN 1: Execute with simulated crash on the first checkpoint (Step 0)
	fmt.Println("\n--- RUN 1: Starting Camunda Process (Billing & CRM Activity) with Simulated Crash ---")
	crashed, err := engine.Execute(instanceID, "run", serverAddr, true)
	if err != nil {
		if crashed {
			fmt.Printf("[HOST] Orchestrator successfully suspended/crashed: %v\n", err)
		} else {
			fmt.Printf("[HOST ERROR] Execution failed: %v\n", err)
			os.Exit(1)
		}
	}

	// Verify snapshot exists
	if _, err := os.Stat(snapshotFile); os.IsNotExist(err) {
		fmt.Println("[HOST ERROR] Snapshot was not created on checkpoint!")
		os.Exit(1)
	}
	fmt.Println("[HOST] Verified that snapshot file was written to disk.")

	// 5. RUN 2: Restore from checkpoint and resume execution from Step 1 (Billing) and Step 2 (CRM)
	fmt.Println("\n--- RUN 2: Restoring Process State from Snapshot and Resuming execution ---")
	crashed, err = engine.Execute(instanceID, "run", serverAddr, false)
	if err != nil {
		fmt.Printf("[HOST ERROR] Resumed execution failed: %v\n", err)
		os.Exit(1)
	}

	if crashed {
		fmt.Println("[HOST ERROR] Resumed execution crashed unexpectedly!")
		os.Exit(1)
	}

	// 6. Final Clean up
	_ = os.Remove(snapshotFile)
	fmt.Println("\n[HOST] Camunda-Temporal Orchestration example completed successfully.")
	os.Exit(0)
}

func startMockServer(addr string) *http.Server {
	mux := http.NewServeMux()

	// Download route (used for reading responses)
	// We dynamically decide which response to return based on path or simple state.
	// In our case, the worker does a single download (read response) which is the billing status.
	mux.HandleFunc("/download", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		billingResponse := `{"status":"success","reference_code":"REF-BILL-550-OK"}`
		_, _ = w.Write([]byte(billingResponse))
	})

	// Upload route (used for sending requests)
	// Since we upload twice (once for billing, once for CRM), we output the received bodies.
	var uploadCount int
	mux.HandleFunc("/upload", func(w http.ResponseWriter, r *http.Request) {
		uploadCount++
		w.WriteHeader(http.StatusOK)

		body, err := io.ReadAll(r.Body)
		if err != nil {
			fmt.Printf("[MOCK SERVICES ERROR] Failed to read upload: %v\n", err)
			return
		}

		if uploadCount == 1 {
			fmt.Printf("[MOCK BILLING SERVICE] Received charge request: %s\n", string(body))
		} else {
			fmt.Printf("[MOCK CRM SERVICE] Received update status request: %s\n", string(body))
		}
	})

	server := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	go func() {
		l, err := net.Listen("tcp", addr)
		if err != nil {
			fmt.Printf("[MOCK SERVICES ERROR] Failed to listen: %v\n", err)
			return
		}
		if err := server.Serve(l); err != nil && err != http.ErrServerClosed {
			fmt.Printf("[MOCK SERVICES ERROR] Serve error: %v\n", err)
		}
	}()

	fmt.Printf("[MOCK SERVICES] Listening on http://%s\n", addr)
	return server
}
