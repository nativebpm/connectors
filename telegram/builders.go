package telegram

import (
	"context"
	"io"
)

// MessageBuilder provides a fluent interface for building and sending text messages.
type MessageBuilder struct {
	client *Client
	params SendMessageParams
}

// NewMessage initializes a fluent builder for sending a text message.
func (c *Client) NewMessage(chatID any, text string) *MessageBuilder {
	return &MessageBuilder{
		client: c,
		params: SendMessageParams{
			ChatID: chatID,
			Text:   text,
		},
	}
}

// ParseMode sets the message parsing mode (e.g., "HTML", "Markdown", "MarkdownV2").
func (b *MessageBuilder) ParseMode(mode string) *MessageBuilder {
	b.params.ParseMode = mode
	return b
}

// DisableWebPagePreview disables the preview of links in the message.
func (b *MessageBuilder) DisableWebPagePreview(disable bool) *MessageBuilder {
	b.params.DisableWebPagePreview = disable
	return b
}

// DisableNotification sends the message silently without sound alerts.
func (b *MessageBuilder) DisableNotification(disable bool) *MessageBuilder {
	b.params.DisableNotification = disable
	return b
}

// ReplyTo sets the ID of the message to reply to.
func (b *MessageBuilder) ReplyTo(messageID int64) *MessageBuilder {
	b.params.ReplyParameters = &ReplyParameters{MessageID: messageID}
	return b
}

// ReplyMarkup attaches an inline keyboard or reply keyboard.
func (b *MessageBuilder) ReplyMarkup(markup any) *MessageBuilder {
	b.params.ReplyMarkup = markup
	return b
}

// Send executes the request and returns the sent Message.
func (b *MessageBuilder) Send(ctx context.Context) (*Message, error) {
	return b.client.SendMessage(ctx, &b.params)
}

// DocumentBuilder provides a fluent interface for building and streaming document uploads.
type DocumentBuilder struct {
	client *Client
	params SendDocumentParams
}

// NewDocument initializes a fluent builder for sending a document stream.
func (c *Client) NewDocument(chatID any, document io.Reader, filename string) *DocumentBuilder {
	return &DocumentBuilder{
		client: c,
		params: SendDocumentParams{
			ChatID:   chatID,
			Document: document,
			Filename: filename,
		},
	}
}

// Caption sets the document caption text.
func (b *DocumentBuilder) Caption(caption string) *DocumentBuilder {
	b.params.Caption = caption
	return b
}

// ParseMode sets the parse mode for the document caption.
func (b *DocumentBuilder) ParseMode(mode string) *DocumentBuilder {
	b.params.ParseMode = mode
	return b
}

// DisableNotification sends the document silently.
func (b *DocumentBuilder) DisableNotification(disable bool) *DocumentBuilder {
	b.params.DisableNotification = disable
	return b
}

// ReplyTo sets the ID of the message to reply to.
func (b *DocumentBuilder) ReplyTo(messageID int64) *DocumentBuilder {
	b.params.ReplyParameters = &ReplyParameters{MessageID: messageID}
	return b
}

// ReplyMarkup attaches a keyboard layout to the document.
func (b *DocumentBuilder) ReplyMarkup(markup any) *DocumentBuilder {
	b.params.ReplyMarkup = markup
	return b
}

// Send executes the streaming request and returns the sent Message.
func (b *DocumentBuilder) Send(ctx context.Context) (*Message, error) {
	return b.client.SendDocument(ctx, &b.params)
}

// PhotoBuilder provides a fluent interface for building and streaming photo uploads.
type PhotoBuilder struct {
	client *Client
	params SendPhotoParams
}

// NewPhoto initializes a fluent builder for sending a photo stream.
func (c *Client) NewPhoto(chatID any, photo io.Reader, filename string) *PhotoBuilder {
	return &PhotoBuilder{
		client: c,
		params: SendPhotoParams{
			ChatID:   chatID,
			Photo:    photo,
			Filename: filename,
		},
	}
}

// Caption sets the photo caption text.
func (b *PhotoBuilder) Caption(caption string) *PhotoBuilder {
	b.params.Caption = caption
	return b
}

// ParseMode sets the parse mode for the photo caption.
func (b *PhotoBuilder) ParseMode(mode string) *PhotoBuilder {
	b.params.ParseMode = mode
	return b
}

// DisableNotification sends the photo silently.
func (b *PhotoBuilder) DisableNotification(disable bool) *PhotoBuilder {
	b.params.DisableNotification = disable
	return b
}

// ReplyTo sets the ID of the message to reply to.
func (b *PhotoBuilder) ReplyTo(messageID int64) *PhotoBuilder {
	b.params.ReplyParameters = &ReplyParameters{MessageID: messageID}
	return b
}

// ReplyMarkup attaches a keyboard layout to the photo.
func (b *PhotoBuilder) ReplyMarkup(markup any) *PhotoBuilder {
	b.params.ReplyMarkup = markup
	return b
}

// Send executes the streaming request and returns the sent Message.
func (b *PhotoBuilder) Send(ctx context.Context) (*Message, error) {
	return b.client.SendPhoto(ctx, &b.params)
}

// InlineKeyboardBuilder provides a fluent builder for InlineKeyboardMarkup.
type InlineKeyboardBuilder struct {
	rows [][]InlineKeyboardButton
}

// NewInlineKeyboard initializes a fluent inline keyboard builder.
func NewInlineKeyboard() *InlineKeyboardBuilder {
	return &InlineKeyboardBuilder{
		rows: make([][]InlineKeyboardButton, 0),
	}
}

// AddButton adds an inline button to the last row, or creates a new row if empty.
func (b *InlineKeyboardBuilder) AddButton(text, callbackData string) *InlineKeyboardBuilder {
	btn := InlineKeyboardButton{
		Text:         text,
		CallbackData: &callbackData,
	}
	if len(b.rows) == 0 {
		b.rows = append(b.rows, []InlineKeyboardButton{btn})
	} else {
		lastIdx := len(b.rows) - 1
		b.rows[lastIdx] = append(b.rows[lastIdx], btn)
	}
	return b
}

// AddURLButton adds a URL inline button to the last row, or creates a new row if empty.
func (b *InlineKeyboardBuilder) AddURLButton(text, url string) *InlineKeyboardBuilder {
	btn := InlineKeyboardButton{
		Text: text,
		URL:  &url,
	}
	if len(b.rows) == 0 {
		b.rows = append(b.rows, []InlineKeyboardButton{btn})
	} else {
		lastIdx := len(b.rows) - 1
		b.rows[lastIdx] = append(b.rows[lastIdx], btn)
	}
	return b
}

// AddButtonRow adds a new row containing a single callback button.
func (b *InlineKeyboardBuilder) AddButtonRow(text, callbackData string) *InlineKeyboardBuilder {
	btn := InlineKeyboardButton{
		Text:         text,
		CallbackData: &callbackData,
	}
	b.rows = append(b.rows, []InlineKeyboardButton{btn})
	return b
}

// AddURLButtonRow adds a new row containing a single URL button.
func (b *InlineKeyboardBuilder) AddURLButtonRow(text, url string) *InlineKeyboardBuilder {
	btn := InlineKeyboardButton{
		Text: text,
		URL:  &url,
	}
	b.rows = append(b.rows, []InlineKeyboardButton{btn})
	return b
}

// Build constructs and returns the InlineKeyboardMarkup structure.
func (b *InlineKeyboardBuilder) Build() InlineKeyboardMarkup {
	return InlineKeyboardMarkup{
		InlineKeyboard: b.rows,
	}
}

// ReplyKeyboardBuilder provides a fluent builder for ReplyKeyboardMarkup.
type ReplyKeyboardBuilder struct {
	rows            [][]KeyboardButton
	resizeKeyboard  bool
	oneTimeKeyboard bool
}

// NewReplyKeyboard initializes a fluent reply keyboard builder.
func NewReplyKeyboard() *ReplyKeyboardBuilder {
	return &ReplyKeyboardBuilder{
		rows: make([][]KeyboardButton, 0),
	}
}

// AddButton adds a keyboard button to the last row, or creates a new row if empty.
func (b *ReplyKeyboardBuilder) AddButton(text string) *ReplyKeyboardBuilder {
	btn := KeyboardButton{Text: text}
	if len(b.rows) == 0 {
		b.rows = append(b.rows, []KeyboardButton{btn})
	} else {
		lastIdx := len(b.rows) - 1
		b.rows[lastIdx] = append(b.rows[lastIdx], btn)
	}
	return b
}

// AddButtonRow adds a new row containing a single reply keyboard button.
func (b *ReplyKeyboardBuilder) AddButtonRow(text string) *ReplyKeyboardBuilder {
	btn := KeyboardButton{Text: text}
	b.rows = append(b.rows, []KeyboardButton{btn})
	return b
}

// ResizeKeyboard requests clients to resize the keyboard vertically for optimal fit.
func (b *ReplyKeyboardBuilder) ResizeKeyboard(resize bool) *ReplyKeyboardBuilder {
	b.resizeKeyboard = resize
	return b
}

// OneTimeKeyboard requests clients to hide the keyboard as soon as it's been used.
func (b *ReplyKeyboardBuilder) OneTimeKeyboard(oneTime bool) *ReplyKeyboardBuilder {
	b.oneTimeKeyboard = oneTime
	return b
}

// Build constructs and returns the ReplyKeyboardMarkup structure.
func (b *ReplyKeyboardBuilder) Build() ReplyKeyboardMarkup {
	return ReplyKeyboardMarkup{
		Keyboard:        b.rows,
		ResizeKeyboard:  b.resizeKeyboard,
		OneTimeKeyboard: b.oneTimeKeyboard,
	}
}
