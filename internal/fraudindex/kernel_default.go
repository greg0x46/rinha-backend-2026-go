//go:build !amd64

package fraudindex

// BlockSquaredDistance fills out with the per-lane squared L2 distance
// between query and block (both in the SoA layout). On non-amd64 builds it
// delegates to the generic Go implementation.
func BlockSquaredDistance(query, block *[KMeansBlockStride]int16, out *[KMeansBlockSize]uint64) {
	blockSquaredDistanceGo(query, block, out)
}
