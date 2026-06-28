package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"sync/atomic"

	"github.com/google/uuid"
	"github.com/nativebpm/connectors/wasmee"
)

// Simple in-memory store for load testing
type memStore struct {
	mu        sync.RWMutex
	snapshots map[string][]byte
	deltas    map[string]map[int][]byte
	oplogs    map[string][]wasmee.OplogEntry
	metadata  map[string]*wasmee.InstanceMeta
}

func newMemStore() *memStore {
	return &memStore{
		snapshots: make(map[string][]byte),
		deltas:    make(map[string]map[int][]byte),
		oplogs:    make(map[string][]wasmee.OplogEntry),
		metadata:  make(map[string]*wasmee.InstanceMeta),
	}
}

func (s *memStore) SaveSnapshot(ctx context.Context, id string, snapshot []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snapshots[id] = snapshot
	return nil
}

func (s *memStore) LoadSnapshot(ctx context.Context, id string) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	data, ok := s.snapshots[id]
	if !ok {
		return nil, errors.New("not found")
	}
	return data, nil
}

func (s *memStore) DeleteSnapshot(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.snapshots, id)
	return nil
}

func (s *memStore) SaveDeltas(ctx context.Context, id string, deltas map[int][]byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.deltas[id] == nil {
		s.deltas[id] = make(map[int][]byte)
	}
	for k, v := range deltas {
		s.deltas[id][k] = v
	}
	return nil
}

func (s *memStore) LoadDeltas(ctx context.Context, id string) (map[int][]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.deltas[id], nil
}

func (s *memStore) TruncateDeltas(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.deltas, id)
	return nil
}

func (s *memStore) SaveOplog(ctx context.Context, id string, entry wasmee.OplogEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.oplogs[id] = append(s.oplogs[id], entry)
	return nil
}

func (s *memStore) LoadOplog(ctx context.Context, id string) ([]wasmee.OplogEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.oplogs[id], nil
}

func (s *memStore) TruncateOplog(ctx context.Context, id string, beforeCallIndex int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	var filtered []wasmee.OplogEntry
	for _, entry := range s.oplogs[id] {
		if entry.CallIndex < beforeCallIndex {
			filtered = append(filtered, entry)
		}
	}
	s.oplogs[id] = filtered
	return nil
}

func (s *memStore) SaveMetadata(ctx context.Context, meta *wasmee.InstanceMeta) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	prev, exists := s.metadata[meta.InstanceID]
	if exists && prev.Version != meta.Version-1 {
		return false, nil
	}
	metaCopy := *meta
	s.metadata[meta.InstanceID] = &metaCopy
	return true, nil
}

func (s *memStore) LoadMetadata(ctx context.Context, id string) (*wasmee.InstanceMeta, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	meta, ok := s.metadata[id]
	if !ok {
		return nil, errors.New("not found")
	}
	metaCopy := *meta
	return &metaCopy, nil
}

func main() {
	wasmPath := "/Users/user/github.com/nativebpm/wasmee/target/wasm32-wasip1/release/wasmee_guest.wasm"
	if _, err := os.Stat(wasmPath); os.IsNotExist(err) {
		wasmPath = "target/wasm32-wasip1/release/wasmee_guest.wasm"
		if _, err := os.Stat(wasmPath); os.IsNotExist(err) {
			wasmPath = "../../wasmee/target/wasm32-wasip1/release/wasmee_guest.wasm"
		}
	}

	wasmBytes, err := os.ReadFile(wasmPath)
	if err != nil {
		log.Fatalf("failed to read guest WASM binary: %v", err)
	}

	store := newMemStore()
	var reqCount uint64

	http.HandleFunc("/execute", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		ctx := r.Context()
		instanceID := "bench-" + uuid.New().String()

		meta := &wasmee.InstanceMeta{
			InstanceID: instanceID,
			WasmHash:   "bench_hash",
			Version:    0,
		}
		_, _ = store.SaveMetadata(ctx, meta)

		crashed, err := wasmee.NewFluentRunner().
			WithContext(ctx).
			WithServerAddress("http://127.0.0.1:8081").
			WithWasmBytes(wasmBytes).
			WithStore(store).
			WithSessionID(instanceID).
			WithEntrypoint("run_test").
			Run()

		if err != nil {
			http.Error(w, fmt.Sprintf("execution failed: %v", err), http.StatusInternalServerError)
			return
		}

		if crashed {
			http.Error(w, "execution crashed", http.StatusInternalServerError)
			return
		}

		count := atomic.AddUint64(&reqCount, 1)
		if count%500 == 0 {
			log.Printf("Processed %d execution requests", count)
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	log.Printf("Wasmee benchmark server listening on :8085 using guest WASM: %s", wasmPath)
	if err := http.ListenAndServe(":8085", nil); err != nil {
		log.Fatalf("HTTP server failed: %v", err)
	}
}
