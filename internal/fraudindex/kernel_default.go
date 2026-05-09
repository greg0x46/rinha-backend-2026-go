//go:build !amd64

package fraudindex

// BlockSquaredDistance fills out with the per-lane squared L2 distance
// between query and block (both in the SoA layout). On non-amd64 builds it
// delegates to the generic Go implementation.
func BlockSquaredDistance(query, block *[KMeansBlockStride]int16, out *[KMeansBlockSize]uint64) {
	blockSquaredDistanceGo(query, block, out)
}

// BlockSquaredDistancePartial mirrors the amd64 partial kernel for non-SIMD
// builds; accumulates dims [dimStart, dimStart+dimCount) into accum.
func BlockSquaredDistancePartial(query, block *[KMeansBlockStride]int16, dimStart, dimCount int, accum *[KMeansBlockSize]uint64) {
	for d := dimStart; d < dimStart+dimCount; d++ {
		base := d * KMeansBlockSize
		for l := 0; l < KMeansBlockSize; l++ {
			delta := int64(query[base+l]) - int64(block[base+l])
			accum[l] += uint64(delta * delta)
		}
	}
}
