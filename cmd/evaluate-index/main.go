package main

import (
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/greg/rinha-be-2026/internal/fraudindex"
	"github.com/greg/rinha-be-2026/internal/httpapi"
)

func main() {
	exactPath := flag.String("exact", "data/references.bin", "quantized exact references path")
	ivfPath := flag.String("ivf", "data/references-ivf.bin", "IVF references path")
	samples := flag.Int("samples", 20, "number of deterministic sample queries")
	flag.Parse()

	if *samples <= 0 {
		log.Fatal("samples must be greater than zero")
	}

	started := time.Now()
	exactIndex, exactManifest, err := fraudindex.LoadQuantizedBinary(*exactPath)
	if err != nil {
		log.Fatalf("load exact index: %v", err)
	}
	exactLoad := time.Since(started)

	started = time.Now()
	ivfIndex, ivfManifest, err := fraudindex.LoadIVFBinary(*ivfPath)
	if err != nil {
		log.Fatalf("load ivf index: %v", err)
	}
	ivfLoad := time.Since(started)

	exactScorer := httpapi.NewQuantizedScorer(exactIndex)
	ivfScorer := httpapi.NewIVFScorer(ivfIndex)

	stride := len(exactIndex.Vectors) / *samples
	if stride == 0 {
		stride = 1
	}

	var exactScoreTime time.Duration
	var ivfScoreTime time.Duration
	decisionMismatches := 0
	scoreMismatches := 0
	compared := 0

	for i := 0; i < *samples && i*stride < len(exactIndex.Vectors); i++ {
		query := dequantize(exactIndex.Vectors[i*stride])

		started = time.Now()
		exactResponse := exactScorer.Score(query)
		exactScoreTime += time.Since(started)

		started = time.Now()
		ivfResponse := ivfScorer.Score(query)
		ivfScoreTime += time.Since(started)

		if exactResponse.Approved != ivfResponse.Approved {
			decisionMismatches++
		}
		if exactResponse.FraudScore != ivfResponse.FraudScore {
			scoreMismatches++
		}
		compared++
	}

	fmt.Printf("exact references: %d loaded in %s\n", exactManifest.References, exactLoad.Round(time.Millisecond))
	fmt.Printf("ivf references: %d lists: %d loaded in %s\n", ivfManifest.References, ivfManifest.NList, ivfLoad.Round(time.Millisecond))
	fmt.Printf("samples: %d\n", compared)
	fmt.Printf("decision mismatches: %d\n", decisionMismatches)
	fmt.Printf("score mismatches: %d\n", scoreMismatches)
	fmt.Printf("exact total score time: %s\n", exactScoreTime.Round(time.Millisecond))
	fmt.Printf("ivf total score time: %s\n", ivfScoreTime.Round(time.Millisecond))
	if compared > 0 {
		fmt.Printf("exact avg score time: %s\n", (exactScoreTime / time.Duration(compared)).Round(time.Millisecond))
		fmt.Printf("ivf avg score time: %s\n", (ivfScoreTime / time.Duration(compared)).Round(time.Millisecond))
	}
}

func dequantize(vector fraudindex.QuantizedVector) fraudindex.Vector {
	var result fraudindex.Vector
	for i, value := range vector {
		result[i] = float32(value) / float32(fraudindex.QuantizationScale)
	}
	return result
}
