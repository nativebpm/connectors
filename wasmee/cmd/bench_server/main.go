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
	"github.com/nativebpm/connectors/wasmee/olme"
)

// Simple in-memory store for load testing
type memStore struct {
	mu        sync.RWMutex
	snapshots map[string][]byte
	deltas    map[string]map[int][]byte
	oplogs    map[string][]olme.OplogEntry
	metadata  map[string]*olme.InstanceMeta
}

func newMemStore() *memStore {
	return &memStore{
		snapshots: make(map[string][]byte),
		deltas:    make(map[string]map[int][]byte),
		oplogs:    make(map[string][]olme.OplogEntry),
		metadata:  make(map[string]*olme.InstanceMeta),
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

func (s *memStore) SaveOplog(ctx context.Context, id string, entry olme.OplogEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.oplogs[id] = append(s.oplogs[id], entry)
	return nil
}

func (s *memStore) LoadOplog(ctx context.Context, id string) ([]olme.OplogEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.oplogs[id], nil
}

func (s *memStore) TruncateOplog(ctx context.Context, id string, beforeCallIndex int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	var filtered []olme.OplogEntry
	for _, entry := range s.oplogs[id] {
		if entry.CallIndex < beforeCallIndex {
			filtered = append(filtered, entry)
		}
	}
	s.oplogs[id] = filtered
	return nil
}

func (s *memStore) SaveMetadata(ctx context.Context, meta *olme.InstanceMeta) (bool, error) {
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

func (s *memStore) LoadMetadata(ctx context.Context, id string) (*olme.InstanceMeta, error) {
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

	runner, err := wasmee.NewRunner(context.Background(), wasmBytes, "localhost:8081")
	if err != nil {
		log.Fatalf("failed to initialize wasmee runner: %v", err)
	}
	defer runner.Close(context.Background())

	store := newMemStore()
	var reqCount uint64

	http.HandleFunc("/execute", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		ctx := r.Context()
		instanceID := "bench-" + uuid.New().String()

		meta := &olme.InstanceMeta{
			InstanceID: instanceID,
			WasmHash:   "bench_hash",
			Version:    0,
		}
		_, _ = store.SaveMetadata(ctx, meta)

		state := olme.NewSessionState(instanceID, store)
		if err := state.Load(ctx); err != nil {
			http.Error(w, fmt.Sprintf("failed to load state: %v", r), http.StatusInternalServerError)
			return
		}

		session := wasmee.NewSession(instanceID, state)

		crashed, _, err := runner.Execute(ctx, session, "run_test", nil)
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
