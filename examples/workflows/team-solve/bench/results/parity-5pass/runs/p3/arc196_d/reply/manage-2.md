### STATUS
done
### TASKS
- [done] Implement a small brute-force oracle and random/exhaustive tester to validate the forbidden-pair characterization and query reduction on tiny cases.
- [done] Implement the complete real solver (replacing the stress tester) using the bad-pair characterization: normalize to (l,r,sign), compute earliest bad partner for same l, same r, opposite same interval, and same-sign strict crossings via four orientation sweeps, build suffix minima, and answer queries.
- [done] Test the real solver on the provided samples, edge cases (touching endpoints, identical opposite, strict crossing, 1-indexing), and max-constraint performance.
