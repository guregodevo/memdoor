package gateway

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strings"
)

// Every sign-up gets a workspace of its own, like Slack (Greg, 2026-10-20:
// "Should be like slack", "Workspace will be important for enterprise but
// for solo"): a solo person types nothing more than their email and password
// and lands in "<their name>'s workspace", admin of it, with the coding team
// seeded; an invite joins the inviter's workspace instead. The named
// workspace of a team is the setup flow and, later, the enterprise form.

// workspaceMaker is what the sign-up handlers ask of the server: the row
// first (the account needs its id), the seeding once the owner exists.
type workspaceMaker interface {
	NewWorkspace(ctx context.Context, name, slugHint string) (slug string, err error)
	SeedWorkspace(ctx context.Context, slug, name, ownerUserID string)
}

// SetWorkspaces gives the adapter the server that makes workspaces.
func (a *AuthGatewayAdapter) SetWorkspaces(m workspaceMaker) { a.workspaces = m }

// personalWorkspaceName is the name a sign-up's own workspace gets when the
// person named none.
func personalWorkspaceName(displayName, username string) string {
	who := strings.TrimSpace(displayName)
	if who == "" {
		who = username
	}
	if who == "" {
		return "My workspace"
	}
	if strings.HasSuffix(strings.ToLower(who), "s") {
		return who + "' workspace"
	}
	return who + "'s workspace"
}

// NewWorkspace inserts a workspace whose slug is slugHint made unique
// (-2, -3, … when taken); the slug is the id, as every newer install.
func (s *Server) NewWorkspace(ctx context.Context, name, slugHint string) (string, error) {
	db, ok := s.repoFactory.DB().(*sql.DB)
	if !ok || db == nil {
		return "", fmt.Errorf("database not available")
	}
	return newWorkspaceRow(ctx, db, name, slugHint)
}

func newWorkspaceRow(ctx context.Context, db *sql.DB, name, slugHint string) (string, error) {
	base := slugifyWorkspace(slugHint)
	if base == "untitled" || base == "" {
		base = slugifyWorkspace(name)
	}
	if len(base) > 40 {
		base = strings.Trim(base[:40], "-")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		name = base
	}
	slug := base
	for i := 2; i < 1000; i++ {
		var n int
		if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM workspaces WHERE slug = ? OR id = ?`, slug, slug).Scan(&n); err != nil {
			return "", fmt.Errorf("workspaces: %w", err)
		}
		if n == 0 {
			break
		}
		slug = fmt.Sprintf("%s-%d", base, i)
	}
	_, err := db.ExecContext(ctx,
		`INSERT INTO workspaces (id, name, slug, owner_id, language, plan, status, created_at, updated_at)
		 VALUES (?, ?, ?, '', 'en', 'free', 'active', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
		slug, name, slug)
	if err != nil {
		return "", fmt.Errorf("create workspace %q: %w", slug, err)
	}
	return slug, nil
}

// SeedWorkspace makes ownerUserID the admin of slug and gives the workspace
// what the setup gives a new one: #general, the chief and the coding team.
func (s *Server) SeedWorkspace(ctx context.Context, slug, name, ownerUserID string) {
	db, ok := s.repoFactory.DB().(*sql.DB)
	if !ok || db == nil {
		return
	}
	if _, err := db.ExecContext(ctx, `UPDATE workspaces SET owner_id = ? WHERE id = ?`, ownerUserID, slug); err != nil {
		s.log.Warn("workspace owner not recorded", slog.String("workspace", slug), slog.String("error", err.Error()))
	}
	if _, err := db.ExecContext(ctx, `UPDATE users SET role = 'admin' WHERE id = ?`, ownerUserID); err != nil {
		s.log.Warn("workspace owner not made admin", slog.String("workspace", slug), slog.String("error", err.Error()))
	}
	s.seedDefaultChannel(ctx, db, slug, ownerUserID)
	s.seedChiefAgent(ctx, db, slug, ownerUserID, name)
	s.seedCoderAgent(ctx, db, slug)
	s.seedNarratorAgent(ctx, db, slug)
	s.seedPlannerAgent(ctx, db, slug)
	s.seedVerifierAgent(ctx, db, slug)
	s.seedRunnerAgent(ctx, db, slug)
	s.log.Info("workspace made for a sign-up", slog.String("workspace", slug), slog.String("owner", ownerUserID))
}

// workspaceForSignUp is the workspace a sign-up with no invite lands in:
// its own, made now. Without a maker (a test, a gateway with no database)
// the service's default stands.
func (a *AuthGatewayAdapter) workspaceForSignUp(ctx context.Context, name, slugHint string) (string, error) {
	if a.workspaces == nil {
		return "", nil
	}
	return a.workspaces.NewWorkspace(ctx, name, slugHint)
}

func (a *AuthGatewayAdapter) seedIfOwn(ctx context.Context, slug, name, ownerUserID string) {
	if a.workspaces == nil || slug == "" {
		return
	}
	a.workspaces.SeedWorkspace(ctx, slug, name, ownerUserID)
}
