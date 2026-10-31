### STATUS
continue

### NEXT
Run N=200000 performance smoke tests (alternating, near-alternating, B^N W^N, random balanced) and tune CDQ/NTT thresholds/caching if needed.

### TASKS
- [done] Derive reduction to prefix events and inclusion-exclusion/renewal recurrence.
- [done] Implement production solver with input handling, factorials, endpoint/no-event cases, event extraction, NTT/CDQ, and alternating fast path (self-tests removed).
- [done] Cross-validate CDQ/NTT against direct DP and brute force (force CDQ on medium random event lists; small exhaustive strings), fix any bugs.
- [todo] Run N=200000 performance smoke tests (alternating, near-alternating, B^N W^N, random balanced) and tune CDQ/NTT thresholds/caching if needed.
- [todo] If performance is still insufficient, switch to a faster convolution strategy (Ga-NTT caching, batched/relaxed convolution, or big-int convolution) or add specialized fast paths.
