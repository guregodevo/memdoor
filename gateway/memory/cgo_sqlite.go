package memory

/*
#cgo CFLAGS: -DSQLITE_ENABLE_FTS5
*/
import "C"

// This file exists solely to set CGO compilation flags for SQLite FTS5 support
// The -DSQLITE_ENABLE_FTS5 flag ensures that SQLite is compiled with Full-Text Search version 5,
// which is required for BM25 ranking in the memory system.
