//go:build !instrument && pprof

package profiling

import "net/http"

func addStages(mux *http.ServeMux) {}
