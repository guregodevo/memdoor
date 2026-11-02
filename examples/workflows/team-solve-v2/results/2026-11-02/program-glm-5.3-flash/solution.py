import sys


def main():
    s = sys.stdin.read().strip()
    seen = {(0, 0): 1}
    ca = cb = cc = 0
    ans = 0
    get = seen.get
    for ch in s:
        if ch == 'a':
            ca += 1
        elif ch == 'b':
            cb += 1
        else:
            cc += 1
        key = (ca - cb, ca - cc)
        n = get(key)
        if n is not None:
            ans += n
            seen[key] = n + 1
        else:
            seen[key] = 1
    print(ans)


main()
