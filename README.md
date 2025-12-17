# OpenEvolve Go

Go implementation of OpenEvolve - an evolutionary coding agent that uses LLMs to optimize code through iterative evolution.

## Features

- **Island-Based Evolution**: Multiple populations evolve separately with periodic migration
- **MAP-Elites Algorithm**: Maintains diversity by mapping programs to feature grid cells
- **Cascade Evaluation**: Multi-stage evaluation with early filtering
- **LLM Integration**: Support for multiple LLM providers with ensemble approach
- **Checkpoint/Resume**: Automatic saving of system state with seamless resume
- **Parallel Processing**: Concurrent program evaluation

## Installation

```bash
go install github.com/ishanwen-byte/openevolve-go/cmd/openevolve@latest
```

## Quick Start

```bash
# Run evolution with example programs
openevolve run examples/function_minimization/initial_program.go \
  examples/function_minimization/evaluator.go \
  --config examples/function_minimization/config.yaml \
  --iterations 1000

# Resume from checkpoint
openevolve run --checkpoint path/to/checkpoint --iterations 100
```

## Development

```bash
# Clone repository
git clone https://github.com/ishanwen-byte/openevolve-go.git
cd openevolve-go

# Install dependencies
go mod download

# Run tests
go test ./...

# Build
go build -o bin/openevolve cmd/openevolve/main.go
```

## Architecture

- **cmd/**: CLI entry points
- **pkg/**: Public libraries
- **internal/**: Internal implementation details
- **examples/**: Example programs and configurations

## License

Apache-2.0