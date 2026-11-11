package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/coder/acp-go-sdk"
)

// The editor picks the model. The gateway pins a model per conversation
// (/api/route, "provider:model", the window's /model) and an editor had no way
// to reach it: session/set_config_option answered method-not-found. With
// MEMDOOR_ACP_MODELS set, a session offers a "model" option; picking one pins
// it on that conversation only (keep:false, so no other conversation's model
// changes), and "default" hands it back to the gateway's model. Unset, there is
// no option, as before.

const (
	acpModelsEnv      = "MEMDOOR_ACP_MODELS"
	acpDefaultNameEnv = "MEMDOOR_ACP_DEFAULT_NAME"
	acpModelOption    = acp.SessionConfigId("model")
	acpModelDefault   = acp.SessionConfigValueId("default")
)

// acpModel is one choice of the model option.
type acpModel struct {
	Pin  string // provider:model, as /api/route takes it
	Name string // what the editor shows
}

// acpModelsFromEnv reads MEMDOOR_ACP_MODELS: "pin|Name" entries separated by
// ";" or newlines. An entry without a name shows its pin.
// MEMDOOR_ACP_DEFAULT_NAME names the default choice.
func acpModelsFromEnv() (models []acpModel, defaultName string) {
	for _, entry := range strings.FieldsFunc(os.Getenv(acpModelsEnv), func(r rune) bool { return r == ';' || r == '\n' }) {
		pin, name, _ := strings.Cut(entry, "|")
		pin, name = strings.TrimSpace(pin), strings.TrimSpace(name)
		if pin == "" || acp.SessionConfigValueId(pin) == acpModelDefault {
			continue
		}
		if name == "" {
			name = pin
		}
		models = append(models, acpModel{Pin: pin, Name: name})
	}
	return models, strings.TrimSpace(os.Getenv(acpDefaultNameEnv))
}

// modelOptions is the session's config: the model option at its current
// value, or nothing when no choices are configured.
func (a *acpAgent) modelOptions(current acp.SessionConfigValueId) []acp.SessionConfigOption {
	if len(a.models) == 0 {
		return nil
	}
	name := "Default"
	if a.defaultModelName != "" {
		name = "Default (" + a.defaultModelName + ")"
	}
	opts := acp.SessionConfigSelectOptionsUngrouped{{Name: name, Value: acpModelDefault}}
	for _, m := range a.models {
		opts = append(opts, acp.SessionConfigSelectOption{Name: m.Name, Value: acp.SessionConfigValueId(m.Pin)})
	}
	if current == "" {
		current = acpModelDefault
	}
	category := acp.SessionConfigOptionCategoryModel
	return []acp.SessionConfigOption{{Select: &acp.SessionConfigOptionSelect{
		Type: "select", Id: acpModelOption, Name: "Model", Category: &category,
		CurrentValue: current, Options: acp.SessionConfigSelectOptions{Ungrouped: &opts},
	}}}
}

// SetSessionConfigOption pins the chosen model on the session's conversation.
func (a *acpAgent) SetSessionConfigOption(ctx context.Context, params acp.SetSessionConfigOptionRequest) (acp.SetSessionConfigOptionResponse, error) {
	v := params.ValueId
	if len(a.models) == 0 {
		return acp.SetSessionConfigOptionResponse{}, acp.NewMethodNotFound(acp.AgentMethodSessionSetConfigOption)
	}
	if v == nil || v.ConfigId != acpModelOption {
		return acp.SetSessionConfigOptionResponse{}, acp.NewInvalidParams(map[string]any{"error": "the only option is model"})
	}
	pin := ""
	if v.Value != acpModelDefault {
		for _, m := range a.models {
			if acp.SessionConfigValueId(m.Pin) == v.Value {
				pin = m.Pin
			}
		}
		if pin == "" {
			return acp.SetSessionConfigOptionResponse{}, acp.NewInvalidParams(map[string]any{"error": fmt.Sprintf("no model %q", v.Value)})
		}
	}
	a.mu.Lock()
	s, ok := a.sessions[v.SessionId]
	a.mu.Unlock()
	if !ok {
		return acp.SetSessionConfigOptionResponse{}, fmt.Errorf("no session %s: session/new first", v.SessionId)
	}
	if err := a.runner.SetModel(ctx, s.conversation, pin); err != nil {
		return acp.SetSessionConfigOptionResponse{}, err
	}
	return acp.SetSessionConfigOptionResponse{ConfigOptions: a.modelOptions(v.Value)}, nil
}

// SetModel pins model on the conversation, for it alone; "" hands it back to
// the gateway's model.
func (r *gatewayRunner) SetModel(ctx context.Context, conversation, model string) error {
	body := map[string]interface{}{"workspace": r.workspace, "channel_id": conversation, "agent": r.agentName(), "keep": false}
	if model == "" {
		body["tier"] = "auto"
	} else {
		body["model"] = model
	}
	var out map[string]interface{}
	if err := r.c.PostJSON("/api/route", body, &out); err != nil {
		return fmt.Errorf("could not set the model to %q: %w", model, err)
	}
	return nil
}
