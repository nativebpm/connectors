package main

import (
	"context"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/nativebpm/connectors/temporal"
	"github.com/nativebpm/connectors/temporal/examples/signal"
	"go.temporal.io/sdk/client"
)

func main() {
	cfg := temporal.LoadFromEnv()

	c, err := temporal.NewClient(cfg)
	if err != nil {
		log.Fatalf("Не удалось создать Temporal клиент: %v", err)
	}
	defer c.Close()

	workflowID := "subscription-workflow-" + uuid.New().String()
	options := client.StartWorkflowOptions{
		ID:        workflowID,
		TaskQueue: cfg.TaskQueue,
	}

	log.Printf("1. Запуск SubscriptionWorkflow с ID: %s на 1 минуту", workflowID)
	// Запускаем на 1 минуту виртуального/реального времени
	run, err := c.ExecuteWorkflow(context.Background(), options, signal.SubscriptionWorkflow, 1*time.Minute)
	if err != nil {
		log.Fatalf("Ошибка при запуске Workflow: %v", err)
	}

	// Даем воркеру немного времени запустить процесс
	time.Sleep(1 * time.Second)

	// 2. Query - Запрос статуса
	queryResp, err := c.QueryWorkflow(context.Background(), workflowID, "", "GetSubscriptionStatus")
	if err != nil {
		log.Fatalf("Ошибка при отправке Query: %v", err)
	}
	var status signal.SubscriptionStatus
	if err := queryResp.Get(&status); err != nil {
		log.Fatalf("Не удалось разобрать статус: %v", err)
	}
	log.Printf("2. Ответ на Query (Начальный статус): State=%s, Billing=%s", status.State, status.BillingInfo)

	// 3. Signal - Обновление биллинга
	log.Printf("3. Отправка сигнала UpdateBillingInfo с новыми реквизитами...")
	err = c.SignalWorkflow(context.Background(), workflowID, "", "UpdateBillingInfo", "PayPal Account")
	if err != nil {
		log.Fatalf("Ошибка при отправке сигнала UpdateBillingInfo: %v", err)
	}

	// Даем воркеру обработать сигнал
	time.Sleep(1 * time.Second)

	// 4. Query - Повторный запрос статуса
	queryResp, err = c.QueryWorkflow(context.Background(), workflowID, "", "GetSubscriptionStatus")
	if err != nil {
		log.Fatalf("Ошибка при отправке Query: %v", err)
	}
	if err := queryResp.Get(&status); err != nil {
		log.Fatalf("Не удалось разобрать статус: %v", err)
	}
	log.Printf("4. Ответ на Query (После сигнала обновления): State=%s, Billing=%s", status.State, status.BillingInfo)

	// 5. Signal - Отмена подписки
	log.Printf("5. Отправка сигнала CancelSubscription...")
	err = c.SignalWorkflow(context.Background(), workflowID, "", "CancelSubscription", nil)
	if err != nil {
		log.Fatalf("Ошибка при отправке сигнала CancelSubscription: %v", err)
	}

	// 6. Ожидание завершения
	var finalResult string
	err = run.Get(context.Background(), &finalResult)
	if err != nil {
		log.Fatalf("Ошибка при выполнении Workflow: %v", err)
	}

	log.Printf("6. Workflow успешно завершен с финальным статусом: %s", finalResult)
}
