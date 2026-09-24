package manticore

import (
	"time"
)

// ProcessInstance represents a BPMN process instance projection record in Manticore.
type ProcessInstance struct {
	ID            uint64    `json:"id"`
	ProcessKey    string    `json:"process_key"`
	Version       int       `json:"version"`
	Status        string    `json:"status"` // e.g. RUNNING, WAITING_USER_TASK, COMPLETED, INCIDENT
	TenantID      int       `json:"tenant_id"`
	Assignee      string    `json:"assignee"`
	StartTime     time.Time `json:"start_time"`
	EndTime       time.Time `json:"end_time"`
	DurationMs    int64     `json:"duration_ms"`
	Amount        float64   `json:"amount"`
	ErrorMessage  string    `json:"error_message"`
	VariablesJSON string    `json:"variables_json"`
}

// AuditLog represents an execution event or error trace in Manticore.
type AuditLog struct {
	ID          uint64    `json:"id"`
	InstanceID  uint64    `json:"instance_id"`
	ProcessKey  string    `json:"process_key"`
	LogLevel    string    `json:"log_level"` // INFO, WARN, ERROR, INCIDENT
	ActivityID  string    `json:"activity_id"`
	Timestamp   time.Time `json:"timestamp"`
	Payload     string    `json:"payload"`
}

// SearchFilter specifies filtering criteria for querying process instances.
type SearchFilter struct {
	ProcessKey   string  `json:"process_key"`
	Status       string  `json:"status"`
	TenantID     *int    `json:"tenant_id"`
	Assignee     string  `json:"assignee"`
	QueryText    string  `json:"query_text"` // MATCH(...) full-text query
	MinAmount    *float64 `json:"min_amount"`
	MaxAmount    *float64 `json:"max_amount"`
	Limit        int     `json:"limit"`
	Offset       int     `json:"offset"`
}

// SearchResult holds search results along with pagination metadata.
type SearchResult struct {
	Total     int64             `json:"total"`
	Instances []ProcessInstance `json:"instances"`
}
