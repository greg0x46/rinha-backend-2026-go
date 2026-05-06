//go:build !instrument

package metrics

import "time"

func Now() time.Time { return time.Time{} }

func Since(t time.Time, stage Stage) time.Time { return t }

func Record(stage Stage, dur time.Duration) {}
