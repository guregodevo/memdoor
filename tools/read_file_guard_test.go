package tools

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadFileRefusesBinaryAndCapsText(t *testing.T) {
	dir := t.TempDir()
	png := filepath.Join(dir, "strip.png")
	os.WriteFile(png, append([]byte("\x89PNG\r\n"), make([]byte, 5000)...), 0o644)
	in, _ := json.Marshal(ReadFileInput{Path: png})
	out, err := ReadFile(in)
	if err != nil || !strings.Contains(out, "is a binary file") || !strings.Contains(out, "use a tool made for that file type") {
		t.Fatalf("png: %v %q", err, out)
	}
	big := filepath.Join(dir, "big.txt")
	os.WriteFile(big, []byte(strings.Repeat("line of text é\n", 12000)), 0o644)
	in, _ = json.Marshal(ReadFileInput{Path: big})
	out, _ = ReadFile(in)
	if strings.Count(out, "line of text") != readFileLines || !strings.Contains(out, "(12000 in all). read_file with offset: 301 to continue]") {
		t.Fatalf("big text is paged: len %d, tail %q", len(out), out[len(out)-100:])
	}
	in, _ = json.Marshal(ReadFileInput{Path: big, All: true})
	out, _ = ReadFile(in)
	if len(out) > readFileMax+300 || !strings.Contains(out, "truncated at 128 KB") {
		t.Fatalf("big text, all: len %d", len(out))
	}
	small := filepath.Join(dir, "a.go")
	os.WriteFile(small, []byte("package a\n// é\n"), 0o644)
	in, _ = json.Marshal(ReadFileInput{Path: small})
	if out, _ = ReadFile(in); out != "package a\n// é\n" {
		t.Fatalf("small: %q", out)
	}
}

func TestReadFileJudgesLongFilesAgainstTheTurnTask(t *testing.T) {
	withDecisions(t, &scriptedDecisions{p: func(q string) float64 {
		if strings.Contains(q, "license") {
			return 0.9
		}
		return 0.05
	}})
	dir := t.TempDir()
	path := filepath.Join(dir, "v.source.json")
	var formats []string
	for i := 0; i < 900; i++ {
		formats = append(formats, `{"format_id":"f`+strings.Repeat("x", 3)+`","url":"https://example.com/`+strings.Repeat("a", 40)+`"}`)
	}
	blob := `{"title":"Sourdough","formats":[` + strings.Join(formats, ",") + `],"license":"Creative Commons Attribution license (reuse allowed)","duration":941}`
	os.WriteFile(path, []byte(blob), 0o644)
	in, _ := json.Marshal(map[string]any{"path": path, "turn_task": "Find the video's licence and duration."})
	out, err := ReadFile(in)
	if err != nil || !strings.Contains(out, "judged against this turn's task") || !strings.Contains(out, "Creative Commons") || len(out) > 20000 {
		t.Fatalf("judged read: %v len %d\n%.300s", err, len(out), out)
	}
	// all: true returns the file.
	in, _ = json.Marshal(map[string]any{"path": path, "turn_task": "x", "all": true})
	if out, _ = ReadFile(in); !strings.HasPrefix(out, `{"title":"Sourdough"`) {
		t.Fatalf("all: %.100s", out)
	}
	// No task: the capped/whole text as before.
	in, _ = json.Marshal(map[string]any{"path": path})
	if out, _ = ReadFile(in); strings.Contains(out, "judged against") {
		t.Fatal("judged without a task")
	}
}
