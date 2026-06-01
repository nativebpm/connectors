# Telegram Bot API Connector for Go

A high-performance, stream-first Go client library for the Telegram Bot API. Built on top of [httpstream](../httpstream), it enables zero-buffer, `O(1)` memory-overhead streaming uploads for media files (documents and photos).

---

## Features

- **Stream-first uploads**: Stream files (documents, images) directly from any `io.Reader` (e.g., S3 streams, file systems, HTTP response bodies) without buffering them in RAM.
- **Structured models**: Native Go structures for common Telegram types (`Message`, `Update`, `User`, `Chat`, `CallbackQuery`, etc.).
- **Interactive keyboards**: Built-in support for `InlineKeyboardMarkup` and `ReplyKeyboardMarkup`.
- **Flexible client configuration**: Easily customize the underlying `http.Client` or the API base URL (useful for local Bot API servers or test mocks).

---

## Installation

Add the package to your module:

```bash
go get github.com/nativebpm/connectors/telegram
```

---

## Usage

### 1. Initializing the Client

By default, the client points to the official Telegram Bot API gateway:

```go
package main

import (
	"log"
	"net/http"
	"time"

	"github.com/nativebpm/connectors/telegram"
)

func main() {
	httpClient := &http.Client{
		Timeout: 30 * time.Second,
	}

	client, err := telegram.NewClient("YOUR_BOT_TOKEN", telegram.WithHTTPClient(httpClient))
	if err != nil {
		log.Fatalf("Failed to initialize client: %v", err)
	}
	
	// Ready to use client
}
```

### 2. Sending Messages (with Markup)

```go
package main

import (
	"context"
	"log"

	"github.com/nativebpm/connectors/telegram"
)

func main() {
	client, _ := telegram.NewClient("YOUR_BOT_TOKEN")

	// Create an inline keyboard
	markup := telegram.NewInlineKeyboard().
		AddButtonRow("Approve Task", "action_approve").
		Build()

	msg, err := client.NewMessage(int64(123456789), "**New BPM Task**: Please review and approve the request.").
		ParseMode("MarkdownV2").
		ReplyMarkup(markup).
		Send(context.Background())
	if err != nil {
		log.Fatalf("Failed to send message: %v", err)
	}

	log.Printf("Sent message ID: %d", msg.MessageID)
}
```

### 3. Streaming File Uploads (Zero-Buffer)

Rather than reading whole files into memory (`[]byte`), you can stream them directly using `io.Reader`:

```go
package main

import (
	"context"
	"log"
	"os"

	"github.com/nativebpm/connectors/telegram"
)

func main() {
	client, _ := telegram.NewClient("YOUR_BOT_TOKEN")

	// Open a file (implements io.Reader)
	file, err := os.Open("invoice.pdf")
	if err != nil {
		log.Fatalf("Failed to open file: %v", err)
	}
	defer file.Close()

	// Send document (streamed directly)
	msg, err := client.NewDocument(int64(123456789), file, "invoice.pdf").
		Caption("Here is your invoice PDF.").
		Send(context.Background())
	if err != nil {
		log.Fatalf("Failed to stream document: %v", err)
	}

	log.Printf("Uploaded file ID: %s", msg.Document.FileID)
}
```

### 4. Fetching Updates (Long-Polling)

Ideal for worker programs or local testing:

```go
package main

import (
	"context"
	"log"

	"github.com/nativebpm/connectors/telegram"
)

func main() {
	client, _ := telegram.NewClient("YOUR_BOT_TOKEN")

	ctx := context.Background()
	err := client.StartPolling(ctx, func(ctx context.Context, update telegram.Update) {
		if update.Message != nil {
			log.Printf("Received message from %s: %s", update.Message.Chat.FirstName, update.Message.Text)
		}
	})
	if err != nil {
		log.Fatalf("Polling failed: %v", err)
	}
}
```
