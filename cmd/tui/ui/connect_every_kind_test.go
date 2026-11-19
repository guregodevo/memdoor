package ui

import (
	"testing"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
)

// connectDriver walks /connect <kind> the way the window does: a key goes to
// the flow first, then to the input line.
type connectDriver struct {
	t   *testing.T
	m   *Model
	got ConnectRequest
}

func openConnect(t *testing.T, kind string) *connectDriver {
	d := &connectDriver{t: t, m: &Model{width: 100, input: textarea.New()}}
	d.m.input.Focus()
	d.m.connectOps = ConnectOps{Connect: func(req ConnectRequest) (ConnectResult, error) {
		d.got = req
		return ConnectResult{OK: true, ID: req.ID}, nil
	}}
	d.m.connectCommand([]string{kind})
	return d
}

func (d *connectDriver) key(k tea.KeyMsg) {
	if handled, _ := d.m.connectKey(k); !handled {
		d.m.input, _ = d.m.input.Update(k)
	}
}

func (d *connectDriver) paste(s string) {
	d.key(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s), Paste: true})
}

func (d *connectDriver) enter() {
	if _, cmd := d.m.connectKey(press(tea.KeyEnter)); cmd != nil {
		cmd()
	}
}

func (d *connectDriver) step() int {
	if d.m.connect == nil {
		return -1
	}
	return d.m.connect.step
}

// Every kind that prefills its key's variable name: a pasted or typed key
// replaces the name, Enter keeps it, Backspace clears it.
func TestConnectKeyReplacesThePrefilledNameForEveryKind(t *testing.T) {
	for _, kind := range ConnectKinds {
		if kind.KeyVar == "" {
			continue
		}
		t.Run(kind.ID, func(t *testing.T) {
			open := func() *connectDriver {
				d := openConnect(t, kind.ID)
				d.enter() // the base, as prefilled
				if d.step() != connectStepKey || d.m.input.Value() != kind.KeyVar {
					t.Fatalf("the key step with %s prefilled: step %d, %q", kind.KeyVar, d.step(), d.m.input.Value())
				}
				return d
			}

			d := open()
			d.paste("pasted-key-123")
			d.enter()
			if d.got.Key != "pasted-key-123" {
				t.Errorf("a pasted key replaces the name: sent %q", d.got.Key)
			}

			d = open()
			for _, r := range "typed-key" {
				d.key(press(tea.KeyRunes, r))
			}
			d.enter()
			if d.got.Key != "typed-key" {
				t.Errorf("a typed key replaces the name: sent %q", d.got.Key)
			}

			d = open()
			d.enter()
			if d.got.Key != kind.KeyVar {
				t.Errorf("enter keeps the name: sent %q", d.got.Key)
			}

			d = open()
			d.key(press(tea.KeyBackspace))
			if v := d.m.input.Value(); v != "" {
				t.Errorf("backspace clears the name: %q", v)
			}
		})
	}
}

// The endpoint kinds prefill no name: the pasted key goes in as it is, and
// the model asked after it goes with it.
func TestConnectEndpointKindsTakeAPastedKey(t *testing.T) {
	for _, id := range []string{"gateway", "custom"} {
		t.Run(id, func(t *testing.T) {
			d := openConnect(t, id)
			if d.step() == connectStepID {
				d.paste("mine")
				d.enter()
			}
			d.enter() // the base, as prefilled
			if d.step() != connectStepKey || d.m.input.Value() != "" {
				t.Fatalf("an empty key step: step %d, %q", d.step(), d.m.input.Value())
			}
			d.paste("pasted-key-123")
			d.enter()
			if d.step() == connectStepModel {
				d.paste("some-model")
				d.enter()
			}
			if d.got.Key != "pasted-key-123" || d.got.Model != "some-model" {
				t.Errorf("sent key %q, model %q", d.got.Key, d.got.Model)
			}
		})
	}
}
