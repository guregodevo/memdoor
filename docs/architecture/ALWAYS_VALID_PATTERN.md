# "Always Valid" Pattern in Memdoor

## The principle

A domain value is checked once, where it enters the program, and never
again. The check lives in a constructor (`Parse*` or `New*`) that returns
the value or an error; code past that point receives a named type and
treats it as valid. This is coding principles 3 (fail-fast factory) and 10
(value objects, not bare strings) in `AGENTS.md`.

Two things follow. A bare `string` or `int64` that means something (a
channel, an actor, a plan) gets its own type, so the compiler keeps a
channel id out of an actor id slot. And a `Validate()` call deep in a
service is a sign the boundary let something through.

## How it is done here

### Identity types

`pkg/shared/actor_id.go` is the shared-kernel example. An `ActorID` is a
tagged string, `human:<id>` or `agent:<id>`; the behaviour sits on the type.

```go
type ActorID string

func NewHumanActorID(identityID string) ActorID {
	return ActorID(fmt.Sprintf("human:%s", identityID))
}

func NewAgentActorID(agentID string) ActorID {
	return ActorID(fmt.Sprintf("agent:%s", agentID))
}

func (id ActorID) Type() ActorType
func (id ActorID) ID() string
func (id ActorID) IsHuman() bool
func (id ActorID) IsAgent() bool
func (id ActorID) Validate() error
```

The two `New*` constructors can only produce a well-formed id. `Validate`
exists for the other direction: an `ActorID` that arrived as text (a token,
a row) is checked by whatever constructs the aggregate it goes into.

The same shape, one per package, for the other identities:

- `pkg/channel/channel_id.go`: `type ChannelID string`, with `IsZero`,
  `Validate` (not empty) and `String`.
- `pkg/message/message_id.go`: `type MessageID int64`, `Validate` refuses
  anything that is not positive.
- `pkg/channel/membership_id.go`: `MembershipID{ChannelID, ActorID}`, a
  composite; `NewMembershipID` builds it and `Validate` delegates to both
  parts. The pair is the identity, so join and leave are idempotent.

### `Parse*` constructors at the boundary

Where a value comes in as text, a `Parse*` function is the only way to get
the typed value, and it fails rather than guesses.

`pkg/plan/plan.go`: a `Plan` is a storage key, and an unknown one must not
resolve to something generous.

```go
func Parse(s string) (Plan, error) {
	switch p := Plan(strings.TrimSpace(strings.ToLower(s))); p {
	case "":
		return Free, nil
	case Free, Enterprise, Pro:
		return p, nil
	case Solo, "team":
		return Pro, nil
	default:
		return "", fmt.Errorf("unknown plan %q (want free | pro | enterprise)", s)
	}
}
```

Its callers are the boundaries: `gateway/billingsvc/plan_handler.go` turns
the error into a 400, `gateway/billingsvc/auth.go` and
`gateway/hosted_state.go` parse what an account or a file says before it is
used.

`pkg/notes/repository.go`: a `ConversationKey` names whose notes these are,
and a notes call with no conversation is refused with a named error.

```go
type ConversationKey string

var ErrNoConversation = errors.New("notes belong to a conversation, and this call has none")

func ParseConversationKey(s string) (ConversationKey, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", ErrNoConversation
	}
	return ConversationKey(s), nil
}
```

`tools/notes.go` calls it on the tool's input; `Repository.Load`, `Append`
and `Delete` take a `ConversationKey`, never a string, so no path of the
repository runs for an empty key.

`pkg/decision/decision.go` has two more: `ParseKind` (a question's answer
shape, `choice | score | boolean`, with the System One aliases) and
`ParseModelRef` (`<provider>[/<model>]`, where `""` is unconfigured and
`off | none | disabled` is deliberately off). `tools/decision_tool.go` and
`gateway/decisions.go` parse the kind at the tool and API edge;
`pkg/decision/service.go` parses the model reference from the setting
before every call.

### Aggregates built by a constructor

`pkg/shared/execution_context.go`: the request's who-and-where. The
constructor is the one place the invariants are checked.

```go
func NewExecutionContext(actorID ActorID, workspaceID string) (*ExecutionContext, error) {
	if err := actorID.Validate(); err != nil {
		return nil, fmt.Errorf("invalid actor_id: %w", err)
	}
	if workspaceID == "" {
		return nil, fmt.Errorf("workspace_id cannot be empty")
	}
	return &ExecutionContext{ActorID: actorID, WorkspaceID: workspaceID}, nil
}
```

`NewExecutionContextWithEmail` is the variant the auth middleware uses
(`pkg/authorization/middleware.go`, after the token is verified: an error
there is a 500, because a verified user with no workspace is a data
problem, not a request problem). `MustNewExecutionContext` panics instead
of returning the error and is reserved for inputs the program itself
produced (`pkg/message/service.go` uses it to build the fallback context,
a fixed CLI actor plus the channel's workspace, when a request arrives
without one). The
`Validate()` method on `ExecutionContext` is marked deprecated in the
source for exactly this reason: nothing built by the constructor can fail it.

`pkg/message/message.go`: a `Message` is append-only, and `NewMessage`
checks the channel id, the author (`authorID.Validate()`) and the content
(`MessageContent.Validate()`, `pkg/message/content.go`: text not empty,
every mention well formed) before it exists. `NewReply` adds a positive
`MessageID` for the parent. Membership follows the same shape in
`pkg/channel/membership.go` (`NewMembership`).

### Factories return an interface

`pkg/notes/file_repository.go` builds the concrete type and hands back the
interface, failing on a missing root rather than storing an empty one.

```go
func NewFileRepository(root string) (Repository, error) {
	if strings.TrimSpace(root) == "" {
		return nil, errors.New("notes: a folder to keep them in is required")
	}
	return &fileRepository{root: root}, nil
}
```

The caller holds a `notes.Repository`; `fileRepository` is unexported.

## When to use it

Use a named type and a `Parse*` constructor for anything that is an
identity or a closed set and arrives as text: an actor, a channel, a
message, a conversation key, a plan, a decision kind, a model reference.
Build an aggregate through `New*` whenever it has an invariant another
layer relies on (`ExecutionContext`, `Message`, `ChannelMembership`).

Do not use it for API payloads and rows (`gateway/` request structs, the
`pkg/domain` models) which are decoded as is and handed to a constructor
afterwards, or for mutable working state where a field can legitimately be
empty while it is filled in.

Two places in the repo still carry a bare string where a type belongs:
`ExecutionContext.WorkspaceID` and `Message.ChannelID` (the `ChannelID`
type lives in `pkg/channel` and is not used on the message side). When one
of them is next touched, it should take a type by principle 11: change the
field, run `go build ./...`, fix what the compiler lists.

## How it is tested

A constructor's test covers both sides of the boundary: the valid form,
with trimming and case where the constructor allows it, and the refusal.
Business-logic tests then build a valid value once and never call
`Validate()`.

- `pkg/plan/plan_test.go`: `TestUnknownPlanIsRefusedRatherThanGuessed`
  (`premium`, `unlimited`, `prro` fail; `"  ENTERPRISE "` parses),
  `TestNoPlanMeansFree`, `TestTheRetiredPrepaidTierReadsAsPro`,
  `TestStorageKeysAreFrozen`.
- `pkg/notes/file_repository_test.go`: `TestParseConversationKeyFailsFast`
  (blank is `ErrNoConversation`; padding is trimmed).
- `pkg/decision/decision_test.go`: `TestModelRef` (`" systemone/jev-latest "`,
  `OFF`, `""`, and the `String` round trip).
- `pkg/message/message_test.go`: `TestActorID` (both constructors, and
  `Validate` refusing `invalid` and `bot:123`).

Run them together with `go test ./pkg/plan/ ./pkg/notes/ ./pkg/decision/
./pkg/message/`.
