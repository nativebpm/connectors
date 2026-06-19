package wasmee

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/nativebpm/connectors/wasmee/olme"
	"github.com/nativebpm/connectors/wasmee/pb"
	"google.golang.org/protobuf/proto"
)

// Runner manages connection to the Rust WASMEE server and execution context.
type Runner struct {
	httpAddr  string
	wasmBytes []byte
}

// NewRunner creates a new Go wasmee Runner.
func NewRunner(ctx context.Context, wasmBytes []byte, httpAddr string) (*Runner, error) {
	if !strings.HasPrefix(httpAddr, "http://") && !strings.HasPrefix(httpAddr, "https://") {
		httpAddr = "http://" + httpAddr
	}
	return &Runner{
		httpAddr:  httpAddr,
		wasmBytes: wasmBytes,
	}, nil
}

// Close is a no-op for the HTTP runner.
func (r *Runner) Close(ctx context.Context) error {
	return nil
}

// Session represents the active execution session.
type Session struct {
	InstanceID    string
	State         *olme.SessionState
	ApiHandler    func(apiName string, request []byte) ([]byte, error)
	crashed       bool
	simulateCrash bool
}

// NewSession creates an execution session bound to OLME state.
func NewSession(instanceID string, state *olme.SessionState) *Session {
	return &Session{
		InstanceID: instanceID,
		State:      state,
	}
}

// EnableCrashSimulation instructs the runner to simulate host failures on checkpoint triggers.
func (s *Session) EnableCrashSimulation(enable bool) {
	s.simulateCrash = enable
}

// Execute triggers the execution of guest WASM on the Rust server.
func (r *Runner) Execute(ctx context.Context, session *Session, entrypoint string, exchangeBuffer []byte, params ...uint64) (bool, []byte, error) {
	baseSnapshotBytes, err := session.State.LoadSnapshot(ctx)
	if err != nil {
		baseSnapshotBytes = nil
	}

	rawDeltas, err := session.State.LoadDeltas(ctx)
	if err != nil {
		rawDeltas = nil
	}
	deltas := make(map[int32][]byte)
	for k, v := range rawDeltas {
		deltas[int32(k)] = v
	}

	rawOplog, err := session.State.LoadOplog(ctx)
	if err != nil {
		rawOplog = nil
	}
	var oplog []*pb.OplogEntry
	for _, entry := range rawOplog {
		oplog = append(oplog, &pb.OplogEntry{
			CallIndex:       int32(entry.CallIndex),
			ApiName:         entry.ApiName,
			RequestPayload:  entry.RequestPayload,
			ResponsePayload: entry.ResponsePayload,
		})
	}

	if baseSnapshotBytes == nil {
		baseSnapshotBytes = []byte{}
	}
	if params == nil {
		params = []uint64{}
	}
	if oplog == nil {
		oplog = []*pb.OplogEntry{}
	}

	reqBody := &pb.ExecuteRequest{
		InstanceId:     session.InstanceID,
		Entrypoint:     entrypoint,
		Params:         params,
		BaseSnapshot:   baseSnapshotBytes,
		MemoryDeltas:   deltas,
		Oplog:          oplog,
		ExchangeBuffer: exchangeBuffer,
	}

	protoBytes, err := proto.Marshal(reqBody)
	if err != nil {
		return false, nil, fmt.Errorf("failed to marshal protobuf payload: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, r.httpAddr+"/execute", bytes.NewReader(protoBytes))
	if err != nil {
		return false, nil, fmt.Errorf("failed to create http request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/x-protobuf")

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return false, nil, fmt.Errorf("failed to execute HTTP call: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return false, nil, fmt.Errorf("http execution failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var respBody pb.ExecuteResponse
	if err := proto.Unmarshal(bodyBytes, &respBody); err != nil {
		return false, nil, fmt.Errorf("failed to decode protobuf response: %w", err)
	}

	// Process checkpoints
	currentSavedIndex := len(rawOplog)
	for _, cp := range respBody.Checkpoints {
		// Save oplog entries that happened BEFORE this checkpoint!
		for _, entry := range respBody.FinalOplog {
			if int(entry.CallIndex) <= int(cp.OplogLen) && int(entry.CallIndex) > currentSavedIndex {
				oe := olme.OplogEntry{
					CallIndex:       int(entry.CallIndex),
					ApiName:         entry.ApiName,
					RequestPayload:  entry.RequestPayload,
					ResponsePayload: entry.ResponsePayload,
				}
				if err := session.State.AddOplogEntry(ctx, oe); err != nil {
					return false, nil, fmt.Errorf("failed to save oplog entry: %w", err)
				}
			}
		}

		if err := session.State.Checkpoint(ctx, cp.Memory); err != nil {
			return false, nil, fmt.Errorf("failed to save checkpoint: %w", err)
		}

		currentSavedIndex = int(cp.OplogLen)

		if session.simulateCrash {
			session.crashed = true
			return true, nil, errors.New("simulated_host_crash")
		}
	}

	// Save final oplog (for entries after the last checkpoint, if any)
	for _, entry := range respBody.FinalOplog {
		if int(entry.CallIndex) > currentSavedIndex {
			oe := olme.OplogEntry{
				CallIndex:       int(entry.CallIndex),
				ApiName:         entry.ApiName,
				RequestPayload:  entry.RequestPayload,
				ResponsePayload: entry.ResponsePayload,
			}
			if err := session.State.AddOplogEntry(ctx, oe); err != nil {
				return false, nil, fmt.Errorf("failed to save oplog entry: %w", err)
			}
		}
	}

	if respBody.Crashed {
		return true, nil, errors.New(respBody.Error)
	}

	return false, respBody.ResponseBytes, nil
}

