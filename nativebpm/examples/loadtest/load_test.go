package main

import (
	"context"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
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

func TestLoad_Concurrently(t *testing.T) {
	client, err := nativebpm.NewClient("http://localhost:8080")
	if err != nil {
		t.Fatalf("Failed to initialize client: %v", err)
	}

	ctx := context.Background()

	// Deploy the process first
	_, err = client.Deploy("userTaskProcess", "User Task Process").
		XML([]byte(userTaskBPMN)).
		Send(ctx)
	if err != nil {
		t.Fatalf("Deploy failed: %v", err)
	}

	concurrency := 20
	if env := os.Getenv("LOAD_CONCURRENCY"); env != "" {
		if val, err := strconv.Atoi(env); err == nil {
			concurrency = val
		}
	}

	iterations := 5
	if env := os.Getenv("LOAD_ITERATIONS"); env != "" {
		if val, err := strconv.Atoi(env); err == nil {
			iterations = val
		}
	}

	var wg sync.WaitGroup
	start := time.Now()

	for worker := 0; worker < concurrency; worker++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				instanceID := uuid.New().String()

				// 1. Start instance
				pi, err := client.StartProcessInstance("userTaskProcess").
					InstanceID(instanceID).
					Variable("applicant", "Worker").
					Variable("worker_id", workerID).
					Send(ctx)
				if err != nil {
					t.Errorf("StartProcessInstance failed for %s: %v", instanceID, err)
					continue
				}

				// 2. Complete task
				if len(pi.WaitingTokens) > 0 {
					taskID := pi.WaitingTokens[0]
					_, err = client.CompleteTask(pi.ID, taskID).
						Variable("approved", true).
						Send(ctx)
					if err != nil {
						t.Errorf("CompleteTask failed for %s task %s: %v", instanceID, taskID, err)
					}
				}
			}
		}(worker)
	}

	wg.Wait()
	duration := time.Since(start)
	totalRuns := concurrency * iterations
	t.Logf("Completed %d process executions in %v (RPS: %.2f)", totalRuns, duration, float64(totalRuns)/duration.Seconds())
}

func TestComplexLoad_Concurrently(t *testing.T) {
	client, err := nativebpm.NewClient("http://localhost:8080")
	if err != nil {
		t.Fatalf("Failed to initialize client: %v", err)
	}

	ctx := context.Background()

	// Read gateways BPMN
	xmlBytes, err := os.ReadFile("../../../../nativebpm/examples/diagrams/gateways.bpmn")
	if err != nil {
		t.Fatalf("Failed to read gateways.bpmn: %v", err)
	}

	// Deploy the process first
	_, err = client.Deploy("gateways_process", "Complex Gateways Process").
		XML(xmlBytes).
		Send(ctx)
	if err != nil {
		t.Fatalf("Deploy failed: %v", err)
	}

	concurrency := 10
	if env := os.Getenv("LOAD_CONCURRENCY"); env != "" {
		if val, err := strconv.Atoi(env); err == nil {
			concurrency = val
		}
	}

	iterations := 5
	if env := os.Getenv("LOAD_ITERATIONS"); env != "" {
		if val, err := strconv.Atoi(env); err == nil {
			iterations = val
		}
	}

	var wg sync.WaitGroup
	start := time.Now()

	for worker := 0; worker < concurrency; worker++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				instanceID := uuid.New().String()

				// 1. Start instance with score variable
				pi, err := client.StartProcessInstance("gateways_process").
					InstanceID(instanceID).
					Variable("score", 60).
					Send(ctx)
				if err != nil {
					t.Errorf("StartProcessInstance failed for %s: %v", instanceID, err)
					continue
				}

				// 2. Complete task "Activity_User_Approve"
				if len(pi.WaitingTokens) > 0 {
					taskID := pi.WaitingTokens[0]
					if taskID != "Activity_User_Approve" {
						t.Errorf("Expected wait state Activity_User_Approve, got: %s", taskID)
						continue
					}
					_, err = client.CompleteTask(pi.ID, taskID).Send(ctx)
					if err != nil {
						t.Errorf("CompleteTask failed for %s task %s: %v", instanceID, taskID, err)
					}
				} else {
					t.Errorf("Expected waiting tokens for instance %s, got none", instanceID)
				}
			}
		}(worker)
	}

	wg.Wait()
	duration := time.Since(start)
	totalRuns := concurrency * iterations
	t.Logf("Completed %d complex process executions in %v (RPS: %.2f)", totalRuns, duration, float64(totalRuns)/duration.Seconds())
}

func TestBPMNCrashRecovery(t *testing.T) {
	client, err := nativebpm.NewClient("http://localhost:8080")
	if err != nil {
		t.Fatalf("Failed to initialize client: %v", err)
	}

	ctx := context.Background()

	// Read gateways BPMN
	xmlBytes, err := os.ReadFile("../../../../nativebpm/examples/diagrams/gateways.bpmn")
	if err != nil {
		t.Fatalf("Failed to read gateways.bpmn: %v", err)
	}

	// Deploy the process
	_, err = client.Deploy("gateways_process", "Complex Gateways Process").
		XML(xmlBytes).
		Send(ctx)
	if err != nil {
		t.Fatalf("Deploy failed: %v", err)
	}

	instanceID := "recovery-test-" + uuid.New().String()

	// 1. Start process instance. It will halt at the UserTask "Activity_User_Approve" wait state.
	pi, err := client.StartProcessInstance("gateways_process").
		InstanceID(instanceID).
		Variable("score", 60).
		Send(ctx)
	if err != nil {
		t.Fatalf("StartProcessInstance failed: %v", err)
	}

	if len(pi.WaitingTokens) == 0 || pi.WaitingTokens[0] != "Activity_User_Approve" {
		t.Fatalf("Expected process to pause at Activity_User_Approve, got: %v", pi.WaitingTokens)
	}

	// 2. Complete the user task. Because the REST API server is stateless and fully backed by Postgres,
	// this simulates resuming the process from DB state as if the server had rebooted/recovered.
	taskID := pi.WaitingTokens[0]
	completedPi, err := client.CompleteTask(pi.ID, taskID).Send(ctx)
	if err != nil {
		t.Fatalf("CompleteTask failed: %v", err)
	}

	// 3. Verify it advanced to the parallel join state (matching Scenario 4 behavior)
	if completedPi.Completed {
		t.Errorf("Expected process to not be completed yet, got completed state: %+v", completedPi)
	}
	if len(completedPi.ActiveTokens) != 2 || completedPi.ActiveTokens[0] != "Gateway_Join_Parallel" {
		t.Errorf("Expected active tokens to have advanced to parallel join gateway, got: %v", completedPi.ActiveTokens)
	}
}

func TestWasmLoad_Concurrently(t *testing.T) {
	client, err := nativebpm.NewClient("http://localhost:8080")
	if err != nil {
		t.Fatalf("Failed to initialize client: %v", err)
	}

	ctx := context.Background()

	// Deploy the process first
	const wasmTaskBPMN = `<?xml version="1.0" encoding="UTF-8"?>
<definitions xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xmlns="http://www.omg.org/spec/BPMN/20100524/MODEL" xmlns:bpmndi="http://www.omg.org/spec/BPMN/20100524/DI" xmlns:omgdc="http://www.omg.org/spec/DD/20100524/DC" xmlns:omgdi="http://www.omg.org/spec/DD/20100524/DI" id="Definitions_1" targetNamespace="http://bpmn.io/schema/bpmn">
  <process id="wasmTaskProcess" name="WASM Task Process" isExecutable="true">
    <startEvent id="StartEvent_1" name="Start">
      <outgoing>Flow_1</outgoing>
    </startEvent>
    <sequenceFlow id="Flow_1" sourceRef="StartEvent_1" targetRef="WasmTask_1" />
    <serviceTask id="WasmTask_1" name="Execute WASM Worker" wasmPath="/Users/user/github.com/nativebpm/connectors/bpmn/examples/orchestration/worker/worker.wasm">
      <incoming>Flow_1</incoming>
      <outgoing>Flow_2</outgoing>
    </serviceTask>
    <sequenceFlow id="Flow_2" sourceRef="WasmTask_1" targetRef="EndEvent_1" />
    <endEvent id="EndEvent_1" name="End">
      <incoming>Flow_2</incoming>
    </endEvent>
  </process>
</definitions>`

	_, err = client.Deploy("wasmTaskProcess", "WASM Task Process").
		XML([]byte(wasmTaskBPMN)).
		Send(ctx)
	if err != nil {
		t.Fatalf("Deploy failed: %v", err)
	}

	concurrency := 10
	if env := os.Getenv("LOAD_CONCURRENCY"); env != "" {
		if val, err := strconv.Atoi(env); err == nil {
			concurrency = val
		}
	}

	iterations := 5
	if env := os.Getenv("LOAD_ITERATIONS"); env != "" {
		if val, err := strconv.Atoi(env); err == nil {
			iterations = val
		}
	}

	var wg sync.WaitGroup
	start := time.Now()

	for worker := 0; worker < concurrency; worker++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				instanceID := uuid.New().String()

				// Start instance with the variable "approved" set to true, as expected by worker.wasm
				pi, err := client.StartProcessInstance("wasmTaskProcess").
					InstanceID(instanceID).
					Variable("approved", true).
					Send(ctx)
				if err != nil {
					t.Errorf("StartProcessInstance failed for %s: %v", instanceID, err)
					continue
				}

				// The WASM service task is executed synchronously when the process instance starts.
				// Since there are no user tasks, the process instance should complete immediately.
				if !pi.Completed {
					t.Errorf("Expected process instance %s to complete, but it is not completed: %+v", instanceID, pi)
				}
				if pi.Variables["payment_status"] != "success" || pi.Variables["transaction_id"] != "TXN-987654321" {
					t.Errorf("Unexpected output variables for %s: %+v", instanceID, pi.Variables)
				}
			}
		}(worker)
	}

	wg.Wait()
	duration := time.Since(start)
	totalRuns := concurrency * iterations
	t.Logf("Completed %d WASM process executions in %v (RPS: %.2f)", totalRuns, duration, float64(totalRuns)/duration.Seconds())
}

func TestWasmCrashRecovery(t *testing.T) {
	client, err := nativebpm.NewClient("http://localhost:8080")
	if err != nil {
		t.Fatalf("Failed to initialize client: %v", err)
	}

	ctx := context.Background()

	// Deploy the process first
	const wasmTaskBPMN = `<?xml version="1.0" encoding="UTF-8"?>
<definitions xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xmlns="http://www.omg.org/spec/BPMN/20100524/MODEL" xmlns:bpmndi="http://www.omg.org/spec/BPMN/20100524/DI" xmlns:omgdc="http://www.omg.org/spec/DD/20100524/DC" xmlns:omgdi="http://www.omg.org/spec/DD/20100524/DI" id="Definitions_1" targetNamespace="http://bpmn.io/schema/bpmn">
  <process id="wasmTaskProcess" name="WASM Task Process" isExecutable="true">
    <startEvent id="StartEvent_1" name="Start">
      <outgoing>Flow_1</outgoing>
    </startEvent>
    <sequenceFlow id="Flow_1" sourceRef="StartEvent_1" targetRef="WasmTask_1" />
    <serviceTask id="WasmTask_1" name="Execute WASM Worker" wasmPath="/Users/user/github.com/nativebpm/connectors/bpmn/examples/orchestration/worker/worker.wasm">
      <incoming>Flow_1</incoming>
      <outgoing>Flow_2</outgoing>
    </serviceTask>
    <sequenceFlow id="Flow_2" sourceRef="WasmTask_1" targetRef="EndEvent_1" />
    <endEvent id="EndEvent_1" name="End">
      <incoming>Flow_2</incoming>
    </endEvent>
  </process>
</definitions>`

	_, err = client.Deploy("wasmTaskProcess", "WASM Task Process").
		XML([]byte(wasmTaskBPMN)).
		Send(ctx)
	if err != nil {
		t.Fatalf("Deploy failed: %v", err)
	}

	instanceID := uuid.New().String()

	// 1. Start process instance with simulate_crash=true. This causes WASM execution to panic/crash on checkpoint.
	_, err = client.StartProcessInstance("wasmTaskProcess").
		InstanceID(instanceID).
		Variable("approved", true).
		Variable("simulate_crash", true).
		Send(ctx)
	if err == nil {
		t.Fatalf("Expected StartProcessInstance to fail/crash, but it succeeded")
	}
	t.Logf("Process failed as expected: %v", err)

	// Verify that the instance exists in the database and is paused/stalled at WasmTask_1
	inst, err := client.GetInstance(ctx, instanceID)
	if err != nil {
		t.Fatalf("Failed to fetch process instance state: %v", err)
	}
	if inst.Completed {
		t.Fatalf("Expected instance to not be completed, but it is completed")
	}
	foundActiveToken := false
	for _, token := range inst.ActiveTokens {
		if token == "WasmTask_1" {
			foundActiveToken = true
			break
		}
	}
	if !foundActiveToken {
		t.Fatalf("Expected WasmTask_1 to be in ActiveTokens, got: %v", inst.ActiveTokens)
	}

	// 2. Disable simulate_crash and call ResumeProcessInstance to resume the execution.
	pi, err := client.ResumeProcessInstance(instanceID).
		Variable("simulate_crash", false).
		Send(ctx)
	if err != nil {
		t.Fatalf("ResumeProcessInstance failed: %v", err)
	}

	// 3. Verify it completed successfully and variable is updated from WASM state
	if !pi.Completed {
		t.Errorf("Expected resumed process instance to complete, but got completed: false")
	}
	if pi.Variables["payment_status"] != "success" || pi.Variables["transaction_id"] != "TXN-987654321" {
		t.Errorf("Unexpected output variables for resumed instance: %+v", pi.Variables)
	}
}

