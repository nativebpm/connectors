package heartbeat

import (
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// HeartbeatWorkflow coordinates process execution using Heartbeats.
func HeartbeatWorkflow(ctx workflow.Context, totalSteps int) (string, error) {
	options := workflow.ActivityOptions{
		// Overall Activity execution timeout
		StartToCloseTimeout: 1 * time.Minute,
		// Maximum time between Heartbeats. If the worker does not send
		// a heartbeat within this time, the server considers the task hung.
		HeartbeatTimeout: 2 * time.Second,
		// Configure retry policy
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    1 * time.Second,
			BackoffCoefficient: 1.0,
			MaximumAttempts:    3,
		},
	}
	ctx = workflow.WithActivityOptions(ctx, options)

	var result string
	err := workflow.ExecuteActivity(ctx, HeartbeatActivity, totalSteps).Get(ctx, &result)
	return result, err
}
