# AGENTS.md audit and redesign boundary

Status: inventory complete; Skill OS is live; canonical simplification waits for observed task behavior

## What was observed

The accessible company files contained 232 paths named `AGENTS.md` but only 35
distinct contents. Most paths are copies inside old PhoneFarm, CRM and
Intelligence worktrees. They are historical task snapshots, not 232 active
policy sources.

The canonical files differ materially:

| Scope | Observed shape | Action |
| --- | --- | --- |
| Codex global | About 61 lines; task isolation, response discipline, evidence standards and UI review | Keep universal protections. Reassess only after every worker path proves the same protection elsewhere. |
| PhoneFarm | About 135 lines; project facts plus extensive incident, release and UI procedures | Keep hard source/release constraints. Move reusable judgment into automatic methods only after production routing is proven. |
| Multica | Concise pointer to `CLAUDE.md`, with a UI handoff pointer | Preserve this shape. Do not duplicate the new methods here. |
| CRM | About 26 lines | Verify current protected source before any edit. |
| Intelligence | About 72 lines | Verify current protected source before any edit. |

Counts describe the inspected snapshots, not a promise that every remote branch
or provider has identical current files.

## Confirmed problems

- Copies in old worktrees make raw file counts look like active policy volume.
- Some canonical project files repeat global rules because not every provider's
  global-instruction inheritance has been proven.
- Long project files mix stable repository facts with conditional procedures,
  which makes every task pay for rules it may never use.
- Dynamic facts and past incident detail can become stale when embedded as
  permanent instructions.

Duplication alone is not proof that a rule should be removed. A repeated safety
rule may still be the only protection on a provider that does not receive the
global file.

## Target structure

### Global instructions

Keep only rules that apply to every repository and task, including task
isolation and the standard for evidence-backed judgment. Avoid project commands,
host paths and conditional runbooks.

### Canonical project instructions

Keep stable facts nearly every task in that repository needs:

- authoritative source and entry documents;
- architecture boundaries that cannot be inferred cheaply;
- hard data, permission and deployment constraints;
- the supported test and release entrypoints;
- short pointers to specialized guidance.

### Automatically selected skills

Use skills for recognizable methods such as outcome ownership, evidence-first
debugging, long-work supervision, research/REA, learning from repeated failures
and independent UI acceptance review. Load only the matching method.

### Deterministic controls

Use code for facts a machine can decide: authorization, state transitions,
locks, retry caps, source identity, required tests, deployment gates and
acceptance evidence. Prose must not be the only enforcement for these rules.

### Brain

Use bounded retrieval for current company knowledge, decisions and evidence.
Brain text is reference data and does not grant authority or certify completion.

## Safe rollout

1. Integrate the automatic Skill OS without deleting existing rules.
2. Prove a real task receives only the matching method on each supported
   provider.
3. Verify whether each provider receives global instructions, project
   instructions or both.
4. Simplify one canonical project file and replay representative failures.
5. Remove a duplicated rule only when the replacement catches the same failure
   without blocking valid work.
6. Roll out to the remaining canonical repositories through their protected
   source workflows.

Never mass-edit historical worktree copies. Never shorten task-isolation or
production-safety rules merely to reduce line count. The success measure is
better future behavior at lower context cost, not smaller files by themselves.
