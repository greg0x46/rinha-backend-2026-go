package fraudindex

type Vector [14]float32

type QuantizedVector [14]int16

type Label uint8

const (
	LabelLegit Label = iota
	LabelFraud
)

type Reference struct {
	Vector Vector
	Label  Label
}

type QuantizedIndex struct {
	Vectors []QuantizedVector
	Labels  []Label
}

type IVFIndex struct {
	Centroids []QuantizedVector
	Offsets   []uint64
	Vectors   []QuantizedVector
	Labels    []Label
}

// KMeansBlockSize is the SoA block width: how many reference vectors are
// distance-evaluated together per block in the kmeans IVF scan.
const KMeansBlockSize = 8

// KMeansBlockStride is the number of int16 values per SoA block:
// Dimensions (14) × KMeansBlockSize (8). Each block stores all 14
// dimensions interleaved across 8 lanes so the inner loop can compute 8
// squared distances in parallel without strided loads.
const KMeansBlockStride = 14 * KMeansBlockSize

// KMeansIVFIndex holds the in-memory kmeans IVF index used by the scoring
// path. Vectors are laid out in SoA blocks of KMeansBlockSize lanes; each
// list ends on a (possibly partial) block, with the remaining lanes left
// empty. Offsets keeps per-list cumulative vector counts so we can derive
// the valid lane count of the last block.
type KMeansIVFIndex struct {
	Centroids        []Vector
	Offsets          []uint64
	BlockListOffsets []uint32
	Blocks           []int16
	BlockLabels      []Label
}
