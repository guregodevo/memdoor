import sys

def main():
    data = sys.stdin.buffer.read().split()
    if not data:
        return
    t = int(data[0])
    idx = 1
    out = []
    append = out.append
    mod = 998244353
    a = 65
    ab = b'A'

    for _ in range(t):
        h = int(data[idx])
        w = int(data[idx + 1])
        idx += 2

        n = h + w
        parent = [-1] * n
        parity = [0] * n
        comps = n
        col_pref = [0] * w

        valid = True
        base = h
        cp = col_pref
        par = parent
        parit = parity
        cols = range(w)

        for i in range(h):
            s = data[idx]
            idx += 1

            if s.count(ab) & 1:
                valid = False
                idx += h - i - 1
                break

            row_pref = 0
            for j in cols:
                if s[j] == a:
                    row_pref ^= 1
                    cp[j] ^= 1
                else:
                    label = 1 ^ row_pref ^ cp[j]

                    x = i
                    accx = 0
                    while par[x] >= 0:
                        p = par[x]
                        accx ^= parit[x]
                        gp = par[p]
                        if gp >= 0:
                            parit[x] ^= parit[p]
                            par[x] = gp
                        x = p
                    rx = x
                    px = accx

                    y = base + j
                    accy = 0
                    while par[y] >= 0:
                        p = par[y]
                        accy ^= parit[y]
                        gp = par[p]
                        if gp >= 0:
                            parit[y] ^= parit[p]
                            par[y] = gp
                        y = p
                    ry = y
                    py = accy

                    if rx == ry:
                        if (px ^ py) != label:
                            valid = False
                            idx += h - i - 1
                            break
                    else:
                        val = px ^ py ^ label
                        if par[rx] > par[ry]:
                            rx, ry = ry, rx
                        par[rx] += par[ry]
                        par[ry] = rx
                        parit[ry] = val
                        comps -= 1

            if not valid:
                break

        if valid and 1 in col_pref:
            valid = False

        if valid:
            append(str(pow(2, comps, mod)))
        else:
            append("0")

    sys.stdout.write("\n".join(out))

if __name__ == "__main__":
    main()