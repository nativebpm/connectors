package main

import (
	"log"

	"github.com/nativebpm/connectors/temporal"
	"github.com/nativebpm/connectors/temporal/examples/signal"
)

func main() {
	cfg := temporal.LoadFromEnv()

	client, err := temporal.NewClient(cfg)
	if err != nil {
		log.Fatalf("Failed to create Temporal client: %v", err)
	}
	defer client.Close()

	w := temporal.NewWorker(client, cfg.TaskQueue)

	// Register Workflow
	w.RegisterWorkflow(signal.SubscriptionWorkflow)

	log.Printf("Worker signal successfully started for Task Queue: %s", cfg.TaskQueue)

	err = w.Run(nil)
	if err != nil {
		log.Fatalf("Worker exited with error: %v", err)
	}
}
