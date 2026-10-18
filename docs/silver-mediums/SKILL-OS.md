# Silver Mediums Skill OS

Status: first production rollout; behavioral observation continues

## Purpose

Multica should give each worker the smallest useful operating method without
requiring the owner to remember a slash command. The method helps the worker
finish the company outcome; it does not replace repository rules, permissions,
tests, release controls, or owner decisions.

## Runtime flow

1. Multica's server reads the task type and bounded current request fields.
2. Deterministic rules select only methods that match the work and filter the
   claim's built-in skill bundles before they reach any worker.
3. The worker receives the selected methods through Multica's existing
   provider-native skill delivery. Current daemons also render their short
   names and reasons; older daemons still receive the same exact bundle subset.
4. Brain supplies only authorized project evidence. Brain text is reference
   data, not permission, acceptance, or current truth by itself.
5. Code and tests enforce predictable gates. A model handles ambiguity,
   diagnosis, implementation, research, and owner-facing decisions.
6. Completion requires evidence for the requested outcome, not merely an agent
   report that its own step ended.

No second skill store, manual slash command, or always-loaded mega-prompt is
introduced.

## First automatic methods

| Method | Loaded for | Required behavior |
| --- | --- | --- |
| Outcome ownership | Work requests, issues, and autopilot runs | Continue to the furthest safe verified result; ask the owner only for a real business, UI, policy, or authority decision. |
| Evidence-first debugging | Failures, audits, incidents, and regressions | Reproduce or observe, form a bounded hypothesis, make the smallest change, and verify the affected workflow. Restore service first during an outage. |
| Long-work supervision | Programs, modernization, orchestration, resumed work, and squad leadership | Track real milestones and evidence, remain quiet when nothing meaningful changes, and surface only a milestone, failure, decision, or completion. |
| Research and REA | Research, comparison, audits, and reverse engineering | Separate facts from preferences and hypotheses, inspect primary evidence, compare options, and turn findings into a decision or smallest useful experiment. |
| Learn and harden | Repeated failures, retrospectives, and requests for a skill or guardrail | Fix the demonstrated recurrence at the lowest effective layer and replay the original failure before broadening policy. |
| UI acceptance review | Substantial interface work and repeated UI corrections | Independently exercise the exact preview against accepted real-user tasks before calling it ready; trivial copy edits stay lightweight. |

## Selection boundaries

- Controller-bound workflows keep their exact immutable instructions and do
  not receive ambient method routing.
- Quick-create remains a focused issue-creation action.
- A plain explanation does not load an implementation method.
- Missing skills are never named in the prompt.
- Older workers receive the same selected bundles from the server even when
  they cannot render the newer routing explanation.
- Each source field is size-bounded before routing.
- Overlap is intentional when the work truly combines methods, such as a
  long-running modernization audit.

## Instruction hierarchy

Use the lowest durable layer that can enforce the behavior:

1. Deterministic product checks for permissions, state, retries, tests,
   deployment, and acceptance facts.
2. Automatically selected skills for reusable ways of working.
3. Project `AGENTS.md` for concise, always-relevant repository facts.
4. Global `AGENTS.md` for rare rules that apply to every task.
5. Brain for current company knowledge and evidence, retrieved only when
   relevant.

Do not copy the same policy into every layer. Historical worktree copies of
`AGENTS.md` are records, not files to rewrite in bulk.

## Initial validation cases

The router tests replay representative Silver Mediums problems:

- an ordinary issue receives outcome ownership;
- a PhoneFarm modernization audit receives outcome, evidence, long-work, and
  research methods;
- a repeated regression adds hardening;
- a resumed issue keeps long-work continuity;
- a research chat routes without a slash command;
- a plain explanatory question stays lightweight;
- substantial UI work receives independent acceptance review while a trivial
  copy edit does not;
- quick-create and controller-bound prompts remain unchanged.

Passing these tests proves deterministic selection and prompt isolation. It
does not prove every worker follows the method or that a production release is
live. Broader task replays and production telemetry must measure completion,
human intervention, retries, context size, and total usage before expanding the
rules.

Current evidence and outstanding release gates are tracked in
[`SKILL-OS-ACCEPTANCE.md`](./SKILL-OS-ACCEPTANCE.md).

## AGENTS.md cleanup rule

Canonical files may be redesigned after their project source and operating
constraints are verified. Keep only stable facts that nearly every task in that
scope needs. Move conditional procedures into an automatic skill and move
machine-verifiable rules into code. Never mass-edit stale worktree snapshots.
The inspected inventory and safe rollout are recorded in
[`AGENTS-MD-AUDIT-20261014.md`](./AGENTS-MD-AUDIT-20261014.md).

Rollback is simple: remove the routing block from prompt construction and stop
shipping the six built-in methods. No task, Brain record, or provider account
depends on them for storage.
