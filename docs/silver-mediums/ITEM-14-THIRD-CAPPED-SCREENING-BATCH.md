# Item 14 third capped screening batch

Date: 2026-09-11  
State: `complete_screening_only`  
Result: `ITEM-14-THIRD-SCREENING-RESULT.md`  
Evidence lane: two isolated local candidate runs; no production/external work

## Purpose

V3 preserves the same T11 case, models, medium effort, skill, permissions,
token ceiling, subscription brake, blind grader, and one-run-at-a-time policy.
It changes only the two harness defects proven by the stopped v1 and v2 runs:

- Candidate instructions now follow the case's exact response contract.
- Candidate facts are delivered inline in the triggering comment rather than
  stored only in an issue description that a no-tools agent cannot fetch.

## Locked launch sequence

1. Create a disposable local issue without assigning an agent and without a
   repository or project.
2. Keep the issue non-running. Do not create an assignment-triggered task.
3. Post one owner comment that explicitly mentions only the seeded candidate
   and contains the exact candidate packet bytes after the mention.
4. Confirm exactly one comment-triggered run exists and its prompt transport is
   `comment_trigger_inline_v1`.
5. Close the run family, reconcile the complete usage receipt, validate the
   response contract, and seal blind scoring before joining model identity.
6. Stop before any second candidate unless the observation is valid and a new
   authoritative account read remains below the batch brake.

The candidate is allowed to return its JSON response but remains forbidden
from tools, files, issue reads, mutation, delegation, external contact, and
production. The triggering comment is the complete case input.

## Automated gates

- V1 is rejected because its array requirement conflicts with T11's object.
- V2 is rejected because direct issue assignment does not inline case facts.
- V3 pins the exact instruction SHA-256 and every skill content SHA-256.
- The daemon regression test proves a T11 comment prompt contains the case id,
  owner message, and response contract; it also proves direct assignment does
  not contain those facts.
- Agentqualification, systemgraph, handler, and server focused suites pass.

V3 completed within both caps but is itself invalidated. Its public packet did
not disclose JSON field types, its assigned skill required a conflicting field
set, and the hidden grader enforced undisclosed arrays. Read
`ITEM-14-THIRD-SCREENING-RESULT.md` before planning another batch. All invalid
paid runs remain counted as experimental cost and are never treated as model
failures or successes.
