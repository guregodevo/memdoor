### STATUS
continue
### NEXT
Implement and verify the final Python program using streaming two-heap median maintenance: compute pref[t] for even prefix length 2t (value = total - 2*sum_lower); if N is even output pref[N//2]; if N is odd scan right-to-left with an initially empty heap and for each even 0-index i combine pref[i//2] with the current suffix value to the right of i, then insert A[i]; use fast stdin and Python ints; run the three samples and small random brute-force tests (N<=8).
### TASKS
- [done] Derive the reduction and edge cases: removal orders correspond to non-crossing matchings; an even block's optimum is sum of its largest half minus sum of its smallest half; for odd N the unmatched element must be at an odd 1-indexed index and separates left/right even blocks.
- [todo] Implement and verify the final Python program using streaming two-heap median maintenance: compute pref[t] for even prefix length 2t (value = total - 2*sum_lower); if N is even output pref[N//2]; if N is odd scan right-to-left with an initially empty heap and for each even 0-index i combine pref[i//2] with the current suffix value to the right of i, then insert A[i]; use fast stdin and Python ints; run the three samples and small random brute-force tests (N<=8).
- [todo] If the two-heap version is too slow or fails, switch to a Fenwick tree over compressed values to compute prefix/suffix smallest-half sums.
