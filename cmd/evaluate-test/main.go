package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math"
	"os"
	"strconv"
	"strings"
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
	sweepQuick := flag.String("sweep-quick", "", "comma-separated kmeans-quick-probe values (enables sweep mode; implies --skip-exact)")
	sweepExpanded := flag.String("sweep-expanded", "", "comma-separated kmeans-expanded-probe values (required with --sweep-quick)")
	flag.Parse()

	if *sweepQuick != "" || *sweepExpanded != "" {
		quicks, err := parseIntList(*sweepQuick)
		if err != nil || len(quicks) == 0 {
			log.Fatalf("invalid --sweep-quick %q: %v", *sweepQuick, err)
		}
		expandeds, err := parseIntList(*sweepExpanded)
		if err != nil || len(expandeds) == 0 {
			log.Fatalf("invalid --sweep-expanded %q: %v", *sweepExpanded, err)
		}
		runSweep(*ivfPath, *testPath, *limit, quicks, expandeds)
		return
	}

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

// runSweep evaluates a matrix of (quick, expanded) probe combinations on
// the same loaded index. Output is one row per combination with FP/FN/E,
// failure rate, weighted-error projection, and avg score time.
func runSweep(ivfPath, testPath string, limit int, quicks, expandeds []int) {
	vectorizer, err := httpapi.NewVectorizer()
	if err != nil {
		log.Fatalf("vectorizer: %v", err)
	}
	t0 := time.Now()
	scorer, manifest, kind, err := loadCandidateScorer(ivfPath)
	if err != nil {
		log.Fatalf("load index: %v", err)
	}
	fmt.Printf("%s index: %d refs, %d lists, loaded in %s\n",
		kind, manifest.References, manifest.NList, time.Since(t0).Round(time.Millisecond))

	t0 = time.Now()
	f, err := os.Open(testPath)
	if err != nil {
		log.Fatalf("open test data: %v", err)
	}
	var tf testFile
	if err := json.NewDecoder(f).Decode(&tf); err != nil {
		log.Fatalf("decode test data: %v", err)
	}
	_ = f.Close()
	entries := tf.Entries
	if limit > 0 && limit < len(entries) {
		entries = entries[:limit]
	}
	fmt.Printf("test entries: %d (loaded in %s)\n", len(entries), time.Since(t0).Round(time.Millisecond))

	// Pre-vectorize once so each cell only times the score path.
	vectors := make([]httpapi.Vector, len(entries))
	for i, e := range entries {
		vectors[i] = vectorizer.Vectorize(e.Request)
	}

	fmt.Println()
	fmt.Printf("%-6s %-9s %-7s %-7s %-5s %-9s %-12s %-9s %-12s\n",
		"quick", "expanded", "FP", "FN", "E", "fail%", "score_avg", "rate_E", "final_proj")
	for _, q := range quicks {
		for _, e := range expandeds {
			httpapi.SetKMeansProbes(q, e)
			var s stat
			start := time.Now()
			for i, entry := range entries {
				resp := scorer.Score(vectors[i])
				s.record(resp.Approved, resp.FraudScore, entry.ExpectedApproved, entry.ExpectedFraudScore)
			}
			s.totalScoreTime = time.Since(start)
			eWeighted := s.fp*1 + s.fn*3
			fmt.Printf("%-6d %-9d %-7d %-7d %-5d %-9.4f %-12s %-9d %-12s\n",
				q, e, s.fp, s.fn, eWeighted,
				100*s.failureRate(),
				(s.totalScoreTime / time.Duration(s.total())).Round(time.Microsecond),
				eWeighted,
				projectedScore(s.fp, s.fn, s.total()))
		}
	}
}

func parseIntList(s string) ([]int, error) {
	if s == "" {
		return nil, nil
	}
	parts := strings.Split(s, ",")
	out := make([]int, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		n, err := strconv.Atoi(p)
		if err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, nil
}

// projectedScore returns a string with the detection-component projection
// for the sweep table. Latency is not measured here, so we only report the
// detection slice (rate component minus E penalty), which is the part the
// sweep can actually move.
func projectedScore(fp, fn, n int) string {
	const K = 1000.0
	const beta = 300.0
	const epsMin = 0.001
	const cap = 3000.0
	if n == 0 {
		return "n/a"
	}
	E := float64(fp + 3*fn)
	eps := E / float64(n)
	if eps < epsMin {
		eps = epsMin
	}
	rate := K * log10(1/eps)
	if rate > cap {
		rate = cap
	}
	penalty := -beta * log10(1+E)
	return fmt.Sprintf("%.1f", rate+penalty)
}

func log10(x float64) float64 { return math.Log10(x) }

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
