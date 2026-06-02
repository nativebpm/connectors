package main

import (
	"log"
	"net/http"
	"os"

	"github.com/nativebpm/connectors/temporal"
	sequinoutbox "github.com/nativebpm/connectors/temporal/examples/sequin-outbox"
)

func main() {
	// Load configuration
	cfg := temporal.LoadFromEnv()

	// Initialize Temporal client
	c, err := temporal.NewClient(cfg)
	if err != nil {
		log.Fatalf("Failed to create Temporal client: %v", err)
	}
	defer c.Close()

	// Setup webhook HTTP handler from the sequinoutbox package
	http.HandleFunc("/delete-user", sequinoutbox.NewWebhookHandler(c, cfg.TaskQueue))

	port := os.Getenv("PORT")
	if port == "" {
		port = "3333"
	}

	log.Printf("Sequin Webhook Server started successfully on port %s", port)
	log.Printf("Listening for POST /delete-user and sending tasks to queue: %s", cfg.TaskQueue)

	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("Failed to start HTTP server: %v", err)
	}
}
