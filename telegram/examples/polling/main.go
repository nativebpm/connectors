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
	var offset int64 = 0

	for {
		ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
		updates, err := client.GetUpdates(ctx, &telegram.GetUpdatesParams{
			Offset:  offset,
			Timeout: 20, // 20s Telegram long-polling timeout
		})
		cancel()

		if err != nil {
			log.Printf("Error polling updates: %v. Retrying in 3 seconds...", err)
			time.Sleep(3 * time.Second)
			continue
		}

		for _, update := range updates {
			// Update the offset to acknowledge receipt of this update
			offset = update.UpdateID + 1

			// 1. Handle incoming text message
			if update.Message != nil && update.Message.Text != "" {
				fmt.Printf("Received message from %s (%d): %s\n", 
					update.Message.Chat.FirstName, 
					update.Message.Chat.ID, 
					update.Message.Text,
				)

				// Reply back to user
				replyCtx, replyCancel := context.WithTimeout(context.Background(), 5*time.Second)
				_, err = client.SendMessage(replyCtx, &telegram.SendMessageParams{
					ChatID: update.Message.Chat.ID,
					Text:   fmt.Sprintf("Hello %s! I received your message: \"%s\"", update.Message.Chat.FirstName, update.Message.Text),
				})
				replyCancel()
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
				answerCtx, answerCancel := context.WithTimeout(context.Background(), 5*time.Second)
				_, err = client.AnswerCallbackQuery(answerCtx, &telegram.AnswerCallbackQueryParams{
					CallbackQueryID: update.CallbackQuery.ID,
					Text:            "Action accepted!",
					ShowAlert:       false,
				})
				answerCancel()
				if err != nil {
					log.Printf("Failed to answer callback query: %v", err)
				}
			}
		}
	}
}
