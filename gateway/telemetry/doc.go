// Package telemetry is the transport that ships local-gateway log
// events to the memdoor.ai monitoring server. It is NOT a separate
// emission API — code that wants to be telemetered just logs
// through the existing gateway/logs system; the wrapper here taps
// the same events on their way to the local sqlite Storage.
//
// Architecture:
//
//	Memdoor gateway (local)
//	┌─────────────────────────────┐
//	│  logger.Info(...)           │  emission, unchanged
//	│         ↓                   │
//	│  logs.Storage.WriteEvents() │  the existing event sink
//	│         ↓                   │
//	│  telemetry.WrapStorage      │  decorator: forwards to inner
//	│    ├─► inner sqlite         │  storage AND enqueues a copy
//	│    └─► sink.Enqueue(events) │  to the local ring
//	│              ↓              │
//	│  gateway/telemetry/ring         │  bounded in-memory (default 200
//	│              ↓              │  events). v0 has NO disk persistence;
//	│                             │  gateway crash before flush =
//	│                             │  losing pending events.
//	│              ↓              │
//	│  gateway/telemetry/flusher      │  background goroutine
//	│              ↓ POST batch   │
//	└──────────────│──────────────┘
//	               ↓
//	       https://memdoor.ai
//	       /api/telemetry              receives batch, writes to its
//	                                   own logs.Storage; the maintainer queries
//	                                   via `memdoor logs query` over
//	                                   the same UX
//
// Privacy model: opt-in (default OFF). Configured via env vars at
// gateway boot — MEMDOOR_TELEMETRY_ENABLED=1 to turn on,
// MEMDOOR_TELEMETRY_TOKEN for the
// anonymous bearer token. v0 filters to WARN+ERROR only — the
// silent failure surface the maintainer's 2026-05-28 dogfood session was
// trying to investigate. Wider filtering (INFO, DEBUG) is a future
// configuration switch when usage analytics become necessary.
//
// What this package does NOT do today:
//   - Disk persistence of the ring. v0 keeps everything in memory.
//     Gateway crash before flush = losing the last interval of
//     events. Acceptable for the "fewer-than-50 events/day per
//     user" workload WARN+ERROR produces.
//   - Per-user authentication. The bearer token deters random POST
//     spam, not impersonation. Phase 2 (multi-tenant memdoor.ai)
//     needs a real auth story.
//   - Compression. Batches are small; gzip would help bandwidth on
//     mobile uplinks but isn't load-bearing today.
package telemetry
