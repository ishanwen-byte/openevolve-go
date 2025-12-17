package database

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"

	"github.com/ishanwen-byte/openevolve-go/internal/types"
)

// ProgramDatabase implements the main database for OpenEvolve
// It combines MAP-Elites algorithm with island-based evolution
type ProgramDatabase struct {
	// Configuration
	config types.DatabaseConfig

	// Storage
	mu sync.RWMutex

	// All programs indexed by ID
	programs map[string]*types.Program

	// Islands for parallel evolution
	islands []*Island

	// Global best program
	globalBest *types.Program
	globalBestScore float64

	// Evolution state
	currentIsland int
	lastIteration int
	lastMigrationGeneration int

	// Statistics
	stats types.EvolutionStats

	// Checkpointing
	checkpointDir string

	// Logger
	logger *logrus.Logger
}

// New creates a new ProgramDatabase with the given configuration
func New(config types.DatabaseConfig, checkpointDir string) *ProgramDatabase {
	// Initialize logger
	logger := logrus.New()
	if config.OutputDir != "" {
		if err := os.MkdirAll(config.OutputDir, 0755); err != nil {
			logger.WithError(err).Warn("Failed to create output directory")
		}
	}

	db := &ProgramDatabase{
		config:      config,
		programs:    make(map[string]*types.Program),
		islands:     make([]*Island, config.NumIslands),
		globalBestScore: math.Inf(-1),
		currentIsland: 0,
		lastIteration: 0,
		lastMigrationGeneration: 0,
		checkpointDir: checkpointDir,
		logger: logger,
		stats: types.EvolutionStats{
			StartTime: time.Now(),
		},
	}

	// Initialize islands
	for i := 0; i < config.NumIslands; i++ {
		db.islands[i] = NewIsland(i, config)
	}

	// Set random seed if specified
	if config.OutputDir != "" {
		// Use a deterministic seed based on output dir for reproducibility
		seed := time.Now().UnixNano()
		rand.Seed(seed)
		logger.Debugf("Database: Set random seed to %d", seed)
	}

	logger.Info(fmt.Sprintf("Initialized program database with %d islands", config.NumIslands))

	return db
}

// AddProgram adds a new program to the database
func (db *ProgramDatabase) AddProgram(program *types.Program, iteration int) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	// Ensure program has ID
	if program.ID == "" {
		program.ID = uuid.New().String()
	}

	// Set timestamp if not set
	if program.CreatedAt.IsZero() {
		now := time.Now()
		program.CreatedAt = now
		program.UpdatedAt = now
	}

	// Add to global programs map
	db.programs[program.ID] = program

	// Determine target island
	targetIsland := db.currentIsland
	if program.IslandID >= 0 && program.IslandID < len(db.islands) {
		targetIsland = program.IslandID
	}

	// Add to island
	island := db.islands[targetIsland]
	island.Programs[program.ID] = program

	// Scale features and add to MAP-Elites grid
	scaledFeatures := island.ScaleFeatures(program.Features)
	program.Features = scaledFeatures
	island.AddToGrid(program)

	// Update island best
	if program.Score > island.BestScore {
		island.BestProgram = program
		island.BestScore = program.Score
		island.BestID = program.ID
	}

	// Update global best
	if program.Score > db.globalBestScore {
		db.globalBest = program
		db.globalBestScore = program.Score
		programID := program.ID
		if len(programID) > 8 {
			programID = programID[:8]
		}
		db.logger.WithFields(logrus.Fields{
			"score":    program.Score,
			"program":  programID,
			"island":   targetIsland,
			"iteration": iteration,
		}).Info("New global best program found")
	}

	// Update statistics
	db.stats.TotalEvaluations++
	if program.Score > 0 { // Assume positive score means success
		db.stats.SuccessfulEvals++
	} else {
		db.stats.FailedEvals++
	}
	db.stats.LastUpdate = time.Now()

	// Rotate to next island
	db.currentIsland = (db.currentIsland + 1) % len(db.islands)

	return nil
}

// GetProgram retrieves a program by ID
func (db *ProgramDatabase) GetProgram(id string) (*types.Program, bool) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	program, exists := db.programs[id]
	return program, exists
}

// SampleFromIsland samples a program from the specified island
func (db *ProgramDatabase) SampleFromIsland(islandID int) (*types.Program, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	if islandID < 0 || islandID >= len(db.islands) {
		return nil, fmt.Errorf("invalid island ID: %d", islandID)
	}

	island := db.islands[islandID]

	// First try to sample from MAP-Elites grid
	program := island.SampleFromGrid()
	if program != nil {
		return program, nil
	}

	// Fallback to sampling from island population
	if len(island.Programs) > 0 {
		// Convert to slice for random sampling
		programs := make([]*types.Program, 0, len(island.Programs))
		for _, p := range island.Programs {
			programs = append(programs, p)
		}

		idx := rand.Intn(len(programs))
		return programs[idx], nil
	}

	return nil, fmt.Errorf("island %d is empty", islandID)
}

// SampleMultiple samples multiple programs, one from each island
func (db *ProgramDatabase) SampleMultiple(count int) ([]*types.Program, error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	if count <= 0 {
		return nil, fmt.Errorf("invalid sample count: %d", count)
	}

	programs := make([]*types.Program, 0, count)

	// Sample from different islands
	for i := 0; i < count && i < len(db.islands); i++ {
		islandID := (db.currentIsland + i) % len(db.islands)
		island := db.islands[islandID]

		// Try MAP-Elites first
		program := island.SampleFromGrid()
		if program != nil {
			programs = append(programs, program)
			continue
		}

		// Fallback to best program
		if island.BestProgram != nil {
			programs = append(programs, island.BestProgram)
		}
	}

	// If we still need more programs, sample globally
	for len(programs) < count && len(db.programs) > 0 {
		// Sample random program from global pool
		idx := rand.Intn(len(db.programs))
		for _, program := range db.programs {
			if idx == 0 {
				programs = append(programs, program)
				break
			}
			idx--
		}
	}

	return programs, nil
}

// MigratePrograms performs migration between islands
func (db *ProgramDatabase) MigratePrograms() error {
	db.mu.Lock()
	defer db.mu.Unlock()

	if len(db.islands) < 2 {
		return nil // No migration needed with single island
	}

	migrated := 0

	// Ring topology migration - each island migrates to next
	for i, island := range db.islands {
		targetIsland := db.islands[(i+1)%len(db.islands)]

		// Select best programs for migration
		candidates := make([]*types.Program, 0)
		for _, program := range island.Programs {
			if program.Score > island.BestScore*0.8 { // Migrate top 20%
				candidates = append(candidates, program)
			}
		}

		// Migrate subset of candidates
		toMigrate := int(float64(len(candidates)) * db.config.MigrationRate)
		if toMigrate < 1 && len(candidates) > 0 {
			toMigrate = 1
		}

		for j := 0; j < toMigrate && j < len(candidates); j++ {
			program := candidates[j]

			// Move to target island
			delete(island.Programs, program.ID)
			program.IslandID = targetIsland.ID
			targetIsland.Programs[program.ID] = program
			targetIsland.AddToGrid(program)

			migrated++
		}

		island.Migrated += migrated
	}

	db.lastMigrationGeneration = db.islands[0].Generation

	db.logger.WithField("migrated", migrated).Info("Completed island migration")

	return nil
}

// GetGlobalBest returns the globally best program
func (db *ProgramDatabase) GetGlobalBest() *types.Program {
	db.mu.RLock()
	defer db.mu.RUnlock()

	return db.globalBest
}

// GetIslandBest returns the best program from each island
func (db *ProgramDatabase) GetIslandBest() []*types.Program {
	db.mu.RLock()
	defer db.mu.RUnlock()

	best := make([]*types.Program, 0, len(db.islands))
	for _, island := range db.islands {
		if island.BestProgram != nil {
			best = append(best, island.BestProgram)
		}
	}

	return best
}

// UpdateGeneration increments generation counter for all islands
func (db *ProgramDatabase) UpdateGeneration() {
	db.mu.Lock()
	defer db.mu.Unlock()

	for _, island := range db.islands {
		island.IncrementGeneration()
	}

	// Check if migration is needed
	if db.islands[0].Generation-db.lastMigrationGeneration >= db.config.MigrationInterval {
		go db.MigratePrograms() // Async migration
	}
}

// SaveCheckpoint saves the database state to a checkpoint file
func (db *ProgramDatabase) SaveCheckpoint(iteration int) error {
	db.mu.RLock()
	defer db.mu.RUnlock()

	if db.checkpointDir == "" {
		return nil
	}

	// Create checkpoint
	checkpoint := &types.Checkpoint{
		Version:    "1.0",
		CreatedAt:  time.Now(),
		Iteration:  iteration,
		Generation: db.islands[0].Generation,
		Islands:    make(map[int]*types.Island),
		GlobalBest: db.globalBest,
		Stats:      db.stats,
	}

	// Convert islands to types.Island
	for _, island := range db.islands {
		// Convert MAPGrid
		grid := types.MAPGrid{
			Dimensions: island.Grid.Dimensions,
			Resolution: island.Grid.Resolution,
			Bounds:     island.Grid.Bounds,
			Cells:      island.Grid.Cells,
			TotalCells: island.Grid.TotalCells,
			FilledCells: island.Grid.FilledCells,
		}

		checkpoint.Islands[island.ID] = &types.Island{
			ID:         island.ID,
			Programs:   island.Programs,
			Grid:       grid,
			BestScore:  island.BestScore,
			BestID:     island.BestID,
			Generation: island.Generation,
			Migrated:   island.Migrated,
		}
	}

	// Serialize to JSON
	data, err := json.MarshalIndent(checkpoint, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal checkpoint: %w", err)
	}

	// Create checkpoint directory
	if err := os.MkdirAll(db.checkpointDir, 0755); err != nil {
		return fmt.Errorf("failed to create checkpoint directory: %w", err)
	}

	// Write checkpoint file
	checkpointFile := filepath.Join(db.checkpointDir, fmt.Sprintf("checkpoint_%d.json", iteration))
	if err := ioutil.WriteFile(checkpointFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write checkpoint file: %w", err)
	}

	// Also write latest checkpoint
	latestFile := filepath.Join(db.checkpointDir, "latest.json")
	if err := ioutil.WriteFile(latestFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write latest checkpoint: %w", err)
	}

	db.logger.WithFields(logrus.Fields{
		"iteration": iteration,
		"file":      checkpointFile,
	}).Info("Saved checkpoint")

	return nil
}

// LoadCheckpoint loads database state from a checkpoint file
func (db *ProgramDatabase) LoadCheckpoint(checkpointPath string) error {
	data, err := ioutil.ReadFile(checkpointPath)
	if err != nil {
		return fmt.Errorf("failed to read checkpoint file: %w", err)
	}

	var checkpoint types.Checkpoint
	if err := json.Unmarshal(data, &checkpoint); err != nil {
		return fmt.Errorf("failed to unmarshal checkpoint: %w", err)
	}

	db.mu.Lock()
	defer db.mu.Unlock()

	// Restore programs
	db.programs = make(map[string]*types.Program)
	for _, island := range checkpoint.Islands {
		for _, program := range island.Programs {
			db.programs[program.ID] = program
		}
	}

	// Restore islands
	db.islands = make([]*Island, len(checkpoint.Islands))
	for id, islandData := range checkpoint.Islands {
		island := NewIsland(id, db.config)
		island.Programs = islandData.Programs

		// Convert types.MAPGrid to MAPGrid
		island.Grid = MAPGrid{
			Dimensions: islandData.Grid.Dimensions,
			Resolution: islandData.Grid.Resolution,
			Bounds:     islandData.Grid.Bounds,
			Cells:      islandData.Grid.Cells,
			TotalCells: islandData.Grid.TotalCells,
			FilledCells: islandData.Grid.FilledCells,
		}

		island.BestScore = islandData.BestScore
		island.BestID = islandData.BestID
		island.Generation = islandData.Generation
		island.Migrated = islandData.Migrated

		// Restore best program reference
		if islandData.BestID != "" {
			island.BestProgram = island.Programs[islandData.BestID]
		}

		db.islands[id] = island
	}

	// Restore global best
	db.globalBest = checkpoint.GlobalBest
	if db.globalBest != nil {
		db.globalBestScore = db.globalBest.Score
	}

	// Restore statistics
	db.stats = checkpoint.Stats
	db.lastIteration = checkpoint.Iteration

	db.logger.WithFields(logrus.Fields{
		"iteration": checkpoint.Iteration,
		"programs":  len(db.programs),
		"file":      checkpointPath,
	}).Info("Loaded checkpoint")

	return nil
}

// GetStats returns current evolution statistics
func (db *ProgramDatabase) GetStats() types.EvolutionStats {
	db.mu.RLock()
	defer db.mu.RUnlock()

	stats := db.stats
	stats.Duration = time.Since(db.stats.StartTime)

	// Calculate average score
	if db.stats.TotalEvaluations > 0 {
		totalScore := 0.0
		count := 0
		for _, program := range db.programs {
			totalScore += program.Score
			count++
		}
		if count > 0 {
			stats.AvgScore = totalScore / float64(count)
		}
	}

	stats.BestScore = db.globalBestScore

	return stats
}

// GetCurrentIsland returns the current island ID
func (db *ProgramDatabase) GetCurrentIsland() int {
	db.mu.RLock()
	defer db.mu.RUnlock()
	return db.currentIsland
}