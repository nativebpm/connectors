package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/nativebpm/connectors/wasmee"
)

// memoryStore implements olme.SnapshotStore in memory.
type memoryStore struct {
	snapshots map[string][]byte
	deltas    map[string]map[int][]byte
	oplogs    map[string][]wasmee.OplogEntry
	metadata  map[string]*wasmee.InstanceMeta
}

func newMemoryStore() *memoryStore {
	return &memoryStore{
		snapshots: make(map[string][]byte),
		deltas:    make(map[string]map[int][]byte),
		oplogs:    make(map[string][]wasmee.OplogEntry),
		metadata:  make(map[string]*wasmee.InstanceMeta),
	}
}

func (s *memoryStore) SaveSnapshot(ctx context.Context, id string, snapshot []byte) error {
	s.snapshots[id] = snapshot
	return nil
}

func (s *memoryStore) LoadSnapshot(ctx context.Context, id string) ([]byte, error) {
	data, ok := s.snapshots[id]
	if !ok {
		return nil, fmt.Errorf("snapshot not found")
	}
	return data, nil
}

func (s *memoryStore) DeleteSnapshot(ctx context.Context, id string) error {
	delete(s.snapshots, id)
	return nil
}

func (s *memoryStore) SaveDeltas(ctx context.Context, id string, deltas map[int][]byte) error {
	if s.deltas[id] == nil {
		s.deltas[id] = make(map[int][]byte)
	}
	for k, v := range deltas {
		s.deltas[id][k] = v
	}
	return nil
}

func (s *memoryStore) LoadDeltas(ctx context.Context, id string) (map[int][]byte, error) {
	return s.deltas[id], nil
}

func (s *memoryStore) TruncateDeltas(ctx context.Context, id string) error {
	delete(s.deltas, id)
	return nil
}

func (s *memoryStore) SaveOplog(ctx context.Context, id string, entry wasmee.OplogEntry) error {
	s.oplogs[id] = append(s.oplogs[id], entry)
	return nil
}

func (s *memoryStore) LoadOplog(ctx context.Context, id string) ([]wasmee.OplogEntry, error) {
	return s.oplogs[id], nil
}

func (s *memoryStore) TruncateOplog(ctx context.Context, id string, beforeCallIndex int) error {
	var filtered []wasmee.OplogEntry
	for _, entry := range s.oplogs[id] {
		if entry.CallIndex < beforeCallIndex {
			filtered = append(filtered, entry)
		}
	}
	s.oplogs[id] = filtered
	return nil
}

func (s *memoryStore) SaveMetadata(ctx context.Context, meta *wasmee.InstanceMeta) (bool, error) {
	s.metadata[meta.InstanceID] = meta
	return true, nil
}

func (s *memoryStore) LoadMetadata(ctx context.Context, id string) (*wasmee.InstanceMeta, error) {
	meta, ok := s.metadata[id]
	if !ok {
		return nil, fmt.Errorf("metadata not found")
	}
	return meta, nil
}

func main() {
	// 1. Locate the precompiled guest WASM file
	wasmPath := filepath.Join("..", "..", "..", "..", "wasmee", "target", "wasm32-wasip1", "release", "wasmee_guest.wasm")
	if _, err := os.Stat(wasmPath); os.IsNotExist(err) {
		wasmPath = filepath.Join("..", "..", "wasmee", "target", "wasm32-wasip1", "release", "wasmee_guest.wasm")
		if _, err := os.Stat(wasmPath); os.IsNotExist(err) {
			wasmPath = filepath.Join("wasmee", "target", "wasm32-wasip1", "release", "wasmee_guest.wasm")
			if _, err := os.Stat(wasmPath); os.IsNotExist(err) {
				fmt.Println("Error: wasmee_guest.wasm not found. Please compile the Rust guest first.")
				os.Exit(1)
			}
		}
	}

	wasmBytes, err := os.ReadFile(wasmPath)
	if err != nil {
		fmt.Printf("Failed to read guest WASM file: %v\n", err)
		os.Exit(1)
	}

	// 2. Initialize context, session metadata, and OLME State
	ctx := context.Background()
	instanceID := "in-memory-session-demo"
	store := newMemoryStore()

	meta := &wasmee.InstanceMeta{
		InstanceID: instanceID,
		WasmHash:   "demo_hash",
		Version:    0,
	}
	_, _ = store.SaveMetadata(ctx, meta)

	// Set local test authorization token
	os.Setenv("API_TOKEN", "test-bearer-token")

	fmt.Printf("[HOST] Triggering execution of guest function \"run_test\" using Fluent API...\n")
	
	// 3. Execute using the FluentRunner
	crashed, err := wasmee.NewFluentRunner().
		WithContext(ctx).
		WithServerAddress("http://localhost:8081").
		WithWasmBytes(wasmBytes).
		WithStore(store).
		WithSessionID(instanceID).
		WithEntrypoint("run_test").
		Run()

	if err != nil {
		fmt.Printf("Execution failed: %v (crashed: %v)\n", err, crashed)
		os.Exit(1)
	}

	fmt.Println("[HOST] Execution completed successfully!")

	// 4. Verify oplog entries saved in our store
	oplogs, err := store.LoadOplog(ctx, instanceID)
	if err != nil {
		fmt.Printf("Failed to load oplog: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("[HOST] Saved oplog entries count: %d\n", len(oplogs))
	for _, entry := range oplogs {
		fmt.Printf(" - CallIndex=%d, ApiName=%q, Request=%q, Response=%q\n",
			entry.CallIndex, entry.ApiName, string(entry.RequestPayload), string(entry.ResponsePayload))
	}
}
