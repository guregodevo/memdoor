package gateway

import (
	"strings"
	"testing"
)

// A LONG TURN ENDS ON A RECAP, NOT A LINE. Eight tools and "Done." is asked
// again; three tools and "Done." is fine; eight tools and a real recap is
// fine. The mutation check: raise the bar past any turn and "Done." stands.
func TestALongTurnEndingOnALineIsAskedForARecap(t *testing.T) {
	if !recapWanted(9, "Done.") {
		t.Fatal("nine tools and one word is not a recap")
	}
	if recapWanted(3, "Done.") {
		t.Fatal("a short turn may end short")
	}
	long := strings.Repeat("Made sara_short_1.mp4, 31 s, captioned. ", 8)
	if recapWanted(12, long) {
		t.Fatal("a real recap is not asked for again")
	}
	if n := recapNudge(9); !strings.Contains(n, "9 tools") || !strings.Contains(n, "what is next") {
		t.Fatalf("the nudge says what a recap is: %q", n)
	}
}
