package llm

import (
	"context"
	"testing"
	"time"

	"github.com/ishanwen-byte/openevolve-go/internal/types"
	"github.com/stretchr/testify/assert"
)

func TestNewOpenAIClient(t *testing.T) {
	config := types.LLMModelConfig{
		Name:    "gpt-4",
		APIKey:  "test-key",
		Timeout: 60,
	}

	client := NewOpenAIClient(config)
	assert.NotNil(t, client)
	assert.Equal(t, config.Name, client.config.Name)
	assert.Equal(t, config.APIKey, client.apiKey)
	assert.Equal(t, "https://api.openai.com/v1", client.baseURL)
	assert.Equal(t, 60*time.Second, client.httpClient.Timeout)
}

func TestNewOpenAIClientWithDefaults(t *testing.T) {
	config := types.LLMModelConfig{
		Name:   "gpt-4",
		APIKey: "test-key",
		// Timeout and other fields are zero
	}

	client := NewOpenAIClient(config)
	assert.NotNil(t, client)
	assert.Equal(t, 60*time.Second, client.httpClient.Timeout) // Should default to 60s
}

func TestNewOpenAIClientWithCustomBaseURL(t *testing.T) {
	config := types.LLMModelConfig{
		Name:    "gpt-4",
		APIKey:  "test-key",
		APIBase: "https://custom.api.com/v1",
		Timeout: 30,
	}

	client := NewOpenAIClient(config)
	assert.NotNil(t, client)
	assert.Equal(t, "https://custom.api.com/v1", client.baseURL)
	assert.Equal(t, 30*time.Second, client.httpClient.Timeout)
}

func TestOpenAIClientIsReasoningModel(t *testing.T) {
	testCases := []struct {
		name     string
		model    string
		expected bool
	}{
		{"o1 model", "o1-preview", true},
		{"o1 prefix", "o1-mini", true},
		{"o3 model", "o3-preview", true},
		{"o3 prefix", "o3-mini", true},
		{"gpt-4", "gpt-4", false},
		{"gpt-3.5", "gpt-3.5-turbo", false},
		{"gpt-5", "gpt-5", true},
		{"custom model", "custom-model", false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			config := types.LLMModelConfig{
				Name:    tc.model,
				APIKey:  "test-key",
				APIBase: "https://api.openai.com/v1",
			}
			client := NewOpenAIClient(config)
			assert.Equal(t, tc.expected, client.isReasoningModel())
		})
	}
}

func TestOpenAIClientGenerate(t *testing.T) {
	config := types.LLMModelConfig{
		Name:    "gpt-4",
		APIKey:  "invalid-key",
		Timeout: 1, // Short timeout to fail fast
		Retries: 1, // Only one retry for test
	}

	client := NewOpenAIClient(config)
	ctx := context.Background()

	// This should fail due to invalid API key
	_, err := client.Generate(ctx, "test prompt")
	assert.Error(t, err)
}

func TestOpenAIClientGenerateWithSystemMessage(t *testing.T) {
	config := types.LLMModelConfig{
		Name:    "gpt-4",
		APIKey:  "invalid-key",
		Timeout: 1,
		Retries: 1,
	}

	client := NewOpenAIClient(config)
	ctx := context.Background()
	messages := []types.LLMMessage{
		{Role: "user", Content: "test message"},
	}

	// This should fail due to invalid API key
	_, err := client.GenerateWithSystemMessage(ctx, "test system", messages)
	assert.Error(t, err)
}

func TestOpenAIClientWithReasoningModel(t *testing.T) {
	config := types.LLMModelConfig{
		Name:    "o1-preview",
		APIKey:  "invalid-key",
		Timeout: 1,
		Retries: 1,
	}

	client := NewOpenAIClient(config)
	ctx := context.Background()

	// Test that reasoning models are handled correctly
	assert.True(t, client.isReasoningModel())

	// This should fail due to invalid API key, but test the reasoning model path
	_, err := client.Generate(ctx, "test prompt")
	assert.Error(t, err)
}

func TestOpenAIClientWithCancellation(t *testing.T) {
	config := types.LLMModelConfig{
		Name:    "gpt-4",
		APIKey:  "invalid-key",
		Timeout: 30, // Long timeout
		Retries: 0,  // No retries for faster test
	}

	client := NewOpenAIClient(config)

	// Create a context that will be cancelled quickly
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// This should fail due to context cancellation
	_, err := client.Generate(ctx, "test prompt")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "context deadline exceeded")
}

func TestGetOrDefaultFunctions(t *testing.T) {
	assert.Equal(t, "default", getOrDefault("", "default"))
	assert.Equal(t, "value", getOrDefault("value", "default"))

	assert.Equal(t, 10, getOrDefaultInt(0, 10))
	assert.Equal(t, 5, getOrDefaultInt(5, 10))

	assert.InDelta(t, 1.5, getOrDefaultFloat64(0, 1.5), 0.001)
	assert.InDelta(t, 2.5, getOrDefaultFloat64(2.5, 1.5), 0.001)
}

func TestHTTPError(t *testing.T) {
	err := &HTTPError{
		StatusCode: 401,
		Message:    "Unauthorized",
	}

	expected := "HTTP 401: Unauthorized"
	assert.Equal(t, expected, err.Error())
}

func TestOpenAIClientRequestFormatting(t *testing.T) {
	config := types.LLMModelConfig{
		Name:           "gpt-4",
		APIKey:         "test-key",
		Timeout:        60,
		Temperature:    0.8,
		TopP:           0.9,
		MaxTokens:      2000,
		RandomSeed:     42,
		ReasoningEffort: stringPtr("medium"),
	}

	client := NewOpenAIClient(config)
	assert.False(t, client.isReasoningModel())

	// Test reasoning effort is preserved
	assert.Equal(t, "medium", *client.config.ReasoningEffort)
	assert.Equal(t, 42, client.config.RandomSeed)
}

func TestOpenAIClientReasoningModelRequestFormatting(t *testing.T) {
	config := types.LLMModelConfig{
		Name:           "o1-preview",
		APIKey:         "test-key",
		Timeout:        60,
		Temperature:    0.8, // Should be ignored for reasoning models
		TopP:           0.9,  // Should be ignored for reasoning models
		MaxTokens:      2000,
		ReasoningEffort: stringPtr("high"),
	}

	client := NewOpenAIClient(config)
	assert.True(t, client.isReasoningModel())

	// Verify reasoning effort is preserved
	assert.Equal(t, "high", *client.config.ReasoningEffort)
}

func TestOpenAIClientWithCustomAPIBase(t *testing.T) {
	config := types.LLMModelConfig{
		Name:    "custom-model",
		APIKey:  "test-key",
		APIBase: "https://custom-endpoint.com/v1",
		Timeout: 60,
	}

	client := NewOpenAIClient(config)
	assert.Equal(t, "https://custom-endpoint.com/v1", client.baseURL)
	// Custom models should not be treated as reasoning models
	assert.False(t, client.isReasoningModel())
}

// Helper function to create string pointers
func stringPtr(s string) *string {
	return &s
}