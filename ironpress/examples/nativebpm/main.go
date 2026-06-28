package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"log/slog"
	"os"
	"time"

	"github.com/nativebpm/connectors/camunda"
	"github.com/nativebpm/connectors/ironpress"
)

// InvoiceTaskHandler handles BPMN task "Generate Invoice PDF"
type InvoiceTaskHandler struct {
	ironpressClient *ironpress.Client
}

// Handle processes the external task fetched from NativeBPM (Camunda Engine)
func (h *InvoiceTaskHandler) Handle(
	ctx context.Context,
	camundaClient *camunda.Client,
	task camunda.ExternalTask,
	complete camunda.CompleteFunc,
	fail camunda.FailFunc,
) error {
	slog.Info("Starting invoice PDF generation task", "taskID", task.ID)

	// 1. Extract process variables passed from the BPMN engine
	invoiceIDVar, ok := task.Variables["invoiceId"]
	if !ok {
		return fmt.Errorf("missing required variable 'invoiceId'")
	}
	invoiceID := invoiceIDVar.Value.(string)

	customerNameVar, ok := task.Variables["customerName"]
	if !ok {
		return fmt.Errorf("missing required variable 'customerName'")
	}
	customerName := customerNameVar.Value.(string)

	amountVar, ok := task.Variables["amount"]
	if !ok {
		return fmt.Errorf("missing required variable 'amount'")
	}
	amount := amountVar.Value.(float64)

	// 2. Generate HTML Invoice template dynamically using variables
	htmlTemplate := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
<style>
  body { font-family: sans-serif; padding: 30px; color: #333; }
  .invoice-box { max-width: 800px; margin: auto; padding: 30px; border: 1px solid #eee; box-shadow: 0 0 10px rgba(0, 0, 0, .15); }
  .title { color: #d32f2f; font-size: 28px; font-weight: bold; margin-bottom: 20px; }
  .details { margin-bottom: 40px; line-height: 1.6; }
  .total { margin-top: 40px; font-size: 20px; font-weight: bold; text-align: right; color: #1b5e20; }
</style>
</head>
<body>
  <div class="invoice-box">
    <div class="title">INVOICE #%s</div>
    <div class="details">
      <strong>Customer Name:</strong> %s<br>
      <strong>Date:</strong> %s<br>
      <strong>Status:</strong> PAID
    </div>
    <hr>
    <div class="total">
      Total Amount: $%.2f
    </div>
  </div>
</body>
</html>
`, invoiceID, customerName, time.Now().Format("2006-01-02"), amount)

	// 3. Generate PDF using ironpress Client (in Pure_WASM_Mode)
	slog.Info("Executing in-memory PDF generation via ironpress WASM...", "invoiceID", invoiceID)
	start := time.Now()

	pdfBytes, err := h.ironpressClient.Convert(ironpress.Pure_WASM_Mode).
		HTML(htmlTemplate).
		PageSize("a4").
		Landscape(false).
		Margin(10).
		Do(ctx)

	if err != nil {
		slog.Error("PDF conversion failed", "error", err)
		return fmt.Errorf("failed to convert HTML to PDF: %w", err)
	}

	slog.Info("PDF generated successfully", "duration", time.Since(start), "size", len(pdfBytes))

	// 4. Save PDF to local storage / S3 or return to BPMN context as Base64 variable
	pdfBase64 := base64.StdEncoding.EncodeToString(pdfBytes)

	// Complete task and pass the PDF result back to the BPMN process variables
	err = complete().
		StringVariable("invoicePdfBase64", pdfBase64).
		Execute()

	if err != nil {
		return fmt.Errorf("failed to complete BPMN task: %w", err)
	}

	slog.Info("BPMN Task completed successfully", "taskID", task.ID)
	return nil
}

func main() {
	// Initialize NativeBPM engine client
	host := "http://localhost:8080"
	workerID := "invoice-pdf-worker"
	cClient, err := camunda.NewClient(host, workerID)
	if err != nil {
		log.Fatalf("Failed to initialize Camunda client: %v", err)
	}

	// Read ironpress WASM bytes
	wasmPath := "/tmp/ironpress_wasm/bin/ironpress.wasm"
	wasmBytes, err := os.ReadFile(wasmPath)
	if err != nil {
		log.Printf("[WARNING] WASM module not found at %s. Please compile it for Pure_WASM_Mode execution.", wasmPath)
	}

	// Initialize ironpress client
	ipClient := ironpress.NewClient(
		ironpress.WithWasm(wasmBytes),
		ironpress.WithHTTP(nil, "http://localhost:8080"), // fallback HTTP options
	)

	// Create BPMN Task Worker
	worker := camunda.NewWorker(cClient, slog.Default())

	// Register Invoice Generator task handler on topic "generate_invoice_pdf"
	handler := &InvoiceTaskHandler{ironpressClient: ipClient}
	worker.RegisterHandler("generate_invoice_pdf", handler, 30000, []string{"invoiceId", "customerName", "amount"})

	log.Println("NativeBPM Worker registered for topic 'generate_invoice_pdf'. Starting polling...")
	
	// Start polling loop (non-blocking for demo purposes)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// In a real application worker.Start(ctx) blocks. We run it in a goroutine for demonstration.
	go worker.Start(ctx)

	// Keep alive briefly for demo
	time.Sleep(1 * time.Second)
	log.Println("Worker running. Exiting demo...")
}
