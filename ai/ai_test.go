package ai

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	"google.golang.org/genai"
)

type mockTransport struct {
	roundTrip func(req *http.Request) (*http.Response, error)
}

func (m *mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return m.roundTrip(req)
}

func TestOpenAIChatSession(t *testing.T) {
	provider := &OpenAIProvider{
		apiKey:  "test-key",
		baseURL: "https://api.openai.com/v1",
		client: &http.Client{
			Transport: &mockTransport{
				roundTrip: func(req *http.Request) (*http.Response, error) {
					respJSON := `{
						"choices": [{
							"message": {
								"role": "assistant",
								"content": "Hello world"
							}
						}],
						"usage": {
							"prompt_tokens": 10,
							"completion_tokens": 5
						}
					}`
					return &http.Response{
						StatusCode: http.StatusOK,
						Body:       io.NopCloser(bytes.NewReader([]byte(respJSON))),
						Header:     make(http.Header),
					}, nil
				},
			},
		},
	}

	session, err := provider.NewChat(context.Background(), &ChatConfig{
		Model: "gpt-4o",
	})
	if err != nil {
		t.Fatalf("failed to create chat session: %v", err)
	}

	resp, err := session.SendMessage(context.Background(), "hi")
	if err != nil {
		t.Fatalf("failed to send message: %v", err)
	}

	if resp.Text != "Hello world" {
		t.Errorf("expected Hello world, got %q", resp.Text)
	}
	if resp.PromptTokens != 10 || resp.CompletionTokens != 5 {
		t.Errorf("unexpected tokens: %+v", resp)
	}
}

func TestGeminiChatSessionMock(t *testing.T) {
	mockHTTPClient := &http.Client{
		Transport: &mockTransport{
			roundTrip: func(req *http.Request) (*http.Response, error) {
				respJSON := `{
					"candidates": [{
						"content": {
							"parts": [{
								"text": "Hello from Gemini mock!"
							}]
						}
					}],
					"usageMetadata": {
						"promptTokenCount": 15,
						"candidatesTokenCount": 10
					}
				}`
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(bytes.NewReader([]byte(respJSON))),
					Header:     make(http.Header),
				}, nil
			},
		},
	}

	ctx := context.Background()
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:     "test-key",
		HTTPClient: mockHTTPClient,
	})
	if err != nil {
		t.Fatalf("failed to create genai client: %v", err)
	}

	provider := &GeminiProvider{client: client}
	session, err := provider.NewChat(ctx, &ChatConfig{
		Model: "gemini-2.5-flash",
	})
	if err != nil {
		t.Fatalf("failed to create gemini session: %v", err)
	}

	resp, err := session.SendMessage(ctx, "hi")
	if err != nil {
		t.Fatalf("failed to send message: %v", err)
	}

	if resp.Text != "Hello from Gemini mock!" {
		t.Errorf("expected Hello from Gemini mock!, got %q", resp.Text)
	}
	if resp.PromptTokens != 15 || resp.CompletionTokens != 10 {
		t.Errorf("unexpected tokens: %+v", resp)
	}
}

func TestGeminiChatSessionLive(t *testing.T) {
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		t.Skip("skipping live test: GEMINI_API_KEY is not set")
		return
	}

	ctx := context.Background()
	provider, err := NewGeminiProvider(ctx, apiKey)
	if err != nil {
		t.Skipf("skipping live test: failed to create Gemini provider: %v", err)
	}

	session, err := provider.NewChat(ctx, &ChatConfig{
		Model: "gemini-2.5-flash",
	})
	if err != nil {
		t.Skipf("skipping live test: failed to start chat: %v", err)
	}

	resp, err := session.SendMessage(ctx, "Hello, respond with exactly the word 'OK'.")
	if err != nil {
		t.Skipf("skipping live test: SendMessage failed: %v", err)
	}

	t.Logf("Live Gemini response: %q (prompt tokens: %d, completion tokens: %d)", resp.Text, resp.PromptTokens, resp.CompletionTokens)
	if !strings.Contains(strings.ToUpper(resp.Text), "OK") {
		t.Errorf("expected response to contain 'OK', got %q", resp.Text)
	}
}

func BenchmarkOpenAIChatSession(b *testing.B) {
	provider := &OpenAIProvider{
		apiKey:  "test-key",
		baseURL: "https://api.openai.com/v1",
		client: &http.Client{
			Transport: &mockTransport{
				roundTrip: func(req *http.Request) (*http.Response, error) {
					respJSON := `{
						"choices": [{
							"message": {
								"role": "assistant",
								"content": "Hello world"
							}
						}],
						"usage": {
							"prompt_tokens": 10,
							"completion_tokens": 5
						}
					}`
					return &http.Response{
						StatusCode: http.StatusOK,
						Body:       io.NopCloser(bytes.NewReader([]byte(respJSON))),
						Header:     make(http.Header),
					}, nil
				},
			},
		},
	}

	ctx := context.Background()
	config := &ChatConfig{
		Model:             "gpt-4o",
		SystemInstruction: "System prompt instructions.",
		Tools: []ToolDefinition{
			{
				Name:        "getWeather",
				Description: "Gets weather",
				Parameters: map[string]interface{}{
					"type": "object",
				},
			},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		session, _ := provider.NewChat(ctx, config)
		_, _ = session.SendMessage(ctx, "hello")
	}
}
