package cmd

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/coder/acp-go-sdk"

	"memdoor/pkg/shared"
	"memdoor/pkg/workflow"
)

// What an editor attaches. The editor offers a file picker and image paste
// only when initialize declares the image and embeddedContext prompt
// capabilities, and promptText kept only text blocks, so an attachment was
// dropped without a word. An attachment now becomes what the window makes of
// one: a file is an @mention, an embedded excerpt is quoted under its file,
// and a picture is saved under the project's .memdoor folder (git-ignored, as
// cmd/tui/ui/paste.go does) and mentioned for the coder to look at with see.

var acpPromptCapabilities = acp.PromptCapabilities{Image: true, EmbeddedContext: true}

// pictureExtensions names a saved picture's file by its MIME type.
var pictureExtensions = map[string]string{
	"image/png": ".png", "image/jpeg": ".jpg", "image/jpg": ".jpg", "image/gif": ".gif", "image/webp": ".webp",
}

// promptText turns the prompt's blocks into the one request the coder reads.
// cwd is the session's project folder, where pictures are kept.
func promptText(cwd string, blocks []acp.ContentBlock) string {
	var parts []string
	for _, c := range blocks {
		switch {
		case c.Text != nil:
			parts = append(parts, c.Text.Text)
		case c.ResourceLink != nil:
			parts = append(parts, "@"+attachmentPath(c.ResourceLink.Uri, c.ResourceLink.Name))
		case c.Image != nil:
			parts = append(parts, savedPicture(cwd, c.Image.Data, c.Image.MimeType))
		case c.Resource != nil:
			r := c.Resource.Resource
			switch {
			case r.TextResourceContents != nil:
				t := r.TextResourceContents
				parts = append(parts, fmt.Sprintf("@%s\n```\n%s\n```", attachmentPath(t.Uri, ""), strings.TrimRight(t.Text, "\n")))
			case r.BlobResourceContents != nil:
				b := r.BlobResourceContents
				mime := ""
				if b.MimeType != nil {
					mime = *b.MimeType
				}
				if _, ok := pictureExtensions[strings.ToLower(mime)]; ok {
					parts = append(parts, savedPicture(cwd, b.Blob, mime))
				} else {
					parts = append(parts, "@"+attachmentPath(b.Uri, ""))
				}
			}
		}
	}
	return strings.Join(parts, "\n")
}

// attachmentPath is a file: URI's local path; any other URI as it is, or the
// name when there is no URI.
func attachmentPath(uri, name string) string {
	if uri == "" {
		return name
	}
	u, err := url.Parse(uri)
	if err != nil || u.Scheme != "file" {
		return uri
	}
	p := u.Path
	if u.Host != "" && u.Host != "localhost" {
		p = "//" + u.Host + p // file://server/share/x is a UNC path
	}
	if runtime.GOOS == "windows" && len(p) >= 3 && p[0] == '/' && p[2] == ':' {
		p = p[1:] // file:///C:/x parses to /C:/x
	}
	return filepath.FromSlash(p)
}

// savedPicture writes a base64 picture to <cwd>/.memdoor/paste-<time>-<n>.<ext>
// and returns its @mention, or a line saying why it was not kept.
func savedPicture(cwd, data, mime string) string {
	ext, ok := pictureExtensions[strings.ToLower(mime)]
	if !ok {
		return fmt.Sprintf("(an attached %s picture was left out: only png, jpeg, gif and webp are kept)", mime)
	}
	raw, err := base64.StdEncoding.DecodeString(data)
	if err != nil || len(raw) == 0 {
		return "(an attached picture could not be read)"
	}
	dir := filepath.Join(cwd, shared.MemdoorDirName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Sprintf("(an attached picture could not be saved: %v)", err)
	}
	workflow.EnsureIgnored(cwd)
	f, err := os.CreateTemp(dir, "paste-"+time.Now().Format("20060102-150405")+"-*"+ext)
	if err != nil {
		return fmt.Sprintf("(an attached picture could not be saved: %v)", err)
	}
	_, werr := f.Write(raw)
	if cerr := f.Close(); werr == nil {
		werr = cerr
	}
	if werr != nil {
		_ = os.Remove(f.Name())
		return fmt.Sprintf("(an attached picture could not be saved: %v)", werr)
	}
	return "@" + f.Name()
}
