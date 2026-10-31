### CODE
```python
import sys

def solve() -> None:
    data = sys.stdin.read().split()
    if not data:
        return

    n = int(data[0])
    s = data[1]
    t = data[2]

    present = [False] * 26
    target = [-1] * 26

    for cs, ct in zip(s, t):
        a = ord(cs) - ord('a')
        b = ord(ct) - ord('a')
        present[a] = True
        if target[a] == -1:
            target[a] = b
        elif target[a] != b:
            print(-1)
            return

    mandatory = 0
    indeg = [0] * 26

    for a in range(26):
        if present[a]:
            b = target[a]
            if b == -1:
                print(-1)
                return
            if b != a:
                mandatory += 1
            indeg[b] += 1

    image_count = sum(1 for x in indeg if x > 0)

    visited = [False] * 26
    closed_cycles = 0

    for start in range(26):
        if not present[start] or visited[start]:
            continue

        path = []
        pos = [-1] * 26
        cur = start

        while present[cur] and not visited[cur]:
            if pos[cur] != -1:
                cycle = path[pos[cur]:]
                if len(cycle) >= 2:
                    closed = True
                    for v in cycle:
                        if indeg[v] != 1:
                            closed = False
                            break
                    if closed:
                        closed_cycles += 1
                break

            pos[cur] = len(path)
            path.append(cur)
            cur = target[cur]

        for v in path:
            visited[v] = True

    if closed_cycles > 0 and image_count == 26:
        print(-1)
    else:
        print(mandatory + closed_cycles)

if __name__ == "__main__":
    solve()
```

### NOTES
- **Core reduction:** All occurrences of one original letter always stay together, so each letter appearing in S must map to exactly one letter in T. If any S-letter maps to two different T-letters, the answer is -1.
- **Graph model:** Build a 26-node functional graph with an edge c -> f(c) for every letter c present in S, where f(c) is its required final letter. Letters absent from S have no outgoing edge.
- **Mandatory lower bound:** Every present letter c with f(c) != c must be used as a source at least once to leave its initial label. Let C be the number of such letters.
- **Closed cycles:** A directed cycle of length at least 2 is closed if every vertex on it has indegree exactly 1 from present sources. This means no outside present letter maps into the cycle.
- **Why closed cycles cost extra:** In a closed cycle, the first move involving a cycle letter cannot go to another occupied cycle letter or to an outside occupied letter, because that would merge classes with different final targets. It must go to an empty label. Completing the rotation then requires one additional operation from that empty label. Hence each closed cycle adds exactly 1 operation.
- **Non-closed cycles:** If a cycle has an incoming edge from outside, that outside letter has the same final target as one cycle vertex and can be used as a buffer. The cycle can be resolved using only the mandatory operations, so no extra cost is added.
- **Feasibility condition:** A closed cycle needs a label that can be made empty. Such a label exists iff at least one letter is absent from T, i.e. image_count < 26. If image_count == 26 and there is at least one closed cycle, the answer is -1.
- **Spare label details:** A letter absent from T may be absent from S, in which case it is already empty, or present in S, in which case its own mandatory move can empty it. Fixed present letters are not free spares because they are present in T and must remain in place.
- **Counting cycles:** Use a small depth-first search over present letters. When a node repeats in the current path, extract the cycle. Count it only if its length is at least 2 and every vertex has indegree exactly 1.
- **Answer formula:** If closed_cycles > 0 and image_count == 26, print -1. Otherwise print C + closed_cycles.
- **Edge cases:** Self-loops are not counted as problematic cycles. If S == T, C = 0 and closed_cycles = 0, so the answer is 0. If all 26 letters appear in T and S differs from T, the mapping is a non-identity permutation with a closed cycle, so the answer is -1.
- **Complexity:** The algorithm scans the strings once and then does O(26) graph work, so it is O(N) time and O(1) memory.

### NEXT
none

### STATUS
solved
