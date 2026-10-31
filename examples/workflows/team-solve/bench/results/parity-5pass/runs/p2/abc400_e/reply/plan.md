### PLAN
Precompute every 400 number up to 10^12 once, because Q is large and A is bounded. A 400 number has the form p^{2a}q^{2b}=(p^a q^b)^2 with distinct primes p<q, so it is enough to enumerate pairs of prime powers with product at most 10^6. Since p<q and pq<=10^6, the smaller prime p is below 1000, which makes the outer enumeration tiny. Sieve primes up to 10^6, then for each small p enumerate p^a, and for each larger prime q up to 10^6/p^a enumerate q^b up to that limit, storing the square. Sort the stored values and answer each query with a binary search for the largest value not exceeding A.

### TASKS
- Build a prime sieve up to 1,000,000 and store the primes; also identify primes <=1000 for the smaller factor.
- Generate all candidates: for each small prime p, iterate pp=p, p^2, ... <=10^6; for each prime q>p with q<=10^6//pp, iterate qq=q, q^2, ... <=10^6//pp and append (pp*qq)^2.
- Sort the candidate list, optionally deduplicating, so it is ready for binary search.
- Read Q and the Q values of A, then for each A compute idx=bisect_right(candidates, A)-1 and output candidates[idx].
- Verify correctness on edge cases: A=36, A=10^12, queries just below generated values, and ensure q is never equal to p.
