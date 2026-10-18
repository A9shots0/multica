---
name: multica-long-work-supervision
description: "Use for long-running jobs, multi-stage programs, overhauls, migrations, overnight work, squads, and supervision that must detect stalls without wasting usage or spamming updates."
user-invocable: false
allowed-tools: Bash(multica *), Bash(git *), Bash(gh *)
---

# Supervise Long Work

Maintain progress toward the end result across long runs without turning
activity into fake progress.

## Define progress

Replace guessed percentages with evidence-backed milestones. Each milestone
has:

- a result that matters to the final outcome;
- an observable proof;
- the next dependency or decision;
- a clear terminal state: passed, failed, inconclusive or waiting.

“Still running,” files changed, elapsed time and messages sent are activity,
not progress.

## Monitor economically

Use deterministic status, process, test, repository and deployment checks
before asking a model to reread or reinterpret work. Reuse the last verified
state and inspect only the delta.

Choose a cadence proportional to the operation. Stay silent while the state is
unchanged and healthy. Notify only for:

- a meaningful milestone;
- a failure, stall or repeated loop;
- a decision or authority only the owner can provide;
- completion of the real outcome.

Do not poll every few seconds, repeatedly reread full histories, or generate
status prose merely to prove supervision is active.

## Detect wasted work

Intervene when evidence shows the worker is repeating an unchanged attempt,
optimizing a side problem, losing the original outcome, declaring success at
an intermediate state, or spending usage without producing new evidence.

Steer only when necessary. Prefer a narrow correction that names the missing
outcome or proof. Do not replace a worker's valid approach because another
approach is merely possible.

## Preserve continuity

Keep a compact ledger of the last meaningful evidence, current blocker, next
safe action, retries already attempted and the next check. When context must be
reconstructed, use that ledger and primary artifacts rather than replaying an
entire conversation.

When Multica supplies a `Workflow continuity ledger`, treat each blocker
fingerprint as its canonical identity. Reuse it across resumed runs. An
unchanged fingerprint is a wait state, not a reason to repeat the audit or
emit another status update. Resume only when the named condition is supported
by a new Current Picture event; a newer revision by itself is not enough.
