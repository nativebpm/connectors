package temporal

import (
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
)

// Worker is a wrapper around the Temporal worker for simplified
// Workflow and Activity registration and lifecycle management.
type Worker struct {
	rawWorker worker.Worker
	taskQueue string
}

// NewWorker initializes a new worker instance for the specified Task Queue.
func NewWorker(client *Client, taskQueue string) *Worker {
	if taskQueue == "" {
		taskQueue = client.config.TaskQueue
	}

	w := worker.New(client.RawClient(), taskQueue, worker.Options{
		MaxConcurrentActivityExecutionSize:     1000,
		MaxConcurrentWorkflowTaskExecutionSize: 1000,
		MaxConcurrentActivityTaskPollers:       16,
		MaxConcurrentWorkflowTaskPollers:       16,
	})
	return &Worker{
		rawWorker: w,
		taskQueue: taskQueue,
	}
}

// RegisterWorkflow registers a Workflow function in the worker.
func (w *Worker) RegisterWorkflow(wf any) {
	w.rawWorker.RegisterWorkflow(wf)
}

// RegisterWorkflowWithOptions registers a Workflow function with custom registration options.
func (w *Worker) RegisterWorkflowWithOptions(wf any, options workflow.RegisterOptions) {
	w.rawWorker.RegisterWorkflowWithOptions(wf, options)
}

// RegisterActivity registers an Activity function or struct in the worker.
func (w *Worker) RegisterActivity(act any) {
	w.rawWorker.RegisterActivity(act)
}

// RegisterActivityWithOptions registers an Activity function or struct with options.
func (w *Worker) RegisterActivityWithOptions(act any, options activity.RegisterOptions) {
	w.rawWorker.RegisterActivityWithOptions(act, options)
}

// Start starts the worker in a non-blocking mode.
func (w *Worker) Start() error {
	return w.rawWorker.Start()
}

// Stop stops the worker, finishing active task processing.
func (w *Worker) Stop() {
	w.rawWorker.Stop()
}

// Run starts the worker in blocking mode until an interrupt signal is received.
func (w *Worker) Run(interruptCh <-chan interface{}) error {
	return w.rawWorker.Run(interruptCh)
}

// RawWorker returns the original SDK worker object.
func (w *Worker) RawWorker() worker.Worker {
	return w.rawWorker
}
