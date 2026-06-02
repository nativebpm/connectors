package sequinoutbox

import (
	"time"

	"go.temporal.io/sdk/workflow"
)

// DeleteUserWorkflow coordinates the deletion of a user across systems.
func DeleteUserWorkflow(ctx workflow.Context, userID string, email string) error {
	ao := workflow.ActivityOptions{
		StartToCloseTimeout: 10 * time.Second,
	}
	ctx = workflow.WithActivityOptions(ctx, ao)

	// Step 1: Clean up external systems (e.g. S3, Stripe)
	err := workflow.ExecuteActivity(ctx, CleanUpExternalSystems, userID).Get(ctx, nil)
	if err != nil {
		return err
	}

	// Step 2: Send deletion confirmation email
	err = workflow.ExecuteActivity(ctx, SendDeletionConfirmation, email).Get(ctx, nil)
	if err != nil {
		return err
	}

	return nil
}
