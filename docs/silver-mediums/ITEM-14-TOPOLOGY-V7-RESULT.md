# Item 14 topology-v7 result

Status: complete, mixed descriptive topology evidence. This batch selects no topology, model, role, squad, or production permission.

Topology-v7 ran the unchanged frozen R03 v2 case using Mercury 2.5 at `screening_default` as executor and Claude Sonnet 5 Medium as the independent reviewer. It completed two counterbalanced Direct/Reviewed pairs with zero retries, zero owner interventions, and zero external side effects.

| Observation | Pair | Arm | Result | Calls | Tokens | Cost | Time |
|---|---:|---|---|---:|---:|---:|---:|
| `v7-r03-p1-direct` | 1 | Direct | FAIL — `competence_failure` | 10 | 34,642 | $0.002941 | 89.682s |
| `v7-r03-p1-reviewed` | 1 | Reviewed | PASS; review 1 passed, no correction | 11 | 44,257 | $0.037474 | 112.784s |
| `v7-r03-p2-reviewed` | 2 | Reviewed | INVALID — `stage_budget_exhaustion`, observation-local | 9 | 34,050 | $0.003105 | 90.642s |
| `v7-r03-p2-direct` | 2 | Direct | PASS | 10 | 33,841 | $0.002800 | 87.549s |

Matched-pair raw deltas are Reviewed minus Direct:

| Pair | Result difference | Call delta | Token delta | Cost delta | Time delta |
|---|---|---:|---:|---:|---:|
| 1 | Direct FAIL; Reviewed PASS without correction | +1 | +9,615 | +$0.034533 | +23.102s |
| 2 | Reviewed INVALID; Direct PASS | -1 | +209 | +$0.000305 | +3.093s |

Pair two is not a valid topology comparison because its Reviewed observation exhausted the frozen execute-stage generated-token allowance. The paid ninth response was sealed and counted exactly, the observation was excluded without retry, the campaign did not halt, and the unaffected Direct observation continued.

Arm and campaign totals:

- Direct: 20 calls, 68,483 tokens, $0.005741, 177.231 seconds.
- Reviewed: 20 calls, 78,307 tokens, $0.040579, 203.426 seconds, with one invalid observation.
- Raw Reviewed-minus-Direct: 0 calls, +9,824 tokens, +$0.034838, +26.195 seconds. This is not a valid aggregate efficacy comparison because the Reviewed arm contains an invalid observation.
- Campaign: 40 calls, 146,790 tokens, $0.046320, 380.657 seconds of summed observation time.

## Interpretation

The executor outcome varied across repeated runs on the identical frozen case: Direct failed once and passed once. Reviewed also produced one pass and one invalid observation. This is useful reliability evidence against drawing conclusions from a single successful run.

Pair one does not prove reviewer rescue. Its Reviewed executor produced a different candidate that passed review immediately; no correction loop ran. The independent reviewer did not repair the failed Direct candidate. Pair two cannot measure review benefit because execution exhausted its stage allowance before review.

Therefore v7 proves neither that review improves Mercury nor that Direct is preferable. It does show that the reviewer dominates cost in the one completed Reviewed path: the single review added $0.034547, while the corresponding executor cost $0.002927. More repetitions and valid paired observations would be needed before any topology inference.

Canonical machine evidence:

- `.qualification-evidence/topology-v7/campaign.json`
- `.qualification-evidence/topology-v7/observations/*/result.json`
- `.qualification-evidence/topology-v7/observations/*/run-state.json`
- `.qualification-evidence/topology-v7/observations/*/hidden-grade.json` where produced

Accounting is exact. Topology-v6 remains separate zero-call invalid calibration evidence. No historical evidence was rewritten and no production selection was made.
