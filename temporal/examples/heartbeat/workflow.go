package heartbeat

import (
	"time"

	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// HeartbeatWorkflow координирует выполнение процесса с использованием Heartbeats.
func HeartbeatWorkflow(ctx workflow.Context, totalSteps int) (string, error) {
	options := workflow.ActivityOptions{
		// Общий таймаут выполнения Activity
		StartToCloseTimeout: 1 * time.Minute,
		// Максимальное время между Heartbeats. Если воркер не пришлет
		// heartbeat в течение этого времени, сервер считает задачу зависшей.
		HeartbeatTimeout: 2 * time.Second,
		// Настройка политики повторных попыток
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
