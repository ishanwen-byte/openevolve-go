package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewManager(t *testing.T) {
	manager := NewManager()
	assert.NotNil(t, manager)
	assert.NotNil(t, manager.config)
	assert.Empty(t, manager.path)
}

func TestLoadAndSave(t *testing.T) {
	// Store original environment variables
	originalVars := map[string]string{
		"OPENAI_API_KEY":  os.Getenv("OPENAI_API_KEY"),
		"OPENAI_MODEL":    os.Getenv("OPENAI_MODEL"),
		"OPENAI_API_BASE": os.Getenv("OPENAI_API_BASE"),
	}

	// Clear environment variables for test
	os.Unsetenv("OPENAI_API_KEY")
	os.Unsetenv("OPENAI_MODEL")
	os.Unsetenv("OPENAI_API_BASE")

	defer func() {
		// Restore original environment variables
		for k, v := range originalVars {
			if v != "" {
				os.Setenv(k, v)
			} else {
				os.Unsetenv(k)
			}
		}
	}()

	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "config_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	configPath := filepath.Join(tempDir, "config.yaml")

	// Test saving default config
	manager := NewManager()
	err = manager.Save(configPath)
	require.NoError(t, err)

	// Verify file was created
	_, err = os.Stat(configPath)
	require.NoError(t, err)

	// Test loading config
	newManager := NewManager()
	err = newManager.Load(configPath)
	require.NoError(t, err)

	// Compare configs
	assert.Equal(t, manager.config, newManager.config)
	assert.Equal(t, configPath, newManager.path)
}

func TestLoadNonExistentFile(t *testing.T) {
	manager := NewManager()
	err := manager.Load("/non/existent/file.yaml")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read config file")
}

func TestInvalidConfig(t *testing.T) {
	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "config_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	configPath := filepath.Join(tempDir, "invalid_config.yaml")

	// Write invalid YAML
	invalidYAML := "invalid: yaml: content: ["
	err = os.WriteFile(configPath, []byte(invalidYAML), 0644)
	require.NoError(t, err)

	// Test loading invalid config
	manager := NewManager()
	err = manager.Load(configPath)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse config file")
}

func TestValidation(t *testing.T) {
	manager := NewManager()
	config := manager.GetConfig()

	// Test valid config passes validation
	err := manager.validate(config)
	assert.NoError(t, err)

	// Test invalid LLM config
	originalAPIBase := config.LLM.APIBase
	config.LLM.APIBase = ""
	err = manager.validate(config)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "LLM API base is required")

	// Restore valid config
	config.LLM.APIBase = originalAPIBase

	// Test invalid database config
	originalNumIslands := config.Database.NumIslands
	config.Database.NumIslands = 0
	err = manager.validate(config)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "number of islands must be positive")

	// Restore valid config
	config.Database.NumIslands = originalNumIslands

	// Test invalid evaluator config
	originalWorkers := config.Evaluator.ParallelWorkers
	config.Evaluator.ParallelWorkers = 0
	err = manager.validate(config)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parallel workers must be positive")

	// Restore valid config
	config.Evaluator.ParallelWorkers = originalWorkers

	// Test invalid controller config
	originalMaxIter := config.Controller.MaxIterations
	config.Controller.MaxIterations = 0
	err = manager.validate(config)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "max iterations must be positive")

	// Restore valid config
	config.Controller.MaxIterations = originalMaxIter
}

func TestEnvOverrides(t *testing.T) {
	manager := NewManager()
	config := getDefaultConfig()

	// Set environment variables
	os.Setenv("OPENAI_API_BASE", "https://custom.api.base")
	os.Setenv("OPENAI_API_KEY", "custom-api-key")
	os.Setenv("OPENAI_MODEL", "custom-model")
	os.Setenv("NUM_ISLANDS", "20")
	os.Setenv("OUTPUT_DIR", "custom-output")
	os.Setenv("MAX_ITERATIONS", "500")
	os.Setenv("SEED", "123")
	os.Setenv("VERBOSE", "true")
	defer func() {
		os.Unsetenv("OPENAI_API_BASE")
		os.Unsetenv("OPENAI_API_KEY")
		os.Unsetenv("OPENAI_MODEL")
		os.Unsetenv("NUM_ISLANDS")
		os.Unsetenv("OUTPUT_DIR")
		os.Unsetenv("MAX_ITERATIONS")
		os.Unsetenv("SEED")
		os.Unsetenv("VERBOSE")
	}()

	// Apply environment overrides
	err := manager.applyEnvOverrides(config)
	require.NoError(t, err)

	// Verify overrides were applied
	assert.Equal(t, "https://custom.api.base", config.LLM.APIBase)
	assert.Equal(t, "custom-api-key", config.LLM.APIKey)
	assert.Equal(t, "custom-model", config.LLM.Models[0].Name)
	assert.Equal(t, 20, config.Database.NumIslands)
	assert.Equal(t, "custom-output", config.Database.OutputDir)
	assert.Equal(t, 500, config.Controller.MaxIterations)
	assert.Equal(t, 123, config.Controller.Seed)
	assert.True(t, config.Controller.Verbose)
}

func TestGetSetConfig(t *testing.T) {
	manager := NewManager()

	// Test getting config
	config := manager.GetConfig()
	assert.NotNil(t, config)

	// Test setting config
	newConfig := getDefaultConfig()
	newConfig.Controller.MaxIterations = 999
	manager.SetConfig(newConfig)

	// Verify config was updated
	updatedConfig := manager.GetConfig()
	assert.Equal(t, 999, updatedConfig.Controller.MaxIterations)
}

func TestCreateDefaultConfig(t *testing.T) {
	// Create temporary directory
	tempDir, err := os.MkdirTemp("", "config_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	configPath := filepath.Join(tempDir, "default_config.yaml")

	// Create default config
	err = CreateDefaultConfig(configPath)
	require.NoError(t, err)

	// Verify file was created and is valid
	_, err = os.Stat(configPath)
	require.NoError(t, err)

	// Load and verify
	manager := NewManager()
	err = manager.Load(configPath)
	require.NoError(t, err)

	config := manager.GetConfig()
	assert.NotNil(t, config)
	assert.Equal(t, 10, config.Database.NumIslands) // DefaultNumIslands
	assert.Equal(t, 1000, config.Controller.MaxIterations) // DefaultMaxIterations
}