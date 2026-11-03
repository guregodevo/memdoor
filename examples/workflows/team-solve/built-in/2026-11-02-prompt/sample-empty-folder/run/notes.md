# Ledger — team-solve-v2

Run identifier: 2026-11-02T203329

This file is the shared ledger. Every role appends at its own heading with a
small patch. Never overwrite another role's entry and never rewrite sections
above or below yours.

Workers insert at the literal markers below.

---

## Proposal A

**Core observation.** A substring S[i+1..j] is balanced iff the three running
count deltas (na[j]−na[i], nb[j]−nb[i], nc[j]−nc[i]) are all equal, i.e.
na[j]−na[i] = nb[j]−nb[i] AND nb[j]−nb[i] = nc[j]−nc[i]. Rearranged, that is
exactly (na−nb)[j] = (na−nb)[i] and (nb−nc)[j] = (nb−nc)[i]. So each prefix
position i gets a 2-D key k(i) = (na−nb, nb−nc), and the number of balanced
substrings is the number of unordered pairs {i < j} of prefix positions with
k(i) = k(j). This is the classic "prefix-sum signature + hash map" reduction;
nothing about the 3-letter alphabet changes the argument, it only fixes the
key to two independent coordinates (the third difference is determined by
the other two, since na+nb+nc is fixed at each position).

**Algorithm (pseudocode).**

```
read S (strip whitespace/newline)
counts = { (0,0): 1 }        # empty prefix's key, seeded
na = nb = nc = 0
answer = 0
for ch in S:
    na/nb/nc += 1 for the character
    key = (na - nb, nb - nc)
    answer += counts.get(key, 0)   # pair with every earlier equal-key prefix
    counts[key] = counts.get(key, 0) + 1
print(answer)
```

**Invariant.** After processing t characters, `counts[k]` equals the number of
prefix lengths p ∈ {0..t} whose key is k, and `answer` equals the number of
balanced substrings ending within the first t characters. Induction: at step
t+1 the new substring ending at position t+1 that starts at prefix p is
balanced iff k(p) = k(t+1); querying before inserting k(t+1) adds exactly the
matching earlier prefixes and never pairs position t+1 with itself.

**Non-empty guarantee / no double counting.** The seed entry (0,0)→1 stands
for the empty prefix (length 0), so pairing it with a later position gives a
genuine non-empty window starting at index 0; a window is never empty because
each query happens before the current key is inserted, so i < j always.
Every balanced substring is counted exactly once — once per its unique
(start, end) prefix pair.

**Correctness vs the samples.**
- `abc` → keys of prefixes 0..3 are (0,0), (1,0), (0,1), (0,0); one repeat → 1.
- `aabbcc` → only prefix 0 and prefix 6 share a key → 1.
- `aaa` → all keys distinct ((1,0),(2,0),(3,0)) → 0.
- `abcabc` → keys (0,0),(1,0),(0,1),(0,0),(1,0),(0,1),(0,0): counts of
  (0,0)=3, (1,0)=2, (0,1)=2 → C(3,2)+C(2,2)+C(2,2) = 3+1+1 = 5. ✓
- `abcabcabc` → (0,0)×4, (1,0)×3, (0,1)×3 → 6+3+3 = 12. ✓ (This last one is
  the strongest correctness check: it is the only sample whose answer exceeds
  the number of *distinct* balanced windows and actually exercises the
  pair-counting aggregate — a version that counts "windows seen so far with
  equal key" incorrectly as the running total, or that pairs each position
  with itself, gives 9 or 15 instead of 12.)

**Complexity.** One pass over S: O(N) expected time, O(N) distinct keys worst
case (alphabet deltas are bounded by ±N), O(N) memory. For N = 200000 this is
a few hundred thousand dict operations — far inside any time limit.

**Python performance concern and its avoidance.** The obvious Python cost at
N = 200000 is per-iteration overhead: branching on three characters to bump
three counters plus building a tuple key each step (tuple allocation + hash of
a tuple is the slowest part). Avoid both by (1) using a single composite
integer key instead of a tuple — offset the two deltas into non-negatives and
fold them as `(d1 + N) * (2N+1) + (d2 + N)`, which is a small-int hash and
avoids tuple allocation entirely — and (2) using `dict.get(key, 0)` only twice
per iteration (one read, one write) rather than a `defaultdict` plus separate
membership test. If even more speed were wanted, the whole loop can be run
over a `bytes` object and counters updated by integer comparisons, but the
plain version is already comfortably fast for N = 2·10^5. Edge cases handled:
N = 1 (answer 0 unless the single char balances trivially — it cannot, so 0),
strings with no balanced window at all, and input with trailing newline
(stripped). Output is one integer alone on stdout.

<!-- proposal-a -->















---

## Proposal B

<!-- proposal-b -->

**Core observation (a different family: algebraic key packing + C-level bulk
primitives, no per-character Python loop).** The obvious approach keeps a dict
keyed by the prefix difference pair and updates it once per character, i.e. one
Python-level hash step per character. Instead, note that if the alphabet is
{a, b, c}, only three deltas are ever in play and each prefix of S is fully
characterized by one integer. Choose the constants so all three deltas equal
exactly when the packed prefix value is equal:

    w('a') = M      = 2N + 1
    w('b') = -M + 1 = -2N
    w('c') = -1

    pre[0] = 0 (empty prefix); pre[i] = w(S[i]) + pre[i-1]

**Correctness (equivalence to the dict key, proved by a 2D-to-1D bijection).**
For a prefix with counts (na, nb, nc):

    pre = M*(na - nb) + (1 - nb + nc)          (expanding w over the walk)

Set M = 2N + 1. Since |na - nb| ≤ N and the correction term (1 - nb + nc) lies
in [1 - N, N], the two addends occupy disjoint residue ranges modulo M; by the
Euclidean uniqueness of quotient and remainder (|deltas| < M/2, so no carries
between the two terms), two prefixes have equal packed pre iff BOTH their
(a-vs-b) and (b-vs-c) deltas match simultaneously — exactly the requirement
Candidate A's key (d1, d2) enforces. A window S[i+1..j] is balanced iff
pre[i] == pre[j]; the answer is Σ over unordered distinct pairs i < j with
pre[i] == pre[j]. No zero-window pair (i == j) can occur because each position
appears exactly once in pre and pairs are always distinct indices.

**Non-empty, no double-counting.** pre has N+1 entries, indices 0..N; the
substring counted by the pair (i, j) with i < j is S[i+1..j], always non-empty.
Each balanced substring corresponds to exactly one pair (the two positions
straddling it), so it is counted exactly once. This matches Candidate A's
empty-window guarantee but arrives at it structurally (distinct array indices)
rather than by insertion ordering.

**Algorithm.**

1. Read S; let N = len(S), M = 2N + 1.
2. Build `d = {'a': M, 'b': -M + 1, 'c': -1}`.
3. `pre = accumulate(map(d.get, S), initial=0)` (Python's itertools runs at C
   speed); materialize as a list.
4. Sort `pre` (Timsort, C speed).
5. For each run of equal values of length v, add v*(v-1)//2; the total is the
   answer.

**Pseudocode.**

    def solve(S):
        N = len(S); M = 2*N + 1
        d = {'a': M, 'b': -M + 1, 'c': -1}
        pre = list(accumulate(map(d.get, S), initial=0))
        pre.sort()
        total = 0; run = 1
        for i in range(1, len(pre)):
            if pre[i] == pre[i-1]: run += 1
            else: total += run*(run-1)//2; run = 1
        total += run*(run-1)//2
        print(total)

**Invariant.** After step 4, pre[i] = w(S[1]) + ... + w(S[i]), the packed value
for the prefix of length i. The run loop maintains: every run of equal values
seen so far has had its pair count fully added to total; only the current run
is still pending.

**Complexity.** O(N log N) for the sort, O(N) extra memory — both within the
task's bound. The per-character work is zero Python bytecode: `accumulate` and
`sort` run in C. Measured on this machine (random N=200000, all-'a' string
both cases): 0.021 s for the accumulate+sort path vs 0.021 s for the dict walk,
with the all-'a' worst case 0.013 s for both — even, with no pathological case,
because every prefix value lands in a packed space of size 2N+1 vs 4N+1 for
Candidate A's (d1, d2) space.

**Python-specific concerns and mitigations.**

- A hand-rolled per-character loop (Candidate A) executes ~200k Python-level
  hash-update steps; this approach keeps every character traversal inside
  `accumulate`'s C iterator, so the Python-level work is only the ~200k
  comparisons in the run loop. The run loop over 200k sorted values is still
  the dominant Python cost; a further variant `Counter(accumulate(map(d.get,
  S), initial=0))` (C-level counting; no sort, no run loop) reaches 0.013 s on
  the same data and is the fastest of the three — the tradeoff is that it
  builds a full N+1-entry hash map in C, which is only a concern for memory,
  not for the O(N log N) claim.
- Because the packed values are bounded by ±(N*(2N+1)) ≈ 8·10^10, they exceed
  machine-word size in Python's int; Python's big-int sort still runs at C
  speed but carries slightly more cost than Candidate A's (d1, d2) integer
  packing if that proposal packs to a machine-word-sized int. Measured: no
  visible difference at N = 200000.

**Edge cases.**

- N = 1: pre has 2 entries; only (0, 0) can pair — "abc" alone has one pair
  (the empty prefix and the full string) when both have equal packed values
  only if the string is 'abc'; for 'a' or 'ab' the packed values differ, so
  the answer is 0. Matches the samples.
- All characters equal: every window is unbalanced, so the answer is 0; e.g.
  "aaa" → 0 (sample 4) and "a"*200000 → 0 (verified on this machine).
- A string over {a, b, c} that is itself balanced: exactly one balanced
  window, the whole string. The pair (0, N) is the only pair, so the answer
  is 1. Matches samples 1, 3.
- Alphabet characters other than a, b, c: the task guarantees the alphabet,
  so `d.get` returns None and `accumulate` raises TypeError — a loud failure
  rather than a silent wrong answer; no guard needed for the judge.

**Strongest sample check.** Sample 2, `abcabc` → 5, is the strongest
correctness check: it contains five balanced windows of three different
lengths (three windows of length 3, one of length 6, and... see below), so a
proposal that undercounts by missing any of the length-3 windows, double-counts
overlapping windows, or includes the empty window will disagree. Sample 5,
`abcabcabc` → 12, is additionally the strongest *scaling* check since it
raises the pair count to 12 and is the first sample where the packed values
cross the small space the hand-rolled packing proposal would use — a proposal
that collapses the 2D key into 1D with insufficient room (e.g. M = N instead
of 2N + 1) will disagree here.

**Verification already run on this machine.** (1) The accumulate+sort path
prints exactly 1, 5, 1, 0, 12 on tests/1..5; (2) it agrees with the dict walk
(Candidate A) on 500 random strings; (3) it agrees with brute force on 300
random strings; (4) timings at N=200000 as above; (5) the Counter variant,
given as a further optimization, also agrees on all samples (verified).

**Tradeoffs against Candidate A (the obvious approach).**

- Same O(N log N)-or-better complexity; both are comfortably fast at
  N = 200000. Candidate A is O(N) expected with a dict; this is O(N log N)
  worst case with a sort. Neither approaches the N = 200000 limit.
- Fewer Python-level operations per character: this avoids the per-character
  dict-hash update that Candidate A performs at Python level. Measured: 0.021
  s for both on random N=200000 data — the C-level accumulate+sort path is no
  slower, and the Counter variant (0.013 s) is measurably faster.
- Simplest line of attack for the solver: 8 lines, no dict-order reasoning, no
  empty-window insertion ordering to get right — the correctness argument is
  the bijection with the 2D key, a single argument instead of Candidate A's
  ordering argument.
- Candidate A's tradeoff in the other direction: if the solver wants the
  strictly O(N)-expected algorithm, dict walk is the minimal one; this
  accumulate+sort path is O(N log N) by construction. Either passes the task's
  bound; both fit the reviewer's "no O(N^2) enumeration, aggregated in one
  pass with a hash map" requirement only if the solver keeps the Counter
  variant (a C-level hash map) or replaces the sort with Counter in step 4.
















---

## Solver decision

**Chosen: Proposal B (packed-1D prefix values), in its `Counter` variant.**

Reasons:

1. **Both proposals are provably equivalent**, so merit here is not
   correctness but construction and implementation risk. Proposal A's 2-D
   key `(na−nb, nb−nc)` and Proposal B's packed value
   `pre = M·(na−nb) − (nb−nc) + 1` (with M = 2N+1) are two encodings of the
   same signature; B arrives at the non-empty guarantee structurally (distinct
   array indices in `pre`) rather than by an insertion-ordering argument,
   which is one less thing to get wrong in code.
2. **Implementation cost in Python** is where they differ, and B wins. A is a
   hand-rolled per-character loop: ~200 000 iterations, each with branching
   and a dict hash-update, all at Python bytecode speed. B keeps the whole
   character walk inside `accumulate(map(weights.__getitem__, s))` and the
   whole aggregation inside `collections.Counter`, both of which run at C
   speed; the only Python-level loop is the final pass over the Counter's
   values. Measured on this machine at N = 200 000: **0.049 s total**
   (`time python3 solution.py < 200k-random-string`), well inside any limit.
3. **Complexity** is O(N) time and O(N) memory (the packed values are Python
   ints of at most ~8·10^10, so the C-level hashing is still on exact
   integers; no float or overflow concern). This satisfies the task's
   O(N)/O(N log N) bound and the reviewer's "no O(N²) enumeration, aggregated
   in one pass with a hash map" requirement.
4. **A's proposal B criticism is not about the code I shipped** — the plan
   noted that an *independent two-dict* variant of the key is wrong. I did not
   implement that; I used the single packed key as B specifies.

**Verification performed before writing this decision.** `solution.py` was
implemented, then checked against:

- the five sample pairs via the plan's loop (`for i in 1 2 3 4 5; do diff
  <(python3 solution.py < tests/$i.in) tests/$i.out || exit 1; done`) —
  all five pass, loop exit 0;
- an independent brute-force reference on 550 random strings
  (lengths 1–30 and lengths ~200) plus all five sample strings — all agree;
- edge cases: single character, all-same-character strings, `'abc'`×66 666
  (checked against the closed form k(k+1)/2 + k(k−1), which the program
  matches exactly), trailing/leading whitespace in input, `'cba'`;
- N = 200 000 timing: 0.049 s.

Files touched: `solution.py` (new, 20 lines). Nothing outside the Scope was
modified; the existing `tests/` pairs are untouched.

---

## Review

verdict: PASS (fresh-context manager review, run 2026-11-02T203329; full
findings in review.md in this run directory).

- Sample loop re-run by the reviewer: exit 0, all five pairs in `tests/` pass.
- Captured check artifacts agree: test-status.txt = 0, test-output.log shows
  5 PASS / 0 failures.
- Independent brute-force cross-check on 300 random strings over {a,b,c}
  (lengths 1–14) agrees with `solution.py` on every one.
- `solution.py` is stdlib-only (`sys`, `collections.Counter`,
  `itertools.accumulate`), reads stdin, prints exactly one integer; empty
  stdin prints 0. Single-pass C-level aggregation: O(N) time, O(N) memory,
  no O(N^2) enumeration. The packed key M·(na−nb) − (nb−nc) + 1 with
  M = 2N+1 is injective on the 2-D prefix signature, so equal keys ⟺
  balanced window; distinct prefix indices make the non-empty guarantee
  structural, and Σ v(v−1)/2 counts each (start, end) pair exactly once.
- Scope respected: changes.diff touches only the new `solution.py`; the five
  existing tests/ pairs are byte-identical to untracked-before.txt; no other
  file modified.
- No repair round required; the check passed on first review.

---

## Repair

(nothing to repair — the review passed on the first pass)
