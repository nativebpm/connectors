package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"google.golang.org/genai"
)

// GenerateRequest defines inputs for the AI provider.
type GenerateRequest struct {
	Model             string  `json:"model"`
	Prompt            string  `json:"prompt"`
	SystemInstruction string  `json:"system_instruction,omitempty"`
	ResponseSchema    string  `json:"response_schema,omitempty"` // JSON Schema string
	Temperature       float64 `json:"temperature,omitempty"`
}

// GenerateResponse holds response text and token metrics.
type GenerateResponse struct {
	Text             string `json:"text"`
	PromptTokens     int    `json:"prompt_tokens"`
	CompletionTokens int    `json:"completion_tokens"`
}

// AIProvider defines the unified interface for interacting with LLMs.
type AIProvider interface {
	Generate(ctx context.Context, req *GenerateRequest) (*GenerateResponse, error)
}

// ==========================================
// Gemini Provider Implementation
// ==========================================

type GeminiProvider struct {
	client *genai.Client
}

func NewGeminiProvider(ctx context.Context, apiKey string) (*GeminiProvider, error) {
	var client *genai.Client
	var err error
	if apiKey != "" {
		client, err = genai.NewClient(ctx, &genai.ClientConfig{APIKey: apiKey})
	} else {
		// Reads GEMINI_API_KEY from environment automatically
		client, err = genai.NewClient(ctx, nil)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to create Gemini GenAI client: %w", err)
	}
	return &GeminiProvider{client: client}, nil
}

func (g *GeminiProvider) Generate(ctx context.Context, req *GenerateRequest) (*GenerateResponse, error) {
	config := &genai.GenerateContentConfig{
		Temperature: &req.Temperature,
	}

	if req.SystemInstruction != "" {
		config.SystemInstruction = &genai.Content{
			Parts: []*genai.Part{{Text: req.SystemInstruction}},
		}
	}

	if req.ResponseSchema != "" {
		config.ResponseMIMEType = "application/json"
		var schema genai.Schema
		if err := json.Unmarshal([]byte(req.ResponseSchema), &schema); err == nil {
			config.ResponseSchema = &schema
		}
	}

	resp, err := g.client.Models.GenerateContent(ctx, req.Model, genai.Text(req.Prompt), config)
	if err != nil {
		return nil, fmt.Errorf("gemini generate content failed: %w", err)
	}

	var text string
	if len(resp.Candidates) > 0 && resp.Candidates[0].Content != nil && len(resp.Candidates[0].Content.Parts) > 0 {
		text = resp.Candidates[0].Content.Parts[0].Text
	}

	var promptTokens, completionTokens int
	if resp.UsageMetadata != nil {
		if resp.UsageMetadata.PromptTokenCount != nil {
			promptTokens = int(*resp.UsageMetadata.PromptTokenCount)
		}
		if resp.UsageMetadata.CandidatesTokenCount != nil {
			completionTokens = int(*resp.UsageMetadata.CandidatesTokenCount)
		}
	}

	return &GenerateResponse{
		Text:             text,
		PromptTokens:     promptTokens,
		CompletionTokens: completionTokens,
	}, nil
}

// ==========================================
// OpenAI-Compatible Direct HTTP Provider (Ollama, DeepSeek, OpenAI)
// ==========================================

type OpenAIProvider struct {
	apiKey  string
	baseURL string
	client  *http.Client
}

func NewOpenAIProvider(apiKey string) (*OpenAIProvider, error) {
	if apiKey == "" {
		apiKey = os.Getenv("OPENAI_API_KEY")
	}
	if apiKey == "" {
		apiKey = "noop"
	}

	baseURL := os.Getenv("OPENAI_BASE_URL")
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}

	return &OpenAIProvider{
		apiKey:  apiKey,
		baseURL: baseURL,
		client:  &http.Client{Timeout: 60 * time.Second},
	}, nil
}

type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIResponseFormat struct {
	Type string `json:"type"`
}

type openAIRequest struct {
	Model          string                `json:"model"`
	Messages       []openAIMessage       `json:"messages"`
	Temperature    float64               `json:"temperature,omitempty"`
	ResponseFormat *openAIResponseFormat `json:"response_format,omitempty"`
}

type openAIChoice struct {
	Message openAIMessage `json:"message"`
}

type openAIUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
}

type openAIResponse struct {
	Choices []openAIChoice `json:"choices"`
	Usage   openAIUsage    `json:"usage"`
	Error   *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

func (o *OpenAIProvider) Generate(ctx context.Context, req *GenerateRequest) (*GenerateResponse, error) {
	messages := make([]openAIMessage, 0)

	if req.SystemInstruction != "" {
		messages = append(messages, openAIMessage{
			Role:    "system",
			Content: req.SystemInstruction,
		})
	}

	messages = append(messages, openAIMessage{
		Role:    "user",
		Content: req.Prompt,
	})

	apiReq := openAIRequest{
		Model:       req.Model,
		Messages:    messages,
		Temperature: req.Temperature,
	}

	if req.ResponseSchema != "" {
		apiReq.ResponseFormat = &openAIResponseFormat{
			Type: "json_object",
		}
	}

	reqBytes, err := json.Marshal(apiReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal openai request: %w", err)
	}

	url := fmt.Sprintf("%s/chat/completions", o.baseURL)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(reqBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create http request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if o.apiKey != "noop" {
		httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", o.apiKey))
	}

	httpResp, err := o.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("http request failed: %w", err)
	}
	defer httpResp.Body.Close()

	bodyBytes, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if httpResp.StatusCode != http.StatusOK {
		var errResp openAIResponse
		_ = json.Unmarshal(bodyBytes, &errResp)
		if errResp.Error != nil {
			return nil, fmt.Errorf("openai api error (status %d): %s", httpResp.StatusCode, errResp.Error.Message)
		}
		return nil, fmt.Errorf("openai api returned status %d: %s", httpResp.StatusCode, string(bodyBytes))
	}

	var apiResp openAIResponse
	if err := json.Unmarshal(bodyBytes, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if len(apiResp.Choices) == 0 {
		return nil, fmt.Errorf("openai returned 0 choices")
	}

	return &GenerateResponse{
		Text:             apiResp.Choices[0].Message.Content,
		PromptTokens:     apiResp.Usage.PromptTokens,
		CompletionTokens: apiResp.Usage.CompletionTokens,
	}, nil
}
