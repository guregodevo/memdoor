### CODE
```python
import sys

def solve():
    data = sys.stdin.read().split()
    if not data:
        return

    n = int(data[0])
    s = data[1]
    t = data[2]

    target = [-1] * 26
    present = [False] * 26

    for cs, ct in zip(s, t):
        a = ord(cs) - ord('a')
        b = ord(ct) - ord('a')
        present[a] = True
        if target[a] == -1:
            target[a] = b
        elif target[a] != b:
            print(-1)
            return

    k = 0
    for a in range(26):
        if present[a] and target[a] != a:
            k += 1

    if k == 0:
        print(0)
        return

    if all(present) and len({target[a] for a in range(26) if present[a]}) == 26:
        print(-1)
        return

    indeg = [0] * 26
    for a in range(26):
        if present[a]:
            indeg[target[a]] += 1

    done = [False] * 26
    bad_cycles = 0

    for start in range(26):
        if not present[start] or target[start] == start or done[start]:
            continue

        path = []
        pos = {}
        cur = start

        while True:
            if cur in pos:
                cycle = path[pos[cur]:]
                isolated = True
                for v in cycle:
                    if indeg[v] != 1:
                        isolated = False
                        break
                if isolated:
                    bad_cycles += 1
                break

            if not present[cur] or target[cur] == cur or done[cur]:
                break

            pos[cur] = len(path)
            path.append(cur)
            cur = target[cur]

        for v in path:
            done[v] = True

    print(k + bad_cycles)

if __name__ == "__main__":
    solve()
```

### NOTES
- **Consistency:** For each letter appearing in S, all its positions must map to the same letter in T. If any S-letter has two different required targets, output -1 immediately.
- **Lower bound k:** Every present letter a with target[a] != a must be the source of at least one operation: the first operation that changes its current class. These first-change operations are distinct for distinct initial letters, so the answer is at least k.
- **Graph model:** Build a functional graph on the 26 lowercase letters with an edge a -> target[a] for every present nonfixed letter a. Edges may point to absent letters or fixed letters; those are sinks. Only directed cycles of length at least 2 matter.
- **Isolated cycle:** A cycle needs one extra operation exactly when no present letter outside the cycle has a direct edge into it. Since each cycle node already has one incoming edge from its cycle predecessor, this is equivalent to indeg[v] == 1 for every v in the cycle, where indeg counts present letters only.
- **Why isolated cycles cost extra:** An isolated cycle has no compatible outside class that can be merged into it. To rotate its labels, one cycle class must be moved to an empty or freed temporary label and later moved back, adding one operation beyond the k mandatory first-source operations. Different isolated cycles cannot share this extra operation because their final target sets are disjoint and incompatible.
- **Buffers:** A temporary label can be an absent letter, or a label freed by processing an acyclic component or a non-isolated cycle. A non-isolated cycle uses its incoming outside node as a buffer and costs no extra operation; nodes feeding into that incoming node can be processed afterward.
- **All-26 injective impossibility:** If k > 0, all 26 letters are present in S, and the target mapping is injective (image size 26), then every occupied label has a distinct final target. Any nontrivial operation would merge two incompatible classes, so no valid first operation exists. Output -1. The identity case is handled earlier by k == 0.
- **Answer formula:** If feasible, the minimum number of operations is k + number_of_isolated_cycles.
- **Implementation complexity:** O(N + 26^2). The graph has at most 26 nodes, so cycle detection by simple path following is trivial.
- **Checked cases:** The four samples produce 4, 0, -1, and 4. S = T gives 0. An all-26 non-identity permutation gives -1. An all-26 non-injective mapping with an isolated cycle gives k + 1. A cycle with an incoming edge gives k. Multiple isolated cycles add one extra each. An absent letter that is also a final target can be used safely by first moving its source letter, freeing the source label as the buffer.

### NEXT
- none

### STATUS
solved
