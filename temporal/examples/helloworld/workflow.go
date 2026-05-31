package helloworld

import (
	"time"

	"go.temporal.io/sdk/workflow"
)

// GreetWorkflow координирует выполнение приветственного процесса.
func GreetWorkflow(ctx workflow.Context, name string) (string, error) {
	options := workflow.ActivityOptions{
		StartToCloseTimeout: 5 * time.Second,
	}
	ctx = workflow.WithActivityOptions(ctx, options)

	var result string
	err := workflow.ExecuteActivity(ctx, GreetActivity, name).Get(ctx, &result)
	return result, err
}
