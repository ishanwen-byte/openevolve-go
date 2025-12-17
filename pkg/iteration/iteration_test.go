package iteration

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ishanwen-byte/openevolve-go/internal/types"
	"github.com/ishanwen-byte/openevolve-go/pkg/database"
	"github.com/ishanwen-byte/openevolve-go/pkg/evaluator"
	"github.com/ishanwen-byte/openevolve-go/pkg/llm"
)

func TestNewIterationWorker(t *testing.T) {
	config := types.Config{
		Database: types.DatabaseConfig{
			NumIslands: 2,
		},
		Prompt: types.PromptConfig{
			SystemMessage: "Test system message",
			Stochasticity: 0.5,
		},
	}

	// Create mock dependencies (would normally be real instances)
	var db *database.ProgramDatabase
	var eval *evaluator.Evaluator
	var ensemble *llm.Ensemble

	worker := NewIterationWorker(config, db, eval, ensemble)

	assert.NotNil(t, worker)
	assert.Equal(t, 2, worker.config.Database.NumIslands)
	assert.Equal(t, 0.5, worker.config.Prompt.Stochasticity)
}

func TestExtractCodeBlocks(t *testing.T) {
	worker := &IterationWorker{}

	tests := []struct {
		name     string
		text     string
		expected []string
	}{
		{
			name: "simple go block",
			text: "Here is some code:\n```go\nfunc main() {\n    println(\"hello\")\n}\n```\nEnd of code",
			expected: []string{
				"func main() {\n    println(\"hello\")\n}",
			},
		},
		{
			name: "multiple blocks",
			text: "First:\n```\ncode1\n```\nSecond:\n```go\ncode2\n```",
			expected: []string{
				"code1",
				"code2",
			},
		},
		{
			name:     "no blocks",
			text:     "Just plain text without code blocks",
			expected: []string{},
		},
		{
			name: "empty blocks",
			text: "```\n```\n```go\n\n```",
			expected: []string{},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			blocks := worker.extractCodeBlocks(test.text)
			assert.Equal(t, test.expected, blocks)
		})
	}
}

func TestParseFullRewrite(t *testing.T) {
	worker := &IterationWorker{}

	tests := []struct {
		name     string
		response string
		expected string
	}{
		{
			name:     "single block",
			response: "Here's the improved code:\n```go\nfunc improved() {}\n```",
			expected: "func improved() {}",
		},
		{
			name:     "largest block",
			response: "Small:\n```\nsmall\n```\nLarge:\n```go\nfunc large() {\n    // multiple lines\n    println(\"hello\")\n}\n```",
			expected: "func large() {\n    // multiple lines\n    println(\"hello\")\n}",
		},
		{
			name:     "no blocks",
			response: "No code blocks here",
			expected: "",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := worker.parseFullRewrite(test.response)
			assert.Equal(t, test.expected, result)
		})
	}
}

func TestApplyDiffs(t *testing.T) {
	worker := &IterationWorker{}

	parentCode := "func old() {\n    println(\"old\")\n}"
	response := "Here's the diff:\n```go\nfunc new() {\n    println(\"new\")\n}\n```"

	newCode, changes, err := worker.applyDiffs(parentCode, response)

	assert.NoError(t, err)
	assert.Equal(t, "func new() {\n    println(\"new\")\n}", newCode)
	assert.Contains(t, changes, "diff")
}

func TestCalculateFitness(t *testing.T) {
	worker := &IterationWorker{}

	// Test without parent
	fitness := worker.calculateFitness(0.8, nil)
	assert.Equal(t, 0.8, fitness)

	// Test with improvement
	parent := &types.Program{Score: 0.7}
	fitness = worker.calculateFitness(0.8, parent)
	assert.Greater(t, fitness, 0.8) // Should have bonus for improvement

	// Test with regression
	parent = &types.Program{Score: 0.9}
	fitness = worker.calculateFitness(0.8, parent)
	assert.Equal(t, 0.8, fitness) // No bonus for regression
}

func TestExtractFeatures(t *testing.T) {
	worker := &IterationWorker{}

	result := &types.EvaluationResult{
		Score:    0.85,
		Duration: 1500 * time.Millisecond,
	}

	features := worker.extractFeatures(result)

	assert.Len(t, features, 2)
	assert.Equal(t, 0.85, features[0]) // Score
	assert.Equal(t, 1.5, features[1])  // Duration in seconds
}

func TestBuildPrompt(t *testing.T) {
	worker := &IterationWorker{
		config: types.Config{
			Prompt: types.PromptConfig{
				SystemMessage:   "Test system",
				EvolutionPrompt: "Improve this code",
			},
		},
	}

	parent := &types.Program{
		Generation: 5,
		Score:      0.75,
		Code:       "func test() {}",
	}

	inspirations := []*types.Program{
		{Score: 0.85, Code: "func better() {}"},
	}

	prompt, err := worker.buildPrompt(parent, inspirations, 10)
	require.NoError(t, err)

	assert.Equal(t, "Test system", prompt.System)
	assert.Contains(t, prompt.User, "Generation 5")
	assert.Contains(t, prompt.User, "Score: 0.750")
	assert.Contains(t, prompt.User, "func test() {}")
	assert.Contains(t, prompt.User, "func better() {}")
	assert.Contains(t, prompt.User, "Improve this code")
	assert.Contains(t, prompt.Context, "Iteration: 10")
	assert.Contains(t, prompt.Context, "Generation: 5")
}

func TestGetMaxCodeLength(t *testing.T) {
	worker := &IterationWorker{}
	maxLength := worker.getMaxCodeLength()
	assert.Equal(t, 50000, maxLength) // Default value
}

func TestIterationResult_GetIterationStats(t *testing.T) {
	parent := &types.Program{
		Score:      0.7,
		Generation: 5,
	}

	child := &types.Program{
		Generation: 6,
	}

	evalResult := &types.EvaluationResult{
		Score:    0.85,
		Duration: 500 * time.Millisecond,
		Success:  true,
	}

	result := &IterationResult{
		Iteration:        10,
		ParentProgram:    parent,
		ChildProgram:     child,
		EvaluationResult: evalResult,
		Duration:         750 * time.Millisecond,
		Changes:          "Full rewrite",
		Artifacts:        map[string]string{"test": "value"},
	}

	stats := result.GetIterationStats()

	assert.Equal(t, 10, stats["iteration"])
	assert.Equal(t, int64(750), stats["duration_ms"])
	assert.Equal(t, 0.7, stats["parent_score"])
	assert.Equal(t, 0.85, stats["child_score"])
	assert.InDelta(t, 0.15, stats["score_improvement"], 0.001)
	assert.Equal(t, 6, stats["generation"])
	assert.Equal(t, "Full rewrite", stats["changes_type"])
	assert.Equal(t, true, stats["evaluation_success"])
	assert.Equal(t, 1, stats["artifacts_count"])
}

func TestIterationResult_ToJSON(t *testing.T) {
	result := &IterationResult{
		Iteration: 5,
		Changes:   "Test changes",
	}

	jsonData, err := result.ToJSON()
	require.NoError(t, err)

	assert.Contains(t, string(jsonData), "iteration")
	assert.Contains(t, string(jsonData), "5")
	assert.Contains(t, string(jsonData), "Test changes")
}

func BenchmarkExtractCodeBlocks(b *testing.B) {
	worker := &IterationWorker{}

	text := "Here is some code:\n" +
		"```go\n" +
		"func main() {\n" +
		"    println(\"hello world\")\n" +
		"}\n" +
		"```\n" +
		"And some more code:\n" +
		"```\n" +
		"func helper() {\n" +
		"    return 42\n" +
		"}\n" +
		"```\n" +
		"End of file"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = worker.extractCodeBlocks(text)
	}
}

func BenchmarkCalculateFitness(b *testing.B) {
	worker := &IterationWorker{}
	parent := &types.Program{Score: 0.7}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = worker.calculateFitness(0.8, parent)
	}
}

func BenchmarkBuildPrompt(b *testing.B) {
	worker := &IterationWorker{
		config: types.Config{
			Prompt: types.PromptConfig{
				SystemMessage:   "Test system",
				EvolutionPrompt: "Improve this",
			},
		},
	}

	parent := &types.Program{
		Generation: 5,
		Score:      0.75,
		Code:       "func test() {}",
	}

	inspirations := []*types.Program{
		{Score: 0.85, Code: "func better() {}"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = worker.buildPrompt(parent, inspirations, 10)
	}
}