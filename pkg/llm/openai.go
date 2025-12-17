package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/ishanwen-byte/openevolve-go/internal/types"
)

// OpenAIClient implements an LLM client for OpenAI-compatible APIs
type OpenAIClient struct {
	config      types.LLMModelConfig
	httpClient  *http.Client
	baseURL     string
	apiKey      string
}

// NewOpenAIClient creates a new OpenAI-compatible LLM client
func NewOpenAIClient(config types.LLMModelConfig) *OpenAIClient {
	timeout := time.Duration(config.Timeout) * time.Second
	if timeout == 0 {
		timeout = 60 * time.Second
	}

	return &OpenAIClient{
		config: config,
		httpClient: &http.Client{
			Timeout: timeout,
		},
		baseURL: getOrDefault(config.APIBase, "https://api.openai.com/v1"),
		apiKey:  config.APIKey,
	}
}

// Generate generates text from a prompt
func (c *OpenAIClient) Generate(ctx context.Context, prompt string) (*types.LLMResponse, error) {
	messages := []types.LLMMessage{
		{Role: "user", Content: prompt},
	}

	systemMessage := getOrDefault(c.config.SystemMessage, "You are an expert programmer helping to evolve and improve code.")

	return c.GenerateWithSystemMessage(ctx, systemMessage, messages)
}

// GenerateWithSystemMessage generates text using a system message and conversational context
func (c *OpenAIClient) GenerateWithSystemMessage(ctx context.Context, systemMessage string, messages []types.LLMMessage) (*types.LLMResponse, error) {
	// Prepare messages with system message first
	allMessages := make([]types.LLMMessage, 0, len(messages)+1)
	allMessages = append(allMessages, types.LLMMessage{Role: "system", Content: systemMessage})
	allMessages = append(allMessages, messages...)

	// Prepare request body
	request := types.LLMRequest{
		Model:       c.config.Name,
		Messages:    allMessages,
		Temperature: getOrDefaultFloat64(c.config.Temperature, 0.7),
		TopP:        getOrDefaultFloat64(c.config.TopP, 0.95),
		MaxTokens:   getOrDefaultInt(c.config.MaxTokens, 4096),
		Timeout:     time.Duration(getOrDefaultInt(c.config.Timeout, 60)) * time.Second,
	}

	// Handle reasoning models (o1, o3 series)
	if c.isReasoningModel() {
		// Reasoning models don't support temperature/top_p
		request.Temperature = 1
		request.TopP = 1
	}

	startTime := time.Now()

	// Retry logic
	maxRetries := getOrDefaultInt(c.config.Retries, 3)
	retryDelay := time.Duration(getOrDefaultInt(c.config.RetryDelay, 5)) * time.Second

	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			// Add jitter to retry delay
			jitter := time.Duration(float64(retryDelay) * (0.5 + 0.5*float64(attempt%2)))
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(jitter):
			}
		}

		response, err := c.makeRequest(ctx, request)
		if err == nil {
			response.Duration = time.Since(startTime)
			return response, nil
		}

		lastErr = err

		// Don't retry on context cancellation
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		// Don't retry on certain HTTP status codes
		if httpErr, ok := err.(*HTTPError); ok {
			if httpErr.StatusCode == 400 || httpErr.StatusCode == 401 || httpErr.StatusCode == 403 {
				return nil, err
			}
		}
	}

	return nil, fmt.Errorf("max retries exceeded: %w", lastErr)
}

// makeRequest makes an HTTP request to the LLM API
func (c *OpenAIClient) makeRequest(ctx context.Context, request types.LLMRequest) (*types.LLMResponse, error) {
	// Prepare request body
	var body bytes.Buffer
	encoder := json.NewEncoder(&body)

	// Create request map to handle different model types
	requestMap := map[string]interface{}{
		"model":    request.Model,
		"messages": request.Messages,
	}

	// Add parameters based on model type
	if c.isReasoningModel() {
		// For reasoning models, use max_completion_tokens instead of max_tokens
		requestMap["max_completion_tokens"] = request.MaxTokens
	} else {
		requestMap["max_tokens"] = request.MaxTokens
		requestMap["temperature"] = request.Temperature
		requestMap["top_p"] = request.TopP
	}

	// Add reasoning effort if specified
	if c.config.ReasoningEffort != nil {
		requestMap["reasoning_effort"] = *c.config.ReasoningEffort
	}

	// Add seed for reproducibility if specified
	if c.config.RandomSeed > 0 {
		requestMap["seed"] = c.config.RandomSeed
	}

	if err := encoder.Encode(requestMap); err != nil {
		return nil, fmt.Errorf("failed to encode request: %w", err)
	}

	// Create HTTP request
	url := fmt.Sprintf("%s/chat/completions", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, "POST", url, &body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiKey))
	req.Header.Set("User-Agent", "OpenEvolve-Go/1.0")

	// Make request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Check for errors
	if resp.StatusCode != http.StatusOK {
		return nil, &HTTPError{
			StatusCode: resp.StatusCode,
			Message:    string(respBody),
		}
	}

	// Parse response
	var openAIResponse OpenAIResponse
	if err := json.Unmarshal(respBody, &openAIResponse); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Convert to internal response format
	if len(openAIResponse.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	return &types.LLMResponse{
		Content: openAIResponse.Choices[0].Message.Content,
		Model:   openAIResponse.Model,
		Usage: types.TokenUsage{
			PromptTokens:     openAIResponse.Usage.PromptTokens,
			CompletionTokens: openAIResponse.Usage.CompletionTokens,
			TotalTokens:      openAIResponse.Usage.TotalTokens,
		},
	}, nil
}

// isReasoningModel checks if the model is a reasoning model (o1, o3 series)
func (c *OpenAIClient) isReasoningModel() bool {
	model := strings.ToLower(c.config.Name)
	reasoningPrefixes := []string{
		"o1-",
		"o1",
		"o3-",
		"o3",
		"o4-",
		"gpt-5-",
		"gpt-5",
		"gpt-oss-120b",
		"gpt-oss-20b",
	}

	for _, prefix := range reasoningPrefixes {
		if strings.HasPrefix(model, prefix) {
			return true
		}
	}

	// Also check if using official OpenAI API
	apiBase := strings.ToLower(c.baseURL)
	isOfficialAPI := strings.Contains(apiBase, "api.openai.com")

	return isOfficialAPI && (strings.HasPrefix(model, "o1") || strings.HasPrefix(model, "o3") || strings.HasPrefix(model, "o4"))
}

// OpenAIResponse represents the OpenAI API response structure
type OpenAIResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index   int `json:"index"`
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

// HTTPError represents an HTTP error from the API
type HTTPError struct {
	StatusCode int
	Message    string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("HTTP %d: %s", e.StatusCode, e.Message)
}

// Helper functions
func getOrDefault(value, defaultValue string) string {
	if value == "" {
		return defaultValue
	}
	return value
}

func getOrDefaultInt(value, defaultValue int) int {
	if value == 0 {
		return defaultValue
	}
	return value
}

func getOrDefaultFloat64(value, defaultValue float64) float64 {
	if value == 0 {
		return defaultValue
	}
	return value
}