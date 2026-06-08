package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/nativebpm/connectors/wasman"
)

func main() {
	// 1. Initialize an in-memory snapshot store.
	// This store persists WASM linear memory snapshots and oplogs entirely in RAM.
	store := wasman.NewMemorySnapshotStore()

	// 2. Load the precompiled WASM module.
	// We use the "dirty_page_oplog.wasm" compiled test module from the package testdata.
	wasmPath := filepath.Join("..", "..", "testdata", "dirty_page_oplog.wasm")
	if _, err := os.Stat(wasmPath); os.IsNotExist(err) {
		// Try fallback if running from root directory
		wasmPath = filepath.Join("connectors", "wasman", "testdata", "dirty_page_oplog.wasm")
		if _, err := os.Stat(wasmPath); os.IsNotExist(err) {
			fmt.Println("Error: dirty_page_oplog.wasm not found. Please compile it first using 'make build-testdata'")
			os.Exit(1)
		}
	}

	fmt.Printf("[HOST] Loading WebAssembly module from: %s\n", wasmPath)
	engine, err := wasman.NewEngine(wasmPath, store)
	if err != nil {
		fmt.Printf("Failed to initialize engine: %v\n", err)
		os.Exit(1)
	}

	// 3. Define the in-memory API Handler callback.
	// The guest WASM logic calls host APIs (such as DB writes, system logs, or HTTP requests).
	// By specifying WithApiHandler, all API calls are routed directly to this Go function closure
	// inside the same address space, bypassing the HTTP network stack entirely.
	apiHandler := func(apiName string, request []byte) ([]byte, error) {
		fmt.Printf("[HOST CALLBACK] API called: %q with request payload: %q\n", apiName, string(request))
		// Return a mock response payload
		return []byte(fmt.Sprintf("mock-response-for-%s", string(request))), nil
	}

	// 4. Configure and run the WASM execution session.
	// We use a fluent builder API to chain the instance configuration.
	// Notice that we DO NOT call `.WithServer(...)` - this ensures no loopback HTTP servers are started,
	// and no local TCP ports are opened or listened on.
	ctx := context.Background()
	instanceID := "in-memory-session-demo"

	fmt.Println("[HOST] Starting execution of WASM session...")
	crashed, err := engine.Session(instanceID).
		WithEntrypoint("run_test").
		WithApiHandler(apiHandler).
		WithCrash(false).
		Run(ctx)

	if err != nil {
		fmt.Printf("Execution failed: %v (crashed: %v)\n", err, crashed)
		os.Exit(1)
	}

	fmt.Println("[HOST] WASM execution completed successfully!")

	// 5. Demonstrate State Resumption / Durability.
	// Since checkpoint() was called inside the WASM core (dirty_page_oplog.go),
	// the engine automatically saved full and delta memory snapshots to the store.
	fmt.Println("[HOST] Loading saved session metadata from store to verify checkpointing...")
	meta, err := store.LoadMetadata(instanceID)
	if err != nil {
		fmt.Printf("Failed to load metadata: %v\n", err)
		os.Exit(1)
	}

	if meta != nil {
		fmt.Printf("[HOST] Saved Session state verified. Current DB version counter: %d\n", meta.Version)
	}
}
