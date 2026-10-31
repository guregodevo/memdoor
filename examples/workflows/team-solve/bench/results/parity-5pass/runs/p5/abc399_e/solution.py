import sys


def solve() -> None:
    data = sys.stdin.buffer.read().split()
    if len(data) < 3:
        return

    s = data[1]
    t = data[2]

    appears_s = [False] * 26
    appears_t = [False] * 26
    req = [-1] * 26

    # Every original letter must have one consistent target letter.
    for cs, ct in zip(s, t):
        a = cs - 97
        b = ct - 97
        appears_s[a] = True
        appears_t[b] = True

        if req[a] == -1:
            req[a] = b
        elif req[a] != b:
            print(-1)
            return

    changed_count = 0
    out = [-1] * 26

    # Functional graph on letters that appear in S and actually need to change.
    for i in range(26):
        if appears_s[i] and req[i] != i:
            changed_count += 1
            out[i] = req[i]

    # Count every directed cycle of length at least 2.
    state = [0] * 26  # 0 = unvisited, 1 = visiting, 2 = done
    pos = [-1] * 26
    cycle_count = 0

    for i in range(26):
        if out[i] == -1 or state[i] != 0:
            continue

        stack = []
        cur = i
        while cur != -1 and state[cur] == 0:
            state[cur] = 1
            pos[cur] = len(stack)
            stack.append(cur)
            cur = out[cur]

        if cur != -1 and state[cur] == 1:
            # Self-loops are impossible here because changed letters have req[i] != i.
            cycle_count += 1

        for v in stack:
            state[v] = 2

    # A buffer exists iff not both S and T use all 26 letters.
    has_buffer = (sum(appears_s) < 26) or (sum(appears_t) < 26)

    if cycle_count > 0 and not has_buffer:
        print(-1)
    else:
        print(changed_count + cycle_count)


if __name__ == "__main__":
    solve()