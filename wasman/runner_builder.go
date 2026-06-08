//go:build !wasm

package wasman

import (
	"context"
	"fmt"
)

// Runner provides a fully fluent API for setting up and executing a WASM module instance.
// It integrates memory/file snapshot store creation, engine loading, session configuration,
// and execution into a single chained workflow with sticky error tracking.
type Runner struct {
	ctx             context.Context
	wasmPath        string
	wasmBytes       []byte
	store           SnapshotStore
	instanceID      string
	entrypoint      string
	apiHandler      func(apiName string, request []byte) ([]byte, error)
	downloadHandler func() ([]byte, error)
	uploadHandler   func(payload []byte) error
	err             error
	crashed         bool
}

// NewRunner creates a new Runner with default context.
func NewRunner() *Runner {
	return &Runner{
		ctx:        context.Background(),
		entrypoint: "run", // Default entrypoint
	}
}

// WithContext overrides the default context.
func (r *Runner) WithContext(ctx context.Context) *Runner {
	if r.err != nil {
		return r
	}
	if ctx == nil {
		r.err = fmt.Errorf("context cannot be nil")
		return r
	}
	r.ctx = ctx
	return r
}

// WithWasmPath specifies the file path to the WASM module.
func (r *Runner) WithWasmPath(wasmPath string) *Runner {
	if r.err != nil {
		return r
	}
	r.wasmPath = wasmPath
	return r
}

// WithWasmBytes loads the WASM module directly from memory bytes.
func (r *Runner) WithWasmBytes(wasmBytes []byte) *Runner {
	if r.err != nil {
		return r
	}
	r.wasmBytes = wasmBytes
	return r
}

// WithStore specifies the SnapshotStore to use.
func (r *Runner) WithStore(store SnapshotStore) *Runner {
	if r.err != nil {
		return r
	}
	if store == nil {
		r.err = fmt.Errorf("store cannot be nil")
		return r
	}
	r.store = store
	return r
}

// WithMemoryStore initializes and uses an in-memory snapshot store.
func (r *Runner) WithMemoryStore() *Runner {
	if r.err != nil {
		return r
	}
	r.store = NewMemorySnapshotStore()
	return r
}

// WithSessionID configures the instance/session ID.
func (r *Runner) WithSessionID(instanceID string) *Runner {
	if r.err != nil {
		return r
	}
	r.instanceID = instanceID
	return r
}

// WithEntrypoint configures the function name to call in the WASM module.
func (r *Runner) WithEntrypoint(entrypoint string) *Runner {
	if r.err != nil {
		return r
	}
	r.entrypoint = entrypoint
	return r
}

// WithApiHandler registers an in-memory host API call handler.
func (r *Runner) WithApiHandler(handler func(apiName string, request []byte) ([]byte, error)) *Runner {
	if r.err != nil {
		return r
	}
	r.apiHandler = handler
	return r
}

// WithDownloadHandler registers an in-memory stream download handler.
func (r *Runner) WithDownloadHandler(handler func() ([]byte, error)) *Runner {
	if r.err != nil {
		return r
	}
	r.downloadHandler = handler
	return r
}

// WithUploadHandler registers an in-memory stream upload handler.
func (r *Runner) WithUploadHandler(handler func(payload []byte) error) *Runner {
	if r.err != nil {
		return r
	}
	r.uploadHandler = handler
	return r
}

// Error returns any sticky error accumulated during chaining.
func (r *Runner) Error() error {
	return r.err
}

// Run compiles the WASM module (if needed), configures the session,
// runs the execution, and returns whether it crashed and any error.
func (r *Runner) Run() (crashed bool, err error) {
	if r.err != nil {
		return false, r.err
	}

	if r.instanceID == "" {
		return false, fmt.Errorf("session ID is required")
	}

	if r.store == nil {
		return false, fmt.Errorf("snapshot store is required")
	}

	var engine *Engine
	if len(r.wasmBytes) > 0 {
		engine, err = NewEngineWithBytes(r.wasmBytes, r.store)
	} else if r.wasmPath != "" {
		engine, err = NewEngine(r.wasmPath, r.store)
	} else {
		return false, fmt.Errorf("either WASM file path or WASM bytes must be configured")
	}

	if err != nil {
		return false, fmt.Errorf("failed to build engine: %w", err)
	}

	exec := engine.Session(r.instanceID).WithEntrypoint(r.entrypoint)
	if r.apiHandler != nil {
		exec.WithApiHandler(r.apiHandler)
	}
	if r.downloadHandler != nil {
		exec.WithDownloadHandler(r.downloadHandler)
	}
	if r.uploadHandler != nil {
		exec.WithUploadHandler(r.uploadHandler)
	}

	crashed, err = exec.Run(r.ctx)
	r.crashed = crashed
	r.err = err
	return crashed, err
}
