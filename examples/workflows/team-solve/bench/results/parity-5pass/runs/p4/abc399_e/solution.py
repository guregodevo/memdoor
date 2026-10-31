import sys

def solve():
    data = sys.stdin.read().split()
    if not data:
        return

    n = int(data[0])
    s = data[1]
    t = data[2]

    target = [-1] * 26
    present = [False] * 26

    for cs, ct in zip(s, t):
        a = ord(cs) - ord('a')
        b = ord(ct) - ord('a')
        present[a] = True
        if target[a] == -1:
            target[a] = b
        elif target[a] != b:
            print(-1)
            return

    k = 0
    for a in range(26):
        if present[a] and target[a] != a:
            k += 1

    if k == 0:
        print(0)
        return

    if all(present) and len({target[a] for a in range(26) if present[a]}) == 26:
        print(-1)
        return

    indeg = [0] * 26
    for a in range(26):
        if present[a]:
            indeg[target[a]] += 1

    done = [False] * 26
    bad_cycles = 0

    for start in range(26):
        if not present[start] or target[start] == start or done[start]:
            continue

        path = []
        pos = {}
        cur = start

        while True:
            if cur in pos:
                cycle = path[pos[cur]:]
                isolated = True
                for v in cycle:
                    if indeg[v] != 1:
                        isolated = False
                        break
                if isolated:
                    bad_cycles += 1
                break

            if not present[cur] or target[cur] == cur or done[cur]:
                break

            pos[cur] = len(path)
            path.append(cur)
            cur = target[cur]

        for v in path:
            done[v] = True

    print(k + bad_cycles)

if __name__ == "__main__":
    solve()