package nativebpm

import (
	"encoding/json"
	"time"
)

// ProcessDefinition represents a deployed BPMN/DMN process definition.
type ProcessDefinition struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	XMLData    []byte    `json:"xml_data"`
	DeployedAt time.Time `json:"deployed_at"`
}

// ProcessInstance represents the active state of a process run.
type ProcessInstance struct {
	ID             string                 `json:"id"`
	ProcessID      string                 `json:"process_id"`
	ActiveTokens   []string               `json:"active_tokens"`
	WaitingTokens  []string               `json:"waiting_tokens"`
	CompletedTasks []string               `json:"completed_tasks"`
	Variables      map[string]interface{} `json:"variables"`
	Completed      bool                   `json:"completed"`
}

// ProcessInstanceRecord represents a persistent snapshot of a process execution state.
type ProcessInstanceRecord struct {
	ID        string    `json:"id"`
	ProcessID string    `json:"process_id"`
	State     []byte    `json:"state"` // JSON serialized ProcessInstance
	Version   int       `json:"version"`
	Completed bool      `json:"completed"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ParseState deserializes the raw state bytes into a structured ProcessInstance.
func (r *ProcessInstanceRecord) ParseState() (*ProcessInstance, error) {
	if len(r.State) == 0 {
		return nil, nil
	}
	var pi ProcessInstance
	if err := json.Unmarshal(r.State, &pi); err != nil {
		return nil, err
	}
	return &pi, nil
}

// LogRecord captures a single process execution event for audit trailing.
type LogRecord struct {
	ID         string    `json:"id"`
	InstanceID string    `json:"instance_id"`
	NodeID     string    `json:"node_id"`
	NodeName   string    `json:"node_name"`
	NodeType   string    `json:"node_type"`
	Action     string    `json:"action"`              // e.g. "start", "enter", "leave", "wait", "complete", "error"
	Variables  []byte    `json:"variables,omitempty"` // Serialized updated variables
	Timestamp  time.Time `json:"timestamp"`
}

// ParseVariables deserializes the raw variable bytes from the log entry.
func (l *LogRecord) ParseVariables() (map[string]interface{}, error) {
	if len(l.Variables) == 0 {
		return nil, nil
	}
	var vars map[string]interface{}
	if err := json.Unmarshal(l.Variables, &vars); err != nil {
		return nil, err
	}
	return vars, nil
}

// DeploymentResponse represents the server response after a successful process deployment.
type DeploymentResponse struct {
	Status    string `json:"status"`
	ProcessID string `json:"process_id"`
	Name      string `json:"name"`
}

// StartInstanceResponse represents the server response after launching a process instance asynchronously.
type StartInstanceResponse struct {
	Status     string `json:"status"`
	InstanceID string `json:"instance_id"`
}

