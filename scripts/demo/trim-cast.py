#!/usr/bin/env python3
"""Trim a sampled cast: during a wait where only a clock ticks, keep one frame per `every` seconds; cap any gap at `cap` seconds."""
import json, re, sys
src, dst, every, cap = sys.argv[1], sys.argv[2], float(sys.argv[3]), float(sys.argv[4])
ansi = re.compile(r"\x1b\[[0-9;]*m")
def shape(s):  # the screen with its clocks blanked, to tell a tick from a change
    return re.sub(r"\b\d+(m\d+)?s\b", "Ns", ansi.sub("", s))
lines = open(src).read().splitlines()
head = lines[0]
frames = [json.loads(l) for l in lines[1:] if l.startswith("[")]
kept, last_shape, last_at = [], None, -1e9
for at, kind, data in frames:
    sh = shape(data)
    if sh == last_shape and at - last_at < every:
        continue
    kept.append([at, kind, data]); last_shape, last_at = sh, at
out, t, prev = [], 0.0, kept[0][0] if kept else 0.0
for at, kind, data in kept:
    t += min(at - prev, cap); prev = at
    out.append([round(t, 3), kind, data])
with open(dst, "w") as f:
    f.write(head + "\n")
    for fr in out:
        f.write(json.dumps(fr) + "\n")
print(f"{len(frames)} frames, {frames[-1][0]:.0f}s  ->  {len(out)} frames, {out[-1][0]:.0f}s")
