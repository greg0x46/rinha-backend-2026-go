//go:build pprof

package profiling

import (
	"net/http"
	"net/http/pprof"
	"runtime/trace"
	"strconv"
	"time"
)

func addPprof(mux *http.ServeMux) {
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	mux.HandleFunc("/debug/trace", traceHandler)
}

func traceHandler(w http.ResponseWriter, r *http.Request) {
	seconds := 5
	if raw := r.URL.Query().Get("seconds"); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 && v <= 60 {
			seconds = v
		}
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", `attachment; filename="trace.out"`)
	if err := trace.Start(w); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	select {
	case <-time.After(time.Duration(seconds) * time.Second):
	case <-r.Context().Done():
	}
	trace.Stop()
}
