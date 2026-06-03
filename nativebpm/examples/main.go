package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/nativebpm/connectors/nativebpm"
)

const userTaskBPMN = `<?xml version="1.0" encoding="UTF-8"?>
<definitions xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xmlns="http://www.omg.org/spec/BPMN/20100524/MODEL" xmlns:bpmndi="http://www.omg.org/spec/BPMN/20100524/DI" xmlns:omgdc="http://www.omg.org/spec/DD/20100524/DC" xmlns:omgdi="http://www.omg.org/spec/DD/20100524/DI" id="Definitions_1" targetNamespace="http://bpmn.io/schema/bpmn" exporter="Camunda Modeler" exporterVersion="4.7.0">
  <process id="userTaskProcess" name="User Task Process" isExecutable="true">
    <startEvent id="StartEvent_1" name="Start">
      <outgoing>Flow_1</outgoing>
    </startEvent>
    <sequenceFlow id="Flow_1" sourceRef="StartEvent_1" targetRef="UserTask_1" />
    <userTask id="UserTask_1" name="Approve Application">
      <incoming>Flow_1</incoming>
      <outgoing>Flow_2</outgoing>
    </userTask>
    <sequenceFlow id="Flow_2" sourceRef="UserTask_1" targetRef="EndEvent_1" />
    <endEvent id="EndEvent_1" name="End">
      <incoming>Flow_2</incoming>
    </endEvent>
  </process>
  <bpmndi:BPMNDiagram id="BPMNDiagram_1">
    <bpmndi:BPMNPlane id="BPMNPlane_1" bpmnElement="userTaskProcess">
      <bpmndi:BPMNEdge id="Flow_1_di" bpmnElement="Flow_1">
        <omgdi:waypoint x="188" y="120" />
        <omgdi:waypoint x="240" y="120" />
      </bpmndi:BPMNEdge>
      <bpmndi:BPMNEdge id="Flow_2_di" bpmnElement="Flow_2">
        <omgdi:waypoint x="340" y="120" />
        <omgdi:waypoint x="392" y="120" />
      </bpmndi:BPMNEdge>
      <bpmndi:BPMNShape id="StartEvent_1_di" bpmnElement="StartEvent_1">
        <omgdc:Bounds x="152" y="102" width="36" height="36" />
        <bpmndi:BPMNLabel>
          <omgdc:Bounds x="158" y="145" width="24" height="14" />
        </bpmndi:BPMNLabel>
      </bpmndi:BPMNShape>
      <bpmndi:BPMNShape id="UserTask_1_di" bpmnElement="UserTask_1">
        <omgdc:Bounds x="240" y="80" width="100" height="80" />
      </bpmndi:BPMNShape>
      <bpmndi:BPMNShape id="EndEvent_1_di" bpmnElement="EndEvent_1">
        <omgdc:Bounds x="392" y="102" width="36" height="36" />
        <bpmndi:BPMNLabel>
          <omgdc:Bounds x="400" y="145" width="20" height="14" />
        </bpmndi:BPMNLabel>
      </bpmndi:BPMNShape>
    </bpmndi:BPMNPlane>
  </bpmndi:BPMNDiagram>
</definitions>`

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	ctx := context.Background()

	// 1. New Client
	client, err := nativebpm.NewClient("http://localhost:8080")
	if err != nil {
		slog.Error("Failed to initialize NativeBPM SDK client", "error", err)
		return
	}

	// 2. Deploy
	deployResp, err := client.Deploy("userTaskProcess", "User Task Process").
		XML([]byte(userTaskBPMN)).
		Send(ctx)
	if err != nil {
		slog.Error("Deploy failed", "error", err)
		return
	}
	slog.Info("Process deployed", "status", deployResp.Status, "processID", deployResp.ProcessID)

	// 3. Start instance
	pi, err := client.StartProcessInstance("userTaskProcess").
		InstanceID("test-instance-1").
		Variable("applicant", "Alice").
		Send(ctx)
	if err != nil {
		slog.Error("StartProcessInstance failed", "error", err)
		return
	}
	slog.Info("Instance started", "id", pi.ID, "completed", pi.Completed, "waiting", pi.WaitingTokens)

	// 4. Complete task
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

	// 5. Get Logs
	logs, err := client.GetInstanceLogs(ctx, "test-instance-1")
	if err != nil {
		slog.Error("GetInstanceLogs failed", "error", err)
		return
	}
	for _, l := range logs {
		slog.Info("Audit Log Record", "nodeID", l.NodeID, "nodeName", l.NodeName, "action", l.Action)
	}
}
