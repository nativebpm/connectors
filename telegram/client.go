package telegram

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/nativebpm/httpstream"
)

// Client represents a Telegram Bot API client.
type Client struct {
	httpstream *httpstream.Client
}

// ClientOption defines functional options for Client initialization.
type ClientOption func(*clientConfig)

type clientConfig struct {
	apiURL     string
	httpClient *http.Client
}

// WithAPIURL sets a custom API URL (useful for self-hosted Bot API servers or mock testing).
func WithAPIURL(url string) ClientOption {
	return func(cfg *clientConfig) {
		cfg.apiURL = url
	}
}

// WithHTTPClient sets a custom http.Client for the underlying requests.
func WithHTTPClient(client *http.Client) ClientOption {
	return func(cfg *clientConfig) {
		cfg.httpClient = client
	}
}

// NewClient creates a new Telegram Bot API client.
func NewClient(token string, opts ...ClientOption) (*Client, error) {
	if token == "" {
		return nil, errors.New("bot token cannot be empty")
	}

	cfg := &clientConfig{
		apiURL: "https://api.telegram.org",
	}
	for _, opt := range opts {
		opt(cfg)
	}

	botURL := fmt.Sprintf("%s/bot%s", cfg.apiURL, token)
	streamClient, err := httpstream.NewClient(cfg.httpClient, botURL)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize httpstream client: %w", err)
	}

	return &Client{
		httpstream: streamClient,
	}, nil
}

// Helper generic function to decode API responses and handle Telegram errors.
func decodeResponse[T any](resp *http.Response) (*T, error) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var apiResp APIResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal API response (HTTP %d, body: %s): %w", resp.StatusCode, string(body), err)
	}

	if !apiResp.Ok {
		return nil, fmt.Errorf("telegram API error (code %d): %s", apiResp.ErrorCode, apiResp.Description)
	}

	var result T
	if err := json.Unmarshal(apiResp.Result, &result); err != nil {
		return nil, fmt.Errorf("failed to unmarshal API result: %w", err)
	}

	return &result, nil
}

// Helper function to apply parameters common to all media-sending requests.
func applyCommonMultipartParams(
	req *httpstream.Multipart,
	caption, parseMode string,
	disableNotification bool,
	replyParams *ReplyParameters,
	replyMarkup any,
) error {
	if caption != "" {
		req.Param("caption", caption)
	}
	if parseMode != "" {
		req.Param("parse_mode", parseMode)
	}
	if disableNotification {
		req.Bool("disable_notification", disableNotification)
	}
	if replyParams != nil {
		b, err := json.Marshal(replyParams)
		if err != nil {
			return fmt.Errorf("failed to marshal reply_parameters: %w", err)
		}
		req.Param("reply_parameters", string(b))
	}
	if replyMarkup != nil {
		b, err := json.Marshal(replyMarkup)
		if err != nil {
			return fmt.Errorf("failed to marshal reply_markup: %w", err)
		}
		req.Param("reply_markup", string(b))
	}
	return nil
}
