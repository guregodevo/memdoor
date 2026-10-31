### CODE
```python
import sys

MOD = 998244353


def find(x, parent, diff):
    # Returns the root.  After the call, diff[x] is the xor from x to its root.
    if parent[x] == x:
        return x

    r = x
    acc = 0
    while parent[r] != r:
        acc ^= diff[r]
        r = parent[r]

    y = x
    while parent[y] != y:
        p = parent[y]
        d = diff[y]
        parent[y] = r
        diff[y] = acc
        acc ^= d
        y = p

    return r


def solve():
    data = sys.stdin.buffer.read().split()
    if not data:
        return

    t = int(data[0])

    # First pass: find the maximum number of DSU nodes needed.
    idx = 1
    max_n = 0
    for _ in range(t):
        H = int(data[idx])
        W = int(data[idx + 1])
        idx += 2
        n = H + W
        if n > max_n:
            max_n = n
        idx += H

    # Reusable weighted XOR DSU arrays.
    parent = list(range(max_n))
    size = [1] * max_n
    diff = [0] * max_n

    par = parent
    sz = size
    dif = diff
    find_func = find

    idx = 1
    out = []
    A = 65  # ord('A')

    for _ in range(t):
        H = int(data[idx])
        W = int(data[idx + 1])
        idx += 2
        n = H + W

        # Reset only the nodes used in this case.
        for i in range(n):
            par[i] = i
            sz[i] = 1
            dif[i] = 0

        col = [0] * W
        col_odd = 0
        comps = n
        bad = False

        for i in range(H):
            s = data[idx]
            idx += 1

            if bad:
                continue

            row_pref = 0

            for j, ch in enumerate(s):
                if ch == A:
                    c = col[j]
                    col[j] = c ^ 1
                    if c:
                        col_odd -= 1
                    else:
                        col_odd += 1
                    row_pref ^= 1
                else:
                    # B tile:
                    # row_bit[i] xor col_bit[j] =
                    # 1 xor parity_of_A_to_left xor parity_of_A_above
                    w = 1 ^ row_pref ^ col[j]

                    ra = find_func(i, par, dif)
                    xa = dif[i]

                    rb = find_func(H + j, par, dif)
                    xb = dif[H + j]

                    if ra == rb:
                        if (xa ^ xb) != w:
                            bad = True
                            break
                    else:
                        val = xa ^ xb ^ w
                        if sz[ra] < sz[rb]:
                            par[ra] = rb
                            dif[ra] = val
                            sz[rb] += sz[ra]
                        else:
                            par[rb] = ra
                            dif[rb] = val
                            sz[ra] += sz[rb]
                        comps -= 1

            if bad:
                continue

            if row_pref:
                bad = True

        if bad or col_odd:
            out.append("0")
        else:
            out.append(str(pow(2, comps, MOD)))

    sys.stdout.write("\n".join(out))


if __name__ == "__main__":
    solve()
```

### NOTES
- **Model:** Put a binary variable on every shared edge midpoint; 1 means a line endpoint is present. The no-dead-end condition means the two cells sharing a midpoint must use the same value.
- **Local constraints:** For an A tile, left/right differ and top/bottom differ. For a B tile, left=right, top=bottom, and left xor top = 1.
- **Row/column reduction:** In row i, horizontal equations are h_j xor h_{j+1} = 1 iff the cell is A. Around the torus this is consistent iff the row has an even number of A tiles. Then h_{i,j} = R_i xor parity of A tiles strictly to the left of j.
- **Vertical reduction:** Similarly, v_{i,j} = C_j xor parity of A tiles strictly above i, and each column must have an even number of A tiles.
- **B equations:** Substituting the row/column parameterization into left xor top = 1 gives R_i xor C_j = 1 xor left_prefix xor above_prefix. A cells add no extra cross equation.
- **Counting:** Build a weighted XOR graph with H row nodes and W column nodes. Column node j is indexed as H+j, not W+j, to avoid overlap with row nodes. Add one edge per B cell with the weight above. If any row/column A parity is odd or the DSU detects inconsistency, answer is 0. Otherwise each connected component contributes one free bit, so answer is 2^{components} modulo 998244353.
- **Implementation:** A first pass over the token list finds max(H+W) so DSU arrays can be reused and reset only for the current case. col_odd tracks the number of columns with odd A parity. If a case becomes impossible, remaining rows are still consumed but not processed.
- **DSU details:** diff[x] stores value[x] xor value[parent[x]]. find returns the root and leaves diff[x] as the xor to the root. Union by size is used. For an equation value[a] xor value[b] = w, the root relation is value[ra] xor value[rb] = xa xor xb xor w.
- **Complexity:** O(total HW alpha) time and O(max(H+W) + input) memory, well within the 1e6 total cell constraint.
- **Sample:** Hand-checked sample outputs are 2, 0, 2, matching the statement.

### NEXT
- none

### STATUS
solved
