deliverable: solution.py

# Task
Given a string S of length N (1 <= N <= 200000) over {a,b,c}, count
non-empty substrings in which the counts of 'a', 'b' and 'c' are all equal.
Read S from stdin, print exactly one integer to stdout.

## Requirements the deliverable must meet
1. Correctness: the printed integer equals the number of non-empty
   substrings (contiguous) with equal counts of all three letters.
   Must reproduce the provided answers:
   - abc -> 1
   - abcabc -> 5
   - aabbcc -> 1
   - aaa -> 0
   - abcabcabc -> 12
   and pass tests/1.in..5.in with outputs tests/1.out..5.out exactly
   (byte-exact: the answer alone, no trailing text; a single trailing
   newline is acceptable and conventional).
2. Complexity: O(N) or O(N log N) worst case for N up to 200000.
   O(N^2) substring enumeration will not pass and must not be shipped.
3. Language: Python 3, standard library only (no pip installs).
4. I/O contract: read one line from stdin (strip trailing newline/whitespace),
   print one integer, nothing else. No prompts, no debug output.
5. Robustness: handle N=1 (answer 0 unless the string could be balanced,
   which it cannot for N=1), strings of a single repeated letter, and
   all-same-letter inputs without perf blowups. Strip whitespace safely;
   empty input after stripping is not in the constraints but should not crash.
6. The answer fits comfortably in Python int (it is at most ~2*10^10).

## Candidate approaches

### Proposal A: prefix-difference hashing (canonical, O(N))
Map a,b,c to offsets and keep running counts (ca,cb,cc). A substring
S[i+1..j] is balanced iff ca[j]-ca[i] == cb[j]-cb[i] == cc[j]-cc[i],
i.e. the pair (ca-cb, ca-cc) is equal at positions i and j. Scan once,
maintaining a dict from (ca-cb, ca-cc) -> number of seen positions
(starting with (0,0):1 for the empty prefix), and add the current count
before incrementing. Time O(N) expected (hash dict), memory O(N) worst case.
This is the standard technique for "equal-counts substring" problems and
is almost certainly the intended solution.
Tradeoffs: simple, provably correct, fast. Requires care that Python dict
operations on tuple keys are fast enough; N=200000 is tiny for Python.
Risk: essentially none, if the key is built correctly.

### Proposal B: run-length / structural decomposition (O(N) with more cases)
Exploit structure: balanced substrings must have length divisible by 3.
Count runs and enumerate length-3k windows smartly via combinatorial
counting over run boundaries. Correct but much fiddlier; the constant
factor and edge cases (runs inside a balanced window) make it error-prone.
Tradeoff: potentially less memory, but no real win over A. Only worth it
if hashing is somehow disallowed (it is not).

### Proposal C: O(N log N) via sorting prefix triples
Store all prefix key pairs and count equal pairs by sorting instead of a
dict. Same answers, slightly slower, more code. No advantage over A.

## Decomposition (what each role should do)
- Ideators (2): independently propose the algorithm they believe is
  optimal, including a proof sketch of why the key (ca-cb, ca-cc) equality
  is equivalent to balance (Proposal A's core claim) or argue for a
  different scheme. Note any edge cases (empty string, single letters,
  very long runs) and rough performance estimates.
- Solver: implement solution.py with Proposal A unless ideators surface
  a reason it is wrong; keep the implementation minimal, stdlib-only,
  and fast (avoid per-character function-call overhead where trivial;
  e.g. iterate over the string directly, use dict.get). Test against all
  five provided cases and against a brute-force O(N^2) checker on random
  small strings (up to length ~12, plus length ~50 spot checks) before
  declaring done.
- Reviewer: verify correctness reasoning, check edge cases and the exact
  output format, confirm the complexity claim, and independently run the
  tests; confirm solution.py contains no stray output and no non-stdlib
  imports.

## Instructions for independent ideators
Work only from this file and task.md. Do not read each other's notes.md
entries; write under your own heading only (<!-- proposal-a --> for
Proposal A owner, <!-- proposal-b --> for Proposal B owner). Each ideator:
state the algorithm in 5-15 lines, argue correctness in 3-5 lines, list
edge cases, estimate runtime for N=200000 in CPython. If you believe the
canonical hashing key is wrong, say precisely why and propose an
alternative key.

## Instructions for the solver
1. Read plan.md; pick the approach the ideators converge on (default A).
2. Write solution.py, minimal, stdlib only.
3. Run: python3 solution.py < tests/i.in for i=1..5; diff against tests/i.out.
4. Write your own brute-force checker and compare on >= 200 random strings
   of lengths 1..15 over {a,b,c}; also test "aaa"*66666-scale inputs for speed.
5. Record the decision and evidence under the Solver decision heading in
   notes.md (small append-only patch; do not touch other sections).

## Instructions for the reviewer
1. Read solution.py and the solver's notes.md entry.
2. Re-derive the key equivalence yourself; do not trust the solver.
3. Run the five tests and any extra fuzz the solver skipped.
4. Write findings under the Review heading in notes.md (append-only).
If the reviewer finds a real bug, the repair step uses the Repair heading;
the solver fixes, retests, and updates notes.md.
