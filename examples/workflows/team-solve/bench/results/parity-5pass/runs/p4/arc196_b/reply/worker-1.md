### CODE
```python
import sys

MOD = 998244353
A_INT = 65
A_BYTE = b'A'


def find(par, dif, x):
    if par[x] < 0:
        return x, 0

    root = x
    total = 0
    while par[root] >= 0:
        total ^= dif[root]
        root = par[root]

    res = total
    while par[x] >= 0:
        p = par[x]
        d = dif[x]
        par[x] = root
        dif[x] = total
        total ^= d
        x = p

    return root, res


def solve():
    data = sys.stdin.buffer.read().split()
    if not data:
        return

    t = int(data[0])
    idx = 1
    out = []
    append = out.append

    f = find
    mod = MOD
    a_int = A_INT
    a_byte = A_BYTE

    for _ in range(t):
        H = int(data[idx])
        W = int(data[idx + 1])
        idx += 2

        start = idx
        idx += H

        col_par = [0] * W
        cp = col_par
        b_count = 0
        bad = False

        # First pass: row parity, column parity, number of B tiles.
        for r in range(H):
            row = data[start + r]
            cnt_a = row.count(a_byte)

            if cnt_a & 1:
                bad = True
                break

            b_count += W - cnt_a

            if cnt_a:
                for j, c in enumerate(row):
                    if c == a_int:
                        cp[j] ^= 1

        if bad:
            append('0')
            continue

        total_cells = H * W

        if b_count == total_cells:
            append('2')
            continue

        if any(col_par):
            append('0')
            continue

        if b_count == 0:
            append(str(pow(2, H + W, mod)))
            continue

        # Parity union-find on H row nodes and W column nodes.
        n = H + W
        parent = [-1] * n
        diff = [0] * n
        comps = n

        par = parent
        dif = diff
        cp = col_par  # already all zero, reuse as column prefix array
        H_off = H
        bad = False

        # Second pass: add B-cell xor constraints.
        for i in range(H):
            row = data[start + i]
            row_pref = 0
            u = i
            ru = -1
            xu = 0

            for j, c in enumerate(row):
                if c == a_int:
                    row_pref ^= 1
                    cp[j] ^= 1
                else:
                    w = 1 ^ row_pref ^ cp[j]

                    if ru < 0:
                        ru, xu = f(par, dif, u)

                    v = H_off + j
                    rv, xv = f(par, dif, v)

                    if ru == rv:
                        if (xu ^ xv) != w:
                            bad = True
                            break
                    else:
                        x = w ^ xu ^ xv

                        # Union by size. par[root] is negative size.
                        if par[ru] > par[rv]:
                            # Attach ru under rv.
                            par[rv] += par[ru]
                            par[ru] = rv
                            dif[ru] = x
                            xu ^= x
                            ru = rv
                        else:
                            # Attach rv under ru.
                            par[ru] += par[rv]
                            par[rv] = ru
                            dif[rv] = x

                        comps -= 1

            if bad:
                break

        if bad:
            append('0')
        else:
            append(str(pow(2, comps, mod)))

    sys.stdout.write('\n'.join(out))


if __name__ == '__main__':
    solve()
```

### NOTES
- **Model:** Replace each tile rotation by endpoint booleans on its four edges. On the torus, shared edge endpoints must be equal, so use one variable per horizontal and vertical grid edge.
- **Tile equations:** For an A cell, left/right endpoint variables differ and top/bottom differ. For a B cell, left/right are equal, top/bottom are equal, and the horizontal value differs from the vertical value.
- **Parity checks:** Xoring the horizontal equations around a row gives the parity of A's in that row; it must be even. Similarly every column must have an even number of A's. Otherwise the answer is 0.
- **Reduction:** When parities are even, all horizontal edges in a row are determined by one row base, the edge left of column 0, and the prefix parity of A's before each cell. All vertical edges in a column are determined by one column base, the edge above row 0, and the prefix parity of A's above each cell.
- **B equations:** For a B cell at (i,j), with row prefix p and column prefix q, the remaining constraint is row_base_i xor column_base_j = 1 xor p xor q. A cells impose no cross-row/column constraint.
- **Counting:** This is a system of xor equations on H+W variables. A parity union-find checks consistency and counts connected components; the answer is 2^(components) modulo 998244353.
- **Special cases:** If all tiles are B, the complete bipartite equations force one component, answer 2. If all tiles are A and parities are even, there are no B equations, answer 2^(H+W). Column parity must still be checked for the all-A case.
- **Implementation notes:** Read all tokens with sys.stdin.buffer.read().split(). The first pass computes row A parity, column A parity, and b_count, breaking on an odd row. Reuse the zero column-parity list as the column prefix array in the second pass. For each B, unite row i and column H+j with label 1 xor row_pref xor col_pref[j]; update row_pref and col_pref only on A cells.
- **Union-find details:** Use negative parent sizes and xor-to-parent. The find is iterative with path compression. In the second pass, cache the current row node's root and xor while scanning the row, updating the cache when that root is attached to another root, to avoid one find per B cell.
- **Complexity:** Total cells over cases is at most 1e6, so time is roughly linear plus near-constant union-find work; memory is proportional to the input tokens, the current grid, and the largest H+W.

### NEXT
none

### STATUS
solved
