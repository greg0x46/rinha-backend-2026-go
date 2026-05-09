package httpapi

import (
	"fmt"
	"math"
	"os"
	"strconv"

	"github.com/greg/rinha-be-2026/internal/fraudindex"
)

const nearestNeighbors = 5
const defaultIVFNProbe = 8
const defaultIVFBoundaryNProbe = 32
const defaultKMeansQuickProbe = 8
const defaultKMeansExpandedProbe = 20

var ivfNProbe = envInt("IVF_NPROBE", defaultIVFNProbe)
var ivfBoundaryNProbe = envInt("IVF_BOUNDARY_NPROBE", defaultIVFBoundaryNProbe)
var ivfBoundaryRetry = os.Getenv("IVF_BOUNDARY_RETRY") != "off"
var kmeansQuickProbe = envInt("KMEANS_QUICK_PROBE", defaultKMeansQuickProbe)
var kmeansExpandedProbe = envInt("KMEANS_EXPANDED_PROBE", defaultKMeansExpandedProbe)

// SetKMeansProbes overrides the package-level probe knobs at runtime.
// Intended for offline sweeps (cmd/evaluate-test); production code should
// configure via env vars instead.
func SetKMeansProbes(quick, expanded int) {
	if quick > 0 {
		kmeansQuickProbe = quick
	}
	if expanded > 0 {
		kmeansExpandedProbe = expanded
	}
}

// maxIVFNProbe is the largest nprobe ever used by Score; the scratch arrays
// for centroid selection are sized to it so the hot path never allocates.
const maxIVFNProbe = 256

func envInt(name string, fallback int) int {
	value, ok := os.LookupEnv(name)
	if !ok {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

type Label = fraudindex.Label
type Reference = fraudindex.Reference

const LabelLegit = fraudindex.LabelLegit
const LabelFraud = fraudindex.LabelFraud

type Scorer struct {
	references     []Reference
	quantizedIndex fraudindex.QuantizedIndex
	ivfIndex       fraudindex.IVFIndex
	kmeansIndex    fraudindex.KMeansIVFIndex
	quantized      bool
	ivf            bool
	kmeans         bool
}

func NewScorer(references []Reference) Scorer {
	return Scorer{references: references}
}

func NewQuantizedScorer(index fraudindex.QuantizedIndex) Scorer {
	return Scorer{quantizedIndex: index, quantized: true}
}

func NewIVFScorer(index fraudindex.IVFIndex) Scorer {
	return Scorer{ivfIndex: index, ivf: true}
}

func NewKMeansIVFScorer(index fraudindex.KMeansIVFIndex) Scorer {
	return Scorer{kmeansIndex: index, kmeans: true}
}

func LoadReferences(path string) ([]Reference, error) {
	references, _, err := fraudindex.LoadBinary(path)
	if err != nil {
		return nil, fmt.Errorf("load binary references: %w", err)
	}
	return references, nil
}

func LoadScorer(path string) (Scorer, error) {
	kmeansIndex, _, kmeansErr := fraudindex.LoadKMeansIVFBinary(path)
	if kmeansErr == nil {
		return NewKMeansIVFScorer(kmeansIndex), nil
	}

	ivfIndex, _, ivfErr := fraudindex.LoadIVFBinary(path)
	if ivfErr == nil {
		return NewIVFScorer(ivfIndex), nil
	}

	index, _, err := fraudindex.LoadQuantizedBinary(path)
	if err == nil {
		return NewQuantizedScorer(index), nil
	}

	references, _, floatErr := fraudindex.LoadBinary(path)
	if floatErr == nil {
		return NewScorer(references), nil
	}

	return Scorer{}, fmt.Errorf("load kmeans ivf references: %v; load ivf references: %v; load quantized references: %v; load float32 references: %w", kmeansErr, ivfErr, err, floatErr)
}

// Frauds returns the number of fraud labels (0..nearestNeighbors) found among
// the closest neighbors of query. It is the primitive the API hot path uses to
// pick a pre-formatted JSON response.
func (s Scorer) Frauds(query Vector) int {
	if s.kmeans {
		return s.fraudsKMeansIVF(query)
	}
	if s.ivf {
		return s.fraudsIVF(query)
	}
	if s.quantized {
		return s.fraudsQuantized(query)
	}
	return s.fraudsExact(query)
}

func (s Scorer) fraudsKMeansIVF(query Vector) int {
	if len(s.kmeansIndex.Blocks) == 0 {
		return 0
	}
	neighbors := s.nearestKMeansIVF(query, kmeansQuickProbe)
	frauds := countQuantizedFrauds(neighbors, s.kmeansIndex.BlockLabels)
	if frauds == 2 || frauds == 3 {
		neighbors = s.nearestKMeansIVF(query, kmeansExpandedProbe)
		frauds = countQuantizedFrauds(neighbors, s.kmeansIndex.BlockLabels)
	}
	return frauds
}

func (s Scorer) Score(query Vector) FraudScoreResponse {
	frauds := s.Frauds(query)
	score := float64(frauds) / nearestNeighbors
	return FraudScoreResponse{Approved: score < 0.6, FraudScore: score}
}

func (s Scorer) fraudsExact(query Vector) int {
	if len(s.references) == 0 {
		return 0
	}
	neighbors := s.nearest(query)
	frauds := 0
	for _, neighbor := range neighbors {
		if neighbor.index < 0 {
			continue
		}
		if s.references[neighbor.index].Label == fraudindex.LabelFraud {
			frauds++
		}
	}
	return frauds
}

func (s Scorer) fraudsQuantized(query Vector) int {
	if len(s.quantizedIndex.Vectors) == 0 {
		return 0
	}
	neighbors := s.nearestQuantized(fraudindex.QuantizeVector(query))
	return countQuantizedFrauds(neighbors, s.quantizedIndex.Labels)
}

func (s Scorer) fraudsIVF(query Vector) int {
	if len(s.ivfIndex.Vectors) == 0 {
		return 0
	}
	quantizedQuery := fraudindex.QuantizeVector(query)
	neighbors := s.nearestIVF(quantizedQuery, ivfNProbe)
	frauds := countQuantizedFrauds(neighbors, s.ivfIndex.Labels)
	if ivfBoundaryRetry && (frauds == 2 || frauds == 3) {
		neighbors = s.nearestIVF(quantizedQuery, ivfBoundaryNProbe)
		frauds = countQuantizedFrauds(neighbors, s.ivfIndex.Labels)
	}
	return frauds
}

func countQuantizedFrauds(neighbors [nearestNeighbors]quantizedNeighbor, labels []fraudindex.Label) int {
	frauds := 0
	for _, neighbor := range neighbors {
		if neighbor.index < 0 {
			continue
		}
		if labels[neighbor.index] == fraudindex.LabelFraud {
			frauds++
		}
	}
	return frauds
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

func (s Scorer) nearestIVF(query fraudindex.QuantizedVector, nprobe int) [nearestNeighbors]quantizedNeighbor {
	best := [nearestNeighbors]quantizedNeighbor{
		{index: -1, distance: math.MaxUint64},
		{index: -1, distance: math.MaxUint64},
		{index: -1, distance: math.MaxUint64},
		{index: -1, distance: math.MaxUint64},
		{index: -1, distance: math.MaxUint64},
	}
	if nprobe > len(s.ivfIndex.Centroids) {
		nprobe = len(s.ivfIndex.Centroids)
	}
	if nprobe <= 0 {
		return best
	}

	var listsBuf [maxIVFNProbe]quantizedNeighbor
	if nprobe > len(listsBuf) {
		nprobe = len(listsBuf)
	}
	lists := s.nearestIVFLists(query, nprobe, listsBuf[:nprobe])
	for _, list := range lists {
		start := s.ivfIndex.Offsets[list.index]
		end := s.ivfIndex.Offsets[list.index+1]
		for i := start; i < end; i++ {
			distance := squaredQuantizedDistance(query, s.ivfIndex.Vectors[int(i)])
			worst := 0
			for j := 1; j < len(best); j++ {
				if best[j].distance > best[worst].distance {
					worst = j
				}
			}
			if distance < best[worst].distance {
				best[worst] = quantizedNeighbor{index: int(i), distance: distance}
			}
		}
	}
	return best
}

func (s Scorer) nearestIVFLists(query fraudindex.QuantizedVector, nprobe int, best []quantizedNeighbor) []quantizedNeighbor {
	for i := range best {
		best[i] = quantizedNeighbor{index: -1, distance: math.MaxUint64}
	}
	for i, centroid := range s.ivfIndex.Centroids {
		distance := squaredQuantizedDistance(query, centroid)
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

func (s Scorer) nearestKMeansIVF(query Vector, nprobe int) [nearestNeighbors]quantizedNeighbor {
	best := [nearestNeighbors]quantizedNeighbor{
		{index: -1, distance: math.MaxUint64},
		{index: -1, distance: math.MaxUint64},
		{index: -1, distance: math.MaxUint64},
		{index: -1, distance: math.MaxUint64},
		{index: -1, distance: math.MaxUint64},
	}
	if nprobe > len(s.kmeansIndex.Centroids) {
		nprobe = len(s.kmeansIndex.Centroids)
	}
	if nprobe <= 0 {
		return best
	}

	var listsBuf [maxIVFNProbe]floatNeighbor
	if nprobe > len(listsBuf) {
		nprobe = len(listsBuf)
	}
	lists := s.nearestKMeansLists(query, nprobe, listsBuf[:nprobe])
	quantizedQuery := fraudindex.QuantizeVector(query)
	var broadcastedQuery [fraudindex.KMeansBlockStride]int16
	fraudindex.BroadcastQuery(quantizedQuery, &broadcastedQuery)
	blocks := s.kmeansIndex.Blocks
	for _, list := range lists {
		listIdx := list.index
		blockStart := s.kmeansIndex.BlockListOffsets[listIdx]
		blockEnd := s.kmeansIndex.BlockListOffsets[listIdx+1]
		if blockStart == blockEnd {
			continue
		}
		listSize := s.kmeansIndex.Offsets[listIdx+1] - s.kmeansIndex.Offsets[listIdx]
		validLast := int(listSize % fraudindex.KMeansBlockSize)
		fullEnd := blockEnd
		if validLast != 0 {
			fullEnd = blockEnd - 1
		}
		for b := blockStart; b < fullEnd; b++ {
			updateTop5FromBlock(&best, &broadcastedQuery, blocks, b, fraudindex.KMeansBlockSize)
		}
		if validLast != 0 {
			updateTop5FromBlock(&best, &broadcastedQuery, blocks, fullEnd, validLast)
		}
	}
	return best
}

// pruneChunks splits the 14-dim distance into three SIMD-sized chunks. The
// scan computes each chunk in turn and bails out the moment all valid lanes
// already exceed the current top-5 worst — the structural insight from
// the rinha-2026 baseline analysis. With high probe counts (quick>=4) the
// scan visits many more blocks but most reject in the first chunk, so the
// average per-request score time stays bounded.
var pruneChunks = [...]struct{ start, count int }{
	{0, 4}, {4, 4}, {8, 6},
}

// updateTop5FromBlock computes per-lane squared distances for the
// validLanes (1..KMeansBlockSize) reference vectors packed at blockIdx
// and merges them into best. Distance is built up in three chunks; after
// each chunk the current min(partial) is compared against the worst slot
// in best, and the rest of the block is skipped if every lane is already
// non-improving.
func updateTop5FromBlock(best *[nearestNeighbors]quantizedNeighbor, query *[fraudindex.KMeansBlockStride]int16, blocks []int16, blockIdx uint32, validLanes int) {
	base := int(blockIdx) * fraudindex.KMeansBlockStride
	blockPtr := (*[fraudindex.KMeansBlockStride]int16)(blocks[base : base+fraudindex.KMeansBlockStride])

	worst := 0
	for j := 1; j < len(best); j++ {
		if best[j].distance > best[worst].distance {
			worst = j
		}
	}
	worstDist := best[worst].distance

	var partial [fraudindex.KMeansBlockSize]uint64
	for ci, chunk := range pruneChunks {
		fraudindex.BlockSquaredDistancePartial(query, blockPtr, chunk.start, chunk.count, &partial)
		if ci == len(pruneChunks)-1 {
			break
		}
		// Find the smallest partial across the valid lanes — if even the
		// best-so-far lane already exceeds worstDist, no lane can win
		// the remaining 14-d-start dims and the block is dead.
		minPartial := uint64(math.MaxUint64)
		for l := 0; l < validLanes; l++ {
			if partial[l] < minPartial {
				minPartial = partial[l]
			}
		}
		if minPartial >= worstDist {
			return
		}
	}

	for l := 0; l < validLanes; l++ {
		d := partial[l]
		if d >= worstDist {
			continue
		}
		best[worst] = quantizedNeighbor{
			index:    int(blockIdx)*fraudindex.KMeansBlockSize + l,
			distance: d,
		}
		// Recompute worst slot after each insertion.
		worst = 0
		for j := 1; j < len(best); j++ {
			if best[j].distance > best[worst].distance {
				worst = j
			}
		}
		worstDist = best[worst].distance
	}
}

func (s Scorer) nearestKMeansLists(query Vector, nprobe int, best []floatNeighbor) []floatNeighbor {
	for i := range best {
		best[i] = floatNeighbor{index: -1, distance: float32(math.Inf(1))}
	}
	for i, centroid := range s.kmeansIndex.Centroids {
		distance := squaredDistance(query, centroid)
		worst := 0
		for j := 1; j < len(best); j++ {
			if best[j].distance > best[worst].distance {
				worst = j
			}
		}
		if distance < best[worst].distance {
			best[worst] = floatNeighbor{index: i, distance: distance}
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

type floatNeighbor struct {
	index    int
	distance float32
}
