# Continuous-work runtime v1

## Purpose

Continuous work lets one accepted issue goal cross a small number of fresh
provider sessions without treating a transcript, task completion, checkpoint,
or handoff as owner acceptance. The first release is deliberately narrow: one
ordinary issue task, at most three declared work packages, and at most two
fresh-session continuations.

## Runtime contract

The server opts a task in. Provider text cannot enable continuity, choose a
different task, increase the package limit, or change lineage.

V1 admission is an off-by-default server rollout decision. Set the server-only
feature flag `continuous_work_checkpoint_v1` (environment override
`FF_CONTINUOUS_WORK_CHECKPOINT_V1=true`, or an equivalent targeted rule in
`MULTICA_FEATURE_FLAGS_FILE`). At claim time the server enrolls only an
ordinary issue task that is bound to exactly one current normalized workflow
execution and to exactly one `local_directory` resource on the authenticated
daemon with `execution_mode: in_place`. The decision is made from database and
feature-flag state; issue descriptions, comments, prompts, and provider output
cannot trigger it. Tasks outside that narrow cohort run normally.

Shadow measurement is controlled independently by the server-only,
off-by-default `continuous_work_observation_v1` flag (environment override
`FF_CONTINUOUS_WORK_OBSERVATION_V1=true`, or a targeted rule in
`MULTICA_FEATURE_FLAGS_FILE`). The server persists that decision on the exact
task at claim time. Only an enrolled task may submit context manifests, live
usage samples, or hash-only loop observations; old servers and unenrolled tasks
therefore receive no probes or writes. Enable observation for a bounded provider
cohort first. Enabling observation does not enable checkpoint rotation.

Targeted checkpoint admission and observation evaluate authoritative claim-time
runtime identity. Rules may use `task_id`, `workspace_id`, `provider`,
`runtime_mode`, `runtime_id`, `agent_id`,
`workspace_provider`, `provider_agent_id`, or `provider_runtime_id`. Provider
and runtime-mode values are normalized to lowercase. Prefer a composite key for
a bounded cohort, and prefer `task_id` for a one-task production proof; a
provider-only allow list enrolls every task using that provider. For example:

```yaml
continuous_work_observation_v1:
  default: false
  allow_by: workspace_provider
  allow:
    - "4e3cdf12-463c-4834-9291-c5ff2e422663:codex"
```

Environment overrides remain authoritative. In particular,
`FF_CONTINUOUS_WORK_OBSERVATION_V1=false` or
`FF_CONTINUOUS_WORK_CHECKPOINT_V1=false` disables even an allowing file rule,
while `true` enables that behavior globally. The production host profile must
omit both overrides during a bounded pilot so the protected task-targeted file
is authoritative.

## Measured Codex checkpoint

Codex is the first provider allowed to turn live context evidence into a
checkpoint request. The app-server's `thread/tokenUsage/updated` event reports
two different quantities that must not be confused:

- `last.totalTokens` is the provider-reported active context for the current
  thread and is the exact numerator;
- `total` is cumulative session traffic used for usage accounting and can never
  authorize interruption.

The provider-reported `modelContextWindow` is the denominator. Missing,
malformed, proxy, or unknown values remain observation-only. At 55% the server
records `would_prepare`. At 70% it records `would_request_checkpoint`; at 80%
it records `would_stop`. The daemon acts only after the server has durably
stored the exact sample and returned a matching hash, decision, and checkpoint
authorization for a task carrying both persisted observation enrollment and a
valid authenticated checkpoint plan.

After authorization, the daemon lets any in-flight tool finish, then asks Codex
to interrupt the active turn through its native protocol. A naturally completed
turn wins the race. Otherwise the daemon resumes the exact same Codex session
for one narrow checkpoint-only turn, with threshold observation disabled to
prevent recursion. That turn must inspect durable workspace state and return a
valid bounded receipt. Missing session identity, a changed session, failed
checkpoint turn, or missing/invalid receipt fails closed and creates no
successor. Only the existing atomic checkpoint transaction may create the
forced-fresh successor.

Claude remains observation-only in this release because this runtime path does
not yet expose an equally reliable live active-context numerator. Provider
compaction is a later fallback: Multica writes and verifies its durable
checkpoint before invoking provider compaction or retiring the old session.

At an admitted package boundary, the worker returns one bounded checkpoint:

```json
{
  "schema_version": 1,
  "package_step": 1,
  "summary": "What this package established",
  "completed": ["Concrete completed item"],
  "next_action": "The next bounded action",
  "verification": ["Evidence already checked"],
  "open_risks": ["Unresolved risk or unknown"]
}
```

The canonical receipt must fit in 2 KiB. The server binds it to the
authenticated task, agent, runtime, issue, workflow execution, scope, and
continuation step. The receipt is progress evidence, not authority, reviewed
Brain memory, production truth, or acceptance.

The checkpoint transaction either performs every action or none:

1. verify the exact server-authored plan and current workflow execution;
2. append the new tamper-evident execution snapshot;
3. record one immutable, idempotent checkpoint receipt;
4. create one explicitly linked fresh-session successor; and
5. finish the source package without publishing the checkpoint as a final
   issue or chat answer.

An exact callback replay returns the existing successor. A changed replay,
stale execution, mismatched lineage, ambiguous state, oversize receipt, or an
attempt beyond the declared package limit fails closed.

## Resume contract

The successor receives current normalized workflow state, current Brain and
Intelligence references under their existing trust boundaries, and one verified
capsule from its exact predecessor. It does not receive the predecessor's
provider transcript or provider session ID. Its exact validated persistent work
directory is copied onto the successor and returned as `prior_work_dir`; the
worker re-checks important checkpoint claims against
the current workspace and authoritative systems, then continues instead of
repeating completed verified work.

V1 permits only working directories whose contents remain available after the
source run finishes. Disposable local worktrees are rejected: terminal cleanup
removes them, so preserving only their old path would be false continuity.

## Controls

- Declared work-package boundaries and exact, server-acknowledged Codex active
  context may trigger a verified checkpoint. Estimated occupancy never does.
- Token, turn, and wall-time limits are independent run budgets. An 80% budget
  mark may request an orderly checkpoint; 100% may interrupt the run.
- A context-window percentage may control nothing unless both numerator and
  denominator are exact.
- Failure retries do not advance the continuation step.
- Repeated-tool detection stays observation-only until checkpoint
  acknowledgement and false-positive evidence justify enforcement.
- A completed worker, green test, merge, build, deploy, checkpoint, or handoff
  does not close the original goal. Only the accepted outcome does.

## Rollout gates

Before automatic use, prove concurrent idempotency, exact replay, changed-replay
rejection, full rollback after injected failures, stale and cross-lineage
rejection, fresh-session identity, persistent file continuity, checkpoint
non-publication, and unchanged normal completion behavior.

After shadow qualification, evaluate Codex and Claude separately with the
matched-pair protocol in `CONTINUOUS-WORK-A-B-V1.md`. Adopt only when the hard
quality gates and savings thresholds pass; do not pool provider results.
