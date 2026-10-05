# ADR-0014: Generated shots through the broker

Date: 2026-09-20 · Status: accepted, live on main (first shot through the app 2026-09-20)

## Context

"A branch for generating short movies" (Greg, 2026-09-20). The vendor that
serves the brain (ADR-0011, Qwen Cloud / DashScope) also generates video
from a prompt: HappyHorse text-to-video, an async task — submit, poll,
download, 3–15 s per shot, 480P–1080P, any of nine ratios. The editor
already narrates over b-roll it fetched or photographed; where no
footage exists for a beat, a shot can be generated.

## Decision

- **Through the broker, with the brain's key.** `POST /v1/brain/video`
  submits, `GET /v1/brain/video/{task_id}` queries; the broker holds
  `MEMDOOR_API_BRAIN_KEY` and speaks the vendor's shape
  (`X-DashScope-Async: enable`, `/services/aigc/video-generation/video-synthesis`,
  `/tasks/{id}`). The client never sees the key or the vendor.
  `MEMDOOR_VIDEO_GEN_URL` / `MEMDOOR_VIDEO_GEN_MODEL` override the vendor
  base and model (`happyhorse-1.1-t2v`).
- **A seat generates; free edits.** A generation costs real money per
  second of video, unlike a chat turn; a free workspace is refused with
  the reason (402, "generating video needs a seat"). The month's generated
  seconds are tallied per workspace in the registry (`videos`).
- **The clip lands like a fetched one.** `clip_generate` (tools) polls
  every 15 s up to 15 min, downloads, measures, writes
  `<out>.source.json` with `generated: true`, the prompt and the credit
  line, and reports the file the way clip_fetch does. `clip_stitch` uses
  it as any source; the credits say "AI-generated video · HappyHorse (Qwen
  Cloud)".
- **Watermark off** on the vendor side: the film carries memdoor's own
  mark under the freemium rule, not the vendor's.

## Consequences

- A short movie is a shot list: N prompts → N shots → one `clip_stitch`
  with narration and a bed (`skills/shortfilm.md`).
- Pricing per generated second is the vendor's and not yet in the
  seat's economics; the tally is the number to read before opening it to
  free workspaces or capping seats.
- Content inspection applies to prompts as to chat; a refused prompt
  comes back as a FAILED task with the vendor's code, named in the error.
