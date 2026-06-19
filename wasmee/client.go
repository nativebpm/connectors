package wasmee

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"gitlab.com/nativebpm/olme"
)

// Bytes is a custom type that serializes/deserializes byte slices as JSON arrays of numbers.
type Bytes []byte

func (b Bytes) MarshalJSON() ([]byte, error) {
	if b == nil {
		return []byte("[]"), nil
	}
	var buf bytes.Buffer
	buf.WriteByte('[')
	for i, x := range b {
		if i > 0 {
			buf.WriteByte(',')
		}
		buf.WriteString(strconv.Itoa(int(x)))
	}
	buf.WriteByte(']')
	return buf.Bytes(), nil
}

func (b *Bytes) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		*b = nil
		return nil
	}
	var arr []int
	if err := json.Unmarshal(data, &arr); err != nil {
		return err
	}
	res := make([]byte, len(arr))
	for i, x := range arr {
		res[i] = byte(x)
	}
	*b = res
	return nil
}

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

type OplogEntry struct {
	CallIndex       int    `json:"call_index"`
	ApiName         string `json:"api_name"`
	RequestPayload  Bytes  `json:"request_payload"`
	ResponsePayload Bytes  `json:"response_payload"`
}

type ExecuteRequest struct {
	InstanceID     string            `json:"instance_id"`
	Entrypoint     string            `json:"entrypoint"`
	Params         []uint64          `json:"params"`
	BaseSnapshot   Bytes             `json:"base_snapshot"`
	MemoryDeltas   map[string]Bytes  `json:"memory_deltas"`
	Oplog          []OplogEntry      `json:"oplog"`
	ExchangeBuffer Bytes             `json:"exchange_buffer"`
}

type CheckpointData struct {
	Memory   Bytes `json:"memory"`
	OplogLen int   `json:"oplog_len"`
}

type ExecuteResponse struct {
	Crashed       bool              `json:"crashed"`
	Error         string            `json:"error"`
	FinalDeltas   map[string]Bytes  `json:"final_deltas"`
	FinalOplog    []OplogEntry      `json:"final_oplog"`
	Checkpoints   []CheckpointData  `json:"checkpoints"`
	ResponseBytes Bytes             `json:"response_bytes"`
}

// Execute triggers the execution of guest WASM on the Rust server.
func (r *Runner) Execute(ctx context.Context, session *Session, entrypoint string, exchangeBuffer []byte, params ...uint64) (bool, []byte, error) {
	baseSnapshotBytes, err := session.State.LoadSnapshot(ctx)
	if err != nil {
		baseSnapshotBytes = nil
	}
	baseSnapshot := Bytes(baseSnapshotBytes)

	rawDeltas, err := session.State.LoadDeltas(ctx)
	if err != nil {
		rawDeltas = nil
	}
	deltas := make(map[string]Bytes)
	for k, v := range rawDeltas {
		deltas[fmt.Sprintf("%d", k)] = Bytes(v)
	}

	rawOplog, err := session.State.LoadOplog(ctx)
	if err != nil {
		rawOplog = nil
	}
	var oplog []OplogEntry
	for _, entry := range rawOplog {
		oplog = append(oplog, OplogEntry{
			CallIndex:       entry.CallIndex,
			ApiName:         entry.ApiName,
			RequestPayload:  Bytes(entry.RequestPayload),
			ResponsePayload: Bytes(entry.ResponsePayload),
		})
	}

	if baseSnapshot == nil {
		baseSnapshot = Bytes{}
	}
	if params == nil {
		params = []uint64{}
	}
	if oplog == nil {
		oplog = []OplogEntry{}
	}

	reqBody := ExecuteRequest{
		InstanceID:     session.InstanceID,
		Entrypoint:     entrypoint,
		Params:         params,
		BaseSnapshot:   baseSnapshot,
		MemoryDeltas:   deltas,
		Oplog:          oplog,
		ExchangeBuffer: Bytes(exchangeBuffer),
	}

	jsonBytes, err := json.Marshal(reqBody)
	if err != nil {
		return false, nil, fmt.Errorf("failed to marshal request payload: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, r.httpAddr+"/execute", bytes.NewReader(jsonBytes))
	if err != nil {
		return false, nil, fmt.Errorf("failed to create http request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return false, nil, fmt.Errorf("failed to execute HTTP call: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return false, nil, fmt.Errorf("http execution failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var respBody ExecuteResponse
	if err := json.NewDecoder(resp.Body).Decode(&respBody); err != nil {
		return false, nil, fmt.Errorf("failed to decode response payload: %w", err)
	}

	// Process checkpoints
	currentSavedIndex := len(rawOplog)
	for _, cp := range respBody.Checkpoints {
		// Save oplog entries that happened BEFORE this checkpoint!
		for _, entry := range respBody.FinalOplog {
			if entry.CallIndex <= cp.OplogLen && entry.CallIndex > currentSavedIndex {
				oe := olme.OplogEntry{
					CallIndex:       entry.CallIndex,
					ApiName:         entry.ApiName,
					RequestPayload:  []byte(entry.RequestPayload),
					ResponsePayload: []byte(entry.ResponsePayload),
				}
				if err := session.State.AddOplogEntry(ctx, oe); err != nil {
					return false, nil, fmt.Errorf("failed to save oplog entry: %w", err)
				}
			}
		}

		if err := session.State.Checkpoint(ctx, []byte(cp.Memory)); err != nil {
			return false, nil, fmt.Errorf("failed to save checkpoint: %w", err)
		}

		currentSavedIndex = cp.OplogLen

		if session.simulateCrash {
			session.crashed = true
			return true, nil, errors.New("simulated_host_crash")
		}
	}

	// Save final oplog (for entries after the last checkpoint, if any)
	for _, entry := range respBody.FinalOplog {
		if entry.CallIndex > currentSavedIndex {
			oe := olme.OplogEntry{
				CallIndex:       entry.CallIndex,
				ApiName:         entry.ApiName,
				RequestPayload:  []byte(entry.RequestPayload),
				ResponsePayload: []byte(entry.ResponsePayload),
			}
			if err := session.State.AddOplogEntry(ctx, oe); err != nil {
				return false, nil, fmt.Errorf("failed to save oplog entry: %w", err)
			}
		}
	}

	if respBody.Crashed {
		return true, nil, errors.New(respBody.Error)
	}

	return false, []byte(respBody.ResponseBytes), nil
}

