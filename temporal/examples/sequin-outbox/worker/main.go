package main

import (
	"log"

	"github.com/nativebpm/connectors/temporal"
	sequinoutbox "github.com/nativebpm/connectors/temporal/examples/sequin-outbox"
)

func main() {
	// Load configuration
	cfg := temporal.LoadFromEnv()

	// Initialize our client
	client, err := temporal.NewClient(cfg)
	if err != nil {
		log.Fatalf("Failed to create Temporal client: %v", err)
	}
	defer client.Close()

	// Initialize worker
	w := temporal.NewWorker(client, cfg.TaskQueue)

	// Register Workflow and Activities
	w.RegisterWorkflow(sequinoutbox.DeleteUserWorkflow)
	w.RegisterActivity(sequinoutbox.CleanUpExternalSystems)
	w.RegisterActivity(sequinoutbox.SendDeletionConfirmation)

	log.Printf("Worker sequin-outbox started successfully for Task Queue: %s", cfg.TaskQueue)

	// Run worker in blocking mode until interrupted
	err = w.Run(nil)
	if err != nil {
		log.Fatalf("Worker exited with error: %v", err)
	}
}
