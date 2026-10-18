# Item 14 topology-v6 result

Status: halted, invalid calibration evidence. Topology-v6 is preserved exactly as observed. It produced no valid candidate observation and supports no role or topology selection.

## Exact observed boundary

| Observation | Intended executor | Arm | Result | Provider calls | Tokens | Cost |
|---|---|---|---|---:|---:|---:|
| `v6-r03-p1-direct` | `realistic.mercury25.screening-default.executor.topology-v6` | Direct | preflight `test_invalidity`; no candidate inference | 0 | 0 | $0.00 |

The campaign was claimed and the sealed R03 v2 workspace was prepared, but the executor was rejected before inference. The durable controller receipt records `TerminalState: invalid`, `FailureClass: test_invalidity`, exact zero-call accounting, and no provider receipts. The campaign therefore remains at revision 1 with its original active claim. That state is evidence and must not be repaired, reclassified, or resumed.

## Cause

The topology-v6 executor used a deliberately fresh configuration identity. The inherited `BuildOpenCodeRealisticActionPlan` first calls `IsFrozenRealisticCandidate`, which admits only an exact `RealisticCandidate` tuple already present in realistic matrices v1-v11. Mercury 2.5 exists in older matrices under other configuration IDs, but the fresh topology-v6 identity does not. The builder rejected it as not in the frozen realistic matrix before OpenCode or OpenRouter ran.

The inherited-binding audit also found the same historical-matrix assumption in `SealRealisticExecutionReceipt`. Fixing only plan construction would allow a provider call but reject any completed fresh-v7 executor at sealing, creating another paid invalid calibration. Search across the realistic execution, verification, evidence, and command paths found these two `IsFrozenRealisticCandidate` call sites as the executor identity gates.

This is a harness binding defect, not a Mercury result. No statement about Mercury competence, reviewed-versus-direct performance, reliability, tokens, speed, or cost can be derived from v6.

## V7 binding audit recommendation

Do not weaken `IsFrozenRealisticCandidate`, edit the original realistic matrices, or change `BuildOpenCodeRealisticActionPlan`; those bytes are part of the accepted realistic freeze.

A fresh v7 should instead add generation-owned executor plan and receipt-sealing boundaries parallel to the accepted generation-owned reviewer builder. They should:

1. Accept both the executor frozen by v7 and the requested executor, then require exact equality across configuration ID, model, effort, and serving provider.
2. Build the same sealed realistic-action prompt, schema, OpenCode configuration, no-tool policy, exact OpenRouter model ID, single provider order, and `allow_fallbacks: false` without consulting or mutating the historical realistic candidate matrix.
3. Keep `RequireCompleteRealisticFreeze` as an upstream admission gate so the case, fixture, controller, verifier, and evidence contract remain unchanged.
4. Seal completed receipts only when the candidate exactly equals the executor frozen by v7, while preserving the existing raw-receipt recomputation, exact-accounting, fixture, evidence, and hidden-verifier checks.
5. Be included in the v7 freeze and protected by tests proving the named v7 executor is accepted for both planning and sealing, wrong identity/model/effort/provider variants are rejected at both boundaries, the original matrix-bound plan and sealer still reject unlisted candidates, and every preflight rejection happens before a provider call.
6. Preserve all v1-v6 bundles and evidence recursively.

No v7 implementation or provider call should begin until the complete inherited-binding audit confirms there are no additional historical matrix assumptions in execution, sealing, grading, routing, or recovery.
