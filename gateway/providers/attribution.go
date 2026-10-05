package providers

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	sharedctx "memdoor/pkg/shared/context"
)

// ATTRIBUTION AS A FIELD (Greg, 2026-10-02: a company gateway "tags every
// request with your email and team for cost attribution"; the UX read of
// omp found only static headers). Four fields travel on every vendor and
// gateway request — user, team, project, session — as headers whose names
// the company chooses:
//
//	MEMDOOR_ATTRIBUTION="user=me@corp.example;team=data-eng"   the person's fields
//	MEMDOOR_ATTRIBUTION_HEADERS="user=X-Email;team=X-Team"      the gateway's names
//	                                                            (default X-Memdoor-User …)
//
// project and session are filled by the gateway from the turn (the
// project's directory name, the conversation's session id) through
// SetAttributionFunc; a field with no value sends no header. The
// company's preset sets the first two variables; MEMDOOR_VENDOR_HEADERS
// stays for anything static beside them.
const (
	AttrUser    = "user"
	AttrTeam    = "team"
	AttrProject = "project"
	AttrSession = "session"
)

var attributionFunc func(ctx context.Context) map[string]string

// SetAttributionFunc registers what the gateway knows about a turn
// (project, session, the signed-in user when it has one).
func SetAttributionFunc(fn func(ctx context.Context) map[string]string) { attributionFunc = fn }

// parsePairs reads "k=v;k=v" (";" or newline separated) into a map.
func parsePairs(raw string) map[string]string {
	out := map[string]string{}
	for _, pair := range strings.FieldsFunc(raw, func(r rune) bool { return r == ';' || r == '\n' }) {
		k, v, ok := strings.Cut(pair, "=")
		if k, v = strings.TrimSpace(strings.ToLower(k)), strings.TrimSpace(v); ok && k != "" && v != "" {
			out[k] = v
		}
	}
	return out
}

// Attribution is the four fields for a request: the environment's, then
// the turn's (the environment wins for a field both name — a preset is
// the company's word).
func Attribution(ctx context.Context) map[string]string {
	out := map[string]string{}
	if attributionFunc != nil && ctx != nil {
		for k, v := range attributionFunc(ctx) {
			if v != "" {
				out[k] = v
			}
		}
	}
	if ctx != nil {
		if sid, _ := ctx.Value(sharedctx.SessionIDKey).(string); sid != "" {
			out[AttrSession] = sid
		}
		if wd, _ := ctx.Value(sharedctx.WorkdirKey).(string); wd != "" {
			out[AttrProject] = filepath.Base(filepath.Clean(wd))
		}
	}
	for k, v := range parsePairs(os.Getenv("MEMDOOR_ATTRIBUTION")) {
		out[k] = v
	}
	return out
}

// attributionHeaderNames is the header for each field: the company's
// names from MEMDOOR_ATTRIBUTION_HEADERS, else X-Memdoor-<Field>.
func attributionHeaderNames() map[string]string {
	names := map[string]string{AttrUser: "X-Memdoor-User", AttrTeam: "X-Memdoor-Team", AttrProject: "X-Memdoor-Project", AttrSession: "X-Memdoor-Session"}
	for k, v := range parsePairs(os.Getenv("MEMDOOR_ATTRIBUTION_HEADERS")) {
		names[k] = v
	}
	return names
}

// setAttributionHeaders puts the turn's fields on a vendor or gateway
// request under the company's names; a field with no value sends nothing.
func setAttributionHeaders(ctx context.Context, h http.Header) {
	names := attributionHeaderNames()
	for field, value := range Attribution(ctx) {
		if name, ok := names[field]; ok && value != "" {
			h.Set(name, value)
		}
	}
}
