package main

import (
	"log"

	"github.com/nativebpm/connectors/temporal"
	"github.com/nativebpm/connectors/temporal/examples/helloworld"
)

func main() {
	// Загружаем конфигурацию
	cfg := temporal.LoadFromEnv()

	// Инициализируем наш клиент
	client, err := temporal.NewClient(cfg)
	if err != nil {
		log.Fatalf("Не удалось создать Temporal клиент: %v", err)
	}
	defer client.Close()

	// Инициализируем воркер
	w := temporal.NewWorker(client, cfg.TaskQueue)

	// Регистрируем Workflow и Activity
	w.RegisterWorkflow(helloworld.GreetWorkflow)
	w.RegisterActivity(helloworld.GreetActivity)

	log.Printf("Воркер helloworld успешно запущен для Task Queue: %s", cfg.TaskQueue)
	
	// Запускаем воркер в блокирующем режиме до прерывания
	err = w.Run(nil)
	if err != nil {
		log.Fatalf("Воркер завершился с ошибкой: %v", err)
	}
}
