package main

import (
	"context"
	"log"

	"github.com/google/uuid"
	"github.com/nativebpm/connectors/temporal"
	"github.com/nativebpm/connectors/temporal/examples/saga"
	"go.temporal.io/sdk/client"
)

func main() {
	cfg := temporal.LoadFromEnv()

	c, err := temporal.NewClient(cfg)
	if err != nil {
		log.Fatalf("Failed to create Temporal client: %v", err)
	}
	defer c.Close()

	// 1. Run successful scenario
	successWorkflowID := "saga-success-workflow-" + uuid.New().String()
	successOptions := client.StartWorkflowOptions{
		ID:        successWorkflowID,
		TaskQueue: cfg.TaskQueue,
	}

	log.Printf("1. Starting successful Saga (Trip to Paris) with ID: %s", successWorkflowID)
	successParams := saga.TripReservationParams{
		Amount:      500.0,
		HotelName:   "Grand Plaza Hotel",
		Destination: "Paris",
	}

	runSuccess, err := c.ExecuteWorkflow(context.Background(), successOptions, saga.TripReservationWorkflow, successParams)
	if err != nil {
		log.Fatalf("Failed to start successful Saga: %v", err)
	}

	var successResult string
	err = runSuccess.Get(context.Background(), &successResult)
	if err != nil {
		log.Fatalf("Error executing successful Saga: %v", err)
	}
	log.Printf("1. Result of successful Saga: %s\n\n", successResult)

	// 2. Run failed scenario at the flight booking stage
	failWorkflowID := "saga-fail-workflow-" + uuid.New().String()
	failOptions := client.StartWorkflowOptions{
		ID:        failWorkflowID,
		TaskQueue: cfg.TaskQueue,
	}

	log.Printf("2. Starting failed Saga (Trip to Fail - flight cancelled) with ID: %s", failWorkflowID)
	failParams := saga.TripReservationParams{
		Amount:      300.0,
		HotelName:   "Cozy Hostel",
		Destination: "Fail", // Will trigger an error in BookFlight
	}

	runFail, err := c.ExecuteWorkflow(context.Background(), failOptions, saga.TripReservationWorkflow, failParams)
	if err != nil {
		log.Fatalf("Failed to start failed Saga: %v", err)
	}

	var failResult string
	err = runFail.Get(context.Background(), &failResult)
	if err == nil {
		log.Fatalf("Expected Saga execution error, but process completed successfully with result: %s", failResult)
	}
	log.Printf("2. Saga successfully failed with error: %v (check worker logs to confirm compensations)", err)
}
