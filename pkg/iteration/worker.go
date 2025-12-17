package iteration

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"

	"github.com/ishanwen-byte/openevolve-go/internal/types"
	"github.com/ishanwen-byte/openevolve-go/pkg/database"
	"github.com/ishanwen-byte/openevolve-go/pkg/evaluator"
	"github.com/ishanwen-byte/openevolve-go/pkg/llm"
)

// IterationWorker handles single evolution iterations
type IterationWorker struct {
	config         types.Config
	db             *database.ProgramDatabase
	evaluator      *evaluator.Evaluator
	llmEnsemble    *llm.Ensemble
	logger         *logrus.Logger
}

// IterationResult represents the result of a single iteration
type IterationResult struct {
	Iteration      int                    `json:"iteration"`
	ParentProgram  *types.Program         `json:"parent_program"`
	ChildProgram   *types.Program         `json:"child_program"`
	EvaluationResult *types.EvaluationResult `json:"evaluation_result"`
	Prompt         PromptData             `json:"prompt"`
	LLMResponse    string                 `json:"llm_response"`
	Duration       time.Duration          `json:"duration"`
	Artifacts      map[string]string      `json:"artifacts"`
	Changes        string                 `json:"changes"`
}

// PromptData contains the prompt information for an iteration
type PromptData struct {
	System   string `json:"system"`
	User     string `json:"user"`
	Context  string `json:"context"`
}

// NewIterationWorker creates a new iteration worker
func NewIterationWorker(
	config types.Config,
	db *database.ProgramDatabase,
	evaluator *evaluator.Evaluator,
	llmEnsemble *llm.Ensemble,
) *IterationWorker {
	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)

	return &IterationWorker{
		config:      config,
		db:          db,
		evaluator:   evaluator,
		llmEnsemble: llmEnsemble,
		logger:      logger,
	}
}

// RunIteration executes a single evolution iteration
func (iw *IterationWorker) RunIteration(ctx context.Context, iteration int) (*IterationResult, error) {
	iw.logger.WithField("iteration", iteration).Debug("Starting iteration")

	startTime := time.Now()
	result := &IterationResult{
		Iteration: iteration,
		Artifacts: make(map[string]string),
	}

	// Sample parent program and inspirations
	parentProgram, inspirations, err := iw.samplePrograms()
	if err != nil {
		return nil, fmt.Errorf("failed to sample programs: %w", err)
	}

	result.ParentProgram = parentProgram

	// Build prompt
	prompt, err := iw.buildPrompt(parentProgram, inspirations, iteration)
	if err != nil {
		return nil, fmt.Errorf("failed to build prompt: %w", err)
	}

	result.Prompt = prompt

	// Generate code modification using LLM
	// Combine system and user messages into a single prompt
	fullPrompt := fmt.Sprintf("System: %s\n\nUser: %s", prompt.System, prompt.User)
	llmResponse, err := iw.llmEnsemble.Generate(ctx, fullPrompt)
	if err != nil {
		return nil, fmt.Errorf("failed to generate LLM response: %w", err)
	}

	result.LLMResponse = llmResponse.Content

	// Parse the LLM response to extract new code
	var childCode string
	var changes string

	if iw.config.Prompt.Stochasticity > 0.5 {
		// Use diff-based evolution
		childCode, changes, err = iw.applyDiffs(parentProgram.Code, llmResponse.Content)
	} else {
		// Use full rewrite
		childCode = iw.parseFullRewrite(llmResponse.Content)
		changes = "Full rewrite"
	}

	if err != nil {
		return nil, fmt.Errorf("failed to parse LLM response: %w", err)
	}

	if childCode == "" {
		return nil, fmt.Errorf("no valid code generated")
	}

	// Check code length
	if len(childCode) > iw.getMaxCodeLength() {
		return nil, fmt.Errorf("generated code exceeds maximum length: %d > %d",
			len(childCode), iw.getMaxCodeLength())
	}

	// Evaluate the child program
	evalResult, err := iw.evaluator.Evaluate(ctx, childCode)
	if err != nil {
		return nil, fmt.Errorf("evaluation failed: %w", err)
	}

	result.EvaluationResult = evalResult

	// Get artifacts if available
	if childID := evalResult.ID; childID != "" {
		artifacts, _ := iw.evaluator.GetArtifacts(childID)
		if artifacts != nil {
			result.Artifacts = artifacts
		}
	}

	// Create child program
	childProgram := &types.Program{
		ID:         uuid.New().String(),
		Code:       childCode,
		Score:      evalResult.Score,
		Fitness:    iw.calculateFitness(evalResult.Score, parentProgram),
		Features:   iw.extractFeatures(evalResult),
		Generation: parentProgram.Generation + 1,
		IslandID:   parentProgram.IslandID,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
		Artifacts:  result.Artifacts,
	}

	result.ChildProgram = childProgram
	result.Changes = changes
	result.Duration = time.Since(startTime)

	// Add child program to database
	if err := iw.db.AddProgram(childProgram, iteration); err != nil {
		iw.logger.WithError(err).Warn("Failed to add child program to database")
	}

	iw.logger.WithFields(logrus.Fields{
		"iteration": iteration,
		"score":     evalResult.Score,
		"duration":  result.Duration,
		"success":   evalResult.Success,
	}).Info("Iteration completed")

	return result, nil
}

// samplePrograms samples a parent program and inspirations from the database
func (iw *IterationWorker) samplePrograms() (*types.Program, []*types.Program, error) {
	// Sample parent program
	parent, err := iw.db.SampleFromIsland(iw.db.GetCurrentIsland())
	if err != nil {
		// Fallback to any island
		for i := 0; i < iw.config.Database.NumIslands; i++ {
			parent, err = iw.db.SampleFromIsland(i)
			if err == nil {
				break
			}
		}
		if err != nil {
			return nil, nil, fmt.Errorf("failed to sample parent program: %w", err)
		}
	}

	// Sample inspiration programs
	inspirations, err := iw.db.SampleMultiple(3) // Get 3 inspirations
	if err != nil {
		iw.logger.WithError(err).Warn("Failed to sample inspirations, continuing without them")
		inspirations = []*types.Program{}
	}

	return parent, inspirations, nil
}

// buildPrompt constructs the evolution prompt
func (iw *IterationWorker) buildPrompt(parent *types.Program, inspirations []*types.Program, iteration int) (PromptData, error) {
	// Build system message
	systemMsg := iw.config.Prompt.SystemMessage
	if systemMsg == "" {
		systemMsg = "You are an expert programmer helping to evolve and improve code."
	}

	// Build user prompt with context
	userPrompt := iw.buildUserPrompt(parent, inspirations, iteration)

	return PromptData{
		System:  systemMsg,
		User:    userPrompt,
		Context: fmt.Sprintf("Iteration: %d, Generation: %d", iteration, parent.Generation),
	}, nil
}

// buildUserPrompt builds the user portion of the prompt
func (iw *IterationWorker) buildUserPrompt(parent *types.Program, inspirations []*types.Program, iteration int) string {
	promptBuilder := strings.Builder{}

	promptBuilder.WriteString(fmt.Sprintf("Current code to improve (Generation %d, Score: %.3f):\n\n",
		parent.Generation, parent.Score))
	promptBuilder.WriteString("```\n")
	promptBuilder.WriteString(parent.Code)
	promptBuilder.WriteString("\n```\n\n")

	if len(inspirations) > 0 {
		promptBuilder.WriteString("Here are some high-scoring similar programs for inspiration:\n\n")
		for i, insp := range inspirations {
			promptBuilder.WriteString(fmt.Sprintf("Example %d (Score: %.3f):\n", i+1, insp.Score))
			promptBuilder.WriteString("```\n")
			// Truncate very long programs
			code := insp.Code
			if len(code) > 1000 {
				code = code[:1000] + "\n... (truncated)"
			}
			promptBuilder.WriteString(code)
			promptBuilder.WriteString("\n```\n\n")
		}
	}

	// Add evolution instructions
	if iw.config.Prompt.EvolutionPrompt != "" {
		promptBuilder.WriteString("Instructions:\n")
		promptBuilder.WriteString(iw.config.Prompt.EvolutionPrompt)
	} else {
		promptBuilder.WriteString("Please improve this code to achieve better performance. ")
		promptBuilder.WriteString("Focus on algorithmic improvements, bug fixes, and optimizations. ")
	}

	return promptBuilder.String()
}

// applyDiffs applies diff-based modifications to the code
func (iw *IterationWorker) applyDiffs(parentCode, llmResponse string) (string, string, error) {
	// Simple diff parser - looks for code blocks with specific markers
	codeBlocks := iw.extractCodeBlocks(llmResponse)
	if len(codeBlocks) == 0 {
		return "", "", fmt.Errorf("no code blocks found in LLM response")
	}

	// For simplicity, use the first code block as the new code
	newCode := codeBlocks[0]
	changes := fmt.Sprintf("Applied %d diff(s)", len(codeBlocks))

	return newCode, changes, nil
}

// parseFullRewrite extracts a complete program rewrite
func (iw *IterationWorker) parseFullRewrite(llmResponse string) string {
	// Look for the largest code block
	codeBlocks := iw.extractCodeBlocks(llmResponse)
	if len(codeBlocks) == 0 {
		return ""
	}

	// Return the largest code block
	largestBlock := ""
	for _, block := range codeBlocks {
		if len(block) > len(largestBlock) {
			largestBlock = block
		}
	}

	return largestBlock
}

// extractCodeBlocks extracts Go code blocks from text
func (iw *IterationWorker) extractCodeBlocks(text string) []string {
	// Pattern to match ```go ... ``` or just ``` ... ```
	// Use a simpler approach since Go doesn't support negative lookahead
	pattern := regexp.MustCompile("```(?:go)?\n?([^`]*)```")
	matches := pattern.FindAllStringSubmatch(text, -1)

	blocks := make([]string, 0)
	for _, match := range matches {
		if len(match) > 1 {
			code := strings.TrimSpace(match[1])
			if code != "" {
				blocks = append(blocks, code)
			}
		}
	}

	return blocks
}

// calculateFitness calculates the fitness score for a program
func (iw *IterationWorker) calculateFitness(score float64, parent *types.Program) float64 {
	// Simple fitness calculation based on score improvement
	fitness := score
	if parent != nil {
		// Bonus for improvement over parent
		improvement := score - parent.Score
		if improvement > 0 {
			fitness += improvement * 0.1 // 10% bonus for improvements
		}
	}
	return fitness
}

// extractFeatures extracts features from evaluation result
func (iw *IterationWorker) extractFeatures(result *types.EvaluationResult) []float64 {
	// Simple feature extraction - can be enhanced
	features := make([]float64, 2) // complexity, diversity

	// Use score as a simple proxy for complexity
	features[0] = result.Score

	// Use duration as a simple proxy for diversity
	features[1] = float64(result.Duration.Milliseconds()) / 1000.0

	return features
}

// getMaxCodeLength returns the maximum allowed code length
func (iw *IterationWorker) getMaxCodeLength() int {
	// Default to 50KB if not specified
	maxLength := 50000
	// Can be made configurable
	return maxLength
}

// GetIterationStats returns statistics about the iteration
func (ir *IterationResult) GetIterationStats() map[string]interface{} {
	stats := map[string]interface{}{
		"iteration":          ir.Iteration,
		"duration_ms":        ir.Duration.Milliseconds(),
		"parent_score":       ir.ParentProgram.Score,
		"child_score":        ir.EvaluationResult.Score,
		"score_improvement":  ir.EvaluationResult.Score - ir.ParentProgram.Score,
		"generation":         ir.ChildProgram.Generation,
		"changes_type":       ir.Changes,
		"evaluation_success": ir.EvaluationResult.Success,
	}

	if len(ir.Artifacts) > 0 {
		stats["artifacts_count"] = len(ir.Artifacts)
	}

	return stats
}

// ToJSON converts the iteration result to JSON
func (ir *IterationResult) ToJSON() ([]byte, error) {
	return json.MarshalIndent(ir, "", "  ")
}