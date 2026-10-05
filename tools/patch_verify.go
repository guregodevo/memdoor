package tools

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Execution-grounded patch verification: after a successful apply_patch that
// touched Go files, compile them and fold PASS/FAIL into the tool result. The
// retry loop then acts as a best-of-N sampler with selection-by-first-pass —
// the model cannot skip the build, and a FAIL arrives with the compiler's own
// output in the same tool result that applied the patch. Opt out with
// a test that turns patchVerify off. Non-Go files keep the instructional fallback.

const patchVerifyTimeout = 30 * time.Second

// autoVerifyGoPatch compiles the touched .go files and returns a "verify:" block
// to append to apply_patch's success message. Empty string means "nothing to
// say" (no Go files touched, no toolchain, opted out, or the build timed out) —
// the caller keeps its instructional fallback.
//
// Two modes, chosen by whether a go.mod governs the touched files:
//   - module: `go build -o <tmpdir> ./...` at the module root — catches
//     cross-file and cross-package breakage; -o keeps binaries out of the repo.
//   - loose files (scratch dirs like ~/memdoor-coder): compile ONLY the touched
//     files, sidestepping stale sibling files with their own func main.
//
// patchVerify is the build check after every Go patch. A safety check, so no
// environment variable turns it off (2026-10-04: "secure gate and avoid those
// env vars"); only a test that is not about the build sets it false.
var patchVerify = true

func autoVerifyGoPatch(cwd string, touched []string) string {
	if !patchVerify || goRootDir() == "" {
		return ""
	}
	var goFiles []string
	for _, t := range touched {
		if !strings.HasSuffix(t, ".go") || strings.HasSuffix(t, "_test.go") {
			continue
		}
		if cwd != "" && !filepath.IsAbs(t) {
			t = filepath.Join(cwd, t)
		}
		if _, err := os.Stat(t); err == nil {
			goFiles = append(goFiles, t)
		}
	}
	if len(goFiles) == 0 {
		return ""
	}

	ctx, cancel := context.WithTimeout(context.Background(), patchVerifyTimeout)
	defer cancel()

	var cmd *exec.Cmd
	var label string
	if mi := findGoMod(filepath.Dir(goFiles[0])); mi != nil {
		out, err := os.MkdirTemp("", "memdoor-verify-")
		if err != nil {
			return ""
		}
		defer os.RemoveAll(out)
		cmd = exec.CommandContext(ctx, "go", "build", "-o", out, "./...")
		cmd.Dir = mi.root
		label = "go build ./..."
	} else {
		// All file-list builds must be one package; keep files from the first
		// file's directory only.
		dir := filepath.Dir(goFiles[0])
		var args []string
		for _, f := range goFiles {
			if filepath.Dir(f) == dir {
				args = append(args, filepath.Base(f))
			}
		}
		build := append([]string{"build"}, buildOutputFlag(filepath.Join(dir, args[0]))...)
		cmd = exec.CommandContext(ctx, "go", append(build, args...)...)
		cmd.Dir = dir
		label = "go build " + strings.Join(args, " ")
	}

	out, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return ""
	}
	if err != nil && strings.Contains(string(out), "no main packages to build") {
		// A library-only module: `-o` has nothing to write and Go refuses
		// the whole build. Without -o nothing lands in the module either,
		// since there is no main package. Live 2026-10-03: a rate-limiter
		// package was told "the code does not compile" after every patch.
		cmd = exec.CommandContext(ctx, "go", "build", "./...")
		cmd.Dir = findGoMod(filepath.Dir(goFiles[0])).root
		out, err = cmd.CombinedOutput()
		if ctx.Err() == context.DeadlineExceeded {
			return ""
		}
	}
	if err == nil {
		return "verify: PASS — `" + label + "` compiled clean. Do not re-run the build for this change."
	}
	msg := strings.TrimSpace(string(out))
	if msg == "" {
		msg = err.Error()
	}
	if len(msg) > 1200 {
		msg = msg[:1200] + "\n… (truncated)"
	}
	return "verify: FAIL — `" + label + "`:\n" + msg + "\n\nThe patch IS applied; the code does not compile. Fix exactly these errors with another patch. Do NOT report done."
}

// buildOutputFlag returns ["-o", os.DevNull] for a package-main file so the
// binary never lands in the working directory; a non-main file-list build
// discards its output on its own, and -o would be rejected for it.
func buildOutputFlag(firstFile string) []string {
	b, err := os.ReadFile(firstFile)
	if err != nil {
		return nil
	}
	for _, line := range strings.Split(string(b), "\n") {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "package ") {
			if strings.TrimSpace(strings.TrimPrefix(t, "package ")) == "main" {
				return []string{"-o", os.DevNull}
			}
			return nil
		}
	}
	return nil
}
