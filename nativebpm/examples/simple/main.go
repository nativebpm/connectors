package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/google/uuid"
	"github.com/nativebpm/connectors/nativebpm"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	ctx := context.Background()

	// 1. Initialize NativeBPM SDK client
	client, err := nativebpm.NewClient("http://localhost:8080")
	if err != nil {
		slog.Error("Failed to initialize NativeBPM SDK client", "error", err)
		return
	}

	// 2. Read BPMN process definition from file
	bpmnXML, err := os.ReadFile("simple.bpmn")
	if err != nil {
		slog.Error("Failed to read simple.bpmn file", "error", err)
		return
	}

	// 3. Deploy
	deployResp, err := client.Deploy("userTaskProcess", "User Task Process").
		XML(bpmnXML).
		Send(ctx)
	if err != nil {
		slog.Error("Deploy failed", "error", err)
		return
	}
	slog.Info("Process deployed successfully", "status", deployResp.Status, "processID", deployResp.ProcessID)

	// 4. Start instance
	instanceID := uuid.New().String()
	startResp, err := client.StartProcessInstance("userTaskProcess").
		InstanceID(instanceID).
		Variable("applicant", "Alice").
		Send(ctx)
	if err != nil {
		slog.Error("StartProcessInstance failed", "error", err)
		return
	}
	slog.Info("Instance start request accepted", "instance_id", startResp.InstanceID, "status", startResp.Status)

	// Poll until the instance is active and waiting for task
	var pi *nativebpm.ProcessInstance
	for i := 0; i < 50; i++ {
		pi, err = client.GetInstance(ctx, startResp.InstanceID)
		if err == nil && len(pi.WaitingTokens) > 0 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if err != nil {
		slog.Error("Failed to fetch instance during polling", "error", err)
		return
	}
	slog.Info("Instance started asynchronously", "id", pi.ID, "completed", pi.Completed, "waiting", pi.WaitingTokens)

	// 5. Complete task
	if len(pi.WaitingTokens) > 0 {
		taskID := pi.WaitingTokens[0]
		pi, err = client.CompleteTask(pi.ID, taskID).
			Variable("approved", true).
			Send(ctx)
		if err != nil {
			slog.Error("CompleteTask failed", "error", err)
			return
		}
		slog.Info("Task completed", "id", pi.ID, "completed", pi.Completed, "waiting", pi.WaitingTokens)
	}

	// 6. Get Logs
	logs, err := client.GetInstanceLogs(ctx, instanceID)
	if err != nil {
		slog.Error("GetInstanceLogs failed", "error", err)
		return
	}
	for _, l := range logs {
		slog.Info("Audit Log Record", "nodeID", l.NodeID, "nodeName", l.NodeName, "action", l.Action)
	}
}
