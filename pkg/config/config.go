package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ishanwen-byte/openevolve-go/internal/constants"
	"github.com/ishanwen-byte/openevolve-go/internal/types"
	"gopkg.in/yaml.v3"
)

// Manager handles configuration loading and validation
type Manager struct {
	config *types.Config
	path   string
}

// NewManager creates a new configuration manager
func NewManager() *Manager {
	return &Manager{
		config: getDefaultConfig(),
	}
}

// Load loads configuration from a file
func (m *Manager) Load(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	config := getDefaultConfig()
	if err := yaml.Unmarshal(data, config); err != nil {
		return fmt.Errorf("failed to parse config file: %w", err)
	}

	// Apply environment variable overrides
	if err := m.applyEnvOverrides(config); err != nil {
		return fmt.Errorf("failed to apply environment overrides: %w", err)
	}

	// Validate configuration
	if err := m.validate(config); err != nil {
		return fmt.Errorf("configuration validation failed: %w", err)
	}

	m.config = config
	m.path = path
	return nil
}

// Save saves configuration to a file
func (m *Manager) Save(path string) error {
	data, err := yaml.Marshal(m.config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// GetConfig returns the current configuration
func (m *Manager) GetConfig() *types.Config {
	return m.config
}

// SetConfig updates the configuration
func (m *Manager) SetConfig(config *types.Config) {
	m.config = config
}

// GetPath returns the configuration file path
func (m *Manager) GetPath() string {
	return m.path
}

// applyEnvOverrides applies environment variable overrides to the configuration
func (m *Manager) applyEnvOverrides(config *types.Config) error {
	// LLM configuration overrides
	if apiBase := os.Getenv("OPENAI_API_BASE"); apiBase != "" {
		config.LLM.APIBase = apiBase
	}
	if apiKey := os.Getenv("OPENAI_API_KEY"); apiKey != "" {
		config.LLM.APIKey = apiKey
	}
	if model := os.Getenv("OPENAI_MODEL"); model != "" {
		if len(config.LLM.Models) == 0 {
			config.LLM.Models = append(config.LLM.Models, types.LLMModelConfig{
				Name:   model,
				Weight: 1.0,
			})
		} else {
			config.LLM.Models[0].Name = model
		}
	}

	// Database configuration overrides
	if numIslands := os.Getenv("NUM_ISLANDS"); numIslands != "" {
		var n int
		if _, err := fmt.Sscanf(numIslands, "%d", &n); err == nil {
			config.Database.NumIslands = n
		}
	}
	if outputDir := os.Getenv("OUTPUT_DIR"); outputDir != "" {
		config.Database.OutputDir = outputDir
	}

	// Controller configuration overrides
	if maxIter := os.Getenv("MAX_ITERATIONS"); maxIter != "" {
		var n int
		if _, err := fmt.Sscanf(maxIter, "%d", &n); err == nil {
			config.Controller.MaxIterations = n
		}
	}
	if seed := os.Getenv("SEED"); seed != "" {
		var n int
		if _, err := fmt.Sscanf(seed, "%d", &n); err == nil {
			config.Controller.Seed = n
		}
	}
	if verbose := os.Getenv("VERBOSE"); verbose != "" {
		config.Controller.Verbose = strings.ToLower(verbose) == "true"
	}

	return nil
}

// validate validates the configuration
func (m *Manager) validate(config *types.Config) error {
	// Validate LLM configuration
	if config.LLM.APIBase == "" {
		return fmt.Errorf("LLM API base is required")
	}
	if len(config.LLM.Models) == 0 && config.LLM.APIKey == "" {
		return fmt.Errorf("at least one LLM model or API key is required")
	}

	// Validate model weights sum to 1.0
	var totalWeight float64
	for _, model := range config.LLM.Models {
		totalWeight += model.Weight
	}
	if len(config.LLM.Models) > 0 && totalWeight <= 0 {
		return fmt.Errorf("sum of model weights must be positive")
	}

	// Validate database configuration
	if config.Database.NumIslands <= 0 {
		return fmt.Errorf("number of islands must be positive")
	}
	if len(config.Database.GridDimensions) == 0 {
		return fmt.Errorf("grid dimensions are required")
	}
	if len(config.Database.GridResolution) != len(config.Database.GridDimensions) {
		return fmt.Errorf("grid resolution must match dimensions")
	}

	// Validate evaluator configuration
	if config.Evaluator.ParallelWorkers <= 0 {
		return fmt.Errorf("parallel workers must be positive")
	}
	if len(config.Evaluator.CascadeStages) == 0 {
		return fmt.Errorf("at least one cascade stage is required")
	}

	// Validate controller configuration
	if config.Controller.MaxIterations <= 0 {
		return fmt.Errorf("max iterations must be positive")
	}
	if config.Controller.ParallelWorkers <= 0 {
		return fmt.Errorf("parallel workers must be positive")
	}

	// Validate paths
	if config.Database.OutputDir == "" {
		config.Database.OutputDir = constants.OutputDir
	}
	if config.Controller.CheckpointDir == "" {
		config.Controller.CheckpointDir = filepath.Join(config.Database.OutputDir, constants.CheckpointDir)
	}

	return nil
}

// getDefaultConfig returns the default configuration
func getDefaultConfig() *types.Config {
	return &types.Config{
		LLM: types.LLMConfig{
			APIBase:       constants.DefaultOpenAIBase,
			Models: []types.LLMModelConfig{
				{
					Name:       constants.GPT4,
					Weight:     1.0,
					Timeout:    constants.DefaultTimeout,
					Retries:    constants.DefaultRetries,
					RetryDelay: constants.DefaultRetryDelay,
				},
			},
			EvaluatorModels: []types.LLMModelConfig{},
			SystemMessage:   constants.DefaultSystemMessage,
			Temperature:     constants.DefaultTemperature,
			TopP:            constants.DefaultTopP,
			MaxTokens:       constants.DefaultMaxTokens,
			Timeout:         constants.DefaultTimeout,
			Retries:         constants.DefaultRetries,
			RetryDelay:      constants.DefaultRetryDelay,
			RandomSeed:      42,
		},
		Database: types.DatabaseConfig{
			NumIslands:        constants.DefaultNumIslands,
			GridDimensions:    []string{"complexity", "novelty"},
			GridResolution:    map[string]int{"complexity": 10, "novelty": 10},
			GridBounds:        map[string][2]float64{"complexity": {0, 1}, "novelty": {0, 1}},
			MigrationInterval: constants.DefaultMigrationInterval,
			MigrationRate:     constants.DefaultMigrationRate,
			MaxProgramsPerCell: constants.DefaultMaxProgramsPerCell,
			CheckpointInterval: constants.DefaultCheckpointInterval,
			OutputDir:         constants.OutputDir,
		},
		Evaluator: types.EvaluatorConfig{
			CascadeStages: []types.CascadeStage{
				{
					Name:      constants.EvalStageValidation,
					Threshold: 0.0,
					Timeout:   10,
					Critical:  true,
				},
				{
					Name:      constants.EvalStageBasic,
					Threshold: 0.5,
					Timeout:   30,
					Critical:  false,
				},
				{
					Name:      constants.EvalStageComprehensive,
					Threshold: 0.8,
					Timeout:   120,
					Critical:  false,
				},
			},
			ParallelWorkers:   constants.DefaultParallelWorkers,
			Timeout:           constants.DefaultTimeout,
			CollectArtifacts:  true,
			ArtifactMaxSize:   constants.DefaultArtifactMaxSize,
		},
		Prompt: types.PromptConfig{
			Templates:       []types.PromptTemplate{},
			SystemMessage:   constants.DefaultSystemMessage,
			EvolutionPrompt: constants.DefaultEvolutionPrompt,
			MutationPrompt:  constants.DefaultMutationPrompt,
			Stochasticity:   constants.DefaultStochasticity,
			IncludeHistory:  true,
			HistoryLength:   constants.DefaultHistoryLength,
		},
		Controller: types.ControllerConfig{
			MaxIterations:   constants.DefaultMaxIterations,
			MaxGenerations:  constants.DefaultMaxGenerations,
			ParallelWorkers: constants.DefaultParallelWorkers,
			CheckpointDir:   filepath.Join(constants.OutputDir, constants.CheckpointDir),
			Seed:            42,
			Verbose:         false,
		},
	}
}

// CreateDefaultConfig creates a default configuration file
func CreateDefaultConfig(path string) error {
	manager := NewManager()
	return manager.Save(path)
}