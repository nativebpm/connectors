package ai

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"testing"
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
