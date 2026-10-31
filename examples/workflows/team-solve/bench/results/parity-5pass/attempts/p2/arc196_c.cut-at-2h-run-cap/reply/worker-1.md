[cut off at the reply limit]
We need solve programming problem. Need output code and notes. Need deeply analyze. We have plan and task. Need implement Python solution. Need ensure correctness, performance N up to 2e5, mod 998244353. Need derive recurrence and CDQ NTT. Let's reason thoroughly.

Problem: directed path 1->2->...->2N. Add N edges from each white to matched black. Count perfect matching between W and B such that final graph strongly connected.

Plan says: base path gives reachability smaller to larger. Strongly connected iff 2N can reach 1, forcing S1=B, S2N=W. For matching, C_k = number of added edges from white > k to black <= k. Strong connectivity equivalent C_k>0 for every 1<=k<2N. First return of C to 0 at full prefix. Group white vertices by b = number of black vertices before them. e_b = number of white vertices before (b+1)-th black vertex, e_N=N. Desired answer H_N, recurrence H_b = (e_b! - sum_{c<b} H_c (e_b-c)!)/(e_b-b)! for valid b. Need compute H[0..N] by CDQ.

Need verify recurrence and definitions. Let's derive to ensure no off-by-one.

Vertices 1..2N. S_i W/B. Path edges i->i+1. Added edges W->B. Strongly connected: since path edges only forward, to go from larger to smaller need added backward edges. It suffices 2N reaches 1? If 2N can reach 1, then any i can reach 2N via path, then 1, then any j via path. Yes.

Consider prefix cut k: edges crossing from >k to <=k are added edges W in (k+1..2N) to B in (1..k). Path edges cross only <=k to >k. For 2N to reach 1, for every k, there must be at least one crossing edge from suffix to prefix? Actually to cross from suffix k+1..2N to prefix 1..k at some point, need an added edge from >k to <=k. If none, no path from any suffix vertex to prefix. Conversely if for every k there is at least one such edge, does 2N reach 1? Need maybe yes? Let's check: define C_k = # added edges from W>k to B<=k. If C_k>0 for all k<2N, then from 2N can follow path to some W? Wait 2N may not be white? But if S2N=W forced. To cross cut k, need an added edge from a white vertex >k to black <=k. But can we reach that white vertex from 2N? Since path goes forward only, from 2N cannot reach smaller white >k if white <2N. Hmm! Strong connectivity condition not simply C_k>0? Let's examine.

Base path edges i->i+1. From 2N, can only go to 2N (no outgoing path) plus added edge if 2N is white to some black. So to reach 1, first step must be added edge from 2N (if W) to some black b. Then from b can path forward to ... Need eventually cross cuts. The condition C_k>0 for all k maybe necessary but not sufficient? Let's test sample1 N=2 S=BWBW: vertices 1B,2W,3B,4W. Pair (2,3),(4,1): added 2->3 (forward), 4->1 (backward). C_1: W>1 to B<=1: 4->1 yes. C_2: W>2 to B<=2: 4->1 yes. C_3: W>3 to B<=3: 4->1 yes. Strong yes. Pair (2,1),(4,3): C_1: 2->1 yes; C_2: W>2 to B<=2? 4->3 no (3>2), 2 not >2 =>0, not strong. OK.

But consider S=BWBW, pair (2,1),(4,1) impossible matching? black 1 used twice no.

Need prove C_k>0 all k sufficient? Suppose C_k>0 for all k. From 2N (must be W) choose added edge to black b1 <=? Since C_{2N-1}>0, there is edge from W=2N? Actually W>2N-1 only vertex 2N, so C_{2N-1} counts added edge from 2N to B<=2N-1. Thus 2N has edge to some black b1<2N. Then from b1 path forward. Need reach 1. Consider current position x. If x=1 done. If x>1, need cross cut x-1? We are at x, can path forward to some white y>=x with added edge to black <=x-1? C_{x-1}>0 gives some white >x-1 to black <=x-1, but white may be <x? Wait >x-1 includes x..2N. We can path from x to any white y>=x. If the crossing edge's white y is >=x, then can reach it and jump to <=x-1, decreasing position. If all crossing edges for cut x-1 originate from white <x? impossible because >x-1 means >=x. So yes. Thus can iteratively decrease. So C_k>0 all k sufficient. Good.

Now count matchings such that C_k>0 all k.

We can model sequence of colors length 2N with N B, N W. For each white vertex, define b = # black vertices before it (0..N). For each black vertex, define w = # white vertices before it? Maybe.

Added edge from white with b black before to black with index j (1..N). The edge crosses cut k if white position > k and black position <= k. In terms of counts? For a white with b black before, its position is after b black and some white. For a black with index j, position after j-1 black and some white. Edge from white (b) to black j crosses cut k iff? Maybe condition for C_k>0 can be transformed to matching noncrossing? Let's derive recurrence.

Plan: group white vertices by b, number of black before them. Let e_b = number of white vertices before the (b+1)-th black vertex (e_N=N). So e_b is count of W among prefix ending just before B_{b+1}? For b=0..N-1. e_N=N total W. Since there are b black before (b+1)-th black, and e_b white before it. The (b+1)-th black position = b+e_b+1. e_b nondecreasing? As b increases, e_b can stay or increase. e_b >= b? Not necessarily? For valid strong, need S1=B => e_0=0? S1=B means before first black no W, e_0=0. S2N=W means after N-th black there is at least one W, so e_N=N > e_{N-1}? Actually e_{N-1} = #W before N-th black. If last vertex W, then e_{N-1} <= N-1? There is at least one W after N-th black, so e_{N-1} < N. e_N=N. Valid b requires e_b >= b? Let's see.

The recurrence H_b = (e_b! - sum_{c<b} H_c (e_b-c)!)/(e_b-b)! for b>0, e_b>=b, e_b>e_{b-1}. H[0]? Maybe H_0? Need base. If b=0? e_0=0 if S1=B. H_0? Recurrence for b=0 would divide by e_0! maybe? But H_0 maybe 1? Let's derive.

We count matchings of all N white to N black such that C_k>0 all k. There is known inclusion-exclusion / first return. H_b maybe number of "good" matchings for prefix? Let's derive to ensure recurrence and base.

Consider first b black vertices? Or prefix up to (b+1)-th black? e_b white before (b+1)-th black. The condition C_k>0 for all k up to position of (b+1)-th black -1? Maybe H_b counts matchings of first e_b white and first b black? Let's inspect recurrence: H_b uses e_b! and sum H_c (e_b-c)! divided by (e_b-b)!.

Suppose we consider prefix P_b consisting of first b black vertices and first e_b white vertices (all vertices before (b+1)-th black). There are e_b white, b black. Maybe H_b counts number of ways to match these e_b white to b black? But e_b may > b, so not perfect. Hmm.

Maybe H_b is number of ways to choose a "valid" partial matching of b black vertices to b of the e_b white vertices? The factor (e_b-b)! suggests remaining whites unmatched? Let's derive from combinatorics.

Condition C_k>0 all k. For each k, at least one added edge from suffix to prefix. This is equivalent to no prefix [1..k] is "closed" under added edges from suffix? Let's define for a matching, consider set of black vertices reachable? Another view: Since path forward, to reach 1 from 2N, we need a sequence of backward edges decreasing. The first time we cross from suffix to prefix? Maybe define minimal prefix that is closed? For a matching, let f be smallest k such that no added edge from >k to <=k. If none, good. If exists, let k be minimal? Then C_k=0 and for all i<k C_i>0? Actually if C_k=0, then no edge from suffix to prefix. For strong fail. The first return of C to 0? C_k as function? Let's compute C_k changes at vertices.

For each added edge W->B, it contributes +1 to C_k for k in [B, W-1] (if B<W) or 0 if B>=W? If edge forward (white before black), B>W, then no k with B<=k<W, contributes 0. If backward, contributes interval [B, W-1]. C_k = number of backward edges whose interval covers k. Strong iff C_k>0 all k=1..2N-1. So union of intervals [B, W-1] for backward edges covers all positions 1..2N-1. Forward edges irrelevant. Since 2N must be W and 1 B, at least one backward.

Counting matchings whose backward edge intervals cover line. This resembles covering by intervals with endpoints constrained by colors.

Group whites by b = #B before white. For a white with b black before, its position is after b black. A black j has position after j-1 black. Edge W(b) -> B(j) is backward iff position W > position B. In terms of b and j and white order? If white has b black before, black j has j-1 black before. If b < j-1, then white before black? Not necessarily if same black count, order by whites. But if b < j-1, definitely white before black (since fewer black before). If b >= j, maybe white after black? If b = j-1, depends on white order relative to black j. But black j is after e_{j-1} whites? Let's define e_b = #W before B_{b+1}. So black j (1-index) has e_{j-1} whites before it. A white with b black before: it is among whites after B_b and before B_{b+1}. There are e_b - e_{b-1} such whites (with e_{-1}=0? e_0 #W before B1). Its position relative to B_j: if b < j-1, before B_j; if b >= j, after B_j; if b = j-1, it is before B_j by definition (since e_b whites before B_{b+1}=B_j). So edge to B_j is backward iff b >= j? Because white after or at? If b=j-1, white before B_j, so forward. If b>=j, white after B_j (since at least j black before white, black j is before), so backward. Thus backward edge from white group b to black j possible only j <= b. Nice.

Interval [B_j, W-1] in terms of cuts k. Need cover all k. Maybe condition C_k>0 translates to for each b? Let's map cuts to black/white indices.

C_k>0 for all k. Consider cuts just before each black? Let k = position of B_{b+1} -1. Then prefix contains first b black and e_b white. C_k counts added edges from white >k (i.e. white groups >= b? Actually whites after B_b? Let's see: prefix before B_{b+1} includes whites with group < b? group c = #B before white. If c <= b-1, white before B_{b+1}; if c >= b, white after B_b, could be before B_{b+1}? Wait group b whites are between B_b and B_{b+1}; they are before B_{b+1}. So prefix before B_{b+1} includes groups 0..b. e_b = #W groups 0..b. Suffix after k includes groups b+1..N. Black <=k are first b black. So C_{pos(B_{b+1})-1} counts edges from white groups >= b+1 to black indices <= b. For strong, need >0 for b=0..N-1? For b=0, prefix before B1 includes no black if S1=B? C=0 automatically? Wait k before first black: if S1=B, k=0 not considered. For k=1? Let's be careful.

C_k for k from 1 to 2N-1. At cut just before B_{b+1}, prefix includes first b black and e_b white. If b=0, prefix before B1 has no black, C=0 but k maybe 0 if S1=B. If S1 not B, impossible. For b>=1, need C>0: at least one edge from white group > b? Actually white >k: groups >= b+1? Since group b whites are before B_{b+1}, so not >k. Black <=k: first b black. So need edge from group c>=b+1 to black j<=b. That is a backward edge with c>=j and c>=b+1.

Also cuts within blocks between black vertices. Suppose between B_b and B_{b+1}, there are whites group b. For cut after some of these whites, prefix includes first b black and t whites of group b. Suffix includes remaining group b whites and later. C counts edges from suffix whites (some group b after cut, plus groups >b) to first b black. Need >0. The minimum C in that block occurs after all group b whites? Actually as cut moves through group b whites, suffix loses whites, so C can decrease. The hardest cut is just before B_{b+1}, after all group b whites. So if C>0 there, then earlier in block? At start after B_b, suffix includes all group b and later; C larger. So condition at block ends suffices. Also cuts after black? Similar. Thus strong iff for every b=1..N-1? C before B_{b+1} >0, and C before B_N? Let's include b=N? Before B_N (b=N-1) need edge from groups >=N to first N-1 black. Also cut after B_N? Suffix includes group N whites (after last black), black <=k first N black. Need C>0 for cuts after B_N before end. Hardest just before end? Since S2N=W, group N whites after B_N. Cut before last vertex? Suffix includes last W, black all N. Need edge from last W to some black. This is b=N? C before B_{N+1} nonexistent; but e_N=N. Condition for b=N? Need edge from group N to black <=N? Since all black. For cut after B_N and before last W, suffix group N whites after cut; hardest just before last W? If last W has edge to black, C>0. This corresponds to b=N? Edge from group N to black <=N. So need for b=1..N: at least one edge from white groups >= b to black <= b-1? Wait for b=N, black <=N? Let's define condition D_b: for each b=1..N, there is at least one edge from white group >= b to black index <= b-1? For b=N, black <=N-1? But cut after B_N before last W: prefix includes all N black, suffix group N whites. Need edge to black <=N (all). That's black <=N, not N-1. Hmm.

Let's index differently. Let b = number of black in prefix at cut just before B_{b+1} for b=0..N-1. Need edge from white group >= b+1 to black <= b. For b=N-1: edge from group N to black <=N-1. But what about cut after B_N? prefix black N, suffix group N whites. Need edge from group N to black <=N. This is weaker? If there is edge from group N to black <=N-1, then also to <=N. But if only edge from group N to black N, then cut after B_N has C>0, but cut before B_N? Need edge from group N to black <=N-1 for b=N-1. So stronger. Since S2N=W, last W group N. If it points to black N (which is before it), then cut before B_N? black N is not <=N-1, so no. Could there be another group N white pointing to <=N-1? If yes. So condition b=N-1 covers after B_N? If edge to <=N-1, then after B_N also covered. If no, fail before B_N. Thus need for b=1..N-1: edge from group >=b+1 to black <=b. Plus S1=B, S2N=W. What about b=0? edge from group >=1 to black <=0 none, but cut before B1 k=0 not considered. If S1=B, first cut k=1? If B1 at pos1, cut after B1: prefix black1, suffix all W. Need edge from group >=1 to black1. This is b=1 condition. Good.

So strong iff S1=B, S2N=W, and for every b=1..N-1, there exists matched edge from a white in groups c>=b+1 to a black j<=b. Equivalently, for every b=1..N-1, not all whites in groups >=b+1 are matched to blacks >b.

This is like matching with forbidden "bad" prefix: For each b, the set of whites with group >=b+1 must not be entirely matched to blacks >b. Since counts: #whites group >=b+1 = N - e_b. #blacks >b = N-b. If N-e_b <= N-b => e_b >= b. If e_b < b, then by pigeonhole, at least one of those whites must match to black <=b, so condition automatically? Wait if fewer whites in suffix than blacks in suffix, cannot all match to suffix blacks, so automatic. If e_b >= b, possible to match all suffix whites to suffix blacks, causing bad. Need avoid for all b.

This resembles counting matchings avoiding a set of "closed" prefixes. Define for each b with e_b >= b, a bad event E_b: all whites with group >=b+1 are matched to blacks >b. Then strong iff no E_b for b=1..N-1? Also need S1=B (e_0=0) and S2N=W (e_{N-1}<N). If e_b < b, E_b impossible. If e_b =? Also if e_b not increasing? e_b can equal e_{b-1} (no whites between B_b and B_{b+1}). Then group b empty. Condition? If e_b=e_{b-1}, then e_b>=b maybe. But recurrence says H_b=0 unless e_b>e_{b-1}. Why? Let's see. If no white between B_b and B_{b+1}, then cut just after B_b? prefix black b, suffix includes group b empty and later. Need edge from groups >=b+1 to black <=b. Same as E_b complement. Maybe possible. Why H_b zero if e_b=e_{b-1}? Maybe H_b counts something for prefix up to B_{b+1} with at least one white in block? Need derive.

Let's derive recurrence via inclusion-exclusion / first bad event.

We count total matchings N! (bijection W to B). Bad events E_b for b where e_b >= b? E_b: all whites in suffix A_b = {groups >= b+1} matched to blacks in suffix B_{>b} = {b+1..N}. Sizes: |A_b| = N - e_b, |B_{>b}| = N - b. If |A_b| > |B_{>b}| (e_b < b), impossible. If |A_b| <= |B_{>b}|, number of matchings satisfying E_b: choose injection from A_b to B_{>b}: P(N-b, N-e_b) = (N-b)!/(b-e_b)!? And match remaining whites (groups <=b) to remaining blacks: (N - (N-e_b))! = e_b! ? Wait total whites N. If A_b fixed to distinct blacks in B_{>b}, remaining whites count e_b, remaining blacks count e_b. Any bijection: e_b!. So count = (N-b)!/(b-e_b)! * e_b! if e_b>=b. But recurrence H_b involves e_b! and (e_b-b)! not N. Maybe H_b is for prefix of size b? Let's see.

Maybe consider reversed/complement? The condition no E_b for all b. Events nested? A_b decreases as b increases? A_b = groups >=b+1. As b increases, A_b shrinks. B_{>b} shrinks. E_b not nested simply.

Could use inclusion-exclusion over subsets of b? Maybe due to chain structure, only minimal bad matters. Define first bad b? If E_b occurs, maybe all smaller? Let's check. If E_b: all whites groups >=b+1 matched to blacks >b. Does it imply E_c for c<b? For c<b, A_c includes A_b plus groups c+1..b. Need all those matched to blacks >c. Since A_b matched to >b >c. But groups c+1..b may be matched to black c+1..b? Not necessarily >c? They could match to black <=c, violating E_c. So not nested.

Maybe define "first return" of C to 0. C_k positive until some k then zero. If C_k=0 at cut before B_{b+1}, that means no edge from groups >=b+1 to black <=b, i.e. E_b. If C becomes zero at some cut, after that? C can become positive again? Intervals can start/end. C_k is coverage count. If zero at k, no interval covers k. Intervals are [B_j, W-1]. For k' > k, intervals covering k' may start after k, so C can become positive. So first zero not necessarily final. But strong requires no zero. Counting no zero maybe via inclusion-exclusion over zeros? Recurrence H_b maybe counts matchings where first zero at b? Let's explore.

Plan says: "strong connectivity is equivalent to C_k>0 for every 1<=k<2N, i.e. the first return of C to 0 is at the full prefix." Group white vertices by b... desired answer H_N, where H_b=0 for empty or impossible blocks and otherwise H_b=(e_b!-sum_{c<b} H_c (e_b-c)!)/(e_b-b)!.

This suggests H_b counts number of matchings of first e_b whites and first b blacks? with first return at b? Let's derive from a known combinatorial decomposition: For a sequence of W/B, count matchings such that for all prefixes? Maybe using "first return to zero" of a walk. e_b is #W before B_{b+1}. Condition e_b>=b? The recurrence resembles counting "indecomposable" objects: H_b = (e_b! - sum_{c<b} H_c (e_b-c)!)/(e_b-b)!.

Let's test with sample1 N=2 S=BWBW. Colors: B1 pos1, W group0? before B1 none e0=0. B2 pos3, W group1? between B1 and B2: vertex2 W => e1=1. B3? N=2 only B1,B2. After B2: vertex4 W group2 e2=N=2. e=[0,1,2]. Valid b: b=1 e1=1>=1 and e1>e0 (1>0). H1=(1! - sum none)/(1-1)! =1. b=2 e2=2>=2 and e2>e1 (2>1). H2=(2! - H1*fact[2-1=1])/(2-2)! = (2-1)=1. Output H2=1. Good.

Sample2 N=4 S=BWWBWBWB. Let's compute e. Positions: 1B,2W,3W,4B,5W,6B,7W,8B. B1 pos1 e0=0. B2 pos4: W before=2 e1=2. B3 pos6: W before=3 e2=3. B4 pos8: W before=4 e3=4. e4=N=4. Valid b: b=1 e1=2>=1, e1>e0 yes. H1=(2!)/(1)! =2. b=2 e2=3>=2, e2>e1 yes. H2=(3! - H1*fact[1])/(1)! = (6-2)=4. b=3 e3=4>=3, e3>e2 yes. H3=(4! - H1*fact[2] - H2*fact[1])/(1)! = (24 -2*2 -4)=16. b=4 e4=4>=4, e4>e3? 4>4 false => H4=0. Output 0. Matches.

Sample3 can test later maybe.

Need understand recurrence to implement confidently. Let's derive H_b.

Maybe H_b is number of "good" matchings for a truncated problem with b black and e_b white? But e_b may > b. Recurrence denominator (e_b-b)! suggests H_b*(e_b-b)! = e_b! - sum H_c (e_b-c)!.

Let F_b = H_b * (e_b-b)! maybe. Then F_b = e_b! - sum_{c<b} H_c (e_b-c)! = e_b! - sum_{c<b} F_c * (e_b-c)!/(e_c-c)!? Not simple. But recurrence as given.

Could H_b be number of matchings of e_b white vertices to b black vertices such that something, times (e_b-b)!? Let's attempt.

Consider prefix before B_{b+1}: has e_b whites, b blacks. Suppose we only care about matchings of these e_b whites to b of the b? Actually there are e_b whites and b blacks in prefix. If e_b>=b. Maybe H_b counts number of ways to match all b black vertices in prefix to b distinct white vertices in prefix such that the induced subgraph on prefix? and the remaining e_b-b whites in prefix are "unmatched" (will match outside). The factor (e_b-b)! could be permutations of remaining whites? Let's see.

For full strong matching, consider the first time (smallest b?) that all black vertices up to b are matched to white vertices up to e_b? Hmm.

Alternative: Use inclusion-exclusion over "bad prefix" E_b. Let G_b be number of matchings where for all c<b, not E_c, but E_b occurs? Then H_b maybe G_b? Recurrence H_b = (e_b! - sum H_c (e_b-c)!)/(e_b-b)! maybe counts matchings whose first bad event is b? Let's test.

Total matchings of e_b prefix whites to b prefix blacks? e_b!/(e_b-b)!? Wait number of injections from b black to e_b white = e_b!/(e_b-b)!. If we count matchings where all b black in prefix are matched to prefix whites (i.e. no edge from prefix black to suffix white? But edges are W->B, so black matched from some white. "All b black in prefix matched to prefix whites" means no white outside prefix matched to black <=b. That's exactly E_b? E_b: all whites outside prefix (groups >=b+1) matched to blacks >b. Equivalent: no outside white matched to prefix black. Since total outside whites N-e_b, outside blacks N-b. If all outside whites to outside blacks, then prefix blacks must be matched to prefix whites (because black needs a white). Yes E_b equivalent to all prefix black matched to prefix white. Count of full matchings satisfying E_b = choose injective assignment of b prefix black to e_b prefix white: P(e_b,b)=e_b!/(e_b-b)! times match remaining outside? Wait if prefix black matched to prefix white, remaining whites (N-b) include e_b-b prefix whites + N-e_b outside whites, remaining blacks N-b outside. Any bijection: (N-b)! . Total = P(e_b,b)*(N-b)! = e_b!/(e_b-b)! * (N-b)!.

But recurrence H_b denominator (e_b-b)! and numerator e_b! - sum H_c (e_b-c)!. If H_b were number of full matchings with first bad at b divided by (N-b)!? Let's see.

Let A_b = number of full matchings satisfying E_b and no E_c for c<b? (first bad b). Then A_b = (N-b)! * [P(e_b,b) - sum_{c<b} ?]. If first bad c<b, then E_c occurs. Does E_c imply something about prefix b? If E_c: all prefix c black matched to prefix c white. For E_b also, need all prefix b black matched to prefix b white. Given E_c, the remaining black c+1..b (b-c) must be matched to prefix b whites not used by first c? The first c black use c of e_c prefix c whites. There are e_b - c remaining prefix b whites available? But some of e_c-c unused prefix c whites also available. Count maybe H_c * (e_b-c)!? Let's derive.

Suppose we count assignments of prefix b black to prefix b white (injections) such that no earlier prefix c black all matched within prefix c? This is like counting injections with first "closed" prefix at b. Number of injections from b black to e_b white with first closed prefix b maybe = e_b!/(e_b-b)! - sum_{c<b} (number first closed c) * (e_b-c)!/(e_b-b)!? Let's compute.

Let I_b = number of injections f from B_prefix_b (size b) to W_prefix_eb (size e_b) such that for all c<b, not all B_prefix_c are mapped into W_prefix_ec. (i.e. first closed prefix is b). Total injections = e_b!/(e_b-b)!.

If first closed c, then B_prefix_c injected into W_prefix_ec in a "first closed c" way: count I_c. Then for remaining b-c black (c+1..b), they can map to any of the remaining e_b - c white vertices (since c used in W_prefix_ec; all other prefix b whites available, including unused W_prefix_ec and W between e_c+1..e_b). Number of injections for remaining = (e_b-c)!/(e_b-b)!? Wait total remaining black b-c, remaining white e_b-c, choose injective: P(e_b-c, b-c) = (e_b-c)!/(e_b-b)!.

Thus I_b = e_b!/(e_b-b)! - sum_{c<b} I_c * (e_b-c)!/(e_b-b)!.

Multiply by (e_b-b)!: I_b*(e_b-b)! = e_b! - sum I_c (e_b-c)!.

This matches recurrence if H_b = I_b? Yes H_b = I_b. Great. H_b counts injections from first b black vertices to first e_b white vertices such that no proper prefix c is "closed" (all first c black matched to first e_c white). Then full matchings with first bad event b? If first closed prefix b in injection, then E_b occurs and no E_c for c<b. For full matching, after fixing injection of prefix b black to prefix b white, remaining N-b black (outside) can be matched to remaining N-b white arbitrarily: (N-b)! ways. But wait if first closed b, does that guarantee E_b occurs? Yes prefix b black matched to prefix b white => outside whites matched to outside blacks? Since total outside whites N-e_b, outside blacks N-b. If prefix b black use b prefix whites, remaining prefix whites e_b-b must match to outside blacks (because outside whites count N-e_b, outside blacks N-b; total remaining whites N-b. It is possible some outside white matches prefix black? No prefix black already matched. So outside whites match outside blacks. Thus E_b. No earlier E_c by first closed. So number full matchings with first bad b = H_b * (N-b)!.

Strong matchings are those with no closed prefix b for b=1..N-1? What about b=N? If all N black matched to all N white (trivially) first closed N always? In full matching, prefix N black all matched to prefix N white (all) always, so first closed N always. Strong iff no closed prefix b for b=1..N-1. Thus number strong = total full matchings - sum_{b=1}^{N-1} H_b*(N-b)!? But plan says answer H_N. How does H_N relate? Let's see.

H_N counts injections from all N black to all N white (bijections) with no proper closed prefix c<N. That's exactly strong matchings! Because injection size N to e_N=N is bijection. First closed N always (all). No proper closed prefix => strong. So answer H_N. Great. Recurrence counts first closed prefix.

Now validity conditions: H_b=0 unless b>0, e_b>=b, and e_b>e_{b-1}. Why e_b>e_{b-1}? If no new white before B_{b+1}, then W_prefix_eb = W_prefix_e{b-1}. For injection first closed b: B_prefix_b to W_prefix_eb. But B_prefix_{b-1} to same W set. If no proper closed c? Could first closed b happen if e_b=e_{b-1}? Suppose e_b=e_{b-1}=e. Then W set same as prefix b-1. If B_prefix_b all mapped into W set, then B_prefix_{b-1} also all mapped into W set, so prefix b-1 closed. Thus first closed cannot be b. So H_b=0. Good. If e_b<b, injection impossible (not enough whites), H_b=0. If b=0? H_0? For recurrence c<b includes c=0? Need define H_0. In injection first closed 0? Empty prefix closed trivially? If include c=0, H_0? Recurrence for b: sum c<b H_c fact[e_b-c]. If H_0=1? Then H_b = (e_b! - 1*fact[e_b])/(e_b-b)! =0 for all, wrong. So H_0 should be 0 or not included. The empty prefix is closed but not considered as first bad? For b>0, first closed b means no c=1..b-1 closed. c=0 trivial but not subtract. So sum c=1..b-1. H_0=0. Good.

Need compute e_b. Input S length 2N. We need e_b for b=0..N. e_b = #W before (b+1)-th B for b=0..N-1; e_N=N. We can scan S, count W, when see B increment black count j (1-index). If j<=N, e_{j-1}=w_count. At end e_N=N. Need ensure S[0]=='B' and S[-1]=='W' else 0. If S[0]!='B', e0>0? Strong impossible because 1 must be black? Let's verify: If S1=W, vertex1 white. To reach 1 from 2N, need edge into 1 from some white. But vertex1 white cannot be black target, no added edge to 1, path edges into 1 none. So impossible. If S2N=B, vertex2N black no outgoing added edge (only path none), impossible. So early exit.

Also if N=1? S length2, must be BW. e0=0, e1=1. H1=(1!)/(0!)=1. Graph: 1B,2W, edge 2->1 plus path 1->2 strong. Output1. If S=WB early 0.

Now main challenge: compute H[1..N] modulo MOD for N up to 2e5, recurrence:
H[b] = (fact[e[b]] - sum_{c=1}^{b-1} H[c] * fact[e[b]-c]) * invfact[e[b]-b] mod MOD,
valid if b>0, e[b]>=b, e[b]>e[b-1] (with e[-1]? for b=0). For invalid H[b]=0.
Need e[b]-c nonnegative? If c<=b-1 and e[b]>=b, then e[b]-c >= e[b]-b+1 >=1? If e[b]=b, e[b]-c >=1 for c<b. OK. fact index up to N.

Naive O(N^2). Need CDQ divide and conquer with NTT. Recurrence is online: H[b] depends on previous H[c] with kernel fact[e[b]-c]. e[b] nondecreasing (since #W before later B nondecreasing). Valid b subset. Need compute all H.

Task gives detailed CDQ approach: leaf threshold 64, global sorted list of nonzero H indices. In apply(l,m,ql,qr), add contributions only to valid b in [ql,qr]; use direct loops when nonzero_count*valid_count is small, otherwise NTT-convolve H[c0..c1] with fact[t_min..t_max], where c0/c1 are first/last nonzero H in [l,m], t_min=max(0, e[first_valid_b]-c0-(c1-c0)), t_max=e[last_valid_b]-c0, and query conv[e[b]-c0-t_min]. Need implement correct fast NTT.

We need design robust CDQ. Let's formalize.

We have array H[0..N]. We compute in increasing b. CDQ solve(l,r): compute H in [l,r] assuming H[<l] known? Standard: solve(l,mid), then apply contributions from c in [l,mid] to b in [mid+1,r], then solve(mid+1,r). But H[b] also has base fact[e[b]] and invfact. We can initialize H[b] = fact[e[b]] for valid b? Then subtract contributions. Or maintain val[b] = fact[e[b]] - sum contributions; after all previous contributions, H[b]=val[b]*invfact[e[b]-b]. In CDQ, when applying from left to right, we need add H[c]*fact[e[b]-c] to val[b] (subtract). Then when reaching leaf b, compute H[b] from val[b]. But CDQ with threshold: for small segment, compute directly in order.

Need be careful: H[c] in left half must be finalized before applying. In solve(l,r): if r-l <= threshold: for b from l to r: if valid, H[b] = val[b]*invfact[e[b]-b] mod; else H[b]=0. But val[b] should already include contributions from all c<b outside segment? In CDQ, yes if we apply contributions from earlier segments. Within small segment, need include contributions from earlier b in same segment. We can do direct loops: for b in l..r: if valid, compute H[b] = (val[b] - sum_{c=l}^{b-1} H[c]*fact[e[b]-c]) * invfact... But val[b] currently includes contributions from c<l. Then update? We don't need update val for future? For b in segment, after computing H[b], for future b' in same segment, direct loop can sum H[c] on fly. Or update val[b'] directly. Simpler: in leaf segment, for b from l to r:
  if valid:
    s = val[b]
    for c in range(l,b): if H[c]!=0: s -= H[c]*fact[e[b]-c]
    H[b] = s*invfact[e[b]-b]
  else H[b]=0
But val[b] not updated for contributions from c in segment; that's fine because we compute directly. However, for future segments >r, they need contributions from all c in [l,r]. The CDQ apply after solve(l,r) to right segments will use H[c] finalized. It doesn't require val updated. Good.

But if we use threshold and solve(l,r) recursively, we need apply contributions from left half to right half before solving right half. For right half's val array, we add contributions from c in left half. Then solve right half. For small segment, val includes contributions from all c<l (including left halves applied). Direct internal contributions handled.

Alternative: initialize val[b]=fact[e[b]] for valid b (or 0 invalid). In apply(l,m,ql,qr), for each c in [l,m] with H[c]!=0, for each valid b in [ql,qr] (b>m), val[b] -= H[c]*fact[e[b]-c]. Then solve right.

Need ensure contributions only to b where e[b]-c >=0. Since b>c and valid e[b]>=b, e[b]-c >=1. For invalid b maybe no need. But if e[b]<b, H[b]=0 and no future? It could still receive contributions? Not needed. We can restrict to valid b (e[b]>=b and e[b]>e[b-1]) because H[b] zero otherwise. But wait if invalid due to e[b]=e[b-1], H[b]=0, but could it be a c for future? H=0 so no. So skip.

Need global sorted list of nonzero H indices? H values computed modulo; could be zero even if valid. For apply, need nonzero H in [l,m] to avoid loops. We can maintain list nonzero_indices sorted as H computed. Since CDQ computes in increasing order? CDQ solve(l,r) computes left then right, so H indices finalized in increasing order overall? Yes standard CDQ: solve(l,mid) finalizes all in left, then apply, then solve right. Thus when H[b] computed, all smaller finalized. We can append b to nonzero list if H[b]!=0. But in recursive apply, left half H known. To get nonzero in [l,m], we can use bisect on global list. However, during solve(l,r), some indices in [l,m] may not be finalized? In apply called after solve(l,m), so yes finalized. Global list contains all finalized nonzero up to m (and maybe some >m? No, because right not solved yet; but due to recursion, could there be finalized indices >m from other branches? CDQ order: solve(1,N): solve(1,mid) finalizes 1..mid, apply, solve(mid+1,N). Within solve(mid+1,N), it finalizes mid+1.. etc. There is no finalized index > current r before solving? In a recursive call solve(l,r), before it is called, all indices <l finalized, indices >=l not. Inside, after solve(l,m), indices l..m finalized. No indices >m finalized yet in this call. But globally, if we are in a right branch, some indices in earlier right subsegments? Still all finalized indices are < current r? Let's see solve(5,8): called after 1..4 finalized. It solves 5..6, then apply, then 7..8. No >8 finalized. So global nonzero list sorted and contains exactly finalized indices < current segment's right? For apply(l,m,ql,qr) with ql=m+1, qr=r, all nonzero in [l,m] are in list; no nonzero >m yet. But if we use bisect, fine.

However, if we use leaf threshold and direct compute, we append as we go. Good.

But task mentions "global sorted list of nonzero H indices" and apply(l,m,ql,qr) add contributions only to valid b in [ql,qr]; use direct loops when nonzero_count*valid_count is small, otherwise NTT-convolve H[c0..c1] with fact[t_min..t_max], where c0/c1 are first/last nonzero H in [l,m], t_min=max(0, e[first_valid_b]-c0-(c1-c0)), t_max=e[last_valid_b]-c0, and query conv[e[b]-c0-t_min]. Need implement this.

We need design NTT convolution for arbitrary ranges and sparse indices.

Recurrence contribution from c in [l,m] to b in [ql,qr]: term H[c] * fact[e[b]-c]. e[b] nondecreasing with b. c range contiguous [l,m] but nonzero sparse. b valid subset contiguous? valid b in [ql,qr] maybe not all, but we can get list valid_b. e[b] nondecreasing. We need for each valid b, sum over nonzero c in [l,m] with c<b (true since c<=m<ql<=b) H[c]*fact[e[b]-c]. This is like convolution of sequence A indexed by c and B indexed by e[b]? Let x = c, y = e[b]. Need sum A[c] * fact[y-c]. If we define A[c]=H[c], F[t]=fact[t]. Then sum_c A[c] F[y-c]. This is convolution (A * F)[y] if A indexed from 0 and F from 0. Specifically conv[k] = sum_i A[i] F[k-i]. For y=e[b], sum_c H[c] fact[e[b]-c] = conv[e[b]] if A[i]=H[i] for i in range, F[j]=fact[j]. But c range [l,m], e[b] up to N. We can convolve segment A over c0..c1 with fact over needed t range.

Because c range may start c0>0. Let A'[i]=H[c0+i] for i=0..L-1 where L=c1-c0+1. Let F'[j]=fact[t_min+j] for j=0..M-1. Then conv'[k] = sum_i A'[i] F'[k-i] = sum_c H[c] fact[t_min + k - (c-c0)] = sum_c H[c] fact[t_min + k - c + c0]. We want fact[e[b]-c]. Set t_min + k - c + c0 = e[b]-c => k = e[b]-c0 - t_min. So query conv'[e[b]-c0-t_min]. Matches task.

Need choose t_min, t_max such that for all valid b in [ql,qr] and all c in [c0,c1]? Actually convolution with F range must cover all fact indices e[b]-c that may be nonzero. Since c in [c0,c1], e[b] in [e_first, e_last] (nondecreasing). The needed fact index t = e[b]-c ranges from min over b,c: e_first - c1 to max: e_last - c0. But if t<0, fact negative not used (term zero). Since c<b and e[b]>=b, t>=? For c<=m<b, e[b]-c >= e[b]-m. Could be 0? If e[b]=b and c=b? c<b so >=1. But if c0 maybe >? t positive. However if c1 > e[b]? Could e[b]-c1 negative for small b and large c. But c1<=m<ql<=b, and e[b]>=b, so e[b]-c1 >= b-c1 >=1. So nonnegative. Wait if valid b e[b]>=b, c1<=m<b, so e[b]-c1 >= b-c1 >=1. So no negative for valid b. But if we include c beyond? c1 last nonzero in [l,m] <=m. So t_min = e[first_valid_b] - c1? But task says t_min=max(0, e[first_valid_b]-c0-(c1-c0)) = e[first_valid_b]-c1. Yes. t_max=e[last_valid_b]-c0. Good.

But if there are no valid b in [ql,qr] or no nonzero c, skip.

Direct loops threshold: if nz_count * valid_count <= some threshold (maybe 4096 or 8192) do nested loops. Need choose for Python performance. NTT overhead high. N=2e5. CDQ levels O(log N). If we NTT at many nodes, total O(N log^2 N)? Need optimize.

Standard CDQ with convolution at each node: total length of convolutions O(N log N) if using full ranges? But here ranges maybe smaller. Need ensure Python can handle. NTT in Python for 2e5 maybe okay if optimized? Need careful.

Alternative maybe use divide and conquer with NTT at each node, total O(N log^2 N) due to CDQ? Let's analyze. At each level, segments disjoint. Convolution length for node roughly (c1-c0+1)+(t_max-t_min+1). c range length <= segment length, t range length <= e[last]-e[first] maybe <= N. Sum over nodes at a level? c ranges disjoint? Nonzero c ranges within left halves disjoint across nodes at same level. t ranges for right halves maybe e ranges disjoint? e nondecreasing, right halves disjoint in b, so e ranges mostly disjoint (can overlap if e constant, but valid requires e increasing; invalid skipped). Sum lengths O(N + #nodes*?) maybe O(N log N). NTT each node length power of two. Sum of ntt lengths over nodes per level maybe O(N log N)? Actually if many small nodes, overhead. Use threshold to direct small.

Python NTT: Need implement iterative Cooley-Tukey. Mod 998244353 primitive root 3. Need fast enough for maybe total ~ few million? Could be okay if optimized. But many NTT calls could be heavy. Need tune threshold.

Maybe there is a simpler O(N log N) using generating functions? Let's explore to possibly avoid complex CDQ? Recurrence H_b = (fact[e_b] - sum_{c<b} H_c fact[e_b-c]) / fact[e_b-b]. e_b nondecreasing, increments 0/1? e_b is #W before B_{b+1}; as b increases by 1, e_b can increase by number of W between B_b and B_{b+1}, possibly >1. But total increments N. Valid requires increment >0. So valid b are exactly black indices where there is at least one W before next B? e_b>e_{b-1}. Number of valid b <= N. e_b values strictly increasing on valid b. Could map valid b to distinct e values. Maybe recurrence can be transformed to simpler convolution over e? Let's investigate.

Let valid b have e_b = x. Since e_b strictly increasing for valid b, and e_b>=b. Invalid H=0. For valid b, denominator fact[e_b-b]. Recurrence:
H_b * fact[e_b-b] = fact[e_b] - sum_{c valid <b} H_c fact[e_b-c].
Let K_b = H_b * fact[e_b-b]? Then K_b = fact[e_b] - sum_{c<b} K_c * fact[e_b-c] / fact[e_c-c]. Not a simple convolution because weight depends on c. But maybe define G_c = H_c / fact[e_c-c]? Then H_c fact[e_b-c] = G_c fact[e_c-c] fact[e_b-c]. Not convolution.

Maybe use e_b as index. Since e_b strictly increasing, c < b implies e_c < e_b. But c not equal e_c. Kernel fact[e_b-c] uses c (black index), not e_c. So CDQ over b natural.

Could use online convolution with arbitrary e? Maybe not.

Need implement CDQ NTT robustly.

Let's detail CDQ algorithm.

Precompute fact[0..N], invfact[0..N]. MOD=998244353.

Input N, S.
If S[0]!='B' or S[-1]!='W': print(0).
Compute e array length N+1. e[0..N-1] from scan; e[N]=N.
But if early exit, no need.
Define valid[b] for b=1..N: e[b]>=b and (b==0? no) e[b]>e[b-1]. For b=N, e[N]=N, need e[N]>e[N-1] (last W after last B) which is ensured by S[-1]=='W'? If last vertex W, then e[N-1] <= N-1, so yes. But if there are multiple W after last B, e[N-1]<N. Good. For b where e[b]<b invalid. Also if e[b]==e[b-1] invalid.
Initialize val = [0]*(N+1). For b=1..N if valid[b]: val[b]=fact[e[b]] else 0. H=[0]*(N+1). nonzero=[] (list of indices with H nonzero). valid_indices maybe list of b valid for quick range queries.

Need apply(l,m,ql,qr): add contributions from c in [l,m] to b in [ql,qr]. But only c with H[c]!=0 and b valid. Since H[c] may be zero. Use nonzero list to get c indices in [l,m]. Use valid_indices to get b indices in [ql,qr]. If either empty return.

Direct if len(c_list)*len(b_list) <= DIRECT_THRESHOLD. For each c in c_list: hc=H[c]; for b in b_list: val[b] = (val[b] - hc*fact[e[b]-c]) % MOD. To optimize, avoid modulo each inner? Use local variables, maybe accumulate in Python int and mod after? Values can grow: threshold maybe 4096, product mod ~1e9, sum 4096*1e18 ~4e21, Python big int okay but maybe slower. We can do val[b] = (val[b] - hc * fact[...]) % MOD each. Or batch: for c, for b, val[b] -= hc*fact; after loops mod all touched? But val used later; can allow negative large. If many direct updates, val magnitude could grow O(updates*MOD^2) if not modded. Better mod periodically. Simpler modulo each update. Threshold small.

NTT case:
- c_list = nonzero indices in [l,m]. c0=c_list[0], c1=c_list[-1]. L = c1-c0+1.
- b_list = valid indices in [ql,qr]. first_b=b_list[0], last_b=b_list[-1]. e_first=e[first_b], e_last=e[last_b]. t_min = e_first - c1 (should >=0; max 0). t_max = e_last - c0. M = t_max - t_min + 1.
- Build A length L: A[i]=H[c0+i] (could zeros inside). Build F length M: F[j]=fact[t_min+j].
- Convolve A and F. Need conv length L+M-1. For each b in b_list: idx = e[b] - c0 - t_min. term = conv[idx] (if 0<=idx<len). val[b] = (val[b] - term) % MOD.

But is it sufficient to use c0..c1 full range including zeros? Yes. F range t_min..t_max covers all needed fact indices for all c in c0..c1 and b in b_list? For a given b, c may produce t=e[b]-c. Minimum t for b is e[b]-c1 >= e_first-c1=t_min. Maximum e[b]-c0 <= e_last-c0=t_max. Good. Convolution query idx as above. Terms where t outside F range? For b and c in ranges, t within. Good.

Potential issue: c_list may not include all c in [l,m] if H zero. c0/c1 first/last nonzero. Zeros inside A okay. But what about c in [l,m] before c0 or after c1 with H=0 no contribution. Fine.

Potential issue: b_list valid in [ql,qr] but e[b] may not be strictly increasing? valid ensures e[b]>e[b-1], but if invalid between, e can stay. For valid b list, e strictly increasing? If b1<b2 both valid, e[b1] < e[b2]? Since e nondecreasing and each valid step increases from previous b, but if invalid in between e may stay; still e[b1] <= e[b2], and if b2 valid, e[b2]>e[b2-1] >= e[b1]? If b2-1 invalid maybe e[b2-1]=e[b1], then e[b2]>e[b1]. So yes strictly increasing among valid. Good.

Need choose NTT length n = power of two >= L+M-1. But L and M can be up to N. Many calls. Need optimize memory/time.

NTT implementation details:
MOD=998244353, G=3.
Need ntt(a, invert). Iterative bit-reversal? Python performance. Common optimized:
def ntt(a, invert):
    n=len(a)
    j=0
    for i in range(1,n):
        bit=n>>1
        while j&bit:
            j^=bit; bit>>=1
        j^=bit
        if i<j: a[i],a[j]=a[j],a[i]
    len_=2
    while len_<=n:
        wlen=pow(G,(MOD-1)//len_,MOD)
        if invert: wlen=pow(wlen,MOD-2,MOD)
        for i in range(0,n,len_):
            w=1
            half=len_>>1
            for j in range(i,i+half):
                u=a[j]; v=a[j+half]*w%MOD
                x=u+v; if x>=MOD: x-=MOD
                y=u-v; if y<0: y+=MOD
                a[j]=x; a[j+half]=y
                w=w*wlen%MOD
        len_<<=1
    if invert:
        inv_n=pow(n,MOD-2,MOD)
        for i in range(n): a[i]=a[i]*inv_n%MOD

This is okay but bit-reversal O(n log n)? Actually O(n). pow per stage O(log MOD) small. For many NTT calls, pow overhead maybe. Could precompute roots? But maybe okay. Need optimize for Python.

Alternative use iterative with precomputed bit-reversal? For varying n, not. Could use cp-alg style.

Need consider total NTT calls. If threshold too low, many small NTTs slow. If threshold too high, direct O(N^2) maybe. Need tune.

N=2e5. CDQ levels ~18. At each level, number of nodes ~2^level. For large nodes, NTT. Suppose threshold 64 for segment length. Leaves direct. Apply called for each internal node. Number of NTT calls maybe O(N/threshold) ~3125 if threshold 64? Actually internal nodes with segment length >64: about N/64 + N/128 + ... ~ O(N/64) ~3125. That's too many NTT calls, even small length ~128, overhead huge. Need higher threshold or optimize apply to skip if small. Direct threshold for product maybe. But number of NTT calls still maybe large if we NTT for any node above leaf. Need choose leaf threshold larger, e.g. 512 or 1024, to reduce calls to ~200-400. But direct cost within leaves O(threshold^2 * N/threshold) = O(N*threshold). If threshold 512, ~1e8 operations too high in Python. But direct internal contributions in leaf segment: for each b, sum over previous in segment nonzero. If all nonzero, ~T^2/2 per leaf, N/T leaves => N*T/2. T=512 => 51 million multiplications, maybe borderline but okay? Plus apply direct/NTT. T=256 =>25M. Need balance.

But apply direct threshold product can handle small nodes without NTT. For nodes with length ~T, product ~T^2/4. If direct threshold e.g. 4096, nodes up to length ~128 direct. NTT calls for larger. Leaf threshold maybe 64? Then direct internal 6.4M, okay. NTT calls ~N/64=3125 too many. But many apply nodes may have no nonzero or no valid? Worst-case all valid and H nonzero. Need reduce NTT calls.

Maybe use a different CDQ strategy: only NTT at nodes where product large, but for small product direct. If leaf threshold 64, apply for node length 128: left nonzero ~64, right valid ~64 product 4096. If direct threshold 4096, direct. Node length 256: product ~16384 -> NTT. Number of NTT nodes at length 256: N/256=781. Still many. Length 512: 390, etc total ~1560 NTT calls. Too many.

Need optimize by using larger leaf threshold or direct threshold. If leaf threshold 512, internal apply at length 1024 product ~262k -> NTT; number NTT nodes ~N/1024=195 plus larger ~390 total. Direct internal leaf cost N*T/2=51M. Maybe okay in Py if optimized? 51M modular multiplications maybe too slow (5-10s?). Need better.

Maybe we can compute leaf segments using NTT too? Or use direct with threshold product and leaf threshold maybe 256. NTT calls ~? If leaf 256, apply length 512 product 16k. If direct threshold 16384, direct; length 1024 product 65k NTT. NTT nodes ~N/1024=195 + ... ~390. Direct leaf cost 25M. Maybe acceptable? 390 NTT calls of varying sizes. Total NTT work? At level length 1024, 195 convolutions length maybe ~? L~512, M~? e range maybe up to 512? n~1024 or 2048. 195*2048 log ~195*22528 ~4.4M butterfly. Next level 97*4096*12 ~4.7M. Sum maybe ~50M butterflies. Python 50M maybe high but possible with optimized? Plus direct 25M. Need maybe okay if time limit unknown (AtCoder? Python maybe 2 sec? Need more efficient).

Could there be a more efficient O(N log N) single convolution? Let's explore deeper. Maybe recurrence can be solved with one NTT using generating functions and e_b monotonic? Let's attempt.

H_b = (fact[e_b] - sum_{c<b} H_c fact[e_b-c]) / fact[e_b-b]. Let valid b. Define A_b = H_b * invfact[e_b-b]? Or something to make convolution.

Let P_b = H_b / fact[e_b-b]? Then H_c fact[e_b-c] = P_c fact[e_c-c] fact[e_b-c]. Not convolution.

Let Q_b = H_b * fact? We have H_b fact[e_b-b] = fact[e_b] - sum H_c fact[e_b-c]. This is like for each b, sum over c of H_c fact[e_b-c] + H_b fact[e_b-b] = fact[e_b]. If we define sequence over b? Kernel fact[e_b-c] where e_b is nondecreasing but not equal b. If e_b=b for all b (alternating BWBW...), then recurrence H_b = (b! - sum_{c<b} H_c (b-c)!)/(b-b)! = b! - sum H_c (b-c)!. This is convolution over b: H_b + sum_{c<b} H_c (b-c)! = b!. Let K_d = d! for d>=1, K_0? Then H_b = b! - (H*K)[b] with H_0? Could solve via generating functions: H(x) = fact(x) - H(x)*K(x). One NTT maybe. But e_b arbitrary.

Maybe transform by e_b: Since valid b has e_b > e_{b-1}, and e_b>=b. Let d_b = e_b - b >=0. Denominator fact[d_b]. Recurrence:
H_b fact[d_b] = fact[e_b] - sum_{c<b} H_c fact[e_b-c].
Let c valid. e_b-c = (e_b-b) + (b-c) = d_b + (b-c). Not only d.

Could use offline divide and conquer is standard.

Maybe use Fenwick/segment tree if e_b increments small? No.

Need implement CDQ NTT efficiently enough. Let's consider using PyPy. AtCoder mod 998244353, N=2e5. Python NTT CDQ might be tight but possible with optimizations and threshold. Need produce correct code, not necessarily fastest? But should pass. We need aim robust.

Could use numpy? Not allowed likely. Pure Python.

Maybe there is a combinatorial closed form? Let's investigate problem more. Counting matchings such that intervals cover. Maybe answer has simple formula in terms of e_b? Sample3 output 240792. Could be product? Let's test small.

Strong iff no closed prefix. H_N counts bijections with no closed prefix. This is like number of permutations avoiding certain "records" based on e_b. e_b is a lattice path (B steps? W steps?). Maybe H_N equals determinant or product via Lindstrom? Let's explore.

We have N black positions, e_b #W before B_{b+1}. Condition for a bijection f: black j -> white f(j) (white index 1..N in order). Edge from white f(j) to black j. Closed prefix b: f(j) <= e_b for all j<=b. Strong iff for all b=1..N-1, exists j<=b with f(j)>e_b. Equivalently, for all b, max_{j<=b} f(j) > e_b. Since f is permutation of 1..N. This is a nice condition! Let's verify: prefix b black matched to prefix e_b white iff all f(j)<=e_b for j<=b. Bad E_b. Strong iff for every b<N, max_{j<=b} f(j) > e_b. Also need S1=B => e_0=0, S2N=W => e_N=N. e_b nondecreasing, e_b>=? If e_b<b, condition automatic because max of b distinct positive <=? Actually max_{j<=b} f(j) >= b (since b distinct numbers), so if e_b < b, automatic. If e_b>=b, need max > e_b.

So count permutations f of [N] such that prefix maximum M_b = max(f(1..b)) > e_b for all b=1..N-1. At b=N, M_N=N=e_N. This is reminiscent of permutations with prescribed lower bounds on prefix maxima. e_b is a nondecreasing sequence from 0 to N. Count permutations where prefix max exceeds e_b. This might have simpler formula! Let's analyze.

We need count permutations π of 1..N such that for all b, max_{i<=b} π_i > e_b. Since e_b nondecreasing integer 0..N. If e_b < b, condition automatic because max >= b > e_b. If e_b >= b, need at least one of first b positions contains a value > e_b.

Equivalently, the first time the prefix maximum reaches value x? Let's define for each value v, position p_v = π^{-1}(v). Prefix max at b is max value among positions <=b. Condition max_{i<=b} π_i > e_b means there exists value v > e_b with p_v <= b. For each b, min position among values > e_b must be <= b. Let q_t = min_{v>t} p_v (first position of a value >t). Condition for b with e_b=t: q_t <= b. Since e_b nondecreasing. Maybe count via positions of records.

Permutation prefix maxima process: The sequence of left-to-right maxima. Values that are new maxima. Condition relates to e_b. Maybe count = product over something? Let's test.

For any permutation, define M_b. M_b is nondecreasing, M_b >= b, M_N=N. Condition M_b > e_b. Since e_b nondecreasing. Count permutations with M_b > e_b. There is known formula: number of permutations with M_b <= k for all b? That's rook polynomials? The event M_b <= e_b for all b? We need avoid all? Actually condition M_b > e_b for all b. The complement is exists b with M_b <= e_b. Since M_b nondecreasing and e_b nondecreasing, the set of b where M_b <= e_b maybe prefix? Let's check. M_b nondecreasing, e_b nondecreasing. Difference not monotonic. But if M_b <= e_b at some b, does it imply for smaller? Not necessarily. Example e=[0,2,2], M=[1,1,3]? M_2=1<=2, M_1=1>0. Could be. But first bad b in H recurrence corresponds to first b where M_b <= e_b? Wait closed prefix b: all first b values <=e_b, i.e. M_b <= e_b. First closed prefix b means M_b <= e_b and for c<b, M_c > e_c. H_b counts permutations with first closed b? Let's see H_b*(N-b)!? H_b injections first closed b. For full permutations, first closed b count = H_b*(N-b)!? Maybe.

Counting permutations with no M_b <= e_b for b<N. This is like permutations avoiding a "forbidden region" under path e_b. Could be counted by determinant / hook-length? Let's explore.

Condition M_b > e_b. Since M_b is max of first b. Equivalent: For each b, among values e_b+1..N, at least one appears in first b positions. Let r_t = min position of values >t. Need r_{e_b} <= b. Since e_b nondecreasing, it suffices to check b where e_b increases? If e_b constant t over interval b1..b2, condition r_t <= b for all b in interval; hardest smallest b? r_t <= b1. b1 is first b with e_b=t? But e_b may start at 0. For b with e_b=t, need r_t <= b. Since b increases, if holds at first b with e_b=t, holds later. So only need check b where e_b changes? Specifically for each t, let b_t = min b such that e_b >= t? Or e_b = t? Condition for b with e_b=t: r_t <= b. The minimum b with e_b=t (if any) is hardest. If e_b jumps over t, then for b with e_b>t, condition r_{e_b} <= b implies r_t? Since values >e_b subset of >t, r_{e_b} >= r_t? Actually min over smaller set >= min over larger set. r_{e_b} <= b does not imply r_t <= b? r_t <= r_{e_b} <= b, yes because set >t larger, min <=. So if condition holds at b with e_b=t', then for all t<=t', r_t <= b. But need at earlier b maybe.

Maybe condition equivalent to for all b, b >= f(e_b) where f(t)=min position of value >t. Counting permutations by positions of suffix minima.

Another view: Permutation π. Define for each position i, value π_i. The prefix max M_i. The condition M_i > e_i. Since e_i is a path. Count permutations whose prefix max path stays strictly above e_i. This is similar to "permutations with bounded prefix maxima" and can be counted by dynamic programming over maximum value. Let dp[i][m] = # permutations of first i positions with max=m and satisfying. Transition: choose new max? But N 2e5, e arbitrary. Maybe can derive O(N log N).

State: after i positions, max = m. To satisfy at i, need m > e_i. Number of permutations of length i with max m and condition. Standard: number of permutations of first i with max m = C(m-1,i-1)*(i-1)!? But condition history.

Maybe process values descending. Let p_v position of value v. Condition r_t <= b_t. r_t = min(p_{t+1},...,p_N). This is suffix minimum of positions. Count permutations of positions p_1..p_N (a permutation) such that suffix minima constraints. For each t, min_{v>t} p_v <= b_t (where b_t maybe min b with e_b>=t?). This is like parking functions? Could be counted by product of (something) using inclusion-exclusion. Let's investigate.

Let e_b nondecreasing. Define for each value threshold t, constraint: at least one of values t+1..N appears in first b_t positions, where b_t = min{b: e_b >= t}? Because if e_b reaches t at b_t, need r_t <= b_t. For t=0, e_b>=0 always b=1? r_0 = min all positions =1 <=1 trivial. For t=N, no values >N, constraint impossible? But b=N? e_N=N, condition at b=N not required; for t=N, no values, ignore. For t from 0 to N-1, need r_t <= b_t. Since e_b may skip? e_b increments by number of W between blacks, can jump >1. If e_b jumps from a to b>a, then for t=a+1..b, b_t = same b (first b with e_b>=t). Constraint r_t <= b. But r_t nonincreasing? As t increases, set >t smaller, r_t nondecreasing. Constraints nested.

Counting permutations p_v (positions of values) such that for all t, min_{v>t} p_v <= b_t. This is equivalent to for each t, not all values >t are in positions >b_t. That is, the set {t+1..N} is not subset of {b_t+1..N}. Since sizes: N-t values, N-b_t positions. If N-t > N-b_t (t<b_t), automatic. If t>=b_t, need at least one. This matches e_b>=b.

This resembles counting permutations with forbidden suffixes: For each t, the last N-b_t positions cannot contain all values >t. Inclusion-exclusion over t? Events A_t: all values >t are in positions >b_t. These events are nested? If A_t occurs (values t+1..N in suffix after b_t), then for s<t, values s+1..N include A_t plus values s+1..t. They could be in prefix, so A_s not implied. For s>t, values s+1..N subset of values t+1..N; if A_t, they are in suffix after b_t, but A_s requires suffix after b_s (b_s >= b_t? Since t<s, b_t <= b_s? e nondecreasing, first reach t earlier, so b_t <= b_s). Suffix after b_s is smaller subset of suffix after b_t. A_t does not imply A_s. So not nested.

Maybe use rook polynomial / determinant: Count permutations π_i with π_i >? Condition M_i > e_i means not (π_1..π_i all <=e_i). This is a set of forbidden patterns: for each i, the first i entries are a subset of [e_i]. This is equivalent to permutation matrix avoiding a Young diagram? Let's map: π_i is row i, column value. Condition M_i > e_i means in first i rows, there is a column > e_i. Bad if all first i rows have columns <= e_i. This is like the permutation matrix's prefix row maxima. Counting permutations with prefix maxima > boundary. There may be formula using inclusion-exclusion over "records" and determinant of triangular matrix. H recurrence already is inclusion-exclusion over first bad prefix. Maybe can solve faster using generating functions if e_b arbitrary? CDQ likely intended.

But maybe H_N can be computed by a simple product over b where e_b increases: Let's test small examples.

N=2, e=[0,1,2] answer1. Product? (e_b - e_{b-1})? no.
N=3 possible e sequences with S1=B,S6=W. Let's enumerate some and compare recurrence.
Case e=[0,1,2,3] (BWBWBW). Recurrence:
H1=1
H2=(2!-H1*1!)=1
H3=(3!-H1*2!-H2*1!)=6-2-1=3. Answer 3? Let's brute mentally permutations π of 3 with M1>0 (always), M2>1 (first two not both <=1 impossible? values distinct, M2>=2>1 automatic), M3 not required? Wait b=1..N-1: b=1 e1=1: M1>1? π1>1. b=2 e2=2: M2>2? max first two >2, i.e. one of first two is 3. Count permutations where π1>1 and 3 in first two. Total? π1=2 or3. If π1=3: remaining 2! =2. If π1=2: need π2=3, remaining 1 =>1. total3. OK.
Case e=[0,2,2,3] (S? B W W B W B? N=3: B1, W,W, B2, W, B3? last B not W invalid; need S6 W so e2? Let's make e=[0,2,3,3]? last W? e3=N=3, e2=3 invalid e3>e2 false. Need e2<3. e=[0,2,2,3] means B1, W,W, B2, B3? no W between B2 and B3? e2=2, then B3, then W? length 6: B W W B B W -> S1 B, S6 W, N=3. e0=0, e1=2, e2=2, e3=3. Recurrence: valid b1 e1=2>0 H1=2!/1!=2. b2 e2=2 not >e1 =>0. b3 e3=3>2 H3=(3! - H1*fact[1])/(0!)=6-2=4. Count permutations M1>2? π1>2 => π1=3. M2>2? max first2>2 already. total permutations with π1=3:2! =2? Wait recurrence says 4. Did I mis condition? e1=2, b=1: need M1 > e1=2, so π1=3 only. b=2: e2=2, need M2>2, already if π1=3. So count 2. Why recurrence H3=4? Let's check e sequence validity: S=B W W B B W. Colors: B1 pos1, W2 pos2, W3 pos3, B2 pos4, B3 pos5, W4? Wait N=3 means 3 W and 3 B. String B W W B B W has W at 2,3,6 (3 W), B at1,4,5. e0 #W before B1=0. e1 #W before B2 (pos4)=2. e2 #W before B3 (pos5)=2. e3=3. Recurrence H1=(2!)/(1!)=2. H2 invalid e2=e1. H3=(3! - H1*fact[3-1=2]? Wait c=1, e3-c=2, not 1. I used fact[1] incorrectly. H3=(6 - 2*2)=2. Good. Answer 2. OK.

Case e=[0,1,3,3]? invalid last e3=e2? S? B W B W W B? last B invalid. Need e2<3. e=[0,1,2,3] done. e=[0,1,1,3]? B W B B W W? N=3: B1,W2,B3,B4,W5,W6. e0=0,e1=1,e2=1,e3=3. Recurrence H1=1. H2 invalid. H3=(6 - H1*fact[2])=4. Count permutations: b1 e1=1: π1>1 (π1=2 or3). b2 e2=1: M2>1 automatic if π1>1. total permutations with π1=2 or3: 4. OK.

Maybe formula product of (e_b - e_{b-1}?)? For e=[0,1,2,3] answer3; increments 1,1,1 product? no. e=[0,2,2,3] answer2; increments 2,0,1. e=[0,1,1,3] answer4.

Could be number of linear extensions of poset? Condition M_b > e_b. Let's derive DP over maximum.

Let dp_i(m) = # valid permutations of first i positions with max exactly m. At step i, to add position i with value x. If x <= current max, choices = m - (i-1) (unused values <=m). If x > m, new max x, choices? But need condition at i: new max > e_i. We can aggregate.

Standard count of permutations with prefix max constraints can be done with DP over m using difference arrays maybe O(N log N). Let's see.

Let F_i(m) = # permutations of length i (first i positions) using distinct values, with max = m, and satisfying constraints for all t<=i. Since values are from 1..N, but max m. For i positions, max m requires m>=i. Number of ways to choose and arrange i values from [m] with max m and constraints. Transition from i-1 to i:
- If new value x <= m and max remains m: from state max m at i-1, choose x among m-(i-1) unused values <m. But if previous max m, value m used, unused <=m count m-(i-1). All x <m. New max m. Constraint at i requires m > e_i.
- If new value x = m (new max) from previous max <m: previous state max k<m, choose x=m (only one). New max m. Constraint m > e_i.
Thus F_i(m) = [m > e_i] * ( (m-(i-1)) * F_{i-1}(m) + sum_{k<m} F_{i-1}(k) ), for m>=i.
Base F_0(0)=1? For i=1, F_1(m) = [m>e1]*(0*F0(m)+sum_{k<m}F0(k)) =1 if m>e1 and m>=1? sum F0(k)=1 for m>=1. So F_1(m)=1 for m>e1. But if e1=0, all m 1..N? Wait first position can be any value m, max=m. Constraint M1>e1. If e1=0, all m>=1 valid. F1(m)=1 for all m. Good.

We need total after N: sum_m F_N(m) (m=N necessarily? Since length N permutation of N values, max must N. Actually if we only use values from [N], after N positions max=N. F_N(N) answer.) This DP is O(N^2) naive. But transition has form:
F_i(m) = [m>e_i] * ( (m-i+1) F_{i-1}(m) + prefix_sum_{k<m} F_{i-1}(k) ).
This is similar to counting permutations with lower bounds on prefix max. Can we optimize? e_i nondecreasing. The indicator [m>e_i] zeros m<=e_i. Since e_i nondecreasing, states m<=e_i become zero and never revive? If m<=e_i, F_i(m)=0. For future i', e_i' >= e_i, so m<=e_i also zero. Thus active m > e_i. Also m>=i.

Maybe maintain G_i(m)=F_i(m)/(m-i)!? Let's transform. Number of permutations of i values from [m] with max m and no constraints? It is (m-1 choose i-1)*(i-1)! = (m-1)!/(m-i)!. Transition maybe simplifies.

Let's derive without constraints: F_i(m) = C(m-1,i-1)*(i-1)! = (m-1)!/(m-i)!.
Transition: (m-i+1)F_{i-1}(m) + sum_{k<m}F_{i-1}(k). For unconstrained, sum_{k=i-1}^{m-1} (k-1)!/(k-i+1)! plus (m-i+1)*(m-2)!/(m-i+1)! =? equals (m-1)!/(m-i)!.

With constraints, F_i(m)=0 for m<=e_i. Since e_i nondecreasing, we can maybe compute F_i(m) for m>e_i using prefix sums. Need answer F_N(N). N=2e5, maybe O(N log N) with data structure? Transition for each i involves all m>e_i. That's O(N^2) if e_i small.

But maybe e_i is #W before B, can be arbitrary up to N. Need faster.

Observe transition can be expressed in terms of H? H recurrence is inclusion-exclusion first bad, maybe CDQ is intended. But DP might allow Fenwick if kernel simple? Let's manipulate.

Let P_i(m)=sum_{k<m} F_{i-1}(k). Then F_i(m) = [m>e_i] * ( (m-i+1)F_{i-1}(m) + P_i(m) ). We need update all m. This is like applying linear operator. Could use generating functions? F_i as function of m. e_i changes by increments. Maybe process intervals where e_i constant. If e_i = t for a run of i values, transition repeated with same t. Could use matrix exponentiation? Not likely.

Another transformation: Let A_i(m)=F_i(m)/(m-i)!? Compute:
F_i(m) = (m-i+1)F_{i-1}(m) + P_{i-1}(m) (where P over k). Divide by (m-i)!:
A_i(m) = (m-i+1) F_{i-1}(m)/(m-i)! + P/(m-i)!.
F_{i-1}(m)/(m-i)! = (m-i+1) F_{i-1}(m)/(m-i+1)!? Since (m-i)! = (m-i+1)!/(m-i+1). So first term = (m-i+1)^2 A_{i-1}(m)? Not nice.

Maybe define B_i(m)=F_i(m)/(m-1)!? Then unconstrained B=1/(m-i)!.
Transition: F_i(m)/(m-1)! = (m-i+1)F_{i-1}(m)/(m-1)! + P/(m-1)! = F_{i-1}(m)/(m-2)! * 1/(m-1)? no.

CDQ seems safer.

But maybe we can solve recurrence with one divide-and-conquer using polynomial multiplications; that's task. Need implement well.

Let's focus on CDQ NTT optimization.

We need compute H[1..N]. Recurrence only depends on valid previous c. We can compress indices to valid b? Since invalid H=0 and no need compute. Let valid list V of length M <= N. For each valid b, e_b strictly increasing. Recurrence for valid index i (b=V[i]) depends on previous valid c=V[j] j<i: H_i = (fact[e_i] - sum_{j<i} H_j fact[e_i - V[j]]) * invfact[e_i - V[i]]. This is same but indices compressed. CDQ over compressed valid indices might reduce size if many invalid. But e_i - c uses original c. We can use compressed array B[i]=b, E[i]=e_b. M maybe N. CDQ over M. This also makes valid_b list contiguous. Good. Let's do that.

Build arrays B[1..M], E[1..M] for valid b. H[1..M]. val[i]=fact[E[i]]. Recurrence:
H[i] = (val[i] - sum_{j<i} H[j]*fact[E[i]-B[j]]) * invfact[E[i]-B[i]] mod.
Need E[i]-B[i]>=0 by valid. E[i]-B[j] >=? Since B[j]<B[i] and E[i]>=B[i], positive.

CDQ over i=1..M. This avoids checking valid in apply; all b in right are valid. Nonzero H list over compressed indices. But c0/c1 in terms of original B values? For convolution, c index in kernel is original B[j], not compressed j. The sequence A over original B range [B[c0], B[c1]] may have gaps (invalid or zero). Length L = B[c1]-B[c0]+1 could be up to N, even if compressed segment small. That's okay total? If many invalid, B gaps large, convolution length may blow up. But M<=N, B range length <=N. At a CDQ node, left half compressed indices j in [l,m], c0=B[first nonzero], c1=B[last nonzero]. L could be large due to gaps. But sum over nodes? If invalid gaps large, maybe L large for few nodes. Worst-case all valid, L ~ segment length. If sparse valid, M small but B range could N; NTT length maybe N for root, okay. But if many nodes with sparse nonzero? Could be overhead. Alternative convolve over compressed index with kernel fact[E[i]-B[j]] not a simple convolution because B[j] not equal j. Need original B range. So L based on B gaps.

Could instead for each node build A over compressed j and use arbitrary kernel? Not convolution. So original range necessary.

If valid indices sparse, direct loops may be better because nonzero_count*valid_count small. NTT only when product large; then likely many valid/nonzero, B range not too sparse. Good.

Use compressed CDQ. M could be 0? If no valid b? But N>=1 and S1=B,S2N=W implies b=N valid? e_N=N>e_{N-1}, e_N>=N, so M>=1. If N=1 M=1.

CDQ solve(l,r) over 1..M.
- if r-l <= THRESH: direct compute for i=l..r:
   s=val[i]
   for j in range(l,i): if H[j]: s -= H[j]*fact[E[i]-B[j]]
   H[i]=s*invfact[E[i]-B[i]]%MOD
   if H[i]: nonzero.append(i)
  But val[i] includes contributions from j<l. Need contributions from j in [l,i-1] direct. Good.
- else mid=(l+r)//2; solve(l,mid); apply(l,mid,mid+1,r); solve(mid+1,r).

apply(l,m,ql,qr): contributions from j in [l,m] to i in [ql,qr]. All i valid. Need nonzero j in [l,m]. Use global nonzero list (compressed indices) and bisect. Since solve(l,m) done, nonzero contains all nonzero up to m (and maybe none >m). But if using compressed, nonzero list sorted. c_idx_list = nonzero[bisect_left(nonzero,l):bisect_right(nonzero,m)]. If empty return.
 b_idx_list = range(ql,qr+1) all valid. But for NTT need first/last b: ql,qr. For direct, loop over i in ql..qr.
 Product = len(c_list)*(qr-ql+1). If <= DIRECT_THRESH: direct.
 Else NTT.

Direct loops optimization:
for j in c_list:
    hj=H[j]; bj=B[j]; fact_local=fact
    for i in range(ql,qr+1):
        val[i] = (val[i] - hj * fact_local[E[i]-bj]) % MOD
This is O(product). Could optimize by local variables and modulo after inner? Let's think.
val[i] initially mod. Subtract up to len(c_list) terms. If we do val[i] -= hj*fact[...] without mod, after many applies val can become large negative. Number of contributions per val total O(M) if direct all? Could be O(M^2) in worst if direct threshold high? But val magnitude could grow huge (big ints) slowing. We can mod after each c loop or after each i. Maybe:
for i in range(ql,qr+1):
    s=val[i]
    for j in c_list:
        s -= H[j]*fact[E[i]-B[j]]
    val[i]=s%MOD
This inner product count same. s can accumulate len(c_list)*MOD^2 ~ threshold*1e18. If threshold 4096, ~4e21 (73 bits) okay. Mod once per i. This reduces modulo operations. But if c_list length maybe up to DIRECT_THRESH / right_len. If right_len=1, c_list length could 4096, s ~4e21 okay. If direct threshold 10000, ~1e22 okay. Python big ints 70-80 bits fine. Use this.
Pseudo:
for i in range(ql, qr+1):
    ei=E[i]
    s=val[i]
    for j in c_list:
        s -= H[j] * fact[ei - B[j]]
    val[i] = s % MOD
But accessing H[j], B[j] each inner. Could precompute lists hj, bj for c_list. Then:
for i in range(ql,qr+1):
    ei=E[i]; s=val[i]
    for hj,bj in pairs:
        s -= hj * fact[ei-bj]
    val[i]=s%MOD
This is okay. If product threshold maybe 8192.

NTT apply:
Need c_list compressed indices. c0=B[c_list[0]], c1=B[c_list[-1]]. L=c1-c0+1.
First b=ql, last=qr. e_first=E[ql], e_last=E[qr] (E increasing). t_min=e_first - c1. Since c1 <= B[m] < B[ql]? m<ql, B[m] < B[ql] (original b increasing). e_first >= B[ql] > B[m] >= c1? Actually c1 could be B of nonzero <=m, so c1 < B[ql] <= e_first. Thus t_min >=1? If e_first=B[ql] and c1=B[ql]-1, t_min=1. So nonnegative. But use max(0).
t_max=e_last - c0. Mlen=t_max-t_min+1.
Build A length L: for pos in range(L): idx = c0+pos; need H for compressed index with B=idx if exists and nonzero. We can't map original b to compressed index easily? We have B array compressed. Could build A by iterating c_list and setting A[B[j]-c0]=H[j], zeros elsewhere. L may be large; initializing [0]*L and setting nonzero is O(L). That's okay because NTT length O(L+M). But if L huge and product large? NTT anyway. Need ensure memory.
Build F length Mlen: fact[t_min + j] for j in range(Mlen). Could slice fact[t_min:t_max+1] (copy). fact list length N+1. Slicing O(Mlen). Good.
Convolve A,F. Then for i in ql..qr: idx = E[i] - c0 - t_min; term=conv[idx]; val[i]=(val[i]-term)%MOD.
Need ensure idx within 0..len(conv)-1. For i=ql, E=e_first, idx=e_first-c0-t_min = c1-c0 = L-1? Wait t_min=e_first-c1, idx=e_first-c0-(e_first-c1)=c1-c0=L-1. Convolution index L-1 corresponds sum A[i]F[L-1-i]. For c=c1 (i=L-1), F[0]=fact[e_first-c1]=t_min. Good. For i=qr, idx=e_last-c0-t_min = e_last-c0-(e_first-c1)= (e_last-e_first)+(c1-c0). Max index? L-1+Mlen-1 = c1-c0 + e_last-c0 - (e_first-c1) = e_last-c0 - e_first + c1 + c1-c0? Let's compute Mlen=t_max-t_min+1=e_last-c0 - (e_first-c1)+1. Max conv index L+M-2 = (c1-c0+1)+(e_last-c0-e_first+c1+1)-2 = e_last-c0-e_first+2c1-c0? Hmm maybe idx for last b with c=c0: e_last-c0-t_min = e_last-c0-e_first+c1. This should be <= L+M-2. L+M-2 = c1-c0 + e_last-c0-e_first+c1 = e_last-c0-e_first+2c1-c0. idx = e_last-c0-e_first+c1. Difference = c1-c0 >=0. OK.

But convolution A*F includes terms for c in [c0,c1] and fact index t in [t_min,t_max]. For a given b, query idx sums over i (c-c0) and j (t-t_min) with i+j=idx, t = t_min+j = e[b]-c. Good.

NTT length n = 1 << ((L+Mlen-2).bit_length()) maybe if L+Mlen-1 >1. If L=0? no. If n small maybe direct would have been used. But NTT for n maybe 256 etc.

Need implement convolution returning list length L+Mlen-1. To save memory, we can ntt in-place. A and F lists. After ntt, multiply, inverse. Then discard.

Potential optimization: If one of A or F length 1, direct? But product threshold likely handles. Could still NTT with length small. We can add if L==1 or Mlen==1 direct? But product maybe large if Mlen large and L=1: direct product = right_len, maybe <= threshold? If right_len large > threshold, L=1, direct O(right_len) which is fine; NTT O(N log N) worse. So direct condition should consider min(product, maybe L*right_len?) Actually product = nz_count*right_len. If nz_count=1, product=right_len. If right_len=100000, product 100000 > threshold, but direct 100k operations is much cheaper than NTT length maybe 200k. So threshold should not force NTT for small nz_count. Use direct if nz_count * right_len <= DIRECT_THRESH OR nz_count <= SMALL_NZ (e.g. 32) OR right_len <= SMALL_RIGHT? More generally estimate direct cost = nz_count * right_len. NTT cost ~ n log n where n ~ L+Mlen. If nz_count small but B range large, direct may still be cheaper. We can choose direct if nz_count * right_len <= max(DIRECT_THRESH, maybe 4*n*logn?) Hard.

Simpler: direct if nz_count * right_len <= 20000? 100k direct maybe okay? If many nodes with nz_count=1 and right_len large, total direct could be O(N log N) ~3.6M, fine. If threshold 20000, node with 1*100000 would NTT unnecessarily. So set direct if nz_count <= 32 or right_len <= 32 or product <= 20000? But if nz_count=32, right_len=100000 product 3.2M, direct maybe too high; NTT better. So condition: if nz_count * right_len <= DIRECT_PRODUCT (e.g. 50000) OR nz_count <= 4? Let's think.

Worst-case many nodes with nz_count=1, right_len large. Direct total across CDQ? For each nonzero c, it contributes to all later i. If we direct for nz_count=1 at many nodes, each pair (c,i) might be processed multiple times? CDQ ensures each pair processed exactly once at the node where c in left, i in right. If we choose direct for nz_count=1, total direct operations = number of pairs where left segment has only one nonzero? Could be O(N log N)? Let's bound: Each pair (c,i) is processed at exactly one apply node (the lowest common ancestor in CDQ). If at that node nz_count=1, direct. Number of such pairs could be O(N^2) if many nodes have nz_count=1? Example H nonzero only at index 1. Then for every i>1, pair (1,i) processed at root? Actually CDQ: solve(1,M). At root left [1,mid] contains c=1, right [mid+1,M] many i; nz_count in left maybe 1 (if only H1 nonzero), direct product=right_len. Then solve right: left [mid+1, ...] has no nonzero, skip. So total O(N). Good. If nonzero at powers? Each pair processed once. Direct cost for a node = nz_count*right_len. Sum over nodes where we choose direct could be large if threshold high. But if choose direct for nz_count<=4, total pairs with left nonzero count <=4. Could be O(N log N)? Consider nonzero indices every other? At a node, nz_count proportional segment length, so not <=4 except small. Pairs processed at small nodes. Total O(N*4*log?) maybe okay.

Maybe use cost model: direct_cost = nz_count * right_len. ntt_cost = n * log2(n) * C (C maybe 3 for two forward one inverse). Choose direct if direct_cost <= ntt_cost * factor. But computing n requires L,Mlen. We can estimate. In Python, direct inner loop is simple multiplication/subtraction; NTT butterfly is heavier (mod multiplications). So direct may be competitive up to maybe 4*n log n. But direct uses Python loops over product; NTT uses Python loops over n log n with more operations. A direct operation: s -= hj*fact[...] (list access, multiply big int, subtract). NTT butterfly: several mod operations, multiplications. Direct maybe 5-10x cheaper per unit? Hard.

We can set direct if direct_cost <= 4 * n * log2(n) maybe. But n can be large due to B gaps. For nz_count=1, L maybe large? If c0=c1, L=1, Mlen ~ e_last-e_first+1 maybe up to N. n ~ N. ntt_cost ~ N log N, direct_cost=right_len ~ N. direct much cheaper. Cost model direct <= 4*nlogn true. Good.

For all nonzero dense, nz_count~left_len, right_len~right_len, L~left_len, Mlen~e range ~right_len? n~left+right. direct_cost~left*right ~ n^2/4, ntt_cost~n log n, NTT better for n large.

So implement adaptive:
- nz = len(c_list), rb = qr-ql+1.
- direct_cost = nz * rb.
- If direct_cost <= DIRECT_ALWAYS (e.g. 20000): direct.
- Else compute L, Mlen, n=pow2(L+Mlen-1), ntt_cost = n * (n.bit_length()-1) maybe. If direct_cost <= ntt_cost * 2? direct else NTT.
But computing L/Mlen requires c0,c1,e_first,e_last cheap. We can do.
Need avoid NTT for very small n (n<128) direct.

What threshold values? Need pass time. Let's estimate. If direct_cost <= 50000 direct. For dense node length 256: left=128,right=128, direct_cost=16384 direct. length512: 256*256=65536, n~512? ntt_cost=512*9=4608, direct_cost > 2*ntt_cost => NTT. So NTT starts at segment length 512. Number NTT nodes ~N/512=390 + ... ~780. Total NTT work maybe ~? At length512, 390 conv n~512? Actually L~256, M~256, n=512, cost 390*512*9=1.8M butterflies. length1024: 195*1024*10=2M. Sum ~20M butterflies. Direct leaf/internal small: direct for nodes length<=256. Pairs processed direct at those levels: level segment 256: 128*128*781? Wait N=2e5, nodes length256: 781, each direct_cost 16k =>12.5M. length128: 1562*4096=6.4M. length64: 3125*1024=3.2M. Leaves threshold? If CDQ leaf threshold maybe 32? Direct internal levels sum ~ N/2 * log? Actually total direct pairs for nodes with length<=256: each pair processed at LCA; pairs with distance <256? Number ~ N*256/2? ~25M. Plus leaf internal. 25M direct operations. Might be okay in PyPy if optimized? 25M list accesses/mults maybe ~2-4s? NTT 20M butterflies maybe ~5-10s. Total maybe high but perhaps acceptable with PyPy if time 5s? Need optimize further.

Could increase direct threshold to reduce NTT calls? If direct up to length512 (direct_cost 65536), direct pairs ~ N*512/2=51M, NTT starts length1024, NTT work ~10M. Direct 51M maybe too high. If direct up to length256 as above balanced.

Maybe use leaf threshold 32 and adaptive direct. CDQ recursion overhead okay.

But we can optimize direct loops using local variables and maybe vectorization? Pure Python.

Alternative: Use NTT for all nodes above small, but implement NTT highly optimized. Need maybe use precomputed roots to speed.

Let's design fast NTT for 998244353 in Python.

Common optimized iterative NTT:
def ntt(a, invert):
    n = len(a)
    j = 0
    for i in range(1, n):
        bit = n >> 1
        while j & bit:
            j ^= bit
            bit >>= 1
        j ^= bit
        if i < j:
            a[i], a[j] = a[j], a[i]
    length = 2
    while length <= n:
        wlen = pow(PRIMITIVE_ROOT, (MOD-1)//length, MOD)
        if invert:
            wlen = pow(wlen, MOD-2, MOD)
        half = length >> 1
        for i in range(0, n, length):
            w = 1
            for j in range(i, i+half):
                u = a[j]
                v = a[j+half] * w % MOD
                x = u + v
                if x >= MOD: x -= MOD
                y = u - v
                if y < 0: y += MOD
                a[j] = x
                a[j+half] = y
                w = w * wlen % MOD
        length <<= 1
    if invert:
        inv_n = pow(n, MOD-2, MOD)
        for i in range(n):
            a[i] = a[i] * inv_n % MOD

This does bit-reversal each call. Could precompute bit-reversal for each n? Many calls with powers of two. We can cache rev arrays? Memory maybe. Or use iterative decimation-in-frequency without bit-reversal? There are NTT implementations that avoid explicit bit-reversal by using in-place with roots and final ordering? Let's recall.

AtCoder Library butterfly uses precomputed roots and no bit-reversal? It performs in-place with specific loops, output in normal order? ACL's butterfly (for NTT) uses Cooley-Tukey with bit-reversal implicitly? Let's examine.

ACL Python implementation (from atcoder library) for modint:
def butterfly(a):
    n=len(a); h=(n-1).bit_length()
    first=True
    for ph in range(1,h+1):
        w=1<<(ph-1); p=1<<(h-ph)
        now=1
        for s in range(w):
            offset=s<<(h-ph+1)
            for i in range(p):
                l=a[i+offset]
                r=a[i+offset+p]*now%mod
                a[i+offset]=(l+r)%mod
                a[i+offset+p]=(l-r)%mod
            now=now*rate2[(~s & -~s).bit_length()-1]%mod
...
This is complex but faster due to precomputed roots.

Simpler: Use iterative with bit-reversal but cache rev for n. Number of distinct n powers up to 262144 (18). Caching rev lists total ~2^19? Sum powers ~524k, memory okay. But generating rev for each n O(n). Could speed bit-reversal. However, applying rev list: for i in range(n): if i<rev[i]: swap. Still O(n). Could precompute rev as array('I')? List of ints memory ~4MB per 262k? Actually Python int 28 bytes, 262k ~7MB, sum 14MB, okay maybe. But caching all rev lists maybe 14MB plus overhead, okay. But generating on fly maybe fine.

Alternative use bit-reversal permutation via iterative j loop is O(n) with while, maybe slower. Precomputed rev can be faster:
rev = [0]*n; for i in range(n): rev[i]=(rev[i>>1]>>1)|((i&1)*(n>>1))
Then for i in range(n//2): if i<rev[i]: swap. But building rev O(n). Cache.

Could use "ntt" from cp-alg with precomputed roots per length to avoid pow each stage. Precompute root_pw? For each length, wlen. pow 18 times per NTT call; if 700 calls, 12600 pow, maybe okay? pow mod ~log 1e9 ~30 multiplications, 378k, negligible. But inverse pow wlen each stage extra. Could precompute inv_wlen. Use arrays roots[stage], iroots[stage]. Stage length=2^k. root = pow(G,(MOD-1)//length,MOD), invroot=pow(root,MOD-2,MOD). Precompute up to max_n. Then ntt uses roots[k]. Good.

Can also precompute bit-reversal? Maybe.

But many NTT calls with small n; function call overhead and list allocations dominate. Need maybe use direct for small.

Could implement convolution using "middle product"? Not needed.

Memory: fact, invfact, E, B, val, H length M+1 ~2e5, fine. NTT arrays length up to maybe 524288? L+Mlen <=? c range up to N, t range up to N, sum up to 2N? Actually L <= N, Mlen <= N, L+Mlen-1 <= 2N. n power up to 524288 for N=2e5 (2N=4e5 -> 524288). Two arrays of 524k Python ints ~ 2*524k*28=29MB plus list pointers 8MB each? Actually list of ints stores pointers 8 bytes + int objects. But small ints? Values mod are Python int objects. [0]*n shares zero object, but after fill distinct ints. Memory maybe high: list pointers 4MB each, int objects 14MB each if distinct => 36MB per array, two 72MB. Plus other arrays maybe >256MB? AtCoder memory 1024MB likely okay. But Python int overhead large. Need be mindful. We can use list of ints; NTT creates many int objects. Could use array('I') to reduce memory but slower? Python list faster.

When building A = [0]*L, setting nonzero values; zeros share object. F = fact[t_min:t_max+1] copies references to existing int objects (fact ints). NTT will replace with new ints. Memory peak maybe A,F,conv? We can reuse A as result: ntt(A), ntt(F), multiply into A, inverse A. So two arrays. F slice length M. A length L. After ntt length n, we need pad to n. We can create A = [0]*n and fill first L; F=[0]*n fill first M. That allocates two n lists. If we instead A length L then extend to n? A = [0]*n; for j in c_list: A[B[j]-c0]=H[j]. F = fact[t_min:t_max+1] + [0]*(n-Mlen). This creates slice M then extend. Peak maybe slice + F. Could do F=[0]*n; F[:Mlen]=fact[t_min:t_max+1] (slice assignment) still creates slice. Maybe okay. To reduce, use F = fact[t_min:t_max+1]; F.extend([0]*(n-Mlen)). A = [0]*n; fill. Peak A n, F M then n. Fine.

Could use convolution function:
def convolution(a, b):
    n=1
    while n < len(a)+len(b)-1: n<<=1
    fa=a[:] + [0]*(n-len(a))
    fb=b[:] + [0]*(n-len(b))
    ntt(fa,False); ntt(fb,False)
    for i in range(n): fa[i]=fa[i]*fb[i]%MOD
    ntt(fa,True)
    return fa[:len(a)+len(b)-1]
But a,b already lists. We can avoid copying a if we build fa directly. For F, we can build fb directly.

In apply NTT:
n=1; need=L+Mlen-1; while n<need: n<<=1
A=[0]*n
for j in c_list: A[B[j]-c0]=H[j]
F=[0]*n
# fill F[0:Mlen] with fact[t_min:t_max+1]
F[:Mlen] = fact[t_min:t_max+1]  # creates slice
ntt(A,False); ntt(F,False)
for i in range(n): A[i]=A[i]*F[i]%MOD
ntt(A,True)
# query
for i in range(ql,qr+1):
    idx=E[i]-c0-t_min
    val[i]=(val[i]-A[idx])%MOD
# discard A,F

Potential optimization: If Mlen == n and L small? F fill slice. OK.

Need ensure A values are mod (H mod). F fact mod.

NTT correctness: For inverse, after ntt inverse, values mod. Need maybe trim.

Now, CDQ recursion depth log M ~18, fine. But Python recursion function calls many nodes ~2*M/threshold? If threshold 32, ~12500 calls, okay. Could set recursionlimit.

Need choose leaf threshold. In solve, if r-l <= LEAF: direct compute internal. But apply also direct/NTT. If LEAF too small, many apply nodes. If LEAF too large, direct leaf cost high. We can set LEAF=32 or 64. Adaptive apply direct handles small. Leaf direct cost N*LEAF/2. LEAF=64 =>6.4M, okay. Number nodes ~N/64=3125 leaves, internal ~3125. Apply nodes ~3125. Adaptive NTT for larger. Good. LEAF=64 maybe fine. If LEAF=128 =>12.8M direct, nodes 1562. Maybe okay. Choose 64 to reduce direct.

But apply direct for nodes length 128,256 etc adds direct pairs. Total direct pairs for nodes with segment length <= maybe 512 if adaptive. With LEAF=64, direct pairs at levels: leaf internal 6.4M, apply length128: N/128*64*64? Wait apply for node length128: left64 right64 product4096, nodes1562 =>6.4M. length256: left128 right128 product16384, nodes781 =>12.8M. length512: product65536, nodes390 =>25.6M if direct; adaptive likely NTT if ntt_cost smaller. So direct total maybe 25M if direct threshold includes 16k but not 65k. Good.

Let's set adaptive:
DIRECT_LIMIT = 20000 maybe. For length256 product16384 direct. length512 product65536 NTT. Good.
Also if nz_count <= 2? direct regardless? But cost model covers: n maybe large, ntt_cost large, direct_cost maybe right_len. If right_len=100000, ntt_cost ~ 524288*19=10M, direct_cost=100k <= 2*ntt_cost, direct. Good.
Use if direct_cost <= 20000 or direct_cost <= 3 * n * n.bit_length(): direct. But computing n for every apply maybe cheap. Need L/Mlen. For dense length512: direct_cost=65536, n~512? n.bit_length=10, 3*n*bl=15360, direct false -> NTT. For length256: direct_cost=16384, n~256? bl=9, 3*256*9=6912, direct false if only cost model, but we want direct maybe? 16k direct vs NTT 256*9=2304 butterflies; NTT likely faster? Maybe NTT for length256 could be faster than 16k direct? NTT 2304 butterflies but each heavy; direct 16k simple. Hard. If we NTT length256, number NTT nodes 781, total butterflies 781*2304=1.8M, maybe okay. Direct 12.8M. NTT might be faster. But many NTT calls overhead. Could set direct limit 4096 only, NTT for length256. Then NTT nodes ~781+390+...=1560, total butterflies maybe 4M? Actually length256 n=256? L=128,M=128 need=255 n=256, cost 2304, 781=>1.8M; length512 n=512 cost4608*390=1.8M; total ~7M. Direct leaf+length128: leaf6.4M + length128 6.4M =12.8M. This may be better. NTT calls 1560 maybe overhead but okay? 1560 NTT calls of small size could be high due to function overhead, bit-reversal, pow. But total n sum per level: length256 nodes 781*256=200k; length512 390*512=200k; each level sum n ~N. Levels NTT from 256 to 262k: ~10 levels, total n processed ~2M per transform? Actually each apply does 3 NTTs (A,F,inv) => 3*sum n log n. Sum n per level ~N, log increases. Total butterflies ~3*N*sum_{levels} log n ~3*2e5*(8+9+...+18) ~3*2e5*125=75M butterfly? Wait each butterfly is inner loop operation. Sum n log n per level: for level with n=256, nodes 781, n log n=256*8=2048, total 1.6M. Next 512*9*390=1.8M, 1024*10*195=2M, ... up to 262144*18*1=4.7M. Sum ~30M. Times 3 transforms? The n log n already per transform? n log n is number of stages*n/2 butterflies? Actually iterative NTT inner loops: for each stage length, n/2 butterflies. Total n/2 * log n butterflies. My n*log n overestimates by 2. So per transform ~15M total, times 3=45M butterflies. Plus direct 13M. Might be high but maybe okay in PyPy with optimized NTT? 45M butterflies each with mod mult, maybe too slow (10s+).

Need reduce NTT calls/transforms. Can we avoid NTT for F (fact) by precomputing its NTT? F range changes (t_min,t_max) per node, not same. But fact sequence is global; maybe use convolution with fact via offline? Not simple.

Maybe use a different algorithm with O(N log N) single or few NTTs. Let's revisit recurrence and possible generating functions.

We have H_i = (fact[E_i] - sum_{j<i} H_j fact[E_i - B_j]) / fact[E_i - B_i]. B_j and E_i are sequences. This is like online convolution with arbitrary x-coordinates B_j and y-coordinates E_i. If B_j and E_i are both increasing, we can use divide and conquer. That's optimal maybe O(N log^2 N). But maybe because E_i - B_i = d_i and E_i nondecreasing, can use CDQ with NTT as intended. Python may still pass if optimized and N=2e5? AtCoder problems with NTT in Python often require careful but possible for 2e5 if total NTT ~ few million, not 45M. Need optimize further or find simpler.

Maybe the recurrence can be computed in O(N log N) using one NTT per level but with total O(N log N) not O(N log^2)? CDQ with convolution at each node total O(N log^2 N) if convolution length proportional segment length? Actually sum over nodes at each level of length = N, and NTT cost length log length, so per level O(N log N), levels log N => O(N log^2 N). But if using "divide and conquer optimization" with convolution of left and right where total length per level N, yes O(N log^2 N). For N=2e5, log^2 ~324, too high. But constants small? Maybe intended C++ only? But task asks Python. Need maybe exploit that e_b increments total N to get O(N log N)? Let's think.

The kernel fact[E_i - B_j] depends on difference of two monotone sequences. If B_j and E_i are both subsets of [N]. This is a convolution over value domain, not index domain. CDQ over index causes log^2. Could instead process by value domain? Since E_i and B_j are values in [N]. We need for each i sum over previous j with B_j < B_i? Actually B_j < B_i. Kernel fact[E_i - B_j]. This is like for each i, query prefix sum over j of H_j * fact[E_i - B_j]. If we could maintain a data structure supporting adding a term H_j at position B_j and querying at x=E_i: sum_j H_j fact[x - B_j]. This is online convolution with point updates and point queries. Since x=E_i nondecreasing (strict for valid i), and updates B_j increasing. We need dynamic convolution. Could use sqrt decomposition: block updates, precompute NTT for block? Maybe O(N sqrt N log N) not better. But maybe because x and B are both increasing, we can use offline Fenwick of polynomials? Hmm.

We can view H_i = fact[E_i] * invfact[E_i-B_i] - invfact[E_i-B_i] * sum_{j<i} H_j fact[E_i-B_j]. Let A_j = H_j. Query Q(x)=sum_{j<i} A_j fact[x-B_j]. Updates at B_j. x increases. This is online convolution on a line. There are data structures with sqrt decomposition: divide B into blocks, for each block precompute convolution of A_j in block with fact, query O(1) per block? Number blocks sqrt N, update O(1), query O(sqrt N) => O(N sqrt N) ~9e7 maybe high but maybe with NTT rebuild blocks? Not.

Could use generating functions and solve via formal power series if B_j = j? But B_j arbitrary valid black indices. However B_j are exactly b where e_b increases. e_b is #W before B_{b+1}. The pair (B_j, E_j) are points on a lattice path. Maybe there is a relation: E_j - B_j = number of W before B_{B_j+1} minus B_j. This is the height of path? The sequence of valid b are those with a W in block. E_j is cumulative W. B_j is cumulative B (excluding current?). Points (B_j, E_j) are corners of the path where W occurs? Let's map path of B/W. e_b is W count before B_{b+1}. Valid b if there is at least one W between B_b and B_{b+1}. Then E_j = e_{B_j}. B_j is black index. The points (B_j, E_j) maybe the positions of W blocks. E_j - B_j = (#W - #B) before B_{B_j+1}? Could be height.

Maybe count strong matchings has formula in terms of path area? Let's explore combinatorially with permutation prefix maxima. Condition M_b > e_b. e_b is path. Count permutations with prefix max above path. There might be a simple product using "cycle lemma" or "parking functions". Let's investigate deeply; could yield O(N) solution.

We count permutations π of [N] such that for all b=1..N-1, max_{i<=b} π_i > e_b. Let e_b nondecreasing, e_0=0, e_N=N, and e_b>=? Not necessarily. This is equivalent to the inverse permutation p_v (position of value v) satisfying for all b, min_{v>e_b} p_v <= b. Let t=e_b. For each t, let b_t = min{b: e_b >= t}. Need min_{v>t} p_v <= b_t. Since e_b may jump, b_t is first black index after t whites? Actually b_t = number of B before the (t+1)-th W? Let's define symmetric sequence. Since S1=B,S2N=W. b_t for t=0..N-1: minimal b such that e_b >= t. This is #B before (t+1)-th W? If e_b is W before B_{b+1}, e_b >= t means before B_{b+1} there are at least t W, so B_{b+1} is after (t+1)-th W? Minimal b is number of B before (t+1)-th W? Let's check. For t=0, b_0=0? e_0=0 if S1=B. min b e_b>=0 is 0. Constraint trivial. For t>0, b_t = #B before (t+1)-th W? Suppose S=BWBW N=2: W1 pos2, W2 pos4. t=1: #B before W2? W2 pos4, B before=2? But e_b: e0=0,e1=1,e2=2. min b e_b>=1 is b=1. #B before (t+1=2)-th W =2, not 1. Maybe #B before (t+1)-th W minus 1? Because e_b before B_{b+1}. e_b>=t means B_{b+1} occurs after at least t W. Minimal b is number of B before the (t+1)-th W minus 1? For W2, B before=2, minus1=1. Yes. For t=0, W1 B before=1, minus1=0. So b_t = c_t -1 where c_t = #B before (t+1)-th W. Let f_t = c_t. Then constraint min_{v>t} p_v <= f_t -1. Equivalently, among values t+1..N, at least one position <= f_t-1, i.e. not all in positions f_t..N.

This is symmetric to original with W/B swapped? The sequence f_t is #B before W_{t+1}. Since S1=B,S2N=W, f_t >= t? Maybe.

Counting permutations p_v (positions of values) such that suffix minima r_t = min_{v>t} p_v <= f_t-1. r_t is nondecreasing in t? As t increases, set shrinks, min can increase. f_t nondecreasing. This is like a parking function condition: r_t <= f_t-1. There is known count of permutations with given "parking functions" maybe product of (f_t - t)? Let's test.

For a permutation p_1..p_N (positions of values 1..N), define r_t = min(p_{t+1},...,p_N). Condition r_t <= f_t-1 for t=0..N-1 (r_0=1 <= f_0-1? f_0=1? S1=B => #B before W1=1, f_0=1, r0=1 <=0 false? Wait t=0 constraint e_0=0, b_0=0, need min_{v>0} p_v <=0 impossible. But b=0 not required. So t from 1? For b>=1, e_b>=? t=e_b. If e_b=0 for b? S1=B e0=0, b=0 not required. For b=1, e1 maybe 0 if no W before B2? But S? If e1=0, condition M1>0 automatic. t=0 constraint not needed. So t where b_t>=1. b_t = min b>=1 with e_b>=t. For t=0, b_t=0 ignore. For t>=1, b_t = #B before (t+1)-th W? Let's recompute: e_b>=t minimal b. e_b is W before B_{b+1}. If (t+1)-th W is before B_{b+1}, then e_b>=t+1? Wait e_b counts W before B_{b+1}. To have at least t W before B_{b+1}, the t-th W (not t+1) must be before B_{b+1}. Minimal b is #B before t-th W? Let's test N=2 BWBW: W1 pos2, W2 pos4. t=1: need e_b>=1. e1=1 (before B2 pos3, W1 before). Minimal b=1. #B before W1 =1? B1 pos1 before W1, yes 1. So b_t = #B before t-th W? For t=1, 1. For t=2: e_b>=2 minimal b=2 (before B3 nonexistent? e2=N=2). #B before W2=2. Yes. So b_t = c_t where c_t = #B before t-th W (for t=1..N). Since S2N=W, c_N <= N-1? Actually W_N last, #B before =N. Wait N=2 W2 pos4, #B before=2 (B1,B2). b_2=2. Constraint for b=2? We only require b=1..N-1, so t=e_b. e_N=N at b=N not required. For t=N, ignore. For t=1..N-1, constraint r_t <= b_t? Earlier condition for b with e_b=t: r_t <= b. Minimal b with e_b>=t is b_t. Need r_t <= b_t. For t=1, b_t=1, r_1=min positions of values 2..N <=1. In BWBW answer permutations? N=2, condition b=1 e1=1: M1>1 => π1=2. In inverse p_2=1, r_1=1 <=1. Good.

So constraints: for t=1..N-1, min_{v>t} p_v <= c_t, where c_t = #B before t-th W. Since S1=B, c_t >=1? S2N=W, c_N=N. c_t nondecreasing, c_t >= t? #B before t-th W can be less than t? Example B W W B W B? N=3: W1 pos2 c1=1, W2 pos3 c2=1, W3 pos5 c3=2. c_t can be <t. If c_t < t, constraint automatic? r_t is min of N-t+1 positions, at least? Minimum position among N-t+1 distinct positions is at most N-(N-t+1)+1 = t. Actually among N-t+1 positions, the minimum <= t. If c_t >= t, automatic? Wait if c_t < t, not automatic. Let's check: r_t <= c_t. Since r_t <= t always (pigeonhole: N-t+1 positions in [1,N], min <= t). If c_t >= t, automatic. If c_t < t, need stronger. This is dual to e_b>=b automatic. Good.

Counting permutations p_v with suffix minima r_t <= c_t. This is similar to "permutations with bounded suffix minima". There may be a simple product: number = ∏_{t=1}^{N} (c_t - t + 1)? Let's test.

For a permutation p_1..p_N (positions of values 1..N), consider building values from N down to 1. r_t = min position among values >t. When adding value t+1, the new minimum r_t = min(r_{t+1}, p_{t+1}). Constraint r_t <= c_t. Counting maybe via choices for p_t? Let's derive.

Process values descending N,N-1,...,1. Maintain current minimum position m = min positions of values >t. Initially for t=N, no values, m=∞. When adding value t (or t+1), choose its position among unused. Constraint after adding values >t: m <= c_t. This is like parking functions. Number of permutations satisfying r_t <= c_t for all t might be ∏_{t=1}^N (c_t - t + 1)? Let's test with examples.

Example BWBW N=2: c1=#B before W1=1, c2=2. Product t=1..N? (c1-1+1)=1, (c2-2+1)=1 =>1. Answer1.
Example e=[0,2,2,3] (S=B W W B B W) N=3. W positions 2,3,6. c1=1, c2=1, c3=3. Product (c1-1+1)=1, (c2-2+1)=0, (c3-3+1)=1 =>0, but answer 2. So not that.
Maybe product over t where c_t < t? No.

Let's compute answer for that example was 2. c=[1,1,3]. Constraints: t=1: r1<=1 (values 2,3 min pos <=1) => one of p2,p3 is 1. t=2: r2<=1 (value3 pos <=1) => p3=1. Thus p3=1, p2 any of 2,3, p1 remaining =>2. Product? c2=1, t=2 gives 0 if formula.

Maybe count = number of ways to assign positions to values descending with constraint current min <= c_t. Let's DP:
Process t from N down to 1. Let m = current minimum position among values >t. Initially m=∞. At step t, choose position x for value t among unused. New m' = min(m,x). Need m' <= c_t for t=1..N-1? For t=N no constraint? Actually after adding value N (t=N-1? Let's index carefully). r_t = min_{v>t} p_v. After processing values N down to t+1, current m=r_t. Constraint for t (1..N-1): m <= c_t. Then process value t.
So at step t (from N-1 down to 1), before choosing p_t, we have m=r_t and must satisfy m<=c_t. Then choose p_t, update m'=min(m,p_t) for next t-1.
Initial after processing value N: m=p_N. Constraint for t=N-1: m<=c_{N-1}. So p_N <= c_{N-1}. Then choose p_{N-1}, etc.
This is like counting sequences p_N,p_{N-1},...,p_1 distinct positions such that prefix minima (from high values) <= c_t.

Can we count greedily? At step t, given current minimum m (<=c_t), choose p_t among unused positions. The new minimum m' must be <= c_{t-1} for next constraint (if t-1>=1). Since c_{t-1} <= c_t? c_t nondecreasing with t, so c_{t-1} <= c_t. The next allowed minimum is <= c_{t-1}, which is stricter (smaller or equal). If current m > c_{t-1}, we must choose p_t <= c_{t-1} to lower minimum. If current m <= c_{t-1}, any unused position works (minimum remains <=c_{t-1}). This suggests a simple count!

Let's formalize. c_t = #B before t-th W, for t=1..N. c_N=N (last W after all B? S2N=W => #B before last W = N). c_t nondecreasing, c_t between 1 and N. Constraints for t=1..N-1: after processing values >t, min m_t <= c_t.
Process t=N down to 1:
- We have chosen positions for values t+1..N, with current minimum m_t. It must satisfy m_t <= c_t (for t=N-1..1). For t=N, no constraint? m_N=∞ before value N; after choosing p_N, m_{N-1}=p_N must <= c_{N-1}.
- Choose p_t from unused positions. New m_{t-1}=min(m_t,p_t) must satisfy m_{t-1} <= c_{t-1} (if t-1>=1).
Since c_{t-1} <= c_t.
Given m_t (which is <=c_t), how many choices for p_t lead to valid next?
If m_t <= c_{t-1}: then regardless of p_t, new min <=m_t<=c_{t-1}. Number choices = N - (N-t) = t unused positions? At step t, values t+1..N chosen (N-t positions), so t unused. All t choices valid.
If m_t > c_{t-1}: then must choose p_t <= c_{t-1}. How many unused positions <=c_{t-1}? Since current minimum m_t > c_{t-1}, none of the chosen positions are <=c_{t-1} (because min > c_{t-1}). Thus all c_{t-1} positions are unused. Choices = c_{t-1}.
But the distribution of m_t matters? Maybe we only need count of states where m_t <= c_{t-1} vs >? However m_t exact may affect future? Future c decreases further. But maybe the number of valid completions depends only on whether current min <= next c, not exact, because if min <= c_{t-1}, all choices valid and new min could be p_t if smaller, which may be <=c_{t-1}; for next step, we need know if new min <= c_{t-2}. If p_t chosen > c_{t-2} and m_t > c_{t-2}, then new min > c_{t-2} invalid? Wait if m_t <= c_{t-1} but could be > c_{t-2}. If choose p_t > c_{t-2}, new min = m_t (if m_t<p_t) or p_t, both >c_{t-2}, violating next constraint. But the choice p_t must ensure new min <= c_{t-2}. So not all t choices valid if m_t > c_{t-2}. My step only ensured next constraint c_{t-1}, but actually when choosing p_t, we need new m_{t-1} <= c_{t-1} (constraint for t-1). That's what we used. For future, state m_{t-1} exact matters.

But maybe we can process with a simple invariant: At step t, all valid states have m_t <= c_t. To choose p_t, need min(m_t,p_t) <= c_{t-1}. Number of choices depends on m_t relative to c_{t-1}: if m_t <= c_{t-1}, choices = t (all unused); if m_t > c_{t-1}, choices = c_{t-1} (must pick <=c_{t-1}). The new m_{t-1} will be <=c_{t-1}. For next step, need distribution of m_{t-1} relative to c_{t-2}. Could maybe aggregate by intervals between c's.

Since c_t is nondecreasing, c_{t-1} <= c_t. The thresholds are nonincreasing as we go down. This is like counting permutations with a "parking" condition; maybe can be done by product of (c_t - t + 1) after all? Let's test with DP small.

Let's derive count via inclusion-exclusion or rook. Condition r_t <= c_t for t=1..N-1. Equivalent: For each t, not all values >t are in positions >c_t. The sets of positions >c_t are nested decreasing as t increases? c_t nondecreasing, so suffix positions >c_t shrink as t increases. Events A_t: all values >t in positions >c_t. For t1<t2, A_{t2} (fewer values, smaller suffix) implies? If all values >t2 in >c_{t2}, does it imply all values >t1 in >c_{t1}? No, values t1+1..t2 could be in prefix. So not nested.

Maybe use determinant of a matrix (Lindstrom-Gessel-Viennot) for permutations with forbidden positions? Condition M_b > e_b means for each b, at least one of first b rows has column >e_b. This is not a simple forbidden position set; it's a global condition. But can be transformed: The set of permutations where M_b <= e_b for some b. The first bad b decomposition gave H recurrence. Maybe H_N has a product formula via "Gessel-Viennot" for paths? Let's examine H recurrence more.

H_b counts injections first b black to first e_b white with no earlier closed prefix. This is exactly number of ways to choose b distinct white indices from [e_b] and assign to b black such that for all c<b, not all first c assigned within [e_c]. This is like counting permutations of a subset with prefix max > e_c. There may be a closed form: H_b = ∏_{i=1}^b (e_i - e_{i-1}?)? Let's test.

For b=1: H1=e1!/(e1-1)! = e1. In examples: e1=1 =>1, e1=2=>2. Product? e1.
For b=2 valid e2>e1>=? H2 = (e2! - H1*(e2-1)!)/(e2-2)! = (e2! - e1*(e2-1)!)/(e2-2)! = (e2-1)!/(e2-2)! * (e2 - e1) = (e2-1)*(e2-e1). If e1=1,e2=2 =>1*1=1. If e1=2,e2=3 =>2*1=2? Let's test sequence e=[0,2,3,?] N=3 maybe S=B W W B W B? last B invalid; but H2=2. Could be.
For b=3: H3 = (e3! - H1(e3-1)! - H2(e3-2)!)/(e3-3)!.
Plug H1=e1, H2=(e2-1)(e2-e1). Is there product? Suppose e=[0,1,2,3]: H1=1,H2=1,H3=3. Product of (e_i - i +1)? i1:1, i2:1, i3:1 =>1 no. Maybe H_b = number of permutations of b with prefix max > e_i? For b=3 e=[0,1,2], count permutations of 3 with M1>1 (π1>1), M2>2 (3 in first2) =3. OK.

Could H_b be determinant of triangular matrix with entries? Recurrence H_b fact[e_b-b] = fact[e_b] - sum H_c fact[e_b-c]. This is like solving a lower triangular linear system:
For each b: sum_{c<=b} H_c fact[e_b-c] = fact[e_b], with c=b term H_b fact[e_b-b]. Let A_{b,c}=fact[e_b-c] for c<=b. Need solve A H = fact[e]. A is lower triangular (in compressed valid order) with diagonal fact[e_b-b]. This is a Toeplitz-like matrix if e_b-c depends on e_b and c. If e_b were b, A is Toeplitz. General e_b arbitrary but monotone. Could use fast triangular solve with divide and conquer (CDQ) as planned. No simple product generally.

Maybe use Bostan-Mori? No.

Given task explicitly asks implement CDQ NTT, we should do that. Need make Python pass. Let's optimize as much as possible.

Potential improvement: Use "relaxed convolution" / online CDQ with NTT but total O(N log N) if we use a single NTT per level? CDQ already per node. Could use "divide and conquer with FFT" total O(N log^2 N). For N=2e5 in Python maybe tough but maybe constraints from AtCoder allow PyPy with optimized NTT? Let's search memory: AtCoder problem? Maybe AGC? Mod 998244353. N 2e5. Python solutions may exist using CDQ NTT? Need be efficient.

Could use PyPy and implement NTT with `array`? Let's benchmark mentally. 45M butterflies in Python likely too slow (>10s). Need reduce total NTT work.

Maybe we can choose direct threshold higher to reduce NTT levels? Direct operations also Python loops but simpler. Which is faster per operation? Let's estimate. A direct inner operation: `s -= hj * fact[ei - bj]` involves list indexing, multiplication of Python ints (<MOD) -> <1e18, subtraction. No modulo. Maybe ~100 ns? Actually Python ~50-100 ns? No, Python loop ~50 ns? More like 50-100 ns in C? Python bytecode ~50 ns? In PyPy maybe 20-50 ns? 25M operations maybe 1-2s? NTT butterfly: multiple Python operations, mod `%` (expensive), maybe 200-500 ns. 45M butterflies maybe 10-20s. So direct may be faster than NTT for moderate sizes. We should favor direct more. Maybe set direct threshold much higher, e.g. direct_cost <= 200000 or 500000. Then NTT only for large nodes, reducing NTT calls. Direct total pairs if threshold 200k: dense nodes length up to sqrt(200k)~447 direct. Direct pairs ~N*447/2=44.7M. NTT starts length512? length512 product65k direct, length1024 product262k NTT. NTT levels from 1024 up: sum butterflies ~3*N*sum_{10..18} log? ~3*2e5*(10+...+18)=3*2e5*126=75M? Wait per transform? Let's recalc: NTT cost per node per transform ~ n/2 log n butterflies. Sum over nodes at level with segment length S: number N/S, n~S (or 2S? L+M~S? For dense, left S/2, M~S/2, need~S, n~S). Sum n log n per level = N log S. Per transform. Three transforms => 1.5 N log S? Actually n/2 log n *3 = 1.5 n log n. Sum level =1.5 N log S. For S=1024 log10 =>3M butterflies; S=2048 log11=>3.3M; ... S=262144 log18=>5.4M. Sum ~30M butterflies. If start at 1024, ~30M. If start at 512, add 2.7M =>33M. Not huge difference. Direct 45M vs 25M. Maybe direct 45M + NTT30M vs direct25M+NTT33M. Similar. Need optimize NTT or direct.

Maybe use direct for all? O(N^2) impossible. But maybe M (valid count) often smaller? Worst-case alternating BWBW... M=N. Need handle.

Can we use a faster convolution method for dense case using one NTT per level with "offline" across nodes? At each CDQ level, we have many independent convolutions of left H segments with fact ranges for right segments. Could batch them into a single large convolution using packing? Maybe. Since at a given level, segments are disjoint in index and e ranges. We could concatenate with gaps to avoid interference? This is like batched convolution. Might reduce overhead but total work same. Python overhead of many NTT calls is high; batching could help. But complex.

Alternative use "CDQ with direct for small and NTT for large" but implement NTT in C? Not possible. Could use `numpy` if available? AtCoder Python environment has numpy? Usually not allowed? AtCoder has PyPy without numpy? Actually AtCoder Python (CPython) may have numpy? Not guaranteed, and competitive programming solutions shouldn't rely. We need pure Python.

Maybe there is an O(N log N) algorithm using formal power series and the fact e_b is a path. Let's search mind: Counting matchings making graph strongly connected on a path with colored vertices. This might be AtCoder AGC? There may be a simple answer: product over i of (something) if S1=B,S2N=W, else 0. Let's test with samples. Sample3 output 240792. Could be product of block sizes? Let's compute for sample3 to see patterns.

Sample3 N=9 S=BWWBWBBBWWBWBBWWBW (length18). Let's index colors:
1 B
2 W
3 W
4 B
5 W
6 B
7 B
8 B
9 W
10 W
11 B
12 W
13 B
14 B
15 W
16 W
17 B
18 W
Count B: 1,4,6,7,8,11,13,14,17 =9. W:2,3,5,9,10,12,15,16,18=9.
Compute e_b #W before B_{b+1}:
B1 pos1: e0=0
B2 pos4: W before pos4: 2,3 => e1=2
B3 pos6: W before:2,3,5 => e2=3
B4 pos7: same 3 => e3=3
B5 pos8: same 3 => e4=3
B6 pos11: W before:2,3,5,9,10 => e5=5
B7 pos13: W before: +12 => e6=6
B8 pos14: same 6 => e7=6
B9 pos17: W before: +15,16 => e8=8
e9=N=9.
Valid b: e_b>=b and e_b>e_{b-1} (e_{-1}? b1 e1>0)
b1 e1=2 valid H1=2
b2 e2=3 valid
b3 e3=3 not >e2? e3=3,e2=3 invalid
b4 e4=3 invalid (also e4<4)
b5 e5=5 valid
b6 e6=6 valid
b7 e7=6 invalid
b8 e8=8 valid
b9 e9=9 valid
M=6? valid indices b:1,2,5,6,8,9.
Let's compute recurrence manually maybe to see answer 240792.
fact: 0=1,1=1,2=2,3=6,4=24,5=120,6=720,7=5040,8=40320,9=362880.
H1: e=2, denom fact1=1 =>2.
H2: e=3, sum H1*fact[2]=2*2=4, fact3=6 => (6-4)*invfact1=2.
H5: b=5,e=5. sum c=1: H1*fact[4]=2*24=48; c=2: H2*fact[3]=2*6=12; total60. fact5=120. denom fact0=1 =>60.
H6: b=6,e=6. sum c1 fact5=2*120=240; c2 fact4=2*24=48; c5 H5*fact1=60; total348. fact6=720 =>372.
H8: b=8,e=8. sum c1 fact7=2*5040=10080; c2 fact6=2*720=1440; c5 fact3=60*6=360; c6 fact2=372*2=744; total12624. fact8=40320. denom fact0=1 =>27696.
H9: b=9,e=9. sum c1 fact8=2*40320=80640; c2 fact7=2*5040=10080; c5 fact4=60*24=1440; c6 fact3=372*6=2232; c8 fact1=27696; total122088. fact9=362880. denom fact0=1 =>240792. Matches sample. Good.

Notice valid b with e_b=b? H5 e=5, H6 e=6, H8 e=8, H9 e=9. Denom fact0. H values large.

Could there be formula using only valid b where e_b=b? In sample, H1 e=2, H2 e=3 not equal. Answer 240792. Maybe product? 2*2*60*372*27696*240792 no.

Maybe H_N equals number of standard Young tableaux of shape given by path? Let's check. Condition M_b > e_b. The complement M_b <= e_b for all b? Permutations whose prefix maxima stay below path. This is related to "permutations with bounded prefix maxima" counted by determinant of binomial coefficients, maybe via inclusion-exclusion. The first bad decomposition H_b might correspond to irreducible components; H_N maybe a determinant or product of differences of e? Let's explore.

Count permutations π with M_b > e_b for all b<N. Let total N! minus those with some M_b <= e_b. The first bad b count = H_b*(N-b)!? We derived H_b injections first closed b. Maybe H_b has closed form: H_b = ∏_{i=1}^b (e_i - e_{i-1}?)? Let's test sample valid b not contiguous. For b=2 e=[2,3], H2=2. Product increments: e1-e0=2, e2-e1=1 =>2. H1=2. For b=5 with e sequence valid? Need include invalid e: e0=0,e1=2,e2=3,e3=3,e4=3,e5=5. H5=60. Product of positive increments? 2*1*2=4 no. Maybe H_b = number of permutations of b with prefix max > e_i (for i<b) and max <=e_b? There is known formula: number of permutations of [e_b] of length b with prefix max constraints? Could be determinant of matrix (e_i - i +1 choose ...). Not simple product.

Let's derive H_b via inclusion-exclusion over closed prefixes. H_b counts injections from [b] to [e_b] such that for all c<b, not (image of [c] subset [e_c]). This is like counting partial permutations avoiding nested forbidden sets. There is a general formula using rook polynomials: number = ∑_{T subset [b-1]} (-1)^{|T|} ... But due to nested sets, maybe only chains matter, leading to recurrence. Could solve with DP over e values? e_b up to N.

Maybe H_b can be computed by a simple stack when e_b has jumps? Let's examine recurrence in terms of d_b=e_b-b. H_b = fact[e_b] * invfact[d_b] - sum H_c fact[e_b-c] invfact[d_b]. If d_b=0 often, H_b = fact[e_b] - sum... In sample many d=0.

Could use polynomial interpolation? No.

Let's focus on implementation. Need ensure code within answer. We can maybe include a fallback for small N naive? Not needed.

Optimization ideas for CDQ NTT:
1. Compress valid indices M. If M small (e.g. <=2000), use naive O(M^2) maybe faster than NTT overhead. We can set if M <= 2000 or 5000, do direct DP O(M^2). For worst M=2e5 use CDQ.
2. In CDQ, use adaptive direct/NTT. Maybe set direct threshold high enough to avoid many small NTTs.
3. Optimize NTT:
   - Precompute roots for each power length up to max_n. Use iterative with bit-reversal? Maybe use a faster "butterfly" implementation from ACL Python that avoids bit-reversal and uses precomputed rate2/rate3. Let's consider implementing ACL's butterfly for mod 998244353. It is optimized for Python? AtCoder Library Python implementation uses `modint` class which is slow. But we can adapt with ints.

ACL butterfly (from atcoder library) for convolution:
```
def butterfly(a):
    n = len(a)
    h = (n - 1).bit_length()
    first = True
    for ph in range(1, h + 1):
        w = 1 << (ph - 1)
        p = 1 << (h - ph)
        now = 1
        for s in range(w):
            offset = s << (h - ph + 1)
            for i in range(p):
                l = a[i + offset]
                r = a[i + offset + p] * now % MOD
                a[i + offset] = (l + r) % MOD
                a[i + offset + p] = (l - r) % MOD
            now = now * rate2[(~s & -~s).bit_length() - 1] % MOD
```
And inverse butterfly similar. This avoids bit-reversal and uses precomputed roots `rate2`, `rate3`. It may be faster because inner loop simpler? It still has modulo for add/sub. It processes in a different order. Need adapt correctly.

Let's recall full ACL Python convolution (from atcoder library):
```
MOD = 998244353
G = 3

def primitive_root(m): ...

def butterfly(a):
    n = len(a)
    h = (n - 1).bit_length()
    first = True
    for ph in range(1, h + 1):
        w = 1 << (ph - 1)
        p = 1 << (h - ph)
        now = 1
        for s in range(w):
            offset = s << (h - ph + 1)
            for i in range(p):
                l = a[i + offset]
                r = a[i + offset + p] * now % MOD
                a[i + offset] = (l + r) % MOD
                a[i + offset + p] = (l - r) % MOD
            now = now * rate2[(~s & -~s).bit_length() - 1] % MOD
```
But `rate2` precomputed:
```
rate2 = [0]*30
irate2 = [0]*30
prod = 1
iprod = 1
for i in range(30):
    rate2[i] = pow(G, (MOD-1) >> (i+2), MOD)
    irate2[i] = pow(rate2[i], MOD-2, MOD)
```
Actually ACL uses `rate2[(~s & -~s).bit_length() - 1]` to update now. This is complex but can be copied.

Inverse butterfly:
```
def butterfly_inv(a):
    n = len(a)
    h = (n - 1).bit_length()
    for ph in range(h, 0, -1):
        w = 1 << (ph - 1)
        p = 1 << (h - ph)
        inow = 1
        for s in range(w):
            offset = s << (h - ph + 1)
            for i in range(p):
                l = a[i + offset]
                r = a[i + offset + p]
                a[i + offset] = (l + r) % MOD
                a[i + offset + p] = (l - r) * inow % MOD
            inow = inow * irate2[(~s & -~s).bit_length() - 1] % MOD
    inv_n = pow(n, MOD-2, MOD)
    for i in range(n): a[i] = a[i] * inv_n % MOD
```
Need verify. This ACL butterfly outputs in normal order? I think yes, it's a decimation-in-frequency/Cooley without bit reversal. It is used in ACL convolution. It may be faster due to no bit-reversal and fewer pow. But inner loops have `% MOD` for add/sub; we can optimize with if.

Let's test mentally for n=2: h=1, ph=1, w=1,p=1, now=1, s=0 offset0, i0: l=a0,r=a1, a0=l+r,a1=l-r. Good.
For n=4: h=2. ph=1: w=1,p=2, now=1, offset0, i=0,1: pairs (0,2),(1,3). ph=2: w=2,p=1. s=0 offset0 now=1: pair (0,1). update now *= rate2[(~0 & -~0).bit_length()-1]. ~0=-1, -~0=1, bit_length1 => index0. rate2[0]=G^{(MOD-1)/4}? For ph=2? s=1 offset=2, now=rate2[0], pair (2,3). This seems correct.

Precomputing rate2 up to max log (19). Need formula. Let's derive from ACL:
```
rate2 = [0]*30
irate2 = [0]*30
prod = 1
iprod = 1
for i in range(30):
    rate2[i] = pow(G, (MOD-1) >> (i+2), MOD)
    irate2[i] = pow(rate2[i], MOD-2, MOD)
```
But update uses `rate2[(~s & -~s).bit_length() - 1]`. For s=0 index0. For s=1: ~1=-2, -~1=2, bit_length2 => index1. For s=2: ~2=-3, -~2=3, bit_length2 => index1. For s=3: -~3=4 bit_length3 => index2. So index = number of trailing zeros of s+1? Actually (s+1) & -(s+1) bit_length-1. Yes. This updates now by multiplying by root for each block.

Need ensure rate2[i] corresponds to primitive 2^{i+2}-th root? For i=0, G^{(MOD-1)/4} = sqrt(-1)? For n=4, ph=2, s=1 now should be primitive 4th root? G^{(MOD-1)/4} yes. Good.

This ACL butterfly may be faster. Let's adapt with local variables and avoid `%` where possible:
Forward:
```
def ntt(a):
    n=len(a); h=n.bit_length()-1
    for ph in range(1,h+1):
        w=1<<(ph-1); p=1<<(h-ph)
        now=1
        for s in range(w):
            offset=s<<(h-ph+1)
            for i in range(p):
                l=a[i+offset]
                r=a[i+offset+p]*now%MOD
                x=l+r
                if x>=MOD: x-=MOD
                y=l-r
                if y<0: y+=MOD
                a[i+offset]=x
                a[i+offset+p]=y
            # update now
            now = now * rate2[(~s & -~s).bit_length()-1] % MOD
```
But note `now` update after each s. For s where p maybe large. Good.
Inverse:
```
def intt(a):
    n=len(a); h=n.bit_length()-1
    for ph in range(h,0,-1):
        w=1<<(ph-1); p=1<<(h-ph)
        inow=1
        for s in range(w):
            offset=s<<(h-ph+1)
            for i in range(p):
                l=a[i+offset]
                r=a[i+offset+p]
                x=l+r
                if x>=MOD: x-=MOD
                y=l-r
                if y<0: y+=MOD
                a[i+offset]=x
                a[i+offset+p]=y*inow%MOD
            inow = inow * irate2[(~s & -~s).bit_length()-1] % MOD
    inv_n=pow(n,MOD-2,MOD)
    for i in range(n): a[i]=a[i]*inv_n%MOD
```
Need verify inverse formula: In ACL, `a[i+offset+p] = (l - r) * inow % mod`. Yes.

This avoids bit-reversal and pow per stage. Precompute rate2 up to max_n.bit_length(). For MOD 998244353, max 2^23. We need up to 19. Use 30.

Potential issue: ACL butterfly assumes n is power of two and uses `h=(n-1).bit_length()`. For n=1, h=0, loops none. Convolution length maybe 1; handle direct.

Let's test ACL butterfly with small convolution manually? Could trust but need be certain. Let's maybe derive or test mentally n=4 forward with input [a0,a1,a2,a3]. ph=1 p=2:
i0: a0'=a0+a2, a2'=a0-a2
i1: a1'=a1+a3, a3'=a1-a3
ph=2 p=1:
s=0 now=1 offset0:
i0: l=a0', r=a1'; a0''=a0'+a1'=a0+a2+a1+a3; a1''=a0'-a1'=a0+a2-a1-a3.
update now=rate2[0]=w4 (primitive 4th root).
s=1 offset=2:
i0: l=a2', r=a3'*w4; a2''=a2'+r = a0-a2 + (a1-a3)w4; a3''=a2'-r.
Expected DFT: X0=a0+a1+a2+a3; X1=a0+a1w+a2w^2+a3w^3 = a0 - a2 + w(a1-a3). Yes a2'' = X1. X2=a0-a1+a2-a3 = a0+a2-a1-a3 = a1''. X3=a0-a1w+a2w^2-a3w^3 = a0-a2 - w(a1-a3) = a3''. Output order [X0,X2,X1,X3]? Wait a1''=X2, a2''=X1. So output is bit-reversed order? For n=4, order [0,2,1,3]. ACL butterfly maybe outputs in bit-reversed order? But convolution in ACL uses butterfly then pointwise then butterfly_inv, and inverse expects same order? If both use same order, pointwise multiplication in bit-reversed order is okay if both transforms produce same permutation. Inverse butterfly likely inverse of that permutation, yielding normal order. So we don't need normal order. Good.

Need ensure pointwise multiplication works with bit-reversed order: If forward outputs F[rev(k)], inverse expects F[rev(k)] and returns f[k]. Since both sequences transformed with same permutation, product in rev order corresponds to product of DFTs in rev order, inverse works. Good.

Let's verify inverse ACL with n=4. It should invert. Trust.

This ACL NTT may be faster. We'll use it.

Precompute rate2/irate2:
```
MAXLOG = 20  # since n up to 2^19? 2N=4e5 <2^19=524288, log=19. But safe 30.
rate2=[0]*30; irate2=[0]*30
for i in range(30):
    rate2[i]=pow(G,(MOD-1) >> (i+2), MOD)
    irate2[i]=pow(rate2[i], MOD-2, MOD)
```
Check for i up to 29, (MOD-1) >> (i+2) integer. MOD-1=119*2^23. For i=29 shift31 ->0, pow(G,0)=1. Fine.

But update index can be up to? s < w = 2^{ph-1}, max s=2^{ph-1}-1, s+1 power of two 2^{ph-1}, bit_length-1=ph-1. ph up to h. So need rate2[h-1]. For n=2^h, h<=19. Precompute 30.

Potential performance: The expression `(~s & -~s).bit_length() - 1` inside s loop (outer) not inner, okay. Could precompute `rate2_update[s]` for s up to max w? Max w=2^{h-1} up to 262144. Precomputing list of length 262144 maybe okay and faster. But s loop total per NTT O(n), computing bit_length maybe overhead. We can precompute `tz = [0]*(max_n//2+1)` where tz[s] = rate2[(s+1).bit_length()-1]? Actually index = (s+1).bit_length()-1 for s+1 power? Wait for s=2, s+1=3 bit_length2 index1, but trailing zeros of s+1? 3 has tz0, not 1. Let's compute formula: (~s & -~s) = s+1? In two's complement, -~s = s+1. ~s & (s+1) gives lowest set bit of s+1? For s=2 (10), s+1=3 (11), lowest set bit 1, bit_length-1=0, but earlier I thought index1. Let's recalc: ~2 = -3, -~2 = 3, -3 & 3 = 1? In Python, -3 & 3 = 1. bit_length 1 => index0. For s=1: -2 & 2 =2 index1. For s=3: -4 &4=4 index2. So index = trailing zeros of s+1. Yes. For s=2 index0. So update now multiplies by rate2[tz(s+1)]. This is ACL.

We can precompute `rate2_tz[s] = rate2[(s+1 & -(s+1)).bit_length()-1]` for s up to max_w-1. But max_w across NTT calls up to n/2. Precompute up to max_n//2 (262144) once. Then in ntt: `now = now * rate2_tz[s] % MOD`. This avoids bit ops. Memory list 262k ints ~7MB, okay. Or compute on fly; s loop total per transform sum n/2? Actually s loop count w per ph, sum w = n-1. So ~n bit ops per transform. Precompute may help.

But rate2_tz depends on rate2. Let's precompute up to MAXN//2 maybe after determining max_n? We can determine max possible n = 1 << ((2*N).bit_length())? Need L+Mlen-1 <=? L<=N, Mlen<=N, need<=2N. max_n=1; while max_n<2*N: max_n<<=1. For N=2e5, max_n=524288. Precompute tz_rate length max_n//2 (262144). For inverse use irate2_tz.

However, in ACL butterfly, `now` update after each s uses rate2[tz(s+1)] independent of ph? Yes. For ph smaller, s max smaller, same. Good.

Let's implement ntt with precomputed `rate2_tz`, `irate2_tz`:
```
def ntt(a):
    n=len(a)
    h=n.bit_length()-1
    for ph in range(1,h+1):
        w=1<<(ph-1)
        p=1<<(h-ph)
        now=1
        shift=h-ph+1
        for s in range(w):
            offset=s<<shift
            base=offset
            end=base+p
            # inner loop
            for i in range(p):
                l=a[base+i]
                r=a[base+i+p]*now%MOD
                x=l+r
                if x>=MOD: x-=MOD
                y=l-r
                if y<0: y+=MOD
                a[base+i]=x
                a[base+i+p]=y
            now = now * rate2_tz[s] % MOD
```
But `rate2_tz[s]` for s up to w-1. For s=0, tz(1)=0 rate2[0]. Wait for ph=1, w=1, s=0, update now after only s not used. It multiplies by rate2[0] but loop ends. Fine. For ph=2, s=0 update to rate2[0], s=1 uses now=rate2[0], then update to rate2[0]*rate2[1]? But expected now for s=1 is rate2[0]? Let's check ACL: now initialized 1. For s=0, after block, now *= rate2[tz(1)=0] => w4. For s=1, uses w4. After s=1, now *= rate2[tz(2)=1] => w4*w8? But no more s. For ph=3, s sequence now values? Expected now for s is product of rate2[tz(k+1)] for k<s? This yields primitive root powers? ACL uses this. Good.

But precomputing `rate2_tz[s]` as single factor, not cumulative. We multiply cumulatively. Good.

Inner loop optimization: Use local variables `a_local=a`, `MOD_local=MOD`, `now_local=now`. Python for loop with range(p). Could use while loop? For small p, for okay. Maybe use `for i in range(base, base+p):` and access `a[i]`, `a[i+p]`. This avoids base+i each? Let's write:
```
for i in range(base, base+p):
    l = a[i]
    r = a[i+p] * now % MOD
    x = l + r
    if x >= MOD: x -= MOD
    y = l - r
    if y < 0: y += MOD
    a[i] = x
    a[i+p] = y
```
This is fine.

Inverse similar:
```
def intt(a):
    n=len(a); h=n.bit_length()-1
    for ph in range(h,0,-1):
        w=1<<(ph-1); p=1<<(h-ph); shift=h-ph+1
        inow=1
        for s in range(w):
            base=s<<shift
            for i in range(base, base+p):
                l=a[i]; r=a[i+p]
                x=l+r
                if x>=MOD: x-=MOD
                y=l-r
                if y<0: y+=MOD
                a[i]=x
                a[i+p]=y*inow%MOD
            inow = inow * irate2_tz[s] % MOD
    inv_n=pow(n,MOD-2,MOD)
    for i in range(n): a[i]=a[i]*inv_n%MOD
```
Need verify inverse with bit-reversed order. Trust ACL.

Could further optimize by combining pointwise multiplication with inverse? No.

Convolution function using ACL ntt:
```
def convolution(a,b):
    need=len(a)+len(b)-1
    n=1<<(need-1).bit_length()
    fa=a[:] + [0]*(n-len(a))
    fb=b[:] + [0]*(n-len(b))
    ntt(fa); ntt(fb)
    for i in range(n): fa[i]=fa[i]*fb[i]%MOD
    intt(fa)
    return fa[:need]
```
But in apply, we can avoid returning full conv; we only need query indices. We can keep fa length n and query. No need slice. Build fa,fb directly.

Potential issue: ACL butterfly modifies list in bit-reversed order; pointwise multiplication okay. Inverse returns normal order? I think yes. Let's test with n=4 forward output [X0,X2,X1,X3]. Inverse ACL should take that and output original. Let's manually test inverse n=4 with input [X0,X2,X1,X3]. h=2.
ph=2 (inverse first): w=2,p=1, inow=1.
s=0 base0: l=X0,r=X2; a0=X0+X2, a1=(X0-X2)*1.
update inow=irate2[0]=w4^{-1}.
s=1 base2: l=X1,r=X3; a2=X1+X3, a3=(X1-X3)*w4^{-1}.
ph=1: w=1,p=2,inow=1.
i=0: l=a0, r=a2; a0=l+r, a2=l-r.
i=1: l=a1, r=a3; a1=l+r, a3=l-r.
Then multiply inv_n.
Compute X0=a0+a1+a2+a3, X2=a0-a1+a2-a3, X1=a0-a2+w(a1-a3), X3=a0-a2-w(a1-a3).
After ph2:
a0=X0+X2=2a0+2a2
a1=X0-X2=2a1+2a3
a2=X1+X3=2a0-2a2
a3=(X1-X3)*w^{-1}= (2w(a1-a3))*w^{-1}=2a1-2a3
ph1:
a0=4a0, a2=4a2, a1=4a1, a3=4a3. inv4 -> original. Good.

So ACL NTT works.

Now, can we precompute fact and invfact efficiently:
```
fact=[1]*(N+1)
for i in range(1,N+1): fact[i]=fact[i-1]*i%MOD
invfact=[1]*(N+1)
invfact[N]=pow(fact[N],MOD-2,MOD)
for i in range(N,0,-1): invfact[i-1]=invfact[i]*i%MOD
```

Compute e and valid:
```
e=[0]*(N+1)
w=0; b=0
for ch in S:
    if ch=='W': w+=1
    else:
        b+=1
        if b<=N: e[b-1]=w
e[N]=N
```
But if S[0]!='B' or S[-1]!='W': print(0). Need still maybe N=1? S='BW' ok.
Then build B,E arrays 1-indexed:
```
B=[0]; E=[0]
for b in range(1,N+1):
    if e[b] >= b and e[b] > e[b-1]:
        B.append(b); E.append(e[b])
M=len(B)-1
if M==0: print(0) # but N valid? maybe if no valid? If N=0 no.
```
For b=N, e[N]=N, e[N]>e[N-1] due last W, e[N]>=N, so included. M>=1.

If M small, naive:
```
H=[0]*(M+1); val=[0]*(M+1)
for i in range(1,M+1): val[i]=fact[E[i]]
for i in range(1,M+1):
    s=val[i]
    for j in range(1,i):
        if H[j]: s -= H[j]*fact[E[i]-B[j]]
    H[i]=s%MOD*invfact[E[i]-B[i]]%MOD
print(H[M])
```
Threshold maybe M<=2000 (2M ops) or 5000 (12.5M) okay. Use 2000 to be safe. But CDQ can handle small too; naive avoids NTT overhead. Set NAIVE_LIMIT=2000 or 3000.

For CDQ, arrays 1-indexed length M+1.
`val=[0]*(M+1); for i in 1..M: val[i]=fact[E[i]]`
`H=[0]*(M+1)`
`nonzero=[]`

Need recursion. Python recursion limit set 1e6.

Function `apply(l,m,ql,qr)`:
- Get c_list. Since nonzero sorted and all nonzero <=m? In apply after solve(l,m), yes. But nonzero may include indices <l and maybe >m? No >m not solved. But to be safe use bisect.
```
import bisect
lo=bisect.bisect_left(nonzero,l)
hi=bisect.bisect_right(nonzero,m)
if lo==hi: return
nz=hi-lo
# c_list = nonzero[lo:hi] maybe slice creates list. For direct/NTT need list. Slicing O(nz). OK.
c_list = nonzero[lo:hi]
```
But for NTT building A, we can iterate over nonzero[lo:hi]. Slicing memory okay. For direct, need pairs. Could avoid slice by using indices lo..hi-1. But for direct inner, accessing nonzero[k] each time. We can create `c_idx = nonzero[lo:hi]`.

- rb = qr-ql+1.
- direct_cost = nz*rb.
- Determine if direct. Need maybe compute ntt estimate only if direct_cost > DIRECT_ALWAYS.
```
if direct_cost <= DIRECT_ALWAYS:
    do_direct
    return
# estimate ntt
c0=B[c_list[0]]; c1=B[c_list[-1]]
L=c1-c0+1
t_min=E[ql]-c1
if t_min<0: t_min=0
t_max=E[qr]-c0
Mlen=t_max-t_min+1
need=L+Mlen-1
n=1<<(need-1).bit_length()
ntt_cost = n * (n.bit_length()-1)  # rough
if direct_cost <= ntt_cost * 2:  # favor direct
    do_direct
else:
    do_ntt
```
But if t_min<0? Shouldn't for valid, but if c1 > E[ql]? Could happen if c1 (black index) > e[ql]? Since c1 < B[ql] <= E[ql] (valid E>=B), so c1 < E[ql], t_min>=1. Good.
Mlen maybe large. If Mlen<=0? no.

Direct function:
```
# precompute pairs? For each c in c_list: hj=H[c], bj=B[c]
for i in range(ql, qr+1):
    ei=E[i]
    s=val[i]
    for c in c_list:
        s -= H[c] * fact[ei - B[c]]
    val[i] = s % MOD
```
This accesses H[c],B[c] repeatedly for each i. Better create lists `hj_list`, `bj_list` once:
```
hj_list=[H[c] for c in c_list]
bj_list=[B[c] for c in c_list]
for i in range(ql,qr+1):
    ei=E[i]; s=val[i]
    for k in range(nz):
        s -= hj_list[k] * fact[ei - bj_list[k]]
    val[i]=s%MOD
```
But creating lists O(nz). For direct_cost small okay. For NTT, we also need c_list to fill A.

Could optimize direct by looping c outer and updating val[i] with modulo less? If c outer:
```
for k in range(nz):
    hj=hj_list[k]; bj=bj_list[k]
    for i in range(ql,qr+1):
        val[i] -= hj * fact[E[i]-bj]
# after all, mod val[i] for i in range
for i in range(ql,qr+1): val[i] %= MOD
```
This may be faster because inner loop over i sequential, E[i] list access, fact index. But val[i] can grow large: total updates per val = nz. If direct_cost up to maybe 200k, nz could be 200k if rb=1, val magnitude 200k*1e18=2e23 (78 bits) okay. If direct_cost threshold 1e6, 1e6*1e18=1e24 (80 bits) okay. Python big ints 3 limbs, okay. Mod once per i at end. This reduces modulo operations from direct_cost to rb. But val[i] may be used in subsequent applies before mod? We mod at end of direct apply, so fine. In NTT apply, we mod val[i] after subtract term. In solve leaf, val[i] should be modded before computing H? We can mod there.

Which loop order faster? c outer has inner loop over rb, uses `val_i` list updates each time (read/write). i outer has inner loop over nz, accumulates local s (faster) and writes val once. i outer likely faster despite repeated H/B list access. We can precompute hj,bj and use local lists. i outer:
```
for i in range(ql, qr+1):
    ei=E[i]; s=val[i]
    for k in range(nz):
        s -= hj_list[k] * fact[ei - bj_list[k]]
    val[i]=s%MOD
```
Inner loop local variables, no list write. Good. If nz small and rb large, i outer does rb*nz. If nz=1, inner one, good. If rb=1, outer one, inner nz, good.

Can further optimize by if nz==1:
```
hj=hj_list[0]; bj=bj_list[0]
for i in range(ql,qr+1): val[i]=(val[i]-hj*fact[E[i]-bj])%MOD
```
Maybe.

For direct with nz maybe up to 1000, rb small, i outer okay.

NTT apply details with compressed indices:
```
c0 = B[c_list[0]]; c1 = B[c_list[-1]]
L = c1 - c0 + 1
t_min = E[ql] - c1
if t_min < 0: t_min = 0
t_max = E[qr] - c0
Mlen = t_max - t_min + 1
need = L + Mlen - 1
n = 1 << (need - 1).bit_length()
A = [0] * n
for c in c_list:
    A[B[c] - c0] = H[c]
F = [0] * n
# fill F[0:Mlen]
F[:Mlen] = fact[t_min:t_max+1]
ntt(A); ntt(F)
for i in range(n): A[i] = A[i] * F[i] % MOD
intt(A)
for i in range(ql, qr+1):
    idx = E[i] - c0 - t_min
    val[i] = (val[i] - A[idx]) % MOD
```
Need ensure `fact[t_min:t_max+1]` length Mlen. t_max <= N? E[qr]<=N, c0>=1, so t_max<=N-1 maybe. t_min>=0. Good.

Potential issue: A length L may be much larger than nz due to gaps. If L large but nz small, direct likely chosen by cost model? direct_cost=nz*rb. ntt estimate uses L+Mlen. If nz small, direct_cost small relative, direct chosen. Good.

But if nz large and L large due to gaps? nz large implies many nonzero in B range, gaps not too huge. OK.

Need choose constants. Let's think performance with ACL NTT. Maybe we can set DIRECT_ALWAYS=8000, and cost factor 1? We want avoid NTT for small. Let's maybe set:
```
DIRECT_ALWAYS = 4096
COST_FACTOR = 1  # direct if direct_cost <= ntt_cost
```
But ntt_cost rough n*log n; actual NTT has 3 transforms and pointwise, so cost ~3*n/2*log n butterflies + n pointwise. direct_cost operations simpler. Maybe direct if direct_cost <= 2*n*log n. Let's set factor 2 or 3. Need empirical but can't. We can choose conservative to reduce NTT calls: factor 4. Direct more, NTT fewer. Direct operations maybe cheaper. Let's estimate dense length512: direct_cost=65536, n=512, nlog=4608, factor4 =>18432, NTT. length256: direct=16384, n=256,nlog=2048,factor4=8192, NTT. length128: direct=4096, n=128,nlog=1024,factor4=4096, direct (<=). So NTT starts at 256. If factor8: length256 direct (16384<=16384), NTT starts512. Direct total more. Which is faster? NTT length256: 781 calls * 3 transforms * (256/2*8=1024 butterflies) = 2.4M butterflies + pointwise 0.2M. Direct length256: 12.8M simple ops. NTT likely faster? But 781 function calls overhead. Maybe NTT. Factor4 okay.

Leaf threshold: If LEAF=32, apply nodes length64: direct_cost=32*32=1024 direct. length128 direct if factor4? direct=4096, n=128,nlog=1024,factor4=4096 direct. length256 NTT. Good. Direct pairs: leaf internal N*32/2=3.2M; apply64: N/64*1024=3.2M; apply128: N/128*4096=6.4M; total 12.8M. NTT from256. Good.

If LEAF=64: leaf internal6.4M; apply128 direct6.4M; apply256 NTT. Total direct12.8M same? Actually leaf64 + apply128 =12.8M, no apply64. Similar. Fewer nodes. Use LEAF=64.

NAIVE_LIMIT maybe 2000. For M=2000, O(2M) okay. CDQ with NTT maybe overhead. Use 2000.

Potential recursion apply direct uses `nonzero` list which grows. In solve leaf, after computing H[i], if H[i]!=0 append i. But in CDQ, when solving left half, nonzero appended. Then apply uses nonzero up to m. Good. However, in solve(l,r) if l>r? no.

Need ensure val[i] includes contributions from all c<i before H[i] computed. CDQ invariant:
- Initially val[i]=fact[E[i]].
- solve(l,r): before call, val[i] for i in [l,r] includes contributions from c<l. H[c] for c<l finalized.
- If leaf: for i l..r, compute H[i] using val[i] plus direct contributions from c in [l,i-1] (which are finalized in this leaf). It does not update val[i] for future, but H[i] appended. Future applies from this segment to right will use H[i]. Good.
- If internal: solve(l,m) finalizes left and val for left not needed. apply adds contributions from left to val[i] for i in [m+1,r]. Now before solve(m+1,r), val[i] includes c<l (old) + c in [l,m]. Good.

But in leaf direct, we compute H[i] = (val[i] - sum_{c=l}^{i-1} H[c]*fact[...]) * invfact. val[i] includes c<l. Good. We don't subtract contributions from c in [l,i-1] from val[i] for future? Not needed because future right segments will get contributions via apply from this leaf segment as a whole, not via val. But what about within same leaf, for i+1, we directly sum H[i]. Good.

Now, one subtlety: In apply NTT, we convolve H[c] for c in [l,m] with fact[t]. But H[c] may be zero for some; A zeros. Good. But val[i] currently includes contributions from c<l. We subtract conv term. The conv term includes all c in [l,m], including c where E[i]-B[c] maybe negative? We argued nonnegative. If t_min set max(0), and A includes c > E[i]? Could that happen? c in [l,m] with B[c] < B[i] <= E[i], so B[c] < E[i], so E[i]-B[c] >=1. Since c1 < B[ql] <= E[ql] <= E[i], all c <=c1 < E[i]. So no negative. Good.

What about c=0? We don't include H0. B[1]>=1. Good.

Now, early exit if S[0]!='B' or S[-1]!='W'. Is that sufficient for impossible? What if e_b<b for some b? Recurrence handles H_N maybe 0? Example sample2 e3=4? Actually e_N=N, valid. If some prefix has more B than W? e_b<b, condition automatic, H may still nonzero. Strong possible? S1=B,S2N=W enough? Let's test N=2 S=BBWW (B1,B2,W3,W4). e0=0,e1=0,e2=2. Valid b1? e1=0>=1 false. b2 e2=2>e1 valid. H2=(2! - sum none? c=1 H1=0)/(0!)=2. Count matchings: whites 3,4 to blacks1,2. Edges: 3->1,4->2 or 3->2,4->1. Graph path 1->2->3->4. Pair (3,1),(4,2): edges 3->1,4->2. From4->2->3->1 strong? 4->2, 2->3,3->1 yes. Pair (3,2),(4,1): 4->1,1->2->3,3->2? 3->2, yes strong? From3 to1:3->2? no edge 3->2? pair 3->2 yes, 2? path to? 2->3, but to1? 4->1 only from4, can 3 reach4? path 3->4, then 4->1. yes. Both strong? C_k: cut1: W>1 to B1: 4->1 yes; cut2: W>2 to B<=2: 3->1/2 or4->1/2 yes; cut3: W>3 to B<=3: 4->1/2 yes. So 2. Recurrence gives2. Good.

If S1=B,S2N=W but some e_b? Always possible? Maybe not sample2 output0 due e_N=e_{N-1}? But S2N=W ensures e_N>e_{N-1}. Sample2 S=BWWBWBWB: last char B? Wait sample2 length8: B W W B W B W B, last B, early exit 0. Yes. So early exit covers sample2. Are there cases S1=B,S2N=W but answer0? Yes maybe if e_b sequence causes H_N=0 mod or actual 0? Let's find N=3 S=B W B W B W? e=[0,1,2,3] answer3. S=B B W W W B? last B no. S=B B W W B W: e0=0,e1=0,e2=2,e3=3. H1=0,H2=(2!)/(0!)? e2=2,b2 denom fact0, sum none? c1=0 =>2. H3=(6 - H2*fact1)=4. Answer4? likely. Maybe always positive if S1=B,S2N=W? Not necessarily modulo could 0, but actual maybe positive? Let's see condition requires for each b with e_b>=b, at least one high value. There is always at least one permutation? For any nondecreasing e with e0=0,eN=N, is there a permutation with M_b>e_b for b<N? Choose π_i=N at i=1? Then M_b=N>e_b for all b<N. So yes at least one (put N first) if e_b<N for b<N. Since e_b<=N-1 for b<N? If S2N=W, e_{N-1}<=N-1, and e_b<=e_{N-1}<=N-1. So π1=N works. Thus answer >0. Mod could be 0 for large N.

Now, need ensure recurrence with H_b=0 for invalid e_b=e_{b-1} but what about b where e_b<b? H=0. In compressed valid list, we skip. Does recurrence for later valid b sum over previous valid c only; invalid H=0 so okay. But the first closed prefix could be invalid b? If e_b<b, closed prefix impossible (not enough whites), so no. If e_b=e_{b-1}, first closed b impossible as argued. So skip.

Let's prove H_b recurrence for valid b with e_b>e_{b-1} and e_b>=b. We did. Need maybe H_b can be negative mod; use `%MOD`.

Now, potential issue with factorial indices: E[i]-B[j] can be up to N-1? E<=N, B>=1, so <=N-1. fact length N+1 ok. E[i]-B[i] >=0. For b=N, E=N,B=N, denom fact0.

Now, let's think about NTT convolution range and t_min/t_max with compressed valid indices. E[ql] and E[qr] are e values for valid b. E is strictly increasing? Yes for valid list. But if there are invalid b between, E[qr] may be much larger. Mlen = E[qr]-c0 - (E[ql]-c1)+1. This can be up to N. L = B[c1]-B[c0]+1. Need L+Mlen-1 maybe >2N? B range <=N, E range <=N, sum <=2N. Good.

But if c0 is not B[l] but first nonzero, L smaller. t_min uses c1 last nonzero. Good.

Could there be a contribution from c in [l,m] with B[c] < c0? No H=0. c0 first nonzero. Good.

Now, direct cost estimate uses nz*rb. But if B gaps large, direct still nz*rb. NTT estimate uses L+Mlen. Good.

Potential optimization: In NTT, if Mlen == 1 (fact index same for all b? E constant? But valid E strictly increasing, rb maybe1? If rb=1, direct likely. If Mlen small and L large, convolution length ~L. Could direct if nz*rb small. OK.

Now, let's consider using `sys.setrecursionlimit(1_000_000)`. CDQ recursion depth small but number calls many; recursion limit not issue.

Potential issue: Python's default recursion with many calls (thousands) okay. But function call overhead for apply/solve maybe. Could implement iterative CDQ? Not necessary.

Need ensure no global variable lookup overhead in hot functions. Use local bindings in functions: `MOD_local=MOD`, `fact_local=fact`, etc. But nested functions can access closures (slower). We can define functions inside solve and bind locals via default arguments? Or global variables and local aliases inside function.

For performance, implement `apply` as nested function with local variables from outer (nonzero, B, E, val, H, fact, invfact). Closure variable access is slightly slower than local. We can pass as default args? E.g.
```
def apply(l,m,ql,qr, B=B, E=E, val=val, H=H, fact=fact, nonzero=nonzero, MOD=MOD):
```
But nonzero list mutates, default reference okay. This makes them local. Similarly solve.

However, `nonzero` is mutated; default reference fine. `bisect_left` local.

NTT functions global with precomputed rate lists. Use local aliases inside.

Let's write NTT functions carefully.

Precompute max_n and rate_tz:
```
max_n = 1
while max_n < 2*N + 5: max_n <<= 1
rate2 = [0]*30; irate2=[0]*30
for i in range(30):
    rate2[i] = pow(G, (MOD-1) >> (i+2), MOD)
    irate2[i] = pow(rate2[i], MOD-2, MOD)
# precompute tz factors up to max_n//2
rate_tz = [1]*(max_n//2 + 1)
irate_tz = [1]*(max_n//2 + 1)
for s in range(max_n//2):
    x = s + 1
    tz = (x & -x).bit_length() - 1
    rate_tz[s] = rate2[tz]
    irate_tz[s] = irate2[tz]
```
But for s=0, x=1,tz=0. Good. For s up to max_n//2 -1. In ntt, w can be n/2 at ph=h. s max w-1 = n/2-1 <= max_n/2-1. Good.

Could precompute only up to max_n//2. If max_n=1? N>=1, max_n>=4? 2*N+5.

NTT functions:
```
def ntt(a):
    n = len(a)
    h = n.bit_length() - 1
    mod = MOD
    rtz = rate_tz
    for ph in range(1, h+1):
        w = 1 << (ph-1)
        p = 1 << (h-ph)
        shift = h - ph + 1
        now = 1
        for s in range(w):
            base = s << shift
            end = base + p
            # local now
            for i in range(base, end):
                l = a[i]
                r = a[i+p] * now % mod
                x = l + r
                if x >= mod: x -= mod
                y = l - r
                if y < 0: y += mod
                a[i] = x
                a[i+p] = y
            now = now * rtz[s] % mod
```
Potential bug: `rtz[s]` for s where s >= len(rtz)? w max n/2 <= max_n/2, s max n/2-1, len max_n/2+1, ok.

Inverse:
```
def intt(a):
    n=len(a); h=n.bit_length()-1; mod=MOD; itz=irate_tz
    for ph in range(h,0,-1):
        w=1<<(ph-1); p=1<<(h-ph); shift=h-ph+1
        inow=1
        for s in range(w):
            base=s<<shift; end=base+p
            for i in range(base,end):
                l=a[i]; r=a[i+p]
                x=l+r
                if x>=mod: x-=mod
                y=l-r
                if y<0: y+=mod
                a[i]=x
                a[i+p]=y*inow%mod
            inow = inow * itz[s] % mod
    inv_n = pow(n, mod-2, mod)
    for i in range(n):
        a[i] = a[i] * inv_n % mod
```
Could optimize final scaling by combining with pointwise? No.

Need test NTT with small convolution. We can't run, but can reason. ACL rate2 precompute: For i=0, rate2[0]=G^{(MOD-1)/4}. For n=4, ph=2 s=1 now=rate2[0], correct. For n=8, let's check now sequence. ph=3 (w=4,p=1). s=0 now=1; update *rate2[0]=w4. s=1 now=w4; update *rate2[1]=w8? rate2[1]=G^{(MOD-1)/8}=w8. now=w4*w8 = w4^? w8^? w4 = w8^2, product w8^3. s=2 now=w8^3; update *rate2[0]=w4 => w8^5. s=3 now=w8^5; update *rate2[2]=w16? not used. Expected twiddle factors for DFT n=8 at stage ph=3? For s=0,1,2,3 maybe 1, w8, w8^3, w8^5? Yes. Good.

Now, convolution pointwise: After forward, A and F in bit-reversed DFT order. Multiply. Inverse. Good.

Potential issue with negative values in A/F? All mod positive. In inverse, y*inow%mod. Good.

Now, direct/NTT apply with val modulo. In direct i outer, `s` can be negative large. `val[i] = s % mod` yields positive. In NTT, `val[i] = (val[i] - A[idx]) % mod`. A[idx] mod positive. Good.

In leaf solve, `s = val[i]` (mod positive) minus direct sum. Could be negative. `H[i] = (s % mod) * invfact[E[i]-B[i]] % mod`. If H[i]==0, don't append. But if H[i] mod 0, it's zero for future; okay.

Naive for small M: We can use same direct O(M^2) without val? Simpler:
```
H=[0]*(M+1)
for i in range(1,M+1):
    s=fact[E[i]]
    for j in range(1,i):
        hj=H[j]
        if hj: s -= hj*fact[E[i]-B[j]]
    H[i]=s%MOD*invfact[E[i]-B[i]]%MOD
print(H[M])
```
This is O(M^2). For M=3000, 4.5M, okay. Set NAIVE_LIMIT=3000 maybe. But if M=5000, 12.5M maybe okay but CDQ maybe also. Use 2000 to be safe.

Now, let's consider if M=1. Naive prints H1=fact[E1]*invfact[E1-B1]. E1>=1. If S=B...W. For N=1 E1=1,B1=1 =>1. Good.

Now, possible issue: The recurrence H_b = (e_b! - sum_{c<b} H_c (e_b-c)!)/(e_b-b)! uses c original black index, not compressed. We use B[j]. Good.

Let's test sample3 with compressed valid B=[1,2,5,6,8,9], E=[2,3,5,6,8,9]. Naive M=6 would compute as manual. Good.

Now, let's think about proof for final answer to include in notes. Need notes concise but complete.

Potential edge cases:
- N=1, S='BW': e=[0,1], valid B=[1], E=[1], H1=1.
- N=1, S='WB': early 0.
- S starts B ends W but e[b] maybe 0 for b>0. valid skip. Example N=2 BBWW: e=[0,0,2], valid b2 only? b1 e1=0<1 skip, b2 e2=2>0 valid. B=[2],E=[2], H1? compressed i=1 corresponds b=2: H=(fact2)/(fact0)=2. Answer2. Good. Note B[1]=2, E[1]=2. Denom E-B=0.
- If valid list has first b>1, recurrence sum over previous valid none. H for first valid = fact[e_b]/fact[e_b-b]. Is that correct? First closed prefix b with no earlier possible closed. Number injections from b black to e_b white with no earlier closed (automatic because earlier e_c<c or e_c=e_{c-1}? If e_c<c impossible; if e_c=e_{c-1} first closed impossible). Count injections P(e_b,b)=e_b!/(e_b-b)!. Yes.

Now, let's examine CDQ with compressed M where B gaps. Suppose valid B=[2] only M=1 naive. If M large but first valid b large, root apply left maybe no nonzero? H for first valid computed in leaf. Contributions to later use B gap. NTT L maybe from first nonzero B to last. OK.

Now, performance of `bisect` on nonzero list in every apply. Number apply ~M/LEAF ~3000, bisect O(log M) negligible. Slicing c_list O(nz). In NTT, we also iterate c_list to fill A. In direct, create hj/bj lists. Could avoid slicing by using `c_list = nonzero[lo:hi]`; okay.

But note: `nonzero` list is global and grows. In apply, `hi=bisect_right(nonzero,m)`. Since no nonzero >m yet, hi could be len(nonzero). But if due to recursion? Let's verify CDQ order with compressed M. solve(1,M): solve(1,mid) appends up to mid. apply. solve(mid+1,M): inside solve(mid+1,mid2) appends up to mid2. apply within right uses nonzero up to mid2, which includes left of right and also earlier left half (1..mid). But apply(l=mid+1,m=mid2,ql=mid2+1,qr=...) should only use c in [l,m], not earlier. We bisect [l,m], so earlier ignored. Good. nonzero may contain indices >m? In solve(mid+1,M), before solving its left child, no indices >mid solved. After solving left child, indices up to mid2 solved. No >mid2. In deeper, same. So hi <= len. Good.

Now, one subtle bug: In leaf solve, we compute H[i] using direct sum over j in [l,i-1]. But what about contributions from j in [l,i-1] that are invalid/H=0? H[j]=0, skip? We can just loop all j and multiply H[j] (zero). For performance, maybe use nonzero list within leaf? Leaf size 64, simple loop okay. But if many zeros, still 64. Use:
```
for i in range(l,r+1):
    s=val[i]
    for j in range(l,i):
        hj=H[j]
        if hj:
            s -= hj * fact[E[i]-B[j]]
    H[i]=s%MOD*invfact[E[i]-B[i]]%MOD
    if H[i]: nonzero.append(i)
```
Good.

Now, direct apply: We use c_list nonzero in [l,m]. But what if some H[c] became zero modulo but c in nonzero? We only append if nonzero, so no.

Now, constants. Let's choose:
- NAIVE_LIMIT = 2000 (or 3000). If M <= 2000, O(2M) fine.
- LEAF = 64.
- DIRECT_ALWAYS = 4096 maybe.
- COST_FACTOR = 4? Let's define:
```
if direct_cost <= 4096:
    direct
else:
    compute n, ntt_cost = n * (n.bit_length()-1)
    if direct_cost <= ntt_cost * 4:
        direct
    else:
        ntt
```
This may direct length256? direct=16384, n=256,nlog=2048, *4=8192 -> NTT. length128 direct=4096 <=4096 direct. Good.
If nz small, e.g. nz=1, rb=100000, direct_cost=100000. L=1, Mlen~100000, n=131072, nlog=2.2M, *4=8.8M, direct chosen. Good.
If nz=10, rb=100000, direct=1M, L maybe 10? Mlen 100k, n=131072, ntt_cost*4=8.8M, direct chosen (1M simple ops) maybe okay. If many such nodes? Could total direct high? Suppose nonzero sparse 10 per left, right large at many levels? Each pair processed once; direct total pairs where nz*rb <=4*nlog. Could be O(N log N * small). Probably okay.
If dense length512: direct=65536, n=512,nlog=4608,*4=18432 -> NTT. Good.

Maybe factor 4 too low/high. We can set factor 8 to direct length256. Which is faster? Hard. NTT length256 calls 781, each 3 transforms. ACL NTT inner loops: n=256, h=8, total butterflies per transform n/2*h=1024, 3=3072, plus pointwise256. 781*3328=2.6M butterfly/pointwise. Direct length256: 12.8M inner ops. NTT likely faster despite calls. Factor4 NTT. Good.

What about length128: direct=4096, NTT n=128, butterflies per transform512, 3=1536+128=1664 per node, nodes1562 =>2.6M. Direct 6.4M. NTT might be faster too. But direct_cost <=4096 triggers direct. Maybe we should NTT length128 as well? 1562 NTT calls overhead maybe high. Direct 6.4M simple maybe okay. Keep direct.

Leaf internal direct 6.4M. Total direct ~12.8M. NTT from256: total butterflies? Level256: nodes781, n maybe 256? But L+Mlen maybe up to 256? For dense, left B range 128, E range right 128, need255 n256. 3 transforms butterflies 3*781*1024=2.4M. Level512: 3*390*2048=2.4M. Level1024: 3*195*4096=2.4M. Each level ~2.4M, 9 levels =>21.6M butterflies. Plus pointwise sum n per level ~N*9=1.8M. Total ~23M. Direct 13M. Total ~36M operations. Might pass in PyPy if optimized? Maybe borderline but okay.

Can we reduce NTT levels by using larger direct? If direct length512, direct adds 25.6M, NTT levels from1024 (8 levels) ~19M, total44M. Worse. If direct length128 and NTT from128: direct leaf6.4+apply64? If LEAF=32, direct leaf3.2+apply64 3.2=6.4, NTT from128 adds level128 2.4M, total NTT24M, direct6.4 =>30M but more NTT calls (1562 extra). Maybe faster? NTT calls overhead. Hard.

Maybe use LEAF=32 and direct threshold 1024, NTT from128. Total direct 6.4M, NTT 24M. Similar. Fewer direct, more NTT calls. I think LEAF=64 good.

Could optimize NTT by using `while` loops instead of `for`? In Python, `for i in range` is optimized. Use local `a_i = a` etc.

Potential issue: ACL butterfly inner loop uses `a[i+p]` where p maybe large. For ph small, p large, inner loop many. Good.

Could use memoryviews? No.

Now, let's consider alternative using `math`? No.

Need ensure modulo multiplication in NTT: `r = a[i+p] * now % mod`. `now` changes per s. For now=1, could skip mod? But not worth.

In pointwise multiplication, `for i in range(n): fa[i] = fa[i] * fb[i] % mod`. Could use list comprehension? `fa = [x*y%mod for x,y in zip(fa,fb)]` creates new list, maybe faster in CPython but memory. In PyPy, loop maybe okay. We can do:
```
for i, x in enumerate(fa):
    fa[i] = x * fb[i] % mod
```
Maybe slower. Use range.

In final scaling intt, loop. Could combine scaling with query? We only need certain indices, but inverse NTT produces all. Need scale all. Could avoid scaling all by scaling queried values? Inverse NTT without final scaling gives n*original. We could skip final scaling in intt and when querying multiply by inv_n. But intt function used only in convolution; we can make `intt(a, scale=True)`. If we skip scaling, A values are n times conv. Then query term = A[idx] * inv_n % MOD. This saves scaling loop over n? But we still need maybe not all values. We only query rb values (right segment length). rb could be much smaller than n. In CDQ, n ~ segment length, rb ~ segment/2, so scaling all n vs rb ~ factor2. Not huge. But for sparse direct? NTT used when rb large. Could save some. However, inverse butterfly final scaling is O(n); pointwise O(n). We can skip and scale only queried indices. But intt function currently scales. We can modify to not scale, and in apply after intt, `inv_n = pow(n, MOD-2, MOD)` (or precompute inv powers) and term = A[idx]*inv_n%MOD. But A values after inverse unscaled are n*conv mod. Need multiply. This saves n multiplications, adds rb multiplications. Since rb <= n/2 maybe save. Also pow per NTT call; can precompute inv_n for powers of two up to max_n. Let's do this optimization.

Define `intt(a)` without final scaling? But maybe we want a function for convolution that returns scaled? We only use in apply. Let's implement `intt(a)` that does inverse butterfly but does NOT multiply by inv_n. Then in apply:
```
intt(A)
inv_n = inv_pow2[n.bit_length()-1]  # precomputed inv[2^k]
for i in range(ql,qr+1):
    term = A[idx] * inv_n % MOD
    val[i] = (val[i] - term) % MOD
```
But wait inverse butterfly as ACL without scaling yields n * original? Yes. We can precompute `inv_pow2[k] = pow(1<<k, MOD-2, MOD)` or `inv_pow2[k] = inv_pow2[k-1]*inv2%MOD`. Since n power of two.

This saves scaling loop. Good. But if we ever need convolution elsewhere, no. Only apply.

Can we also skip pointwise multiplication for zeros? No.

Modify intt:
```
def intt(a):
    ... inverse butterfly ...
    # no scaling
```
Then in apply, after intt(A), `inv_n = inv_pow2[n.bit_length()-1]`. Query term = A[idx] * inv_n % MOD.

But note: A after pointwise and inverse unscaled is n * convolution modulo. Since all operations mod, yes.

Precompute inv_pow2 up to max_log:
```
inv_pow2=[1]*(max_log+1)
inv2=(MOD+1)//2
for i in range(1,max_log+1): inv_pow2[i]=inv_pow2[i-1]*inv2%MOD
```
max_log = max_n.bit_length()-1.

This also means if we wanted to use A values for multiple queries, multiply each. rb maybe up to n/2. Good.

Can we avoid modulo in query term by `val[i] = (val[i] - A[idx] * inv_n) % MOD`. OK.

Now, in NTT forward, do we need to scale input? No.

Let's test convolution with unscaled inverse: For n=4, inverse without scaling gives 4*original. Query multiply inv4. Good.

Now, precompute rate_tz up to max_n//2. If max_n maybe 1? N>=1, max_n while <2*N+5 => at least 8? For N=1, 2*1+5=7, max_n=8. OK.

Now, direct apply with val not modded? We mod at end. In NTT apply, val[i] should be mod before subtract? It is mod from previous operations. We do `(val[i] - term) % MOD`. Good.

Now, one more optimization: In NTT apply, building F via slice `F[:Mlen] = fact[t_min:t_max+1]` copies references. Then ntt modifies F, so fact list unaffected (ints immutable). Good. But slice creates temporary list length Mlen. Could fill loop to avoid temporary? Slice in C faster. Memory okay. Use slice.

Building A: `A = [0]*n`; for c in c_list: A[B[c]-c0]=H[c]. If c_list large, Python loop. Could build A over full B range by iterating pos and mapping? c_list loop better if nz << L. If dense, nz~L, loop L. OK.

Could use `A[c0:c1+1] = ...`? Need array with zeros and H at positions. Not easy.

Now, potential issue with `fact[t_min:t_max+1]` if t_min=0 and t_max=N? length N+1. Mlen maybe N+1? t_max=E[qr]-c0. E[qr]<=N, c0>=1 => <=N-1. t_min>=0. length <=N. OK.

Now, let's think about correctness of CDQ with NTT when H values in left are modulo. Recurrence linear over mod, okay.

Now, let's consider if `val[i]` can be negative before leaf due to direct apply not modded? We mod at end of direct apply. NTT apply mods. Initial positive. So val always mod positive after each apply. In leaf direct, we compute s=val[i] - sum, then H. We don't update val[i] to s%MOD, but H computed. val[i] not used later for i (since i finalized). For future i in same leaf, val[i] for future not affected by current H (we sum directly). So val[i] for finalized i can remain old; not used. Good.

But in leaf, for i, direct sum over j in [l,i-1] uses H[j] computed. If H[j] zero, skip. Good.

Now, if M is large but many H zero, nonzero list small, apply direct often. Good.

Let's test small cases manually with CDQ logic.
N=2 BWBW: M=2, B=[1,2],E=[1,2]. If NAIVE_LIMIT 2000, naive. H1=1,H2=1.
If CDQ: M=2, LEAF=64, solve(1,2) leaf:
i=1 s=fact1=1, no sum, H1=1*invfact0=1 append1.
i=2 s=fact2=2, j=1 H1 fact[E2-B1=1]=1 =>s=1, H2=1*invfact0=1. output1.
BBWW: M=1 B=[2],E=[2]. H1=fact2*invfact0=2.
Sample2 early 0.
Sample3 M=6 naive.

Now, let's consider a case where M>NAIVE and CDQ. Hard to test but trust.

Potential bug in valid condition: `e[b] > e[b-1]` for b=1 uses e[0]. e[0] is #W before B1. If S1=B, e0=0. If S1 not B early exit. Good. For b=N, e[N]=N, e[N-1] computed. If last char W, e[N-1] <= N-1, so valid if e[N]>=N yes. If there are no W after last B? last char B early exit. Good.

What if S1=B but e0>0? impossible because first char B, w=0 before first B. Good.

Now, let's revisit strong connectivity equivalence to ensure no missing condition about vertex 2N white and vertex1 black. We forced. If S1=B,S2N=W and C_k>0 all k, strong. We counted no closed prefix b=1..N-1. Does C_k>0 all k exactly no closed prefix? We argued hardest cuts before B_{b+1}. Let's formalize:
For any cut k, let b = # black vertices <= k. Then prefix contains first b black and some whites among first e_b? Actually if k is between B_b and B_{b+1}, prefix contains first b black and t whites of group b (0<=t<=e_b-e_{b-1}). C_k counts edges from whites >k to black <=b. The minimum over t in that block occurs at t = e_b-e_{b-1} (just before B_{b+1}), because suffix whites decrease. At that cut, suffix whites are groups >=b+1, black <=b. So C_k>0 for all k iff for all b=1..N-1, there is edge from group >=b+1 to black <=b. Also need cut after B_N? b=N, prefix all black, suffix group N whites. Minimum just before end? Suffix whites after cut; if last W at end, cut before last W suffix one W. Need edge from that W to black <=N. This is included in b=N? Our no closed prefix b=N? For b=N, condition would be edge from group >=N+1? none. Wait earlier we said strong requires C_k>0 for k after B_N. Let's examine with no closed prefix b=1..N-1 only. Does that ensure cut after B_N? Suppose N=2, S=B B W W (BBWW). e1=0, no closed b1 automatic. Strong? We found both matchings strong. Cut after B2 (k=2): prefix black1,2; suffix W3,4. Need edge from W3/4 to black1/2. Since all whites are suffix, all edges go to black1/2, so yes. In general after B_N, all remaining whites are group N, all black are prefix. Any matching has edges from these whites to some black (all black), so C_k>0 as long as there is at least one white after k. The hardest cut is just before the last vertex? If last vertex W, suffix one white, its edge to some black (all black) gives C>0. So no extra condition. The closed prefix b=N is trivially true (all black matched to all white) but not a bad event. Strong requires no bad b=1..N-1. Good.

What about cut before B1 if S1=B: k=0 not considered. If S1 not B impossible. Good.

Now, count matchings with no bad b. Bad b: all whites group >=b+1 matched to black >b, equivalently first b black matched to first e_b white. If e_b<b, impossible. If e_b=e_{b-1}, can first b black matched to first e_b white without first b-1 black matched to first e_{b-1} white? Since e_b=e_{b-1}, the white set same. If first b black all in set, then first b-1 black all in set, so bad b-1. Thus first bad cannot be at b with no new white. Good.

First bad decomposition: For a full matching, let b be smallest bad prefix (1..N). Since b=N always bad? At b=N, first N black matched to first N white (all) yes. So smallest bad exists in 1..N. Strong iff smallest bad is N. Number with smallest bad b: choose injection of first b black to first e_b white with no smaller bad (H_b), then match remaining N-b black to remaining N-b white arbitrarily ((N-b)!). But H_b as defined counts injections with no smaller bad. Does every such injection extend to (N-b)! full matchings with smallest bad b? If injection has no smaller bad and first b black in first e_b white, then bad b occurs. Could a smaller bad occur in full matching due to remaining whites? Smaller bad c<b depends only on first c black and first e_c white. Since injection restricted to first c black is same and no smaller bad in injection, full matching no smaller bad. Remaining matchings don't affect first c black. Good. Extensions: remaining black are b+1..N, remaining whites are all whites except the b chosen. Count (N-b)! regardless of which b chosen? Yes, remaining whites count N-b, black count N-b, any bijection. So count = H_b*(N-b)!.

Total N! = sum_{b=1}^N H_b*(N-b)!. Strong = H_N*(0)! = H_N. This also gives a check: sum H_b*(N-b)! = N!. Could use for testing small.

Now, H_b recurrence: Let I_b = H_b. Total injections from first b black to first e_b white = P(e_b,b)=e_b!/(e_b-b)!. Partition by first bad c<=b. If first bad c<b, then first c black injected to first e_c white with first bad c: H_c ways. The remaining b-c black (c+1..b) can be injected into any of the remaining e_b-c white vertices (since c used in first e_c; all other first e_b whites available) in P(e_b-c, b-c) = (e_b-c)!/(e_b-b)! ways. Thus total = H_b + sum_{c<b} H_c * (e_b-c)!/(e_b-b)!. Multiply: H_b*(e_b-b)! = e_b! - sum H_c (e_b-c)!. Good.

Now, implement. Need ensure modulo division by fact[e_b-b] using invfact. Since e_b-b <=N.

Now, let's think about possible integer overflow? Python no.

Now, code structure:
```
import sys, bisect
MOD=998244353
G=3

def main():
    input=sys.stdin.readline
    N=int(input().strip())
    S=input().strip()
    if S[0]!='B' or S[-1]!='W':
        print(0); return
    e=[0]*(N+1)
    w=0; b=0
    for ch in S:
        if ch=='W': w+=1
        else:
            b+=1
            if b<=N: e[b-1]=w
    e[N]=N
    B=[0]; E=[0]
    for b in range(1,N+1):
        if e[b] >= b and e[b] > e[b-1]:
            B.append(b); E.append(e[b])
    M=len(B)-1
    # fact/invfact
    fact=[1]*(N+1)
    for i in range(1,N+1): fact[i]=fact[i-1]*i%MOD
    invfact=[1]*(N+1)
    invfact[N]=pow(fact[N], MOD-2, MOD)
    for i in range(N,0,-1): invfact[i-1]=invfact[i]*i%MOD
    if M <= 2000:
       H=[0]*(M+1)
       for i in range(1,M+1):
           s=fact[E[i]]
           ei=E[i]
           for j in range(1,i):
               hj=H[j]
               if hj:
                   s -= hj * fact[ei - B[j]]
           H[i]=s%MOD*invfact[E[i]-B[i]]%MOD
       print(H[M]%MOD); return
    # precompute NTT max_n etc
    ...
    val=[0]*(M+1)
    for i in range(1,M+1): val[i]=fact[E[i]]
    H=[0]*(M+1)
    nonzero=[]
    # define ntt/intt using precomputed rate_tz
    # define apply, solve
    solve(1,M)
    print(H[M]%MOD)
if __name__=='__main__': main()
```

Need precompute NTT only if M>NAIVE. But max_n depends on N. Do after naive check to save time for small.

Precompute rate_tz:
```
max_n=1
while max_n < 2*N + 5: max_n <<= 1
max_log=max_n.bit_length()-1
rate2=[0]*30; irate2=[0]*30
for i in range(30):
    rate2[i]=pow(G, (MOD-1) >> (i+2), MOD)
    irate2[i]=pow(rate2[i], MOD-2, MOD)
half=max_n//2
rate_tz=[1]*(half+1)
irate_tz=[1]*(half+1)
for s in range(half):
    x=s+1
    tz=(x & -x).bit_length()-1
    rate_tz[s]=rate2[tz]
    irate_tz[s]=irate2[tz]
inv_pow2=[1]*(max_log+1)
inv2=(MOD+1)//2
for i in range(1,max_log+1): inv_pow2[i]=inv_pow2[i-1]*inv2%MOD
```
But rate2 list length 30; tz can be up to? s up to half-1, x up to half, tz up to log2(half) <= max_log-1. For max_n=524288, half=262144, x max 262144, tz=18. rate2[18] exists. Good.

Could precompute rate2 only up to max_log+1 instead of 30. But 30 fine.

NTT functions need access rate_tz, irate_tz, MOD. Define inside main after precompute to capture locals? If global, rate_tz global. For speed, define inside main and use default args? Let's define inside main:
```
def ntt(a, mod=MOD, rtz=rate_tz):
   ...
```
But `rate_tz` defined in main; default captures. Good. `intt` similarly.

However, nested functions with default args still closure? Defaults are local. Good.

Apply function nested uses ntt/intt, inv_pow2, etc.

Potential issue: Python's recursion with nested functions and many local variables. OK.

Let's write apply with local aliases:
```
def apply(l, m, ql, qr):
    lo = bisect_left(nonzero, l)
    hi = bisect_right(nonzero, m)
    if lo == hi: return
    c_list = nonzero[lo:hi]
    nz = hi - lo
    rb = qr - ql + 1
    direct_cost = nz * rb
    if direct_cost <= DIRECT_ALWAYS:
        hj_list = [H[c] for c in c_list]
        bj_list = [B[c] for c in c_list]
        fact_l = fact; E_l=E; val_l=val
        for i in range(ql, qr+1):
            ei = E_l[i]
            s = val_l[i]
            # loop
            for k in range(nz):
                s -= hj_list[k] * fact_l[ei - bj_list[k]]
            val_l[i] = s % MOD
        return
    c0 = B[c_list[0]]; c1 = B[c_list[-1]]
    L = c1 - c0 + 1
    t_min = E[ql] - c1
    if t_min < 0: t_min = 0
    t_max = E[qr] - c0
    Mlen = t_max - t_min + 1
    need = L + Mlen - 1
    n = 1 << (need - 1).bit_length()
    ntt_cost = n * (n.bit_length() - 1)
    if direct_cost <= ntt_cost * 4:
        # direct as above
        ...
        return
    # NTT
    A = [0] * n
    for c in c_list:
        A[B[c] - c0] = H[c]
    F = [0] * n
    F[:Mlen] = fact[t_min:t_max+1]
    ntt(A); ntt(F)
    for i in range(n):
        A[i] = A[i] * F[i] % MOD
    intt(A)
    inv_n = inv_pow2[n.bit_length()-1]
    for i in range(ql, qr+1):
        idx = E[i] - c0 - t_min
        term = A[idx] * inv_n % MOD
        val[i] = (val[i] - term) % MOD
```
Need `bisect_left`, `bisect_right` local. Import `from bisect import bisect_left, bisect_right`.

In direct branch after cost estimate, we need hj_list/bj_list. We can factor direct code into inner function? Function call overhead. Duplicate code or define local helper? Duplicate okay.

To avoid creating hj_list/bj_list twice, we can create before branch if direct. But for NTT we don't need. In branch direct, create.

Direct loop optimization: If nz==1, special:
```
if nz == 1:
    hj=hj_list[0]; bj=bj_list[0]
    for i in range(ql,qr+1):
        val[i] = (val[i] - hj * fact[E[i]-bj]) % MOD
    return
```
This avoids inner range. If nz small maybe. For general, i outer.

Could also if rb==1:
```
i=ql; ei=E[i]; s=val[i]; for k...; val[i]=s%MOD
```
But general handles.

Maybe use `for hj,bj in zip(hj_list,bj_list):` inside i loop? Zip creates iterator each i, slow. Use index range.

Could store `hj_list` and `bj_list` as local, `nz_local=nz`. In inner loop, `for k in range(nz_local):`. Good.

Potential big int growth in direct: If direct_cost up to maybe ntt_cost*4. ntt_cost can be large (e.g. n=524288, nlog=9.8M, *4=39M). direct_cost could be 39M, val magnitude 39M*1e18=3.9e25 (86 bits, 3 limbs) okay but inner operations 39M maybe too slow. But such direct only if nz*rb <=39M and NTT cost high. Could happen nz=1,rb=200k direct 200k, fine. nz=100,rb=200k direct20M, maybe direct chosen if n large? L maybe 100, Mlen 200k, n=262144, ntt_cost=4.7M,*4=18.8M, direct20M > -> NTT. So direct max maybe ~18M. OK.

But val magnitude if direct 18M updates per val? If rb=1, nz=18M impossible nz<=M=2e5. Max per val nz=2e5, magnitude 2e23. OK.

Now, solve function:
```
def solve(l, r):
    if r - l <= LEAF:  # if r-l < LEAF? Need include size. If r-l <= 64 means size <=65. Use if r-l < LEAF.
        for i in range(l, r+1):
            s = val[i]
            ei = E[i]
            for j in range(l, i):
                hj = H[j]
                if hj:
                    s -= hj * fact[ei - B[j]]
            hi_val = (s % MOD) * invfact[ei - B[i]] % MOD
            H[i] = hi_val
            if hi_val:
                nonzero.append(i)
        return
    mid = (l + r) >> 1
    solve(l, mid)
    apply(l, mid, mid+1, r)
    solve(mid+1, r)
```
Use `if r - l < LEAF` so segment size <=LEAF. If LEAF=64, size 64 when r-l=63. Good.

In leaf, `ei - B[i]` nonnegative. `fact[ei - B[j]]` index positive. Good.

Potential issue: In leaf, val[i] may not include contributions from j in [l,i-1] if those j were computed in same leaf but H[j] zero? We sum. Good.

Now, after solve(1,M), H[M] computed. Print.

Let's think about memory of recursion and lists. OK.

Now, let's verify NTT precompute rate2 formula with max_log. For MOD 998244353 = 119*2^23+1. Primitive root 3. For length 2^ph, wlen = G^{(MOD-1)/2^ph}. In ACL rate2[i] = G^{(MOD-1) >> (i+2)} = G^{(MOD-1)/2^{i+2}}. For ph, s update uses rate2[tz]. The now for s at stage ph should be primitive 2^ph root raised to some odd? For ph=2, s=1 now=rate2[0]=G^{(MOD-1)/4}=w4. Good. For ph=3, s=1 now=rate2[0]=w4, but expected w8? Wait earlier I thought for n=8 stage ph=3, s=1 now should be w8? Let's re-evaluate ACL algorithm.

For n=8, h=3.
ph=1: p=4, w=1, now=1. Pairs distance4.
ph=2: w=2,p=2. s=0 now=1 pairs (0,2),(1,3) within first half? offset0. update now=rate2[0]=w4. s=1 offset=4? shift=h-ph+1=2, offset=4. pairs (4,6),(5,7) with now=w4.
ph=3: w=4,p=1. shift=1. s=0 offset0 now=1 pair (0,1). update now*=rate2[0]=w4. s=1 offset2 now=w4 pair (2,3). update now*=rate2[1]=w8 => now=w4*w8 = w8^2*w8 = w8^3. s=2 offset4 now=w8^3 pair (4,5). update now*=rate2[0]=w4 => w8^5. s=3 offset6 now=w8^5 pair (6,7).
Expected DFT n=8 after previous stages? Let's derive twiddle for final stage combining two 4-point DFTs. For k=0..3, X_k = A_k + w8^k B_k, X_{k+4}=A_k - w8^k B_k. The pairs in final stage: (0,1) for k=0 twiddle1; (2,3) for k=1? But due to bit-reversed order, maybe pair (2,3) corresponds k=1 twiddle w8? Wait our now for s=1 is w4, not w8. Hmm maybe ACL's order different. Let's check ACL rate2 update: For s=1, now after s=0 update is rate2[0] = G^{(MOD-1)/4} = w4. But expected w8? Maybe rate2[0] is actually primitive 4th root, but for n=8 final stage s=1 should be primitive 8th root? Let's look at ACL code more carefully. I may have misremembered rate2 indexing. In ACL, `rate2[(~s & -~s).bit_length() - 1]` where `rate2[i]` maybe `pow(g, (mod-1) >> (i+2), mod)` yes. For s=1, index? ~1=-2, -~1=2, -2 & 2 = 2, bit_length 2, -1 =>1? Wait earlier I computed index for s=1 as 1, not 0! Let's recalc: For s=0: ~0=-1, -~0=1, -1 & 1 =1, bit_length1, index0. For s=1: ~1=-2, -~1=2, -2 & 2 = 2, bit_length2, index1. Ah I made mistake: index = trailing zeros of s+1? s+1=2, tz=1. Yes. For s=2: s+1=3, tz=0 index0. For s=3: s+1=4, tz=2 index2. So update sequence: s=0 now*=rate2[0]=w4. s=1 uses w4, then now*=rate2[1]=w8 => w4*w8 = w8^3. s=2 uses w8^3, then now*=rate2[0]=w4 => w8^5. s=3 uses w8^5. Still s=1 now w4, not w8. But maybe expected twiddle for pair (2,3) is w4 due to bit-reversed ordering? Let's test n=8 forward output order maybe not standard. The algorithm is known correct, so trust.

But my precompute `tz=(x & -x).bit_length()-1` for x=s+1 gives index: s=0 x1 tz0; s=1 x2 tz1; s=2 x3 tz0; s=3 x4 tz2. This matches `(~s & -~s)`? Let's verify Python: -~s = s+1. ~s & (s+1) gives lowest set bit of s+1? For s=1, ~1=-2 (..1110), 2 (10), & =2 yes. For s=2, ~2=-3 (..1101), 3 (11), & =1. Yes. So tz of s+1. Good.

Now, rate2[1]=G^{(MOD-1)/8}=w8. Good.

Let's test n=4 with this: ph=2, s=0 now1, update rate2[0]=w4; s=1 uses w4. Expected final stage pair (2,3) twiddle w4 for n=4, correct. For n=8, final stage pair (2,3) maybe twiddle w4 due to bit-reversed BFT? Let's manually compute ACL n=8 maybe output order [X0,X4,X2,X6,X1,X5,X3,X7]? Twiddles differ. Trust ACL.

Need ensure precompute rate_tz uses tz of s+1, not bit_length of s+1 -1 (which for non-power gives floor log). We use `(x & -x).bit_length()-1`, correct.

Now, inverse irate_tz similarly.

Let's maybe include a small self-test? Not in final code (would slow). But we can reason.

Potential issue: `max_n` while < 2*N+5. For convolution need L+Mlen-1. L<=B[c1]-B[c0]+1 <= N. Mlen<=E[qr]-c0 - (E[ql]-c1)+1 <= N? Let's bound worst: c0=1,c1=N? L=N. t_min=E[ql]-N maybe 0, t_max=E[qr]-1 <=N-1, Mlen<=N. need<=2N-1. 2*N+5 safe. n power of two >=need. max_n maybe 524288 for N=2e5 (2N+5=400005, next pow 524288). Good.

Now, let's consider if M>NAIVE but max_n precompute half=262144, rate_tz length 262145. In ntt for n=524288, ph=19, w=262144, s max262143, rtz[s] exists. Good.

Now, possible performance issue: Precomputing rate_tz loop 262k with bit ops, fine.

Now, let's think about using `sys.setrecursionlimit(1 << 20)`.

Now, let's revisit direct/NTT cost and constants. We can maybe set `DIRECT_ALWAYS = 2048` and factor 2 to use NTT more. But Python NTT calls overhead. Let's choose a balanced set:
- LEAF = 64
- DIRECT_ALWAYS = 4096
- COST_FACTOR = 3 or 4.
I lean factor 4 to favor direct for moderate. But if direct too much, TLE. NTT in Python with ACL maybe not too slow. Let's estimate actual operations: 23M butterflies. Each butterfly: two list reads, one multiply mod, add/sub, two writes, update w? Actually w update per s not per butterfly. Inner loop has `% mod` for r. That's expensive. 23M `%` operations maybe very slow (5-10s). Direct 13M multiplications no mod (big int) plus 13M list accesses, maybe also slow. Need reduce total.

Can we optimize NTT inner loop to avoid `%` for r? `r = a[i+p] * now % mod`. Since a and now < mod, product <1e18, Python `%` needed. Could use `r = (a[i+p] * now) % mod`. No alternative. Direct multiplication no mod but product <1e18, s accumulates big. Direct inner: `s -= hj * fact[...]` no mod, product <1e18, s big. This may be faster than NTT butterfly. So favor direct more. Maybe factor 8 or 16. But direct O(N^2) risk. Let's estimate if factor 16: dense length512 direct? direct65536, ntt_cost4608*16=73728 -> direct. length1024 direct262144, n=1024,nlog10240,*16=163840 -> NTT. So NTT starts1024. Direct total: leaf6.4 + apply128 6.4 + apply256 12.8 + apply512 25.6 =51.2M. NTT levels from1024: 8 levels *2.4M=19.2M butterflies. Total ~70M operations. Factor4 total ~36M. Factor4 likely better. Factor8: direct length256? direct16384, ntt2048*8=16384 direct; length512 NTT. Direct total 38.4M, NTT21.6M =>60M. Factor4 best maybe.

But NTT butterfly operation heavier than direct. If weight NTT 3x direct, factor4 total weighted: direct13M + 23M*3=82M; factor8 direct38M+21.6*3=103M; factor2 direct? NTT from128: direct6.4 + NTT24M*3=78M. Similar. Hard.

Maybe use PyPy where `%` is optimized? Still.

Could reduce NTT transforms by using real convolution? No.

Maybe use a different CDQ: Instead of convolving with fact range for each node, we can precompute NTT of fact segments? Fact is global; for each node F range [t_min,t_max]. Many nodes at same level have disjoint t ranges? E ranges for right halves are disjoint (since E increasing with b). At a given CDQ level, right segments [ql,qr] are disjoint in b, so E ranges are disjoint (strictly increasing, no overlap). Thus F ranges for nodes at same level are disjoint in value domain! Similarly A ranges (B c0..c1) for left segments are disjoint? Left segments disjoint in b, B ranges disjoint. This suggests we could batch all convolutions at a level into one or few large convolutions by placing A and F in global arrays with offsets, maybe reducing overhead and using larger NTTs. But total work same or better due to fewer transforms? If we concatenate independent convolutions with zero padding to avoid cross terms, one large NTT of size sum n_i maybe less than sum n_i log n_i? Actually one NTT of size S where S >= sum n_i + gaps has cost S log S, which can be larger than sum n_i log n_i if many small. But fewer Python function calls. Could pack with gaps to avoid interference: For each convolution need length need_i. If we place A_i at offset, F_i at offset, cross terms may interfere. Need zero gaps of length need_i-1 between. Total size sum (need_i + gap) ~2 sum need_i, maybe larger. Not worth.

Could use "offline convolution with multiple queries" via one NTT of global fact and global H? Since H unknown online, CDQ needed.

Maybe use `numba`? No.

Given task explicitly asks CDQ NTT, likely expected Python can pass with careful constants. We'll proceed.

Let's consider using `array('I')` for NTT arrays to reduce memory and maybe speed? Python array element access converts to Python int, slower. List better.

Could use `list` of int but values small. OK.

Now, let's think about possible bug in ACL inverse without scaling: The inverse butterfly as written uses `inow` update with irate_tz. Does it require scaling by inv_n at end; we skip. Good. But does inverse butterfly assume input in bit-reversed order from forward? Yes.

Let's test n=2: forward ph=1: a0=l+r,a1=l-r. inverse ph=1: l=a0,r=a1; a0=l+r=2orig0, a1=(l-r)*1=2orig1. unscaled. Good.

n=4 we tested. Good.

Now, precompute inv_pow2: For n=1, bit_length-1=0, inv=1. But NTT not called for n=1? Could if need=1. But direct likely. If n=1, ntt h=0 loops none, intt none, inv_n=1. OK.

Now, let's consider if `need = L + Mlen - 1` could be 0? L>=1,Mlen>=1. OK.

Now, direct branch after cost estimate: We need `hj_list` and `bj_list`. If `nz` large and direct chosen due to small rb, creating lists O(nz) okay. If direct chosen due to small nz, okay.

Could optimize direct by using `for c in c_list:` and local `H_l=H; B_l=B; fact_l=fact; E_l=E; val_l=val`. Inside i loop, `for c in c_list: s -= H_l[c] * fact_l[ei - B_l[c]]`. This avoids hj/bj lists but does two list lookups H[c],B[c] per inner. Precomputing hj/bj reduces to two list lookups hj_list[k],bj_list[k] (still two) but no H/B indexing with c. Similar. Precompute cost O(nz). Use hj/bj.

If nz==1 special. If nz==2 maybe unroll? Not necessary.

Direct code:
```
hj_list = [0]*nz; bj_list=[0]*nz
for k,c in enumerate(c_list): hj_list[k]=H[c]; bj_list[k]=B[c]
if nz == 1:
    hj=hj_list[0]; bj=bj_list[0]
    for i in range(ql, qr+1):
        val[i] = (val[i] - hj * fact[E[i]-bj]) % MOD
else:
    for i in range(ql, qr+1):
        ei=E[i]; s=val[i]
        for k in range(nz):
            s -= hj_list[k] * fact[ei - bj_list[k]]
        val[i] = s % MOD
```
In nz==1, modulo each i. Could accumulate? val[i] independent, one update, modulo needed to keep small. OK.

In general, if rb large and nz small, s magnitude small. If nz large and rb small, s magnitude large but mod once.

Now, in leaf direct, we can also special if segment small. Use similar.

Potential issue: In leaf, `val[i]` may be not mod if previous direct apply didn't mod? We mod in direct apply. NTT apply mods. Initial mod. Good.

Now, let's think about using `s %= MOD` vs `s % MOD`. Same.

Now, let's consider if H[i] computed in leaf should be appended to nonzero before or after? After. For future i in same leaf, H[i] used directly, not via nonzero. For apply after leaf, nonzero includes all. Good.

Now, one subtle CDQ issue: In solve(l,r) internal, after solve(l,mid), nonzero includes indices in [l,mid] and also earlier <l. apply uses bisect [l,mid]. Good. Then solve(mid+1,r). Inside solve(mid+1,r), its left child solve(mid+1,mid2) will compute H using val that includes contributions from [l,mid] (applied) and earlier <l (from ancestors). But what about contributions from indices in [mid+1, mid2-1] within same left child? Leaf/internal handles. Good.

Now, let's consider if `val` for indices in right half receives contributions from left half multiple times? CDQ each pair (c,i) processed at exactly one node: the lowest node where c in left, i in right. Ancestor applies? At root, c in left, i in right processed. In deeper, if both in same half, not processed at root? Wait root apply processes all c in left half to all i in right half. If c and i are in different halves at root, pair processed at root. If they are in same half, root doesn't process; deeper processes. Thus each pair once. Good.

Now, let's think about possible modulo negative in NTT A/F. A values H mod positive, F fact positive. NTT operations keep mod positive. Pointwise positive. Inverse positive unscaled (mod). Query term positive. val subtract mod.

Now, let's maybe include a fallback if NTT max_n precompute too large memory? N=2e5, fine.

Now, let's revisit problem statement: "Partition the 2N vertices into N pairs, each consisting of one white vertex and one black vertex. For each pair, add a directed edge from the white vertex to the black vertex." Pairs are unordered? A partition into pairs: each white matched to a black. Counting ways: a bijection from whites to blacks. Our permutation π maps black j to white index? We counted bijections. Pairs (white,black) unordered but each pair has one white one black, so bijection. Good.

Now, let's ensure white indices in our permutation are in order of vertices (1..N whites). We used white index t (1..N) as value. e_b = number of white vertices before B_{b+1}. First e_b white vertices are those before B_{b+1}. Good. Black index b. Matching black b to white value π_b. Bad prefix b if all π_1..π_b <= e_b. Good.

Now, let's consider if two whites have same position? No.

Now, let's maybe derive c_t symmetric not needed.

Now, potential issue with factorial `fact[e_b-c]` where e_b-c could be >N? e_b<=N,c>=1 => <=N-1. OK.

Now, let's think about if `e[b] > e[b-1]` condition for b=N: e[N]=N, e[N-1] maybe N? If last char W, e[N-1] <= N-1, so true. If S[-1]=='W' but there are N whites before last B? impossible because last W after last B, so at least one W after, e[N-1]<=N-1. Good.

Now, let's consider input reading: S may have newline. Use strip. If N maybe line with spaces? OK.

Now, let's think about final answer modulo. H[M] already mod. Print.

Now, let's maybe include comments in code? Fine.

Now, let's examine ACL NTT rate precompute more rigorously. I want to be 100% sure. Let's look up ACL Python convolution code from memory:
```
def butterfly(a):
    n = len(a)
    h = (n - 1).bit_length()
    first = True
    for ph in range(1, h + 1):
        w = 1 << (ph - 1)
        p = 1 << (h - ph)
        now = 1
        for s in range(w):
            offset = s << (h - ph + 1)
            for i in range(p):
                l = a[i + offset]
                r = a[i + offset + p] * now % mod
                a[i + offset] = (l + r) % mod
                a[i + offset + p] = (l - r) % mod
            now = now * rate2[(~s & -~s).bit_length() - 1] % mod
```
Yes.
Inverse:
```
def butterfly_inv(a):
    n = len(a)
    h = (n - 1).bit_length()
    for ph in range(h, 0, -1):
        w = 1 << (ph - 1)
        p = 1 << (h - ph)
        inow = 1
        for s in range(w):
            offset = s << (h - ph + 1)
            for i in range(p):
                l = a[i + offset]
                r = a[i + offset + p]
                a[i + offset] = (l + r) % mod
                a[i + offset + p] = (l - r) * inow % mod
            inow = inow * irate2[(~s & -~s).bit_length() - 1] % mod
    inv_n = pow(n, mod-2, mod)
    for i in range(n): a[i] = a[i] * inv_n % mod
```
Yes.

Our precompute `rate2[i] = pow(G, (MOD-1) >> (i+2), MOD)`. ACL precompute maybe:
```
rate2 = [0]*30
irate2 = [0]*30
prod = 1
iprod = 1
for i in range(30):
    rate2[i] = pow(G, (MOD-1) >> (i+2), MOD)
    irate2[i] = pow(rate2[i], MOD-2, MOD)
```
Yes.

Now, one nuance: ACL's `rate2` update uses `rate2[(~s & -~s).bit_length() - 1]`, but `~s & -~s` in Python for s=0 gives 1, bit_length1 index0. Good. Our precompute `x=s+1; tz=(x & -x).bit_length()-1` equivalent? For s=1, x=2, tz1. For s=2, x=3, tz0. For s=3, x=4, tz2. Yes.

Now, let's consider if `rate_tz` length half+1 but s can equal half? In ntt, w=1<<(ph-1), max ph=h, w=n/2. s in range(w) => max n/2-1. half=max_n//2. If n=max_n, max s=half-1. OK. If n smaller, s<half. Good.

Now, let's think about using `n.bit_length()-1` for inv_pow2 index. If n=1, index0. If n=524288, bit_length=20? 524288=2^19, bit_length=20, index19. max_log=19. inv_pow2 length20. Good.

Now, let's consider if `max_n` while < 2*N+5 yields max_n maybe 1 for N=0, but N>=1. OK.

Now, let's maybe optimize fact/invfact precompute with local mod. Fine.

Now, let's think about possible TLE due to naive limit 2000: If M=2000, O(2M) =2M? Actually sum i ~2M? 2000*2000/2=2M, fine. If M=5000, 12.5M maybe okay but CDQ maybe also. Keep 2000.

Now, let's consider if M is just above 2000, CDQ with NTT precompute and maybe few NTT. OK.

Now, let's think about if all H zero? But H_N positive actual, mod could zero. If H values zero, nonzero list small, CDQ fast. If H[M]=0, print0.

Now, let's maybe include a verification identity for small in notes? Not in code.

Now, potential bug: In valid list, we use `e[b] > e[b-1]`. For b where e[b]>=b but e[b]==e[b-1], H=0. But what if e[b-1] < b-1 (invalid) and e[b]==e[b-1]? Could first bad b be possible? Example e0=0,e1=0,e2=0? e2<2 invalid. If e_b=e_{b-1} but e_b>=b? Since e_{b-1}=e_b>=b > b-1, so previous e_{b-1}>=b > b-1, valid previous? It could be valid if e_{b-1}>e_{b-2}. Then first bad b impossible as argued because white set same. If previous invalid due to e_{b-1}<b-1 cannot have e_b>=b with equality. So skip fine.

Now, let's think about if e[b] can jump by more than 1. Valid. Recurrence handles.

Now, let's consider if `fact[ei - bj]` index in direct/NTT could be negative due to B[j] > E[i]? We argued B[j] < B[i] <= E[i]. Since valid E[i]>=B[i]. For j<i in compressed, B[j] < B[i]. So E[i]-B[j] >= B[i]-B[j] >=1. Good.

Now, let's think about if in NTT t_min = E[ql] - c1. c1 is B[last nonzero in left] < B[ql] <= E[ql], so positive. If c1 maybe 0? B indices start1. OK.

Now, let's consider if `Mlen = t_max - t_min + 1` could be huge and `fact[t_min:t_max+1]` slice length huge. OK.

Now, let's think about possible memory peak in NTT apply: A n list, F n list, plus c_list slice, hj/bj not in NTT. n max 524288. A list pointers 4MB, int objects initially zeros shared. F slice creates Mlen int references (existing fact ints) plus list pointers 4MB. During ntt, A and F elements become new ints. At peak, A and F each have n distinct ints? For A, initially zeros shared, after ntt all distinct. F initially references fact ints, after ntt distinct. So int objects ~2*n*28=29MB, list pointers ~8MB, total ~37MB. Plus fact/invfact etc ~ (N+1)*2*28 + pointers ~14MB? Actually fact list 200k pointers 1.6MB + ints 5.6MB =7.2MB, invfact same, E/B/val/H ~4*7.2=28.8MB. Total maybe 80MB. OK.

But during `F[:Mlen] = fact[t_min:t_max+1]`, the slice temporary list of Mlen pointers ~1.6MB, fine. During ntt, old fact ints still referenced by fact list, F new ints. OK.

Now, let's consider if Python's garbage collection overhead with many NTT arrays. Could disable GC? Lists of ints no cycles, GC not needed. We can `import gc; gc.disable()` to speed. At end no matter. Add `gc.disable()` maybe. But if memory, GC disabled okay. Use `import gc; gc.disable()`.

Now, let's think about using `sys.stdout.write(str(ans))` vs print. Print fine.

Now, let's maybe write code with comments minimal.

Before final, let's do a mental small CDQ test with M=3, LEAF maybe 2 to force internal. Suppose B=[1,2,3],E=[1,2,3]. val=[fact1=1,fact2=2,fact3=6]. solve(1,3), mid2? If LEAF=2, r-l=2 not <2, mid=2. solve(1,2) leaf size2:
i1 H1=1 append1.
i2 s=val2=2 - H1*fact[1]=1 =>1 H2=1 append2.
apply(1,2,3,3): c_list [1,2], nz2, rb1 direct. i3 ei3 s=6 - H1*fact2(2) - H2*fact1(1)=3. val3=3.
solve(3,3) leaf: i3 s=val3=3, no internal, H3=3*invfact0=3. Good.

If apply NTT for this small? direct.

Now, test with B gaps: B=[2,5],E=[2,5]. M=2 leaf. i1 H1=fact2/invfact0=2. i2 s=fact5 - H1*fact[5-2=3]=120-2*6=108, denom fact0 =>108. Recurrence original b=2? e2=5? But b=2 e=5, H2=(120 - H1*6)/1=108. OK.

Now, let's think about if `e[b]` can be less than b for some b but later valid. Example BBWW N=2: valid B=[2],E=[2]. H1=2. Good.

Now, let's consider if S1=B,S2N=W but M maybe 0? N=0 no. For N=1, B=[1]. OK.

Now, let's think about proof of early exit S1=B,S2N=W. If S1=W, vertex1 has no incoming path edge (from 0) and cannot be black target, so no incoming added edge? Actually added edges go from white to black. Vertex1 white, so it is source, not target. No edge into 1. Strong impossible. If S2N=B, vertex2N black, no path outgoing to 2N+1, and as black not source of added edge. No outgoing edge, strong impossible unless N=0. Good.

Now, let's consider if there are multiple edges? Matching pairs distinct, but added edge may duplicate path edge (white before black). Allowed. Our count includes forward edges. Good.

Now, let's think about if final graph strongly connected requires no duplicate? No.

Now, let's maybe include in notes the recurrence derivation and CDQ.

Now, potential issue: The task says "In apply(l,m,ql,qr), add contributions only to valid b in [ql,qr]; use direct loops when nonzero_count*valid_count is small, otherwise NTT-convolve H[c0..c1] with fact[t_min..t_max], where c0/c1 are the first/last nonzero H in [l,m], t_min=max(0, e[first_valid_b]-c0-(c1-c0)), t_max=e[last_valid_b]-c0, and query conv[e[b]-c0-t_min]." Our compressed valid list means [ql,qr] all valid. t_min formula: e[first]-c0-(c1-c0)=e[first]-c1. We use that. Good.

Now, let's think about if `nonzero` list contains indices with H nonzero but B range c0..c1 includes invalid/zero. A zeros. Good.

Now, let's consider if direct cost uses `nz*rb` but if B gaps large, direct still cheap. NTT estimate uses L+Mlen. Good.

Now, let's maybe tune `DIRECT_ALWAYS` to 8192? If direct length256 product16384, cost estimate ntt*4=8192 -> NTT. If DIRECT_ALWAYS 8192, length128 product4096 direct, length256 still NTT. No change. If 16384, length256 direct (since <=16384) before cost? We check direct_cost <= DIRECT_ALWAYS first. That would direct length256. Maybe not. Keep 4096.

Now, let's think about if `ntt_cost = n * (n.bit_length() - 1)` underestimates because 3 transforms. We multiply by factor 4. OK.

Now, let's consider if `need` small but n=1, `n.bit_length()-1=0`, ntt_cost=0. If direct_cost >0 and direct_cost <=0 false, NTT. But if need=1, convolution trivial. Could happen L=1,Mlen=1. Then direct_cost = nz*rb. nz maybe1, rb maybe1, direct_cost1 > DIRECT_ALWAYS? no if 1<=4096 direct. If nz large but L=1 impossible nz<=1. So direct. OK.

Now, let's think about if `Mlen` computed with t_min max(0) but if t_min negative, F starts at 0, query idx = E[i]-c0-t_min. For c > E[i], term should be zero but A includes c and F index negative? If t_min=0, for c>E[i], e[i]-c negative, not represented. But convolution query might include terms with t negative? Let's examine if t_min negative possible. We argued no for valid. But if due to c1 > E[ql] (could if left nonzero B > E[ql]? But B[c1] < B[ql] <= E[ql], so no). So safe. If it happened, setting t_min=0 would include fact[0] for negative? Query idx maybe not align. But not possible.

Now, let's think about if E[ql] - c1 could be 0? If B[ql]=E[ql] and c1=B[ql]-1, t_min=1. If c1 could equal E[ql]? c1 < B[ql] <=E, so no. So t_min>=1. But max0 fine.

Now, let's consider if `fact[t_min:t_max+1]` with t_min maybe >t_max? If E[qr] < c0? But c0 < B[ql] <=E[ql]<=E[qr], so t_max>=E[qr]-c0 >=1. Mlen positive.

Now, let's think about possible off-by-one in e array scan. For each B, after increment b, set e[b-1]=w (W before this B). For last B, e[N-1]=W before last B. e[N]=N. Good. If S has B at end, e[N-1]=N? But early exit if last B. If not early, last W, e[N-1]<=N-1.

Now, let's consider if S length maybe includes spaces? strip.

Now, let's maybe include `if N == 1` special? Not needed.

Now, let's think about if modulo 998244353 is prime, invfact ok.

Now, let's consider if H recurrence can produce negative s before mod; Python `%` positive. Good.

Now, let's maybe write final code now in mind.

Potential performance improvements in NTT:
- Use local variable `a_i = a`? Already local parameter.
- In inner loop, `ap = a[i+p]` maybe. Code:
```
for i in range(base, end):
    l = a[i]
    r = a[i+p] * now % mod
    x = l + r
    if x >= mod: x -= mod
    y = l - r
    if y < 0: y += mod
    a[i] = x
    a[i+p] = y
```
This is okay.
- `now = now * rtz[s] % mod` after inner. If rtz[s]==1, could skip? Many s have rate2[0] not 1. Not worth.

Inverse inner:
```
for i in range(base, end):
    l = a[i]
    r = a[i+p]
    x = l + r
    if x >= mod: x -= mod
    y = l - r
    if y < 0: y += mod
    a[i] = x
    a[i+p] = y * inow % mod
```
If inow==1, still mod. Could special? Not.

Pointwise multiplication:
```
for i in range(n):
    A[i] = A[i] * F[i] % mod
```
Could use local `A_i=A; F_i=F; mod=MOD`.

Now, let's think about if we can free F after pointwise before intt to save memory? `F = None` after pointwise. But intt only uses A. We can `del F` before intt to reduce peak? Peak during pointwise has A,F. After pointwise, F not needed. `del F` may free list and ints (but ints maybe shared? F ints distinct). This reduces memory before intt (A only). Good. Add `del F`.

Also after NTT apply, `del A` maybe not needed, function returns.

In direct branch, `hj_list,bj_list` local freed after.

Now, let's consider if `nonzero` list slicing `c_list = nonzero[lo:hi]` creates list. For NTT, we need iterate. For direct, we need. Could avoid slice by using `c_list = nonzero[lo:hi]` anyway. OK.

Now, let's think about if `bisect_right(nonzero, m)` each apply O(log len). Good.

Now, let's consider if `nonzero` list is mutated while iterating? apply not iterating nonzero directly except slice. solve leaf appends. No concurrent.

Now, let's maybe include `from bisect import bisect_left, bisect_right` at top.

Now, let's think about if recursion solve uses `LEAF` constant from outer. Good.

Now, let's consider if M large but N small? M<=N. OK.

Now, let's maybe test sample3 with naive code mentally done.

Now, let's think about possible issue with `e[b] > e[b-1]` for b where e[b-1] not set? e[0] set if first B. If S1=B, e0=0. If N=1, loop b=1 uses e[0]. Good.

Now, let's consider if input S may have lowercase? No.

Now, let's think about if answer for sample2 early exit 0. Good.

Now, let's maybe derive an alternative O(N log N) using generating functions to see if we can simplify code. Suppose we define polynomial P(x)=sum_{valid b} H_b x^{?}. Recurrence: H_b fact[e_b-b] + sum_{c<b} H_c fact[e_b-c] = fact[e_b]. If e_b-c = (e_b-b)+(b-c). Let d_b=e_b-b. Then H_b fact[d_b] + sum_{c<b} H_c fact[d_b + (b-c)] = fact[e_b]. This is like convolution over b with kernel fact[d_b + b-c], where d_b varies with b. Not Toeplitz. CDQ needed.

Could use online convolution with arbitrary d_b via divide and conquer. OK.

Now, let's think about if we can use `fact` as a polynomial and evaluate at points? No.

Now, let's ensure final code is self-contained and reads stdin.

Now, let's consider if PyPy recursion with nested functions and large local variables might hit recursion stack? Depth ~18, fine.

Now, let's think about if `gc.disable()` could cause memory not freed for NTT arrays? Reference counting frees immediately in CPython; in PyPy, GC disabled may prevent collection of cyclic? Lists no cycles, but PyPy uses GC for all objects; disabling GC may cause memory not reclaimed? In PyPy, `gc.disable()` disables automatic GC, but reference counting? PyPy doesn't have refcount, so memory may not be freed until GC, causing high memory if many NTT arrays allocated. AtCoder PyPy, disabling GC can be dangerous for memory. Maybe don't disable GC, or disable only CPython? We can avoid `gc.disable()` to be safe. Or call `gc.collect()` after large NTT? Not. Since many allocations, GC overhead maybe but memory. I think leave GC enabled. AtCoder PyPy GC handles. Disabling may cause memory blow. So no gc.disable.

Now, let's think about if using `del F` helps PyPy? It may not free immediately but marks. OK.

Now, let's consider if we should use `sys.setrecursionlimit(1_000_000)`.

Now, let's maybe write code with `main()` and call.

Potential bug: In `apply`, after NTT, `intt(A)` unscaled. But `A` values are n*conv mod. We multiply by inv_n per query. However, `intt` inverse butterfly as ACL without scaling also assumes input length n and produces n*original in normal order. We tested. Good.

Now, let's think about if `inv_pow2` precomputed up to max_log, but n maybe 1<<k where k<=max_log. Good.

Now, let's consider if `max_n` while < 2*N+5, but convolution need maybe L+Mlen-1 where L=N, Mlen=N+1? Could Mlen be N+1? t_min=0, t_max=N? t_max=E[qr]-c0. E[qr]<=N, c0>=1 => <=N-1. So Mlen<=N. need<=2N-1. 2N+5 safe. If c0=0 (not), t_max=N, Mlen=N+1, need=2N, still 2N+5 safe.

Now, let's think about if `fact` length N+1, slice t_max+1 <=N? If t_max=N-1, ok. If t_max=N (c0=0), slice N+1 ok. fact length N+1 indices 0..N. Good.

Now, let's consider if `E[i]-B[i]` can be N? E<=N,B>=1 => <=N-1. invfact length ok.

Now, let's maybe include type hints? Not needed.

Now, let's think about possible integer sign in `y = l - r; if y < 0: y += mod`. Since l,r in [0,mod), y in (-mod,mod), one add enough. x in [0,2mod), one subtract enough. Good.

In inverse, `y * inow % mod`, y in [0,mod), inow in [0,mod). Good.

Now, let's consider if in forward, `r = a[i+p] * now % mod`, now in [0,mod). Good.

Now, let's think about if `rate_tz[s]` maybe 0? No.

Now, let's maybe precompute `rate_tz` and `irate_tz` as lists of ints. For s where tz index maybe >29? max_log<=19, no.

Now, let's consider if `pow(rate2[i], MOD-2, MOD)` for i up to 29 where rate2[i]=1 for large i, pow(1)=1. OK. Could compute irate2 as pow(G, MOD-1 - ((MOD-1) >> (i+2)), MOD) but fine.

Now, let's think about if `G=3` primitive root for 998244353. Yes.

Now, let's maybe include a small `if M == 0: print(0); return` after valid list. Though shouldn't.

Now, let's consider if `S[0]` and `S[-1]` access safe for N>=1 length2. Yes.

Now, let's think about if there are trailing spaces in S, strip removes. If S could be empty? No.

Now, let's maybe write final code.

But before final, let's explore if there is a hidden issue with counting partitions: Pairs are unordered, but matching white to black is a bijection. If two pairs swapped? Partition into pairs: set of N unordered pairs. A bijection from whites to blacks uniquely defines set of pairs. No overcount. Good.

Now, let's think about if added edge from white to black where white=black? Impossible colors differ. Good.

Now, let's consider if path edges plus added edges can have cycles. Strong condition as derived.

Now, let's maybe prove C_k>0 all k iff no bad prefix. For cut just before B_{b+1}, C = # edges from whites after B_b (groups >=b+1) to black <=b. Bad prefix b means no such edge. If bad, C=0 at that cut. If no bad, C>0 at those cuts. For cuts inside block, C >= C at end? As cut moves forward through whites group b, suffix loses whites, so C can only decrease. At start of block (after B_b), suffix includes group b whites plus later; C may be larger. At end (before B_{b+1}), suffix excludes group b, minimal. So if end >0, all in block >0. Cuts after black? The block includes after B_b before B_{b+1}; cut immediately after B_b has prefix black b, suffix all whites after B_b (including group b). C >= end. Good. Cuts before B1 none. After B_N, suffix group N whites; as cut moves through group N, C decreases, minimal before last vertex. If last vertex W, its edge to some black (all black) gives C>=1. So no extra. Good.

Now, let's think about if last vertex W but its matched black could be itself? No black. Edge to some black, so C before last vertex =1. Good.

Now, let's consider if S2N=W but group N has multiple whites; minimal cut before last W suffix one white, edge to black, C=1. Good.

Now, let's think about if S1=B but first block group0 has whites before B2? Cut after B1 before first group0 white: prefix black1, suffix all whites. C counts edges from all whites to black1. If no edge to black1, bad b=1 (e1=#W before B2). If e1=0 (no group0), bad impossible? Cut after B1: suffix all whites, black1. There is at least one edge from some white to black1? Since black1 must be matched to some white, yes. So C>0 automatic. Our bad b=1 with e1=0: first 1 black matched to first 0 white impossible, so no bad. Good.

Now, let's think about if e1>0, bad b=1 means black1 matched to a white before B2 (group0). Then cut after B1? If black1 matched to group0 white (which is after B1? group0 whites are between B1 and B2, so >B1), edge from white >B1 to black1 <=B1, so C after B1 would be >0. Wait bad b=1: all whites group >=1 (after B1) matched to black >1. Equivalent black1 matched to group0 white (before B2 but after B1). Then cut after B1: suffix includes group0 whites and later; black1 prefix. Edge from group0 white to black1 crosses, so C>0. But bad prefix b=1 corresponds to cut before B2 (after all group0 whites), not after B1. At cut before B2, group0 whites are in prefix, so no edge from suffix to black1 if black1 matched to group0. C=0. Strong fails because cannot cross from after B2 to before B2. Good.

Now, let's think about if first bad b=N? Strong. H_N counts bijections with no bad b<N. Good.

Now, let's consider if H_N can be computed by total N! - sum_{b<N} H_b*(N-b)!; but we compute directly.

Now, let's maybe include in notes that H_0 not used.

Now, potential issue: In recurrence, sum c<b over original b, but if c invalid H_c=0. In compressed, sum over previous valid. Good.

Now, let's think about if `e[b]` is nondecreasing but valid E strictly increasing. In NTT, E[ql]..E[qr] strictly increasing, so t range length maybe less than rb? If E jumps, Mlen can be >rb. OK.

Now, let's consider if B gaps cause L large but nz small. Direct cost model handles. But in NTT branch, if L large and Mlen large, n large. If nz small but direct_cost > threshold due to rb huge, cost model may choose direct if direct_cost <=4*nlog. Example nz=100, rb=200000, direct=20M, L maybe 100 (if nonzero close), Mlen 200k, n=262144, nlog=4.7M, *4=18.8M, NTT chosen (20M>18.8). NTT cost 3 transforms of 262k ~ maybe 14M butterflies, direct 20M simple. Which is faster? Maybe direct. Factor 5 would direct. Hard. We can set factor 6? Then dense length512: ntt_cost4608*6=27648, direct65536 NTT. length1024: 10240*6=61440, direct262144 NTT. So NTT starts1024 still. Factor6 direct for nz100 case. Direct total maybe increases for sparse. I think factor4 okay.

Maybe use a more accurate NTT cost: `ntt_cost = 3 * n * (n.bit_length()-1) // 2 + n` (butterflies + pointwise). Then compare direct_cost <= ntt_cost * C where C maybe 2 (direct op cheaper). Let's define:
```
ntt_cost = (3 * n * (n.bit_length()-1) // 2) + n
if direct_cost <= ntt_cost * 2: direct
```
For dense length512: n=512, log9, ntt_cost=3*512*9/2+512=6912+512=7424, *2=14848, direct65536 -> NTT. length256: n=256,log8, ntt_cost=3072+256=3328,*2=6656, direct16384 -> NTT. length128: n=128,log7, ntt_cost=1344+128=1472,*2=2944, direct4096 -> NTT (if no DIRECT_ALWAYS). But we have DIRECT_ALWAYS 4096 direct. So length128 direct. For nz100,rb200k: n=262144,log18, ntt_cost=3*262144*18/2+262144=7,077,888+262,144=7,340,032,*2=14.68M, direct20M -> NTT. If factor3 ->22M direct. Which is better? Unknown.

Maybe keep simple `ntt_cost = n * (n.bit_length()-1)` and factor4. Equivalent to above factor? nlog*4 vs (1.5nlog+n)*2 ~3nlog+2n, similar. OK.

Now, let's think about if direct branch with `s` big and no mod until end could be slower due to big int growth. If nz*rb large, s per i grows with nz. If nz=1000, product 1e18*1000=1e21 (70 bits), 3 limbs, okay. If nz=200000, rb=1, s 2e23 (78 bits), 3 limbs. Python big int multiplication of 30-bit*30-bit -> 60-bit, addition to 80-bit, okay. Mod at end of 200k terms? For rb=1, one mod of 80-bit, fine. Direct inner no mod, good.

Now, let's consider if in direct i outer, `s` is Python int, `hj_list[k] * fact[...]` creates new int each time. OK.

Now, let's maybe optimize direct by using `for hj, bj in zip(hj_list, bj_list):` might be slower due to tuple unpack. Use index.

Could store combined list of tuples? `pairs = list(zip(hj_list,bj_list))`, inner `for hj,bj in pairs:` tuple unpack maybe okay but creates tuples. Not.

Now, let's think about if `fact` list access with `ei - bj_list[k]` where bj_list[k] maybe large. OK.

Now, let's consider if we can precompute for each c a list of fact[E[i]-B[c]] for i in right? That's what convolution does. Direct no.

Now, let's think about if NTT branch query loop `for i in range(ql, qr+1): idx = E[i] - c0 - t_min; term = A[idx] * inv_n % MOD; val[i] = (val[i] - term) % MOD`. Since E[i] increasing, idx increasing. Could use local variables. OK.

Now, let's consider if `A[idx]` might be negative? No.

Now, let's think about if `val[i] - term` negative, `%` positive. OK.

Now, let's maybe include `MOD` local in apply: `mod = MOD`.

Now, let's write code carefully.

Pseudo final:
```
import sys
from bisect import bisect_left, bisect_right

MOD=998244353
G=3

def main():
    input=sys.stdin.readline
    N=int(input().strip())
    S=input().strip()
    if S[0] != 'B' or S[-1] != 'W':
        print(0); return
    e=[0]*(N+1)
    w=0; b=0
    for ch in S:
        if ch=='W':
            w+=1
        else:
            b+=1
            if b<=N:
                e[b-1]=w
    e[N]=N
    B=[0]; E=[0]
    for b in range(1,N+1):
        if e[b] >= b and e[b] > e[b-1]:
            B.append(b); E.append(e[b])
    M=len(B)-1
    if M==0:
        print(0); return
    fact=[1]*(N+1)
    for i in range(1,N+1): fact[i]=fact[i-1]*i%MOD
    invfact=[1]*(N+1)
    invfact[N]=pow(fact[N], MOD-2, MOD)
    for i in range(N,0,-1): invfact[i-1]=invfact[i]*i%MOD
    if M <= 2000:
        H=[0]*(M+1)
        for i in range(1,M+1):
            ei=E[i]
            s=fact[ei]
            for j in range(1,i):
                hj=H[j]
                if hj:
                    s -= hj * fact[ei - B[j]]
            H[i]=s%MOD*invfact[ei - B[i]]%MOD
        print(H[M]%MOD); return
    # NTT precompute
    max_n=1
    while max_n < 2*N + 5:
        max_n <<= 1
    max_log=max_n.bit_length()-1
    rate2=[0]*30; irate2=[0]*30
    for i in range(30):
        rate2[i]=pow(G, (MOD-1) >> (i+2), MOD)
        irate2[i]=pow(rate2[i], MOD-2, MOD)
    half=max_n//2
    rate_tz=[1]*(half+1)
    irate_tz=[1]*(half+1)
    for s in range(half):
        x=s+1
        tz=(x & -x).bit_length()-1
        rate_tz[s]=rate2[tz]
        irate_tz[s]=irate2[tz]
    inv_pow2=[1]*(max_log+1)
    inv2=(MOD+1)//2
    for i in range(1,max_log+1):
        inv_pow2[i]=inv_pow2[i-1]*inv2%MOD

    def ntt(a):
        n=len(a)
        h=n.bit_length()-1
        mod=MOD
        rtz=rate_tz
        for ph in range(1,h+1):
            w=1<<(ph-1)
            p=1<<(h-ph)
            shift=h-ph+1
            now=1
            for s in range(w):
                base=s<<shift
                end=base+p
                for i in range(base,end):
                    l=a[i]
                    r=a[i+p]*now%mod
                    x=l+r
                    if x>=mod: x-=mod
                    y=l-r
                    if y<0: y+=mod
                    a[i]=x
                    a[i+p]=y
                now=now*rtz[s]%mod

    def intt(a):
        n=len(a)
        h=n.bit_length()-1
        mod=MOD
        itz=irate_tz
        for ph in range(h,0,-1):
            w=1<<(ph-1)
            p=1<<(h-ph)
            shift=h-ph+1
            inow=1
            for s in range(w):
                base=s<<shift
                end=base+p
                for i in range(base,end):
                    l=a[i]
                    r=a[i+p]
                    x=l+r
                    if x>=mod: x-=mod
                    y=l-r
                    if y<0: y+=mod
                    a[i]=x
                    a[i+p]=y*inow%mod
                inow=inow*itz[s]%mod
        # no scaling

    val=[0]*(M+1)
    for i in range(1,M+1): val[i]=fact[E[i]]
    H=[0]*(M+1)
    nonzero=[]
    LEAF=64
    DIRECT_ALWAYS=4096
    COST_FACTOR=4

    def apply(l,m,ql,qr):
        lo=bisect_left(nonzero,l)
        hi=bisect_right(nonzero,m)
        if lo==hi: return
        c_list=nonzero[lo:hi]
        nz=hi-lo
        rb=qr-ql+1
        direct_cost=nz*rb
        mod=MOD
        if direct_cost <= DIRECT_ALWAYS:
            hj_list=[0]*nz; bj_list=[0]*nz
            for k,c in enumerate(c_list):
                hj_list[k]=H[c]; bj_list[k]=B[c]
            if nz==1:
                hj=hj_list[0]; bj=bj_list[0]
                for i in range(ql,qr+1):
                    val[i]=(val[i]-hj*fact[E[i]-bj])%mod
            else:
                for i in range(ql,qr+1):
                    ei=E[i]
                    s=val[i]
                    for k in range(nz):
                        s -= hj_list[k]*fact[ei-bj_list[k]]
                    val[i]=s%mod
            return
        c0=B[c_list[0]]; c1=B[c_list[-1]]
        L=c1-c0+1
        t_min=E[ql]-c1
        if t_min<0: t_min=0
        t_max=E[qr]-c0
        Mlen=t_max-t_min+1
        need=L+Mlen-1
        n=1 << (need-1).bit_length()
        ntt_cost=n*(n.bit_length()-1)
        if direct_cost <= ntt_cost * COST_FACTOR:
            hj_list=[0]*nz; bj_list=[0]*nz
            for k,c in enumerate(c_list):
                hj_list[k]=H[c]; bj_list[k]=B[c]
            if nz==1:
                hj=hj_list[0]; bj=bj_list[0]
                for i in range(ql,qr+1):
                    val[i]=(val[i]-hj*fact[E[i]-bj])%mod
            else:
                for i in range(ql,qr+1):
                    ei=E[i]
                    s=val[i]
                    for k in range(nz):
                        s -= hj_list[k]*fact[ei-bj_list[k]]
                    val[i]=s%mod
            return
        A=[0]*n
        for c in c_list:
            A[B[c]-c0]=H[c]
        F=[0]*n
        F[:Mlen]=fact[t_min:t_max+1]
        ntt(A)
        ntt(F)
        for i in range(n):
            A[i]=A[i]*F[i]%mod
        del F
        intt(A)
        inv_n=inv_pow2[n.bit_length()-1]
        for i in range(ql,qr+1):
            idx=E[i]-c0-t_min
            term=A[idx]*inv_n%mod
            val[i]=(val[i]-term)%mod
        # del A

    sys.setrecursionlimit(1_000_000)
    def solve(l,r):
        if r-l < LEAF:
            for i in range(l,r+1):
                ei=E[i]
                s=val[i]
                for j in range(l,i):
                    hj=H[j]
                    if hj:
                        s -= hj*fact[ei-B[j]]
                hi_val=s%MOD*invfact[ei-B[i]]%MOD
                H[i]=hi_val
                if hi_val:
                    nonzero.append(i)
            return
        mid=(l+r)>>1
        solve(l,mid)
        apply(l,mid,mid+1,r)
        solve(mid+1,r)

    solve(1,M)
    print(H[M]%MOD)

if __name__=='__main__': main()
```

Need check `sys.setrecursionlimit` called after defining solve? It can be before. Fine.

Potential issue: In `apply`, `fact` and `E`, `B`, `val`, `H` are closure variables. In direct branch, `fact` lookup closure each inner. Could bind local `fact_l=fact; E_l=E; B_l=B; val_l=val; H_l=H` at start of apply for speed. Let's do:
```
fact_l=fact; E_l=E; B_l=B; val_l=val; H_l=H
```
Then use. But `val_l` is list, modifications reflect. `H_l` for reading. In NTT branch, `A[B_l[c]-c0]=H_l[c]`. Good. `nonzero` closure. `bisect_left` global.

In solve leaf, bind locals too:
```
fact_l=fact; E_l=E; B_l=B; val_l=val; H_l=H; invfact_l=invfact; mod=MOD
```
But H_l assignment `H_l[i]=hi_val`. nonzero append.

Let's adjust for speed.

Apply with local bindings:
```
def apply(l,m,ql,qr):
    lo=bisect_left(nonzero,l); hi=bisect_right(nonzero,m)
    if lo==hi: return
    c_list=nonzero[lo:hi]
    nz=hi-lo; rb=qr-ql+1
    direct_cost=nz*rb
    mod=MOD
    fact_l=fact; E_l=E; B_l=B; val_l=val; H_l=H
    if direct_cost <= DIRECT_ALWAYS:
       ... use fact_l,E_l,val_l
       return
    c0=B_l[c_list[0]]; c1=B_l[c_list[-1]]
    ...
    if direct...:
       ...
    A=[0]*n
    for c in c_list: A[B_l[c]-c0]=H_l[c]
    F[:Mlen]=fact_l[t_min:t_max+1]
    ...
    for i in range(ql,qr+1):
       idx=E_l[i]-c0-t_min
       term=A[idx]*inv_n%mod
       val_l[i]=(val_l[i]-term)%mod
```
Good.

In solve leaf:
```
def solve(l,r):
    if r-l < LEAF:
        fact_l=fact; E_l=E; B_l=B; val_l=val; H_l=H; invfact_l=invfact; mod=MOD
        for i in range(l,r+1):
            ei=E_l[i]
            s=val_l[i]
            for j in range(l,i):
                hj=H_l[j]
                if hj:
                    s -= hj*fact_l[ei-B_l[j]]
            hi_val=s%mod*invfact_l[ei-B_l[i]]%mod
            H_l[i]=hi_val
            if hi_val: nonzero.append(i)
        return
    mid=(l+r)>>1
    solve(l,mid)
    apply(l,mid,mid+1,r)
    solve(mid+1,r)
```
Good.

Potential issue: `nonzero` is a list; in solve leaf, `nonzero.append` closure. OK.

Now, let's think about if `DIRECT_ALWAYS` and `COST_FACTOR` should be local constants in apply. They are closure. Could bind default args:
```
def apply(l,m,ql,qr, DIRECT_ALWAYS=4096, COST_FACTOR=4):
```
Not necessary.

Now, let's consider if `ntt` and `intt` use `rate_tz` closure. They are nested, closure lookup for rtz assigned local at start. Good.

Now, let's think about if `rate_tz` length half+1 but `ntt` for n smaller uses s up to n/2-1, ok. For n=1, h=0, loops none, no rtz access.

Now, let's consider if `intt` for n=1, h=0, loops none, no scaling. A[0] remains pointwise product (which is n*conv? n=1, ok). inv_n=1.

Now, let's think about if `need-1` bit_length when need=0? no.

Now, let's consider if `Mlen` could be 0 if t_max<t_min due to t_min max0? Not possible. But to be safe, if Mlen<=0 return? Not needed.

Now, let's think about if `fact_l[t_min:t_max+1]` length maybe less than Mlen if t_max+1 > N+1? t_max<=N, ok. If t_min<0 set0.

Now, let's consider if `A` length n but L maybe >n? n>=L+Mlen-1 >=L, ok. F Mlen<=n.

Now, let's think about if direct branch after cost estimate duplicates code. Could define inner `do_direct()` but function call overhead per apply maybe okay but duplicate fine.

Now, let's consider if `c_list` slice for large nz (e.g. 100k) in NTT branch memory ~0.8MB pointers, ok. In direct branch with large nz and rb small, slice ok. Could avoid slice by using `nonzero[lo:hi]` anyway.

Now, let's think about if `nonzero` list can contain indices not finalized? In apply after solve left, yes finalized. In solve leaf, we append as we go; but apply not called concurrently. Good.

Now, let's consider if H values can be negative? We store mod positive.

Now, let's maybe test with a random small N by brute mentally? Could write but not run. Let's do N=2 all valid strings S1=B,S4=W:
- BWBW: e=[0,1,2], answer1.
- BBWW: e=[0,0,2], answer2.
- BWWB? last B no.
- BWBW only? N=2 length4 with 2B2W startB endW: BWBW, BBWW. Also B W W B no, B B W W yes. What about B W B W yes. So answers 1,2. Let's brute BBWW we did 2. Good.

N=3 startB endW strings count? Could test one: S=B W B W B W (alternating) e=[0,1,2,3], answer3. Brute permutations π of 3 with M1>1 (π1≠1), M2>2 (3 in first2). Count: π1=2, π2=3, π3=1; π1=3, π2=1/2 =>3. Good.
S=B B W W B W: e0=0,e1=0,e2=2,e3=3. Conditions: b1 e1=0 auto; b2 e2=2: M2>2 => 3 in first2. Count permutations with 3 in first2: 2*2=4. Recurrence H2? valid b2 e2=2 H2=2!/(0!)=2? Wait compressed B=[2,3],E=[2,3]. H1 (b=2)=2. H2 (b=3): fact3=6 - H1*fact[3-2=1]=6-2=4. Answer4. Good.
S=B W W B B W: e=[0,2,2,3], answer2 (we did). Conditions: b1 e1=2: M1>2 => π1=3. b2 e2=2: M2>2 auto. Count π1=3, remaining 2! =2. Recurrence H1=2, H2 (b=3): fact3 - H1*fact[1]=6-2=4? Wait compressed valid B=[1,3]? e1=2 valid b1, e2=2 invalid, e3=3 valid b3. B=[1,3], E=[2,3]. H1=2. H2 for b=3: fact3 - H1*fact[3-1=2]=6-2*2=2. Answer2. Good. I earlier used H2=2? yes.
S=B B W B W W? length6: B1,B2,W3,B4,W5,W6. e0=0,e1=0,e2=1,e3=3. Conditions: b1 auto, b2 e2=1 auto (M2>=2>1), b3? N-1=2 only, so all permutations? StartB endW, answer 6? Recurrence valid b2? e2=1<2 invalid; b3 e3=3 valid B=[3],E=[3], H1=6. Answer6. Is every matching strong? S: B B W B W W. Need C_k>0 all k. Cut before B2 (k=1): prefix B1, suffix all W and B2? Added edges from W>1 to B1. Black1 must be matched to some W (all W >1), so yes. Cut before B3 (k=3? B3 pos4, prefix B1,B2,W3): need edge from W>3 (W5,W6) to B1/B2. Is it possible all W5,W6 matched to B3? There are 2 whites, 1 black B3, impossible, so at least one to B1/B2. Cut before B4 (k=4? B4 pos5? Wait N=3, B3 pos4, B4? no N=3, last B pos4? String B B W B W W: B at1,2,4. B3 pos4. Cut before B3 (k=3) as above. Cut after B3 (k=4): prefix all B, suffix W5,W6, edges to any B, yes. So all 6. Good.

Now, let's think about if answer can be N! when e_b<b for all b<N. That means before each B_{b+1}, fewer W than B, i.e. path has more B than W in every prefix before last? Then any matching strong? Probably. Recurrence first valid b=N, H_N=N!. Good.

Now, let's consider if modulo 998244353 and N up to 2e5, fact[N] nonzero (MOD > N). Good.

Now, let's think about if CDQ with NTT computes H in mod, but recurrence involves division by fact[e_b-b] which is invertible since e_b-b < MOD. Good.

Now, potential issue: In NTT convolution, we convolve A (length L) with F (length Mlen). A indices correspond to c-c0. F indices t-t_min. The convolution index for query is e[b]-c0-t_min. Let's verify with an example: c0=2,c1=5,L=4. t_min=e_first-5. For b with e=10, c=3 (i=1), t=7. F index t-t_min=7-(e_first-5). Suppose e_first=6, t_min=1, F index6. Conv index i + Fidx =1+6=7. Query idx=e-c0-t_min=10-2-1=7. Good.

Now, let's think about if F range should start at t_min = e_first - c1, but for c=c1 and b=first, t=t_min. F index0. Conv index for c=c1 (i=L-1) and F0 is L-1. Query idx for first b: e_first-c0-t_min = e_first-c0-(e_first-c1)=c1-c0=L-1. Good.

Now, let's consider if there are c in [c0,c1] with H=0, A zero. OK.

Now, let's think about if `t_max = E[qr] - c0`. For last b and c=c0, t=t_max. F index Mlen-1. Conv index for c0 (i=0) + Mlen-1 = Mlen-1. Query idx last: E[qr]-c0-t_min = t_max-t_min = Mlen-1. Good.

Now, let's consider if E[qr]-c0 could be less than t_min? No.

Now, let's think about if `need = L + Mlen - 1` and n power of two. If need=1, n=1. `1 << (need-1).bit_length()`: for need=1, (0).bit_length()=0, 1<<0=1. Good.

Now, let's consider if `max_n` precompute half maybe 0 if max_n=1, but N>=1 max_n>=8. OK.

Now, let's think about if `rate2` precompute 30 enough for max_n maybe if N=2e5 max_log19. Yes. If constraints larger? 2e5. 30 safe for 2^29.

Now, let's consider if `pow(G, (MOD-1) >> (i+2), MOD)` for i=29 shift31, MOD-1 ~2^23*119, shift31=0, pow=1. OK.

Now, let's think about if `irate2[i]=pow(rate2[i], MOD-2, MOD)` for rate2=1 gives1. OK.

Now, let's maybe optimize precompute rate_tz: Instead of loop half with bit ops, could compute recursively, but half 262k fine.

Now, let's consider if `bisect_left` and `bisect_right` are global functions; in apply, lookup global each call. Could bind `bl=bisect_left; br=bisect_right` in outer and use closure. Do:
```
bl=bisect_left; br=bisect_right
...
lo=bl(nonzero,l); hi=br(nonzero,m)
```
Good.

Now, let's think about if `nonzero` list is empty in apply, lo=hi=0. OK.

Now, let's consider if in solve leaf, for i, inner loop j from l to i-1. If l large and H[j] zero, skip. Could use nonzero list within leaf to reduce, but leaf size64, fine.

Now, let's think about if `val[i]` for i in leaf may not include contributions from c in [l,i-1] that are nonzero but H[c] computed earlier in leaf. We sum. Good.

Now, let's consider if `val[i]` for i in leaf may include contributions from c in [l,i-1] already via apply? No, apply only from left halves to right halves, not within same leaf. Ancestor apply could have processed c in [l,i-1] if they were in different halves at ancestor? But if both in same leaf, they are in same segment at all ancestors down to leaf, so no ancestor apply processed them. If c<l, ancestor applies processed. Good.

Now, let's think about if `val[i]` for i in right half after apply includes contributions from left half [l,mid]. But what about contributions from c in [l,mid] to i in [mid+1,r] where c H zero? A zeros. Good.

Now, let's consider if `H` values in left half are final modulo. Yes.

Now, let's think about if there is any need to mod `val[i]` after direct leaf internal? We compute H but leave val[i] old. Not used. For future i in same leaf, val[i] old (without internal contributions) but we sum internal directly. Good. For future right segments, contributions from this leaf are added via apply using H, not val. So val for finalized indices irrelevant.

Now, let's consider if in apply direct, we update val[i] for i in right half. These i not finalized yet. Good.

Now, let's think about if multiple applies update same val[i] (from different left segments at different levels). Yes, val accumulates. We mod after each apply. Good.

Now, let's consider if direct apply with `s=val[i]` and then `val[i]=s%mod` loses contributions from previous applies? No, s starts with current val (modded), subtract new contributions, mod. Good.

Now, let's think about if NTT apply with `val[i]=(val[i]-term)%mod` similar.

Now, potential issue: In NTT apply, `A` after intt unscaled contains n*conv mod. We multiply by inv_n per query. But `A[idx]` might be large? mod. Good.

Now, let's consider if we should scale `A` in place before query to avoid multiplying each query? If rb close to n, scaling all n maybe similar. We choose per query. If rb large (dense), rb ~ n/2, scaling per query half of n, saves half. Good.

Now, let's think about if `inv_n` multiplication per query can be combined with subtraction: `val[i] = (val[i] - A[idx] * inv_n) % mod`. OK.

Now, let's consider if `A[idx]` for idx beyond len? Shouldn't. Could assert in debug, not final.

Now, let's think about if `E_l[i] - c0 - t_min` could be negative for first i? It equals L-1 >=0. For later larger. Max <= L+Mlen-2 < n. Good.

Now, let's consider if `t_min` max0 but if actual min t >0, F starts earlier with fact indices not needed. Convolution includes extra terms? If t_min smaller than actual min, query idx shifts, but F has extra low indices. For a given b,c, t=e[b]-c. If t_min < actual min, F index t-t_min positive. Convolution query still sums correct terms because A and F align. Extra F indices don't affect query except through convolution indices? They could cause cross terms where i+j=idx with j corresponding to t' not equal e[b]-c? Let's verify: For query idx = e[b]-c0-t_min. Sum over i (c-c0) and j (t'-t_min) with i+j=idx => t' = t_min + j = t_min + idx - i = e[b]-c0 - (c-c0) = e[b]-c. So only correct t'. Extra F indices just not used for that idx. Good.

Now, let's think about if t_max larger than actual max, extra high F indices not used. OK.

Now, let's consider if `L` based on c0/c1 first/last nonzero, but there may be c in [l,m] with H nonzero outside? No c0/c1 cover all nonzero in range. Good.

Now, let's think about if `nonzero` list sorted by compressed index, which is increasing B. Yes, appended in increasing i. B increasing. Good.

Now, let's consider if `bisect_right(nonzero,m)` where m compressed index. Good.

Now, let's think about if M maybe 2001, CDQ precompute NTT max_n based on N maybe 2001, small. OK.

Now, let's consider if fact/invfact precompute before naive check? We need fact for naive. Could compute after M check? Need fact for naive and CDQ. OK.

Now, let's think about if early exit before fact saves time. Good.

Now, let's consider if S[0]!='B' or S[-1]!='W' but N=0? N>=1.

Now, let's maybe include comments in code for clarity but not too many.

Now, let's think about possible issue with Python's `pow` for invfact[N] when N=0? N>=1. OK.

Now, let's consider if `fact[N]` maybe 0 mod if N>=MOD, but N=2e5<MOD. OK.

Now, let's think about if the problem expects modulo 998244353, yes.

Now, let's maybe explore if there is a simpler DP with O(N log N) using Fenwick and factorial transforms. Suppose we maintain polynomial P(x)=sum_{c processed} H_c x^{B_c}. Query at x? Need sum H_c fact[E_i-B_c]. fact[k] = k! = integral? Could represent fact[k] as coefficient of something. fact[k] = ∫? Not.

Could use generating function F(z)=sum fact[k] z^k. Then sum H_c fact[E_i-B_c] is coefficient? It's like convolution of sequence H_c at positions B_c with fact. Online convolution. CDQ is standard.

Now, let's think about if we can use "online convolution" with relaxed NTT in O(N log N) by maintaining blocks and updating? There is algorithm for online convolution where you process in blocks, rebuild NTT for previous blocks. Complexity O(N log^2 N) or O(N log N) with sqrt. CDQ fine.

Now, let's consider if Python solution might TLE; maybe we can further optimize by using a block online convolution instead of CDQ? Let's evaluate.

Block size B~700. Process i from1..M. Maintain list of previous H. For each block of i, precompute convolution of all previous H? Standard: For each block, contributions from previous blocks can be computed by convolving block of H with fact? But H in previous blocks known. Could for each block, build A of all previous H? That would be O(N^2/B * log N). Not better. CDQ is better.

Maybe use "divide and conquer with direct for small" is intended.

Now, let's think about if we can reduce NTT transforms by reusing NTT of F for nodes at same level? F ranges disjoint but fact is same global sequence. If we precompute NTT of fact segments? For each node, F is a contiguous segment of fact. NTT of a segment depends on segment and length. Could precompute for all possible segments? Too many. But at a given level, F segments are disjoint and same length? Right segments at same level have same b-length, but E ranges lengths may vary due to jumps. Not uniform.

Now, let's consider if using `fact` slice and NTT each time is heavy. Could use direct for nodes where Mlen small. Cost model.

Now, let's think about if `ntt` function with ACL butterfly has a bug for inverse without scaling due to missing `first` flag? ACL forward has `first = True` maybe for optimization? I recall ACL Python code has `first = True` and in butterfly, if first: ...? Let's check memory: AtCoder Library Python `butterfly` maybe:
```
def butterfly(a):
    n = len(a)
    h = (n - 1).bit_length()
    first = True
    for ph in range(1, h + 1):
        w = 1 << (ph - 1)
        p = 1 << (h - ph)
        now = 1
        for s in range(w):
            offset = s << (h - ph + 1)
            for i in range(p):
                l = a[i + offset]
                r = a[i + offset + p] * now % mod
                a[i + offset] = (l + r) % mod
                a[i + offset + p] = (l - r) % mod
            now = now * rate2[(~s & -~s).bit_length() - 1] % mod
```
No first? Maybe `first` used in `butterfly_inv`? Not sure. The algorithm as written should work.

Let's test n=8 manually partially to ensure twiddle sequence. We can compare with standard DFT bit-reversed output. Use small mod? Hard. But ACL is trusted.

One nuance: In forward, after each s, `now = now * rate2[(~s & -~s).bit_length() - 1] % mod`. For s=0, index0. For s=1, index1. For s=2, index0. For s=3, index2. This yields now sequence: 1, w4, w4*w8=w8^3, w8^3*w4=w8^5. For n=8 final stage, expected twiddles for pairs in bit-reversed order maybe [1, w4, w8^3, w8^5]? Let's derive standard DIT with bit-reversed input? The output order maybe [X0,X4,X2,X6,X1,X5,X3,X7]. Final stage combines 4-point DFTs of even/odd? Twiddles could be [1, w4, w8^3, w8^5]. Seems plausible. Good.

Now, let's think about if `rate2` precompute should be `pow(G, (MOD-1) >> (i+2), MOD)` or `pow(G, (MOD-1) // (1 << (i+2)), MOD)`. Same.

Now, let's consider if `irate2` should be inverse of rate2, yes.

Now, let's think about if in inverse, `inow` update uses `irate2[tz]` after block. Sequence inverse. Good.

Now, let's maybe include a small convolution test in notes? Not code.

Now, potential issue: In `intt`, we skip final scaling, but the inverse butterfly as ACL without scaling produces n*original only if the forward butterfly did not scale. Yes.

Now, let's think about if pointwise multiplication should be done after forward transforms in bit-reversed order. Yes.

Now, let's consider if `A` and `F` lengths n power of two. We pad zeros. Good.

Now, let's think about if `F[:Mlen] = fact_l[t_min:t_max+1]` when Mlen=0? no. If Mlen=n, slice length n, ok. If Mlen<n, rest zeros.

Now, let's consider if `A` fill with H[c] where H[c] maybe 0? c_list nonzero, so nonzero. But if H[c] mod 0 not in list. Good.

Now, let's think about if `c_list` could be large and `A` fill loop O(nz). If nz ~ L dense, O(L). OK.

Now, let's consider if direct branch after cost estimate uses `hj_list` and `bj_list` creation O(nz). If direct_cost large but direct chosen due to cost factor, nz maybe large, rb small. O(nz) okay. If nz huge 200k, rb=1, direct_cost200k, ntt_cost maybe? n maybe large, direct likely chosen? Creating list 200k, inner 200k, okay. But if many such applies? Each pair processed once; total O(N log N)? If rb=1 at many nodes? A pair with i in right of size1? CDQ leaves size64, apply right size maybe64 not1 except small M. So not many.

Now, let's think about if `DIRECT_ALWAYS=4096` and `COST_FACTOR=4` could choose direct for a node with nz=200k, rb=1? direct_cost200k, n maybe? c0..c1 range maybe N, Mlen maybe E[qr]-c0 ~N, n=524288, ntt_cost=9.8M, *4=39M, direct chosen. 200k direct ops, fine. NTT would be heavy. Good.

Now, let's consider if direct for nz=1000, rb=1000 (1M), n maybe 2048? ntt_cost=2048*11=22528,*4=90k, NTT chosen. Good.

Now, let's think about if `ntt_cost` should include 3 transforms; factor4 maybe enough. OK.

Now, let's consider if we should set `LEAF=32` to reduce direct leaf cost and allow NTT for length64? But NTT calls more. LEAF=64 okay.

Now, let's think about if M just above 2000, CDQ with LEAF64, NTT maybe for nodes length256. Precompute max_n maybe 4096. OK.

Now, let's consider if there is a risk of Python recursion + nested functions causing slow due to closure variable lookups in hot solve/apply. We bind locals. Good.

Now, let's think about if `nonzero` list append in solve leaf could be optimized by local `append=nonzero.append`. Do:
```
append=nonzero.append
...
if hi_val: append(i)
```
Good.

In apply, `c_list=nonzero[lo:hi]` uses nonzero. OK.

Now, let's consider if `val_l[i] = s % mod` in direct branch where s may be negative large. `%` expensive but once per i. OK.

Now, let's think about if in leaf, `s` may be negative large due to internal sum up to 64 terms, small. OK.

Now, let's consider if `fact_l[ei - B_l[j]]` index maybe 0? For j<i, B[j]<B[i]<=ei, so ei-B[j]>=1. But if B[i]=ei and B[j]=ei? impossible B[j]<B[i]. So >=1. fact[0] not used in sum except maybe if c=0 not. OK.

Now, let's think about if `invfact_l[ei - B_l[i]]` index 0 possible. Yes.

Now, let's consider if `e[b]` can be equal to b-1? invalid. Good.

Now, let's maybe include a fallback for if `M` is large but `N` small? M<=N.

Now, let's think about if the problem's modulo 998244353 supports NTT length up to 2^23. Our max_n 2^19, fine.

Now, let's consider if `rate_tz` precompute uses `rate2[tz]` where tz maybe 29 if half large, but half 262k tz18. OK.

Now, let's think about if `pow` in precompute 30 times okay.

Now, let's consider if we can avoid `irate2` pow by `irate2[i]=pow(G, MOD-1 - ((MOD-1) >> (i+2)), MOD)`, but fine.

Now, let's think about if `MOD-1 >> (i+2)` for i=29 is 0, rate2=1, irate2=1. OK.

Now, let's maybe write final code with proper indentation.

Before final, let's revisit the plan/notes requirement. Final answer must have sections:
### CODE
```python
...
```
### NOTES
<bullets, no markdown headings, no bold-only lines, no ALLCAPS lines>
### NEXT
<bullet list or none>
### STATUS
solved or continue

We need output exactly these sections. The code inside fenced block. Notes as bullets starting with '- **Topic:** ...'? The instruction says Organise it as '- **Topic:** ...' bullets. But also says Do NOT use markdown headings (#, ##), bold-only lines, or ALLCAPS: lines anywhere inside this section. A bullet with bold topic is okay? It says use '- **Topic:** ...' bullets. That includes bold but not bold-only line. Should be fine. Avoid ALLCAPS. Keep under 800 words.

Need ensure notes don't contain lines starting with # or all caps. Use bullets.

Now, let's think if final code should include comments? OK.

Let's maybe refine code for speed:
- In `ntt`, `for ph in range(1, h + 1):` and inner `for s in range(w):`. The variable `p` maybe 1 for large ph, inner loop one. OK.
- Use `while` for inner? For loops in Python are okay. But `for i in range(base, end):` creates range object. OK.
- In `ntt`, `a[i+p]` computed twice? Once for r, once for assignment. Could store `j=i+p`. But then assignment `a[j]`. Code:
```
j = i + p
l = a[i]
r = a[j] * now % mod
...
a[i]=x; a[j]=y
```
This avoids addition twice? Maybe. Use:
```
for i in range(base, end):
    j = i + p
    l = a[i]
    r = a[j] * now % mod
    ...
```
Similarly inverse. Slight overhead variable j. OK.

- In pointwise, use local `A_i=A; F_i=F; mod=MOD; for i in range(n): A_i[i]=A_i[i]*F_i[i]%mod`.

- In query, local `A_l=A; E_l=E_l; val_l=val_l; inv=inv_n; c0t=c0+t_min?` idx = E_l[i] - (c0+t_min). Precompute `base_idx = c0 + t_min`. Then `idx = E_l[i] - base_idx`. Good.

- In direct, precompute `bj_list` and `hj_list`. Inner `for k in range(nz): s -= hj_list[k] * fact_l[ei - bj_list[k]]`. Could store `fact_l` local. OK.

- In direct nz==1, use local `val_l`, `E_l`, `fact_l`.

Now, let's consider if `val_l[i] = (val_l[i] - hj * fact_l[E_l[i] - bj]) % mod` in nz==1 does modulo each i. If rb large, okay. Could accumulate? Each i independent one update, need mod to keep small. OK.

Now, let's think about if in direct general, `s` can be negative and large; `s % mod` at end. Good.

Now, let's consider if in leaf, inner loop `for j in range(l, i):` and `if hj:` branch. If many nonzero, branch overhead. Could omit if and multiply zero? Multiplication by zero still cost. Branch maybe good if sparse. In dense, branch always true, overhead. Leaf size64, negligible.

Now, let's think about if in apply direct, c_list all nonzero by definition, no branch.

Now, let's consider if `nonzero` list can be used in leaf to sum only nonzero j in [l,i). Since leaf size64, not needed. But if H sparse, branch skip.

Now, let's think about if `M` large and H dense, nonzero list length M. Bisect O(log M). OK.

Now, let's consider if `c_list = nonzero[lo:hi]` for dense large creates list of 100k ints. In NTT branch, we then iterate to fill A. Could avoid slice by using `for k in range(lo,hi): c=nonzero[k]`. But for direct, we need hj/bj lists. For NTT, we can fill A directly from nonzero range without c_list, and get c0=nonzero[lo], c1=nonzero[hi-1]. This avoids slice memory/time. Let's optimize:
```
c0 = B[nonzero[lo]]; c1 = B[nonzero[hi-1]]
nz = hi-lo
...
if direct:
   hj_list=[0]*nz; bj_list=[0]*nz
   for k in range(nz):
       c=nonzero[lo+k]; hj_list[k]=H[c]; bj_list[k]=B[c]
...
if NTT:
   A=[0]*n
   for k in range(nz):
       c=nonzero[lo+k]; A[B[c]-c0]=H[c]
```
This avoids c_list slice. Good. Use `nonzero_l=nonzero` local. Let's adjust apply.

Apply:
```
def apply(l,m,ql,qr):
    lo=bl(nonzero,l); hi=br(nonzero,m)
    if lo==hi: return
    nz=hi-lo; rb=qr-ql+1
    direct_cost=nz*rb
    mod=MOD; fact_l=fact; E_l=E; B_l=B; val_l=val; H_l=H; nz_list=nonzero
    if direct_cost <= DIRECT_ALWAYS:
        hj_list=[0]*nz; bj_list=[0]*nz
        for k in range(nz):
            c=nz_list[lo+k]
            hj_list[k]=H_l[c]; bj_list[k]=B_l[c]
        ...
        return
    c0=B_l[nz_list[lo]]; c1=B_l[nz_list[hi-1]]
    ...
    if direct_cost <= ntt_cost*COST_FACTOR:
        ... same direct ...
        return
    A=[0]*n
    for k in range(nz):
        c=nz_list[lo+k]
        A[B_l[c]-c0]=H_l[c]
    ...
```
Good.

In direct branch after cost estimate, we duplicate hj_list creation. Could define a local inline function? Duplicate okay. To avoid duplicate, we could first decide `use_direct = direct_cost <= DIRECT_ALWAYS or ...` but need c0/c1 for ntt_cost. We can compute c0/c1 before direct? c0/c1 cheap. Then decide. Then if direct, create lists. Let's structure:
```
c0 = B_l[nz_list[lo]]; c1 = B_l[nz_list[hi-1]]
if direct_cost <= DIRECT_ALWAYS:
    use_direct=True
else:
    compute L,t_min,t_max,Mlen,n,ntt_cost
    use_direct = direct_cost <= ntt_cost*COST_FACTOR
if use_direct:
    create lists and direct
    return
# else use precomputed L,t_min,t_max,Mlen,n for NTT
```
This avoids duplicate and computes c0/c1 always. Good. Need if direct_cost <= DIRECT_ALWAYS, we don't need L etc but c0/c1 already. OK.

Apply revised:
```
def apply(...):
    lo=...; hi=...
    if lo==hi: return
    nz=hi-lo; rb=qr-ql+1
    direct_cost=nz*rb
    mod=MOD; fact_l=fact; E_l=E; B_l=B; val_l=val; H_l=H; nz_list=nonzero
    c0=B_l[nz_list[lo]]; c1=B_l[nz_list[hi-1]]
    if direct_cost <= DIRECT_ALWAYS:
        use_direct=True
    else:
        L=c1-c0+1
        t_min=E_l[ql]-c1
        if t_min<0: t_min=0
        t_max=E_l[qr]-c0
        Mlen=t_max-t_min+1
        need=L+Mlen-1
        n=1 << (need-1).bit_length()
        ntt_cost=n*(n.bit_length()-1)
        use_direct = direct_cost <= ntt_cost * COST_FACTOR
    if use_direct:
        hj_list=[0]*nz; bj_list=[0]*nz
        for k in range(nz):
            c=nz_list[lo+k]
            hj_list[k]=H_l[c]; bj_list[k]=B_l[c]
        if nz==1: ...
        else: ...
        return
    # NTT uses L,t_min,t_max,Mlen,n defined in else. But if direct_cost<=DIRECT_ALWAYS, use_direct true, so not needed.
    A=[0]*n
    for k in range(nz): ...
```
Python scoping: variables L,t_min,t_max,Mlen,n defined in else; if use_direct false, else executed, so defined. OK.

Now, in direct branch, if nz==1, we can avoid hj_list/bj_list? We already created. Could special before creating? If nz==1, get c directly. But direct branch may be called with nz==1 often. Optimize:
```
if nz == 1:
    c=nz_list[lo]; hj=H_l[c]; bj=B_l[c]
    for i ...
    return
# else create lists
```
This avoids list allocation for nz1. Do this in direct branch. For nz>1 create lists.

Direct branch:
```
if nz == 1:
    c=nz_list[lo]; hj=H_l[c]; bj=B_l[c]
    for i in range(ql,qr+1):
        val_l[i] = (val_l[i] - hj * fact_l[E_l[i] - bj]) % mod
    return
hj_list=[0]*nz; bj_list=[0]*nz
for k in range(nz): ...
for i ...
```
Good.

In NTT branch, if nz==1 but direct not chosen? Could happen if direct_cost large and ntt_cost small? nz=1, direct_cost=rb. ntt_cost maybe small if L=1,Mlen small, but then direct_cost likely <=? If rb huge and Mlen huge, ntt_cost large, direct chosen. If rb huge and Mlen small? Mlen small means E range small, but rb huge with E strictly increasing? E range at least rb-1 (since valid E strictly increasing integers), so Mlen >= rb-1? Actually E[qr]-E[ql] >= rb-1. t_max-t_min = E[qr]-c0 - (E[ql]-c1) = (E[qr]-E[ql]) + (c1-c0) >= rb-1 +0. So Mlen>=rb. n>=Mlen>=rb. ntt_cost ~ rb log rb, direct_cost=rb. direct_cost <=4*ntt_cost, direct chosen. So nz1 always direct. Good.

Now, in NTT branch, nz>1.

Now, let's think about if `E_l[qr]-E_l[ql]` can be less than rb-1? E strictly increasing integer, so at least rb-1. Good.

Now, let's consider if `Mlen` can be much larger than rb due to E jumps and B gaps. OK.

Now, let's think about if `L` can be much larger than nz due to B gaps. OK.

Now, let's consider if `ntt_cost` with n maybe 1 and direct_cost >? If n=1, ntt_cost=0, use_direct false if direct_cost>0 and not <=DIRECT_ALWAYS? But if n=1, L=Mlen=1, nz<=1, rb<=1? Since Mlen>=rb, rb=1, direct_cost=nz<=1 <=DIRECT_ALWAYS, so direct. OK.

Now, let's think about if `COST_FACTOR` should be an integer. Yes.

Now, let's consider if `ntt_cost` zero for n=1 and direct_cost > DIRECT_ALWAYS impossible. OK.

Now, let's maybe set `DIRECT_ALWAYS=8192`? With nz1 direct anyway. For dense length256 direct_cost16384, if DIRECT_ALWAYS8192, cost estimate ntt_cost2048*4=8192, use_direct false (16384>8192) NTT. If DIRECT_ALWAYS16384, direct. We choose 4096, NTT. OK.

Now, let's think about if NTT for length256 many calls maybe overhead. Could set COST_FACTOR=8 to direct length256. Which is better? Let's approximate Python NTT length256: 3 transforms * (n/2*log=1024) =3072 butterflies. Each butterfly has one mod mult, add/sub. Pointwise256. Total ~3328 heavy ops per node. 781 nodes =>2.6M heavy. Direct length256: 12.8M simple ops. Heavy op maybe 3-5x simple, so NTT ~8-13M simple equivalent, comparable. NTT also function overhead 781*3=2343 calls, small. I think NTT okay. Keep factor4.

Now, let's consider if length128 direct: 6.4M simple. NTT length128: nodes1562, butterflies per transform512, 3=1536+128=1664 heavy =>2.6M heavy, maybe comparable. But direct avoids 4686 NTT calls. Keep direct.

Now, let's think about if leaf64 direct internal 6.4M simple. Could NTT for leaf? No.

Total heavy NTT from length256 up: levels 256,512,1024,2048,4096,8192,16384,32768,65536,131076? M=200k, max segment 262144? CDQ root segment M maybe 200k, n for root convolution maybe up to 524288? Levels: segment size S (right+left) from 256 to 262144. Number nodes M/S. n for convolution ~S? For root S=262144? M=200k, root left100k right100k, L~100k, Mlen~100k, need200k, n=262144. So levels n=256,512,1024,2048,4096,8192,16384,32768,65536,131072,262144 (11 levels). Sum heavy ~3*N/2*log per level? For n=262144, nodes1, butterflies per transform 262144/2*18=2.36M, 3=7.08M. Sum over levels maybe 30M butterflies. OK.

Now, let's think about if Python can do 30M butterflies. Each butterfly inner loop has `%` mod. 30M `%` might be too slow (maybe 10s+). But many levels smaller. Could be borderline. Need maybe further optimize NTT.

Can we use a faster NTT implementation with precomputed roots per stage and no bit-reversal? ACL already. Could use `pow` roots per stage? ACL precomputes rate_tz. Good.

Could use `numba` no.

Could use `pypy` JIT optimizes loops. AtCoder PyPy may handle 30M loops in ~2-3s? Maybe. Python CPython maybe 10s. The environment unknown. We need provide best pure Python.

Maybe we can reduce NTT levels by using a larger leaf/direct threshold if direct simple ops are faster in PyPy. PyPy JIT can optimize direct loops well, NTT loops with modulo maybe also. Hard.

Let's consider using `DIRECT_ALWAYS=16384` and `COST_FACTOR=8` to direct length256, NTT from512. Direct total 38M simple, NTT heavy 27M? Actually remove level256 heavy 2.4M, add direct12.8M. If heavy 3x simple, direct worse. If heavy 5x, similar. Keep NTT.

Maybe use `COST_FACTOR=2` to NTT length128 too? Direct total 6.4M, heavy add level128 2.6M. If heavy 3x, 7.8M vs direct6.4M, maybe NTT slightly worse. Keep direct.

Now, let's think about if we can use a hybrid: for NTT length <=512, use direct? That's current (length256 NTT, length128 direct). Maybe length256 direct? Not sure. We can set `DIRECT_ALWAYS=16384` to direct length256. Let's evaluate memory/time. 12.8M direct inner operations. In PyPy, 12.8M simple loops maybe ~0.5-1s? NTT 2.6M heavy maybe ~1s. Similar. Direct simpler and fewer NTT calls. Maybe set DIRECT_ALWAYS=16384. Then length256 direct, NTT from512. Total direct: leaf6.4 + apply128 6.4 + apply256 12.8 =25.6M. NTT levels from512: 10 levels, heavy ~27M? Wait remove level256 2.4M, heavy ~27.6M? Previously 30M, remove 2.4 =>27.6. Direct +12.8. If heavy 3x, weighted 25.6+82.8=108 vs previous 12.8+90=103. Similar. If heavy 2x, previous 38.4, new70.8, previous better. So keep NTT length256.

Now, let's think about if we can optimize NTT by using `mod` local and avoiding `%` for r when now==1. In many butterflies now=1? For each stage, s=0 now=1, p maybe large. For ph=1, p=n/2, now=1 for all n/2 butterflies. We can special case now==1 to avoid multiplication/mod. This could save significant (first stage half of butterflies). Also in other stages, some s have now=1? Only s=0. For ph=1, p=n/2, now=1. For ph>1, s=0 p=n/2^{ph}, now=1. Total butterflies with now=1 = sum_{ph} n/2^{ph} = n-1. About n per transform, small compared to n/2 log n. But ph=1 is n/2, significant for small n. Could implement:
```
if now == 1:
   for i...:
      l=a[i]; r=a[j]
      x=l+r; ... y=l-r; ...
else:
   for i...:
      r=a[j]*now%mod
```
This adds branch per s, not per butterfly. For s=0, use no mult. Good. In inverse, inow==1 for s=0, but y*inow%mod can skip. Let's add for speed.

Forward ntt inner:
```
if now == 1:
    for i in range(base,end):
        j=i+p
        l=a[i]; r=a[j]
        x=l+r; ...
else:
    for i in range(base,end):
        j=i+p
        l=a[i]; r=a[j]*now%mod
        ...
```
Inverse:
```
if inow == 1:
    for i...:
        l=a[i]; r=a[j]
        x=l+r; ...
        a[i]=x; a[j]=y
else:
    for i...:
        ... a[j]=y*inow%mod
```
This saves mod mult for s=0 blocks. Good.

Could also special if now == mod-1? Not common.

Now, in forward, for s where now maybe 1 only s=0. For ph=1, p=n/2, saves n/2 mults. For ph=2, p=n/4, etc. Total n-1 mults saved per transform. Not huge but good.

Now, let's think about if `now` can be 1 for other s due to root order? Maybe not.

Now, in pointwise multiplication, if F[i]==0 or A[i]==0? Many zeros? A sparse maybe, but after NTT dense. No.

Now, let's consider if we can use `a[i] = x; a[j] = y` where x,y already mod. Good.

Now, let's think about if in inverse, `y` after add/sub mod, then if inow==1 assign y. Good.

Now, let's update ntt/intt code with special now==1.

NTT forward:
```
def ntt(a):
    n=len(a); h=n.bit_length()-1; mod=MOD; rtz=rate_tz
    for ph in range(1,h+1):
        w=1<<(ph-1); p=1<<(h-ph); shift=h-ph+1
        now=1
        for s in range(w):
            base=s<<shift; end=base+p
            if now == 1:
                for i in range(base,end):
                    j=i+p
                    l=a[i]; r=a[j]
                    x=l+r
                    if x>=mod: x-=mod
                    y=l-r
                    if y<0: y+=mod
                    a[i]=x; a[j]=y
            else:
                for i in range(base,end):
                    j=i+p
                    l=a[i]; r=a[j]*now%mod
                    x=l+r
                    if x>=mod: x-=mod
                    y=l-r
                    if y<0: y+=mod
                    a[i]=x; a[j]=y
            now=now*rtz[s]%mod
```
Inverse similar.

Now, let's think about if `now` update after s=0 uses rtz[0], which may be 1? For MOD, rate2[0]=sqrt(-1) not1. OK.

Now, let's consider if `rtz[s]` lookup for s where now==1? s=0, rtz[0]. OK.

Now, let's think about if `h=0` (n=1), loops none. Good.

Now, let's consider if `rate_tz` precompute for s=0 is rate2[0]. Good.

Now, let's maybe precompute `rate_tz` and `irate_tz` as tuples for faster? List fine.

Now, let's think about if `pow` in intt final scaling skipped. Good.

Now, let's consider if we need to scale `A` by inv_n before query; we do per query. If rb is large, many multiplications. Could scale all A if rb > n//4? But rb <= n/2 typically. Per query saves. If rb close to n (possible if L small? Mlen~rb, n~Mlen, rb~n), per query n multiplications vs scaling n multiplications same. But per query also subtraction. OK.

Now, let's think about if `inv_n` multiplication can be avoided by incorporating into F before NTT? If we multiply F by inv_n before forward, then convolution result scaled by inv_n, inverse unscaled gives conv (not n*conv)? Wait forward linear: if F' = F * inv_n, then pointwise A*F', inverse unscaled gives n*(A*F') = n*inv_n*(A*F) = A*F. Yes! We can scale F by inv_n before NTT, then after inverse unscaled A contains correct convolution, no per-query scaling. But scaling F length n (or Mlen) before NTT costs Mlen multiplications, similar to per-query rb. Since rb <= Mlen? Mlen >= rb, so scaling F is >= per-query. But scaling F can be done in Python loop over Mlen, maybe similar. Per-query also does subtraction. Not clear. Current per-query okay.

Could scale A instead? A length L maybe <rb. If L < rb, scaling A by inv_n before NTT costs L multiplications, then inverse unscaled gives conv. Since L may be much smaller than rb (sparse c range? L could be small). This is a good optimization! We can multiply A by inv_n before forward NTT (or after pointwise?) If we scale A by inv_n before forward, linearity: A' = A*inv_n, F unchanged. Pointwise A'*F, inverse unscaled gives n*(A'*F) = A*F. Correct. Cost L multiplications. L can be much smaller than rb. In dense, L~rb/2, similar. In sparse, L small. Let's do this: after building A, before ntt(A), multiply each nonzero? A has zeros and nz nonzeros. We can scale only nonzero entries when filling: `A[pos] = H[c] * inv_n % mod`. Then no per-query scaling. But wait inverse unscaled gives n*(scaled A * F) = n*inv_n*(A*F)=A*F. Yes. This is better: scale A entries (nz multiplications) instead of rb query multiplications. nz <= L, and often nz <= rb? In dense, nz~L~rb/2, saves half. In sparse, saves a lot. Let's adopt.

Then in NTT branch:
```
inv_n = inv_pow2[n.bit_length()-1]
A=[0]*n
for k in range(nz):
    c=nz_list[lo+k]
    A[B_l[c]-c0] = H_l[c] * inv_n % mod
...
ntt(A); ntt(F); pointwise; intt(A)
for i in range(ql,qr+1):
    val_l[i] = (val_l[i] - A[idx]) % mod
```
No inv_n in query. Great. Need ensure if A entry gets multiple H? Each c unique position, no. If H[c]*inv_n mod. Good.

This changes convolution result to correct conv. Let's verify: Forward transform linear over mod. Let A_scaled = A * inv_n (elementwise). Conv = A_scaled * F. Inverse unscaled returns n*Conv = n*inv_n*(A*F)=A*F. Yes.

This saves query multiplications and maybe allows intt no scaling. Good.

Now, if n=1, inv_n=1. OK.

Now, in direct branch no.

Now, let's think about if scaling A by inv_n before NTT affects values if A has zeros; we only set nonzero. Good.

Now, pointwise multiplication A[i]*F[i] where A already scaled. Good.

Now, query term = A[idx] (mod). Good.

This is a nice optimization.

Could also scale F by inv_n if L > nz? We scale only nz entries, minimal. Good.

Now, let's consider if `H_l[c] * inv_n % mod` for each c in NTT branch. nz maybe large, but less than rb. OK.

Now, let's think about if we can avoid modulo in scaling by letting A entries be H*inv_n (product <1e18) and NTT expects mod values? NTT operations assume inputs <mod for add/sub one-step. If input >=mod, x=l+r could exceed 2mod, one subtract not enough. So need mod. OK.

Now, let's update NTT branch.

Now, let's consider if `intt` unscaled with A scaled gives correct conv mod. Yes.

Now, let's think about if we should scale A after pointwise instead? Same cost n. Before is nz.

Now, let's consider if `inv_n` precomputed as modular inverse of n. Since n power of two <MOD, ok.

Now, let's think about if `A` entries scaled by inv_n, but convolution length n includes cyclic convolution? NTT computes cyclic convolution modulo x^n-1. We choose n >= need, so linear convolution no wrap. Scaling linear. OK.

Now, let's consider if `F` values fact are mod. Good.

Now, let's think about if direct branch uses `val_l[i] = s % mod` where s may be negative. OK.

Now, let's maybe adjust `ntt_cost` estimate to account for scaling A nz? Not needed.

Now, let's think about if we can use `array` for `fact`? List fine.

Now, let's consider if there is a possibility of `M` being 0 after valid list if S1=B,S2N=W? For N=1, valid b=1. For N>1, b=N valid. So M>=1. But keep.

Now, let's think about if `e[b] > e[b-1]` for b=N uses e[N-1] computed. If last char W, yes. If there are multiple W after last B, e[N-1] < N. Good.

Now, let's consider if `e` array for b where no B? N B total. OK.

Now, let's think about if input S may contain newline after N, `input().strip()` ok.

Now, let's maybe include `if not S: return`? Not needed.

Now, let's think about if code uses `1_000_000` underscores, Python 3.6+ ok. AtCoder Python 3.11. OK.

Now, let's consider if final answer should be just code and notes, no extra text. Yes.

Now, let's maybe write notes.

Notes content:
- Strong connectivity reduces to 2N reaching 1; requires S1=B,S2N=W.
- Define C_k crossing edges; need C_k>0 all cuts. Hardest cuts before each B_{b+1}; bad prefix b if first b black matched to first e_b white.
- e_b #W before (b+1)-th B, e_N=N. Count permutations π of white indices assigned to black indices; bad b iff max_{j<=b} π_j <= e_b.
- First bad prefix decomposition: H_b = #injections of first b black into first e_b white with no earlier bad. Answer H_N. Recurrence as given. H_b=0 unless b>0, e_b>=b, e_b>e_{b-1}.
- Compute valid b compressed to arrays B,E. Recurrence H_i = (fact[E_i] - sum_{j<i} H_j fact[E_i-B_j]) * invfact[E_i-B_i].
- CDQ over compressed indices. val[i] starts fact[E_i], accumulates negative contributions. solve(l,r): solve left, apply left->right, solve right. Leaf direct.
- apply: nonzero H indices in left via global sorted list. If product small direct. Else convolution: A over original black index range c0..c1 with H[c] scaled by inv_n, F=fact[t_min..t_max], t_min=E[ql]-c1, t_max=E[qr]-c0. Query val[i]-=conv[E[i]-c0-t_min].
- NTT mod 998244353, ACL butterfly, no final scaling because A pre-scaled by inv_n.
- Constants: naive M<=2000, leaf64, direct threshold/cost factor.

Need keep under 800 words, bullets. Avoid headings. Use '- **Topic:** ...'. No ALLCAPS. Ensure no line starts with #. Bold topic okay.

Now, let's think if notes mention "ALLCAPS" no. Use lowercase.

Now, let's consider if final code has any syntax errors. Let's write carefully.

Full code draft:
```
import sys
from bisect import bisect_left, bisect_right

MOD = 998244353
G = 3

def main():
    input = sys.stdin.readline
    N_line = input().strip()
    if not N_line:
        return
    N = int(N_line)
    S = input().strip()
    if S[0] != 'B' or S[-1] != 'W':
        print(0)
        return

    e = [0] * (N + 1)
    w = 0
    b = 0
    for ch in S:
        if ch == 'W':
            w += 1
        else:
            b += 1
            if b <= N:
                e[b - 1] = w
    e[N] = N

    B = [0]
    E = [0]
    for b in range(1, N + 1):
        if e[b] >= b and e[b] > e[b - 1]:
            B.append(b)
            E.append(e[b])
    M = len(B) - 1
    if M == 0:
        print(0)
        return

    fact = [1] * (N + 1)
    for i in range(1, N + 1):
        fact[i] = fact[i - 1] * i % MOD
    invfact = [1] * (N + 1)
    invfact[N] = pow(fact[N], MOD - 2, MOD)
    for i in range(N, 0, -1):
        invfact[i - 1] = invfact[i] * i % MOD

    if M <= 2000:
        H = [0] * (M + 1)
        for i in range(1, M + 1):
            ei = E[i]
            s = fact[ei]
            for j in range(1, i):
                hj = H[j]
                if hj:
                    s -= hj * fact[ei - B[j]]
            H[i] = s % MOD * invfact[ei - B[i]] % MOD
        print(H[M] % MOD)
        return

    max_n = 1
    while max_n < 2 * N + 5:
        max_n <<= 1
    max_log = max_n.bit_length() - 1

    rate2 = [0] * 30
    irate2 = [0] * 30
    for i in range(30):
        rate2[i] = pow(G, (MOD - 1) >> (i + 2), MOD)
        irate2[i] = pow(rate2[i], MOD - 2, MOD)

    half = max_n // 2
    rate_tz = [1] * (half + 1)
    irate_tz = [1] * (half + 1)
    for s in range(half):
        x = s + 1
        tz = (x & -x).bit_length() - 1
        rate_tz[s] = rate2[tz]
        irate_tz[s] = irate2[tz]

    inv_pow2 = [1] * (max_log + 1)
    inv2 = (MOD + 1) // 2
    for i in range(1, max_log + 1):
        inv_pow2[i] = inv_pow2[i - 1] * inv2 % MOD

    def ntt(a):
        n = len(a)
        h = n.bit_length() - 1
        mod = MOD
        rtz = rate_tz
        for ph in range(1, h + 1):
            w = 1 << (ph - 1)
            p = 1 << (h - ph)
            shift = h - ph + 1
            now = 1
            for s in range(w):
                base = s << shift
                end = base + p
                if now == 1:
                    for i in range(base, end):
                        j = i + p
                        l = a[i]
                        r = a[j]
                        x = l + r
                        if x >= mod:
                            x -= mod
                        y = l - r
                        if y < 0:
                            y += mod
                        a[i] = x
                        a[j] = y
                else:
                    for i in range(base, end):
                        j = i + p
                        l = a[i]
                        r = a[j] * now % mod
                        x = l + r
                        if x >= mod:
                            x -= mod
                        y = l - r
                        if y < 0:
                            y += mod
                        a[i] = x
                        a[j] = y
                now = now * rtz[s] % mod

    def intt(a):
        n = len(a)
        h = n.bit_length() - 1
        mod = MOD
        itz = irate_tz
        for ph in range(h, 0, -1):
            w = 1 << (ph - 1)
            p = 1 << (h - ph)
            shift = h - ph + 1
            inow = 1
            for s in range(w):
                base = s << shift
                end = base + p
                if inow == 1:
                    for i in range(base, end):
                        j = i + p
                        l = a[i]
                        r = a[j]
                        x = l + r
                        if x >= mod:
                            x -= mod
                        y = l - r
                        if y < 0:
                            y += mod
                        a[i] = x
                        a[j] = y
                else:
                    for i in range(base, end):
                        j = i + p
                        l = a[i]
                        r = a[j]
                        x = l + r
                        if x >= mod:
                            x -= mod
                        y = l - r
                        if y < 0:
                            y += mod
                        a[i] = x
                        a[j] = y * inow % mod
                inow = inow * itz[s] % mod

    val = [0] * (M + 1)
    for i in range(1, M + 1):
        val[i] = fact[E[i]]
    H = [0] * (M + 1)
    nonzero = []

    LEAF = 64
    DIRECT_ALWAYS = 4096
    COST_FACTOR = 4
    bl = bisect_left
    br = bisect_right

    def apply(l, m, ql, qr):
        lo = bl(nonzero, l)
        hi = br(nonzero, m)
        if lo == hi:
            return
        nz = hi - lo
        rb = qr - ql + 1
        direct_cost = nz * rb
        mod = MOD
        fact_l = fact
        E_l = E
        B_l = B
        val_l = val
        H_l = H
        nz_list = nonzero

        c0 = B_l[nz_list[lo]]
        c1 = B_l[nz_list[hi - 1]]

        if direct_cost <= DIRECT_ALWAYS:
            use_direct = True
        else:
            L = c1 - c0 + 1
            t_min = E_l[ql] - c1
            if t_min < 0:
                t_min = 0
            t_max = E_l[qr] - c0
            Mlen = t_max - t_min + 1
            need = L + Mlen - 1
            n = 1 << (need - 1).bit_length()
            ntt_cost = n * (n.bit_length() - 1)
            use_direct = direct_cost <= ntt_cost * COST_FACTOR

        if use_direct:
            if nz == 1:
                c = nz_list[lo]
                hj = H_l[c]
                bj = B_l[c]
                for i in range(ql, qr + 1):
                    val_l[i] = (val_l[i] - hj * fact_l[E_l[i] - bj]) % mod
                return
            hj_list = [0] * nz
            bj_list = [0] * nz
            for k in range(nz):
                c = nz_list[lo + k]
                hj_list[k] = H_l[c]
                bj_list[k] = B_l[c]
            for i in range(ql, qr + 1):
                ei = E_l[i]
                s = val_l[i]
                for k in range(nz):
                    s -= hj_list[k] * fact_l[ei - bj_list[k]]
                val_l[i] = s % mod
            return

        # NTT branch: L, t_min, t_max, Mlen, n are defined in the else above.
        inv_n = inv_pow2[n.bit_length() - 1]
        A = [0] * n
        for k in range(nz):
            c = nz_list[lo + k]
            A[B_l[c] - c0] = H_l[c] * inv_n % mod
        F = [0] * n
        F[:Mlen] = fact_l[t_min:t_max + 1]
        ntt(A)
        ntt(F)
        for i in range(n):
            A[i] = A[i] * F[i] % mod
        del F
        intt(A)
        base_idx = c0 + t_min
        for i in range(ql, qr + 1):
            idx = E_l[i] - base_idx
            val_l[i] = (val_l[i] - A[idx]) % mod

    sys.setrecursionlimit(1_000_000)

    def solve(l, r):
        if r - l < LEAF:
            fact_l = fact
            E_l = E
            B_l = B
            val_l = val
            H_l = H
            invfact_l = invfact
            mod = MOD
            append = nonzero.append
            for i in range(l, r + 1):
                ei = E_l[i]
                s = val_l[i]
                for j in range(l, i):
                    hj = H_l[j]
                    if hj:
                        s -= hj * fact_l[ei - B_l[j]]
                hi_val = s % mod * invfact_l[ei - B_l[i]] % mod
                H_l[i] = hi_val
                if hi_val:
                    append(i)
            return
        mid = (l + r) >> 1
        solve(l, mid)
        apply(l, mid, mid + 1, r)
        solve(mid + 1, r)

    solve(1, M)
    print(H[M] % MOD)

if __name__ == "__main__":
    main()
```

Need check variable `M` shadowed by `Mlen`? We use M for valid count, Mlen local. OK.

Potential issue: In apply, if `direct_cost > DIRECT_ALWAYS`, we compute `L,t_min,t_max,Mlen,need,n,ntt_cost`. If `use_direct` true, these variables defined but not used. If `use_direct` false, defined. Good. But Python's compiler sees `L` etc assigned in else, used in NTT branch. Since there is a path where use_direct false only if else executed, OK. No UnboundLocalError.

Potential issue: `ntt_cost = n * (n.bit_length() - 1)`. If n=1, ntt_cost=0. But direct_cost > DIRECT_ALWAYS impossible as argued. If it happened, use_direct false, NTT with n=1. inv_pow2 index0. A length1, F length1. ntt h=0, intt h=0. Query idx? L=Mlen=1, base_idx=c0+t_min, E[ql]-base_idx=0. OK.

Potential issue: In NTT branch, `F[:Mlen] = fact_l[t_min:t_max + 1]`. If Mlen maybe > n? n >= L+Mlen-1 >= Mlen, ok. If slice length Mlen. Good.

Potential issue: `A[B_l[c] - c0] = H_l[c] * inv_n % mod`. If two c map to same position? B_l[c] unique for compressed valid indices, so no. If H_l[c] zero? c from nonzero, nonzero. Good.

Potential issue: In direct branch, `val_l[i] = (val_l[i] - hj * fact_l[E_l[i] - bj]) % mod`. If `E_l[i] - bj` negative? We argued no. But if due to bug, list index negative would wrap. Could add assert? Not in final. Trust.

Potential issue: In leaf solve, `for j in range(l, i):` includes j where H_l[j] maybe not computed? We compute i increasing, so yes. If l=1, range empty.

Potential issue: `nonzero` list used in apply may include indices >m? We argued no. But let's double-check with CDQ and leaf threshold. solve(1,M): solve left, apply, solve right. In solve right, before its apply, nonzero includes left of right and earlier. For apply within right (l=mid+1,m=mid2), nonzero may include indices in [mid+1,mid2] (just solved) and also indices <mid+1. It does not include >mid2. Good. Because solve right's left child just finished, no right child. In deeper, same. So bisect_right(m) safe. Even if it included >m (shouldn't), bisect_right would exclude. Good.

Potential issue: In apply, `c0 = B_l[nz_list[lo]]`, `c1 = B_l[nz_list[hi - 1]]`. If nonzero list has indices outside [l,m] due to bisect? lo/hi restrict. Good.

Potential issue: In NTT branch, `t_min = E_l[ql] - c1`. If c1 > E_l[ql], t_min negative set0. But then query idx maybe not align for c>c? We argued impossible. If it happened, scaling A by inv_n and F from 0 would include terms for negative t? For c with e[b]-c negative, the convolution query might include fact[0] incorrectly? Let's analyze if t_min set0 but actual min negative. For a b,c with t negative, the desired term should be 0. In convolution with F starting at 0, query idx = e[b]-c0. Sum over i+j=idx. For c with i=c-c0, j=idx-i = e[b]-c. If e[b]-c negative, j negative, not included. So no incorrect term. For c with t positive, j positive. Good. Extra F low indices not used. So max0 safe.

Potential issue: `t_max = E_l[qr] - c0`. If t_max > N? E<=N,c0>=1 => <=N-1. OK.

Potential issue: In NTT branch, `need = L + Mlen - 1`. If t_min set0 but actual min negative, Mlen larger, n larger, but correct. OK.

Potential issue: In `ntt`, `rtz[s]` for s up to w-1. `rate_tz` length half+1 where half=max_n//2. For n=max_n, w max n/2=half, s max half-1. OK. For n smaller, ok.

Potential issue: In `intt`, `itz[s]` same.

Potential issue: Precompute `rate2` length 30. If max_n maybe 2^23 for N up to 4e6? Constraints 2e5, max_log19. OK. If N larger, 30 enough up to 2^28? MOD supports 2^23, but max_n cannot exceed 2^23 for NTT. 30 safe.

Potential issue: `pow(G, (MOD - 1) >> (i + 2), MOD)` for i=29 shift31, exponent0. OK.

Potential issue: In `ntt`, for `now == 1` branch, `r = a[j]` not multiplied. But if `a[j]` maybe >=mod? NTT maintains mod. Input A/F mod. Good.

Potential issue: In `intt`, for `inow == 1` branch, `a[j]=y` not multiplied. Good.

Potential issue: In `intt`, after inverse unscaled, values are n*conv mod. But we scaled A by inv_n, so values conv mod. However, the inverse butterfly without scaling also multiplies by n at the end implicitly. We accounted. Let's verify with n=2: A_scaled = A*inv2. Forward: [a0+a1, a0-a1] scaled? Actually A_scaled entries. F. Pointwise. Inverse unscaled: [2*(A0F0+A1F1), 2*(A0F0-A1F1)] = 2*inv2*(A*F) = A*F. Good.

Potential issue: In NTT branch, we scale A entries by inv_n, but A has zeros. The forward transform of scaled A is scaled. Good.

Potential issue: In direct branch, we do not scale. Good.

Potential issue: In leaf solve, `val_l[i]` may have been scaled? No, val stores contributions mod, not scaled.

Potential issue: In apply NTT, after `intt(A)`, A contains convolution mod. But due to scaling A by inv_n, yes. Let's test with simple: A=[1],F=[2],n=1,inv_n=1, A_scaled=1, conv=2. OK. n=2: A=[1,0],F=[2,3], inv2. A_scaled=[inv2,0]. Conv linear should [2,3]. NTT: forward A_scaled, F, pointwise, inverse unscaled gives 2*(A_scaled*F) = [2,3]. Good.

Now, let's think about if `inv_pow2` index for n maybe max_log, precomputed. Good.

Now, let's consider if `max_n` while < 2*N+5 could be larger than MOD's max NTT length 2^23? N=2e5, no. If N close 4e6, 2N=8e6 >2^23, NTT invalid. Constraints safe.

Now, let's think about if `fact_l[t_min:t_max + 1]` creates a list of references to ints; F initially zeros, slice assignment replaces first Mlen with references. Then ntt(F) modifies F entries, not fact. Good.

Now, let's consider if memory of `F` slice temporary plus F list. OK.

Now, let's think about if `del F` before intt frees F, but in PyPy maybe not immediate. OK.

Now, let's consider if we should `del A` after apply? Function returns, local freed. OK.

Now, let's think about if recursion solve uses `apply` which uses `nonzero` list that is being appended in solve leaf. No concurrent modification during apply. Good.

Now, let's consider if `nonzero` list can be large and bisect uses Python C, fast.

Now, let's think about if `M <= 2000` naive uses O(M^2) with Python loops. 2M operations, fine. Could set 3000 (4.5M) maybe fine. Keep 2000.

Now, let's consider if for M=2001, CDQ precompute NTT and maybe direct only (no NTT if small). OK.

Now, let's think about if `S` length maybe not 2N due to input issue; assume valid.

Now, let's maybe include a small comment in code about A pre-scaled. OK.

Now, let's think about if final code's `main` reads N_line and if empty returns. Good.

Now, let's consider if `S[0]` and `S[-1]` for empty S would error, but N>=1. If input missing, N_line empty return.

Now, let's think about if there is a possibility that `e[b] > e[b-1]` but `e[b] < b` for b=N? e[N]=N, no. For b<N, skip.

Now, let's consider if `e` values for invalid b not used except e[b-1] in valid condition. We compute all.

Now, let's think about if `e[b]` can be N before b=N? If all W before last B, but last char W so no. If last char W, e[N-1]<=N-1. For b<N, e[b]<=N-1. OK.

Now, let's consider if answer H_N modulo could be negative? H stored mod positive. Print.

Now, let's think about if in naive, `s` can be negative large (i up to2000, product 1e18*2000=2e21), `%` ok.

Now, let's consider if in CDQ leaf, `s` internal sum up to64 terms, small. val[i] may have accumulated many contributions modded. OK.

Now, let's think about if in direct apply, `s` accumulates nz terms. If nz large (e.g. 100k) and rb small, s 1e23, `%` ok. Python big int 3 limbs. Good.

Now, let's consider if in direct apply, `hj_list[k] * fact_l[ei - bj_list[k]]` product of two mod ints <1e18, Python int 60 bits. s grows. OK.

Now, let's think about if in NTT, `A[i] = A[i] * F[i] % mod` product <1e18. OK.

Now, let's consider if in NTT inner, `r = a[j] * now % mod`, product <1e18. OK.

Now, let's think about if `x = l + r` can be >=2mod? l,r<mod, sum<2mod, one subtract enough. `y = l - r` > -mod, one add enough. Good.

Now, let's consider if in inverse, `y * inow % mod`, y<mod, inow<mod. OK.

Now, let's think about if `now` update `now = now * rtz[s] % mod` after inner. If `rtz[s]` maybe 1, ok.

Now, let's consider if `rate_tz` precompute for s=half? We don't use s=half. length half+1 safe.

Now, let's think about if `max_n` maybe 2, half=1, rate_tz length2, loop s=0. OK.

Now, let's consider if `ntt` with n not power of two? We always pass power of two. Good.

Now, let's think about if `need` maybe 0? no.

Now, let's consider if `COST_FACTOR` and `DIRECT_ALWAYS` should be tuned for CPython vs PyPy. We can't know. Maybe choose slightly more direct to avoid NTT overhead in CPython? If judge uses CPython, NTT 30M loops with `%` likely TLE. Direct 25M loops maybe also TLE but perhaps less? Need best chance. AtCoder uses PyPy for Python submissions usually. The problem likely expects C++ but Python maybe allowed? We need produce Python solution. Could there be a simpler O(N log N) with fewer NTT? Let's revisit to see if we can reduce complexity.

Maybe H_N can be computed by a single determinant using the Lindstrom-Gessel-Viennot theorem, and determinant of a Hessenberg matrix can be computed O(N^2) not good. But maybe matrix is triangular with factorial entries, determinant product? Let's explore more; could yield O(N) formula.

We count permutations π with M_b > e_b. This is equivalent to π avoiding the region under path e_b. There is a known formula for number of permutations with prescribed prefix maxima lower bounds: If a_i is nondecreasing, number of permutations with max_{i<=k} π_i > a_k for all k is ∏_{k=1}^N (a_k - a_{k-1})? Let's test.

For permutations with M_k <= a_k for all k (upper bounds), count is ∏ (a_k - a_{k-1})? Actually number of permutations where M_k <= a_k for all k (i.e. first k elements from [a_k]) is ∏_{k=1}^N (a_k - k +1)? Let's derive: To have M_k <= a_k, at step k choose an unused element <=a_k. Number of choices at step k given previous? If a_k nondecreasing and a_k>=k, the number of permutations with M_k <= a_k for all k is ∏_{k=1}^N (a_k - k +1)? Check N=2, a=[1,2]: permutations with M1<=1 (π1=1), M2<=2 all: 1. Product (1-1+1)*(2-2+1)=1. a=[2,2]: M1<=2 all, M2<=2 all:2. Product (2)*(1)=2. a=[1,1] invalid. This is number of permutations whose inverse? It is like parking functions.

We need M_k > e_k, complement of M_k <= e_k for some k. Since conditions M_k <= e_k are not monotone? The set of permutations satisfying all M_k <= e_k (upper bounded) count product if e_k nondecreasing and e_k>=k. But we need avoid all upper bounds, not satisfy all. Inclusion-exclusion over first bad gave H. Maybe H_N = N! - sum ... no simple product.

But maybe by complement symmetry, number with M_k > e_k for all k equals number of permutations with M_k <= N - something? Let's transform values: Let σ_k = N+1 - π_k. Prefix max of π > e_k iff prefix min of σ < N+1-e_k. Not simpler.

Could use rook polynomial of a Ferrers board. The condition M_k <= e_k for all k means permutation matrix lies in a Ferrers board with row k columns <=e_k. Number of permutations in Ferrers board is ∏ (e_k - k +1) if e_k nondecreasing. We need permutations not in union of events? First bad b means for prefix b, matrix in board with row i<=e_i for i<=b, and for c<b not. H_b maybe number of permutations in board b avoiding smaller boards. There is a known formula for "indecomposable" permutations with respect to a board: H_b = ∏? Let's compute H_b for board with row bounds e_i. Total permutations in board b (first b rows, columns <=e_i, but columns can be any up to e_b? Actually injection first b black to first e_b white with row i bound e_i? Bad prefix c means first c rows columns <=e_c. H_b counts injections of b rows into e_b columns such that for all c<b, row c not all within e_c. This is like permutations of a Ferrers board that are "indecomposable". There may be a formula using determinant of a matrix with entries binomial(e_i, i). But not product.

Let's test H_b for e=[0,2,3] b=2: H2=2. Product of (e_i - e_{i-1}?) 2*1=2. For e=[0,1,2,3] b=3: H3=3, product increments 1*1*1=1 no. Maybe H_b = number of permutations of b with M_i > e_i, which for e_i=i-1 (alternating) is derangements? For N=3 e=[0,1,2], count permutations with M1>1 (π1≠1), M2>2 (3 in first2) =3. Not derangements (2). For N=4 e=[0,1,2,3], count? Recurrence H4 = 4! - H1*3! - H2*2! - H3*1! =24-6-2-3=13. This sequence 1,1,3,13? Maybe number of permutations with no prefix max <=i-1, i.e. M_i >= i+1. This is "permutations with no fixed point in prefix max"? Sequence maybe 1,1,3,13,71? Not simple product.

So CDQ likely needed.

Could use generating functions for alternating case, but general no.

Now, let's think about if we can use a Fenwick tree with polynomial values at factorial indices. Query sum H_c fact[E_i-B_c]. If we process i increasing, E_i nondecreasing. For each c, function f_c(x)=H_c fact[x-B_c] for x>=B_c. We need point queries at x=E_i. This is like adding a shifted factorial sequence. We could use a data structure that supports adding a shifted factorial and point query. Since factorial sequence satisfies recurrence fact[k]=k*fact[k-1]. Maybe we can maintain moments? For a fixed shift B, fact[x-B] as function of x is not a polynomial of fixed degree, but can be represented in basis of falling factorials? fact[x-B] = Γ(x-B+1). Not finite.

But x up to N, we could use difference arrays with multiplicative updates? If we add H_c at position B_c, query at x: sum H_c fact[x-B_c]. This is convolution. Online convolution can be done with sqrt decomposition: block size S. For each block of c, precompute convolution of H in block with fact to answer queries in O(1) per block? Let's explore: Divide c indices into blocks of size S. For each completed block, build an array conv_block[t] = sum_{c in block} H_c fact[t-B_c] for t range. Then query at x sums conv_block[x] over completed blocks plus direct for current block. Number blocks N/S, query O(N/S), update block rebuild O(S log S?) Total O(N^2/S + N*S log S). Choose S~sqrt(N log N) ~1000, O(4e7) maybe. CDQ NTT O(N log^2 N) ~? Similar. Sqrt might be simpler and maybe faster in Python? Let's evaluate.

Online sqrt algorithm:
Process i=1..M. Maintain blocks of previous H. For each block (size S) once complete, compute its contribution array for all possible x? x up to N. For block with c indices B_c, need conv[t] for t=0..N maybe. We can compute by direct O(S*N) too big, or NTT per block O(N log N) per block -> (N/S)*N log N too big. But we only need queries at future E_i, which are M points. Could for each block compute contributions to all future i by direct O(S * remaining) -> total O(N^2). Not good.

Could use NTT per block with fact length N: (N/S) NTTs of size N, too big.

CDQ better.

Now, let's think about if we can use "relaxed convolution" with blocks and NTT of size block*? There is algorithm O(N log N) for online convolution using divide and conquer (CDQ) actually O(N log^2 N) standard, but with NTT and careful maybe O(N log N) if using "online convolution" with generating functions? For arbitrary online, CDQ is O(N log^2 N). There are algorithms O(N log N) using relaxed convolution with FFT? Maybe complex.

Given task explicitly CDQ, proceed.

Now, let's consider if Python CDQ NTT might still pass with N=2e5 if optimized. We can further reduce NTT work by using a larger leaf and direct for small, but we chose. Maybe we can set `LEAF=128` to reduce NTT calls? Direct leaf cost 12.8M, apply128? If LEAF=128, segments size128 leaf, apply for size256 direct? Let's see: solve leaf size128 direct internal 12.8M. Internal apply for node size256: left128 right128 direct_cost16384. With DIRECT_ALWAYS4096, cost estimate NTT (ntt_cost2048*4=8192) -> NTT. So NTT from256 still. Direct total 12.8M (vs 6.4+6.4=12.8 same). Fewer nodes/calls. LEAF=128 may be better (less recursion, same direct pairs? Pairs within leaf size128: N*128/2=12.8M. With LEAF=64, pairs within leaf6.4 + apply128 6.4 =12.8. Same. NTT from256 same. Fewer apply calls for size128 (they become leaf internal direct). Direct leaf inner loops maybe slightly slower due to no precomputed hj/bj? But similar. LEAF=128 reduces function calls and bisect. Let's set LEAF=128? Direct leaf inner for each i loops j from l to i, uses H[j] branch. For size128, 8128 ops per leaf, N/128=1562 leaves =>12.7M. With LEAF=64, leaf 2016*3125=6.3M plus apply128 direct: apply direct uses precomputed hj/bj and no branch, 4096*1562=6.4M. Similar. LEAF=128 simpler and fewer calls. But leaf direct uses branch `if hj` and H lookup, maybe slower than apply direct precomputed. However, apply direct for size128 also has inner loop over nz (all nonzero) and fact lookup. Similar. LEAF=128 may be okay. If H sparse, leaf branch skips, apply direct c_list nonzero only, better. But if sparse, NTT/direct cost small. I'd keep LEAF=64 to use apply direct for size128 with nonzero list (skips zeros) and precomputed lists. But function call overhead maybe. Hard.

Maybe LEAF=32: leaf3.2M, apply64 direct3.2M, apply128 direct6.4M, total12.8M, more calls. LEAF=64 seems balanced.

Now, let's think about if `DIRECT_ALWAYS=4096` means apply128 direct (4096) yes. apply256 NTT. Good.

Now, let's consider if we can use `LEAF=32` and `DIRECT_ALWAYS=1024` to NTT from128? More NTT. No.

Now, let's think about if `COST_FACTOR=4` maybe too low for CPython. Could set 8 to direct apply256. Let's decide. Since NTT heavy, maybe direct apply256 (12.8M simple) vs NTT 2.6M heavy. If heavy 5x, direct 12.8 vs 13, similar. If heavy 3x, NTT better. PyPy heavy maybe 2x? NTT better. Keep factor4.

Now, let's consider if we can make NTT faster by using `mod` as local and maybe `if x >= mod: x -= mod` branches. Good.

Could use `x = l + r; x -= mod if x >= mod else 0`? Branch as is fine.

Could use `y = l - r; y += mod if y < 0 else 0`. As is.

Now, let's think about if in `ntt`, `now == 1` branch for s=0, but `now` is Python int, comparison cheap. For s>0, false. Good.

Now, let's consider if in `intt`, `inow == 1` branch for s=0. Good.

Now, let's think about if `rate_tz` precompute could be done with `tz = (x & -x).bit_length() - 1`; for x power of two, bit_length-1 = log2. For x not power, trailing zeros. Good.

Now, let's consider if `rate2` index for tz maybe 0 for many s. OK.

Now, let's think about if `pow(rate2[i], MOD-2, MOD)` for i where rate2[i]=0? No.

Now, let's consider if `G=3` is primitive root. Yes.

Now, let's maybe include a fallback if `M` is large but `N` small? no.

Now, let's think about if the recurrence uses `fact[e_b-c]` where c original black index, but in compressed B[j] maybe not equal j. We use B. Good.

Now, let's consider if `e[b]` can be less than b for b=N? no.

Now, let's think about if `e[b] > e[b-1]` condition for b=1 when e0 maybe >0 if S1 not B, but early exit. OK.

Now, let's consider if there is a case S1=B,S2N=W but e[b] not strictly increasing for valid b? We skip equal. Good.

Now, let's think about if H_b for invalid b could be nonzero in recurrence if we didn't skip? We skip. Good.

Now, let's consider if the first bad prefix b could be invalid due to e_b=e_{b-1} but e_b>=b? We argued no. Let's prove: If first b black all matched to first e_b white, and e_b=e_{b-1}, then first b-1 black all matched to first e_{b-1} white, so bad b-1. Thus not first. Good.

Now, let's think about if e_b<b invalid: not enough white to match first b black, so bad impossible. Good.

Now, let's consider if H_N valid always? e_N=N>=N, e_N>e_{N-1} due last W. Yes.

Now, let's think about if answer H_N could be computed as val[N]*invfact[0] after CDQ. H[M] stored.

Now, let's consider if in CDQ, `val` for index M receives contributions from all c<M via applies. Then leaf computes H[M]. Good.

Now, let's think about if `val` for indices in a leaf receives contributions from c in same leaf? No, direct sum. Good.

Now, let's consider if `val` for indices in a leaf receives contributions from c in earlier part of same leaf via ancestor apply? No, same leaf means same segment at all levels, so no. Good.

Now, let's think about if `nonzero` list append in leaf after computing H[i] means for later i in same leaf, H[i-1] available. Good.

Now, let's consider if in leaf, `val_l[i]` might have been updated by apply from left half of an ancestor where i in right half. Yes. Good.

Now, let's think about if in apply direct, we update `val_l[i]` for all i in right half, including those that may be invalid? In compressed, all valid. Good.

Now, let's consider if in apply NTT, `F` range t_min..t_max maybe includes fact indices beyond N? t_max<=N-1. OK.

Now, let's think about if `fact_l[t_min:t_max + 1]` when t_min=0,t_max=N-1 length N. OK.

Now, let's consider if `A` length n maybe much larger than L due to Mlen. We fill only L positions. OK.

Now, let's think about if `A` scaling by inv_n should be done after ntt? No, before.

Now, let's consider if `intt` without scaling but A scaled by inv_n yields conv, but what about the forward transform of A scaled by inv_n: Since inv_n is a scalar, forward(A*inv_n)=inv_n*forward(A). Pointwise with F. Inverse unscaled gives n*inv_n*conv=conv. Good.

Now, let's think about if `inv_n` is modular inverse of n, and n may not be invertible if n multiple of MOD. n power of two <MOD, invertible. Good.

Now, let's consider if `max_n` maybe 1<<19=524288, inv_pow2 length20. Good.

Now, let's think about if in `ntt`, `h = n.bit_length() - 1`. For n=524288, h=19. `rate_tz` length half+1=262145. In ph=19, w=262144, s max262143, ok.

Now, let's consider if in `ntt`, `shift = h - ph + 1`. For ph=1, shift=h, base=s<<h (s=0), p=1<<(h-1). Good. For ph=h, shift=1, p=1, w=1<<(h-1), base=s<<1. Good.

Now, let's think about if ACL butterfly uses `offset = s << (h - ph + 1)`, yes.

Now, let's consider if in inverse, same shifts. Good.

Now, let's think about if `now` update in forward should happen after each s, including last. It does, but last not used. OK.

Now, let's consider if `rate_tz[s]` for s last maybe rate2[log2(w)]? For ph=h, s=w-1=2^{h-1}-1, s+1=2^{h-1}, tz=h-1. rate2[h-1]=G^{(MOD-1)/2^h}=primitive n-th root. Update after last not used. OK.

Now, let's think about if `irate_tz` similarly.

Now, let's consider if we should precompute `rate_tz` only up to half, but for ph smaller, s max smaller. OK.

Now, let's think about if `pow` for invfact[N] with N=200000, fine.

Now, let's consider if code uses too much recursion stack due to `sys.setrecursionlimit` after defining solve? It's before solve call, after definition. OK.

Now, let's think about if `main` returns after early exit, no NTT precompute. Good.

Now, let's consider if `S` may contain carriage return `
`, strip removes.

Now, let's think about if output should have newline. print adds.

Now, let's maybe include a small note in code that `intt` does not scale. OK.

Now, let's consider if there is any issue with using `M` as valid count and `Mlen` local; no.

Now, let's think about if in apply, `use_direct` variable maybe referenced before assignment if `direct_cost <= DIRECT_ALWAYS` false and else computes, true. OK.

Now, let's consider if in apply NTT branch, variables `L,t_min,t_max,Mlen,n` are defined in else. If `direct_cost > DIRECT_ALWAYS` but `use_direct` true, we return before NTT. If false, else executed. OK.

Now, let's think about if `ntt_cost` could be 0 and `use_direct` false, then NTT branch with n=1. Variables defined. OK.

Now, let's consider if `need - 1` bit_length for need=1 gives0, n=1. OK.

Now, let's think about if `F[:Mlen] = fact_l[t_min:t_max + 1]` when Mlen=0? no.

Now, let's consider if `A` fill loop uses `B_l[c] - c0` index. Since c0 is min B among nonzero in range, index >=0. c1 max, index <L. Good.

Now, let's think about if `H_l[c] * inv_n % mod` could be 0 if H_l[c] nonzero but inv_n nonzero, no. Unless mod, but H nonzero mod, product nonzero mod (field). OK.

Now, let's consider if in direct branch, `val_l[i] = s % mod` where s maybe Python int negative. OK.

Now, let's think about if in leaf, `hi_val = s % mod * invfact_l[ei - B_l[i]] % mod`. If `ei - B_l[i]` maybe negative? valid ensures >=0. Good.

Now, let's consider if `invfact_l` length N+1, index up to N. OK.

Now, let's think about if in naive, `H[i] = s % MOD * invfact[ei - B[i]] % MOD`. Good.

Now, let's consider if `fact[ei - B[j]]` index in naive maybe 0? If B[j]=ei? But B[j]<B[i]<=ei, so if B[i]=ei and B[j]=ei? no. If B[i]>ei? valid B[i]<=ei. So index >=1. OK.

Now, let's think about if there is a possibility of `e[b]` not sorted due to scan? It is nondecreasing. Good.

Now, let's consider if `e[N]=N` overwrites e[N-1]? No, e length N+1, e[N] separate.

Now, let's think about if scan sets e[b-1] for b=N (last B) to w before last B. If last char W, w before last B <=N-1. Good.

Now, let's consider if S has N W and N B, scan w ends N. Good.

Now, let's think about if early exit S[-1]!='W' catches sample2. Yes.

Now, let's consider if S[0]!='B' but S[-1]=='W', answer0. Good.

Now, let's think about if there is a case S1=B,S2N=W but no valid b except N? M=1, naive. H1=N! maybe. Example all B then all W: S=BBB...WWW. e_b=0 for b<N, e_N=N. Valid b=N only. H_N=N!/(0!)=N!. Is every matching strong? If all B first, all W last, any edge from W to B is backward crossing all cuts? For cut k within B prefix, suffix includes all W, black prefix some; since all W matched to B, at least one edge to prefix? If prefix black count b, there are b black in prefix, each matched to some W (all suffix), so C_k>=b>0. After B prefix, all black prefix, all W suffix, C>0. Yes all N! matchings strong. Good.

Now, let's think about if all W then all B but S1=W early0. Good.

Now, let's consider if answer can be 0 modulo even if actual positive. CDQ mod handles.

Now, let's think about if `nonzero` list should include index if H[i] mod nonzero. If H[i] actual multiple of MOD, it contributes 0 mod, skip. Good.

Now, let's consider if in apply, `nz_list = nonzero` and then in solve leaf `append = nonzero.append`. If apply is called while solve leaf appending? No.

Now, let's think about if Python's closure `nonzero` in apply sees updates. Yes.

Now, let's consider if `bisect_right(nonzero, m)` where nonzero is list of ints, m int. OK.

Now, let's think about if `lo` and `hi` are Python ints. OK.

Now, let's consider if `direct_cost = nz * rb` could be large (up to 4e10) but Python int fine.

Now, let's think about if `ntt_cost * COST_FACTOR` could be large, fine.

Now, let's consider if `ntt_cost` estimate for n=1 is0, direct_cost maybe >0, use_direct false, NTT branch. But as argued not. If it happens, `inv_n=inv_pow2[0]=1`, A length1, F length1, ntt/intt no loops, query. OK.

Now, let's think about if `F[:Mlen] = fact_l[t_min:t_max + 1]` when Mlen=1 and n=1, ok.

Now, let's consider if in NTT branch, after `del F`, the list F may still be referenced by slice? No, slice temporary gone.

Now, let's think about if `A` after intt contains convolution but maybe values not reduced? intt operations keep mod. Good.

Now, let's consider if in `intt`, final values are n*conv mod but with A scaled, conv mod. Since all operations mod, yes.

Now, let's think about if in `ntt`, input A entries scaled by inv_n, F entries fact. Both <mod. Good.

Now, let's consider if in `ntt`, `now` update uses `rtz[s]` where `rtz` list length half+1. For n smaller than max_n, h smaller, s max < half. OK.

Now, let's think about if `rate2` precompute 30 but `tz` could be 30 if half >=2^30, not with constraints. OK.

Now, let's consider if `max_n` while loop for N=200000: 2*N+5=400005, max_n sequence 1,2,4,...,262144,524288. max_log=19. inv_pow2 length20. Good.

Now, let's think about if `inv_pow2[i] = inv_pow2[i-1]*inv2%MOD` gives inverse of 2^i. Yes.

Now, let's consider if `inv2=(MOD+1)//2` correct for 998244353 (odd). Yes.

Now, let's think about if in `ntt`, `h = n.bit_length() - 1` for n power of two. If n=1, h=0. Good.

Now, let's consider if in `intt`, `h=0`, loops none, no scaling. Good.

Now, let's think about if we should call `ntt(F)` after `ntt(A)`. Order doesn't matter. OK.

Now, let's consider if pointwise multiplication loop can be optimized with list comprehension: `A = [(x*y)%mod for x,y in zip(A,F)]` creates new list, maybe faster in CPython but memory. In PyPy, loop maybe JIT. Keep loop.

Could use `for i, x in enumerate(A): A[i] = x * F[i] % mod`. Range index maybe faster? Not sure. Use range.

Now, let's think about if in direct branch, `for k in range(nz): s -= hj_list[k] * fact_l[ei - bj_list[k]]` could be optimized by localizing `hj_list_k`? No.

Could use `zip(hj_list,bj_list)` inside i loop: `for hj,bj in zip(hj_list,bj_list): s -= hj*fact_l[ei-bj]`. Zip iterator creation per i, tuple unpack. Index loop maybe faster. Keep index.

Now, let's consider if in leaf, inner loop `for j in range(l,i): hj=H_l[j]; if hj: s -= hj*fact_l[ei-B_l[j]]`. Could precompute B_l[j] and H_l[j]? Leaf small. OK.

Now, let's think about if `LEAF=64` and `DIRECT_ALWAYS=4096` means apply for segment size128 direct, but if H sparse nz small, direct. If H dense, direct 4096. Good.

Now, let's consider if `COST_FACTOR=4` and `ntt_cost` rough. For segment size256 dense, n maybe 256? Actually L=128, Mlen maybe E range right. If E increments by1, Mlen=128, need255,n=256, ntt_cost=2048, *4=8192, direct16384 -> NTT. Good.

If E jumps large, Mlen large, n larger, NTT even more. If B gaps large, L large, n larger. Good.

Now, let's think about if direct_cost small but ntt_cost zero? direct. OK.

Now, let's consider if in apply, `c0` and `c1` are original black indices of first/last nonzero. If nonzero list has gaps, L may be large. NTT cost estimate uses L. Good.

Now, let's think about if `t_min` and `t_max` should use `E_l[ql]` and `E_l[qr]` (first/last valid b in right). Since E increasing, min/max. Good.

Now, let's consider if right segment [ql,qr] all valid but E may have jumps; min E[ql], max E[qr]. Good.

Now, let's think about if `Mlen = t_max - t_min + 1` could be negative if t_min > t_max due to c1 > E[qr]? c1 < B[ql] <= E[ql] <= E[qr], so no.

Now, let's consider if `base_idx = c0 + t_min`, query idx = E[i] - base_idx. For first i, idx = E[ql]-c0-t_min = c1-c0 = L-1. For last, idx = E[qr]-c0-t_min = t_max-t_min = Mlen-1. Wait earlier I thought max idx maybe L+Mlen-2, but query only uses indices from L-1 to Mlen-1? That seems suspicious. Let's re-evaluate! This is critical.

We have A length L (indices 0..L-1 for c=c0..c1). F length Mlen (indices 0..Mlen-1 for t=t_min..t_max). Convolution index k = i + j. For a given b with e=E[i], and c=c0+i, t=e-c. F index j = t - t_min = e - (c0+i) - t_min = (e - c0 - t_min) - i. So k = i + j = e - c0 - t_min. Indeed for a fixed b, all c terms contribute to the same convolution index k = e - c0 - t_min. As b varies, k varies from for first b: e_first - c0 - t_min = e_first - c0 - (e_first - c1) = c1 - c0 = L-1. For last b: e_last - c0 - t_min = e_last - c0 - (e_first - c1) = (e_last - e_first) + (c1 - c0). This can be larger than Mlen-1? Mlen-1 = t_max - t_min = e_last - c0 - (e_first - c1) = same! Yes Mlen-1 = e_last - c0 - e_first + c1. So last idx = Mlen-1. Thus query indices range from L-1 to Mlen-1. But convolution length L+Mlen-1 has indices 0..L+Mlen-2. We only query a diagonal band of length rb (<=Mlen-L+1 maybe). Is that correct? Let's test with small: c0=2,c1=5,L=4. e_first=6,t_min=1. e_last=10,t_max=8,Mlen=8. Query first idx=3, last idx=7. Convolution indices 0..10. Terms for b with e=7: idx=7-2-1=4. Sum c=2 (i0) t=5 (j4), c=3(i1)t=4(j3), c=4(i2)t=3(j2), c=5(i3)t=2(j1). All within F. Good. For e=6 idx3: c=5 j0, c=4 j? t=2? Wait e=6,c=4 t=2, j=1, i=2 =>k3; c=3 t=3 j2 i1 k3; c=2 t=4 j3 i0 k3. Good. So query indices indeed from L-1 to Mlen-1 if e_last-e_first >=? If Mlen < L-1? Could happen if E range small and B range large? But E strictly increasing over rb, B range over nz. Since c1 < B[ql] <= E[ql], and c0 maybe much smaller, L can be larger than Mlen? Example c0=1,c1=100,L=100, e_first=101,e_last=102 (rb=2), t_min=1,t_max=101,Mlen=101. Mlen > L. In general t_max - t_min = (E[qr]-E[ql]) + (c1-c0) >= (rb-1)+(L-1). So Mlen-1 >= L-1 + rb-1 >= L-1. So query range within 0..Mlen-1, and Mlen-1 <= L+Mlen-2. Good. We only need a portion of convolution. But NTT computes full. OK.

This also means we could use middle product to compute only diagonal band, but not.

Now, let's think about if scaling A by inv_n and querying A[idx] works for idx in this range. Yes.

Now, potential issue: Since we only query indices up to Mlen-1, and Mlen may be less than L+Mlen-1, high indices not needed. But NTT full.

Now, let's consider if we can reduce F length to only needed t range for all b,c. We did t_min..t_max. Good.

Now, let's think about if A length L could be reduced to only nonzero c positions with gaps? Convolution requires contiguous index for difference c. Gaps zeros. OK.

Now, let's consider if we can use sparse convolution for small nz direct. We do.

Now, let's think about if in NTT branch, `F[:Mlen] = fact_l[t_min:t_max + 1]` but we only need F indices 0..Mlen-1. OK.

Now, let's consider if `A` scaled by inv_n, but we only query low indices up to Mlen-1. The inverse NTT computes all. OK.

Now, let's think about if there is a risk of cyclic convolution wrap affecting queried indices if n < L+Mlen-1. We choose n >= need. Good.

Now, let's consider if `need = L + Mlen - 1` maybe much larger than max queried index Mlen-1. Could choose smaller n? For linear convolution, to avoid wrap for queried indices up to Mlen-1, we need n > max_query + max_i? Cyclic wrap from high indices could affect low indices if n <= max_query + something. Standard need full length. Could use middle product with smaller n? Not worth.

Now, let's think about if `max_n` precompute based on 2N+5, but need = L+Mlen-1 <= (c1-c0+1)+(t_max-t_min+1)-1. c1-c0 <= N, t_max-t_min <= N, need<=2N. OK.

Now, let's consider if `t_max-t_min` can be N? c0=1,c1=N? t_min=E[ql]-N maybe 0, t_max=E[qr]-1 <=N-1, diff<=N-1. L<=N. need<=2N-1. OK.

Now, let's think about if `max_n` while <2*N+5 gives n maybe 2^19 for N=200000, need max399999, n524288. Good.

Now, let's consider if `rate2` precompute 30 but max_log19. OK.

Now, let's think about if in `ntt`, for n=524288, h=19, ph loops 19. Inner total butterflies n/2*19=4.98M per transform. 3 transforms ~15M for root. Plus other levels. OK.

Now, let's consider if Python recursion and NTT arrays allocation for root: A,F length524288. Memory ok. Time root maybe 1-2s in PyPy? Maybe.

Now, let's think about if we can avoid NTT for root if direct_cost huge? Root dense direct_cost ~1e10, NTT necessary.

Now, let's consider if there is a way to use `sys.setswitchinterval`? No.

Now, let's think about if final code should avoid type annotations for speed. No annotations.

Now, let's consider if `input = sys.stdin.readline` and `N_line = input().strip()`; if there are leading blank lines, N_line empty return. OK.

Now, let's think about if `S` could be split across lines? No.

Now, let's consider if `S` length 2N but `strip()` removes whitespace, ok.

Now, let's think about if modulo operations in direct branch with `% mod` for each i, but s may be negative. OK.

Now, let's consider if in direct branch nz>1, we create `hj_list` and `bj_list` of length nz. If nz large (e.g. 100k) and direct chosen due to rb=1, memory ~2*100k ints pointers, ok. Time O(nz). Good.

Now, let's think about if in direct branch nz>1 and rb large, direct_cost maybe <=threshold, nz small. OK.

Now, let's consider if in apply, `c0` and `c1` computed even if direct_cost small. Need nz_list[lo], hi-1. OK.

Now, let's think about if `nonzero` list can be empty but lo==hi return before c0. Good.

Now, let's consider if in solve, `mid=(l+r)>>1`. For l=1,r=2, r-l=1<LEAF leaf, no internal. For M large, balanced.

Now, let's think about if `LEAF=64` and M=65, r-l=64 not <64, internal mid33, left size33, right32. OK.

Now, let's consider if `apply(l,mid,mid+1,r)` with ql>qr? If mid+1<=r since l<r. OK.

Now, let's think about if `solve(1,M)` with M=0? We return earlier. M=1 leaf.

Now, let's consider if in naive M<=2000, we don't use CDQ. Good.

Now, let's think about if fact/invfact precompute before naive could be skipped for early exit. We do after early and valid. OK.

Now, let's consider if `e` array values for b where no B? All b 1..N have B. OK.

Now, let's think about if `e[b] > e[b-1]` for b=1 uses e[0] which is W before first B. If S1=B, e0=0. If there are W before first B but S1 not B early exit. OK.

Now, let's consider if `S[0]!='B'` but maybe vertex1 white could still be reached by added edge from white to black? Vertex1 white cannot be black target, so no. Good.

Now, let's think about if `S[-1]!='W'` but vertex2N black could have path edge? No outgoing path, black not source. Good.

Now, let's consider if final graph strongly connected if 2N reaches1, even if some vertex cannot reach2N? Path edges from any vertex to larger, so any vertex can reach2N via path. Yes.

Now, let's think about if C_k>0 all k is sufficient for 2N reaches1. We gave iterative argument. Need ensure at each step we can reach a white that has crossing edge. Suppose at position x, need cross cut x-1. C_{x-1}>0 gives some white y > x-1 with edge to black z <= x-1. Since we are at x, path forward can reach y if y>=x. y > x-1 => y>=x. Good. Then jump to z < x. Repeat. Terminate at1. Good.

Now, let's consider if C_k counts edges from white >k to black <=k, but if the white y is not reachable from x because there is a gap? Path edges go through all vertices, no obstacles. Reachable. Good.

Now, let's think about if C_k>0 for all k can be checked at block ends. For cut inside block after B_b before B_{b+1}, prefix black b, suffix whites after cut. As cut moves right, suffix loses whites, so C nonincreasing. Minimum at end before B_{b+1}. For cut after B_{b+1}? That's start of next block, prefix black b+1, suffix loses black? C can increase because black prefix increases (more target blacks) and suffix same? The minimum in each block is before next B. Also cut after last B, suffix whites group N, C nonincreasing as cut moves through whites, minimum before last W, which is >0 because last W has an edge to some black (all black). This corresponds to b=N? Not in bad prefixes. Good.

Now, let's think about if bad prefix b condition exactly C before B_{b+1}=0. C before B_{b+1} counts edges from whites after B_b (groups >=b+1) to black <=b. If zero, all those whites matched to black >b. Since total whites groups >=b+1 = N-e_b, black >b = N-b. If e_b<b, impossible by pigeonhole, so C>0 automatic. If e_b>=b, possible. Bad prefix b in matching: first b black matched to first e_b white. Equivalent? If all whites groups >=b+1 matched to black >b, then no outside white to prefix black, so prefix black matched to prefix white. Conversely if prefix black matched to prefix white, outside whites matched to outside blacks (since prefix black all used), so C=0. Good.

Now, let's consider if first bad prefix b=N always: prefix all black matched to all white, C before B_{N+1} nonexistent. We use as base.

Now, let's think about if H_b recurrence total injections P(e_b,b) = e_b!/(e_b-b)! only valid if e_b>=b. We skip invalid. Good.

Now, let's consider if H_b for b valid but e_b=e_{b-1} would recurrence maybe produce nonzero? We set zero. If we didn't skip, recurrence might produce something but first bad impossible. Good.

Now, let's think about if compressed valid list skips invalid, but recurrence for later valid b sums over c<b all H_c, invalid zero. Good.

Now, let's consider if `e[b] > e[b-1]` condition for b=N with e[N]=N and e[N-1] maybe N? If last char W, e[N-1] <=N-1. Good.

Now, let's think about if there is a case S1=B,S2N=W but e[N-1]=N? That would mean all N whites before last B, but last char W after last B, impossible because would be N+1 whites. Good.

Now, let's consider if `e` scan for last B sets e[N-1]=w before last B. If last char W, w before last B <=N-1. Good.

Now, let's think about if `fact` and `invfact` lists use N+1, but `E[i]-B[j]` can be N? If B[j]=0? no. Max E=N, B[j]=1 => N-1. Denom E-B[i] max N-1. fact[N] used for val when E=N. OK.

Now, let's consider if in NTT F slice t_max can be N? If c0=0 no, but if c0=1, t_max=N-1. fact[N] not in F except maybe if c0=0. OK.

Now, let's think about if `fact_l[t_min:t_max + 1]` with t_max=N-1 length N-t_min. OK.

Now, let's consider if in direct branch, `fact_l[E_l[i] - bj]` index could be N? If bj=0 no. OK.

Now, let's think about if `B` list original black indices start at 1. Good.

Now, let's consider if `E` list e values start at maybe 0 for first valid? If first valid b>1, E could be 0? Valid requires e[b]>=b>1, so E>=2. If b=1, e1 could be0? valid requires e1>=1, so E>=1. So E[i]>=B[i]>=1. Good.

Now, let's think about if `val[i]=fact[E[i]]` for E[i]=0? Not for valid. But if M=0 no. OK.

Now, let's consider if in naive, `s=fact[ei]` with ei maybe0? no.

Now, let's think about if `H` values can be large but mod.

Now, let's consider if final answer for sample3 with code naive (M=6) would compute 240792. We manually did. Good.

Now, let's think about if sample1 M=2 naive. Good.

Now, let's consider if sample2 early exit. Good.

Now, let's think about if there are hidden tests with N=200000 alternating BWBW... (S1=B,S2N=W). M=N, e_b=b. Recurrence H_b for alternating. CDQ dense. This is worst-case. Need ensure performance.

For alternating, B[i]=i, E[i]=i. Valid all. Recurrence H_i = (i! - sum_{j<i} H_j (i-j)!). This is convolution over index. CDQ NTT ranges: For node left [l,m], right [m+1,r], c0=l? first nonzero maybe l if H nonzero. c1=m. L=m-l+1. t_min=E[ql]-c1 = (m+1)-m=1. t_max=E[qr]-c0 = r-l. Mlen=r-l. need=L+Mlen-1 = (m-l+1)+(r-l)-1 = r-l+1? Actually segment size S=r-l+1, left size S/2, right S/2, L=S/2, Mlen=S, need=1.5S? n power ~2S. NTT cost per node ~2S log(2S). Sum per level ~N log N. OK.

H values for alternating: H1=1,H2=1,H3=3,H4=13,H5=71? Nonzero likely. Dense.

Now, let's think about if in alternating, t_min=1, F starts fact[1], not fact[0]. Good. Query idx = E[i]-c0-t_min = i-l-1. For first right i=m+1, idx=0? Wait c0=l, t_min=(m+1)-m=1, idx=m+1-l-1=m-l = L-1? If l=1,m=2,ql=3, idx=3-1-1=1, L=2, L-1=1. Good. F index for c=m (i=1) t=1 j=0, conv index1. OK.

Now, let's consider if fact[0] never used in convolution for alternating? t_min=1. But fact[0] might be used if c1=E[ql]? c1=m, E[ql]=m+1, t_min=1. No. In general t_min can be0 if c1=E[ql]. But c1 < B[ql] <=E[ql], equality possible if B[ql]=E[ql] and c1=B[ql]-1? Then t_min=1. If c1 could equal E[ql]? c1 is black index of a previous valid c, c1 < B[ql]. If B[ql]=E[ql], c1<=E[ql]-1, t_min>=1. If B[ql]<E[ql], c1 could equal E[ql]? c1 < B[ql] < E[ql], no. So t_min>=1 always? Since c1 < B[ql] <= E[ql], c1 <= E[ql]-1, t_min>=1. So fact[0] not used in contributions (c<b, e_b-c>=1). Good. t_min max0 unnecessary but safe.

Now, let's think about if `fact[0]` used in denom invfact[0]. Yes.

Now, let's consider if in NTT F slice starts at 1 usually, length Mlen. OK.

Now, let's think about if `A` scaling by inv_n and F fact[1..] yields conv. Good.

Now, let's consider if in direct branch, `fact_l[ei - bj]` index at least1. OK.

Now, let's think about if `val[i]` initial fact[E[i]] can be large mod. OK.

Now, let's consider if in CDQ, contributions from c to i where E[i]-B[c] maybe >N? no.

Now, let's think about if there is any issue with using `bisect_right` on `nonzero` while it is being appended in solve leaf after apply? No.

Now, let's consider if `nonzero` list could be used in apply before all H in left half computed? apply called after solve left. Good.

Now, let's think about if `solve` leaf computes H for i in [l,r] but `val[i]` may not include contributions from c in [l,i-1] that are in nonzero list? We sum directly. Good.

Now, let's consider if in leaf, for i, `val[i]` may include contributions from c in [l,i-1] if an ancestor apply processed them? As argued no because same segment. But what if leaf is right child of an internal node, and c in [l,i-1] where l is left boundary of leaf, some c may be in left half of an ancestor? No, leaf is a segment; all indices in leaf are in same half at every ancestor until leaf. Ancestor apply only crosses between left and right halves of that ancestor. If both c and i are in leaf, they are in same half at all ancestors, so no apply. Good.

Now, let's think about if `val[i]` for i in leaf includes contributions from c < l via all ancestor applies where i in right half and c in left half. Yes.

Now, let's consider if `val[i]` for i in leaf includes contributions from c in [l,i-1] via apply within a larger segment if leaf was right half of that segment? Wait leaf [l,r] could be right half of its parent [L,R]. Then c in [l,i-1] are in same right half, parent apply processed c in [L,mid] to i in [mid+1,R], not c in right half. So no. Good.

Now, let's think about if CDQ order with leaf threshold: For segment size <=LEAF, we don't recursively apply within it, but direct sum handles all internal pairs. Good.

Now, let's consider if direct sum in leaf should include contributions from c in [l,i-1] even if H[c]=0? skip. Good.

Now, let's think about if `H[c]` for c in [l,i-1] is final modulo. Yes computed earlier in leaf.

Now, let's consider if in leaf, `val[i]` might be negative? We keep val mod after applies. Initial mod. Direct apply mods. NTT apply mods. So val positive. s positive minus terms. OK.

Now, let's think about if in apply direct, after updating val[i] with modulo, val[i] positive. Good.

Now, let's consider if in apply NTT, after updating val[i] with modulo, positive. Good.

Now, let's think about if in NTT branch, `A` after intt contains conv mod, but due to scaling A by inv_n, conv values are mod. Good.

Now, let's consider if `A[idx]` could be negative? no.

Now, let's think about if `val_l[i] - A[idx]` negative, `% mod`. OK.

Now, let's consider if in `ntt`, after forward, values mod. In pointwise, product mod. In inverse, values mod. Good.

Now, let's think about if `intt` without scaling but with A scaled by inv_n: The inverse butterfly includes multiplications by inverse roots but not by inv_n. It produces n times the inverse DFT. Since forward DFT is unnormalized, inverse unnormalized gives n*original. We scaled original by inv_n, so result original. Good.

Now, let's consider if `inv_n` should be inverse of n modulo, not of 2^h? n=2^h. inv_pow2[h]. Good.

Now, let's think about if `max_log` maybe 0 if max_n=1, inv_pow2 length1. N>=1 max_n>=8. OK.

Now, let's consider if `rate_tz` precompute uses `rate2[tz]` where tz maybe 0 for s=0,2,6,... Good.

Now, let's think about if ACL butterfly requires `rate2` precomputed up to `h` but we use `rate_tz` with tz. Good.

Now, let's consider if there is a bug in `rate_tz` for s where s+1 has trailing zeros >29? Not with constraints.

Now, let's think about if `pow(G, (MOD - 1) >> (i + 2), MOD)` for i=0 gives G^{(MOD-1)/4}, which is primitive 4th root. For n=2 forward, ph=1, now=1, no rate used. For n=4, ph=2 s=1 uses rate2[0]. Good.

Now, let's consider if inverse `irate2[0]` inverse of primitive 4th root. Good.

Now, let's think about if in `intt`, for ph=h down to1, `inow` update uses `itz[s]`. For n=4, ph=2 s=0 inow1, update irate2[0]; s=1 uses inverse w4. Good.

Now, let's consider if in `intt`, `a[i] = x; a[j] = y * inow % mod` for inow==1 branch assigns y. Good.

Now, let's think about if in `ntt`, `now` update after s uses `rtz[s]`, but for s where `now` was 1 branch, still update. Good.

Now, let's consider if in `ntt`, for ph=1, w=1, s=0, now=1 branch, update now=rate2[0] not used. OK.

Now, let's think about if in `ntt`, for ph=2, w=2, s=0 now1 branch, update now=rate2[0]; s=1 now!=1 branch. Good.

Now, let's consider if in `intt`, for ph=1, w=1, inow1 branch, update not used. OK.

Now, let's think about if `rate_tz` and `irate_tz` lists are captured in ntt/intt. They are local in main, closure. We assign `rtz=rate_tz` local at function start. Good.

Now, let's consider if `MOD` global lookup in ntt? We assign `mod=MOD`. Good.

Now, let's think about if `fact_l` in apply is local, but `fact` list may be large. OK.

Now, let's consider if in apply direct, `for i in range(ql, qr + 1):` and inner `for k in range(nz):` if nz=0? We returned if lo==hi. nz>0.

Now, let's think about if in apply direct nz==1, we don't create lists. Good.

Now, let's consider if in apply direct nz>1, `hj_list` and `bj_list` values are Python ints. OK.

Now, let's think about if in apply NTT, `A` fill loop uses `H_l[c] * inv_n % mod`. If `H_l[c]` is 0? c from nonzero, no. But if H became 0 due to mod? nonzero only nonzero. OK.

Now, let's consider if in apply NTT, `F` slice assignment with `fact_l[t_min:t_max + 1]` where t_min maybe 0, t_max maybe N-1. OK.

Now, let's think about if `F` list initially zeros, slice assignment length Mlen. If Mlen < n, rest zeros. If Mlen == n, ok. If Mlen > n (shouldn't), slice assignment would resize list? In Python, assigning a slice of different length can change list length. But Mlen <= n. Need ensure. n >= L+Mlen-1 >= Mlen. OK.

Now, let's consider if `A` list length n, fill indices <L<=n. OK.

Now, let's think about if `ntt(A)` modifies A in place. Good.

Now, let's consider if `del F` before `intt(A)` might free memory but in CPython yes. In PyPy maybe not immediate, but ok.

Now, let's think about if after `intt(A)`, A values are conv but maybe need to trim? We query indices. OK.

Now, let's consider if `base_idx = c0 + t_min`, query idx = E_l[i] - base_idx. Since E_l[i] up to N, base_idx up to N, idx nonnegative. Good.

Now, let's think about if `idx` could be >= len(A) due to t_max? For last i, idx=Mlen-1 < n. For first, L-1 < n. Good.

Now, let's consider if in NTT branch, `L,t_min,t_max,Mlen,n` variables are from else. If `direct_cost > DIRECT_ALWAYS` but `use_direct` true, we return. If false, else executed. Good.

Now, let's think about if Python's scoping for variables assigned in else and used later in same function is fine. Yes.

Now, let's consider if `ntt_cost = n * (n.bit_length() - 1)` for n=1 gives0, `use_direct = direct_cost <= 0` false if direct_cost>0. But direct_cost > DIRECT_ALWAYS (4096) impossible with n=1? Let's prove: n=1 => need=1 => L=Mlen=1. L=1 => c0=c1, nz=1. Mlen=1 => t_max=t_min. Mlen = E[qr]-c0 - (E[ql]-c1)+1 = E[qr]-E[ql]+1 (since c0=c1) =1 => E[qr]=E[ql]. But E strictly increasing for valid right segment, so rb=1. direct_cost=1*1=1 <=4096. So direct. OK.

Now, let's think about if `E` strictly increasing for valid list. Yes. If right segment rb>1, E[qr]>E[ql], Mlen>=L? OK.

Now, let's consider if `B` strictly increasing for valid list. Yes.

Now, let's think about if `nonzero` list indices are compressed, but B values strictly increasing. Good.

Now, let's consider if in apply, `c0 = B_l[nz_list[lo]]`, `c1 = B_l[nz_list[hi - 1]]`. If nonzero list has indices not contiguous, c0/c1 min/max. Good.

Now, let's think about if `L = c1 - c0 + 1` could be much larger than segment size due to invalid gaps. NTT cost accounts. Direct cost uses nz*rb, maybe direct chosen if nz small. Good.

Now, let's consider if in direct branch with large B gaps, `fact_l[ei - bj_list[k]]` index maybe large but within N. OK.

Now, let's think about if in NTT branch with large B gaps, A length large with zeros. NTT cost high, but direct may be chosen if nz small. If nz large but gaps large, L large, NTT high; direct_cost=nz*rb maybe also high. Which is better? If gaps large, nz cannot be too large relative to L? Could be many valid in a sparse range. NTT maybe still better than direct if nz*rb huge. OK.

Now, let's consider if there is a risk of TLE due to many NTT with large L due to gaps. Worst-case valid indices sparse but many? M<=N. If gaps large, M smaller. CDQ over M, L ranges over original B gaps. At a node, L could be as large as N even if nz small. But if nz small, direct chosen. If nz large, gaps not too large on average. Total NTT work maybe bounded by O(N log^2 N) with original coordinate length? Could be worse if valid indices are every other (gaps 1) L~nz. If valid indices are clustered with large gaps, M smaller. OK.

Now, let's think about if `max_n` based on N, not M, so NTT arrays can be large even if M small but gaps large. If M just above 2000 and B range N=200000 with large gaps, root NTT n maybe 524288, but direct_cost maybe nz*rb ~1e6, ntt_cost ~9.8M, factor4 =>39M, direct chosen (1e6). So no huge NTT. If M=100000 with large gaps? B range N=200000, gaps average2, L~2nz. OK.

Now, let's consider if direct_cost threshold could choose direct for a node with nz=1000,rb=1000 (1e6) and n=524288 (ntt_cost9.8M*4=39M), direct 1e6, good. If many such nodes, total direct maybe? Each pair processed once, direct pairs where nz*rb <=4*nlog. Could be O(N log N * something). Probably ok.

Now, let's think about if we should increase `COST_FACTOR` to 8 to direct more in sparse large n cases. It would also direct dense length512? For dense length512, ntt_cost4608*8=36864, direct65536 -> NTT still. length256: 2048*8=16384, direct16384 -> direct (<=). So factor8 directs length256. Factor4 NTT length256. Which is better? Hard. Maybe factor8 safer for CPython (less NTT). But direct 12.8M extra. I think NTT length256 in PyPy maybe okay. Keep factor4.

Now, let's consider if `DIRECT_ALWAYS=4096` and factor4 means length128 direct, length256 NTT. If we set factor8, length256 direct. Maybe choose factor6? length256: 2048*6=12288 <16384 NTT. factor8 direct. Let's keep 4.

Now, let's think about if `LEAF=64` and `DIRECT_ALWAYS=4096` means apply size128 direct exactly 4096. If H dense. If H sparse, direct. Good.

Now, let's consider if in apply size128 direct, `nz` could be 64, rb64, direct_cost4096. Inner loops 4096 per node, nodes1562 =>6.4M. OK.

Now, let's think about if in leaf size64, internal pairs 2016 per leaf, nodes3125 =>6.3M. Total direct 12.7M. Good.

Now, let's consider if NTT levels from size256: number nodes781, n maybe512? For alternating, segment size256, L=128,Mlen=256,need383,n512. ntt_cost estimate 512*9=4608, but actual n=512. 781 nodes. Heavy. OK.

Now, let's think about if we can reduce NTT length for alternating by using t_min=1, Mlen=256, L=128, need383, n512. Could we choose n=256? Need linear convolution length383, n must >=383 to avoid wrap. But we only query indices 127..255. Could use n=256 with cyclic wrap? High indices 256..382 would wrap to 0..126, not queried? We query 127..255, wrap from high indices to low 0..126, so safe! In general, if we only query indices [L-1, Mlen-1], we might choose n > Mlen-1 and n > (L+Mlen-2) - (L-1)? Need avoid wrap into queried range. Cyclic convolution index k mod n. A high index h wraps to h-n. We need no h in [n, L+Mlen-2] such that h-n in [L-1, Mlen-1]. Equivalent n > max_query - min_high? Let's derive. Queried Q=[L-1, Mlen-1]. High indices H=[n, L+Mlen-2]. Wrap to [0, L+Mlen-2-n]. To avoid intersect Q, need L+Mlen-2-n < L-1 => n > Mlen-1. Or if wrap range starts > Mlen-1: n <=? Actually if n <= Mlen-1, wrap range [0, L+Mlen-2-n] may include Q. Condition to avoid: L+Mlen-2-n < L-1 => n > Mlen-1. So n >= Mlen. For alternating size256, Mlen=256, n=256 would suffice! We chose n=512 because full need383. This could halve NTT size for many cases! Great. We only need a "middle product" band from L-1 to Mlen-1. We can choose n as power of two >= Mlen (not L+Mlen-1) provided n > Mlen-1, i.e. n >= Mlen. But also need n >=? To compute convolution indices up to Mlen-1 without wrap from high indices, condition n >= Mlen is enough? Let's verify with small: L=4,Mlen=8, need11, query 3..7. Choose n=8 (>=Mlen). High indices 8..10 wrap to0..2, not queried. Good. Choose n=8. If Mlen=4,L=8, query7..3? Wait query range L-1=7 to Mlen-1=3, invalid if L-1 > Mlen-1. But we argued Mlen-1 >= L-1 + rb-1? Actually Mlen-1 = (E[qr]-E[ql]) + (c1-c0) >= (rb-1)+(L-1). So Mlen-1 >= L-1. Thus query range valid. If L=8,Mlen=8, query7..7, n=8, high indices8..14 wrap0..6, not queried. Good. If L=1,Mlen=100, query0..99, n=128? Need n>=Mlen=100, high indices100..100? need=100, n=128, no high. If n=100 not power. Power >=100. Good.

Thus we can set `n = 1 << (Mlen - 1).bit_length()` (power of two >= Mlen) instead of need=L+Mlen-1, as long as Mlen >= L? Is Mlen always >= L? Mlen-1 = (E[qr]-E[ql]) + (c1-c0). E[qr]-E[ql] >= rb-1 (valid E strictly increasing). c1-c0 = L-1. So Mlen-1 >= L-1 + rb-1 >= L-1. Thus Mlen >= L. Great! Because right segment has at least one valid b, rb>=1. So Mlen >= L. Therefore n >= Mlen >= L. And condition n >= Mlen avoids wrap into query range. This can significantly reduce NTT length: from L+Mlen to Mlen. In alternating, Mlen=segment size, L=half, n=segment size instead of 2*segment size. Halves NTT work! Excellent.

Let's verify with general: A length L, F length Mlen, Mlen>=L. We need convolution coefficients k in [L-1, Mlen-1]. If we do cyclic convolution modulo x^n-1 with n power of two >= Mlen, then for any true coefficient k in [0, L+Mlen-2], it contributes to cyclic index k mod n. We need for queried k0 in [L-1,Mlen-1], no other true coefficient k1 = k0 + t*n (t>=1) exists. Max true k = L+Mlen-2. Since n >= Mlen, k0 + n >= L-1 + Mlen. Max true k = L+Mlen-2. Compare L-1+Mlen <= L+Mlen-1. For n=Mlen, k0+n min = L-1+Mlen. Max true = L+Mlen-2. If L-1+Mlen > L+Mlen-2? L+Mlen-1 > L+Mlen-2, yes. So no wrap. For n>Mlen also. Good. Thus n >= Mlen sufficient.

This is a major optimization. We should use `n = 1 << (Mlen - 1).bit_length()` (if Mlen=1, n=1). But need also n >= L? Mlen>=L, so ok. Let's prove Mlen>=L with t_min maybe max(0). If t_min set0 instead of e_first-c1 (which is >=1), Mlen = t_max - 0 +1 = E[qr]-c0+1. L=c1-c0+1. Need Mlen>=L? E[qr]-c0+1 >= c1-c0+1 => E[qr]>=c1. True since c1 < B[ql] <= E[ql] <= E[qr]. So even with max0, Mlen>=L. Good.

Thus NTT length can be power of two >= Mlen, not L+Mlen-1. This reduces max n: Mlen <= t_max-t_min+1 <= (E[qr]-c0) - (E[ql]-c1)+1 <= N + L? But Mlen can be up to N+L? Wait t_max=E[qr]-c0 <=N-1, t_min>=0, Mlen<=N. Actually t_min>=0, t_max<=N-1, so Mlen<=N. Previously need<=2N. Now n<=next pow2(N) = 262144 for N=2e5, instead of 524288. Great! Max NTT length halved. Total work significantly reduced.

Let's update apply:
```
Mlen = t_max - t_min + 1
# Mlen >= L
n = 1 << (Mlen - 1).bit_length()
```
No need `need`. But ensure if Mlen=0 no. Mlen>=1.

Now, `ntt_cost = n * (n.bit_length() - 1)` with n smaller. Direct/NTT threshold changes: NTT more attractive. For alternating size256, Mlen=256,n=256, ntt_cost2048, direct16384, factor4 =>8192, NTT. Same. For root, Mlen~100k? n=131072 instead of262144, work half. Great.

Need adjust `max_n` precompute: max Mlen <= N? t_min>=0,t_max<=N-1, so Mlen<=N. But if t_min=0,t_max=N? c0=0 no. Safe max_n while < N+5 maybe. But to be safe use 2*N+5 still, or N+5. Since n power >=Mlen<=N, max_n next pow2(N). Use `while max_n < N + 5`. For N=200000, max_n=262144. rate_tz half=131072. Good. But if t_max=N (if c0=0) Mlen=N+1, not possible. Use 2*N+5 safe but precompute larger half 262144, still ok. To save memory/time, use `while max_n < N + 5`. Need be certain Mlen<=N. t_min = max(0, E[ql]-c1). Since c1>=1, E[ql]<=N, t_min<=N-1. t_max=E[qr]-c0 <= N-1. Mlen <= N - t_min <= N. If t_min=0, t_max<=N-1 => Mlen<=N. Yes. So max_n next pow2(N) enough. For N=1, max_n while <6 =>8. OK.

But wait, if t_min set0 and t_max=N? c0 could be0? B indices start1, so no. Good.

Let's update max_n to `while max_n < N + 5`. This reduces rate_tz precompute and memory. But if Mlen=N and N is power of two? Need n >= Mlen. If N=262144? Constraints 2e5, but if N=262144, N+5 next pow 524288? Actually while max_n < N+5, if N=262144, max_n becomes524288, but Mlen<=N=262144, n could be262144. We overallocate by factor2. Could use `while max_n < N: max_n <<=1; if max_n < N: ...` Simpler: `max_n = 1 << (N - 1).bit_length()` (power of two >=N). For N=1, (0).bit_length=0, max_n=1. But NTT n maybe1. rate_tz half=0? Need handle. Use `max_n = 1 << (N.bit_length() - 1)`? Let's compute: power of two >= N: `1 << (N - 1).bit_length()` for N>0. For N=1, (0).bit_length=0 =>1. For N=200000, (199999).bit_length=18? 2^17=131072,2^18=262144, bit_length of199999 is18, 1<<18=262144. Good. Use that. If N=0 not.

But if Mlen can be N+1? no. Use max_n = 1 << (N - 1).bit_length(). For N=1, max_n=1, half=0, rate_tz length1. ntt with n=1 h=0 no rtz access. Good. But if Mlen=1, n=1. OK.

However, in apply, `n = 1 << (Mlen - 1).bit_length()`. If Mlen=1, n=1. Good.

Precompute rate_tz length half+1 where half=max_n//2. If max_n=1, half=0, rate_tz=[1]. ntt n=1 no access. If n=2, half=1, s max0, rate_tz[0] exists. Good.

Let's update max_n precompute to power of two >= N. But wait, Mlen can be N? If N=200000, max_n=262144. Good. If N is power of two, e.g. N=131072, max_n=131072. Mlen<=N, n can be131072. half=65536. In ntt n=131072, ph=17, w=65536, s max65535, rate_tz length65537, ok.

Now, with n smaller, `inv_pow2` max_log = max_n.bit_length()-1. For max_n=1, max_log=0. Good.

Now, update `ntt_cost` estimate: n smaller, NTT more likely. For dense length256, Mlen=256,n=256, same as before? Previously n=512, now256, ntt_cost2048 vs4608. Factor4 threshold8192, direct16384 NTT. For length128, Mlen=128,n=128, ntt_cost1024, factor4=4096, direct4096 direct (<=). Good. For length512, Mlen=512,n=512, ntt_cost4608, factor4=18432, direct65536 NTT. Good.

Root alternating M=200k: root left100k,right100k, Mlen? E[qr]-E[ql] ~100k, c1-c0~100k, Mlen~200k? Wait for alternating, c0=l, c1=m, E[ql]=m+1, E[qr]=r. t_min=1, t_max=r-l, Mlen=r-l = segment size S. For root S=200k, Mlen=200k, n=262144. Previously need=300k,n=524288. Halved. Good.

Now, total NTT work roughly half. Great.

Need ensure cyclic convolution with n>=Mlen but A length L may be equal Mlen. If n=Mlen, high true coefficients from L+Mlen-2 down to n wrap to 0..L-2. Query range L-1..Mlen-1, safe. Good.

Let's update apply code:
```
Mlen = t_max - t_min + 1
# Mlen >= L, but just in case:
if Mlen < L: Mlen = L  # not needed, but if due to max0? We proved. Could set n based on max(Mlen,L) to be safe.
n = 1 << (Mlen - 1).bit_length()
```
If we set Mlen = max(Mlen, L), but F length must be Mlen for fact range. If L > Mlen (shouldn't), we would need F length at least L? Actually F length is fact range length; if L>Mlen, query range L-1..Mlen-1 invalid. But impossible. For safety, we can compute `n = 1 << (max(Mlen, L) - 1).bit_length()` but F length remains Mlen. If L>Mlen, n>=L, but query range? Not needed. To be safe, use `n = 1 << (Mlen - 1).bit_length()` and assert Mlen>=L? No asserts. Maybe use `if Mlen < L: Mlen = L` would extend F with zeros beyond fact range, but t_max would need extend? If L>Mlen, fact range insufficient for some c? But impossible. We can leave.

Now, `ntt_cost = n * (n.bit_length() - 1)`.

Now, `max_n` precompute power >= N. But if Mlen can be N and N=1, max_n=1. rate_tz half0. ntt n=1 no access. If n=2, max_n must be >=2. For N=2, max_n=2? `1 << (N-1).bit_length()` =1<<1=2. half=1, rate_tz length2. ntt n=2 ph=1 w=1 s=0 rtz[0] exists. Good.

Now, let's think about if `Mlen` could be 0? no.

Now, let's consider if `F[:Mlen] = fact_l[t_min:t_max + 1]` with Mlen maybe less than L? no.

Now, let's think about if `A` length n, but L could be >n if Mlen<L. impossible. To be safe, we can set `n = 1 << (max(Mlen, L) - 1).bit_length()` and if Mlen<L, extend F with zeros? But F length Mlen, slice assignment ok, rest zeros. If L>n? n>=L. Query range? If L>Mlen, first query idx L-1 may be >=n? n>=L so ok, but convolution true length L+Mlen-1, cyclic wrap? Query range L-1..Mlen-1 invalid (start>end). Not possible. We can ignore.

Now, let's update `max_n` precompute in code:
```
max_n = 1 << (N - 1).bit_length()
```
For N=1, max_n=1. But `rate_tz` half=0, loop range(0) none. `inv_pow2` length max_log+1 where max_log=0. Good.

But in apply, `n` could be 1 << (Mlen-1).bit_length(). If Mlen=N=1, n=1. If Mlen=2, n=2. max_n>=N>=Mlen, so n<=max_n. Good.

Now, let's think about if `rate_tz` length half+1 where half=max_n//2. If max_n=1, half=0. If n=1, no access. If n=2, max_n>=2, half>=1, access s=0. Good.

Now, let's consider if `max_n` power >=N but Mlen could be N+1 if t_min=0,t_max=N? We argued no. But to be ultra safe, use `max_n = 1 << (N).bit_length()` (power > N unless N power? For N=200000, 262144; for N=262144, 524288). This overallocates if N power. Memory still okay (half 262144 for N=262144, but constraints 2e5). Use `while max_n < N + 1` maybe. Simpler: `max_n = 1; while max_n < N + 1: max_n <<= 1`. For N=200000, 262144. For N=1, max_n=2 (over). Fine. This ensures Mlen<=N+1. Let's use `while max_n < N + 1` to be safe. For N=200000, same. For N=1, max_n=2, half1, ok. Slight over. Good.

Now, let's think about if `ntt` with n=2 and max_n=2, half1, rate_tz[0] exists. Good.

Now, let's update code accordingly.

Now, let's reconsider NTT length with n>=Mlen. We scale A by inv_n. Inverse unscaled gives n*conv. Since n may be less than full convolution length but no wrap into queried range. The cyclic convolution modulo x^n-1 equals linear convolution for queried indices. Good. We don't care about other indices. So correct.

Need maybe mention in notes: use cyclic NTT length power of two >= Mlen because only diagonal band queried and Mlen>=L, avoiding wrap.

Now, let's think about if `Mlen>=L` proof relies on E strictly increasing over right segment. If right segment rb=1, Mlen-1 = E[ql]-c0 - (E[ql]-c1) = c1-c0 = L-1, so Mlen=L. Good. If rb>1, E[qr]-E[ql]>=rb-1>=1, Mlen>L. Good.

Now, let's consider if t_min max0 could make Mlen larger, still >=L. Good.

Now, let's think about if `F` length Mlen but n maybe equal Mlen. If Mlen not power of two, n next power. Good.

Now, let's consider if `ntt_cost` estimate should use n (power) and log. Good.

Now, let's think about if direct/NTT threshold with smaller n may choose NTT for length128? direct4096, n=128, ntt_cost1024*4=4096, direct (<=). If factor4, direct. If we want NTT length128, factor3. Keep direct.

Now, let's consider if with n smaller, root NTT length 262144, max_log18. Precompute rate_tz half131072. Good.

Now, let's think about if `max_n` while < N+1 for N=200000 gives 262144, max_log18. But n could be 262144, bit_length19? 262144=2^18, bit_length=19, max_log = max_n.bit_length()-1 =18. inv_pow2 length19 indices0..18. n.bit_length()-1=18. Good.

Now, let's consider if `rate_tz` half=131072, for n=262144, ph=18, w=131072, s max131071, rate_tz length131073, ok.

Now, let's think about if `rate2` precompute 30 enough for max_log18. yes.

Now, let's consider if in `ntt`, `h = n.bit_length() - 1`. For n=262144, h=18. ph range1..18. Good.

Now, let's think about if `shift = h - ph + 1`. For ph=18, shift=1. Good.

Now, let's consider if `p = 1 << (h - ph)`. For ph=18, p=1. Good.

Now, let's think about if `now` update uses `rtz[s]` where s up to w-1=131071, rate_tz length131073. Good.

Now, let's consider if `rate_tz` precompute loop `for s in range(half):` with half=131072, s max131071. Good.

Now, let's think about if `max_n` over N+1 but Mlen maybe N+1? If N=1, max_n=2, Mlen<=1. OK.

Now, let's consider if in apply, `n = 1 << (Mlen - 1).bit_length()`. If Mlen=0, error. no.

Now, let's think about if `Mlen` can be larger than max_n due to t_min negative set0 and t_max=N? no. But if it is, n>max_n and rate_tz index out of range. To be safe, we could precompute max_n while < 2*N+5 as before, but with n>=Mlen<=N+1, N+1 enough. Use `while max_n < N + 2` maybe. Let's use `while max_n < N + 5` to be safe, still 262144 for 2e5. If N=262144, N+5 ->524288, over but constraints not. OK. Use N+5 for safety. Memory half 262144 for N=2e5? Wait N+5=200005, next pow 262144. half131072. Good. If N=262144, N+5 next pow524288, half262144, memory ok maybe. Constraints 2e5. Use N+5.

Now, let's think about if `max_n` with N+5 for N=1 gives8? 1+5=6, next pow8. Over but fine. half4. rate_tz length5. ntt n maybe1 or2. OK.

Now, let's consider if overprecomputing rate_tz for N=1 small. OK.

Now, let's update code max_n while < N + 5.

Now, let's think about if `ntt_cost` with n smaller may make NTT chosen for more nodes, increasing number of NTT calls. For length128 direct still. For length256 NTT. Number NTT nodes same. Work half. Good.

Now, let's consider if we can further reduce NTT length to power >= max(Mlen, L) but Mlen>=L. OK.

Now, let's think about if we can reduce F length to only needed t range for queried band? We already t_min..t_max. Could t_min be larger? For queried band k in [L-1,Mlen-1], F indices j = k-i. For i 0..L-1, j ranges from k-(L-1) to k. For k min L-1, j min0, max L-1. For k max Mlen-1, j min Mlen-L, max Mlen-1. Thus needed F indices 0..Mlen-1 all. A indices all 0..L-1. OK.

Now, let's consider if we can reduce A length to L? yes.

Now, let's think about if cyclic convolution with n>=Mlen but A length L, F length Mlen, true coefficients up to L+Mlen-2. High coefficients wrap to 0..L-2. Query starts L-1. Safe. Good.

Now, let's consider if `Mlen` could be much larger than n? n power >=Mlen. OK.

Now, let's think about if in NTT branch, `F[:Mlen] = fact_l[t_min:t_max + 1]` and n maybe >Mlen. Rest zeros. Good.

Now, let's consider if `A` fill indices up to L-1, and L<=Mlen<=n. Good.

Now, let's think about if `base_idx = c0 + t_min`, query idx = E[i]-base_idx. For first i, idx=L-1. For last, idx=Mlen-1. Since n>=Mlen, idx<n. Good.

Now, let's consider if `E[i]-base_idx` could be negative for first? L-1>=0. Good.

Now, let's think about if `t_min` max0 but actual min t positive, query first idx = E[ql]-c0-t_min. If t_min=0 but actual min t = E[ql]-c1 >0, then first idx = E[ql]-c0 = c1-c0 + (E[ql]-c1) = L-1 + actual_min. This is >L-1. But convolution with F starting at0: For first b, terms c have t=e-c. The smallest t is actual_min>0, F index actual_min. For c=c1 (i=L-1), j=actual_min, k=L-1+actual_min. Query idx matches. F indices 0..actual_min-1 unused. High wrap? n>=Mlen where Mlen=t_max+1 (since t_min0). True max coefficient L+Mlen-2. Query range starts L-1+actual_min, ends Mlen-1. High wrap to 0..L+Mlen-2-n. Need ensure no wrap into query. Condition n > max_query - min_high? With n>=Mlen, min high wrap 0, max wrap L+Mlen-2-n. Query min = L-1+actual_min. Need L+Mlen-2-n < L-1+actual_min => n > Mlen-1-actual_min. Since n>=Mlen, true. So safe. Good.

Now, let's think about if `Mlen` with t_min0 can be larger than needed, n maybe larger, but safe.

Now, let's consider if we can set t_min = E[ql] - c1 (not max0) to reduce Mlen. Since it's always >=1, we can use that directly. But if due to some edge c1>E[ql] (impossible), negative. We can keep max0. It may increase Mlen if actual min negative (impossible). No issue.

Now, let's think about if `t_min = E_l[ql] - c1` could be 0? If c1=E[ql], impossible as c1<B[ql]<=E[ql], but if B[ql]=E[ql]+? no. So >=1. OK.

Now, let's consider if `t_max = E_l[qr] - c0` could be N? c0=0 no. OK.

Now, let's think about if `Mlen` maybe 1 and n=1, `ntt` h=0, `intt` h=0. Pointwise loop range1. Query idx? L=1, t_min=t_max, base_idx=c0+t_min, E[ql]-base_idx=0. Good.

Now, let's consider if in apply NTT branch, `ntt_cost = n * (n.bit_length() - 1)`. For n=1, 0. But direct likely. OK.

Now, let's think about if `COST_FACTOR` should be adjusted due to smaller n. Maybe factor4 still. For length128 direct4096, ntt_cost1024*4=4096 direct. For length256 NTT. Good.

Now, let's consider if with n smaller, NTT length256 work: 3 transforms *256/2*8=3072 butterflies, pointwise256, total3328 per node, nodes781 =>2.6M. Direct length256 12.8M. NTT likely better. Good.

Now, let's think about if we can set `DIRECT_ALWAYS=2048` to NTT length128? direct4096, ntt_cost1024*4=4096 direct if <=. If factor3, NTT. Keep.

Now, let's consider if `LEAF=64` direct leaf 6.4M, apply128 direct6.4M. Could set LEAF=32 to reduce leaf direct to3.2M, apply64 direct3.2M, apply128 direct6.4M total12.8M same, more calls. Keep64.

Now, let's think about if `LEAF=128` direct leaf12.8M, apply256 NTT, total direct12.8M, fewer calls. Maybe LEAF=128 better. With LEAF=64, direct leaf+apply128=12.8M, plus more function calls. LEAF=128 leaf direct uses branch and H lookup, apply128 direct uses precomputed lists. Which is faster? Apply direct for size128: for each node, create hj/bj lists length64, then for each i 64 inner. Leaf direct for size128: for each i, inner j up to127, with branch. Similar operations. Leaf direct doesn't create lists but does H[j] lookup and branch. Apply direct does H[c] lookup once per c, then inner uses hj_list. Might be faster. But LEAF=128 avoids apply128 and bisect. Hard. Keep LEAF=64 as planned.

Now, let's think about if in leaf direct, for i, inner j from l to i-1, but H[j] may be zero. If dense, branch true. If sparse, skip. OK.

Now, let's consider if in apply direct, c_list nonzero only, so no branch. Good.

Now, let's think about if `nonzero` list in apply for size128 dense has nz=64, rb=64. It creates lists length64. OK.

Now, let's consider if in apply direct, `val_l[i] = s % mod` for each i. If s negative large, modulo. OK.

Now, let's think about if in leaf direct, `hi_val = s % mod * invfact_l[ei - B_l[i]] % mod`. If s negative, modulo. OK.

Now, let's consider if in naive, `s` can be negative large, modulo. OK.

Now, let's think about if we should use `sys.stdout.write` for final print to avoid space? print fine.

Now, let's consider if code length okay.

Now, let's maybe write final code with the optimized n.

Double-check variable names: In apply, `Mlen` local shadows outer `M`? No, outer M valid count. Python local `Mlen` distinct. OK.

Now, let's think about if in apply, after else computing `n`, if `use_direct` false, we use `n`. If `use_direct` true, we return. Good.

Now, let's consider if `ntt_cost` variable maybe not defined if direct_cost <= DIRECT_ALWAYS and use_direct true, but not used. OK.

Now, let's think about if in apply NTT branch, `L` variable defined in else. If direct_cost > DIRECT_ALWAYS, else executed. Good.

Now, let's consider if `Mlen` could be very large and `fact_l[t_min:t_max + 1]` slice length Mlen, but `t_max + 1` maybe > len(fact_l) if t_max=N? fact length N+1, index N+1 slice end ok if t_max=N? slice end N+1 valid. If t_max=N+1, error. Not possible. To be safe, could `t_max = min(t_max, N)`. But not needed. Maybe add `if t_max > N: t_max = N` for safety. And `if t_min < 0: t_min=0`. This won't hurt. If t_max>N, Mlen maybe N-t_min+1. But fact slice end N+1. Let's add safety:
```
if t_max > N: t_max = N
```
But N from outer. In apply local, N closure. Could add. Not necessary but safe. If t_max clipped, Mlen maybe smaller, but if actual needed t>N impossible. OK.

Now, let's think about if `t_min` > `t_max` after clipping? no.

Now, let's consider if `Mlen = t_max - t_min + 1` and if Mlen <=0 return? Could add:
```
if Mlen <= 0: return
```
Not needed.

Now, let's think about if `n = 1 << (Mlen - 1).bit_length()` when Mlen=0 error. Add if Mlen<=0 return for safety.

Now, let's consider if `max_n` precompute while < N+5, but if Mlen clipped to N+1? n maybe next pow >=N+1, max_n >=N+5, ok.

Now, let's think about if in NTT branch, `A` length n, but `L` could be >n if Mlen<L and we didn't adjust. To be safe, set `if Mlen < L: Mlen = L`? But F length would be extended with zeros beyond fact range. If L>Mlen impossible, but for safety we can do:
```
if Mlen < L:
    Mlen = L
```
But then `F[:Mlen] = fact_l[t_min:t_max+1]` length less than Mlen, slice assignment with shorter list will shrink F? Actually if F length n, assigning a slice of length Mlen with a shorter iterable will resize F to n - Mlen + len(iterable), breaking length. Need ensure iterable length Mlen. If we extend Mlen, need fill zeros. Better not. Since impossible, skip.

Could compute `n = 1 << (max(Mlen, L) - 1).bit_length()` without changing Mlen. If L>Mlen, n>=L, F length Mlen, A length L. Cyclic wrap? Query range invalid. Not needed.

Now, let's think about if `Mlen>=L` proof uses `E[qr]-E[ql] >= rb-1`. E strictly increasing for valid indices. Yes. If right segment [ql,qr] all valid. Good.

Now, let's consider if `E` strictly increasing for valid list even if e jumps? Yes, valid condition e[b]>e[b-1], but if there are invalid between, e can stay; for two valid b1<b2, e[b2] > e[b2-1] >= e[b1] (since e nondecreasing), so strict. Good.

Now, let's think about if `B` strictly increasing, yes.

Now, let's consider if `c1-c0 = L-1` could be less than number of nonzero? No, B gaps positive, L-1 >= nz-1. OK.

Now, let's think about if `Mlen-1 = (E[qr]-E[ql]) + (c1-c0)` if t_min=E[ql]-c1. If t_min max0 and actual t_min larger? If t_min set0, Mlen-1 = E[qr]-c0. Need >=L-1? E[qr]>=c1, yes. Good.

Now, let's consider if `E[qr]>=c1` always: c1 < B[ql] <= E[ql] <= E[qr]. Yes.

Now, let's think about if `B[ql] <= E[ql]` valid. Yes.

Now, let's consider if in apply, `ql` and `qr` are compressed indices, E_l[ql] valid. Good.

Now, let's think about if `nonzero` list could have c1 from left half with B[c1] >= B[ql]? Since c1 <= m < ql, B[c1] < B[ql]. Good.

Now, let's consider if `t_min = E_l[ql] - c1` positive. Good.

Now, let's think about if in NTT branch, `F` fact range starts at t_min maybe >0. fact[0] not included. OK.

Now, let's consider if in direct branch, `fact_l[ei - bj]` where ei-bj could be 0? c<b, e_b>=b, so e_b-c >=1. Good.

Now, let's think about if in leaf, `ei - B_l[j]` for j<i, B[j]<B[i]<=ei, >=1. Good.

Now, let's consider if in naive, same.

Now, let's think about if `invfact[ei - B[i]]` index could be 0, yes. invfact[0]=1.

Now, let's consider if `fact[ei]` for ei=N, fact[N] computed. Good.

Now, let's think about if modulo 998244353 is int, operations ok.

Now, let's consider if final code should include `#` comments? OK.

Now, let's maybe write final code in final answer. But before, let's think if there is any hidden issue with ACL NTT and using n smaller than full convolution length but >=Mlen. We only query a band. Cyclic convolution modulo x^n-1: coefficient at k (0<=k<n) = sum_{t} linear_coeff[k + t*n]. We need for k in [L-1,Mlen-1], no t>=1. Max linear coeff index = L+Mlen-2. Need k+n > max for all k in range. Minimum k = L-1. Need L-1+n > L+Mlen-2 => n > Mlen-1. Since n>=Mlen, true. Good. So correct.

Now, let's think about if `n` power of two >=Mlen but Mlen maybe 0? no.

Now, let's consider if `Mlen` can be larger than max_n due to N+5? no.

Now, let's think about if `rate_tz` precompute with max_n maybe 8 for N=1, but n could be? Mlen<=1, n=1. OK.

Now, let's consider if in `ntt`, for n=1, h=0, loops none. But `rtz=rate_tz` local, no access. Good.

Now, let's think about if in `intt`, for n=1, h=0, loops none. Good.

Now, let's consider if pointwise loop for n=1 works.

Now, let's think about if in apply NTT branch, `inv_n = inv_pow2[n.bit_length() - 1]`. For n=1, index0. inv_pow2 length max_log+1, max_log>=0. Good.

Now, let's consider if `max_n` while < N+5 for N=1 gives8, max_log3, inv_pow2 length4. OK.

Now, let's think about if `rate2` precompute 30 but max_log maybe3. OK.

Now, let's consider if in `ntt`, `now = now * rtz[s] % mod` for s where rtz[s] maybe 1. OK.

Now, let's think about if `rate_tz` values for s beyond precomputed half? n<=max_n, s max n/2-1 <= max_n/2-1 = half-1. OK.

Now, let's consider if `max_n` while < N+5 could be 1 if N=0, but N>=1. OK.

Now, let's think about if input N line may have trailing spaces, int(strip) ok.

Now, let's consider if S line may be empty due to blank line, but constraints no. If empty, S[0] error. Could guard `if len(S)<2: print(0); return`. Not necessary but safe. Add:
```
if len(S) < 2:
    print(0); return
```
But N>=1 length2. OK.

Now, let's think about if `S` contains lowercase? no.

Now, let's consider if in scan, `b` increments for B. At end b=N. e[N-1] set. e[N]=N. Good.

Now, let's think about if `w` at end N. Good.

Now, let's consider if `e[b]` for b where B occurs after all W? e maybe N. But last char W, so last B before last W, e[N-1]<=N-1. For earlier B, e<=N-1. OK.

Now, let's think about if valid list includes b where e[b]=N before b=N? If last char W, no B after all W, so e[b]<N for b<N. But could e[b]=N for b<N if all W before B_{b+1} and last W? If all W before B_{b+1}, then no W after, but last char W after last B, contradiction if b+1<=N. So e[b]<N for b<N. OK.

Now, let's consider if `e[b] >= b` can fail for many. OK.

Now, let's think about if answer for all B then all W is N! mod. Our recurrence M=1 (b=N valid) H1=fact[N]*invfact[0]=N!. Good.

Now, let's consider if answer for alternating N=4 is 13. Let's brute maybe. Recurrence H4=13. OK.

Now, let's think about if CDQ with compressed M=4 and naive (M<=2000) would compute. Good.

Now, let's consider if for large M, H values maybe zero mod causing nonzero list sparse. CDQ adapts.

Now, let's think about if in apply, `lo=bl(nonzero,l)`, `hi=br(nonzero,m)`. If nonzero list contains indices <l and >m, bisect works. Good.

Now, let's consider if `nonzero` list is sorted because appended in increasing i. CDQ computes i in increasing order? Let's verify: solve(1,M) left then right, so yes global order. Within leaf, increasing. Thus nonzero sorted. Good.

Now, let's think about if in solve, after apply left->right, we solve right. The right solve will append indices >mid. So nonzero remains sorted. Good.

Now, let's consider if in apply, `nz_list = nonzero` and then `c = nz_list[lo + k]`. If nonzero list is mutated during apply? No, apply not recursive, no appends. Good.

Now, let's think about if in solve leaf, `append = nonzero.append` local, and we append. Good.

Now, let's consider if in apply direct, `val_l[i] = s % mod` where `s` maybe a Python int large. OK.

Now, let's think about if in apply NTT, after `intt(A)`, A values are mod but maybe not reduced? intt operations reduce. Good.

Now, let's consider if in `intt`, final values are n*conv mod but with A scaled, conv mod. Since all operations mod, values in [0,mod). Good.

Now, let's think about if in `ntt`, input A scaled by inv_n, but inv_n maybe not int? It is int. Good.

Now, let's consider if in `ntt`, `r = a[j] * now % mod` where `now` maybe 1 branch skipped. Good.

Now, let's think about if in `ntt`, `x = l + r; if x >= mod: x -= mod`. Since l,r < mod, x <2mod. Good.

Now, let's consider if in `intt`, `y = l - r; if y < 0: y += mod`. Since l,r<mod, y > -mod. Good.

Now, let's think about if in `intt`, `a[j] = y * inow % mod`, y<mod, inow<mod. Good.

Now, let's consider if in `ntt`, after `a[i]=x; a[j]=y`, x,y<mod. Good.

Now, let's think about if in pointwise, `A[i] = A[i] * F[i] % mod`, product <1e18. Good.

Now, let's consider if in direct, `s -= hj_list[k] * fact_l[ei - bj_list[k]]`, product <1e18, s can grow. OK.

Now, let's think about if in leaf, `s -= hj * fact_l[ei - B_l[j]]`, product <1e18, s up to64 terms. OK.

Now, let's consider if in naive, s up to2000 terms, 2e21. OK.

Now, let's think about if Python's `%` on negative big int is okay. Yes.

Now, let's consider if final code should avoid `del F` because in PyPy `del` may cost? It may help memory. Keep.

Now, let's think about if after NTT branch, `A` list large remains until function returns. OK.

Now, let's consider if recursion depth and large local variables in apply could keep memory? apply returns, locals freed. OK.

Now, let's think about if `sys.setrecursionlimit` should be before defining solve? It can be before call. OK.

Now, let's consider if `main` function with nested functions and large lists may hit Python's recursion limit? depth small.

Now, let's think about if there is any issue with using `bisect` on a list that is being appended in a nested function while another nested function has a local reference `nz_list = nonzero`. No concurrent.

Now, let's consider if in apply, `hi = br(nonzero, m)` where m is compressed index. If nonzero contains m? yes if H[m] nonzero and solved. Include. Good.

Now, let's think about if in solve, `apply(l, mid, mid + 1, r)` after solve(l,mid). If mid < l? no.

Now, let's consider if `LEAF` threshold and `r-l < LEAF`: For segment size 1, r-l=0 leaf. For size64, r-l=63 leaf. For size65, internal. Good.

Now, let's think about if `M` maybe 2001, solve root size2001, mid1001, etc. OK.

Now, let's consider if in naive M<=2000, O(2M) maybe 2 million, fine. Could set 3000 to avoid CDQ overhead for 2000-3000. But 2000 safe. If M=2001, CDQ precompute NTT maybe overkill but okay. Could set 3000 (4.5M) still fine. Let's set `NAIVE_LIMIT = 3000` to reduce CDQ for small. 4.5M inner loops in Python okay. But if time limit tight, 4.5M fine. Use 3000.

Now, let's think about if naive with M=3000 and dense: sum 4.5M, each with branch and multiplication. OK. CDQ for 3000 would precompute NTT and maybe direct only (since small), but overhead. Naive better. Set 3000.

Now, let's consider if M=5000, CDQ. OK.

Now, let's think about if in naive, we can optimize by local variables:
```
fact_l=fact; B_l=B; E_l=E; invfact_l=invfact; H_l=H; mod=MOD
for i...
```
Do that.

Now, let's consider if in naive, `for j in range(1,i): hj=H_l[j]; if hj: s -= hj * fact_l[ei - B_l[j]]`. Good.

Now, let's think about if `H_l` initially zeros, append? Not needed.

Now, let's consider if final answer for M=0 (shouldn't) print0. OK.

Now, let's think about if early exit S[0]!='B' or S[-1]!='W' before computing e. Good.

Now, let's consider if S length maybe 2N but N large, scan O(N). Good.

Now, let's think about if fact/invfact precompute O(N). Good.

Now, let's consider if total time dominated by CDQ NTT. OK.

Now, let's maybe include in notes the optimized NTT length (power of two >= Mlen) due to queried band. Good.

Now, let's think about if there is any issue with modulo inverse of n when n not power of two? n power. inv_pow2 precomputed. Good.

Now, let's consider if `inv_pow2` length max_log+1 where max_log = max_n.bit_length()-1. If n=max_n, index max_log. Good.

Now, let's think about if `max_n` while < N+5, for N=200000 max_n=262144, max_log=18. If n=262144, index18. inv_pow2[18] computed. Good.

Now, let's consider if `rate2` precompute 30 but `tz` for s=half-1 where half=131072, x=131072, tz=17? Wait half=max_n//2=131072, s max131071, x=131072=2^17, tz=17. max_log=18. rate2[17] exists. For n=max_n=2^18, ph=18, w=2^17, s max2^17-1, tz max17. rate2[17]=G^{(MOD-1)/2^{19}}? For n=2^18, final stage s last uses rate2[17]? Expected primitive 2^18 root? Let's check: For n=2^h, final ph=h, s max 2^{h-1}-1, s+1=2^{h-1}, tz=h-1. rate2[h-1]=G^{(MOD-1)/2^{h+1}}? Wait rate2[i]=G^{(MOD-1)/2^{i+2}}. For i=h-1, exponent (MOD-1)/2^{h+1}, which is primitive 2^{h+1} root, not 2^h. But update after last s not used. The `now` used for s=2^{h-1}-1 is product of previous updates. Let's compute for h=3 (n=8), final ph=3, s max3, tz for s=3 is2, rate2[2]=G^{(MOD-1)/16}=primitive16 root, update after last not used. now used for s=3 was product rate2[0]*rate2[1]*rate2[0]? = w4*w8*w4 = w8^? w4=w8^2, product w8^5, correct. So rate2[h-1] only used for update after last, not for twiddle. For s where tz=h-1? s=2^{h-1}-1, update after last. So rate2[h-1] not used as twiddle? For s=2^{h-2}-1 maybe tz=h-2, update to now for next s. The maximum twiddle now may involve rate2[h-2]. But precomputing h-1 safe. For n=2^18, rate2[17] maybe used for update after last, not needed but ok. rate2 length30.

Now, let's think about if `rate_tz` for s=half-1 uses rate2[17], ok.

Now, let's consider if `irate_tz` similarly.

Now, let's think about if ACL butterfly with `rate2` precomputed as above is correct for all n up to 2^23. Yes.

Now, let's consider if we should use `MOD = 998244353` and `G = 3`. Good.

Now, let's think about if in `ntt`, `now` update after s uses `rtz[s]`, but for s where `rtz[s]` maybe 1 (if tz large beyond max root?), not. OK.

Now, let's consider if in `intt`, `inow` update after s uses `itz[s]`. Good.

Now, let's think about if in `ntt`, for `now == 1` branch, we skip multiplication by now. But what if `now` is 1 mod but not integer 1? It is integer 1. Good.

Now, let's consider if in `intt`, `inow == 1` branch. Good.

Now, let's think about if in `ntt`, `now` can become 1 again for some s? Possibly if root order cycles? For primitive roots, now sequence odd powers, not 1 until full cycle length 2^ph, but s range 2^{ph-1}, so no. Only s=0. OK.

Now, let's consider if in `intt`, inow sequence similarly.

Now, let's think about if we can further optimize by unrolling inner loop for p=1? For large ph, p=1, inner loop one. The overhead of for loop and branch maybe. Could special case p==1:
```
if p == 1:
   i=base; j=i+1; ...
else:
   for i...
```
This might speed large ph stages (many s, p small). For p=1, w=n/2, half of butterflies? Actually stage ph=h has p=1, w=n/2, total n/2 butterflies. Special casing could help. For p=2, w=n/4, total n/2 butterflies. Many stages with small p. But code complexity. Maybe not necessary.

Could use while loop for inner to avoid range object? For p small, for loop overhead. But PyPy optimizes. Keep simple.

Now, let's think about if in `ntt`, `base = s << shift`, `end = base + p`. For p=1, end=base+1. OK.

Now, let's consider if in `intt`, same.

Now, let's think about if in apply NTT, `F[:Mlen] = fact_l[t_min:t_max + 1]` uses slice assignment which may be O(Mlen) in C, fast. Good.

Now, let's consider if `A` fill loop in Python O(nz). If nz dense ~L~Mlen/2, O(n). OK.

Now, let's think about if in direct branch, creating `hj_list` and `bj_list` with `[0]*nz` and filling O(nz). If direct_cost small, ok. If direct chosen for nz large rb small, O(nz) comparable to direct_cost. OK.

Now, let's consider if in apply direct nz==1, we don't create lists. Good.

Now, let's think about if in apply direct nz>1 but rb==1, we create lists length nz and inner loop nz once. Could instead loop c directly without lists to avoid list allocation:
```
if rb == 1:
   i=ql; ei=E_l[i]; s=val_l[i]
   for k in range(nz): c=nz_list[lo+k]; s -= H_l[c]*fact_l[ei-B_l[c]]
   val_l[i]=s%mod; return
```
This avoids lists. Could add for speed in sparse large nz rb small. Similarly if nz small, lists ok. Let's add a special case `if rb == 1` before list creation. Also if nz==1 already. For rb==1 and nz>1:
```
i = ql
ei = E_l[i]
s = val_l[i]
for k in range(nz):
    c = nz_list[lo + k]
    s -= H_l[c] * fact_l[ei - B_l[c]]
val_l[i] = s % mod
return
```
This is good. If rb small but >1, lists ok.

Similarly, if nz==1, loop over rb. Good.

Now, let's think about if in leaf direct, segment size small, no need special.

Now, let's consider if in apply direct, `rb` could be 0? ql<=qr, no.

Now, let's think about if in apply NTT branch, `nz` could be 1 but direct not chosen? We argued direct chosen. But if not, A fill one entry. OK.

Now, let's consider if in apply, `direct_cost = nz * rb` could overflow? Python no.

Now, let's think about if `COST_FACTOR` should be float? int.

Now, let's consider if in apply, `ntt_cost = n * (n.bit_length() - 1)` maybe underestimates because 3 transforms. Factor4 accounts. OK.

Now, let's think about if in apply, `use_direct = direct_cost <= ntt_cost * COST_FACTOR`. If ntt_cost=0, direct false unless direct_cost0. OK.

Now, let's consider if in apply, after NTT, we don't mod `val_l[i]` if term=0? We do `%`. OK.

Now, let's think about if in NTT branch, `A[idx]` maybe not int? int.

Now, let's consider if in `intt`, after inverse, values are n*conv mod, but with A scaled by inv_n. Since inv_n is modular inverse, n*inv_n=1 mod. Good.

Now, let's think about if `inv_n` should be applied to A before NTT or after pointwise? Before is fine. If A has zeros, scaling only nonzero. Good.

Now, let's consider if scaling A by inv_n changes the convolution coefficients by inv_n, but inverse unscaled multiplies by n. Yes.

Now, let's think about if we could scale F by inv_n instead if Mlen < nz? We scale nz entries, which is <=L<=Mlen, so nz <=Mlen. Scaling A is no more than scaling F. Good.

Now, let's consider if in NTT branch, `A` entries are H[c]*inv_n mod. If H[c] nonzero, product nonzero. OK.

Now, let's think about if in direct branch, we subtract contributions from H[c] (not scaled). Good.

Now, let's consider if in leaf, H[i] computed from val[i] which has contributions from previous applies (mod). Good.

Now, let's think about if in CDQ, val[i] initial fact[E[i]] but for invalid i not in compressed. OK.

Now, let's consider if in compressed recurrence, denominator invfact[E[i]-B[i]]. If E[i]-B[i] maybe large, invfact precomputed. Good.

Now, let's think about if in naive, `H[i]` for i maybe 0 if s%mod=0. OK.

Now, let's consider if final answer H[M] maybe not computed if M=0. We return0.

Now, let's think about if there is any issue with using `e[b] > e[b-1]` for b=1 when e[0] maybe not set if first char not B, but early exit. OK.

Now, let's consider if S1=B but there are W before first B? impossible. e0=0.

Now, let's think about if S2N=W but there are B after last W? impossible. e[N-1] set.

Now, let's consider if scan `for ch in S:` with S length 2N, O(N). Good.

Now, let's think about if input N and S may have extra whitespace lines, `input().strip()` for S could read empty if blank line. Not in valid input.

Now, let's consider if final code should be compatible with Python 3.8+. Uses bit_length, bisect. OK.

Now, let's think about if `sys.setrecursionlimit(1_000_000)` with underscore ok.

Now, let's consider if in notes, we should mention modulo and NTT root. OK.

Now, let's maybe produce final code. Ensure no syntax errors.

Let's write code fully:

```
import sys
from bisect import bisect_left, bisect_right

MOD = 998244353
G = 3

def main():
    input = sys.stdin.readline
    line = input().strip()
    if not line:
        return
    N = int(line)
    S = input().strip()
    if len(S) < 2:
        print(0)
        return
    if S[0] != 'B' or S[-1] != 'W':
        print(0)
        return
    e = [0]*(N+1)
    w=0; b=0
    for ch in S:
        if ch == 'W':
            w += 1
        else:
            b += 1
            if b <= N:
                e[b-1] = w
    e[N] = N
    B=[0]; E=[0]
    for b in range(1,N+1):
        if e[b] >= b and e[b] > e[b-1]:
            B.append(b); E.append(e[b])
    M = len(B)-1
    if M == 0:
        print(0); return
    fact=[1]*(N+1)
    for i in range(1,N+1): fact[i]=fact[i-1]*i%MOD
    invfact=[1]*(N+1)
    invfact[N]=pow(fact[N], MOD-2, MOD)
    for i in range(N,0,-1): invfact[i-1]=invfact[i]*i%MOD
    NAIVE_LIMIT=3000
    if M <= NAIVE_LIMIT:
        H=[0]*(M+1)
        fact_l=fact; B_l=B; E_l=E; invfact_l=invfact; mod=MOD
        for i in range(1,M+1):
            ei=E_l[i]
            s=fact_l[ei]
            for j in range(1,i):
                hj=H[j]
                if hj:
                    s -= hj * fact_l[ei - B_l[j]]
            H[i] = s % mod * invfact_l[ei - B_l[i]] % mod
        print(H[M] % mod)
        return
    max_n=1
    while max_n < N + 5:
        max_n <<= 1
    max_log = max_n.bit_length() - 1
    rate2=[0]*30; irate2=[0]*30
    for i in range(30):
        rate2[i]=pow(G, (MOD-1) >> (i+2), MOD)
        irate2[i]=pow(rate2[i], MOD-2, MOD)
    half=max_n//2
    rate_tz=[1]*(half+1)
    irate_tz=[1]*(half+1)
    for s in range(half):
        x=s+1
        tz=(x & -x).bit_length()-1
        rate_tz[s]=rate2[tz]
        irate_tz[s]=irate2[tz]
    inv_pow2=[1]*(max_log+1)
    inv2=(MOD+1)//2
    for i in range(1,max_log+1):
        inv_pow2[i]=inv_pow2[i-1]*inv2%MOD

    def ntt(a):
        n=len(a)
        h=n.bit_length()-1
        mod=MOD
        rtz=rate_tz
        for ph in range(1,h+1):
            w=1 << (ph-1)
            p=1 << (h-ph)
            shift=h-ph+1
            now=1
            for s in range(w):
                base=s << shift
                end=base+p
                if now == 1:
                    for i in range(base,end):
                        j=i+p
                        l=a[i]
                        r=a[j]
                        x=l+r
                        if x>=mod: x-=mod
                        y=l-r
                        if y<0: y+=mod
                        a[i]=x
                        a[j]=y
                else:
                    for i in range(base,end):
                        j=i+p
                        l=a[i]
                        r=a[j]*now%mod
                        x=l+r
                        if x>=mod: x-=mod
                        y=l-r
                        if y<0: y+=mod
                        a[i]=x
                        a[j]=y
                now=now*rtz[s]%mod

    def intt(a):
        n=len(a)
        h=n.bit_length()-1
        mod=MOD
        itz=irate_tz
        for ph in range(h,0,-1):
            w=1 << (ph-1)
            p=1 << (h-ph)
            shift=h-ph+1
            inow=1
            for s in range(w):
                base=s << shift
                end=base+p
                if inow == 1:
                    for i in range(base,end):
                        j=i+p
                        l=a[i]
                        r=a[j]
                        x=l+r
                        if x>=mod: x-=mod
                        y=l-r
                        if y<0: y+=mod
                        a[i]=x
                        a[j]=y
                else:
                    for i in range(base,end):
                        j=i+p
                        l=a[i]
                        r=a[j]
                        x=l+r
                        if x>=mod: x-=mod
                        y=l-r
                        if y<0: y+=mod
                        a[i]=x
                        a[j]=y*inow%mod
                inow=inow*itz[s]%mod

    val=[0]*(M+1)
    for i in range(1,M+1):
        val[i]=fact[E[i]]
    H=[0]*(M+1)
    nonzero=[]
    LEAF=64
    DIRECT_ALWAYS=4096
    COST_FACTOR=4
    bl=bisect_left
    br=bisect_right

    def apply(l,m,ql,qr):
        lo=bl(nonzero,l)
        hi=br(nonzero,m)
        if lo==hi:
            return
        nz=hi-lo
        rb=qr-ql+1
        direct_cost=nz*rb
        mod=MOD
        fact_l=fact
        E_l=E
        B_l=B
        val_l=val
        H_l=H
        nz_list=nonzero
        c0=B_l[nz_list[lo]]
        c1=B_l[nz_list[hi-1]]
        if direct_cost <= DIRECT_ALWAYS:
            use_direct=True
        else:
            L=c1-c0+1
            t_min=E_l[ql]-c1
            if t_min<0: t_min=0
            t_max=E_l[qr]-c0
            if t_max>N: t_max=N
            Mlen=t_max-t_min+1
            if Mlen <= 0:
                return
            n=1 << (Mlen-1).bit_length()
            ntt_cost=n*(n.bit_length()-1)
            use_direct=direct_cost <= ntt_cost * COST_FACTOR
        if use_direct:
            if nz == 1:
                c=nz_list[lo]
                hj=H_l[c]
                bj=B_l[c]
                for i in range(ql, qr+1):
                    val_l[i] = (val_l[i] - hj * fact_l[E_l[i] - bj]) % mod
                return
            if rb == 1:
                i=ql
                ei=E_l[i]
                s=val_l[i]
                for k in range(nz):
                    c=nz_list[lo+k]
                    s -= H_l[c] * fact_l[ei - B_l[c]]
                val_l[i] = s % mod
                return
            hj_list=[0]*nz
            bj_list=[0]*nz
            for k in range(nz):
                c=nz_list[lo+k]
                hj_list[k]=H_l[c]
                bj_list[k]=B_l[c]
            for i in range(ql, qr+1):
                ei=E_l[i]
                s=val_l[i]
                for k in range(nz):
                    s -= hj_list[k] * fact_l[ei - bj_list[k]]
                val_l[i] = s % mod
            return
        # NTT branch
        inv_n=inv_pow2[n.bit_length()-1]
        A=[0]*n
        for k in range(nz):
            c=nz_list[lo+k]
            A[B_l[c]-c0] = H_l[c] * inv_n % mod
        F=[0]*n
        F[:Mlen] = fact_l[t_min:t_max+1]
        ntt(A)
        ntt(F)
        for i in range(n):
            A[i] = A[i] * F[i] % mod
        del F
        intt(A)
        base_idx=c0+t_min
        for i in range(ql, qr+1):
            idx=E_l[i]-base_idx
            val_l[i] = (val_l[i] - A[idx]) % mod

    sys.setrecursionlimit(1_000_000)

    def solve(l,r):
        if r-l < LEAF:
            fact_l=fact
            E_l=E
            B_l=B
            val_l=val
            H_l=H
            invfact_l=invfact
            mod=MOD
            append=nonzero.append
            for i in range(l,r+1):
                ei=E_l[i]
                s=val_l[i]
                for j in range(l,i):
                    hj=H_l[j]
                    if hj:
                        s -= hj * fact_l[ei - B_l[j]]
                hi_val=s % mod * invfact_l[ei - B_l[i]] % mod
                H_l[i]=hi_val
                if hi_val:
                    append(i)
            return
        mid=(l+r)>>1
        solve(l,mid)
        apply(l,mid,mid+1,r)
        solve(mid+1,r)

    solve(1,M)
    print(H[M] % MOD)

if __name__ == "__main__":
    main()
```

Check variable `N` used in apply for t_max clipping. It's from outer main, closure. OK.

Potential issue: In apply, if `direct_cost > DIRECT_ALWAYS`, we compute `L,t_min,t_max,Mlen,n,ntt_cost`. If `Mlen <=0`, return. If `use_direct` true, we go to direct branch. But variables `L,t_min,t_max,Mlen,n` are defined. If `use_direct` false, NTT branch uses them. Good.

Potential issue: If `direct_cost <= DIRECT_ALWAYS`, `use_direct=True`, but variables `L,t_min,t_max,Mlen,n` not defined. Direct branch returns, so NTT not executed. Good.

Potential issue: In direct branch, if `nz==1` or `rb==1`, we return. If not, create lists. Good.

Potential issue: In direct branch `rb==1`, we use `for k in range(nz): c=nz_list[lo+k]`. This loops over nonzero c. Good. It doesn't mod until end. s magnitude nz*1e18, nz could be large (up to M). If rb==1 and direct chosen, direct_cost=nz. Could be up to maybe ntt_cost*4. ntt_cost maybe large, nz up to 200k, s 2e23, ok. Mod once.

Potential issue: In direct branch general, `hj_list` and `bj_list` creation. If nz large and rb small but >1, s per i nz terms, ok.

Potential issue: In NTT branch, `F[:Mlen] = fact_l[t_min:t_max+1]`. If `Mlen` > n? n power >=Mlen, ok. If slice length Mlen, ok. If `t_max` clipped to N, slice end N+1, length N-t_min+1 = Mlen (since t_max=N). OK.

Potential issue: In NTT branch, `A[B_l[c]-c0]` index maybe >=n if L>n. We believe L<=Mlen<=n. Let's maybe add a safety: if `B_l[c]-c0 >= n`, but impossible. Could avoid.

Potential issue: In NTT branch, query `idx=E_l[i]-base_idx`. Could be negative or >=n if assumptions fail. Could add if 0<=idx<n else 0, but would hide bugs. Maybe add safety for robustness:
```
if 0 <= idx < n:
    val_l[i] = (val_l[i] - A[idx]) % mod
```
But if idx out of range due to bug, contribution missing. Not needed. Could add to prevent crash on unexpected, but performance slight. We can trust.

Potential issue: In `ntt`, `rtz[s]` for s maybe out of range if n>max_n. max_n while <N+5, n<=next pow >=Mlen<=N+? Should be <=max_n. If Mlen=N+1 and max_n next pow >=N+5, ok. Good.

Potential issue: In `intt`, no final scaling. We scale A by inv_n. But what about F? The convolution result after inverse unscaled is n*(A_scaled*F) = conv. Good.

Let's test with a tiny convolution manually using our n choice. Suppose A length L=2 [a0,a1], F length Mlen=2 [f0,f1], n=2 (Mlen=2). Cyclic convolution length2: coeff0=a0f0+a1f1, coeff1=a0f1+a1f0. Linear conv length3: [a0f0, a0f1+a1f0, a1f1]. Query range L-1=1 to Mlen-1=1, only coeff1. Cyclic coeff1 = a0f1+a1f0, correct. High linear coeff2 wraps to0, not queried. Good.

Suppose L=3,Mlen=3,n=4? Mlen=3, n=4. Query 2..2. Linear coeff2=a0f2+a1f1+a2f0. Cyclic coeff2 same (no wrap because max4? linear max4, n4, coeff4 wraps0). Good. If n=3 not power. OK.

Suppose L=3,Mlen=5,n=8. Query2..4. Linear max6, no wrap. Good.

Now, let's think about if Mlen can be much larger than L, n>=Mlen, no wrap into query. Good.

Now, let's consider if in NTT branch, we only need coefficients up to Mlen-1, but cyclic convolution with n>=Mlen also ensures coefficients 0..Mlen-1 are linear (no wrap from high into them) because high wrap to 0..L-2 (since max L+Mlen-2 - n <= L-2 if n>=Mlen). Query starts L-1. Good.

Now, let's think about if `L` could be 0? nz>0, c0<=c1, L>=1.

Now, let's consider if in apply, `c0` and `c1` are original black indices, but `B_l[c]` for c in nonzero. Good.

Now, let's think about if in direct branch, `fact_l[E_l[i] - bj]` index maybe >N? E<=N,bj>=1 => <=N-1. OK.

Now, let's consider if in naive, `fact_l[ei - B_l[j]]` index maybe 0? no.

Now, let's think about if in leaf, `invfact_l[ei - B_l[i]]` index maybe negative if invalid? valid. OK.

Now, let's consider if `e[b] > e[b-1]` for b=N with e[N]=N, e[N-1] maybe N? If last char W, no. If input invalid but S[-1]=='W' ensures. OK.

Now, let's think about if S has N W and N B but S[-1]=='W', e[N-1] could be N if last W is before last B? No last char W, last B before it, so at least one W after last B, e[N-1]<=N-1. Good.

Now, let's consider if early exit S[0]!='B' or S[-1]!='W' is sufficient for strong impossible. Yes.

Now, let's think about if there is a case S1=B,S2N=W but answer 0 modulo. Code prints 0 if H[M] mod0. OK.

Now, let's consider if in notes, we should mention that H_0 is not used. OK.

Now, let's think about if final answer should be status solved. Yes.

Now, let's maybe refine notes to be under 800 words. Use bullets.

Notes draft:
- **Strong connectivity reduction:** path edges only go forward, so strong iff 2N reaches1. This forces S1=B,S2N=W. For cut k, let C_k be added edges from white >k to black <=k; need C_k>0 for all k. The minimum in each block between black vertices is just before next black, so bad prefix b (1<=b<N) is no edge from whites after B_b to black <=b, equivalent to first b black matched to first e_b white.
- **e_b and permutation model:** e_b=#W before (b+1)-th B, e_N=N. A matching is a permutation pi of white indices assigned to black indices. Bad b iff max_{j<=b} pi_j <= e_b. Strong iff no bad b<N.
- **First bad decomposition:** H_b = #injections of first b black into first e_b white with no smaller bad prefix. Answer H_N. H_b=0 unless b>0, e_b>=b, e_b>e_{b-1}. Total injections e_b!/(e_b-b)! partition by first bad c: H_b(e_b-b)! = e_b! - sum_{c<b} H_c(e_b-c)!, giving recurrence.
- **Compression:** keep only valid b in arrays B,E (1-indexed). Recurrence H_i=(fact[E_i]-sum_{j<i}H_j fact[E_i-B_j])*invfact[E_i-B_i].
- **CDQ:** val[i] starts fact[E_i] and accumulates negative contributions from previous j. solve(l,r): solve left, apply left->right, solve right. Leaf threshold 64 computes internal contributions directly. Global sorted nonzero list gives nonzero H in left range.
- **apply direct/NTT:** For left nonzero c0..c1 and right valid ql..qr, contribution is sum H_c fact[E_i-B_c]. If nz*rb small, direct loops (with special nz=1/rb=1). Otherwise convolution: A over original black index range c0..c1 (A[B_c-c0]=H_c*inv_n), F=fact[t_min..t_max] with t_min=E[ql]-c1, t_max=E[qr]-c0. Query index E_i-c0-t_min.
- **NTT length optimization:** Only coefficients L-1..Mlen-1 are queried, and Mlen>=L, so a cyclic NTT of length power of two >=Mlen is enough; high coefficients wrap below L-1. This halves max length (<=next pow2(N)). A is pre-scaled by inv_n and inverse NTT is unscaled, so queried values are already the linear convolution.
- **NTT implementation:** mod 998244353, primitive root 3, ACL-style butterfly with precomputed trailing-zero rate factors; no bit-reversal permutation; inverse omits final scaling.
- **Constants:** naive for M<=3000; leaf 64; direct if nz*rb<=4096 or <=4*n*log n; otherwise NTT.

This is under 800 words. Avoid headings. Good.

Now, let's think if notes mention "ALLCAPS" no. Use bold topic with `**`. The instruction says do not use bold-only lines; our lines are bullets with bold topic and text, ok.

Now, let's consider if final code's comments include ALLCAPS? No.

Now, let's think about if code uses `NAIVE_LIMIT` all caps variable, that's in code not notes. OK.

Now, let's consider if final answer should include only sections, no extra. Yes.

Before final, let's double-check the recurrence with compressed indices and CDQ val. Suppose valid list skips invalid c. In recurrence sum over c<b all original, invalid H=0. Good. But what about c=0? H_0 not included. In first bad decomposition, total injections partition by first bad c from 1..b. No c=0. Good.

Now, let's think about if H_b for b valid but e_b=e_{b-1} set0. In compressed, skipped. But in recurrence for later b, sum over c<b includes this invalid c with H=0. Good.

Now, let's consider if e_b<b invalid but e_b>e_{b-1}; H=0. Could first bad b be impossible due to not enough whites. Yes.

Now, let's think about if total injections P(e_b,b) for e_b>=b. If e_b<b, P=0. Recurrence would divide by negative factorial, invalid. Skip.

Now, let's consider if in valid condition `e[b] > e[b-1]` for b=1 with e0=0. If e1=0 (no W before B2), invalid because e1>=1 false. Good.

Now, let's think about if in scan, e[0] for first B set to w before first B. If S1=B, w=0. If S1 not B early exit, but e0 maybe >0. OK.

Now, let's consider if in sample3, valid list B=[1,2,5,6,8,9], E=[2,3,5,6,8,9]. M=6 naive. Code computes. Good.

Now, let's think about if in naive, for i=5 (b=5,E=5), sum j=1..4 includes j=3? H[3] corresponds b=6? Wait compressed indices: i=3 is b=5? Let's map: i1 b1, i2 b2, i3 b5, i4 b6, i5 b8, i6 b9. For i=3 (b5), sum j=1,2 (b1,b2). H[3] not computed yet. Good. Recurrence sum c<b original, previous valid only. Good.

Now, let's consider if in CDQ, compressed index order matches original b order. Yes.

Now, let's think about if in apply, right segment [ql,qr] compressed, E increasing. Good.

Now, let's consider if in NTT branch, `t_min=E_l[ql]-c1`. c1 is original black index of last nonzero in left, which may be less than B[ql]-1 if some H zero at end. Then t_min larger than minimal possible? Actually if last nonzero c1 < B[m], then for b=ql, terms from c>c1 are zero, so minimal t among nonzero c is E[ql]-c1. Good. F range can start there. If there are zero H after c1, no contributions. Good.

Similarly c0 first nonzero, t_max=E[qr]-c0. If zeros before c0, no contributions. Good.

Now, let's think about if A length L=c1-c0+1 includes zeros for invalid/zero H between. Good.

Now, let's consider if `Mlen` with c0/c1 nonzero may be smaller than if using full left range. Good.

Now, let's think about if `Mlen>=L` still holds with c0/c1 nonzero? c1 < B[ql] <= E[ql], c0>=1. Mlen-1 = E[qr]-c0 - (E[ql]-c1) = (E[qr]-E[ql]) + (c1-c0). E[qr]-E[ql] >= rb-1 >=1 if rb>1, c1-c0>=nz-1>=0. L-1=c1-c0. So Mlen-1 >= L-1 + rb-1 >= L-1. If rb=1, Mlen=L. Good.

Now, let's consider if `t_min` max0 could make Mlen larger, still >=L. Good.

Now, let's think about if in NTT branch, `n` power >=Mlen, but if Mlen=0 return. OK.

Now, let's consider if in apply, `t_max` clipped to N. If t_max clipped, Mlen maybe smaller than actual needed if actual t_max>N. But actual t_max=E[qr]-c0 <=N-1, so no clip. OK.

Now, let's think about if in `ntt`, for n maybe not power of two? We set power. Good.

Now, let's consider if in `intt`, after inverse unscaled, values are n*conv mod. Since A scaled by inv_n, conv. But what about the fact that we used cyclic convolution length n, not full? For queried indices, cyclic=linear. Good.

Now, let's think about if in NTT branch, we subtract `A[idx]` from val. A[idx] is contribution sum H_c fact[E_i-B_c] mod. Yes.

Now, let's consider if in direct branch, we subtract same. Good.

Now, let's think about if in leaf, we subtract internal contributions directly, not via val. Good.

Now, let's consider if in solve, after leaf computes H, it does not update val for future within same leaf, but future right segments get contributions via apply from this leaf as a whole. For indices within same leaf, direct sum handled. Good.

Now, let's think about if in solve internal, after solve left, apply left->right updates val for all right indices. Then solve right. Within right, contributions from left are in val. Contributions from within right handled recursively/direct. Good.

Now, let's consider if in apply, it updates val for right indices even if some right indices invalid? Compressed all valid. Good.

Now, let's think about if in apply, it uses H values from left that are final. Yes.

Now, let's consider if in apply, `nonzero` list may not include some left H if H=0 mod. They contribute 0. Good.

Now, let's think about if in leaf, `hi_val` could be 0 mod, not appended. Good.

Now, let's consider if in naive, H list not used for nonzero, just direct. Good.

Now, let's think about if final code's `max_n` precompute uses `N+5`, but if N=200000, max_n=262144. `rate_tz` half=131072. In ntt for n=262144, ph=18, w=131072, s max131071, rtz index ok. For n=131072, ph=17, w=65536, ok.

Now, let's consider if `rate2` precompute 30 but `tz` for s=131071 x=131072 tz=17, ok.

Now, let's think about if `irate2` for i=17 computed. OK.

Now, let's consider if in `ntt`, `now=now*rtz[s]%mod` for s=131071 uses rtz[131071]=rate2[17]. OK.

Now, let's think about if in `intt`, `inow=inow*itz[s]%mod` for s=131071 uses irate2[17]. OK.

Now, let's consider if `max_log` for max_n=262144 is 18, inv_pow2 length19. n.bit_length()-1 for n=262144 is18. OK.

Now, let's think about if in apply NTT, `inv_n=inv_pow2[n.bit_length()-1]`. If n=1, index0. OK.

Now, let's consider if in `ntt`, `h=n.bit_length()-1` for n=1 is0, range(1,1) empty. Good.

Now, let's think about if in `intt`, `h=0`, range(0,0,-1) empty. Good.

Now, let's consider if in pointwise loop for n=1, `A[0]=A[0]*F[0]%mod`. Good.

Now, let's think about if in apply NTT, `F[:Mlen] = fact_l[t_min:t_max+1]` when n=1,Mlen=1. OK.

Now, let's consider if in apply NTT, `A[B_l[c]-c0] = H_l[c] * inv_n % mod`. If multiple c same B? B unique. OK.

Now, let's think about if in direct branch, `val_l[i] = (val_l[i] - hj * fact_l[E_l[i] - bj]) % mod` for nz==1. If rb large, modulo each i. Could accumulate? Each i independent, one subtraction, modulo needed to keep val small. OK.

Now, let's consider if in direct branch rb==1, we don't mod inside loop, mod once. Good.

Now, let's think about if in direct branch general, we mod once per i. Good.

Now, let's consider if in leaf, we mod once per i. Good.

Now, let's think about if in naive, mod once per i. Good.

Now, let's consider if in apply NTT, after intt, A values mod. We mod val per i. Good.

Now, let's think about if in apply NTT, `del F` before intt may free F but in PyPy maybe not; ok.

Now, let's consider if in apply NTT, after function returns, A freed. OK.

Now, let's think about if there is any chance of hitting Python's maximum recursion due to `solve` calling itself many times? Depth log, no.

Now, let's consider if `sys.setrecursionlimit` after defining solve but before call. OK.

Now, let's think about if code uses `input` variable shadowing built-in, ok.

Now, let's consider if final answer should not directly test samples. Code reads stdin. OK.

Now, let's maybe include a small comment in code about `intt` unscaled. Not necessary but helpful.

Now, let's think about if there is a possibility that `Mlen` is not an int? It is.

Now, let's consider if in apply, `if t_max > N: t_max = N` uses N from outer. If N is large, ok. If t_max clipped, Mlen maybe less than L? Suppose t_max clipped to N but actual t_max>N (impossible). If it happened, Mlen could be <L, n maybe <L, A index out of range. But impossible. Could add safety `if Mlen < L: Mlen = L` and fill F with zeros beyond fact range. But slice assignment issue. We can implement safe extension:
```
if Mlen < L:
    # should not happen, but extend F range with zeros
    F = [0]*n
    F[:t_max-t_min+1] = fact_l[t_min:t_max+1]
    Mlen = L
```
But n computed after Mlen. Too complex. Trust proof.

Now, let's think about if `E_l[qr] - c0` could be less than `E_l[ql] - c1`? No because E[qr]>=E[ql] and c0<=c1, so t_max - t_min = E[qr]-E[ql]+c1-c0 >=0.

Now, let's consider if `t_min` max0 could make t_min > t_max? If E[ql]-c1 > E[qr]-c0? That would mean E[qr]-E[ql] < c1-c0. But c1-c0 < B[ql]-c0 <= E[ql]-c0, and E[qr]>=E[ql], so no. If t_min set0, even smaller. OK.

Now, let's think about if in apply, `c0` and `c1` are original black indices, but `B_l[c]` maybe not sorted in nonzero? nonzero sorted by compressed index, B increasing. Good.

Now, let's consider if in `ntt`, the ACL butterfly expects input length power of two and mod prime. Yes.

Now, let's think about if in `ntt`, for `now == 1` branch, we don't multiply `r` by now, but if now=1, ok. For `inow == 1`, ok.

Now, let's consider if in `ntt`, `now` update after s uses `rtz[s]` even when `now==1` branch. Good.

Now, let's think about if in `intt`, `inow` update after s uses `itz[s]`. Good.

Now, let's consider if in `ntt`, `rtz` list values are mod ints. Good.

Now, let's think about if in `intt`, `itz` list values mod ints. Good.

Now, let's consider if in precompute `rate2[i] = pow(G, (MOD - 1) >> (i + 2), MOD)`, for i=29 exponent0, rate2=1. `irate2=1`. OK.

Now, let's think about if `G=3` is primitive root for 998244353. Yes.

Now, let's consider if in `ntt`, `mod=MOD` local, but `MOD` global. OK.

Now, let's think about if in `apply`, `mod=MOD` local. OK.

Now, let's consider if in `solve`, `mod=MOD` local. OK.

Now, let's think about if in naive, `mod=MOD` local. OK.

Now, let's consider if final code's `print(H[M] % MOD)` uses global MOD. OK.

Now, let's think about if in early exit, we don't consume S? We read S before. OK.

Now, let's consider if input N line may have spaces, int(strip). OK.

Now, let's think about if S line may be very long, strip O(N). OK.

Now, let's consider if in scan, `if b <= N` always true for B count up to N. Could omit, but safe.

Now, let's think about if `e[N]=N` after scan, but e[N] maybe previously set if b=N+1? no.

Now, let's consider if `w` at end N, but not used. OK.

Now, let's think about if valid list building uses `e[b-1]` for b=1, e[0] set. If S1=B, e0=0. If S1 not B early. OK.

Now, let's consider if `e[b] > e[b-1]` for b where e[b-1] maybe 0 and e[b]=0, false. Good.

Now, let's think about if `e[b] >= b` for b=N, e[N]=N true. Good.

Now, let's consider if M could be less than N, compressed. OK.

Now, let's think about if in CDQ, `val` length M+1, indices 1..M. OK.

Now, let's consider if in apply, `ql,qr` within 1..M. solve calls with mid+1<=r. OK.

Now, let's think about if in solve leaf, `for j in range(l,i)` when l=i, empty. OK.

Now, let's consider if in solve leaf, `append=nonzero.append` local, but if H[i] nonzero, append. Good.

Now, let's think about if in apply, `nz_list=nonzero` local, but nonzero may be appended later, not during apply. OK.

Now, let's consider if in apply, `lo=bl(nonzero,l)` and `hi=br(nonzero,m)`. If nonzero list is empty, lo=hi=0. OK.

Now, let's think about if in apply, `c0=B_l[nz_list[lo]]` when lo==hi? returned. OK.

Now, let's consider if in apply, `hi-1` valid. OK.

Now, let's think about if in direct branch general, `for k in range(nz): s -= hj_list[k] * fact_l[ei - bj_list[k]]`. If `ei - bj_list[k]` negative, list index from end. But impossible. OK.

Now, let's consider if in NTT branch, `A[B_l[c]-c0]` index maybe negative? c>=c0, no.

Now, let's think about if in NTT branch, `F[:Mlen] = fact_l[t_min:t_max+1]` if `t_min` > `t_max`? Mlen<=0 return. OK.

Now, let's consider if in NTT branch, `n` maybe 0? Mlen>0, bit_length. OK.

Now, let's think about if in NTT branch, `ntt(A)` and `ntt(F)` modify lists. Good.

Now, let's consider if in NTT branch, after pointwise, `del F`, then `intt(A)`. Good.

Now, let's think about if in NTT branch, `base_idx=c0+t_min`, query idx. If t_min clipped? ok.

Now, let's consider if in NTT branch, `A[idx]` maybe not the contribution if idx < L-1? For first i idx=L-1. For later idx increases by E increments (>=1). So idx>=L-1. Good.

Now, let's think about if in NTT branch, `idx` could skip some values, but A has all. OK.

Now, let's consider if in NTT branch, `val_l[i] = (val_l[i] - A[idx]) % mod`. If A[idx] is contribution from left nonzero. Good.

Now, let's think about if in apply direct, we update val for all i in right, including those that may later be in left of a sub-apply? Yes, val accumulates all previous contributions. Good.

Now, let's consider if in solve, after apply left->right, val for right includes left contributions. Then solve right may apply its left->right, adding more. Good.

Now, let's think about if in solve, val for left indices after solve left not updated with internal contributions, but not needed. Good.

Now, let's consider if in leaf, val for i includes contributions from c<l, but not c in [l,i-1]. We sum. Good.

Now, let's think about if in leaf, after computing H[i], we don't update val[i+1] with H[i]; direct sum for i+1 will include H[i]. Good.

Now, let's consider if in leaf, H[i] could be used in apply after leaf for right segments. nonzero appended. Good.

Now, let's think about if in apply, `nonzero` list includes H from left leaf and earlier. Good.

Now, let's consider if in CDQ, the order of appending nonzero in left solve is increasing, so bisect works. Good.

Now, let's think about if in naive, no CDQ. OK.

Now, let's consider if final code might be too long but acceptable.

Now, let's think about if there is any issue with using `pow` for invfact[N] when N=0? N>=1. OK.

Now, let's consider if in precompute rate2, `pow(G, (MOD - 1) >> (i + 2), MOD)` for i=29 exponent0, ok. For i maybe 30? range30 i0..29. OK.

Now, let's think about if `max_log` could be >29 if N huge, but constraints no. If N=2e5, 18. OK.

Now, let's consider if in `ntt`, `rtz[s]` for s up to half-1, half maybe 131072, rate_tz length131073. OK.

Now, let's think about if in `intt`, `itz[s]` same.

Now, let's consider if in `ntt`, `now` update after last s uses rtz[s] maybe out of range? s max w-1 <= half-1. OK.

Now, let's think about if in `ntt`, for ph small, w small, s small. OK.

Now, let's consider if in `ntt`, `p` can be large, `end=base+p` <= n? For ph=1, w=1, base0,p=n/2,end=n/2. For ph=h, w=n/2, base=(n/2-1)*1? shift=1, base=n-2,p=1,end=n-1? Wait for ph=h, p=1, shift=1, s max w-1=n/2-1, base=n-2, end=n-1. But we need process pairs (n-2,n-1)? What about pair (n-1?) Let's check ACL butterfly for n=4,h=2,ph=2,w=2,p=1,shift=1. s=0 base0 process (0,1); s=1 base2 process (2,3). end=base+p=3, range(2,3) i=2, j=3. Good. For n=8,ph=3,w=4,p=1,shift=1: s=0 base0 (0,1); s=1 base2 (2,3); s=2 base4 (4,5); s=3 base6 (6,7). end=7, range(6,7) i=6,j=7. Good. So end=base+p, range excludes end, j=i+p <= base+p = end, for i=end-1, j=end. For s last, end=n-1? Wait n=
