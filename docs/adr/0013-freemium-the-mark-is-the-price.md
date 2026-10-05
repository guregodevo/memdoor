# ADR-0013 — Freemium: the mark is the price

Status: SUPERSEDED (2026-10-03). The clipper moved out of Memdoor, so no
film is counted and nothing is marked. Greg: "refuse free leases and remove
the film counter". A free workspace is never leased the hosted model and the
broker refuses its chat; free runs on the person's own key.

Status when accepted: ACCEPTED (2026-09-19). Greg: "shouldn't it be free when the
background is true?" → "freemium?" → **"10 is good, build it."** Amends
ADR-0008 (free = local only, watermarked) and follows ADR-0012 (the API
brain is the only brain).

## Context

The go-to-market is a channel of shorts cut with Memdoor, each carrying
memdoor.ai (2026-09-19). CapCut grew the same way: for years every free
export carried its name, and the watermark was the marketing budget. Free
users who post marked shorts are placements we did not pay for.

ADR-0008 made the free tier local-only because a rented GPU cost dollars an
hour whether or not anyone used it. On the API brain a finished short costs
a few tenths of a cent (two days of heavy use, every film and all the
testing: $0.90). Giving the good brain to the free tier is affordable; the
cap is what keeps it so.

## Decision

- **Free** gets the good brain for **ten finished shorts a month**, every
  one carrying the mark, which reads as the address: memdoor.ai.
- **A seat ($149/mo)** is unlimited and unmarked. Nothing else differs.
- A finished short is a captioned clip or a stitched film (`tools.FilmMade`),
  never a cut, a tighten or a conversion on the way there. The gateway
  counts them and tells the broker on its next ask or beat; the broker
  keeps the month's count per workspace (`films` in the registry, UTC
  months) and answers `local` with reason `free cap` past ten. The window
  shows "free · 7 of 10 shorts left this month" and, past the cap, the seat
  button under a sentence about the ten — never "no brain here".
- The channel's own shorts pass `brand: true`, so the mark stays on a paid
  seat by choice.

## Consequences

The free tier now costs money — about a dollar a month for a user who makes
all ten — and returns ten marked shorts. That is the trade. A scraper farm
is bounded by the cap per workspace; a workspace is a sign-in, so the bound
is per account. The landing page and the seat gate must say the new tier
(open: the landing still reads "private testing").
