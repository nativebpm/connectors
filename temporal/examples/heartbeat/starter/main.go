package main

import (
	"context"
	"log"

	"github.com/google/uuid"
	"github.com/nativebpm/connectors/temporal"
	"github.com/nativebpm/connectors/temporal/examples/heartbeat"
	"go.temporal.io/sdk/client"
)

func main() {
	// Load configuration from environment
	cfg := temporal.LoadFromEnv()

	// Initialize Temporal client
	c, err := temporal.NewClient(cfg)
	if err != nil {
		log.Fatalf("Failed to create Temporal client: %v", err)
	}
	defer c.Close()

	workflowID := "heartbeat-workflow-" + uuid.New().String()
	options := client.StartWorkflowOptions{
		ID:        workflowID,
		TaskQueue: cfg.TaskQueue,
	}

	log.Printf("Starting Workflow with ID: %s", workflowID)

	// Start Workflow with total steps = 10
	run, err := c.ExecuteWorkflow(context.Background(), options, heartbeat.HeartbeatWorkflow, 10)
	if err != nil {
		log.Fatalf("Failed to start Workflow: %v", err)
	}

	var result string
	// Wait for execution result
	err = run.Get(context.Background(), &result)
	if err != nil {
		log.Fatalf("Failed to get Workflow result: %v", err)
	}

	log.Printf("Workflow completed successfully! Result: %s", result)
}
