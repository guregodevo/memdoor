# Longest Balanced Substring Count — solution

**Deliverable:** `solution.py` (Python 3, standard library only). Reads one
line from stdin, prints one integer — the number of non-empty substrings of
S in which the counts of 'a', 'b' and 'c' are all equal.

## Approach (Proposal A: prefix-difference hashing, O(N))

Let `ca[i], cb[i], cc[i]` be the counts of a, b, c in the prefix `s[0..i)`.
The substring `s[i..j)` is balanced iff
`ca[j]-ca[i] == cb[j]-cb[i] == cc[j]-cc[i]`, i.e. iff the two differences
`(ca-cb, ca-cc)` are equal at prefix positions i and j. (The third
condition is implied by the other two because `ca+cb+cc = j-i`.) So the
answer is the number of pairs i < j with equal key `(ca-cb, ca-cc)`.

One left-to-right scan maintains the running triple; a dict seeded with
`{(0,0): 1}` (the empty prefix) maps key → number of earlier prefixes with
that key. At each position we add `seen[key]` to the answer **before**
inserting the current prefix, which keeps the pairing strict (i < j) and
counts each balanced substring exactly once.

Complexity: O(N) expected time (one dict op per character), O(N) memory,
≈0.04–0.11 s in CPython at N = 200000. The answer (≤ ~2·10^10) fits in a
native Python int.

## Requirements met

1. Correctness — verified against `tests/1.in..5.in`, outputs byte-equal to
   `tests/1.out..5.out`; the five documented cases (abc→1, abcabc→5,
   aabbcc→1, aaa→0, abcabcabc→12) all reproduce.
2. Complexity — single-pass hashing, O(N) expected; no O(N^2) enumeration
   anywhere in the shipped code.
3. Language — Python 3, only `import sys` (standard library).
4. I/O contract — `input().strip()` then one `print(ans)`; no prompts, no
   debug output (only the conventional single trailing newline, which
   plan.md explicitly allows).
5. Robustness — N=1 → 0; all-same-letter inputs → 0 with keys strictly
   monotone (no dict blowup); trailing whitespace/newline stripped; empty
   input after stripping prints 0 without crashing.
6. Overflow — Python ints are arbitrary precision; max ≈ 2·10^10 is fine.

## Verification evidence

- `for i in 1..5: python3 solution.py < tests/i.in | diff - tests/i.out`
  → all five byte-exact.
- Fuzz vs an independent brute-force counter: 225 random strings of length
  1–15, 20 at length 20, 10 at length 50, plus edge cases ('', 'a', 'ab',
  'aabbcc', 'abcabcabc', 'aaa', whitespace-only) — 0 mismatches.
- Independent O(N^2) cross-check on `abc`×200: direct enumeration counts
  59900 balanced substrings; solution.py agrees.
- Speed at N=200000: `abc`×66667 → 0.04 s (answer 6666700000), `a`×200000
  → 0.02 s (0), random {a,b,c} → 0.11 s.

The full decision rationale and test log are recorded under the Solver
decision heading in `notes.md`; approach selection between Proposals A/B/C
is justified there (all three keyings are provably equivalent; A is the
single-pass, least error-prone form and the one both ideators recommended
shipping).
