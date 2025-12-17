package database

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ishanwen-byte/openevolve-go/internal/types"
)

func TestNewIsland(t *testing.T) {
	config := types.DatabaseConfig{
		NumIslands:     4,
		GridDimensions: []string{"complexity", "diversity"},
		GridResolution: map[string]int{"complexity": 10, "diversity": 10},
		GridBounds:     map[string][2]float64{"complexity": {0, 1}, "diversity": {0, 1}},
	}

	island := NewIsland(0, config)

	assert.NotNil(t, island)
	assert.Equal(t, 0, island.ID)
	assert.NotNil(t, island.Programs)
	assert.Equal(t, 10*10, island.Grid.TotalCells)
	assert.Equal(t, 0, island.Grid.FilledCells)
	assert.Equal(t, 0, island.Generation)
}

func TestIslandAddToGrid(t *testing.T) {
	config := types.DatabaseConfig{
		GridDimensions: []string{"complexity", "diversity"},
		GridResolution: map[string]int{"complexity": 5, "diversity": 5},
		GridBounds:     map[string][2]float64{"complexity": {0, 1}, "diversity": {0, 1}},
	}

	island := NewIsland(0, config)

	// Create test program
	program := &types.Program{
		ID:       "test1",
		Code:     "func test() {}",
		Score:    0.8,
		Features: []float64{0.3, 0.7}, // complexity=0.3, diversity=0.7
	}

	// Add to grid
	added := island.AddToGrid(program)
	assert.True(t, added)
	assert.Equal(t, 1, island.Grid.FilledCells)

	// Add another program with same features but higher score
	program2 := &types.Program{
		ID:       "test2",
		Code:     "func test2() {}",
		Score:    0.9,
		Features: []float64{0.3, 0.7},
	}

	added = island.AddToGrid(program2)
	assert.True(t, added) // Should replace the first program
	assert.Equal(t, 1, island.Grid.FilledCells) // Still only one cell filled

	// Verify the better program is in the cell
	cellKey := island.calculateCellKey(program.Features)
	stored := island.Grid.Cells[cellKey]
	assert.Equal(t, "test2", stored.ID)
	assert.Equal(t, 0.9, stored.Score)
}

func TestIslandGetBestProgram(t *testing.T) {
	island := NewIsland(0, types.DatabaseConfig{})

	// Initially no programs
	assert.Nil(t, island.GetBestProgram())

	// Add programs
	program1 := &types.Program{ID: "test1", Score: 0.5}
	program2 := &types.Program{ID: "test2", Score: 0.8}

	island.Programs["test1"] = program1
	island.Programs["test2"] = program2

	// Should return the best
	best := island.GetBestProgram()
	assert.NotNil(t, best)
	assert.Equal(t, "test2", best.ID)
	assert.Equal(t, 0.8, best.Score)
}

func TestProgramDatabase_New(t *testing.T) {
	config := types.DatabaseConfig{
		NumIslands:     3,
		GridDimensions: []string{"complexity"},
		GridResolution: map[string]int{"complexity": 10},
		GridBounds:     map[string][2]float64{"complexity": {0, 1}},
	}

	db := New(config, "")

	assert.NotNil(t, db)
	assert.Equal(t, 3, len(db.islands))
	assert.Equal(t, 0, db.currentIsland)
	assert.NotNil(t, db.programs)
}

func TestProgramDatabase_AddProgram(t *testing.T) {
	config := types.DatabaseConfig{
		NumIslands: 2,
	}

	db := New(config, "")

	program := &types.Program{
		ID:       "test1",
		Code:     "func test() {}",
		Score:    0.7,
		Features: []float64{0.5},
	}

	err := db.AddProgram(program, 1)
	require.NoError(t, err)

	// Verify program was added
	stored, exists := db.GetProgram("test1")
	assert.True(t, exists)
	assert.Equal(t, program, stored)

	// Should be global best
	best := db.GetGlobalBest()
	assert.Equal(t, program, best)
}

func TestProgramDatabase_SampleFromIsland(t *testing.T) {
	config := types.DatabaseConfig{
		NumIslands:     2,
		GridDimensions: []string{"complexity"},
		GridResolution: map[string]int{"complexity": 5},
		GridBounds:     map[string][2]float64{"complexity": {0, 1}},
	}

	db := New(config, "")

	// Add a program to island 0
	program := &types.Program{
		ID:       "test1",
		Code:     "func test() {}",
		Score:    0.8,
		Features: []float64{0.5},
		IslandID: 0,
	}

	err := db.AddProgram(program, 1)
	require.NoError(t, err)

	// Sample from island 0
	sampled, err := db.SampleFromIsland(0)
	require.NoError(t, err)
	assert.Equal(t, program, sampled)

	// Sampling from empty island should return error
	_, err = db.SampleFromIsland(1)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "is empty")
}

func TestProgramDatabase_Migration(t *testing.T) {
	config := types.DatabaseConfig{
		NumIslands:        3,
		MigrationInterval: 1,
		MigrationRate:     0.5,
		GridDimensions:    []string{"complexity"},
		GridResolution:    map[string]int{"complexity": 5},
		GridBounds:        map[string][2]float64{"complexity": {0, 1}},
	}

	db := New(config, "")

	// Add programs to islands
	for i := 0; i < 3; i++ {
		for j := 0; j < 4; j++ {
			program := &types.Program{
				ID:       fmt.Sprintf("island%d_prog%d", i, j),
				Code:     fmt.Sprintf("func test%d() {}", j),
				Score:    float64(j) * 0.1,
				Features: []float64{float64(j) * 0.1},
				IslandID: i,
			}
			db.AddProgram(program, 1)
		}
	}

	// Perform migration
	err := db.MigratePrograms()
	require.NoError(t, err)

	// Verify programs were migrated
	// Each island should now have programs from neighboring islands
	totalPrograms := 0
	for _, island := range db.islands {
		totalPrograms += len(island.Programs)
	}
	assert.Equal(t, 12, totalPrograms) // Total should remain the same
}

func TestProgramDatabase_SaveAndLoadCheckpoint(t *testing.T) {
	// Create temporary directory for checkpoints
	tempDir := t.TempDir()

	config := types.DatabaseConfig{
		NumIslands:        2,
		MigrationInterval: 10,
		MigrationRate:     0.1,
		GridDimensions:    []string{"complexity"},
		GridResolution:    map[string]int{"complexity": 5},
		GridBounds:        map[string][2]float64{"complexity": {0, 1}},
	}

	// Create original database
	db1 := New(config, tempDir)

	// Add programs
	program1 := &types.Program{
		ID:       "test1",
		Code:     "func test1() {}",
		Score:    0.8,
		Features: []float64{0.5},
		IslandID: 0,
	}
	program2 := &types.Program{
		ID:       "test2",
		Code:     "func test2() {}",
		Score:    0.9,
		Features: []float64{0.7},
		IslandID: 1,
	}

	err := db1.AddProgram(program1, 5)
	require.NoError(t, err)
	err = db1.AddProgram(program2, 5)
	require.NoError(t, err)

	// Save checkpoint
	err = db1.SaveCheckpoint(5)
	require.NoError(t, err)

	// Create new database and load checkpoint
	db2 := New(config, tempDir)
	checkpointFile := tempDir + "/checkpoint_5.json"
	err = db2.LoadCheckpoint(checkpointFile)
	require.NoError(t, err)

	// Verify programs were loaded
	loaded1, exists := db2.GetProgram("test1")
	assert.True(t, exists)
	assert.Equal(t, program1.Code, loaded1.Code)
	assert.Equal(t, program1.Score, loaded1.Score)

	loaded2, exists := db2.GetProgram("test2")
	assert.True(t, exists)
	assert.Equal(t, program2.Code, loaded2.Code)
	assert.Equal(t, program2.Score, loaded2.Score)

	// Verify global best
	best := db2.GetGlobalBest()
	assert.Equal(t, "test2", best.ID) // Should be the higher scoring program
}

func TestProgramDatabase_GetStats(t *testing.T) {
	config := types.DatabaseConfig{
		NumIslands: 1,
	}

	db := New(config, "")

	// Add some programs
	for i := 0; i < 3; i++ {
		program := &types.Program{
			ID:    fmt.Sprintf("test%d", i),
			Code:  fmt.Sprintf("func test%d() {}", i),
			Score: float64(i) * 0.3,
		}
		db.AddProgram(program, 1)
	}

	stats := db.GetStats()
	assert.Equal(t, int64(3), stats.TotalEvaluations)
	assert.Equal(t, int64(2), stats.SuccessfulEvals) // Only scores > 0 count as successful
	assert.Equal(t, 0.6, stats.BestScore)            // Highest score
	assert.Equal(t, 0.3, stats.AvgScore)             // Average of 0, 0.3, 0.6
}

func TestIslandCalculateCellKey(t *testing.T) {
	config := types.DatabaseConfig{
		GridDimensions: []string{"complexity", "diversity"},
		GridResolution: map[string]int{"complexity": 5, "diversity": 5},
		GridBounds:     map[string][2]float64{"complexity": {0, 1}, "diversity": {0, 1}},
	}

	island := NewIsland(0, config)

	// Test feature vector mapping
	features := []float64{0.0, 0.0} // Should map to cell 0,0
	key := island.calculateCellKey(features)
	assert.Equal(t, "complexity:0;diversity:0;", key)

	features = []float64{1.0, 1.0} // Should map to cell 4,4
	key = island.calculateCellKey(features)
	assert.Equal(t, "complexity:4;diversity:4;", key)

	features = []float64{0.5, 0.5} // Should map to cell 2,2
	key = island.calculateCellKey(features)
	assert.Equal(t, "complexity:2;diversity:2;", key)
}

func TestIslandScaleFeatures(t *testing.T) {
	island := NewIsland(0, types.DatabaseConfig{
		GridDimensions: []string{"complexity", "diversity"},
		GridResolution: map[string]int{"complexity": 10, "diversity": 10},
		GridBounds:     map[string][2]float64{"complexity": {0, 10}, "diversity": {-5, 5}},
	})

	// Test with no statistics (should return as-is)
	features := []float64{5.0, 0.0}
	scaled := island.ScaleFeatures(features)
	assert.Equal(t, []float64{5.0, 0.0}, scaled)

	// Add some programs to build statistics
	program1 := &types.Program{Features: []float64{2.0, -2.0}}
	program2 := &types.Program{Features: []float64{8.0, 2.0}}
	island.updateFeatureStats(program1)
	island.updateFeatureStats(program2)

	// Now scaling should work
	features = []float64{5.0, 0.0}
	scaled = island.ScaleFeatures(features)
	assert.Equal(t, 0.5, scaled[0])  // (5-2)/(8-2) = 0.5
	assert.Equal(t, 0.5, scaled[1])  // (0-(-2))/(2-(-2)) = 0.5
}

func BenchmarkProgramDatabase_AddProgram(b *testing.B) {
	config := types.DatabaseConfig{
		NumIslands: 10,
	}

	db := New(config, "")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		program := &types.Program{
			ID:       fmt.Sprintf("test%d", i),
			Code:     fmt.Sprintf("func test%d() {}", i),
			Score:    rand.Float64(),
			Features: []float64{rand.Float64(), rand.Float64()},
		}
		db.AddProgram(program, i)
	}
}