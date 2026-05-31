package main

import (
	"log"

	"github.com/nativebpm/connectors/temporal"
	"github.com/nativebpm/connectors/temporal/examples/signal"
)

func main() {
	cfg := temporal.LoadFromEnv()

	client, err := temporal.NewClient(cfg)
	if err != nil {
		log.Fatalf("Не удалось создать Temporal клиент: %v", err)
	}
	defer client.Close()

	w := temporal.NewWorker(client, cfg.TaskQueue)

	// Регистрируем Workflow
	w.RegisterWorkflow(signal.SubscriptionWorkflow)

	log.Printf("Воркер signal успешно запущен для Task Queue: %s", cfg.TaskQueue)

	err = w.Run(nil)
	if err != nil {
		log.Fatalf("Воркер завершился с ошибкой: %v", err)
	}
}
