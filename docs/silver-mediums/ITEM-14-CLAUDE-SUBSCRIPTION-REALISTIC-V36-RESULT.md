# Item 14 Claude Subscription Realistic v36 — Result

Status: **complete candidate-local efficiency failure; testing only**.

The frozen direct batch required R03 v2 to pass before R04 v1. Claude subscription `claude-sonnet-4-6` Medium ended R03 as `observation_resource_exhaustion` after the terminal provider turn exceeded the frozen 200,000-micro cost-equivalent ceiling.

| Case | Result | Calls | Input | Output | Reasoning reported | Cost-equivalent | Retries |
|---|---|---:|---:|---:|---:|---:|---:|
| R03 v2 | FAIL — resource exhaustion | 10 | 57,508 | 2,371 | 0 | 206,700 micros | 0 |
| R04 v1 | Policy-skipped — candidate eliminated | 0 | 0 | 0 | 0 | 0 | 0 |

Before the ceiling was crossed, the controller completed four reads, four scoped edits, tests, and a rollback check. It did not reach verification or finish. Its deterministic native failure was `response_or_tool_protocol_failure`; the campaign-level observation is resource exhaustion because the terminal turn raised the accumulated cost-equivalent above the frozen R03 cap.

Evidence:

- Freeze SHA-256: `86ef9a33a047ede5f004ef73f35d76c6d8ef3400f32b8a89a4a2f025867afe6b`
- Definition SHA-256: `572a649ad9e65d9ab3106aeeaf941d8e373322aba7befed686b6861ebf66eb44`
- Ledger SHA-256: `5050160aed2233d80447e6fbf34e43b559b1c1f863d02ef45c8cd492b1b31a24`
- Evidence root: `.qualification-evidence/claude-subscription-realistic-v36`
- Provider-free replay: two claims valid, zero calls, exact usage
- `candidate_eliminated=true`; `global_halt=false`

The 206,700-micro amount is provider-reported subscription cost metadata, not marginal cash spend. This result eliminates only this exact configuration under the frozen R03 efficiency cap. It does not eliminate Claude globally or determine other efforts, protocols, or topologies.

No model, role, squad, topology, permission, or production configuration is selected.
