package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nativebpm/connectors/camunda"
	"github.com/nativebpm/connectors/nativebpm"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 1. Initialize NativeBPM SDK Client (communicates with our platform on port 8080)
	nativeClient, err := nativebpm.NewClient("http://localhost:8080")
	if err != nil {
		slog.Error("Failed to initialize NativeBPM SDK client", "error", err)
		return
	}

	// 2. Initialize Camunda Client (communicates with external Camunda on port 8082)
	camundaClient, err := camunda.NewClient("http://localhost:8082", "hybrid-bridge-worker")
	if err != nil {
		slog.Error("Failed to initialize Camunda client", "error", err)
		return
	}

	// 3. Create External Task Worker for Camunda
	worker := camunda.NewWorker(camundaClient, logger)

	// Register a task handler. When Camunda requires WASM execution, this worker catches it,
	// deploys/starts a process instance in NativeBPM, and reports results back to Camunda.
	worker.RegisterHandler("delegate-to-nativebpm", camunda.TaskHandlerFunc(func(ctx context.Context, client *camunda.Client, task camunda.ExternalTask, complete camunda.CompleteFunc, fail camunda.FailFunc) error {
		slog.Info("Received task delegation from Camunda", "taskID", task.ID, "variables", task.Variables)

		// Extract variables passed from Camunda
		var amount interface{}
		if v, ok := task.Variables["amount"]; ok {
			amount = v.Value
		}

		// Forward execution to NativeBPM Platform
		slog.Info("Triggering workflow execution in NativeBPM platform...", "process", "userTaskProcess")
		pi, err := nativeClient.StartProcessInstance("userTaskProcess").
			InstanceID(task.ID).
			Variable("amount", amount).
			Variable("source", "camunda").
			Send(ctx)
		if err != nil {
			slog.Error("Failed to forward process execution to NativeBPM platform", "error", err)
			return err
		}

		slog.Info("NativeBPM platform instance started successfully", "nativeInstanceID", pi.ID)

		// Complete external task in Camunda, sending back the NativeBPM execution context
		return complete().
			StringVariable("nativebpm_instance_id", pi.ID).
			StringVariable("bridge_timestamp", time.Now().Format(time.RFC3339)).
			StringVariable("execution_status", "delegated").
			Execute()
	}), 30000, []string{"amount"})

	// Start worker polling loop in background
	slog.Info("Starting Camunda-to-NativeBPM bridge worker...")
	go func() {
		worker.Start(ctx)
	}()

	// Wait for shutdown signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	slog.Info("Shutting down bridge worker...")
}
