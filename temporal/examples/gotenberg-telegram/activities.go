package gotenbergtelegram

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/nativebpm/connectors/gotenberg"
	"github.com/nativebpm/connectors/telegram"
)

// Activities definition
type Activities struct {
	gotenbergClient *gotenberg.Client
	telegramClient  *telegram.Client
}

// NewActivities initializes Activities struct with respective clients
func NewActivities(gotenbergClient *gotenberg.Client, telegramClient *telegram.Client) *Activities {
	return &Activities{
		gotenbergClient: gotenbergClient,
		telegramClient:  telegramClient,
	}
}

// ConvertHTMLToPDF converts HTML string content into PDF bytes using Gotenberg Chromium
func (a *Activities) ConvertHTMLToPDF(ctx context.Context, htmlContent string) ([]byte, error) {
	if htmlContent == "" {
		return nil, fmt.Errorf("html content cannot be empty")
	}

	reader := strings.NewReader(htmlContent)
	resp, err := a.gotenbergClient.Chromium().ConvertHTML(ctx, reader).Send()
	if err != nil {
		return nil, fmt.Errorf("gotenberg conversion failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("gotenberg returned status %d: %s", resp.StatusCode, string(body))
	}

	pdfBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read PDF response body: %w", err)
	}

	return pdfBytes, nil
}

// SendTelegramDocument uploads the generated PDF bytes to Telegram
func (a *Activities) SendTelegramDocument(ctx context.Context, chatID int64, pdfBytes []byte, filename string) error {
	if len(pdfBytes) == 0 {
		return fmt.Errorf("pdf bytes cannot be empty")
	}
	if filename == "" {
		filename = "document.pdf"
	}

	pdfReader := bytes.NewReader(pdfBytes)
	_, err := a.telegramClient.NewDocument(chatID, pdfReader, filename).
		Caption("Ваш документ успешно сгенерирован и отправлен нативной оркестрацией Temporal!").
		Send(ctx)
	if err != nil {
		return fmt.Errorf("failed to send Telegram document: %w", err)
	}

	return nil
}
