import sys

def main():
    s = sys.stdin.read().strip()
    count_a = count_b = count_c = 0
    freq = {(0, 0): 1}
    result = 0
    for ch in s:
        if ch == 'a':
            count_a += 1
        elif ch == 'b':
            count_b += 1
        else:
            count_c += 1
        diff_a_b = count_a - count_b
        diff_b_c = count_b - count_c
        key = (diff_a_b, diff_b_c)
        if key in freq:
            result += freq[key]
            freq[key] += 1
        else:
            freq[key] = 1
    print(result)

if __name__ == '__main__':
    main()
