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

// ToolDefinition defines a function tool that the LLM agent can call.
type ToolDefinition struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"` // JSON Schema of parameters
}

// ToolCall represents a tool call requested by the LLM.
type ToolCall struct {
	ID        string                 `json:"id"`
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments"`
}

// GenerateResponse holds the text response, tool calls, and token metrics.
type GenerateResponse struct {
	Text             string     `json:"text"`
	ToolCalls        []ToolCall `json:"tool_calls,omitempty"`
	PromptTokens     int        `json:"prompt_tokens"`
	CompletionTokens int        `json:"completion_tokens"`
}

// ChatConfig defines the options for starting a new chat session.
type ChatConfig struct {
	Model             string           `json:"model"`
	SystemInstruction string           `json:"system_instruction,omitempty"`
	Tools             []ToolDefinition `json:"tools,omitempty"`
	Temperature       float64          `json:"temperature,omitempty"`
	ResponseSchema    string           `json:"response_schema,omitempty"` // JSON Schema string
}

type ToolResult struct {
	ID     string      `json:"id"`
	Name   string      `json:"name"`
	Result interface{} `json:"result"`
}

// ChatSession maintains conversation history and supports multi-turn tool calling.
type ChatSession interface {
	SendMessage(ctx context.Context, text string) (*GenerateResponse, error)
	SendToolResults(ctx context.Context, results []ToolResult) (*GenerateResponse, error)
}

// AIProvider defines the unified interface for interacting with LLM providers.
type AIProvider interface {
	NewChat(ctx context.Context, config *ChatConfig) (ChatSession, error)
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
		client, err = genai.NewClient(ctx, nil)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to create Gemini GenAI client: %w", err)
	}
	return &GeminiProvider{client: client}, nil
}

type geminiChatSession struct {
	chat *genai.Chat
}

func (g *GeminiProvider) NewChat(ctx context.Context, config *ChatConfig) (ChatSession, error) {
	temp32 := float32(config.Temperature)
	genaiConfig := &genai.GenerateContentConfig{
		Temperature: &temp32,
	}

	if config.SystemInstruction != "" {
		genaiConfig.SystemInstruction = &genai.Content{
			Parts: []*genai.Part{{Text: config.SystemInstruction}},
		}
	}

	if config.ResponseSchema != "" {
		genaiConfig.ResponseMIMEType = "application/json"
		var schema genai.Schema
		if err := json.Unmarshal([]byte(config.ResponseSchema), &schema); err == nil {
			genaiConfig.ResponseSchema = &schema
		}
	}

	if len(config.Tools) > 0 {
		var declarations []*genai.FunctionDeclaration
		for _, tool := range config.Tools {
			var genaiSchema genai.Schema
			paramBytes, err := json.Marshal(tool.Parameters)
			if err == nil {
				_ = json.Unmarshal(paramBytes, &genaiSchema)
			}

			declarations = append(declarations, &genai.FunctionDeclaration{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  &genaiSchema,
			})
		}
		genaiConfig.Tools = []*genai.Tool{
			{FunctionDeclarations: declarations},
		}
	}

	chat, err := g.client.Chats.Create(ctx, config.Model, genaiConfig, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create gemini chat session: %w", err)
	}
	return &geminiChatSession{chat: chat}, nil
}

func (s *geminiChatSession) SendMessage(ctx context.Context, text string) (*GenerateResponse, error) {
	resp, err := s.chat.SendMessage(ctx, genai.Part{Text: text})
	if err != nil {
		return nil, fmt.Errorf("gemini send message failed: %w", err)
	}
	return parseGeminiResponse(resp)
}

func (s *geminiChatSession) SendToolResults(ctx context.Context, results []ToolResult) (*GenerateResponse, error) {
	var parts []genai.Part
	for _, res := range results {
		var resultMap map[string]interface{}
		resultBytes, err := json.Marshal(res.Result)
		if err == nil {
			_ = json.Unmarshal(resultBytes, &resultMap)
		}
		parts = append(parts, genai.Part{
			FunctionResponse: &genai.FunctionResponse{
				Name:     res.Name,
				Response: resultMap,
				ID:       res.ID,
			},
		})
	}

	resp, err := s.chat.SendMessage(ctx, parts...)
	if err != nil {
		return nil, fmt.Errorf("gemini send tool results failed: %w", err)
	}
	return parseGeminiResponse(resp)
}

func parseGeminiResponse(resp *genai.GenerateContentResponse) (*GenerateResponse, error) {
	var text string
	var toolCalls []ToolCall

	if len(resp.Candidates) > 0 && resp.Candidates[0].Content != nil {
		for _, part := range resp.Candidates[0].Content.Parts {
			if part.Text != "" {
				text += part.Text
			}
			if part.FunctionCall != nil {
				toolCalls = append(toolCalls, ToolCall{
					ID:        part.FunctionCall.ID,
					Name:      part.FunctionCall.Name,
					Arguments: part.FunctionCall.Args,
				})
			}
		}
	}

	var promptTokens, completionTokens int
	if resp.UsageMetadata != nil {
		promptTokens = int(resp.UsageMetadata.PromptTokenCount)
		completionTokens = int(resp.UsageMetadata.CandidatesTokenCount)
	}

	return &GenerateResponse{
		Text:             text,
		ToolCalls:        toolCalls,
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
	Role       string            `json:"role"`
	Content    string            `json:"content,omitempty"`
	Name       string            `json:"name,omitempty"`
	ToolCallID string            `json:"tool_call_id,omitempty"`
	ToolCalls  []openAIToolCall  `json:"tool_calls,omitempty"`
}

type openAIToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

type openAIFunctionDefinition struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Parameters  map[string]interface{} `json:"parameters,omitempty"`
}

type openAIToolDefinition struct {
	Type     string                   `json:"type"`
	Function openAIFunctionDefinition `json:"function"`
}

type openAIResponseFormat struct {
	Type string `json:"type"`
}

type openAIRequest struct {
	Model          string                 `json:"model"`
	Messages       []openAIMessage        `json:"messages"`
	Tools          []openAIToolDefinition `json:"tools,omitempty"`
	Temperature    float64                `json:"temperature,omitempty"`
	ResponseFormat *openAIResponseFormat  `json:"response_format,omitempty"`
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

type openAIChatSession struct {
	provider       *OpenAIProvider
	model          string
	temperature    float64
	responseSchema string
	tools          []openAIToolDefinition
	history        []openAIMessage
}

func (o *OpenAIProvider) NewChat(ctx context.Context, config *ChatConfig) (ChatSession, error) {
	var history []openAIMessage
	if config.SystemInstruction != "" {
		history = append(history, openAIMessage{
			Role:    "system",
			Content: config.SystemInstruction,
		})
	}

	var openAITools []openAIToolDefinition
	for _, tool := range config.Tools {
		openAITools = append(openAITools, openAIToolDefinition{
			Type: "function",
			Function: openAIFunctionDefinition{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  tool.Parameters,
			},
		})
	}

	return &openAIChatSession{
		provider:       o,
		model:          config.Model,
		temperature:    config.Temperature,
		responseSchema: config.ResponseSchema,
		tools:          openAITools,
		history:        history,
	}, nil
}

func (s *openAIChatSession) SendMessage(ctx context.Context, text string) (*GenerateResponse, error) {
	s.history = append(s.history, openAIMessage{
		Role:    "user",
		Content: text,
	})
	return s.executeRequest(ctx)
}

func (s *openAIChatSession) SendToolResults(ctx context.Context, results []ToolResult) (*GenerateResponse, error) {
	for _, res := range results {
		resultBytes, _ := json.Marshal(res.Result)
		s.history = append(s.history, openAIMessage{
			Role:       "tool",
			Name:       res.Name,
			ToolCallID: res.ID,
			Content:    string(resultBytes),
		})
	}
	return s.executeRequest(ctx)
}

func (s *openAIChatSession) executeRequest(ctx context.Context) (*GenerateResponse, error) {
	apiReq := openAIRequest{
		Model:       s.model,
		Messages:    s.history,
		Temperature: s.temperature,
		Tools:       s.tools,
	}

	if s.responseSchema != "" {
		apiReq.ResponseFormat = &openAIResponseFormat{
			Type: "json_object",
		}
	}

	reqBytes, err := json.Marshal(apiReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal openai request: %w", err)
	}

	url := fmt.Sprintf("%s/chat/completions", s.provider.baseURL)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(reqBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create http request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	if s.provider.apiKey != "noop" {
		httpReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.provider.apiKey))
	}

	httpResp, err := s.provider.client.Do(httpReq)
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

	assistantMessage := apiResp.Choices[0].Message
	s.history = append(s.history, assistantMessage)

	var toolCalls []ToolCall
	for _, tc := range assistantMessage.ToolCalls {
		var args map[string]interface{}
		_ = json.Unmarshal([]byte(tc.Function.Arguments), &args)

		toolCalls = append(toolCalls, ToolCall{
			ID:        tc.ID,
			Name:      tc.Function.Name,
			Arguments: args,
		})
	}

	return &GenerateResponse{
		Text:             assistantMessage.Content,
		ToolCalls:        toolCalls,
		PromptTokens:     apiResp.Usage.PromptTokens,
		CompletionTokens: apiResp.Usage.CompletionTokens,
	}, nil
}
