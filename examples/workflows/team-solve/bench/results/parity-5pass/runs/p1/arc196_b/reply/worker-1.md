### CODE
```python
import sys

MOD = 998244353
A_ORD = 65


def find(parent, xpar, x):
    acc = 0
    while True:
        p = parent[x]
        if p < 0:
            return x, acc
        acc ^= xpar[x]
        gp = parent[p]
        if gp >= 0:
            xpar[x] ^= xpar[p]
            parent[x] = gp
        x = p


def solve():
    data = sys.stdin.buffer.read().split()
    if not data:
        return

    t = int(data[0])
    idx = 1
    out = []
    append = out.append
    find_func = find
    mod = MOD
    a_ord = A_ORD

    for _ in range(t):
        H = int(data[idx])
        W = int(data[idx + 1])
        idx += 2

        n = H + W
        parent = [-1] * n
        xpar = [0] * n
        comp = n
        col = [0] * W
        valid = True

        for i in range(H):
            s = data[idx]
            idx += 1
            prefix = 0

            for j, c in enumerate(s):
                if c == a_ord:
                    prefix ^= 1
                    col[j] ^= 1
                else:
                    w = 1 ^ col[j] ^ prefix
                    ra, pa = find_func(parent, xpar, i)
                    rb, pb = find_func(parent, xpar, H + j)

                    if ra == rb:
                        if (pa ^ pb) != w:
                            valid = False
                            break
                    else:
                        tpar = pa ^ pb ^ w
                        if parent[ra] > parent[rb]:
                            ra, rb = rb, ra
                        parent[ra] += parent[rb]
                        parent[rb] = ra
                        xpar[rb] = tpar
                        comp -= 1

            if valid and prefix:
                valid = False

            if not valid:
                idx += H - i - 1
                break

        if valid and 1 in col:
            valid = False

        if valid:
            append(str(pow(2, comp, mod)))
        else:
            append("0")

    sys.stdout.write("\n".join(out))


if __name__ == "__main__":
    solve()
```

### NOTES
- **Edge-bit model:** After identifying endpoints on each shared torus edge, let V_{i,j} be the bit on the top edge of cell (i,j) and H_{i,j} the bit on its left edge. A tile gives V_{i,j} xor V_{i+1,j}=1 and H_{i,j} xor H_{i,j+1}=1. B tile gives V_{i,j}=V_{i+1,j}, H_{i,j}=H_{i,j+1}, and V_{i,j} xor H_{i,j}=1.
- **Torus parity:** Summing the A-flip equations around a column/row forces every column and every row to contain an even number of A's. The code maintains column parity while scanning and checks row parity from the row prefix at the end of each row.
- **Reduced variables:** When parities are even, all vertical bits in column j are determined by x_j=V_{0,j} and the column A-prefix; all horizontal bits in row i are determined by y_i=H_{i,0} and the row A-prefix.
- **B-cell constraints:** For a B at (i,j), V_{i,j} xor H_{i,j}=1 becomes x_j xor y_i = 1 xor col_parity[j] xor row_prefix. This is a parity edge between row node i and column node H+j.
- **Counting:** If any row/column parity is odd or a DSU parity contradiction occurs, answer is 0. Otherwise the H+W variables form a labeled bipartite graph; each connected component has one free bit, so answer is 2^{components} mod 998244353. Isolated all-A rows/columns are isolated components and correctly contribute factors of 2.
- **Implementation details:** Read all tokens. For each case allocate parent (negative size) and xor-to-parent arrays of length H+W, and comp=H+W. DSU find uses path halving and returns (root, xor to root). Union by size; on same root check stored parity. Process rows as bytes; A=65 toggles prefix and col[j], B unions. If invalid, skip remaining row tokens by advancing idx. Final column check uses `1 in col`.
- **Complexity:** O(HW alpha(H+W)) per case, total O(1e6 alpha) over input; memory O(H+W) per case plus the token list.
- **Sample check:** Manual trace of the three samples gives 2, 0, 2, matching the expected output.

### NEXT
- none

### STATUS
solved
