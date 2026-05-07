package fraudindex

// BroadcastQuery copies each dimension of a quantized query into the SoA
// block layout: 8 consecutive int16 lanes per dimension. The result can be
// passed to BlockSquaredDistance alongside any reference block.
func BroadcastQuery(query QuantizedVector, out *[KMeansBlockStride]int16) {
	for d := 0; d < 14; d++ {
		v := query[d]
		base := d * KMeansBlockSize
		out[base+0] = v
		out[base+1] = v
		out[base+2] = v
		out[base+3] = v
		out[base+4] = v
		out[base+5] = v
		out[base+6] = v
		out[base+7] = v
	}
}

// blockSquaredDistanceGo is the architecture-independent reference kernel
// used as the fallback on non-amd64 builds and as the oracle in tests. It
// expects query and block in the SoA layout: dim d, lane l at index
// d*KMeansBlockSize+l.
func blockSquaredDistanceGo(query, block *[KMeansBlockStride]int16, out *[KMeansBlockSize]uint64) {
	var dist [KMeansBlockSize]uint64
	for d := 0; d < 14; d++ {
		base := d * KMeansBlockSize
		for l := 0; l < KMeansBlockSize; l++ {
			delta := int64(query[base+l]) - int64(block[base+l])
			dist[l] += uint64(delta * delta)
		}
	}
	*out = dist
}
