package wasmee

import (
	"context"
	"fmt"
)

const PageSize = 65536 // 64KB WASM page size

// HashPage computes the FNV-64a hash of a memory page.
func HashPage(data []byte) uint64 {
	var hash uint64 = 14695981039346656037
	for _, b := range data {
		hash ^= uint64(b)
		hash *= 1099511628211
	}
	return hash
}

// CalculateDeltas splits current memory into pages, hashes them, and returns modified pages.
func CalculateDeltas(currentMemory []byte, previousPageHashes map[int]uint64) (map[int][]byte, map[int]uint64) {
	deltas := make(map[int][]byte)
	newHashes := make(map[int]uint64)

	numPages := (len(currentMemory) + PageSize - 1) / PageSize

	for i := 0; i < numPages; i++ {
		start := i * PageSize
		end := start + PageSize
		if end > len(currentMemory) {
			end = len(currentMemory)
		}

		pageData := currentMemory[start:end]
		h := HashPage(pageData)
		newHashes[i] = h

		prevHash, ok := previousPageHashes[i]
		if !ok || prevHash != h {
			// Page is dirty or new
			deltaData := make([]byte, len(pageData))
			copy(deltaData, pageData)
			deltas[i] = deltaData
		}
	}

	return deltas, newHashes
}

// RestoreMemory compiles full linear memory from base snapshot and deltas.
func RestoreMemory(baseSnapshot []byte, deltas map[int][]byte) ([]byte, error) {
	maxPage := -1
	for p := range deltas {
		if p > maxPage {
			maxPage = p
		}
	}

	neededSize := (maxPage + 1) * PageSize
	baseSize := len(baseSnapshot)
	if neededSize < baseSize {
		neededSize = baseSize
	}

	restored := make([]byte, neededSize)
	copy(restored, baseSnapshot)

	for p, data := range deltas {
		offset := p * PageSize
		copy(restored[offset:offset+len(data)], data)
	}

	return restored, nil
}

// SessionState tracks the execution state context of a workflow instance.
type SessionState struct {
	store      SnapshotStore
	instanceID string
	meta       *InstanceMeta
	callIndex  int
	oplog      []OplogEntry
	pageHashes map[int]uint64
}

// NewSessionState initializes a SessionState.
func NewSessionState(instanceID string, store SnapshotStore) *SessionState {
	return &SessionState{
		store:      store,
		instanceID: instanceID,
		pageHashes: make(map[int]uint64),
	}
}

// Load restores the session metadata and oplog from store.
func (s *SessionState) Load(ctx context.Context) error {
	meta, err := s.store.LoadMetadata(ctx, s.instanceID)
	if err != nil {
		return fmt.Errorf("failed to load metadata: %w", err)
	}
	if meta == nil {
		meta = &InstanceMeta{
			InstanceID: s.instanceID,
			Version:    0,
		}
	}
	s.meta = meta

	oplog, err := s.store.LoadOplog(ctx, s.instanceID)
	if err != nil {
		return fmt.Errorf("failed to load oplog: %w", err)
	}
	s.oplog = oplog
	s.callIndex = 0

	return nil
}

// GetOrExecuteCall performs a call replay check or executes the callback.
func (s *SessionState) GetOrExecuteCall(ctx context.Context, apiName string, request []byte, execute func() ([]byte, error)) ([]byte, error) {
	s.callIndex++

	// 1. Check if call was already executed in oplog (Replay)
	if s.callIndex-1 < len(s.oplog) {
		entry := s.oplog[s.callIndex-1]
		if entry.ApiName != apiName {
			return nil, fmt.Errorf("oplog drift detected at index %d: expected %s, found %s", s.callIndex, apiName, entry.ApiName)
		}
		return entry.ResponsePayload, nil
	}

	// 2. If it's a new call, execute it fresh
	response, err := execute()
	if err != nil {
		return nil, err
	}

	entry := OplogEntry{
		CallIndex:       s.callIndex,
		ApiName:         apiName,
		RequestPayload:  request,
		ResponsePayload: response,
	}

	if err := s.store.SaveOplog(ctx, s.instanceID, entry); err != nil {
		return nil, fmt.Errorf("failed to save oplog entry: %w", err)
	}

	s.oplog = append(s.oplog, entry)
	return response, nil
}

// Checkpoint saves linear memory pages and updates state metadata.
func (s *SessionState) Checkpoint(ctx context.Context, currentMemory []byte) error {
	deltas, newHashes := CalculateDeltas(currentMemory, s.pageHashes)
	s.pageHashes = newHashes

	s.meta.Version++

	// Save base snapshot on first checkpoint
	if s.meta.Version == 1 {
		if err := s.store.SaveSnapshot(ctx, s.instanceID, currentMemory); err != nil {
			return fmt.Errorf("failed to save base snapshot: %w", err)
		}
	} else if len(deltas) > 0 {
		if err := s.store.SaveDeltas(ctx, s.instanceID, deltas); err != nil {
			return fmt.Errorf("failed to save memory deltas: %w", err)
		}
	}

	ok, err := s.store.SaveMetadata(ctx, s.meta)
	if err != nil {
		return fmt.Errorf("failed to save metadata: %w", err)
	}
	if !ok {
		return fmt.Errorf("optimistic concurrency conflict (OCC): failed to save metadata")
	}

	return nil
}

// GetMeta returns the current lifecycle metadata of the instance.
func (s *SessionState) GetMeta() *InstanceMeta {
	return s.meta
}

// SetMeta sets or initializes the lifecycle metadata.
func (s *SessionState) SetMeta(meta *InstanceMeta) {
	s.meta = meta
}

// GetCallIndex returns the current call pointer.
func (s *SessionState) GetCallIndex() int {
	return s.callIndex
}

// LoadSnapshot loads the base snapshot from store.
func (s *SessionState) LoadSnapshot(ctx context.Context) ([]byte, error) {
	return s.store.LoadSnapshot(ctx, s.instanceID)
}

// LoadDeltas loads the memory deltas from store.
func (s *SessionState) LoadDeltas(ctx context.Context) (map[int][]byte, error) {
	return s.store.LoadDeltas(ctx, s.instanceID)
}

// LoadOplog loads the oplog from store.
func (s *SessionState) LoadOplog(ctx context.Context) ([]OplogEntry, error) {
	return s.store.LoadOplog(ctx, s.instanceID)
}

// SetCallIndex updates the current call pointer.
func (s *SessionState) SetCallIndex(index int) {
	s.callIndex = index
}

// AddOplogEntry appends an entry to the oplog and saves it to the store.
func (s *SessionState) AddOplogEntry(ctx context.Context, entry OplogEntry) error {
	if err := s.store.SaveOplog(ctx, s.instanceID, entry); err != nil {
		return err
	}
	s.oplog = append(s.oplog, entry)
	s.callIndex = len(s.oplog)
	return nil
}
