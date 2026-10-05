package sqlitedriver

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func open(t *testing.T, driver, dsn string) *sql.DB {
	t.Helper()
	db, err := sql.Open(driver, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func pragma(t *testing.T, db *sql.DB, name string) string {
	t.Helper()
	var v string
	if err := db.QueryRow("PRAGMA " + name).Scan(&v); err != nil {
		t.Fatalf("PRAGMA %s: %v", name, err)
	}
	return v
}

// The go-sqlite3 options the callers write still set what they say: modernc
// ignores options it does not know, so an untranslated _foreign_keys=on would
// open a database without foreign keys and nothing would complain.
func TestMattnOptionsStillApply(t *testing.T) {
	path := filepath.Join(t.TempDir(), "a.db")
	db := open(t, Name, "file:"+path+"?_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=on")
	if got := pragma(t, db, "foreign_keys"); got != "1" {
		t.Errorf("foreign_keys = %s, want 1", got)
	}
	if got := pragma(t, db, "busy_timeout"); got != "5000" {
		t.Errorf("busy_timeout = %s, want 5000", got)
	}
	if got := strings.ToLower(pragma(t, db, "journal_mode")); got != "wal" {
		t.Errorf("journal_mode = %s, want wal", got)
	}
}

// REGEXP works on every connection of the log store's pool, not the first.
func TestRegexpOnEveryConnection(t *testing.T) {
	db := open(t, LogsDriverName, "file:"+filepath.Join(t.TempDir(), "l.db")+"?_regexp=1")
	db.SetMaxOpenConns(4)
	if _, err := db.Exec(`CREATE TABLE l(m TEXT); INSERT INTO l VALUES ('brain answered'), ('tool failed')`); err != nil {
		t.Fatal(err)
	}
	conns := make([]*sql.Conn, 4)
	for i := range conns {
		c, err := db.Conn(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		conns[i] = c
	}
	for i, c := range conns {
		var n int
		if err := c.QueryRowContext(t.Context(), `SELECT count(*) FROM l WHERE m REGEXP 'fail'`).Scan(&n); err != nil || n != 1 {
			t.Errorf("connection %d: n=%d err=%v", i, n, err)
		}
		c.Close()
	}
}

// A time is written the way go-sqlite3 wrote it, so rows from before the
// switch and after it compare correctly as text.
func TestTimeWrittenInTheOldLayout(t *testing.T) {
	db := open(t, Name, filepath.Join(t.TempDir(), "t.db"))
	if _, err := db.Exec(`CREATE TABLE e(at DATETIME)`); err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 9, 29, 9, 30, 0, 0, time.FixedZone("CEST", 2*3600))
	if _, err := db.Exec(`INSERT INTO e VALUES (?)`, at); err != nil {
		t.Fatal(err)
	}
	var raw string
	if err := db.QueryRow(`SELECT CAST(at AS TEXT) FROM e`).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	if raw != "2026-09-29 09:30:00+02:00" {
		t.Fatalf("stored %q, want go-sqlite3's layout 2026-09-29 09:30:00+02:00", raw)
	}
	var back time.Time
	if err := db.QueryRow(`SELECT at FROM e`).Scan(&back); err != nil || !back.Equal(at) {
		t.Fatalf("read back %v (%v), want %v", back, err, at)
	}
}

// FTS5 is compiled in: the memory store's ranked search no longer depends on
// a build flag.
func TestFTS5IsAvailable(t *testing.T) {
	db := open(t, Name, filepath.Join(t.TempDir(), "f.db"))
	if _, err := db.Exec(`CREATE VIRTUAL TABLE f USING fts5(body)`); err != nil {
		t.Fatalf("fts5: %v", err)
	}
}

// A connection waits for a lock rather than failing at once: go-sqlite3
// waited 5 s by default, and the gateway writes from several goroutines.
func TestConcurrentWritersWait(t *testing.T) {
	path := filepath.Join(t.TempDir(), "w.db")
	if got := pragma(t, open(t, Name, path), "busy_timeout"); got != "5000" {
		t.Fatalf("default busy_timeout = %s, want 5000", got)
	}
	db := open(t, Name, path)
	if _, err := db.Exec(`CREATE TABLE w(n INTEGER)`); err != nil {
		t.Fatal(err)
	}
	errs := make(chan error, 16)
	for i := 0; i < 16; i++ {
		go func(i int) {
			other, err := sql.Open(Name, path)
			if err != nil {
				errs <- err
				return
			}
			defer other.Close()
			_, err = other.Exec(`INSERT INTO w VALUES (?)`, i)
			errs <- err
		}(i)
	}
	for i := 0; i < 16; i++ {
		if err := <-errs; err != nil {
			t.Fatalf("a concurrent writer failed instead of waiting: %v", err)
		}
	}
}
