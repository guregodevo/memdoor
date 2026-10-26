#!/usr/bin/env python3
"""Trim a sampled cast: during a wait where only a clock ticks, keep one frame per `every` seconds; cap any gap at `cap` seconds;
an optional fifth argument `rate` keeps at most one frame per `rate` seconds even while the screen changes (a long run whose
tool frames stream: team-solve, 512 s, 4.4 MB at 4 frames/s)."""
import json, re, sys
src, dst, every, cap = sys.argv[1], sys.argv[2], float(sys.argv[3]), float(sys.argv[4])
rate = float(sys.argv[5]) if len(sys.argv) > 5 else 0.0
ansi = re.compile(r"\x1b\[[0-9;]*m")
def shape(s):  # the screen with its clocks and spinners blanked, to tell a tick from a change
    s = re.sub(r"[\u2800-\u28ff\u2588\u258c\u258f]", "", ansi.sub("", s))  # the braille spinner and the cursor block (a peer turn spun for five minutes, 2026-10-26)
    s = "\n".join(l.rstrip() for l in s.split("\r\n"))  # a blinking cursor leaves a trailing space behind
    s = re.sub(r"[A-Za-z]+… \(", "…(", s)  # the status verb the window rotates while it waits (Noodling…, Spelunking…)
    return re.sub(r"\b\d+(m\d+)?s\b", "Ns", s)
lines = open(src).read().splitlines()
head = lines[0]
frames = [json.loads(l) for l in lines[1:] if l.startswith("[")]
kept, last_shape, last_at = [], None, -1e9
for at, kind, data in frames:
    sh = shape(data)
    if sh == last_shape and at - last_at < every:
        continue
    if rate and at - last_at < rate and at != frames[-1][0]:
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
