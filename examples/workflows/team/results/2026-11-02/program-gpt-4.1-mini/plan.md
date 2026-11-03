## Plan

1. Read the input string S.
2. Initialize prefix counts for 'a', 'b', and 'c' to 0.
3. Create a dictionary to map (diff_a_b, diff_b_c) to frequency of prefix occurrences.
4. Initially, add (0, 0) with frequency 1 to represent empty prefix.
5. Iterate over the string, update counts and compute current (diff_a_b, diff_b_c).
6. For each prefix, add the frequency in the map for the current diff pair to the answer.
7. Increment the frequency of the current diff pair in the map.
8. Print the accumulated answer.

This algorithm runs in O(N) time and uses O(N) memory which fits the constraint N ≤ 200000.

