### STATUS
continue
### NEXT
Run a dev-only cross-validation of the CDQ path: compare cdq_solve against direct_dp_small on many random valid event lists with t=2000..5000 (forcing NTT branches) and brute-force small N<=8 strings; fix any mismatches in CDQ/NTT, keeping final code clean.
### TASKS
- [done] Derive reduction to prefix events and inclusion-exclusion/renewal recurrence.
- [done] Implement production solver with input handling, factorials, endpoint/no-event cases, event extraction, NTT/CDQ, and alternating fast path (self-tests removed).
- [todo] Cross-validate CDQ/NTT against direct DP and brute force (force CDQ on medium random event lists; small exhaustive strings), fix any bugs.
- [todo] Run N=200000 performance smoke tests (alternating, near-alternating, B^N W^N, random balanced) and tune CDQ/NTT thresholds/caching if needed.
- [todo] If performance is still insufficient, switch to a faster convolution strategy (Ga-NTT caching, batched/relaxed convolution, or big-int convolution) or add specialized fast paths.
