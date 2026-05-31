package temporal

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"sync"
	"time"

	_ "github.com/lib/pq"
	"github.com/sequinstream/sequin-go"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/workflow"
)

// customTaskRecord представляет схему строки в таблице custom_task_queue, получаемую из Sequin CDC
type customTaskRecord struct {
	ID         int    `json:"id"`
	TaskType   string `json:"task_type"`
	Payload    string `json:"payload"`
	WorkflowID string `json:"workflow_id"`
	RunID      string `json:"run_id"`
}

// CDCActivities содержит активности для делегирования задач в CDC очередь
type CDCActivities struct {
	db *sql.DB
}

// NewCDCActivities создает экземпляр CDCActivities и подключается к базе данных PostgreSQL.
func NewCDCActivities(cfg *Config) (*CDCActivities, error) {
	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		cfg.DBHost, cfg.DBPort, cfg.DBUser, cfg.DBPassword, cfg.DBName)
	
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, err
	}

	// Настраиваем пул соединений для высоких нагрузок
	db.SetMaxOpenConns(150)
	db.SetMaxIdleConns(150)
	db.SetConnMaxLifetime(5 * time.Minute)

	// Проверяем соединение
	err = db.Ping()
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping postgres: %w", err)
	}

	return &CDCActivities{db: db}, nil
}

// Close закрывает соединение с базой данных.
func (a *CDCActivities) Close() error {
	if a.db != nil {
		return a.db.Close()
	}
	return nil
}

// DelegateToSequin записывает задачу в таблицу custom_task_queue СУБД, инициируя WAL CDC репликацию.
func (a *CDCActivities) DelegateToSequin(ctx context.Context, taskType string, payload string) error {
	info := activity.GetInfo(ctx)
	workflowID := info.WorkflowExecution.ID
	runID := info.WorkflowExecution.RunID

	query := `INSERT INTO custom_task_queue (task_type, payload, workflow_id, run_id) VALUES ($1, $2, $3, $4)`
	_, err := a.db.ExecContext(ctx, query, taskType, payload, workflowID, runID)
	if err != nil {
		activity.GetLogger(ctx).Error("Failed to insert task to custom_task_queue", "error", err)
		return err
	}

	return nil
}

// AwaitCDCResult приостанавливает выполнение Workflow до получения сигнала завершения от CDC воркера.
func AwaitCDCResult(ctx workflow.Context, signalName string, resultTarget any) error {
	signalChan := workflow.GetSignalChannel(ctx, signalName)
	signalChan.Receive(ctx, resultTarget)
	return nil
}

// GreetCDCWorkflow представляет пример Workflow, использующего делегирование задач в CDC-очередь.
func GreetCDCWorkflow(ctx workflow.Context, name string) (string, error) {
	options := workflow.ActivityOptions{
		StartToCloseTimeout: 5 * time.Second,
	}
	ctx = workflow.WithActivityOptions(ctx, options)

	var activities *CDCActivities

	// Шаг 1: Записываем задачу в БД (WAL CDC)
	err := workflow.ExecuteActivity(ctx, activities.DelegateToSequin, "greet-cdc", name).Get(ctx, nil)
	if err != nil {
		return "", err
	}

	// Шаг 2: Ждем сигнал завершения от CDC воркера
	var result string
	err = AwaitCDCResult(ctx, "TaskCompletedSignal", &result)
	return result, err
}

// CDCHandler описывает функцию-обработчик CDC задачи
type CDCHandler func(ctx context.Context, payload string) (string, error)

// SequinCDCWorker опрашивает Sequin CDC и выполняет задачи, возвращая результат в Temporal через сигналы.
type SequinCDCWorker struct {
	temporalClient *Client
	sequinClient   *sequin.Client
	consumer       string
	logger         *slog.Logger
	handlers       map[string]CDCHandler
	wg             sync.WaitGroup
	maxConcurrency int
	taskSemaphore  chan struct{}
}

// NewSequinCDCWorker создает новый экземпляр SequinCDCWorker.
func NewSequinCDCWorker(temporalClient *Client, sequinURL string, consumer string, logger *slog.Logger) (*SequinCDCWorker, error) {
	if logger == nil {
		logger = slog.Default()
	}

	token := os.Getenv("SEQUIN_API_TOKEN")
	if token == "" {
		token = "sequin_loadtest_secret_token_12345"
	}

	// Оптимизируем глобальный HTTP транспорт для клиентов Sequin и завершений задач
	if transport, ok := http.DefaultTransport.(*http.Transport); ok {
		transport.MaxIdleConns = 300
		transport.MaxIdleConnsPerHost = 300
		transport.IdleConnTimeout = 90 * time.Second
	}

	opts := &sequin.ClientOptions{
		BaseURL: sequinURL,
	}
	sequinClient := sequin.NewClient(token, opts)

	maxConcurrency := 50

	return &SequinCDCWorker{
		temporalClient: temporalClient,
		sequinClient:   sequinClient,
		consumer:       consumer,
		logger:         logger,
		handlers:       make(map[string]CDCHandler),
		maxConcurrency: maxConcurrency,
		taskSemaphore:  make(chan struct{}, maxConcurrency),
	}, nil
}

// RegisterHandler регистрирует обработчик для определенного типа задач.
func (w *SequinCDCWorker) RegisterHandler(taskType string, handler CDCHandler) *SequinCDCWorker {
	w.handlers[taskType] = handler
	return w
}

// SetMaxConcurrency настраивает максимальную параллельность выполнения задач.
func (w *SequinCDCWorker) SetMaxConcurrency(maxConcurrency int) *SequinCDCWorker {
	if maxConcurrency < 1 {
		maxConcurrency = 1
	}
	w.maxConcurrency = maxConcurrency
	w.taskSemaphore = make(chan struct{}, maxConcurrency)
	return w
}

// Start запускает воркер CDC опроса и обработки в неблокирующем режиме.
func (w *SequinCDCWorker) Start(ctx context.Context) {
	w.logger.Info("Starting Sequin CDC Worker for Temporal tasks",
		"consumer", w.consumer,
		"max_concurrency", w.maxConcurrency,
	)

	go func() {
		for {
			select {
			case <-ctx.Done():
				w.logger.Info("Stopping CDC Worker, waiting for active tasks...")
				w.wg.Wait()
				return
			default:
				// Опрашиваем Sequin
				batchSize := w.maxConcurrency
				if batchSize > 50 {
					batchSize = 50
				}

				msgs, err := w.sequinClient.Receive(ctx, w.consumer, &sequin.ReceiveParams{
					BatchSize: batchSize,
					WaitFor:   5000, // 5s long poll
				})
				if err != nil {
					w.logger.Error("Failed to receive messages from Sequin", "error", err)
					time.Sleep(1 * time.Second)
					continue
				}

				if len(msgs) == 0 {
					continue
				}

				for _, msg := range msgs {
					select {
					case <-ctx.Done():
						return
					case w.taskSemaphore <- struct{}{}:
					}

					w.wg.Add(1)
					go func(m sequin.Message) {
						defer func() {
							<-w.taskSemaphore
							w.wg.Done()
						}()
						w.processMessage(ctx, m)
					}(msg)
				}
			}
		}
	}()
}

func (w *SequinCDCWorker) processMessage(ctx context.Context, msg sequin.Message) {
	// Десериализуем запись
	var record customTaskRecord
	if err := json.Unmarshal(msg.Record, &record); err != nil {
		w.logger.Error("Failed to parse task record", "error", err)
		_ = w.sequinClient.Ack(ctx, w.consumer, []string{msg.AckID})
		return
	}

	handler, ok := w.handlers[record.TaskType]
	if !ok {
		// Нет обработчика, удаляем задачу из очереди
		_ = w.sequinClient.Ack(ctx, w.consumer, []string{msg.AckID})
		return
	}

	// Выполняем задачу
	result, err := handler(ctx, record.Payload)
	if err != nil {
		w.logger.Error("CDC handler returned error", "task_id", record.ID, "error", err)
		// Отправляем NACK для повторной попытки
		_ = w.sequinClient.Nack(ctx, w.consumer, []string{msg.AckID})
		return
	}

	// Отправляем сигнал завершения обратно в Temporal
	// По умолчанию сигнал называется "TaskCompletedSignal"
	err = w.temporalClient.SignalWorkflow(ctx, record.WorkflowID, record.RunID, "TaskCompletedSignal", result)
	if err != nil {
		w.logger.Error("Failed to signal workflow back to Temporal", "workflow_id", record.WorkflowID, "error", err)
		// Возвращаем в очередь, чтобы повторить попытку отправки сигнала
		_ = w.sequinClient.Nack(ctx, w.consumer, []string{msg.AckID})
		return
	}

	// Подтверждаем обработку в Sequin
	err = w.sequinClient.Ack(ctx, w.consumer, []string{msg.AckID})
	if err != nil {
		w.logger.Error("Failed to ack message in Sequin", "ack_id", msg.AckID, "error", err)
	}
}
