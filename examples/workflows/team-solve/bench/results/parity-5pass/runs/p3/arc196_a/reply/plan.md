### PLAN
Model the removals as a noncrossing matching, and note that for any even-length segment the maximum score is the sum of its largest half minus the sum of its smallest half. The upper bound follows by orienting each matched pair from the larger value to the smaller value and bounding the resulting signed sum; the lower bound follows by coloring the largest half as plus and the smallest half as minus and repeatedly deleting adjacent opposite-colored pairs. For odd N=2K+1, the final leftover must be at an odd original index, and no pair can cross it, so the answer is the maximum over odd positions m of f(left of m)+f(right of m), where f is the even-segment value. Compute f for all even prefixes and suffixes online with a two-heap structure that maintains the lower half, upper half, and their sums while inserting elements in pairs. For even N use f on the whole array; for odd N combine the prefix and suffix values and output the maximum, giving O(N log N) time.

### TASKS
- Implement a MedianSplit class using a max-heap for the smallest half and a min-heap for the largest half, with add(x) rebalancing to lower size total//2 and score() returning sum_upper - sum_lower.
- Handle even N by inserting all A_i into one MedianSplit and printing score().
- For odd N=2K+1, build pref[0..K] where pref[i] is the score after inserting the first 2i elements, with pref[0]=0.
- Build suff[0..K] where suff[i] is the score of the suffix starting after index 2i by inserting pairs from right to left, with suff[K]=0.
- Output max(pref[i]+suff[i] for i=0..K), corresponding to leaving the odd-indexed element A[2i] unmatched.
