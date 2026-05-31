package signal

import (
	"time"

	"go.temporal.io/sdk/workflow"
)

// SubscriptionStatus представляет текущий статус подписки.
type SubscriptionStatus struct {
	State       string    `json:"state"`
	BillingInfo string    `json:"billingInfo"`
	UpdatedTime time.Time `json:"updatedTime"`
}

// SubscriptionWorkflow моделирует процесс подписки с возможностью обновления биллинга и отмены.
func SubscriptionWorkflow(ctx workflow.Context, duration time.Duration) (string, error) {
	logger := workflow.GetLogger(ctx)
	logger.Info("Начало выполнения SubscriptionWorkflow", "duration", duration)

	status := SubscriptionStatus{
		State:       "Active",
		BillingInfo: "Default Billing Method",
		UpdatedTime: workflow.Now(ctx),
	}

	// Регистрируем Query-хендлер для возврата текущего статуса
	err := workflow.SetQueryHandler(ctx, "GetSubscriptionStatus", func() (SubscriptionStatus, error) {
		return status, nil
	})
	if err != nil {
		logger.Error("Не удалось зарегистрировать QueryHandler", "error", err)
		return "", err
	}

	// Создаем селектор для ожидания различных сигналов или таймаута
	selector := workflow.NewSelector(ctx)

	// Сигнал отмены
	cancelChan := workflow.GetSignalChannel(ctx, "CancelSubscription")
	selector.AddReceive(cancelChan, func(c workflow.ReceiveChannel, more bool) {
		c.Receive(ctx, nil)
		status.State = "Cancelled"
		status.UpdatedTime = workflow.Now(ctx)
		logger.Info("Получен сигнал CancelSubscription")
	})

	// Сигнал обновления биллинга
	billingChan := workflow.GetSignalChannel(ctx, "UpdateBillingInfo")
	selector.AddReceive(billingChan, func(c workflow.ReceiveChannel, more bool) {
		var newBilling string
		c.Receive(ctx, &newBilling)
		status.BillingInfo = newBilling
		status.State = "Updated"
		status.UpdatedTime = workflow.Now(ctx)
		logger.Info("Получен сигнал UpdateBillingInfo", "newBilling", newBilling)
	})

	// Таймаут подписки
	selector.AddFuture(workflow.NewTimer(ctx, duration), func(f workflow.Future) {
		if status.State != "Cancelled" {
			status.State = "Expired"
			status.UpdatedTime = workflow.Now(ctx)
			logger.Info("Срок действия подписки истек")
		}
	})

	// Ждем наступления одного из событий: таймаут подписки или сигнал отмены
	for status.State == "Active" || status.State == "Updated" {
		selector.Select(ctx)
	}

	return status.State, nil
}
