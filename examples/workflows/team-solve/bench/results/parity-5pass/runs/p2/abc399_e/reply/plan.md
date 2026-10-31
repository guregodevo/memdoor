### PLAN
First verify that each character in S has a single corresponding character in T; if any S character maps to two different T characters, the answer is -1. Build a 26-node functional graph for characters present in S, with edge c -> its target, and count C, the number of present characters whose target differs from themselves. Find all directed cycles of length at least 2, and mark a cycle as closed if every vertex has indegree exactly 1, meaning no outside character maps into that cycle. A closed cycle requires one extra temporary operation, while a cycle with an incoming edge can be resolved using that incoming character's required move. If a closed cycle exists, feasibility requires a spare label (a letter absent from S or a present letter not on any directed cycle); otherwise the minimum is C plus the number of closed cycles.

### TASKS
- Read N, S, and T; build present[26] and target[26], and immediately output -1 if a present S character is assigned more than one target.
- Compute C as the number of present characters c with target[c] != c, and compute indeg[26] by counting present sources for each target letter.
- Traverse the 26-node functional graph to mark all nodes that lie on directed cycles and count cycles of length >= 2 whose vertices all have indeg == 1 (closed cycles); self-loops are marked but not counted.
- Set spare = (present_count < 26) or any present node not marked as in a cycle; if closed_cycles > 0 and not spare, set the answer to -1.
- Otherwise print C + closed_cycles, and verify edge cases such as S == T, full-alphabet permutation cycles, cycles with incoming edges, and absent-letter buffers.
