package evaluator

import (
	"context"
	"fmt"
	"os/exec"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/ishanwen-byte/openevolve-go/internal/types"
)

// CascadeStage represents a stage in the cascade evaluation
type CascadeStage struct {
	Name      string        `json:"name"`
	Threshold float64       `json:"threshold"`
	Timeout   time.Duration `json:"timeout"`
	Critical  bool          `json:"critical"`
}

// CascadeEvaluator handles multi-stage cascade evaluation
type CascadeEvaluator struct {
	stages    []CascadeStage
	logger    *logrus.Logger
	programPath string
}

// NewCascadeEvaluator creates a new cascade evaluator
func NewCascadeEvaluator(stages []types.CascadeStage, programPath string) *CascadeEvaluator {
	logger := logrus.New()

	cascadeStages := make([]CascadeStage, len(stages))
	for i, stage := range stages {
		cascadeStages[i] = CascadeStage{
			Name:      stage.Name,
			Threshold: stage.Threshold,
			Timeout:   time.Duration(stage.Timeout) * time.Second,
			Critical:  stage.Critical,
		}
	}

	return &CascadeEvaluator{
		stages:      cascadeStages,
		logger:      logger,
		programPath: programPath,
	}
}

// Evaluate runs cascade evaluation through all stages
func (ce *CascadeEvaluator) Evaluate(ctx context.Context) (*types.EvaluationResult, error) {
	result := &types.EvaluationResult{
		ID:      fmt.Sprintf("cascade-%d", time.Now().UnixNano()),
		Success: false,
		Artifacts: make(map[string]string),
	}

	startTime := time.Now()
	defer func() {
		result.Duration = time.Since(startTime)
	}()

	// Run through each stage
	for i, stage := range ce.stages {
		stageResult, err := ce.runStage(ctx, stage, i+1)
		if err != nil {
			result.Error = err.Error()
			result.Artifacts["failure_stage"] = stage.Name
			result.Artifacts["stage_error"] = err.Error()
			ce.logger.WithFields(logrus.Fields{
				"stage": stage.Name,
				"error": err,
			}).Error("Stage evaluation failed")
			return result, err
		}

		// Check if stage passed threshold
		if stageResult.Score < stage.Threshold {
			result.Success = false
			result.Score = stageResult.Score
			result.Error = fmt.Sprintf("Stage %s failed threshold: %.3f < %.3f",
				stage.Name, stageResult.Score, stage.Threshold)
			result.Artifacts["failure_stage"] = stage.Name
			result.Artifacts["threshold_failed"] = "true"

			// If this is a critical stage, return immediately
			if stage.Critical {
				return result, fmt.Errorf("critical stage %s failed threshold", stage.Name)
			}

			// Non-critical stage failed, continue with warning
			ce.logger.WithFields(logrus.Fields{
				"stage":     stage.Name,
				"score":     stageResult.Score,
				"threshold": stage.Threshold,
			}).Warn("Stage failed threshold but continuing")
		}

		// Update result with stage metrics
		if stageResult.Score > result.Score {
			result.Score = stageResult.Score
		}

		// Merge artifacts
		for k, v := range stageResult.Artifacts {
			result.Artifacts[k] = v
		}
	}

	// All stages completed successfully
	result.Success = true
	return result, nil
}

// runStage executes a single cascade stage
func (ce *CascadeEvaluator) runStage(ctx context.Context, stage CascadeStage, stageNumber int) (*types.EvaluationResult, error) {
	ce.logger.WithFields(logrus.Fields{
		"stage": stage.Name,
		"number": stageNumber,
	}).Debug("Running cascade stage")

	// Create context with timeout
	stageCtx, cancel := context.WithTimeout(ctx, stage.Timeout)
	defer cancel()

	// Prepare command to run stage evaluation function
	cmd := exec.CommandContext(stageCtx, "go", "run",
		"-tags", "evaluator",
		ce.programPath,
		fmt.Sprintf("--stage=stage%d", stageNumber))

	// Run the command
	output, err := cmd.CombinedOutput()

	result := &types.EvaluationResult{
		ID:        fmt.Sprintf("stage%d-%s", stageNumber, stage.Name),
		Success:   false,
		Artifacts: make(map[string]string),
		Duration:  0,
	}

	// Check for timeout
	if stageCtx.Err() == context.DeadlineExceeded {
		result.Error = fmt.Sprintf("Stage %s timed out after %v", stage.Name, stage.Timeout)
		result.Artifacts["timeout"] = "true"
		result.Artifacts["timeout_duration"] = stage.Timeout.String()
		return result, fmt.Errorf("stage %s timed out", stage.Name)
	}

	// Check for execution error
	if err != nil {
		result.Error = fmt.Sprintf("Stage %s execution failed: %v", stage.Name, err)
		result.Artifacts["stderr"] = string(output)
		result.Artifacts["error"] = err.Error()
		return result, fmt.Errorf("stage execution failed: %w", err)
	}

	// Parse output to extract score
	// Expected format: "SCORE: <score>" or JSON output
	score := ce.parseScoreOutput(string(output))
	result.Score = score
	result.Artifacts["stdout"] = string(output)

	// If score is valid, mark as successful
	if score >= 0 {
		result.Success = true
	}

	ce.logger.WithFields(logrus.Fields{
		"stage": stage.Name,
		"score": score,
	}).Debug("Stage completed")

	return result, nil
}

// parseScoreOutput extracts score from stage output
func (ce *CascadeEvaluator) parseScoreOutput(output string) float64 {
	// Try to parse JSON first (simplified)
	// In a real implementation, you'd use a proper JSON parser
	lines := []string{output}

	// Look for score patterns
	for _, line := range lines {
		// Simple pattern matching for "SCORE: <value>"
		if len(line) > 7 && line[:7] == "SCORE: " {
			var score float64
			_, err := fmt.Sscanf(line[7:], "%f", &score)
			if err == nil {
				return score
			}
		}

		// Add more parsing patterns as needed
	}

	// Default score if parsing fails
	ce.logger.WithField("output", output).Warn("Could not parse score from output")
	return -1.0
}