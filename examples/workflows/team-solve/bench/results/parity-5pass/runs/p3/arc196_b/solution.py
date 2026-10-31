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