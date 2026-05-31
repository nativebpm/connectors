package temporal

import (
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"
)

// Worker является оберткой над воркером Temporal для упрощенной регистрации 
// Workflow и Activity и управления их жизненным циклом.
type Worker struct {
	rawWorker worker.Worker
	taskQueue string
}

// NewWorker инициализирует новый экземпляр воркера для заданной очереди задач (Task Queue).
func NewWorker(client *Client, taskQueue string) *Worker {
	if taskQueue == "" {
		taskQueue = client.config.TaskQueue
	}

	w := worker.New(client.RawClient(), taskQueue, worker.Options{})
	return &Worker{
		rawWorker: w,
		taskQueue: taskQueue,
	}
}

// RegisterWorkflow регистрирует функцию Workflow в воркере.
func (w *Worker) RegisterWorkflow(wf any) {
	w.rawWorker.RegisterWorkflow(wf)
}

// RegisterWorkflowWithOptions регистрирует функцию Workflow с кастомными настройками регистрации.
func (w *Worker) RegisterWorkflowWithOptions(wf any, options workflow.RegisterOptions) {
	w.rawWorker.RegisterWorkflowWithOptions(wf, options)
}

// RegisterActivity регистрирует функцию или структуру Activity в воркере.
func (w *Worker) RegisterActivity(act any) {
	w.rawWorker.RegisterActivity(act)
}

// RegisterActivityWithOptions регистрирует функцию или структуру Activity с настройками.
func (w *Worker) RegisterActivityWithOptions(act any, options activity.RegisterOptions) {
	w.rawWorker.RegisterActivityWithOptions(act, options)
}

// Start запускает воркер в неблокирующем режиме.
func (w *Worker) Start() error {
	return w.rawWorker.Start()
}

// Stop останавливает воркер, завершая обработку текущих задач.
func (w *Worker) Stop() {
	w.rawWorker.Stop()
}

// Run запускает воркер в блокирующем режиме до получения сигнала прерывания.
func (w *Worker) Run(interruptCh <-chan interface{}) error {
	return w.rawWorker.Run(interruptCh)
}

// RawWorker возвращает оригинальный объект воркера SDK.
func (w *Worker) RawWorker() worker.Worker {
	return w.rawWorker
}
