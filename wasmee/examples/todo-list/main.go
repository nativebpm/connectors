package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/nativebpm/connectors/wasmee"
	"gitlab.com/nativebpm/olme"
)

// memoryStore implements olme.SnapshotStore in memory.
type memoryStore struct {
	snapshots map[string][]byte
	deltas    map[string]map[int][]byte
	oplogs    map[string][]olme.OplogEntry
	metadata  map[string]*olme.InstanceMeta
}

func newMemoryStore() *memoryStore {
	return &memoryStore{
		snapshots: make(map[string][]byte),
		deltas:    make(map[string]map[int][]byte),
		oplogs:    make(map[string][]olme.OplogEntry),
		metadata:  make(map[string]*olme.InstanceMeta),
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

func (s *memoryStore) SaveOplog(ctx context.Context, id string, entry olme.OplogEntry) error {
	s.oplogs[id] = append(s.oplogs[id], entry)
	return nil
}

func (s *memoryStore) LoadOplog(ctx context.Context, id string) ([]olme.OplogEntry, error) {
	return s.oplogs[id], nil
}

func (s *memoryStore) TruncateOplog(ctx context.Context, id string, beforeCallIndex int) error {
	var filtered []olme.OplogEntry
	for _, entry := range s.oplogs[id] {
		if entry.CallIndex < beforeCallIndex {
			filtered = append(filtered, entry)
		}
	}
	s.oplogs[id] = filtered
	return nil
}

func (s *memoryStore) SaveMetadata(ctx context.Context, meta *olme.InstanceMeta) (bool, error) {
	s.metadata[meta.InstanceID] = meta
	return true, nil
}

func (s *memoryStore) LoadMetadata(ctx context.Context, id string) (*olme.InstanceMeta, error) {
	meta, ok := s.metadata[id]
	if !ok {
		return nil, fmt.Errorf("metadata not found")
	}
	return meta, nil
}

// GraphDefinition matches the structure used by the Rust guest engine.
type GraphDefinition struct {
	ID          string               `json:"id"`
	Name        string               `json:"name"`
	Nodes       map[string]GraphNode `json:"nodes"`
	Connections []Connection         `json:"connections"`
	StartNodeID string               `json:"start_node_id"`
}

type GraphNode struct {
	ID   string `json:"id"`
	Type string `json:"type"`
	Name string `json:"name"`
}

type Connection struct {
	ID        string `json:"id"`
	SourceRef string `json:"source_ref"`
	TargetRef string `json:"target_ref"`
}

type ProcessInstance struct {
	ID                       string                 `json:"id"`
	ProcessID                string                 `json:"process_id"`
	ActiveActivityInstances  []string               `json:"active_activity_instances"`
	WaitingActivityInstances []string               `json:"waiting_activity_instances"`
	CompletedNodes           []string               `json:"completed_nodes"`
	Variables                map[string]interface{} `json:"variables"`
	Completed                bool                   `json:"completed"`
}

func main() {
	// 1. Locate the precompiled guest WASM file
	wasmPath := filepath.Join("..", "..", "..", "wasmee", "target", "wasm32-wasip1", "release", "wasmee_guest.wasm")
	if _, err := os.Stat(wasmPath); os.IsNotExist(err) {
		wasmPath = filepath.Join("wasmee", "target", "wasm32-wasip1", "release", "wasmee_guest.wasm")
		if _, err := os.Stat(wasmPath); os.IsNotExist(err) {
			fmt.Println("Error: wasmee_guest.wasm not found. Please compile the Rust guest first.")
			os.Exit(1)
		}
	}

	wasmBytes, err := os.ReadFile(wasmPath)
	if err != nil {
		fmt.Printf("Failed to read guest WASM file: %v\n", err)
		os.Exit(1)
	}

	ctx := context.Background()
	instanceID := "todo-list-demo-session"
	store := newMemoryStore()

	meta := &olme.InstanceMeta{
		InstanceID: instanceID,
		WasmHash:   "todo_process_hash",
		Version:    0,
	}
	_, _ = store.SaveMetadata(ctx, meta)

	state := olme.NewSessionState(instanceID, store)
	if err := state.Load(ctx); err != nil {
		fmt.Printf("Failed to load session state: %v\n", err)
		os.Exit(1)
	}

	// 2. Initialize the wasmee HTTP runner (expects server listening on :8081)
	runner, err := wasmee.NewRunner(ctx, wasmBytes, "http://localhost:8081")
	if err != nil {
		fmt.Printf("Failed to initialize wasmee runner: %v\n", err)
		os.Exit(1)
	}

	session := wasmee.NewSession(instanceID, state)

	// Define Todo List process
	graph := GraphDefinition{
		ID:   "todoProcess",
		Name: "Todo List Process",
		Nodes: map[string]GraphNode{
			"start":         {ID: "start", Type: "StartEvent", Name: "Start"},
			"add_todo":      {ID: "add_todo", Type: "UserTask", Name: "Add Todo Item"},
			"complete_todo": {ID: "complete_todo", Type: "UserTask", Name: "Complete Todo Item"},
			"end":           {ID: "end", Type: "EndEvent", Name: "End"},
		},
		Connections: []Connection{
			{ID: "flow1", SourceRef: "start", TargetRef: "add_todo"},
			{ID: "flow2", SourceRef: "add_todo", TargetRef: "complete_todo"},
			{ID: "flow3", SourceRef: "complete_todo", TargetRef: "end"},
		},
		StartNodeID: "start",
	}

	graphBytes, _ := json.Marshal(graph)
	variables := map[string]interface{}{}
	variablesBytes, _ := json.Marshal(variables)

	// Step 1: Execute process (starts it and loops to first UserTask wait state)
	fmt.Println("[HOST] Starting execution of Todo List BPMN process...")
	exchangeBuffer := make([]byte, len(graphBytes)+len(variablesBytes))
	copy(exchangeBuffer[0:len(graphBytes)], graphBytes)
	copy(exchangeBuffer[len(graphBytes):], variablesBytes)

	_, respBytes, err := runner.Execute(ctx, session, "execute", exchangeBuffer, uint64(len(graphBytes)), uint64(len(variablesBytes)))
	if err != nil {
		fmt.Printf("Failed to start process: %v\n", err)
		os.Exit(1)
	}

	var pi ProcessInstance
	_ = json.Unmarshal(respBytes, &pi)

	fmt.Printf("[HOST] Process started! ID: %s, Completed: %t\n", pi.ID, pi.Completed)
	fmt.Printf("[HOST] Active Nodes: %v\n", pi.ActiveActivityInstances)
	fmt.Printf("[HOST] Waiting Nodes: %v\n", pi.WaitingActivityInstances)
	fmt.Printf("[HOST] Variables: %v\n", pi.Variables)

	// Step 2: Complete "add_todo" task (simulate adding a task)
	fmt.Println("\n[HOST] Completing \"add_todo\" task and adding todo item variable...")
	pi.Variables["todo_item"] = "Task 1: Learn Rust WebAssembly"
	instanceBytes, _ := json.Marshal(pi)
	completedTaskID := "add_todo"

	taskIDOffset := len(graphBytes) + len(instanceBytes)
	exchangeBuffer2 := make([]byte, taskIDOffset+len(completedTaskID))
	copy(exchangeBuffer2[0:len(graphBytes)], graphBytes)
	copy(exchangeBuffer2[len(graphBytes):taskIDOffset], instanceBytes)
	copy(exchangeBuffer2[taskIDOffset:], []byte(completedTaskID))

	_, respBytes2, err := runner.Execute(ctx, session, "resume", exchangeBuffer2, uint64(len(graphBytes)), uint64(len(instanceBytes)), uint64(taskIDOffset), uint64(len(completedTaskID)))
	if err != nil {
		fmt.Printf("Failed to resume process: %v\n", err)
		os.Exit(1)
	}

	_ = json.Unmarshal(respBytes2, &pi)

	fmt.Printf("[HOST] Process resumed! Completed: %t\n", pi.Completed)
	fmt.Printf("[HOST] Waiting Nodes: %v\n", pi.WaitingActivityInstances)
	fmt.Printf("[HOST] Variables: %v\n", pi.Variables)

	// Step 3: Complete "complete_todo" task (simulate completing the task)
	fmt.Println("\n[HOST] Completing \"complete_todo\" task...")
	pi.Variables["todo_item_completed"] = true
	instanceBytes2, _ := json.Marshal(pi)
	completedTaskID2 := "complete_todo"

	taskIDOffset2 := len(graphBytes) + len(instanceBytes2)
	exchangeBuffer3 := make([]byte, taskIDOffset2+len(completedTaskID2))
	copy(exchangeBuffer3[0:len(graphBytes)], graphBytes)
	copy(exchangeBuffer3[len(graphBytes):taskIDOffset2], instanceBytes2)
	copy(exchangeBuffer3[taskIDOffset2:], []byte(completedTaskID2))

	_, respBytes3, err := runner.Execute(ctx, session, "resume", exchangeBuffer3, uint64(len(graphBytes)), uint64(len(instanceBytes2)), uint64(taskIDOffset2), uint64(len(completedTaskID2)))
	if err != nil {
		fmt.Printf("Failed to complete process: %v\n", err)
		os.Exit(1)
	}

	_ = json.Unmarshal(respBytes3, &pi)

	fmt.Printf("[HOST] Process finished! Completed: %t\n", pi.Completed)
	fmt.Printf("[HOST] Variables: %v\n", pi.Variables)
}
