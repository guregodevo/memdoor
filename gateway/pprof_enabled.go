//go:build pprof

package gateway

// Importing net/http/pprof registers the /debug/pprof/* handlers on
// http.DefaultServeMux via its init(). This file is compiled in ONLY with
// `-tags pprof`, so the routes exist exclusively in profiling builds — used
// locally or for load-testing, never the public production binary. Build:
//
//	go build -tags pprof ./...
import _ "net/http/pprof"

func registerPprof() {}
