package database

import (
	"fmt"
	"math"
	"time"

	"github.com/ishanwen-byte/openevolve-go/internal/types"
)

// Island represents an island in the island-based evolution model
// Each island maintains its own MAP-Elites grid and population
type Island struct {
	// Island identification
	ID int `json:"id"`

	// Programs in this island
	Programs map[string]*types.Program `json:"programs"`

	// MAP-Elites grid for quality-diversity
	Grid MAPGrid `json:"grid"`

	// Best program in this island
	BestProgram *types.Program `json:"best_program,omitempty"`
	BestScore   float64       `json:"best_score"`
	BestID      string        `json:"best_id"`

	// Island evolution state
	Generation int `json:"generation"`
	Migrated   int `json:"migrated"`

	// Feature statistics for scaling
	FeatureStats map[string]FeatureStats `json:"feature_stats"`
}

// FeatureStats tracks statistics for a feature dimension
type FeatureStats struct {
	Min   float64   `json:"min"`
	Max   float64   `json:"max"`
	Mean  float64   `json:"mean"`
	Std   float64   `json:"std"`
	Count int       `json:"count"`
	LastUpdate time.Time `json:"last_update"`
}

// MAPGrid implements the MAP-Elites grid for quality-diversity
type MAPGrid struct {
	// Grid configuration
	Dimensions []string          `json:"dimensions"`
	Resolution map[string]int    `json:"resolution"`
	Bounds     map[string][2]float64 `json:"bounds"`

	// Grid cells - key is a serialized feature vector
	Cells map[string]*types.Program `json:"cells"`

	// Grid statistics
	TotalCells int `json:"total_cells"`
	FilledCells int `json:"filled_cells"`
}

// NewIsland creates a new island with the given ID and configuration
func NewIsland(id int, config types.DatabaseConfig) *Island {
	// Initialize grid
	grid := MAPGrid{
		Dimensions: config.GridDimensions,
		Resolution: config.GridResolution,
		Bounds:     config.GridBounds,
		Cells:      make(map[string]*types.Program),
	}

	// Calculate total cells
	totalCells := 1
	for _, dim := range config.GridDimensions {
		if res, ok := config.GridResolution[dim]; ok {
			totalCells *= res
		} else {
			totalCells *= 10 // Default resolution
		}
	}
	grid.TotalCells = totalCells

	// Initialize feature stats
	featureStats := make(map[string]FeatureStats)
	for _, dim := range config.GridDimensions {
		featureStats[dim] = FeatureStats{
			Min:   math.Inf(1),
			Max:   math.Inf(-1),
			Count: 0,
		}
	}

	return &Island{
		ID:           id,
		Programs:     make(map[string]*types.Program),
		Grid:         grid,
		BestScore:    math.Inf(-1),
		Generation:   0,
		Migrated:     0,
		FeatureStats: featureStats,
	}
}

// AddToGrid adds a program to the MAP-Elites grid if it's better than the current occupant
func (i *Island) AddToGrid(program *types.Program) bool {
	// Calculate grid cell key
	cellKey := i.calculateCellKey(program.Features)

	// Check if cell is empty or new program is better
	existing, exists := i.Grid.Cells[cellKey]
	if !exists || program.Score > existing.Score {
		// Add to grid
		i.Grid.Cells[cellKey] = program

		// Update filled cells count
		if !exists {
			i.Grid.FilledCells++
		}

		// Update feature statistics
		i.updateFeatureStats(program)

		return true
	}

	return false
}

// GetFromGrid retrieves a program from the grid by feature vector
func (i *Island) GetFromGrid(features []float64) *types.Program {
	cellKey := i.calculateCellKey(features)
	return i.Grid.Cells[cellKey]
}

// SampleFromGrid samples a program from the filled grid cells
func (i *Island) SampleFromGrid() *types.Program {
	if len(i.Grid.Cells) == 0 {
		return nil
	}

	// Convert to slice for random sampling
	programs := make([]*types.Program, 0, len(i.Grid.Cells))
	for _, program := range i.Grid.Cells {
		programs = append(programs, program)
	}

	// Simple random sampling (can be enhanced with weighted sampling)
	if len(programs) > 0 {
		idx := time.Now().Nanosecond() % len(programs)
		return programs[idx]
	}

	return nil
}

// GetBestProgram returns the best program in this island
func (i *Island) GetBestProgram() *types.Program {
	if i.BestProgram == nil && len(i.Programs) > 0 {
		// Find best program if not cached
		for _, program := range i.Programs {
			if program.Score > i.BestScore {
				i.BestProgram = program
				i.BestScore = program.Score
				i.BestID = program.ID
			}
		}
	}

	return i.BestProgram
}

// IncrementGeneration increments the generation counter
func (i *Island) IncrementGeneration() {
	i.Generation++
}

// GetOccupancy returns the grid occupancy rate
func (i *Island) GetOccupancy() float64 {
	if i.Grid.TotalCells == 0 {
		return 0.0
	}
	return float64(i.Grid.FilledCells) / float64(i.Grid.TotalCells)
}

// calculateCellKey converts feature vector to grid cell key
func (i *Island) calculateCellKey(features []float64) string {
	if len(features) != len(i.Grid.Dimensions) {
		return ""
	}

	key := ""
	for dimIdx, dim := range i.Grid.Dimensions {
		if dimIdx >= len(features) {
			break
		}

		feature := features[dimIdx]

		// Get bounds for this dimension
		bounds, ok := i.Grid.Bounds[dim]
		if !ok {
			// Default bounds
			bounds = [2]float64{0.0, 1.0}
		}

		// Get resolution for this dimension
		resolution, ok := i.Grid.Resolution[dim]
		if !ok {
			resolution = 10 // Default resolution
		}

		// Normalize feature to [0, 1]
		normalized := (feature - bounds[0]) / (bounds[1] - bounds[0])
		if normalized < 0 {
			normalized = 0
		} else if normalized > 1 {
			normalized = 1
		}

		// Convert to grid index
		index := int(normalized * float64(resolution-1))

		key += fmt.Sprintf("%s:%d;", dim, index)
	}

	return key
}

// updateFeatureStats updates the running statistics for features
func (i *Island) updateFeatureStats(program *types.Program) {
	for dimIdx, dim := range i.Grid.Dimensions {
		if dimIdx >= len(program.Features) {
			continue
		}

		feature := program.Features[dimIdx]
		stats := i.FeatureStats[dim]

		// Update min/max
		if feature < stats.Min {
			stats.Min = feature
		}
		if feature > stats.Max {
			stats.Max = feature
		}

		// Update running mean and standard deviation
		stats.Count++
		if stats.Count == 1 {
			stats.Mean = feature
			stats.Std = 0
		} else {
			// Online algorithm for mean and std
			delta := feature - stats.Mean
			stats.Mean += delta / float64(stats.Count)
			delta2 := feature - stats.Mean
			stats.Std = math.Sqrt(((float64(stats.Count-1))*stats.Std*stats.Std + delta*delta2) / float64(stats.Count))
		}

		stats.LastUpdate = time.Now()
		i.FeatureStats[dim] = stats
	}
}

// ScaleFeatures scales features using the configured method
func (i *Island) ScaleFeatures(features []float64) []float64 {
	scaled := make([]float64, len(features))

	for dimIdx, dim := range i.Grid.Dimensions {
		if dimIdx >= len(features) {
			continue
		}

		feature := features[dimIdx]
		stats := i.FeatureStats[dim]

		if stats.Count == 0 {
			// No statistics yet, use as-is
			scaled[dimIdx] = feature
			continue
		}

		// Min-max scaling
		if stats.Max > stats.Min {
			scaled[dimIdx] = (feature - stats.Min) / (stats.Max - stats.Min)
		} else {
			scaled[dimIdx] = 0.5 // Default when all values are the same
		}

		// Clamp to [0, 1]
		if scaled[dimIdx] < 0 {
			scaled[dimIdx] = 0
		} else if scaled[dimIdx] > 1 {
			scaled[dimIdx] = 1
		}
	}

	return scaled
}