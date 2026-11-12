package logs

import (
	"fmt"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"memdoor/pkg/shared"
)

// DefaultDir is where the gateway keeps its event log, ~/.memdoor/logs.
func DefaultDir() string { return shared.MemdoorHome("logs") }

// DefaultPath is the event log's file in DefaultDir.
func DefaultPath() string { return filepath.Join(DefaultDir(), "events.db") }

// ParseDuration reads a --since: a Go duration ("90m") or whole days ("7d").
func ParseDuration(s string) (time.Duration, error) {
	if strings.HasSuffix(s, "d") {
		var d int
		if _, err := fmt.Sscanf(strings.TrimSuffix(s, "d"), "%d", &d); err != nil {
			return 0, fmt.Errorf("invalid duration: %s", s)
		}
		return time.Duration(d) * 24 * time.Hour, nil
	}
	return time.ParseDuration(s)
}

// QueryFromParams is the query GET /api/logs/query takes from its URL
// parameters (since, levels, components, session, run, errors_only, regex,
// workspace, limit). `memdoor logs query` reading this machine's file builds
// the same one.
func QueryFromParams(v url.Values) (*Query, error) {
	qb := NewQueryBuilder()
	if since := v.Get("since"); since != "" {
		d, err := ParseDuration(since)
		if err != nil {
			return nil, fmt.Errorf("invalid since: %w", err)
		}
		qb = qb.Since(d)
	}
	if levels := v.Get("levels"); levels != "" {
		var ls []Level
		for _, l := range strings.Split(levels, ",") {
			ls = append(ls, Level(strings.TrimSpace(l)))
		}
		qb = qb.Levels(ls...)
	}
	if components := v.Get("components"); components != "" {
		qb = qb.Components(strings.Split(components, ",")...)
	}
	if session := v.Get("session"); session != "" {
		qb = qb.Session(session)
	}
	if run := v.Get("run"); run != "" {
		qb = qb.RunID(run)
	}
	if v.Get("errors_only") == "true" {
		qb = qb.ErrorsOnly()
	}
	if regex := v.Get("regex"); regex != "" {
		qb = qb.MessageRegex(regex)
	}
	// Workspace filter: projects on data.workspace, which the
	// workspace-scoped event writers attach via slog.String("workspace", ws).
	if workspace := v.Get("workspace"); workspace != "" {
		qb = qb.Workspace(workspace)
	}
	if limit, err := strconv.Atoi(v.Get("limit")); err == nil {
		qb = qb.Limit(limit)
	}
	return qb.Build(), nil
}

// ErrorsQueryFromParams is the query GET /api/logs/errors takes: the errors
// since (default 24h), at most limit (default 20).
func ErrorsQueryFromParams(v url.Values) (*Query, error) {
	since := v.Get("since")
	if since == "" {
		since = "24h"
	}
	d, err := ParseDuration(since)
	if err != nil {
		return nil, fmt.Errorf("invalid since: %w", err)
	}
	limit := 20
	if l, err := strconv.Atoi(v.Get("limit")); err == nil {
		limit = l
	}
	return NewQueryBuilder().Since(d).Levels(LevelError).Limit(limit).Build(), nil
}
