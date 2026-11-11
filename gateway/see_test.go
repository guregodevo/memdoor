package gateway

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"memdoor/gateway/providers"
	sharedctx "memdoor/pkg/shared/context"
)

// see sends the picture the coder names, from the turn's project, to the
// turn's model as an image part, and returns what the model saw.
func TestSeeShowsAPictureInTheProjectToTheTurnsModel(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	asked := fakeOpenRouter(t, "A login form; the Submit button overlaps the footer.")
	project := t.TempDir()
	if err := os.WriteFile(filepath.Join(project, "shot.png"), []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR"), 0o644); err != nil {
		t.Fatal(err)
	}
	ar := &AgentRuntime{clientFactory: providers.NewClientFactory()}
	ctx := context.WithValue(context.WithValue(context.Background(), sharedctx.WorkdirKey, project), "buddy_agent_name", "coder")
	in, _ := json.Marshal(map[string]string{"path": "@shot.png", "question": "is anything misaligned?"})

	out, err := ar.runSee(ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "overlaps the footer") {
		t.Fatalf("answer %q", out)
	}
	if len(*asked) != 1 {
		t.Fatalf("%d requests", len(*asked))
	}
	got := (*asked)[0]
	for _, want := range []string{"image_url", "data:image/png;base64,", "is anything misaligned?"} {
		if !strings.Contains(got, want) {
			t.Errorf("the request lacks %q: %s", want, got)
		}
	}
}

func TestSeeRefusesBeforeAskingAModel(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	asked := fakeOpenRouter(t, "never")
	ar := &AgentRuntime{clientFactory: providers.NewClientFactory()}
	ctx := context.WithValue(context.Background(), sharedctx.WorkdirKey, t.TempDir())
	in, _ := json.Marshal(map[string]string{"path": "missing.png"})
	if _, err := ar.runSee(ctx, in); err == nil {
		t.Fatal("a missing picture is an error")
	}
	if len(*asked) != 0 {
		t.Fatalf("the model was asked about a missing picture: %v", *asked)
	}
}
