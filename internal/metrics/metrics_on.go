//go:build instrument

package metrics

import (
	"fmt"
	"net/http"
	"sync/atomic"
	"time"
)

var bucketThresholds = [...]time.Duration{
	100 * time.Microsecond,
	500 * time.Microsecond,
	1 * time.Millisecond,
	5 * time.Millisecond,
	20 * time.Millisecond,
	100 * time.Millisecond,
	1 * time.Second,
}

var bucketLabels = [...]string{
	"<=100us",
	"<=500us",
	"<=1ms",
	"<=5ms",
	"<=20ms",
	"<=100ms",
	"<=1s",
	">1s",
}

const numBuckets = len(bucketThresholds) + 1

var (
	counters [NumStages][numBuckets]atomic.Uint64
	totalNs  [NumStages]atomic.Uint64
)

func Now() time.Time { return time.Now() }

func Since(t time.Time, stage Stage) time.Time {
	now := time.Now()
	Record(stage, now.Sub(t))
	return now
}

func Record(stage Stage, dur time.Duration) {
	bucket := len(bucketThresholds)
	for i, threshold := range bucketThresholds {
		if dur <= threshold {
			bucket = i
			break
		}
	}
	counters[stage][bucket].Add(1)
	if dur > 0 {
		totalNs[stage].Add(uint64(dur.Nanoseconds()))
	}
}

func StagesHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	fmt.Fprintln(w, "stage\tbucket\tcount")
	for s := Stage(0); s < NumStages; s++ {
		var total uint64
		for b := 0; b < numBuckets; b++ {
			c := counters[s][b].Load()
			total += c
			fmt.Fprintf(w, "%s\t%s\t%d\n", stageNames[s], bucketLabels[b], c)
		}
		fmt.Fprintf(w, "%s\ttotal\t%d\n", stageNames[s], total)
		fmt.Fprintf(w, "%s\ttotal_ns\t%d\n", stageNames[s], totalNs[s].Load())
	}
}
