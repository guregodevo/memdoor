package tools

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"memdoor/pkg/shell"
)

// see — the coder looks at a picture. The coder's prompt tells it to look at a
// pasted @…png with see; with no such tool it decoded the pixels with Python
// in bash. The gateway runs the call in the turn (agent_runtime_see.go): the
// picture goes to the turn's model in a one-off request and the answer comes
// back as text, so the conversation stays text.

type SeeInput struct {
	Path     string `json:"path" jsonschema_description:"The picture: an @…png the person pasted, or a png, jpeg, gif or webp file."`
	Question string `json:"question,omitempty" jsonschema_description:"Optional. What to look for. Without it the picture is described in full."`
}

var SeeDefinition = ToolDefinition{
	Name:        "see",
	Description: "Look at a picture (png, jpeg, gif, webp): returns its layout, its text word for word and what looks wrong, or the answer to your question.",
	InputSchema: GenerateSchema[SeeInput](),
	Function: func(json.RawMessage) (string, error) {
		return "", fmt.Errorf("see runs only inside an agent's turn")
	},
}

// SeeMaxBytes is the largest picture see sends: the model APIs refuse more.
const SeeMaxBytes = 5 << 20

var seeMediaTypes = map[string]bool{"image/png": true, "image/jpeg": true, "image/gif": true, "image/webp": true}

// Picture is a file read for see: its media type and its bytes in base64.
type Picture struct {
	Path      string
	MediaType string
	Data      string
}

// ReadPicture reads the picture a see call names. A leading @ is the
// mention's, and a relative path is the project's (workdir), not the
// gateway's. A file over SeeMaxBytes is refused without being loaded whole.
func ReadPicture(path, workdir string) (Picture, error) {
	path = shell.NativePath(strings.TrimPrefix(strings.TrimSpace(path), "@"))
	if path == "" {
		return Picture{}, fmt.Errorf("see: give the picture's path")
	}
	if !filepath.IsAbs(path) && workdir != "" {
		path = filepath.Join(workdir, path)
	}
	f, err := os.Open(path)
	if err != nil {
		return Picture{}, fmt.Errorf("see: %w", err)
	}
	defer f.Close()
	raw, err := io.ReadAll(io.LimitReader(f, SeeMaxBytes+1))
	if err != nil {
		return Picture{}, fmt.Errorf("see: %w", err)
	}
	if len(raw) > SeeMaxBytes {
		return Picture{}, fmt.Errorf("see: %s is over 5 MB, shrink it first", path)
	}
	mediaType := http.DetectContentType(raw)
	if !seeMediaTypes[mediaType] {
		return Picture{}, fmt.Errorf("see: %s is %s, not a png, jpeg, gif or webp picture", path, mediaType)
	}
	return Picture{Path: path, MediaType: mediaType, Data: base64.StdEncoding.EncodeToString(raw)}, nil
}
