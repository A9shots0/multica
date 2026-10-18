---
name: multica-outcome-ownership
description: "Use for implementation or operational work that must reach a usable business result, not stop at research, code, a PR, a build, or a deployment. Not for explanation-only requests."
user-invocable: false
allowed-tools: Bash(multica *), Bash(git *), Bash(gh *)
---

# Own the Outcome

Convert the request into an observable result and carry it as far as the
current authority and tools allow.

## Establish the contract

Before substantial work, identify:

- the result the owner or employee must be able to use;
- the evidence that would prove it works;
- the important limits, especially business, UI, policy and production limits;
- the next safe action already authorized.

Resolve technical facts from code, tools and current evidence. Ask the owner
only for a real business choice, UI/product decision, policy change, budget
change, or authority the task does not already grant. State the choices in
plain language and explain what each changes.

## Keep states separate

Never collapse these into “done”:

- planned;
- coded;
- tested;
- merged;
- built;
- deployed;
- working in the real workflow;
- accepted by the owner or responsible system.

Report the furthest state supported by evidence. A worker saying it finished
is evidence of a claim, not acceptance.

When Multica supplies a `Workflow continuity ledger`, carry its original goal
and acceptance version through the run. Only `acceptance.accepted: true` from
that server-authored projection closes the goal. Do not manufacture acceptance
from your own completion, a handoff, or an unreviewed Brain claim.

## Read shared evidence precisely

Treat receipt namespaces as API contracts. A cited `source_event:N` belongs to
the company event reader: call `company_events` with `afterId=N-1`,
`throughId=N` and a small `limit`. Never pass it to `brain_evidence`, which
accepts a Brain evidence UUID and immutable revision.

When an exact lookup fails because its selector is invalid, correct it once
using the known project binding and cited ID. After a second similar `400` or
`404`, stop and report the unresolved tool, namespace, project and ID. Do not
guess ID variants or scan broad history.

## Execute to the safe boundary

Continue through normal implementation, focused verification, integration and
release steps when they are inside the task's existing authority. Do not stop
at a document, local patch, PR or green health endpoint when the requested
outcome is further downstream.

Stop only for a genuine missing decision, authority, dependency or unsafe
condition. Name the exact blocker and preserve a useful next action. Do not
replace available investigation with a technical question to the owner.

## Communicate for the owner

Use plain English, short updates and no emojis. Lead with what changed for the
company or user. Include technical identifiers only when they help locate
evidence or recover the work.

The final result states:

1. what is usable now;
2. what evidence proves it;
3. what remains incomplete or uncertain;
4. whether the owner must decide or do anything.
