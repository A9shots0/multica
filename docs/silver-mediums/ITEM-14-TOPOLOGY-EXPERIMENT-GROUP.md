# Item 14 execution-topology experiment group

Date: 2026-09-12
Status: `completed_with_invalid_observations`; no production role or topology selected
Evidence stage: frozen local synthetic execution; result in `ITEM-14-TOPOLOGY-EXPERIMENT-RESULT.md`

## Question

Does adding an independent model review and one bounded correction opportunity
improve verified outcomes enough to justify its extra runs, time, tokens, and
cost over one capable agent working the same outcome directly?

This group tests that question. It does not assume that a multi-agent system is
better, and it does not treat a polished reviewer response as a better business
outcome.

## Frozen cases

The first comparison uses two already calibrated realistic cases without
changing their fixtures or hidden outcome graders:

| Case | Capability | Why included |
|---|---|---|
| R01 v2 | Cross-system authority and current-truth repair | Tests whether review catches incomplete consumer or evidence coverage |
| R03 v2 | Regression, rollback truth, and a still-open successor outcome | Tests whether review prevents a locally successful repair from being reported as the complete business outcome |

R02 Browser and R04 interruption recovery remain separate capability evidence.
They are not added merely to make this initial comparison larger.

The group contains six observations: two independently recreated R03 pairs
and one independently recreated R01 pair. Execution order is counterbalanced:
R03 pair one runs Direct then Reviewed; R01 runs Reviewed then Direct; R03 pair
two runs Reviewed then Direct. Pair members must begin from identical task,
fixture, and initial-tree hashes. The invalid historical R03 phrase-match
observation is not used as a discriminator or seeded into either arm.

## Experimental arms

Both arms use the same Claude Sonnet 5 Medium executor configuration, exact
OpenRouter/Anthropic routing, instructions, tools, permissions, fixture, and
hidden final grader.

### A — direct bounded executor

1. One executor receives the public case and works through the existing
   deterministic tool controller.
2. The existing hidden grader evaluates the sealed final workspace and summary.
3. The grader measures the result but cannot correct it.

Expected model runs: one executor run family.

### B — reviewed bounded executor

1. The identical executor works the identical public case through the same
   deterministic tool controller.
2. A fresh independent reviewer receives the public acceptance contract,
   sealed artifact inventory, inspectable hash-bound tool/evidence contents,
   and executor summary. It does not
   receive hidden-grader rules and cannot edit or execute tools.
3. `PASS` closes the candidate for hidden final grading. `FAIL` must return a
   structured correction packet containing failed public criteria, evidence,
   and the smallest required correction.
4. One fresh correction executor may act on that packet in the same sealed
   workspace. It gets no full chat history and no owner interaction.
5. One fresh final reviewer evaluates the corrected evidence. A second failure
   ends the arm; no additional loop is allowed.
6. The unchanged hidden grader evaluates the final sealed workspace and
   owner-facing summary.

Expected stages before execution: executor plus reviewer. A correction executor
and final reviewer may be activated only after a structured first-review
failure. The entire reviewed arm reserves its worst-case token and cost budget
before the first call; dynamic stages spend only inside that reservation.

## Reviewer contract

The reviewer must return exactly one structured verdict:

- `verdict`: `PASS` or `FAIL`;
- `failed_criteria`: public acceptance-criterion identifiers only;
- `evidence`: artifact or receipt identifiers supporting the verdict;
- `required_correction`: empty on PASS; bounded and actionable on FAIL;
- `owner_decision_required`: false for these synthetic cases.

Every evidence identifier in the request has inspectable content and a
harness-recomputed SHA-256. A PASS must cite every required public evidence
identifier. The reviewer fails its own observation if it fabricates evidence, uses hidden
grader language, requests external action, changes scope, asks the owner to
relay information, or returns an unstructured essay.

## Measurement and fairness

- The same clean fixture is recreated independently for every arm/case pair.
- The direct arm is never penalized for having fewer runs; lower cost is part of
  the comparison.
- The reviewed arm receives no stronger executor, extra tools, hidden answer,
  or larger per-run model configuration. Its only treatment difference is
  independent review and at most one bounded correction.
- The hidden grader runs only after an arm ends and cannot feed corrections back
  to either arm.
- A run counts even when it fails, is invalid, or produces no useful output.
- Record provider, model, effort, prompt/config hashes, serving provider, calls,
  fresh/cached/output/reasoning tokens where exposed, dollar cost, summed run
  time, end-to-end time, retries, corrections, and owner interventions.
- Primary result: mandatory hidden-gate outcome for each arm/case.
- Secondary result: total operational burden required to reach that result.
- One two-case group can falsify a topology or show a cost difference; it cannot
  establish broad production superiority or select permanent agents.

## Budgets and stops

- Six planned arm/case observations.
- Direct executor: 75,000 total and 20,000 generated tokens.
- Reviewed initial executor: 60,000 total and 14,000 generated tokens.
- Reviewed correction executor: 36,000 total and 8,000 generated tokens.
- Reviewer: one call, 12,000 total-token reservation, 4,000 generated-token
  ceiling.
- Direct observation reservation: 75,000 total tokens, 20,000 generated
  tokens, 15 provider turns, eight minutes, and $0.30.
- Reviewed observation reservation: 120,000 total tokens, 30,000 generated
  tokens, 25 provider turns across execution/review/correction, twelve minutes,
  and $0.60.
- Group ceiling: 585,000 provider tokens and $3.00 incremental OpenRouter spend.
- Provider fallbacks: disabled. Automatic retries: zero. Owner interventions:
  zero.
- An arm/case competence or protocol failure ends only that arm/case.
- Halt the group only for credential failure, uncontrolled spending, unsafe
  external action, or evidence corruption that makes later results unusable.
- A non-safety harness defect invalidates only affected observations and is
  added to the post-group correction list; unaffected observations continue.
- A crash-stranded active claim is never retried. Recovery preserves its
  durable artifacts, reconciles incremental provider spend conservatively,
  records only that observation invalid with ambiguous accounting, and unlocks
  the next unaffected observation. A sealed result is reused exactly.

## Pre-provider validation gate

Before any call:

1. Bind both arms to byte-identical case sources and hidden graders.
2. Prove clean workspaces cannot share mutations.
3. Calibrate the reviewer against strong passes, semantic paraphrases, subtle
   failures, fabricated evidence, scope expansion, and adversarial instructions.
4. Prove a reviewer cannot edit, execute, see hidden grader rules, or create an
   unbounded correction loop.
5. Prove the complete run manifest and budget include reviewer and dynamic
   correction runs.
6. Run one independent review of the complete group, apply at most one
   consolidated correction, then freeze cases, contracts, matrix, controller,
   accounting, and stop behavior before provider calls.

No roles, squads, permanent model assignments, or production permissions are
changed by this experiment.

## Frozen interpretation

- Direct FAIL and Reviewed PASS after correction is one containment win, not
  proof of general superiority.
- Both PASS means review showed no outcome benefit on that pair; compare burden.
- Both FAIL means review failed to rescue the outcome.
- Direct PASS and Reviewed FAIL is controller harm.
- Both R03 pairs must show the same containment direction before calling it
  repeated evidence; otherwise the R03 result is mixed or inconclusive.
- R01 measures clean-path overhead and cannot independently prove topology
  quality.
