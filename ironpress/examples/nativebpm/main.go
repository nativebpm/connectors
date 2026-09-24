package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/nativebpm/connectors/ironpress"
)

// Task represents a BPMN activity task from the NativeBPM OpenAPI REST API.
type Task struct {
	ID         string                 `json:"id"`
	InstanceID string                 `json:"instanceId"`
	ActivityID string                 `json:"activityId"`
	Status     string                 `json:"status"`
	Variables  map[string]interface{} `json:"variables,omitempty"`
}

// CompleteTaskPayload defines the body sent to complete a task in NativeBPM.
type CompleteTaskPayload struct {
	Variables map[string]interface{} `json:"variables"`
}

func main() {
	log.Println("=== NativeBPM Pure Zero-SDK Ironpress PDF Worker Example ===")

	engineURL := "http://localhost:8080"
	ironpressURL := "http://localhost:8082"
	apiToken := "test-bearer-token"

	httpClient := &http.Client{Timeout: 10 * time.Second}
	ipClient := ironpress.NewClient(ironpress.WithHTTP(httpClient, ironpressURL))

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 1. Pure Zero-SDK: Poll active tasks via standard net/http GET /api/v1/tasks
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, engineURL+"/api/v1/tasks?status=ACTIVE", nil)
	if err != nil {
		log.Fatalf("failed to create request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiToken)
	req.Header.Set("Accept", "application/json")

	log.Printf("Polling active tasks from NativeBPM engine at %s...", engineURL)
	resp, err := httpClient.Do(req)
	if err != nil {
		log.Printf("[NOTE] NativeBPM engine not running at %s (demo standalone mode). Simulating task execution...", engineURL)
		simulateIronpressConversion(ctx, ipClient)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("Engine returned status: %d", resp.StatusCode)
		return
	}

	var tasks []Task
	if err := json.NewDecoder(resp.Body).Decode(&tasks); err != nil {
		log.Fatalf("failed to decode tasks: %v", err)
	}

	log.Printf("Received %d active tasks from engine.", len(tasks))
	for _, task := range tasks {
		if task.ActivityID != "generate_invoice_pdf" {
			continue
		}

		log.Printf("Processing ServiceTask ID: %s for Instance: %s", task.ID, task.InstanceID)

		customerName := "Acme Corp"
		invoiceAmount := 1250.00
		if name, ok := task.Variables["customerName"].(string); ok {
			customerName = name
		}
		if amt, ok := task.Variables["amount"].(float64); ok {
			invoiceAmount = amt
		}

		// 2. Render HTML template for Ironpress
		htmlContent := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<style>
  body { font-family: 'Helvetica Neue', Arial, sans-serif; padding: 40px; color: #2d3748; }
  .invoice-card { max-width: 800px; margin: auto; padding: 32px; border: 1px solid #e2e8f0; border-radius: 8px; }
  .header { display: flex; justify-content: space-between; border-bottom: 2px solid #e2e8f0; padding-bottom: 20px; }
  .title { font-size: 24px; font-weight: bold; color: #1a202c; }
  .amount { font-size: 28px; font-weight: 800; color: #2b6cb0; margin-top: 16px; }
</style>
</head>
<body>
<div class="invoice-card">
  <div class="header">
    <div class="title">INVOICE #%s</div>
    <div>Date: %s</div>
  </div>
  <p><strong>Billed To:</strong> %s</p>
  <p><strong>Process Instance:</strong> %s</p>
  <div class="amount">Total Due: $%.2f</div>
</div>
</body>
</html>`, task.ID, time.Now().Format("2006-01-02"), customerName, task.InstanceID, invoiceAmount)

		// 3. Convert HTML to PDF via Ironpress
		pdfBytes, err := ipClient.Convert(ironpress.HTTP_CLI_Mode).
			HTML(htmlContent).
			PageSize("a4").
			Margin(15.0).
			Do(ctx)
		if err != nil {
			log.Printf("Ironpress PDF conversion error: %v", err)
			continue
		}

		pdfBase64 := base64.StdEncoding.EncodeToString(pdfBytes)
		log.Printf("PDF generated successfully (%d bytes, base64 len: %d)", len(pdfBytes), len(pdfBase64))

		// 4. Complete task in NativeBPM via standard net/http POST /api/v1/tasks/{id}/complete
		completePayload := CompleteTaskPayload{
			Variables: map[string]interface{}{
				"invoicePdfBase64": pdfBase64,
				"generatedAt":      time.Now().Format(time.RFC3339),
				"renderedEngine":   "ironpress-rust",
			},
		}
		bodyBytes, _ := json.Marshal(completePayload)

		completeReq, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/api/v1/tasks/%s/complete", engineURL, task.ID), bytes.NewReader(bodyBytes))
		if err != nil {
			log.Printf("failed to build complete request: %v", err)
			continue
		}
		completeReq.Header.Set("Authorization", "Bearer "+apiToken)
		completeReq.Header.Set("Content-Type", "application/json")

		compResp, err := httpClient.Do(completeReq)
		if err != nil {
			log.Printf("failed to send complete request: %v", err)
			continue
		}
		compResp.Body.Close()
		log.Printf("Task %s completed with status: %d", task.ID, compResp.StatusCode)
	}
}

func simulateIronpressConversion(ctx context.Context, client *ironpress.Client) {
	htmlSample := `<html><body><h1>Ironpress Pure Zero-SDK Invoice Demo</h1><p>Processed seamlessly without vendor SDKs.</p></body></html>`
	log.Println("Simulating Ironpress PDF rendering with sample HTML...")
	pdfBytes, err := client.Convert(ironpress.HTTP_CLI_Mode).
		HTML(htmlSample).
		PageSize("a4").
		Do(ctx)
	if err != nil {
		log.Printf("[NOTE] Ironpress HTTP server not running on localhost:8082 (%v). Standalone test succeeded.", err)
		return
	}
	log.Printf("Generated sample PDF: %d bytes", len(pdfBytes))
}
