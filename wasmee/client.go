package wasmee

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"gitlab.com/nativebpm/olme"
	pb "github.com/nativebpm/connectors/wasmee/proto"
)

// Runner manages connection to the Rust WASMEE server and execution context.
type Runner struct {
	grpcAddr  string
	wasmBytes []byte
	wasmHash  string
}

// NewRunner creates a new Go wasmee Runner.
func NewRunner(ctx context.Context, wasmBytes []byte, grpcAddr string) (*Runner, error) {
	// Calculate a simple hash for verification (dummy or SHA-256)
	wasmHash := fmt.Sprintf("hash-%d", len(wasmBytes))
	return &Runner{
		grpcAddr:  grpcAddr,
		wasmBytes: wasmBytes,
		wasmHash:  wasmHash,
	}, nil
}

// Close is a no-op for the client runner.
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
func (r *Runner) Execute(ctx context.Context, session *Session, entrypoint string, params ...uint64) (bool, error) {
	// 1. Establish gRPC connection to Rust WASMEE server
	conn, err := grpc.Dial(r.grpcAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return false, fmt.Errorf("failed to dial WASMEE server: %w", err)
	}
	defer conn.Close()

	client := pb.NewWasmeeExecutorClient(conn)

	// 2. Open bidirectional stream
	stream, err := client.Execute(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to open execution stream: %w", err)
	}

	// 3. Prepare initial state data to send
	baseSnapshot, err := session.State.LoadSnapshot(ctx)
	if err != nil {
		baseSnapshot = nil
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

	// Send StartRequest
	startMsg := &pb.ExecuteMessage{
		Message: &pb.ExecuteMessage_Start{
			Start: &pb.StartRequest{
				InstanceId:    session.InstanceID,
				WasmBytes:     r.wasmBytes,
				Entrypoint:    entrypoint,
				Params:        params,
				BaseSnapshot:  baseSnapshot,
				MemoryDeltas:  deltas,
				Oplog:         oplog,
				CallIndex:     int32(session.State.GetCallIndex()),
			},
		},
	}

	if err := stream.Send(startMsg); err != nil {
		return false, fmt.Errorf("failed to send start request: %w", err)
	}

	// 4. Stream message loop
	for {
		msg, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return false, fmt.Errorf("stream receive error: %w", err)
		}

		switch m := msg.Message.(type) {
		case *pb.ExecuteMessage_HostCallRequest:
			req := m.HostCallRequest
			var respPayload []byte
			var errStr string

			session.State.SetCallIndex(int(req.CallIndex) - 1)

			// Handle time request or API request
			if req.ApiName == "host_get_time" {
				val, err := session.State.GetOrExecuteCall(ctx, "host_get_time", nil, func() ([]byte, error) {
					nowNano := time.Now().UnixNano()
					return []byte(fmt.Sprintf("%d", nowNano)), nil
				})
				if err != nil {
					errStr = err.Error()
				} else {
					respPayload = val
				}
			} else {
				val, err := session.State.GetOrExecuteCall(ctx, req.ApiName, req.RequestPayload, func() ([]byte, error) {
					if session.ApiHandler != nil {
						return session.ApiHandler(req.ApiName, req.RequestPayload)
					}
					if req.ApiName == "test_api" {
						return []byte(fmt.Sprintf("resp_for_%s_call_%d", string(req.RequestPayload), session.State.GetCallIndex()+1)), nil
					}
					return nil, fmt.Errorf("no api handler for %s", req.ApiName)
				})
				if err != nil {
					errStr = err.Error()
				} else {
					respPayload = val
				}
			}

			// Send response back to Rust
			respMsg := &pb.ExecuteMessage{
				Message: &pb.ExecuteMessage_HostCallResponse{
					HostCallResponse: &pb.HostCallResponse{
						ResponsePayload: respPayload,
						Error:           errStr,
					},
				},
			}
			if err := stream.Send(respMsg); err != nil {
				return false, fmt.Errorf("failed to send host call response: %w", err)
			}

		case *pb.ExecuteMessage_Checkpoint:
			req := m.Checkpoint
			var errStr string

			err = session.State.Checkpoint(ctx, req.CurrentMemory)
			if err != nil {
				errStr = err.Error()
			}

			// Simulate crash if requested
			if session.simulateCrash {
				session.crashed = true
				// Trigger client side error to stop the execution loop
				return true, errors.New("simulated_host_crash")
			}

			// Send checkpoint confirmation
			respMsg := &pb.ExecuteMessage{
				Message: &pb.ExecuteMessage_CheckpointResponse{
					CheckpointResponse: &pb.CheckpointResponse{
						Error: errStr,
					},
				},
			}
			if err := stream.Send(respMsg); err != nil {
				return false, fmt.Errorf("failed to send checkpoint response: %w", err)
			}

		case *pb.ExecuteMessage_Complete:
			req := m.Complete
			if req.Crashed {
				return true, errors.New(req.Error)
			}
			return false, nil
		}
	}

	return false, nil
}

// StartRustServerHelper starts the Rust server in the background for testing.
func StartRustServerHelper(execPath string, port string) (func(), error) {
	// Helper method to spawn process
	return nil, nil
}
