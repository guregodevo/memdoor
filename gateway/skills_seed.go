package gateway

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"memdoor/gateway/logs"
	"memdoor/pkg/shared"
	"memdoor/skills"
)

// SeedEmbeddedSkills copies the embedded skill library to the managed skills
// dir (~/.memdoor/skills) at boot, making DISK the source of truth: edits
// there take effect on the next skill call with no rebuild, and the embedded
// copy is only a fallback for a machine that has never booted.
//
// The seed never destroys user work. A manifest of shipped-content hashes
// (.seed-manifest.json) distinguishes "still as we shipped it" from "user
// edited": an absent file is written; an unedited file is refreshed when the
// shipped version changes; an edited file is left alone forever (a WARN notes
// when the shipped version has moved on so the divergence is visible).
func SeedEmbeddedSkills() {
	dir := shared.MemdoorHome("skills")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}
	log := logs.New("Skills")

	manifestPath := filepath.Join(dir, ".seed-manifest.json")
	manifest := map[string]string{}
	if b, err := os.ReadFile(manifestPath); err == nil {
		_ = json.Unmarshal(b, &manifest)
	}

	entries, err := skills.FS.ReadDir(".")
	if err != nil {
		return
	}
	var seeded, refreshed, diverged, retired int
	shippedNow := map[string]bool{}
	for _, e := range entries {
		shippedNow[e.Name()] = true
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		shipped, err := skills.FS.ReadFile(e.Name())
		if err != nil {
			continue
		}
		shippedHash := hashBytes(shipped)
		target := filepath.Join(dir, e.Name())
		onDisk, readErr := os.ReadFile(target)
		switch {
		case os.IsNotExist(readErr):
			if os.WriteFile(target, shipped, 0o644) == nil {
				manifest[e.Name()] = shippedHash
				seeded++
			}
		case readErr == nil && hashBytes(onDisk) == manifest[e.Name()]:
			// Unedited since last seed — track the shipped version.
			if hashBytes(onDisk) != shippedHash {
				if os.WriteFile(target, shipped, 0o644) == nil {
					manifest[e.Name()] = shippedHash
					refreshed++
				}
			}
		case readErr == nil && hashBytes(onDisk) == shippedHash:
			// THE FILE IS THE SHIPPED ONE, whatever the manifest remembers:
			// someone copied it in (a skill edited in the repo and synced
			// by hand while a run was live, 2026-09-18). Adopt it, or the
			// stale hash makes the NEXT shipped change look like a local
			// edit and the file is never refreshed again.
			if manifest[e.Name()] != shippedHash {
				manifest[e.Name()] = shippedHash
			}
		case readErr == nil && hashBytes(onDisk) != shippedHash:
			// User-edited: theirs wins, permanently. Surface drift only when
			// the shipped version also changed since we last seeded.
			if manifest[e.Name()] != "" && manifest[e.Name()] != shippedHash {
				log.Warn(fmt.Sprintf("skill %s: local edits kept, but the shipped version changed — diff against the embedded copy if interested", e.Name()),
					slog.String("skill", e.Name()), slog.String("path", target))
				diverged++
			}
		}
	}
	// A SKILL THE BINARY NO LONGER SHIPS IS RETIRED (2026-09-27: skills for a
	// deleted feature, and fifteen written for a small model, were still on
	// disk, and disk wins, so they stayed loadable and listed). Only what the
	// seed wrote and nobody edited is removed; an edited copy stays and becomes
	// the user's.
	for name, hash := range manifest {
		if shippedNow[name] {
			continue
		}
		target := filepath.Join(dir, name)
		if onDisk, err := os.ReadFile(target); err == nil && hashBytes(onDisk) == hash {
			if os.Remove(target) == nil {
				retired++
			}
		}
		delete(manifest, name)
	}
	if b, err := json.MarshalIndent(manifest, "", "  "); err == nil {
		_ = os.WriteFile(manifestPath, b, 0o644)
	}
	if seeded+refreshed+retired > 0 || diverged > 0 {
		log.Info(fmt.Sprintf("Skill seed: %d written, %d refreshed, %d retired, %d locally-edited kept", seeded, refreshed, retired, diverged),
			slog.Int("seeded", seeded), slog.Int("refreshed", refreshed), slog.Int("retired", retired), slog.Int("diverged", diverged),
			slog.String("dir", dir))
	}
}

func hashBytes(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}
