# Item 14 fourth screening result

Date: 2026-09-11  
Verdict: `INVALID`; batch stopped after one of two planned observations

## Owner result

V4 does not show that Luna is better or worse than Terra. It showed that the
normal Multica agent workflow is the wrong lane for low-cost isolated model
screening: the agent must inspect Multica and post its answer, so a supposedly
tool-free one-turn comparison became a seven-tool workflow with a large
repeated context bill. No second model run was admitted.

## Sealed evidence

- Candidate: `screen.generalist.luna.medium.v4`
- Issue: `SILV-37` / `01a092a5-3650-770b-badc-5828686c43fe`
- Trigger comment: `01a092a5-5f05-7ce6-a1fb-3c746b14153f`
- Run: `01a092a5-5f12-7a8f-8c17-e51f0090719c`
- Session: `01a092a5-6970-7bd2-b6b7-767783cd4267`
- Output SHA-256: `f642717808b38d9e309fcaeef78b419216b317173819ee7fca199d8c395c2088`
- Runtime receipt: one completed run, zero retries, zero owner interventions,
  seven tool calls
- Usage: 19,910 fresh input + 153,856 cached input + 2,617 output = 176,383
  counted tokens
- Capacity: authoritative weekly account read remained 54% before and after
- External boundary: no production, external message, repository checkout, or
  Phone Farm operation occurred

The exact machine-readable receipt is
`server/internal/agentqualification/screening_batch_v4_result.json`.

## Grader correction

The first deterministic grade incorrectly rejected `untouched_scope` because
it required exact phrases even though the response preserved database roles,
authentication payload, CRM identity and grants, and downstream contracts.
The corrected grader accepts those equivalent expressions. Regrading the
unaltered output gives eight passing checks and one failure: `owner_outcome`.
The response correctly rejected the proposed canonical-role rewrite but did
not phrase the result as the employee regaining access to the affected page.

This counterfactual grade is useful prompt evidence only. It does not change
the observation from `INVALID` because the execution contract and token budget
were both violated.

## Required next lane

Before another paid comparison, qualification needs a direct one-turn runner:

1. isolated Codex home with authentication only;
2. no repository, workspace skills, Multica issue transport, or inherited
   project instructions;
3. public packet and JSON schema supplied directly in the initial turn;
4. final output captured directly instead of posted through tools;
5. automatic invalidation on any tool event;
6. exact usage receipt and a reservation based on measured one-turn overhead;
7. a zero-model dry run proving hashes, ordering, schema, storage, and stop
   behavior before the next paid launch.

Actual Multica workflow trials remain necessary later for finalists. They are
an end-to-end reliability phase, not the cheap first-pass model screen.
