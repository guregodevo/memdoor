# ADR-0009 — The app is composition: a window around the TUI, with no logic in it

> **Superseded 2026-09-27.** The Mac app was removed (`app/macos`, `scripts/app`): Memdoor is command line only, installed with `install.sh`.

Status: ACCEPTED (2026-09-05). Greg, in one sitting: "app and tui local have
same behaviour… there are just front end with same backend… so do not
diverge… it's a thin layer without logic. App embeds the TUI… it's
composition."

## Context

The creator app (ADR-0005's TUI, in a native window) exists because Sara does
not use a terminal. The cheap way to build it was to embed what already
works: a SwiftUI window hosting libghostty, running `memdoor tui --app`
against the same gateway (see the Mac app section of docs/roadmap/MUST.md).

The expensive way is the one that creeps in by itself. Every `if appMode` is
a second product to keep true, and each one hides the real question behind a
proxy for the window. Two of those were written and removed the same night:
the footer hid the model "because it's the app" when the rule Greg had given
was "only admin can see it" — a fact about the PERSON — and a proposal to
give the app its own session model was refused twice.

## Decision

**The Swift layer is a window, not a product.** It may own only what a
terminal cannot do for itself: the surface (keys, mouse, pixels), the drop
zone, the clips pane, the pasteboard, and starting the gateway process. It
decides nothing.

**Every rule lives behind the seam, where the terminal gets it for free:**

- who may see what → the API answers it (an admin's gateway returns the
  model, the rate and the balance; a creator's does not);
- what a plan allows → the broker's lease answer carries it (state,
  watermark, seats — ADR-0007, ADR-0008);
- what a tool does, what an agent is → the gateway and the skills.

**The test before writing Swift:** would this change also be true in
`memdoor tui`? If not, it is in the wrong file.

**What legitimately differs** is presentation the terminal cannot do (a
native pane, a drop target) and the two product choices app mode already
carries: the creator's welcome, and the clipper answering instead of the
coder. A new session opens a new channel in both — settled twice; continuity
across sessions comes from the agent's memory and the project folder, never
from the app having its own session model.

## Consequences

- A bug in the window is a bug in the window: Escape not reaching the
  terminal (2026-09-05) was a Swift fix, because the terminal already knew
  what to do with the key and never received it.
- The app updates by shipping a new bundle around the same CLI; the rules
  it obeys ship with the gateway (docs/roadmap/MUST.md: updates must be
  easy, and the App Store is out — its sandbox forbids the bash tool that
  the product IS).
- If the Swift layer starts growing rules, the fix is to move them down,
  not to mirror them into the TUI. Today it is ~1,065 lines, half of that
  the libghostty surface.
