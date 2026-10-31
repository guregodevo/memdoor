### CODE
```python
import sys


def solve() -> None:
    data = sys.stdin.buffer.read().split()
    if len(data) < 3:
        return

    s = data[1]
    t = data[2]

    appears_s = [False] * 26
    appears_t = [False] * 26
    req = [-1] * 26

    # Every original letter must have one consistent target letter.
    for cs, ct in zip(s, t):
        a = cs - 97
        b = ct - 97
        appears_s[a] = True
        appears_t[b] = True

        if req[a] == -1:
            req[a] = b
        elif req[a] != b:
            print(-1)
            return

    changed_count = 0
    out = [-1] * 26

    # Functional graph on letters that appear in S and actually need to change.
    for i in range(26):
        if appears_s[i] and req[i] != i:
            changed_count += 1
            out[i] = req[i]

    # Count every directed cycle of length at least 2.
    state = [0] * 26  # 0 = unvisited, 1 = visiting, 2 = done
    pos = [-1] * 26
    cycle_count = 0

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
            # Self-loops are impossible here because changed letters have req[i] != i.
            cycle_count += 1

        for v in stack:
            state[v] = 2

    # A buffer exists iff not both S and T use all 26 letters.
    has_buffer = (sum(appears_s) < 26) or (sum(appears_t) < 26)

    if cycle_count > 0 and not has_buffer:
        print(-1)
    else:
        print(changed_count + cycle_count)


if __name__ == "__main__":
    solve()
```

### NOTES
- **Consistency:** For each letter c appearing in S, all positions with S[i] = c must have the same T[i]. If not, the pile of c would have to split, so output -1. Store this target as req[c].
- **Changed letters:** A letter is changed if it appears in S and req[c] != c. Each changed original letter must be used as a source at least once, giving the base lower bound changed_count.
- **Functional graph:** Build out[c] = req[c] only for changed appearing letters; all other letters have out[c] = -1. Edges into fixed letters or absent letters are sinks. Self-loops are excluded because changed letters satisfy req[c] != c.
- **Cycle counting:** Count every directed cycle of length at least 2 in this 26-node functional graph. Tails feeding into a cycle do not create additional cycles, but the cycle itself still counts.
- **Buffer condition:** A temporary buffer is available iff some letter is absent from S or some letter appears in S but not in T. Equivalently, not both S and T use all 26 letters. In code this is sum(appears_s) < 26 or sum(appears_t) < 26.
- **Why buffer matters:** If S and T both use all 26 letters, the number of distinct letters is 26 initially and finally. An operation x -> y never increases the number of distinct letters; if y is already present it decreases it, and no absent y exists. Thus any positive operation would make the final distinct count impossible.
- **Answer formula:** If there are no cycles, the answer is changed_count. If cycles exist and a buffer exists, the answer is changed_count + cycle_count. If cycles exist and no buffer exists, output -1.
- **Why each cycle adds one:** A cycle of k changed letters cannot be rotated using only the k mandatory first moves. It needs one extra move through a buffer. One buffer can be reused for all cycles, so add one per cycle, not one per cycle node.
- **Open cycles:** Cycles with incoming tails are still counted. Incoming tails may help create or use a buffer, but they do not remove the need for one extra operation for the cycle itself.
- **Buffer creation when S uses all 26:** If T does not use all 26, some S-only letter is not required in the final string. It can be made empty using mandatory moves, then reused as a buffer. It does not need to be absent from T initially.
- **Implementation:** Read input as bytes. Scan S and T once to build req, appears_s, and appears_t. Use a 0/1/2 traversal on 26 nodes to count cycles. A back edge to a visiting node counts one cycle. Then apply the buffer condition and print the result.
- **Pitfalls:** Do not count self-loops. Do not use the old closed-cycle-only rule; all cycles count. Do not count positions; count letters. Do not treat fixed letters as free buffers unless they become empty through mandatory moves. If inconsistency is found, return -1 before graph processing.

### NEXT
- none

### STATUS
solved
