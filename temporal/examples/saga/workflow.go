package saga

import (
	"time"

	"go.temporal.io/sdk/workflow"
)

type TripReservationParams struct {
	Amount      float64
	HotelName   string
	Destination string
}

// TripReservationWorkflow coordinates transactions and compensations via the Saga pattern.
func TripReservationWorkflow(ctx workflow.Context, params TripReservationParams) (string, error) {
	options := workflow.ActivityOptions{
		StartToCloseTimeout: 5 * time.Second,
	}
	ctx = workflow.WithActivityOptions(ctx, options)

	var activities *TripReservationActivities
	logger := workflow.GetLogger(ctx)

	// List of compensations to execute in case of failure (LIFO)
	var compensations []func(ctx workflow.Context) error

	defer func() {
		// If execution failed (e.g. panic or returned error),
		// execute the recorded compensations.
		// In Temporal it is crucial to use workflow.NewDisconnectedContext to execute
		// compensations even if the parent workflow context was cancelled (e.g. on timeout).
	}()

	// 1. Credit reservation
	var creditResult string
	err := workflow.ExecuteActivity(ctx, activities.ReserveCredit, params.Amount).Get(ctx, &creditResult)
	if err != nil {
		logger.Error("ReserveCredit failed", "error", err)
		return "", err
	}
	// Add compensation to the stack
	compensations = append(compensations, func(ctx workflow.Context) error {
		return workflow.ExecuteActivity(ctx, activities.RefundCredit, params.Amount).Get(ctx, nil)
	})

	// 2. Hotel booking
	var hotelResult string
	err = workflow.ExecuteActivity(ctx, activities.BookHotel, params.HotelName).Get(ctx, &hotelResult)
	if err != nil {
		logger.Error("BookHotel failed, initiating saga rollback", "error", err)
		executeCompensations(ctx, compensations)
		return "", err
	}
	// Add compensation
	compensations = append(compensations, func(ctx workflow.Context) error {
		return workflow.ExecuteActivity(ctx, activities.CancelHotel, params.HotelName).Get(ctx, nil)
	})

	// 3. Flight booking
	var flightResult string
	err = workflow.ExecuteActivity(ctx, activities.BookFlight, params.Destination).Get(ctx, &flightResult)
	if err != nil {
		logger.Error("BookFlight failed, initiating saga rollback", "error", err)
		executeCompensations(ctx, compensations)
		return "", err
	}

	logger.Info("Trip reservation completed successfully!")
	return "Trip successfully reserved!", nil
}

// executeCompensations runs compensations in reverse order (LIFO)
func executeCompensations(ctx workflow.Context, compensations []func(workflow.Context) error) {
	// Create disconnected context to execute compensations,
	// so they won't be aborted if the main context was cancelled.
	disconnectedCtx, _ := workflow.NewDisconnectedContext(ctx)
	logger := workflow.GetLogger(ctx)

	logger.Info("Running compensations (saga rollback)...")
	for i := len(compensations) - 1; i >= 0; i-- {
		err := compensations[i](disconnectedCtx)
		if err != nil {
			logger.Error("Failed to execute compensation", "index", i, "error", err)
			// In production systems, this might require manual review queue escalation
		}
	}
}
