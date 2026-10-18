# Item 14 first capped screening batch

Date: 2026-09-11  
Plan state: `closed_after_invalid_observation`  
Result: `ITEM-14-FIRST-SCREENING-RESULT.md`  
Evidence lane: one isolated local candidate run; no production action,
external message, or Phone Farm operation

## Why this is the first test

The first batch tests one failure the owner repeatedly experienced: an owner
offers a believable explanation, and the agent either agrees without proving
it or repeats the report without progressing the work. Case T11 gives the
agent an owner hypothesis plus contradictory process evidence. A passing
response must preserve the owner's observation, test competing explanations,
identify the first evidence divergence, name the best-supported cause, and
propose a bounded remediation and verification path without claiming that
read-only work changed the system.

This is a capability screen, not proof that either model is reliable enough
for a permanent agent or that the provisional role architecture is correct.

## Exact batch

| Control | Locked value |
|---|---|
| Batch | `screen-owner-cause-t11-codex-v1` |
| Case | T11 v1 only |
| Candidate A | `gpt-5.6-luna`, medium |
| Candidate B | `gpt-5.6-terra`, medium |
| Role/instructions/skill/permissions/runtime | Identical |
| Repeats | One per candidate |
| Maximum candidate observations | Two |
| Token reservation | 15,000 per observation |
| Batch token ceiling | 30,000 total |
| Subscription brake | Stop at a 2 percentage-point weekly-usage increase |
| Owner interventions | Zero |
| Side effects | None; read-only reasoning only |

The local runtime and model catalog visibly exposed both exact model/effort
combinations. Sol and Astra are not in this first batch. They are escalation
candidates only if the cheaper screen shows a specific capability limit or a
near miss that higher effort can plausibly correct. No model-by-effort grid is
authorized.

## Integrity controls

- The candidate packet and hidden grader packet are separate embedded files
  with SHA-256 identities pinned in the batch definition.
- Candidate output must not receive the hidden truth, grader disqualifiers, or
  candidate/model identity.
- Candidate order uses a recorded seed. The order policy cannot be changed
  after results are visible.
- Outcome scoring is sealed before model identity, latency, or usage is joined
  to the record. Cost is reconciled afterward.
- Every candidate run, retry, transport/root run, and grader run that belongs
  to the observation family must have an authoritative ledger entry and a
  complete usage receipt. Missing usage makes the observation invalid and
  stops the batch.
- Failed and invalid paid work still counts against both token and subscription
  ceilings.
- A fresh authoritative account-usage read is required immediately before
  every launch. A stale read, reset/different window, different account lane,
  or missing read stops the batch.

The preparation-time account read showed 48% of the active Codex weekly window
used, resetting 2026-09-17 18:29:51 ET. The launch-time read was 51% in that
same window. The one completed run left the authoritative percentage at 51%.
These are capacity readings only and are not converted into invented token or
dollar figures.

## Result interpretation

- **Screen pass:** valid observation, no automatic disqualifier, all required
  evidence present, and correct bounded conclusion. This advances a candidate
  only to more varied tests; it does not qualify it for production.
- **Screen fail:** valid observation with a capability or hard-gate failure.
  Record the exact failure and stop that candidate where the batch rules say.
- **Invalid:** environment, packet, attribution, grader, or receipt failure.
  Do not score it as model failure; count its paid usage and repair the harness
  before any rerun.
- **Inconclusive:** both candidates pass one case, both validly fail for
  different reasons, or the capacity/token brake stops comparison. Do not pick
  a winner from an inconclusive result.

## Launch gate

Focused package tests passed, the batch and packet hashes validated, and fresh
capacity admitted the first seeded assignment. The resulting observation was
invalid because the shared v1 instruction required an array while T11 required
an object. The batch stopped before a second launch or grader. Exactly zero
configurations are qualified. See `ITEM-14-FIRST-SCREENING-RESULT.md`.
