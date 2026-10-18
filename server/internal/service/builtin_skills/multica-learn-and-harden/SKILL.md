---
name: multica-learn-and-harden
description: "Use for retrospectives, repeated failures, regressions, context bloat, AGENTS.md cleanup, skill design, and turning a proven good workflow into a durable guardrail."
user-invocable: false
allowed-tools: Bash(multica *), Bash(git *), Bash(gh *)
---

# Learn and Harden

Turn demonstrated behavior into the smallest durable improvement without
accumulating another wall of instructions.

## Establish the pattern

Use real runs, artifacts and failures. Identify the repeated behavior, its
cost, the condition that triggers it and the result that should replace it.
Do not generalize from one annoyance when the wider rule is unproven.

## Choose the enforcement layer

Prefer, in order:

1. a deterministic product check, type, test, lock or state machine when a
   machine can decide the rule;
2. a task-specific skill for judgment needed only in a recognizable workflow;
3. a short project instruction for a fact that is always relevant in that
   repository;
4. a short global instruction only for behavior that truly applies everywhere;
5. Brain for evidence, decisions and context that should be retrieved when
   relevant, not injected into every prompt.

Do not keep the same rule in several layers. Leave a short pointer where
discovery needs it and maintain the detail in one place.

## Harden safely

Propose the correction, then replay representative earlier failures or a
focused test matrix. Check that the guard catches the bad behavior without
blocking valid work. Retrospective suggestions are candidates, not permission
to rewrite policy automatically.

Remove obsolete, duplicated and contradictory guidance after the replacement
is proven. Historical worktree copies are evidence snapshots; do not rewrite
them as if they were active configuration.

Record what changed, which behavior it prevents, the evidence used, and how to
roll it back. A skill is successful when future agents make the better decision
without the owner remembering to invoke it.
