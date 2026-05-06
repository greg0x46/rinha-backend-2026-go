package httpapi

import (
	"fmt"
	"math"

	"github.com/greg/rinha-be-2026/internal/fraudindex"
)

const nearestNeighbors = 5

type Label = fraudindex.Label
type Reference = fraudindex.Reference

const LabelLegit = fraudindex.LabelLegit
const LabelFraud = fraudindex.LabelFraud

type Scorer struct {
	references []Reference
}

func NewScorer(references []Reference) Scorer {
	return Scorer{references: references}
}

func LoadReferences(path string) ([]Reference, error) {
	references, _, err := fraudindex.LoadBinary(path)
	if err != nil {
		return nil, fmt.Errorf("load binary references: %w", err)
	}
	return references, nil
}

func (s Scorer) Score(query Vector) FraudScoreResponse {
	if len(s.references) == 0 {
		return FraudScoreResponse{Approved: true, FraudScore: 0}
	}

	neighbors := s.nearest(query)
	frauds := 0
	found := 0
	for _, neighbor := range neighbors {
		if neighbor.index < 0 {
			continue
		}
		found++
		if s.references[neighbor.index].Label == fraudindex.LabelFraud {
			frauds++
		}
	}
	if found == 0 {
		return FraudScoreResponse{Approved: true, FraudScore: 0}
	}

	score := float64(frauds) / nearestNeighbors
	return FraudScoreResponse{
		Approved:   score < 0.6,
		FraudScore: score,
	}
}

func (s Scorer) nearest(query Vector) [nearestNeighbors]neighbor {
	best := [nearestNeighbors]neighbor{
		{index: -1, distance: float32(math.Inf(1))},
		{index: -1, distance: float32(math.Inf(1))},
		{index: -1, distance: float32(math.Inf(1))},
		{index: -1, distance: float32(math.Inf(1))},
		{index: -1, distance: float32(math.Inf(1))},
	}

	for i, reference := range s.references {
		distance := squaredDistance(query, reference.Vector)
		worst := 0
		for j := 1; j < len(best); j++ {
			if best[j].distance > best[worst].distance {
				worst = j
			}
		}
		if distance < best[worst].distance {
			best[worst] = neighbor{index: i, distance: distance}
		}
	}

	return best
}

func squaredDistance(a, b Vector) float32 {
	var distance float32
	for i := range a {
		delta := a[i] - b[i]
		distance += delta * delta
	}
	return distance
}

type neighbor struct {
	index    int
	distance float32
}
