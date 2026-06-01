package telegram

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"
)

// SendMessageParams represents parameters for SendMessage.
type SendMessageParams struct {
	ChatID                any              `json:"chat_id"` // int64 or string (username/channel)
	Text                  string           `json:"text"`
	ParseMode             string           `json:"parse_mode,omitempty"`
	DisableWebPagePreview bool             `json:"disable_web_page_preview,omitempty"`
	DisableNotification   bool             `json:"disable_notification,omitempty"`
	ReplyParameters       *ReplyParameters `json:"reply_parameters,omitempty"`
	ReplyMarkup           any              `json:"reply_markup,omitempty"` // InlineKeyboardMarkup, ReplyKeyboardMarkup, etc.
}

// SendMessage sends a text message.
func (c *Client) SendMessage(ctx context.Context, params *SendMessageParams) (*Message, error) {
	if params == nil {
		return nil, errors.New("params cannot be nil")
	}

	resp, err := c.httpstream.POST(ctx, "/sendMessage").JSON(params).Send()
	if err != nil {
		return nil, fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	return decodeResponse[Message](resp)
}

// SendDocumentParams represents parameters for SendDocument.
type SendDocumentParams struct {
	ChatID              any              // int64 or string
	Document            io.Reader        // streamed file
	Filename            string           // filename for multipart form
	Caption             string           // optional caption
	ParseMode           string           // optional parse mode for caption
	DisableNotification bool             // optional
	ReplyParameters     *ReplyParameters // optional
	ReplyMarkup         any              // optional
}

// SendDocument sends a general file. The file is streamed without complete buffering in memory.
func (c *Client) SendDocument(ctx context.Context, params *SendDocumentParams) (*Message, error) {
	if params == nil {
		return nil, errors.New("params cannot be nil")
	}
	if params.Document == nil {
		return nil, errors.New("document reader cannot be nil")
	}

	req := c.httpstream.Multipart(ctx, "/sendDocument").
		Param("chat_id", fmt.Sprint(params.ChatID)).
		File("document", params.Filename, params.Document)

	if err := applyCommonMultipartParams(req, params.Caption, params.ParseMode, params.DisableNotification, params.ReplyParameters, params.ReplyMarkup); err != nil {
		return nil, err
	}

	resp, err := req.Send()
	if err != nil {
		return nil, fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	return decodeResponse[Message](resp)
}

// SendPhotoParams represents parameters for SendPhoto.
type SendPhotoParams struct {
	ChatID              any              // int64 or string
	Photo               io.Reader        // streamed image
	Filename            string           // filename for multipart form (e.g. photo.jpg)
	Caption             string           // optional caption
	ParseMode           string           // optional parse mode for caption
	DisableNotification bool             // optional
	ReplyParameters     *ReplyParameters // optional
	ReplyMarkup         any              // optional
}

// SendPhoto sends a photo. The image is streamed without complete buffering in memory.
func (c *Client) SendPhoto(ctx context.Context, params *SendPhotoParams) (*Message, error) {
	if params == nil {
		return nil, errors.New("params cannot be nil")
	}
	if params.Photo == nil {
		return nil, errors.New("photo reader cannot be nil")
	}

	req := c.httpstream.Multipart(ctx, "/sendPhoto").
		Param("chat_id", fmt.Sprint(params.ChatID)).
		File("photo", params.Filename, params.Photo)

	if err := applyCommonMultipartParams(req, params.Caption, params.ParseMode, params.DisableNotification, params.ReplyParameters, params.ReplyMarkup); err != nil {
		return nil, err
	}

	resp, err := req.Send()
	if err != nil {
		return nil, fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	return decodeResponse[Message](resp)
}

// AnswerCallbackQueryParams represents parameters for AnswerCallbackQuery.
type AnswerCallbackQueryParams struct {
	CallbackQueryID string `json:"callback_query_id"`
	Text            string `json:"text,omitempty"`
	ShowAlert       bool   `json:"show_alert,omitempty"`
	URL             string `json:"url,omitempty"`
	CacheTime       int    `json:"cache_time,omitempty"`
}

// AnswerCallbackQuery sends a response to an interactive callback query.
func (c *Client) AnswerCallbackQuery(ctx context.Context, params *AnswerCallbackQueryParams) (bool, error) {
	if params == nil {
		return false, errors.New("params cannot be nil")
	}

	resp, err := c.httpstream.POST(ctx, "/answerCallbackQuery").JSON(params).Send()
	if err != nil {
		return false, fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	res, err := decodeResponse[bool](resp)
	if err != nil {
		return false, err
	}
	return *res, nil
}

// SetWebhookParams represents parameters for SetWebhook.
type SetWebhookParams struct {
	URL            string   `json:"url"`
	IPAddress      string   `json:"ip_address,omitempty"`
	MaxConnections int      `json:"max_connections,omitempty"`
	AllowedUpdates []string `json:"allowed_updates,omitempty"`
}

// SetWebhook registers a webhook URL for receiving updates.
func (c *Client) SetWebhook(ctx context.Context, params *SetWebhookParams) (bool, error) {
	if params == nil {
		return false, errors.New("params cannot be nil")
	}

	resp, err := c.httpstream.POST(ctx, "/setWebhook").JSON(params).Send()
	if err != nil {
		return false, fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	res, err := decodeResponse[bool](resp)
	if err != nil {
		return false, err
	}
	return *res, nil
}

// GetUpdatesParams represents parameters for GetUpdates.
type GetUpdatesParams struct {
	Offset         int64    `json:"offset,omitempty"`
	Limit          int      `json:"limit,omitempty"`
	Timeout        int      `json:"timeout,omitempty"`
	AllowedUpdates []string `json:"allowed_updates,omitempty"`
}

// GetUpdates fetches updates using long-polling.
func (c *Client) GetUpdates(ctx context.Context, params *GetUpdatesParams) ([]Update, error) {
	if params == nil {
		params = &GetUpdatesParams{}
	}

	resp, err := c.httpstream.POST(ctx, "/getUpdates").JSON(params).Send()
	if err != nil {
		return nil, fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	res, err := decodeResponse[[]Update](resp)
	if err != nil {
		return nil, err
	}
	if res == nil {
		return nil, nil
	}
	return *res, nil
}

// StartPolling starts a polling loop to receive updates. It blocks until the context is cancelled.
func (c *Client) StartPolling(ctx context.Context, handler func(ctx context.Context, update Update)) error {
	var offset int64 = 0
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// Use a timeout slightly longer than the long-polling timeout (20s)
		pollCtx, pollCancel := context.WithTimeout(ctx, 30*time.Second)
		updates, err := c.GetUpdates(pollCtx, &GetUpdatesParams{
			Offset:  offset,
			Timeout: 20,
		})
		pollCancel()

		if err != nil {
			if errors.Is(err, context.Canceled) {
				return ctx.Err()
			}
			// Wait a bit before retrying to prevent hot looping on errors
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(3 * time.Second):
			}
			continue
		}

		for _, update := range updates {
			offset = update.UpdateID + 1
			handler(ctx, update)
		}
	}
}
