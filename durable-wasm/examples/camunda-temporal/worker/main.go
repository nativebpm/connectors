package main

import (
	"encoding/json"
	"io"
	"unsafe"
)

// Global state variables for business logic
var (
	step          int32 = 0
	transactionID string = "TX-9988-1122"
	amountToBill  float64 = 550.00
	billingStatus string = "pending"
)

// Host function imports
//go:wasmimport env checkpoint
func checkpoint()

//go:wasmimport env stream_data
func stream_data(direction int32, ptr uint32, length uint32) int32

// StreamReader wraps host functions to implement standard io.Reader inside WASM
type StreamReader struct {
	direction int32
}

func (r *StreamReader) Read(p []byte) (n int, err error) {
	if len(p) == 0 {
		return 0, nil
	}
	ptr := uint32(uintptr(unsafe.Pointer(&p[0])))
	bytesRead := stream_data(r.direction, ptr, uint32(len(p)))
	if bytesRead < 0 {
		return 0, io.ErrUnexpectedEOF
	}
	if bytesRead == 0 {
		return 0, io.EOF
	}
	return int(bytesRead), nil
}

// StreamWriter wraps host functions to implement standard io.Writer inside WASM
type StreamWriter struct {
	direction int32
}

func (w *StreamWriter) Write(p []byte) (n int, err error) {
	if len(p) == 0 {
		return 0, nil
	}
	ptr := uint32(uintptr(unsafe.Pointer(&p[0])))
	bytesWritten := stream_data(w.direction, ptr, uint32(len(p)))
	if bytesWritten < 0 {
		return 0, io.ErrClosedPipe
	}
	return int(bytesWritten), nil
}

// Structured request/response bodies for external services
type BillingRequest struct {
	TxID   string  `json:"tx_id"`
	Amount float64 `json:"amount"`
}

type BillingResponse struct {
	Status    string `json:"status"`
	Reference string `json:"reference_code"`
}

type CRMRequest struct {
	TxID   string `json:"tx_id"`
	Status string `json:"status"`
}

// FinalResult represents the structured entity saved to the persistent database
type FinalResult struct {
	TxID      string  `json:"tx_id"`
	Amount    float64 `json:"amount"`
	Status    string  `json:"status"`
	RefCode   string  `json:"reference_code"`
	Completed bool    `json:"completed"`
}

//export run
func run() int32 {
	for {
		switch step {
		case 0:
			println("[CAMUNDA-TEMPORAL WORKER] Step 0: Starting orchestration activity...")
			println("[CAMUNDA-TEMPORAL WORKER] Transaction initialized:", transactionID)
			step = 1
			println("[CAMUNDA-TEMPORAL WORKER] Step 0 completed. Saving state to checkpoint.")
			checkpoint()

		case 1:
			println("[CAMUNDA-TEMPORAL WORKER] Step 1: Performing billing charge via Temporal Activity...")
			
			writer := &StreamWriter{direction: 1}
			reader := &StreamReader{direction: 0}

			// Encode billing request
			req := BillingRequest{TxID: transactionID, Amount: amountToBill}
			err := json.NewEncoder(writer).Encode(req)
			if err != nil {
				println("[CAMUNDA-TEMPORAL WORKER] Billing request encode failed:", err.Error())
				return -1
			}

			// Flush/Signal EOF on upload
			var dummy [1]byte
			stream_data(1, uint32(uintptr(unsafe.Pointer(&dummy[0]))), 0)

			// Read response from Billing Service
			var resp BillingResponse
			err = json.NewDecoder(reader).Decode(&resp)
			if err != nil {
				println("[CAMUNDA-TEMPORAL WORKER] Billing response decode failed:", err.Error())
				return -1
			}

			billingStatus = resp.Status
			println("[CAMUNDA-TEMPORAL WORKER] Billing reference received:", resp.Reference)

			if billingStatus != "success" {
				println("[CAMUNDA-TEMPORAL WORKER] Billing charge failed, aborting process.")
				return -2
			}

			step = 2
			println("[CAMUNDA-TEMPORAL WORKER] Step 1 completed. Saving state to checkpoint.")
			checkpoint()

		case 2:
			println("[CAMUNDA-TEMPORAL WORKER] Step 2: Updating CRM / Database status (Camunda Task)...")

			writer := &StreamWriter{direction: 1}
			
			// Send CRM update request
			req := CRMRequest{TxID: transactionID, Status: "completed"}
			err := json.NewEncoder(writer).Encode(req)
			if err != nil {
				println("[CAMUNDA-TEMPORAL WORKER] CRM request encode failed:", err.Error())
				return -1
			}

			// Flush upload
			var dummy [1]byte
			stream_data(1, uint32(uintptr(unsafe.Pointer(&dummy[0]))), 0)
			
			println("[CAMUNDA-TEMPORAL WORKER] CRM updated successfully.")

			step = 3
			println("[CAMUNDA-TEMPORAL WORKER] Step 2 completed. Saving checkpoint.")
			checkpoint()

		case 3:
			println("[CAMUNDA-TEMPORAL WORKER] Step 3: Saving final structured record to persistent Database...")

			writer := &StreamWriter{direction: 1}

			// Construct the final database record
			result := FinalResult{
				TxID:      transactionID,
				Amount:    amountToBill,
				Status:    "success",
				RefCode:   "REF-BILL-550-OK",
				Completed: true,
			}

			// Stream JSON payload directly to the host's DB endpoint
			err := json.NewEncoder(writer).Encode(result)
			if err != nil {
				println("[CAMUNDA-TEMPORAL WORKER] DB record encode failed:", err.Error())
				return -1
			}

			// Flush upload
			var dummy [1]byte
			stream_data(1, uint32(uintptr(unsafe.Pointer(&dummy[0]))), 0)

			println("[CAMUNDA-TEMPORAL WORKER] DB record saved successfully.")

			step = 4
			println("[CAMUNDA-TEMPORAL WORKER] Step 3 completed. Saving final checkpoint.")
			checkpoint()

		case 4:
			println("[CAMUNDA-TEMPORAL WORKER] Step 4: Completing Camunda BPM process.")
			println("[CAMUNDA-TEMPORAL WORKER] Orchestration completed successfully.")
			return 1
		}
	}
}

func main() {}
