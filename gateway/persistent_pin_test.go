package gateway

import (
	"os"
	"path/filepath"
	"testing"
)

// The pin stays: written by /model, taken by every conversation that has no
// route of its own, removed by /model auto.
func TestThePinStays(t *testing.T) {
	dir := t.TempDir()
	old := persistentPinPath
	persistentPinPath = func() string { return filepath.Join(dir, ".memdoor", "model.json") }
	t.Cleanup(func() { persistentPinPath = old })

	if readPersistentPin("greg") != nil {
		t.Fatal("no file, no pin")
	}
	if err := writePersistentPin("greg", &persistentPin{Model: "zai-org/GLM-5.3", Sort: "price"}); err != nil {
		t.Fatal(err)
	}
	if p := readPersistentPin("greg"); p == nil || p.Model != "zai-org/GLM-5.3" || p.Sort != "price" {
		t.Fatalf("read back: %+v", p)
	}
	turn := &Session{ID: "s1", Metadata: map[string]interface{}{}}
	reg := &Session{ID: "s1", Metadata: map[string]interface{}{}}
	if !applyPersistentPin("greg", turn, reg) {
		t.Fatal("an unrouted conversation must take the kept pin")
	}
	for _, s := range []*Session{turn, reg} {
		if m, _ := s.GetMetadataValue(sessionModelKey); m != "zai-org/GLM-5.3" || !sessionPinned(s) || sessionReason(s) != reasonPinned {
			t.Fatalf("pinned: %v %v %q", m, sessionPinned(s), sessionReason(s))
		}
	}
	// The route view of a fresh conversation says pinned, before any turn.
	if v := viewRoute(nil, turn, "coder"); !v.Pinned || v.Model != "zai-org/GLM-5.3" || v.Reason != reasonPinned {
		t.Fatalf("the footer must show the kept pin: %+v", v)
	}
	own := &Session{ID: "s2", Metadata: map[string]interface{}{}}
	if err := pinModel(own, "deepseek/deepseek-flash", "", ""); err != nil {
		t.Fatal(err)
	}
	if applyPersistentPin("greg", own) {
		t.Fatal("a conversation with its own pin keeps it")
	}
	if err := writePersistentPin("greg", &persistentPin{Rung: 2}); err != nil {
		t.Fatal(err)
	}
	fresh := &Session{ID: "s3", Metadata: map[string]interface{}{}}
	if !applyPersistentPin("greg", fresh) {
		t.Fatal("a rung pin applies")
	}
	if tier, _ := fresh.GetMetadataValue(sessionTierKey); tier != 1 {
		t.Fatalf("rung 2 is tier 1, got %v", tier)
	}
	if err := writePersistentPin("greg", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(persistentPinPath()); !os.IsNotExist(err) {
		t.Fatal("auto removes the file")
	}
}

// A WORKFLOW STEP FOLLOWS THE KEPT PIN when its task names no model, and its
// own `model:` wins when it does (dogfood 2026-10-04: a resumed run answered
// on the ladder while the footer said "pinned by you").
func TestAWorkflowStepFollowsTheKeptPin(t *testing.T) {
	dir := t.TempDir()
	old := persistentPinPath
	persistentPinPath = func() string { return filepath.Join(dir, ".memdoor", "model.json") }
	t.Cleanup(func() { persistentPinPath = old })
	if err := writePersistentPin("greg", &persistentPin{Model: "deepseek/deepseek-flash"}); err != nil {
		t.Fatal(err)
	}
	step := &Session{ID: "workflow:w:p:a", Metadata: map[string]interface{}{}}
	if err := pinTaskModel(step, "", "greg"); err != nil {
		t.Fatal(err)
	}
	if m, _ := step.GetMetadataValue(sessionModelKey); m != "deepseek/deepseek-flash" {
		t.Fatalf("a step with no model of its own takes the kept pin: %v", m)
	}
	own := &Session{ID: "workflow:w:p:b", Metadata: map[string]interface{}{}}
	if err := pinTaskModel(own, "zai-org/GLM-5.3", "greg"); err != nil {
		t.Fatal(err)
	}
	if m, _ := own.GetMetadataValue(sessionModelKey); m != "zai-org/GLM-5.3" {
		t.Fatalf("the task's own model wins: %v", m)
	}
}

// THE PIN IS THE PERSON'S. On a gateway several people use, one person's
// /model never becomes another's default; a run with no known starter (a
// schedule) takes the gateway's pin only when one person kept one.
// Mutation check: read the pin without the actor and Bob starts on Alice's.
func TestThePinIsThePersons(t *testing.T) {
	dir := t.TempDir()
	old := persistentPinPath
	persistentPinPath = func() string { return filepath.Join(dir, ".memdoor", "model.json") }
	t.Cleanup(func() { persistentPinPath = old })

	if err := writePersistentPin("alice", &persistentPin{Model: "zai-org/GLM-5.3"}); err != nil {
		t.Fatal(err)
	}
	bob := &Session{ID: "b1", Metadata: map[string]interface{}{}}
	if applyPersistentPin("bob", bob) {
		t.Fatal("Bob must not start on Alice's pin")
	}
	alice := &Session{ID: "a1", Metadata: map[string]interface{}{}}
	if !applyPersistentPin("alice", alice) {
		t.Fatal("Alice's new conversation starts on her pin")
	}
	if p := readPersistentPin(""); p == nil || p.Model != "zai-org/GLM-5.3" {
		t.Fatalf("one pin on the gateway: a schedule takes it, got %+v", p)
	}
	if err := writePersistentPin("bob", &persistentPin{Rung: 2}); err != nil {
		t.Fatal(err)
	}
	if p := readPersistentPin(""); p != nil {
		t.Fatalf("two people's pins: a schedule borrows nobody's, got %+v", p)
	}
	if err := writePersistentPin("alice", nil); err != nil {
		t.Fatal(err)
	}
	if p := readPersistentPin("bob"); p == nil || p.Rung != 2 {
		t.Fatalf("Alice's auto leaves Bob's pin: %+v", p)
	}
}
