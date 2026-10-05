package gateway

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"memdoor/gateway/logs"
	"net/http"
	"regexp"
	"strings"

	"memdoor/gateway/providers"
	"memdoor/pkg/authorization"
	"memdoor/pkg/shared"
)

// Legible routing (docs/roadmap/MUST.md, "TUI table stakes"; SHOULD.md
// phase 4): the person sees which rung of the agent's ladder a
// conversation runs on and why, and can pin one. Never an opaque auto
// mode. GET /api/route?workspace=&channel_id=&agent= reads it; POST the same
// fields with "tier" (1-based rung, or 0 / "auto") pins or releases.
//
// The rung lives on the session (route_tier, turn_tier.go); a pin adds
// route_pinned and route_reason, and the turn runs on exactly that rung.

const (
	sessionPinnedKey  = "route_pinned"
	sessionReasonKey  = "route_reason"
	sessionModelKey   = "route_model"       // a model pinned by id, off the ladder
	sessionSortKey    = "route_sort"        // host preference for it: price | throughput | latency | default
	sessionOrderKey   = "route_order"       // the person's own host order, comma-separated
	sessionEffortKey  = "route_effort"      // reasoning effort the person chose (Shift+Tab); "" = auto
	sessionEffortUsed = "route_effort_used" // the last turn's effort, and where it came from:
	sessionEffortFrom = "route_effort_from" // "chosen", "decision model" or "default"
	reasonPinned      = "pinned by you"
)

// modelIDRe: vendor/model as OpenRouter names them, or a provider's own id
// (claude-haiku-4-5-20251001, a gateway's slug) now that every connected
// provider's list is pinnable (providers/registry.go, 2026-10-02). Whether
// a provider actually lists it is the client's catalogue check.
var modelIDRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:/-]*$`) // Baseten's ids carry case: zai-org/GLM-5.3-Flash

func sessionString(s *Session, key string) string {
	if s == nil {
		return ""
	}
	v, _ := s.GetMetadataValue(key)
	str, _ := v.(string)
	return str
}

// sessionModel is the model the person pinned by id, "" when the ladder rules.
func sessionModel(s *Session) string {
	if s == nil {
		return ""
	}
	v, _ := s.GetMetadataValue(sessionModelKey)
	m, _ := v.(string)
	return m
}

// routeView is what the screen shows.
type routeView struct {
	Agent         string   `json:"agent"`
	Tier          int      `json:"tier"`                     // 0-based rung
	Rung          int      `json:"rung"`                     // 1-based, for people
	Rungs         []string `json:"rungs"`                    // the ladder's models, cheap first
	RungProviders []string `json:"rung_providers,omitempty"` // which provider serves each rung (registry.go), parallel to Rungs
	Model         string   `json:"model"`                    // the rung's model
	Reason        string   `json:"reason"`                   // why this rung
	Pinned        bool     `json:"pinned"`
	Custom        bool     `json:"custom"`             // Model is a pin by id, not a rung
	Provider      string   `json:"provider,omitempty"` // which provider serves Model (providers/registry.go)
	Sort          string   `json:"sort,omitempty"`     // host preference for a pin by id
	Order         string   `json:"order,omitempty"`    // the person's host order for it
	Effort        string   `json:"effort"`             // low | medium | high, or "" for auto
	// EffortUsed and EffortFrom are what the last turn ran at and why, so
	// the footer shows the effort in use, not only "auto".
	EffortUsed string `json:"effort_used,omitempty"`
	EffortFrom string `json:"effort_from,omitempty"`
}

// ladderFor is the agent's ladder as the start engine carries it; one rung when
// the agent has no ladder.
func ladderFor(re *providers.RemoteEngine, agent string) []string {
	if re == nil {
		return nil
	}
	if l := re.AgentLadders[strings.ToLower(agent)]; len(l) > 0 {
		return append([]string(nil), l...)
	}
	if m := re.AgentModels[strings.ToLower(agent)]; m != "" {
		return []string{m}
	}
	if re.Model != "" {
		return []string{re.Model}
	}
	return nil
}

// routeReason words the rung: a pin's reason, an escalation's, or the
// default.
func routeReason(tier int, pinned bool, reason string) string {
	if reason != "" {
		return reason
	}
	if pinned {
		return reasonPinned
	}
	if tier == 0 {
		return "first rung · the default"
	}
	return fmt.Sprintf("rung %d", tier+1)
}

// sessionRouted says the gateway holds a rung for this session at all (a
// pin, a release, later an escalation). From then on the gateway's tier is
// the truth.
func sessionRouted(s *Session) bool {
	if s == nil {
		return false
	}
	_, ok := s.GetMetadataValue(sessionTierKey)
	return ok
}

func sessionPinned(s *Session) bool {
	if s == nil {
		return false
	}
	v, _ := s.GetMetadataValue(sessionPinnedKey)
	b, _ := v.(bool)
	return b
}

func sessionReason(s *Session) string {
	if s == nil {
		return ""
	}
	v, _ := s.GetMetadataValue(sessionReasonKey)
	r, _ := v.(string)
	return r
}

// viewRoute reads a session's route against the start engine.
func viewRoute(re *providers.RemoteEngine, s *Session, agent string) routeView {
	rungs := ladderFor(re, agent)
	tier := sessionTier(s)
	if len(rungs) > 0 && tier >= len(rungs) {
		tier = len(rungs) - 1
	}
	v := routeView{Agent: agent, Tier: tier, Rung: tier + 1, Rungs: rungs, Pinned: sessionPinned(s), Reason: routeReason(tier, sessionPinned(s), sessionReason(s)), Effort: sessionString(s, sessionEffortKey),
		EffortUsed: sessionString(s, sessionEffortUsed), EffortFrom: sessionString(s, sessionEffortFrom)}
	if tier < len(rungs) {
		v.Model = rungs[tier]
	}
	v.Provider = providers.ActiveProviderID()
	// Each rung's provider is a fact read from the registry, not the
	// ladder's engine assumed for all (a review of 6d8f9e2 flagged the
	// assumption, 2026-10-03): the ladder is the active engine's, and a rung
	// another provider lists under the same id stays the active one's.
	for range rungs {
		v.RungProviders = append(v.RungProviders, v.Provider)
	}
	if m := sessionModel(s); m != "" {
		v.Model, v.Custom, v.Rung, v.Tier = m, true, 0, 0
		v.Sort, v.Order = sessionString(s, sessionSortKey), sessionString(s, sessionOrderKey)
		// A pin by id is served by the provider that lists it — another
		// one than the active engine's, when /model named a model of its.
		if p, model, ok := providers.FindModel(context.Background(), m); ok {
			v.Provider, v.Model = p.ID, model.ID
		}
	}
	return v
}

// pinRoute holds a rung for the session (1-based rung; 0 releases the pin
// and the session goes back to the first rung).
func pinRoute(s *Session, rung, ladderLen int) error {
	if rung < 0 || (ladderLen > 0 && rung > ladderLen) {
		return fmt.Errorf("rung must be between 1 and %d (0 or auto releases the pin)", ladderLen)
	}
	s.SetMetadata(sessionModelKey, "")
	s.SetMetadata(sessionSortKey, "")
	s.SetMetadata(sessionOrderKey, "")
	if rung == 0 {
		s.SetMetadata(sessionTierKey, 0)
		s.SetMetadata(sessionPinnedKey, false)
		s.SetMetadata(sessionReasonKey, "")
		return nil
	}
	s.SetMetadata(sessionTierKey, rung-1)
	s.SetMetadata(sessionPinnedKey, true)
	s.SetMetadata(sessionReasonKey, reasonPinned)
	return nil
}

// pinModel holds a model by id for the session — any model the catalogue
// offers, off the ladder (Greg, 2026-09-26: "user can change models").
func pinModel(s *Session, id, sortBy, order string) error {
	id = strings.TrimSpace(id)
	if !modelIDRe.MatchString(id) {
		return fmt.Errorf("not a model id: %q (an id as /model-search lists them)", id)
	}
	switch sortBy = strings.ToLower(strings.TrimSpace(sortBy)); sortBy {
	case "", "default", "price", "throughput", "latency":
	default:
		return fmt.Errorf("the host preference is price, throughput, latency or default (got %q)", sortBy)
	}
	s.SetMetadata(sessionModelKey, id)
	s.SetMetadata(sessionSortKey, sortBy)
	s.SetMetadata(sessionOrderKey, strings.ToLower(strings.ReplaceAll(strings.TrimSpace(order), " ", "")))
	s.SetMetadata(sessionTierKey, 0)
	s.SetMetadata(sessionPinnedKey, true)
	s.SetMetadata(sessionReasonKey, reasonPinned)
	return nil
}

// setEffort holds the reasoning effort for the session; "auto" (or "")
// hands it back to the turn (turn_effort.go).
func setEffort(s *Session, effort string) error {
	switch e := strings.ToLower(strings.TrimSpace(effort)); e {
	case "auto", "":
		s.SetMetadata(sessionEffortKey, "")
	case "low", "medium", "high":
		s.SetMetadata(sessionEffortKey, e)
	default:
		return fmt.Errorf("effort is low, medium, high or auto (got %q)", effort)
	}
	return nil
}

// handleRoute is GET/POST /api/route.
func (s *Server) handleRoute(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	var in struct {
		Workspace string      `json:"workspace"`
		ChannelID string      `json:"channel_id"`
		Agent     string      `json:"agent"`
		Tier      interface{} `json:"tier"`   // number (1-based rung, 0 = auto) or "auto"
		Model     string      `json:"model"`  // or a model id, off the ladder
		Sort      string      `json:"sort"`   // with it: price | throughput | latency | default
		Order     string      `json:"order"`  // with it: the person's host order, comma-separated
		Effort    *string     `json:"effort"` // alone: the reasoning effort, low | medium | high | auto
	}
	switch r.Method {
	case http.MethodGet:
		q := r.URL.Query()
		in.Workspace, in.ChannelID, in.Agent = q.Get("workspace"), q.Get("channel_id"), q.Get("agent")
	case http.MethodPost:
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
			http.Error(w, `{"error": "bad json"}`, http.StatusBadRequest)
			return
		}
	default:
		http.Error(w, `{"error": "GET or POST"}`, http.StatusMethodNotAllowed)
		return
	}
	in.Agent = strings.ToLower(strings.TrimSpace(in.Agent))
	scope := strings.TrimSpace(in.Workspace)
	if scope == "" || in.ChannelID == "" || in.Agent == "" {
		http.Error(w, `{"error": "workspace, channel_id and agent are required"}`, http.StatusBadRequest)
		return
	}
	key := shared.NewChannelSessionID(scope, in.ChannelID)
	re := providers.ActiveRemoteEngine()
	if r.Method == http.MethodGet {
		sess, _ := s.sessions.GetSession(key) // nil before the first turn
		if sess == nil || !sessionRouted(sess) {
			// A conversation with no route of its own will start on the kept
			// pin (persistent_pin.go): the footer says so before the first
			// turn, not after it (Greg, 2026-10-03: "if someone pins it he
			// should see it pinned").
			tmp := &Session{ID: key, Metadata: map[string]interface{}{}}
			if applyPersistentPin(string(authorization.GetActorID(r.Context())), tmp) {
				sess = tmp
			}
		}
		_ = json.NewEncoder(w).Encode(viewRoute(re, sess, in.Agent))
		return
	}
	rung := 0
	switch t := in.Tier.(type) {
	case float64:
		rung = int(t)
	case string:
		if strings.TrimSpace(strings.ToLower(t)) != "auto" && strings.TrimSpace(t) != "" {
			fmt.Sscanf(t, "%d", &rung)
		}
	}
	sess, err := s.sessions.GetOrCreateSession(key, "main")
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error": %q}`, err.Error()), http.StatusInternalServerError)
		return
	}
	var perr error
	if in.Effort != nil && in.Model == "" && in.Tier == nil {
		perr = setEffort(sess, *in.Effort)
	} else if in.Model != "" {
		perr = pinModel(sess, in.Model, in.Sort, in.Order)
	} else {
		perr = pinRoute(sess, rung, len(ladderFor(re, in.Agent)))
	}
	if perr != nil {
		http.Error(w, fmt.Sprintf(`{"error": %q}`, perr.Error()), http.StatusBadRequest)
		return
	}
	if in.Effort == nil || in.Model != "" || in.Tier != nil {
		// THE PIN STAYS (persistent_pin.go): kept for every new
		// conversation until auto.
		var keep *persistentPin
		switch {
		case in.Model != "":
			keep = &persistentPin{Model: in.Model, Sort: in.Sort, Order: in.Order}
		case rung > 0:
			keep = &persistentPin{Rung: rung}
		}
		if err := writePersistentPin(string(authorization.GetActorID(r.Context())), keep); err != nil {
			logs.New("Agent").Warn("pin not kept: " + err.Error())
		}
	}
	v := viewRoute(re, sess, in.Agent)
	logs.New("Agent").Info("route pinned", slog.String("session", key), slog.String("agent", in.Agent),
		slog.Int("rung", v.Rung), slog.String("model", v.Model), slog.Bool("pinned", v.Pinned))
	_ = json.NewEncoder(w).Encode(v)
}
