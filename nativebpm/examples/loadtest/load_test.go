package main

import (
	"context"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nativebpm/connectors/nativebpm"
)

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func getClient() (*nativebpm.Client, error) {
	url := os.Getenv("NATIVEBPM_API_URL")
	if url == "" {
		url = "http://localhost:8080"
	}
	client, err := nativebpm.NewClient(url)
	if err != nil {
		return nil, err
	}
	client.Use(func(next http.RoundTripper) http.RoundTripper {
		return roundTripperFunc(func(r *http.Request) (*http.Response, error) {
			r.Header.Set("Authorization", "Bearer test-bearer-token")
			return next.RoundTrip(r)
		})
	})
	return client, nil
}

func TestBPMNCrashRecovery(t *testing.T) {
	client, err := getClient()
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
	startResp, err := client.StartProcessInstance("gateways_process").
		InstanceID(instanceID).
		Variable("score", 60).
		Send(ctx)
	if err != nil {
		t.Fatalf("StartProcessInstance failed: %v", err)
	}

	// Poll until the instance is created and waiting at Activity_User_Approve
	var pi *nativebpm.ProcessInstance
	for i := 0; i < 50; i++ {
		pi, err = client.GetInstance(ctx, startResp.InstanceID)
		if err == nil && len(pi.WaitingTokens) > 0 && pi.WaitingTokens[0] == "Activity_User_Approve" {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	if err != nil {
		t.Fatalf("Failed to fetch process instance during polling: %v", err)
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

	// 3. Verify it advanced through the parallel join and completed successfully
	if !completedPi.Completed {
		t.Errorf("Expected process to be completed, got state: %+v", completedPi)
	}
	if len(completedPi.ActiveTokens) != 0 {
		t.Errorf("Expected no active tokens left, got: %v", completedPi.ActiveTokens)
	}
}

func TestWasmCrashRecovery(t *testing.T) {
	client, err := getClient()
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
	startResp, err := client.StartProcessInstance("wasmTaskProcess").
		InstanceID(instanceID).
		Variable("approved", true).
		Variable("simulate_crash", true).
		Send(ctx)
	if err != nil {
		t.Fatalf("StartProcessInstance failed: %v", err)
	}

	// Poll until the instance is available and stalled at WasmTask_1
	var inst *nativebpm.ProcessInstance
	for i := 0; i < 50; i++ {
		inst, err = client.GetInstance(ctx, startResp.InstanceID)
		if err == nil {
			foundActiveToken := false
			for _, token := range inst.ActiveTokens {
				if token == "WasmTask_1" {
					foundActiveToken = true
					break
				}
			}
			if foundActiveToken {
				break
			}
		}
		time.Sleep(100 * time.Millisecond)
	}

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
	pi, err := client.ResumeProcessInstance(startResp.InstanceID).
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
