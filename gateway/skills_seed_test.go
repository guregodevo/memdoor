package gateway

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"memdoor/skills"
)

// seedInto redirects HOME so SeedEmbeddedSkills writes into a temp dir.
func seedInto(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	SeedEmbeddedSkills()
	return filepath.Join(home, ".memdoor", "skills")
}

func TestSeedWritesEmbeddedSkills(t *testing.T) {
	dir := seedInto(t)
	entries, err := skills.FS.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		disk, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatalf("seed did not write %s: %v", e.Name(), err)
		}
		embedded, _ := skills.FS.ReadFile(e.Name())
		if string(disk) != string(embedded) {
			t.Fatalf("%s: seeded content differs from embedded", e.Name())
		}
	}
	if _, err := os.Stat(filepath.Join(dir, ".seed-manifest.json")); err != nil {
		t.Fatal("manifest not written")
	}
}

func TestSeedNeverClobbersUserEdits(t *testing.T) {
	dir := seedInto(t)
	// User edits a seeded skill…
	entries, _ := skills.FS.ReadDir(".")
	var name string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
			name = e.Name()
			break
		}
	}
	edited := "# my custom version\n"
	if err := os.WriteFile(filepath.Join(dir, name), []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}
	// …and a re-seed must leave it alone.
	SeedEmbeddedSkills()
	got, _ := os.ReadFile(filepath.Join(dir, name))
	if string(got) != edited {
		t.Fatalf("re-seed clobbered user edit of %s", name)
	}
}

// A file synced in by hand that IS the shipped one is adopted, not
// mistaken for a local edit: the manifest catches up, so a later
// shipped change can still refresh it. The mutation check: drop the
// adoption case and the stale hash survives.
func TestSeedAdoptsAFileThatMatchesTheShippedOne(t *testing.T) {
	dir := seedInto(t)
	entries, _ := skills.FS.ReadDir(".")
	var name string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
			name = e.Name()
			break
		}
	}
	// A stale manifest beside a file identical to the shipped copy: what a
	// hand sync of a newer skill leaves behind.
	if err := os.WriteFile(filepath.Join(dir, ".seed-manifest.json"), []byte(`{"`+name+`":"deadbeef"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	SeedEmbeddedSkills()
	manifest, err := os.ReadFile(filepath.Join(dir, ".seed-manifest.json"))
	if err != nil || strings.Contains(string(manifest), "deadbeef") {
		t.Fatalf("the manifest must adopt the file it matches: %s %v", manifest, err)
	}
	// A real local edit is still kept.
	if err := os.WriteFile(filepath.Join(dir, name), []byte("# mine\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	SeedEmbeddedSkills()
	if b, _ := os.ReadFile(filepath.Join(dir, name)); string(b) != "# mine\n" {
		t.Fatal("a real local edit must still be kept")
	}
}

// A skill the binary stopped shipping is removed from disk when nobody edited
// it, and kept when someone did (2026-09-27: the deleted feature's skills and older skills stayed
// loadable because disk wins over the embedded set).
func TestSeedRetiresSkillsNoLongerShipped(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir := filepath.Join(home, ".memdoor", "skills")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	gone, edited := []byte("# old shipped skill\n"), []byte("# mine now\n")
	_ = os.WriteFile(filepath.Join(dir, "gone.md"), gone, 0o644)
	_ = os.WriteFile(filepath.Join(dir, "edited.md"), edited, 0o644)
	manifest, _ := json.Marshal(map[string]string{"gone.md": hashBytes(gone), "edited.md": hashBytes([]byte("# as shipped\n"))})
	_ = os.WriteFile(filepath.Join(dir, ".seed-manifest.json"), manifest, 0o644)

	SeedEmbeddedSkills()

	if _, err := os.Stat(filepath.Join(dir, "gone.md")); !os.IsNotExist(err) {
		t.Error("an unedited skill no longer shipped must be removed")
	}
	if _, err := os.Stat(filepath.Join(dir, "edited.md")); err != nil {
		t.Error("an edited copy is the user's and must stay")
	}
	if _, err := os.Stat(filepath.Join(dir, "review.md")); err != nil {
		t.Error("shipped skills are still seeded")
	}
}
