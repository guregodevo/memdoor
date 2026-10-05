package cmd

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"memdoor/pkg/shared"
	_ "memdoor/pkg/sqlitedriver"

	"memdoor/pkg/auth"
)

// The engine's own user, adopted by the signed-in account (account.go). Moved
// here from the removed 'memdoor admin' command (2026-10-03), which shared it.

// adoptLocalUser makes the engine's user the signed-in account's: the user
// at engineEmail is moved to accountEmail (unless the account already has
// a user here) and given password. A var for the same reason.
var adoptLocalUser = func(ctx context.Context, dbPath, engineEmail, accountEmail, password string) error {
	db, err := openLocalStore(dbPath)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()
	repo := auth.NewSQLiteRepository(db)
	svc := auth.NewService(repo, nil, auth.NewTokenGenerator(), slog.Default(), "", "")
	return adoptUser(ctx, repo, repo, svc, engineEmail, accountEmail, password)
}

// openLocalStore opens the engine's store the way the gateway does, so the
// two coexist instead of locking each other out. dbPath empty means the
// default ~/.memdoor/data/memdoor.db.
func openLocalStore(dbPath string) (*sql.DB, error) {
	if dbPath == "" {
		dbPath = shared.MemdoorHome("data", "memdoor.db")
	}
	if _, err := os.Stat(dbPath); err != nil {
		return nil, fmt.Errorf("local database not found at %s: %w", dbPath, err)
	}
	dsn := fmt.Sprintf("file:%s?_journal_mode=WAL&_busy_timeout=5000&_foreign_keys=on", dbPath)
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", dbPath, err)
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping %s: %w", dbPath, err)
	}
	return db, nil
}

// emailSetter is the slice of the auth repository adoption needs: move a
// user to another address (the sqlite repository satisfies it).
type emailSetter interface {
	SetUserEmail(ctx context.Context, userID, email string) error
}

// adoptUser resolves the engine's user and makes it the account's.
//
// "It should be with the user logged in" (Greg, 2026-09-18): the engine
// user on this Mac IS whoever signed in — the user keeps its id, so every
// workspace, session and page it owns comes along; only the address and
// the password change. When the account already has a user here, that one
// is taken and nothing is renamed.
func adoptUser(ctx context.Context, repo userByEmailLookup, emails emailSetter, svc passwordSetter, engineEmail, accountEmail, password string) error {
	accountEmail = strings.TrimSpace(accountEmail)
	if accountEmail == "" {
		return fmt.Errorf("no account to adopt the engine's user for")
	}
	user, _, err := repo.GetUserByEmail(ctx, accountEmail)
	if err != nil {
		if strings.TrimSpace(engineEmail) == "" {
			return fmt.Errorf("no user with email %q in the local database: %w", accountEmail, err)
		}
		if user, _, err = repo.GetUserByEmail(ctx, engineEmail); err != nil {
			return fmt.Errorf("no user with email %q in the local database: %w", engineEmail, err)
		}
		if !strings.EqualFold(user.Email, accountEmail) {
			if err := emails.SetUserEmail(ctx, user.ID, accountEmail); err != nil {
				return fmt.Errorf("move %s to %s: %w", engineEmail, accountEmail, err)
			}
		}
	}
	if err := svc.AdminSetPassword(ctx, user.ID, password); err != nil {
		return fmt.Errorf("set password for %s: %w", accountEmail, err)
	}
	return nil
}

// userByEmailLookup is the slice of the auth repository this command needs:
// resolve an email to a user (auth.Repository satisfies it).
type userByEmailLookup interface {
	GetUserByEmail(ctx context.Context, email string) (*auth.User, string, error)
}

// passwordSetter is the slice of the auth service this command needs:
// set a user's password without the old one (auth.Service satisfies it).
type passwordSetter interface {
	AdminSetPassword(ctx context.Context, userID, newPassword string) error
}
