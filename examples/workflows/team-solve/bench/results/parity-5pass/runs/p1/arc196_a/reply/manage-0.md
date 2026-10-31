### STATUS
continue
### NEXT
Implement and validate the correct O(N log N) Python solution and produce final code in ```python delimiters: compute f for every even prefix and suffix using two heaps (f = sum largest half - sum smallest half); for even N print f(N); for odd N print max_{k=0}^{N//2} pref[k] + suff[N//2-k]; run the provided samples (without hardcoding), brute-force random N<=8 against exhaustive adjacent-removal DP, and check performance for N=3e5.
### TASKS
- [done] Ideation: identify noncrossing matching model, even-block sorted-half-difference formula, and odd-N survivor split at odd original positions.
- [todo] Implement and validate the correct O(N log N) Python solution: two heaps for all even prefix/suffix half-differences; even N output full value; odd N max over k of pref[k] + suff[N//2-k]; run samples, brute-force small N, and performance check.
- [todo] If validation fails, switch to a different approach (Fenwick/order-statistics or interval DP for debugging) rather than patching the same idea.
