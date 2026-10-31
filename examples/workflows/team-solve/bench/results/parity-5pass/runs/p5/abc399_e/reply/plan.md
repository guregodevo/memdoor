### PLAN
First verify that every character of S has a single consistent target in T; if any character maps to two different characters, output -1. Build a 26-node directed graph where each source character c that appears in S points to its required target req[c]. Count changed characters C where req[c] != c; each such character must be used as the source of at least one operation. Find all directed cycles of length at least two, and mark a cycle closed if no character outside the cycle points into it; open cycles can be resolved without extra operations, while each closed cycle needs one temporary letter and adds one operation. If a closed cycle exists but no temporary letter is available (all 26 letters appear in S and every changed character lies in closed cycles), output -1; otherwise the answer is C plus the number of closed cycles.

### TASKS
- Read N, S, and T, fill req[26], and immediately mark the case impossible if a source letter is assigned two different target letters.
- Compute changed_count, the set of source letters, and the 26-letter functional graph edges c -> req[c] (ignoring edges to letters not in S for cycle detection).
- Detect all non-self cycles, and classify each cycle as closed if every vertex in it has indegree exactly one from source letters, otherwise open.
- Determine whether a spare temporary letter exists: either some letter is absent from S, or some changed letter is not contained in a closed cycle; if closed cycles exist and no spare exists, mark the case impossible.
- Implement the final O(N + 26) Python solution, printing changed_count + closed_cycle_count when possible and -1 otherwise.
