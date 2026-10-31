- **Model:** Pairings are permutations `pi` from black order to white order. With path edges only forward, strong connectivity is equivalent to every proper prefix having an added edge entering it. For prefix before the `(a+1)`-st black, let `h[a]` be the number of whites before that black. Bad event `E_a`: `max(pi(1..a)) <= h[a]`.
- **Redundant events:** If `h[a] < a`, `E_a` is empty. If `h[a] = h[a-1]`, then `E_a` is contained in `E_{a-1}`. Keep only active `a` with `h[a] >= a` and `h[a] != h[a-1]` (or `a=1`).
- **Inclusion-exclusion recurrence:** For active indices `a_i` and `K_i=h[a_i]`, define signed grouped sums `A_i` by  
  `A_i = -invfact[K_i-a_i] * (fact[K_i] + sum_{p<i} A_p * fact[K_i-a_p])`.  
  Answer is `fact[N] + sum_i A_i * fact[N-a_i]`.
- **Active construction:** Scan the string once. When the `b`-th black is seen (`b>=2`), `a=b-1`, `h[a]=current white count`. Keep active if above condition holds. This avoids storing the full `h` array.
- **Early exits:** If `S[0]=='W'` or `S[-1]=='B'`, answer is `0`. For `N=1`, only `BW` survives and answer is `1`.
- **Full alternating fast path:** If `S == "BW"*N`, then `a_i=i`, `K_i=i`. The recurrence becomes the inverse of `F(x)=sum i! x^i`. Compute `B=1/F` up to `N` by Newton FPS and answer `fact[N] + sum_{i=1}^{N-1} B[i] fact[N-i]`.
- **Generalized consecutive fast path:** If active indices are consecutive `a_i = d+i` and `K_i = a_i+c` for constant `c`, recurrence is solvable with one FPS inverse of `F_j = fact[j+c]/fact[c]`. For `d=0`, `A_i=B_i`. For `d>0`, `A_i` is a short convolution of `F[0..d]` with `B`, computed directly if `d` is small or by NTT otherwise. This covers near-alternating worst cases.
- **CDQ convolution:** For general active lists, compute `A` online. In a node `[l,r)`, after solving left half, add to right half: `sum_{p in left} A[p] fact[K[i]-a[p]]`. This is a convolution with a shifted factorial array. Use direct loops for small products; otherwise NTT.
- **NTT details:** Mod `998244353`, primitive root `3`. Use in-place DIT/DIF pair without bit reversal, precomputed twiddle roots for all needed powers, and special-cased half=1/2 stages for speed.
- **Complexity:** Alternating and consecutive-constant cases are `O(N log N)`. General case is CDQ with NTT, roughly `O(M log^2 M)`, with adaptive direct cutoffs to reduce small-transform overhead.
- **Modulo safety:** Direct sums are accumulated as Python integers and reduced once per affected entry; values stay within a few hundred bits, which is safe and faster than per-addition modulo.
