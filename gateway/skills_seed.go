package gateway

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"memdoor/gateway/logs"
	"memdoor/pkg/shared"
	"memdoor/skills"
	"memdoor/workflows"
)

// SeedEmbeddedSkills copies the embedded skill library to the managed skills
// dir (~/.memdoor/skills) at boot, making DISK the source of truth: edits
// there take effect on the next skill call with no rebuild, and the embedded
// copy is only a fallback for a machine that has never booted.
func SeedEmbeddedSkills() {
	seedEmbedded(skills.FS, shared.MemdoorHome("skills"), "Skill", func(rel string) string { return rel })
}

// SeedEmbeddedWorkflows copies the built-in workflows (pr-review, write-tests,
// explain-failure …) to the person's library, ~/.memdoor/workflows, where any
// project finds them after its own (workflow.Find): `memdoor workflow run
// pr-review` works in a repo with no .memdoor at all, the GitHub action's
// first job included (2026-10-27). A workflow is one unit: a directory of
// that name the seed did not write is the person's, and nothing is added
// to it.
func SeedEmbeddedWorkflows() {
	seedEmbedded(workflows.FS, shared.MemdoorHome("workflows"), "Workflow", func(rel string) string {
		return strings.SplitN(rel, "/", 2)[0]
	})
}

// seedEmbedded copies every file of fsys under dir. The seed never destroys
// user work. A manifest of shipped-content hashes (.seed-manifest.json)
// distinguishes "still as we shipped it" from "user edited": an absent file
// is written (unless its unit, as unit names it, is on disk and the seed
// never wrote any of it); an unedited file is refreshed when the shipped
// version changes; an edited file is left alone forever (a WARN notes when
// the shipped version has moved on so the divergence is visible).
func seedEmbedded(fsys fs.FS, dir, kind string, unit func(rel string) string) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}
	log := logs.New(kind + "s")

	manifestPath := filepath.Join(dir, ".seed-manifest.json")
	manifest := map[string]string{}
	if b, err := os.ReadFile(manifestPath); err == nil {
		_ = json.Unmarshal(b, &manifest)
	}

	var files []string
	_ = fs.WalkDir(fsys, ".", func(rel string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && !strings.HasSuffix(rel, ".go") {
			files = append(files, rel)
		}
		return nil
	})
	ours := map[string]bool{}
	for rel := range manifest {
		ours[unit(rel)] = true
	}
	var seeded, refreshed, diverged, retired int
	shippedNow := map[string]bool{}
	for _, rel := range files {
		shippedNow[rel] = true
	}
	for _, rel := range files {
		shipped, err := fs.ReadFile(fsys, rel)
		if err != nil {
			continue
		}
		shippedHash := hashBytes(shipped)
		target := filepath.Join(dir, filepath.FromSlash(rel))
		onDisk, readErr := os.ReadFile(target)
		switch {
		case os.IsNotExist(readErr):
			if u := unit(rel); u != rel && !ours[u] {
				if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(u))); err == nil {
					continue // the person's own, of the same name
				}
			}
			if os.MkdirAll(filepath.Dir(target), 0o755) == nil && os.WriteFile(target, shipped, 0o644) == nil {
				manifest[rel] = shippedHash
				ours[unit(rel)] = true
				seeded++
			}
		case readErr == nil && hashBytes(onDisk) == manifest[rel]:
			// Unedited since last seed — track the shipped version.
			if hashBytes(onDisk) != shippedHash {
				if os.WriteFile(target, shipped, 0o644) == nil {
					manifest[rel] = shippedHash
					refreshed++
				}
			}
		case readErr == nil && hashBytes(onDisk) == shippedHash:
			// THE FILE IS THE SHIPPED ONE, whatever the manifest remembers:
			// someone copied it in (a skill edited in the repo and synced
			// by hand while a run was live, 2026-09-18). Adopt it, or the
			// stale hash makes the NEXT shipped change look like a local
			// edit and the file is never refreshed again.
			if manifest[rel] != shippedHash {
				manifest[rel] = shippedHash
			}
		case readErr == nil && hashBytes(onDisk) != shippedHash:
			// User-edited: theirs wins, permanently. Surface drift only when
			// the shipped version also changed since we last seeded.
			if manifest[rel] != "" && manifest[rel] != shippedHash {
				log.Warn(fmt.Sprintf("%s %s: local edits kept, but the shipped version changed — diff against the embedded copy if interested", strings.ToLower(kind), rel),
					slog.String("file", rel), slog.String("path", target))
				diverged++
			}
		}
	}
	// A FILE THE BINARY NO LONGER SHIPS IS RETIRED (2026-09-27: skills for a
	// deleted feature, and fifteen written for a small model, were still on
	// disk, and disk wins, so they stayed loadable and listed). Only what the
	// seed wrote and nobody edited is removed; an edited copy stays and becomes
	// the user's. A directory the removal empties goes with it.
	for rel, hash := range manifest {
		if shippedNow[rel] {
			continue
		}
		target := filepath.Join(dir, filepath.FromSlash(rel))
		if onDisk, err := os.ReadFile(target); err == nil && hashBytes(onDisk) == hash {
			if os.Remove(target) == nil {
				retired++
				for d := filepath.Dir(target); d != dir && os.Remove(d) == nil; d = filepath.Dir(d) {
				}
			}
		}
		delete(manifest, rel)
	}
	if b, err := json.MarshalIndent(manifest, "", "  "); err == nil {
		_ = os.WriteFile(manifestPath, b, 0o644)
	}
	if seeded+refreshed+retired > 0 || diverged > 0 {
		log.Info(fmt.Sprintf("%s seed: %d written, %d refreshed, %d retired, %d locally-edited kept", kind, seeded, refreshed, retired, diverged),
			slog.Int("seeded", seeded), slog.Int("refreshed", refreshed), slog.Int("retired", retired), slog.Int("diverged", diverged),
			slog.String("dir", dir))
	}
}

func hashBytes(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}
