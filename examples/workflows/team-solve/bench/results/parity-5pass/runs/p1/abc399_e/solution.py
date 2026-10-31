import sys
from collections import deque

def solve():
    data = sys.stdin.read().split()
    if not data:
        return

    N = int(data[0])
    S = data[1]
    T = data[2]

    target = [-1] * 26
    present = [False] * 26

    for s, t in zip(S, T):
        si = ord(s) - ord('a')
        ti = ord(t) - ord('a')
        present[si] = True

        if target[si] == -1:
            target[si] = ti
        elif target[si] != ti:
            print(-1)
            return

    present_count = sum(present)

    bad_count = 0
    color_freq = [0] * 26
    indeg = [0] * 26
    out = [-1] * 26

    for c in range(26):
        if not present[c]:
            continue

        t = target[c]
        color_freq[t] += 1

        if t != c:
            bad_count += 1
            if present[t]:
                out[c] = t
                indeg[t] += 1

    spare = present_count < 26 or any(f >= 2 for f in color_freq)

    orig_indeg = indeg[:]

    q = deque([i for i in range(26) if indeg[i] == 0])
    while q:
        u = q.popleft()
        v = out[u]
        if v != -1:
            indeg[v] -= 1
            if indeg[v] == 0:
                q.append(v)

    visited = [False] * 26
    bare_cycles = 0

    for i in range(26):
        if indeg[i] > 0 and not visited[i]:
            cur = i
            cycle = []

            while not visited[cur]:
                visited[cur] = True
                cycle.append(cur)
                cur = out[cur]
                if cur == -1:
                    break

            if cur != -1 and all(orig_indeg[v] == 1 for v in cycle):
                bare_cycles += 1

    if bare_cycles > 0 and not spare:
        print(-1)
    else:
        print(bad_count + bare_cycles)

if __name__ == "__main__":
    solve()