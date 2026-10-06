package billingsvc

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"net/mail"
	"strings"
	"time"

	"memdoor/gateway/email"
	"memdoor/gateway/logs"
	"memdoor/gateway/ratelimit"
	"memdoor/pkg/plan"
)

// Sign-in by email code — the creator's front door.
//
// The device flow above (`memdoor login`) is for a developer with a
// terminal and a workspace name. A coach has neither: she has the email she
// paid with. So the Mac app asks for that email, this service mails a
// six-digit code, she types it, and the app holds a token from then on.
//
//	POST /v1/auth/email     {email}        → a code is mailed (always "sent")
//	POST /v1/auth/code      {email, code}  → {token, workspace, plan}
//	GET  /v1/me             bearer         → who am I, what do I have
//	POST /v1/checkout/seat  {email, period} → Stripe checkout URL, no account needed first
//
// An email IS an account: the first sign-in creates a free workspace named
// after it, and paying (before or after) upgrades that same workspace. The
// order the page sells — pay, download, sign in, use — and the order a
// curious download takes — sign in, try on your own key, pay — both land
// on one row.

const (
	emailCodeTTL      = 10 * time.Minute
	emailCodeAttempts = 5
	emailCodeDigits   = 6
	workspaceSlugMax  = 24
)

// authEmailLimiter: five code requests per address per hour. A person asks
// once, maybe twice; a script asks for hundreds.
var authEmailLimiter = ratelimit.NewPerClientLimiter(5, time.Hour)

// checkoutLimiter: the same for checkout links, which cost a Stripe call.
var checkoutLimiter = ratelimit.NewPerClientLimiter(5, time.Hour)

// emailCode is one outstanding sign-in code, keyed by email in the store.
type emailCode struct {
	Code      string    `json:"code"`
	CreatedAt time.Time `json:"created_at"`
	Attempts  int       `json:"attempts"`
}

// mailer is what the service needs from email: one call. The interface
// lives here, in the consumer, so tests can stand a fake in.
type mailer interface {
	Send(email.Email) error
}

func normalizeEmail(s string) (string, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return "", fmt.Errorf("email is required")
	}
	addr, err := mail.ParseAddress(s)
	if err != nil || addr.Address != s || len(s) > 254 {
		return "", fmt.Errorf("that does not look like an email address")
	}
	return s, nil
}

// clientAddr is the address the rate limiters key on: X-Real-IP, which nginx
// sets to the peer's address on every proxied location, else the peer itself.
// Never X-Forwarded-For: nginx appends to it, so its first value is whatever
// the client wrote, and a limiter keyed on it is no limiter.
func clientAddr(r *http.Request) string {
	if real := strings.TrimSpace(r.Header.Get("X-Real-IP")); real != "" {
		return real
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// sixDigits is a code a person can read off a phone and type: leading
// zeros kept, so it is always six characters.
func sixDigits() string {
	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		return "000000"
	}
	return fmt.Sprintf("%0*d", emailCodeDigits, n.Int64())
}

// workspaceSlugFor names the workspace an email gets: the local part, made
// safe, kept short. "sara.pop@gmail.com" → "sara-pop". The caller adds a
// suffix when that name is already someone else's.
func workspaceSlugFor(addr string) string {
	local := addr
	if i := strings.Index(addr, "@"); i >= 0 {
		local = addr[:i]
	}
	var b strings.Builder
	lastDash := true
	for _, r := range strings.ToLower(local) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastDash = false
		default:
			if !lastDash {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}
	s := strings.Trim(b.String(), "-")
	if len(s) > workspaceSlugMax {
		s = strings.Trim(s[:workspaceSlugMax], "-")
	}
	if s == "" {
		s = "creator"
	}
	return s
}

// accountByEmailLocked finds the workspace an email owns. Accounts are keyed
// by workspace and there are hundreds, not millions: a scan is the index.
func (a *authStore) accountByEmailLocked(addr string) (string, *account) {
	for ws, acct := range a.Accounts {
		if acct.Email == addr {
			return ws, acct
		}
	}
	return "", nil
}

// ensureAccountLocked returns the workspace for an email, creating a free
// one on first sight. The name is the email's local part; a clash with
// someone else's gets a short random suffix.
func (a *authStore) ensureAccountLocked(addr string) (string, *account, error) {
	if ws, acct := a.accountByEmailLocked(addr); acct != nil {
		return ws, acct, nil
	}
	base := workspaceSlugFor(addr)
	ws := base
	for tries := 0; tries < 20; tries++ {
		if existing := a.Accounts[ws]; existing == nil {
			break
		}
		ws = base + "-" + randCode(2)
	}
	acct, err := newAccount(ws, addr)
	if err != nil {
		return "", nil, err
	}
	a.Accounts[ws] = acct
	return ws, acct, nil
}

// handleAuthEmail — POST /v1/auth/email {email}. Mails a code. The answer
// is the same whether or not the address is known: an attacker learns
// nothing, and a first-time coach is not told to "register" first.
func (s *Server) handleAuthEmail(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, fmt.Errorf("POST only"))
		return
	}
	if !authEmailLimiter.Allow(clientAddr(r)) {
		writeErr(w, http.StatusTooManyRequests, fmt.Errorf("too many codes requested, try again later"))
		return
	}
	var req struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10)).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("invalid body"))
		return
	}
	addr, err := normalizeEmail(req.Email)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	// Per email as well as per address: five fresh codes an hour for one
	// inbox, whatever the addresses asking, so a code cannot be guessed by
	// requesting new ones.
	if !authEmailLimiter.Allow("email:" + addr) {
		writeErr(w, http.StatusTooManyRequests, fmt.Errorf("too many codes requested for this email, try again later"))
		return
	}
	if s.mail == nil {
		writeErr(w, http.StatusServiceUnavailable, fmt.Errorf("sign-in by email is not configured on this service"))
		return
	}
	code := sixDigits()
	s.auth.mu.Lock()
	s.auth.Codes[addr] = &emailCode{Code: code, CreatedAt: time.Now()}
	s.auth.flushLocked()
	s.auth.mu.Unlock()

	body := fmt.Sprintf(`Your Memdoor sign-in code is

    %s

Type it in the terminal: memdoor login you@example.com. It works for %d minutes and once.

If you did not ask for it, ignore this email — nothing happens without the code.

— Memdoor
`, code, int(emailCodeTTL.Minutes()))
	if err := s.mail.Send(email.Email{To: addr, Subject: "Your Memdoor code: " + code, Body: body}); err != nil {
		logs.New("Auth").WithError(err).Warn("sign-in code not sent")
		writeErr(w, http.StatusBadGateway, fmt.Errorf("the code could not be emailed right now, try again in a minute"))
		return
	}
	logs.New("Auth").Info("sign-in code sent", "email", addr)
	writeJSON(w, map[string]any{"sent": true, "ttl_s": int(emailCodeTTL.Seconds())})
}

// handleAuthCode — POST /v1/auth/code {email, code}. The code proves the
// inbox; the inbox is the account. Returns the token the app keeps.
func (s *Server) handleAuthCode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, fmt.Errorf("POST only"))
		return
	}
	var req struct {
		Email string `json:"email"`
		Code  string `json:"code"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10)).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("invalid body"))
		return
	}
	addr, err := normalizeEmail(req.Email)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	code := strings.TrimSpace(req.Code)
	s.auth.mu.Lock()
	defer s.auth.mu.Unlock()
	ec := s.auth.Codes[addr]
	switch {
	case ec == nil:
		writeErr(w, http.StatusUnauthorized, fmt.Errorf("no code was requested for this email — ask for a new one"))
		return
	case time.Since(ec.CreatedAt) > emailCodeTTL:
		delete(s.auth.Codes, addr)
		s.auth.flushLocked()
		writeErr(w, http.StatusGone, fmt.Errorf("that code has expired — ask for a new one"))
		return
	case ec.Code != code:
		// SAY WHAT IS LEFT (onboarding battle test, 2026-09-27). "that code is
		// not right" told a person nothing: not that the code is six digits
		// from the newest email, not that it dies after ten minutes, not that
		// the fifth wrong try throws it away and needs a new one. Someone who
		// pasted the example from the docs learned only that they were wrong.
		ec.Attempts++
		left := emailCodeAttempts - ec.Attempts
		if ec.Attempts >= emailCodeAttempts {
			delete(s.auth.Codes, addr)
			s.auth.flushLocked()
			writeErr(w, http.StatusUnauthorized, fmt.Errorf("that code is not right, and that was the last try — ask for a new one"))
			return
		}
		s.auth.flushLocked()
		writeErr(w, http.StatusUnauthorized, fmt.Errorf("that code is not right — six digits from the newest email, good for %d minutes (%d %s left)",
			int(emailCodeTTL.Minutes()), left, map[bool]string{true: "try", false: "tries"}[left == 1]))
		return
	}
	delete(s.auth.Codes, addr)
	ws, acct, err := s.auth.ensureAccountLocked(addr)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	token := "mdt_" + randCode(24)
	s.auth.Tokens[token] = ws
	s.auth.flushLocked()
	logs.New("Auth").Info("signed in by email code", "email", addr, "workspace", ws)
	writeJSON(w, s.meLocked(ws, acct, token))
}

// meLocked is the account as the app sees it. The token is included only
// when one was just minted.
func (s *Server) meLocked(ws string, acct *account, token string) map[string]any {
	p := s.auth.planForLocked(ws)
	out := map[string]any{
		"email":     acct.Email,
		"workspace": ws,
		"plan":      string(p),
		"billing":   acct.Billing,
	}
	if token != "" {
		out["token"] = token
	}
	return out
}

// handleMe — GET /v1/me. What the bearer token is: the email, the
// workspace, the plan. The app shows this and decides whether to offer a
// seat.
func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeErr(w, http.StatusMethodNotAllowed, fmt.Errorf("GET only"))
		return
	}
	ws, err := s.authWorkspace(r)
	if err != nil {
		writeErr(w, http.StatusUnauthorized, err)
		return
	}
	s.auth.mu.Lock()
	defer s.auth.mu.Unlock()
	acct := s.auth.Accounts[ws]
	if acct == nil {
		// An operator token names a workspace nobody signed in to.
		acct = &account{Workspace: ws}
	}
	writeJSON(w, s.meLocked(ws, acct, ""))
}

// handleCheckoutSeat — POST /v1/checkout/seat {email, period}. The landing
// page's button: no sign-in first, because the page is where a coach
// decides and the app is where she signs in afterwards. The email creates
// (or finds) the account the subscription will land on, so the webhook
// that already promotes a workspace needs no new path.
func (s *Server) handleCheckoutSeat(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, fmt.Errorf("POST only"))
		return
	}
	if !checkoutLimiter.Allow(clientAddr(r)) {
		writeErr(w, http.StatusTooManyRequests, fmt.Errorf("too many attempts, try again later"))
		return
	}
	var req struct {
		Email  string `json:"email"`
		Period string `json:"period"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4<<10)).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("invalid body"))
		return
	}
	addr, err := normalizeEmail(req.Email)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err)
		return
	}
	if req.Period == "" {
		req.Period = "monthly"
	}
	s.auth.mu.Lock()
	ws, acct, err := s.auth.ensureAccountLocked(addr)
	if err == nil {
		s.auth.flushLocked()
	}
	already := acct != nil && plan.Plan(acct.Plan) == plan.Pro
	s.auth.mu.Unlock()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err)
		return
	}
	if already {
		writeErr(w, http.StatusConflict, fmt.Errorf("this email already has a seat — sign in with it: memdoor login <your email>"))
		return
	}
	link, id, err := CreateSubscriptionCheckout(ws, req.Period, addr)
	if err != nil {
		logs.New("Billing").WithError(err).Warn("seat checkout not created")
		writeErr(w, http.StatusBadGateway, fmt.Errorf("checkout is not available right now — email hello@memdoor.ai and we will set you up"))
		return
	}
	logs.New("Billing").Info("seat checkout " + id + " for " + ws + " (" + req.Period + ")")
	writeJSON(w, map[string]string{"url": link, "session_id": id, "workspace": ws})
}

// welcomeAfterSubscription is the "pay → download" bridge for someone who
// closed the tab: the seat is active, here is the app, sign in with this
// email. Best effort; the subscription row is the record.
func (s *Server) welcomeAfterSubscription(ws string) {
	if s.mail == nil {
		return
	}
	s.auth.mu.Lock()
	acct := s.auth.Accounts[ws]
	s.auth.mu.Unlock()
	if acct == nil || acct.Email == "" {
		return
	}
	// WHAT THE FIRST SUBSCRIBER READS. This said $149 and "drop a video" until
	// 2026-09-27: the price and the product had both moved and the one email a
	// paying person receives had not. It is the command line now, and the seat is
	// remote control at $10 (2026-10-04: workflows and the decision model are
	// free; Pro is remote control and the hosted scheduler when it comes).
	body := fmt.Sprintf(`Your Memdoor subscription is active. Remote control is on.

1. Install it, if you have not:  curl -fsSL https://memdoor.ai/install.sh | bash
2. Sign in:  memdoor login %s     (a code arrives here; type it)
3. Your own key pays for inference, any provider's:
   export OPEN_ROUTER_API_KEY=sk-or-…   (or ANTHROPIC_API_KEY, DEEPSEEK_API_KEY, …; or memdoor connect)
   The decision model runs on an OpenRouter key or a decision key of your own.
4. Start working:  cd your-project && memdoor tui

From now on /remote in the TUI opens your session on your phone, sealed end
to end, and every workflow run's state is kept on memdoor.ai: a workflow can
wait on what another produced, from any machine, and memdoor workflow history
answers from anywhere. Runs with the laptop closed are coming.

$10 a month, cancel any month. Reply to this email if anything is unclear — a
person answers.

— Greg, memdoor.ai
`, acct.Email)
	go func() {
		if err := s.mail.Send(email.Email{To: acct.Email, Subject: "Your Memdoor subscription is active — remote control is on", Body: body}); err != nil {
			logs.New("Billing").WithError(err).Warn("welcome email not sent", "to", acct.Email)
		}
	}()
}
