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

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	callbackData := "task_approve_123"
	markup := telegram.InlineKeyboardMarkup{
		InlineKeyboard: [][]telegram.InlineKeyboardButton{
			{
				{
					Text:         "Approve Document 📄",
					CallbackData: &callbackData,
				},
			},
		},
	}

	msg, err := client.SendMessage(ctx, &telegram.SendMessageParams{
		ChatID:    chatID,
		Text:      "🔔 *New BPM Task*:\n\nA new purchase request needs your approval.",
		ParseMode: "Markdown",
		ReplyMarkup: markup,
	})
	if err != nil {
		log.Fatalf("Failed to send message: %v", err)
	}

	fmt.Printf("Successfully sent message! Message ID: %d\n", msg.MessageID)
}
