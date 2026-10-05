package cmd

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"memdoor/cmd/tui/ui"
)

// A conversation too long for one share keeps its newest messages and says
// how many of the oldest were left out.
func TestShareDropsTheOldestToFit(t *testing.T) {
	var msgs []ui.Message
	for i := 0; i < 600; i++ {
		msgs = append(msgs, ui.Message{Role: "assistant", Content: strings.Repeat("x", 4000)})
	}
	msgs = append(msgs, ui.Message{Role: "user", Content: "the last question"})
	key := make([]byte, 32)
	sealed, doc, err := sealShare(newShareDoc(msgs, time.Now()), key)
	if err != nil {
		t.Fatal(err)
	}
	if len(sealed) > shareMaxBytes || doc.Dropped == 0 {
		t.Fatalf("%d bytes, %d dropped", len(sealed), doc.Dropped)
	}
	if last := doc.Messages[len(doc.Messages)-1]; last.Text != "the last question" {
		t.Fatalf("the newest must stay: %q", last.Text)
	}
}

func TestShareNeedsAnAccount(t *testing.T) {
	if _, err := tuiShare(shareSession{ChannelID: "c"})(""); err == nil || !strings.Contains(err.Error(), "memdoor login") {
		t.Fatalf("no account: %v", err)
	}
}

// The page (web/src/shareCrypto.ts, run in Node) opens what the terminal
// sealed, with the key from the link: the contract between the two sides.
func TestShareOpensInThePage(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is not installed")
	}
	esbuild, _ := filepath.Abs("../../../web/node_modules/.bin/esbuild")
	if _, err := os.Stat(esbuild); err != nil {
		t.Skip("web/node_modules not installed")
	}
	dir := t.TempDir()
	src, _ := filepath.Abs("../../../web/src/shareCrypto.ts")
	if out, err := exec.Command(esbuild, src, "--bundle", "--format=esm", "--outfile="+filepath.Join(dir, "shareCrypto.mjs")).CombinedOutput(); err != nil {
		t.Fatalf("esbuild: %v\n%s", err, out)
	}
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	sealed, _, err := sealShare(newShareDoc([]ui.Message{
		{Role: "user", Content: "what is /share?"},
		{Role: "assistant", Content: "A read-only link, **sealed** here."},
	}, time.Now()), key)
	if err != nil {
		t.Fatal(err)
	}
	driver := filepath.Join(dir, "check.mjs")
	_ = os.WriteFile(driver, []byte(`
import { openShare } from './shareCrypto.mjs';
import { readFileSync } from 'node:fs';
const [key, file] = process.argv.slice(2);
const doc = await openShare(key, new Uint8Array(readFileSync(file)));
let wrong = 'rejected';
try { await openShare('AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA', new Uint8Array(readFileSync(file))); wrong = 'opened'; } catch {}
console.log(JSON.stringify({ title: doc.title, n: doc.messages.length, last: doc.messages[1].text, wrong }));
`), 0o644)
	blob := filepath.Join(dir, "share.bin")
	_ = os.WriteFile(blob, sealed, 0o600)
	out, err := exec.Command(node, driver, base64.RawURLEncoding.EncodeToString(key), blob).CombinedOutput()
	if err != nil {
		t.Fatalf("node: %v\n%s", err, out)
	}
	var got struct {
		Title, Last, Wrong string
		N                  int
	}
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("node output: %s", out)
	}
	if got.Title != "what is /share?" || got.N != 2 || got.Last != "A read-only link, **sealed** here." || got.Wrong != "rejected" {
		t.Fatalf("the page read: %+v", got)
	}
}
