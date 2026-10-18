# Issue-started lane bridge — root cause, 2026-09-17

**Finding:** the issue bridge fights the workflow controller's model instead of using
it. Every remaining failure follows from that. No more proof runs until the design
below is chosen and built.

Scope: `server/cmd/implementation-lane-bridge`,
`scripts/implementation-lane/issue-local/local_instance.py`, the daemon's claim and
execution path (`server/internal/daemon/daemon.go`,
`server/internal/daemon/workflow_assignment_runtime.go`), the controller's workflow
assignment (`server/internal/service/workflow_legacy_guard.go`, `task.go`), and the
proven native path (`docs/silver-mediums/NATIVE-ACCEPTANCE-RESULT-20260916.md`,
`scripts/production-pilot/completion-acceptance-v3/`). Everything ran on the
disposable loopback instance only.

## Every proof run and the layer it failed at

| Run (issue) | Time (UTC) | Reached | Failed at | Layer |
|---|---|---|---|---|
| 1 `01a0acbf` (owner-run) | 00:23 | daemon claimed the implementer task | lane refused admission: a concurrent CRM lane worktree was present; then surface reader crashed on object `signals` | lane host concurrency; proof script |
| 2 (no issue) | 02:28 | server up | daemon never registered: it was started inside its own workspaces root, where its permanent guard marker made every later start refuse | local instance launch |
| 3 `01a0ae11` | 06:32 | **lane passed** | implementer was a direct `@implementer` task with no squad, so its result comment woke the squad leader as agent chatter; the bridge failed that leader task and the workflow stopped. The reviewer task was then refused at claim | workflow routing; bridge role model |
| 4 `01a0ae1f` | 06:48 | leader delegation correct, implementer squad-scoped | lane's Codex review rejected the change (test did not call `ApiClient.listAgents`; missing vitest node directive) | lane quality gates |
| 5 `01a0ae2c` | 07:02 | routing correct again | lane regression: strict `tsc` on the new test (executor cannot run `tsc`), and unredacted logging of agent payloads | lane quality gates |
| 6 `01a0ae3c` | 07:20 | **lane passed**, `implementation_ready` accepted, reviewer routed | reviewer task refused at claim: "bounded workflow execution denied before exposure: workflow assignment identity or exact runtime selector mismatch" | **controller execution model vs bridge** |

- **Fixed along the way, and still valid:**
  - single-run host lock (lane v11);
  - daemon launch directory;
  - object signal codes;
  - squad-scoped delegation through a leader turn;
  - named failure reasons.
- **Runs 4 and 5** failed honestly on generated-code quality. The gates did their job.
- **Runs 3 and 6** reached the reviewer and hit the real design problem.

## The two paths side by side

| Step | Proven native path (#40, 2026-09-16) | Bridge path |
|---|---|---|
| Who executes a turn | a controller-bound **workflow broker** executable, launched by the daemon under a host-issued grant (permission receipt, authority helper, `MULTICA_WORKFLOW_CONTROLLER_ID`) | a custom binary set as `MULTICA_CLAUDE_PATH`, impersonating the Claude CLI |
| What a turn may do | model-only, tool-free, one turn; result is a bounded structured verdict (600-byte guard) the broker publishes | runs a 10–20 minute Python lane with an executor and a Codex reviewer, reads and writes files, calls the API with the task token |
| Implementer / publisher | owner mention; the publisher's pass emits `implementation_ready` through the broker | runs 4–6: leader delegation; the bridge emits `implementation_ready` itself with the task token |
| Reviewer | routed by the controller as a **bounded workflow task**, executed tool-free with the exact assignment packet; its pass emits `review_passed` | the bridge expects an ordinary tool-using turn that re-reads lane evidence from disk and posts `review_passed` |
| Correction, retry, leader | denied by grant | the bridge had to invent a leader turn to avoid stopping the workflow |

## Why the reviewer turn cannot work under the bridge

A reviewer task created by `EnqueueTaskForWorkflowTransition` carries a
`LegacyAssignmentV1` packet. At claim and at launch, the daemon treats it as a bounded
workflow (`daemon.go`, `isolatedToolFree`):

1. `validateBoundedWorkflowClaimExposure` requires the claimed task to match the
   assignment exactly: agent, issue, squad, runtime, provider, model and reasoning
   effort. It also requires no instructions, custom args or environment, MCP, repos or
   ambient context. Run 6 failed here.
2. Even with a match, execution is isolated: max 1 turn, tool policy `none`, a staged
   executable, and a guarded child environment **without the task-scoped token**.
   Custom runtime commands are refused as "not bound adapters". The bridge needs the
   `mat_` token and API writes, so it would fail at `loadConfig` even after the
   selector check.
3. `LegacyToolPolicy` without a broker grant is refused: "legacy broker authority is
   retired". The only supported way to run a workflow turn is the broker grant used
   by #40.

So the leader turn, the delegation and the event-posting implementer all work, but the
reviewer (and any correction) is structurally outside what the bridge can be.

## Options

| Option | What changes | Cost | Risk |
|---|---|---|---|
| **A. Lane outside the workflow controller** | No workflow plan for lane issues. The bridge runs as an ordinary assigned-agent task. The lane's own Codex review is the review. The bridge moves the issue to Done or Blocked and posts the result. | Small (bridge + proof script). Can be proven with one run. | Current Picture and Now show no workflow stage for lane issues; Issue, Inbox and Board still do. |
| **B. Lane as implementer, native reviewer** (recommended) | The lane stays the implementer (delegated task, as proven in runs 3–6). The reviewer becomes a real bound provider turn under a workflow-broker grant, as in #40. It reviews the lane's evidence packet and emits `review_passed` or `review_failed`. | Medium: a broker variant for lane evidence, grant issuance in the local proof, one subscription review turn per issue. | Reuses only proven mechanisms. The extra review is a real model turn with bounded output. |
| **C. Admit a tool-using lane under a broker grant** | Extend the grant contract so a workflow turn may run the lane. | Large; a capability-boundary change. | Security review needed. Contradicts the broker's no-tools, no-retry design. |

**Recommendation: B.** It keeps the controller's model intact, needs no new
capability class, and gives the owner the full five-surface picture.

- **Engineering, not an owner decision:** building B, and choosing between A and B.
- **The owner's call:** C, because it widens what a workflow grant may do.

## What is not claimed

No issue-started lane run has reached Done. The implementer side is proven twice
(runs 3 and 6: lane passed and `implementation_ready` accepted). The reviewer side is
not proven through the bridge, and by design it cannot be.

## Option B chosen — build scope (2026-09-17T08:10Z)

Planning decision: build B, with the lane's internal Codex review kept and the broker
grant not widened. B reuses the #40 mechanism, which has these parts:

1. **Reviewer broker** (after `server/cmd/item14-integration-pilot-broker`):
   - Re-verify the lane evidence deterministically, reusing the bridge's
     `verifyEvidence`: patch and outcome digests, gates recorded as passed.
   - Ask the tool-free, single-turn Claude child to classify that verification
     (`--tools ""`, `--max-turns 1`, exact model and effort).
   - Post `review_passed` or `review_failed` itself.
   - Build it bound to the exact issue and role IDs, with a digest receipt.
2. **Controller-enabled proof server**:
   - `MULTICA_WORKFLOW_CONTROLLER_ID` and the permission receipt store.
   - An authority helper (after
     `scripts/production-pilot/completion-acceptance-v3/authority`) that issues
     and revokes the `qualified-runtime-binding.v2` receipt for the reviewer
     role, using the `claude_code_subscription` profile kind and `review` as the
     only allowed task kind.
3. **Role artifact**:
   - The receipt must name a role in the embedded, SHA-pinned role artifact set
     (`agentqualification/role_artifact_bindings_v2.go`).
   - Reuse the reviewer role the #40 acceptance was issued under. Adding a role
     changes the qualification registry.
4. **Credential custody**: an opaque copy of the existing Claude subscription
   file into a private task profile, removed at cleanup, as in #40.
5. **Proof harness**: `local_instance.py prove`.
   - The implementer stays the delegated lane task.
   - The reviewer agent runs on a broker runtime whose `MULTICA_CLAUDE_PATH`
     is the reviewer broker, separate from the implementer's bridge runtime.
   - Issue the grant before launch; revoke it and remove custody at cleanup.
   - Run the proof exactly once.

**Order:** after the release candidate is ready, per the planning decision. The
candidate is currently blocked on GitHub Actions (account-wide startup failure).

## Option B built; the single proof run — 2026-09-17T08:15Z

**Built and pushed** (`acc894489`, `8a9bf0485`, `f59f9bd4e`, `ebbbf8f39`), with
provider-free tests:
- `pkg/laneevidence` (5 tests);
- `cmd/implementation-lane-review-broker` (9 tests);
- `cmd/implementation-lane-review-authority` (3 tests);
- harness (5 tests).

The grant reuses the qualified role artifact the #40 grant used and allows only the
`review` task kind. It adds no role, task kind or capability.

**Found and fixed on the way:** the Windows checkout wrote the embedded,
SHA-pinned qualification artifacts and the system graph role catalog as CRLF, so
every grant integrity check failed closed. No server built from this checkout
could issue or honour a grant. `.gitattributes` now keeps both packages LF.

**Setup rehearsal (no launch, no task, no provider call):** grant prepared,
issued and revoked; provider profile and contract removed.

**The single proof run** (issue `01a0ae61`, 08:00–08:09Z) reached the furthest
point so far:

| Step | Result |
|---|---|
| Owner mention of the squad → leader turn | completed; delegated once |
| Implementer (squad-scoped) → lane run | **passed**; `implementation_ready` accepted |
| Controller routes the reviewer | **the review grant was applied**: the reviewer's transition task became a guarded broker execution |
| Daemon starts the reviewer task | **refused**: `guarded_execution_authority_unavailable: authenticated daemon id and package digest are required` |
| Surfaces | Issue blocked, Now technical stop, Inbox `workflow_incident`, not on the Done board; consistent |

**Cause:**
- The proof daemon authenticates with a personal access token (`mul_`, from
  `POST /api/tokens`). Only a scoped daemon token (`mdt_`, a hashed
  `daemon_token` row bound to the daemon id) gives a request an authenticated
  daemon id (`middleware/daemon_auth.go`).
- Guarded task start requires that id (`permission_receipt_store.go:466`).
- The #40 acceptance provisioned exactly such a token (`manage.py
  provision_daemon`).
- Grant issuance, the controller, bounded routing and the broker path all worked
  up to this point.

**Next fix, not applied (single-run rule):** the harness provisions the daemon
the way #40 did:
1. create a random `mdt_` token and store only its SHA-256 in `daemon_token`
   with the daemon id, the workspace and an expiry;
2. write it to the daemon profile instead of a PAT;
3. delete the row at shutdown.

That is a harness-only change. The next proof run would be the first one that
can exercise the review broker itself.

## The one exception run — 2026-09-17T08:35Z

The planning chat allowed one more run after the harness-only daemon identity fix
(`b9fc52f38`).
- The owner session registers the runtime offline.
- A random `mdt_` token goes into the daemon profile, with only its SHA-256
  stored in `daemon_token`, bound to the daemon id, workspace and expiry.
- The row is deleted at shutdown.
- Harness tests: 8/8.
- A no-launch rehearsal confirmed the daemon comes online on the scoped token
  and the grant issues and revokes.

**Result: no verdict.**
- The run (issue `01a0ae6d`, from 08:13Z) was **stopped by the host memory
  guard** while the lane's executor was working. That was a few minutes in,
  before the lane finished and before the reviewer turn.
- The leader had delegated correctly and the implementer task was running.
- The kill ended the proof process and its children, so its shutdown did not
  run.

**Cleanup, run by hand with `local_instance.py down`, then verified:**
- review grant revoked;
- provider profile (copied sign-in) removed;
- daemon profile removed;
- proof database container stopped, taking the daemon token row with it.

**Left in place deliberately:**
- the interrupted lane worktree
  `C:/Users/a9sho/.codex/worktrees/implementation-lane/issue-proof-list-agents-20260917T081325Z-edb99685`,
  which holds the executor's uncaptured partial edits (`packages/core/api/client.ts`,
  `schemas.ts`, a new test);
- its evidence `.qualification-evidence/implementation-lane/issue-proof-list-agents-20260917T081325Z-edb99685/`.

The lane's orphan sweep will refuse new runs while that dirty worktree exists.
Removing it is a deliberate step.

**Status:** per the planning decision, no further runs without a design review.
- **Unproven:** the daemon identity fix under a real reviewer start. The
  rehearsal proves only that the daemon runs on the scoped token.
- **What blocks a clean run:** host memory during the lane's executor and test
  steps. The same kernel-pool leak blocks the release web build.

### Interrupted worktree preserved as a patch and removed — 2026-09-17T08:45Z

- **Saved** in
  `.qualification-evidence/implementation-lane/issue-proof-list-agents-20260917T081325Z-edb99685/`:
  - `interrupted-worktree.patch`: 3 files, `packages/core/api/client.ts`,
    `schemas.ts` and the new `list-agents-response.test.ts`. It applies cleanly
    to the current checkout.
  - `interrupted-worktree-status.txt`: head `b9fc52f38` and git status.
- **Removed the worktree deliberately.**
  - Its 496 dependency junctions were unlinked first, as the lane's own cleanup
    does, so the shared warm install was untouched (verified intact).
  - The orphan sweep is no longer blocked.
- **No further heavy runs.** Host memory is the blocker and the owner is deciding.
