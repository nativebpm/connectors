package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"

	"github.com/google/uuid"
	"github.com/nativebpm/connectors/temporal"
	"github.com/nativebpm/connectors/temporal/examples/helloworld"
	"go.temporal.io/sdk/client"
)

func main() {
	log.Println("Initializing Temporal Load Test HTTP Server...")

	cfg := temporal.LoadFromEnv()
	// Set default host/port if not present
	if cfg.HostPort == "" {
		cfg.HostPort = "127.0.0.1:7233"
	}
	if cfg.TaskQueue == "" {
		cfg.TaskQueue = "default-task-queue"
	}

	// Initialize client
	c, err := temporal.NewClient(cfg)
	if err != nil {
		log.Fatalf("Failed to create Temporal client: %v", err)
	}
	defer c.Close()

	// Initialize and start worker in background
	w := temporal.NewWorker(c, cfg.TaskQueue)
	w.RegisterWorkflow(helloworld.GreetWorkflow)
	w.RegisterActivity(helloworld.GreetActivity)

	err = w.Start()
	if err != nil {
		log.Fatalf("Failed to start worker: %v", err)
	}
	defer w.Stop()

	log.Printf("Temporal worker started on queue: %s", cfg.TaskQueue)

	// HTTP handlers
	http.HandleFunc("/start", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		workflowID := "loadtest-workflow-" + uuid.New().String()
		options := client.StartWorkflowOptions{
			ID:        workflowID,
			TaskQueue: cfg.TaskQueue,
		}

		// Execute workflow (E2E with waiting)
		run, err := c.ExecuteWorkflow(context.Background(), options, helloworld.GreetWorkflow, "LoadTest")
		if err != nil {
			log.Printf("Error starting workflow: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		var result string
		err = run.Get(context.Background(), &result)
		if err != nil {
			log.Printf("Error executing workflow: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"status": "completed",
			"result": result,
			"id":     workflowID,
		})
	})

	http.HandleFunc("/ingest", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		workflowID := "loadtest-workflow-" + uuid.New().String()
		options := client.StartWorkflowOptions{
			ID:        workflowID,
			TaskQueue: cfg.TaskQueue,
		}

		// Execute workflow asynchronously
		_, err := c.ExecuteWorkflow(context.Background(), options, helloworld.GreetWorkflow, "LoadTest")
		if err != nil {
			log.Printf("Error starting workflow: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"status": "accepted",
			"id":     workflowID,
		})
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8085"
	}

	log.Printf("Temporal Load Test HTTP Server listening on port %s...", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("HTTP server failed: %v", err)
	}
}
