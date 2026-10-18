# Item 14 topology-v5 result

Status: complete, valid descriptive topology evidence. This batch selects no
topology, model, role, squad, or production permission.

Topology-v5 ran the unchanged frozen R03 v2 case with GPT-5.4 Medium through
the pinned OpenRouter/OpenAI configuration. It completed two counterbalanced
Direct/Reviewed matched pairs with zero retries, zero owner interventions, and
zero external side effects. All four observations passed the hidden grade.

| Observation | Pair | Arm | Result | Calls | Tokens | Cost | Time |
|---|---:|---|---|---:|---:|---:|---:|
| `v5-r03-p1-direct` | 1 | Direct | PASS | 15 | 43,375 | $0.166431 | 143.057s |
| `v5-r03-p1-reviewed` | 1 | Reviewed | PASS | 15 | 45,884 | $0.167153 | 126.408s |
| `v5-r03-p2-reviewed` | 2 | Reviewed | PASS | 16 | 49,211 | $0.194194 | 333.843s |
| `v5-r03-p2-direct` | 2 | Direct | PASS | 14 | 37,905 | $0.132217 | 126.282s |

Matched-pair deltas are Reviewed minus Direct:

| Pair | Result difference | Call delta | Token delta | Cost delta | Time delta |
|---|---|---:|---:|---:|---:|
| 1 | Both PASS | 0 | +2,509 | +$0.000722 | -16.649s |
| 2 | Both PASS | +2 | +11,306 | +$0.061977 | +207.561s |
| Combined | Direct 2/2 PASS; Reviewed 2/2 PASS | +2 | +13,815 | +$0.062699 | +190.912s |

Arm totals:

- Direct: 29 calls, 81,280 tokens, $0.298648, 269.339 seconds.
- Reviewed: 31 calls, 95,095 tokens, $0.361347, 460.251 seconds.
- Campaign: 60 calls, 176,375 tokens, $0.659995, 729.590 seconds of
  summed observation time.

Accounting is exact. The campaign ledger also preserves one administrative
post-batch note: the first operator invocation used a relative artifact root,
created a claim, and failed before workspace creation or provider access. That
zero-call claim was cleared; it produced zero tokens and zero cost and did not
retry a paid observation.

Interpretation is deliberately narrow. Reviewed added measurable aggregate
cost, tokens, calls, and elapsed time without changing the 2/2 outcome in this
small R03 sample. Pair one happened to finish faster under Reviewed, while pair
two was substantially slower. Four passes are not enough to prove one topology
is generally better, and this batch does not authorize a production choice.

Canonical machine evidence:

- `.qualification-evidence/topology-v5/campaign.json`
- `.qualification-evidence/topology-v5/observations/*/result.json`
- `.qualification-evidence/topology-v5/observations/*/run-state.json`
- `.qualification-evidence/topology-v5/observations/*/hidden-grade.json`

Topology-v4 remains separate invalid calibration evidence. V4 completed one
valid Direct PASS, then a candidate-local executor protocol failure exposed a
pre-call reviewer-binding harness defect that was misclassified as global
evidence corruption. Its halted 18-call, 45,599-token, $0.172581 record remains
immutable and is not combined with v5.
