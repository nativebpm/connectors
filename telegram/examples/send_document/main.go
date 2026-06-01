package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/nativebpm/connectors/telegram"
)

func main() {
	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	chatIDStr := os.Getenv("TELEGRAM_CHAT_ID")

	if token == "" || chatIDStr == "" {
		log.Fatal("TELEGRAM_BOT_TOKEN and TELEGRAM_CHAT_ID environment variables must be set")
	}

	chatID, err := strconv.ParseInt(chatIDStr, 10, 64)
	if err != nil {
		log.Fatalf("Invalid TELEGRAM_CHAT_ID: %v", err)
	}

	client, err := telegram.NewClient(token)
	if err != nil {
		log.Fatalf("Failed to initialize client: %v", err)
	}

	// Open the module's README.md to stream it to Telegram
	file, err := os.Open("../../README.md")
	if err != nil {
		log.Fatalf("Failed to open file: %v", err)
	}
	defer file.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	msg, err := client.NewDocument(chatID, file, "telegram-readme.md").
		Caption("Here is the README documentation streamed directly from the disk.").
		Send(ctx)
	if err != nil {
		log.Fatalf("Failed to stream document: %v", err)
	}

	fmt.Printf("Document sent successfully! File ID: %s\n", msg.Document.FileID)
}
