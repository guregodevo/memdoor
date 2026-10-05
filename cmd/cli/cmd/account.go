package cmd

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"memdoor/pkg/shared"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// memdoor account — the creator's sign-in.
//
// A coach has one thing: the email she paid with. `memdoor account login`
// asks memdoor.ai to mail a code to it, takes the code, and from then on
// this Mac holds her account token. Behind that one step
// the gateway is prepared without a question asked: the directories,
// the first user, the workspace named like her account. The Mac app runs
// exactly these commands behind its sign-in screen (ADR-0009: the app
// decides nothing), and a terminal user runs them by hand.
//
//	memdoor account login sara@example.com      # mails a code, asks for it
//	memdoor account status                      # who is signed in, what plan
//	memdoor account subscribe                   # Pro's checkout (remote control), in the browser
//	memdoor account logout

const (
	accountRecordFile = "account.json"  // ~/.memdoor/account.json: {email, workspace}
	localLoginFile    = "local-login"   // ~/.memdoor/local-login: the engine's own user, generated
	billingTokenFile  = "billing-token" // ~/.memdoor/billing-token: what leases the brain
	localPasswordLen  = 24
)

// accountRecord is what this Mac remembers about the sign-in, so status
// works offline and the app can greet by email.
type accountRecord struct {
	Email     string `json:"email"`
	Workspace string `json:"workspace"`
}

// localLogin is the engine's own user, created for the account on first
// sign-in with a password nobody types. Kept so a lost session can be
// renewed without a person involved.
type localLogin struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// meAnswer is /v1/me and /v1/auth/code: the account as the service sees it.
type meAnswer struct {
	Token     string `json:"token,omitempty"`
	Email     string `json:"email"`
	Workspace string `json:"workspace"`
	Plan      string `json:"plan"`
	Billing   string `json:"billing"`
}

// accountStatus is what `account status --json` prints: the account plus
// whether this Mac is ready to use it.
type accountStatus struct {
	SignedIn bool `json:"signed_in"`
	// Engine is the user this Mac's own gateway knows you as — the other
	// identity. It stays when you sign out of memdoor.ai: the agent on your
	// key needs it, the seat does not.
	Engine     string `json:"engine,omitempty"`
	Email      string `json:"email,omitempty"`
	Workspace  string `json:"workspace,omitempty"`
	Plan       string `json:"plan,omitempty"`
	Billing    string `json:"billing,omitempty"`
	LocalReady bool   `json:"local_ready"`
	Note       string `json:"note,omitempty"`
	// Brain is what the engine says about the brain, once this Mac is
	// ready: the window opens the terminal only on a brain that answers,
	// and shows the seat or the wait otherwise.
	Brain *brainRead `json:"brain,omitempty"`
}

var (
	accountJSON     bool
	accountCode     string
	accountSendOnly bool
	accountYearly   bool
)

var accountCmd = &cobra.Command{
	Use:   "account",
	Short: "Sign in with your email (a code arrives, you type it)",
}

var accountLoginCmd = &cobra.Command{
	Use:   "login [email]",
	Short: "Sign in: a code is emailed, type it here (short: memdoor login)",
	Example: `  memdoor account login sara@example.com
  memdoor account login sara@example.com --send-only        # the app: step one
  memdoor account login sara@example.com --code 482913 --json  # the app: step two`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		email := ""
		if len(args) == 1 {
			email = strings.TrimSpace(args[0])
		}
		if email == "" {
			if !term.IsTerminal(int(os.Stdin.Fd())) {
				return fmt.Errorf("an email is required: memdoor login you@example.com")
			}
			email = prompt("Email: ")
		}
		if accountCode == "" {
			var sent struct {
				Sent bool `json:"sent"`
			}
			if err := brokerCallNoAuth(http.MethodPost, "/v1/auth/email", map[string]string{"email": email}, &sent); err != nil {
				return err
			}
			if accountSendOnly {
				if accountJSON {
					return printJSON(map[string]any{"sent": true, "email": strings.ToLower(email)})
				}
				fmt.Printf("A code is on its way to %s.\n", email)
				return nil
			}
			if !term.IsTerminal(int(os.Stdin.Fd())) {
				return fmt.Errorf("a code was emailed to %s — run again with --code <the six digits>", email)
			}
			fmt.Printf("A code is on its way to %s.\n", email)
			accountCode = prompt("Code: ")
		}
		var me meAnswer
		if err := brokerCallNoAuth(http.MethodPost, "/v1/auth/code",
			map[string]string{"email": email, "code": strings.TrimSpace(accountCode)}, &me); err != nil {
			return err
		}
		if me.Token == "" {
			return fmt.Errorf("signed in, but no token came back")
		}
		if err := writeHomeFile(billingTokenFile, []byte(me.Token), 0o600); err != nil {
			return err
		}
		rec, _ := json.Marshal(accountRecord{Email: me.Email, Workspace: me.Workspace})
		if err := writeHomeFile(accountRecordFile, rec, 0o600); err != nil {
			return err
		}
		ready, note := ensureLocalEngine(me.Email, me.Workspace)
		st := accountStatus{SignedIn: true, Email: me.Email, Workspace: me.Workspace, Plan: me.Plan, Billing: me.Billing, LocalReady: ready, Note: note}
		if ready {
			if r := readBrain(NewClient()); r.State != "" {
				st.Brain = &r
			}
		}
		if accountJSON {
			return printJSON(st)
		}
		fmt.Printf("✓ Signed in as %s (workspace %s, %s).\n", st.Email, st.Workspace, describePlan(st.Plan, st.Billing))
		if !ready {
			fmt.Printf("⚠ %s\n", note)
		}
		fmt.Println("  Next: cd <your-project> && memdoor tui")
		return nil
	},
}

var accountStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Who is signed in on this Mac, and what the seat is",
	RunE: func(cmd *cobra.Command, args []string) error {
		st := currentAccountStatus()
		if accountJSON {
			return printJSON(st)
		}
		fmt.Println(engineLine(st.Engine))
		if !st.SignedIn {
			fmt.Println("memdoor.ai    not signed in (workflows run free here; Pro is remote control and the hosted scheduler) — memdoor login you@example.com")
			return nil
		}
		fmt.Printf("memdoor.ai    %s · workspace %s · %s\n", st.Email, st.Workspace, describePlan(st.Plan, st.Billing))
		if !st.LocalReady {
			fmt.Printf("⚠ %s\n", st.Note)
		}
		return nil
	},
}

var accountLogoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Sign out of memdoor.ai on this Mac (the engine stays signed in)",
	RunE:  func(cmd *cobra.Command, args []string) error { return signOut() },
}

// signOut forgets the memdoor.ai sign-in on this Mac: the seat's token and
// the account record. The engine's own session stays — a free user's agent
// runs on it, and nothing would renew it without a memdoor.ai account
// (2026-10-03: two logouts, each forgetting a different half).
func signOut() error {
	for _, f := range []string{billingTokenFile, accountRecordFile} {
		_ = os.Remove(homePath(f))
	}
	fmt.Println("Signed out of memdoor.ai: workflows are off on this Mac.")
	if creds, err := loadCredentials(); err == nil {
		fmt.Printf("The engine stays signed in as %s, so the agent keeps working on your key.\n", creds.Email)
	}
	return nil
}

// engineLine is the status line for this Mac's own gateway user.
func engineLine(email string) string {
	if email == "" {
		return "This Mac      no engine session — memdoor setup, or memdoor login"
	}
	return "This Mac      engine user " + email
}

var accountSubscribeCmd = &cobra.Command{
	Use: "subscribe",
	// PRO IS THE HOSTED SCHEDULER (Greg, 2026-10-04: "everyone can run
	// schedule with local cron"). Workflows, local schedules, inference and
	// the decision model run free on the person's own machine and key; a seat
	// buys remote control and the hosted workflow state (laptop-closed runs coming), never a model key
	// (Greg, 2026-10-04). The number is safe to say here from
	// 2026-09-27: the live price was changed and a real checkout session
	// quotes $10.00 USD (scripts/set-seat-price.sh switches it again if it moves).
	Short: "Subscribe to Pro: remote control and the hosted workflow state, always on your own key; $10 a month",
	RunE: func(cmd *cobra.Command, args []string) error {
		period := "monthly"
		if accountYearly {
			period = "yearly"
		}
		var out struct {
			URL string `json:"url"`
		}
		if err := brokerCall(http.MethodPost, "/v1/subscribe", map[string]string{"period": period}, &out); err != nil {
			return err
		}
		if accountJSON {
			return printJSON(map[string]string{"url": out.URL, "period": period})
		}
		fmt.Println("Opening the checkout in your browser…")
		if err := openInBrowser(out.URL); err != nil {
			fmt.Println(out.URL)
		}
		return nil
	},
}

// currentAccountStatus reads what this Mac remembers, then asks the service
// for the plan. Offline, the remembered email still answers.
func currentAccountStatus() accountStatus {
	st := accountStatus{}
	if creds, err := loadCredentials(); err == nil {
		st.Engine = creds.Email
	}
	token := brokerToken()
	if token == "" {
		st.Note = "not signed in"
		return st
	}
	st.SignedIn = true
	if b, err := os.ReadFile(homePath(accountRecordFile)); err == nil {
		var rec accountRecord
		if json.Unmarshal(b, &rec) == nil {
			st.Email, st.Workspace = rec.Email, rec.Workspace
		}
	}
	var me meAnswer
	if err := brokerCall(http.MethodGet, "/v1/me", nil, &me); err == nil {
		st.Email, st.Workspace, st.Plan, st.Billing = me.Email, me.Workspace, me.Plan, me.Billing
	} else if strings.Contains(err.Error(), "unauthorized") {
		// The token is no longer honoured: say so rather than show stale
		// facts as current ones.
		st.SignedIn = false
		st.Note = "the sign-in has expired — run memdoor login again"
		return st
	}
	ready, signInAgain, note := localSessionReady()
	if !ready {
		st.Note = note
		// Signing in again is the step that sets the engine's own user up
		// (ensureLocalEngine), and the window shows its page for it. When
		// that cannot help — the user was made by hand — the note says what
		// does, and the sign-in stays as it is.
		st.SignedIn = !signInAgain
		return st
	}
	st.LocalReady = gatewayReachable()
	if !st.LocalReady {
		st.Note = "the engine is not running — it starts with the app"
		return st
	}
	if r := readBrain(NewClient()); r.State != "" {
		st.Brain = &r
	}
	return st
}

// localSessionReady reports whether this Mac's session will be honoured,
// renewing it from the engine's own user when it has lapsed.
//
// A session that had run out was reported READY: the file had a token, so
// it counted, and the window opened the terminal on it — which died in 78 ms
// with a message about a channel (2026-09-18). The token carries its expiry;
// a lapsed one is renewed here without asking, from the login this Mac
// generated at first sign-in, and only when that is not there does the
// answer become "sign in again".
func localSessionReady() (ready, signInAgain bool, note string) {
	if sessionProblem() == nil {
		return true, false, ""
	}
	if err := renewLocalSession(); err == nil {
		return true, false, ""
	}
	// Nothing on disk renews it. The sign-in does (ensureLocalEngine adopts
	// the engine's user), so that is where to send the person.
	if _, err := loadCredentials(); err == nil {
		return false, true, "the session on this Mac has run out — sign in again to renew it"
	}
	return false, true, "this Mac has no session with the engine — sign in again to set it up"
}

// ensureLocalEngine makes this Mac able to run the account: directories,
// a running engine, the engine's own first user and a workspace named like
// the account. Nothing is asked; every step is idempotent. What is printed
// by the steps goes to stderr so --json stays clean.
func ensureLocalEngine(email, workspace string) (bool, string) {
	stdout := os.Stdout
	os.Stdout = os.Stderr
	defer func() { os.Stdout = stdout }()

	configPath, err := setupLocalDirs(shared.MemdoorHome("workspace"))
	if err != nil {
		return false, "could not prepare ~/.memdoor: " + err.Error()
	}
	ensureGatewayRunning()
	if !gatewayReachable() {
		return false, "the engine did not start — see ~/.memdoor/gateway.log"
	}
	c := NewClient()
	var status struct {
		Initialized   bool   `json:"initialized"`
		WorkspaceSlug string `json:"workspace_slug"`
	}
	if err := c.GetJSON("/api/setup/status", &status); err != nil {
		return false, "the engine did not answer: " + err.Error()
	}
	if !status.Initialized {
		password := randomPassword()
		slug, err := registerAdminViaSetup(email, password, workspace)
		if err != nil {
			return false, err.Error()
		}
		rec, _ := json.Marshal(localLogin{Email: email, Password: password})
		_ = writeHomeFile(localLoginFile, rec, 0o600)
		persistWorkspaceSlug(configPath, slug)
		return true, ""
	}
	if err := localSessionFor(email); err != nil {
		return false, "this Mac's engine could not be signed in to: " + err.Error()
	}
	persistWorkspaceSlug(configPath, status.WorkspaceSlug)
	return true, ""
}

// localSessionFor leaves this Mac with a session the engine will honour:
// the one it has, if it is live; else a renewal from the generated login;
// else the engine's user is adopted.
//
// The first check was "is there a token", and a token that had run out
// passed it: the sign-in reported the Mac ready, the window opened the
// terminal, and the terminal died on the very session the sign-in was
// meant to renew (2026-09-18, second time round). Live means live.
func localSessionFor(email string) error {
	if sessionProblem() == nil {
		// A live session, for someone else: the engine's user becomes the
		// account's ("it should be with the user logged in").
		if creds, err := loadCredentials(); err == nil && !strings.EqualFold(creds.Email, email) {
			return adoptLocalEngine(email)
		}
		return nil
	}
	if err := renewLocalSession(); err == nil {
		if creds, err := loadCredentials(); err == nil && !strings.EqualFold(creds.Email, email) {
			return adoptLocalEngine(email)
		}
		return nil
	}
	// Nothing on disk renews it: the engine's user was made by hand. The
	// sign-in just proved the email, and this Mac's engine is the app's to
	// run — so the app takes the user over, the way it would have made it.
	return adoptLocalEngine(email)
}

// adoptLocalEngine makes the engine's user the signed-in account's: the
// user this Mac's session belonged to is moved to the account's email and
// given a generated password, the login file first sign-in writes is
// written, and a session is opened on it.
//
// "The user should be redirected to auth page" and "it should be with the
// user logged in" (Greg, 2026-09-18): a note telling a person to type a
// command in a terminal is not the app, and the engine's user is whoever
// signed in — not a name from before the app existed.
func adoptLocalEngine(accountEmail string) error {
	engineEmail := ""
	if creds, err := loadCredentials(); err == nil {
		engineEmail = creds.Email
	}
	password := randomPassword()
	if err := adoptLocalUser(context.Background(), "", engineEmail, accountEmail, password); err != nil {
		return err
	}
	rec, _ := json.Marshal(localLogin{Email: accountEmail, Password: password})
	if err := writeHomeFile(localLoginFile, rec, 0o600); err != nil {
		return err
	}
	return loginLocal(accountEmail, password)
}

// renewLocalSession signs in to the engine again with the user this Mac
// generated at first sign-in (~/.memdoor/local-login). An engine whose user
// was made by hand has no such file, and the error says so.
func renewLocalSession() error {
	var ll localLogin
	b, err := os.ReadFile(homePath(localLoginFile))
	if err != nil {
		return fmt.Errorf("no generated login on this Mac: %w", err)
	}
	if json.Unmarshal(b, &ll) != nil || ll.Password == "" {
		return fmt.Errorf("the generated login on this Mac is unreadable")
	}
	return loginLocal(ll.Email, ll.Password)
}

// loginLocal signs in to this machine's gateway and saves the session, the way
// `memdoor auth login-direct` does.
func loginLocal(email, password string) error {
	c := NewClient()
	var result struct {
		Token string `json:"token"`
		User  struct {
			Email string `json:"email"`
		} `json:"user"`
	}
	if err := c.PostJSON("/api/auth/login", map[string]string{"email": email, "password": password}, &result); err != nil {
		return err
	}
	return saveCredentials(&credentials{Token: result.Token, Email: result.User.Email})
}

func describePlan(p, billing string) string {
	switch p {
	case "pro":
		if billing != "" {
			return "seat, " + billing
		}
		return "seat"
	case "enterprise":
		return "enterprise"
	default:
		return "free"
	}
}

func randomPassword() string {
	b := make([]byte, localPasswordLen)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func homePath(name string) string {
	return shared.MemdoorHome(name)
}

func writeHomeFile(name string, data []byte, mode os.FileMode) error {
	p := homePath(name)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	return os.WriteFile(p, data, mode)
}

func printJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func prompt(label string) string {
	fmt.Print(label)
	line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	return strings.TrimSpace(line)
}

func init() {
	accountLoginCmd.Flags().StringVar(&accountCode, "code", "", "The six-digit code from the email")
	accountLoginCmd.Flags().BoolVar(&accountSendOnly, "send-only", false, "Only mail the code; verify later with --code")
	accountLoginCmd.Flags().BoolVar(&accountJSON, "json", false, "Print the result as JSON")
	accountStatusCmd.Flags().BoolVar(&accountJSON, "json", false, "Print the status as JSON")
	accountSubscribeCmd.Flags().BoolVar(&accountYearly, "yearly", false, "Yearly billing instead of monthly")
	accountSubscribeCmd.Flags().BoolVar(&accountJSON, "json", false, "Print the checkout URL as JSON instead of opening it")
	accountCmd.AddCommand(accountLoginCmd, accountStatusCmd, accountLogoutCmd, accountSubscribeCmd)
	rootCmd.AddCommand(accountCmd)
}
