package heartbeat

import (
	"context"
	"fmt"
	"time"

	"go.temporal.io/sdk/activity"
)

// HeartbeatProgress stores the current execution state of the task.
type HeartbeatProgress struct {
	CompletedStep int `json:"completed_step"`
}

// HeartbeatActivity executes a long-running task step-by-step, sending Heartbeats.
func HeartbeatActivity(ctx context.Context, totalSteps int) (string, error) {
	info := activity.GetInfo(ctx)

	// Start from step 0
	progress := HeartbeatProgress{
		CompletedStep: 0,
	}

	// Check if there is saved progress from previous attempts (Attempt > 1)
	if activity.HasHeartbeatDetails(ctx) {
		var prevProgress HeartbeatProgress
		if err := activity.GetHeartbeatDetails(ctx, &prevProgress); err == nil {
			progress = prevProgress
			activity.GetLogger(ctx).Info("Found heartbeat details. Resuming progress", "CompletedStep", progress.CompletedStep, "Attempt", info.Attempt)
		} else {
			activity.GetLogger(ctx).Error("Failed to decode heartbeat details", "Error", err)
		}
	}

	// Calculate next step
	startStep := progress.CompletedStep + 1
	activity.GetLogger(ctx).Info("Starting activity processing", "StartStep", startStep, "TotalSteps", totalSteps, "Attempt", info.Attempt)

	for step := startStep; step <= totalSteps; step++ {
		// Check for cancellation before doing work
		select {
		case <-ctx.Done():
			activity.GetLogger(ctx).Warn("Activity context was cancelled", "Attempt", info.Attempt)
			return "", ctx.Err()
		default:
		}

		// Simulate executing step (work takes 1 second)
		activity.GetLogger(ctx).Info("Processing step", "Step", step, "Attempt", info.Attempt)
		time.Sleep(1 * time.Second)

		// On the first attempt (Attempt 1) at step 4, simulate a hang/problem
		// We sleep for 4 seconds, which is longer than the HeartbeatTimeout (2 seconds)
		if info.Attempt == 1 && step == 4 {
			activity.GetLogger(ctx).Warn("[SIMULATION] Freezing worker on Attempt 1 at step 4 (sleeping 4s without heartbeating)...")
			time.Sleep(4 * time.Second)

			// After long sleep, check if server cancelled the context
			select {
			case <-ctx.Done():
				activity.GetLogger(ctx).Error("[SIMULATION] Attempt 1 timed out by server due to missing heartbeat!", "Error", ctx.Err())
				return "", ctx.Err()
			default:
			}
		}

		// Update progress and send Heartbeat
		progress.CompletedStep = step
		activity.RecordHeartbeat(ctx, progress)
		activity.GetLogger(ctx).Info("Heartbeat recorded successfully", "CompletedStep", step, "Attempt", info.Attempt)
	}

	return fmt.Sprintf("All %d steps completed successfully on attempt %d!", totalSteps, info.Attempt), nil
}
