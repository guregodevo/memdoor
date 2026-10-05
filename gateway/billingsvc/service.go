package billingsvc

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"memdoor/gateway/email"
	"memdoor/gateway/logs"
	"memdoor/gateway/telemetry"
	"memdoor/pkg/repository/sqlite"
)

// listenAddr is where the broker listens: the loopback, always. nginx is
// the only thing that faces the internet; the three nodes and the VPS
// gateway reach it at 127.0.0.1. Measured on 2026-09-19: ":18790" answered
// the world directly (a 200 from a laptop), token-protected but exposed.
func listenAddr(port int) string {
	return fmt.Sprintf("127.0.0.1:%d", port)
}

// Package billingsvc is the HTTP mount for the money domain — the thin
// transport half of `memdoor billing`, which runs ONLY on memdoor.ai.
// A self-hosted gateway never starts this process, so it holds no Stripe
// secrets.

// Server is the billing service.
type Server struct {
	port int
	auth *authStore
	// mail sends sign-in codes and the welcome after a seat is paid. nil
	// when the service has no email provider, in which case sign-in by
	// email says so instead of pretending.
	mail mailer
	// mailProvider is what /v1/health reports, so "can this service send a
	// sign-in code" is answerable from outside without reading its env.
	mailProvider string
}

// New opens the billing database and wires the domain service: one
// process, restarted by systemd, the only writer of billing-auth.json.
//
// The logger is initialized here: this process starts independently of the
// gateway, and its first log line must not panic on an uninitialized
// global (live: it did).
func New(dbPath string, port int) (*Server, error) {
	logDir := filepath.Join(filepath.Dir(dbPath), "logs")
	if err := logs.InitGlobalLogger(logDir, false); err != nil {
		return nil, fmt.Errorf("billing logger: %w", err)
	}
	factory, err := sqlite.NewSQLiteFactory(dbPath)
	if err != nil {
		return nil, fmt.Errorf("billing db: %w", err)
	}
	_ = factory // opens and migrates billing.db, which the nightly backup copies
	s := &Server{port: port, auth: newAuthStore(authStorePath(dbPath))}
	if os.Getenv("EMAIL_PROVIDER") != "" {
		m, err := email.NewService(slog.Default())
		if err != nil {
			return nil, fmt.Errorf("billing email: %w", err)
		}
		s.mail = m
		s.mailProvider = os.Getenv("EMAIL_PROVIDER")
	}
	return s, nil
}

// Start mounts the v1 API and serves until the context is cancelled.
func (s *Server) Start(ctx context.Context) error {
	log := logs.New("Billing")
	mux := http.NewServeMux()
	// Health reports the VERSION, not just liveness.
	//
	// "ok" answers a question nobody was asking. On 2026-08-29 the broker was
	// healthy and seven commits behind, and the only way to find out was to
	// diff the timestamps on the download binaries — a proxy that happens to
	// work because deploy.sh scp's them, and would stop working the day it
	// does not. A deploy either shipped or it did not, and the service is the
	// only thing that actually knows.
	mux.HandleFunc("/v1/health", func(w http.ResponseWriter, r *http.Request) {
		status := map[string]interface{}{
			"status": "ok", "service": "billing",
			"version": telemetry.GatewayVersion, // "" when built without ldflags
			"email":   s.mailProvider,           // "" = sign-in by email code is off
			// The two facts a failed sign-in or checkout comes down to, readable
			// without a shell on the box: is the mail key there, is the seat's
			// Stripe price there. Booleans only; never the values.
			"email_key":  os.Getenv("RESEND_API_KEY") != "" || os.Getenv("SMTP_PASSWORD") != "",
			"seat_price": os.Getenv("STRIPE_PRO_MONTHLY_PRICE") != "" && os.Getenv("STRIPE_SECRET_KEY") != "",
		}
		// The backup's age, so a stale copy shows wherever health is read
		// (deploy's verify, a curl) instead of only in a journal nobody
		// opens. "" = no backup found where the timer writes them.
		status["last_backup"], status["backup_stale"] = lastBackup(backupRootDefault, time.Now())
		writeJSON(w, status)
	})
	mux.HandleFunc("/v1/webhook/stripe", s.handleStripeWebhook)
	mux.HandleFunc("/v1/subscribe", s.handleSubscribe)
	mux.HandleFunc("/v1/plan", s.handleSetPlan)
	// The creator's sign-in: the email she paid with, a code in her inbox.
	mux.HandleFunc("/v1/auth/email", s.handleAuthEmail)
	mux.HandleFunc("/v1/auth/code", s.handleAuthCode)
	mux.HandleFunc("/v1/me", s.handleMe)
	mux.HandleFunc("/v1/checkout/seat", s.handleCheckoutSeat)

	srv := &http.Server{
		Addr:              listenAddr(s.port),
		Handler:           mux,
		ReadHeaderTimeout: 15 * time.Second,
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()
	log.Info(fmt.Sprintf("billing service listening on :%d", s.port))
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

// authWorkspace resolves the caller's workspace. MVP: a shared service
// token plus an explicit workspace parameter; per-user memdoor tokens
// land with the broker gate.
func (s *Server) authWorkspace(r *http.Request) (string, error) {
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if token == "" {
		return "", fmt.Errorf("unauthorized")
	}
	// A user token issued by `memdoor login` carries its OWN workspace — it can
	// never act on another one. The ?workspace= parameter is therefore not a
	// selector for a user; it is an ASSERTION of the account the caller
	// believes it is acting on, and a mismatch is refused rather than silently
	// redirected.
	//
	// Silent redirection is the dangerous shape: the CLI resolves a workspace
	// from the directory you are standing in, so someone whose token belongs to
	// one account and whose project pins another would spend a balance they
	// were not looking at, and the receipt would name a workspace they never
	// chose.
	if ws := s.auth.WorkspaceFor(token); ws != "" {
		if named := r.URL.Query().Get("workspace"); named != "" && named != ws {
			return "", fmt.Errorf(
				"this sign-in belongs to workspace %q, but the request names %q — run `memdoor login` from that workspace, or switch with `memdoor workspace use`",
				ws, named)
		}
		return ws, nil
	}
	// The operator/service token may act on any workspace it names.
	if op := os.Getenv("MEMDOOR_BILLING_TOKEN"); op != "" && token == op {
		ws := r.URL.Query().Get("workspace")
		if ws == "" {
			return "", fmt.Errorf("workspace required")
		}
		return ws, nil
	}
	return "", fmt.Errorf("unauthorized")
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
}
