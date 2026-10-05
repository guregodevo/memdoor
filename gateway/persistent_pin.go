package gateway

import (
	"encoding/json"
	"memdoor/pkg/shared"
	"os"
	"path/filepath"
)

// THE PIN STAYS (Greg, 2026-10-03: "it didnt pin it when i opened another
// session" → "it stays persistent", "the pin does not depend on workdir").
// A pin used to belong to the conversation, and a new window started fresh
// on the first rung. Now `/model` also writes the pin to the gateway's
// ~/.memdoor/model.json, every new conversation starts on it, and `/model
// auto` removes it. A conversation that pins its own model keeps that.
//
// The pin is the PERSON's: the file keeps one per actor, so on a gateway
// several people use, one person's /model never becomes everyone's default
// (2026-10-04).

const persistentPinFile = "model.json"

// persistentPin is what the file holds: a model by id with its host
// preference, or a rung of the ladder.
type persistentPin struct {
	Model string `json:"model,omitempty"`
	Sort  string `json:"sort,omitempty"`
	Order string `json:"order,omitempty"`
	Rung  int    `json:"rung,omitempty"` // 1-based
}

// persistentPinPath is a var so a test can point it at a temp dir.
var persistentPinPath = func() string {
	return shared.MemdoorHome(persistentPinFile)
}

// pinsFile is the file: each actor's kept pin.
type pinsFile struct {
	ByActor map[string]*persistentPin `json:"by_actor"`
}

func readPins() pinsFile {
	f := pinsFile{ByActor: map[string]*persistentPin{}}
	path := persistentPinPath()
	if path == "" {
		return f
	}
	if b, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(b, &f)
		if f.ByActor == nil {
			f.ByActor = map[string]*persistentPin{}
		}
	}
	return f
}

// readPersistentPin is the actor's kept pin, or nil. A run with no known
// actor (a scheduled workflow) takes the gateway's pin only when one person
// has kept one: the single-user machine keeps working, and on a shared
// gateway it borrows nobody's.
func readPersistentPin(actor string) *persistentPin {
	pins := readPins().ByActor
	p := pins[actor]
	if p == nil && actor == "" && len(pins) == 1 {
		for _, only := range pins {
			p = only
		}
	}
	if p == nil || (p.Model == "" && p.Rung <= 0) {
		return nil
	}
	return p
}

// writePersistentPin keeps the actor's pin; a nil pin removes it.
func writePersistentPin(actor string, p *persistentPin) error {
	path := persistentPinPath()
	if path == "" {
		return nil
	}
	f := readPins()
	if p == nil {
		delete(f.ByActor, actor)
	} else {
		f.ByActor[actor] = p
	}
	if len(f.ByActor) == 0 {
		err := os.Remove(path)
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, _ := json.MarshalIndent(f, "", "  ")
	return os.WriteFile(path, append(b, '\n'), 0o644)
}

// applyPersistentPin puts the actor's kept pin on a conversation that has no
// route of its own yet: the turn's copy and the registered session the footer
// reads. It reports whether a pin was applied.
func applyPersistentPin(actor string, sessions ...*Session) bool {
	var target *Session
	for _, s := range sessions {
		if s != nil && s.ID != "" {
			target = s
			break
		}
	}
	if target == nil || sessionRouted(target) {
		return false
	}
	p := readPersistentPin(actor)
	if p == nil {
		return false
	}
	for _, s := range sessions {
		if s == nil {
			continue
		}
		var err error
		if p.Model != "" {
			err = pinModel(s, p.Model, p.Sort, p.Order)
		} else {
			err = pinRoute(s, p.Rung, 0)
		}
		if err != nil {
			return false
		}
	}
	return true
}
