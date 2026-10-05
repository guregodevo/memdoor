#!/usr/bin/env python3
"""Record a tmux pane as an asciinema v2 cast: the rendered screen (colours kept), sampled, until a marker shows or time runs out."""
import json, subprocess, sys, time
sess, out, cols, rows, until, limit = sys.argv[1], sys.argv[2], int(sys.argv[3]), int(sys.argv[4]), sys.argv[5], float(sys.argv[6])
f = open(out, "w")
start = time.time()
f.write(json.dumps({"version": 2, "width": cols, "height": rows, "timestamp": int(start), "title": "memdoor: a workflow from a sentence"}) + "\n")
last = None
def grab(ansi):
    return subprocess.run(["tmux", "capture-pane", "-t", sess, "-p"] + (["-e"] if ansi else []), capture_output=True, text=True).stdout
while time.time() - start < limit:
    scr = grab(True)
    if scr != last:
        f.write(json.dumps([round(time.time() - start, 3), "o", "\x1b[H\x1b[2J" + scr.replace("\n", "\r\n")]) + "\n"); f.flush()
        last = scr
        if until and until in grab(False):
            time.sleep(2.5)
            f.write(json.dumps([round(time.time() - start, 3), "o", "\x1b[H\x1b[2J" + grab(True).replace("\n", "\r\n")]) + "\n")
            break
    time.sleep(0.25)
f.close()
print("recorded %.0fs" % (time.time() - start))
