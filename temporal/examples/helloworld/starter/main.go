package main

import (
	"context"
	"log"

	"github.com/google/uuid"
	"github.com/nativebpm/connectors/temporal"
	"github.com/nativebpm/connectors/temporal/examples/helloworld"
	"go.temporal.io/sdk/client"
)

func main() {
	// Load configuration
	cfg := temporal.LoadFromEnv()

	// Initialize our client
	c, err := temporal.NewClient(cfg)
	if err != nil {
		log.Fatalf("Failed to create Temporal client: %v", err)
	}
	defer c.Close()

	workflowID := "greeting-workflow-" + uuid.New().String()
	options := client.StartWorkflowOptions{
		ID:        workflowID,
		TaskQueue: cfg.TaskQueue,
	}

	log.Printf("Starting Workflow with ID: %s", workflowID)

	// Start Workflow
	run, err := c.ExecuteWorkflow(context.Background(), options, helloworld.GreetWorkflow, "Temporal")
	if err != nil {
		log.Fatalf("Error starting Workflow: %v", err)
	}

	var result string
	// Wait for execution result
	err = run.Get(context.Background(), &result)
	if err != nil {
		log.Fatalf("Error getting Workflow result: %v", err)
	}

	log.Printf("Workflow completed successfully! Result: %s", result)
}
