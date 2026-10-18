---
name: multica-ui-acceptance-review
description: "Use for substantial UI implementation, redesign, or repeatedly failing UI corrections that need independent review against real user tasks. Not for prose advice or trivial copy edits."
user-invocable: false
allowed-tools: Bash(multica *), Bash(git *), Bash(gh *)
---

# Review UI Acceptance Independently

Verify the exact working candidate against the user's real tasks before calling
substantial UI work ready. Owner approval remains authoritative; an agent's
taste does not replace it.

## Establish acceptance before implementation

Read the original requirements, accepted decisions, project UI guidance and
surface ledger where they exist. Turn them into observable scenarios with:

- the user task and expected result;
- the exact candidate identity;
- realistic data, role and screen scope;
- observed evidence;
- pass, fail or untested.

Include functional behavior and visual composition: hierarchy, density,
legibility, complete information, affordances, repeated use and important
empty, loading and error states. Select only scenarios relevant to this change.

## Separate builder and reviewer

When current-task collaboration is available, give a separate reviewer the
working preview, original requirements, candidate identity and authorized data
scope. Do not provide a suggested verdict. Never create or steer another
top-level conversation for the review.

The reviewer derives scenarios from the requirements, exercises complete tasks
in the actual preview and records concrete observations. A screenshot, clean
build or lack of overflow does not by itself prove usability. A scenario with
no adequate observation is untested, not passed.

If an independent reviewer or interactive preview is unavailable, perform the
same scenarios as a self-review and state that limitation. Do not claim an
independent or interactive pass.

## Correct and recheck

Fix confirmed failures inside the authorized scope, then recheck the changed
candidate and affected scenarios. Preserve valid unaffected evidence. Allow at
most two focused correction/review cycles before reporting the exact remaining
blocker; do not create an endless aesthetic loop.

Return defects with reproduction, evidence, user impact and the requirement
violated. Return a pass only for the tested scope and exact candidate. Keep the
acceptance record in the task's existing evidence or ledger location, or in
`ui-review/acceptance.md` when none exists.

This method is unnecessary for a trivial copy edit or prose-only design advice.
It is required when the UI structure, workflow, hierarchy, or repeated failed
correction makes a false "ready" claim costly.
