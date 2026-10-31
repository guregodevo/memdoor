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