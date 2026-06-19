package olme

import (
	"context"
	"errors"
	"math/rand"
	"testing"
)

type inMemorySnapshotStore struct {
	snapshots map[string][]byte
	deltas    map[string]map[int][]byte
	oplogs    map[string][]OplogEntry
	metadata  map[string]*InstanceMeta
}

func newInMemoryStore() *inMemorySnapshotStore {
	return &inMemorySnapshotStore{
		snapshots: make(map[string][]byte),
		deltas:    make(map[string]map[int][]byte),
		oplogs:    make(map[string][]OplogEntry),
		metadata:  make(map[string]*InstanceMeta),
	}
}

func (s *inMemorySnapshotStore) SaveSnapshot(ctx context.Context, id string, snapshot []byte) error {
	s.snapshots[id] = snapshot
	return nil
}

func (s *inMemorySnapshotStore) LoadSnapshot(ctx context.Context, id string) ([]byte, error) {
	data, ok := s.snapshots[id]
	if !ok {
		return nil, errors.New("not found")
	}
	return data, nil
}

func (s *inMemorySnapshotStore) DeleteSnapshot(ctx context.Context, id string) error {
	delete(s.snapshots, id)
	return nil
}

func (s *inMemorySnapshotStore) SaveDeltas(ctx context.Context, id string, deltas map[int][]byte) error {
	if s.deltas[id] == nil {
		s.deltas[id] = make(map[int][]byte)
	}
	for k, v := range deltas {
		s.deltas[id][k] = v
	}
	return nil
}

func (s *inMemorySnapshotStore) LoadDeltas(ctx context.Context, id string) (map[int][]byte, error) {
	return s.deltas[id], nil
}

func (s *inMemorySnapshotStore) TruncateDeltas(ctx context.Context, id string) error {
	delete(s.deltas, id)
	return nil
}

func (s *inMemorySnapshotStore) SaveOplog(ctx context.Context, id string, entry OplogEntry) error {
	s.oplogs[id] = append(s.oplogs[id], entry)
	return nil
}

func (s *inMemorySnapshotStore) LoadOplog(ctx context.Context, id string) ([]OplogEntry, error) {
	return s.oplogs[id], nil
}

func (s *inMemorySnapshotStore) TruncateOplog(ctx context.Context, id string, beforeCallIndex int) error {
	var filtered []OplogEntry
	for _, entry := range s.oplogs[id] {
		if entry.CallIndex < beforeCallIndex {
			filtered = append(filtered, entry)
		}
	}
	s.oplogs[id] = filtered
	return nil
}

func (s *inMemorySnapshotStore) SaveMetadata(ctx context.Context, meta *InstanceMeta) (bool, error) {
	prev, exists := s.metadata[meta.InstanceID]
	if exists && prev.Version != meta.Version-1 {
		return false, nil
	}
	s.metadata[meta.InstanceID] = meta
	return true, nil
}

func (s *inMemorySnapshotStore) LoadMetadata(ctx context.Context, id string) (*InstanceMeta, error) {
	meta, ok := s.metadata[id]
	if !ok {
		return nil, errors.New("not found")
	}
	return meta, nil
}

func TestMemoryDiffingAndRestoration(t *testing.T) {
	// 1. Generate random base snapshot (2 pages = 128KB)
	base := make([]byte, PageSize*2)
	rand.Read(base)

	// 2. Clone it and modify page index 1 (dirty)
	modified := make([]byte, len(base))
	copy(modified, base)
	modified[PageSize+10] = modified[PageSize+10] ^ 0xFF // flip bits

	// 3. Compute deltas
	_, hashes := CalculateDeltas(base, nil)
	deltas, _ := CalculateDeltas(modified, hashes)

	if len(deltas) != 1 {
		t.Fatalf("expected exactly 1 dirty page delta, found: %d", len(deltas))
	}
	if _, ok := deltas[1]; !ok {
		t.Fatalf("expected page index 1 to be dirty")
	}

	// 4. Restore memory from base and deltas
	restored, err := RestoreMemory(base, deltas)
	if err != nil {
		t.Fatalf("failed to restore memory: %v", err)
	}

	for i := range modified {
		if restored[i] != modified[i] {
			t.Fatalf("mismatch at byte %d: expected %x, got %x", i, modified[i], restored[i])
		}
	}
}

func TestOplogReplay(t *testing.T) {
	store := newInMemoryStore()
	ctx := context.Background()
	instanceID := "test-instance"

	meta := &InstanceMeta{
		InstanceID: instanceID,
		WasmHash:   "dummy_hash",
		Version:    0,
	}
	store.SaveMetadata(ctx, meta)

	session := NewSessionState(instanceID, store)
	if err := session.Load(ctx); err != nil {
		t.Fatalf("failed to load session: %v", err)
	}

	// Call 1: fresh execution
	execCount := 0
	resp, err := session.GetOrExecuteCall(ctx, "test_api", []byte("req1"), func() ([]byte, error) {
		execCount++
		return []byte("resp1"), nil
	})
	if err != nil {
		t.Fatalf("unexpected call error: %v", err)
	}
	if string(resp) != "resp1" || execCount != 1 {
		t.Fatalf("invalid fresh call result")
	}

	// Load session again to simulate reload/restart
	session2 := NewSessionState(instanceID, store)
	if err := session2.Load(ctx); err != nil {
		t.Fatalf("failed to reload session: %v", err)
	}

	// Call 1: replay (callback should NOT run, count stays same)
	respReplayed, err := session2.GetOrExecuteCall(ctx, "test_api", []byte("req1"), func() ([]byte, error) {
		execCount++
		return []byte("resp_not_called"), nil
	})
	if err != nil {
		t.Fatalf("unexpected replay error: %v", err)
	}
	if string(respReplayed) != "resp1" || execCount != 1 {
		t.Fatalf("replay failed or re-invoked callback")
	}

	// Load session again to simulate reload/restart for drift test
	session3 := NewSessionState(instanceID, store)
	if err := session3.Load(ctx); err != nil {
		t.Fatalf("failed to reload session: %v", err)
	}

	// Call 1: test API drift detection
	_, errDrift := session3.GetOrExecuteCall(ctx, "mismatched_api", []byte("req1"), func() ([]byte, error) {
		return []byte("resp"), nil
	})
	if errDrift == nil {
		t.Fatalf("expected error from API name mismatch (drift)")
	}
}
