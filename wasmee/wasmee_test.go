package wasmee

import (
	"context"
	"errors"
	"net"
	"os"
	"os/exec"
	"testing"
	"time"

	"gitlab.com/nativebpm/olme"
)

func startRustServer(t *testing.T) func() {
	cmd := exec.Command("../../wasmee/target/debug/wasmee")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start rust server: %v", err)
	}

	for i := 0; i < 50; i++ {
		conn, err := net.DialTimeout("tcp", "localhost:8081", 50*time.Millisecond)
		if err == nil {
			conn.Close()
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	return func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}
}

type testInMemoryStore struct {
	snapshots map[string][]byte
	deltas    map[string]map[int][]byte
	oplogs    map[string][]olme.OplogEntry
	metadata  map[string]*olme.InstanceMeta
}

func newTestStore() *testInMemoryStore {
	return &testInMemoryStore{
		snapshots: make(map[string][]byte),
		deltas:    make(map[string]map[int][]byte),
		oplogs:    make(map[string][]olme.OplogEntry),
		metadata:  make(map[string]*olme.InstanceMeta),
	}
}

func (s *testInMemoryStore) SaveSnapshot(ctx context.Context, id string, snapshot []byte) error {
	s.snapshots[id] = snapshot
	return nil
}

func (s *testInMemoryStore) LoadSnapshot(ctx context.Context, id string) ([]byte, error) {
	data, ok := s.snapshots[id]
	if !ok {
		return nil, errors.New("not found")
	}
	return data, nil
}

func (s *testInMemoryStore) DeleteSnapshot(ctx context.Context, id string) error {
	delete(s.snapshots, id)
	return nil
}

func (s *testInMemoryStore) SaveDeltas(ctx context.Context, id string, deltas map[int][]byte) error {
	if s.deltas[id] == nil {
		s.deltas[id] = make(map[int][]byte)
	}
	for k, v := range deltas {
		s.deltas[id][k] = v
	}
	return nil
}

func (s *testInMemoryStore) LoadDeltas(ctx context.Context, id string) (map[int][]byte, error) {
	return s.deltas[id], nil
}

func (s *testInMemoryStore) TruncateDeltas(ctx context.Context, id string) error {
	delete(s.deltas, id)
	return nil
}

func (s *testInMemoryStore) SaveOplog(ctx context.Context, id string, entry olme.OplogEntry) error {
	s.oplogs[id] = append(s.oplogs[id], entry)
	return nil
}

func (s *testInMemoryStore) LoadOplog(ctx context.Context, id string) ([]olme.OplogEntry, error) {
	return s.oplogs[id], nil
}

func (s *testInMemoryStore) TruncateOplog(ctx context.Context, id string, beforeCallIndex int) error {
	var filtered []olme.OplogEntry
	for _, entry := range s.oplogs[id] {
		if entry.CallIndex < beforeCallIndex {
			filtered = append(filtered, entry)
		}
	}
	s.oplogs[id] = filtered
	return nil
}

func (s *testInMemoryStore) SaveMetadata(ctx context.Context, meta *olme.InstanceMeta) (bool, error) {
	prev, exists := s.metadata[meta.InstanceID]
	if exists && prev.Version != meta.Version-1 {
		return false, nil
	}
	metaCopy := *meta
	s.metadata[meta.InstanceID] = &metaCopy
	return true, nil
}

func (s *testInMemoryStore) LoadMetadata(ctx context.Context, id string) (*olme.InstanceMeta, error) {
	meta, ok := s.metadata[id]
	if !ok {
		return nil, errors.New("not found")
	}
	metaCopy := *meta
	return &metaCopy, nil
}

func TestWasmRunnerExecution(t *testing.T) {
	ctx := context.Background()
	instanceID := "test-wasm-execution-instance"

	// 1. Read compiled test WASM file from wasmee workspace
	wasmBytes, err := os.ReadFile("../../wasmee/target/wasm32-wasip1/release/wasmee_guest.wasm")
	if err != nil {
		t.Fatalf("failed to read test WASM binary: %v", err)
	}

	store := newTestStore()
	meta := &olme.InstanceMeta{
		InstanceID: instanceID,
		WasmHash:   "test_hash",
		Version:    0,
	}
	store.SaveMetadata(ctx, meta)

	state := olme.NewSessionState(instanceID, store)
	if err := state.Load(ctx); err != nil {
		t.Fatalf("failed to load state: %v", err)
	}

	cleanup := startRustServer(t)
	defer cleanup()

	runner, err := NewRunner(ctx, wasmBytes, "localhost:8081")
	if err != nil {
		t.Fatalf("failed to create runner: %v", err)
	}
	defer runner.Close(ctx)

	session := NewSession(instanceID, state)

	// RUN 1: Starts fresh, executes up to checkpoint 1, and finishes successfully.
	crashed, _, err := runner.Execute(ctx, session, "run_test", nil)
	if err != nil {
		t.Fatalf("execution failed: %v", err)
	}
	if crashed {
		t.Fatalf("unexpected crash detected")
	}

	// 2. Validate that checkpoint 1 saved a base snapshot (Version = 1)
	snapshot, err := store.LoadSnapshot(ctx, instanceID)
	if err != nil {
		t.Fatalf("failed to load base snapshot: %v", err)
	}
	if len(snapshot) == 0 {
		t.Fatalf("base snapshot is empty")
	}

	// Validate oplog contains 2 calls: test_api("hello") and test_api("world")
	oplogs, err := store.LoadOplog(ctx, instanceID)
	if err != nil {
		t.Fatalf("failed to load oplog: %v", err)
	}
	if len(oplogs) != 2 {
		t.Fatalf("expected exactly 2 oplog entries, got %d", len(oplogs))
	}
	if oplogs[0].ApiName != "test_api" || string(oplogs[0].RequestPayload) != "hello" {
		t.Fatalf("invalid first call entry in oplog")
	}
	if oplogs[1].ApiName != "test_api" || string(oplogs[1].RequestPayload) != "world" {
		t.Fatalf("invalid second call entry in oplog")
	}

	// Validate memory deltas exist (checkpoint 2 saved changes to offset 70000 on page 1)
	deltas, err := store.LoadDeltas(ctx, instanceID)
	if err != nil {
		t.Fatalf("failed to load memory deltas: %v", err)
	}
	if len(deltas) == 0 {
		t.Fatalf("expected memory deltas for dirty pages, got none")
	}
	if _, ok := deltas[1]; !ok {
		t.Fatalf("expected page index 1 to be marked as dirty")
	}
}

func TestWasmRunnerSimulatedCrashRecovery(t *testing.T) {
	ctx := context.Background()
	instanceID := "test-crash-recovery-instance"

	wasmBytes, err := os.ReadFile("../../wasmee/target/wasm32-wasip1/release/wasmee_guest.wasm")
	if err != nil {
		t.Fatalf("failed to read test WASM binary: %v", err)
	}

	store := newTestStore()
	meta := &olme.InstanceMeta{
		InstanceID: instanceID,
		WasmHash:   "test_hash",
		Version:    0,
	}
	store.SaveMetadata(ctx, meta)

	state := olme.NewSessionState(instanceID, store)
	if err := state.Load(ctx); err != nil {
		t.Fatalf("failed to load state: %v", err)
	}

	cleanup := startRustServer(t)
	defer cleanup()

	runner, err := NewRunner(ctx, wasmBytes, "localhost:8081")
	if err != nil {
		t.Fatalf("failed to create runner: %v", err)
	}
	defer runner.Close(ctx)

	session := NewSession(instanceID, state)
	session.EnableCrashSimulation(true)

	// RUN 1: Starts fresh, hits checkpoint 1, saves state, and crashes.
	crashed, _, err := runner.Execute(ctx, session, "run_test", nil)
	if err == nil || !crashed {
		t.Fatalf("expected simulated host crash, got nil or no crash")
	}

	// Verify checkpoint 1 saved version 1 snapshot metadata
	metaReload, err := store.LoadMetadata(ctx, instanceID)
	if err != nil {
		t.Fatalf("failed to load metadata: %v", err)
	}
	if metaReload.Version != 1 {
		t.Fatalf("expected snapshot version 1, got %d", metaReload.Version)
	}

	// Verify oplog contains only 1 call (test_api("hello")) before crash
	oplogs, err := store.LoadOplog(ctx, instanceID)
	if err != nil {
		t.Fatalf("failed to load oplog: %v", err)
	}
	if len(oplogs) != 1 {
		t.Fatalf("expected exactly 1 oplog entry before crash, got %d", len(oplogs))
	}

	// RUN 2: Reload state and resume from crash point (disabling crash simulation)
	state2 := olme.NewSessionState(instanceID, store)
	if err := state2.Load(ctx); err != nil {
		t.Fatalf("failed to load state for resume: %v", err)
	}

	session2 := NewSession(instanceID, state2)
	session2.EnableCrashSimulation(false)

	// Keep track of handler execution (should replay "hello" from oplog, and run "world" fresh)
	execCount := 0
	session2.ApiHandler = func(apiName string, request []byte) ([]byte, error) {
		execCount++
		return []byte("resp_for_api"), nil
	}

	crashed2, _, err := runner.Execute(ctx, session2, "run_test", nil)
	if err != nil {
		t.Fatalf("resume execution failed: %v", err)
	}
	if crashed2 {
		t.Fatalf("unexpected crash on resume run")
	}

	// Verify the final oplog has 2 entries and the second entry is the live call for "world"
	oplogs2, err := store.LoadOplog(ctx, instanceID)
	if err != nil {
		t.Fatalf("failed to load final oplog: %v", err)
	}
	t.Logf("Final oplog count: %d", len(oplogs2))
	for i, entry := range oplogs2 {
		t.Logf("Oplog[%d]: CallIndex=%d, ApiName=%s, RequestPayload=%s, ResponsePayload=%s", i, entry.CallIndex, entry.ApiName, string(entry.RequestPayload), string(entry.ResponsePayload))
	}
	if len(oplogs2) != 2 {
		t.Fatalf("expected exactly 2 oplog entries after recovery, got %d", len(oplogs2))
	}
	if oplogs2[1].ApiName != "test_api" || string(oplogs2[1].RequestPayload) != "world" {
		t.Fatalf("invalid second call entry in final oplog: got Api=%s Request=%s", oplogs2[1].ApiName, string(oplogs2[1].RequestPayload))
	}

	// Verify final version is 3
	metaFinal, err := store.LoadMetadata(ctx, instanceID)
	if err != nil {
		t.Fatalf("failed to load final metadata: %v", err)
	}
	if metaFinal.Version != 3 {
		t.Fatalf("expected final version 3, got %d", metaFinal.Version)
	}
}
