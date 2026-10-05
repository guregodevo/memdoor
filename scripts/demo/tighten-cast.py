#!/usr/bin/env python3
"""Tighten a cast to about TARGET seconds: the typing at the start stays near
real speed, the run after it becomes an even timelapse, the last frame holds.

  tighten-cast.py in.cast out.cast [target_seconds=35] [typing_seconds=12]

(2026-10-04, "the videos are too long": the takes ran 1-3 minutes.)"""
import json, sys

src, dst = sys.argv[1], sys.argv[2]
target = float(sys.argv[3]) if len(sys.argv) > 3 else 35.0
typing = float(sys.argv[4]) if len(sys.argv) > 4 else 12.0
STEP, HOLD = 0.4, 3.0

lines = open(src).read().splitlines()
head, frames = json.loads(lines[0]), [json.loads(l) for l in lines[1:] if l.strip()]
frames = [f for f in frames if f[2]]  # drop empty spacers
t0 = frames[0][0]
a = [f for f in frames if f[0] - t0 <= typing]
b = [f for f in frames if f[0] - t0 > typing]

out, clock, prev = [], 0.0, None
for f in a:  # typing: real order, gaps capped
    if prev is not None:
        clock += min(f[0] - prev, 0.15)
    out.append([round(clock, 3), "o", f[2]]); prev = f[0]

if b:
    budget = max(target - clock - HOLD, 6.0)
    n = max(int(budget / STEP), 2)
    span = b[-1][0] - b[0][0]
    keep, last_i = [], -1
    for k in range(n):
        want = b[0][0] + span * k / (n - 1)
        i = min(range(len(b)), key=lambda j: abs(b[j][0] - want))
        if i != last_i:
            keep.append(b[i]); last_i = i
    if keep[-1] is not b[-1]:
        keep.append(b[-1])
    for f in keep:
        clock += STEP
        out.append([round(clock, 3), "o", f[2]])

out.append([round(clock + HOLD, 3), "o", ""])
with open(dst, "w") as fh:
    fh.write(json.dumps(head) + "\n")
    for f in out:
        fh.write(json.dumps(f) + "\n")
print(f"{src}: {frames[-1][0]-t0:.0f}s -> {out[-1][0]:.0f}s, {len(out)} frames")
