package fraudindex

import (
	"math/rand"
	"testing"
)

func TestBlockSquaredDistanceMatchesGoReference(t *testing.T) {
	rng := rand.New(rand.NewSource(0x2026_1503))
	for trial := 0; trial < 200; trial++ {
		var query, block [KMeansBlockStride]int16
		for i := range query {
			query[i] = int16(rng.Intn(65535) - 32767)
			block[i] = int16(rng.Intn(65535) - 32767)
		}

		var got, want [KMeansBlockSize]uint64
		BlockSquaredDistance(&query, &block, &got)
		blockSquaredDistanceGo(&query, &block, &want)

		if got != want {
			t.Fatalf("trial %d: got %v want %v", trial, got, want)
		}
	}
}

func TestBlockSquaredDistanceEdgeRanges(t *testing.T) {
	cases := []struct {
		name        string
		queryFill   int16
		blockFill   int16
		expectedSum uint64
	}{
		{name: "zeros", queryFill: 0, blockFill: 0, expectedSum: 0},
		{name: "max-vs-min-int16", queryFill: 32767, blockFill: -32767, expectedSum: 14 * 65534 * 65534},
		{name: "min-vs-max-int16", queryFill: -32767, blockFill: 32767, expectedSum: 14 * 65534 * 65534},
		{name: "same-large", queryFill: 32767, blockFill: 32767, expectedSum: 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var query, block [KMeansBlockStride]int16
			for i := range query {
				query[i] = tc.queryFill
				block[i] = tc.blockFill
			}
			var got [KMeansBlockSize]uint64
			BlockSquaredDistance(&query, &block, &got)
			for lane, distance := range got {
				if distance != tc.expectedSum {
					t.Fatalf("lane %d: distance = %d, want %d", lane, distance, tc.expectedSum)
				}
			}
		})
	}
}

func BenchmarkBlockSquaredDistance(b *testing.B) {
	rng := rand.New(rand.NewSource(0x2026_dada))
	var query, block [KMeansBlockStride]int16
	for i := range query {
		query[i] = int16(rng.Intn(65535) - 32767)
		block[i] = int16(rng.Intn(65535) - 32767)
	}
	var out [KMeansBlockSize]uint64

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		BlockSquaredDistance(&query, &block, &out)
	}
}

func BenchmarkBlockSquaredDistanceGo(b *testing.B) {
	rng := rand.New(rand.NewSource(0x2026_dada))
	var query, block [KMeansBlockStride]int16
	for i := range query {
		query[i] = int16(rng.Intn(65535) - 32767)
		block[i] = int16(rng.Intn(65535) - 32767)
	}
	var out [KMeansBlockSize]uint64

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		blockSquaredDistanceGo(&query, &block, &out)
	}
}
