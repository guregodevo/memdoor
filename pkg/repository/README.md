# Repositories

The domain reaches the database only through the interfaces in this package;
`gateway/` and `pkg/` services never write SQL. One implementation exists,
SQLite, in `sqlite/`.

## Shape

- `interfaces.go` declares one interface per aggregate (`UserRepository`,
  `ChannelRepository`, `BuddyRepository`, `SessionRepository`,
  `CronJobRepository`, `AgentTurnRepository`, …) and `RepositoryFactory`, the
  one object that hands them out. Some aggregates keep their repository
  interface beside their domain type instead: `message.Repository`,
  `channel.MembershipRepository`, `filestore.Repository`.
- `agent_secret.go` is the per-agent keyring (`AgentSecretRepository`): values
  are write-only from the outside and read back only through the agent's
  `get_secret` tool.
- `sqlite/factory.go` opens the database, creates the schema (`CREATE TABLE IF
  NOT EXISTS`, so a new table appears on the next start) and builds every
  repository once; `gateway/repository.NewRepositoryFactory` is the only
  caller, from `gateway/server.go`.
- `sqlite/stubs.go` holds the parts a single-tenant install does not need: a
  SQLite gateway has one workspace, made at setup, so the workspace repository
  reads but does not create, update or delete.

## Rules

- A constructor returns the interface, never the struct (`NewBuddyRepository(db)
  repository.BuddyRepository`).
- Identities are value objects (`shared.ActorID`, `message.MessageID`,
  `domain.WorkspaceID`), parsed at the boundary; a repository never takes a
  bare string where a type exists.
- Tests use the mocks in `gateway/handler_test_helpers_test.go` or an
  in-memory SQLite file; nothing in a test opens `~/.memdoor`.
- Migrations are additive: a schema change is a new `CREATE TABLE` or
  `ALTER TABLE ADD COLUMN` in `factory.go`, guarded so a second start is a
  no-op.
