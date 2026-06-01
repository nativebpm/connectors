package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/nativebpm/connectors/telegram"
)

func main() {
	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	if token == "" {
		log.Fatal("TELEGRAM_BOT_TOKEN environment variable must be set")
	}

	client, err := telegram.NewClient(token)
	if err != nil {
		log.Fatalf("Failed to initialize client: %v", err)
	}

	fmt.Println("Starting Telegram Bot updates polling loop...")

	ctx := context.Background()
	err = client.StartPolling(ctx, func(ctx context.Context, update telegram.Update) {
		// 1. Handle incoming text message
		if update.Message != nil && update.Message.Text != "" {
			fmt.Printf("Received message from %s (%d): %s\n", 
				update.Message.Chat.FirstName, 
				update.Message.Chat.ID, 
				update.Message.Text,
			)

			// Reply back to user
			replyCtx, replyCancel := context.WithTimeout(ctx, 5*time.Second)
			defer replyCancel()
			_, err = client.NewMessage(update.Message.Chat.ID, fmt.Sprintf("Hello %s! I received your message: \"%s\"", update.Message.Chat.FirstName, update.Message.Text)).
				Send(replyCtx)
			if err != nil {
				log.Printf("Failed to reply: %v", err)
			}
		}

		// 2. Handle button callback query
		if update.CallbackQuery != nil {
			fmt.Printf("Received callback button click: ID=%s, Data=%s\n", 
				update.CallbackQuery.ID, 
				update.CallbackQuery.Data,
			)

			// Answer the callback query to remove loading spinner on button
			answerCtx, answerCancel := context.WithTimeout(ctx, 5*time.Second)
			defer answerCancel()
			_, err = client.AnswerCallbackQuery(answerCtx, &telegram.AnswerCallbackQueryParams{
				CallbackQueryID: update.CallbackQuery.ID,
				Text:            "Action accepted!",
				ShowAlert:       false,
			})
			if err != nil {
				log.Printf("Failed to answer callback query: %v", err)
			}
		}
	})

	if err != nil && err != context.Canceled {
		log.Fatalf("Polling error: %v", err)
	}
}
