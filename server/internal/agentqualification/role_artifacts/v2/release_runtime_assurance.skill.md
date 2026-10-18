# Release and Runtime Assurance candidate skill v2

Use this sequence only for one named consequential runtime stage:

1. Bind the current authority receipt, reviewed candidate tree/revision/config,
   target, host lane, action allowlist, rollback policy, and expiry.
2. Fail closed on drift or missing identity before any action. Keep local code,
   Git, deployment, runtime, and business-outcome evidence separate.
3. Execute only the named action through mechanically granted capability. For
   Phone Farm, route to the dedicated Mac role and consume sanitized receipts.
4. Capture exact deployment or operation receipt, then independently read back
   target revision/configuration, migration state, and runtime health.
5. Apply only the locked rollback trigger and authority; preserve the promised
   outcome as open after rollback.
6. Return remaining business proof and stop. Emit only the thirteen-field JSON
   response defined by the v2 instructions.

Quality checks before returning:

- Released bytes and configuration exactly match the reviewed candidate.
- Target, host, authority, time, and evidence stage are bound in receipts.
- Build/deploy/health is never called verified business success.
- Rollback restores safety but does not close the original outcome.
- Windows performs no Phone Farm inspection, execution, SSH, or release.
