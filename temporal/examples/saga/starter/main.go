package main

import (
	"context"
	"log"

	"github.com/google/uuid"
	"github.com/nativebpm/connectors/temporal"
	"github.com/nativebpm/connectors/temporal/examples/saga"
	"go.temporal.io/sdk/client"
)

func main() {
	cfg := temporal.LoadFromEnv()

	c, err := temporal.NewClient(cfg)
	if err != nil {
		log.Fatalf("Не удалось создать Temporal клиент: %v", err)
	}
	defer c.Close()

	// 1. Запускаем успешный сценарий
	successWorkflowID := "saga-success-workflow-" + uuid.New().String()
	successOptions := client.StartWorkflowOptions{
		ID:        successWorkflowID,
		TaskQueue: cfg.TaskQueue,
	}

	log.Printf("1. Запуск успешной Саги (Тур в Paris) с ID: %s", successWorkflowID)
	successParams := saga.TripReservationParams{
		Amount:      500.0,
		HotelName:   "Grand Plaza Hotel",
		Destination: "Paris",
	}

	runSuccess, err := c.ExecuteWorkflow(context.Background(), successOptions, saga.TripReservationWorkflow, successParams)
	if err != nil {
		log.Fatalf("Ошибка при запуске успешной Саги: %v", err)
	}

	var successResult string
	err = runSuccess.Get(context.Background(), &successResult)
	if err != nil {
		log.Fatalf("Ошибка выполнения успешной Саги: %v", err)
	}
	log.Printf("1. Результат успешной Саги: %s\n\n", successResult)

	// 2. Запускаем сценарий со сбоем на этапе бронирования перелета
	failWorkflowID := "saga-fail-workflow-" + uuid.New().String()
	failOptions := client.StartWorkflowOptions{
		ID:        failWorkflowID,
		TaskQueue: cfg.TaskQueue,
	}

	log.Printf("2. Запуск неуспешной Саги (Тур в Fail - отмена рейса) с ID: %s", failWorkflowID)
	failParams := saga.TripReservationParams{
		Amount:      300.0,
		HotelName:   "Cozy Hostel",
		Destination: "Fail", // Вызовет ошибку в BookFlight
	}

	runFail, err := c.ExecuteWorkflow(context.Background(), failOptions, saga.TripReservationWorkflow, failParams)
	if err != nil {
		log.Fatalf("Ошибка при запуске неуспешной Саги: %v", err)
	}

	var failResult string
	err = runFail.Get(context.Background(), &failResult)
	if err == nil {
		log.Fatalf("Ожидалась ошибка выполнения Саги, но процесс завершился успешно с результатом: %s", failResult)
	}
	log.Printf("2. Сага успешно завершилась с ошибкой: %v (проверьте логи воркера для подтверждения выполнения компенсаций)", err)
}
