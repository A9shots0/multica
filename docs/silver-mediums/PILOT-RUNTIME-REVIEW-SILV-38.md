# SILV-38 runtime review and consolidated correction

Recorded 2026-09-15 17:54Z. Scope: the temporary operational harness under
`scripts/production-pilot`, its exact executable builds, and the controller
setting in the pinned production overlay. The accepted application build is
unchanged: `60fea34b9770-local-6f0af10d34dd` (5,865 inputs).

## Independent findings received

The separate read-only reviewer `/root/upstream_audit` returned these findings
before the interrupted turn. This document records those delivered findings
and the integrator's correction evidence; it is not a second review or a claim
that the reviewer re-approved the corrected code.

1. Partial staging could leave resources without the final contract, and a
   repeat could create duplicate agents. Cleanup stopped before removing local
   credentials if server revocation or deregistration failed.
2. The writer never received the saved planner output because native task input
   is deliberately replaced by the scoped prompt.
3. The provider working directory was reused, including fixture-profile residue.
4. First registration with a workspace daemon token leaves a new runtime without
   an owner; the ordinary private-runtime access gate correctly rejects it.
5. The integrator also identified that cleanup checked for links only after
   resolving the path, making that check ineffective.

## One consolidated correction

- Atomic setup journal records intents before writes and exact identities after
  writes. Exact-name/owner lookups recover successful writes with lost responses.
  Repeating parked staging reuses the same agents, squad and membership.
- The owner's normal short-lived session registers the exact runtime as offline.
  A four-hour daemon token then serves the real native daemon. Staging and the
  authority helper both require the exact owner and private visibility.
- Writer and correction prompts require the persisted, validated `PLAN.json`
  and include its complete content and SHA-256.
- Each provider invocation creates a unique temporary root with an empty
  `workspace` child. Fixture profile material is a sibling. Cleanup validates the
  exact absolute root, its parent and link status before removal.
- Cleanup records each independent result, removes local credential copies even
  when server steps fail, supports setup without a final contract, and restores
  the original issue assignment only when no task was ever dispatched. Literal
  profile/junction checks precede resolution and recursive removal.

## Verification

At 17:52Z, all eight Go broker tests and five Python failure-injection tests
passed. The Go fixture is a real child process with fake output and no provider
credential access. New checks cover fresh empty workspaces and removal, complete
planner-input continuity, lost agent-create responses and idempotent resumption,
owner registration before token creation, rejection of ownerless runtimes,
cleanup despite independent server failures, and literal junction rejection.
Windows broker and Linux in-container authority helper builds succeeded.
The application source check retained its exact 5,865 inputs and SHA-256
`6f0af10d34dd77de23dc5ff4a67ef1a79270b33671a8f9a89fe694b5303de16e`.

This is provider-free readiness evidence. Real provider operation, task progress,
independent handoff review and native completion remain separate until observed.
The frozen manifest under `.qualification-evidence/production-pilot-SILV-38`
binds this record, harness, overlay and executable digests for fresh authority.
