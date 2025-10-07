package camunda

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"
)

// MockHandler for testing
type MockHandler struct {
	called bool
	err    error
}

func (m *MockHandler) Handle(ctx context.Context, client *Client, task ExternalTask) error {
	m.called = true
	return m.err
}

func TestNewWorker(t *testing.T) {
	client, err := NewClient("http://localhost:8080", "test-worker")
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError}))
	worker := NewWorker(client, logger)

	if worker == nil {
		t.Fatal("Expected worker to be created")
	}

	if worker.client != client {
		t.Error("Expected worker client to match provided client")
	}

	if worker.logger != logger {
		t.Error("Expected worker logger to match provided logger")
	}

	if worker.maxTasks != 10 {
		t.Errorf("Expected default maxTasks to be 10, got %d", worker.maxTasks)
	}

	if worker.pollInterval != 5*time.Second {
		t.Errorf("Expected default pollInterval to be 5s, got %v", worker.pollInterval)
	}
}

func TestNewWorker_NilLogger(t *testing.T) {
	client, err := NewClient("http://localhost:8080", "test-worker")
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	worker := NewWorker(client, nil)

	if worker == nil {
		t.Fatal("Expected worker to be created")
	}

	if worker.logger == nil {
		t.Error("Expected worker to have a default logger when nil is provided")
	}
}

func TestWorker_RegisterHandler(t *testing.T) {
	client, _ := NewClient("http://localhost:8080", "test-worker")
	logger := slog.Default()
	worker := NewWorker(client, logger)

	handler := &MockHandler{}
	result := worker.RegisterHandler("testTopic", handler, 60000, []string{"var1"})

	// Check fluent API
	if result != worker {
		t.Error("Expected RegisterHandler to return the worker for chaining")
	}

	// Check handler registration
	if len(worker.handlers) != 1 {
		t.Errorf("Expected 1 handler, got %d", len(worker.handlers))
	}

	if worker.handlers["testTopic"] != handler {
		t.Error("Expected handler to be registered for testTopic")
	}

	// Check topic registration
	if len(worker.topics) != 1 {
		t.Errorf("Expected 1 topic, got %d", len(worker.topics))
	}

	topic := worker.topics[0]
	if topic.TopicName != "testTopic" {
		t.Errorf("Expected topic name 'testTopic', got '%s'", topic.TopicName)
	}

	if topic.LockDuration != 60000 {
		t.Errorf("Expected lock duration 60000, got %d", topic.LockDuration)
	}

	if len(topic.Variables) != 1 || topic.Variables[0] != "var1" {
		t.Errorf("Expected variables ['var1'], got %v", topic.Variables)
	}
}

func TestWorker_SetMaxTasks(t *testing.T) {
	client, _ := NewClient("http://localhost:8080", "test-worker")
	worker := NewWorker(client, nil)

	result := worker.SetMaxTasks(20)

	// Check fluent API
	if result != worker {
		t.Error("Expected SetMaxTasks to return the worker for chaining")
	}

	if worker.maxTasks != 20 {
		t.Errorf("Expected maxTasks to be 20, got %d", worker.maxTasks)
	}
}

func TestWorker_SetPollInterval(t *testing.T) {
	client, _ := NewClient("http://localhost:8080", "test-worker")
	worker := NewWorker(client, nil)

	interval := 10 * time.Second
	result := worker.SetPollInterval(interval)

	// Check fluent API
	if result != worker {
		t.Error("Expected SetPollInterval to return the worker for chaining")
	}

	if worker.pollInterval != interval {
		t.Errorf("Expected pollInterval to be %v, got %v", interval, worker.pollInterval)
	}
}

func TestWorker_FluentAPI(t *testing.T) {
	client, _ := NewClient("http://localhost:8080", "test-worker")
	handler := &MockHandler{}

	// Test method chaining
	worker := NewWorker(client, nil).
		RegisterHandler("topic1", handler, 60000, []string{"var1"}).
		RegisterHandler("topic2", handler, 60000, []string{"var2"}).
		SetMaxTasks(15).
		SetPollInterval(3 * time.Second)

	if worker == nil {
		t.Fatal("Expected worker to be created")
	}

	if len(worker.handlers) != 2 {
		t.Errorf("Expected 2 handlers, got %d", len(worker.handlers))
	}

	if worker.maxTasks != 15 {
		t.Errorf("Expected maxTasks to be 15, got %d", worker.maxTasks)
	}

	if worker.pollInterval != 3*time.Second {
		t.Errorf("Expected pollInterval to be 3s, got %v", worker.pollInterval)
	}
}

func TestTaskHandler_Interface(t *testing.T) {
	// Compile-time check that MockHandler implements TaskHandler
	var _ TaskHandler = (*MockHandler)(nil)
}
