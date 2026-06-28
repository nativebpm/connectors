package wasmee

import (
	"context"
	"fmt"
)

// FluentRunner provides a fully fluent API for setting up and executing a WASM module instance on WASMEE.
// It tracks sticky errors so that chaining is clean and safe.
type FluentRunner struct {
	ctx            context.Context
	httpAddr       string
	wasmBytes      []byte
	store          SnapshotStore
	instanceID     string
	entrypoint     string
	exchangeBuffer []byte
	params         []uint64
	err            error
	crashed        bool
	simulateCrash  bool
	responseBytes  []byte
}

// NewFluentRunner creates a new FluentRunner.
func NewFluentRunner() *FluentRunner {
	return &FluentRunner{
		ctx:        context.Background(),
		entrypoint: "execute", // Default entrypoint
	}
}

// WithContext sets the execution context.
func (r *FluentRunner) WithContext(ctx context.Context) *FluentRunner {
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

// WithServerAddress sets the WASMEE HTTP server address.
func (r *FluentRunner) WithServerAddress(addr string) *FluentRunner {
	if r.err != nil {
		return r
	}
	r.httpAddr = addr
	return r
}

// WithWasmBytes sets the Wasm module bytes.
func (r *FluentRunner) WithWasmBytes(wasmBytes []byte) *FluentRunner {
	if r.err != nil {
		return r
	}
	r.wasmBytes = wasmBytes
	return r
}

// WithStore sets the SnapshotStore to use.
func (r *FluentRunner) WithStore(store SnapshotStore) *FluentRunner {
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

// WithSessionID configures the instance/session ID.
func (r *FluentRunner) WithSessionID(instanceID string) *FluentRunner {
	if r.err != nil {
		return r
	}
	r.instanceID = instanceID
	return r
}

// WithEntrypoint configures the function name to call in Wasm.
func (r *FluentRunner) WithEntrypoint(entrypoint string) *FluentRunner {
	if r.err != nil {
		return r
	}
	r.entrypoint = entrypoint
	return r
}

// WithExchangeBuffer sets the inputs in the exchange buffer.
func (r *FluentRunner) WithExchangeBuffer(buf []byte) *FluentRunner {
	if r.err != nil {
		return r
	}
	r.exchangeBuffer = buf
	return r
}

// WithArgs sets the arguments list.
func (r *FluentRunner) WithArgs(params ...uint64) *FluentRunner {
	if r.err != nil {
		return r
	}
	r.params = params
	return r
}

// WithCrashSimulation configures whether to simulate host crash on checkpoints.
func (r *FluentRunner) WithCrashSimulation(enable bool) *FluentRunner {
	if r.err != nil {
		return r
	}
	r.simulateCrash = enable
	return r
}


// Error returns any accumulated error.
func (r *FluentRunner) Error() error {
	return r.err
}

// Response returns the execution output bytes.
func (r *FluentRunner) Response() []byte {
	return r.responseBytes
}

// Run executes the module on WASMEE and returns whether it crashed and any error.
func (r *FluentRunner) Run() (bool, error) {
	if r.err != nil {
		return false, r.err
	}
	if r.instanceID == "" {
		return false, fmt.Errorf("session/instance ID is required")
	}
	if r.httpAddr == "" {
		return false, fmt.Errorf("server address is required")
	}
	if r.store == nil {
		return false, fmt.Errorf("snapshot store is required")
	}

	state := NewSessionState(r.instanceID, r.store)
	if err := state.Load(r.ctx); err != nil {
		return false, fmt.Errorf("failed to load session state: %w", err)
	}

	runner, err := NewRunner(r.ctx, r.wasmBytes, r.httpAddr)
	if err != nil {
		return false, fmt.Errorf("failed to create runner: %w", err)
	}

	session := NewSession(r.instanceID, state)
	session.EnableCrashSimulation(r.simulateCrash)

	crashed, respBytes, err := runner.Execute(r.ctx, session, r.entrypoint, r.exchangeBuffer, r.params...)
	r.crashed = crashed
	r.responseBytes = respBytes
	r.err = err

	return crashed, err
}
