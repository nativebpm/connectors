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

	// 1. Initialize client
	client, err := nativebpm.NewClient("http://localhost:8080")
	if err != nil {
		slog.Error("Failed to initialize NativeBPM SDK client", "error", err)
		return
	}

	// 2. Read complex BPMN file
	bpmnXML, err := os.ReadFile("complex.bpmn")
	if err != nil {
		slog.Error("Failed to read complex.bpmn file", "error", err)
		return
	}

	// 3. Deploy
	deployResp, err := client.Deploy("complexProcess", "Complex Branching Process").
		XML(bpmnXML).
		Send(ctx)
	if err != nil {
		slog.Error("Deploy failed", "error", err)
		return
	}
	slog.Info("Complex Process deployed successfully", "processID", deployResp.ProcessID)

	// ----------------------------------------------------
	// Scenario A: Low Amount ($500) -> Auto Approved (No User Task)
	// ----------------------------------------------------
	slog.Info("=== Executing Scenario A: Low Amount ($500) ===")
	instA_ID := uuid.New().String()
	startRespA, err := client.StartProcessInstance("complexProcess").
		InstanceID(instA_ID).
		Variable("amount", 500).
		Send(ctx)
	if err != nil {
		slog.Error("Scenario A start failed", "error", err)
		return
	}
	slog.Info("Scenario A start request accepted", "instance_id", startRespA.InstanceID, "status", startRespA.Status)

	// Poll until the instance is completed (auto approved)
	var piA *nativebpm.ProcessInstance
	for i := 0; i < 50; i++ {
		piA, err = client.GetInstance(ctx, startRespA.InstanceID)
		if err == nil && piA.Completed {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if err != nil {
		slog.Error("Failed to fetch Scenario A instance", "error", err)
		return
	}
	slog.Info("Scenario A Completed", "id", piA.ID, "completed", piA.Completed, "waiting", piA.WaitingActivityInstances)

	// ----------------------------------------------------
	// Scenario B: High Amount ($1500) -> Requires Manager Approval (User Task)
	// ----------------------------------------------------
	slog.Info("=== Executing Scenario B: High Amount ($1500) ===")
	instB_ID := uuid.New().String()
	startRespB, err := client.StartProcessInstance("complexProcess").
		InstanceID(instB_ID).
		Variable("amount", 1500).
		Send(ctx)
	if err != nil {
		slog.Error("Scenario B start failed", "error", err)
		return
	}
	slog.Info("Scenario B start request accepted", "instance_id", startRespB.InstanceID, "status", startRespB.Status)

	// Poll until the instance is active and waiting for task
	var piB *nativebpm.ProcessInstance
	for i := 0; i < 50; i++ {
		piB, err = client.GetInstance(ctx, startRespB.InstanceID)
		if err == nil && len(piB.WaitingActivityInstances) > 0 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if err != nil {
		slog.Error("Failed to fetch Scenario B instance", "error", err)
		return
	}
	slog.Info("Scenario B Started", "id", piB.ID, "completed", piB.Completed, "waiting", piB.WaitingActivityInstances)

	// Complete Manager Approval task for Scenario B
	if len(piB.WaitingActivityInstances) > 0 {
		taskID := piB.WaitingActivityInstances[0]
		slog.Info("Completing Manager Approval task for Scenario B", "taskID", taskID)
		piB, err = client.CompleteTask(piB.ID, taskID).
			Variable("manager_approved", true).
			Send(ctx)
		if err != nil {
			slog.Error("Failed to complete Manager Approval task", "error", err)
			return
		}
		slog.Info("Scenario B Task Completed", "id", piB.ID, "completed", piB.Completed, "waiting", piB.WaitingActivityInstances)
	}

	// ----------------------------------------------------
	// Print Logs to compare execution paths
	// ----------------------------------------------------
	slog.Info("=== Fetching Audit Logs ===")

	logsA, err := client.GetInstanceLogs(ctx, instA_ID)
	if err == nil {
		slog.Info("--- Logs for Scenario A (Low Amount) ---")
		for _, l := range logsA {
			slog.Info("Audit Log", "nodeID", l.NodeID, "nodeName", l.NodeName, "action", l.Action)
		}
	}

	logsB, err := client.GetInstanceLogs(ctx, instB_ID)
	if err == nil {
		slog.Info("--- Logs for Scenario B (High Amount) ---")
		for _, l := range logsB {
			slog.Info("Audit Log", "nodeID", l.NodeID, "nodeName", l.NodeName, "action", l.Action)
		}
	}
}
