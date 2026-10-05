package gateway

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	marioapi "github.com/guregodevo/mario/api"
	mario "github.com/guregodevo/mario/workflow"

	"memdoor/gateway/logs"
	"memdoor/pkg/plan"
	"memdoor/pkg/shared"
)

// THE HOSTED STATE (Pro, 2026-10-05). A Pro account keeps EVERY run's state
// in mario-state on memdoor.ai — one table per workspace, written by every
// project and every person on it — instead of the project's .memdoor/runs.db.
// The workspace is the unit of sharing (Greg: "a project id is a workspace",
// "a user has its workspace"): an `external: true` task of one workflow is a
// task another workflow of the workspace produced, by name and partition, as
// in mario, from whichever machine ran it. Where the state lives is decided
// when the run starts and cannot be changed after ("we can't share it later
// after we run it", "it's structural"), so a Pro run whose store is
// unreachable does not start, and says so; it never quietly becomes a local
// one. The tasks still run on this gateway, on this key: only the state
// travels. Free keeps the local table, complete for anything self-contained.

const (
	hostedStateDefault  = "https://memdoor.ai/state"
	hostedPlanTTL       = 10 * time.Minute
	hostedStateTimeout  = 5 * time.Second
	accountTokenFile    = "billing-token" // ~/.memdoor/billing-token, written by memdoor login / account
	planFile            = "plan"          // prefix of ~/.memdoor/plan-<hash of the account token>: the plan billing last named for that account, for when billing does not answer
	hostedStateUnusable = "the hosted workflow state at %s is unreachable (%v) — a Pro run keeps its state there and does not start without it"
	planUnknown         = "the account's plan is not known: billing at %s did not answer (%v) and it has not answered for this account on this machine before — a run cannot start until it does once (its state would live in the wrong place otherwise)"
)

// hostedStateURL is mario-state's address: memdoor.ai, or a trusted override
// (loopback or memdoor.ai itself, as the billing URL).
func hostedStateURL() string {
	if v := os.Getenv("MEMDOOR_STATE_URL"); v != "" && shared.AccountHostTrusted(v) {
		return strings.TrimRight(v, "/")
	}
	return hostedStateDefault
}

// accountToken is the memdoor.ai account token on this machine, "" when
// nobody signed in: a gateway with none runs every workflow locally.
func accountToken() string {
	b, err := os.ReadFile(shared.MemdoorHome(accountTokenFile))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

// billingAccount asks the billing service whose a token is and on which plan
// (GET /v1/me). The relay and the hosted state ask the same question.
func billingAccount(client *http.Client, base, token string) (workspace string, p plan.Plan, err error) {
	req, err := http.NewRequest(http.MethodGet, strings.TrimRight(base, "/")+"/v1/me", nil)
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := client.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("billing unreachable: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("billing refused the token: %s", resp.Status)
	}
	var me struct {
		Workspace string `json:"workspace"`
		Plan      string `json:"plan"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&me); err != nil {
		return "", "", fmt.Errorf("billing answer: %w", err)
	}
	if me.Workspace == "" {
		return "", "", errors.New("billing named no account for the token")
	}
	p, err = plan.Parse(me.Plan)
	if err != nil {
		return me.Workspace, plan.Free, nil
	}
	return me.Workspace, p, nil
}

// hostedState decides, per run start, whether this gateway's runs live on
// memdoor.ai: the account token, the plan it holds (asked of billing, kept
// for a while), and whether mario-state answers.
type hostedState struct {
	billing, state string
	token          func() string
	client         *http.Client
	now            func() time.Time
	// planPath keeps the plan billing last named across restarts, so a
	// billing outage changes nothing for an account billing has answered
	// for before: Pro stays strict, free stays local.
	planPath string

	mu        sync.Mutex
	plan      plan.Plan
	checked   time.Time
	repo      mario.WorkflowRepository
	repoToken string // the token repo was built with: a new token is a new repository
}

func newHostedState(billing, state string, token func() string) *hostedState {
	return &hostedState{billing: billing, state: strings.TrimRight(state, "/"), token: token,
		client: &http.Client{Timeout: hostedStateTimeout}, now: time.Now, planPath: shared.MemdoorHome(planFile)}
}

// planPathFor is the plan file for one account token: keyed to the token, so
// an account that changed never reads the plan of the one before it (review
// finding, 2026-10-05).
func (h *hostedState) planPathFor(token string) string {
	if h.planPath == "" || token == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(token))
	return h.planPath + "-" + hex.EncodeToString(sum[:6])
}

// rememberPlan keeps the plan on disk, best effort.
func (h *hostedState) rememberPlan(token string, p plan.Plan) {
	if path := h.planPathFor(token); path != "" {
		_ = os.WriteFile(path, []byte(string(p)+"\n"), 0o600)
	}
}

// knownPlan is the plan billing last named for this account on this machine,
// "" when none.
func (h *hostedState) knownPlan(token string) plan.Plan {
	if path := h.planPathFor(token); path != "" {
		b, err := os.ReadFile(path)
		if err != nil {
			return ""
		}
		p, err := plan.Parse(strings.TrimSpace(string(b)))
		if err != nil {
			return ""
		}
		return p
	}
	return ""
}

// repository is the hosted repository for a run starting now: nil (and no
// error) for a gateway with no account or a free one, which run locally; the
// REST repository for a Pro one; an error for a Pro one whose store does not
// answer, which is the run not starting.
func (h *hostedState) repository() (mario.WorkflowRepository, error) {
	if h == nil {
		return nil, nil
	}
	token := h.token()
	if token == "" {
		return nil, nil
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	log := logs.New("Workflow")
	if token != h.repoToken {
		// Another account signed in on this machine: nothing known about
		// the last one carries over (review finding, 2026-10-05).
		h.plan, h.repo, h.repoToken = "", nil, token
	}
	if h.plan == "" || h.now().Sub(h.checked) > hostedPlanTTL {
		_, p, err := billingAccount(h.client, h.billing, token)
		switch {
		case err == nil:
			h.plan, h.checked = p, h.now()
			h.rememberPlan(token, p)
		case h.plan != "":
			log.Warn("billing did not answer; the plan last known stands", slog.String("plan", string(h.plan)), slog.String("error", err.Error()))
		case h.knownPlan(token) != "":
			// THE PLAN LAST NAMED STANDS THROUGH AN OUTAGE: a Pro account's
			// run is never local, a free account's never blocked, because
			// of a billing service that is down (strict, Greg 2026-10-05:
			// "it's structural").
			h.plan, h.checked = h.knownPlan(token), h.now()
			log.Warn("billing did not answer; the plan it last named stands", slog.String("plan", string(h.plan)), slog.String("error", err.Error()))
		default:
			return nil, fmt.Errorf(planUnknown, h.billing, err)
		}
	}
	if h.plan == plan.Free {
		h.repo = nil
		return nil, nil
	}
	if h.repo != nil {
		return h.repo, nil
	}
	resp, err := h.client.Get(h.state + "/v1/health")
	if err != nil {
		return nil, fmt.Errorf(hostedStateUnusable, h.state, err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(hostedStateUnusable, h.state, resp.Status)
	}
	h.repo = marioapi.NewRESTWorkflowRepository(h.state, nil).WithToken(token)
	log.Info("runs keep their state on memdoor.ai", slog.String("state", h.state), slog.String("plan", string(h.plan)))
	return h.repo, nil
}
