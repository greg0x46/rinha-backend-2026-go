// Package profiling exposes optional debug endpoints (pprof, runtime/trace,
// stage histogram) on an internal listener. With both build tags off, all
// entry points compile to no-ops and no listener is started.
package profiling
