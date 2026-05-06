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
	references     []Reference
	quantizedIndex fraudindex.QuantizedIndex
	quantized      bool
}

func NewScorer(references []Reference) Scorer {
	return Scorer{references: references}
}

func NewQuantizedScorer(index fraudindex.QuantizedIndex) Scorer {
	return Scorer{quantizedIndex: index, quantized: true}
}

func LoadReferences(path string) ([]Reference, error) {
	references, _, err := fraudindex.LoadBinary(path)
	if err != nil {
		return nil, fmt.Errorf("load binary references: %w", err)
	}
	return references, nil
}

func LoadScorer(path string) (Scorer, error) {
	index, _, err := fraudindex.LoadQuantizedBinary(path)
	if err == nil {
		return NewQuantizedScorer(index), nil
	}

	references, _, floatErr := fraudindex.LoadBinary(path)
	if floatErr == nil {
		return NewScorer(references), nil
	}

	return Scorer{}, fmt.Errorf("load quantized references: %v; load float32 references: %w", err, floatErr)
}

func (s Scorer) Score(query Vector) FraudScoreResponse {
	if s.quantized {
		return s.scoreQuantized(query)
	}

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

func (s Scorer) scoreQuantized(query Vector) FraudScoreResponse {
	if len(s.quantizedIndex.Vectors) == 0 {
		return FraudScoreResponse{Approved: true, FraudScore: 0}
	}

	neighbors := s.nearestQuantized(fraudindex.QuantizeVector(query))
	frauds := 0
	found := 0
	for _, neighbor := range neighbors {
		if neighbor.index < 0 {
			continue
		}
		found++
		if s.quantizedIndex.Labels[neighbor.index] == fraudindex.LabelFraud {
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

func (s Scorer) nearestQuantized(query fraudindex.QuantizedVector) [nearestNeighbors]quantizedNeighbor {
	best := [nearestNeighbors]quantizedNeighbor{
		{index: -1, distance: math.MaxUint64},
		{index: -1, distance: math.MaxUint64},
		{index: -1, distance: math.MaxUint64},
		{index: -1, distance: math.MaxUint64},
		{index: -1, distance: math.MaxUint64},
	}

	for i, vector := range s.quantizedIndex.Vectors {
		distance := squaredQuantizedDistance(query, vector)
		worst := 0
		for j := 1; j < len(best); j++ {
			if best[j].distance > best[worst].distance {
				worst = j
			}
		}
		if distance < best[worst].distance {
			best[worst] = quantizedNeighbor{index: i, distance: distance}
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

func squaredQuantizedDistance(a, b fraudindex.QuantizedVector) uint64 {
	var distance uint64
	for i := range a {
		delta := int64(a[i]) - int64(b[i])
		distance += uint64(delta * delta)
	}
	return distance
}

type neighbor struct {
	index    int
	distance float32
}

type quantizedNeighbor struct {
	index    int
	distance uint64
}
