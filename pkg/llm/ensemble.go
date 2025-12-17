package llm

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"sync"
	"time"

	"github.com/ishanwen-byte/openevolve-go/internal/types"
)

// Client interface defines the common interface for LLM clients
type Client interface {
	Generate(ctx context.Context, prompt string) (*types.LLMResponse, error)
	GenerateWithSystemMessage(ctx context.Context, systemMessage string, messages []types.LLMMessage) (*types.LLMResponse, error)
}

// Ensemble implements an ensemble of LLM clients with weighted selection
type Ensemble struct {
	clients   []Client
	weights   []float64
	totalWeight float64
	rand      *rand.Rand
	mu        sync.RWMutex
}

// NewEnsemble creates a new LLM ensemble from the given configuration
func NewEnsemble(configs []types.LLMModelConfig) (*Ensemble, error) {
	if len(configs) == 0 {
		return nil, fmt.Errorf("at least one model configuration is required")
	}

	ensemble := &Ensemble{
		clients: make([]Client, 0, len(configs)),
		weights: make([]float64, len(configs)),
	}

	// Initialize clients and normalize weights
	var totalWeight float64
	for i, cfg := range configs {
		// Create client based on configuration
		client, err := createClient(cfg)
		if err != nil {
			return nil, fmt.Errorf("failed to create client for model %s: %w", cfg.Name, err)
		}

		ensemble.clients = append(ensemble.clients, client)
		ensemble.weights[i] = cfg.Weight
		totalWeight += cfg.Weight
	}

	// Normalize weights
	if totalWeight > 0 {
		for i := range ensemble.weights {
			ensemble.weights[i] /= totalWeight
		}
		ensemble.totalWeight = totalWeight
	} else {
		// If all weights are zero, use equal weights
		equalWeight := 1.0 / float64(len(configs))
		for i := range ensemble.weights {
			ensemble.weights[i] = equalWeight
		}
		ensemble.totalWeight = 1.0
	}

	// Initialize random state
	seed := time.Now().UnixNano()
	if len(configs) > 0 && configs[0].RandomSeed > 0 {
		seed = int64(configs[0].RandomSeed)
	}
	ensemble.rand = rand.New(rand.NewSource(seed))

	// Log ensemble configuration
	log.Printf("Initialized LLM ensemble with %d models:", len(ensemble.clients))
	for i, cfg := range configs {
		log.Printf("  - %s (weight: %.2f)", cfg.Name, ensemble.weights[i])
	}

	return ensemble, nil
}

// Generate generates text using a randomly selected model based on weights
func (e *Ensemble) Generate(ctx context.Context, prompt string) (*types.LLMResponse, error) {
	client, err := e.selectClient()
	if err != nil {
		return nil, err
	}

	response, err := client.Generate(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("generation failed: %w", err)
	}

	// Add ensemble metadata
	response.Model = fmt.Sprintf("ensemble[%s]", response.Model)
	return response, nil
}

// GenerateWithSystemMessage generates text using a system message and conversational context
func (e *Ensemble) GenerateWithSystemMessage(ctx context.Context, systemMessage string, messages []types.LLMMessage) (*types.LLMResponse, error) {
	client, err := e.selectClient()
	if err != nil {
		return nil, err
	}

	response, err := client.GenerateWithSystemMessage(ctx, systemMessage, messages)
	if err != nil {
		return nil, fmt.Errorf("generation with context failed: %w", err)
	}

	// Add ensemble metadata
	response.Model = fmt.Sprintf("ensemble[%s]", response.Model)
	return response, nil
}

// GenerateMultiple generates multiple texts in parallel
func (e *Ensemble) GenerateMultiple(ctx context.Context, prompt string, n int) ([]*types.LLMResponse, error) {
	responses := make([]*types.LLMResponse, n)
	errors := make([]error, n)

	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			response, err := e.Generate(ctx, prompt)
			responses[index] = response
			errors[index] = err
		}(i)
	}
	wg.Wait()

	// Check for errors
	for _, err := range errors {
		if err != nil {
			return nil, err
		}
	}

	return responses, nil
}

// ParallelGenerate generates responses for multiple prompts in parallel
func (e *Ensemble) ParallelGenerate(ctx context.Context, prompts []string) ([]*types.LLMResponse, error) {
	responses := make([]*types.LLMResponse, len(prompts))
	errors := make([]error, len(prompts))

	var wg sync.WaitGroup
	for i, prompt := range prompts {
		wg.Add(1)
		go func(index int, p string) {
			defer wg.Done()
			response, err := e.Generate(ctx, p)
			responses[index] = response
			errors[index] = err
		}(i, prompt)
	}
	wg.Wait()

	// Check for errors
	for _, err := range errors {
		if err != nil {
			return nil, err
		}
	}

	return responses, nil
}

// GenerateAll generates text using all available models
func (e *Ensemble) GenerateAll(ctx context.Context, systemMessage string, messages []types.LLMMessage) ([]*types.LLMResponse, error) {
	e.mu.RLock()
	clients := make([]Client, len(e.clients))
	copy(clients, e.clients)
	e.mu.RUnlock()

	responses := make([]*types.LLMResponse, len(clients))
	errors := make([]error, len(clients))

	var wg sync.WaitGroup
	for i, client := range clients {
		wg.Add(1)
		go func(index int, c Client) {
			defer wg.Done()
			response, err := c.GenerateWithSystemMessage(ctx, systemMessage, messages)
			responses[index] = response
			errors[index] = err
		}(i, client)
	}
	wg.Wait()

	// Check for errors
	for _, err := range errors {
		if err != nil {
			return responses, err // Return partial results with error
		}
	}

	return responses, nil
}

// selectClient selects a client based on weights
func (e *Ensemble) selectClient() (Client, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if len(e.clients) == 0 {
		return nil, fmt.Errorf("no clients available in ensemble")
	}

	// Use weighted random selection
	r := e.rand.Float64()
	cumulative := 0.0

	for i, weight := range e.weights {
		cumulative += weight
		if r <= cumulative {
			log.Printf("Selected model with index %d and weight %.2f", i, weight)
			return e.clients[i], nil
		}
	}

	// Fallback to last client (shouldn't happen if weights sum to 1.0)
	return e.clients[len(e.clients)-1], nil
}

// createClient creates an LLM client based on the configuration
func createClient(cfg types.LLMModelConfig) (Client, error) {
	// Set defaults if not provided
	if cfg.Name == "" {
		cfg.Name = "gpt-4"
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 60
	}
	if cfg.Retries == 0 {
		cfg.Retries = 3
	}
	if cfg.RetryDelay == 0 {
		cfg.RetryDelay = 5
	}

	// For now, only support OpenAI-compatible clients
	// Can be extended to support other providers in the future
	return NewOpenAIClient(cfg), nil
}

// GetStats returns statistics about the ensemble
func (e *Ensemble) GetStats() map[string]interface{} {
	e.mu.RLock()
	defer e.mu.RUnlock()

	return map[string]interface{}{
		"num_clients":   len(e.clients),
		"total_weight":  e.totalWeight,
		"weights":       e.weights,
	}
}