package main

import (
	"context"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/nativebpm/connectors/temporal"
	"github.com/nativebpm/connectors/temporal/examples/signal"
	"go.temporal.io/sdk/client"
)

func main() {
	cfg := temporal.LoadFromEnv()

	c, err := temporal.NewClient(cfg)
	if err != nil {
		log.Fatalf("Failed to create Temporal client: %v", err)
	}
	defer c.Close()

	workflowID := "subscription-workflow-" + uuid.New().String()
	options := client.StartWorkflowOptions{
		ID:        workflowID,
		TaskQueue: cfg.TaskQueue,
	}

	log.Printf("1. Starting SubscriptionWorkflow with ID: %s for 1 minute", workflowID)
	// Start for 1 minute of virtual/real time
	run, err := c.ExecuteWorkflow(context.Background(), options, signal.SubscriptionWorkflow, 1*time.Minute)
	if err != nil {
		log.Fatalf("Error starting Workflow: %v", err)
	}

	// Allow some time for worker to start the process
	time.Sleep(1 * time.Second)

	// 2. Query - Request status
	queryResp, err := c.QueryWorkflow(context.Background(), workflowID, "", "GetSubscriptionStatus")
	if err != nil {
		log.Fatalf("Error sending Query: %v", err)
	}
	var status signal.SubscriptionStatus
	if err := queryResp.Get(&status); err != nil {
		log.Fatalf("Failed to parse status: %v", err)
	}
	log.Printf("2. Query response (Initial status): State=%s, Billing=%s", status.State, status.BillingInfo)

	// 3. Signal - Update billing info
	log.Printf("3. Sending UpdateBillingInfo signal with new billing details...")
	err = c.SignalWorkflow(context.Background(), workflowID, "", "UpdateBillingInfo", "PayPal Account")
	if err != nil {
		log.Fatalf("Error sending UpdateBillingInfo signal: %v", err)
	}

	// Allow worker to process the signal
	time.Sleep(1 * time.Second)

	// 4. Query - Request updated status
	queryResp, err = c.QueryWorkflow(context.Background(), workflowID, "", "GetSubscriptionStatus")
	if err != nil {
		log.Fatalf("Error sending Query: %v", err)
	}
	if err := queryResp.Get(&status); err != nil {
		log.Fatalf("Failed to parse status: %v", err)
	}
	log.Printf("4. Query response (After billing update signal): State=%s, Billing=%s", status.State, status.BillingInfo)

	// 5. Signal - Cancel subscription
	log.Printf("5. Sending CancelSubscription signal...")
	err = c.SignalWorkflow(context.Background(), workflowID, "", "CancelSubscription", nil)
	if err != nil {
		log.Fatalf("Error sending CancelSubscription signal: %v", err)
	}

	// 6. Wait for completion
	var finalResult string
	err = run.Get(context.Background(), &finalResult)
	if err != nil {
		log.Fatalf("Error executing Workflow: %v", err)
	}

	log.Printf("6. Workflow completed successfully with final status: %s", finalResult)
}
