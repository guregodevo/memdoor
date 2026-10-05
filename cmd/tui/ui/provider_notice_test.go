package ui

import (
	"strings"
	"testing"
)

// A provider notice (a retry after silence or a refusal) stays on screen as
// a line of its own, so a long wait says why.
func TestAProviderNoticeIsShown(t *testing.T) {
	m, _, _ := freshModel(t, nil)
	nm, _ := m.Update(providerNoticeMsg{text: "The model's provider sent nothing for 1m0s — asking again."})
	*m = nm.(Model)
	if !strings.Contains(m.renderMessages(), "sent nothing for 1m0s") {
		t.Fatal("the notice must be on screen")
	}
}
