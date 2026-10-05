//go:build !pprof

package gateway

// registerPprof is a no-op in the default build. Crucially, this file does
// NOT import net/http/pprof — that package's init() registers /debug/pprof/
// on http.DefaultServeMux (which nginx fronts), so merely importing it would
// expose heap dumps (in-memory session tokens) and a CPU-profile DoS to
// anonymous callers. Profiling is opt-in at BUILD time via `-tags pprof`.
func registerPprof() {}
