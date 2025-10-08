package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nativebpm/connectors/camunda"
	"github.com/nativebpm/connectors/camunda/examples/loan-granting/handlers"
)

func main() {
	logger := slog.Default()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create a new Camunda client
	client, err := camunda.NewClient("http://localhost:8080", "loan-worker")
	if err != nil {
		logger.Error("Failed to create client", "error", err)
		return
	}

	// Add logging middleware
	client.WithLogger(logger)

	// Deploy the BPMN process
	if err := deployProcess(ctx, client, logger); err != nil {
		logger.Error("Failed to deploy process", "error", err)
		return
	}

	// Start a test process instance
	if err := startTestProcessInstance(ctx, client, logger); err != nil {
		logger.Error("Failed to start test process instance", "error", err)
		return
	}

	// Create and configure the worker
	w := createWorker(client, logger)

	// Handle graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		logger.Info("Shutdown signal received, stopping worker...")
		cancel()
	}()

	// Start the worker (blocking call)
	logger.Info("Starting external task worker... Press Ctrl+C to stop")
	w.Start(ctx)

	// Worker stopped gracefully
	logger.Info("Worker stopped gracefully")
}

// deployProcess deploys the BPMN process definition
func deployProcess(ctx context.Context, client *camunda.Client, logger *slog.Logger) error {
	file, err := os.Open("bpmn/loan-granting.bpmn")
	if err != nil {
		return err
	}
	defer file.Close()

	deploymentID, err := client.DeployProcess(ctx, "loan-granting-deployment", file, file.Name())
	if err != nil {
		return err
	}

	logger.Info("Deployed BPMN process", "deploymentID", deploymentID)
	return nil
}

// startTestProcessInstance starts a test process instance
func startTestProcessInstance(ctx context.Context, client *camunda.Client, logger *slog.Logger) error {
	processInstanceID, err := client.StartProcessInstance(ctx, "loan_process", map[string]any{})
	if err != nil {
		return err
	}

	logger.Info("Started test process instance", "id", processInstanceID)
	return nil
}

// createWorker creates and configures the external task worker
func createWorker(client *camunda.Client, logger *slog.Logger) *camunda.Worker {
	// Create handlers
	creditScoreChecker := handlers.NewCreditScoreChecker(logger)
	loanGranter := handlers.NewLoanGranter(logger)
	requestRejecter := handlers.NewRequestRejecter(logger)

	// Create worker and register handlers
	w := camunda.NewWorker(client, logger)
	w.RegisterHandler("creditScoreChecker", creditScoreChecker, 60000, []string{"defaultScore"})
	w.RegisterHandler("loanGranter", loanGranter, 60000, []string{"score"})
	w.RegisterHandler("requestRejecter", requestRejecter, 60000, []string{"score"})
	w.SetMaxTasks(10)
	w.SetPollInterval(5 * time.Second)

	logger.Info("Worker configured",
		"topics", 3,
		"maxTasks", 10,
		"pollInterval", "5s")

	return w
}
