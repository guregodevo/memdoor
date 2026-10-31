import sys

def solve():
    data = sys.stdin.read().split()
    if not data:
        return

    n = int(data[0])
    s = data[1]
    t = data[2]

    present = [False] * 26
    target = [-1] * 26

    for cs, ct in zip(s, t):
        c = ord(cs) - ord('a')
        d = ord(ct) - ord('a')
        present[c] = True
        if target[c] == -1:
            target[c] = d
        elif target[c] != d:
            print(-1)
            return

    changed = [False] * 26
    changed_count = 0
    for c in range(26):
        if present[c] and target[c] != c:
            changed[c] = True
            changed_count += 1

    if changed_count == 0:
        print(0)
        return

    free_exists = any(not present[c] for c in range(26))

    in_cycle = [False] * 26
    for c in range(26):
        if not changed[c]:
            continue
        cur = c
        for _ in range(26):
            if not changed[cur]:
                break
            cur = target[cur]
            if cur == c:
                in_cycle[c] = True
                break

    cycle_id = [-1] * 26
    cycle_count = 0
    for i in range(26):
        if in_cycle[i] and cycle_id[i] == -1:
            cid = cycle_count
            cur = i
            while cycle_id[cur] == -1:
                cycle_id[cur] = cid
                cur = target[cur]
            cycle_count += 1

    has_tree = [False] * cycle_count
    tree_exists = False

    for u in range(26):
        if changed[u] and not in_cycle[u]:
            tree_exists = True
            v = target[u]
            if in_cycle[v]:
                has_tree[cycle_id[v]] = True

    isolated_cycles = sum(1 for h in has_tree if not h)

    if not (free_exists or tree_exists):
        print(-1)
    else:
        print(changed_count + isolated_cycles)

if __name__ == "__main__":
    solve()