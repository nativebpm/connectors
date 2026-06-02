package main

import (
	"context"
	"log/slog"
	"os"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
	"github.com/nativebpm/connectors/temporal"
	gotenbergtelegram "github.com/nativebpm/connectors/temporal/examples/gotenberg-telegram"
	"go.temporal.io/sdk/client"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	_ = godotenv.Load("temporal.env")

	// Get target chat ID
	chatIDStr := os.Getenv("TELEGRAM_CHAT_ID")
	if chatIDStr == "" {
		logger.Error("TELEGRAM_CHAT_ID environment variable is not set")
		os.Exit(1)
	}
	chatID, err := strconv.ParseInt(chatIDStr, 10, 64)
	if err != nil {
		logger.Error("Failed to parse TELEGRAM_CHAT_ID as integer", "error", err)
		os.Exit(1)
	}

	// 1. Initialize Temporal Client
	cfg := temporal.LoadFromEnv()
	c, err := temporal.NewClient(cfg)
	if err != nil {
		logger.Error("Failed to create Temporal client", "error", err)
		os.Exit(1)
	}
	defer c.Close()

	// 2. Define beautiful HTML content to convert to PDF
	htmlContent := `
<!DOCTYPE html>
<html>
<head>
    <meta charset="utf-8">
    <style>
        body { font-family: sans-serif; padding: 30px; color: #333; }
        h1 { color: #4F46E5; border-bottom: 2px solid #E5E7EB; padding-bottom: 10px; }
        p { font-size: 16px; line-height: 1.5; }
        .footer { margin-top: 50px; font-size: 12px; color: #9CA3AF; text-align: center; }
    </style>
</head>
<body>
    <h1>Отчет о нативной интеграции</h1>
    <p>Приветствуем вас!</p>
    <p>Этот документ сгенерирован с помощью микросервиса <strong>Gotenberg Chromium</strong> и отправлен вам в Telegram через нативный сценарий оркестрации <strong>Temporal</strong>.</p>
    <p>Наш монорепозиторий коннекторов позволяет объединять различные системы быстро и без лишней сложности (такой как промежуточные таблицы БД или CDC).</p>
    <div class="footer">Сгенерировано в: ` + time.Now().Format("2006-01-02 15:04:05") + `</div>
</body>
</html>`

	workflowID := "document-generation-workflow-" + uuid.New().String()
	options := client.StartWorkflowOptions{
		ID:        workflowID,
		TaskQueue: cfg.TaskQueue,
	}

	logger.Info("Starting DocumentGenerationWorkflow", "workflow_id", workflowID, "chat_id", chatID)

	// 3. Start Workflow
	run, err := c.ExecuteWorkflow(context.Background(), options, gotenbergtelegram.DocumentGenerationWorkflow, htmlContent, chatID, "temporal_report.pdf")
	if err != nil {
		logger.Error("Failed to start workflow", "error", err)
		os.Exit(1)
	}

	// 4. Wait for completion
	err = run.Get(context.Background(), nil)
	if err != nil {
		logger.Error("Workflow failed", "error", err)
		os.Exit(1)
	}

	logger.Info("Workflow completed successfully! Document sent to Telegram.")
}
