//go:build !pprof && instrument

package profiling

import "net/http"

func addPprof(mux *http.ServeMux) {}
