package olme

import (
	"context"
)

// InstanceMeta holds execution lifecycle data for OCC (Optimistic Concurrency Control).
type InstanceMeta struct {
	InstanceID string `json:"instance_id"`
	WasmHash   string `json:"wasm_hash"`
	Version    int    `json:"version"`
	Completed  bool   `json:"completed"`
	Metadata   []byte `json:"metadata,omitempty"`
}

// OplogEntry represents a recorded host api call.
type OplogEntry struct {
	CallIndex       int    `json:"call_index"`
	ApiName         string `json:"api_name"`
	RequestPayload  []byte `json:"request_payload"`
	ResponsePayload []byte `json:"response_payload"`
}

// SnapshotStore defines the abstraction layer for saving/loading checkpoints.
type SnapshotStore interface {
	SaveSnapshot(ctx context.Context, id string, snapshot []byte) error
	LoadSnapshot(ctx context.Context, id string) ([]byte, error)
	DeleteSnapshot(ctx context.Context, id string) error

	// Delta Snapshots (dirty page segments)
	SaveDeltas(ctx context.Context, id string, deltas map[int][]byte) error
	LoadDeltas(ctx context.Context, id string) (map[int][]byte, error)
	TruncateDeltas(ctx context.Context, id string) error

	// Oplog Entries
	SaveOplog(ctx context.Context, id string, entry OplogEntry) error
	LoadOplog(ctx context.Context, id string) ([]OplogEntry, error)
	TruncateOplog(ctx context.Context, id string, beforeCallIndex int) error

	// Metadata
	SaveMetadata(ctx context.Context, meta *InstanceMeta) (bool, error)
	LoadMetadata(ctx context.Context, id string) (*InstanceMeta, error)
}
