package telegram

import "encoding/json"

// APIResponse represents the standard response wrapper from the Telegram Bot API.
type APIResponse struct {
	Ok          bool            `json:"ok"`
	Result      json.RawMessage `json:"result,omitempty"`
	Description string          `json:"description,omitempty"`
	ErrorCode   int             `json:"error_code,omitempty"`
}

// User represents a Telegram user or bot.
type User struct {
	ID        int64  `json:"id"`
	IsBot     bool   `json:"is_bot"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name,omitempty"`
	Username  string `json:"username,omitempty"`
}

// Chat represents a Telegram chat.
type Chat struct {
	ID        int64  `json:"id"`
	Type      string `json:"type"` // "private", "group", "supergroup" or "channel"
	Title     string `json:"title,omitempty"`
	Username  string `json:"username,omitempty"`
	FirstName string `json:"first_name,omitempty"`
	LastName  string `json:"last_name,omitempty"`
}

// PhotoSize represents one size of a photo or a file / sticker photo.
type PhotoSize struct {
	FileID       string `json:"file_id"`
	FileUniqueID string `json:"file_unique_id"`
	Width        int    `json:"width"`
	Height       int    `json:"height"`
	FileSize     int64  `json:"file_size,omitempty"`
}

// Document represents a general file (as opposed to photos, voice messages and audio files).
type Document struct {
	FileID       string     `json:"file_id"`
	FileUniqueID string     `json:"file_unique_id"`
	Thumbnail    *PhotoSize `json:"thumbnail,omitempty"`
	FileName     string     `json:"file_name,omitempty"`
	MimeType     string     `json:"mime_type,omitempty"`
	FileSize     int64      `json:"file_size,omitempty"`
}

// Message represents a Telegram message.
type Message struct {
	MessageID  int64       `json:"message_id"`
	From       *User       `json:"from,omitempty"`
	SenderChat *Chat       `json:"sender_chat,omitempty"`
	Date       int64       `json:"date"`
	Chat       Chat        `json:"chat"`
	Text       string      `json:"text,omitempty"`
	Photo      []PhotoSize `json:"photo,omitempty"`
	Document   *Document   `json:"document,omitempty"`
}

// CallbackQuery represents an incoming callback query from a callback button in an inline keyboard.
type CallbackQuery struct {
	ID              string   `json:"id"`
	From            User     `json:"from"`
	Message         *Message `json:"message,omitempty"`
	InlineMessageID string   `json:"inline_message_id,omitempty"`
	ChatInstance    string   `json:"chat_instance"`
	Data            string   `json:"data,omitempty"`
}

// Update represents an incoming update from Telegram.
type Update struct {
	UpdateID      int64          `json:"update_id"`
	Message       *Message       `json:"message,omitempty"`
	CallbackQuery *CallbackQuery `json:"callback_query,omitempty"`
}

// InlineKeyboardButton represents one button of an inline keyboard.
type InlineKeyboardButton struct {
	Text         string  `json:"text"`
	CallbackData *string `json:"callback_data,omitempty"`
	URL          *string `json:"url,omitempty"`
}

// InlineKeyboardMarkup represents an inline keyboard that appears right next to the message it belongs to.
type InlineKeyboardMarkup struct {
	InlineKeyboard [][]InlineKeyboardButton `json:"inline_keyboard"`
}

// KeyboardButton represents one button of the reply keyboard.
type KeyboardButton struct {
	Text string `json:"text"`
}

// ReplyKeyboardMarkup represents a custom keyboard with reply options.
type ReplyKeyboardMarkup struct {
	Keyboard        [][]KeyboardButton `json:"keyboard"`
	ResizeKeyboard  bool               `json:"resize_keyboard,omitempty"`
	OneTimeKeyboard bool               `json:"one_time_keyboard,omitempty"`
}

// ReplyParameters describes reply parameters for the message.
type ReplyParameters struct {
	MessageID int64 `json:"message_id"`
}

// MessageText returns the text of the message if available, otherwise an empty string.
func (u Update) MessageText() string {
	if u.Message == nil {
		return ""
	}
	return u.Message.Text
}

// ChatID returns the chat ID for the update if available, otherwise 0.
func (u Update) ChatID() int64 {
	if u.Message != nil {
		return u.Message.Chat.ID
	}
	if u.CallbackQuery != nil && u.CallbackQuery.Message != nil {
		return u.CallbackQuery.Message.Chat.ID
	}
	return 0
}

// SenderFirstName returns the first name of the sender if available, otherwise an empty string.
func (u Update) SenderFirstName() string {
	if u.Message != nil && u.Message.From != nil {
		return u.Message.From.FirstName
	}
	if u.CallbackQuery != nil {
		return u.CallbackQuery.From.FirstName
	}
	return ""
}

// CallbackData returns the data associated with the callback query if available, otherwise an empty string.
func (u Update) CallbackData() string {
	if u.CallbackQuery == nil {
		return ""
	}
	return u.CallbackQuery.Data
}

// CallbackID returns the callback query ID if available, otherwise an empty string.
func (u Update) CallbackID() string {
	if u.CallbackQuery == nil {
		return ""
	}
	return u.CallbackQuery.ID
}

