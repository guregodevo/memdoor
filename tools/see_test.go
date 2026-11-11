package tools

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var pngHeader = []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR")

func TestReadPictureReadsAMentionInTheProject(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "shot.png"), pngHeader, 0o644); err != nil {
		t.Fatal(err)
	}
	p, err := ReadPicture(" @shot.png ", dir)
	if err != nil {
		t.Fatal(err)
	}
	if p.MediaType != "image/png" || p.Data != base64.StdEncoding.EncodeToString(pngHeader) || p.Path != filepath.Join(dir, "shot.png") {
		t.Fatalf("%+v", p)
	}
}

func TestReadPictureRefusesWhatIsNotAPicture(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "notes.png"), []byte("just text"), 0o644)
	if _, err := ReadPicture("notes.png", dir); err == nil || !strings.Contains(err.Error(), "not a png") {
		t.Fatalf("%v", err)
	}
	if _, err := ReadPicture("missing.png", dir); err == nil {
		t.Fatal("a missing file is an error")
	}
	if _, err := ReadPicture("  ", dir); err == nil {
		t.Fatal("no path is an error")
	}
}

func TestReadPictureRefusesAPictureOverTheLimit(t *testing.T) {
	p := filepath.Join(t.TempDir(), "big.png")
	_ = os.WriteFile(p, append(append([]byte{}, pngHeader...), make([]byte, SeeMaxBytes)...), 0o644)
	if _, err := ReadPicture(p, ""); err == nil || !strings.Contains(err.Error(), "over 5 MB") {
		t.Fatalf("%v", err)
	}
}
