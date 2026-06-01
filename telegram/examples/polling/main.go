package main

import (
	"context"
	"fmt"
	"log"
	"os"

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
		if text := update.MessageText(); text != "" {
			fmt.Printf("Received message from %s (%d): %s\n", 
				update.SenderFirstName(), 
				update.ChatID(), 
				text,
			)

			// Reply back to user using parent context directly
			_, err = client.NewMessage(update.ChatID(), fmt.Sprintf("Hello %s! I received your message: \"%s\"", update.SenderFirstName(), text)).
				Send(ctx)
			if err != nil {
				log.Printf("Failed to reply: %v", err)
			}
		}

		// 2. Handle button callback query
		if data := update.CallbackData(); data != "" {
			fmt.Printf("Received callback button click: ID=%s, Data=%s\n", 
				update.CallbackID(), 
				data,
			)

			// Answer the callback query to remove loading spinner on button
			_, err = client.AnswerCallbackQuery(ctx, &telegram.AnswerCallbackQueryParams{
				CallbackQueryID: update.CallbackID(),
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
