package handlers

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/nativebpm/connectors/camunda"
)

// LoanGranter handles loan granting tasks
type LoanGranter struct {
	logger *slog.Logger
}

// NewLoanGranter creates a new loan granter handler
func NewLoanGranter(logger *slog.Logger) *LoanGranter {
	return &LoanGranter{
		logger: logger,
	}
}

// Handle processes a loan granting task
func (h *LoanGranter) Handle(ctx context.Context, client *camunda.Client, task camunda.ExternalTask) error {
	h.logger.Info("Processing loan grant", "taskID", task.ID, "processInstanceID", task.ProcessInstanceID)

	// Extract credit score from task variables
	scoreVar, ok := task.Variables["score"]
	if !ok {
		return fmt.Errorf("score variable not found in task")
	}

	score, ok := scoreVar.Value.(float64)
	if !ok {
		return fmt.Errorf("score is not a number: %T", scoreVar.Value)
	}

	h.logger.Info("Granting loan", "score", score, "taskID", task.ID)

	// Simulate loan processing
	time.Sleep(2 * time.Second)

	// Calculate loan amount based on score
	loanAmount := score * 1500.0 // Simple calculation for demo

	// Complete the task with results
	variables := map[string]camunda.Variable{
		"loanGranted": camunda.BooleanVariable(true),
		"loanAmount":  camunda.DoubleVariable(loanAmount),
	}

	err := client.Complete(task.ID).
		Context(ctx).
		Variables(variables).
		Execute()
	if err != nil {
		return err
	}

	h.logger.Info("Loan granted successfully", "taskID", task.ID, "amount", loanAmount)
	return nil
}
