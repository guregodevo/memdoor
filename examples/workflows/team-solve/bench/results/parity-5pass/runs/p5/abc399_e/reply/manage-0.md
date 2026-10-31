### STATUS
continue

### NEXT
Implement the complete Python solution using the functional-graph closed-cycle formula: read input, check consistency, count changed appearing letters, count closed cycles (cycles with no incoming edge from outside), and output changed_count + closed_cycle_count if no closed cycles or a buffer exists, else -1.

### TASKS
- [done] Derive the closed-cycle formula: answer = changed_count + closed_cycle_count, with -1 when closed cycles exist but no buffer (absent letter or changed letter outside closed cycles).
- [todo] Implement the complete Python solution using the functional-graph closed-cycle formula: read input, check consistency, count changed appearing letters, count closed cycles (cycles with no incoming edge from outside), and output changed_count + closed_cycle_count if no closed cycles or a buffer exists, else -1.
- [todo] Validate the implementation on samples and edge cases: inconsistent mapping, open/closed cycles, multiple closed cycles, full alphabet permutations, absent buffers, and acyclic dependencies.
- [todo] If validation fails, switch to a constructive 26-letter scheduler or small-alphabet BFS oracle to identify the failing case.
