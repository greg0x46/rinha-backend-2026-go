//go:build amd64

package fraudindex

// BlockSquaredDistance fills out with the per-lane squared L2 distance
// between query and block (both in the SoA layout). On amd64 it is
// implemented in assembly using SSE4.1 (PMOVSXWD / PMULLD / PMOVZXDQ /
// PADDQ); see kernel_amd64.s.
//
//go:noescape
func BlockSquaredDistance(query, block *[KMeansBlockStride]int16, out *[KMeansBlockSize]uint64)

// BlockSquaredDistancePartial accumulates the per-lane squared L2 distance
// over dims [dimStart, dimStart+dimCount) into accum, which is both
// read and written. Lets the caller chunk the 14-dim distance and bail
// out between chunks when the partial already exceeds the worst of the
// running top-5 (early termination). See kernel_partial_amd64.s.
//
//go:noescape
func BlockSquaredDistancePartial(query, block *[KMeansBlockStride]int16, dimStart, dimCount int, accum *[KMeansBlockSize]uint64)
