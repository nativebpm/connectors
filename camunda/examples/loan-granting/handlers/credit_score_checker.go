package handlers

import (
	"context"
	"log/slog"
	"time"

	"github.com/nativebpm/connectors/camunda"
)

// CreditScoreChecker handles credit score checking tasks
type CreditScoreChecker struct {
	logger *slog.Logger
}

// NewCreditScoreChecker creates a new credit score checker handler
func NewCreditScoreChecker(logger *slog.Logger) *CreditScoreChecker {
	return &CreditScoreChecker{
		logger: logger,
	}
}

// Handle processes a credit score checking task
func (h *CreditScoreChecker) Handle(ctx context.Context, client *camunda.Client, task camunda.ExternalTask) error {
	h.logger.Info("Checking credit scores", "taskID", task.ID, "processInstanceID", task.ProcessInstanceID)

	// Simulate credit score check - in real scenario this would call external services
	time.Sleep(2 * time.Second)

	// Return array of scores
	scores := []int{7, 8, 6}
	h.logger.Info("Credit scores calculated", "scores", scores, "taskID", task.ID)

	// Complete the task with results
	variables := map[string]camunda.Variable{
		"creditScores": camunda.JSONVariable(scores),
	}

	err := client.Complete(task.ID).
		Context(ctx).
		Variables(variables).
		Execute()
	if err != nil {
		return err
	}

	h.logger.Info("Credit score check completed", "taskID", task.ID)
	return nil
}
