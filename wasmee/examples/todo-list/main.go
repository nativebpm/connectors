package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"github.com/nativebpm/connectors/jsonschema"
	"github.com/nativebpm/connectors/wasmee"
)

// memoryStore implements olme.SnapshotStore in memory.
type memoryStore struct {
	mu        sync.Mutex
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
	s.mu.Lock()
	defer s.mu.Unlock()
	s.snapshots[id] = snapshot
	return nil
}

func (s *memoryStore) LoadSnapshot(ctx context.Context, id string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, ok := s.snapshots[id]
	if !ok {
		return nil, fmt.Errorf("snapshot not found")
	}
	return data, nil
}

func (s *memoryStore) DeleteSnapshot(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.snapshots, id)
	return nil
}

func (s *memoryStore) SaveDeltas(ctx context.Context, id string, deltas map[int][]byte) error {
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

func (s *memoryStore) LoadDeltas(ctx context.Context, id string) (map[int][]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.deltas[id], nil
}

func (s *memoryStore) TruncateDeltas(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.deltas, id)
	return nil
}

func (s *memoryStore) SaveOplog(ctx context.Context, id string, entry wasmee.OplogEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.oplogs[id] = append(s.oplogs[id], entry)
	return nil
}

func (s *memoryStore) LoadOplog(ctx context.Context, id string) ([]wasmee.OplogEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.oplogs[id], nil
}

func (s *memoryStore) TruncateOplog(ctx context.Context, id string, beforeCallIndex int) error {
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

func (s *memoryStore) SaveMetadata(ctx context.Context, meta *wasmee.InstanceMeta) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.metadata[meta.InstanceID] = meta
	return true, nil
}

func (s *memoryStore) LoadMetadata(ctx context.Context, id string) (*wasmee.InstanceMeta, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	meta, ok := s.metadata[id]
	if !ok {
		return nil, nil // Return nil, nil if metadata is not found (avoids panic)
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

// JSON schemas for our BPMN User Tasks
var taskSchemas = map[string]string{
	"add_todo": `{
		"type": "object",
		"title": "Add Todo Item",
		"properties": {
			"todo_item": {
				"type": "string",
				"title": "Task Description",
				"ui:widget": "text"
			},
			"priority": {
				"type": "string",
				"title": "Priority",
				"enum": ["Low", "Medium", "High"],
				"ui:widget": "select",
				"default": "Medium"
			}
		},
		"required": ["todo_item"]
	}`,
	"complete_todo": `{
		"type": "object",
		"title": "Complete Todo Item",
		"properties": {
			"todo_item": {
				"type": "string",
				"title": "Task to Complete",
				"ui:widget": "text"
			},
			"priority": {
				"type": "string",
				"title": "Priority",
				"ui:widget": "text"
			},
			"todo_item_completed": {
				"type": "boolean",
				"title": "Mark as Completed",
				"ui:widget": "checkbox"
			}
		},
		"required": ["todo_item_completed"]
	}`,
}

var (
	wasmBytes       []byte
	store           *memoryStore
	instanceID      = "todo-list-web-session"
	graph           GraphDefinition
	graphBytes      []byte
	currentInstance ProcessInstance
	stateMu         sync.Mutex
)

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

	var err error
	wasmBytes, err = os.ReadFile(wasmPath)
	if err != nil {
		fmt.Printf("Failed to read guest WASM file: %v\n", err)
		os.Exit(1)
	}

	ctx := context.Background()
	store = newMemoryStore()

	// Set local test authorization token
	os.Setenv("API_TOKEN", "test-bearer-token")

	// Define Todo List process
	graph = GraphDefinition{
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
	graphBytes, _ = json.Marshal(graph)

	// Start or reset process initial state
	if err := startNewProcessInstance(ctx); err != nil {
		fmt.Printf("Failed to start initial process: %v\n", err)
		os.Exit(1)
	}

	// Set up HTTP routes
	http.HandleFunc("/", handleHome)
	http.HandleFunc("/api/state", handleState)
	http.HandleFunc("/api/submit", handleSubmit)
	http.HandleFunc("/api/reset", handleReset)

	port := "8085"
	fmt.Printf("Wasmee Todo List Web App is running on http://localhost:%s\n", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		fmt.Printf("HTTP server failed: %v\n", err)
	}
}

func startNewProcessInstance(ctx context.Context) error {
	stateMu.Lock()
	defer stateMu.Unlock()

	// Clear memory store for this session
	store.mu.Lock()
	delete(store.snapshots, instanceID)
	delete(store.deltas, instanceID)
	delete(store.oplogs, instanceID)
	delete(store.metadata, instanceID)
	store.mu.Unlock()

	meta := &wasmee.InstanceMeta{
		InstanceID: instanceID,
		WasmHash:   "todo_process_hash",
		Version:    0,
	}
	_, _ = store.SaveMetadata(ctx, meta)

	state := wasmee.NewSessionState(instanceID, store)
	if err := state.Load(ctx); err != nil {
		return err
	}

	variables := map[string]interface{}{}
	variablesBytes, _ := json.Marshal(variables)

	exchangeBuffer := make([]byte, len(graphBytes)+len(variablesBytes))
	copy(exchangeBuffer[0:len(graphBytes)], graphBytes)
	copy(exchangeBuffer[len(graphBytes):], variablesBytes)

	fluentRunner := wasmee.NewFluentRunner().
		WithContext(ctx).
		WithServerAddress("http://localhost:8081").
		WithWasmBytes(wasmBytes).
		WithStore(store).
		WithSessionID(instanceID).
		WithEntrypoint("execute").
		WithExchangeBuffer(exchangeBuffer).
		WithArgs(uint64(len(graphBytes)), uint64(len(variablesBytes)))

	crashed, err := fluentRunner.Run()
	if err != nil {
		return fmt.Errorf("wasmee execute failed: %w (crashed: %t)", err, crashed)
	}

	return json.Unmarshal(fluentRunner.Response(), &currentInstance)
}

func handleHome(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(htmlContent))
}

func handleState(w http.ResponseWriter, r *http.Request) {
	stateMu.Lock()
	defer stateMu.Unlock()

	w.Header().Set("Content-Type", "application/json")

	// Get widgets if there is a waiting UserTask
	var widgets []*jsonschema.UIWidgetSpec
	if len(currentInstance.WaitingActivityInstances) > 0 {
		activeTask := currentInstance.WaitingActivityInstances[0]
		if schema, ok := taskSchemas[activeTask]; ok {
			var err error
			widgets, err = jsonschema.ParseSchema(schema, currentInstance.Variables)
			if err != nil {
				http.Error(w, "Failed to parse schema: "+err.Error(), http.StatusInternalServerError)
				return
			}
		}
	}

	response := map[string]interface{}{
		"completed":     currentInstance.Completed,
		"active_nodes":  currentInstance.ActiveActivityInstances,
		"waiting_nodes": currentInstance.WaitingActivityInstances,
		"variables":     currentInstance.Variables,
		"widgets":       widgets,
	}

	json.NewEncoder(w).Encode(response)
}

func handleSubmit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST allowed", http.StatusMethodNotAllowed)
		return
	}

	var formVars map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&formVars); err != nil {
		http.Error(w, "Invalid body: "+err.Error(), http.StatusBadRequest)
		return
	}

	stateMu.Lock()
	defer stateMu.Unlock()

	ctx := r.Context()
	if len(currentInstance.WaitingActivityInstances) == 0 {
		http.Error(w, "No active waiting task to submit", http.StatusBadRequest)
		return
	}

	activeTask := currentInstance.WaitingActivityInstances[0]

	// Merge form variables into process variables
	if currentInstance.Variables == nil {
		currentInstance.Variables = make(map[string]interface{})
	}
	for k, v := range formVars {
		currentInstance.Variables[k] = v
	}

	state := wasmee.NewSessionState(instanceID, store)
	if err := state.Load(ctx); err != nil {
		http.Error(w, "Failed to load state: "+err.Error(), http.StatusInternalServerError)
		return
	}

	instanceBytes, _ := json.Marshal(currentInstance)

	taskIDOffset := len(graphBytes) + len(instanceBytes)
	exchangeBuffer := make([]byte, taskIDOffset+len(activeTask))
	copy(exchangeBuffer[0:len(graphBytes)], graphBytes)
	copy(exchangeBuffer[len(graphBytes):taskIDOffset], instanceBytes)
	copy(exchangeBuffer[taskIDOffset:], []byte(activeTask))

	fluentRunner := wasmee.NewFluentRunner().
		WithContext(ctx).
		WithServerAddress("http://localhost:8081").
		WithWasmBytes(wasmBytes).
		WithStore(store).
		WithSessionID(instanceID).
		WithEntrypoint("resume").
		WithExchangeBuffer(exchangeBuffer).
		WithArgs(uint64(len(graphBytes)), uint64(len(instanceBytes)), uint64(taskIDOffset), uint64(len(activeTask)))

	crashed, err := fluentRunner.Run()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to resume wasm process: %v (crashed: %t)", err, crashed), http.StatusInternalServerError)
		return
	}

	if err := json.Unmarshal(fluentRunner.Response(), &currentInstance); err != nil {
		http.Error(w, "Failed to unmarshal response: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"success"}`))
}

func handleReset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := startNewProcessInstance(r.Context()); err != nil {
		http.Error(w, "Failed to reset process: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status":"success"}`))
}

const htmlContent = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>NativeBPM Wasmee - Todo List Example</title>
    <link rel="preconnect" href="https://fonts.googleapis.com">
    <link rel="preconnect" href="https://fonts.gstatic.com" crossorigin>
    <link href="https://fonts.googleapis.com/css2?family=Outfit:wght@300;400;500;600;700&display=swap" rel="stylesheet">
    <style>
        :root {
            --primary: #8b5cf6;
            --primary-hover: #7c3aed;
            --bg: #030712;
            --card-bg: rgba(255, 255, 255, 0.03);
            --card-border: rgba(255, 255, 255, 0.06);
            --text: #f3f4f6;
            --text-muted: #9ca3af;
        }

        body {
            font-family: 'Outfit', sans-serif;
            background-color: var(--bg);
            background-image: radial-gradient(circle at 50% -20%, #1e1b4b 0%, var(--bg) 70%);
            color: var(--text);
            margin: 0;
            padding: 0;
            min-height: 100vh;
            display: flex;
            flex-direction: column;
            align-items: center;
            justify-content: center;
        }

        .container {
            width: 100%;
            max-width: 600px;
            padding: 2rem;
            box-sizing: border-box;
        }

        .card {
            background: var(--card-bg);
            backdrop-filter: blur(16px);
            -webkit-backdrop-filter: blur(16px);
            border: 1px solid var(--card-border);
            border-radius: 24px;
            padding: 2.5rem;
            box-shadow: 0 25px 50px -12px rgba(0, 0, 0, 0.5);
            margin-bottom: 2rem;
            transition: all 0.3s ease;
        }

        h1 {
            font-size: 2rem;
            font-weight: 700;
            margin-top: 0;
            margin-bottom: 0.5rem;
            background: linear-gradient(135deg, #a78bfa 0%, #ec4899 100%);
            -webkit-background-clip: text;
            -webkit-text-fill-color: transparent;
            text-align: center;
        }

        .subtitle {
            text-align: center;
            color: var(--text-muted);
            font-size: 0.9rem;
            margin-bottom: 2rem;
        }

        .status-badge {
            display: inline-flex;
            align-items: center;
            gap: 0.5rem;
            padding: 0.5rem 1rem;
            border-radius: 9999px;
            font-size: 0.8rem;
            font-weight: 600;
            background: rgba(139, 92, 246, 0.1);
            border: 1px solid rgba(139, 92, 246, 0.2);
            color: #c084fc;
            margin-bottom: 1.5rem;
        }

        .status-badge.completed {
            background: rgba(16, 185, 129, 0.1);
            border: 1px solid rgba(16, 185, 129, 0.2);
            color: #34d399;
        }

        .form-group {
            margin-bottom: 1.5rem;
            display: flex;
            flex-direction: column;
            gap: 0.5rem;
        }

        label {
            font-size: 0.9rem;
            font-weight: 500;
            color: var(--text);
        }

        input[type="text"], select {
            background: rgba(0, 0, 0, 0.3);
            border: 1px solid rgba(255, 255, 255, 0.1);
            border-radius: 12px;
            padding: 0.75rem 1rem;
            color: var(--text);
            font-family: inherit;
            font-size: 0.95rem;
            transition: all 0.2s ease;
            width: 100%;
            box-sizing: border-box;
        }

        input[type="text"]:focus, select:focus {
            outline: none;
            border-color: var(--primary);
            box-shadow: 0 0 0 3px rgba(139, 92, 246, 0.2);
        }

        input[readonly] {
            background: rgba(255, 255, 255, 0.01);
            border-color: rgba(255, 255, 255, 0.03);
            color: var(--text-muted);
            cursor: not-allowed;
        }

        .checkbox-container {
            display: flex;
            align-items: center;
            gap: 0.75rem;
            cursor: pointer;
            user-select: none;
            padding: 0.5rem 0;
        }

        .checkbox-container input {
            display: none;
        }

        .checkmark {
            width: 20px;
            height: 20px;
            border: 2px solid rgba(255, 255, 255, 0.2);
            border-radius: 6px;
            display: flex;
            align-items: center;
            justify-content: center;
            transition: all 0.2s ease;
        }

        .checkbox-container:hover .checkmark {
            border-color: var(--primary);
        }

        .checkbox-container input:checked + .checkmark {
            background: var(--primary);
            border-color: var(--primary);
        }

        .checkmark::after {
            content: "✓";
            color: white;
            font-size: 12px;
            display: none;
            font-weight: bold;
        }

        .checkbox-container input:checked + .checkmark::after {
            display: block;
        }

        .btn {
            background: linear-gradient(135deg, var(--primary) 0%, #6d28d9 100%);
            border: none;
            border-radius: 12px;
            color: white;
            padding: 0.85rem 1.5rem;
            font-size: 0.95rem;
            font-weight: 600;
            font-family: inherit;
            cursor: pointer;
            transition: all 0.2s ease;
            display: flex;
            align-items: center;
            justify-content: center;
            width: 100%;
            box-shadow: 0 4px 12px rgba(139, 92, 246, 0.3);
        }

        .btn:hover {
            transform: translateY(-1px);
            box-shadow: 0 6px 20px rgba(139, 92, 246, 0.4);
        }

        .btn:active {
            transform: translateY(1px);
        }

        .btn-secondary {
            background: rgba(255, 255, 255, 0.05);
            border: 1px solid rgba(255, 255, 255, 0.1);
            color: var(--text);
            box-shadow: none;
            margin-top: 1rem;
        }

        .btn-secondary:hover {
            background: rgba(255, 255, 255, 0.08);
            box-shadow: none;
        }

        .variables-card {
            background: rgba(0, 0, 0, 0.2);
            border: 1px solid rgba(255, 255, 255, 0.03);
            border-radius: 16px;
            padding: 1.5rem;
            font-family: monospace;
            font-size: 0.85rem;
            overflow-x: auto;
            color: #34d399;
        }

        .variables-title {
            font-size: 0.8rem;
            text-transform: uppercase;
            letter-spacing: 0.05em;
            color: var(--text-muted);
            margin-bottom: 0.75rem;
            font-weight: 600;
        }

        .fade-in {
            animation: fadeIn 0.4s ease-out forwards;
        }

        @keyframes fadeIn {
            from { opacity: 0; transform: translateY(10px); }
            to { opacity: 1; transform: translateY(0); }
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="card fade-in">
            <h1>Todo List Process</h1>
            <div class="subtitle">Durable WebAssembly BPMN orchestration in Rust wasmee</div>

            <div id="status-container" style="text-align: center;">
                <!-- Status Badge and Active State goes here -->
            </div>

            <form id="dynamic-form" onsubmit="event.preventDefault(); submitTask();">
                <div id="form-fields">
                    <!-- JSON Schema widgets will be rendered here -->
                </div>
                <button type="submit" id="submit-btn" class="btn" style="margin-top: 1.5rem; display: none;">Submit Task</button>
            </form>

            <button onclick="resetProcess()" id="reset-btn" class="btn btn-secondary">Restart Process</button>
        </div>

        <div class="card fade-in" style="animation-delay: 0.1s;">
            <div class="variables-title">Process Variables</div>
            <pre class="variables-card" id="variables-json">{}</pre>
        </div>
    </div>

    <script>
        async function updateUI() {
            const res = await fetch('/api/state');
            const state = await res.json();

            // Render status
            const statusContainer = document.getElementById('status-container');
            const submitBtn = document.getElementById('submit-btn');
            
            if (state.completed) {
                statusContainer.innerHTML = '<div class="status-badge completed">✓ Process Completed</div>' +
                    '<p style="text-align: center; color: var(--text-muted); font-size: 0.95rem; margin-bottom: 1.5rem;">' +
                    'All steps of the BPMN todo process completed successfully!' +
                    '</p>';
                document.getElementById('form-fields').innerHTML = '';
                submitBtn.style.display = 'none';
            } else {
                const activeTask = state.active_nodes[0] || 'Unknown';
                const taskLabel = activeTask === 'add_todo' ? 'Add Todo Item' : 'Complete Todo Item';
                statusContainer.innerHTML = '<div class="status-badge">⚡ Active Task: ' + taskLabel + '</div>';
                
                // Render widgets
                const formFields = document.getElementById('form-fields');
                formFields.innerHTML = '';
                
                if (state.widgets && state.widgets.length > 0) {
                    state.widgets.forEach(w => {
                        const group = document.createElement('div');
                        group.className = 'form-group';
                        
                        const label = document.createElement('label');
                        label.textContent = w.label;
                        group.appendChild(label);

                        if (w.widget === 'checkbox' || w.type === 'boolean') {
                            const container = document.createElement('label');
                            container.className = 'checkbox-container';
                            
                            const input = document.createElement('input');
                            input.type = 'checkbox';
                            input.name = w.name;
                            input.checked = !!w.value;
                            
                            const mark = document.createElement('span');
                            mark.className = 'checkmark';
                            
                            container.appendChild(input);
                            container.appendChild(mark);
                            
                            const text = document.createElement('span');
                            text.textContent = 'Check to complete';
                            container.appendChild(text);
                            
                            group.appendChild(container);
                        } else if (w.widget === 'select' && w.options) {
                            const select = document.createElement('select');
                            select.name = w.name;
                            w.options.forEach(opt => {
                                const option = document.createElement('option');
                                option.value = opt;
                                option.textContent = opt;
                                if (opt === w.value) option.selected = true;
                                select.appendChild(option);
                            });
                            group.appendChild(select);
                        } else {
                            const input = document.createElement('input');
                            input.type = 'text';
                            input.name = w.name;
                            input.value = w.value || '';
                            if (w.required) input.required = true;
                            
                            // Check if field is readOnly for complete_todo step
                            if (activeTask === 'complete_todo' && (w.name === 'todo_item' || w.name === 'priority')) {
                                input.readOnly = true;
                            }
                            
                            group.appendChild(input);
                        }
                        
                        formFields.appendChild(group);
                    });
                    submitBtn.style.display = 'block';
                } else {
                    submitBtn.style.display = 'none';
                }
            }

            // Render variables JSON
            document.getElementById('variables-json').textContent = JSON.stringify(state.variables, null, 2);
        }

        async function submitTask() {
            const form = document.getElementById('dynamic-form');
            const formData = new FormData(form);
            const payload = {};
            
            // Collect fields
            for (let [key, val] of formData.entries()) {
                payload[key] = val;
            }
            
            // Checkboxes might not be in FormData if unchecked, or value is "on"
            const checkboxes = form.querySelectorAll('input[type="checkbox"]');
            checkboxes.forEach(cb => {
                payload[cb.name] = cb.checked;
            });

            await fetch('/api/submit', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(payload)
            });

            form.reset();
            updateUI();
        }

        async function resetProcess() {
            await fetch('/api/reset', { method: 'POST' });
            updateUI();
        }

        // Init UI
        updateUI();
    </script>
</body>
</html>
`
