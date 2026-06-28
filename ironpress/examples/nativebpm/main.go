package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/nativebpm/connectors/ironpress"
	"gitlab.com/nativebpm/sdk/go"
)

func main() {
	// 1. Initialize the official NativeBPM Go SDK Client using Fluent API
	log.Println("Initializing NativeBPM SDK Client...")
	host := "http://localhost:8080"
	token := "nativebpm-api-auth-token-123"
	
	nbClient, err := nativebpm.NewClient(host, token)
	if err != nil {
		log.Fatalf("Failed to create NativeBPM client: %v", err)
	}

	// 2. Read ironpress WASM bytes
	wasmPath := "/tmp/ironpress_wasm/bin/ironpress.wasm"
	wasmBytes, err := os.ReadFile(wasmPath)
	if err != nil {
		log.Printf("[WARNING] WASM module not found at %s. Please compile it for Pure_WASM_Mode execution.", wasmPath)
	}

	// Initialize ironpress client
	ipClient := ironpress.NewClient(
		ironpress.WithWasm(wasmBytes),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 3. Simulating a workflow task processing cycle:
	// Let's assume we are integrating a microservice that polls/processes steps.
	// We list ACTIVE human tasks awaiting invoice generation:
	log.Println("Fetching active tasks from NativeBPM engine...")
	tasks, err := nbClient.Tasks().List().
		WithStatus("ACTIVE").
		Send(ctx)

	if err != nil {
		log.Printf("[NOTE] NativeBPM engine not reachable (this is normal in local testing). Error: %v", err)
		return
	}

	log.Printf("Found %d active tasks.", len(tasks))

	for _, task := range tasks {
		if task.ActivityId != "generate_invoice" {
			continue
		}

		log.Printf("Processing Task ID: %s for Instance: %s", task.Id, task.InstanceId)

		// Claim the task to this worker to prevent concurrent execution
		_, err = nbClient.Tasks().Claim(task.Id).
			WithAssignee("invoice-pdf-generator-service").
			Send(ctx)
		if err != nil {
			log.Printf("Failed to claim task %s: %v", task.Id, err)
			continue
		}

		// Extract customer invoice variables (mocked variables logic for demo)
		customerName := "John Doe"
		invoiceAmount := 450.00

		// Render the HTML string
		htmlContent := fmt.Sprintf(`
			<html>
			<body>
				<h1>Invoice %s</h1>
				<p>Customer: %s</p>
				<p>Total: $%.2f</p>
			</body>
			</html>
		`, task.Id, customerName, invoiceAmount)

		// Convert HTML to PDF using ironpress Client (in Pure_WASM_Mode)
		pdfBytes, err := ipClient.Convert(ironpress.Pure_WASM_Mode).
			HTML(htmlContent).
			PageSize("letter").
			Do(ctx)

		if err != nil {
			log.Printf("Failed to generate PDF: %v", err)
			continue
		}

		// Base64 encode the generated PDF to store in process context
		pdfBase64 := base64.StdEncoding.EncodeToString(pdfBytes)

		// 4. Complete the task using Fluent API, passing the generated PDF variable back to the process
		log.Printf("Completing task %s inside NativeBPM engine...", task.Id)
		_, err = nbClient.Tasks().Complete(task.Id).
			WithVariable("invoicePdfBase64", pdfBase64).
			WithVariable("generationTime", time.Now().Format(time.RFC3339)).
			Send(ctx)

		if err != nil {
			log.Printf("Failed to complete task %s: %v", task.Id, err)
			continue
		}

		log.Printf("Task %s completed successfully!", task.Id)
	}
}
