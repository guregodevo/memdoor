### CODE
```python
import sys


def solve() -> None:
    data = sys.stdin.buffer.read().split()
    if not data:
        return

    s = data[1]
    t = data[2]

    appears = [False] * 26
    req = [-1] * 26

    # Consistency: every original letter must have one fixed target letter.
    for cs, ct in zip(s, t):
        a = cs - 97
        b = ct - 97
        if not appears[a]:
            appears[a] = True
            req[a] = b
        elif req[a] != b:
            print(-1)
            return

    changed_count = 0
    out = [-1] * 26

    # Functional graph on appearing letters that actually need to move.
    for i in range(26):
        if appears[i] and req[i] != i:
            changed_count += 1
            out[i] = req[i]

    # Find all directed cycles of length at least 2.
    state = [0] * 26  # 0 = unvisited, 1 = visiting, 2 = done
    pos = [-1] * 26
    cycles = []

    for i in range(26):
        if out[i] == -1 or state[i] != 0:
            continue

        stack = []
        cur = i
        while cur != -1 and state[cur] == 0:
            state[cur] = 1
            pos[cur] = len(stack)
            stack.append(cur)
            cur = out[cur]

        if cur != -1 and state[cur] == 1:
            cycles.append(stack[pos[cur]:])

        for v in stack:
            state[v] = 2

    in_closed_cycle = [False] * 26
    closed_cycle_count = 0

    # A cycle is closed if no changed appearing letter outside it points into it.
    for cyc in cycles:
        cyc_set = set(cyc)
        closed = True

        for u in range(26):
            if out[u] != -1 and out[u] in cyc_set and u not in cyc_set:
                closed = False
                break

        if closed:
            closed_cycle_count += 1
            for v in cyc:
                in_closed_cycle[v] = True

    if closed_cycle_count > 0:
        has_buffer = False

        # An absent letter is an initially empty buffer.
        for i in range(26):
            if not appears[i]:
                has_buffer = True
                break

        # Otherwise, any changed letter outside closed cycles can be freed
        # by resolving its own non-closed component.
        if not has_buffer:
            for i in range(26):
                if appears[i] and req[i] != i and not in_closed_cycle[i]:
                    has_buffer = True
                    break

        if not has_buffer:
            print(-1)
        else:
            print(changed_count + closed_cycle_count)
    else:
        print(changed_count)


if __name__ == "__main__":
    solve()
```

### NOTES
- **Invariant:** For every letter c appearing in S, all positions with S[i]=c must have the same T[i]; otherwise the pile of c would have to split, so print -1. Store req[c] as that target.
- **Token model:** Treat each current letter as a position holding a label equal to the final letter required by the original pile(s) there. A move x->y is valid only if y is empty or has the same label as x. This model is exact because piles with different final labels can never be merged.
- **Base count:** Let changed_count be the number of appearing letters c with req[c] != c. Each such original letter must be the source of its first move, so at least changed_count operations are needed. All acyclic components and open cycles can be resolved with exactly that many moves by processing from sinks/cycle breakers outward.
- **Graph:** Build out[c] = req[c] only for appearing changed letters. Non-appearing letters and fixed letters are sinks. Find directed cycles of length at least 2 in this 26-node functional graph. Ignore self-loops.
- **Closed cycles:** A cycle is closed if no changed appearing letter outside the cycle has an edge into any node of the cycle. Such a cycle has no duplicate label entering it, so its labels are unique and it cannot be rotated without an empty buffer. Each closed cycle adds exactly one extra operation.
- **Open cycles:** If a cycle has an incoming edge from outside, the outside node has the same final label as one cycle node. That duplicate label breaks the cyclic precedence, so the whole component can be resolved with only the mandatory changed_count moves.
- **Buffer availability:** If there is at least one closed cycle, an empty position is needed. It exists if some letter does not appear in S, or if some changed appearing letter is not inside a closed cycle. In the latter case, resolving that non-closed component with its mandatory moves leaves a leaf position empty, and that empty position can be reused for every closed cycle.
- **Impossible case:** If closed cycles exist, all 26 letters appear in S, and every changed letter lies in a closed cycle, then every position is occupied by a unique label and no valid move is possible; print -1. Fixed letters cannot serve as buffers because their labels do not match any cycle label and they cannot be moved without extra operations.
- **Implementation notes:** Scan S and T once for consistency. Use state 0/1/2 traversal on 26 nodes to collect cycles. For each cycle, scan all 26 nodes to see if an outside edge enters it. Count closed cycles. Check buffer as above. Output changed_count + closed_count, or -1 in the impossible case.
- **Pitfalls:** Do not count self-loops as cycles. Do not treat fixed appearing letters as buffers. Do not require a distinct buffer per closed cycle; one empty letter is reused. Edges from non-appearing letters do not exist, so they do not make a cycle open, but absent letters do provide buffers.

### NEXT
- none

### STATUS
solved
