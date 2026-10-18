# Item 14 realistic reliability repeat

Date: 2026-09-12
Evidence stage: `local-test-design-reviewed`; no provider call, production action, role
selection, or Phone Farm operation is authorized by this document

## Purpose

Measure whether the sole valid R01-R04 survivor can reproduce its result once
under the identical material configuration. This is a reliability observation,
not a production qualification or a claim that Claude is the correct role
model.

## Frozen case group

The repeat uses the already reviewed and frozen realistic group without
changing fixtures, candidate instructions, graders, tools, permissions, or
evidence requirements:

| Stage | Cases | Gate |
|---|---|---|
| V10 | R01, R03, R04 in order | Stop this candidate after its first valid failure; do not retry or modify the case |
| V11 | R02 | Admit only if V10 passes all three cases; use the trusted in-app Browser capture boundary |

The material candidate configuration stays Claude Sonnet 5, Medium effort,
OpenRouter transport, Anthropic as the exact serving provider, no fallbacks,
the same frozen instructions and tool policy, zero retries, and zero planned
owner interventions. Matrix-specific IDs distinguish new observations from
the earlier capability observations; they do not represent a changed material
configuration.

## Accounting and stop behavior

- V10 ceiling: 225,000 total tokens and $5.00 incremental provider spend,
  with a 75,000-token reservation before each of its three observations.
- V11 ceiling: 75,000 total tokens and $5.00 incremental provider spend, with
  its full 75,000-token reservation required before its single observation.
- Every provider call, token, cost, tool event, elapsed time, retry, and owner
  intervention remains in the immutable observation receipts.
- A valid competence/protocol failure records and stops only this candidate's
  remaining repeat cases.
- The batch halts only for test invalidity, evidence corruption, unsafe
  execution, provider transport failure, or budget breach.
- Pending matrices can be inspected and planned but cannot execute. Provider
  execution remains code-locked until each selected matrix is independently
  reviewed and marked `frozen_reviewed`.
- V11 remains unassignable until the same campaign ledger contains valid PASS
  observations for V10 R01, R03, and R04.
- The 20,000 generated-token ceiling applies cumulatively to output plus
  reasoning across the complete observation, not separately to each call.

## Required review before calls

One independent review inspected the complete existing frozen R01-R04 case
and grader group, V10/V11 run matrices, exact material-configuration parity,
accounting, candidate-local stop behavior, Browser boundary, and the pending
matrix execution lock. It found three P1 enforcement blockers: V11 did not
mechanically require V10 passes, generated-token accounting was per call rather
than cumulative, and campaign tokens were checked only after spend. One
consolidated correction added all three fail-closed gates and focused
regression tests. V10 and V11 are now frozen; no reviewer loop was opened.

## Interpretation

Two valid passes per case would establish only a small initial reliability
sample. Any valid repeat failure disproves perfect repeatability and remains
part of the dataset. After this group, the next planned work is a separately
frozen single-agent versus controller-led system comparison on an identical
outcome, counting every run and intervention. No role selection occurs here.

## Completed observations

| Case | Result | Calls | Tokens | Cost | Time | Interpretation |
|---|---:|---:|---:|---:|---:|---|
| R01 | PASS | 9 | 27,248 | $0.077104 | about 1m | Valid repeat pass |
| R03 | INVALID | 12 | 48,968 | $0.140888 | about 2m | The summary truthfully preserved `outcome_status:open`; the frozen phrase matcher rejected the colon-form field notation. The raw result and original FAIL grade remain preserved and no provider retry occurred. |
| R04 | PASS | 8 | 24,518 | $0.071996 | about 2m | Valid same-family interruption/recovery repeat, run through the unchanged frozen R04 matrix after the R03 observation was classified invalid. |

The completed paid group used 29 provider calls, 100,734 tokens, and $0.289988.
R03 is excluded from competence scoring. R04 was not blocked by that
non-safety test imperfection. The deferred R02 repeat was not part of the
owner-directed R01/R03/R04 batch and was not run. No production role was
selected.

Post-batch correction: the declaration parser now recognizes exact machine
field forms `outcome_status:open`, `outcome_status=open`,
`requested_outcome:open`, and `requested_outcome=open`. This changes no case,
artifact requirement, safety boundary, or provider answer, and the invalid R03
observation is not regraded or retried.
