import sys
from bisect import bisect_right

def main():
    data = sys.stdin.buffer.read().split()
    if not data:
        return
    q = int(data[0])

    M = 1_000_000

    # omega[n] = number of distinct prime factors of n
    omega = bytearray(M + 1)
    for i in range(2, M + 1):
        if omega[i] == 0:  # i is prime
            for j in range(i, M + 1, i):
                omega[j] += 1

    # All 400 numbers up to 10^12 are squares of bases <= 10^6
    # having exactly two distinct prime factors.
    vals = [i * i for i in range(2, M + 1) if omega[i] == 2]

    out = []
    append = out.append
    br = bisect_right

    for idx in range(1, q + 1):
        a = int(data[idx])
        append(str(vals[br(vals, a) - 1]))

    sys.stdout.write("\n".join(out))

if __name__ == "__main__":
    main()