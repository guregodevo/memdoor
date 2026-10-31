### CODE
```python
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
```

### NOTES
- **Model:** Positions with the same initial letter always stay together. A valid sequence never merges boxes with different final targets, because that would make them end at the same letter. Treat each present source letter as a box whose home is its target letter.
- **Feasibility:** For every source letter in S, all corresponding T letters must be identical; otherwise print -1.
- **Base cost:** If source c is present and target[c] != c, the first change of those positions must use source c. Hence at least bad_count operations are necessary, and this bound is achievable except for bare cycles.
- **Graph:** Add edge c -> target[c] for present c with target[c] != c and target[c] present. Fixed letters and absent targets are sinks. Nontrivial cycles in this graph are the only obstruction needing extra moves.
- **Bare cycle:** A cycle is bare when no outside present source maps into it, equivalently every vertex has original indegree 1. If a cycle has an outside incoming edge u -> v, the predecessor of v can move into u (same final target), and u's required move carries it to v, so no extra operation is needed.
- **Buffer/spare:** A bare cycle needs one extra operation to rotate it. This is possible iff there is an empty label (present_count < 26) or a duplicate final target (some color_freq[t] >= 2). A duplicate target lets two same-home boxes merge, freeing a label; the freed label can be reused for every bare cycle.
- **Answer:** If feasible, minimum = bad_count + bare_cycles. If bare_cycles > 0 and no spare, print -1.
- **Implementation details:** Use 26 arrays. Build target and present while checking conflicts. color_freq counts distinct source letters per target. indeg counts bad edges to present targets. Save original indeg before Kahn. Kahn removes indegree-zero nodes; remaining indeg > 0 nodes are exactly cycle nodes. Traverse each remaining cycle and count it bare iff all its vertices have original indeg 1.
- **Edge cases:** Ignore fixed self-loops. Absent targets are sinks, not cycles. A duplicate target may involve a fixed letter. After Kahn, cycle indegrees become 1 even for non-bare cycles, so bare detection must use original indegrees. One buffer can serve multiple bare cycles. If all 26 letters are present and no target is duplicated, the mapping is a permutation and any nontrivial cycle is impossible.
- **Samples:** Sample 1 has four nonfixed letters and no cycles, answer 4. Sample 2 is identity, answer 0. Sample 3 has a conflict for a, answer -1. Sample 4 is a bare 3-cycle with an empty letter available, answer 3 + 1 = 4.

### NEXT
- none

### STATUS
solved
