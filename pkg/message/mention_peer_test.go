package message

import (
	"context"
	"strings"
	"testing"

	sharedctx "memdoor/pkg/shared/context"
)

func TestATurnAddressedToNobodyIsNamed(t *testing.T) {
	m := &Message{Content: MessageContent{Text: "@vps:coder say hi"}}
	if name, ok := unansweredMention(m); !ok || name != "vps:coder" {
		t.Fatalf("nobody answers to vps:coder: %q %v", name, ok)
	}
	m.Content.Mentions = []Mention{{ActorID: "agent:vps:coder", Name: "vps:coder"}}
	if _, ok := unansweredMention(m); ok {
		t.Fatal("resolved: answered")
	}
	for _, text := range []string{"fix the build", "tell @nobody to wait", "@"} {
		if _, ok := unansweredMention(&Message{Content: MessageContent{Text: text}}); ok {
			t.Errorf("%q opens with no mention", text)
		}
	}
}

func TestAPeersAgentIsAMention(t *testing.T) {
	got := ParseMentions("@box:coder review this, then @coder: merge it; cc @admin@localhost and @chief")
	if strings.Join(got, " ") != "box:coder coder admin@localhost chief" {
		t.Fatalf("mentions: %v", got)
	}
	cases := map[string][3]any{"box:coder": {"box", "coder", true}, "coder": {"", "", false}, "box:": {"", "", false}, ":coder": {"", "", false}}
	for in, want := range cases {
		peer, agent, ok := PeerAgent(in)
		if peer != want[0] || agent != want[1] || ok != want[2] {
			t.Errorf("%q: %q %q %v", in, peer, agent, ok)
		}
	}
}

func TestATurnAPeerAskedForWakesNobodyHere(t *testing.T) {
	if askedByPeer(context.Background()) {
		t.Fatal("a plain turn is not a peer's")
	}
	ctx := context.WithValue(context.Background(), sharedctx.AskedByPeerKey, true)
	if !askedByPeer(ctx) {
		t.Fatal("the flag was set")
	}
	next := asyncTurnContext(context.WithValue(context.WithValue(ctx, sharedctx.WorkdirKey, "/srv/app"), sharedctx.PermissionModeKey, "plan"))
	if wd, _ := next.Value(sharedctx.WorkdirKey).(string); wd != "/srv/app" {
		t.Errorf("the next turn lost the project: %q", wd)
	}
	if mode, _ := next.Value(sharedctx.PermissionModeKey).(string); mode != "plan" {
		t.Errorf("the next turn lost the mode: %q", mode)
	}
	if askedByPeer(next) {
		t.Error("the peer flag is the asked turn's alone, not its chain's")
	}
}

func TestWhatAPeersAgentIsAsked(t *testing.T) {
	human := &Message{AuthorID: "human:00000000-0000-0000-0000-000000000010", Content: MessageContent{Text: "@box:coder print the path"}}
	got := peerAskText(human, "box", "coder")
	if !strings.HasPrefix(got, "print the path\n\n(Asked from another gateway by a person, as @box:coder.") {
		t.Fatalf("a person's ask: %q", got)
	}
	agent := &Message{AuthorID: "agent:coder", Content: MessageContent{Text: "@box:coder what is your path?"}}
	got = peerAskText(agent, "box", "coder")
	if !strings.Contains(got, "by the agent coder, as @box:coder.") || !strings.Contains(got, "posted in the thread there, as box:coder") {
		t.Fatalf("an agent's ask: %q", got)
	}
}
