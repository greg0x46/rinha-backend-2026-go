package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/greg/rinha-be-2026/internal/fraudindex"
	"github.com/greg/rinha-be-2026/internal/httpapi"
)

type testFile struct {
	Stats   map[string]any `json:"stats"`
	Entries []testEntry    `json:"entries"`
}

type testEntry struct {
	Request            httpapi.FraudScoreRequest `json:"request"`
	ExpectedApproved   bool                      `json:"expected_approved"`
	ExpectedFraudScore float64                   `json:"expected_fraud_score"`
}

type stat struct {
	tp, tn, fp, fn int
	scoreEqual     int
	totalScoreTime time.Duration
}

func (s *stat) record(predicted bool, predictedScore float64, expected bool, expectedScore float64) {
	if predicted && expected {
		s.tn++
	} else if !predicted && !expected {
		s.tp++
	} else if !predicted && expected {
		s.fp++
	} else {
		s.fn++
	}
	if predictedScore == expectedScore {
		s.scoreEqual++
	}
}

func (s stat) total() int { return s.tp + s.tn + s.fp + s.fn }

func (s stat) accuracy() float64 {
	t := s.total()
	if t == 0 {
		return 0
	}
	return float64(s.tp+s.tn) / float64(t)
}

func (s stat) failureRate() float64 {
	t := s.total()
	if t == 0 {
		return 0
	}
	return float64(s.fp+s.fn) / float64(t)
}

func main() {
	exactPath := flag.String("exact", "data/references.bin", "quantized exact references path (skipped if file missing)")
	ivfPath := flag.String("ivf", "data/references-ivf.bin", "IVF references path")
	testPath := flag.String("test", "../test/test-data.json", "test-data.json path")
	limit := flag.Int("limit", 0, "limit number of entries (0 = all)")
	exactLimit := flag.Int("exact-limit", 0, "limit number of entries for exact comparison (0 = match -limit)")
	skipExact := flag.Bool("skip-exact", false, "skip exact comparison entirely")
	timeIVF := flag.Bool("time-ivf", true, "measure IVF score time")
	timeExact := flag.Bool("time-exact", false, "measure exact score time (slow)")
	flag.Parse()

	vectorizer, err := httpapi.NewVectorizer()
	if err != nil {
		log.Fatalf("vectorizer: %v", err)
	}

	t0 := time.Now()
	ivfScorer, ivfManifest, ivfKind, err := loadCandidateScorer(*ivfPath)
	if err != nil {
		log.Fatalf("load candidate index: %v", err)
	}
	fmt.Printf("%s index: %d refs, %d lists, loaded in %s\n",
		ivfKind, ivfManifest.References, ivfManifest.NList, time.Since(t0).Round(time.Millisecond))

	var exactScorer httpapi.Scorer
	hasExact := false
	if !*skipExact {
		if _, err := os.Stat(*exactPath); err == nil {
			t0 = time.Now()
			exactIndex, exactManifest, err := fraudindex.LoadQuantizedBinary(*exactPath)
			if err == nil {
				exactScorer = httpapi.NewQuantizedScorer(exactIndex)
				hasExact = true
				fmt.Printf("exact index: %d refs, loaded in %s\n",
					exactManifest.References, time.Since(t0).Round(time.Millisecond))
			} else {
				fmt.Printf("exact index unavailable (%v); skipping exact comparison\n", err)
			}
		} else {
			fmt.Printf("exact index path %q missing; skipping exact comparison\n", *exactPath)
		}
	} else {
		fmt.Println("exact comparison skipped (--skip-exact)")
	}

	t0 = time.Now()
	f, err := os.Open(*testPath)
	if err != nil {
		log.Fatalf("open test data: %v", err)
	}
	var tf testFile
	if err := json.NewDecoder(f).Decode(&tf); err != nil {
		log.Fatalf("decode test data: %v", err)
	}
	_ = f.Close()
	fmt.Printf("test entries: %d (loaded in %s)\n", len(tf.Entries), time.Since(t0).Round(time.Millisecond))

	entries := tf.Entries
	if *limit > 0 && *limit < len(entries) {
		entries = entries[:*limit]
	}
	exactBudget := len(entries)
	if *exactLimit > 0 && *exactLimit < exactBudget {
		exactBudget = *exactLimit
	}

	var ivfStats stat
	var exactStats stat
	disagree := 0
	disagreeBothWrong := 0
	disagreeIVFOnly := 0
	disagreeExactOnly := 0

	for idx, entry := range entries {
		vec := vectorizer.Vectorize(entry.Request)

		var ivfStart time.Time
		if *timeIVF {
			ivfStart = time.Now()
		}
		ivfResp := ivfScorer.Score(vec)
		if *timeIVF {
			ivfStats.totalScoreTime += time.Since(ivfStart)
		}
		ivfStats.record(ivfResp.Approved, ivfResp.FraudScore, entry.ExpectedApproved, entry.ExpectedFraudScore)

		if hasExact && idx < exactBudget {
			var exactStart time.Time
			if *timeExact {
				exactStart = time.Now()
			}
			exactResp := exactScorer.Score(vec)
			if *timeExact {
				exactStats.totalScoreTime += time.Since(exactStart)
			}
			exactStats.record(exactResp.Approved, exactResp.FraudScore, entry.ExpectedApproved, entry.ExpectedFraudScore)

			if ivfResp.Approved != exactResp.Approved {
				disagree++
				ivfRight := ivfResp.Approved == entry.ExpectedApproved
				exactRight := exactResp.Approved == entry.ExpectedApproved
				switch {
				case ivfRight && !exactRight:
					disagreeIVFOnly++
				case !ivfRight && exactRight:
					disagreeExactOnly++
				case !ivfRight && !exactRight:
					disagreeBothWrong++
				}
			}
		}
	}

	fmt.Println()
	fmt.Println("=== IVF vs ground truth ===")
	printStats(ivfStats, *timeIVF)
	if hasExact {
		fmt.Println()
		fmt.Println("=== Exact vs ground truth ===")
		printStats(exactStats, *timeExact)
		fmt.Println()
		fmt.Println("=== IVF vs Exact (decision) ===")
		fmt.Printf("disagreements: %d / %d (%.4f%%)\n", disagree, exactBudget, 100*float64(disagree)/float64(exactBudget))
		fmt.Printf("  IVF correct, exact wrong: %d\n", disagreeIVFOnly)
		fmt.Printf("  IVF wrong, exact correct: %d\n", disagreeExactOnly)
		fmt.Printf("  both wrong (different way): %d\n", disagreeBothWrong)
	}
}

func loadCandidateScorer(path string) (httpapi.Scorer, fraudindex.Manifest, string, error) {
	kmeansIndex, kmeansManifest, err := fraudindex.LoadKMeansIVFBinary(path)
	if err == nil {
		return httpapi.NewKMeansIVFScorer(kmeansIndex), kmeansManifest, "kmeans-ivf", nil
	}
	ivfIndex, ivfManifest, ivfErr := fraudindex.LoadIVFBinary(path)
	if ivfErr == nil {
		return httpapi.NewIVFScorer(ivfIndex), ivfManifest, "ivf", nil
	}
	return httpapi.Scorer{}, fraudindex.Manifest{}, "", fmt.Errorf("kmeans ivf: %v; ivf: %w", err, ivfErr)
}

func printStats(s stat, withTime bool) {
	t := s.total()
	fmt.Printf("samples: %d\n", t)
	fmt.Printf("confusion (predicted vs expected): TP=%d TN=%d FP=%d FN=%d\n", s.tp, s.tn, s.fp, s.fn)
	fmt.Printf("accuracy: %.4f%% | failure rate: %.4f%% (cutoff is 15%%)\n",
		100*s.accuracy(), 100*s.failureRate())
	fmt.Printf("score-equal-to-expected: %d / %d (%.4f%%)\n",
		s.scoreEqual, t, 100*float64(s.scoreEqual)/float64(t))
	if withTime && t > 0 {
		fmt.Printf("score time total: %s | avg: %s\n",
			s.totalScoreTime.Round(time.Millisecond),
			(s.totalScoreTime / time.Duration(t)).Round(time.Microsecond))
	}
}
