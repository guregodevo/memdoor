### STATUS
continue
### NEXT
Write a fresh, complete Python 3 program (not a patch) that reads stdin, computes the answer with the validated formula: for every even prefix/suffix maintain lower/upper halves with two heaps and record sum_upper - sum_lower; for even N print pref[N//2]; for odd N print max_{t=0..N//2} pref[t] + suff[N//2-t]; ensure it writes the integer to stdout with a newline; then run the three samples and a brute-force interval DP for all N<=9 random arrays to verify, and include the final code in the required delimiters.
### TASKS
- [done] Ideation: identify noncrossing matching model, even-block sorted-half-difference formula, and odd-N survivor split at odd original positions.
- [todo] Implement a fresh standalone Python 3 solution using two-heap prefix/suffix half-differences, with correct I/O/print, and validate on samples, brute-force small N, and a 3e5 performance check.
- [todo] If validation fails, switch to a different data structure (Fenwick/order-statistics) or use interval DP to isolate the failing case, rather than patching the previous empty/buggy code.
