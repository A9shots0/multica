---
name: multica-evidence-first-debugging
description: "Use for bugs, failures, outages, regressions, incorrect behavior, risky fixes, and audits that must prove cause and recovery. Not for ordinary feature work with no failure to explain."
user-invocable: false
allowed-tools: Bash(multica *), Bash(git *), Bash(gh *)
---

# Debug From Evidence

Find the failing boundary, change the smallest justified input, and prove the
real workflow recovered.

## During an outage

When people cannot perform a core workflow or data is at risk, restore service
first with the narrowest safe reversible action already authorized. Preserve
evidence and recovery options. Diagnose the forward fix after service is
stable unless recovery itself would increase the harm.

## Investigation loop

1. Reproduce the exact failure or establish why reproduction is currently
   impossible.
2. Observe the layer where expected behavior first differs from reality.
3. Form one falsifiable explanation tied to that evidence.
4. Change the smallest relevant input.
5. Re-run the same failing path, then the nearest regression checks.

Do not repeat an unchanged failed command, release or experiment. Record what
changed before a retry. If the hypothesis fails, update it from the new
evidence rather than adding unrelated fixes.

## Evidence lanes

Keep these separate:

- source and configuration;
- tests and builds;
- merge and artifact identity;
- deployment and process health;
- the affected API or employee workflow;
- sustained behavior after the first success.

A green health endpoint does not prove the affected workflow works. A passing
test does not prove that build is live. A deployed process does not prove the
business result is correct.

## Completion

State the observed cause, the bounded correction, the exact checks performed,
and any coverage still missing. When evidence is inconclusive, say so and
leave the result unaccepted rather than turning a guess into policy.
