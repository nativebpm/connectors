package durable

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"sync"
	"time"
	"unsafe"

	"github.com/bytecodealliance/wasmtime-go/v20"
	"github.com/nativebpm/httpstream"
)

// SnapshotStore abstracts the storage backend for linear memory snapshots.
type SnapshotStore interface {
	Save(id string, snapshot []byte) error
	Load(id string) ([]byte, error)
	Delete(id string) error
}

// FileSnapshotStore implements SnapshotStore using the local file system.
type FileSnapshotStore struct {
	Dir string
}

func (f *FileSnapshotStore) Save(id string, snapshot []byte) error {
	path := fmt.Sprintf("%s.bin", id)
	if f.Dir != "" {
		path = fmt.Sprintf("%s/%s.bin", f.Dir, id)
	}
	return os.WriteFile(path, snapshot, 0644)
}

func (f *FileSnapshotStore) Load(id string) ([]byte, error) {
	path := fmt.Sprintf("%s.bin", id)
	if f.Dir != "" {
		path = fmt.Sprintf("%s/%s.bin", f.Dir, id)
	}
	return os.ReadFile(path)
}

func (f *FileSnapshotStore) Delete(id string) error {
	path := fmt.Sprintf("%s.bin", id)
	if f.Dir != "" {
		path = fmt.Sprintf("%s/%s.bin", f.Dir, id)
	}
	err := os.Remove(path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}


// Engine coordinates execution, compilation, and snapshotting of WASM modules.
type Engine struct {
	wasmEngine *wasmtime.Engine
	module     *wasmtime.Module
	store      SnapshotStore
}

// Session tracks the dynamic execution state of a running WASM instance.
type Session struct {
	engine                  *Engine
	store                   *wasmtime.Store
	memory                  *wasmtime.Memory
	instanceID              string
	serverAddr              string
	shouldCrashOnCheckpoint bool
	crashed                 bool
	httpClient              *http.Client

	// Upload Stream-first context
	uploadPipeW   *io.PipeWriter
	uploadErrChan chan error

	// Download Stream-first context
	downloadResp *http.Response
	downloadEOF  bool
}

// Active session registry for Wasmtime host callbacks.
var (
	activeSession *Session
	sessionMutex  sync.Mutex
)

// NewEngine creates a new reusable WASM Durable Execution Engine.
func NewEngine(wasmPath string, store SnapshotStore) (*Engine, error) {
	wasmEngine := wasmtime.NewEngine()
	module, err := wasmtime.NewModuleFromFile(wasmEngine, wasmPath)
	if err != nil {
		return nil, fmt.Errorf("failed to compile WASM module: %w", err)
	}

	return &Engine{
		wasmEngine: wasmEngine,
		module:     module,
		store:      store,
	}, nil
}

// Execute runs the WASM instance with a given entrypoint and session context.
// If it finds a saved snapshot, it automatically restores the linear memory.
func (e *Engine) Execute(instanceID string, entrypoint string, serverAddr string, shouldCrash bool) (bool, error) {
	sessionMutex.Lock()
	session := &Session{
		engine:                  e,
		instanceID:              instanceID,
		serverAddr:              serverAddr,
		shouldCrashOnCheckpoint: shouldCrash,
		httpClient:              &http.Client{Timeout: 30 * time.Second},
	}
	activeSession = session
	sessionMutex.Unlock()

	store := wasmtime.NewStore(e.wasmEngine)
	session.store = store

	// Configure WASI
	wasiConfig := wasmtime.NewWasiConfig()
	wasiConfig.InheritStdout()
	wasiConfig.InheritStderr()
	store.SetWasi(wasiConfig)

	// Create Linker and define WASI imports
	linker := wasmtime.NewLinker(e.wasmEngine)
	err := linker.DefineWasi()
	if err != nil {
		return false, fmt.Errorf("failed to link WASI: %w", err)
	}

	// Register Host Function: checkpoint
	err = linker.DefineFunc(store, "env", "checkpoint", func(caller *wasmtime.Caller) *wasmtime.Trap {
		sessionMutex.Lock()
		s := activeSession
		sessionMutex.Unlock()

		slog.Info("[ENGINE] 'checkpoint' invoked", "instance_id", s.instanceID)

		ext := caller.GetExport("memory")
		if ext == nil {
			return wasmtime.NewTrap("memory export not found")
		}
		mem := ext.Memory()

		// Read and snapshot the linear memory safely using unsafe.Slice
		ptr := mem.Data(store)
		size := mem.DataSize(store)
		if size == 0 {
			return wasmtime.NewTrap("memory data size is zero")
		}
		memoryBytes := unsafe.Slice((*byte)(ptr), size)

		snapshotCopy := make([]byte, len(memoryBytes))
		copy(snapshotCopy, memoryBytes)

		// Save memory snapshot using the SnapshotStore interface
		err := s.engine.store.Save(s.instanceID, snapshotCopy)
		if err != nil {
			slog.Error("[ENGINE] Failed to save snapshot", "error", err)
			return wasmtime.NewTrap("failed to write snapshot")
		}
		slog.Info("[ENGINE] Snapshot successfully saved", "bytes", len(snapshotCopy))

		if s.shouldCrashOnCheckpoint {
			s.crashed = true
			slog.Warn("[ENGINE] Simulating host crash. Aborting WASM execution.")
			return wasmtime.NewTrap("simulated_host_crash")
		}

		return nil
	})
	if err != nil {
		return false, fmt.Errorf("failed to register 'checkpoint': %w", err)
	}

	// Register Host Function: stream_data
	err = linker.DefineFunc(store, "env", "stream_data", func(caller *wasmtime.Caller, direction int32, ptr int32, length int32) int32 {
		sessionMutex.Lock()
		s := activeSession
		sessionMutex.Unlock()

		ext := caller.GetExport("memory")
		if ext == nil {
			slog.Error("[ENGINE] stream_data: memory export not found")
			return -1
		}
		mem := ext.Memory()
		s.memory = mem

		if direction == 0 {
			return s.handleDownload(ptr, length)
		} else if direction == 1 {
			return s.handleUpload(ptr, length)
		}

		slog.Error("[ENGINE] stream_data: invalid direction", "direction", direction)
		return -1
	})
	if err != nil {
		return false, fmt.Errorf("failed to register 'stream_data': %w", err)
	}

	// Instantiate the WASM module
	instance, err := linker.Instantiate(store, e.module)
	if err != nil {
		return false, fmt.Errorf("failed to instantiate WASM: %w", err)
	}

	// Fetch memory export from the new instance
	ext := instance.GetExport(store, "memory")
	if ext == nil {
		return false, fmt.Errorf("failed to find memory export on instantiation")
	}
	session.memory = ext.Memory()

	// RESTORE: Check if there is an existing snapshot to restore
	snapshot, err := e.store.Load(instanceID)
	if err == nil && len(snapshot) > 0 {
		slog.Info("[ENGINE] Found saved snapshot. Restoring memory...", "instance_id", instanceID)
		
		currentPages := session.memory.Size(store)
		neededPages := uint64((len(snapshot) + 65535) / 65536)

		if neededPages > currentPages {
			growPages := neededPages - currentPages
			slog.Info("[ENGINE] Growing memory", "pages", growPages)
			_, err = session.memory.Grow(store, growPages)
			if err != nil {
				return false, fmt.Errorf("failed to grow memory for snapshot: %w", err)
			}
		}

		ptr := session.memory.Data(store)
		size := session.memory.DataSize(store)
		memoryBytes := unsafe.Slice((*byte)(ptr), size)
		copy(memoryBytes, snapshot)
		slog.Info("[ENGINE] Memory snapshot successfully restored")
	}

	// Locate entrypoint
	runFunc := instance.GetFunc(store, entrypoint)
	if runFunc == nil {
		return false, fmt.Errorf("entrypoint function '%s' not found", entrypoint)
	}

	slog.Info("[ENGINE] Invoking entrypoint", "entrypoint", entrypoint)
	result, err := runFunc.Call(store)
	if err != nil {
		if session.crashed {
			return true, err // True indicates a simulated crash occurred
		}
		return false, err
	}

	if result != nil {
		slog.Info("[ENGINE] Execution completed", "result", result)
	} else {
		slog.Info("[ENGINE] Execution completed successfully with no return value")
	}

	return false, nil
}

func (s *Session) handleDownload(ptr int32, length int32) int32 {
	if s.downloadEOF {
		return 0
	}

	if s.downloadResp == nil {
		url := fmt.Sprintf("http://%s/download", s.serverAddr)
		slog.Info("[ENGINE] GET Request (Stream-first)", "url", url)
		resp, err := httpstream.NewRequest(context.Background(), *s.httpClient, "GET", url).Send()
		if err != nil {
			slog.Error("[ENGINE] GET failed", "error", err)
			return -1
		}
		s.downloadResp = resp
	}

	buf := make([]byte, length)
	n, err := s.downloadResp.Body.Read(buf)
	if n > 0 {
		mPtr := s.memory.Data(s.store)
		mSize := s.memory.DataSize(s.store)
		memoryBytes := unsafe.Slice((*byte)(mPtr), mSize)
		copy(memoryBytes[ptr:ptr+int32(n)], buf[:n])
	}

	if err == io.EOF {
		slog.Info("[ENGINE] GET Stream EOF. Closing response")
		s.downloadResp.Body.Close()
		s.downloadResp = nil
		s.downloadEOF = true
		return int32(n)
	}

	if err != nil {
		slog.Error("[ENGINE] Read failed", "error", err)
		s.downloadResp.Body.Close()
		s.downloadResp = nil
		return -1
	}

	return int32(n)
}

func (s *Session) handleUpload(ptr int32, length int32) int32 {
	if s.uploadPipeW == nil {
		url := fmt.Sprintf("http://%s/upload", s.serverAddr)
		slog.Info("[ENGINE] POST Request (Stream-first via io.Pipe)", "url", url)

		pipeReader, pipeWriter := io.Pipe()
		s.uploadPipeW = pipeWriter
		s.uploadErrChan = make(chan error, 1)

		go func() {
			resp, err := httpstream.NewRequest(context.Background(), *s.httpClient, "POST", url).
				Body(pipeReader, "application/octet-stream").
				Send()
			if err != nil {
				pipeReader.CloseWithError(err)
				s.uploadErrChan <- err
				return
			}
			defer resp.Body.Close()

			_, _ = io.Copy(io.Discard, resp.Body)
			s.uploadErrChan <- nil
		}()
	}

	if length == 0 {
		slog.Info("[ENGINE] Closing upload stream (EOF). Waiting for response")
		s.uploadPipeW.Close()
		err := <-s.uploadErrChan
		s.uploadPipeW = nil

		// Reset download stream state to allow next download requests
		s.downloadResp = nil
		s.downloadEOF = false

		if err != nil {
			slog.Error("[ENGINE] POST failed", "error", err)
			return -1
		}
		slog.Info("[ENGINE] POST completed successfully")
		return 0
	}

	mPtr := s.memory.Data(s.store)
	mSize := s.memory.DataSize(s.store)
	memoryBytes := unsafe.Slice((*byte)(mPtr), mSize)
	dataToWrite := memoryBytes[ptr : ptr+length]

	n, err := s.uploadPipeW.Write(dataToWrite)
	if err != nil {
		slog.Error("[ENGINE] Write to pipe failed", "error", err)
		return -1
	}

	return int32(n)
}
