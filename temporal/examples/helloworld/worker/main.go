package main

import (
	"log"

	"github.com/nativebpm/connectors/temporal"
	"github.com/nativebpm/connectors/temporal/examples/helloworld"
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

	// Register Workflow and Activity
	w.RegisterWorkflow(helloworld.GreetWorkflow)
	w.RegisterActivity(helloworld.GreetActivity)

	log.Printf("Worker helloworld started successfully for Task Queue: %s", cfg.TaskQueue)

	// Run worker in blocking mode until interrupted
	err = w.Run(nil)
	if err != nil {
		log.Fatalf("Worker exited with error: %v", err)
	}
}
