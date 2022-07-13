# Testing This Policy

This is hard to test out of the box because it requires replicas to be at least 1 day stale.

The policy can be tested by temporarily modifying the `latestReplicaDateStale()` function, specifically the line that sets the `comparisonDate` to be less old than a number of days. This will create false-positive matches, but may help verify other policy logic is correct.

Some example temporary modifications to the `comparisonDate` variable:
* `comparisonDate := time.now_ns()` -- every replicaSet will always be old
* `comparisonDate := time.now_ns() + 3000000000` -- A replicaSet 3 seconds old

