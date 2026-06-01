package heartbeat

import (
	"context"
	"fmt"
	"time"

	"go.temporal.io/sdk/activity"
)

// HeartbeatProgress хранит текущее состояние выполнения задачи.
type HeartbeatProgress struct {
	CompletedStep int `json:"completed_step"`
}

// HeartbeatActivity выполняет длительную задачу пошагово, отправляя Heartbeats.
func HeartbeatActivity(ctx context.Context, totalSteps int) (string, error) {
	info := activity.GetInfo(ctx)
	
	// Начинаем с 0-го шага
	progress := HeartbeatProgress{
		CompletedStep: 0,
	}

	// Проверяем, есть ли сохраненный прогресс от предыдущих попыток (Attempt > 1)
	if activity.HasHeartbeatDetails(ctx) {
		var prevProgress HeartbeatProgress
		if err := activity.GetHeartbeatDetails(ctx, &prevProgress); err == nil {
			progress = prevProgress
			activity.GetLogger(ctx).Info("Found heartbeat details. Resuming progress", "CompletedStep", progress.CompletedStep, "Attempt", info.Attempt)
		} else {
			activity.GetLogger(ctx).Error("Failed to decode heartbeat details", "Error", err)
		}
	}

	// Вычисляем следующий шаг
	startStep := progress.CompletedStep + 1
	activity.GetLogger(ctx).Info("Starting activity processing", "StartStep", startStep, "TotalSteps", totalSteps, "Attempt", info.Attempt)

	for step := startStep; step <= totalSteps; step++ {
		// Проверяем отмену перед выполнением работы
		select {
		case <-ctx.Done():
			activity.GetLogger(ctx).Warn("Activity context was cancelled", "Attempt", info.Attempt)
			return "", ctx.Err()
		default:
		}

		// Имитируем выполнение шага (работа занимает 1 секунду)
		activity.GetLogger(ctx).Info("Processing step", "Step", step, "Attempt", info.Attempt)
		time.Sleep(1 * time.Second)

		// На первой попытке (Attempt 1) на шаге 4 имитируем зависание/проблему
		// Мы засыпаем на 4 секунды, что больше, чем HeartbeatTimeout (2 секунды)
		if info.Attempt == 1 && step == 4 {
			activity.GetLogger(ctx).Warn("[SIMULATION] Freezing worker on Attempt 1 at step 4 (sleeping 4s without heartbeating)...")
			time.Sleep(4 * time.Second)
			
			// После долгого сна проверяем, не отменил ли сервер контекст
			select {
			case <-ctx.Done():
				activity.GetLogger(ctx).Error("[SIMULATION] Attempt 1 timed out by server due to missing heartbeat!", "Error", ctx.Err())
				return "", ctx.Err()
			default:
			}
		}

		// Обновляем прогресс и отправляем Heartbeat
		progress.CompletedStep = step
		activity.RecordHeartbeat(ctx, progress)
		activity.GetLogger(ctx).Info("Heartbeat recorded successfully", "CompletedStep", step, "Attempt", info.Attempt)
	}

	return fmt.Sprintf("All %d steps completed successfully on attempt %d!", totalSteps, info.Attempt), nil
}
