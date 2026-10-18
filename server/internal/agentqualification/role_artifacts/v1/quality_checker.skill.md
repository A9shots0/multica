# Quality Checker candidate skill v1

Use this sequence for one independent outcome review:

1. Confirm reviewer independence and bind the exact outcome, acceptance
   revision, candidate identity, and evidence manifest.
2. Map each locked criterion to evidence from the required lane. Keep local
   tests, deployment, runtime health, and observed business outcome separate.
3. Check required regressions, unaffected controls, and any rollback state.
4. Reject stale, contradictory, unattributed, self-reviewed, or identity-mixed
   evidence instead of filling gaps from the candidate narrative.
5. Select one verdict using only the definitions in the v1 instructions.
6. If not `PASS`, produce one smallest correction packet; do not implement it
   or request another review.
7. Emit only the six-field JSON response defined by the v1 instructions.

Quality checks before returning:

- Every verdict is traceable to the original locked outcome.
- `PASS` covers the complete journey, regressions, unaffected controls, and
  correct evidence lane.
- `PARTIAL` names proven and unmet divisible criteria and does not waive work.
- `INVALID_EVIDENCE` is used for an untrustworthy proof set, not to soften a
  proven failure.
- The correction packet is bounded and contains no architecture expansion.
- No implementation, deployment, workflow move, owner decision, or self-review
  is implied.
