package main

import (
	"context"
	"fmt"
	"log"
	"time"

	restateconn "github.com/nativebpm/connectors/restate"
	restate "github.com/restatedev/sdk-go"
)

// OrderWorkflow defines a durable workflow for order processing.
type OrderWorkflow struct{}

// Run executes the workflow orchestration steps durably.
func (OrderWorkflow) Run(ctx restate.WorkflowContext, orderID string) (string, error) {
	fmt.Printf("Workflow started for Order ID: %s\n", orderID)

	// Step 1: Validate payment (durable side effect)
	paymentStatus, err := restate.Run(ctx, func(ctx restate.RunContext) (string, error) {
		fmt.Printf("[%s] Validating payment...\n", orderID)
		// Simulating payment gateway call
		return "PAID", nil
	})
	if err != nil {
		return "", fmt.Errorf("payment validation failed: %w", err)
	}
	fmt.Printf("[%s] Payment status resolved: %s\n", orderID, paymentStatus)

	// Step 2: Durable Sleep (pauses execution and suspends resources)
	fmt.Printf("[%s] Waiting for warehouse packaging...\n", orderID)
	err = restate.Sleep(ctx, 3*time.Second)
	if err != nil {
		return "", fmt.Errorf("sleep interrupted: %w", err)
	}

	// Step 3: Ship items (durable side effect)
	shipResult, err := restate.Run(ctx, func(ctx restate.RunContext) (string, error) {
		fmt.Printf("[%s] Dispatching package to courier...\n", orderID)
		return "SHIPPED", nil
	})
	if err != nil {
		return "", fmt.Errorf("shipping dispatch failed: %w", err)
	}
	fmt.Printf("[%s] Shipping result resolved: %s\n", orderID, shipResult)

	return "SUCCESS", nil
}

func main() {
	cfg, err := restateconn.NewConfigBuilder().
		FromEnv().
		WithHostPort("0.0.0.0:9080").
		Build()
	if err != nil {
		log.Fatalf("Failed to build config: %v", err)
	}

	fmt.Printf("Starting Order Workflow service on %s...\n", cfg.HostPort)
	
	// Create server, bind workflow, and start
	err = restateconn.NewServer(cfg).
		Bind(OrderWorkflow{}).
		Start(context.Background())
	
	if err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
