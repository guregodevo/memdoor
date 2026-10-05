# ADR 0018 — Hue says what a thing is; luminance says how much it matters

Status: ACCEPTED (2026-10-01, shipped in commit `18a4d3a`) · Affects
`cmd/tui/ui/*`, enforced by `cmd/tui/ui/palette_test.go`

## Context

The TUI palette grew eight grey steps (235, 236, 238, 240, 241, 242, 245,
252) doing three unrelated jobs — text, structure, and surfaces — with no
rule saying which step could do which job. Three of them sat at contrast no
text survives: 238 at 1.7:1, 240 at 2.3:1, 241 at 2.7:1 against a dark
terminal, and 240 was carrying a tool's own prose. Worse, row selection
routed a highlight through a hue token (`colRead` slate under bright text):
1.55:1 — a row you could see and could not read.

## Decision

1. **Hue says what kind; luminance says how important.** A colour's hue
   (read-blue, error-red, ok-green, plan-blue) answers "what is this line";
   only its lightness answers "how much attention it deserves". Never route
   a hierarchy (primary vs secondary vs structure) through hue.

2. **Four luminance levels, each with the job it can do** (see the palette
   block in `cmd/tui/ui/model_view.go`):
   - `colText` (252, 10.8:1) — what you are meant to read
   - `colDim` (245, 4.8:1) — secondary text, WCAG AA
   - `colFaint` (242, 3.2:1) — ancillary marks: line numbers, struck-out
     items
   - `colRule` (238, 1.7:1) — STRUCTURE ONLY: gutters, borders,
     separators. Never text.
   `colFill` (236) is the one tinted surface (diff bands, the todos bar,
   code blocks).

3. **Selection is luminance, not hue.** A selected row is `colSelFG`
   (`colText`) on `colSelBG` (`colRule`), 6.3:1 on the fill, the fill 1.7:1
   off the page; the → cursor beside the row carries the accent. All
   selection surfaces (slash dropdown, MCP panel, route picker, page strip,
   model picker) wear the same pair.

4. **Contrast is measured, not trusted.** `palette_test.go` computes WCAG
   contrast and fails the build: every text colour has a floor against the
   page; `colRule` has a ceiling (structure must not reach text contrast);
   the selected row must be legible on its fill and visible off the page;
   success and failure must differ in luminance as well as hue (colour-blind
   safety).

## Consequences

- New TUI colour choices go through the palette constants in
  `cmd/tui/ui/model_view.go`; raw `lipgloss.Color("NNN")` greys are banned
  (the only remaining raw codes are the hue tokens 245/252/111 and
  diff colours, which are semantic).
- A colour change that breaks a contrast floor is caught by
  `go test ./cmd/tui/ui/ -run TestThePalette`, not by an eye.

## What would falsify this

- A terminal theme or colour profile where the measured contrast of the
  four levels against the rendered page diverges from the 256-colour math
  in `palette_test.go` badly enough that `colFaint` text becomes unreadable
  in practice.
