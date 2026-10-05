// Package sqlitedriver registers the SQLite drivers every database in Memdoor
// opens, under the names callers already use: "sqlite3", and LogsDriverName
// (the same, plus a REGEXP function on every connection).
//
// It is modernc.org/sqlite — SQLite translated to Go — in every build (Greg,
// 2026-09-29: "Pure go is better"). The Linux and Windows downloads are built
// on a Mac with no C cross-compiler, and with mattn/go-sqlite3 they linked its
// cgo stub, whose every Open fails with "Binary was compiled with
// 'CGO_ENABLED=0'": the Linux download could not start a gateway at all
// (checked in the live binary, 2026-09-29). One driver everywhere means what
// runs on a Mac is what runs on Linux.
//
// What it costs: sqlite-vec, the vector-search extension, exists only for the
// C driver, so semantic search over stored embeddings (gateway/rag) is off.
// None of the coder's tools use it.
//
// Import it for its side effect: _ "memdoor/pkg/sqlitedriver".
package sqlitedriver

// LogsDriverName is the driver the log store opens: SQLite with REGEXP
// installed on every connection the pool opens, not just the first.
const LogsDriverName = "sqlite3_logs_regexp"

// Name is the driver every other database opens.
const Name = "sqlite3"
