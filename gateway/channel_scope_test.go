package gateway

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"memdoor/pkg/auth"
	"memdoor/pkg/authorization"
	"memdoor/pkg/channel"
	"memdoor/pkg/domain"
	"memdoor/pkg/shared"
	sharedctx "memdoor/pkg/shared/context"

	"github.com/google/uuid"
)

// requestIn is a signed-in person's request as the auth middleware leaves
// it: the actor and the workspace of their row on the context.
func requestIn(method, path, actor, workspace string) *http.Request {
	req := httptest.NewRequest(method, path, nil)
	ctx := context.WithValue(req.Context(), sharedctx.ActorIDKey, shared.ActorID(actor))
	ctx = shared.WithExecutionContext(ctx, &shared.ExecutionContext{ActorID: shared.ActorID(actor), WorkspaceID: workspace})
	return req.WithContext(ctx)
}

func channelIn(name, workspace string) *domain.Channel {
	now := time.Now()
	return &domain.Channel{ID: uuid.New(), Name: name, WorkspaceScoped: domain.WorkspaceScoped{WorkspaceID: workspace},
		Timestamps: domain.Timestamps{CreatedAt: now, UpdatedAt: now}}
}

// One user, one workspace (Greg, 2026-10-20, after a fresh sign-up on
// memdoor.ai listed and read another account's channels): a person lists the
// channels of their workspace and reaches no other, public or not.
func TestAPersonReachesOnlyTheChannelsOfTheirWorkspace(t *testing.T) {
	cs := newTestChatServer()
	repo := cs.channelRepo.(*mockChannelRepo)
	gregGeneral, gregPrivate, bobGeneral := channelIn("general", "hackernews"), channelIn("greg-private", "hackernews"), channelIn("general", "bob")
	repo.channels = append(repo.channels, gregGeneral, gregPrivate, bobGeneral)
	cs.authzService = authorization.NewService(nonMember{&mockMembershipRepo{}}, repo, &mockBuddyRepo{}, nil)

	list := func(actor, workspace string) []string {
		rr := httptest.NewRecorder()
		cs.handleRooms(rr, requestIn("GET", "/api/channels", actor, workspace))
		if rr.Code != http.StatusOK {
			t.Fatalf("%s lists: %d %s", actor, rr.Code, rr.Body.String())
		}
		var resp struct {
			Channels []struct{ Name string } `json:"channels"`
		}
		_ = json.NewDecoder(rr.Body).Decode(&resp)
		var names []string
		for _, c := range resp.Channels {
			names = append(names, c.Name)
		}
		return names
	}
	if got := list("human:bob", "bob"); len(got) != 1 || got[0] != "general" {
		t.Fatalf("bob sees his own general and nothing of hackernews: %v", got)
	}
	if got := list("human:greg", "hackernews"); len(got) != 2 {
		t.Fatalf("greg sees his two: %v", got)
	}
	if got := list("system:internal", "default"); len(got) != 3 {
		t.Fatalf("the gateway's own caller sees all: %v", got)
	}

	// Reading another workspace's public channel by id is refused at the
	// rule every read goes through.
	read := func(actor, workspace string, ch *domain.Channel) bool {
		ok, err := cs.authzService.CanReadChannel(requestIn("GET", "/", actor, workspace).Context(), shared.ActorID(actor), channel.ChannelID(ch.ID.String()))
		if err != nil {
			t.Fatalf("%s reads %s: %v", actor, ch.Name, err)
		}
		return ok
	}
	if read("human:bob", "bob", gregPrivate) || read("human:bob", "bob", gregGeneral) {
		t.Fatal("bob reads greg's public channels by id")
	}
	if !read("human:greg", "hackernews", gregPrivate) || !read("human:bob", "bob", bobGeneral) {
		t.Fatal("each reads their own public channel")
	}
	if !read("agent:coder", "", gregPrivate) {
		t.Fatal("an agent passes")
	}
}

// userFake answers GetUserByID for the socket check.
type userFake struct {
	auth.Service
	users map[string]string // actor -> workspace
}

func (f *userFake) GetUserByID(_ context.Context, id string) (*auth.User, error) {
	ws, ok := f.users[id]
	if !ok {
		return nil, context.Canceled
	}
	u := &auth.User{}
	u.ID, u.WorkspaceID = id, ws
	return u, nil
}

// The socket follows the same rule: a person follows a channel of their own
// workspace only; agents and the local listener pass.
func TestTheSocketFollowsOnlyTheChannelsOfTheUsersWorkspace(t *testing.T) {
	repo := &mockChannelRepo{}
	gregGeneral, bobGeneral := channelIn("general", "hackernews"), channelIn("general", "bob")
	repo.channels = append(repo.channels, gregGeneral, bobGeneral)
	s := &Server{
		authzService: authorization.NewService(nonMember{&mockMembershipRepo{}}, repo, &mockBuddyRepo{}, nil),
		authAdapter:  NewAuthGatewayAdapter(&userFake{users: map[string]string{"human:bob": "bob", "human:greg": "hackernews"}}),
	}
	ctx := context.Background()
	if s.wsMayFollowChannel(ctx, "human:bob", gregGeneral.ID.String()) {
		t.Fatal("bob follows greg's channel")
	}
	if !s.wsMayFollowChannel(ctx, "human:bob", bobGeneral.ID.String()) {
		t.Fatal("bob follows his own public channel")
	}
	if s.wsMayFollowChannel(ctx, "human:nobody", bobGeneral.ID.String()) {
		t.Fatal("an unknown user follows nothing")
	}
	if !s.wsMayFollowChannel(ctx, "agent:coder", gregGeneral.ID.String()) || !s.wsMayFollowChannel(ctx, wsActorLocal, gregGeneral.ID.String()) {
		t.Fatal("agents and the local listener pass")
	}
}

// nonMember is a membership repository where nobody is a member: the
// reads that follow go through the public-channel rule.
type nonMember struct{ *mockMembershipRepo }

func (nonMember) IsMember(context.Context, channel.ChannelID, shared.ActorID) (bool, error) {
	return false, nil
}
