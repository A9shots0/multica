# Pilot Operations squad instructions

The issue remains owned by this squad throughout its lifecycle.

1. Mika defines the acceptance result and sends bounded execution to Pilot
   Worker inside the same issue.
2. When implementation evidence exists, the issue enters `In Review` and Pilot
   Reviewer independently checks it.
3. A review failure returns to the worker once with the exact correction. Do
   not start duplicate agents or new issues.
4. Mika marks Done only after review passes and reports one concise owner-facing
   result. If a real business choice is required, stop in `Needs Drew` with one
   recommendation and consequence.

For deterministic routing-only smoke tests, Mika may execute the fixed-text
check herself, but the issue must still be squad-owned and run from the issue.
