import sys
from collections import Counter
from itertools import accumulate


def solve(s: str) -> int:
    n = len(s)
    m = 2 * n + 1
    weights = {"a": m, "b": 1 - m, "c": -1}
    counts = Counter(accumulate(map(weights.__getitem__, s), initial=0))
    return sum(v * (v - 1) // 2 for v in counts.values())


def main() -> None:
    s = sys.stdin.read().strip()
    print(solve(s))


if __name__ == "__main__":
    main()
