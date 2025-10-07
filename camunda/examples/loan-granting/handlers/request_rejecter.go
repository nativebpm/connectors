package handlers

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/nativebpm/connectors/camunda"
)

// RequestRejecter handles loan rejection tasks
type RequestRejecter struct {
	logger *slog.Logger
}

// NewRequestRejecter creates a new request rejecter handler
func NewRequestRejecter(logger *slog.Logger) *RequestRejecter {
	return &RequestRejecter{
		logger: logger,
	}
}

// Handle processes a loan rejection task
func (h *RequestRejecter) Handle(ctx context.Context, client *camunda.Client, task camunda.ExternalTask) error {
	h.logger.Info("Processing loan rejection", "taskID", task.ID, "processInstanceID", task.ProcessInstanceID)

	// Extract credit score from task variables
	scoreVar, ok := task.Variables["score"]
	if !ok {
		return fmt.Errorf("score variable not found in task")
	}

	score, ok := scoreVar.Value.(float64)
	if !ok {
		return fmt.Errorf("score is not a number: %T", scoreVar.Value)
	}

	h.logger.Info("Rejecting loan request", "score", score, "taskID", task.ID)

	// Simulate rejection processing
	time.Sleep(2 * time.Second)

	// Prepare rejection reason
	reason := fmt.Sprintf("Low credit score: %.1f", score)

	// Complete the task with results
	variables := map[string]camunda.Variable{
		"loanRejected": camunda.BooleanVariable(true),
		"reason":       camunda.StringVariable(reason),
	}

	err := client.Complete(task.ID).
		Context(ctx).
		Variables(variables).
		Execute()
	if err != nil {
		return err
	}

	h.logger.Info("Loan request rejected", "taskID", task.ID, "reason", reason)
	return nil
}
