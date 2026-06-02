package camunda

import (
	"context"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const integrationBPMN = `<?xml version="1.0" encoding="UTF-8"?>
<bpmn:definitions xmlns:bpmn="http://www.omg.org/spec/BPMN/20100524/MODEL" xmlns:bpmndi="http://www.omg.org/spec/BPMN/20100524/DI" xmlns:dc="http://www.omg.org/spec/DD/20100524/DC" xmlns:camunda="http://camunda.org/schema/1.0/bpmn" xmlns:di="http://www.omg.org/spec/DD/20100524/DI" id="Definitions_1" targetNamespace="http://bpmn.io/schema/bpmn" exporter="Camunda Modeler" exporterVersion="5.0.0">
  <bpmn:process id="IntegrationTestProcess" name="Integration Test Process" isExecutable="true" camunda:historyTimeToLive="180">
    <bpmn:startEvent id="StartEvent_1" name="Start">
      <bpmn:outgoing>Flow_1</bpmn:outgoing>
    </bpmn:startEvent>
    <bpmn:sequenceFlow id="Flow_1" sourceRef="StartEvent_1" targetRef="Activity_WasmTask" />
    <bpmn:serviceTask id="Activity_WasmTask" name="Integration Task" camunda:type="external" camunda:topic="integration-test-topic">
      <bpmn:incoming>Flow_1</bpmn:incoming>
      <bpmn:outgoing>Flow_2</bpmn:outgoing>
    </bpmn:serviceTask>
    <bpmn:sequenceFlow id="Flow_2" sourceRef="Activity_WasmTask" targetRef="EndEvent_1" />
    <bpmn:endEvent id="EndEvent_1" name="End">
      <bpmn:incoming>Flow_2</bpmn:incoming>
    </bpmn:endEvent>
  </bpmn:process>
</bpmn:definitions>`

func TestCamundaIntegration_RealServer(t *testing.T) {
	// 1. Check if local Camunda Server is running
	clientHTTP := &http.Client{Timeout: 500 * time.Millisecond}
	resp, err := clientHTTP.Get("http://localhost:8080/engine-rest/version")
	if err != nil {
		t.Skip("Skipping integration test: Camunda Server is not running on http://localhost:8080/engine-rest")
		return
	}
	resp.Body.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 2. Initialize Client
	client, err := NewClient("http://localhost:8080", "integration-test-worker")
	require.NoError(t, err)

	// 3. Deploy Process definition to Camunda
	deployID, err := client.DeployProcess(ctx, "integration-deployment", strings.NewReader(integrationBPMN), "integration.bpmn")
	require.NoError(t, err)
	defer func() {
		// Cleanup deployment
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cleanupCancel()
		_ = client.DeleteDeployment(cleanupCtx, deployID, true)
	}()

	// 4. Start Process Instance
	businessKey := "biz-integration-" + time.Now().Format("20060102-150405")
	processInstanceID, err := client.StartProcessInstance(ctx, "IntegrationTestProcess", businessKey, nil)
	require.NoError(t, err)
	assert.NotEmpty(t, processInstanceID)

	// 5. Setup Worker to poll and complete the external task
	w := NewWorker(client, nil)
	w.SetMaxTasks(1)
	w.SetPollInterval(100 * time.Millisecond)

	taskProcessedChan := make(chan bool, 1)

	w.RegisterHandler("integration-test-topic", TaskHandlerFunc(func(ctx context.Context, c *Client, task ExternalTask, complete CompleteFunc, fail FailFunc) error {
		if task.BusinessKey == businessKey {
			// Complete task in Camunda
			errComplete := complete().Execute()
			if errComplete != nil {
				return errComplete
			}
			taskProcessedChan <- true
		}
		return nil
	}), 60000, []string{})

	// Start worker in background
	go w.Start(ctx)

	// 6. Wait for worker to poll and successfully complete the task
	select {
	case processed := <-taskProcessedChan:
		assert.True(t, processed, "Task must be processed successfully by the worker")
	case <-time.After(10 * time.Second):
		t.Fatal("Timeout waiting for Camunda worker to process task")
	}
}
