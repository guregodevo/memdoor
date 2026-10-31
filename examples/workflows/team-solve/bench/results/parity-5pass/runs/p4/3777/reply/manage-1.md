### STATUS
continue

### NEXT
Run the provided examples and edge cases: all zeros, k=0, repeated ones, zeros enabling sums not reachable by positive-only subsequences, |k|>sum(nums), limit smaller than positive elements, and product 0 fallback; additionally brute-force random small cases against exhaustive enumeration and fix any mismatches.

### TASKS
- [done] Implement the full `Solution.maxProduct` using product-major positive bitset DP (snapshot per positive x, mask shifts, add single states) and separate zero-product reachability DP (no-zero/has-zero parity bitsets), with final scan for highest positive product then zero fallback.
- [todo] Run the provided examples and edge cases: all zeros, k=0, repeated ones, zeros enabling sums not reachable by positive-only subsequences, |k|>sum(nums), limit smaller than positive elements, and product 0 fallback; additionally brute-force random small cases against exhaustive enumeration and fix any mismatches.
