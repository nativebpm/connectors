package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/nativebpm/connectors/camunda"
)

func main() {
	logger := slog.Default()

	// Create a new Camunda client
	client, err := camunda.NewClient("http://localhost:8080", "loan-worker")
	if err != nil {
		logger.Error("Failed to create client", "error", err)
		return
	}

	// Add logging middleware using fluent API
	client.WithLogger(logger)

	// Deploy the BPMN process
	file, err := os.Open("bpmn/loan-granting.bpmn")
	if err != nil {
		logger.Error("Failed to open BPMN file", "error", err)
		return
	}
	defer file.Close()

	deploymentID, err := client.DeployProcess(context.Background(), "loan-granting-deployment", file, file.Name())
	if err != nil {
		logger.Error("Failed to deploy BPMN", "error", err)
		return
	}
	logger.Info("Deployed BPMN process", "deploymentID", deploymentID)

	// Define topics to subscribe to based on loan-granting.bpmn process
	topics := []camunda.TopicRequest{
		{
			TopicName:    "creditScoreChecker",
			LockDuration: 60000, // 1 minute
			Variables:    []string{"defaultScore"},
		},
		{
			TopicName:    "loanGranter",
			LockDuration: 60000,
			Variables:    []string{"score"},
		},
		{
			TopicName:    "requestRejecter",
			LockDuration: 60000,
			Variables:    []string{"score"},
		},
	}

	ctx := context.Background()

	// Poll for tasks in a loop
	logger.Info("Starting worker to poll for loan tasks...")
	client.PollTasks(ctx, topics, 10, processLoanTask)
}

func processLoanTask(client *camunda.Client, task camunda.ExternalTask) {
	logger := slog.Default()
	ctx := context.Background()
	logger.Info("Processing loan task", "taskID", task.ID, "topic", task.TopicName)

	// Simulate processing based on topic
	time.Sleep(2 * time.Second)

	variables := make(map[string]camunda.Variable)

	switch task.TopicName {
	case "creditScoreChecker":
		// Simulate credit score check - return array of scores
		scores := []int{7, 8, 6} // Example scores
		variables["creditScores"] = camunda.JSONVariable(scores)
		logger.Info("Checked credit scores", "scores", scores)

	case "loanGranter":
		// Grant loan for good score
		if scoreVar, ok := task.Variables["score"]; ok {
			score := scoreVar.Value.(float64) // Assuming it's a number
			logger.Info("Granting loan", "score", score)
			variables["loanGranted"] = camunda.BooleanVariable(true)
			variables["loanAmount"] = camunda.DoubleVariable(10000.00)
		}

	case "requestRejecter":
		// Reject loan for bad score
		if scoreVar, ok := task.Variables["score"]; ok {
			score := scoreVar.Value.(float64)
			logger.Info("Rejecting loan request", "score", score)
			variables["loanRejected"] = camunda.BooleanVariable(true)
			variables["reason"] = camunda.StringVariable("Low credit score")
		}
	}

	// Complete the task
	err := client.Complete(ctx, task.ID, variables, nil)
	if err != nil {
		logger.Error("Failed to complete task", "taskID", task.ID, "error", err)
		// Handle failure
		err := client.HandleFailure(ctx, task.ID, "Processing failed", "Detailed error message", 3, 30000)
		if err != nil {
			logger.Error("Failed to handle failure for task", "taskID", task.ID, "error", err)
		}
	} else {
		logger.Info("Completed loan task", "taskID", task.ID, "topic", task.TopicName)
	}
}
