package constants

// Application constants
const (
	Name        = "OpenEvolve-Go"
	Version     = "1.0.0"
	Description = "Go implementation of OpenEvolve - evolutionary coding agent"

	// Default configuration values
	DefaultNumIslands        = 10
	DefaultMaxIterations     = 1000
	DefaultMaxGenerations    = 100
	DefaultParallelWorkers   = 4
	DefaultTimeout          = 60 // seconds
	DefaultRetries          = 3
	DefaultRetryDelay       = 5 // seconds
	DefaultTemperature      = 0.7
	DefaultTopP             = 0.95
	DefaultMaxTokens        = 4096
	DefaultMigrationInterval = 10
	DefaultMigrationRate    = 0.1

	// Grid defaults
	DefaultGridResolution = 10
	DefaultMaxProgramsPerCell = 1
	DefaultCheckpointInterval = 100

	// Artifact defaults
	DefaultArtifactMaxSize = 10 * 1024 // 10KB

	// File extensions
	PythonExt = ".py"
	GoExt     = ".go"
	RustExt   = ".rs"
	JavaExt   = ".java"
	CppExt    = ".cpp"
	JSExt     = ".js"

	// Directory names
	OutputDir     = "openevolve_output"
	CheckpointDir = "checkpoints"
	ArtifactsDir  = "artifacts"
	LogsDir       = "logs"

	// Prompt defaults
	DefaultSystemMessage = "You are an expert programmer helping to evolve and improve code."
	DefaultEvolutionPrompt = "Please improve the following code:"
	DefaultMutationPrompt = "Please apply a mutation to the following code:"
	DefaultStochasticity = 0.1
	DefaultHistoryLength = 5

	// OpenAI API
	DefaultOpenAIBase = "https://api.openai.com/v1"

	// Exit codes
	ExitSuccess = 0
	ExitError   = 1
	ExitInterrupt = 2
)

// Model names
const (
	GPT4          = "gpt-4"
	GPT4Turbo     = "gpt-4-turbo"
	GPT35Turbo    = "gpt-3.5-turbo"
	GPT4o         = "gpt-4o"
	GPT4oMini     = "gpt-4o-mini"
	O1Preview     = "o1-preview"
	O1Mini        = "o1-mini"
)

// Reasoning effort levels
const (
	ReasoningEffortLow    = "low"
	ReasoningEffortMedium = "medium"
	ReasoningEffortHigh   = "high"
)

// Evaluation stages
const (
	EvalStageValidation = "validation"
	EvalStageBasic      = "basic"
	EvalStageComprehensive = "comprehensive"
)