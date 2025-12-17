package evaluator

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"

	"github.com/ishanwen-byte/openevolve-go/internal/types"
)

// Evaluator handles program evaluation with support for cascade evaluation
type Evaluator struct {
	config       types.EvaluatorConfig
	programPath  string
	logger       *logrus.Logger

	// Worker pool for parallel evaluation
	workerPool   *WorkerPool
	mu           sync.RWMutex

	// Artifact storage
	artifactsDir string
	pendingArtifacts map[string]map[string]string
}

// WorkerPool manages parallel evaluation workers
type WorkerPool struct {
	maxWorkers int
	jobs       chan *EvaluationJob
	results    chan *types.EvaluationResult
	wg         sync.WaitGroup
	ctx        context.Context
	cancel     context.CancelFunc
}

// EvaluationJob represents a single evaluation task
type EvaluationJob struct {
	ID          string
	Code        string
	ProgramPath string
	Context     context.Context
	ResultChan  chan *types.EvaluationResult
}

// New creates a new Evaluator instance
func New(config types.EvaluatorConfig, programPath string) (*Evaluator, error) {
	// Validate program path
	if _, err := os.Stat(programPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("evaluation program not found: %s", programPath)
	}

	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)

	// Create artifacts directory if enabled
	var artifactsDir string
	if config.CollectArtifacts {
		artifactsDir = filepath.Join(os.TempDir(), "openevolve-eval-artifacts")
		if err := os.MkdirAll(artifactsDir, 0755); err != nil {
			logger.WithError(err).Warn("Failed to create artifacts directory")
		}
	}

	evaluator := &Evaluator{
		config:          config,
		programPath:     programPath,
		logger:          logger,
		artifactsDir:    artifactsDir,
		pendingArtifacts: make(map[string]map[string]string),
	}

	// Initialize worker pool
	evaluator.workerPool = NewWorkerPool(config.ParallelWorkers)
	go evaluator.workerPool.Start()

	logger.WithFields(logrus.Fields{
		"program":      programPath,
		"parallel":     config.ParallelWorkers,
		"cascade":      len(config.CascadeStages) > 0,
		"artifacts":    config.CollectArtifacts,
	}).Info("Initialized evaluator")

	return evaluator, nil
}

// NewWorkerPool creates a new worker pool
func NewWorkerPool(maxWorkers int) *WorkerPool {
	ctx, cancel := context.WithCancel(context.Background())
	return &WorkerPool{
		maxWorkers: maxWorkers,
		jobs:       make(chan *EvaluationJob, maxWorkers*2),
		results:    make(chan *types.EvaluationResult, maxWorkers),
		ctx:        ctx,
		cancel:     cancel,
	}
}

// Start starts the worker pool
func (wp *WorkerPool) Start() {
	for i := 0; i < wp.maxWorkers; i++ {
		wp.wg.Add(1)
		go wp.worker(i)
	}
}

// Stop stops the worker pool
func (wp *WorkerPool) Stop() {
	wp.cancel()
	close(wp.jobs)
	wp.wg.Wait()
	close(wp.results)
}

// worker processes evaluation jobs
func (wp *WorkerPool) worker(id int) {
	defer wp.wg.Done()

	for {
		select {
		case job := <-wp.jobs:
			if job == nil {
				return
			}

			result := wp.processJob(job)
			select {
			case job.ResultChan <- result:
			case <-wp.ctx.Done():
				return
			}

		case <-wp.ctx.Done():
			return
		}
	}
}

// processJob handles a single evaluation job
func (wp *WorkerPool) processJob(job *EvaluationJob) *types.EvaluationResult {
	result := &types.EvaluationResult{
		ID:       job.ID,
		Success:  false,
		Artifacts: make(map[string]string),
	}

	startTime := time.Now()
	defer func() {
		result.Duration = time.Since(startTime)
	}()

	// Create temporary file for program code
	tempFile, err := ioutil.TempFile("", fmt.Sprintf("eval-%s-*.go", job.ID))
	if err != nil {
		result.Error = fmt.Sprintf("Failed to create temp file: %v", err)
		return result
	}
	tempPath := tempFile.Name()
	defer os.Remove(tempPath)

	// Write program code to temp file
	if _, err := tempFile.Write([]byte(job.Code)); err != nil {
		result.Error = fmt.Sprintf("Failed to write program code: %v", err)
		tempFile.Close()
		return result
	}
	tempFile.Close()

	// Choose evaluation method
	if len(job.ProgramPath) > 0 {
		// Use cascade evaluation if configured
		result = wp.evaluateCascade(job.Context, tempPath, job.ProgramPath)
	} else {
		// Direct evaluation
		result = wp.evaluateDirect(job.Context, tempPath)
	}

	return result
}

// Evaluate evaluates a single program
func (e *Evaluator) Evaluate(ctx context.Context, code string) (*types.EvaluationResult, error) {
	jobID := uuid.New().String()

	// Create result channel
	resultChan := make(chan *types.EvaluationResult, 1)

	// Create job
	job := &EvaluationJob{
		ID:          jobID,
		Code:        code,
		ProgramPath: e.programPath,
		Context:     ctx,
		ResultChan:  resultChan,
	}

	// Submit job
	select {
	case e.workerPool.jobs <- job:
		// Job submitted successfully
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	// Wait for result
	select {
	case result := <-resultChan:
		// Store artifacts if enabled
		if e.config.CollectArtifacts && len(result.Artifacts) > 0 {
			e.mu.Lock()
			e.pendingArtifacts[jobID] = result.Artifacts
			e.mu.Unlock()
		}

		return result, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// EvaluateBatch evaluates multiple programs in parallel
func (e *Evaluator) EvaluateBatch(ctx context.Context, programs []string) ([]*types.EvaluationResult, error) {
	results := make([]*types.EvaluationResult, len(programs))
	errors := make([]error, len(programs))

	// Use WaitGroup to wait for all evaluations
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, e.config.ParallelWorkers)

	for i, program := range programs {
		wg.Add(1)
		go func(idx int, code string) {
			defer wg.Done()

			// Acquire semaphore
			select {
			case semaphore <- struct{}{}:
			case <-ctx.Done():
				errors[idx] = ctx.Err()
				return
			}
			defer func() { <-semaphore }()

			result, err := e.Evaluate(ctx, code)
			if err != nil {
				errors[idx] = err
				return
			}

			results[idx] = result
		}(i, program)
	}

	wg.Wait()

	// Check for errors
	for _, err := range errors {
		if err != nil {
			return results, err
		}
	}

	return results, nil
}

// evaluateDirect performs direct program evaluation
func (wp *WorkerPool) evaluateDirect(ctx context.Context, programPath string) *types.EvaluationResult {
	result := &types.EvaluationResult{
		Success:  false,
		Artifacts: make(map[string]string),
	}

	// Create context with timeout
	evalCtx, cancel := context.WithTimeout(ctx, 30*time.Second) // Default timeout
	defer cancel()

	// Run the program
	cmd := exec.CommandContext(evalCtx, "go", "run", programPath)
	output, err := cmd.CombinedOutput()

	if evalCtx.Err() == context.DeadlineExceeded {
		result.Error = "Program evaluation timed out"
		result.Artifacts["timeout"] = "true"
		return result
	}

	if err != nil {
		result.Error = fmt.Sprintf("Program execution failed: %v", err)
		result.Artifacts["stderr"] = string(output)
		return result
	}

	// Parse output for score
	result.Score = wp.parseScoreOutput(string(output))
	result.Success = result.Score >= 0
	result.Artifacts["stdout"] = string(output)

	return result
}

// evaluateCascade performs cascade evaluation
func (wp *WorkerPool) evaluateCascade(ctx context.Context, programPath string, evaluatorPath string) *types.EvaluationResult {
	// For now, implement a simple cascade evaluation
	// In a full implementation, you would load the evaluator and call cascade stages

	result := &types.EvaluationResult{
		Success:  false,
		Artifacts: make(map[string]string),
	}

	// Create context with timeout
	evalCtx, cancel := context.WithTimeout(ctx, 60*time.Second) // Default timeout
	defer cancel()

	// Run the evaluator with the program as argument
	cmd := exec.CommandContext(evalCtx, "go", "run", evaluatorPath, programPath)
	output, err := cmd.CombinedOutput()

	if evalCtx.Err() == context.DeadlineExceeded {
		result.Error = "Cascade evaluation timed out"
		result.Artifacts["timeout"] = "true"
		return result
	}

	if err != nil {
		result.Error = fmt.Sprintf("Cascade evaluation failed: %v", err)
		result.Artifacts["stderr"] = string(output)
		return result
	}

	// Try to parse JSON output first
	var evalResult struct {
		Score     float64            `json:"score"`
		Success   bool               `json:"success"`
		Artifacts map[string]string  `json:"artifacts"`
		Error     string             `json:"error"`
		Metrics   map[string]float64 `json:"metrics"`
	}

	if json.Unmarshal(output, &evalResult) == nil {
		result.Score = evalResult.Score
		result.Success = evalResult.Success
		result.Error = evalResult.Error
		if evalResult.Artifacts != nil {
			result.Artifacts = evalResult.Artifacts
		}
	} else {
		// Fallback to simple score parsing
		result.Score = wp.parseScoreOutput(string(output))
		result.Success = result.Score >= 0
		result.Artifacts["stdout"] = string(output)
	}

	return result
}

// parseScoreOutput extracts score from program output
func (wp *WorkerPool) parseScoreOutput(output string) float64 {
	// Try to parse JSON
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(output), &result); err == nil {
		if score, ok := result["score"].(float64); ok {
			return score
		}
	}

	// Simple pattern matching
	// Look for patterns like "SCORE: 0.85" or just a number
	lines := []string{output}
	for _, line := range lines {
		if len(line) > 7 && line[:7] == "SCORE: " {
			var score float64
			_, err := fmt.Sscanf(line[7:], "%f", &score)
			if err == nil {
				return score
			}
		}

		// Try to parse as JSON number
		var score float64
		if _, err := fmt.Sscanf(line, "%f", &score); err == nil {
			return score
		}
	}

	return -1.0
}

// GetArtifacts retrieves stored artifacts for a program
func (e *Evaluator) GetArtifacts(programID string) (map[string]string, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	artifacts, exists := e.pendingArtifacts[programID]
	return artifacts, exists
}

// ClearArtifacts removes stored artifacts for a program
func (e *Evaluator) ClearArtifacts(programID string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	delete(e.pendingArtifacts, programID)
}

// Close shuts down the evaluator
func (e *Evaluator) Close() {
	if e.workerPool != nil {
		e.workerPool.Stop()
	}

	e.logger.Info("Evaluator shutdown complete")
}