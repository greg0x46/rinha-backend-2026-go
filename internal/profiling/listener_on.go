//go:build pprof || instrument

package profiling

import (
	"log/slog"
	"net/http"
	"time"
)

// Bind on all interfaces inside the container so docker's host port mapping
// (127.0.0.1:606N -> container:6060) can reach it. Privacy is enforced by the
// host-side publish (loopback only) plus nginx not having a route for /debug.
const debugAddr = ":6060"

func StartDebugListener() {
	mux := http.NewServeMux()
	addPprof(mux)
	addStages(mux)

	server := &http.Server{
		Addr:              debugAddr,
		Handler:           mux,
		ReadHeaderTimeout: 2 * time.Second,
	}
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Warn("debug listener stopped", "err", err)
		}
	}()
	slog.Info("debug listener started", "addr", debugAddr)
}
