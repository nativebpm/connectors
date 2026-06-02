package durable

import (
	"context"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"
	"unsafe"

	"github.com/bytecodealliance/wasmtime-go/v20"
	"github.com/nativebpm/httpstream"
)

// OplogEntry represents a single external call log.
type OplogEntry struct {
	CallIndex       int    `json:"call_index"`
	ApiName         string `json:"api_name"`
	RequestPayload  []byte `json:"request_payload"`
	ResponsePayload []byte `json:"response_payload"`
}

// SnapshotStore abstracts the storage backend for linear memory snapshots, deltas, and oplog.
type SnapshotStore interface {
	Save(id string, snapshot []byte) error
	Load(id string) ([]byte, error)
	Delete(id string) error

	// Delta Snapshots
	SaveDeltas(id string, deltas map[int][]byte) error
	LoadDeltas(id string) (map[int][]byte, error)

	// Oplog
	SaveOplog(id string, callIndex int, apiName string, request []byte, response []byte) error
	LoadOplog(id string) ([]OplogEntry, error)
}

// FileSnapshotStore implements SnapshotStore using the local file system.
type FileSnapshotStore struct {
	Dir string
}

func (f *FileSnapshotStore) SaveDeltas(id string, deltas map[int][]byte) error {
	path := fmt.Sprintf("%s_deltas.json", id)
	if f.Dir != "" {
		path = fmt.Sprintf("%s/%s_deltas.json", f.Dir, id)
	}
	current := make(map[int][]byte)
	data, err := os.ReadFile(path)
	if err == nil {
		_ = json.Unmarshal(data, &current)
	}
	for k, v := range deltas {
		current[k] = v
	}
	newData, err := json.Marshal(current)
	if err != nil {
		return err
	}
	return os.WriteFile(path, newData, 0644)
}

func (f *FileSnapshotStore) LoadDeltas(id string) (map[int][]byte, error) {
	path := fmt.Sprintf("%s_deltas.json", id)
	if f.Dir != "" {
		path = fmt.Sprintf("%s/%s_deltas.json", f.Dir, id)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var deltas map[int][]byte
	err = json.Unmarshal(data, &deltas)
	return deltas, err
}

func (f *FileSnapshotStore) SaveOplog(id string, callIndex int, apiName string, request []byte, response []byte) error {
	path := fmt.Sprintf("%s_oplog.json", id)
	if f.Dir != "" {
		path = fmt.Sprintf("%s/%s_oplog.json", f.Dir, id)
	}
	var list []OplogEntry
	data, err := os.ReadFile(path)
	if err == nil {
		_ = json.Unmarshal(data, &list)
	}
	list = append(list, OplogEntry{
		CallIndex:       callIndex,
		ApiName:         apiName,
		RequestPayload:  request,
		ResponsePayload: response,
	})
	newData, err := json.Marshal(list)
	if err != nil {
		return err
	}
	return os.WriteFile(path, newData, 0644)
}

func (f *FileSnapshotStore) LoadOplog(id string) ([]OplogEntry, error) {
	path := fmt.Sprintf("%s_oplog.json", id)
	if f.Dir != "" {
		path = fmt.Sprintf("%s/%s_oplog.json", f.Dir, id)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var list []OplogEntry
	err = json.Unmarshal(data, &list)
	return list, err
}

func (f *FileSnapshotStore) Delete(id string) error {
	path := fmt.Sprintf("%s.bin", id)
	pathDeltas := fmt.Sprintf("%s_deltas.json", id)
	pathOplog := fmt.Sprintf("%s_oplog.json", id)
	if f.Dir != "" {
		path = fmt.Sprintf("%s/%s.bin", f.Dir, id)
		pathDeltas = fmt.Sprintf("%s/%s_deltas.json", f.Dir, id)
		pathOplog = fmt.Sprintf("%s/%s_oplog.json", f.Dir, id)
	}
	_ = os.Remove(path)
	_ = os.Remove(pathDeltas)
	_ = os.Remove(pathOplog)
	return nil
}

// Engine coordinates execution, compilation, and snapshotting of WASM modules.
type Engine struct {
	wasmEngine *wasmtime.Engine
	module     *wasmtime.Module
	store      SnapshotStore
	httpClient *http.Client
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

	// Dirty-page snapshot hashes and Oplog progress
	pageHashes map[int]uint64
	callIndex  int

	// Upload Stream-first context
	uploadPipeW   *io.PipeWriter
	uploadErrChan chan error

	// Download Stream-first context
	downloadResp *http.Response
	downloadEOF  bool
}

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
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     90 * time.Second,
			},
		},
	}, nil
}

// nolint:gocyclo
// Execute runs the WASM instance with a given entrypoint and session context.
// If it finds a saved snapshot, it automatically restores the linear memory.
func (e *Engine) Execute(instanceID string, entrypoint string, serverAddr string, shouldCrash bool) (bool, error) {
	session := &Session{
		engine:                  e,
		instanceID:              instanceID,
		serverAddr:              serverAddr,
		shouldCrashOnCheckpoint: shouldCrash,
	}

	// Guarantee cleanup of HTTP connections and pipes on return
	defer func() {
		if session.downloadResp != nil {
			session.downloadResp.Body.Close()
		}
		if session.uploadPipeW != nil {
			session.uploadPipeW.Close()
		}
	}()

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

	// Register Host Function: checkpoint (using local closure)
	err = linker.DefineFunc(store, "env", "checkpoint", func(caller *wasmtime.Caller) *wasmtime.Trap {
		slog.Info("[ENGINE] 'checkpoint' invoked", "instance_id", session.instanceID)

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

		// 1. Full Snapshot (backward compatibility)
		snapshotCopy := make([]byte, len(memoryBytes))
		copy(snapshotCopy, memoryBytes)
		err := e.store.Save(session.instanceID, snapshotCopy)
		if err != nil {
			slog.Error("[ENGINE] Failed to save full snapshot", "error", err)
			return wasmtime.NewTrap("failed to write snapshot")
		}

		// 2. Dirty-Page delta snapshotting
		blockSize := 4096
		numBlocks := len(memoryBytes) / blockSize
		deltas := make(map[int][]byte)
		for i := 0; i < numBlocks; i++ {
			start := i * blockSize
			end := start + blockSize
			if end > len(memoryBytes) {
				end = len(memoryBytes)
			}
			blockData := memoryBytes[start:end]
			h := fnv.New64a()
			h.Write(blockData)
			hashVal := h.Sum64()

			prevHash, exists := session.pageHashes[i]
			if !exists || prevHash != hashVal {
				blockCopy := make([]byte, len(blockData))
				copy(blockCopy, blockData)
				deltas[i] = blockCopy
				session.pageHashes[i] = hashVal
			}
		}

		if len(deltas) > 0 {
			err = e.store.SaveDeltas(session.instanceID, deltas)
			if err != nil {
				slog.Error("[ENGINE] Failed to save memory deltas", "error", err)
				return wasmtime.NewTrap("failed to write memory deltas")
			}
			slog.Info("[ENGINE] Memory deltas successfully saved", "dirty_blocks", len(deltas))
		}

		if session.shouldCrashOnCheckpoint {
			session.crashed = true
			slog.Warn("[ENGINE] Simulating host crash. Aborting WASM execution.")
			return wasmtime.NewTrap("simulated_host_crash")
		}

		return nil
	})
	if err != nil {
		return false, fmt.Errorf("failed to register 'checkpoint': %w", err)
	}

	// Register Host Function: host_call_api
	err = linker.DefineFunc(store, "env", "host_call_api", func(
		caller *wasmtime.Caller,
		apiNamePtr int32, apiNameLen int32,
		reqPtr int32, reqLen int32,
		respPtr int32, respMaxLen int32,
	) int32 {

		ext := caller.GetExport("memory")
		if ext == nil {
			slog.Error("[ENGINE] host_call_api: memory export not found")
			return -1
		}
		mem := ext.Memory()
		session.memory = mem

		mPtr := mem.Data(store)
		mSize := mem.DataSize(store)
		memoryBytes := unsafe.Slice((*byte)(mPtr), mSize)

		if apiNamePtr < 0 || apiNameLen < 0 || int(apiNamePtr)+int(apiNameLen) > len(memoryBytes) {
			slog.Error("[ENGINE] host_call_api: memory access out of bounds for api name")
			return -1
		}
		apiName := string(memoryBytes[apiNamePtr : apiNamePtr+apiNameLen])

		if reqPtr < 0 || reqLen < 0 || int(reqPtr)+int(reqLen) > len(memoryBytes) {
			slog.Error("[ENGINE] host_call_api: memory access out of bounds for request")
			return -1
		}
		request := memoryBytes[reqPtr : reqPtr+reqLen]

		session.callIndex++
		callIdx := session.callIndex

		// Check Oplog Replay
		oplog, err := e.store.LoadOplog(session.instanceID)
		if err == nil {
			for _, entry := range oplog {
				if entry.CallIndex == callIdx {
					slog.Info("[ENGINE] Oplog Replay", "api", apiName, "call_index", callIdx)
					if int(respMaxLen) < len(entry.ResponsePayload) {
						slog.Error("[ENGINE] Oplog Replay: response buffer too small")
						return -2
					}
					copy(memoryBytes[respPtr:respPtr+int32(len(entry.ResponsePayload))], entry.ResponsePayload)
					return int32(len(entry.ResponsePayload))
				}
			}
		}

		// Perform real operation
		slog.Info("[ENGINE] Oplog Execution: invoking real API call", "api", apiName, "call_index", callIdx)
		var response []byte

		if apiName == "test_api" {
			// For testing / simulation purposes
			time.Sleep(10 * time.Millisecond)
			response = []byte(fmt.Sprintf("resp_for_%s_call_%d", string(request), callIdx))
		} else {
			// Real external REST API call if endpoint matches serverAddr
			url := fmt.Sprintf("http://%s/%s", session.serverAddr, apiName)
			resp, err := e.httpClient.Post(url, "application/octet-stream", io.NopCloser(strings.NewReader(string(request))))
			if err != nil {
				slog.Error("[ENGINE] Real API call failed", "url", url, "error", err)
				return -1
			}
			defer resp.Body.Close()
			response, _ = io.ReadAll(resp.Body)
		}

		// Save response to Oplog
		err = e.store.SaveOplog(session.instanceID, callIdx, apiName, request, response)
		if err != nil {
			slog.Error("[ENGINE] Failed to save Oplog", "error", err)
			return -1
		}

		if int(respMaxLen) < len(response) {
			slog.Error("[ENGINE] Response buffer too small for live response")
			return -2
		}
		copy(memoryBytes[respPtr:respPtr+int32(len(response))], response)
		return int32(len(response))
	})
	if err != nil {
		return false, fmt.Errorf("failed to register 'host_call_api': %w", err)
	}

	// Register Host Function: stream_data (using local closure)
	err = linker.DefineFunc(store, "env", "stream_data", func(caller *wasmtime.Caller, direction int32, ptr int32, length int32) int32 {
		ext := caller.GetExport("memory")
		if ext == nil {
			slog.Error("[ENGINE] stream_data: memory export not found")
			return -1
		}
		mem := ext.Memory()
		session.memory = mem

		if direction == 0 {
			return session.handleDownload(ptr, length)
		} else if direction == 1 {
			return session.handleUpload(ptr, length)
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

	// RESTORE: Check if there is an existing snapshot to restore (tries deltas first, then full snapshot)
	deltas, err := e.store.LoadDeltas(instanceID)
	if err == nil && len(deltas) > 0 {
		slog.Info("[ENGINE] Found saved memory deltas. Restoring memory...", "instance_id", instanceID)
		maxPageIndex := 0
		for idx := range deltas {
			if idx > maxPageIndex {
				maxPageIndex = idx
			}
		}
		neededSize := (maxPageIndex + 1) * 4096
		neededPages := (uint64(neededSize) + 65535) / 65536
		currentPages := session.memory.Size(store)
		if neededPages > currentPages {
			growPages := neededPages - currentPages
			slog.Info("[ENGINE] Growing memory for deltas", "pages", growPages)
			_, err = session.memory.Grow(store, growPages)
			if err != nil {
				return false, fmt.Errorf("failed to grow memory for deltas: %w", err)
			}
		}
		ptr := session.memory.Data(store)
		size := session.memory.DataSize(store)
		memoryBytes := unsafe.Slice((*byte)(ptr), size)

		session.pageHashes = make(map[int]uint64)
		for idx, data := range deltas {
			start := idx * 4096
			copy(memoryBytes[start:start+len(data)], data)
			h := fnv.New64a()
			h.Write(data)
			session.pageHashes[idx] = h.Sum64()
		}
		slog.Info("[ENGINE] Memory successfully restored from deltas", "restored_pages", len(deltas))
	} else {
		session.pageHashes = make(map[int]uint64)
		snapshot, err := e.store.Load(instanceID)
		if err == nil && len(snapshot) > 0 {
			slog.Info("[ENGINE] Found saved full snapshot. Restoring memory...", "instance_id", instanceID)
			currentPages := session.memory.Size(store)
			neededPages := (uint64(len(snapshot)) + 65535) / 65536
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

			// Populate base page hashes
			blockSize := 4096
			numBlocks := len(memoryBytes) / blockSize
			for i := 0; i < numBlocks; i++ {
				start := i * blockSize
				end := start + blockSize
				h := fnv.New64a()
				h.Write(memoryBytes[start:end])
				session.pageHashes[i] = h.Sum64()
			}
			slog.Info("[ENGINE] Memory full snapshot successfully restored")
		}
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

	mPtr := s.memory.Data(s.store)
	mSize := s.memory.DataSize(s.store)
	memoryBytes := unsafe.Slice((*byte)(mPtr), mSize)

	// Validate bounds before copy
	if ptr < 0 || length < 0 || int(ptr)+int(length) > len(memoryBytes) {
		slog.Error("[ENGINE] Memory access out of bounds in handleDownload", "ptr", ptr, "length", length, "mem_size", len(memoryBytes))
		return -1
	}

	if s.downloadResp == nil {
		url := fmt.Sprintf("http://%s/download", s.serverAddr)
		slog.Info("[ENGINE] GET Request (Stream-first)", "url", url)
		resp, err := httpstream.NewRequest(context.Background(), *s.engine.httpClient, "GET", url).Send()
		if err != nil {
			slog.Error("[ENGINE] GET failed", "error", err)
			return -1
		}
		s.downloadResp = resp
	}

	buf := make([]byte, length)
	n, err := s.downloadResp.Body.Read(buf)
	if n > 0 {
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
	mPtr := s.memory.Data(s.store)
	mSize := s.memory.DataSize(s.store)
	memoryBytes := unsafe.Slice((*byte)(mPtr), mSize)

	// Validate bounds before access
	if ptr < 0 || length < 0 || int(ptr)+int(length) > len(memoryBytes) {
		slog.Error("[ENGINE] Memory access out of bounds in handleUpload", "ptr", ptr, "length", length, "mem_size", len(memoryBytes))
		return -1
	}

	if s.uploadPipeW == nil {
		url := fmt.Sprintf("http://%s/upload", s.serverAddr)
		slog.Info("[ENGINE] POST Request (Stream-first via io.Pipe)", "url", url)

		pipeReader, pipeWriter := io.Pipe()
		s.uploadPipeW = pipeWriter
		s.uploadErrChan = make(chan error, 1)

		go func() {
			resp, err := httpstream.NewRequest(context.Background(), *s.engine.httpClient, "POST", url).
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

	dataToWrite := memoryBytes[ptr : ptr+length]
	n, err := s.uploadPipeW.Write(dataToWrite)
	if err != nil {
		slog.Error("[ENGINE] Write to pipe failed", "error", err)
		return -1
	}

	return int32(n)
}
