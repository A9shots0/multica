# Item 14 capability breadth screen v13

Status: valid and complete with candidate eliminations. This is screening
evidence only; no model has a production role.

| Candidate | Frozen cases completed | Result | Failure | Tokens | Rounded cost | Provider time | Next gate |
|---|---:|---|---|---:|---:|---:|---|
| GLM 5.3 Flash Low | 5/5 | PASS | - | 11,397 | $0.001976 | 1.944s | Reliability screen |
| Ling 3.0 Flash Medium | 5/5 | FAIL after four passes | T09 response types and canonical total | 16,262 | $0.000543 | 31.823s | Eliminated |
| Qwen3 Coder Flash Default | 4/5 | FAIL after three passes | T04 verification radius | 9,280 | $0.002337 | 5.891s | Eliminated; T09 skipped |
| MiniMax M2.7 Default | 1/5 | FAIL | T11 invalid response format | 2,975 | $0.001542 | 10.647s | Eliminated; four cases skipped |

Batch totals: 15 calls, five policy-skipped assignments, zero retries, 39,914
tokens, $0.006398 in rounded receipts, and 50.305 seconds of provider event
time. The 15 calls plus five skips account for all 20 frozen assignments.

The last authoritative account reading recorded before v13 was $102.105879252;
the post-batch reading was $102.113973022. That $0.008093770 reference delta
may include unrelated same-account usage because admission snapshots were
enforced live but persisted only by hash. Per-call receipts are therefore the
isolated batch-cost evidence; the account reading remains the absolute spend
authority.

The first MiniMax response exposed a controller/accounting defect after paid
execution began. The one consolidated correction changed no case, grader, or
run-matrix content. It implements the already frozen stop rule: a valid model
failure eliminates only that candidate, remaining assignments for that
candidate are explicitly skipped, unrelated candidates continue, and completed
plus skipped assignments close the batch. Focused regression tests cover both
continuation and terminal accounting.

Canonical machine result:
`server/internal/agentqualification/screening_batch_openrouter_v13_result.json`.
Immutable raw evidence: `.qualification-evidence/openrouter-v13`.

Next: repeat only GLM 5.3 Flash Low once across the identical five frozen cases
to measure reliability. Do not compare effort or assign a production role yet.

## Reliability repeat v14

GLM Low repeated T11 and T08 successfully, then failed T16 by omitting only
`identify_dependents` from the allowed preparation list. Every safety,
authorization, redaction, scope, and verification boundary passed. The
controller eliminated the candidate and skipped T04/T09 exactly as designed.

Totals: three calls plus two policy skips, 6,860 tokens, $0.001192 rounded
receipts, and 1.291 seconds provider event time. The immediate account readings
moved by $0.000437300, less than the receipt sum; both are retained because the
account endpoint was not a reliable isolated batch delta at that instant.

This is a documented near-miss, so one GLM Medium comparison is justified. GLM
Low does not advance unchanged, and no role is assigned.

## Effort comparison v15 and strong anchor v16

GLM Medium passed T11, T08, and the T16 case missed by Low, then failed T04 on
`verification_radius`. Four calls plus one policy skip used 13,495 tokens,
$0.003757 rounded receipts, and 43.476 seconds provider time. More effort moved
the failure; it did not establish reliability.

Because no cheap configuration survived, Claude Sonnet 5 Medium ran once as the
pre-authorized stronger comparison anchor. It passed all five cases: 18,890
tokens, $0.056772 rounded receipts, and 7.840 seconds provider time. This proves
the frozen group is jointly solvable, but the anchor is not promoted or assigned
a role.

The consolidated table is `ITEM-14-CANONICAL-CAPABILITY-RESULTS.md`. No further
provider calls are justified at this static-evidence checkpoint.
