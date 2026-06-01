package main

import (
	"log"

	"github.com/nativebpm/connectors/temporal"
	"github.com/nativebpm/connectors/temporal/examples/heartbeat"
)

func main() {
	// Загружаем конфигурацию из окружения
	cfg := temporal.LoadFromEnv()

	// Инициализируем клиент Temporal
	client, err := temporal.NewClient(cfg)
	if err != nil {
		log.Fatalf("Не удалось создать Temporal клиент: %v", err)
	}
	defer client.Close()

	// Инициализируем воркер
	w := temporal.NewWorker(client, cfg.TaskQueue)

	// Регистрируем Workflow и Activity
	w.RegisterWorkflow(heartbeat.HeartbeatWorkflow)
	w.RegisterActivity(heartbeat.HeartbeatActivity)

	log.Printf("Воркер heartbeat успешно запущен для Task Queue: %s", cfg.TaskQueue)
	
	// Запускаем воркер в блокирующем режиме до прерывания
	err = w.Run(nil)
	if err != nil {
		log.Fatalf("Воркер завершился с ошибкой: %v", err)
	}
}
