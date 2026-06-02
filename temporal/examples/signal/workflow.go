package signal

import (
	"time"

	"go.temporal.io/sdk/workflow"
)

// SubscriptionStatus represents the current subscription status.
type SubscriptionStatus struct {
	State       string    `json:"state"`
	BillingInfo string    `json:"billingInfo"`
	UpdatedTime time.Time `json:"updatedTime"`
}

// SubscriptionWorkflow models the subscription process with options to update billing and cancel.
func SubscriptionWorkflow(ctx workflow.Context, duration time.Duration) (string, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Starting SubscriptionWorkflow", "duration", duration)

	status := SubscriptionStatus{
		State:       "Active",
		BillingInfo: "Default Billing Method",
		UpdatedTime: workflow.Now(ctx),
	}

	// Register Query handler to return current status
	err := workflow.SetQueryHandler(ctx, "GetSubscriptionStatus", func() (SubscriptionStatus, error) {
		return status, nil
	})
	if err != nil {
		logger.Error("Failed to register QueryHandler", "error", err)
		return "", err
	}

	// Create selector to wait for various signals or timeout
	selector := workflow.NewSelector(ctx)

	// Cancel signal
	cancelChan := workflow.GetSignalChannel(ctx, "CancelSubscription")
	selector.AddReceive(cancelChan, func(c workflow.ReceiveChannel, more bool) {
		c.Receive(ctx, nil)
		status.State = "Canceled"
		status.UpdatedTime = workflow.Now(ctx)
		logger.Info("Received CancelSubscription signal")
	})

	// Billing update signal
	billingChan := workflow.GetSignalChannel(ctx, "UpdateBillingInfo")
	selector.AddReceive(billingChan, func(c workflow.ReceiveChannel, more bool) {
		var newBilling string
		c.Receive(ctx, &newBilling)
		status.BillingInfo = newBilling
		status.State = "Updated"
		status.UpdatedTime = workflow.Now(ctx)
		logger.Info("Received UpdateBillingInfo signal", "newBilling", newBilling)
	})

	// Subscription timeout
	selector.AddFuture(workflow.NewTimer(ctx, duration), func(f workflow.Future) {
		if status.State != "Canceled" {
			status.State = "Expired"
			status.UpdatedTime = workflow.Now(ctx)
			logger.Info("Subscription expired")
		}
	})

	// Wait for events: subscription timeout or cancel signal
	for status.State == "Active" || status.State == "Updated" {
		selector.Select(ctx)
	}

	return status.State, nil
}
