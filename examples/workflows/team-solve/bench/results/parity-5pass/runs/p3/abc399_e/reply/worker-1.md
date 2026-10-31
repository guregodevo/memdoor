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

    present = [False] * 26
    target = [-1] * 26

    for cs, ct in zip(s, t):
        c = ord(cs) - ord('a')
        d = ord(ct) - ord('a')
        present[c] = True
        if target[c] == -1:
            target[c] = d
        elif target[c] != d:
            print(-1)
            return

    changed = [False] * 26
    changed_count = 0
    for c in range(26):
        if present[c] and target[c] != c:
            changed[c] = True
            changed_count += 1

    if changed_count == 0:
        print(0)
        return

    free_exists = any(not present[c] for c in range(26))

    in_cycle = [False] * 26
    for c in range(26):
        if not changed[c]:
            continue
        cur = c
        for _ in range(26):
            if not changed[cur]:
                break
            cur = target[cur]
            if cur == c:
                in_cycle[c] = True
                break

    cycle_id = [-1] * 26
    cycle_count = 0
    for i in range(26):
        if in_cycle[i] and cycle_id[i] == -1:
            cid = cycle_count
            cur = i
            while cycle_id[cur] == -1:
                cycle_id[cur] = cid
                cur = target[cur]
            cycle_count += 1

    has_tree = [False] * cycle_count
    tree_exists = False

    for u in range(26):
        if changed[u] and not in_cycle[u]:
            tree_exists = True
            v = target[u]
            if in_cycle[v]:
                has_tree[cycle_id[v]] = True

    isolated_cycles = sum(1 for h in has_tree if not h)

    if not (free_exists or tree_exists):
        print(-1)
    else:
        print(changed_count + isolated_cycles)

if __name__ == "__main__":
    solve()
```

### NOTES
- **Core invariant:** Positions that start with the same letter always remain together, because every operation replaces all occurrences of a letter uniformly. Therefore, for each letter c appearing in S, all corresponding T characters must be identical; otherwise the answer is -1.
- **Required mapping:** For each present letter c, define f(c) as the unique T character required at positions where S has c. If f(c) != c, c must be changed. Build a directed edge c -> f(c) only for present c with f(c) != c.
- **Base lower bound:** Every changed letter c must be chosen as x at least once, because its original occurrences start as c and must end as f(c) != c. Thus the answer is at least changed_count.
- **Functional graph structure:** The graph has outdegree at most one on changed letters. Components are either acyclic trees ending at a fixed/absent letter, or directed cycles with trees feeding into them.
- **Why cycles need help:** A directed cycle of length at least two cannot be resolved using only its own letters. Any operation x -> y with both x and y inside the cycle merges two groups that must end at different letters, which is irreversible.
- **Tree-fed cycles:** If a non-cycle changed vertex has a path into a cycle, equivalently some non-cycle changed u has target[u] inside that cycle, then that cycle can be resolved without an extra operation. The mandatory operation of the tree vertex can serve as the temporary return into the cycle.
- **Isolated cycles:** A cycle with no incoming non-cycle changed vertex needs one extra operation if an empty temporary label is available. An empty label exists initially if some letter is absent from S, or can be created by processing any non-cycle changed component. Each isolated cycle adds exactly 1 to the answer.
- **Impossibility case:** If no letter is absent from S and there is no non-cycle changed vertex while changed_count > 0, then all 26 letters are present and the required mapping is a non-identity permutation. Every nontrivial replacement operation is non-injective on the 26-letter alphabet, so any nonempty sequence is non-injective and cannot realize a permutation. Print -1.
- **Final formula:** If changed_count == 0, print 0. Otherwise, if neither free_exists nor tree_exists, print -1. Otherwise print changed_count + isolated_cycles.
- **Implementation details:** The alphabet size is only 26, so cycle detection can be done by following up to 26 steps from each changed letter and checking whether it returns to itself. Cycle ids are assigned by walking unassigned in-cycle nodes. A cycle is marked as tree-fed if any non-cycle changed vertex points directly to a node in that cycle; direct checking is sufficient because any path into a cycle has a final edge from a non-cycle node.
- **Edge cases:** Absent letters that appear in T can still be used as temporary labels if scheduled before the operation that finally maps some letter to them. Fixed present letters cannot be used as free temporaries without extra cost, so they do not avoid the permutation impossibility. Multiple isolated cycles each require their own extra operation, even if one empty label is reused.
- **Complexity:** The consistency check is O(N), and all graph processing is O(26), so total time is O(N) and memory is O(1) beyond the input strings.

### NEXT
- none

### STATUS
solved
