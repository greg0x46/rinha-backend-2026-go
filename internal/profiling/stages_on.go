//go:build instrument

package profiling

import (
	"net/http"

	"github.com/greg/rinha-be-2026/internal/metrics"
)

func addStages(mux *http.ServeMux) {
	mux.HandleFunc("/debug/stages", metrics.StagesHandler)
}
