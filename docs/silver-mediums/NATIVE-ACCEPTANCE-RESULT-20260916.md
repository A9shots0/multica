# Native publication acceptance result — 2026-09-16

**Outcome: accepted.** One publisher turn and one automatically routed independent
reviewer turn completed natively on the exact private release. The issue reads
Done consistently across Issue, Current Picture, Now, Inbox and Board with no
operator assistance, no technical stop, no retry and no correction. All temporary
permissions, profiles, credential copies, runtimes and agents were revoked and
removed afterwards, and a matching backup, isolated restore, rollback rehearsal
and OneDrive readback passed.

Snapshot: 2026-09-16T00:35Z to 2026-09-16T01:24Z (UTC). Source
`60fea34b9770-local-9eaa711d97b1`, backend image
`sha256:e710767a6769ae581c9ff94978246a03f815f6afead02cfb9d5a7b560a878801`.
Evidence: `.qualification-evidence/native-acceptance-v3-20260915/`.

## What this accepts, and what it does not

This accepts that the product can complete a real issue automatically: an owner
mention started a publisher, the publisher's pass emitted `implementation_ready`,
the workflow routed an independent reviewer on its own, and the reviewer's pass
emitted `review_passed`, after which every owner-facing surface settled to Done.

It does not accept a permanent role or model selection, unrestricted automatic
execution, public exposure, or any further provider work. It is one bounded run
under one grant, now closed.

## The correction that made this pass

SILV-39's single publisher turn on 2026-09-15 returned a valid pass, but its
926-byte summary exceeded the broker's 600-byte guard, so nothing was published
and no reviewer ran. That failure and its consumed slot remain immutable.

The corrected instruction asks for **one ASCII sentence of 10 to 400 characters**
while the broker keeps the 600-byte hard ceiling. Both turns complied
comfortably:

| Slot | Characters | UTF-8 bytes | ASCII | Verdict | Findings |
|---|---|---|---|---|---|
| Publisher | 269 | 269 | yes | pass | 0 |
| Reviewer | 300 | 300 | yes | pass | 0 |

## The run

Fresh isolated bindings were created for this grant. Nothing from the cancelled
v2 trial or the failed SILV-39 slot was reused.

| Binding | Value |
|---|---|
| Harness | `scripts/production-pilot/completion-acceptance-v3/` |
| State | `.qualification-evidence/production-pilot-SILV-38-completion-acceptance-v3/` |
| Issue | #40 `01a0a7a4-2415-7ada-9a47-667a5bc6a094` |
| Daemon / profile | `silv38-production-pilot-completion-acceptance-v3-20260916` |
| Binaries | `silv38-completion-acceptance-v3-broker.exe`, `completion-acceptance-v3-multica.exe`, `silv38-completion-acceptance-v3-authority` |
| Roles | planner `9a05b2de`, publisher `9f71276a`, reviewer `91079e1f` |
| Authority receipts | 3 issued, 3 revoked |

The v3 harness is the independently reviewed v2 baseline with binding-only
substitutions; every file reproduces byte-for-byte from v2 by substituting the
directory name, the role-name suffix, the profile/daemon date and the issue
title. Provider-free checks passed first: 13 broker tests plus 4 byte-boundary
subcases, and 6 mocked controller tests.

One further binding correction was required and is recorded in the v3
qualification receipt. The authority helper pinned the superseded v1 issue title,
which neither v2 nor v3 could ever satisfy — it rejected the run with "issue must
be parked in its exact squad before receipts". Its expected title now names this
acceptance issue. The check itself is unchanged: exact title, exact squad,
backlog status and zero existing tasks.

### Sequence

1. `prepare-resources` — new issue in Backlog, offline private runtime, three
   private roles, squad and workflow plan. Zero tasks, zero provider contact.
2. `build-bound` — broker, CLI and Linux authority helper built with the exact
   issue and role IDs; digests recorded.
3. `provision-daemon`, `start-daemon` — isolated private runtime online with a
   scoped expiring daemon token. Provider profile still absent.
4. `prepare-authority`, `issue-authority` — three immutable exact grants stored
   by the host controller.
5. `prepare-provider` — the existing Claude Code subscription file copied
   opaquely into the private task profile. Credential bytes were never parsed,
   printed or hashed.
6. `launch-publication` — one owner mention of the publisher, planner and
   reviewer suppressed from incidental routing.

### Result

Two tasks, both completed, zero failed:

| Task | Role | Outcome |
|---|---|---|
| `01a0a7b3-ae94-71b3-bd0f-ed7ccab3b958` | publisher | completed → `implementation_ready` |
| `01a0a7b4-1287-715c-bfc0-c9527b1cfb9d` | reviewer | completed → `review_passed` |

The reviewer was routed by the workflow, not by a second owner action.

Publication integrity: the published comment's parent is the owner's launch
comment (native `parent_id`), it quotes the approved digest
`2df9d83d52cf882f7b399b4b823d6d0cc975d41be6856b8838b276d6f494541c`, and its body
is byte-identical to the approved candidate
(`651a5f6c435367a9f0e00f30d1271582263fecde16505385e4f44f0bde7830cf`
LF-normalised on both sides). The provider never rewrote the document.

The provider frames contain only `thinking`, `text` and `result` — no tool use,
no MCP, one turn per slot.

## Consistent completion across the five surfaces

| Surface | Reading |
|---|---|
| Issue | `status = done`, revision 7 |
| Current Picture | stage `complete`, outcome `satisfied`, execution `completed`, attention `resolved`, milestone `complete`, terminal reason empty, zero unresolved incidents/decisions/dependencies, independent review recorded |
| Now | owner summary revision-aligned (1 = 1), lane `done`, signals `outcome_complete` + `outcome_done`, next action none, owner action required **no**, latest change `review_recorded` / `succeeded` |
| Inbox | one owner-facing entry, type `task_completed`, severity `info`, issue status `done`, body "The accepted outcome is independently verified and complete.", `independently_verified = true`. No `workflow_incident`. |
| Board | returned by the Done query; lane `done` |

The Now and Board lanes were derived with the product's own
`operationalLaneForIssue` rule in
`packages/views/issues/components/operational-workflow.tsx`, against the live API
response.

**Boundary:** this is API and projection evidence, read from the same endpoints
and the same lane rule the web and desktop clients use. It is not a rendered
screenshot. The Orca Browser path was excluded by the owner's instruction for
this run, and the Chrome extension was not connected, so no visual capture was
taken. Nothing else in this report depends on that.

## Preservation

SILV-38 and SILV-39 are byte-identical before and after this run:

| Issue | Tasks | Events | Comments |
|---|---|---|---|
| SILV-38 (#38) | 7 unchanged (6 failed, 1 completed) | unchanged | 12 unchanged |
| SILV-39 (#39) | 1 unchanged (failed) | unchanged | 2 unchanged |

Workspace totals moved only by this run: 50 → 51 issues, 118 → 120 tasks. Active
agents returned to 10. The earlier cancelled v2 trial keeps its
`stopped_by_owner` record untouched.

## Revocation and cleanup

`fully_clean: true` in `cleanup-verification.json`:

- 3 authority receipts issued, 3 revoked, 0 still active.
- Runtime deregistered offline; zero online runtimes anywhere.
- Zero daemon tokens for this daemon and zero in the workspace.
- Temporary provider profile removed; temporary daemon profile directory removed
  after its log was preserved (the log contains no secret values — only a token
  length).
- Owner session file removed. The owner's own subscription credential file is
  untouched.
- All three temporary agents archived; none active.
- Zero pilot processes running; zero active tasks anywhere.

The harness's own cleanup reported one failure: it stops the daemon through
`pwsh`, which is not on PATH on this host. The process was stopped separately
under the same identity guards (matching executable path and daemon id) and
verified absent.

## Recovery closure

Run `20260916T011317Z-a6a72487`, artifact
`multica-recovery-20260916T011317Z-a6a72487.smkdr`
(`7e649e7e3089ecf4d9cc795b162f6279bd8c9b92560526804bf92a20e3f619cd`,
378,705,960 bytes):

| Check | Result |
|---|---|
| Backup | verified, all 10 components, replication copy verified |
| Isolated restore | verified (drill `6f6f6f0268`), 145 user tables, uploads path/checksum manifest matched, recovery point age 13.6s |
| Application + rollback rehearsal | verified (drill `appf3980d42e3`), cleanup verified |
| OneDrive off-machine readback | verified 01:23:37Z by dehydrate/rehydrate hash match, key custody verified |
| Watchdog | healthy, backup current, zero consecutive health failures |

The rollback rehearsal booted both the current release image and the rollback
image against the restored database. Both reported health and readiness ok, 603
migrations before and after with none added or removed, and identical row counts
(1 user, 1 workspace, 51 issues, 149 comments, 120 tasks, 0 runnable).

The OneDrive readback needed a retry: the verifier hashes the local cloud copy
without retrying, and a 361 MB artifact is still uploading when the backup step
returns. It passed on the next attempt roughly ten minutes later. Worth fixing in
the verifier rather than working around each time.

Production was never restarted or modified: `started_at 2026-09-15T22:12:43Z`
before and after, all three containers healthy on the same images.

## What was not done

No retries, no extra provider turns, no tools or MCP, no correction call, no
unrelated implementation, no public exposure, no outbound communication, no Orca
and no Phone Farm access. No production data was mutated beyond this run's own
issue, its two tasks and their comments and events.

## Next decision

Nothing is outstanding from this grant. A permanent role/model selection, a wider
automatic-execution scope, or any further provider run each need a separate
explicit authorization.
