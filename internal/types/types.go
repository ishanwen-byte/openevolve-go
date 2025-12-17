package types

import (
	"time"
)

// Program represents a program being evolved
type Program struct {
	ID          string            `json:"id"`
	Code        string            `json:"code"`
	Features    []float64         `json:"features"`
	Score       float64           `json:"score"`
	Fitness     float64           `json:"fitness"`
	Generation  int               `json:"generation"`
	IslandID    int               `json:"island_id"`
	Artifacts   map[string]string `json:"artifacts"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// Island represents an island in the island-based evolution
type Island struct {
	ID         int                  `json:"id"`
	Programs   map[string]*Program  `json:"programs"`
	Grid       MAPGrid              `json:"grid"`
	BestScore  float64              `json:"best_score"`
	BestID     string               `json:"best_id"`
	Generation int                  `json:"generation"`
	Migrated   int                  `json:"migrated"`
}

// MAPGrid represents the MAP-Elites grid for quality-diversity
type MAPGrid struct {
	Dimensions []string          `json:"dimensions"`
	Resolution map[string]int    `json:"resolution"`
	Bounds     map[string][2]float64 `json:"bounds"`
	Cells      map[string]*Program `json:"cells"`
}

// EvaluationResult represents the result of program evaluation
type EvaluationResult struct {
	ID       string            `json:"id"`
	Score    float64           `json:"score"`
	Fitness  float64           `json:"fitness"`
	Features []float64         `json:"features"`
	Success  bool              `json:"success"`
	Artifacts map[string]string `json:"artifacts"`
	Error    string            `json:"error,omitempty"`
	Duration time.Duration     `json:"duration"`
}

// LLMRequest represents a request to an LLM
type LLMRequest struct {
	Model       string            `json:"model"`
	Messages    []LLMMessage      `json:"messages"`
	Temperature float64           `json:"temperature"`
	TopP        float64           `json:"top_p"`
	MaxTokens   int               `json:"max_tokens"`
	Timeout     time.Duration     `json:"timeout"`
}

// LLMMessage represents a message in an LLM conversation
type LLMMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// LLMResponse represents a response from an LLM
type LLMResponse struct {
	Content   string        `json:"content"`
	Model     string        `json:"model"`
	Usage     TokenUsage    `json:"usage"`
	Duration  time.Duration `json:"duration"`
	Error     error         `json:"error,omitempty"`
}

// TokenUsage represents token usage statistics
type TokenUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// IterationState represents the state of an evolution iteration
type IterationState struct {
	Iteration   int       `json:"iteration"`
	Generation  int       `json:"generation"`
	Program     *Program  `json:"program"`
	Inspirations []*Program `json:"inspirations"`
	Results     []*EvaluationResult `json:"results"`
	StartTime   time.Time `json:"start_time"`
	EndTime     time.Time `json:"end_time"`
}

// Checkpoint represents a saved state of the evolution system
type Checkpoint struct {
	Version      string              `json:"version"`
	CreatedAt    time.Time           `json:"created_at"`
	Iteration    int                 `json:"iteration"`
	Generation   int                 `json:"generation"`
	Islands      map[int]*Island     `json:"islands"`
	GlobalBest   *Program            `json:"global_best"`
	Config       map[string]interface{} `json:"config"`
	Stats        EvolutionStats      `json:"stats"`
}

// EvolutionStats tracks statistics about the evolution process
type EvolutionStats struct {
	TotalEvaluations int64         `json:"total_evaluations"`
	SuccessfulEvals  int64         `json:"successful_evals"`
	FailedEvals      int64         `json:"failed_evals"`
	TotalMutations   int64         `json:"total_mutations"`
	AvgScore         float64       `json:"avg_score"`
	BestScore        float64       `json:"best_score"`
	Duration         time.Duration `json:"duration"`
	StartTime        time.Time     `json:"start_time"`
	LastUpdate       time.Time     `json:"last_update"`
}

// PromptTemplate represents a template for generating prompts
type PromptTemplate struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Template    string            `json:"template"`
	Variables   map[string]string `json:"variables"`
}

// MutationType represents different types of mutations
type MutationType string

const (
	MutationTypeSubstitution   MutationType = "substitution"
	MutationTypeInsertion      MutationType = "insertion"
	MutationTypeDeletion       MutationType = "deletion"
	MutationTypeBlockSwap      MutationType = "block_swap"
	MutationTypeParameterTweak MutationType = "parameter_tweak"
)

// Config represents the main configuration
type Config struct {
	LLM       LLMConfig       `yaml:"llm" json:"llm"`
	Database  DatabaseConfig  `yaml:"database" json:"database"`
	Evaluator EvaluatorConfig `yaml:"evaluator" json:"evaluator"`
	Prompt    PromptConfig    `yaml:"prompt" json:"prompt"`
	Controller ControllerConfig `yaml:"controller" json:"controller"`
}

// LLMConfig represents LLM configuration
type LLMConfig struct {
	APIBase          string                  `yaml:"api_base" json:"api_base"`
	APIKey           string                  `yaml:"api_key" json:"api_key"`
	Models           []LLMModelConfig        `yaml:"models" json:"models"`
	EvaluatorModels  []LLMModelConfig        `yaml:"evaluator_models" json:"evaluator_models"`
	SystemMessage    string                  `yaml:"system_message" json:"system_message"`
	Temperature      float64                 `yaml:"temperature" json:"temperature"`
	TopP             float64                 `yaml:"top_p" json:"top_p"`
	MaxTokens        int                     `yaml:"max_tokens" json:"max_tokens"`
	Timeout          int                     `yaml:"timeout" json:"timeout"`
	Retries          int                     `yaml:"retries" json:"retries"`
	RetryDelay       int                     `yaml:"retry_delay" json:"retry_delay"`
	RandomSeed       int                     `yaml:"random_seed" json:"random_seed"`
	ReasoningEffort  *string                 `yaml:"reasoning_effort" json:"reasoning_effort"`
}

// LLMModelConfig represents configuration for a single LLM model
type LLMModelConfig struct {
	Name             string  `yaml:"name" json:"name"`
	Weight           float64 `yaml:"weight" json:"weight"`
	APIBase          string  `yaml:"api_base" json:"api_base"`
	APIKey           string  `yaml:"api_key" json:"api_key"`
	SystemMessage    string  `yaml:"system_message" json:"system_message"`
	Temperature      float64 `yaml:"temperature" json:"temperature"`
	TopP             float64 `yaml:"top_p" json:"top_p"`
	MaxTokens        int     `yaml:"max_tokens" json:"max_tokens"`
	Timeout          int     `yaml:"timeout" json:"timeout"`
	Retries          int     `yaml:"retries" json:"retries"`
	RetryDelay       int     `yaml:"retry_delay" json:"retry_delay"`
	RandomSeed       int     `yaml:"random_seed" json:"random_seed"`
	ReasoningEffort  *string `yaml:"reasoning_effort" json:"reasoning_effort"`
}

// DatabaseConfig represents database configuration
type DatabaseConfig struct {
	NumIslands        int               `yaml:"num_islands" json:"num_islands"`
	GridDimensions    []string          `yaml:"grid_dimensions" json:"grid_dimensions"`
	GridResolution    map[string]int    `yaml:"grid_resolution" json:"grid_resolution"`
	GridBounds        map[string][2]float64 `yaml:"grid_bounds" json:"grid_bounds"`
	MigrationInterval int               `yaml:"migration_interval" json:"migration_interval"`
	MigrationRate     float64           `yaml:"migration_rate" json:"migration_rate"`
	MaxProgramsPerCell int              `yaml:"max_programs_per_cell" json:"max_programs_per_cell"`
	CheckpointInterval int              `yaml:"checkpoint_interval" json:"checkpoint_interval"`
	OutputDir         string            `yaml:"output_dir" json:"output_dir"`
}

// EvaluatorConfig represents evaluator configuration
type EvaluatorConfig struct {
	CascadeStages     []CascadeStage    `yaml:"cascade_stages" json:"cascade_stages"`
	ParallelWorkers   int               `yaml:"parallel_workers" json:"parallel_workers"`
	Timeout           int               `yaml:"timeout" json:"timeout"`
	CollectArtifacts  bool              `yaml:"collect_artifacts" json:"collect_artifacts"`
	ArtifactMaxSize   int               `yaml:"artifact_max_size" json:"artifact_max_size"`
}

// CascadeStage represents a stage in cascade evaluation
type CascadeStage struct {
	Name         string  `yaml:"name" json:"name"`
	Threshold    float64 `yaml:"threshold" json:"threshold"`
	Timeout      int     `yaml:"timeout" json:"timeout"`
	Critical     bool    `yaml:"critical" json:"critical"`
}

// PromptConfig represents prompt configuration
type PromptConfig struct {
	Templates        []PromptTemplate   `yaml:"templates" json:"templates"`
	SystemMessage    string             `yaml:"system_message" json:"system_message"`
	EvolutionPrompt  string             `yaml:"evolution_prompt" json:"evolution_prompt"`
	MutationPrompt   string             `yaml:"mutation_prompt" json:"mutation_prompt"`
	Stochasticity    float64            `yaml:"stochasticity" json:"stochasticity"`
	IncludeHistory   bool               `yaml:"include_history" json:"include_history"`
	HistoryLength    int                `yaml:"history_length" json:"history_length"`
}

// ControllerConfig represents controller configuration
type ControllerConfig struct {
	MaxIterations    int               `yaml:"max_iterations" json:"max_iterations"`
	MaxGenerations   int               `yaml:"max_generations" json:"max_generations"`
	TargetScore      *float64          `yaml:"target_score" json:"target_score"`
	ParallelWorkers  int               `yaml:"parallel_workers" json:"parallel_workers"`
	CheckpointDir    string            `yaml:"checkpoint_dir" json:"checkpoint_dir"`
	ResumeFrom       string            `yaml:"resume_from" json:"resume_from"`
	Seed             int               `yaml:"seed" json:"seed"`
	Verbose          bool              `yaml:"verbose" json:"verbose"`
}