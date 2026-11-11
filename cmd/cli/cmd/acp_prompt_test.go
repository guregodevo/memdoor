package cmd

import (
	"context"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/coder/acp-go-sdk"
)

// What an editor attaches reaches the coder: a file as an @mention, an
// excerpt quoted under its file, a picture saved where git ignores it and
// mentioned; text stays as it was.
func TestAnEditorsAttachmentsReachTheCoder(t *testing.T) {
	cwd := t.TempDir()
	png := []byte("\x89PNG\r\n\x1a\nshot")
	mime := "image/png"
	got := promptText(cwd, []acp.ContentBlock{
		acp.TextBlock("look at these"),
		{ResourceLink: &acp.ContentBlockResourceLink{Uri: "file://" + filepath.Join(cwd, "notes.txt"), Name: "notes.txt"}},
		{Resource: &acp.ContentBlockResource{Resource: acp.EmbeddedResourceResource{TextResourceContents: &acp.TextResourceContents{Uri: "file:///src/a.go", Text: "package a\n"}}}},
		{Image: &acp.ContentBlockImage{Data: base64.StdEncoding.EncodeToString(png), MimeType: "image/png"}},
		{Resource: &acp.ContentBlockResource{Resource: acp.EmbeddedResourceResource{BlobResourceContents: &acp.BlobResourceContents{Uri: "file:///x.bin", Blob: "AA==", MimeType: &mime}}}},
	})
	lines := strings.Split(got, "\n")
	if lines[0] != "look at these" || lines[1] != "@"+filepath.Join(cwd, "notes.txt") {
		t.Fatalf("%q", got)
	}
	if !strings.Contains(got, "@"+filepath.FromSlash("/src/a.go")+"\n```\npackage a\n```") {
		t.Fatalf("the excerpt: %q", got)
	}
	pics, _ := filepath.Glob(filepath.Join(cwd, ".memdoor", "paste-*.png"))
	if len(pics) != 2 {
		t.Fatalf("pictures saved: %v", pics)
	}
	var kept []string
	for _, p := range pics {
		b, _ := os.ReadFile(p)
		kept = append(kept, string(b))
	}
	if !strings.Contains(strings.Join(kept, "|"), string(png)) {
		t.Fatalf("the pasted picture's bytes were not kept: %q", kept)
	}
	for _, p := range pics {
		if !strings.Contains(got, "@"+p) {
			t.Errorf("%s not mentioned in %q", p, got)
		}
	}
	ignore, _ := os.ReadFile(filepath.Join(cwd, ".memdoor", ".gitignore"))
	if !strings.Contains(string(ignore), "paste-*.png") {
		t.Fatalf(".gitignore: %q", ignore)
	}
}

func TestAPictureThatCannotBeKeptSaysSo(t *testing.T) {
	cwd := t.TempDir()
	got := promptText(cwd, []acp.ContentBlock{
		{Image: &acp.ContentBlockImage{Data: "AA==", MimeType: "image/tiff"}},
		{Image: &acp.ContentBlockImage{Data: "not base64!", MimeType: "image/jpeg"}},
	})
	if !strings.Contains(got, "image/tiff picture was left out") || !strings.Contains(got, "could not be read") {
		t.Fatalf("%q", got)
	}
}

// The model option exists only when MEMDOOR_ACP_MODELS lists models; then a
// pick pins it on the session's conversation and "default" hands it back.
func TestAnEditorPicksTheModelOnlyWhenModelsAreConfigured(t *testing.T) {
	runner := &fakeRunner{}
	a := newACPAgent(runner)
	ctx := context.Background()
	sess, err := a.NewSession(ctx, acp.NewSessionRequest{Cwd: t.TempDir()})
	if err != nil || sess.ConfigOptions != nil {
		t.Fatalf("no models configured, yet options %+v (%v)", sess.ConfigOptions, err)
	}
	if _, err := a.SetSessionConfigOption(ctx, acp.SetSessionConfigOptionRequest{ValueId: &acp.SetSessionConfigOptionValueId{SessionId: sess.SessionId, ConfigId: acpModelOption, Value: "x"}}); err == nil {
		t.Fatal("no models configured: set_config_option is not offered")
	}

	t.Setenv(acpModelsEnv, "anthropic:claude-haiku-5-5|Haiku; openrouter:z-ai/glm-5.3")
	t.Setenv(acpDefaultNameEnv, "GLM Flash")
	a.models, a.defaultModelName = acpModelsFromEnv()
	sess, _ = a.NewSession(ctx, acp.NewSessionRequest{Cwd: t.TempDir()})
	sel := sess.ConfigOptions[0].Select
	opts := *sel.Options.Ungrouped
	if sel.CurrentValue != acpModelDefault || len(opts) != 3 || opts[0].Name != "Default (GLM Flash)" || opts[1].Name != "Haiku" || opts[2].Name != "openrouter:z-ai/glm-5.3" {
		t.Fatalf("%+v %+v", sel, opts)
	}
	set := func(v acp.SessionConfigValueId) error {
		_, err := a.SetSessionConfigOption(ctx, acp.SetSessionConfigOptionRequest{ValueId: &acp.SetSessionConfigOptionValueId{SessionId: sess.SessionId, ConfigId: acpModelOption, Value: v}})
		return err
	}
	if err := set("anthropic:claude-haiku-5-5"); err != nil {
		t.Fatal(err)
	}
	if err := set(acpModelDefault); err != nil {
		t.Fatal(err)
	}
	if err := set("openai:not-offered"); err == nil {
		t.Fatal("a model not offered is refused")
	}
	if strings.Join(runner.models, ",") != "conv-1=anthropic:claude-haiku-5-5,conv-1=" {
		t.Fatalf("pins %v", runner.models)
	}
}
