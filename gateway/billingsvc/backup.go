package billingsvc

import (
	"database/sql"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Backups — the state the broker cannot recreate, copied every night.
//
// Measured on memdoor.ai (2026-09-19): nothing under /var/lib/memdoor was
// ever copied anywhere. A disk failure loses every account, seat and plan.
// What is worth copying is small: the ledger db, and the auth file beside
// it.
//
// The database is copied with VACUUM INTO, which is a consistent snapshot
// through the WAL, then OPENED and integrity-checked: a backup that was
// never read back is a hope, not a backup.
const (
	// backupKeepDefault is how many dated folders are kept.
	backupKeepDefault = 30
	backupStamp       = "2006-01-02T150405Z"
	// backupRootDefault is where the timer writes (scripts/deploy.sh).
	backupRootDefault = "/var/backups/memdoor"
	// backupStaleAfter: a nightly copy older than this missed a night.
	backupStaleAfter = 26 * time.Hour
)

// backupSidecars are the files beside the ledger that hold broker state.
var backupSidecars = []string{"billing-auth.json"}

// BackupReport is what one run produced.
type BackupReport struct {
	Dir    string
	Files  []string
	Pruned []string
}

// Backup copies the ledger and its sidecars into outRoot/<stamp>/, verifies
// the copy, and prunes the oldest folders past keep.
func Backup(dbPath, outRoot string, keep int, now time.Time) (BackupReport, error) {
	var rep BackupReport
	if keep <= 0 {
		keep = backupKeepDefault
	}
	if _, err := os.Stat(dbPath); err != nil {
		return rep, fmt.Errorf("backup: %w", err)
	}
	dir := filepath.Join(outRoot, now.UTC().Format(backupStamp))
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return rep, fmt.Errorf("backup: %w", err)
	}
	rep.Dir = dir
	copyDB := filepath.Join(dir, filepath.Base(dbPath))
	if err := snapshotSQLite(dbPath, copyDB); err != nil {
		return rep, err
	}
	if err := verifySQLite(copyDB); err != nil {
		return rep, err
	}
	rep.Files = append(rep.Files, copyDB)
	for _, name := range backupSidecars {
		src := filepath.Join(filepath.Dir(dbPath), name)
		if _, err := os.Stat(src); err != nil {
			continue
		}
		dst := filepath.Join(dir, name)
		if err := copyFile(src, dst); err != nil {
			return rep, fmt.Errorf("backup %s: %w", name, err)
		}
		rep.Files = append(rep.Files, dst)
	}
	pruned, err := pruneBackups(outRoot, keep, dir)
	if err != nil {
		return rep, err
	}
	rep.Pruned = pruned
	return rep, nil
}

// snapshotSQLite writes a consistent copy of a live database.
func snapshotSQLite(src, dst string) error {
	db, err := sql.Open("sqlite3", src+"?mode=ro")
	if err != nil {
		return fmt.Errorf("backup open: %w", err)
	}
	defer db.Close()
	if _, err := db.Exec("VACUUM INTO ?", dst); err != nil {
		return fmt.Errorf("backup vacuum into: %w", err)
	}
	return nil
}

// verifySQLite opens the copy and asks it whether it is whole.
func verifySQLite(path string) error {
	db, err := sql.Open("sqlite3", path+"?mode=ro")
	if err != nil {
		return fmt.Errorf("backup verify: %w", err)
	}
	defer db.Close()
	var verdict string
	if err := db.QueryRow("PRAGMA integrity_check").Scan(&verdict); err != nil {
		return fmt.Errorf("backup verify: %w", err)
	}
	if verdict != "ok" {
		return fmt.Errorf("backup verify: %s", verdict)
	}
	var tables int
	if err := db.QueryRow("SELECT count(*) FROM sqlite_master WHERE type='table'").Scan(&tables); err != nil {
		return fmt.Errorf("backup verify: %w", err)
	}
	if tables == 0 {
		return fmt.Errorf("backup verify: the copy has no tables")
	}
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// pruneBackups removes the oldest dated folders so that keep remain. Only
// folders named like a stamp are touched; the one just written never is.
func pruneBackups(outRoot string, keep int, justWritten string) ([]string, error) {
	entries, err := os.ReadDir(outRoot)
	if err != nil {
		return nil, fmt.Errorf("backup prune: %w", err)
	}
	var stamps []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if _, err := time.Parse(backupStamp, e.Name()); err != nil {
			continue
		}
		stamps = append(stamps, e.Name())
	}
	sort.Strings(stamps)
	var pruned []string
	for len(stamps) > keep {
		victim := filepath.Join(outRoot, stamps[0])
		stamps = stamps[1:]
		if victim == justWritten {
			continue
		}
		if err := os.RemoveAll(victim); err != nil {
			return pruned, fmt.Errorf("backup prune: %w", err)
		}
		pruned = append(pruned, victim)
	}
	return pruned, nil
}

// String is the one-line receipt a run prints.
func (r BackupReport) String() string {
	names := make([]string, 0, len(r.Files))
	for _, f := range r.Files {
		names = append(names, filepath.Base(f))
	}
	s := fmt.Sprintf("backed up %s (%s)", r.Dir, strings.Join(names, ", "))
	if len(r.Pruned) > 0 {
		s += fmt.Sprintf("; pruned %d", len(r.Pruned))
	}
	return s
}

// lastBackup is the newest dated folder under root and whether it is
// older than a missed night. "" and true when there is none.
func lastBackup(root string, now time.Time) (string, bool) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return "", true
	}
	newest := ""
	for _, e := range entries {
		if e.IsDir() && len(e.Name()) == len(backupStamp) {
			if _, err := time.Parse(backupStamp, e.Name()); err == nil && e.Name() > newest {
				newest = e.Name()
			}
		}
	}
	if newest == "" {
		return "", true
	}
	at, _ := time.Parse(backupStamp, newest)
	return newest, now.Sub(at) > backupStaleAfter
}
