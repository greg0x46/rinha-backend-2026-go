//go:build amd64

package fraudindex

// BlockSquaredDistance fills out with the per-lane squared L2 distance
// between query and block (both in the SoA layout). On amd64 it is
// implemented in assembly using SSE4.1 (PMOVSXWD / PMULLD / PMOVZXDQ /
// PADDQ); see kernel_amd64.s.
//
//go:noescape
func BlockSquaredDistance(query, block *[KMeansBlockStride]int16, out *[KMeansBlockSize]uint64)
