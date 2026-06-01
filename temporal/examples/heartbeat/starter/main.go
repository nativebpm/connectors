package main

import (
	"context"
	"log"

	"github.com/google/uuid"
	"github.com/nativebpm/connectors/temporal"
	"github.com/nativebpm/connectors/temporal/examples/heartbeat"
	"go.temporal.io/sdk/client"
)

func main() {
	// Загружаем конфигурацию из окружения
	cfg := temporal.LoadFromEnv()

	// Инициализируем клиент Temporal
	c, err := temporal.NewClient(cfg)
	if err != nil {
		log.Fatalf("Не удалось создать Temporal клиент: %v", err)
	}
	defer c.Close()

	workflowID := "heartbeat-workflow-" + uuid.New().String()
	options := client.StartWorkflowOptions{
		ID:        workflowID,
		TaskQueue: cfg.TaskQueue,
	}

	log.Printf("Запуск Workflow с ID: %s", workflowID)

	// Запускаем Workflow с общим количеством шагов = 10
	run, err := c.ExecuteWorkflow(context.Background(), options, heartbeat.HeartbeatWorkflow, 10)
	if err != nil {
		log.Fatalf("Ошибка при запуске Workflow: %v", err)
	}

	var result string
	// Ожидаем результат выполнения
	err = run.Get(context.Background(), &result)
	if err != nil {
		log.Fatalf("Ошибка при получении результата Workflow: %v", err)
	}

	log.Printf("Workflow успешно завершен! Результат: %s", result)
}
