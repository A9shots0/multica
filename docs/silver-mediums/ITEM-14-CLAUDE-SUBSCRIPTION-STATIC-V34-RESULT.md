# Item 14 Claude Subscription Static Breadth/Reliability v34 — Result

Status: **complete static breadth pass; testing only**.

Claude Code subscription 2.1.270-compatible `claude-sonnet-4-6` at medium effort passed all six frozen cases in the required order: T12 v4, T11 v4, T08 v3, T16 v3, T04 v1, and corrected T09 v2.

| Result | Calls | Input | Output | Provider cost-equivalent | Retries |
|---|---:|---:|---:|---:|---:|
| 6/6 PASS | 6 | 35,643 | 4,082 | 253,410 micros | 0 |

The cost figure is provider-reported subscription metadata, not marginal cash spend. The candidate was not eliminated and the campaign did not globally halt.

Evidence:

- Freeze SHA-256: `424041cd761a56b72db4bb92c48f7518901f0975f608d37e3cb687ba0a92f9a3`
- Ledger SHA-256: `21c3089b90b841513d3107477df339fe874aec9c93d71e3d8fbe37aac8207163`
- Evidence root: `.qualification-evidence/claude-subscription-static-v34`
- `candidate_eliminated=false`; `global_halt=false`

The original provider-free replay produced a false-positive mismatch because it compared semantically equivalent omitted slices as nil versus empty. The frozen provider evidence and grades were not changed. The separate post-batch verifier at `server/cmd/claude-subscription-static-v34-grade-replay-v1` re-parses every stored response, recomputes the frozen grader result, compares grade semantics, and validates all six PASS results with zero provider calls. Its focused tests pass.

This result does not select a model, role, squad, topology, permission, or production configuration. The next existing testing step is a separately frozen static reliability repeat; it is not production promotion.
