package billingsvc

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// A BACKUP IS READ BACK, NOT HOPED FOR. The copy holds the rows, the
// sidecars ride along, and the oldest folders go once there are more than
// keep. Mutation checks: skip the VACUUM INTO and the copy is missing; skip
// the prune and three folders stay.
func TestABackupIsACopyThatOpens(t *testing.T) {
	state := t.TempDir()
	dbPath := filepath.Join(state, "billing.db")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("PRAGMA journal_mode=WAL; CREATE TABLE credits(ws TEXT, cents INTEGER); INSERT INTO credits VALUES('sara', 14900)"); err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := os.WriteFile(filepath.Join(state, "billing-auth.json"), []byte(`{"films":{"sara":{"month":"2026-09","n":3}}}`), 0o600); err != nil {
		t.Fatal(err)
	}

	out := filepath.Join(state, "backups")
	t0 := time.Date(2026, 9, 19, 3, 17, 0, 0, time.UTC)
	rep, err := Backup(dbPath, out, 2, t0)
	if err != nil {
		t.Fatal(err)
	}
	copyDB := filepath.Join(rep.Dir, "billing.db")
	c, err := sql.Open("sqlite3", copyDB+"?mode=ro")
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	var cents int
	if err := c.QueryRow("SELECT cents FROM credits WHERE ws='sara'").Scan(&cents); err != nil || cents != 14900 {
		t.Fatalf("the copy holds the ledger: %v %d", err, cents)
	}
	if b, err := os.ReadFile(filepath.Join(rep.Dir, "billing-auth.json")); err != nil || len(b) == 0 {
		t.Fatalf("the sidecar rides along: %v", err)
	}

	// Two more nights: keep=2 leaves the two newest.
	if _, err := Backup(dbPath, out, 2, t0.Add(24*time.Hour)); err != nil {
		t.Fatal(err)
	}
	rep3, err := Backup(dbPath, out, 2, t0.Add(48*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(rep3.Pruned) != 1 || filepath.Base(rep3.Pruned[0]) != t0.Format(backupStamp) {
		t.Fatalf("the oldest goes: %v", rep3.Pruned)
	}
	left, _ := os.ReadDir(out)
	if len(left) != 2 {
		t.Fatalf("two remain, got %d", len(left))
	}
	if _, err := Backup(filepath.Join(state, "missing.db"), out, 2, t0); err == nil {
		t.Fatal("no database, no silent success")
	}
}

// HEALTH SAYS WHEN THE BACKUP IS STALE. The newest folder is named; older
// than a missed night is stale; none at all is stale. Mutation check: a
// threshold of a year and last night's copy is fresh forever.
func TestHealthSaysWhenTheBackupIsStale(t *testing.T) {
	root := t.TempDir()
	t0 := time.Date(2026, 9, 20, 3, 17, 0, 0, time.UTC)
	for _, d := range []time.Duration{-48 * time.Hour, -24 * time.Hour} {
		if err := os.Mkdir(filepath.Join(root, t0.Add(d).Format(backupStamp)), 0o700); err != nil {
			t.Fatal(err)
		}
	}
	if name, stale := lastBackup(root, t0.Add(time.Hour)); name != t0.Add(-24*time.Hour).Format(backupStamp) || stale {
		t.Fatalf("last night's copy is fresh: %q stale=%v", name, stale)
	}
	if _, stale := lastBackup(root, t0.Add(30*time.Hour)); !stale {
		t.Fatal("a missed night is stale")
	}
	if name, stale := lastBackup(filepath.Join(root, "nowhere"), t0); name != "" || !stale {
		t.Fatal("no backup at all is stale")
	}
}
