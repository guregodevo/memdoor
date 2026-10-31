import sys

MOD = 998244353


def solve():
    data = sys.stdin.buffer.read().split()
    if not data:
        return

    t = int(data[0])
    idx = 1
    out = []

    for _ in range(t):
        H = int(data[idx])
        W = int(data[idx + 1])
        idx += 2

        n = H + W
        parent = list(range(n))
        size = [1] * n
        diff = [0] * n
        col_pref = [0] * W

        par = parent
        sz = size
        dif = diff
        cp = col_pref

        comps = n
        ok = True
        cols = range(W)

        for i in range(H):
            if not ok:
                idx += H - i
                break

            s = data[idx]
            idx += 1
            rp = 0

            for j in cols:
                c = s[j]
                if c == 65:  # 'A'
                    rp ^= 1
                    cp[j] ^= 1
                else:
                    w = 1 ^ cp[j] ^ rp

                    # find row vertex i
                    rx = i
                    px = 0
                    while True:
                        p = par[rx]
                        if p == rx:
                            break
                        gp = par[p]
                        if gp != p:
                            dif[rx] ^= dif[p]
                            par[rx] = gp
                            p = gp
                        px ^= dif[rx]
                        rx = p

                    # find column vertex H + j
                    ry = H + j
                    py = 0
                    while True:
                        p = par[ry]
                        if p == ry:
                            break
                        gp = par[p]
                        if gp != p:
                            dif[ry] ^= dif[p]
                            par[ry] = gp
                            p = gp
                        py ^= dif[ry]
                        ry = p

                    if rx == ry:
                        if (px ^ py) != w:
                            ok = False
                            break
                    else:
                        if sz[rx] < sz[ry]:
                            par[rx] = ry
                            dif[rx] = px ^ py ^ w
                            sz[ry] += sz[rx]
                        else:
                            par[ry] = rx
                            dif[ry] = px ^ py ^ w
                            sz[rx] += sz[ry]
                        comps -= 1

            if ok and rp:
                ok = False

            if not ok:
                idx += H - i - 1
                break

        if ok and any(cp):
            ok = False

        if ok:
            out.append(str(pow(2, comps, MOD)))
        else:
            out.append("0")

    sys.stdout.write("\n".join(out))


if __name__ == "__main__":
    solve()