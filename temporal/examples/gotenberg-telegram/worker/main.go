package main

import (
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/joho/godotenv"
	"github.com/nativebpm/connectors/gotenberg"
	"github.com/nativebpm/connectors/telegram"
	"github.com/nativebpm/connectors/temporal"
	gotenbergtelegram "github.com/nativebpm/connectors/temporal/examples/gotenberg-telegram"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	// Load env file if available
	_ = godotenv.Load("temporal.env")

	// Get configuration
	gotenbergURL := os.Getenv("GOTENBERG_URL")
	if gotenbergURL == "" {
		gotenbergURL = "http://localhost:3000"
	}
	telegramToken := os.Getenv("TELEGRAM_BOT_TOKEN")
	if telegramToken == "" {
		logger.Error("TELEGRAM_BOT_TOKEN environment variable is not set")
		os.Exit(1)
	}

	// 1. Initialize Gotenberg Client
	httpClient := &http.Client{Timeout: 30 * time.Second}
	gotenbergClient, err := gotenberg.NewClient(httpClient, gotenbergURL)
	if err != nil {
		logger.Error("Failed to initialize Gotenberg client", "error", err)
		os.Exit(1)
	}

	// 2. Initialize Telegram Client
	telegramClient, err := telegram.NewClient(telegramToken)
	if err != nil {
		logger.Error("Failed to initialize Telegram client", "error", err)
		os.Exit(1)
	}

	// 3. Initialize Temporal Client
	cfg := temporal.LoadFromEnv()
	temporalClient, err := temporal.NewClient(cfg)
	if err != nil {
		logger.Error("Failed to initialize Temporal client", "error", err)
		os.Exit(1)
	}
	defer temporalClient.Close()

	// 4. Initialize Worker
	w := temporal.NewWorker(temporalClient, cfg.TaskQueue)

	// 5. Register Workflow and Activities
	w.RegisterWorkflow(gotenbergtelegram.DocumentGenerationWorkflow)
	
	activities := gotenbergtelegram.NewActivities(gotenbergClient, telegramClient)
	w.RegisterActivity(activities.ConvertHTMLToPDF)
	w.RegisterActivity(activities.SendTelegramDocument)

	logger.Info("Starting Gotenberg-Telegram Integration Worker", "queue", cfg.TaskQueue)

	// 6. Run Worker
	err = w.Run(nil)
	if err != nil {
		logger.Error("Worker crash", "error", err)
		os.Exit(1)
	}
}
