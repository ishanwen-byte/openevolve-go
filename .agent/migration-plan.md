# OpenEvolve Python to Go Migration Plan

## Overview
This document outlines the migration strategy for converting OpenEvolve from Python to Go while maintaining the same architecture and functionality.

## Current Status

### Python Repository (Source)
- **Architecture**: Mature, feature-complete implementation
- **Core Components**: Controller, Database, Evaluator, LLM integration, Iteration workers
- **Key Features**: MAP-Elites algorithm, island-based evolution, cascade evaluation, checkpoint/resume
- **Testing**: Unit tests and integration tests with LLM support

### Go Repository (Target)
- **Current State**: Initial structure with types and basic LLM integration
- **Completed**:
  - Type definitions in `internal/types/types.go`
  - Configuration handling in `pkg/config/`
  - Basic LLM ensemble implementation in `pkg/llm/`
- **Missing**: Core components (Controller, Database, Evaluator, Iteration)

## Migration Strategy: 80% Implementation, 20% Testing

### Phase 1: Core Infrastructure (First 30%)

#### 1.1 Database Component (`pkg/database/`)
- [ ] Implement `ProgramDatabase` struct with MAP-Elites algorithm
- [ ] Island management and migration logic
- [ ] Checkpoint save/load functionality
- [ ] JSON serialization for persistence
- [ ] Grid-based program storage with feature mapping

#### 1.2 Evaluator Component (`pkg/evaluator/`)
- [ ] Cascade evaluation pattern with stages
- [ ] Parallel evaluation workers
- [ ] Timeout and error handling
- [ ] Artifact collection system
- [ ] Program execution sandbox

#### 1.3 Iteration Worker (`pkg/iteration/`)
- [ ] Worker process implementation
- [ ] Program sampling from islands
- [ ] LLM mutation generation
- [ ] Evaluation coordination
- [ ] Database integration

### Phase 2: Core Components (Next 30%)

#### 2.1 Controller (`pkg/controller/`)
- [ ] Main evolution orchestrator
- [ ] ProcessPoolExecutor replacement with goroutines
- [ ] Checkpoint management
- [ ] Evolution loop implementation
- [ ] Statistics tracking

#### 2.2 Utilities (`internal/utils/`)
- [ ] Code utilities (language detection, edit distance)
- [ ] Format utilities (metrics, improvement formatting)
- [ ] Async utilities for concurrent operations
- [ ] Metrics utilities

### Phase 3: Integration & CLI (Next 20%)

#### 3.1 CLI Application (`cmd/openevolve/`)
- [ ] Command-line argument parsing
- [ ] Configuration loading
- [ ] Evolution execution entry point
- [ ] Progress reporting
- [ ] Signal handling (graceful shutdown)

#### 3.2 Examples Migration
- [ ] Port key examples from Python to Go
- [ ] Function minimization example
- [ ] Circle packing example
- [ ] LLM prompt optimization example

### Phase 4: Testing (Final 20%)

#### 4.1 Unit Tests
- [ ] Database tests (MAP-Elites, islands, migration)
- [ ] Evaluator tests (cascade, timeout, artifacts)
- [ ] Controller tests (evolution loop, checkpoint)
- [ ] LLM tests (ensemble, retry logic)
- [ ] Utility function tests

#### 4.2 End-to-End Tests
- [ ] Complete evolution workflow tests
- [ ] Checkpoint/resume functionality tests
- [ ] Error handling and recovery tests
- [ ] Performance benchmarks
- [ ] Integration tests with mock LLMs

## Technical Considerations

### 1. Concurrency Model
- **Python**: Uses ProcessPoolExecutor for true parallelism
- **Go**: Use goroutines with worker pools
- **Benefit**: More efficient, lower memory overhead

### 2. Error Handling
- **Python**: Exception-based with try/except
- **Go**: Error returns with explicit handling
- **Pattern**: Wrap errors with context for debugging

### 3. Configuration
- **Python**: YAML files with dataclasses
- **Go**: YAML files with struct tags
- **Maintain**: Same configuration schema for compatibility

### 4. Checkpoint Format
- **Python**: JSON-based with Python-specific types
- **Go**: JSON with Go types (maintain compatibility)
- **Strategy**: Use same JSON schema for interoperability

### 5. LLM Integration
- **Python**: HTTP clients with async/await
- **Go**: HTTP clients with goroutines
- **Keep**: Same API interfaces and retry logic

## Dependencies

### Required Go Modules
```go
- gopkg.in/yaml.v3  // Configuration
- github.com/stretchr/testify // Testing
- github.com/gorilla/mux // HTTP routing (for evaluator API)
- github.com/google/uuid // UUID generation
- github.com/sirupsen/logrus // Logging
```

### Additional Dependencies (as needed)
- Program execution: `os/exec` packages
- File system operations: `pkg/filesystem`
- Metrics collection: `pkg/metrics`
- WebSocket for real-time updates: `gorilla/websocket`

## File Structure Plan

```
openevolve-go/
├── cmd/
│   └── openevolve/
│       └── main.go              # CLI entry point
├── internal/
│   ├── constants/
│   │   └── constants.go         # System constants
│   ├── types/
│   │   └── types.go            # Core type definitions
│   └── utils/
│       ├── code.go             # Code utilities
│       ├── format.go           # Format utilities
│       └── async.go            # Async utilities
├── pkg/
│   ├── config/
│   │   ├── config.go           # Configuration handling
│   │   └── config_test.go
│   ├── database/
│   │   ├── database.go         # MAP-Elites database
│   │   ├── island.go           # Island management
│   │   └── database_test.go
│   ├── evaluator/
│   │   ├── evaluator.go        # Cascade evaluator
│   │   ├── cascade.go          # Cascade stages
│   │   └── evaluator_test.go
│   ├── llm/
│   │   ├── ensemble.go         # LLM ensemble
│   │   ├── openai.go           # OpenAI client
│   │   └── llm_test.go
│   ├── iteration/
│   │   ├── worker.go           # Iteration worker
│   │   └── iteration_test.go
│   └── controller/
│       ├── controller.go       # Main controller
│       └── controller_test.go
├── examples/
│   ├── function_minimization/
│   ├── circle_packing/
│   └── llm_prompt_optimization/
├── tests/
│   ├── e2e/
│   │   └── evolution_test.go   # End-to-end tests
│   └── integration/
│       └── checkpoint_test.go  # Integration tests
└── scripts/
    └── run_examples.sh         # Example runner
```

## Success Criteria

1. **Functional Parity**: All Python features work in Go
2. **Performance**: Equal or better performance than Python
3. **Compatibility**: Can load Python-generated checkpoints
4. **Test Coverage**: >80% code coverage
5. **Documentation**: Complete API documentation and examples

## Timeline (Estimated)

- **Phase 1**: 3-4 days (Database, Evaluator, Iteration)
- **Phase 2**: 2-3 days (Controller, Utilities)
- **Phase 3**: 2 days (CLI, Examples)
- **Phase 4**: 2-3 days (Testing, Documentation)

**Total**: 9-12 days for complete migration

## Next Steps

1. Start with Database component (core MAP-Elites algorithm)
2. Implement Evaluator with cascade pattern
3. Build Iteration workers
4. Integrate everything in Controller
5. Add CLI and examples
6. Comprehensive testing

Each component will be implemented with tests immediately after completion to ensure correctness.