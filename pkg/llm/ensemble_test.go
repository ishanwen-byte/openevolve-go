package llm

import (
	"context"
	"testing"

	"github.com/ishanwen-byte/openevolve-go/internal/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewEnsemble(t *testing.T) {
	configs := []types.LLMModelConfig{
		{
			Name:    "gpt-4",
			Weight:  0.7,
			APIKey:  "test-key-1",
			Timeout: 60,
		},
		{
			Name:    "gpt-3.5-turbo",
			Weight:  0.3,
			APIKey:  "test-key-2",
			Timeout: 30,
		},
	}

	ensemble, err := NewEnsemble(configs)
	require.NoError(t, err)
	assert.NotNil(t, ensemble)
	assert.Equal(t, 2, len(ensemble.clients))
	assert.Equal(t, 2, len(ensemble.weights))
	assert.InDelta(t, 0.7, ensemble.weights[0], 0.001)
	assert.InDelta(t, 0.3, ensemble.weights[1], 0.001)
}

func TestNewEnsembleWithZeroWeights(t *testing.T) {
	configs := []types.LLMModelConfig{
		{
			Name:    "gpt-4",
			Weight:  0,
			APIKey:  "test-key",
			Timeout: 60,
		},
		{
			Name:    "gpt-3.5-turbo",
			Weight:  0,
			APIKey:  "test-key",
			Timeout: 30,
		},
	}

	ensemble, err := NewEnsemble(configs)
	require.NoError(t, err)
	assert.InDelta(t, 0.5, ensemble.weights[0], 0.001)
	assert.InDelta(t, 0.5, ensemble.weights[1], 0.001)
}

func TestNewEnsembleWithNoConfigs(t *testing.T) {
	ensemble, err := NewEnsemble([]types.LLMModelConfig{})
	assert.Error(t, err)
	assert.Nil(t, ensemble)
	assert.Contains(t, err.Error(), "at least one model configuration is required")
}

func TestNewEnsembleWithRandomSeed(t *testing.T) {
	seed := 42
	configs := []types.LLMModelConfig{
		{
			Name:       "gpt-4",
			Weight:     1.0,
			APIKey:     "test-key",
			Timeout:    60,
			RandomSeed: seed,
		},
	}

	ensemble, err := NewEnsemble(configs)
	require.NoError(t, err)
	assert.NotNil(t, ensemble.rand)

	// Test deterministic behavior by checking if multiple selections are consistent
	// This is a rough test since selection is random, but with fixed seed it should be deterministic
	selectedCounts := make(map[int]int)
	for i := 0; i < 100; i++ {
		client, err := ensemble.selectClient()
		require.NoError(t, err)
		// Find the index of the selected client
		for j, c := range ensemble.clients {
			if c == client {
				selectedCounts[j]++
				break
			}
		}
	}
	// With only one client, only index 0 should be selected
	assert.Equal(t, 100, selectedCounts[0])
}

func TestEnsembleGenerate(t *testing.T) {
	// This test would require mocking HTTP responses
	// For now, we'll test the ensemble logic without actual API calls
	configs := []types.LLMModelConfig{
		{
			Name:    "gpt-4",
			Weight:  1.0,
			APIKey:  "test-key",
			Timeout: 1, // Very short timeout to fail fast
		},
	}

	ensemble, err := NewEnsemble(configs)
	require.NoError(t, err)

	ctx := context.Background()
	// This should fail due to invalid API, but tests the ensemble logic
	_, err = ensemble.Generate(ctx, "test prompt")
	// We expect an error since we're using fake API keys
	assert.Error(t, err)
}

func TestEnsembleGenerateMultiple(t *testing.T) {
	configs := []types.LLMModelConfig{
		{
			Name:    "gpt-4",
			Weight:  1.0,
			APIKey:  "test-key",
			Timeout: 1,
		},
	}

	ensemble, err := NewEnsemble(configs)
	require.NoError(t, err)

	ctx := context.Background()
	_, err = ensemble.GenerateMultiple(ctx, "test prompt", 3)
	// Should fail due to invalid API
	assert.Error(t, err)
}

func TestEnsembleParallelGenerate(t *testing.T) {
	configs := []types.LLMModelConfig{
		{
			Name:    "gpt-4",
			Weight:  1.0,
			APIKey:  "test-key",
			Timeout: 1,
		},
	}

	ensemble, err := NewEnsemble(configs)
	require.NoError(t, err)

	ctx := context.Background()
	prompts := []string{"prompt 1", "prompt 2", "prompt 3"}
	_, err = ensemble.ParallelGenerate(ctx, prompts)
	// Should fail due to invalid API
	assert.Error(t, err)
}

func TestEnsembleGetStats(t *testing.T) {
	configs := []types.LLMModelConfig{
		{
			Name:    "gpt-4",
			Weight:  0.7,
			APIKey:  "test-key",
			Timeout: 60,
		},
		{
			Name:    "gpt-3.5-turbo",
			Weight:  0.3,
			APIKey:  "test-key",
			Timeout: 30,
		},
	}

	ensemble, err := NewEnsemble(configs)
	require.NoError(t, err)

	stats := ensemble.GetStats()
	assert.Equal(t, 2, stats["num_clients"])
	assert.InDelta(t, 1.0, stats["total_weight"], 0.001)

	weights, ok := stats["weights"].([]float64)
	require.True(t, ok)
	assert.Equal(t, 2, len(weights))
	assert.InDelta(t, 0.7, weights[0], 0.001)
	assert.InDelta(t, 0.3, weights[1], 0.001)
}

func TestEnsembleSelectClient(t *testing.T) {
	configs := []types.LLMModelConfig{
		{
			Name:    "gpt-4",
			Weight:  0.8,
			APIKey:  "test-key",
			Timeout: 60,
		},
		{
			Name:    "gpt-3.5-turbo",
			Weight:  0.2,
			APIKey:  "test-key",
			Timeout: 30,
		},
	}

	ensemble, err := NewEnsemble(configs)
	require.NoError(t, err)

	// Test multiple selections to see distribution
	selectedCounts := make(map[int]int)
	for i := 0; i < 1000; i++ {
		client, err := ensemble.selectClient()
		require.NoError(t, err)

		// Find the index of the selected client
		for j, c := range ensemble.clients {
			if c == client {
				selectedCounts[j]++
				break
			}
		}
	}

	// The first client should be selected more often due to higher weight
	assert.True(t, selectedCounts[0] > selectedCounts[1])
	// Rough check: should be approximately 80% for first client, 20% for second
	ratio := float64(selectedCounts[0]) / float64(selectedCounts[1])
	assert.InDelta(t, 4.0, ratio, 1.0) // Allow some variance
}

func TestEnsembleGenerateWithSystemMessage(t *testing.T) {
	configs := []types.LLMModelConfig{
		{
			Name:    "gpt-4",
			Weight:  1.0,
			APIKey:  "test-key",
			Timeout: 1,
		},
	}

	ensemble, err := NewEnsemble(configs)
	require.NoError(t, err)

	ctx := context.Background()
	messages := []types.LLMMessage{
		{Role: "user", Content: "test message"},
	}
	_, err = ensemble.GenerateWithSystemMessage(ctx, "test system", messages)
	// Should fail due to invalid API
	assert.Error(t, err)
}

func TestEnsembleGenerateAll(t *testing.T) {
	configs := []types.LLMModelConfig{
		{
			Name:    "gpt-4",
			Weight:  0.5,
			APIKey:  "test-key",
			Timeout: 1,
		},
		{
			Name:    "gpt-3.5-turbo",
			Weight:  0.5,
			APIKey:  "test-key",
			Timeout: 1,
		},
	}

	ensemble, err := NewEnsemble(configs)
	require.NoError(t, err)

	ctx := context.Background()
	messages := []types.LLMMessage{
		{Role: "user", Content: "test message"},
	}
	responses, err := ensemble.GenerateAll(ctx, "test system", messages)
	// Should fail due to invalid API, but return partial results
	assert.Error(t, err)
	assert.Equal(t, 2, len(responses)) // One response per client
}