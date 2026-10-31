### STATUS
done
### NEXT

### TASKS
- [done] Implement a standalone brute-force oracle and theorem validator for small cases comparing Bellman-Ford feasibility with the bad-pair condition.
- [done] Write the complete standalone Python solver using the compressed-coordinate CDQ specified in NEXT and make both samples pass.
- [done] Stress-test the current solver against the brute-force oracle on small random cases and targeted edge cases (shared l/r, reversed pairs, opposite-sign crossings/nesting, all same sign, length-2 intervals); fix any mismatches.
- [done] Benchmark the solver on worst-case N=4e5, M=Q=2e5; tune BASE/local bindings or switch to a different crossing-detection method if it is too slow.
