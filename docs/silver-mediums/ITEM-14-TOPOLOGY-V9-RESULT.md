# Item 14 topology-v9 reciprocal R04 result

Status: complete, mixed descriptive testing evidence. This batch selects no
model, topology, role, squad, or production permission.

Topology-v9 ran the unchanged frozen R04 v1 exact-once resume case with Claude
Sonnet 5 Medium as executor and GPT-5.4 Medium as independent reviewer. It
completed the fixed counterbalanced sequence with zero retries, zero owner
interventions, and zero external side effects.

| Observation | Pair | Arm | Result | Calls | Tokens | Cost | Time |
|---|---:|---|---|---:|---:|---:|---:|
| `v9-r04-p1-direct` | 1 | Direct | FAIL — `competence_failure` | 9 | 29,248 | $0.081457 | 67.539s |
| `v9-r04-p1-reviewed` | 1 | Reviewed | INVALID — `stage_budget_exhaustion`; no retry | 17 | 60,242 | $0.205454 | 159.977s |
| `v9-r04-p2-reviewed` | 2 | Reviewed | FAIL — `competence_failure` | 9 | 33,323 | $0.129142 | 92.939s |
| `v9-r04-p2-direct` | 2 | Direct | FAIL — `competence_failure` | 7 | 21,324 | $0.061505 | 53.060s |

Pair one is excluded because the Reviewed correction used its frozen stage
allowance; the paid crossing response was sealed, the next call was prevented,
and the observation was preserved without retry. Pair two is valid: both arms
failed. Review added 2 calls, 11,999 tokens, $0.067637, and 39.879 seconds
without changing the outcome.

Direct totaled 16 calls, 50,572 tokens, $0.142962, and 120.599 seconds.
Reviewed totaled 26 calls, 93,565 tokens, $0.334596, and 252.916 seconds, but
includes one invalid observation. Campaign total was 42 calls, 144,137 tokens,
$0.477558, and 373.515 seconds. Accounting is exact.

The reciprocal result does not show a reviewer advantage. Claude failed R04 in
both valid Direct and Reviewed observations, while review materially increased
cost and time. This remains capability evidence only and makes no production
selection.

Canonical machine evidence:

- `.qualification-evidence/topology-v9/campaign.json`
- `.qualification-evidence/topology-v9/observations/*/result.json`
- `.qualification-evidence/topology-v9/observations/*/run-state.json`
- `.qualification-evidence/topology-v9/observations/*/hidden-grade.json` where produced
