# Migration TODO List

## Active Tasks

### Phase 1: Core Infrastructure

#### Database Component (`pkg/database/`)
- [ ] `database.go` - Implement ProgramDatabase with MAP-Elites
  - [ ] New() constructor
  - [ ] AddProgram() method
  - [ ] GetProgram() method
  - [ ] SampleFromIsland() method
  - [ ] MigratePrograms() method
  - [ ] SaveCheckpoint() method
  - [ ] LoadCheckpoint() method
  - [ ] GetGlobalBest() method
  - [ ] UpdateStats() method
- [ ] `island.go` - Island management
  - [ ] NewIsland() constructor
  - [ ] AddToGrid() method for MAP-Elites
  - [ ] GetFromGrid() method
  - [ ] GetBestProgram() method
  - [ ] IncrementGeneration() method

#### Evaluator Component (`pkg/evaluator/`)
- [ ] `evaluator.go` - Main evaluator
  - [ ] New() constructor
  - [ ] Evaluate() method with cascade
  - [ ] EvaluateBatch() for parallel evaluation
  - [ ] setupSandbox() method
- [ ] `cascade.go` - Cascade evaluation stages
  - [ ] NewCascadeStage() constructor
  - [ ] RunStage() method
  - [ ] ShouldContinue() method

#### Iteration Component (`pkg/iteration/`)
- [ ] `worker.go` - Iteration worker
  - [ ] New() constructor
  - [ ] Run() method for single iteration
  - [ ] generateMutations() method
  - [ ] evaluatePrograms() method
  - [ ] updateDatabase() method

### Phase 2: Core Components

#### Controller Component (`pkg/controller/`)
- [ ] `controller.go` - Main controller
  - [ ] New() constructor
  - [ ] Run() method for evolution loop
  - [ ] setupSignalHandlers() method
  - [ ] checkpointHandler() method
  - [ ] printProgress() method

#### Utilities (`internal/utils/`)
- [ ] `code.go` - Code utilities
  - [ ] ExtractLanguage() function
  - [ ] CalculateEditDistance() function
  - [ ] ValidateSyntax() function
- [ ] `format.go` - Format utilities
  - [ ] FormatMetrics() function
  - [ ] FormatImprovement() function
- [ ] `async.go` - Async utilities
  - [ ] WorkerPool() struct
  - [ ] NewWorkerPool() constructor

### Phase 3: CLI & Examples

#### CLI (`cmd/openevolve/`)
- [ ] `main.go` - CLI application
  - [ ] parseArgs() function
  - [ ] loadConfig() function
  - [ ] runEvolution() function
  - [ ] handleSignals() function

#### Examples
- [ ] `examples/function_minimization/`
  - [ ] `initial_program.go`
  - [ ] `evaluator.go`
  - [ ] `config.yaml`
- [ ] `examples/circle_packing/`
  - [ ] `initial_program.go`
  - [ ] `evaluator.go`
  - [ ] `config.yaml`

### Phase 4: Testing

#### Unit Tests
- [ ] `pkg/database/database_test.go`
- [ ] `pkg/evaluator/evaluator_test.go`
- [ ] `pkg/iteration/iteration_test.go`
- [ ] `pkg/controller/controller_test.go`
- [ ] `internal/utils/*_test.go`

#### Integration Tests
- [ ] `tests/e2e/evolution_test.go`
- [ ] `tests/integration/checkpoint_test.go`
- [ ] `tests/integration/migration_test.go`

## Completed Tasks
- [x] Type definitions in `internal/types/types.go`
- [x] Configuration handling in `pkg/config/`
- [x] Basic LLM ensemble in `pkg/llm/`
- [x] Migration plan document

## Notes
- Each task should be committed and pushed immediately after completion
- Follow Go naming conventions and best practices
- Include comprehensive error handling
- Add context to all errors for debugging
- Write tests concurrently with implementation