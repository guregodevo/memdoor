import sys

def solve() -> None:
    data = sys.stdin.read().split()
    if not data:
        return

    n = int(data[0])
    s = data[1]
    t = data[2]

    present = [False] * 26
    target = [-1] * 26

    for cs, ct in zip(s, t):
        a = ord(cs) - ord('a')
        b = ord(ct) - ord('a')
        present[a] = True
        if target[a] == -1:
            target[a] = b
        elif target[a] != b:
            print(-1)
            return

    mandatory = 0
    indeg = [0] * 26

    for a in range(26):
        if present[a]:
            b = target[a]
            if b == -1:
                print(-1)
                return
            if b != a:
                mandatory += 1
            indeg[b] += 1

    image_count = sum(1 for x in indeg if x > 0)

    visited = [False] * 26
    closed_cycles = 0

    for start in range(26):
        if not present[start] or visited[start]:
            continue

        path = []
        pos = [-1] * 26
        cur = start

        while present[cur] and not visited[cur]:
            if pos[cur] != -1:
                cycle = path[pos[cur]:]
                if len(cycle) >= 2:
                    closed = True
                    for v in cycle:
                        if indeg[v] != 1:
                            closed = False
                            break
                    if closed:
                        closed_cycles += 1
                break

            pos[cur] = len(path)
            path.append(cur)
            cur = target[cur]

        for v in path:
            visited[v] = True

    if closed_cycles > 0 and image_count == 26:
        print(-1)
    else:
        print(mandatory + closed_cycles)

if __name__ == "__main__":
    solve()