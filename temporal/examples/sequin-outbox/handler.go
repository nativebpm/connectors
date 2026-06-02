package sequinoutbox

import (
	"context"
	"encoding/json"
	"log"
	"net/http"

	"go.temporal.io/sdk/client"
)

// WorkflowStarter defines the minimum interface needed to start a workflow.
type WorkflowStarter interface {
	ExecuteWorkflow(ctx context.Context, options client.StartWorkflowOptions, workflow any, args ...any) (client.WorkflowRun, error)
}

// UserRecord represents the structure of user data in Sequin events.
type UserRecord struct {
	ID        string `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Email     string `json:"email"`
}

// SequinWebhookPayload represents the payload sent by Sequin Webhook Sink.
type SequinWebhookPayload struct {
	Action    string      `json:"action"`
	Record    *UserRecord `json:"record"`
	OldRecord *UserRecord `json:"old_record"`
}

// NewWebhookHandler returns a handler that processes Sequin webhooks and starts Temporal workflows.
func NewWebhookHandler(starter WorkflowStarter, taskQueue string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var payload SequinWebhookPayload
		err := json.NewDecoder(r.Body).Decode(&payload)
		if err != nil {
			log.Printf("Error decoding JSON payload: %v", err)
			http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
			return
		}

		// We only trigger deletion workflow for delete actions
		if payload.Action != "delete" {
			log.Printf("Ignoring non-delete action: %s", payload.Action)
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("Action ignored"))
			return
		}

		// On delete, record is null and data is in old_record
		user := payload.OldRecord
		if user == nil {
			log.Printf("Error: delete action received but old_record is nil")
			http.Error(w, "Missing old_record for delete action", http.StatusBadRequest)
			return
		}

		if user.ID == "" || user.Email == "" {
			log.Printf("Error: missing required user fields (id=%s, email=%s)", user.ID, user.Email)
			http.Error(w, "Missing required user fields", http.StatusBadRequest)
			return
		}

		log.Printf("[Webhook] Received delete event for user: %s (%s)", user.ID, user.Email)

		// Start Temporal workflow asynchronously
		workflowID := "delete-user-" + user.ID
		options := client.StartWorkflowOptions{
			ID:        workflowID,
			TaskQueue: taskQueue,
		}

		run, err := starter.ExecuteWorkflow(context.Background(), options, DeleteUserWorkflow, user.ID, user.Email)
		if err != nil {
			log.Printf("Failed to start DeleteUserWorkflow: %v", err)
			http.Error(w, "Failed to start workflow", http.StatusInternalServerError)
			return
		}

		log.Printf("[Webhook] Successfully started DeleteUserWorkflow. ID: %s, RunID: %s", run.GetID(), run.GetRunID())

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Workflow started successfully"))
	}
}
