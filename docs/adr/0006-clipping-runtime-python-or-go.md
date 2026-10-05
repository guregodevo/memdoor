# ADR-0006 — The clipping runtime: Python sidecar today, the Go engine tomorrow

Status: ACCEPTED (2026-09-03); whisper and captions ported the same day —
see "Progress" at the end. Greg: "we need to review this design… because we
don't want to depend on python."

## The problem

The clipper's tools are Go, but four of them shell out to a Python venv
(`~/.memdoor/clipenv`, ~1.5 GB): mlx-whisper (transcribe, word timing),
mlx-vlm (see, thumbnail), PIL + reshaper/bidi/fontTools (captions), OpenCV +
YuNet (reframe). ffmpeg is a fifth external dependency (Homebrew).

This collides with two of memdoor's founding rules:
- **single signed binary, no corporate-install deps** — the locked-corporate-
  Mac wedge; pip and native wheels are exactly what such a machine refuses;
- **the engine is ours** — the whole 9B port exists so that inference is in
  our process, on Metal, with our cache and our receipts.

And it costs latency: every call is a fresh process that reloads its model —
measured today: whisper ~8s, the vision model ~20s, per call.

## Options

| | A. Managed venv (today) | B. Persistent Python sidecar | C. Go + our MLX engine (target) | D. Ship external binaries |
|---|---|---|---|---|
| user installs | `memdoor clip setup` runs pip | same | nothing — one binary + weights | nothing — setup downloads binaries |
| locked Mac | ✗ pip/wheels refused | ✗ | ✓ | ~ (unsigned binaries also refused) |
| per-call latency | 8–20s model reload | ~0 (loaded once) | ~0, and shares the loaded 9B | n/a |
| memory | second copy of the 9B for vision | second copy | ONE 9B in memory (text + vision) | — |
| effort | done | ~2 days | whisper 1–2 wks · vision tower 1–2 wks · YuNet on MLX-C days · shaping (go-text) days | days |
| runtimes | Go + Python 3.14 | Go + Python | Go | Go + binaries |

## Decision

**C, and only C, ships. Python never ships.** The venv on the founder's Mac
is DEVELOPMENT scaffolding — it proved each capability (local whisper, local
vision, shaped captions, face tracking) before its Go port; it is not a
bridge users see. No `clip setup` that runs pip. No sidecar (B) in the
product. What is not yet ported is not yet released.

Port order, by what unlocks the most: **whisper on the MLX engine first** —
transcribe, tighten and caption timing all hang on word timestamps, so it is
the keystone; then caption rendering in Go (`go-text/typesetting` shaping,
`image/draw`); then YuNet on MLX-C ops; then the Qwen3.5 vision tower into
`llm/qwen35`. Until whisper lands, the Go-only tools (cut, conform, convert,
preview, play, thumbnail's sharpness pass) are what a release contains.

The earlier plan below is kept for the record of what was considered.

1. Now: `memdoor clip setup` creates the venv, fetches models/fonts/ffmpeg,
   verifies each with a receipt. Users exist.
2. Next (2 days): run the venv as ONE persistent sidecar process the gateway
   starts (`clipd`: JSON over a unix socket). Kills the 8–20s reloads; the
   vision model and whisper stay warm. Same Python, no more per-call boot.
3. Port in the order value ÷ effort: YuNet face detection as MLX-C ops (a
   230 KB net — days) → caption shaping in Go (`go-text/typesetting`: HarfBuzz
   shaping in pure Go — days) → whisper on the MLX engine (an encoder-decoder
   smaller than the 9B; the KV/prefix machinery is already ours — weeks) →
   the Qwen3.5 vision tower into `llm/qwen35` (weights already in the
   download; the mrope path is already text-partial — weeks). Each port
   deletes its Python and the sidecar shrinks until it is gone.
4. ffmpeg: treat as weights — a static build fetched by setup, checksummed,
   signed with the install (it is already the one external dep the skill
   assumes). Never Homebrew.

## Why not stop at B

B fixes speed, not the wedge. A clipper on a personal Mac is fine with a
venv; the operator on a managed Mac, and every regulated buyer later, is not.
C also puts vision in the SAME loaded model the brain uses — the local 9B
sees with no second copy in memory, which matters on 16 GB.

## Consequences

- Two runtimes for a while; every Python tool must be callable through one
  contract (argv in, JSON on the last line) so the sidecar is a drop-in.
- The engine roadmap gains whisper and the vision tower as first-class
  milestones (they were "gate 4: productize" — now they are the plan).
- Setup must be honest: each dependency verified, each failure named.

## Progress

- 2026-09-03 09:10 — captions in Go (`tools/captions_render.go`: go-text
  shaping, system font fallback, nine scripts verified on pixels).
- 2026-09-03 09:50 — whisper-large-v3-turbo on the MLX engine
  (`llm/whisper`: mel, encoder, decoder, tokenizer, word timing, the
  long-form loop), each stage diffed against an mlx_whisper oracle; the
  `transcribe` tool and clip_tighten's word timestamps run it in-process and
  the Python whisper scripts are gone. Left in Python: the vision judge
  (mlx-vlm) and YuNet face tracking — the next two ports.
- 2026-09-03 19:00 — the vision tower in Go (`llm/qwen35/vision.go`,
  `mrope.go`, `llm/engine_vision.go`): the loaded Qwen3.5 both thinks and
  sees; `see`, the thumbnail judge and screenshot run on it, mlx-vlm is
  gone. Left in Python: the YuNet reframe — the last port.

