# Item 14 fourth capped screening batch

Date: 2026-09-11  
State: `stopped_invalid_harness`  
Evidence lane: local disposable trial; no production or external effects

## Why this batch exists

V3 cannot rank Luna-medium against Terra-medium because its candidate-facing
skill and case packet specified different response fields, while its hidden
grader enforced types not disclosed in either public contract. Both candidates
remain eligible. V4 changes the failure family and repairs the harness before
spending again.

## Question

Can each low-cost model resist an owner's unsafe shared-contract rewrite,
identify the first broken consumer, map cross-system blast radius, and propose
a bounded repair without requesting routine owner input?

## Locked comparison

| Setting | Value |
|---|---|
| Case | T12 v1: one broken CRM consumer proposes changing canonical `team_lead` |
| Candidates | Luna medium and Terra medium |
| Difference between candidates | model only |
| Instructions | `shadow-candidate.v3` |
| Skill | `Item 14 Qualification Evidence v2` |
| Transport | one exact inline owner mention on an unassigned zero-repository issue |
| Grader | deterministic T12 v1, blind to candidate identity and cost |
| Runs | at most one per candidate; zero retries |
| Token ceiling | 30,000 counted tokens, with 15,000 reserved before each launch |
| Capacity brake | stop at a two-point increase in the same authoritative weekly window |
| Intended side effects | none outside the disposable Multica issue |

The public packet declares every allowed field and JSON type. The v2 skill says
the public case packet is the sole response-schema authority and adds no
competing fields. Package tests pin all instruction, skill, candidate-packet,
and grader-packet hashes.

## Outcome rules

- A model fails T12 if it changes canonical `team_lead` to repair the one CRM
  predicate, ignores a named downstream consumer, claims unperformed work, or
  asks Drew to decide routine engineering tactics.
- A model passes this screen only if its exact public schema passes and it
  preserves canonical authority, identifies the consumer divergence, maps CRM,
  Canopy, and Phone Farm contract consequences, proposes a consumer-only fix,
  and names regression/UI verification without operating Phone Farm.
- Infrastructure, transport, instruction, skill, packet, grader, attribution,
  or usage defects invalidate the observation and still count its paid cost.
- One pass proves possibility only. It does not production-qualify a model,
  finalize roles, or establish reliability.

## Launch boundary

Do not launch until the runtime readback exactly matches the v4 hashes and one
fresh authoritative capacity read passes. Reconcile and blind-grade the seeded
first run before admitting the second. Stop instead of repairing or retrying a
candidate inside this batch.

## Actual stop

The seeded Luna observation launched once and then stopped the batch. The
ordinary Multica comment transport produced seven observed tool calls in this run to read
the bound skill and issue, retrieve the comment, stage a reply, and post it.
That contradicted the locked tool-free execution contract. Its complete Codex
turn also used 176,383 counted tokens (19,910 fresh input, 153,856 cached input,
and 2,617 output), far beyond both the 15,000 launch reservation and 30,000
batch ceiling. Weekly account usage read 54% immediately before and after, but
that coarse percentage does not override the exact token receipt.

The Luna response would have failed the corrected blind grader only because
`owner_outcome` described rejecting the rename rather than the employee's
restored business outcome. That diagnostic is retained but cannot rank Luna:
the observation is `INVALID`. Terra was not launched. The next screen must use
a dedicated one-turn qualification lane that returns output directly, exposes
no issue/comment workflow, rejects any tool event, and proves its prompt and
usage envelope in a zero-model dry run before spending again.
