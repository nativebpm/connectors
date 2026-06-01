package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/bytecodealliance/wasmtime-go/v20"
)

// Session holds the runtime state of the current WASM execution session.
type Session struct {
	engine     *wasmtime.Engine
	store      *wasmtime.Store
	memory     *wasmtime.Memory
	httpClient *http.Client
	serverAddr string

	// Simulated crash flag
	shouldCrashOnCheckpoint bool
	crashed                 bool

	// State for Stream-first HTTP Upload
	uploadPipeW   *io.PipeWriter
	uploadErrChan chan error

	// State for Stream-first HTTP Download
	downloadResp *http.Response
}

// Global active session pointer.
// Since host functions are registered in the Wasmtime Linker and called by the WASM instance,
// they need a way to access the current session context.
var (
	activeSession *Session
	sessionMutex  sync.Mutex
)

const (
	snapshotFile = "snapshot.bin"
	serverPort   = 18080
)

func main() {
	fmt.Println("[HOST] Starting Durable WASM Execution Engine Orchestrator...")

	// 1. Start the mock HTTP Server on a background goroutine
	serverAddr := fmt.Sprintf("localhost:%d", serverPort)
	mockServer := startMockServer(serverAddr)
	defer mockServer.Shutdown(context.Background())

	// Wait briefly for the server to bind to the port
	time.Sleep(100 * time.Millisecond)

	// Clean up any stale snapshot from previous runs
	_ = os.Remove(snapshotFile)

	// 2. Initialize the WASM Engine and Compile the Module
	engine := wasmtime.NewEngine()
	wasmPath := filepath.Join("..", "worker", "worker.wasm")
	module, err := wasmtime.NewModuleFromFile(engine, wasmPath)
	if err != nil {
		fmt.Printf("[HOST ERROR] Failed to compile WASM module: %v\n", err)
		fmt.Println("[HOST ERROR] Make sure to build the worker first by running 'make build-worker'")
		os.Exit(1)
	}

	// 3. First execution run (normal boot -> checkpoint -> simulated crash)
	fmt.Println("\n--- RUN 1: Booting from scratch, simulating crash at step 0 ---")
	session1 := &Session{
		engine:                  engine,
		httpClient:              &http.Client{Timeout: 30 * time.Second},
		serverAddr:              serverAddr,
		shouldCrashOnCheckpoint: true, // We want to crash at the first checkpoint
	}

	err = runWasmInstance(session1, module, "run", nil)
	if err != nil {
		if session1.crashed {
			fmt.Printf("[HOST] Execution interrupted as expected: %v\n", err)
		} else {
			fmt.Printf("[HOST ERROR] Execution failed unexpectedly: %v\n", err)
			os.Exit(1)
		}
	}

	// Verify that a snapshot was actually written to disk
	if _, err := os.Stat(snapshotFile); os.IsNotExist(err) {
		fmt.Println("[HOST ERROR] Snapshot file was not created!")
		os.Exit(1)
	}
	fmt.Println("[HOST] Verified that snapshot.bin was successfully written to disk.")

	// 4. Second execution run (restore from snapshot -> resume step 1 & 2 -> complete)
	fmt.Println("\n--- RUN 2: Restoring from snapshot.bin and resuming execution ---")
	snapshotBytes, err := os.ReadFile(snapshotFile)
	if err != nil {
		fmt.Printf("[HOST ERROR] Failed to read snapshot file: %v\n", err)
		os.Exit(1)
	}

	session2 := &Session{
		engine:                  engine,
		httpClient:              &http.Client{Timeout: 30 * time.Second},
		serverAddr:              serverAddr,
		shouldCrashOnCheckpoint: false, // Do not crash this time
	}

	err = runWasmInstance(session2, module, "run", snapshotBytes)
	if err != nil {
		fmt.Printf("[HOST ERROR] Resumed execution failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("\n--- Durable Execution Lifecycle Completed Successfully ---")
}

// runWasmInstance initializes the Wasmtime environment, registers host functions,
// optionally restores the linear memory from a snapshot, and invokes the entrypoint.
func runWasmInstance(session *Session, module *wasmtime.Module, entrypoint string, snapshot []byte) error {
	// Set active session for the host function callbacks
	sessionMutex.Lock()
	activeSession = session
	sessionMutex.Unlock()

	// Create a store for this instance
	store := wasmtime.NewStore(session.engine)
	session.store = store

	// Set up WASI configuration
	wasiConfig := wasmtime.NewWasiConfig()
	wasiConfig.InheritStdout()
	wasiConfig.InheritStderr()
	store.SetWasi(wasiConfig)

	// Create Linker to resolve imports
	linker := wasmtime.NewLinker(session.engine)
	err := linker.DefineWasi()
	if err != nil {
		return fmt.Errorf("failed to link WASI: %w", err)
	}

	// Register Host Function: checkpoint
	err = linker.DefineFunc(store, "env", "checkpoint", func(caller *wasmtime.Caller) *wasmtime.Trap {
		sessionMutex.Lock()
		s := activeSession
		sessionMutex.Unlock()

		fmt.Println("[HOST] 'checkpoint' called by WASM worker.")

		// Get the exported memory from the caller
		ext := caller.GetExport("memory")
		if ext == nil {
			return wasmtime.NewTrap("failed to export memory")
		}
		mem := ext.Memory()

		// Read the raw linear memory
		memoryBytes := mem.Data(store)

		// Create a copy of the memory data to prevent modifications before/during disk write
		snapshotData := make([]byte, len(memoryBytes))
		copy(snapshotData, memoryBytes)

		// Save the memory snapshot to disk
		err := os.WriteFile(snapshotFile, snapshotData, 0644)
		if err != nil {
			fmt.Printf("[HOST ERROR] Failed to save snapshot to disk: %v\n", err)
			return wasmtime.NewTrap("failed to write snapshot file")
		}
		fmt.Printf("[HOST] Saved memory snapshot (%d bytes) to disk.\n", len(snapshotData))

		// If we are simulating a crash, return a trap to abort the execution
		if s.shouldCrashOnCheckpoint {
			s.crashed = true
			fmt.Println("[HOST] Simulating host crash. Aborting WASM execution.")
			return wasmtime.NewTrap("simulated_host_crash")
		}

		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to register 'checkpoint' host function: %w", err)
	}

	// Register Host Function: stream_data
	err = linker.DefineFunc(store, "env", "stream_data", func(caller *wasmtime.Caller, direction int32, ptr int32, length int32) int32 {
		sessionMutex.Lock()
		s := activeSession
		sessionMutex.Unlock()

		// Fetch memory export
		ext := caller.GetExport("memory")
		if ext == nil {
			fmt.Println("[HOST ERROR] stream_data: memory export not found")
			return -1
		}
		mem := ext.Memory()
		session.memory = mem

		if direction == 0 {
			// direction = 0: Read from network (Download) into WASM memory
			return s.handleDownload(ptr, length)
		} else if direction == 1 {
			// direction = 1: Write to network (Upload) from WASM memory
			return s.handleUpload(ptr, length)
		}

		fmt.Printf("[HOST ERROR] stream_data: invalid direction %d\n", direction)
		return -1
	})
	if err != nil {
		return fmt.Errorf("failed to register 'stream_data' host function: %w", err)
	}

	// Instantiate the WASM module
	instance, err := linker.Instantiate(store, module)
	if err != nil {
		return fmt.Errorf("failed to instantiate WASM module: %w", err)
	}

	// Locate the memory export to save it in session for convenience
	ext := instance.GetExport(store, "memory")
	if ext == nil {
		return fmt.Errorf("failed to find memory export on instantiation")
	}
	session.memory = ext.Memory()

	// RESTORE MECHANISM: If we have a memory snapshot, restore it into the new instance
	if len(snapshot) > 0 {
		fmt.Printf("[HOST] Restoring linear memory snapshot (%d bytes) into clean WASM instance...\n", len(snapshot))

		// Ensure the new instance's linear memory is large enough to hold the snapshot
		currentPages := session.memory.Size(store)
		neededPages := uint64((len(snapshot) + 65535) / 65536)

		if neededPages > currentPages {
			growPages := neededPages - currentPages
			fmt.Printf("[HOST] Growing WASM memory by %d pages to match snapshot size...\n", growPages)
			_, err = session.memory.Grow(store, growPages)
			if err != nil {
				return fmt.Errorf("failed to grow WASM memory for snapshot: %w", err)
			}
		}

		// Write snapshot data directly into WASM linear memory
		memoryBytes := session.memory.Data(store)
		copy(memoryBytes, snapshot)
		fmt.Println("[HOST] Memory snapshot successfully restored.")
	}

	// Find the entrypoint function
	runFunc := instance.GetFunc(store, entrypoint)
	if runFunc == nil {
		return fmt.Errorf("entrypoint function '%s' not found", entrypoint)
	}

	// Call the entrypoint function
	fmt.Printf("[HOST] Calling entrypoint function '%s'...\n", entrypoint)
	result, err := runFunc.Call(store)
	if err != nil {
		return err
	}

	if len(result) > 0 {
		fmt.Printf("[HOST] Worker returned: %v\n", result[0])
	}
	return nil
}

// handleDownload downloads chunks from the HTTP server and writes them into WASM memory.
// Strict O(1) memory usage: reads only what was requested.
func (s *Session) handleDownload(ptr int32, length int32) int32 {
	if s.downloadResp == nil {
		url := fmt.Sprintf("http://%s/download", s.serverAddr)
		fmt.Printf("[HOST] Initiating HTTP GET request to %s (Stream-first)\n", url)
		resp, err := s.httpClient.Get(url)
		if err != nil {
			fmt.Printf("[HOST ERROR] HTTP GET request failed: %v\n", err)
			return -1
		}
		s.downloadResp = resp
	}

	// Read up to 'length' bytes directly from the response body
	buf := make([]byte, length)
	n, err := s.downloadResp.Body.Read(buf)
	if n > 0 {
		// Copy read bytes directly into WASM linear memory
		memData := s.memory.Data(s.store)
		copy(memData[ptr:ptr+int32(n)], buf[:n])
	}

	if err == io.EOF {
		fmt.Println("[HOST] HTTP GET stream reached EOF. Closing response body.")
		s.downloadResp.Body.Close()
		s.downloadResp = nil
		return int32(n)
	}

	if err != nil {
		fmt.Printf("[HOST ERROR] Failed to read from HTTP GET stream: %v\n", err)
		s.downloadResp.Body.Close()
		s.downloadResp = nil
		return -1
	}

	return int32(n)
}

// handleUpload uploads chunks from WASM memory to the HTTP server.
// Strict O(1) memory usage: data flows through io.Pipe directly to the HTTP request body.
func (s *Session) handleUpload(ptr int32, length int32) int32 {
	if s.uploadPipeW == nil {
		url := fmt.Sprintf("http://%s/upload", s.serverAddr)
		fmt.Printf("[HOST] Initiating HTTP POST request to %s (Stream-first via io.Pipe)\n", url)

		pipeReader, pipeWriter := io.Pipe()
		s.uploadPipeW = pipeWriter
		s.uploadErrChan = make(chan error, 1)

		// Start HTTP client request in a background goroutine since writing to pipe is blocking
		go func() {
			req, err := http.NewRequest("POST", url, pipeReader)
			if err != nil {
				pipeReader.CloseWithError(err)
				s.uploadErrChan <- err
				return
			}
			req.Header.Set("Content-Type", "application/octet-stream")

			resp, err := s.httpClient.Do(req)
			if err != nil {
				pipeReader.CloseWithError(err)
				s.uploadErrChan <- err
				return
			}
			defer resp.Body.Close()

			// Read response to ensure server processed the whole body
			_, _ = io.Copy(io.Discard, resp.Body)
			s.uploadErrChan <- nil
		}()
	}

	// If length is 0, it signals that the stream is finished
	if length == 0 {
		fmt.Println("[HOST] Closing upload stream (EOF). Waiting for HTTP response...")
		s.uploadPipeW.Close()
		err := <-s.uploadErrChan
		s.uploadPipeW = nil
		if err != nil {
			fmt.Printf("[HOST ERROR] HTTP POST request failed: %v\n", err)
			return -1
		}
		fmt.Println("[HOST] HTTP POST request completed successfully.")
		return 0
	}

	// Write data from WASM memory directly into the pipe writer
	memData := s.memory.Data(s.store)
	dataToWrite := memData[ptr : ptr+length]

	n, err := s.uploadPipeW.Write(dataToWrite)
	if err != nil {
		fmt.Printf("[HOST ERROR] Failed writing chunk to upload pipe: %v\n", err)
		return -1
	}

	return int32(n)
}

// startMockServer starts a local HTTP server that emulates file download and upload endpoints.
func startMockServer(addr string) *http.Server {
	mux := http.NewServeMux()

	// Download route: generates and streams a long text in lower case.
	mux.HandleFunc("/download", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.WriteHeader(http.StatusOK)

		// Generate 50KB of lowercase text to download
		line := []byte("durable execution engine base on webassembly and tinygo stream processing test line.\n")
		for i := 0; i < 600; i++ {
			_, _ = w.Write(line)
		}
	})

	// Upload route: reads data and prints stats to verify it is uppercase.
	mux.HandleFunc("/upload", func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, 4096)
		totalBytes := 0
		allUppercase := true

		for {
			n, err := r.Body.Read(buf)
			if n > 0 {
				totalBytes += n
				// Verify if the transformation in WASM succeeded
				for i := 0; i < n; i++ {
					// If we find any lowercase letters, mark validation failed
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

		fmt.Printf("[MOCK SERVER] Received total %d bytes. All Uppercase: %t\n", totalBytes, allUppercase)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	server := &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	// Run server in background
	go func() {
		// Use custom listener to fail fast if port is taken
		l, err := net.Listen("tcp", addr)
		if err != nil {
			fmt.Printf("[MOCK SERVER ERROR] Failed to listen on %s: %v\n", addr, err)
			return
		}
		if err := server.Serve(l); err != nil && err != http.ErrServerClosed {
			fmt.Printf("[MOCK SERVER ERROR] Serve error: %v\n", err)
		}
	}()

	fmt.Printf("[MOCK SERVER] Listening on http://%s\n", addr)
	return server
}
