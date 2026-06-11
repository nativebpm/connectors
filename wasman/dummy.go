//go:build wasm

package wasman

import (
	"context"
)

type OplogEntry struct {
	CallIndex       int    `json:"call_index"`
	ApiName         string `json:"api_name"`
	RequestPayload  []byte `json:"request_payload"`
	ResponsePayload []byte `json:"response_payload"`
}

type InstanceMeta struct {
	InstanceID     string `json:"instance_id"`
	WasmHash       string `json:"wasm_hash"`
	Version        int    `json:"version"`
	ETag           string `json:"etag,omitempty"`
	ProcessID      string `json:"process_id,omitempty"`
	DefinitionHash string `json:"definition_hash,omitempty"`
	BusinessKey    string `json:"business_key,omitempty"`
	BpmnState      []byte `json:"bpmn_state,omitempty"`
	Completed      bool   `json:"completed,omitempty"`
}

type SnapshotStore interface {
	Save(id string, snapshot []byte) error
	Load(id string) ([]byte, error)
	Delete(id string) error

	SaveDeltas(id string, deltas map[int][]byte) error
	LoadDeltas(id string) (map[int][]byte, error)
	TruncateDeltas(id string) error

	SaveOplog(id string, callIndex int, apiName string, request []byte, response []byte) error
	LoadOplog(id string) ([]OplogEntry, error)
	TruncateOplog(id string, beforeCallIndex int) error

	SaveMetadata(meta *InstanceMeta) (bool, error)
	LoadMetadata(id string) (*InstanceMeta, error)

	SaveWasm(hash string, wasmBytes []byte) error
	LoadWasm(hash string) ([]byte, error)

	UpdateActiveIndex(ctx context.Context, id string, info []byte, completed bool) error
	LoadActiveIndex(ctx context.Context) ([]byte, error)
}

type Engine struct {
	store SnapshotStore
}

func (e *Engine) Store() SnapshotStore {
	return e.store
}

type TestRunner struct {
	err error
}

func NewTestRunner() *TestRunner {
	return &TestRunner{}
}

func (tr *TestRunner) WithContext(ctx context.Context) *TestRunner {
	return tr
}

func (tr *TestRunner) WithWasmPath(wasmPath string) *TestRunner {
	return tr
}

func (tr *TestRunner) WithStore(store SnapshotStore) *TestRunner {
	return tr
}

func (tr *TestRunner) WithSessionID(instanceID string) *TestRunner {
	return tr
}

func (tr *TestRunner) WithServer(serverAddr string) *TestRunner {
	return tr
}

func (tr *TestRunner) WithCrash(shouldCrash bool) *TestRunner {
	return tr
}

func (tr *TestRunner) Run() (crashed bool, err error) {
	return false, nil
}
