# SILV-38 manual recovery 2

The original failed task `01a0a634-ff16-700a-b631-0726ac8e2229`, its consumed
slot, snapshot and frozen harness are preserved. Its credentials and authority
were cleaned up before this separate recovery. Only that named historical task
is excluded from the zero-new-task staging check. See
`docs/silver-mediums/SILV-38-MANUAL-RECOVERY-20260915.md` for the failure boundary.

This operational harness runs only issue
`01a0a5fc-a49d-766b-b483-cf85a129c38c` in the private production workspace.
It does not change the reviewed application build.

## Work and authority

The September 15 owner go-live grant authorizes one genuine low-risk issue.
The deliverable is a current CRM engineering handoff. A host broker reads only
the CRM checkout's Git metadata, checks the paths returned by that repository's
worktree list, and reads the named CRM/hub guidance. It captures one dated
snapshot. Separate tool-free Claude Code sessions plan, write and independently
review the handoff from those facts. The model produces the actual analysis;
the host does not substitute a canned report or an approval marker.

Provider route: the existing Claude Code subscription, Sonnet 4.6 with medium
effort and the previously disclosed background Haiku behavior. This is a
temporary bounded route, not permanent role or exact-single-model qualification.
No new provider credential source is permitted. Temporary opaque copies and
task-local daemon credentials must be removed or revoked at closure.

Maximum three further provider invocations: one planner, one writer and one
independent reviewer. Correction provider calls are denied. Including the two
failed slots, the overall maximum is five, matching the original five-call cap.
Each invocation has
one turn, no provider retry, no tools, no MCP, no session persistence and an
empty working directory. Durable exclusive role/attempt markers are written
before provider start. The application's workflow controller also suppresses
ordinary automatic retries for enrolled workflow tasks. No global retry default
is changed. A failed launch is not silently replayed.

Fresh immutable runtime receipts bind the exact issue, runtime, agent, controller,
broker and provider executable digests, model, role artifact, environment and
expiry. Each task receives its own one-use native capability before claim.
Only the broker can post the scoped progress, artifact and workflow events.
Its fixed HTTP destination is loopback port 8080 on the authorized host.

## Evidence boundaries

Git facts prove local state only. Cached upstream refs are explicitly cached.
No fetch, application startup, CRM database access, production mutation, outbound
message, Phone Farm access, public exposure or repository change is in scope.
The handoff includes the separate remote, paired deployment, runtime and business
checks still needed before a CRM release. Missing worktrees never authorize
cleanup. The review assesses the captured snapshot, not later concurrent edits.

Before launch: focused provider-free checks of this bounded incident correction;
fresh recovery capsule for the controller configuration; exact image/source
readback; fresh per-issue receipts. After completion: native Issue/Inbox/Board
readback, authority and process cleanup, matching backup/restore/cloud readback.

The old CEF integration replay and its exhausted receipts are historical evidence
only. None is relabeled as this issue's business result.

Setup writes an atomic journal before mutating resources, establishes the normal
owner identity while the runtime is offline, then starts the native daemon using
its short-lived scoped token. Parked staging can resume after a partial failure.
The writer receives the saved planner result and its digest. Each provider call
gets a unique empty workspace that is removed after the child exits. Cleanup
attempts independent steps even when server cleanup fails and records any
remaining failure without retaining local credential copies unnecessarily.

The original broker rejected reasoning blocks that the native Claude adapter
already supports. This recovery discards them in memory and forwards only final
text. Safe response metadata and finite error classifications are retained;
raw provider diagnostics, prompts and reasoning text are not written to receipts.
