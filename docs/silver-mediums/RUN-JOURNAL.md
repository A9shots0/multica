# Silver Mediums run journal

## release-20260918 deployed - 2026-09-18T02:05Z

- Honest not-started status live; review passed clean after catching one real regression.
- Backup, isolated restore, OneDrive readback, rollback rehearsal, preflight, deploy all verified.
- Mac Studio runtime registered; Phone Farm roles created there; SILV-41 assigned, not started.
- Five tooling/product defects fixed; Docker memory capped.

## release-20260917 deployed - 2026-09-17T22:15Z

- Deploy verified: health, readiness, data counts, loopback-only boundary,
  images identical to the archived candidate.
- Backup + isolated restore 9 min; rollback rehearsal 1 min; preflight 6 s;
  deploy under a minute with no database restart.
- Option B proof passed first run; five surfaces agree done; cleanup verified.

## release-20260917 qualification - 2026-09-17T20:50Z

- Rebuilt three times (source drift, then two code fixes). Final source
  `6389f0d06`; images `2e7bed89` / `ba6bf43f`; archive verified.
- Checks: all green, ~6.5 min at full parallelism in UTC.
- Review: fail on two pre-existing, already-deployed conditions (issue delete
  vs owner notifications; migration 522 cascading FKs). Neither is in the delta.
- Backup/rehearsal/preflight blocked by the failed qualification record.

## Exception proof run — 2026-09-17T08:35Z

- Scoped daemon token fix; rehearsal passed.
- The run was killed by host memory during the lane executor. Manual `down`
  verified the grant revoked and custody removed. The interrupted lane worktree
  is kept.

## Option B build and single proof — 2026-09-17T08:15Z

- Built pkg/laneevidence, the review broker and the review authority helper, plus
  harness grant flow; pinned the qualification packages to LF.
- Rehearsal passed. Proof: lane passed; the reviewer's guarded start was refused
  (daemon authenticated with a PAT, not an `mdt_` daemon token). Stopped per the
  single-run rule.

## Release candidate attempt and bridge root cause — 2026-09-17T07:55Z

- **Proof runs 2–6:**
  - fixed daemon launch directory, squad-scoped delegation and failure reasons;
  - runs 4–5 failed on lane quality gates;
  - run 6 passed the lane and then hit the bounded reviewer claim.
- **Release tooling:** committed.
- **Candidate backend image:** built.
- **Web image:** stopped three times for host memory; recorded as blocked.
- **Bridge root cause:** written up with options A/B/C; B recommended.

## Pulse impersonation exclusion — 2026-09-17T05:25Z

- **First lane run failed:** its objective (ingest answers 202) was impossible,
  because the global read-only guard answers 403 first. The Codex reviewer
  caught that, and also an out-of-scope hack in the correction.
- **v2 succeeded:** template-only change plus tests, one correction, fails-before
  shown.
- **Branch re-run:** 93 Python and 28 Node tests passed.

## CRM Pulse audit and fix; lane v10/v11; proof collision — 2026-09-17T00:50Z

- Pulse audit in a scratch SMCrm clone at `1291141`, including SQLite probes.
  Main cause of undercounting: whole-batch rejection at partial navigation.
- Lane v10 (Python/mjs test recognition). Pulse fix run succeeded after one
  Codex-driven correction. Branch `514b6f1` in the scratch clone passes 29 Python
  and 28 Node tests; patch and bundle saved.
- The owner's issue proof collided with that run. Fixed the proof script and
  added the v11 single-run lock.

## Stalled hand-off fix — 2026-09-17T00:10Z (system clock)

- Completion now dispatches accepted, undispatched events once; if that fails,
  the issue gets a technical stop and an Inbox incident. Fails-before shown;
  62/62 handler and 22/22 service tests on a disposable Postgres, removed
  afterwards. Commit `539187f5e`; push blocked by the local classifier.

## Workflow missing-dispatch test — 2026-09-17T00:40Z

- Updated the stale test to the accepted replay rule and added a pinned finding
  test for an unrecoverable hand-off after completion; 61/61 on a disposable
  Postgres, removed afterwards.

## Parked-work block — 2026-09-16T23:30Z-00:15Z

- Built and smoke-tested the one-step `prove` mode; fixed stale-session reuse
  and per-surface error capture.
- Diagnosed the always-failing workflow recovery test; not edited.
- CRM: throwaway clone at `1291141e`, candidates checked, none fit; clone
  removed, `C:/Dev/CRM` untouched.

## Issue-started lane milestone — 2026-09-16T22:45Z-23:25Z

- Probed Codex non-interactively. The operator's config pulled in an MCP server,
  and a default fresh home blocked file reads and let Codex guess from the web.
  A minimal isolated home reads correctly.
- Lane v9: Codex reviewer (`gpt-5.6-luna`/high), fallback recorded, 54 tests.
- Built the issue bridge (Go) and the local instance script. Local server,
  owner sign-in and workspace succeeded.
- Daemon start was refused by the session safety check. Not worked around;
  local server and database stopped.
- Codex proof run: executor avoided the trial 2 defect, and Codex found 2 real
  issues. Passed after one correction. Applied; 547 + 90 tests pass, `tsc` clean.
- CRM job skipped (depends on the blocked step).

## Lane v8 and trial 4 — 2026-09-16T22:00Z-22:45Z

- Owner, via the planning session: no reboot, ever; fit the lane to the machine
  and record peak memory.
- Built v8: warm install via junctions with workspace redirection, a dependency
  fingerprint, backoff memory waits, a process-tree sampler. 50 tests. Real-repo
  smoke test passed.
- Trial 4, five runs: 1-4 clean, 5 passed after its correction (a `tsc`
  regression caught an unused import). $6.54; lane peak 770-1,187 MB; no waits,
  no kills.
- Human checks against the Go handlers found no functional defect; one false
  comment in run 5 was corrected.
- Applied as `91ef80938` after merging overlapping patches by hand (`tsc` caught
  one bad merge). 535 + 556 tests pass, `tsc` clean.

## Owner answers acted on — 2026-09-16T20:00Z-21:00Z

- Standing rule received: make engineering decisions without asking; stop only
  for money, external visibility, irreversible deletion, live deploys, Phone Farm
  or business choices.
- Old checkout compared read-only (`GIT_OPTIONAL_LOCKS=0`, no object writes).
  1,696/1,749 identical, 51 unique files archived and verified on `A:`.
- Cancel-task and leader-check panics fixed, tests proven on a disposable
  Postgres. A pre-existing failing workflow test was found.
- Lane v6 frozen, trial 3 started. Runs 1-2 passed; run 3 was killed during
  review and run 4 during install, both by the session for low memory. Both were
  cleaned up by hand after the guard confirmed nothing uncaptured; authority was
  revoked.
- Applying runs 1-2 exposed two defects no gate saw; fixed and committed.
- Lane v7 frozen with regression checks and a narrower contract check; trial 4
  pre-registered.
- Pushed to the private remote. No deployment, no production data, nothing
  public.

## Trial 2 on lane v5 — 2026-09-16T19:13Z-19:50Z

- 19:13Z owner: "memory is down. Do it now." Memory 81% load, 5.7 GB available.
- Online copy: secret scan of the 37 non-upstream commits found no local secret
  value. `gh repo create --private` was refused by this session's safety check.
  Nothing created or pushed.
- Five task specs pre-registered in `42b306e6b` before any run, all from the
  recorded API compatibility rule: every response must pass through a schema,
  with a malformed-response test.
- 19:24Z-19:45Z five runs, one each: every run admitted, freeze intact, first
  attempt, no correction, fails-before detected on all, cleanup safe on all.
  $3.92 total.
- Human check: runs 1-4 no defect. Run 5 schema requires `issue_id`, which the
  server omits for chat tasks, so chat timelines would render empty. Tests and
  reviewer missed it. **Trial not clean.**
- Applied runs 1-4 as four commits; 467/467 core API tests; core typecheck clean.
- Found: saved patch files are CRLF-mangled (text-mode write); `CancelTask`
  panics on malformed task id.

## Lane revision 2 built and frozen — 2026-09-16T18:30Z

- Owner approval relayed through another session: online copy first, then the
  lane revision and five more tasks. The repository creation and push were not
  performed on the relayed approval; read-only checks only. Nothing created or
  pushed.
- Built the approved revision: Go commands for Go tasks, orphan sweep,
  mechanical fails-before check, no JS install for Go tasks, reviewer
  out-of-scope list, cleanup guard, memory floor, richer receipts.
- Found while building: the server's DB tests default to `localhost:5432`, where
  a native PostgreSQL listens, so the lane now points `DATABASE_URL` at an
  unreachable port. Other apps' database connection variables and this session's
  messaging token were in the inherited environment, so they are now withheld.
  `GOTOOLCHAIN=local` broke Go (the host needs a toolchain switch) and was
  removed.
- 36/36 provider-free tests. The Go slug test passes offline under the sandbox
  environment. Frozen as v5 with hash enforcement. Clean-pass definition
  written before any run.
- Live admission at 18:26Z refused on memory: 91% load.
- No provider run, no deployment, no push, no production data change.

## Implementation-lane repeatability trial complete — 2026-09-16T09:05Z

**Plain result: five real code changes completed through the new lane, each on
the first attempt of its final run with no correction. GO for supervised use;
not yet the unsupervised standard.**

- 06:30Z-07:10Z — task 2, 180 inbox translations. First run failed: the parity
  test passed and the executor self-certified, but the independent reviewer
  caught a wrong character for "Hold". Its correction was silently denied by a
  harness permission defect (fixed with `acceptEdits`, lane freeze v2). The
  re-run passed first attempt at $0.33 and the parity suite is 160/160. This
  session's earlier claim that bulk work is expensive was retracted; that cost
  was the permission defect.
- 07:24Z-07:47Z — task 3 blocked by host and harness: two host memory kills
  before any provider session (freeze v3), an invalid pnpm flag, and a task spec
  whose baseline already passed, correctly refused by the lane's own guard.
- 07:47Z — task 3 passed, but its two new files were never captured and were
  deleted with the worktree: the most serious harness defect, a run that passes
  every check and loses the work (fixed, freeze v4).
- 07:57Z — task 3 re-run passed first attempt with the same design as before.
  Verified by hand that 2 of its 5 tests fail against the old code, and that
  whole-suite differences were memory-pressure flakiness (workspace-tab 8/8 six
  times with and without the change).
- 08:28Z — task 4 passed first attempt. The reviewer justified a change with a
  false claim about Go's `path.Ext`; the change was verified harmless on 3,026
  inputs. Both sessions missed a false "CI re-runs the generator" claim,
  corrected by hand.
- 08:44Z — task 5 passed first attempt. The executor checked the TODO's
  "immutable, safe to cache" premise, found it partly false (hard delete allows
  slug reuse) and declined to cache. It also made one unrequested but verified
  priority change in slug resolution. Go build, vet and 11 database-free
  subtests pass.

Totals: 13 runs, $7.77, 53 minutes of agent time; $3.29 for the five final runs.
Seven of eight problems were harness, spec or host; one was the agent, and the
reviewer caught it.

Reconciliation next step identified and not performed: a stable
`silver-mediums/canonical` branch checked out at `C:/Dev/multica` from the shared
repository, plus a bundle to `A:/Multica-Recovery`, leaving the dirty
`C:/Dev/SMK-multica-selfhost-pilot` checkout untouched.

Full record: `IMPLEMENTATION-LANE-20260916.md`.

## Implementation lane built, proved and frozen — 2026-09-16T05:50Z

**Plain result: an agent changed real code for the first time, under full
safeguards, and it worked.** The document-only repeatability trial was paused on
the owner's decision after this session showed the accepted publication workflow
cannot change code at all.

- 05:00Z — traced in source that the accepted publication workflow's provider
  child has no tools and no repository access, and that every accepted
  "implementation → review" proof used a **fake** provider. Reported it as a
  blocker rather than routing around it. The owner chose to pause the
  document-only trial and build a code-changing lane first.
- 05:10Z–05:25Z — preserved the accepted state. 1,635 files committed across four
  commits by explicit path; working tree clean; nothing pushed; the other three
  checkouts untouched. Scanned for secrets first and found none.
- 05:25Z — archived the gitignored 1.7 GB `.qualification-evidence/` tree to
  `A:/Multica-Recovery/Evidence-Archive/` with a per-file SHA-256 manifest, and
  verified it by reading all 340 members back **out of the archive** rather than
  re-reading the originals. The 28 existing encrypted backups are untouched.
- 05:30Z–05:40Z — built `scripts/implementation-lane/`: isolated detached
  worktree, offline lockfile-pinned dependencies, executor session confined by
  the CLI's `--restricted` mode and a tool allowlist, harness-run verification,
  independent read-only reviewer session, at most one bounded correction,
  cleanup. 16 provider-free tests pass. Worktree lifecycle verified separately
  before any provider call.
- 05:40Z–05:42Z — two harness failures, both recorded and neither retried into a
  success: `pnpm` is a `.CMD` shim `subprocess` cannot resolve without a shell,
  and subprocess text decoding defaulted to cp1252 and could not read the CJK
  bundles. Both times the safety machinery worked correctly — authority revoked,
  worktree removed, nothing left behind. Corrected the lane and re-attempted as a
  new run.
- 05:45Z–05:50Z — first real proof, run `20260916T054524Z-3f9f7e6f`: six missing
  chat translations in ja, ko and zh-Hans. Baseline failed as expected, executor
  49.2 s over 11 turns, harness-run verification then passed, containment showed
  3 files touched and 0 outside scope, independent reviewer passed in 4.4 s, no
  correction needed. $0.3521 cost-equivalent. Translations verified accurate
  against the English source and committed as `243c5a6df`.
- 05:52Z — froze the lane in `FROZEN.json` so the repeatability trial runs
  against unchanged sources.

Trial: task 1 chat parity done; task 2 inbox parity (180 keys, volume and long
context) running; task 3 the workspace-leave known debt at `CLAUDE.md:179`, a
**judgment** task with no existing test where the executor must decide what
correct behaviour is, write the test, then fix it. Tasks 4 and 5 to be selected
from recorded evidence.

Task 3 was added on a proposal relayed from the owner's planning session by
another Claude session; this session independently agreed. Recorded because the
reasoning matters: 180 keys is *bigger* than 6, not *harder* — a test already
defines correct in both. Without a judgment task the trial would leave the
day-to-day question unanswered.

The go/no-go will answer three questions separately: does it repeat, is the work
good, is it worth the cost.

Full record: `IMPLEMENTATION-LANE-20260916.md`.

## Controlled operational adoption DEPLOYED — 2026-09-16T03:12Z

**Plain result: the accepted automatic workflow is now usable for ordinary
internal issues, under a written rule the owner can switch off; two recorded
defects are fixed; and the app carries a second product identity.** Deployed to
the private instance and verified. Zero provider calls, zero credential reads,
zero dispatched issues.

- 02:10Z–02:20Z — corrected both acceptance harnesses and three recovery scripts
  to resolve PowerShell 7 without relying on PATH. Windows PowerShell 5, a
  zero-length Microsoft Store alias, and a bad explicit override are all rejected;
  the daemon-stop identity guards are byte-identical and the daemon-id pattern is
  now asserted by test. Pre-correction harness bytes preserved and hash-verified.
- 02:12Z–02:20Z — corrected `Verify-OneDriveRecoveryCopy.ps1` to wait for the
  cloud copy to reach its recorded size and settle before hashing, to re-issue the
  dehydrate verb while waiting, and to give off-machine custody its own budget.
  The exact-hash requirement is unchanged: a settled, complete, non-matching copy
  still fails, and fails promptly rather than after the timeout.
- 02:12Z–02:20Z — added the initial operating configuration, its controller
  lifecycle registry, an immutable transcription of the accepted run, and a
  provider-free checker command. One owner-started internal issue at a time, two
  provider tasks, zero corrections, zero retries, zero permanent agent seats,
  twelve owner-gated stop conditions.
- 02:20Z–02:30Z — added the secondary product identity in the sidebar footer and
  on all three sign-in steps, with an About surface, in four languages. The
  Multica mark, name, copyright and attribution are unmodified and now more
  visible than before.
- 02:30Z–02:50Z — one independent review returned REVISE with 15 findings; all 15
  addressed in one consolidated correction. Four were record-honesty defects: a
  gate described as enforcing that nothing consults, a revocation a two-line edit
  could undo, a live configuration with no recorded activation, and a failed
  review with no declared destination. One About claim could not be substantiated
  and was reworded to the skills feature that exists.
  `provider-unavailable: orca runtime_unavailable (orca-runtime.json absent) |
  fallback: in-session independent agent instance | lane: independent review`.
- 02:46Z — rebuilt. The build-input delta against the deployed release is exactly
  17 files: 10 added, 7 changed, 0 removed. Nothing unrelated.
- 02:54Z–03:04Z — release gates passed, then backup `20260916T025449Z-ea0d859d`,
  isolated restore `ae3e246f4e`, and application/rollback rehearsal
  `app8701907fc9` against **both** the new and the rollback image on the restored
  database: 603 migrations unchanged, identical row counts, cleanup verified.
  Rollback proved before deploying.
- 03:04Z — deployed `60fea34b9770-local-24b0121ef7a0`. Health ok, readiness
  `db: ok` / `migrations: ok`, container identity and health matched.
- 03:04Z–03:12Z — post-deployment verification: data preserved exactly (1 user,
  1 workspace, 51 issues, 149 comments, 120 tasks, 0 runnable, 603 migrations,
  10 active agents, 0 online runtimes); runtime policy unchanged; loopback-only
  listeners; private Tailscale route with no public Funnel; operating
  configuration re-checked provider-free on the deployed source. Closing recovery
  point `20260916T030645Z-89971836` verified end to end.
- Both OneDrive readbacks passed on the **first** attempt on a 735,238,104-byte
  artifact — roughly twice the size of the 361 MB artifact that used to fail —
  with `local_copy_attempts: 2`. The defect fix is proved in production
  conditions, not only in tests.

Rendered branding was verified with Playwright against real Chromium: five
checks on the candidate at 200 / 256 / 360 px, phone 390 px and 1920 px, and
three against the deployed instance at 390 / 1280 / 1920 px. Neither the Orca
Browser (runtime unavailable) nor the Chrome extension (not connected) was
available. The signed-in surface was rendered against the candidate preview,
whose frontend image is byte-identical to the one now serving production; no
account, session or data was created on the private instance to check branding.

Recorded, not outstanding: a probe that imported
`scripts/completion-recovery-rehearsal.py` executed it — these scripts run on
import. It passed, cleaned up and left production untouched, but it overwrote one
prior rehearsal receipt for the same release and check. Run these scripts; never
import them.

Also recorded: after deployment, the offline release-wrapper test caught that
the bounded image-archive check assumed every index entry declares a media type.
It now treats a missing one as an image, so nothing can be demoted into the
ignored branch. `scripts/` is not a build input, so the deployed images are
unchanged; both the new and the previously accepted archives were re-verified
against the corrected checker and their receipts still hold.

Also recorded: nine `agentqualification` tests and the `inbox`/`chat`
locale-parity tests were already failing before this change. The failing set was
captured with and without the new files and is byte-identical.

Full report: `ADOPTION-MILESTONE-RESULT-20260916.md`. Configuration:
`INITIAL-OPERATING-CONFIGURATION-20260916.md`. Evidence:
`.qualification-evidence/adoption-20260916/` and
`.qualification-evidence/production-adoption-20260916/`.

Next: nothing outstanding. Starting a real issue under this configuration is a
separate owner action. Widening the work class, enabling the correction cycle,
adding a second review provider, or granting recovery-sensitive authority each
need a successor record and its own evidence.

## Native publication acceptance PASSED — 2026-09-16T01:24:29+00:00

Evidence stage: runtime behaviour, on the exact deployed private release.

Owner granted one final bounded native acceptance. It passed.

- 00:19Z — captured the preservation baseline for SILV-38, SILV-39 and workspace
  totals (50 issues, 118 tasks, 0 active, 0 online runtimes, 0 daemon tokens).
  Confirmed the deployed source `60fea34b9770-local-9eaa711d97b1` and image
  `e710767a6769` healthy, and no drift in the reviewed harness sources.
- 00:29Z — created fresh bindings rather than resuming the cancelled v2 trial,
  whose `stopped_by_owner` record is left untouched. Harness
  `scripts/production-pilot/completion-acceptance-v3/` is a binding-only
  derivation of the reviewed v2 baseline; every file reproduces byte-for-byte by
  substituting the directory name, role-name suffix, profile/daemon date and
  issue title. Provider-free checks passed first: 13 broker tests plus 4
  byte-boundary subcases, and 6 mocked controller tests.
- 00:35Z — `prepare-resources`: new issue #40 in Backlog, offline private
  runtime, three private roles, squad and workflow plan. Zero tasks, zero
  provider contact.
- 00:39Z — `build-bound`, `provision-daemon`, `start-daemon`. Runtime online with
  a scoped expiring daemon token; provider profile still absent.
- 00:44Z — `prepare-authority` rejected the run: the authority helper pinned the
  superseded v1 issue title, which neither v2 nor v3 could ever satisfy. Aligned
  that one binding to the acceptance issue title, requalified, rebuilt. The check
  itself is unchanged — exact title, exact squad, backlog, zero tasks.
- 00:46Z — three exact authority receipts issued.
- 00:51Z — `prepare-provider`: the existing Claude Code subscription copied
  opaquely into the private task profile. Credential bytes never parsed, printed
  or hashed.
- 00:52Z — `launch-publication`: one owner mention of the publisher, planner and
  reviewer suppressed.
- 00:53Z — both turns completed. Publisher task
  `01a0a7b3-ae94-71b3-bd0f-ed7ccab3b958` → `implementation_ready`; the workflow
  routed reviewer task `01a0a7b4-1287-715c-bfc0-c9527b1cfb9d` on its own →
  `review_passed`. Zero failed tasks, zero retries, zero corrections.
- Summaries complied with the corrected instruction: publisher 269 characters,
  reviewer 300, both ASCII, both far under the retained 600-byte ceiling. This is
  exactly what SILV-39's 926-byte summary failed.
- Publication integrity: parent is the owner's launch comment, the approved
  digest is quoted, and the published body is byte-identical to the approved
  candidate. Provider frames show only thinking/text/result — no tools, no MCP.
- Surfaces: Issue `done`; Current Picture stage `complete`, outcome `satisfied`,
  attention `resolved`, zero unresolved incidents; Now lane `done` with owner
  action required **no**; Inbox one `task_completed` info entry,
  "independently verified and complete", no `workflow_incident`; Board lane
  `done`. Lanes derived with the product's own `operationalLaneForIssue` rule
  against the live API. API and projection evidence, not a screenshot — the Orca
  Browser path was excluded by the owner's instruction and the Chrome extension
  was not connected.
- 01:07Z — cleanup. Three of three authority receipts revoked, runtime
  deregistered, daemon token deleted, both temporary profiles removed, owner
  session removed, three temporary agents archived. The harness's own daemon stop
  failed because it calls `pwsh`, which is not on this host's PATH; the process
  was stopped separately under the same identity guards and verified absent.
  Independent readback: `fully_clean: true`.
- Preservation confirmed: SILV-38 and SILV-39 byte-identical before and after.
  Workspace totals moved only by this run, 50 → 51 issues and 118 → 120 tasks.
- 01:13Z–01:24Z — recovery closure on run `20260916T011317Z-a6a72487`: backup
  verified, isolated restore verified (145 tables, uploads manifest matched),
  application/rollback rehearsal verified against both the current and rollback
  images with 603 migrations unchanged and identical row counts, OneDrive
  off-machine readback verified at 01:23:37Z by dehydrate/rehydrate hash match,
  watchdog healthy. The OneDrive step needed one retry — the verifier hashes the
  local cloud copy without retrying while a ~361 MB artifact is still uploading.
- Production was never restarted or modified: `started_at 2026-09-15T22:12:43Z`
  before and after, all three containers healthy on the same images.

No retries, extra provider turns, tools, MCP, unrelated implementation, public
exposure, outbound communication, Orca or Phone Farm access were used.

Full report: `NATIVE-ACCEPTANCE-RESULT-20260916.md`. Evidence:
`.qualification-evidence/native-acceptance-v3-20260915/`.

Next: nothing outstanding from this grant. A permanent role/model selection, a
wider automatic-execution scope, or any further provider run each need separate
explicit authorization.

## Fresh bounded native trial v2 STOPPED by owner — 2026-09-15T23:02:18.159389+00:00

**Current outcome: authorization revoked before any live setup or provider call.** Zero publisher/reviewer runs, zero new issues/agents/runtimes/tokens, zero active tasks and zero online runtimes. Production remains 50 issues and 118 historical tasks on source 60fea34b9770-local-9eaa711d97b1. SILV-38 assisted completion and the failed SILV-39 observation are preserved; native automatic success remains unaccepted.

Exact interruption: copied the frozen harness to scripts/production-pilot/completion-acceptance-v2, applied only the approved summary instruction and fresh binding names, and ran provider-free checks. Thirteen Go tests plus four byte-boundary subcases and six mocked controller tests passed. The local test process finished; no temporary execution remains. prepare-resources, build-bound, daemon provisioning, authority issuance, credential copying and launch were never invoked. No temporary runtime, authority, profile or credential required removal; absence was verified. Local qualification is marked stopped_by_owner so the harness's existing qualification gate rejects execution. Preparation and evidence are retained.

### Claude-ready continuation

- Worktree: C:/Users/a9sho/.codex/worktrees/completion-20260915/SMK-multica-selfhost-pilot. Original owner/A: worktrees and all failed histories remain preserved.
- Read .qualification-evidence/native-acceptance-v2-20260915/interruption.json and test-stop.json first. Fresh state is .qualification-evidence/production-pilot-SILV-38-completion-acceptance-v2; it contains no issue, launch, consumed slot, authority or credential binding.
- Treat this trial as cancelled. Do not resume the prepared harness, re-enable its qualification, call a provider, start a reviewer/retry, deploy or begin another slice. Further work requires a new explicit owner instruction. The earlier fresh-run approval has been revoked.
- Latest verified recovery remains 20260915T223229Z-50923941: isolated restore, corrected/rollback application checks, OneDrive hash readback at 22:45:45Z and healthy watchdog. No production change occurred after that closure in this interrupted trial, so no additional recovery run was started.
- The prompt proposal is locally tested only. Automatic completion is still unaccepted. The earlier correction result report and SILV-39 failure evidence remain authoritative for completed work.


Append-only operational record. Never store secrets or raw private workspace
content here.

## 2026-09-09 - Initiative created

Evidence stage: local-only

- Preserved the dirty CRM checkout.
- Preserved `C:\Dev\SMK-multica-control-plane`; it contains unfinished Astra
  work and is not the pilot source.
- Preserved the dirty `C:\Dev\Multica-Pilot-Sandbox` checkout.
- Cloned official Multica into `C:\Dev\SMK-multica-selfhost-pilot`.
- Created branch `codex/selfhost-pilot-20260909` from upstream main at
  `9fab6da917953c6f1a967a17beef045aff3d0ce5`.
- Latest published tag observed locally: `v0.4.42`.
- Verified Docker CLI 29.3.1 and Compose 5.1.1 are installed.
- Docker Linux engine probe failed because Docker Desktop was not running.
- No container, database, daemon, cloud workspace, production service, SMS,
  or repository outside this new checkout was changed.

Next: boot the pinned local stack while keeping the cloud daemon configuration
untouched.

## 2026-09-09 - Docker startup prerequisite blocked

Evidence stage: local-only

- Launched Docker Desktop 4.68.0 in the background.
- Docker created its Linux-engine pipe, but CLI server queries never returned.
- Docker's host log recorded a fatal failure initializing the inference manager
  because it could not remove
  `C:\Users\a9sho\AppData\Local\Docker\run\dockerInference`.
- Verified that exact object is a zero-byte reparse point under Docker's local
  run directory and predates this launch.
- Attempted Docker's own shutdown command; it hung. Stopped only the crashed
  Docker Desktop/backend processes.
- Attempted a recoverable rename of the exact stale object; Windows refused
  access. No Docker data, volume, image, configuration, or workspace was
  deleted.
- WSL reported `Wsl/Service/CreateInstance/E_UNEXPECTED`, so it could not safely
  move the object from inside Linux either.
- Investigation log:
  `C:\tmp\bugfix-20260909-1640-docker-desktop-stale-inference-socket.md`.

Current blocker: Windows must restart to release the malformed socket and WSL
state before the Docker engine can be verified. Do not use Docker factory reset.

Next: after restart, run a Docker server-version probe. Continue the Multica
stack only if the engine answers normally.

## 2026-09-09 - Local infrastructure gate passed

Evidence stage: runtime-verified for local infrastructure only

- Drew reopened Docker Desktop. Engine 29.3.1 answered normally; no Windows
  restart or Docker factory reset was needed.
- Created an ignored `.env` from the official example with random JWT,
  PostgreSQL, and VCS secrets. Secret values were not printed or committed.
- Pinned both Multica application images to published release `v0.4.42`.
- Started three loopback-only services: PostgreSQL 17 with pgvector, Multica
  backend, and Multica frontend. Raw application ports are not externally
  exposed.
- Backend `/healthz` reported status, database, and migrations `ok`.
- Frontend returned HTTP 200 and the built-in browser visibly rendered the
  clean local `/login` page.
- Restarted the entire Compose stack. PostgreSQL retained the same system
  identifier and exactly 485 migration rows before and after restart. Backend
  and frontend recovered healthy with zero container restarts recorded before
  the controlled test.
- Persistent Docker volumes exist for PostgreSQL and backend uploads.
- Existing Multica cloud CLI/daemon configuration was not changed. No account,
  workspace, agent, issue, SMS, public endpoint, or migration was created.

Next: obtain action-time confirmation to create one disposable account on the
local instance, then lock open signup.

## 2026-09-09 - Local account and workspace created

Evidence stage: runtime-verified for local authentication and configuration

- Used the local login page at `http://127.0.0.1:3000`; no public or hosted
  Multica environment was used.
- The local instance generated a login code but could not email it because no
  email provider is configured. Retrieved only the newest code from the local
  backend log and used it without printing the full log or recording the code.
- Completed local sign-in and skipped the optional personal profile survey.
- Created the disposable workspace `Silver Mediums Pilot` with issue prefix
  `SILV`.
- Changed the ignored local `.env` from `ALLOW_SIGNUP=true` to
  `ALLOW_SIGNUP=false`, recreated the backend container, and verified
  `/api/config` reports signup disabled.
- Rechecked backend health after recreation: application, database, and
  migrations all reported healthy. All three local containers remain running.
- Existing hosted Multica configuration, daemon, workspace, and agents were
  not read, changed, or reconnected.
- Source documentation confirms `--profile <name>` isolates server address,
  token, default workspace, and daemon state under
  `~/.multica/profiles/<name>/`. The current Windows shell does not resolve a
  `multica` executable, so no CLI profile or daemon has been created yet.

Next: install or locate the official CLI, create only a named pilot profile,
and prove its files and daemon state are separate before running a test agent.

## 2026-09-09 - Isolated daemon and assignment tests

Evidence stage: runtime-verified for the local pilot only

- Installed official Multica CLI `v0.4.42` from the published Windows release;
  the installer verified the release checksum.
- Created named profile `silver-mediums-pilot` under its own profile directory.
  The existing default and Desktop profiles were not repointed.
- Authenticated the named profile against the loopback-only pilot and started
  daemon `v0.4.42`. It sees one workspace and the Codex, Claude, Grok, and
  OpenCode runtimes available on this computer.
- Created local Mika on Codex `gpt-5.6-sol`, medium reasoning, with concurrency
  limited to two.
- First smoke test `SILV-1` proved chat-to-issue creation and the requested
  result, but failed the ownership gate: the issue stayed unassigned and had
  zero issue runs. Mika did the work directly in the chat run.
- Added tracked pilot Mika instructions and the `Pilot Operations` squad led by
  Mika. The instructions require squad ownership before issue work begins.
- Second smoke test `SILV-2` was assigned to `Pilot Operations`, started a real
  issue run, retained squad ownership, posted exactly `SQUAD ROUTING VERIFIED`,
  and completed Done without repository or external-system access.
- The second test exposed cost/latency debt: a trivial request used a full chat
  intake run followed by a full squad leader run and took about 90 seconds.
  Manual squad assignment bypasses intake, but is a fallback rather than the
  intended owner workflow.

Next: separate lightweight intake from execution, add a distinct reviewer, and
prove truthful Working/In Review transitions without requiring Drew to route
ordinary work manually.

## 2026-09-09 - Pilot squad roles separated

Evidence stage: runtime-verified configuration; review flow not yet verified

- Added `Pilot Worker` on Codex `gpt-5.6-luna` with low reasoning and one-run
  concurrency for bounded execution.
- Added `Pilot Reviewer` on Codex `gpt-5.6-sol` with medium reasoning and
  one-run concurrency. Its instructions prohibit implementing work it reviews.
- Added both agents to `Pilot Operations` as executor and reviewer while Mika
  remains leader. Added explicit same-issue worker/reviewer squad instructions.
- This configuration does not yet prove that Mika performs the handoffs or
  truthful Working/In Review transitions. That requires the next controlled
  issue.

Next: run one deterministic worker-to-reviewer handoff, measure runs/tokens and
status transitions, then decide whether stock orchestration is efficient enough
or needs a backend change.

## 2026-09-09 - Full squad and review smoke test

Evidence stage: runtime-verified local pilot behavior

- Created controlled issue `SILV-3`, assigned to `Pilot Operations` from the
  start. It remained squad-owned for its entire lifecycle.
- Mika moved the issue from Todo to Working and mentioned Pilot Worker inside
  the issue. Pilot Worker posted the required exact result.
- The worker comment automatically woke Mika. Mika moved the issue to In Review
  and mentioned Pilot Reviewer inside the same issue.
- Pilot Reviewer independently checked the result and posted the required exact
  pass. That comment automatically woke Mika, which marked the issue Done.
- Drew did not poll, route an individual agent, or trigger either handoff.
- The issue reached Done 2 minutes 59 seconds after creation; the final Mika run
  completed after 3 minutes 13 seconds.
- The correct result required five agent runs: three Mika runs, one worker run,
  and one reviewer run. Aggregate usage was 65,611 new input tokens, 5,544
  output tokens, and 864,000 cached input tokens.
- Result: squad ownership, truthful Working/In Review transitions, automatic
  handoffs, and independent review passed. Token and latency acceptance failed.

Next: locate the stock leader-wakeup path and replace deterministic transitions
with bounded orchestration that does not require a full Mika run after every
specialist comment. Repeat the same test before broadening the pilot.

## 2026-09-09 - Model research distilled

Evidence stage: supplied research recorded; product behavior not yet changed

- Read all three owner-supplied research notes about model capability, Luna's
  economical role, and dynamic routing architecture.
- Added `MODEL-ROUTING-RESEARCH.md` as the durable pilot policy and explicitly
  separated research leads from runtime-verified facts.
- The policy treats agents as roles and models as per-stage compute, defaults
  routine work toward the cheapest capable tier, and escalates only for
  ambiguity, risk, conflicting evidence, or demonstrated bounded failure.
- The policy also requires compact escalation packets, deterministic workflow
  transitions, retry caps, minimal evidence retrieval, and per-run telemetry.
- No agent model, squad, hosted workspace, provider, or production system was
  changed from this research alone.

Next: isolate why `SILV-3` reloaded 864,000 cached tokens and determine whether
stock Multica can select models per stage before implementing the smallest
pilot-only routing change.

## 2026-09-09 - Mixed-model workflow and completion-rule repair

Evidence stage: source-tested and runtime-verified on the local pilot

- Changed only pilot agents: Mika to Luna/low, Pilot Worker remained Luna/low,
  and Pilot Reviewer to Terra/medium. Hosted Multica was not changed.
- `SILV-4` completed worker and independent-review work but remained In Review.
  Its final Mika run explicitly followed stock Multica's instruction to reserve
  Done for a human. That contradicted the pilot squad instruction to mark routine
  technical work Done after independent review.
- Added a regression contract, proved it failed against the old source, then
  changed the owning-squad status briefing. A different squad member's review
  may complete routine technical work; real business or owner acceptance still
  waits for the human.
- Ran the focused handler contract tests against a separate migrated PostgreSQL
  database: pass.
- Built `multica-backend:dev` from this checkout and recreated only the local
  pilot backend. PostgreSQL and the frontend remained in place. `/health`
  returned OK and the frontend returned HTTP 200.
- `SILV-5` then followed the complete path automatically: squad owned ->
  Working -> Pilot Worker exact result -> In Review -> Pilot Reviewer exact
  pass -> Done. It reached Done in 2 minutes 3 seconds without Drew routing,
  reviewing, or approving technical completion.
- Five runs remain: three Mika, one worker, one reviewer. Usage was 79,277 fresh
  input, 4,131 output, and 362,496 cached input tokens. This is about 52% fewer
  total reported tokens than SILV-3, but fresh input increased about 21% and a
  single sample cannot establish stable savings.

Next: replace predictable post-worker and post-review coordination with bounded
deterministic transitions or a compact evaluator so Mika does not need three
full model runs for a routine successful workflow. Repeat the same test and
require fewer runs as well as lower tokens.

## 2026-09-09 - External model-diagram feedback assessed

Evidence stage: research recorded; no product or runtime change

- Read the owner-supplied ChatGPT feedback as educational input, not as an
  instruction or an authoritative understanding of Silver Mediums.
- Preserved the compatible ideas: static model-per-agent design first,
  responsibility-based agent selection, deterministic release machinery,
  compact independent-review packets, concise role instructions, selective
  shared knowledge, and code-enforced gates.
- Kept Mika=Luna, reviewer=Terra, hard engineering=Sol, and rare audit=Astra as
  test hypotheses only. `SILV-5` is not broad enough to validate those choices
  across real work.
- Explicitly rejected waking Mika for every meaningful specialist result as the
  default because the pilot already measured that behavior as a five-run token
  problem. Also rejected adopting the sample org chart or collapsing distinct
  deployment/verification truth into one vague state without an agent and state
  audit.
- No agents, models, squads, services, hosted settings, or runtime state changed.

Next remains the measured Phase 3 efficiency fix, followed by a
responsibility/permission/context/workload audit before broad agent redesign.

## 2026-09-10 - Failed-review correction and retry ceiling

Evidence stage: source-tested and runtime-verified on the local pilot

- Added structured failed-review packets and one automatic correction limit.
- A second failed review now stops at Blocked and alerts the squad leader rather
  than looping indefinitely.
- `SILV-7` exposed that workflow-plan creation is not yet enforced at intake.
- `SILV-8` exposed and reproduced a comment-before-event race that created
  duplicate reviewers and Mika relays. The run was contained and rejected.
- Changed controller-owned workflow comments to display-only and moved the
  missing-event recovery decision to the task-completion boundary.
- Focused database tests prove a dispatched event does not wake Mika and a
  missing dispatch wakes the squad leader exactly once.
- Installed backend commit `ecd79eb56` on the loopback-only pilot. Hosted
  Multica and production systems were not changed.
- `SILV-9` proved fail -> one direct correction -> independent re-review -> Done
  in five runs with one Mika intake and no leftovers.
- `SILV-10` proved a repeated failure stops at Blocked after one correction and
  produces one squad-leader escalation with no leftovers.
- Token efficiency remains unacceptable: SILV-9 reported 130,307 fresh input
  plus 1,017,344 cached input tokens; SILV-10 reported 184,935 fresh input plus
  1,294,080 cached input tokens.

Next: enforce workflow-plan creation before the first delegation, then prove
Needs You and exact-step resume. Do not optimize the full model roster until the
controller paths are stable enough to benchmark consistently.

## 2026-09-09 - Main Multica failures added as pilot acceptance cases

Evidence stage: owner screenshots and report; main workspace not operated

- Preserved the owner's report that approved releases and a specific CRM DM
  outcome remain difficult to observe and advance in main Multica.
- Screenshots showed Mika asking for release approval after the owner had
  already repeatedly authorized shipping, a Phone Farm investigation closed
  without its repair being implemented, stale/contradictory updates until the
  owner forced a new audit, and a recoverable skill-bundle failure that stopped
  at diagnosis until the owner asked whether it had actually been fixed.
- Long release runs showed real intermediate work but no controller-owned stall
  rule or concise owner-facing change summary. This is not evidence that every
  long run was hung; it is evidence that the owner cannot reliably distinguish
  progress from a stall.
- Added durable approval, investigation-versus-repair, recoverable-blocker,
  useful-update, release-stall, and messaging-outcome scenarios to the owner
  acceptance matrix.
- No main/hosted issue, agent, release, runtime, repository, or production
  system was changed.

Next remains local workflow-plan enforcement. These new scenarios belong in
later owner-decision, deployment, recovery, notification, and inbox slices.

## 2026-09-09 - Phone Farm release-regression report added to acceptance

Evidence stage: owner screenshots and report; Phone Farm host not accessed

- Preserved the owner's report that a later Phone Farm deployment can revive a
  previously fixed employee-facing bug.
- The supplied screenshots attribute one recurrence to a rebuilt UI bundle
  omitting an employee-access build setting. This is useful incident evidence,
  but it is not independently verified Phone Farm runtime truth from Windows.
- Added a permanent acceptance rule: every later release must verify the new
  change and the previously fixed employee-critical behavior against the exact
  source, assets, build configuration, and runtime contract being released.
- No Phone Farm repository, Mac runtime, hosted Multica state, or production
  system was inspected or changed.

Next remains local squad workflow-plan enforcement. Release-regression
execution and rollback proof belong in the later deployment slice.

## 2026-09-09 - Automatic squad workflow plan proven

Evidence stage: source-tested and runtime-verified on the local pilot

- Added a reusable, explicit workflow template to each opted-in squad. The
  template names worker, reviewer, optional release/verifier roles, owner and
  deployment gates, and the correction limit.
- The controller installs that template on an assigned issue before it creates
  Mika's first leader task. It does not overwrite an existing issue plan.
- Stale templates fail closed when a configured agent is archived or removed
  from the squad. Unconfigured squads retain stock behavior.
- Installed the template for local Pilot Operations. `SILV-11` was created
  already assigned to that squad; no manual issue workflow plan was created.
- Database timestamps prove the plan existed before Mika's first task.
  `SILV-11` then completed Mika once -> worker -> reviewer -> Done with exactly
  three completed runs, two events, and zero unfinished tasks.
- Usage was 77,898 fresh input, 4,177 output, and 568,576 cached input tokens.
  The controller path passed; token cost did not.
- Hosted Multica, Phone Farm, and production systems were not changed.

Next: implement Needs You as one structured owner decision with exact-step
resume. Do not redesign the full agent/model roster yet.

## 2026-09-09 - One owner decision and exact worker resume proven

Evidence stage: source-tested, production-built, and browser-verified on the
local pilot

- Added a structured owner-decision event containing one question, a
  recommendation, the consequence, and bounded answer options. Incomplete
  decision packets fail closed.
- The decision creates one durable Inbox alert transactionally. The Inbox keeps
  that alert above newer comment noise, renders the full decision as buttons,
  and archives it after the owner answers.
- Answering is restricted to the accountable member. The controller resumes the
  exact suspended workflow state and exact worker, and injects the recorded
  answer into that worker's briefing. Mika is not used as a relay.
- `SILV-12` exposed two live integration defects: newer comment noise hid the
  decision card, and task completion treated `needs_owner` as a missing event
  and woke Mika. Both defects received regression coverage and were corrected.
- `SILV-13` then proved the clean path in the real local UI: one Mika intake,
  one worker decision request, one visible Inbox card, owner click, the same
  worker role resumed, one independent reviewer, and Done. All four runs closed
  with no extra Mika run and no unfinished task.
- `SILV-13` used 123,767 fresh input, 5,192 output, and 529,408 cached input
  tokens across four runs. Workflow correctness passed; efficiency did not.
- Backend handler/reducer/CLI tests passed against the isolated migrated test
  database. The backend and frontend Docker production builds passed; only the
  repository's existing CSS pseudo-element warnings were emitted.
- A broader `cmd/multica` package run was not a clean gate: unrelated existing
  Windows/path and disk-usage tests failed or stalled. The owner-decision CLI
  tests are covered by the focused command run instead.
- Hosted Multica, Phone Farm, and production systems were not changed.

Next: implement the deployment and live-verification state path, including a
permanent-regression suite and rollback signal. SMS delivery, interruption
recovery, the final board, Mika response style, and model optimization remain
separate later milestones.

## 2026-09-09 - Feature implementation paused for system reconciliation

Evidence stage: documentation-only; no runtime or product behavior changed

- Drew rejected continuing feature-by-feature without a coherent definition
  of how Board, Issue, Inbox, Chat, phone/SMS, workflow state, owner feedback,
  release, recovery, and future squads fit together.
- Read the complete pilot planning set, the maintained Silver Mediums Multica
  blueprint, the role-coverage map, the go-live summary, and the relevant
  approved company decisions. Inspected the current owner-decision reducer,
  handler, Inbox grouping, mutation, and decision-card source.
- Confirmed documentation drift: this handoff stopped at SILV-5 while the
  journal reached SILV-13; the initiative still directed implementation; pilot
  role instructions still describe Mika relays superseded by the controller;
  retry exhaustion is stored as Blocked despite Blocked being defined as an
  external dependency; and the current Inbox/decision implementation does not
  yet satisfy the stronger intended owner experience.
- Created a temporary requirements draft and external interview log under
  `C:\tmp` so unresolved proposals are not mistaken for approved project
  policy.
- The canonical knowledge-hub checkout is heavily occupied with existing
  tracked and untracked work. It was consulted but not edited or registered as
  part of this pass; adding the pilot to the hub must use a clean, reconciled
  hub lane rather than overlapping that work.
- Updated only the initiative and handoff resume pointers. No code, local
  container, daemon, agent, squad, hosted workspace, production system, SMS,
  or Phone Farm state changed.

Next: complete the evidence/decision/conflict register, canonical lifecycle,
surface contracts, scenario catalog, and validation gates. Resolve genuine
business tradeoffs with Drew. Permanent agent and squad design comes last.

## 2026-09-10 - Hosted workflow and interaction census

Evidence stage: read-only hosted record audit; no production verification

- Audited all 33 Silver Mediums HQ issues at two snapshots and retrieved their
  run histories, active runs, usage, timelines, and root-thread indexes.
- Read the ten active hosted chat histories as interaction sequences: owner
  request, Mika reply, owner correction/repetition, issue action, and recorded
  outcome. The final chat snapshot contained 515 messages: 258 owner messages
  and 257 Mika replies.
- Found only two active runs among eight cards labelled Working/In progress.
  Found six generic In Review cards conflicting with explicit historical
  closeouts and multiple Needs You requests that are obsolete or require
  cross-issue reconciliation before asking Drew again.
- Found an active Inbox containing one raw assignment while 23 archived items
  were only task failures, mentions, assignments, and quick-create completion.
  It did not expose the current business consequences or the real owner work.
- Measured 2,827 issue runs, including 468 failures; 416 failures were
  quota/capacity-related. Metered records reported 46.49M input, 18.84M output,
  5.75B cache-read, and 103.83M cache-write tokens. Recorded dollar cost is
  unavailable because the tokens were marked uncosted.
- Documented recurring loops: claims before evidence, Working without
  execution, lost approval, repeated owner polling, audit mistaken for repair,
  internal review pushed onto Drew, ambiguous Live/deployed/verified language,
  and regressions discovered by the owner.
- Added `HOSTED-WORKFLOW-INTERACTION-AUDIT-20260910.md` as the evidence baseline
  for the overhaul. No hosted issue/chat/daemon, local runtime, production
  system, SMS path, or Phone Farm technical surface was changed.

Next: derive the scenario catalog and unified cross-surface contract from these
observed loops, then reconcile it with the acceptance matrix before resuming
implementation. Permanent agent and squad design remains last.

## 2026-09-10 - Scenario catalog and unified operating contract drafted

Evidence stage: requirements synthesis only; no runtime or product behavior changed

- Read the current acceptance matrix, workflow contract, full pilot evidence,
  role fixtures, model research, deterministic reducer, owner-decision handler,
  worker briefing path, and Inbox grouping/types.
- Ran three read-only analysis lanes: scenario catalog, cross-surface contract,
  and adversarial requirements conflict review. The primary agent reconciled
  their findings against the source and hosted audit rather than accepting any
  one lane as policy.
- Drafted fourteen required scenario families: normal work, UI direction,
  audit-to-fix, typed owner decisions, stalls, review exhaustion, quota,
  deployment, regression/recovery, interruption, cross-project/Phone Farm,
  notification outage, concurrency, and scope/cancellation.
- Replaced the unsafe single-status assumption in the draft with five related
  truths: business outcome, work execution, release condition, owner attention,
  and evidence. Board, Issue, Inbox, Chat, and phone are projections and
  interaction surfaces over the same durable records.
- Resolved technical contradictions in the requirements draft: generic text no
  longer implies approval; review cannot imply Live; internal retry exhaustion
  is not an external Blocked dependency; production failure does not blindly
  roll back over a newer/incompatible release; and multiple decisions or work
  branches cannot be collapsed into one representative Inbox item/state.
- Added requirements for rich UI feedback, owner-accessible previews, exact
  checkpoint recovery, evidence freshness/supersession, meaningful-progress
  clocks, issue-wide token/cost ceilings, platform outage notification,
  concurrency isolation, cancellation, and staged self-host migration.
- Discovery artifacts remain under `C:\tmp`: the main requirements draft,
  scenario catalog, owner walkthrough, and interview log. They are intentionally marked
  incomplete until bounded owner policy choices are ratified.
- No code, database, container, daemon, agent, squad, hosted workspace, SMS,
  production system, or Phone Farm technical surface changed.

Next: walk through approval continuity and notification examples with Drew,
finalize the requirements with no unresolved placeholders, then reconcile the
project acceptance matrix. Permanent agent and squad design remains last.

## 2026-09-10 - Bounded outcome authority policy ratified

Evidence stage: owner policy decision; implementation not yet tested

- Drew selected bounded outcome authority rather than exact-artifact approval
  or unlimited goal authority.
- The intended experience is: Drew states the business result once and the
  system completes everything it can safely complete without repeatedly asking
  about technical corrections, rebuilds, handoffs, retries, or restarts.
- Authority remains valid only while the recorded behavior, scope, target,
  audience, risk, exclusions, recovery conditions, and spending bounds remain
  unchanged. Material changes require fresh owner authority before the
  consequential action.
- Every changed candidate still requires applicable independent technical
  review, regression checks, target reconciliation, and production/business
  verification. Agent confidence cannot establish that a change stayed within
  bounds.
- Updated the discovery requirements and acceptance matrix. No code, runtime,
  agent, hosted workspace, SMS, production system, or Phone Farm technical
  surface changed.

Next: define the concrete owner interruption and SMS policy, then validate the
complete cross-surface walkthrough before finalizing the requirements.

## 2026-09-10 - Notification and owner-attention policy drafted

Evidence stage: requirements synthesis only; outbound remains disabled

- Re-read the maintained alert blueprint, go-live gate, hosted audit, and the
  prior notification-monitor evidence. Existing Observe-mode health and source
  candidates do not prove owner SMS, delivery, deduplication, or truthful
  notification admission.
- Defined four lanes: raw agent/task telemetry stays in Issue history; meaningful
  progress updates Board and Issue; decisions/consequences enter Inbox; only
  urgent business impact or time-critical owner action interrupts by SMS.
- Required deterministic severity, stable incident/decision fingerprints,
  provider reconciliation before ambiguous retry, separate detected/accepted/
  delivered/read/acknowledged/resolved states, paired recovery notices, bounded
  reminders, and independent Multica-outage detection.
- Strengthened the first-pilot reply boundary after adversarial review: SMS may
  acknowledge or attach discussion, but consequential approval/rejection uses
  the authenticated current-decision screen. Direct option-code authority is a
  later separately tested capability.
- Proposed defaults for owner validation: 10 PM–8 AM Eastern quiet hours for
  noncritical messages; no routine completion SMS; one Inbox completion for
  verified production Live; one SMS resolution after an urgent incident; and
  no backup approver/channel until Drew explicitly configures one.
- Added a twenty-case pre-activation matrix covering stale evidence, duplicate
  processes/callbacks, crash boundaries, dry-run isolation, recovery truth,
  provider ambiguity, quiet hours, unavailable owner, and Multica outage.
- No message, provider call, code, runtime, agent, hosted workspace, production
  system, or Phone Farm technical surface changed.

Next: obtain Drew's notification-preference decision, then finalize the
cross-surface requirements and reconcile the acceptance matrix.

## 2026-09-10 - Owner notification preferences ratified

Evidence stage: owner policy decision only; outbound remains disabled

- Drew rejected system quiet hours and will use device Do Not Disturb.
- Every independently verified completed business outcome must generate exactly
  one SMS. An agent run, build, review, deployment, or intermediate recovery
  finishing is not the promised business outcome and cannot trigger that notice.
- For the first pilot, SMS is the interruption and the authenticated current-
  decision screen is the authority surface. `ACK` only acknowledges; direct
  approve/reject option codes remain deferred until separately tested.
- Drew is the only recipient and approver. There is no backup recipient,
  fallback approver, or inferred substitute authority.
- Updated the specification, notification policy, owner walkthrough, discovery
  log, handoff, and acceptance matrix. Expanded acceptance coverage for typed
  responses, UI feedback, stale decisions, concurrent isolation, cancellation,
  quota/capacity, outage, and notification delivery truth.
- No message, provider call, code, runtime, agent, hosted workspace, production
  system, or Phone Farm technical surface changed.

Next: reconcile the remaining cross-surface owner experience into one concise
walkthrough and complete the requirements gate before implementation planning.

## 2026-09-10 - Concise cross-surface approval view prepared

Evidence stage: requirements presentation only

- Added `docs/silver-mediums/OWNER-APPROVAL-SUMMARY.md` so Drew can validate the
  whole owner experience without reading the long engineering specification.
- The summary preserves the one-truth surface split, bounded authority,
  independently verified completion definition, SMS choices, and the boundary
  between requirements approval and any later outbound/production authority.
- No code, runtime, agent, model, SMS, hosted workspace, production system, or
  Phone Farm technical surface changed.

Next: obtain one explicit yes/no on the concise owner experience. If accepted,
finalize the requirements record and begin implementation planning from the
scenario gates rather than resuming an older isolated controller slice.

## 2026-09-10 - Operating requirements accepted

Evidence stage: owner policy acceptance; implementation remains local-only

- Drew accepted the concise combined Board, Issue, Inbox, Chat, and phone
  experience and said the team was good to proceed.
- Marked the specification, walkthrough, and discovery checklist complete for
  implementation planning and local pilot tests.
- This approval does not activate outbound SMS, migrate hosted Multica, deploy
  production changes, permit Windows Phone Farm technical work, or choose the
  permanent agent/model roster.
- No source code, runtime, agent, model, provider, hosted workspace, production
  system, or Phone Farm technical surface changed in this step.

Next: research the existing self-hosted code paths and produce an ordered
mini-PR implementation plan with tests, migration boundaries, and rollback
gates. Do not implement until that plan is separately approved.

## 2026-09-10 - Item 8D regression recovery accepted locally

Evidence stage: local/fake-provider contract and disposable-PostgreSQL proof

- A current exact independent regression now reopens the requested outcome and
  creates one fingerprinted incident without rewriting the failed forward
  release.
- Known-good rollback authority can cross issue ownership but is exact to the
  workspace, target, generation, and release. It requires prior Live/Restored,
  exact passed independent review, and current unexpired runtime, business, and
  regression proof.
- Recovery creates a deterministic candidate and new monotonic generation in
  the incident issue, frozen only to the known-good build, artifact, runtime,
  and review lineage. Unknown or irreversible recovery contains and
  TechnicalStops instead of guessing.
- Restore requires a target-CAS-proven rollback plus four pairwise-distinct
  exact proofs from the frozen independent verifier. The incident resolves but
  the originally requested outcome remains open.
- The exact cross-issue DB vertical, repeated DB and pure suites, crash/replay,
  concurrency, stale-generation, containment, migrations, all six regression
  and restoration transaction failpoints, zero-residue predecessor-proof
  failures, formatting, compile, Go vet, sqlc vet, and diff checks passed.
  Independent final review returned GO with P0=0/P1=0.
- No real provider, production runtime, deployment, production data, outbound
  message, agent roster, browser, or Phone Farm system changed.

Next: item 9. Freeze one deterministic typed owner projection, complete its
batch/realtime read path, then build the operational Board and Issue Now
experience. Keep Inbox/Chat, mobile, SMS, and real provider adapters in their
later roadmap boundaries.

## 2026-09-10 - Item 9.0 owner-summary contract accepted locally

Evidence stage: local contract and test proof; no owner UI changed

- Added one server-owned typed owner summary derived only from validated
  Current Picture truth and copied identically into compact and detail reads.
- Aligned Go and Core v3 release semantics without relabeling legacy v2 data.
  Invalid, contradictory, duplicate, and unknown truth fails closed.
- Kept technical stop, recovery, owner attention, blocked work, verification,
  release, review, active work, automatic wait, Done, and verified Live
  semantically distinct. Restoration keeps the requested outcome open.
- Restricted deployment identity, automatic timers/events, next actions, and
  meaningful changes to exact typed evidence and finite stable codes.
- Focused/repeated Go, 308 Core tests, TypeScript, lint, vet, sqlc, formatting,
  and diff gates passed. Independent final review returned GO with P0=0/P1=0.
- No UI, runtime, provider, deployment, browser, production, roster, SMS, or
  Phone Farm system changed.

Next: item 9.1 only: finish batch and realtime propagation plus the paginated
meaningful-activity API before rendering Board or Issue components.

## 2026-09-10 - Item 9 operational Board and Issue Now accepted locally

Evidence stage: local implementation, tests, container runtime, and Browser

- Completed workspace-scoped batch/detail reads, typed realtime invalidation,
  and revision-bound newest-first meaningful activity.
- Replaced every Board grouping with one non-draggable operational view. It has
  no avatar clutter or `No value` lane and shows current state, meaningful
  progress, and next step from the server-owned summary.
- Verified local browser fixtures for active work, automatic waiting, Needs You,
  independent review, releasing, deployed-but-unverified, verified Live,
  regression, rollback, and Done. Legacy rows visibly fail closed as Status
  unavailable.
- The first Issue browser pass exposed that `GET /api/issues/:id` omitted the
  compact Current Picture used by the Board. Added the one workspace-scoped
  detail enrichment and a focused regression test, rebuilt the backend, and
  verified `SILV-906` now opens with `Now` first and shows `Deployed, not
  verified yet`, meaningful progress, owner need, and verification-pending
  release state.
- UI tests passed 219/219; full Core passed 145 files/1,778 tests; the complete
  handler package and focused detail regression passed; builds, health, vet,
  typecheck, lint, sqlc, and diff checks passed. The single final independent
  review returned GO with P0=0/P1=0; no reviewer loop was started.
- `SILV-901` through `SILV-910` are local browser fixtures marked in metadata.
  They do not claim real workflow execution, deployment, runtime, or business
  acceptance. No hosted, production, SMS, roster, or Phone Farm system changed.

Next: stop at the accepted Item 9 boundary. Item 10 begins only after an
explicit continuation.

## 2026-09-11 — Item 10 Inbox and Chat overhaul

Evidence stage: local implementation, focused tests, production container, and
built-in Browser. Hosted Multica, production, SMS, agents, models, and Phone
Farm were not touched.

- Inbox projects only genuine owner decisions, explicitly material incidents,
  explicitly meaningful updates, verified completions, and existing bounded
  creation/quota outcomes. Raw task failures, ordinary comments, review noise,
  heartbeats, and unclassified status changes are absent. Browser count was
  exactly Needs You 1, Incidents 1, Updates 1, and Verified 1 despite the
  underlying legacy noise.
- Inbox selection and archive use the attention-item id. Browser opened
  independent completion, decision, update, and incident records without
  issue-wide collapse.
- The bounded panel exposes what happened, Current Picture context when
  available, recommendation/consequence when available, latest meaningful
  progress, direct-work controls, comment/correction, evidence/full issue, and
  acknowledgment without embedding the full timeline. Profile photos were
  removed from rows.
- Authorization remains truthfully atomic: only Approve and Reject call the v2
  decision command. Correction, request revisions, partial acceptance, scope
  change, and cancellation are separate issue-bound directives. Cancellation
  first moves the issue to Cancelled with run suppression, then records the
  reason. Legacy decision rows without a v2 receipt visibly fail closed.
- Chat visibly states that it shapes new work and explains cross-issue
  questions, while decisions, corrections, evidence, and progress continue in
  Inbox. Mika's runtime prompt binds concrete work to exactly one issue and
  forbids using Chat to poll or relay workflow status.
- Focused Views Inbox tests passed 110/110, focused Core Inbox tests passed
  53/53, corrected control tests passed 14/14, focused Go handler and Chat
  prompt tests passed, TypeScript and scoped lint passed, and the production
  Docker build completed. It emitted only the pre-existing CSS optimizer
  warnings for `::highlight`.
- Browser proof covered the noise-free categorized list, item-addressable URLs,
  incident, verified completion, meaningful update, Needs You, bounded
  correction entry, evidence link, acknowledgment, legacy fail-closed warning,
  and the Chat boundary. Local rows are presentation fixtures only.
- The one final independent review returned NO-GO with P0=0/P1=5. It found
  ordinary-comment owner directives, missing real workflow producers,
  issue-wide archive leakage, raw-noise unread counts, and a prompt-only Chat
  boundary.
- One consolidated correction pass closed all five findings: typed/versioned
  transactional owner directives now stop old work; Current Picture projection
  creates durable owner records; archive and unread semantics are item-level
  and owner-relevant on both sides of the API; and server guards prevent Chat
  from mutating existing operational work.
- Corrected proof: focused Go handler/migration/command/projection suites PASS;
  Core 136/136 focused tests PASS; Views 76/76 focused tests PASS; both package
  typechecks PASS; production Docker build PASS with two pre-existing
  `::highlight` optimizer warnings.
- Built-in Browser proof on the rebuilt stack showed Needs You, Incidents,
  Updates, and Verified; independent item URLs; bounded controls; legacy
  fail-closed handling; and the Chat boundary. Isolated `SILV-911` then recorded
  a real `correct` directive with expected issue revision 1, exact owner text,
  durable comment link, and issue revision advancement to 2.
- Acknowledging the verified-completion record for `SILV-13` archived only that
  record: the separate Needs You record for the same issue stayed active, and
  the completion remained independently visible in Archived.
- No second reviewer loop was started. Browser fixture `SILV-911` is marked
  `item_10_authoritative_direction`; it proves the local action boundary, not
  production or hosted execution.
- Owner presentation correction: the four category totals are now full-width,
  keyboard-accessible toggle filters; decorative row icons were removed; title
  and meaningful-detail text each receive two lines; and the desktop Inbox pane
  now has a 320px minimum and 360px default instead of collapsing to 240px.
  Focused Views tests passed 68/68, Views typecheck and scoped lint passed, the
  production frontend image rebuilt, and the built-in Browser proved the
  Incident filter reduces the list to its one matching item and toggles back to
  all five. Item 11 remains untouched.

Next: stop at Item 10. Item 11 mobile parity has not begun.

## 2026-09-11 — Item 11 mobile parity

Evidence stage: local implementation, focused mobile/shared-web tests,
production exports, private host readback, and built-in Browser compact view.
Hosted Multica, production, SMS, agents/models, and Phone Farm were not touched.

- Mobile Inbox now uses the accepted owner-control projection: genuine Needs
  You, important incidents, meaningful updates, and verified completions only.
  Each material inbox item remains independently addressable, including
  multiple records for one issue, and acknowledgment archives only that item.
- Rows use their width for two-line titles and meaningful details; actor avatars
  and duplicate status glyphs are gone. The four counts are usable filters.
- Tapping a record opens a bounded full-screen control surface with current
  state, meaningful progress, next step, recommendation/consequence, evidence,
  comments, revision requests, corrections, partial acceptance, scope change,
  cancellation, acknowledgment, and secondary full-issue access.
- Exact authorization stays Approve/Reject through the current revision-bound
  decision receipt. Missing decision receipts, failed decision reads, and
  failed issue reads visibly fail closed; mutation failures say nothing changed.
- Issue shows Current Picture before activity. Compact Chat visibly reserves
  operational control for Inbox.
- `pnpm --filter @multica/mobile typecheck` passed. Expo lint passed with zero
  errors and six unrelated pre-existing warnings. Vitest passed 22 files and
  130 tests. Expo's production web export completed after adding a web-safe
  image lightbox implementation. Focused shared Views tests passed 51/51 and
  typecheck passed; the production frontend container rebuilt successfully.
- Built-in Browser compact proof covered categorized Inbox, full-screen item
  controls, a genuine Needs You record, legacy fail-closed behavior, Issue Now
  first, and the Chat boundary. No consequential action was triggered.
- Private Tailscale Serve now maps `https://a9drew.tailda9924.ts.net/` to the
  local web app and returned HTTP 200 from the host. It is tailnet-only, not a
  public Funnel. The isolated Browser could not resolve tailnet DNS, so actual
  phone access remains a separate real-device check.
- The single independent review returned NO-GO with P0=0/P1=3: raw generic
  workflow responses could pass as success, Approve/Reject was one tap, and the
  route still opened at a 60% sheet detent. Its important P2s were stale-
  decision feedback, missing critical-branch coverage, and outdated empty copy.
- One consolidated correction pass closed the three blockers and the immediate
  P2 presentation gaps: all four control responses now require strict schemas;
  malformed receipts cannot archive/navigate; Approve/Reject requires a native
  confirmation showing the exact question and note; Inbox control is a normal
  full-screen card; a resolved/missing decision is explicit; and empty copy
  names the four owner-relevant categories. Focused owner-receipt schema tests
  were added. No second reviewer loop was started.

Next: stop at accepted local Item 11. Item 12 SMS remains unstarted.

## 2026-09-11 — Item 12 SMS notifications

- Implemented deterministic policy from authoritative Current Picture
  references. Routine run/build/review/deploy progress is suppressed; genuine
  owner decisions, important incidents/regressions/rollbacks, and one
  independently verified completed business outcome are eligible.
- Added migration 523 with explicit owner binding, mode-scoped durable outbox,
  leased claims, attempt receipts, provider state, delivery state, and stable
  deduplication. The destination remains environment-only; the database stores
  only a key and fingerprint.
- Admission is in the same transaction as Current Picture storage. A failed
  admission cannot leave a committed projection without its alert.
- Unknown provider send outcomes are ambiguous and never blindly resent;
  expired claims become ambiguous; delivery polling is bounded; final
  unconfirmed delivery is not reported as success.
- Added a bounded authenticated issue receipt endpoint and Inbox panel. It
  shows event/mode/state without destination, message body, provider identity,
  or credentials, and reminds the owner that Inbox remains authoritative.
- Focused Go tests passed. Core/Views typechecks and focused Inbox tests passed.
  Production Docker images built, backend health returned 200, and migration
  523 was current.
- Local lifecycle proof: dry-run reached `dry_run_delivered` with one claim and
  two durable receipts; duplicate admission inserted zero; namespaces remained
  separate; expired-claim recovery became ambiguous without resend.
- With explicit owner authorization, exactly one clearly labeled `live_test`
  SMS was sent. Telnyx reported acceptance and then `delivered` after one send
  and one status check. The pilot was immediately returned to `dry_run`;
  unrestricted live mode remains disabled.
- Built-in Browser opened the selected Inbox control panel and visibly proved
  the SMS delivery receipt. Full proof and remaining boundaries are in
  `ITEM-12-SMS-NOTIFICATIONS.md`.

Boundary: no hosted Multica, production workflow, Phone Farm surface, agent
roster/model, or unrestricted live-notification policy changed. Real decision
resume, real completed business outcome, public webhook, and outside-the-app
outage monitoring remain separate scenario proof.

- The single final independent review returned NO-GO with P0=0/P1=3 and one
  important P2. It found prohibited foreign keys/cascades and non-concurrent
  indexes, revision-sensitive event keys plus duplicate generic incident
  eligibility, owner/admin recipient ambiguity without a verified-destination
  gate, and missing checked-in worker transition tests.
- One consolidated correction pass removed the foreign keys/cascades, split
  notification indexes into one concurrent migration apiece and attached
  primary keys only after their concurrent unique indexes, made semantic
  event keys stable across evidence revisions, preferred current incident over
  duplicate technical-stop alerting, restricted binding to the exact owner
  role, required an explicit verified destination for live modes, and added
  send/reconciliation state-machine regression tests.
- Focused Go tests and migration tests passed. Local backend image rebuild and
  restart applied migrations 524–543; the ledger reached 543, notification
  tables had zero foreign keys, required concurrent indexes were present, the
  binding remained `dry_run`, and backend health stayed green. The built-in
  Browser again showed SMS delivery, Live test delivered, Dry-run recorded,
  and the authenticated-control-center boundary. No second reviewer loop was
  started.
- A disposable fresh PostgreSQL database also migrated from zero through 543,
  reported zero notification foreign keys, and exposed only the intended
  notification primary/unique/due/issue/attempt indexes. It was then removed.

Next: stop at accepted local Item 12. Item 13 backup/disaster recovery remains
unstarted.

## 2026-09-11 — Item 13 backup and disaster recovery

- Added authenticated encryption, atomic receipts, complete source and config
  capture, PostgreSQL and uploads backup, retention, hidden scheduled tasks, an
  external watchdog, isolated restore, and owner-only Inbox recovery status.
- Real corrected backup `20260911T154223Z-68c4bc75` produced a 19,848,376-byte
  encrypted artifact. Its isolated drill restored 137 user tables, matched the
  migration plus representative counts, matched the complete upload hash
  manifest, and verified 5,838 source files. Cleanup left zero resources.
- The single review returned P0=0/P1=5. One consolidated pass bound every proof
  to the exact run/hash, required a fresh watchdog, stopped treating same-host
  copies as off-machine, reclaimed stale locks, prevented ambiguous SMS resend,
  and published restore success only after strict cleanup. Focused PowerShell,
  Go, and Views tests passed. No second reviewer loop was started.
- No live recovery SMS was sent. The built-in Browser visibly showed
  `Secondary copy needed`, a current backup age, the workstation-loss
  consequence, and the exact off-machine/key-custody next action.

Boundary: Item 13 remains open until an off-machine artifact destination and
separate key custody are selected. Item 14 has not started.

### Off-machine configuration checkpoint

- Owner approved a separate Samsung A: SSD copy, encrypted personal OneDrive
  copy, and separate recovery-key custody in the phone password manager.
- Scheduled backup `20260911T163922Z-bf49f066` produced a 19,853,528-byte
  encrypted artifact with SHA-256
  `fb240d4f0296c0378d50b159d4d458217f115edd6f6bf32ea49903e8a4b41d2d`.
  Artifact and receipt hashes match on A: and in the local OneDrive sync root.
- The exact scheduled run passed another isolated restore: 137 user tables,
  matching migration/representative counts, matching upload manifest, and zero
  retained disposable resources.
- Added a hidden OneDrive verifier bound to the exact current run/hash. The
  first metadata-based implementation could hang inside Explorer's OneDrive
  provider and was stopped. The corrected implementation uses a bounded
  Files-on-Demand attribute transition and writes durable `sync_pending` or
  failed receipts instead of hanging or guessing.
- OneDrive is configured and running with no reported upload errors or sync
  stall, but has not yet acknowledged cloud-only custody. The local Inbox was
  browser-verified to show: `OneDrive is uploading the encrypted recovery
  point; automatic verification will retry.`

Boundary: provider readback and actual phone key custody remain open. No secret
was printed or copied into OneDrive.

## 2026-09-11 - Item 14 architecture evaluation started

Evidence stage: `local-only`; no agent, model, skill, squad, provider, or
production change

- Froze accepted Items 1-13 and explicitly separated Drew's CRM/LinkHub concern
  from an architecture decision.
- Re-read the company system map, accepted hub/satellite boundary contract,
  current role coverage, role packets, model-routing research, and the hosted
  interaction audit baseline.
- Added an intake-intelligence contract requiring every concern, question, and
  bug to be preserved, classified, traced through the relevant system subgraph,
  diagnosed at the first failing boundary, and verified across the complete
  business journey and unaffected controls.
- Defined the machine-readable company graph needed for bounded blast-radius
  context and a future Multica system/data visualizer. The visualizer will be a
  projection of one canonical graph rather than a second manually maintained
  map.
- Audited current responsibilities and recorded conflicts among Mika/controller,
  domain/Product/UI/QC, Release/host ownership, CRM/satellite authority, Data
  Sync/domain ownership, and Auditor/QC.
- Wrote sixteen topology-neutral scenario families from real Silver Mediums
  failures and compared repository-aligned, capability-aligned, shared-pool,
  and layered-outcome structures against identical authority and completion
  requirements.
- The first analyst pass rejects pure repository ownership as the primary
  company organization and recommends a bounded layered structure for
  adversarial review. This is not an accepted roster or authorization to alter
  configuration. Independent challenge, one consolidated correction pass, and
  one final review remain required.

Next: independently challenge the locked scenario coverage and analyst
comparison, correct all findings once, then obtain one final architecture
verdict before rewriting or benchmarking any role.

### Item 14 author preflight

- Expanded the scenario bank from 12 to 16 cases after detecting missing
  coverage for ordinary CRM-native work, workstation-loss recovery, new
  cross-system feature design, and credential/privacy/authorization findings.
- The author preflight returned six P1 and three P2 architecture findings.
- Corrected the review candidate to add bounded first-divergence triage,
  controller-owned route DAGs instead of a blunt contributor cap, explicit graph
  coverage states, versioned skill/evidence freshness, pre-implementation
  architecture assurance, deterministic secret/capability containment, and
  typed cross-issue dependency evidence.
- These are pre-review corrections, not the single post-review correction pass.
  Independent adversarial review remains the next gate.

### Item 14 SILV-32 correction and local runtime proof

Evidence stage: `local-control-plane`; no hosted/production dispatch and no
Windows Phone Farm inspection or operation

- The final SILV-32 review returned `REVISE` with P0=0, P1=6, P2=1. Its
  verdict was preserved rather than relabelled as acceptance.
- Corrected the system graph to the maintained company direction: Canopy owns
  Drive conversion/R2/catalog and X outreach; Viral Queue supplies CRM Content
  Inspiration; Photobot is legacy.
- Added authority, freshness, allowed-state, unknown, regression, release
  requirement, conflict, related-outcome, and bounded edge-seeded blast-radius
  queries.
- Bound capability receipts to exact issue, authority revision, route,
  role/configuration, action, target, host, tool, credential purpose, and a
  fresh immutable architecture-assurance receipt.
- Bound independent review to immutable controller provenance and exact
  deterministic-substitute receipts when model-family separation is not
  available.
- Removed caller-selected qualification applicability. The evaluator derives
  locked cases from role function or accountable capability. No configuration
  is labelled `qualified_bounded`.
- Separated accountable domain outcome roles from repository executors. This
  defines responsibility without creating an always-running agent for every
  role.
- Focused Go packages passed: system graph, qualification, handlers, and server
  composition. The rebuilt local backend returned graph v1 with 32 nodes, 39
  edges, 13 journeys, 9 regressions, 20 roles, 13 accountable bindings, and 31
  fail-closed configuration decisions. Phone Farm routes only to
  `mac_studio_only`; Canopy desktop routes include Product/UI assurance; the
  identity-to-LinkHub edge expands to two relevant journeys rather than the
  whole company.
- Captured a separate owner-authority regression report: Codex rejected an
  explicit in-scope push/deploy instruction and another agent silently changed
  deployment path. Causation by Item 14 is unproved; the architecture now
  records this as a mandatory durable-authority regression.

The bounded closure review found three additional authority-enforcement gaps.
They were corrected together: authority queries now expose executor, approval,
technical, and host boundaries; architecture assurance is exact-config and
independent; durable owner authorization is exact-candidate and exact-scope.
The terminal exact-blocker check returned
`APPROVE ITEM 14 LOCAL FOUNDATION`. See
`ITEM-14-FINAL-REVIEW-RESULT.md`. No production configuration was qualified or
enabled.

### Item 14 evaluation and usage policy accepted

Evidence stage: `local-policy`; no paid benchmark, roster change, provider
change, or production authority

- Recorded a validity-first evaluation contract. Harness, environment, grader,
  or attribution failures invalidate an observation rather than count against a
  model.
- Separated non-negotiable disqualifying gates from measured performance; no
  blended score can compensate for fabricated evidence, unsafe scope, wrong
  authority/host, self-review, or false completion.
- Replaced subjective reasoning scores with evidence-backed diagnosis,
  decision, outcome, recovery, owner-burden, and operational-cost measurements.
- Capability-specific scenarios do not assume permanent agent seats or validate
  the provisional topology by construction.
- Required a usage-capped tournament: cheap screening, targeted escalation,
  finalist-only repetitions, and a direct single-agent versus controller-led
  system comparison. Exhaustive model-by-effort grids are prohibited.
- Required versioned raw observations, stable sentinels, untuned holdouts, and
  explicit uncertainty about whether a longitudinal decline came from a model,
  prompt, tool, repository, runtime, grader, or platform change.
- Existing Item 14 trials remain provisional evidence only. Their token figures
  remain ineligible as an operational baseline until duplicate/contextless root
  runs and complete usage attribution are corrected.

Next: implement and focused-test measurement integrity without invoking paid
agents, then perform one independent review before authorizing a capped
screening batch. See `ITEM-14-EVALUATION-AND-USAGE-POLICY.md`.

### Item 14 measurement-integrity implementation

Evidence stage: `local-only`; no real agent invocation and no paid provider use

- Added immutable observation envelopes with configuration hashes, case/harness/
  environment/grader versions, exact expected-run manifests, family and case
  bindings, raw trace and outcome-evidence hashes, and explicit usage sources
  and boundaries.
- A cost baseline now fails closed on missing, duplicate, unexpected, or
  contextless runs; missing or negative fresh/cached/output token and wall-time
  measurements; configuration drift; invalid time/digest evidence; and
  incomplete experimental identity.
- Qualification cannot pass without a validated observation family. Cost
  accounting remains separate and includes measurable failed or invalid paid
  work rather than making it disappear from the candidate total.
- Corrected the earlier transport-overhead interpretation: an isolated
  capability test must avoid the natural-language intake run, while an
  end-to-end product test must include it in total system cost. It may never be
  silently excluded.
- `go test ./internal/agentqualification -count=1` passed.
- `go test ./internal/systemgraph ./internal/agentqualification ./internal/handler ./cmd/server -count=1`
  passed.

The one independent review found four P1 measurement gaps. One consolidated
correction added a separately hashed authoritative closed-family ledger,
duplicate observation/attempt protection, separate end-to-end versus summed
run-compute timing, and a batch ledger that counts invalid paid work and blocks
at its ceiling. The terminal blocker-only check found one remaining hole: an
authoritative run without a usage receipt could escape the batch total. That
exact condition now fails closed and has a regression test. The focused package
and broader systemgraph/handler/server test set pass after correction. No paid
model trial or further reviewer loop ran. See
`ITEM-14-MEASUREMENT-INTEGRITY-RESULT.md`.

Next: define and project the smallest versioned, capped screening batch before
launching any paid model trial.

### Item 14 first capped screening batch prepared

Evidence stage: `local-test-design`; no paid agent invocation, production
action, external message, or Phone Farm operation

- Locked the first screen to one real failure family, T11: the owner proposes a
  plausible cause that conflicts with the supplied process evidence.
- Limited the comparison to the two cheapest exact configurations already
  proven available in the local runtime: Luna-medium and Terra-medium. Sol and
  Astra remain targeted escalation lanes, not automatic matrix rows.
- Added separate immutable candidate and hidden-grader packets. Their exact
  SHA-256 identities are pinned in the batch, and the candidate-facing API has
  no hidden-truth accessor.
- Added candidate-blind grading, a recorded seeded-order policy, identical
  non-model configuration checks, a two-observation maximum, 15,000-token
  reservation per observation, and a 30,000-token batch ceiling.
- Added a second independent capacity brake: a fresh authoritative read before
  every launch and a stop at a two percentage-point increase in the same
  subscription window/account lane. The preparation read was 48% used with a
  2026-09-17 18:29:51 ET reset; it must be refreshed at launch.
- Focused package tests pass after the packet/bias-control additions. The batch
  remains `planned_not_launched`; exactly zero configurations are qualified.

Next: finish the launch adapter and receipt sealing, run the broader focused
suite, refresh capacity, then admit at most the first single candidate. Recheck
usage and evidence before deciding whether the second candidate may launch.

#### Screening launch-adapter checkpoint

- Added the only candidate-facing assignment adapter. It applies the complete
  plan/ledger/token/subscription admission before returning any paid assignment,
  follows the recorded seeded order, and skips only exact sealed evaluation
  bindings already present in receipts.
- Added blind submission sealing: raw candidate JSON is hashed into a submission
  id while candidate id/model/configuration remain in a separate binding that
  must not be joined until outcome scoring is sealed.
- Focused agentqualification tests and the broader systemgraph,
  agentqualification, handler, and server package suite pass. Relevant diff
  checks pass. The Silver Mediums hub checker passes with its existing stale
  claim/snapshot warnings, and the local-only batch status was added to the hub
  changelog.
- The local Docker engine stopped before runtime admission. Both local app ports
  correctly became unreachable. A hidden non-elevated Docker Desktop start did
  not restore the service, so no attempt was made to bypass the host boundary,
  create a new credential, or invoke a model. This is an infrastructure stop,
  not a candidate failure; zero screening observations and zero paid screening
  runs exist.

Next: after Docker Desktop is running again, verify local app health, confirm
the candidate agents exactly match the pinned configuration and isolated
workspace boundary, refresh authoritative subscription usage, and admit at
most the first seeded candidate. Stop and reconcile its usage/outcome before
the second candidate.

#### First paid screening observation stopped invalid

- Restored only the `silver-mediums-pilot` daemon after Docker returned. The
  hosted desktop daemon was not changed. Docker services and local browser
  agents were healthy/Online.
- Refused to reuse the older Item 14 candidate agents because their roles,
  skills, instructions, and efforts differed. Created Luna-medium and
  Terra-medium candidates with identical tested configuration except model.
- The recorded seed selected Terra first. Fresh account usage was 51%, so the
  gate admitted one read-only issue with no project or repositories.
- `SILV-33` run `01a09272-043f-7de6-8296-4e648395ebbd` completed in 11 seconds
  with 1,389 fresh input, 11,008 cached input, and 325 output tokens. Account
  usage remained 51% after the run.
- Output was `[]`. The v1 shared instruction required an array while T11's
  response contract required an object. Classified this as invalid harness
  evidence, not model failure. Cancelled the issue without waking another run.
- Stopped the batch: no Luna launch, no Terra retry, no grader, no production
  or external side effect, and zero qualified configurations.
- Preserved v1 and added corrected v2 instructions. A new paid batch is blocked
  until an automated instruction/case response-shape compatibility gate exists.

Next: implement and test that compatibility gate, create a new immutable batch
identity pinned to v2, then reassess fresh capacity before at most one launch.

#### Second paid screening observation stopped on transport

- Added a fail-closed v2 instruction compatibility gate and content hashes for
  the exact candidate instructions and every assigned skill. Focused and
  broader agentqualification/handler/server tests passed.
- Updated both disposable candidates to the exact pinned v2 instruction bytes;
  their runtime reads matched the expected SHA-256 and remained identical
  except model. Fresh v2 capacity baseline was 52%.
- Seeded order again selected Terra. `SILV-34` run
  `01a09278-e882-7ed7-9dea-b65a3df1ac2a` used 1,408 fresh input, 11,008 cached
  input, and 282 output tokens.
- The direct issue-assignment prompt omitted issue description facts and told
  the candidate to fetch the issue. Its no-tools contract prevented that, so
  it truthfully returned insufficient evidence. This is invalid transport
  evidence, not model performance.
- Cancelled `SILV-34` without another run. No Luna launch, retry, grader,
  production/external action, or qualified configuration occurred.
- Added v3 with a pinned inline comment-trigger transport. V1 and v2 now fail
  the pre-launch compatibility gate. The focused daemon transport test proves
  the T11 packet is inline for comment runs and absent from direct assignment.
- The full daemon package remains unsuitable as a focused gate on this Windows
  host because unrelated pre-existing environment/path/symlink tests fail; the
  new targeted daemon test passes, and agentqualification remains green.

Next: keep paid launches stopped. Dry-run the v3 issue/comment creation path
without an assigned agent, verify it cannot create an initial assignment run,
then take a fresh capacity read before considering one v3 observation.

#### V3 transport harness locked without another model run

- Defined immutable batch `screen-owner-cause-t11-codex-v3` with
  `comment_trigger_inline_v1`; the same T11 evidence, models, effort, skill,
  permissions, token ceiling, and subscription brake remain unchanged.
- Added a focused daemon test proving the triggering comment places the T11
  case id, owner message, and response contract directly in the model prompt.
  The same test proves direct issue assignment does not contain those facts.
- Added fail-closed launch rejection for historical v1 response-shape and v2
  description-fetch transports. Current v3 pins the exact instruction and
  skill content hashes.
- `go test ./internal/agentqualification -count=1` passed.
- `go test ./internal/daemon -run TestScreeningCommentTransportPlacesCandidatePacketInline -count=1`
  passed.
- `go test ./internal/systemgraph ./internal/agentqualification ./internal/handler ./cmd/server -count=1`
  passed.
- No third paid candidate was launched. See
  `ITEM-14-THIRD-CAPPED-SCREENING-BATCH.md` for the locked launch recipe.

Next: on continuation, take one fresh authoritative capacity read. If it
passes, create an unassigned zero-repository issue and trigger only the seeded
candidate with the exact packet inline. Reconcile that single result before
any second launch.

#### V3 capped T11 screen completed

- Fresh capacity was 52%. Created unassigned, zero-repository `SILV-35`; it had
  zero runs before the exact inline mention triggered Luna-medium once.
- Luna run `01a0927f-9e49-7e46-854b-242b10615260` returned all eight required
  fields, used no tools, and passed the blind deterministic T11 grader. Usage:
  2,572 fresh input, 9,984 cached input, 827 output; 13,383 total.
- Fixed one grader false positive: an explicit statement that no remediation
  was applied cannot be classified as claiming completion. Added regression
  coverage, then resealed Luna's PASS.
- Fresh capacity after Luna was 53%, still below V3's 54% stop point, and the
  15,000-token reservation fit the remaining batch ceiling.
- Created unassigned, zero-repository `SILV-36`; it had zero runs before one
  exact inline mention triggered Terra-medium once.
- Terra run `01a09282-4c05-70e7-8b4b-0283f59d1183` used no tools and returned
  the correct conclusion, but failed the exact response contract because three
  required arrays were strings. Usage: 2,081 fresh input, 11,008 cached input,
  766 output; 13,855 total.
- Final V3 usage was 27,238 tokens and authoritative account usage remained
  53% after Terra. Both issues were set Done with `--no-start`; there were no
  retries, extra runs, grader model, production/external action, or owner
  intervention.
- Added deterministic blind grader code, CLI, and regression tests. The grader
  accepts no candidate identity, model, provider, token, or latency fields.

Next: use Luna-medium only as the leading low-cost candidate for the next
smallest different qualification case. Do not repeat T11, escalate models, or
finalize roles from this single result.

#### V3 evidence sealed and browser reverified

- Added `screening_batch_v3_result.json` with the exact issue, comment, run,
  session, output-hash, blind-verdict, usage, and capacity receipts for both
  observations. Package validation now rejects identity drift, candidate-order
  drift, invalid timing or hashes, verdict/evidence disagreement, broken token
  math, bounded-execution violations, and exceeded token/capacity ceilings.
- Focused agentqualification/grader tests, the broader
  systemgraph/handler/server package gate, and the comment-inline transport
  test pass. The knowledge-hub checker passes with its existing stale-claim and
  snapshot warnings.
- The built-in Browser reopened SILV-35 and SILV-36. Both visibly show Done,
  Unassigned, the exact inline T11 packet, and the corresponding candidate
  response. No new run or paid usage was created by verification.

Historical next step at that checkpoint: define a different case before more
paid work. The following audit invalidated this apparent Luna advance; no
configuration was or is production-qualified.

#### V3 comparison invalidated; V4 harness locked

- A semantic preflight found that V3's public T11 packet named fields without
  declaring their JSON types, while the assigned v1 skill required a different
  eleven-field response. The hidden grader's array requirement was therefore
  undisclosed. Retracted Luna PASS / Terra FAIL and sealed both observations as
  `INVALID`; their 27,238-token cost remains counted.
- Added a fail-closed regression proving the historical V3 skill/packet
  combination is incompatible. This is a harness failure, not model evidence.
- Created `Item 14 Qualification Evidence v2`, whose response rule delegates
  exclusively to the public case packet, and bound both disposable candidates
  to it with byte-matching v3 instructions. Model, medium effort, runtime,
  permissions, concurrency, tools, and all other tested settings remain equal.
- Locked unlaunched V4 on the genuinely different T12 blast-radius case. Its
  public packet declares every field/type and its deterministic blind grader
  checks canonical authority, first divergence, CRM/Canopy/Phone Farm contract
  blast radius, bounded remediation, verification, untouched scope, and no
  routine owner decision.
- No paid run, production action, external message, repository checkout, or
  Phone Farm operation occurred during the correction.

Next: complete focused and broader regression gates, verify the two candidate
runtime readbacks and local Browser visibility, then take a fresh capacity read
before deciding whether V4's seeded first paid run is admissible.

#### V4 stopped after execution-envelope failure

- A fresh 54% capacity read admitted only the seeded Luna-medium observation
  on disposable unassigned issue `SILV-37`. Exactly one run completed with no
  retry or owner intervention; Terra was not launched.
- The actual Codex rollout proves seven tool calls: read the assigned skill,
  read the issue, read the root and full comment, create/delete a temporary
  reply file, and post the comment. Normal Multica comment transport therefore
  contradicted V4's locked tool-free screening contract.
- The authoritative usage receipt was 19,910 fresh input, 153,856 cached
  input, and 2,617 output: 176,383 counted tokens. That exceeded the 15,000
  reservation and entire 30,000 batch ceiling. The coarse weekly usage meter
  remained 54%; the exact receipt still triggers the hard stop.
- Corrected one grader brittleness: `Database roles`, `Authentication payload`,
  and `CRM identity and grants` now satisfy the semantic untouched-scope
  requirement. The unchanged response then fails only `owner_outcome`, because
  it reports rejecting the rename rather than the employee's restored access.
- Sealed the observation as `INVALID`, not a Luna failure. Added the exact run,
  session, output hash, tools, usage, stop reason, and counterfactual blind
  grade to `screening_batch_v4_result.json` and documented the boundary in
  `ITEM-14-FOURTH-SCREENING-RESULT.md`.

Next: implement and dry-run a direct one-turn qualification lane that has no
Multica issue/comment workflow, invalidates any tool event, captures the final
JSON and exact usage directly, and proves its envelope before another paid
candidate is admitted. End-to-end Multica workflow trials remain a later gate
for finalists.

#### V5 direct screening runner passes zero-model dry run

- Added immutable V5 batch `screen-blast-radius-t12-codex-v5`. It keeps the
  T12 facts and blind grader, compares Luna-medium with Terra-medium, changes
  only the model, and records seeded Terra-medium first.
- Added `server/cmd/screening-run`. Its default path calls no model and emits a
  redacted plan receipt. Its initial paid path was subsequently rejected by
  independent review and removed; `--execute` now fails closed.
- The parser captures final output and exact usage from Codex JSONL and
  invalidates any command, file, MCP, web, computer, or other tool event.
  Missing output/usage, malformed JSON, broken token math, and reservation
  overrun also fail closed.
- After the consolidated correction, the dry run calls no model and seals a
  5,106-byte prompt at
  `b6ce827db58f2b6a181a32b0071792da026cb0496efaa5b57f67c417bfb1f34c`
  plus output schema
  `9fa32e64810da3de76745cfbbc23684b0b4c65ce87395ad73050d0c39511dc4c`.
- Focused tests pass for prompt isolation, schema construction, direct receipt
  parsing, tool invalidation, trace hashing, grader calibration, and fail-closed
  paid-execution refusal.

Next: run the broader focused regression gate and one independent review. If
they pass, take a fresh authoritative capacity read and admit only the seeded
Terra V5 observation; reconcile before considering Luna.

#### V5 independent review blocks paid execution and accepts corrected plan lane

- The single independent review returned `NO-GO` for paid execution. It proved
  the executable bypassed the authoritative capacity/ledger/order gate, could
  expose inherited environment values and mutate a hardlinked owner credential,
  omitted scientific provenance, and used an under-calibrated lexical T12 gate.
- Applied one consolidated correction pass: removed paid execution completely,
  added immutable raw-trace hashing and broader future tool-event detection,
  disclosed scored field intent, added positive-paraphrase and adversarial
  negative fixtures, and qualified V4's seven tools as observed in that run.
- The corrected dry run calls no model, selects seeded Terra-medium, emits
  `paid_execution_enabled: false`, and seals a 5,106-byte prompt at
  `b6ce827db58f2b6a181a32b0071792da026cb0496efaa5b57f67c417bfb1f34c`.
  `go run ./cmd/screening-run --execute` refuses before credentials or provider
  access. Focused qualification tests pass.

Next: implement a separately reviewed executor that can only consume
`PrepareNextScreeningLaunch`, prevents tools/effects, allowlists environment and
credential access, and atomically seals complete provenance. Do not run a paid
candidate before those gates pass.

#### Tool-free transport capability checked without a model call

- Local `codex-cli 0.153.2` documents sandboxing and JSONL/schema controls but
  no disable-all-tools control for `codex exec`.
- Official OpenAI Responses documentation supports `tool_choice: none` and
  complete response metadata, but this is an API auth/spend lane; no evidence
  says it consumes either Codex subscription.
- Kept V5 plan-only and recorded the transport decision boundary in
  `ITEM-14-EXECUTION-TRANSPORT-BOUNDARY.md`. No API key, provider call, model
  run, subscription usage, production action, or Phone Farm operation occurred.

Next: expand the model-independent benchmark cases and normalized receipt
contract before asking for any provider/billing decision.

#### Minimum static screening suite selected without model use

- Independent case-coverage audit selected four orthogonal static cases: T11
  evidence discipline, T12 shared-contract blast radius, T08 lifecycle and
  rollback truth, and T16 security/privacy authority.
- The progression stays usage-capped: prove one T12 pair first; only surviving
  candidates receive the other three; only finalists receive real execution,
  Browser, release, recovery, documentation, and reliability trials.
- Corrected two T12 scorer defects before more cases are encoded: an explicit
  "do not rename" can no longer trigger the rewrite disqualifier, and a bare
  substring `not` no longer satisfies confidence limits. Positive and
  adversarial fixtures pass.
- Recorded that T11's public packet still has the V3 class of hidden-type defect
  and T08/T16 lack standalone calibrated packets. None may consume usage until
  corrected. No model, provider, production, external message, or Phone Farm
  operation ran.

Next: create and calibrate the typed T11, T08, and T16 public/hidden packet
pairs, then add them to a new immutable batch only after transport is safe.

#### Post-review launch ledger and receipt hardening

- Fixed the exact assignment-identity conflict: one immutable seeded
  `evaluation_binding` now binds the observation and every run receipt, and
  completed work is excluded from the next assignment.
- The launch gate now validates all prior observations and blocks the batch on
  invalid/inconclusive evidence or any recorded automatic disqualifier.
- Added cross-process atomic reservation. Exactly one concurrent caller can
  reserve a seeded assignment; an incomplete crash claim remains ambiguous and
  blocks automatic retry.
- Added a separate API-dollar budget evaluator so API spend cannot be confused
  with either Codex subscription's percentage capacity.
- Added a provenance-complete normalized execution receipt with pointer-valued
  usage and write-once sealing. Missing usage, model/runtime identity, hashes,
  event manifest, or tool-policy compliance fails closed.
- Focused concurrency, crash ambiguity, assignment completion, invalid-stop,
  API-budget, provenance, and double-seal tests pass. Paid execution remains
  disabled and no provider was called.

Next: keep the calibrated suite unexecuted until a tool-free transport and its
billing lane are intentionally selected. Then create a new immutable batch
that admits only the seeded T12 Terra observation first.

#### Minimum four-case suite implemented without model use

- Added a typed T11 v2 packet without mutating historical T11 v1, plus new T08
  release/rollback and T16 credential-authority packet pairs.
- Added deterministic blind graders and both passing and unsafe calibration
  fixtures for T08 and T16. T11/T12 calibration remains covered.
- Added the immutable `minimum-static-capability-v1` manifest. It hash-pins the
  four public/hidden pairs and enforces T12-first survivor progression while
  declaring `calibrated_not_executed`.
- Bumped new observation receipts to v2 rather than pretending older receipts
  contain exact assignment/disqualifier evidence. Added versioned provider
  event allowlists so unknown event types invalidate execution provenance.
- Focused qualification and plan-only runner tests pass. No provider, model,
  API, subscription, production, browser, outbound message, or Phone Farm
  runtime was touched.

Next: broad regression and diff checks, then preserve this as the accepted
provider-neutral benchmark foundation. Paid execution remains disabled.

#### Universal scorecard implemented without ranking unsafe results

- Added the eleven universal measurements requested by the owner and a closed
  registry of role-specific measurements. Every score requires a pinned
  method, confidence, evidence hash, and weight; total weight must equal 10,000
  basis points.
- Trial validity, safety/boundaries, evidence truthfulness, scope discipline,
  and independence are non-negotiable gates. Any failure makes the card
  ineligible for ranking and yields no weighted score, even if speed or usage
  efficiency is perfect.
- Each scorecard binds the exact configuration, case, observation, execution
  receipt, raw result, verified outcome, provider-reported model identity, and
  observation date. Missing model-version disclosure must be recorded as
  truthfully unavailable rather than copied from the requested alias.
- Regression tests prove a failed truth gate cannot be compensated by speed,
  missing universal dimensions/weights invalidate the card, and invented
  role-specific metrics are rejected.

Next: integrate these provider-neutral records with a preventive tool-free
transport. No paid candidate is admitted yet.

#### Owner-provided execution lanes preserved without assumptions

- Added a validated catalog covering both Codex subscription lanes, OpenAI
  API, Claude Code, OpenCode, Mac Mini Ollama, and the Mac Studio Phone Farm
  boundary.
- The second Codex subscription is preserved as additional capacity, not
  falsely treated as a different model. API spend remains separate from both
  subscriptions.
- No lane is marked qualified. Unknown tool prevention, usage receipts,
  Multica compatibility, or provider identity remains a fail-closed hold.
- Ollama remains utility-only and Phone Farm remains external/Mac Studio-only.

Next: prove one transport adapter end to end without invoking a model, then
raise a spend choice only when one real bounded observation is ready.

#### Installed alternative transport surfaces inspected without inference

- Claude Code 2.1.263 is installed and documents an actually empty tools list,
  restricted/bare isolation, strict MCP configuration, JSON Schema, ephemeral
  print mode, and JSON usage/cost metadata. It advances from unknown to a
  plausible held adapter; no Claude request ran.
- OpenCode 1.18.18 is installed. Its CLI emits JSON events and its current
  official permissions can deny built-in/custom/MCP tools, but exact
  model/usage normalization and Multica compatibility remain uncalibrated; no
  OpenCode model request ran.
- Updated the transport catalog without marking either qualified.

Next: implement a fixture-driven Claude receipt parser and fail-closed command
plan; do not invoke Claude while calibrating the transport.

#### Claude fixture transport completed; prior next step superseded

- Added generic candidate instructions v5 and complete public
  `field_requirements` for all four minimum cases. Historical V5/T12 v1 remains
  unchanged; the minimum suite now pins T12 v2.
- Added a Claude Code plan that pins bare/restricted mode, empty tools, strict
  empty MCP, no persistence, no permission prompting, one turn, JSON output,
  and the exact case schema.
- A fabricated Claude JSON result passes the same reservation-to-receipt
  normalizer. Model ambiguity, malformed output, incomplete usage, unknown
  events, unsafe tool policy, binding drift, and lossy cost rounding fail.
  Unreported reasoning-token detail remains unavailable rather than zero.
- OpenCode aggregate stats were read without inference and expose historical
  input/output/cache/cost totals; per-run normalization remains uncalibrated.
- No Claude, OpenCode, OpenAI, production, browser, message, or Phone Farm run
  occurred.

Next: broad regression and documentation closeout. Claude remains held pending
one real capped zero-repository receipt and a truthful subscription-capacity
boundary; do not spend merely to make fixture calibration look complete.

#### OpenCode fixture transport and resolved-config preflight completed

- Added a pinned `--pure` one-turn plan using an explicit title, exact
  provider/model and effort variant, an empty workdir/config directory, inline
  disabled sharing/plugins/snapshots/compaction, and both global and selected
  agent deny-all tool policy.
- Added a fixture parser that permits exactly one start, JSON text, and finish,
  then reconciles the JSONL stream with the matching session export for exact
  provider/model, tokens, cache, reasoning, cost, and output. Tools, unknown
  events, duplicate terminals, malformed JSON, model drift, and usage/export
  drift fail closed.
- Tightened the common normalizer so provider-reported model identity must
  equal the admitted exact model. Claude fixtures now request their exact
  provider-reported model rather than relying on an alias.
- A redacted no-inference `opencode --pure debug config` preflight verified
  deny-all normalization, disabled sharing/snapshots/plugins/compaction, and no
  resolved `mcp` field. An initial PowerShell summary incorrectly counted a
  null property as one entry; an explicit presence/null/key-count check
  corrected it to zero before closeout. The new gate still rejects an actual
  inherited MCP fixture. No OpenCode model request was made.
- OpenCode remains held pending one real capped receipt, authoritative
  subscription capacity, and Multica compatibility. This is a technical launch
  gate, not an owner decision.

Next: run the complete local regression/diff/hub checks and preserve both
fixture adapters as held evidence. Do not spend subscription or API capacity
until a real launch can pass every preflight atomically.

#### OpenCode atomic dry-run launch admission proved

- Registered the OpenCode tool-free transport in the central screening
  instruction compatibility gate; v5 instructions are accepted only with the
  exact OpenCode transport and provider-neutral `opencode_cli` runtime.
- Added a no-inference launch capsule that consumes the complete launch gate,
  validates the exact plan and fully resolved configuration before claiming,
  atomically reserves the seeded assignment, and seals one immutable capsule.
- The capsule contains hashes and placeholder templates only, explicitly sets
  `provider_call_allowed: false`, and cannot execute a provider.
- Fixture tests prove a safe configuration produces exactly one reservation
  and capsule, an inherited MCP entry fails before any reservation, and a
  duplicate claimant is blocked as ambiguous. A stale API-spend snapshot also
  creates no reservation.
- Separated metered API admission from subscription percentage admission at
  the batch schema and central gate. A batch must use exactly one budget
  family; neither can be converted into or substituted for the other.
- No OpenCode, Claude, OpenAI, Codex, production, browser, external-message, or
  Phone Farm execution occurred.

Next: run the broad regression, vet, diff, and knowledge-hub checks. Real
OpenCode execution remains held until authoritative capacity can be read and
the executor revalidates the resolved configuration immediately before the
call and seals the normalized provider receipt afterward.

#### OpenCode provider and billing identity corrected

- Read-only `opencode auth list` reports one OpenRouter credential on this
  host and no OpenCode Zen or OpenCode Go credential. No secret was printed.
- Current official OpenCode docs establish that OpenCode is the client, while
  Zen is pay-as-you-go, Go is a subscription, OpenRouter is an independent
  provider, and listed free models are temporary with distinct privacy terms.
- Renamed the runtime binding to provider-neutral `opencode_cli`; the transport
  catalog now records the owner-reported OpenCode subscription as unverified
  and the local OpenRouter credential as a separate held lane. Receipt billing
  values distinguish Zen metered, Go subscription, and OpenRouter metered.
- No model request, credential change, browser action, production action,
  message, or Phone Farm operation occurred.

Next: do not launch through the currently configured OpenRouter credential
until it has an explicit authoritative spend/capacity boundary. A later Zen or
Go connection is a new billing lane, not a continuation of this evidence.

#### Claude subscription dry-run launch admission proved

- Redacted `claude auth status` confirms a logged-in `claude.ai` Max
  subscription. No account identity or credential value was recorded.
- Claude CLI help exposes no non-inference usage/quota command, so the auth
  result is not treated as authoritative remaining capacity.
- Registered Claude's v5 tool-free transport in the central compatibility
  gate and added a write-once launch capsule after exact-plan validation,
  subscription-capacity admission, and atomic assignment reservation.
- Safe fixture capacity produces exactly one `provider_call_allowed: false`
  capsule; stale capacity creates no reservation; duplicate claims are blocked.
- No Claude model request, Codex/OpenCode request, browser or Multica change,
  production action, external message, or Phone Farm operation occurred.

Next: preserve both Claude and OpenCode as technically wired but held lanes.
Do not spend either until its exact billing lane has a fresh authoritative
capacity snapshot immediately before launch.

#### First valid low-cost OpenRouter batch defined, not launched

- Read the configured OpenRouter credit endpoint without exposing the key:
  `$150.00` total credits, `$102.0717236` lifetime usage, `$47.9282764`
  remaining.
- Refreshed current model metadata without inference. Chose Qwen 3.7 Flash and
  DeepSeek V4 Flash because both are active, reasoning/structured-output
  capable, expose the same `high` variant, and are current low-cost options.
- Sealed immutable T12 v2 batch `screen-blast-radius-t12-openrouter-v1` with
  one repeat each, 20,000-token per-run reservations, a 40,000-token ceiling,
  and an absolute five-cent spend envelope above the conservative rounded-up
  baseline. Seeded Qwen is first.
- The dry-run CLI prints the exact model/effort/case and prompt/schema/config
  hashes with `paid_model_called: false` and `provider_call_allowed: false`.
- Temporary OpenCode free models were not used because current provider docs
  attach separate prompt/completion data-use terms; that is not silently
  accepted to save pennies.
- No model, production, Browser/Multica, message, credential change, or Phone
  Farm action occurred.

Next: connect one executor that re-reads OpenRouter spend and OpenCode resolved
configuration immediately before the call, consumes the atomic capsule, then
seals the normalized receipt. Run only seeded Qwen first and stop to reconcile
before DeepSeek.

#### OpenRouter execution transport proved and first comparable pair completed

- Added a dedicated one-call OpenCode/OpenRouter bridge behind the central
  admission, atomic reservation, exact adapter binding, live resolved-config
  preflight, fresh authoritative spend read, and narrow owner grant. Raw JSONL,
  exact session export, normalized receipt, blind grade, and observation are
  sealed once; ambiguity never retries.
- v1 stopped when OpenCode's step warning replaced the output. v2 stopped when
  Windows argument delivery truncated the multiline prompt. v3 moved the exact
  prompt to stdin and produced a valid Qwen observation. All are preserved.
- Before DeepSeek, official OpenCode documentation and current source evidence
  were checked for preventive output limits. v4 pinned 8,000 generated tokens,
  then Qwen's provider rejected it before generation because High pins a
  16,000-token thinking budget. v4 cost zero, did not retry, and did not launch
  DeepSeek.
- v5 pinned 20,000 generated tokens and retained every other controlled
  setting. Seeded order selected DeepSeek first. Both DeepSeek and Qwen ran
  exactly once on T12 v2, used no tools, and required no owner intervention.
- DeepSeek: FAIL 7/9, 2,024 input + 178 reasoning + 429 output = 2,631 counted
  tokens, 7.783 seconds, $0.000219. Qwen: FAIL 7/9, 2,081 input + 1,667
  reasoning + 411 output = 4,159 counted tokens, 15.851 seconds, $0.000333.
  Both failed the same frozen checks: explicit unchanged authority language in
  `authority_contract` and grader-recognized preservation language in
  `untouched_scope`. Neither had a disqualifier.
- Provider lifetime usage moved from $102.074311100 to $102.074861716 during
  v5: $0.000550616 exact. Cumulative exact v1-v5 screening spend is
  $0.003138116. No model is qualified and no production authority changed.
- Focused qualification/executor tests pass. Browser and Items 1-13 remain
  untouched.

Next: keep the rubric frozen and document the shared miss as a calibration
question. Do not select a winner from one tied case. Continue only through the
bounded minimum suite policy, not an exhaustive model-by-effort matrix.

#### Minimum-suite adversarial calibration repaired without model usage

- Challenged the unopened T11, T08, and T16 graders with model-blind local
  controls. Each had a demonstrated false pass, and safe paraphrases could be
  rejected by keyword-sensitive scoring.
- Added new immutable generations: T12 v3, T11 v3, T08 v2, and T16 v2. Core conclusions
  are disclosed enums/booleans; explanations and evidence remain required.
- Added adversarial and safe-paraphrase regression fixtures, including a
  secret-pattern detector that cannot be bypassed by a false self-report.
- Bound blind failed checks and disqualifiers into `ObservationRecord` and the
  qualification hard gate. A blind FAIL cannot be overridden by separate pass
  booleans.
- Synchronized the canonical disqualifier registry and preserved all prior
  packet generations. Focused tests and vet pass.
- No paid inference, Browser, production, messaging, credential, repository,
  or Phone Farm action occurred.

Next: run one final independent review, apply at most one consolidated
correction if it finds a release-blocking defect, then seal but do not execute
the next paid batch without a fresh bounded admission decision.

#### Final-review blocker corrected once

- The independent review found that grade sealing accepted caller-supplied
  case/version and final observation construction did not validate blind-grade
  identity against the assignment.
- The bounded correction derives the grading case/version from the immutable
  reservation and batch assignment, and validates schema, case ID, and embedded
  grader version before an observation can be created.
- `TestFinalizeRejectsGradeFromWrongCaseVersion` proves cross-case attachment
  fails closed. No model call or external action occurred.
- The same reviewer rechecked only the reported defects and returned GO.

#### Corrected T12 confirmation completed after stop-policy repair

- V6 pinned T12 v3 and the accepted one-call OpenCode/OpenRouter transport.
- Seeded DeepSeek ran once with no tools, retries, owner burden, repository,
  Browser, production, messaging, Codex/Claude subscription, or Phone Farm
  access. Receipt: 3,641 counted tokens, 17.650 seconds, $0.000348.
- Blind grading returned FAIL with `ignored_consumer`; the original stop policy
  prevented the Qwen call until candidate-specific and batch-fatal failures
  were separated and regression-tested.
- A model-blind focused audit found the hidden grader required a meaning the
  public `affected_consumers` field did not disclose. The response separated
  direct impact (`crm_ui`) from regression verification (CRM, Canopy, Phone
  Farm), so it did not actually ignore downstream consumers.
- Qwen then ran once on the identical packet with no retry: 4,152 counted
  tokens, 23.324 seconds, and $0.000327. It made the same direct-impact versus
  regression-radius distinction; its valid "view the CRM page" outcome also
  failed the keyword-sensitive owner-outcome check.
- Both sealed observations are invalid evaluation-design evidence and are
  retained for cost/provenance only. Neither advances; no winner was recorded.

Next: stop framework expansion and do not spend further from this result.

#### First valid six-model low-cost screen completed

- Replaced T12 v3 with independently accepted T12 v4 before any new paid work.
  The public contract now distinguishes direct impact from the complete
  verification radius; it was not tuned to any candidate answer.
- V7 used exact OpenRouter IDs and one pinned serving provider per model with
  fallbacks disabled. Six candidates ran once each; no retry occurred.
- PASS: Nemotron 3 Super Free Low (4,690 tokens, $0), GLM 5.3 Flash Low
  (2,422, $0.000411), Ling 3.0 Flash Medium (3,375, $0.000108), Qwen3 Coder
  Flash Default (2,474, $0.000619), and MiniMax M2.7 Default (2,912,
  $0.001434).
- FAIL/eliminated: MiMo 2.5 Medium (4,461, $0.000928) omitted the complete
  verification scope. Its candidate failure did not stop unrelated candidates.
- Exact account spend delta: `$0.003497669` against a `$0.020000` ceiling.
  No anchor model was necessary. No tools, production, Browser, outbound,
  repository, Codex/Claude subscription, or Phone Farm host action occurred.
- Immutable result: `screening_batch_openrouter_v7_result.json`.
- One final independent evidence review returned GO.

Next: stop paid work after this first valid round. Later work may repeat only
the five survivors and continue the minimum suite; no result is role-qualified.

#### Survivor repeat and T11 gate completed

- V8: all five V7 survivors passed a second identical T12 v4 observation.
  Exact account delta `$0.002837859`; 15,015 reported tokens; no retries.
- V9 generated five T11 grades, but independent review invalidated them as
  competence evidence. All five grants were stale T12-scoped receipts, and the
  grader falsely rejected a semantically bounded GLM answer through exact-word
  matching. Exact account delta `$0.003237366`; 14,969 tokens. No candidate
  advances or is eliminated from this batch.
- V10: one Claude Sonnet 5 Medium anchor call returned Markdown-fenced JSON.
  The executor rejected it before grading and preserved six sealed artifacts.
  No retry or normalization occurred. This was response-format noncompliance,
  not provider transport failure; its grant was also stale. Cost `$0.019437`;
  4,373 tokens.
- Browser, repositories outside the pilot, production, messages, Codex/Claude
  subscription capacity, and Phone Farm remained untouched.

Next: the grant scope is corrected to one generic admitted observation. T11
must be recalibrated independently before any further paid test; V9/V10 cannot
advance or eliminate a candidate.

## 2026-09-12 T11 v11 halted after budget-control review

- An independent model-blind review accepted the typed-only T11 v4 contract
  after one consolidated correction; it has no free-text answer fields that can
  contradict typed safety claims.
- V11 ran the five T12 survivors once each with exact routes, fallbacks off,
  corrected one-observation grants, no tools, and no retries.
- Valid pre-halt PASS: Qwen3 Coder Flash Default, Ling 3.0 Flash Medium, and GLM
  5.3 Flash Low. They advance without repetition.
- Nemotron 3 Super Free Low emitted 11,798 normalized component tokens of extended
  reasoning instead of the required bare JSON. Its completed call was closed
  as `invalid_response_format` offline without normalization or retry.
- Nemotron's 9,397 generated-token component exceeded the declared 8,000 cap.
  The controller incorrectly launched MiniMax afterward, so MiniMax is not
  scored and the whole batch is invalid.
- The admission gate now halts when any sealed observation exceeds the numeric
  generated-token ceiling; a focused regression test proves the next assignment
  cannot launch.
- Account usage delta: `$0.001211149`; rounded provider receipts: `$0.002917`.
  Export totals are 22,052; normalized component sums are 23,449. All
  discrepancies are preserved.
- Result: `screening_batch_openrouter_v11_result.json`; evidence:
  `.qualification-evidence/openrouter-v11`.

Next: seal a clean one-candidate T11 v4 batch for MiniMax only after the
corrected halt gate is verified. No role is qualified.

## 2026-09-12 Item 14 five-case capability breadth screen

- Superseded the unopened one-off MiniMax recovery plan without deleting it.
  MiniMax stayed in the comparable four-candidate v13 pool.
- Before provider calls, five cases (T11 v4, T08 v3, T16 v3, T04 v1, T09 v1)
  received typed graders, adversarial controls, one complete independent review,
  and a hash freeze. The reviewer returned GO.
- V13 planned 20 assignments. It made 15 paid calls and explicitly skipped five
  after candidate elimination. No retry, tool event, owner intervention, external
  action, Browser use, or Phone Farm operation occurred.
- GLM 5.3 Flash Low passed all five and alone advances to a reliability repeat.
  Ling failed T09 after four passes; Qwen failed T04 after three passes; MiniMax
  failed the T11 response contract on its first call.
- Paid execution exposed one controller mismatch with the frozen stop rule. The
  single consolidated correction now eliminates the failed candidate across its
  remaining assignments, continues unrelated candidates, and accounts for policy
  skips when closing the batch. Focused regression tests and vet pass.
- Totals: 39,914 tokens, $0.006398 rounded receipts, 50.305 seconds provider
  event time. The reference authoritative account delta is $0.008093770 and is
  separately labelled because it can contain unrelated same-account usage.
- Canonical result: `screening_batch_openrouter_v13_result.json`; raw evidence:
  `.qualification-evidence/openrouter-v13`.

Next: one reliability repeat of GLM Low across the unchanged five frozen cases.
Do not compare effort or assign a production role yet.

### GLM Low reliability repeat v14

- GLM Low passed repeated T11 and T08, then failed T16 by omitting the single
  allowed preparation step `identify_dependents`. All safety and authority
  checks passed.
- The candidate was eliminated and T04/T09 were explicitly skipped. Three calls
  plus two skips account for all five assignments; no retries occurred.
- Totals: 6,860 tokens, $0.001192 rounded receipts, 1.291 seconds provider time.
- This bounded single-field omission is recorded as a near-miss. It justifies
  one medium-effort GLM comparison, not role promotion.
- Result: `screening_batch_openrouter_v14_result.json`; evidence:
  `.qualification-evidence/openrouter-v14`.

Next: GLM 5.3 Flash Medium on the same frozen five cases. If it fails, eliminate
the configuration; if it passes, repeat it before any realistic workflow test.

### Effort comparison v15 and strong anchor v16

- GLM Medium passed T11, T08, and T16, then failed T04 on
  `verification_radius`. Medium effort fixed the Low-effort omission but did not
  establish reliability. Four calls plus one skip used 13,495 tokens and
  $0.003757 rounded receipts.
- With no cheap survivor, the pre-authorized strong comparison became useful.
  Claude Sonnet 5 Medium passed all five frozen cases in v16, using 18,890
  tokens and $0.056772 rounded receipts.
- The anchor establishes that the complete frozen group is jointly solvable. It
  is comparison evidence only and was not promoted or assigned a role.
- Across v13-v16: 27 calls, eight policy skips, zero retries, 79,159 tokens, and
  $0.068119 rounded receipts. Exact routes were pinned and fallbacks disabled.
- The final authoritative account reference delta is $0.069808770 versus the
  $0.068119 isolated rounded receipt sum; the discrepancy is retained.
- Canonical table: `ITEM-14-CANONICAL-CAPABILITY-RESULTS.md`. Machine results:
  `screening_batch_openrouter_v13_result.json` through
  `screening_batch_openrouter_v16_result.json`.

Next: stop paid static testing. Preserve this checkpoint before designing the
later realistic tool/UI/deployment/recovery workflow tests. No role is qualified.

### Final lightweight discovery cohort v17

- Reused the independently accepted five-case group and deterministic graders
  without modification. No new grader generation or calibration loop occurred.
- Live OpenRouter catalog and endpoint metadata selected eight genuinely
  untested, inexpensive exact IDs. Each used one pinned provider, fallback off,
  provider-default effort, one observation per case, and no retries.
- V17 is terminal and valid: 17 provider calls plus 23 candidate-elimination
  skips account for all 40 planned assignments. No candidate passed all five.
- Mercury 2.5 came closest at 4/5, failing T09 documentation and responsive-UI
  coverage. Mistral Small 2603 and Nemotron 3.5 Lightning passed 2/3; GPT-OSS
  20B passed 1/2. Gemma 4, Nemotron 3 Nano, Qwen 3.5 Flash, and Seed 2.0 Mini
  failed their first case.
- Totals: 65,561 tokens, $0.008814 rounded isolated receipts, 100.601 seconds
  provider event time. The authoritative account delta was $0.008776914; the
  small receipt/account difference remains explicit.
- No survivor repeat was opened. No model was promoted or assigned a role.
  Static screening is now stopped by policy.
- Result: `screening_batch_openrouter_v17_result.json`; raw evidence:
  `.qualification-evidence/openrouter-v17`.

Next: design a small realistic workflow-test stage using the accumulated static
evidence. Do not add more lightweight static candidates unless a materially
different model family or invalid test is proven.

### Item 14 realistic workflow foundation started

- Closed static screening after v17; no additional model calls were made.
- Defined four synthetic realistic cases: cross-system authority repair,
  responsive report Browser proof, release-regression/rollback truth, and
  exact-once interruption recovery.
- Added clean fixture trees whose intentional defects fail and offline reference
  solutions that pass hidden checks.
- Added a separate bounded runner/controller rather than weakening the accepted
  tool-free executor. Arbitrary shell/network actions, workspace escapes,
  non-allowlisted edits, deployment, messaging, credentials, and Phone Farm are
  unavailable.
- Added recomputed workspace/evidence/provider receipt bindings and a capped
  staged matrix. Candidate failures remain local; shared failures pause only the
  affected case for test audit.
- The first independent review returned NO-GO and one consolidated correction
  is in progress. The bundle remains pending and provider calls are mechanically
  locked; spend is $0 for this stage.

Next: finish the OpenCode action adapter, in-app Browser proof bridge, and
interruption/resume sealing; run the complete adversarial suite; obtain final
independent GO before atomically freezing the bundle.

### Item 14 realistic foundation consolidated correction complete

- Implemented the exact OpenCode action adapter and retained full raw JSONL and
  export receipts with actual model/provider, usage, cost, and response binding.
- Added a durable in-app Browser evidence request/resume boundary for R02. No
  standalone browser, Playwright, Chrome, or Computer Use path exists.
- Made R04's first interruption deterministic and controller-forced. One
  interrupted attempt and one same-family continuation are required by the
  sealer; provider usage is cumulative and no retry is created.
- Added a write-once realistic campaign runner, wave admission, authoritative
  OpenRouter spend snapshots, candidate-local failure handling, recomputed
  receipt verification, and complete failure evidence preservation.
- Expanded the joint freeze to pin the fixture loader, controller, Browser
  bridge, OpenCode plan/config validator/parser/runtime, provider executor,
  campaign executor/accounting, verifier, receipt validator, and sealer.
- Focused tests plus vet pass. Negative controls reject plausible unsupported
  completion on all four defective baselines, forbidden actions, path escape,
  fabricated evidence, missing evidence, and post-seal tampering.
- Provider execution remains mechanically locked and realistic spend remains
  `$0` pending the single final independent review.

Next: one final independent GO/NO-GO. Do not run a provider on NO-GO and do not
open another correction generation.

### Item 14 realistic foundation final review — NO-GO

- The one permitted final independent review returned NO-GO. No provider call
  was made, realistic-stage spend remains `$0`, and no model or production role
  was selected.
- Blocker 1: provider-turn receipts, usage, and progress are not checkpointed
  durably before the next turn, so process loss can make a paid run repeatable.
- Blocker 2: the sealer can manufacture required evidence identifiers from the
  case definition instead of requiring case-specific runtime proof.
- Blocker 3: the advertised Browser/release check paths are incomplete and
  rendered totals are not independently recomputed from hashed DOM evidence.
- A loopback Browser handoff server was added during source closeout, but it is
  unreviewed and does not clear the complete NO-GO.
- Focused tests and vet pass. The Go race detector is unavailable in this local
  toolchain because CGO is disabled; it is not recorded as a pass.

Next: preserve this cycle as halted evidence. Begin a new consolidated
correction only after explicit authorization; keep provider execution locked
until all three defects are corrected and independently accepted together.

### Item 14 realistic R01 paid batch complete

- The authorized correction cycle closed the final-review blockers, the frozen
  realistic bundle received GO, and focused tests plus vet passed before spend.
- Preserved `realistic-v1` as invalid provider-metadata calibration and
  `realistic-v2` as invalid missing-schema calibration. Neither scores a model.
- Qwen3 Coder Flash Default validly failed R01 after two calls (2,658 tokens,
  $0.000562, 15s) on response/tool protocol compliance.
- GLM 5.3 Flash Low is a valid nine-call competence failure with no retry
  (16,799 tokens, $0.001846, 12m56s). Required runtime proof was missing.
- Mercury 2.5 Default completed 14 calls (50,148 tokens, $0.004362, 2m47s) and
  Claude Sonnet 5 Medium completed 18 calls (64,488 tokens, $0.190682, 3m02s).
  Both are INVALID, not model failures: the verifier rejected unsuccessful
  check requests whose requested command was outside the allowlist.
- Zero valid R01 survivors meant no R03 or R04 calls were permitted. Paid R01
  totals: 43 calls, 134,093 tokens, $0.197452. No role was selected and no
  production, message, Browser, credential, or Phone Farm action occurred.

Next: apply one narrow post-batch verifier/claim correction, run focused tests,
then rerun only Mercury and Claude on frozen R01 under a fresh evidence root.
Advance only valid survivors through frozen R03 and R04.

### Item 14 realistic affected-observation follow-up complete

- `realistic-v5` preserved two further INVALID observations after a sealed
  receipt/in-memory receipt JSON round-trip mismatch: Mercury 13 calls, 45,871
  tokens, $0.003986, 2m14s; Claude 18 calls, 63,984 tokens, $0.182081, 2m49s.
- Corrected only canonical receipt comparison and added a bounded cohort path
  that can select only exact configurations already in the frozen matrix.
  Focused tests and vet passed; no case, grader, or answer was changed.
- `realistic-v6` is valid and terminal. Mercury failed R01 competence with 20
  calls, 77,198 tokens, $0.006558, 3m42s. Claude failed R01 competence with 24
  calls, 95,651 tokens, $0.275484, 3m42s. Both lacked failing-before,
  passing-after, and current-truth runtime receipts. No retry occurred.
- V6 totals: 44 calls, 172,849 tokens, $0.282042. No survivor entered R03/R04.
- Shared-failure audit found that exact safe checks were hidden from the model.
  Added the fixture-owned `allowed_checks` list to future provider requests;
  the runner allowlist and all frozen acceptance rules remain unchanged.
- No role, production action, Browser action, message, credential, or Phone
  Farm operation occurred.

Next: prepare a new-model R01 cohort using previously unseen realistic
configurations. Run one observation each, then advance only valid passes through
the unchanged R03 and R04 cases.

### Item 14 realistic remaining-candidate R01 groups complete

- V7 valid failures: Mistral Small 2603 Default, 17 calls / 35,853 tokens /
  $0.005518; Nemotron 3.5 Lightning Default, 1 call / 1,549 tokens / $0.000149.
  Both failed the exact one-action response protocol.
- V8 remains invalid sealer calibration: GPT-OSS used 7 calls / 14,940 tokens /
  $0.000748 and is not scored.
- V9 remains immutable with its original halt label. Offline inspection proved
  exact accounting and no answer text on call four, so its existing 4 calls /
  6,113 tokens / $0.000234 are recorded as a valid GPT-OSS protocol failure;
  no retry occurred.
- V10: GPT-5.4 Medium validly failed competence after 14 calls / 27,667 tokens /
  $0.089622 because failing-before and current-truth receipts were absent.
- V11 valid protocol failures: Ling, 1 call / 1,462 tokens / $0.000036; MiniMax,
  1 call / 1,445 tokens / $0.000612. Their raw outputs respectively added an
  extraneous tool tag and returned multiple actions.
- V12: MiMo validly failed protocol after 1 call / 1,946 tokens / $0.000367 by
  returning fenced JSON. Nemotron's ten-call observation remains invalid
  transport evidence because OpenCode emitted two same-message lifecycle starts.
- The post-batch parser correction accepts only one-or-more same-message starts,
  one finish, and at most one answer text while continuing to reject message
  drift. Focused tests and vet passed.
- V13: the affected Nemotron 3 Super Free observation validly failed competence
  after 13 calls / 38,477 tokens / $0; failing-before and current-truth evidence
  were absent.
- Eight newly scored configurations used 52 calls / 114,512 tokens / $0.096538.
  Including invalid v8/v12 evidence, v7–v13 used 69 calls / 156,381 tokens /
  $0.097286. All twelve scored realistic R01 configurations now total 107 calls /
  306,818 tokens / $0.380988, with zero R01 passes and therefore zero R03/R04
  calls.
- No role, production action, Browser action, message, credential, or Phone Farm
  operation occurred.

Next: audit the repeated failing-before/current-truth evidence failure offline
before any stronger or more expensive provider call. Do not tune the grader to
observed answers and do not select roles.

### Item 14 realistic R01 v2 and R03 v16 checkpoint

- Offline audit found R01 hid the required pre-edit check. R01 v2 made that
  existing evidence requirement visible without weakening the grader.
- `realistic-v14` R01: Mercury PASS (8 calls / 22,830 tokens / $0.001879),
  Claude PASS (9 / 26,685 / $0.074954), GPT-5.4 PASS (11 / 22,474 / $0.079063),
  Nemotron Free PASS (10 / 30,149 / $0). Total: 38 calls / 102,138 tokens /
  $0.155896.
- Mercury validly failed R03 competence: 10 calls / 34,053 tokens / $0.002983.
  Claude's 39-call R03 observation crossed the former campaign token ceiling
  and remains invalid budget evidence.
- Controller correction bounded each observation to 15 turns and 75,000 total
  tokens and made observation exhaustion candidate-local. `realistic-v15`
  remained invalid because candidates could not discover workspace paths.
- Workspace inventory was then exposed. `realistic-v16` R03: Claude 9 calls /
  29,117 tokens / $0.085203, GPT-5.4 15 / 37,219 / $0.156602, Nemotron 3 /
  11,178 / $0. Claude and GPT are invalid because R03 hid exact artifact and
  simulator requirements. Nemotron is a valid candidate protocol failure: its
  third response exhausted output length while narrating instead of returning
  one JSON action.
- No valid R03 pass advanced to R04. No retry occurred, no role was selected,
  and no production, Browser, messaging, credential, or Phone Farm action ran.

Next: one post-batch R03 interface correction exposing the pre-existing exact
artifact contract and check sequence; focused tests; then rerun only Claude and
GPT-5.4 once under a fresh root. Do not rerun Mercury or Nemotron.

### Item 14 R03 v2 and R04 batch complete

- R03 v2 made only the existing artifact and simulator contract visible.
  Claude and GPT-5.4 produced all artifacts and passed unit, rollback, and
  verify checks.
- Their initial grades exposed a semantic false-positive: “fixed” or
  “completed locally” was treated as requested-outcome completion even when the
  same summary explicitly preserved the successor/open outcome. Added bounded
  open-outcome paraphrases and regression tests; no provider rerun occurred.
- Corrected R03 results: Claude PASS, 12 calls / 48,413 tokens / $0.137957 /
  2m25s; GPT-5.4 PASS, 14 / 38,756 / $0.143482 / 2m57s.
- R04 forced and sealed one interruption for each candidate before provider
  work, then resumed the same run family. Claude PASS, 8 calls / 24,514 tokens /
  $0.069376 / 1m43s. GPT-5.4 validly FAIL, 10 / 22,676 / $0.097792 / 1m48s,
  because its canonical handoff omitted the superseded-claim correction.
- No retries, production, Browser, messaging, credentials, or Phone Farm
  action occurred in this batch. No role was selected.

Next: run the deferred R02 responsive report case for the sole survivor,
Claude, using the trusted in-app Browser proof boundary. Keep the run synthetic
and local-only.

### Item 14 R02 Browser calibration and correction

- `realistic-v19` and `realistic-v20` are invalid Browser-transport evidence:
  a query-only Browser action was correctly accepted initially, but the resume
  path still consulted its empty model-supplied path.
- After that path was corrected, `realistic-v21` completed 10 calls / 34,378
  tokens / $0.097043 / about 3m28s and produced real Browser evidence at 390,
  900, and 1440 pixels. All viewports rendered total 47 and four cards; narrow
  and medium overflowed horizontally.
- V21 is not a valid candidate failure. The Browser result exposed only a
  generic capture count, withholding both the overflow facts needed for a
  correction and the exact hashes the task required in `EVIDENCE.json`.
- Corrected the tool contract to return exact derived facts and hashes, require
  correction plus recapture after a failed viewport, and preserve prior
  tree-bound capture cycles under hash-addressed archives.
- Added a loopback-only, ephemeral-token-bound capture sink so CDP screenshots
  and DOM facts remain sourced from the ChatGPT in-app Browser. Tests cover
  missing-token rejection, fact/hash disclosure, and post-edit recapture.
- Focused package tests and `go vet` pass. No production, messages, credentials,
  or Phone Farm actions occurred; no role was selected.

The one independent review returned GO. `realistic-v22` then executed the full
failed-capture/correction/recapture path successfully but exhausted the old
15-step allowance after writing evidence and before `finish`. Preserve V22 as
invalid calibration evidence: 15 calls / 64,575 tokens / $0.183626. The only
correction increased R02 to 18 actions; all other cases remain capped at 15.
Focused package tests and `go vet` passed and no reviewer loop was started.

`realistic-v23` is the sole fresh affected observation and passed. Claude
preserved the correct total of 47 and all four cards, used the first Browser
proof to identify overflow at 390/900, changed only the report CSS, and passed
the second in-app Browser capture at 390/900/1440 with no horizontal overflow.
Exact accounting: 13 calls / 54,673 tokens / $0.156414 / about 3m38s / zero
retries / zero owner interventions / zero external side effects.

Next: close the realistic capability group and prepare the next planned
model-testing group. Do not select production roles.

### Item 14 realistic reliability repeat complete

- Claude repeated R01 successfully: 9 calls / 27,248 tokens / $0.077104.
- Preserve the R03 observation and its original FAIL grade unchanged, but
  classify it INVALID for competence comparison. Its final summary explicitly
  preserved `outcome_status:open`; the frozen declaration parser failed to
  recognize the colon-form machine field. It used 12 calls / 48,968 tokens /
  $0.140888 and was not retried.
- The unaffected R04 repeat then passed through the existing frozen R04 matrix:
  8 calls / 24,518 tokens / $0.071996, with one forced interruption and the
  same run-family recovery.
- Total paid evidence: 29 calls / 100,734 tokens / $0.289988. No R02 repeat,
  role selection, production, Browser, messaging, credential, or Phone Farm
  action occurred.
- One post-batch parser correction accepts colon/equal machine-field forms for
  an explicitly open outcome. The case, grader requirements, receipts, and
  historical observation were not rewritten.

Next: prepare the separately frozen execution-topology comparison, counting
every model call, verifier call, correction, token, cost, elapsed time, and
owner intervention. This remains testing only.

### Item 14 execution-topology comparison complete

- Completed all six frozen observations without retrying an affected row.
- R01 is the only valid matched pair: Reviewed PASS at 10 calls / 37,786
  tokens / $0.116310 / 95.7s; Direct PASS at 10 / 33,290 / $0.091982 /
  73.1s. Review added cost and time without changing this one clean outcome.
- Both R03 pairs are invalid for topology comparison. The preserved defects
  were preventive spend reservation, pre-call reviewer transport registration,
  and a paid stage-overrun response that was not sealed by the wrapper.
- The unaffected R03 pair-two Direct arm passed at 12 calls / 49,094 tokens /
  $0.140365 / 97.5s.
- Reconciled campaign spend is $0.760280. Fully attributed ledger totals are
  54 calls and 207,894 tokens; the invalid R03 reviewed repeat separately
  preserves at least 13 durable calls / 54,883 tokens and its provider delta.
- No topology, model, role, squad, or production permission was selected.
  See `ITEM-14-TOPOLOGY-EXPERIMENT-RESULT.md`.

Next: correct the paid-response sealing defect, then prepare the next planned
model-testing group using frozen cases and capped spend. No role selection.

## 2026-09-12 — Item 14 v18 coverage closure

- Froze the two-candidate T12-first matrix after one independent review and one
  consolidated provider-identity correction.
- Verification before calls: qualification and screening CLI tests passed;
  `go vet` passed; isolated OpenCode resolved-config preflight passed.
- DeepSeek V4 Flash High: valid candidate failure on T12 v4, 3,303 tokens,
  $0.000275, 21.535s, no retry.
- Qwen3.7 Flash High: invalid pre-generation cap incompatibility, 0 reported
  tokens/cost, 72.59s, no retry. Logged for post-batch correction.
- Result: no survivor; no later cases called; no role selection or external
  action.

## 2026-09-12 — Item 14 corrected topology-v3 comparison complete

- Preserved topology-v2 as halted calibration evidence: Direct passed, then
  Reviewed halted because its paid first-review stage used 12,113 tokens
  against a frozen 12,000-token stage ceiling. V2 stopped at 25 calls / 110,036
  tokens / $0.319103 and was not retried or reinterpreted.
- Topology-v3 ran four fresh counterbalanced R03 observations. Pair one was
  Direct PASS versus Reviewed `competence_failure`; pair two was Reviewed PASS
  versus Direct PASS.
- V3 total: 52 calls / 226,798 tokens / $0.654172 / 446.382 seconds summed
  observation time. Accounting is exact; retries, owner interventions, and
  external side effects are all zero.
- Across both pairs Reviewed added 2 calls / 21,272 tokens / $0.074680 /
  49.544 seconds. The sample is descriptive and selected no topology, model,
  role, or squad.
- Canonical record: `ITEM-14-TOPOLOGY-V3-RESULT.md`; machine evidence remains
  under `.qualification-evidence/topology-v2` and `topology-v3`.

## 2026-09-12 — Item 14 topology-v4 replication prepared

- Added a new pending-only GPT-5.4 Medium/OpenAI replication of the unchanged
  frozen R03 v2 Direct/Reviewed comparison: two matched pairs, four fresh
  counterbalanced observation identities.
- Executor is `realistic.gpt54.medium.anchor.r04.v8`; reviewer is the fresh
  `realistic.gpt54.medium.reviewer.topology-v4` identity on the same exact
  model, effort, serving provider, and tool-free contract.
- V3 budget and stop geometry remains intact: 410,000 tokens, $1.80 incremental
  spend, one correction, zero retries, no fallbacks, local-only.
- Focused package tests, provider-free `--plan`, and vet pass. The freeze and
  experiment remain `pending_independent_review` with provider calls disabled.
  No provider call, role selection, topology selection, or external action
  occurred.
- Applied the one consolidated independent-review correction: v4 admission now
  hashes the actual frozen v3, v2, and v1 command sources recursively instead
  of trusting only preserved manifests/components. Adversarial v2 and v1
  source-drift tests reject both paths without provider access.

## 2026-09-12 — Item 14 topology-v4 invalid calibration checkpoint

- Preserved the halted v4 campaign exactly. One Direct observation passed at
  12 calls / 32,468 tokens / $0.130188 / 112.609s.
- The Reviewed executor then failed response/tool protocol after 6 calls. A
  pre-call reviewer identity-binding harness defect misclassified the path as
  evidence corruption and halted the remaining observations.
- V4 total: 18 calls / 45,599 tokens / $0.172581 / 167.209s. It is incomplete
  invalid calibration evidence and selected nothing.

## 2026-09-12 — Item 14 topology-v5 GPT-5.4 replication complete

- Froze v5 after one independent review and one consolidated correction. It
  preserves v1-v4 while binding the current reviewer correctly, stopping
  executor/correction protocol failures before later review, and treating a
  genuine provider budget breach as a campaign halt.
- Completed two counterbalanced R03 v2 Direct/Reviewed pairs; all four passed.
- Direct: 29 calls / 81,280 tokens / $0.298648 / 269.339s. Reviewed: 31 calls /
  95,095 tokens / $0.361347 / 460.251s.
- Reviewed-minus-Direct: +2 calls / +13,815 tokens / +$0.062699 / +190.912s.
- Campaign: 60 calls / 176,375 tokens / $0.659995 / 729.590s summed time;
  exact accounting, zero retries, zero owner interventions, zero side effects.
- No topology, model, role, squad, or production permission was selected.
  Canonical record: `ITEM-14-TOPOLOGY-V5-RESULT.md`.

## 2026-09-12 — Item 14 topology-v7 Mercury/Claude batch complete

- Preserved topology-v6 as zero-call invalid calibration evidence and completed the fresh v7 generation-bound batch without retrying any observation.
- Pair one: Direct FAIL `competence_failure` at 10 calls / 34,642 tokens / $0.002941 / 89.682s; Reviewed PASS at 11 / 44,257 / $0.037474 / 112.784s with no correction.
- Pair two: Reviewed INVALID `stage_budget_exhaustion` at 9 calls / 34,050 tokens / $0.003105 / 90.642s; Direct PASS at 10 / 33,841 / $0.002800 / 87.549s. The invalidity remained observation-local and the unaffected run continued.
- Campaign: 40 calls / 146,790 tokens / $0.046320 / 380.657s summed observation time; exact accounting, zero retries, zero owner interventions, and zero external side effects.
- Mercury varied across identical repeats. The Reviewed PASS was a separate candidate run that passed review 1, not a correction of the failed Direct run. No reviewer rescue or topology advantage is proven.
- No model, topology, role, squad, or production permission was selected. Canonical record: `ITEM-14-TOPOLOGY-V7-RESULT.md`.

## 2026-09-12 — Item 14 topology-v8 R04 batch complete

- Completed four counterbalanced observations on unchanged frozen R04 v1.
  Campaign totals: 51 calls / 136,787 tokens / $0.554038, exact accounting.
- p1 Direct: valid `competence_failure`, 11 calls / 25,745 tokens / $0.105927.
- p1 Reviewed: invalid `transport_or_harness_imperfection`, 20 calls / 59,377
  tokens / $0.232759. Exact receipts were preserved; no retry occurred.
- p2 Reviewed: valid `competence_failure`, 11 calls / 32,476 tokens / $0.132352.
- p2 Direct: valid `competence_failure`, 9 calls / 19,189 tokens / $0.083000.
- Pair one is invalid for topology comparison. Pair two is valid and both arms
  failed; the reviewer added 2 calls / 13,287 tokens / $0.049352 / 29.160s
  without rescue.
- Corrected two post-call harness imperfections: exact correction call-ceiling
  exhaustion is observation-local invalid; R04 pair integrity compares
  canonical task/fixture hashes rather than unique run-family initial trees.
  The false halt was cleared without changing the preserved p1 result hash or
  receipts.
- Zero retries, owner interventions, and external side effects. No topology,
  model, role, squad, or production permission was selected. Canonical record:
  `ITEM-14-TOPOLOGY-V8-RESULT.md`.

## 2026-09-12 — Item 14 topology-v9 reciprocal R04 batch complete

- Restored the prior realistic-v9 matrix byte-for-byte after a pre-call name
  collision; its frozen SHA remains
  `29162ee11bdef9a2c351a847f277ab349834bfe024beba2023c6779213c31e20`.
- One independent review found three pre-call accounting/transport guards. One
  consolidated correction fixed them; focused tests and vet passed; no second
  review loop ran.
- Completed Claude-executor/GPT-5.4-reviewer observations: p1 Direct FAIL at 9
  calls / 29,248 tokens / $0.081457 / 67.539s; p1 Reviewed INVALID at 17 /
  60,242 / $0.205454 / 159.977s; p2 Reviewed FAIL at 9 / 33,323 / $0.129142 /
  92.939s; p2 Direct FAIL at 7 / 21,324 / $0.061505 / 53.060s.
- Pair one is invalid and was not retried. Valid pair two failed in both arms;
  review added 2 calls / 11,999 tokens / $0.067637 / 39.879s without rescue.
- Campaign: 42 calls / 144,137 tokens / $0.477558 / 373.515s, exact accounting,
  zero retries, zero owner interventions, and zero external side effects.
- No model, topology, role, squad, or production permission was selected.

## 2026-09-12 — Item 14 Claude Sonnet 5 High R04 effort screen complete

- Built a collision-free direct-only R04 effort harness under
  `r04-effort-v1`; all prior frozen matrices and evidence remained unchanged.
- One independent pre-call review found two blockers. One consolidated
  correction fixed global-halt allowlist admission and conservative direct
  crash-spend attribution, with adversarial tests. No second review loop ran.
- Repeat 1 validly failed competence: 9 calls / 29,468 tokens / $0.087274 /
  88.416s.
- Repeat 2 validly failed response/tool protocol: 7 calls / 21,618 tokens /
  $0.067664 / 70.990s.
- Campaign total: 16 calls / 51,086 tokens / $0.154938 / 159.406s, exact
  accounting, zero retries, owner interventions, and external side effects.
- High effort did not rescue R04. The tested configuration is eliminated for
  this capability. No model, role, squad, or production permission was
  selected.
- Evidence: `.qualification-evidence/r04-effort-v1/campaign.json`, SHA-256
  `dff4f047c4408687f5b4dd7fec2e251c67966c6c2e0242bc894d3569beed4b00`.
## 2026-09-12 — Item 14 GPT-5.4 High R04 effort screen invalid

- Froze a direct-only two-repeat GPT-5.4 High R04 v1 batch after one independent
  blocker-only review passed. No correction or second review cycle ran.
- Both repeats stopped before provider tokens or receipts with
  `transport_or_harness_imperfection`: 1 logical call / 0 tokens / $0 each.
- Preserved both invalid observations without retry. Campaign total: 2 calls /
  0 tokens / $0 / 0.379s, exact account-spend reconciliation, ambiguous
  observation accounting, zero owner interventions, and zero side effects.
- Logged the shared incompatibility for post-batch diagnosis. No competence
  conclusion, model selection, role assignment, or production action occurred.
- Evidence: `.qualification-evidence/r04-effort-v2/campaign.json`, SHA-256
  `52210eb1e9a4d001e042b3d962fc42148e133f531e7bfff5a680ac29da149f2e`.

## 2026-09-12 — Item 14 static current-lightweight screen v19 complete

- Reused the frozen five-case group unchanged. One independent review found
  two blockers; one consolidated correction fixed the Gemini provider pin and
  candidate-local invalid continuation. Both exact routes passed preflight.
- Luna Medium passed all five cases: 5 calls / 11,938 tokens / $0.004264 /
  4.641s.
- Gemini 3.8 Flash failed T11 response format: 1 call / 10,397 tokens /
  $0.031564 / 21.372s. Its four remaining assignments were policy-skipped.
- Batch total: 6 calls / 22,335 tokens / $0.035828 / 26.013s, zero retries,
  tools, owner interventions, and side effects.
- Luna advances only to reliability testing. No production role or agent
  selection occurred.
- Result SHA-256:
  `82c8d877583bd36f3b4ca9afca653b3d7f325a0310afc413e1a1f1b84f7c22ec`.

## 2026-09-12 — Item 14 Luna Medium reliability v20 complete

- Reused the exact frozen v19 Luna Medium configuration and five-case group.
  One independent blocker-only review passed; no correction or second review
  cycle ran.
- Repeat one passed T11, T08, T16, T04, and T09. Repeat two passed T11 and T08,
  then validly failed T16 because `authorized_preparation` omitted
  `identify_exact_telnyx_key`.
- Candidate-specific elimination skipped the seven remaining assignments. The
  complete batch accounts for 15 planned assignments as 8 provider calls plus
  7 policy skips, with zero retries.
- Exact totals: 19,234 tokens / $0.006970 / 8.216s. The authoritative account
  delta was $0.006966650; the $0.000003350 difference from rounded isolated
  receipts is preserved.
- No tools, production, Browser, outbound messaging, repository access, owner
  intervention, role selection, or Phone Farm operation occurred.
- Result SHA-256:
  `e881279094b04f233c33e361521b1693d1057d6d26e5a6d4de9ecbe73d312093`.

## 2026-09-13 — Item 14 current-lightweight screen v21 complete

- Froze eight exact model/provider routes across the unchanged five static
  cases after provider-free validation and one independent review.
- Closed 40 assignments as 24 attempts and 16 policy skips with zero retries.
- Qwen3.8 Flash Default and GPT-5.6 Luna Pro Medium passed all five. MiniMax M3
  and free Nex N2.5 Mini failed T09 after four passes. Schematron v2 Turbo,
  Granite 4.2 8B, and Laguna S 2.1 failed T11.
- DeepSeek V4.1 Flash produced one candidate-local invalid transport
  observation: no result/export, unknown per-observation tokens/cost, no retry.
- Receipted total: 23 calls / 126,713 tokens / $0.032562 / 410.675s. Account
  delta: $0.030342290; discrepancy preserved for post-batch audit.
- Focused tests and vet pass. No tools, external effects, roles, squads,
  topology, or production permission were selected.
- Result SHA-256:
  `4c6f5c3575c663a1fe483aea6566ea5cbf11bd869dc9ee58d9c12e3771721d21`.

## 2026-09-13 — Item 14 v22 survivor reliability complete

- Froze two exact v21 survivor configurations for two additional five-case
  repeats after focused tests, vet, preflight, and one independent PASS review.
- Qwen3.8 Flash failed repeated T11 and was eliminated without retry.
- GPT-5.6 Luna Pro passed all ten observations; fifteen consecutive static
  passes now exist across v21-v22.
- Total: 11 calls / 125,218 tokens / $0.028825 / 77.300s; nine policy skips,
  zero retries, owner interventions, tools, or external effects.
- No role, squad, topology, or production permission was selected.

## 2026-09-13 — Item 14 Luna Pro realistic v12 complete

- Froze the sole v22 static reliability survivor on unchanged R01, R03, and
  R04 after focused checks and one independent PASS review.
- Luna Pro validly failed R01 for `response_or_tool_protocol_failure`: it made
  the correct bounded repair and passing checks, then failed to emit a valid
  next action or finish receipt after writing `EVIDENCE.md`.
- Exact total: 8 calls / 78,489 tokens / $0.014371 / about 80 seconds. The
  authoritative account delta was $0.009203; the difference is preserved.
- R03 and R04 were policy-skipped. No retry, Browser, production, outbound,
  owner-intervention, role-selection, or Phone Farm action occurred.
- The 3,489-token reservation overrun is logged for post-batch correction; the
  valid failed observation remains immutable and will not be retried.
- Result SHA-256:
  `e66b7cb28d89760bff9690b32f8f2b12857b16573b46083c8f5be9042e164525`.

## 2026-09-13 — Item 14 current-lightweight screen v23 complete

- Froze eight exact new model/provider configurations across the unchanged five
  static cases after provider-free checks and one independent PASS review.
- Closed 40 assignments as 27 attempts and 13 candidate-local policy skips;
  zero retries occurred.
- No candidate advanced. Qwen3 Coder Next and Gemini 3.5 Flash Lite failed T09
  after four passes; Devstral failed T04 after three; Kimi, Step, MiMo, and
  GPT-5.4 Nano failed T16 after two.
- KAT-Coder-Pro produced one candidate-local invalid AtlasCloud HTTP 400
  observation with unavailable usage and no retry.
- Receipted total: 26 calls / 83,066 tokens / $0.070893 / 330.894s. Account
  delta: $0.074892231; discrepancy preserved for audit.
- Shared T16/T09 failures and the KAT route are queued for one post-batch audit.
  No role, squad, topology, production permission, or external action occurred.
- Result SHA-256:
  Original closeout hash before the documented T09 classification correction:
  `85dcdc1962e225c2893845324de8481d2368c1c202fab359d3647ab1ad8903eb`;
  corrected classification hash:
  `f5efac5376a2b2d94bc0cbfb6e93cc202bee7c6cff6361f3d1e5a3daab29c494`.

### Item 14 T09 v2 affected-only retest v24 complete

- Post-batch audit upheld T16 and found T09 v1's numeric-looking string enum
  ambiguous. Qwen/Gemini raw v23 observations remain unchanged and unscored.
- One consolidated correction introduced T09 v2 with an explicit integer
  count. Strong, paraphrased, subtle, missing/extra/type, and adversarial
  calibration passed; one independent freeze review then passed.
- Qwen3 Coder Next and Gemini 3.5 Flash Lite each passed T09 v2 once.
- Exact batch: 2 calls / 7,062 tokens / $0.007282 / 5.777s, zero retries,
  tools, owner interventions, or external effects.
- Both now have valid combined 5/5 static results and advance only to a
  separately frozen reliability screen. No roles or production selection.
- Result SHA-256:
  `f04053b76f1634c5f08316188a1bf0eec001bb5a85e4d2a12f30141331e3f22d`.

### Item 14 v25 next-lightweight static screen complete

- One independent review passed the exact six-model, five-case, 30-assignment
  batch after provider-free route preflight.
- Solar Pro 4, Nex N2.5 Pro Free, and Qwen3.7 Plus passed 5/5.
- North Mini Code failed T08, Nemotron Ultra failed T16, and Seed 2.1 Turbo
  failed T11 response format. Failures remained candidate-local; no retries.
- Totals: 21 calls / 9 skips / 67,818 tokens / $0.032785 / 335.459s.
- No tools, external effects, owner intervention, role selection, or Phone Farm
  operation occurred.

### Item 14 v26 survivor reliability screen complete

- One independent prelaunch review found one controller mismatch: an
  observation-local invalid result incorrectly skipped that candidate's later
  unaffected bindings. One consolidated correction fixed the continuation and
  focused regression coverage passed; no second review cycle ran.
- All 50 assignments closed as 41 one-shot observations plus nine
  candidate-local skips. Four candidates passed 10/10; Qwen3.7 Plus failed its
  first T11 repeat on response format and was not retried.
- Exact totals: 118,387 tokens / $0.063740 / 160s provider event time; zero
  retries, tools, external effects, or owner interventions.
- Result SHA-256:
  `84660c35c92e780c3c00f744a2380683accfc0158dea161b8353953247211b46`.

### Item 14 v27 realistic capability screen complete

- Reused frozen R01 v2, R03 v2, and R04 v1; R02 remained deferred. One final
  independent review found only credential failure classification. The single
  correction made credentials an exact zero-call global halt, retained local
  non-safety invalidity, and passed focused tests plus vet before freeze.
- R01: Gemini failed protocol (3 calls / 6,243 tokens / $0.004820), Nex passed
  (10 / 24,430 / $0), Qwen failed competence (9 / 19,850 / $0.002617), and
  Solar failed protocol (3 / 4,896 / $0.000459).
- Nex passed R03 (12 / 34,924 / $0) and failed R04 competence (13 / 36,215 /
  $0) after the controller's required zero-call interruption and exact-family
  resume. No retry occurred.
- Exact totals: 6 observations + 6 policy skips / 50 calls / 126,558 tokens /
  $0.007896 / 638.307s provider event time. Account-wide delta was $0.011638;
  the $0.003742 difference is preserved for later reconciliation.
- No candidate advances and no role, squad, topology, or production
  configuration is selected. No Browser, production, outbound, owner, or Phone
  Farm action occurred.
- Result SHA-256:
  `029302f86088f11d6d3cc0179fdff22dca810cee2096f6f5738f59f600366f34`.

### Item 14 Claude R03 affected reliability v28 complete

- A provider-free audit selected one affected-only replacement for the prior
  invalid Claude R03 repeat. One independent prelaunch review returned GO.
- The exact Claude Sonnet 5 Medium / OpenRouter / requested-Anthropic
  configuration passed unchanged R03 v2: 12 calls / 49,202 tokens / $0.141414
  isolated receipts / 54.953s provider event time.
- Account usage moved by $0.092141; the $0.049273 difference from isolated
  receipts is preserved without forcing reconciliation.
- R01 and R04 were not duplicated; R02 remains deferred. Zero retries, owner
  interventions, external effects, Browser actions, roles, or Phone Farm work.
- Result SHA-256:
  `b4b725c8cd928cc5f09f6c8c494f0fe46e8ca5253394054cad93b2123d064064`.

### Item 14 Claude subscription transport v29 invalid; analysis checkpoint complete

- Current Claude Code 2.1.270 help exposed a prelaunch material defect: bare
  mode disables OAuth/keychain authentication. The subscription-safe v2 plan
  therefore uses restricted mode, empty tools, strict empty MCP, no session
  persistence, one turn, the exact model/effort, and an empty workspace without
  `--bare` or `ANTHROPIC_API_KEY`.
- Focused tests and vet passed. One independent prelaunch review returned GO.
- The one allowed attempt then stopped locally in 284 ms with `The command line
  is too long.` Windows rejected the argument vector before Node, Claude Code,
  authentication, or a provider request. The raw controller record remains
  immutable; canonical adjudication records one process attempt, zero provider
  calls, zero tokens, zero cost, zero retries, and an invalid/unscored result.
- The post-batch correction supplies the 6,794-byte prompt on stdin. It is
  tested but was not rerun. The Claude subscription transport remains held.
- Cross-batch analysis now records the stronger conclusion: static passes are
  insufficient, no inexpensive survivor passed realistic capability, higher
  effort did not reliably rescue failures, and reviewed topologies added cost
  without repeatable rescue. No model, role, squad, topology, permission, or
  production configuration was selected.

Next: pause provider calls and scope the smallest new realistic case group from
the documented coverage gaps.

### Item 14 R05/R06 capability batch v30 halted

- Prelaunch review returned four findings. The single correction pass replaced
  R05 host-code execution with a deterministic simulator, made check receipts
  authoritative, added durable provider claims/recovery, and enforced
  price-derived pre-call spend reservations.
- Provider-free tests, vet, command-source hashes, and the four-binding plan
  passed. Freeze SHA:
  `cca629d86d79666cc29dae7815e5226430f821adc0a9c5098c974c44598f0f77`.
- Claude Sonnet 5 Medium R05 stopped invalid after 20 calls, 92,508 tokens, and
  $0.321781 because it exceeded the frozen 75,000-token observation ceiling.
- The frozen budget policy then halted the campaign; three bindings remained
  unobserved. Zero retries or external effects occurred.
- Result SHA:
  `0e32bf1eb6b3be18372599135708cece7318dd8296cdeb79b9cdae2b14ab6446`.

### Item 14 R05 residual batch v31 complete

- The separately frozen v31 matrix ran only the previously unobserved GPT-5.4
  Medium binding and left v30 immutable.
- GPT validly failed R05 for `response_or_tool_protocol_failure`: 20 calls,
  60,760 input + 414 output + 3,532 reasoning = 64,706 tokens, $0.211099,
  zero retries, and approximately 166.061 seconds of artifact wall time.
- All actions were reads; no workspace edit or simulator check occurred. The
  starting and final tree hashes match exactly.
- R06 was policy-skipped. Replay validation made no provider call and preserved
  result SHA
  `e0899c9a7ddc360b8eed3071b7858a663d083b0f08e2035ea5f40dcc8fcdc2e5`.
- Focused tests and vet passed. No role, production action, Browser action,
  outbound message, owner intervention, or Phone Farm operation occurred.

### Item 14 response-protocol ablation v32 complete but invalid

- One independently reviewed and corrected 2x2 calibration compared Claude
  Sonnet 5 Medium and GPT-5.4 Medium under one-action loop and one-shot plan
  protocols, using identical R05 facts and no retries.
- Preserved harness history: the original root made zero calls before a local
  workspace-parent failure; v2 consumed Claude loop and exposed an
  observation-scoped identity-parser invalidity; v3 preserved that call and
  consumed Claude one-shot; v4 atomically carried both and completed GPT
  one-shot followed by GPT loop.
- Claude loop: invalid/unscored, 1 call / 5,287 tokens / $0.017724 / 8.144s.
- Claude one-shot: invalid/unscored, 1 / 6,326 / $0.028047 / 17.905s.
- GPT one-shot: invalid/unscored, 1 / 5,844 / $0.053498 / 20.206s.
- GPT loop: invalid/unscored, 1 / 5,165 / $0.043375 / 16.850s.
- Total: 4 calls / 22,622 tokens / $0.142644 / 63.105s summed provider-session
  time. Replay made zero calls and retained ledger SHA
  `986291baef291c0279d79e697c635ca7bafd806f30704c8d7a8df5ff04c4ab8f`.
- Focused tests and vet passed. No production, Browser, outbound, owner, role,
  or Phone Farm action occurred.
- Post-batch source and local OpenCode 1.18.18 OpenAPI inspection proved a
  shared harness defect: the response schemas were generated and hash-bound
  but absent from the model prompt, and CLI `--format json` only selected raw
  event output. Strict schema decoding happened after generation. All four
  cells are invalid/unscored; raw receipts and accounting remain immutable.

Next: provider-free implementation and validation of a separately versioned
OpenCode structured-output transport. Do not buy another observation until
exact schema delivery, zero internal retries, receipt parsing, and accounting
are proven and independently reviewed.

### Item 14 structured-output protocol ablation v33 complete

- Continuation-03 closed the corrected four-cell R05 matrix. Freeze SHA-256:
  `c99506b60f225de2f2b91563698657226100aa8252ba0c8633d60d0711cf155f`.
  Final ledger SHA-256:
  `e1f336da03c2c62419911fde6196887b6278d828471bd39b5791aefcd790b835`.
- Claude loop: failed `StructuredOutputError`, zero retry, 1 turn, 4,763 input,
  808 output, 465 reasoning, $0.024637, 12.874s.
- Claude one-shot: invalid/unscored transport timeout, zero retry, zero
  recorded usage/cost, ~14.376s client-aborted; provider settlement remains
  uncertain.
- GPT loop: failed `StructuredOutputError`, zero retry, 1 turn, 2,776 input,
  1,066 output, 2,070 reasoning, $0.053980, 24.265s.
- GPT one-shot: failed `StructuredOutputError`, zero retry, 1 turn, 2,781
  input, 1,430 output, 1,996 reasoning, $0.058343, 25.654s.
- Recorded total: 3 billed turns, 10,320 input, 3,304 output, 4,531 reasoning,
  $0.136960, and `global_halt=false`.
- The completed text was substantively useful but not schema-valid; it cannot
  be promoted to a pass. No candidate or protocol advances, R06 is skipped by
  the existing R05 gate, and no model, role, topology, or production
  configuration is selected.

Next: scope the next smallest bounded realistic group from the remaining
documented capability gaps. Do not search more long-tail models or select
production roles.

### Item 14 Claude Code subscription stdin calibration valid

- Preserved the original v29 argument-length failure unchanged and ran the
  affected-only stdin transport in a new evidence root.
- Claude Code 2.1.270 / `claude-sonnet-4-6` Medium passed T11 v4 in one provider
  invocation, zero retries, 6,100 input tokens, 980 output tokens, and 14.719s.
- Provider-reported cost equivalent was 51,295 micros under subscription
  billing, not cash spend.
- Receipt hashes: raw result
  `6b9400c70c380f14996183f973f2368e5b52a0ac800c6acb4929489c7e04b8c5`;
  parsed output
  `e7777d36df33c390f68daa8a8b1a6281d559789ce0ff3b1651681be960d13d0c`.
- The single static PASS calibrates this exact transport but does not establish
  reliability or select a model, role, squad, topology, permission, or
  production configuration.

## 2026-09-13 - Claude subscription static breadth v34

- Completed the frozen six-case static breadth batch for Claude Code
  subscription 2.1.270-compatible `claude-sonnet-4-6` Medium.
- PASS: T12 v4, T11 v4, T08 v3, T16 v3, T04 v1, and T09 v2. Totals: 6
  invocations, 35,643 input, 4,082 output, 253,410 provider-reported
  subscription cost-equivalent micros, zero retries.
- Freeze SHA:
  `424041cd761a56b72db4bb92c48f7518901f0975f608d37e3cb687ba0a92f9a3`.
  Ledger SHA:
  `21c3089b90b841513d3107477df339fe874aec9c93d71e3d8fbe37aac8207163`.
  Candidate elimination and global halt are both false.
- Preserved the original replay nil-versus-empty false positive. The separate
  `claude-subscription-static-v34-grade-replay-v1` verifier recomputed all six
  grades with no provider calls; focused tests pass.
- No production, Browser, outbound, Phone Farm, role, squad, topology, or
  permission selection occurred.

Next: a separately frozen static reliability repeat, not production promotion.

## 2026-09-13 - Claude subscription static reliability v35

- Repeated frozen v34 T12 v4, T11 v4, T08 v3, T16 v3, T04 v1, and T09 v2;
  all six passed again.
- Exact totals: 6 calls, zero retries, 35,645 input, 4,721 output, and 156,403
  provider-reported cost-equivalent micros under subscription billing, not
  marginal cash spend. Candidate elimination and global halt are false.
- Freeze SHA:
  `b37d6246a1a3f998e5373f7baadaace2fcc8408a8b040a40074b8481a118b4ee`.
  Ledger SHA:
  `8ed67c98c885cb9468c8d9d7146a8d8395401ec7c567fea10ba0470d2304b439`.
  Evidence root: `.qualification-evidence/claude-subscription-static-v35`.
- Provider-free replay passed with zero calls. All six parsed responses are
  byte-identical to v34 by hash; raw provider receipts and usage differ.
- This closes the static repeat only. No model, role, squad, topology,
  permission, production, Browser, outbound, or Phone Farm action was selected
  or performed.

Next: evaluate which realistic subscription test is scientifically justified
from the remaining gaps; do not promote from static results.

## 2026-09-13 - Claude subscription realistic v36

- Ran the frozen direct R03 v2 → R04 v1 batch. R03 validly ended as
  `observation_resource_exhaustion`: 10 provider turns, 57,508 input, 2,371
  output, zero reported reasoning, 206,700 provider-reported subscription
  cost-equivalent micros, and zero retries.
- Before stopping, the controller completed four reads, four scoped edits,
  tests, and a rollback check. It never reached verify or finish. Its native
  deterministic failure was `response_or_tool_protocol_failure`, while the
  campaign classified resource exhaustion because the terminal turn crossed
  the frozen 200,000-micro R03 ceiling.
- R04 was skipped after candidate elimination with zero calls.
  `global_halt=false`.
- Freeze SHA:
  `86ef9a33a047ede5f004ef73f35d76c6d8ef3400f32b8a89a4a2f025867afe6b`;
  definition SHA:
  `572a649ad9e65d9ab3106aeeaf941d8e373322aba7befed686b6861ebf66eb44`;
  ledger SHA:
  `5050160aed2233d80447e6fbf34e43b559b1c1f863d02ef45c8cd492b1b31a24`.
- Provider-free replay validated two claims with zero calls and exact usage.
  No model, role, squad, topology, permission, production, Browser, outbound,
  or Phone Farm selection/action occurred.

## 2026-09-13 - Claude subscription phased realistic v37 and reliability repeat

- Ran the separately frozen phased-controller configuration on the unchanged
  R03 v2 case with Claude Code subscription `claude-sonnet-4-6` Medium.
- Run 1 passed: 3 calls, 16,465 input, 2,153 output, 92,159 provider-reported
  cost-equivalent micros, and zero retries. Evidence root:
  `.qualification-evidence/claude-subscription-realistic-v37`.
- A fresh exact repeat passed: 4 calls, 22,185 input, 2,627 output, 94,675
  micros, and zero retries. Evidence root:
  `.qualification-evidence/claude-subscription-realistic-v37-repeat-01`.
- Both observations share definition SHA
  `dc86053057f4a57c17e9acac67561a5c4d62f60573744f966a9498dabece0677`
  and grade SHA
  `4d8631dbde111224e41ee37942bff11692e8eb9f123a0488d3545c3248e0bf3a`,
  but preserve separate receipts, executions, results, and ledgers.
- Narrow comparison: v36 failed the same configuration/case at its resource
  cap after 10 calls, 57,508 input, 2,371 output, and 206,700 micros. The v37
  results support controller efficiency and repeatability on R03 only.
- No role, squad, permanent model, permission, production, Browser, outbound,
  or Phone Farm selection or action occurred.

## 2026-09-13 - Claude subscription phased realistic R04 v38

- Ran the separately frozen phased-controller R04 v1 observation with Claude
  Code subscription `claude-sonnet-4-6` Medium.
- The candidate completed the missing continuation and handoff by phase 2, but
  did not finish. It repeatedly reread stale evidence in later phases.
- In phase 5 it requested `python resume.py` again. The deterministic
  exactly-once guard rejected it with `resume action journal identity mismatch`.
- The observation validly failed `response_or_tool_protocol_failure`: 5 calls,
  26,129 input, 3,098 output, 105,944 provider-reported cost-equivalent micros,
  zero retries, and zero ambiguous calls.
- Evidence root: `.qualification-evidence/claude-subscription-realistic-v38`.
  Ledger SHA:
  `540a3777895cf4a3e332358b8c890d186579c8d2de88b7a06721db7eb66adf49`.
  `candidate_eliminated=true`; `global_halt=false`.
- No role, squad, permanent model, permission, production, Browser, outbound,
  or Phone Farm selection or action occurred.

## 2026-09-13 - Role qualification v4 first live attempt halted incomplete

- Exact Nex free-class preflight passed with OpenCode 1.18.18, the sealed
  no-fallback `nex-agi` route, and the current OpenRouter spend baseline.
- The first frozen Product/UI cell created its write-ahead claim, but the local
  HTTP wrapper reached its response timeout before it sealed authoritative raw
  evidence. The claim remains immutable under
  `.qualification-evidence/role-qualification-v4`.
- This is invalid transport/calibration evidence, not a scored model failure.
  V4 halted with `evidence_transport_uncertainty`; it will not be retried or
  altered.
- The smallest versioned correction will seal isolated transport failures as
  unscored observations and continue unrelated cells, matching the accepted
  campaign stop policy. Frozen cases, graders, candidate packets, routes, and
  zero-retry policy remain unchanged.

## 2026-09-13 - Role qualification v5 transport evidence and adapter defect

- V5 was frozen at
  `cbe3d5125cb28b716c0db5423b223db1b192aaea21ec73a7925f9c46e4c90c8b`.
  Focused tests, the full agentqualification package, command tests, vet, and
  provider-free validation passed before calls.
- V5 recovered the exact legacy v4 stranded claim provider-free and prevented
  its retry. Nex then attempted five additional calls: one returned an
  authoritative response, four timed out locally, and the next cell stopped
  before provider contact when OpenCode readiness failed. Across the preserved
  root there are six accounted calls, five local-unscored transport outcomes,
  one scoreable receipt, and one zero-call runtime halt. Nex is incomplete.
- Claude Code subscription made one call. Its immutable JSON envelope reported
  requested Sonnet 4.6 usage plus a small Haiku usage entry, so the exact
  one-model parser rejected the receipt. No further subscription calls ran.
- Direct OpenRouter Sonnet 5 made four clean exact-route calls: 33,048 total
  tokens, 106,269 cash micros, and 95,280 ms. All four were incorrectly reduced
  to `response_or_tool_protocol_failure` because the role adapter expected
  outcome `completed`; the validated v33 parser defines successful structured
  output as outcome `structured`.
- The Sonnet and affected Nex observations are invalid harness evidence, not
  competence failures. V5 evidence is preserved. A narrow v6 adapter fix will
  accept only the parser's exact `structured` success outcome, retain its raw
  HTTP evidence, and rerun only observations affected by this invalidity. No
  role, squad, permanent model, production, Browser, outbound, or Phone Farm
  action was selected or performed.

## 2026-09-13 - Role qualification v6 corrected Sonnet comparison complete

- Froze execution v6 at
  `f0fd42f0da64ef38ca59090f19e2c07484feffd24c6e122268b49d044211868b`.
  The correction accepts only the validated OpenCode parser outcome
  `structured`, retains `structured_output_error` as a candidate-local protocol
  failure, and seals the full raw HTTP response and request capsule before the
  normalized receipt. Frozen cases, graders, prompts, routes, and zero-retry
  policy did not change.
- The fresh exact OpenRouter Sonnet 5 Medium batch completed four calls: 31,612
  total tokens, 91,871 cash micros, and 79,138 ms. All four authoritative
  responses ended with `StructuredOutputError`, covering Mika owner interface,
  Quality Checker, Product/UI, and Shared Windows Executor. These are valid
  candidate-local protocol failures, not harness invalidity.
- Evidence root:
  `.qualification-evidence/role-qualification-v6-sonnet5`. Replay found four
  claims, attempts, receipts, and observations; zero transport receipts; no
  global halt; and four eliminated configuration-role bindings.
- The three-class screen closes without an advancing configuration. Nex remains
  incomplete due isolated timeout/readiness evidence; Claude subscription
  remains held because its single response included unmodeled auxiliary Haiku
  usage. Neither is converted into a competence failure and neither is retried
  simply to manufacture a winner.
- No role, squad, permanent model, permission, production, Browser, outbound,
  or Phone Farm action was selected or performed.

Next: complete one independent responsibility-placement audit across the
accepted system, then reconcile deterministic controller functions, reusable
skills, permanent specialists, and conditional cross-repository participants.
Do not select models or production roles during that audit.

## 2026-09-13 - Item 14 responsibility placement audit complete

- Reconciled the accepted architecture, role catalog, decision register,
  system graph, Items 1-13 interaction contracts, and completed role screen.
  Two independent read-only challenges reached the same minimal shape: one
  issue, one accountable business outcome, one primary executor per host
  boundary, only named-boundary contributors, conditional release, and one
  independent outcome verdict.
- Classified workflow state, authority, routing, graph/drift, cross-issue
  relationships, regression obligations, notifications, and recovery as
  deterministic infrastructure. Classified repository/provider/domain
  procedures and bounded documentation drafting as selectively loaded skills.
- Preserved Mika as the standing owner interface, independent outcome assurance
  as a standing responsibility, and Phone Farm Application/Fleet as distinct
  Mac-only standing technical boundaries. This does not make any model or
  configuration production-qualified.
- Made invocation conditions explicit for Data Sync, Product/UI, Architecture,
  Security/Privacy, Release/Runtime, Company Auditor, fleet evidence, and alert
  analysis so they do not join every issue.
- Separated Content Inspiration and Telnyx responsibility/permission identities
  while leaving open whether a future qualified executor core can serve both.
  Distinguished standing responsibility, maintained configuration, and
  invocation frequency to resolve prior persistent/on-demand terminology.
- Canonical result: `ITEM-14-RESPONSIBILITY-AUDIT.md`. No provider, production,
  Browser, outbound, or Phone Farm action occurred.

Next: implement replayable deterministic cross-issue consequence receipts,
versioned regression contribution history, and graph/knowledge-hub drift checks
before expanding agent bundles or deciding permanent seats.

## 2026-09-13 - Item 14 deterministic responsibility safeguards accepted locally

- Built immutable cross-issue consequence receipts, a versioned permanent
  regression registry, and a stewardship manifest/checker for the system graph,
  role catalog, and maintained Silver Mediums knowledge sources.
- Final independent review returned REVISE with four P1 findings and no P0:
  unauthenticated consequence creation, caller-under-scoped regression lookup,
  rejected regression evidence lost on replay, and an incomplete two-file
  stewardship source set.
- Applied exactly one consolidated correction. Consequence receipts now require
  trusted controller authority and hash-bound current durable snapshots.
  Regression admission requires an exact controller-bound full-journey scope.
  Rejected stale/conflicting attempts and findings are immutable, chained,
  restart-replayable, idempotent, and tamper-detecting. Stewardship now declares
  a complete authority profile and derives every registered project's context
  and changelog from the bound registry.
- Proof passed:
  `go test ./internal/systemgraph/... ./cmd/system-stewardship-check -count=1`,
  `go vet ./internal/systemgraph/... ./cmd/system-stewardship-check`, targeted
  diff checks, and the real checker against `C:\Dev\Silver-Mediums-Knowledge`.
  The checker returned `valid: true`, the expected graph and role hashes, and
  `source_count: 20`.
- These are local provider-free libraries and an executable checker. They are
  not database-backed or connected to live dispatch, tools, credentials, or
  production. No provider, Browser, outbound, production, or Phone Farm action
  occurred. No model, role, squad, or permanent seat was selected.

Next: build exact instruction and skill bundles only for responsibilities that
survived the placement audit, then validate them provider-free before any
additional qualification or runtime wiring.

## 2026-09-13 - Item 14 remaining role artifact set accepted provider-free

- Created seven candidate-only instruction/skill pairs for the remaining
  LLM-executed responsibility boundaries: Phone Farm Application, Phone Fleet
  Reliability, Data Sync, Release/Runtime, Company Auditor, Architecture
  Assurance, and Security/Privacy. Preserved the four immutable v1 pairs.
- Kept deterministic workflow/documentation responsibilities and nine
  accountable-outcome identities out of the paid-agent seat list. Retained Mac
  alert analysis as one skill-only candidate with no monitoring, suppression,
  acknowledgement, host access, or action authority.
- Corrected the machine catalog so Content Inspiration and Telnyx use separate
  configuration-only identities and disjoint capabilities/skills. Added a
  regression test proving neither identity inherits the other's capability.
- Final independent review found no P0 and two P1 completeness concerns: an
  unlisted embedded file could escape the v2 manifest, and future
  configuration binding was ambiguous. One consolidated correction now rejects
  every unmanifested file and records `deferred_fail_closed` configuration
  binding. Older provisional model-labelled decisions do not activate v2.
- The accepted v2 manifest hash-binds 16 files, seven role pairs, and one
  skill-only artifact. Full agentqualification/systemgraph/checker tests pass;
  vet and the real 20-source stewardship check pass.
- No provider, production, Browser, outbound, credential, database, dispatch,
  or Phone Farm action occurred. No model, role configuration, squad, or
  permanent seat was selected.

Next: connect exact configuration and capability enforcement to dispatch,
claim, tool, skill, environment, and credential paths, then prove denied
configurations cannot act. Do not bind historical provisional model labels as
selections.

## 2026-09-13 - Item 14 live enforcement seams mapped

- Mapped every main task creation path and the shared single/batch claim
  response builder. The claim builder is the last common fail-closed boundary
  before plugin tools, MCP connections, mutable agent instructions, custom
  environment, skills, and later task credentials are exposed.
- Confirmed the v2 role artifact set is model-neutral and that the older
  accepted-configuration file remains historical comparison evidence, not a
  live v2 binding authority.
- Fixed the next implementation boundary: opt-in guarded envelopes only;
  envelope absence preserves legacy behavior; guarded v2 work rejects while no
  qualified runtime binding exists; claim and later broker checks must repeat
  validation; consequential raw-shell work remains denied until an actual
  command interceptor exists.
- Provider calls, production, Browser, outbound communication, credentials,
  and Phone Farm were not touched. Implementation is underway and the next
  takeover should inspect the newest focused test receipts before repeating
  this mapping.

## 2026-09-13 - Item 14 guarded dispatch and claim checkpoint

- Added a model-neutral v2 role-artifact binding catalog. It binds the seven
  candidate role packages to exact paths and hashes without containing or
  resolving any model, provider, effort, runtime, or production authority.
- Added a strict opt-in guarded-task envelope. Artifact identity and future
  runtime/model qualification are separate receipts. No such runtime receipt
  is qualified yet, so guarded work deterministically rejects as
  `guarded_configuration_unbound`; historical model-labelled decisions are not
  consulted.
- Wired guarded validation before correction, independent-review, and release-
  verification task insertion. Wired the same validation at the very start of
  the shared single/batch claim builder, before agent instructions, plugin
  tools, MCP, skills, custom environment, runtime configuration, or later task
  credentials can be exposed. Invalid claims are terminally settled to prevent
  reclaim loops. Contexts without the reserved key retain legacy behavior.
- Proof passed: `go test ./internal/agentqualification
  ./internal/workflowaction ./internal/handler -run
  'GuardedDispatch|GuardedTask|BuildClaimedTaskResponse' -count=1` and
  `go vet ./internal/agentqualification ./internal/workflowaction
  ./internal/handler`.
- The one independent review returned three P1 findings and no P0: no typed
  producer boundary, missing actual execution-identity comparison, and Observe
  paths that could accept a guarded task claim would later reject. The single
  consolidated correction added the typed attachment boundary, exact
  workspace/issue/agent/runtime binding in envelope, qualification receipt,
  Apply, Observe, and claim, plus adversarial substitution coverage. No second
  reviewer loop was run.
- This is not the completed runtime gate. No runtime/model configuration is
  qualified or automatically stamped, and exact enforcement still must reach
  brokered tool calls, lazy skill loading, environment and credential purpose,
  and the documented raw-shell boundary. No provider, Browser, production,
  outbound, credential, or Phone Farm action occurred.
- Post-correction combined proof passed across `agentqualification`,
  `workflowaction`, `handler`, `systemgraph`, and the stewardship checker,
  followed by vet. The executable checker validated all 20 maintained hub
  sources with graph hash `ff4119b1d70ced0a943f8dd056a1f3851e24c975ebacfad37dfc76d9aa75d1fa`
  and role hash `c89c5f619c5f1a724e255b9014bfc882295a2c00504191bfc48569a5bbcc63e2`.

## 2026-09-13 - Item 14 guarded Remote MCP and plugin gateway checkpoint

- Added the shared `TaskCapabilityScopeV1` last-mile validator and guarded
  Remote MCP/plugin-hook integrations. Nil guards preserve legacy behavior.
- Remote MCP validates before startup credential resolution/discovery and
  before per-call credential refresh/upstream contact. Plugin hooks validate
  before the signed invoker using independent task/runtime facts and an exact
  controller-mapped tool destination. Consequential raw shell remains denied.
- Final independent review: REVISE, one P1, no P0. The initial plugin path
  copied actual identity/targets from its receipt. One consolidated correction
  separated gateway facts from authorization and added identity/destination
  drift tests. No further review loop was run.
- Proof passed: `go test ./internal/agentqualification ./internal/daemon -run
  'TaskCapability|RemoteMCP|PluginHook' -count=1`; `go vet
  ./internal/agentqualification ./internal/daemon`; and scoped `git diff
  --check`.
- This does not claim live guarded execution. Positive guard plumbing,
  lazy-skill checks, and environment/credential-purpose exposure remain. No
  provider, Browser, production, outbound, credential, or Phone Farm action
  occurred.
- A broader package run kept `agentqualification`, `workflowaction`, `handler`,
  `systemgraph`, and the stewardship command green, but the unfiltered daemon
  package retained unrelated machine/environment-sensitive baseline failures
  involving CLI self-heal, local skill/MCP discovery, Windows symlinks and
  worktree fixtures. The focused changed-surface daemon tests remain green;
  these broader failures are not claimed as resolved or caused by this slice.
- The real 20-source stewardship check passed after documentation rebinding:
  `valid:true`, graph SHA
  `ff4119b1d70ced0a943f8dd056a1f3851e24c975ebacfad37dfc76d9aa75d1fa`,
  role SHA
  `c89c5f619c5f1a724e255b9014bfc882295a2c00504191bfc48569a5bbcc63e2`.

## 2026-09-13 - Item 14 guarded execution claim-wire checkpoint

- Added a versioned `GuardedExecutionPackageV1`, separate from issue prose and
  `WorkflowPacket`. It exact-binds the qualified runtime receipt ID to the
  capability scope and validates independently resolved task, runtime,
  provider, model, effort, host, role, configuration, and plugin-target facts.
- Added optional mirrored `guarded_execution` fields to the server claim and
  daemon Task types. They are not populated from task context and remain empty
  until authenticated durable loaders and a qualification decision exist.
- Final independent review: REVISE, one P1, no P0. Ordinary typed JSON decode
  would discard unknown future authority fields. One consolidated correction
  added strict Task-wire extraction/decoding and rejects present null,
  duplicate, unknown, malformed, or trailing guarded packages while absent
  remains legacy. No second review loop was run.
- Proof passed: focused `GuardedExecutionPackage`, `TaskCapability`,
  `GuardedDispatch`, handler claim, daemon wire, Remote MCP, and PluginHook
  tests; vet across `agentqualification`, `handler`, and `daemon`; scoped diff
  checks.
- No provider, Browser, production, outbound, credential, model selection, or
  Phone Farm action occurred. Next is durable controller-owned receipt and
  capability loading before daemon pre-exposure and skill/environment guards.

## 2026-09-13 - Item 14 durable permission-receipt storage accepted

- Added migrations 544-553 for qualified-runtime receipts, task capability
  grants, and separate revocations. The tables remain relationship-free; every
  index is a separate concurrent migration. Receipt/grant payloads are
  canonical-hash bound to typed scope columns.
- Added the controller-only `PermissionReceiptStore`. It stores exact receipts,
  rejects conflicting duplicate IDs and multiple grants for one task, loads
  only exact unexpired/unrevoked scope, and reconstructs a guarded execution
  package only after strict payload, digest, identity, artifact, and capability
  validation.
- Final independent review returned two High findings and one Medium gap: an
  UPDATE-only trigger allowed deletion/unrevocation, revocation could target a
  nonexistent or wrong-workspace ID, and proof used only an in-memory querier.
  The single consolidated correction blocks UPDATE and DELETE, requires and
  validates the exact same-controller target, compares exact tombstones on
  replay, and adds isolated real-PostgreSQL coverage. No second review loop was
  run.
- Proof passed: full `go test ./internal/agentqualification -count=1`; focused
  guarded-permission migration tests; generated-query compilation; `go vet`
  for qualification, migrations, and generated queries; scoped diff checks;
  and `TestPermissionReceiptStorePostgresContracts` against a disposable
  PostgreSQL 17 container. The database test covered real concurrent indexes,
  primary keys, duplicate and one-task uniqueness, UPDATE/DELETE denial,
  wrong-workspace denial, two-connection concurrent revocation, and exclusion
  of revoked authority. The disposable container was removed afterward.
- Go's race detector could not run because the Windows Go build has CGO
  disabled. Deterministic concurrency tests and the real two-connection
  PostgreSQL test passed; no broader race-proof claim is made.
- Unverified and intentionally not claimed: no positive runtime/model
  qualification exists; no claim producer attaches the durable package; no
  guarded daemon execution, skill/environment/credential filtering, native
  shell interception, provider call, Browser action, production action,
  outbound message, credential access, model selection, or Phone Farm action
  occurred.
- Exact continuation: wire the durable loader into the pre-exposure claim path,
  recheck revocation at final claim, then enforce skill identity/hash and
  custom-environment/credential-purpose limits before any exposure. Keep
  consequential native shell/file access denied pending an equivalent
  interceptor or process sandbox.

## 2026-09-13 - Item 14 guarded pre-exposure protection accepted

- Wired controller-owned durable runtime and task-capability receipt loading
  into the shared single/batch claim builder before payload construction.
- Guarded payloads use exact embedded, hash-bound instructions plus one inline
  skill. Mutable/lazy skills and custom env/args, direct MCP, runtime config,
  connected apps, Remote MCP, and plugin hooks cannot enter the claim.
- Authority is reloaded immediately before claim finalization and inside the
  database transaction before task tokens, daemon tokens, or comment receipts.
- The daemon validates the package and independent host/runtime facts before
  repository registration, then intentionally stops at
  `guarded_environment_not_isolated`. Existing providers still inherit the
  daemon HOME/profile and native tools, so no unsafe positive path was opened.
- One independent review found five gaps; one consolidated correction addressed
  them without a reviewer loop.
- Proof passed: focused guarded/host/legacy/authority tests across qualification,
  daemon, handler, and service packages; vet for those packages; and scoped
  diff checks (line-ending warnings only).
- Complete: authenticated claim construction, exact role artifacts, unbound
  exposure denial, guarded lazy-skill denial, transactional revocation check,
  and pre-repository daemon stop. Unverified/not implemented: an isolated child
  environment, a tool-free or sandboxed provider lane, positive guarded
  execution, and guarded broker credentials.
- Owner-reported usage is 3%, below the 4% start threshold. No further slice
  was begun. Exact next slice: isolated allowlisted child environment plus a
  tool-free execution boundary. No provider, Browser, production, outbound,
  credential, model-selection, or Phone Farm action occurred.

## 2026-09-13 - Item 14 isolated child boundary implementation checkpoint

- Bound future qualified-runtime receipts to the exact executable bytes, host
  OS, environment-name allowlist, tool-free adapter, and daemon-parent-only
  host-attestation custody. No receipt or production grant was issued.
- Moved the final guarded host/executable check ahead of repository
  registration. Guarded execution rejects custom runtime commands and skips
  model-catalog probing; the package remains the only model/effort input and
  `service_tier` is forced empty.
- Added a fresh task-local child environment with isolated HOME, XDG,
  provider-config, and temp roots. It does not inherit `os.Environ`, does not
  read/export the task token, and has no `MULTICA_*`, host-attestation, provider
  credential, custom-env, or direct-MCP value.
- Added preventive tool-free adapters for Claude and OpenCode plus fail-closed
  detection if either provider nevertheless emits a tool event. This enables a
  model-only lane; it is not an OS filesystem sandbox and grants no brokered
  tools or credentials.
- Focused local tests pass in `pkg/agent`, `internal/agentqualification`,
  `internal/daemon`, and `internal/handler`, including a spawned fake-child
  readback proving parent environment secrets are absent. Broader verification
  and the required one independent review are next.
- No model test, provider call, model/role selection, credential read,
  Browser/Orca action, production/outbound action, or Phone Farm access
  occurred.

## 2026-09-13 - Item 14 isolated child boundary accepted

- One independent read-only review completed with no P0, three P1 findings,
  and one P2 proof gap: the new child boundary had been added to a v1 runtime
  receipt, executable pathname hashing retained a check/use gap, observed tool
  events did not immediately stop the child, and the environment test had not
  crossed both actual adapter launch paths.
- One consolidated correction addressed the complete review once. Child-bound
  qualification now requires `silver-mediums.qualified-runtime-binding.v2`;
  stored or presented v1 receipts cannot masquerade as child-bound evidence.
  The daemon copies from an opened installed executable while hashing, verifies
  the task-owned read-only staged copy, uses only that staged pathname, and
  rechecks its receipt-bound digest immediately before launch.
- Claude and OpenCode now hard-kill their owned process tree on the first
  observed tool event and force a failed, empty-output result. Cross-platform
  fake children launched through both real `Execute` paths prove the exact
  child environment/cwd/argv, no inherited host attestation or parent secret,
  and no delayed second action after either provider's tool event.
- Evidence boundary: this is an isolated writable profile/environment and a
  provider-configured model-only lane, not a general OS syscall, filesystem, or
  network sandbox. Tool prevention is primary; event detection is a drift trip
  wire. No native shell/file access, brokered tool, MCP connection, credential,
  task token, or host-attestation value is exposed to the child.
- Post-correction proof passed: full `go test ./internal/agentqualification
  -count=1`; full `go test ./internal/handler -count=1`; focused guarded tests
  in `pkg/agent` and `internal/daemon`; focused guarded migration tests; `go
  vet ./internal/agentqualification ./internal/daemon ./internal/handler
  ./internal/service ./pkg/agent`; and scoped diff checks.
- Broad baseline remains explicit: unfiltered daemon and agent packages still
  fail unrelated Windows/machine/ambient fixtures (installed CLI self-heal,
  persistent identity, workdir locking, executable-extension assumptions, and
  local skill/MCP discovery). The service package still fails the same four
  ambient built-in-skill conformance tests. Focused changed-surface tests are
  green; these baseline failures were neither caused nor fixed by this slice.
- The requested next slice is complete locally. Positive guarded execution is
  still not authorized or available because no controller-issued v2 runtime
  receipt exists. No model test, provider call, model/role selection,
  credential read, Browser/Orca action, production/outbound action, or Phone
  Farm access occurred. No second review loop ran.
- Updated the Silver Mediums hub and rebound its current-state source receipt.
  `check_hub.py` passes while retaining its dated snapshot/stale-claim warnings;
  the real 20-source stewardship command returns `valid:true` with unchanged
  graph SHA `ff4119b1d70ced0a943f8dd056a1f3851e24c975ebacfad37dfc76d9aa75d1fa`
  and role SHA
  `c89c5f619c5f1a724e255b9014bfc882295a2c00504191bfc48569a5bbcc63e2`.

## 2026-09-13 - Item 14 controller authority lifecycle started

- Preserved the existing dirty branch and accepted Items 1-13 plus the
  completed isolated-child boundary. No prior model/role evaluation was
  reopened.
- The next failure-closed boundary is durable one-shot execution consumption,
  atomically coupled to the real task start transition. Claim delivery alone
  will not consume authority; a second launch cannot reuse it.
- The planned provider-free proof uses a locally compiled fake provider only,
  but crosses controller issuance, authenticated claim, daemon executable
  staging, and the real tool-free adapter. It will not inspect credentials or
  contact a provider.
- A separately bounded real model-only shadow remains unstarted because the
  current authority explicitly prohibits provider calls, credential access,
  spending, and role/model selection.

## 2026-09-13 - Item 14 provider-free authority proof checkpoint

- Added controller-owned combined issuance and immutable launch consumption
  for the exact `qualified-runtime-binding.v2` package. Exact issuance retries
  converge; conflicting issuance remains non-executable.
- Bound one-shot consumption to task/workspace/issue/executor/runtime, receipt,
  grant, authenticated daemon, canonical package SHA-256, and current time.
  It is appended in the same transaction that moves the task from dispatched
  to running, so denial rolls the transition back.
- Added an exact-package presentation check at the HTTP start boundary. The
  daemon hashes the precise guarded package it is about to launch; the server
  independently reloads and hashes controller-owned state. Wrong digest and
  wrong daemon fail before consumption or child execution.
- A disposable PostgreSQL 17 database was migrated through 557. The real DB
  contract passed for duplicate issuance, wrong digest, wrong daemon, exact
  one-shot consumption, replay denial, append-only rows, and scope-safe
  revocation. The container was removed afterward.
- A provider-free fake executable crossed the real authenticated claim/start
  handlers, daemon client, preparation lease, executable staging, and real
  tool-free Claude adapter. The approved package launched once and returned
  `provider-free-ok`. Altered package, wrong presented digest, revoked runtime
  qualification, expired runtime authority, duplicate launch, and replay all
  produced zero additional fixture launches and no second consumption.
- Full `internal/agentqualification` and `internal/migrations` tests pass. A
  broad `pkg/agent` run retained unrelated existing Windows/ambient failures
  (executable fixture suffixes, shell-specific repair text, owned-process timing,
  and missing ACP fixture executables); no guarded changed-surface failure is
  reported from that broad lane. The single independent review is now running.
- No provider call, credential access, tool enablement, role/model selection,
  Orca/Browser, production/outbound action, or Phone Farm access occurred.
  Real model-only execution remains separately blocked on explicit provider,
  credential/subscription or spend, and role/model authority.

## 2026-09-13 - Item 14 controller authority lifecycle accepted

- The required independent read-only review returned REVISE with no P0, two
  P1 findings, and one P2 proof gap. A guarded task could start after a restart
  that lost controller configuration, the consumed package digest was not
  rechecked at the last child-launch boundary, and the shadow did not send a
  semantically valid mutation through `runTask` or prove expired authority
  left zero consumption.
- Applied one consolidated correction. Guarded claim/start hooks are now
  always installed and preserve legacy tasks while denying guarded work when
  controller identity is unavailable. The daemon recomputes the canonical
  package digest immediately before `executeAndDrain` and compares it with the
  digest consumed at start. No second review loop was run.
- Expanded the full-path shadow with a simulated controller-config-loss
  restart, a self-consistent added evidence receipt that passes semantic host
  validation but fails the controller digest, and an explicit zero-consumption
  check for expiry. Runtime qualification revocation is exercised directly.
- Final disposable PostgreSQL 17 proof passed after migration through 557:
  `TestPermissionReceiptStorePostgresContracts` and
  `TestGuardedAuthorityFakeProviderShadow`. One exact package yielded one
  append-only consumption and one fake child result, `provider-free-ok`.
  Missing controller, semantic alteration, coherent package drift, wrong
  digest, wrong daemon, expiry, runtime revocation, duplicate start, and replay
  yielded no unauthorized child launch. The container was removed.
- Final local gates passed: full `go test ./internal/agentqualification
  -count=1`; full `go test ./internal/handler -count=1`; focused guarded
  migration, service, daemon, and adapter tests; `go vet` across qualification,
  daemon, handler, service, and agent packages; and scoped diff checks. The
  unrelated broad agent-package Windows/ambient baseline remains recorded and
  unchanged.
- Evidence limit: the test crosses real queue claim, start transaction, daemon
  client/preparation/staging, and tool-free adapter with an already-
  authenticated daemon context and a local no-network executable. It does not
  retest token authentication, call a provider, read a credential, grant a
  tool, select a role/model, use Orca/Browser, touch production/outbound state,
  or access Phone Farm.
- The smallest real model-only shadow was not attempted. It requires new
  provider/credential/subscription or spending authority and a model/role
  choice, which the current instruction explicitly withholds.
- Updated the Silver Mediums hub and rebound the `CURRENT_STATE.md` receipt to
  `b37a44a438b0edbc6c15cc701059de0aab043fd56f7416cf7c29caa82cfb90d6`.
  `check_hub.py` passes with its existing stale-claim/dated-snapshot warnings;
  the real 20-source stewardship check returns `valid:true` with unchanged
  graph SHA `ff4119b1d70ced0a943f8dd056a1f3851e24c975ebacfad37dfc76d9aa75d1fa`
  and role SHA
  `c89c5f619c5f1a724e255b9014bfc882295a2c00504191bfc48569a5bbcc63e2`.

## 2026-09-13 - Item 14 real model-only shadow authority proposal

- Preserved the accepted provider-free chain and did not reopen qualification,
  choose a production role/model, or issue execution authority.
- Selected the already-supported guarded Claude Code subscription adapter as
  the narrowest proposed route. The exact `claude-sonnet-4-6` Medium values are
  temporary test fixtures based on the prior proven subscription transport,
  not a promotion or production choice.
- Current official Claude Code authentication mechanics and local source agree
  on the blocking detail: Windows subscription credentials live in the normal
  user profile, while the guarded child points `CLAUDE_CONFIG_DIR` at a new
  empty task-local profile. The child therefore has no subscription credential.
- Froze a one-use credential-capsule proposal. After controller consumption,
  the daemon parent would byte-copy only
  `C:\Users\a9sho\.claude\.credentials.json` into the task-local Claude profile
  without parsing, displaying, logging, hashing, or persisting secret content;
  the copy must be removed and proven absent after the one process. The source
  credential remains untouched.
- Rejected API-key billing, a newly minted one-year setup token, interactive
  login, and exposing the whole normal Claude profile as broader than necessary.
- Found one required provider-free precursor: guarded execution currently
  builds the ordinary task prompt, which instructs a normal agent to use the
  Multica CLI. The real shadow needs a controller-bound fixed guarded prompt
  whose final rendered SHA-256 is rechecked at the last launch boundary.
- Documented the temporary executor, existing immutable role artifact, fixed
  harmless prompt, ten-minute one-shot lifetime, zero-retry rule, pass/fail
  conditions, cleanup requirement, and exact owner grant in
  `ITEM-14-REAL-MODEL-SHADOW-AUTHORITY.md`.
- No credential file was opened or tested for existence, no Claude/runtime
  process was started, no provider was contacted, no spend/subscription usage
  occurred, and no receipt/grant was issued. No tools, repository, production,
  outbound, Browser/Orca, or Phone Farm surface was used.
- The next action is intentionally paused for owner authority covering exactly
  one read-only credential byte-copy, one temporary child-lifetime copy, one
  Claude subscription inference with zero retries, and deletion/absence proof.
  The credential-capsule and prompt-binding code must pass a fake-only preflight
  before that call. Claude's upstream/fork-maintenance audit stays separate and
  remains required before migration.
- Updated the Silver Mediums hub with this locked decision and rebound its
  `CURRENT_STATE.md` stewardship receipt to
  `af1555f3e7d1e06f08437ab347c869db066fb3f578e4276d650f23fd51cc6a32`.
- Validation passed: `python scripts/check_hub.py` returned OK with the same
  stale-claim and dated-snapshot warning classes; system-graph and stewardship
  command tests passed; the real 20-source command returned `valid:true` with
  graph SHA
  `ff4119b1d70ced0a943f8dd056a1f3851e24c975ebacfad37dfc76d9aa75d1fa`
  and role SHA
  `c89c5f619c5f1a724e255b9014bfc882295a2c00504191bfc48569a5bbcc63e2`;
  scoped documentation/manifest diff checks had no errors.

## 2026-09-13 - Item 14 authorized real-shadow fake precursor

- Recorded the owner's exact bounded authority from
  `ITEM-14-REAL-MODEL-SHADOW-AUTHORITY.md`; it covers one opaque copy of the
  existing Claude credential into a fresh profile, one
  `claude-sonnet-4-6` Medium subscription inference, zero retries, and immediate
  deletion/absence proof only.
- Added controller-bound `task-capability-scope.v2` model-only authority. It
  hashes the fixed payload and exact final rendering of the immutable Data Sync
  role artifacts plus that payload, and fixes one call, one turn, zero retries,
  exact credential path/purpose/copy policy, and immediate verified cleanup.
  V1 scopes reject model-only authority; V2 scopes require it.
- Added a credential capsule that opens the source exactly once through the
  daemon parent, copies bytes without decoding/hashing/logging them, creates
  only task-local `.credentials.json`, deletes immediately after provider exit,
  and makes verified absence a terminal success condition.
- Focused unit tests pass for exact prompt rendering, altered prompt/hash/path/
  purpose/policy/call/retry denial, one fake credential-source open, opaque copy,
  and idempotent verified cleanup.
- The real dispatch/claim/start/daemon fake shadow passes against a disposable
  schema in the existing local development PostgreSQL. One exact fake package
  launched once with zero tools, one consumption, and no remaining credential
  copy. Altered model, prompt, task identity, executable, credential source,
  package digest, revoked/expired authority, duplicate start, and replay all
  produced no extra launch. The exact-name loopback database proxy was removed.
- A provider-free local audit found the Windows Claude npm entry is a relative
  shim. Guarded resolution now selects, hashes, and stages the bundled native
  `claude.exe`; its focused test and the build-tagged integration compile gate
  pass.
- No real credential path was opened, tested for existence, read, hashed, or
  logged. No Claude process or provider call ran. One independent read-only
  review is running before the sole authorized call.

## 2026-09-13 - Item 14 real-shadow independent review correction

- The one independent review returned REVISE with no P0 and four P1 findings:
  ambient database selection was not fail-closed; guarded Claude could
  auto-approve a control request; requested model identity was being mistaken
  for provider-observed identity; and one CLI process did not alone disable
  Claude's internal failed-request retries. It also identified a P2 risk from
  credential-bearing diagnostics reaching ordinary logs/transcripts.
- Applied one consolidated correction with no second review loop. The database
  helper now requires an explicit loopback-only Item 14 DSN. Guarded Claude
  kills any control request without replying, suppresses stderr plus live
  transcript/log messages, and returns generic failure text rather than child
  diagnostics. Provider-reported model identities and `num_turns` are preserved
  separately from requested configuration.
- Added the official Claude Code `CLAUDE_CODE_MAX_RETRIES=0` control to the
  exact environment allowlist and backend validation. Anthropic documents it
  as the number of retries for failed API requests; zero therefore makes the
  one process/one turn a one-attempt transport. The installed Claude Code
  2.1.270 native binary contains the control.
- Focused guarded tests pass, including control-request-first process kill and
  missing/nonzero retry-control denial. The corrected full fake database path
  again produced one exact launch, one provider-reported fake model, one turn,
  zero tools, one consumption, and verified credential-copy absence.
- The exact real harness then passed a prepare-only run: native executable
  binding, controller issuance, and real claim completed, package SHA-256 was
  recorded, and the harness returned before `runTask` with
  `credential_opened=false` and `provider_started=false`. The real credential
  remains unopened and no Claude/provider call has occurred.

## 2026-09-13 - Item 14 single authorized real shadow terminal result

- Ran the build-tagged real shadow exactly once with the owner-authorized
  credential source, `claude-sonnet-4-6` Medium fixture, fixed harmless prompt,
  one process, one turn, and `CLAUDE_CODE_MAX_RETRIES=0`. No retry or second
  controller/CLI invocation was made. The later two-model receipt prevents
  certifying that this single CLI process made only one provider inference.
- The real queue path claimed and started the task and the guarded Claude child
  exited after six seconds. No tool event occurred and the provider reported
  `num_turns=1`.
- The provider terminal receipt reported two model identities:
  `claude-haiku-4-5-20251001` and `claude-sonnet-4-6`. The approved package
  allowed only the latter, so the exact provider-model assertion failed closed.
  This is a terminal boundary failure, not a model-quality result.
- The failure occurred before output-contract and usage acceptance. Those
  conditions are therefore unverified rather than passed. No task completion
  was recorded from the rejected provider result.
- The credential capsule cleanup completed before the assertion. The exact
  task-local credential copy, its temporary test root, and the exact-name
  loopback proxy were independently checked and were absent. The original
  credential content was never displayed, parsed, hashed, logged, or changed.
- No role or model was promoted or permanently selected. Complete local shadow
  workflows remain gated because the positive boundary proof did not pass. Any
  further real call requires a new owner grant and a separately bounded design;
  Claude upstream/fork maintenance remains a separate pre-migration audit.
- Final provider-free checks passed: qualification, guarded agent/daemon,
  integration compile-only, focused vet, hub validation, system-graph and
  stewardship tests, and the real 20-source validation. The hub
  `CURRENT_STATE.md` receipt is
  `a2dda25ddb3aaf420626d33fde6276d1b7c0c985aefd7e9a5aea91d01782a9cc`.

## 2026-09-14 - Item 14 exact-model transport v1 provider-free candidate

- Preserved the sole real Claude Code observation as terminal fail-closed
  evidence and corrected its authority record so no stale sentence can be read
  as permission to retry it.
- Official configuration and the observed multi-model receipt establish the
  boundary mismatch: Claude Code's selected session model does not prove all
  internal requests use that model, and terminal model usage is post-hoc. A
  same-model Haiku override would still not prove only one provider request.
- Implemented a versioned direct Anthropic Messages candidate policy and a
  single-use daemon harness. The request gate validates one exact endpoint,
  model, prompt hash, body shape, output ceiling, timeout, API version, and
  credential presence before one fake-provider `RoundTrip`. The POST body is
  non-replayable, redirects are denied, tools are absent, and the strict
  response gate requires the same model, positive usage, text-only end-turn
  content, and zero server-tool-use metadata.
- Fake credential/provider tests pass for the exact request and deny altered
  policy/prompt/model, sequential and concurrent duplicates, transport failure,
  redirects, tool content, server-tool-use metadata, and unknown response
  fields without a second fake-provider request.
- The sole independent review returned REVISE with no P0, one P1 stale-
  authority defect, and three P2 proof limitations. One consolidated correction
  resolved the stale grant, redirect, and response-decoding issues and recorded
  that an arbitrary injected `RoundTripper` is fake-only evidence. Any real
  wiring must use a fresh pinned standard transport under new authority.
- No credential path was inspected, no socket or provider CLI was opened, and
  no provider, role/model selection, upstream audit, production/outbound, or
  Phone Farm surface was contacted.

## 2026-09-14 - Item 14 exact-model transport v1 final verification

- Focused qualification tests passed once. The guarded exact-model daemon
  suite passed 20 repetitions, including concurrent duplicate execution. The
  adjacent guarded credential, child, authority, and agent regressions passed;
  the `agentintegration` package compiled without running an integration test;
  and focused vet passed.
- `go test -race` could not start because this Windows Go environment has CGO
  disabled. No race-detector result is claimed. The concurrent single-use test
  was retained and repeated 20 times as the available focused concurrency gate.
- The Silver Mediums hub validator passed with its existing stale-claim
  warnings. Concurrent CRM and Viral Queue changelog edits changed two
  stewardship-source hashes; after the hub validator accepted the current
  source set, only their receipts were rebound. No changelog content was edited
  by this Item 14 slice.
- The real 20-source stewardship validation then passed with graph SHA-256
  `ff4119b1d70ced0a943f8dd056a1f3851e24c975ebacfad37dfc76d9aa75d1fa`.
  The bound hub `CURRENT_STATE.md` SHA-256 is
  `16ae1ab2afab0ee6d53ef69e376d2214c6d095412ac394e21d29478543d40744`.
- This slice stops before any credential inspection, real transport
  construction, dispatch activation, or provider call. Those actions require
  a new separately bounded owner grant.

## 2026-09-14 - Item 14 direct Messages real-proof preflight

- The owner granted one direct metered `claude-sonnet-4-6` fixture request,
  capped at `$1`, with zero retries, redirects, fallbacks, proxies, tools,
  repository/production context, or outbound actions. This does not select a
  final model or role.
- Froze `ITEM-14-DIRECT-MESSAGES-REAL-PROOF-AUTHORITY.md` before credential
  access. The one eligible credential mechanism is the inherited
  `ANTHROPIC_API_KEY`; prepare-only cannot read it and no `.env` or alternate
  source is searched.
- Official 2026-09-14 list prices are `$3`/million input and `$15`/million
  output tokens. The tightened 64-output-token request reserves 100,000 input
  tokens, for a 300,960-microdollar worst-case policy reservation below the
  1,000,000-microdollar authority ceiling. It requests standard-only/global
  serving and no caching or fast mode.
- Added a fresh no-proxy/no-custom-retry Go transport with disabled redirects,
  HTTP/2 forcing, keep-alive reuse, compression, and body replay. The executor
  remains consumed before credential access.
- The exact fake suite passed 20 repetitions after tightening, including its
  concurrent duplicate case and cost receipt. The build-tagged prepare-only
  harness passed with zero requests, no credential open, and no provider
  start. One provider-free independent review is in progress before the sole
  real attempt.

## 2026-09-14 - Item 14 direct Messages pre-call review correction

- The one independent provider-free review returned REVISE. Its P0 concern
  compared the harness against the exhausted 2026-09-13 subscription-file
  authority; the owner's current message separately and explicitly authorizes
  the inherited Anthropic API credential and one metered direct request. The
  new authority record remains exact and was not expanded.
- Applied the two applicable P1 findings in one consolidated pass. The strict
  decoder now recognizes current documented nullable container/stop details,
  cache details, citations, and nested thinking-token metadata, while failing
  closed on any unexpected container, stop detail, citation, cache/tool use,
  or impossible thinking count.
- Temporary credential buffers are now wiped on every success and error path;
  fake tests cover success, malformed input, read failure, and close failure.
- The corrected exact-model suite passed 20 repetitions; prepare-only again
  recorded zero requests, no credential open, and no provider start; focused
  vet passed. The single real attempt is eligible and has not yet started.

## 2026-09-14 - Item 14 sole direct Messages attempt failed before provider

- Invoked the frozen real-run harness exactly once. It could not obtain the
  inherited `ANTHROPIC_API_KEY` from the approved process-environment mechanism
  and failed before constructing or sending an HTTP request.
- Terminal receipt: requested fixture `claude-sonnet-4-6`; provider model
  unobserved; zero requests/retries/tokens/cost; 0 ms harness timing; result
  `approved credential mechanism is unavailable`.
- No credential bytes were obtained and no credential artifact was created or
  retained. The emitted cleanup flag is honestly preserved as false because
  the expected source open/close lifecycle never began. No secret value or
  derivative was displayed, logged, hashed, preserved, or documented.
- This is a valid fail-closed credential-admission observation, not a real
  provider/model proof. The authority is exhausted. No `.env` loading,
  alternate source, credential substitution, or retry is authorized.

## 2026-09-14 - Item 14 complete local shadow suite v1

- Preserved the sole direct Messages attempt as terminal zero-request
  fail-closed evidence and made no credential or provider access.
- Froze four overlapping local journeys covering intake, implementation,
  independent review, release truth, recovery, Inbox, Chat, mobile, and
  notifications. The manifest SHA-256 is
  `9c8803d41b12b99fa7286ecd379d82ca3f2497bae8f181f1d4c524e8912e32be`.
- Bound 12 evidence contracts and the exact hashes of every supporting test
  harness. Controller state, local code, remote Git, deployed revision,
  observed runtime, and business outcome remain separate evidence lanes.
- Added a model-neutral shadow reference for accepted frozen v1/v2 role
  artifacts. It cannot carry runtime/model/provider/credential/tool authority
  and is not accepted by guarded dispatch. No role or model was qualified,
  promoted, or selected.
- One independent review returned REVISE. Its material findings were that nine
  label-only cases did not make complete workflows, v1 artifacts needed a
  shadow-only bridge, direct Chat mutation denial lacked handler proof, and
  notifications lacked admission-to-dry-run deduplication proof. It also
  clarified one automatic correction/two review attempts and preserved Item
  13 external custody as `secondary_copy_needed`.
- Applied one consolidated correction with no second review: four complete
  journeys, exact harness bindings, Chat endpoint mutation guards, a Current
  Picture notification admission/outbox/dry-run/deduplication test, expanded
  manifest/receipt mutation denial, and explicit zero Phone Farm authority.
- A fake run produced exactly 22 ordered receipts with unique replay keys and
  zero provider, credential, tool, repository, production, outbound, and Phone
  Farm counters. The focused suite passed 20 repetitions.
- Created a fresh PostgreSQL 17 container with a test-only identity, migrated
  it through 557, and passed all bound workflow-controller tests plus the new
  Chat and notification guards. Delegated-failure recovery/settlement cases
  passed with verbose evidence. The exact test container was removed and its
  absence verified; its disposable test data is not recoverable.
- Pure workflow-state, command, projection, and notification-policy packages
  passed. Core owner control passed 2 tests, Views owner Inbox passed 3, and
  Mobile schema/display passed 12. Focused Go vet and scoped diff checks pass.
  One earlier root Vitest call used the wrong package setup; package-local
  reruns passed, so no product defect is attributed to it.
- This is local/provider-free contract and application-path evidence only. It
  is not provider, deployment, production runtime, or business acceptance.
  The real exact-model gate remains open under exhausted authority; the
  upstream audit was not started.
- Updated the Silver Mediums hub and rebound the maintained-source receipt.
  The hub validator passes with its existing stale-claim and dated-snapshot
  warnings. The real 20-source check returns `valid:true`, graph SHA
  `ff4119b1d70ced0a943f8dd056a1f3851e24c975ebacfad37dfc76d9aa75d1fa`,
  role SHA
  `c89c5f619c5f1a724e255b9014bfc882295a2c00504191bfc48569a5bbcc63e2`,
  and hub `CURRENT_STATE.md` SHA
  `a0f99f3b2e07dcb7e1ff24d0dd2f0e2f135c7d4fbf354d2863e6cc3781bec5f4`.
  Validation also found a concurrent Viral Queue changelog change; this slice
  did not edit it and rebound only its source hash.
- Security note: one local Docker diagnostic used to investigate database
  isolation emitted existing non-Anthropic application environment values in
  task tool output. No value was reused, copied into code/docs, or used to
  access a service, and no Anthropic credential was obtained. Treat the
  affected local credentials as exposed and rotate them under separate
  credential-change authority.

## 2026-09-14 - Item 14 provider boundary v2 and provider-free OpenRouter precursor

- Recorded the owner's corrected routing rule: Claude/Anthropic may use only
  the Claude Code subscription; OpenAI/Codex may use only the two Codex
  subscriptions; OpenRouter may use only the established small non-Claude v26
  survivor pool inside the existing cumulative `$40` ceiling.
- Preserved the Claude Code two-model receipt as limitation evidence. No retry,
  alias weakening, bypass, credential access, or provider call occurred.
- Added an execution allowlist and a central OpenCode runtime-start gate.
  Claude/Anthropic, OpenAI, unknown, altered, eliminated, and mismatched
  OpenRouter routes fail before executable lookup, credential access, or
  inference. Historical result artifacts remain unchanged.
- Froze the temporary Qwen3 Coder Next/Alibaba fixture in
  `silver-mediums.openrouter-local-shadow-definition.v1`, SHA
  `a6bdf8b55413977cbb4f124872f71b377f86720bc891e5db1db0b2e1b4eb2fba`.
  It binds the four-workflow local suite and is explicitly not a final model,
  role, squad, or production selection.
- Bounded the possible real work to four calls total, one per workflow, zero
  retries/fallbacks, 1,200 generated tokens per call, and a `$1` incremental
  cap under the existing baseline `102,104,668`/absolute `142,104,668`-micro
  `$40` ceiling.
- Provider-free plan and fake commands passed. The fake run contains four
  simulated exact-route responses, zero real provider requests, zero retries,
  and zero tool, repository, production, outbound, Phone Farm, or selection
  actions. Mutation tests reject identity drift, response drift, duplicates,
  replays, retries, tools, and excess spend.
- Prepare-only passed all four plans with zero credential reads and zero
  provider calls. Every resolved config hash is
  `eaac844ec6988b720941fec2cd396cb18ea6586b80bfc5bc309460a28d3c4c4a`.
  Four fresh task-local runtime roots were removed and exact absence verified.
- The real runner is write-once: it creates an exclusive suite root, seals a
  run claim and per-workflow claim before each possible call, never retries,
  and requires actual model plus OpenRouter-selected serving-provider metadata
  before accepting evidence. Raw run/export, usage, cost, timing, response,
  cleanup, and before/after authoritative spend are retained locally.
- Focused agentqualification and screening-run tests pass. One independent
  provider-free pre-call review is in progress before real execution.

## 2026-09-14 - Item 14 OpenRouter review correction and terminal real observation

- Correction to the preceding live checkpoint: the final frozen definition
  SHA is `00a59b865276362a47c9c73777624ef64eb7fda55cf0f44393b1ba88235fce4a`,
  and the four prepare-only resolved configuration hashes are
  `b56f1077f1a76772ab939bbdfa414267a5db98530804706d676c92420f257a29`.
  The enforcement is additive to the Item 14 lane; historical frozen runner
  sources were restored unchanged and were not given new authority.
- One independent provider-free review returned REVISE. It found terminal-only
  assistant inspection, alternate-root replay, post-hoc `$1` enforcement,
  inherited prepare profiles, a fake path outside the real state machine, and
  missing exact eligibility-artifact binding.
- Applied exactly one consolidated correction: full-session assistant/model/
  tool/selected-provider inspection; one fixed global authority claim;
  price-derived pre-call reservation plus sealed post-exposure accounting;
  fresh empty prepare profiles; dependency-injected fake and real execution
  through the same claims/lifecycle; and exact v26 eligibility SHA binding.
  No second review ran.
- The corrected fake lifecycle passed with four simulated responses, zero real
  requests, and every forbidden counter zero. Prepare-only passed all four
  plans with zero credential reads/provider calls and verified removal of all
  four temporary roots. Focused agentqualification and screening-run tests
  passed before the authorized call.
- Invoked the frozen real command exactly once. Workflow
  `local-intake-decision-coordination` made one request and the guard halted the
  suite before workflows 2-4. Requested and observed model were exactly
  `openrouter/qwen/qwen3-coder-next`; the full session had one assistant, zero
  summary/background assistants, one model, and zero tool parts.
- The proof failed closed for two independent evidence reasons. `alibaba` was
  requested with fallbacks disabled, but the sealed OpenRouter/OpenCode export
  omitted selected-serving-provider metadata, so the actual provider remains
  unobserved. The returned JSON also added unapproved fields and failed the
  strict frozen response contract.
- Sealed usage is 1,133 input, 0 cached input, 265 output, and 0 reasoning
  tokens. Provider-reported raw cost is `$0.00034796` (`348` normalized
  micros). Trace timing is
  989 ms; the before/after accounting window is 8,276 ms. Authoritative account
  snapshots both read 111,578,819 micros, so their observed delta is 0 while
  the finer provider receipt retains the nonzero call cost.
- Cleanup verified. A provider-free reconciler bound the claims, raw hashes,
  full-session inspection, response failure, tokens, cost, timing, and spend
  snapshots into write-once `terminal-failure-receipt.json` and
  `accounting.json`. Replay/overwrite is denied. The sole real authority is
  exhausted and no retry or further provider call is authorized.
- No role/model was selected, no credential content was displayed or retained,
  and no repository context, tool, production/outbound action, upstream audit,
  Orca/Browser, or Phone Farm surface was given to the model.
- Final provider-free fake and prepare-only commands passed. The exact
  boundary/hidden-assistant/mutation/claim/spend/duplicate/replay tests passed
  20 repetitions; full `agentqualification` and `screening-run` packages and
  focused vet passed.
- A deliberate second provider-free reconciliation failed on the existing
  write-once terminal receipt. The actual evidence root contains only workflow
  1; `terminal-failure-receipt.json` and `accounting.json` exist, while
  aggregate-success `run.json` does not.
- Updated the Silver Mediums hub policy, accepted decision, current state,
  company changelog, and atomic claim. `check_hub.py` passes with the existing
  146 stale-claim plus dated snapshot/registry warnings. The real 20-source
  stewardship checker passes with graph SHA
  `ff4119b1d70ced0a943f8dd056a1f3851e24c975ebacfad37dfc76d9aa75d1fa`,
  role SHA
  `c89c5f619c5f1a724e255b9014bfc882295a2c00504191bfc48569a5bbcc63e2`,
  and bound hub `CURRENT_STATE.md` SHA
  `ac0f65d6683acbc60878bc249031968985d1d0bf6b0b39e7b96475edbef5a413`.

## 2026-09-14 - Item 14 provider-free roadmap closure

- Preserved the terminal OpenRouter observation and exhausted authority. No
  provider, credential, model/role selection, production/outbound action,
  Browser/Orca, upstream audit, or Phone Farm surface was accessed.
- Added migrations 558-574 for three controller-owned append-only workflow
  records: cross-issue consequence receipts, accepted regression change
  records, and rejected regression attempts. There are no foreign keys or
  cascades; every index has a separate `CREATE INDEX CONCURRENTLY` migration.
- Added `internal/systemgraphstore`. Consequence creation locks the workspace,
  both issue owners in UUID order, and both current issue/outcome snapshots in
  one transaction. It returns an exact idempotent replay and rejects altered
  reuse, missing ownership, different issues, or stale snapshot claims.
- Regression changes now require exact typed `workflow-evidence:<uuid>` rows
  that are current, non-superseded, and non-contradicted on the exact source
  issue. The store replays complete accepted and rejected hash chains before
  appending add/update/supersede/retire outcomes, and later release/QC selection
  is reconstructed after restart.
- Added consequence, regression-change, and regression-rejected records to the
  deterministic issue projection input ledger. Accepted consequence/regression
  changes are owner-meaningful; rejected attempts stay rebuildable audit
  evidence without being reported as progress.
- Added `silver-mediums-stewardship-check` and
  `silver-mediums-item14-acceptance` Make targets. Stewardship runs first and
  therefore blocks provider-free acceptance if the graph, role catalog, or any
  maintained hub source drifts.
- The first acceptance execution failed closed on three concurrently changed
  maintained changelogs. The current files and hub were validated, only those
  source hashes/times were rebound, and the real checker then returned
  `valid:true` for all 20 sources. This is observed acceptance-path drift
  protection, not a synthetic-only test.
- A fresh disposable PostgreSQL 17 instance applied migrations 558-574 and
  passed consequence create/replay, altered and stale denial, two-issue lookup,
  typed/current evidence enforcement, concurrent exact regression convergence,
  add/update/supersede/retire, conflicting-attempt audit, restart recovery, and
  UPDATE/DELETE rejection. The exact container was removed.
- One consolidated review correction repaired the evidence-row lock syntax,
  added workspace/issue owner and evidence locks through commit, canonicalized
  issue lookup identity, rejected bare untyped evidence UUIDs, and moved exact
  sealed-regression replay ahead of current-evidence admission. A prior result
  now replays identically after its evidence is superseded, while every new
  accepted/rejected append still requires current evidence. The database test
  was rerun after the correction and passed.
- A documentation reconciliation initially edited the hash-frozen role-design
  decision register. Both qualification manifest tests failed closed on its
  changed SHA. The edit was removed, exact accepted SHA
  `44025c5503da7f0b7047d4133caccfd3ef1ce06aca925df038c19a1d30fd1c94`
  was restored, and no frozen qualification binding was changed. Current status
  is recorded only in the designated mutable continuation/progress/audit files.
- Full affected tests pass for `systemgraph`, `systemgraphstore`,
  `workflowprojection`, `agentqualification`, `handler`, and the stewardship
  command. Focused `go vet` and `go test ./... -run '^$'` across every server
  package pass. GNU Make is not installed on this Windows host, so the new
  target's exact commands were run directly. Final real
  stewardship output is `valid:true`, `source_count:20`, graph SHA
  `ff4119b1d70ced0a943f8dd056a1f3851e24c975ebacfad37dfc76d9aa75d1fa`,
  role SHA
  `c89c5f619c5f1a724e255b9014bfc882295a2c00504191bfc48569a5bbcc63e2`,
  and bound hub current-state SHA
  `c696f740aabedd78a8e52244d0ed0427b99d306f9a6df08158c6f96333336a37`.
- Reconciled the canonical roadmap: immutable guarded runtime enforcement and
  the complete local shadow workflow suite are locally complete. Deterministic
  responsibility gates 1-4 are closed locally. The only Item 14 gates left are
  valid comparative configuration qualification and the business decision on
  permanent seats versus shared execution.
- This is the natural provider-free stopping point. The owner must either
  authorize a new separately bounded eligible experiment with measurable
  actual-serving-provider evidence, or close Item 14 without a positive
  provider proof and without selecting final production roles/models.
- Final hygiene: the pilot targeted diff checks pass, the frozen role-decision
  SHA remains exact, and no Item 14 PostgreSQL test container remains. The hub's
  three edited files pass targeted diff checks; its whole-worktree diff check
  still reports pre-existing trailing whitespace in dirty `AGENTS.md`, which
  this slice did not edit.
## 2026-09-14 13:12 - Item 14 qualification authority resumed

- The owner corrected the prior continuation conclusion: the remaining model
  testing and qualification work is already authorized within the established
  provider boundaries and cumulative budgets. Choosing the next temporary
  configuration for testing is an engineering decision, not an owner gate.
- This clarification does not reopen or permit reuse of the terminal Claude
  Code or OpenRouter evidence roots, does not weaken the Claude Code
  Sonnet-plus-background-Haiku observation, and does not authorize Claude or
  OpenAI models through OpenRouter.
- Work resumes with a new versioned direct OpenRouter transport precursor for
  an eligible small non-Claude route. Fake-provider and prepare-only gates must
  pass before any bounded real inference. Production, external business
  actions, Phone Farm, upstream-audit, and permanent role selection remain out
  of scope until supported by later evidence and authority.

## 2026-09-14 13:26 - Item 14 direct OpenRouter v2 precursor

- Verified from OpenRouter's current primary documentation that the direct Chat
  Completions endpoint can return `openrouter_metadata` containing the
  requested model, selected endpoint, and provider attempts when explicitly
  enabled. This supplies the serving-provider evidence the terminal OpenCode
  wrapper omitted.
- Froze a new V2 definition and evidence root; v1 remains untouched and cannot
  be replayed. Qwen3 Coder Next via Alibaba remains a temporary eligible
  fixture, not a role/model selection.
- The exact request has one model, provider `order:[alibaba]`,
  `only:[alibaba]`, `allow_fallbacks:false`, `require_parameters:true`, strict
  JSON schema, no tools/plugins/fallback models, zero retries, zero redirects,
  and the existing budget boundaries.
- Plan, fake, and prepare-only commands passed with zero credential reads and
  zero real calls. Focused internal and command tests pass, including mutation,
  duplicate/replay, hidden-tool, exact-attempt, candidate-local response
  failure, and global provider-drift behavior.

## 2026-09-14 13:40 - V2 zero-inference terminal and V3 correction

- The V2 command ran exactly once. OpenRouter rejected the request at parameter
  filtering with router attempt 0 and no provider attempt; account usage stayed
  exactly 111,582,154 micros before and after. The terminal response and
  provider-free reconciliation are sealed. V2 cannot be replayed.
- The response nevertheless supplied the missing route fact: Alibaba offered
  exact endpoint model `qwen/qwen3-coder-next-2025-02-03`, but did not support
  the requested strict structured-output parameter.
- Froze V3 as the smallest correction. It removes only provider-side schema
  enforcement and performs the unchanged strict contract check in the
  controller. Exact route, selected endpoint, attempt count, no-tool,
  no-fallback, no-retry, no-redirect, and spend constraints remain intact.
- V3 provider-free plan/fake/prepare gates and focused tests pass. One
  consolidated review correction distinguishes non-selected endpoint inventory
  from the one selected/attempted route and restores explicit price-reservation
  recomputation. No additional provider call has yet occurred.

## 2026-09-14 13:58 - V3 terminal observation and V4 precursor

- The exact V3 command ran once. It observed exact alias
  `qwen/qwen3-coder-next`, top-level provider `Alibaba`, and one selected
  Alibaba endpoint `qwen/qwen3-coder-next-2025-02-03`, with router attempt `1`,
  one provider inference, and zero retry. This is positive exact-route evidence
  for that request.
- The V3 harness still failed closed because the successful response omitted
  its expected detailed `attempts` array. The candidate output independently
  failed the frozen response field contract. The terminal response hash is
  `52e3ebb36be48988d3eaef1db17c20e73551173e5618e2338479066a2226b98e`;
  usage was 321 input and 268 output tokens with a 499-micro receipt cost.
  Immediate account snapshots stayed at 111,582,154 micros, so both the
  provider receipt and lagging account readback are retained. Reconciliation
  sealed V3 and forbids replay.
- Froze V4, initially SHA-256
  `ebad9780e2369b6014d7da0f782f76facce0aba483e127a9add585a11f361b65`,
  bound to the V3 terminal response. It contains only the three workflows V3
  never called and cannot qualify the candidate after the antecedent failure.
- Provider-free review found and corrected one pre-call overfit: detailed
  attempt metadata may be absent or contain one exact Alibaba/model attempt;
  attempt value `1`, exact top-level identity, and one selected exact endpoint
  remain mandatory, while a second or mismatched attempt fails closed.
- V4 plan, fake, and prepare-only pass; focused tests pass 20 repetitions; the
  full affected packages and focused vet pass. The evidence root and global
  claim remain absent. One independent review is pending before any V4 call.

## 2026-09-14 14:07 - V4 independent review and consolidated correction

- The independent provider-free review returned REVISE. Its assertions that
  V4 lacked a transport/replay lifecycle and that Alibaba was absent from the
  V3 terminal metadata were traced to the earlier v1 observation; the V4
  executor already has the direct one-shot lifecycle and the V3 terminal
  receipt records the exact Alibaba route.
- The review nevertheless found valid provenance and partial-settlement gaps.
  One consolidated correction now embeds and strictly reparses the hash-bound
  V3 terminal receipt, freezes all three canonical request hashes and the
  response schema hash, binds the exact system prompt, and directly tests no,
  one exact, wrong, and multiple detailed attempts.
- Post-exposure global halts now seal a terminal receipt with safe response
  identity/hash, tokens/cost when present, account readback when available,
  credential cleanup, exhausted authority, and no retry. A fake paid
  provider-drift case proves the stop occurs after one request and alternate-
  root replay is denied.
- The corrected V4 definition SHA-256 is
  `c9368b0dbb9f9bf3a01c6dc5d5a554fa840ab0ce4d144f5d1f62ea2522190af6`;
  its embedded V3 terminal receipt SHA-256 is
  `116630044075634e7d6bf479fd5efd2c0cc6c33542e44f26eaa3bf4369780ff1`.
  No second review loop ran.

## 2026-09-14 14:11 - V4 terminal exact-route result

- Invoked the frozen V4 command exactly once. The three previously unattempted
  workflows each made one request; there were zero retries.
- Every response proved the complete required route: requested and actual
  alias `qwen/qwen3-coder-next`, top-level provider `Alibaba`, selected
  endpoint provider `Alibaba`, selected dated endpoint model
  `qwen/qwen3-coder-next-2025-02-03`, and router attempt `1`.
- All three outputs failed the unchanged strict field contract. The model used
  substitute `has_*` or `any_*` fields and omitted the exact required
  repository, production, outbound-action, and role-selection fields. No
  output was normalized, repaired, retried, or promoted.
- V4 usage was 988 input and 758 output tokens with 1,434 microdollars of
  provider-receipt cost. Per-request durations were 1,838 ms, 1,977 ms, and
  1,813 ms. Combined V3+V4 usage is 1,309 input plus 1,026 output tokens and
  1,933 microdollars, with zero of four workflow contracts passing.
- Immediate account snapshots stayed at 111,582,652 microdollars. That value
  had incorporated V3's prior lagged cost but had not yet incorporated V4's
  receipts; receipt and account-readback evidence remain separate.
- Accounting records one credential read, no persisted credential copy, and
  memory zeroing. Tool, repository, production, outbound-action, and final
  role/model-selection counts are zero.
- The sealed evidence root is
  `.qualification-evidence/item14-openrouter-direct-shadow-v4/`. Its three
  response hashes are
  `7f147252c72bb81552d89a2d6fca9eeea153058fc750d87a54134b395ba6714c`,
  `1f71497d6bbd8959688aed4e09ae28b2212e1dcda573dd595684a20359680cac`,
  and
  `b72de826baa98430eaec9b9f39583acb3b28d64516ccb47568e65281a1d7380a`.
  V4 and its authority are terminal and cannot be replayed.
- This proves the direct OpenRouter exact-model/exact-serving-provider
  transport for the eligible small non-Claude lane, but eliminates this Qwen
  fixture for the frozen local-shadow contract. No permanent model or role was
  selected.

## 2026-09-14 14:17 - Codex subscription model-only precursor held

- Ran three provider-free loopback captures while progressively removing
  repository location, project documents, ambient config, inherited desktop
  tool/session variables, and every relevant tool-bearing Codex CLI feature.
  All used dummy credentials and zero real provider access.
- The decisive external-empty-directory capture made one fake
  `POST /v1/responses` request with exact `gpt-5.6-luna`, zero request and
  stream retries, streaming enabled, storage disabled, no top-level tools
  field, and no repository document.
- The request still carried an `additional_tools` developer item exposing
  `functions.exec`, `functions.wait`, and `functions.request_user_input`, with
  `tool_choice:"auto"`. Disabling the code-mode host makes those tools fail at
  execution but does not remove the callable model surface.
- Froze the safe request facts in
  `codex_subscription_boundary_v1.json` at SHA-256
  `de00340714977c7060b8ae46a6ca779b16ba5cb25aa547c2a42f4313997915c3`.
  Added current transport catalog v2 at SHA-256
  `8a98dcbfdfaa5fe16ec814473a656a46067740d82940270e0eaea1ab8cbca348`
  and preserved historical v1 for frozen antecedents.
- Focused mutation and catalog tests passed 20 repetitions. The three exact
  disposable roots were removed with verified absence. They contained only
  generated task-local profile material and the dummy-provider setup.
- Codex CLI 0.153.2 is therefore held as unsuitable for Item 14's exact
  model-only boundary. No real Codex call was made or justified. This is a
  transport result, not a competence verdict or final provider/model choice.

## 2026-09-14 14:39 - Provider-free role/squad operating contract

- Completed the frozen V4 batch record first: its exact Qwen/Alibaba transport
  proof remains valid, all four V3/V4 workflow outputs failed the unchanged
  contract, and every OpenRouter shadow authority remains sealed. No further
  small-model exploration was started.
- Added `silver-mediums.role-squad-operating-contract.v1`, binding the exact
  current role catalog, system graph, 19-case bank, v1 role artifacts, and v2
  role artifact bindings. The contract records 21 roles, 10 controller-only
  accountable identities, three maintained candidate cores, six conditional
  packets, two external Mac roles, one skill-only packet, three squads, and 13
  system journeys.
- Froze all 19 case routes with exact participant order, squad, accountable
  identity, host, shared skill packets, and release-stage declaration. Every
  graph journey capability is covered by at least one locked case.
- Resolved the Product/UI collision without rewriting frozen artifacts: exact
  UI acceptance uses the shared executor plus Product/UI skill; ambiguous
  experience invokes Product/UI before implementation; consequential taste is
  one Mika/owner gate with execution blocked. A shared skill never counts as
  independent review.
- One local self-adversarial review returned REVISE with no P0. One consolidated
  correction fixed the misspelled Mac role, removed a duplicate graph hash
  helper, replaced journey counting with exact capability coverage, and added
  explicit ordered-stage, historical-label, shared-skill, squad, primary,
  duplicate, and Mac-host validation.
- The recorded deterministic result is hash-bound and revalidated on load.
  Contract SHA-256 is
  `b3f88549e6a92df5708c7a89c53cb31b0b034cfbf7639f52c96e76659ae8fe09`;
  result SHA-256 is
  `5602a14e5e4631a5c2590be0dc37a1ab0d381b906b8465374812b7cd4514f010`.
- Focused role/squad tests passed 20 repetitions. Full
  `go test ./internal/agentqualification ./internal/systemgraph` passed.
  Provider requests, credential reads, production/outbound actions, model
  selections, and permanent-seat selections remained zero. Phone Farm was
  represented only by existing catalog data and was not accessed from Windows.

## 2026-09-14 14:49 - Local Item 14 documentation stewardship gate

- Added `silver-mediums.item14-stewardship.v1` to bind the exact local Item 14
  authority/evidence reading separately from the existing Silver Mediums hub
  manifest. Sources are classified as frozen authority, machine contract,
  accepted evidence, or mutable current status.
- The source set includes the frozen decision register, current graph/roles/
  cases, both role-artifact generations, the new role/squad contract and
  result, accepted architecture/benchmark/workflow/transport records, and all
  three canonical continuation documents.
- Added a read-only checker and wired it ahead of provider-free Item 14 tests
  and vet. Definition drift, unknown fields, missing or duplicate sources,
  unsafe paths, class/mutability drift, ordering drift, and changed source
  hashes are denied.
- Focused tests and a real local-source read pass. This work made no provider
  request, credential read, production/outbound action, Phone Farm access,
  model choice, or permanent-seat choice.

## 2026-09-14 14:56 - Provider-free all-route role/squad rehearsal

- Rehearsed all 19 locked qualification cases and the independent company-audit
  route using only the current model-neutral operating contract and accepted
  v1/v2 role artifacts.
- The deterministic trace contains 59 ordered, controller-owned participant
  stages and 10 shared-skill attachments. All 69 references resolve exact
  instruction and skill hashes across 11 role packets. Observed concurrency is
  one; each route ends with one distinct Quality Checker.
- The locked cases retain all 76 required evidence fields at the final verdict.
  Earlier participants cannot claim that final contract, shared skills cannot
  become workers/reviewers, and no receipt may follow the verdict.
- Trace SHA-256 is
  `9f208e29dba1273c342c160c1acf471f50fd61127cc8d590c9437fa5022adce4`;
  recorded result SHA-256 is
  `d0fef123bab7d08ff591eef7f5633dc9e04f481018190b19a31da9cd25e96a81`.
- Mutation checks reject artifact, contribution, order, independence,
  concurrency, and authority drift. Provider, credential, tool, repository,
  production, outbound, Phone Farm-access, model-selection, and permanent-seat
  counts are zero.

## 2026-09-14 15:14 - Primary-subscription role-trial owner gate prepared

- Prepared `silver-mediums.primary-subscription-role-trial-gate.v1` without
  contacting a provider, accessing a subscription or credential, copying a
  fixture, opening Browser evidence, or selecting a permanent role/model.
- The plan compares the shared Windows executor with an inline exact
  Architecture/Product-UI skill against a separately invoked conditional
  participant followed by the shared executor. R01 and R02 run in both Claude
  Code and Codex lanes as eight observations totaling exactly 12 named CLI
  participant sessions.
- Provider-native tools are forbidden. If authorized, the controller may
  mediate only embedded-fixture reads, edits, allowlisted checks, and R02's
  in-app local-loopback Browser evidence. Each arm gets a fresh disposable
  fixture; the hidden verifier supplies the independent verdict with no
  provider session.
- A self-adversarial review found ambiguous session accounting, provider-paid
  reviewer scope, and Browser breadth. The one consolidated correction added
  the exact invocation matrix, provider-free verdict mode, zero retries,
  relaunches, or automatic correction replays, and the local in-app Browser
  boundary. Focused mutation tests passed 20 repetitions.
- Claude's requested-Sonnet plus background-Haiku fact remains unchanged. The
  owner must accept that disclosed composite attribution or exclude all four
  Claude observations, leaving six exact Codex participant sessions. Neither
  choice weakens or repeats the terminal exact-one-model transport result.
- The next action is provider- and subscription-dependent. The machine gate
  remains closed until the owner grants one of the exact alternatives in
  `ITEM-14-PRIMARY-SUBSCRIPTION-ROLE-TRIAL-DECISION.md`.

## 2026-09-14 15:20 - Provider-free Item 14 acceptance complete

- The external Silver Mediums stewardship checker passed all 20 sources. The
  local Item 14 checker passed 27 sources: one frozen authority, nine machine
  contracts, 14 accepted-evidence documents, and three mutable current-status
  documents.
- GNU Make is unavailable on this Windows host. Ran the exact commands behind
  `silver-mediums-item14-acceptance` directly: the complete scoped Go test set
  passed and the matching `go vet` set was clean.
- Repeated the operating-contract, all-route rehearsal,
  primary-subscription-gate, and local-stewardship tests together 20 times;
  all passed.
- Verified the primary-subscription owner gate at SHA-256
  `d21f6b459bdf83757f6c7d27ac103309d659f5eb2a0dc8f13e0272439ed4e405`,
  the accepted role-design register unchanged at
  `44025c5503da7f0b7047d4133caccfd3ef1ce06aca925df038c19a1d30fd1c94`,
  the role/squad contract unchanged at
  `b3f88549e6a92df5708c7a89c53cb31b0b034cfbf7639f52c96e76659ae8fe09`,
  and rehearsal result unchanged at
  `d0fef123bab7d08ff591eef7f5633dc9e04f481018190b19a31da9cd25e96a81`.
  The sealed V4 root still exists and no command invoked it.
- No provider, subscription, credential, Browser, production, outbound,
  Phone Farm, model-selection, or permanent-seat authority was used. All safe
  provider-free roadmap work is complete; the documented subscription-trial
  choice is now the genuine owner-only boundary.

## 2026-09-14 16:05 - Owner authorization bound and full fake trial passed

- Accepted the owner's exact authorization for the frozen 12-session
  controller-mediated disposable-fixture trial. Claude attribution is limited
  to exact requested Sonnet primary plus disclosed background Haiku for this
  trial; no permanent model, role, provider, or seat selection follows.
- Added `silver-mediums.primary-subscription-role-trial.v1`, SHA-256
  `bd412ada00e9dbccd26c42423fed193957afcf516e4e6bf0e347ba1338b9233a`.
  It exactly binds 12 participant ordinals across eight R01/R02 structural
  observations, both subscription lanes, zero retries/fallbacks/relaunches,
  controller-only fixture actions, local in-app Browser scope, and immediate
  disposable-workspace cleanup.
- Added restricted Claude and Codex CLI adapters. Claude requires reported
  Sonnet and permits only the already accepted Haiku identity; Codex pins
  `gpt-5.6-luna`, reports model identity as unavailable when the CLI omits it,
  and invalidates any native tool event. Neither adapter stores prompts or
  credential material.
- Added focused mutation, action-order, parser, third-model, native-tool,
  cleanup, duplicate-execution, and verification-replay tests.
- The first fake-root attempt stopped before any provider invocation because
  an artifact path was relative to the command working directory. It remains
  preserved. The correction embedded the exact role artifacts rather than
  depending on repository-relative participant access.
- The corrected terminal precursor is
  `.qualification-evidence/item14-primary-subscription-role-trial-v1-fake-v3/`.
  It completed 12/12 fake CLI sessions and eight observations with all hidden
  grades passing, four complete fake R02 viewport sets, no tool/retry/fallback
  event, and verified fixture/provider-workdir absence. Terminal ledger
  SHA-256 is
  `10f79077e8b1393e758fed79fc6d9919e5053431fa4cac0bbb442204909c4cc0`.
- No real provider call, credential access, production/business action, Phone
  Farm access, OpenRouter access, deployment, or permanent selection occurred
  in this precursor slice.

## 2026-09-14 16:13 - Claude lane completed without replay

- Consumed the six frozen Claude Code participant sessions exactly once. Every
  successful terminal receipt reported requested primary
  `claude-sonnet-4-6`; no background Haiku happened to appear in these six
  receipts. The already accepted composite limitation remains unchanged.
- Both R01 observations passed the provider-free hidden verifier. The inline
  R02 observation also passed after controller-owned in-app Browser capture at
  390, 820, and 1440 pixels: total 47, four rendered cards, and no horizontal
  overflow at every width.
- The separate R02 Product/UI specialist session succeeded. Its executor
  session returned an otherwise structured plan that omitted the required
  `local-static-preview report` check. The controller rejected it as a response
  protocol failure. That observation is preserved as invalid; it was not
  repaired, replayed, or sent to Browser.
- All six provider work directories and all four Claude observation fixture/
  initial-copy pairs were removed with verified absence. Native tool events,
  retries, fallbacks, repository/production/business/Phone Farm access, and
  permanent selections remain zero.

## 2026-09-14 16:24 - Codex session 7 preserved; one bounded correction applied

- Codex ordinal 7 (`codex-r01-inline`) was invoked once and failed before a
  usable candidate. The isolated process still inherited the user's Codex
  profile: the CLI reported a malformed user hook configuration, then the
  provider rejected the executor output schema because optional properties
  were not also listed as required. The observation was cleaned and preserved
  as invalid. There was no retry, relaunch, fallback, or correction replay.
- The raw JSONL remains hash-bound. It contains two `item.completed` records
  with item type `error`; those are provider/CLI diagnostics, not native tool
  executions. The ledger classification was reconciled from two diagnostic
  records to zero tool executions without changing session 7's terminal
  failure, request count, raw hash, or cleanup evidence.
- One provider-free consolidated correction applies only to remaining Codex
  ordinals 8-12. Codex response schemas now require every declared property
  and use nullable strings for action fields that do not apply. Each session
  gets a fresh task-local profile containing only an opaque streamed copy of
  the existing subscription authentication file; the profile is inside the
  already verified disposable work directory and is removed immediately.
  Credential contents are not printed, logged, hashed, retained, or included
  in evidence.
- Added fail-closed total/generated-token ceilings plus focused tests for fake
  completion, schema strictness, diagnostic classification, task-local auth
  copying, ceilings, and correction idempotency. All passed. Correction record
  SHA-256 is
  `62b067e949f6dc4a23870678a1cea09bfdf1091f8d8efa3dd662ff53708c3edb`;
  the correction itself made zero provider calls.
- Live ledger after correction: seven consumed sessions, seven provider calls,
  five remaining authorized sessions, zero retries/fallbacks/native-tool
  executions/repository accesses/production accesses/business actions/Phone
  Farm accesses/permanent selections.

## 2026-09-14 16:27 - Codex R01 separate observation passed

- Consumed Codex ordinals 8 and 9 exactly once through separate disposable
  task-local profiles. Each request pinned `gpt-5.6-luna` at Medium with zero
  retries/fallbacks; the CLI receipt does not expose trustworthy actual model
  identity, so no stronger attribution is claimed.
- Architecture Assurance returned valid structured evidence and the shared
  executor returned a valid controller action envelope. The controller changed
  only `consumer_faulty/normalize.py` and `EVIDENCE.md` in the fresh embedded
  R01 fixture, ran the allowlisted pre/post check, and the provider-free hidden
  verifier returned PASS.
- Session usage was 8,480 input/578 output tokens in 15.406 seconds and 9,114
  input/578 output tokens in 14.589 seconds. Each receipt records one
  subscription provider call and verified provider-workdir removal.
- The observation's fixture and initial-copy roots were removed with verified
  absence. Live totals are 9/12 sessions and nine provider calls, with zero
  retries, fallbacks, native tools, repository/production/business/Phone Farm
  accesses, or permanent selections.

## 2026-09-14 16:31 - Codex R02 inline observation passed

- Consumed ordinal 10 exactly once: pinned `gpt-5.6-luna`, Medium, 9,569 input
  and 1,215 output tokens, 27.712 seconds, one provider call, no reported tool
  event, and verified disposable-profile removal. Actual model identity remains
  unavailable from the CLI receipt.
- The participant returned the complete controller protocol, including both
  allowlisted checks and exactly one Browser request. The controller modified
  only the fresh embedded R02 fixture and paused without another provider turn.
- The in-app Browser captured the one-time loopback preview at 390, 820, and
  1440 pixels. Every width rendered total 47 and four cards without horizontal
  overflow. Screenshot and DOM hashes were sealed, the hidden verifier returned
  PASS, and the capture token was deleted.
- Provider profile, fixture root, and initial-copy root are absent. Live totals
  are 10/12 sessions and ten provider calls; retries, fallbacks, native tools,
  repository/production/business/Phone Farm accesses, and permanent selections
  remain zero.

## 2026-09-14 16:35 - Twelve-session subscription trial sealed

- Consumed Codex ordinals 11 and 12 once each. Product/UI evidence and the
  shared executor envelope were structurally valid, but the executor supplied
  natural-language edit instructions instead of complete literal JavaScript
  and CSS. The controller remained inside the disposable fixture, the first
  allowlisted check failed, Browser was never opened, and cleanup was verified.
  The outcome remains a terminal controller-action failure with no replay.
- Provider-free verification sealed all 12 sessions and eight observations.
  Outcomes are five PASS, two invalid, and one controller-action failure. All
  provider workdirs and fixture/initial-copy roots are absent.
- Ledger counters: 12 provider calls; zero retries, fallbacks, native tools,
  repository accesses, production accesses, business actions, Phone Farm
  accesses, or permanent selections. Provider authority is exhausted; no
  thirteenth call is authorized.
- Claude aggregate: 43,004 input/12,801 output tokens, 219.484 seconds,
  $0.372820 API-equivalent subscription estimate. Every Claude receipt reported
  only requested Sonnet; accepted possible background Haiku remains a transport
  limitation rather than an observed event in this trial.
- Codex aggregate: 45,755 input/3,702 output tokens, 95.121 seconds, no provider
  cash receipt. Every command pinned Luna/Medium; actual model identity is not
  emitted by the CLI and is not claimed.
- Terminal ledger SHA-256:
  `5098d32f8781628547dde360f2c8f7b48bfdd733247d16c8b40990d0c3a53713`.
  Evidence-manifest SHA-256:
  `7190de705aa51b213df2d92986fcfb725e9a00fdf0915fc1802f6568e3a12167`.

## 2026-09-14 16:49 - Independent review passed; stage contract accepted

- Added a separate provider-free reviewer over the immutable trial evidence.
  It independently recomputes authority/definition identity, session/raw hashes,
  model and token boundaries, terminal cleanup, hidden-verdict artifacts,
  Browser capture hashes, correction bounds, and the evidence manifest.
- All 13 checks passed. The focused integration test copied the evidence root,
  altered one raw provider response, and received `REVISE`; the canonical root
  remained untouched. Review SHA-256 is
  `f217c433a147b2aaa745d4c87b04182da4095cd1d5c7742e32f2b7ac43b46bec`.
- Recorded the trial comparison without promotion: inline exact skill passed
  3/4 cells and separate specialist plus executor passed 2/4. One run per cell
  and an early Codex harness failure make this insufficient for a topology,
  role, model, or permanent-seat selection.
- Both separate Product/UI specialist sessions completed, yet both downstream
  executors failed the full route. Added an additive provider-free role-stage
  contract that binds all 11 exact response schemas and controller handoffs.
  Specialist evidence is advisory only; the controller reissues the full
  packet; any mediated edit must contain complete literal bytes; invalid or
  failed stages receive no automatic repair/replay.
- The contract covers 20 routes, 59 participant stages, and 10 shared-skill
  attachments. Focused mutation tests reject provider/selection/authority
  expansion, direct handoff, missing packets/fields, nonliteral edits,
  automatic replay, Phone Farm reclassification, and trial-finding drift.
  Contract SHA-256 is
  `f0105e916d826407b499d0de76c39e11fe9c29c96570fa04c5ef18f49ed705f5`.
- No provider, credential, repository, production, outbound, Phone Farm, model,
  or permanent-seat action occurred in this slice.

## 2026-09-14 16:57 - Full role-stage and local-shadow rehearsal passed

- Replayed the new response/handoff contract across the existing 20-route
  all-role rehearsal and all four complete local shadow workflows.
- The deterministic result contains 91 receipts: 69 all-route and 22
  local-shadow; 89 bind an exact role response, two remain controller-only
  outcome bindings, 79 reissue the complete controller packet, 10 remain
  non-worker shared-skill attachments, and 20 require self-contained executor
  action payloads.
- All 112 role-qualified response fields resolve from their immutable
  instruction examples and paired skills. Both terminal Product/UI downstream
  handoff failures remain source facts.
- Mutation tests reject altered response classes, direct handoff, incomplete
  packet reissue, authority-bearing prior evidence, omitted executor action
  obligations, promoted shared skills, controller execution, provider calls,
  selections, or trace drift.
- Combined trace SHA-256 is
  `7cad59a9b3beef094ba47846f8c5b4adb47d2d1863dd43ec895ee5cd90690054`;
  recorded result SHA-256 is
  `45c7294b5a15b36a9bd702fc3241aaeaf2697282897d99eb0b362e9a38d02217`.
  All provider/credential/tool/repository/production/outbound/Phone Farm/
  selection counters are zero.

## 2026-09-14 17:11 - Provider-free consolidation complete; owner decision reached

- Expanded the local Item 14 stewardship envelope from 27 to 34 sources. The
  added sources are the terminal primary-subscription definition/result and
  result document, the role-stage contract/rehearsal and contract document,
  and the final configuration decision packet.
- Local stewardship passes with one frozen authority, 13 machine contracts,
  17 accepted-evidence documents, three current-status documents, and zero
  provider/credential/production/selection counters.
- Ran the complete scoped Go package suite used by the Item 14 acceptance
  target; all packages passed. Matching `go vet` was clean.
- Repeated the new deterministic stewardship, trial, role-stage, and mutation
  checks 20 times; all passed in 368.672 seconds.
- Re-verified the sealed trial without execution: 12 sessions/calls, five
  PASS, two INVALID, one FAILED, zero retries/fallbacks/native tools or scoped
  external effects. The independent reviewer again passed 13/13 checks and the
  91-receipt stage rehearsal recomputed its accepted trace.
- The separate shared Silver Mediums hub prerequisite failed closed on current
  hash drift in `projects/canopy-backend/CHANGELOG.md` and
  `projects/canopy-desktop/CHANGELOG.md`. Those concurrent, out-of-scope hub
  changes were preserved and were not imported into Item 14. The local Item 14
  result is green; the aggregate Make target remains red until the hub's own
  steward reconciles that independent manifest.
- A final review found no new actionable defect. The already accepted bounded
  Codex transport correction and every terminal observation remain unchanged.
- Added `ITEM-14-FINAL-CONFIGURATION-DECISION.md`. The evidence does not qualify
  a permanent configuration or seat. Recommended next state is hold with no
  permanent selection; a new reliability trial requires a separately capped
  owner grant. No provider call, credential access, production action, Phone
  Farm access, model selection, or permanent-seat selection occurred.

## 2026-09-14 18:47 -04:00 - Item 14 fresh reliability trial v2 sealed

- Preserved the original primary-subscription trial and its canonical 5 PASS /
  2 INVALID / 1 FAILED result byte-for-byte.
- Froze a new 12-session reliability definition: three repetitions per
  Claude/Codex by R01/R02 inline shared-executor-plus-exact-skill cell.
- The first full fake run exposed a Windows replace-existing failure only after
  all 12 synthetic sessions. No subscription CLI or credential was used.
- A separate provider-free review recorded five findings. One consolidated
  correction added Windows-safe atomic replacement, strict selected-definition
  and authority/runtime checks, receipt-derived call totals, truthful
  pre-workdir absence, and immutable post-seal manifests.
- Focused repeated mutation tests passed. A fresh corrected fake campaign
  sealed 12/12 synthetic sessions and 12/12 cleaned observations. The original
  v1 ledger re-verified under the stronger checks with its exact historical
  schema exception constrained to corrected invalid session 7.
- Prepared a fresh real evidence root at zero sessions and zero calls, then
  consumed all 12 owner-authorized subscription sessions exactly once.
- Final observation outcome: six PASS, zero INVALID, six FAILED. Each lane
  passed R01 3/3 and R02 0/3. Five R02 plans stopped at deterministic controller
  failure. One Codex R02 plan reached the in-app Browser; captures at 390, 820,
  and 1440 pixels were accepted, but its hidden verdict was FAIL.
- All 12 provider workdirs and all 12 fixture/initial-copy pairs are absent.
  The one-time Browser token is absent. Retries, fallbacks, native tools, real
  repository/production/business/Phone Farm access, permanent selections, and
  post-start corrections are zero.
- Claude totals: 43,484 input, 17,080 cached input, 15,550 output tokens,
  239.440 seconds, and $0.396763 CLI API-equivalent estimate. All six receipts
  reported Sonnet; accepted possible background Haiku remains a limitation.
- Codex totals: 56,475 input, 5,362 output tokens, 151.585 seconds, and no
  provider cash-cost receipt. All calls pinned `gpt-5.6-luna`/Medium; actual
  identity remains unavailable.
- Ledger SHA-256:
  `cea2000f9811d0ae5254ea8c86a94faf793389d9340cf29677ebb3f4478e730c`.
  Manifest SHA-256:
  `c81d4c185bca6a01c1fa51670af6c9987e5e9ff55e0f7e569b09204f115f8eb2`.
  Independent review: 12/12 PASS, SHA-256
  `f215234e6bf352255e72bef543160aeb6580d82daf385ac793fc825123c0dd8f`.
  Machine result SHA-256:
  `9f99664568ff701cadddbac537a7548cebfd88fbd2bc7d3d1415176ddb1f1f1a`.
- Evidence supports only a reversible local R01-like topology: shared Windows
  executor plus exact architecture-assurance skill inline. Provider/model
  selection remains unset. Product/UI and R02 remain unqualified. Production
  remains forbidden.

Next: provider-free only. Freeze and fake-test the provisional topology binding,
then continue structural qualification of the complete role/squad/instruction/
skill set. No additional provider call is authorized.

## 2026-09-14 19:13 -04:00 - Provider-free provisional scope and R02 candidate complete

- Froze the R01-only local provisional configuration: shared Windows executor,
  exact architecture-assurance skill inline, frozen deterministic fixture,
  provider/model/squad unselected, no automatic dispatch, and every external
  effect disabled.
- The 20-scenario boundary rehearsal allowed only the exact configuration and
  denied 19 mutations. Configuration SHA-256 is
  `017408d91dbe84c9bb8c966b179ff2e66bb390115865905da732b2188f3ad991`;
  rehearsal SHA-256 is
  `2e84fb129e5ddac47762511d29ff02b7ee5ec582a35f092e2df23f38ebbd9176`;
  trace SHA-256 is
  `b89baa151438144fd1bc5bebd34f10829112ffa68dc3f1f2d4315fcc4c8c9f66`.
- Added the v2 readiness matrix covering 11 role packets, 11 instruction/skill
  pairs, ten controller-only outcomes, one Mac-only skill packet, and three
  squads. Structural coverage is complete; only the shared executor is
  runtime-provisional in the R01 inline-skill scope and zero squads are active.
  Matrix SHA-256 is
  `4116a6d8b0b026d313dfe205d4cb62f8b7c0cf63a9532e2ba98b704483f43c8f`.
- Derived the Product/UI R02 v3 correction candidate without changing terminal
  evidence. It removes known pre-repair terminal checks, requires complete
  literal JS and responsive CSS, keeps `EVIDENCE.json` controller-owned, and
  fixes one six-action sequence. The fake workflow and hidden verifier pass;
  observed-failure and authority-expansion mutations fail closed. Contract
  SHA-256 is
  `9816381e4916d76cd6f6faf295ee41cd921ffd2f29926ed1097cbb8d2bb9e309`.
- Added v2 successor stewardship. It retains the v1 manifest byte-for-byte,
  verifies all 31 predecessor immutable sources, and binds 16 new/current
  sources. The initial local successor check passed 47/47 sources with all
  authority counters zero.
- The complete scoped Go package suite passed; matching `go vet` was clean.
  Focused stewardship/reliability/configuration/readiness/R02 tests passed 20
  repetitions. The real trial reverified as 12 calls, six PASS, six FAIL, and
  zero forbidden effects. Its independent reviewer again passed 12/12 checks.
- The Make executable is unavailable on this Windows host, so the target's Go
  commands were run directly. The separate hub command still fails closed on
  the same two out-of-scope Canopy changelog hash changes. No hub source was
  modified or adopted.
- No provider, credential, native tool, real repository, production, outbound,
  Phone Farm, activation, or selection action occurred in this provider-free
  slice.

Next owner-only gate: hold the safest R01-only local provisional scope, or issue
a new separately capped authorization to measure the v3 R02 packet at runtime.
No provider call is currently authorized, and Product/UI remains unqualified.

## 2026-09-14 20:06 -04:00 - Closeout integration pilot packet frozen

- The owner closed Item 14 at its qualification boundary and authorized one
  separate harmless local integration/preservation workflow in the exact-byte
  restored A: pilot. The original dirty C: checkout remains untouched.
- Started a unique current-source compose stack on loopback ports
  13000/18080/15432. Backend, database migrations, and frontend login checks
  passed; the pre-existing 3000/8080 stack was not changed.
- Created the isolated profile/workspace with one-task concurrency, a local
  evidence workspaces root, 20-minute task timeout, and daemon updates/reloads
  disabled. A zero-workspace login unexpectedly attempted to open a system
  browser and waited; it was interrupted once and the workspace was created via
  CLI. No provider call occurred.
- Froze the exact issue packet, ten-root allowlist, new report path, maintained
  Phone Farm docs-only boundary, read-only command policy, evidence schema, and
  UI acceptance. `C:\Dev\SMC`, SSH, Mac/device access, production, deployment,
  outbound, destructive, credential, and shared-access actions remain denied.
- Corrected the pre-provider execution cap from a three-task shorthand to the
  complete five-task maximum required by the automatic squad lifecycle: intake,
  executor, reviewer, and only on rejection correction plus re-review. Zero
  automatic retry/fallback, Anthropic API, or OpenRouter is allowed. The
  temporary Sonnet-primary binding is not a final selection.

Next: start the isolated daemon through the locally tested Claude discovery
wrapper, create temporary agents/skill/squad/template, submit the issue through
real Multica intake, then independently verify workflow, report, and background
Board/Issue/Inbox projections.

## 2026-09-14 20:14 -04:00 - Integration pilot ready for single dispatch

- Started the isolated daemon with one runtime advertisement: Claude Code
  2.1.271 through the local discovery wrapper. All non-Claude runtime paths are
  pinned to missing executables; daemon updates/reloads and model fallbacks are
  off. Startup consumed no inference.
- Created one exact architecture skill plus distinct temporary executor,
  reviewer, and intake-leader agents. Every agent is private, one-task-only,
  pinned to temporary Sonnet-primary/Medium, and has explicit empty MCP config.
- Created `CEF Inventory Pilot Squad` with the intake leader and distinct
  implementer/reviewer. Its automatic template allows one correction and has no
  release, deployment, production-verification, or owner-acceptance gate.
- Created `CEF-1` unassigned with the exact frozen packet. It has zero runs and
  the isolated report directory is absent. No provider inference or report
  write has occurred.

Next: assign `CEF-1` to the squad exactly once and monitor the five-task ceiling,
structured events, report confinement, and independent-review result without a
manual rerun.

## 2026-09-14 20:20 -04:00 - Provider-free dispatch harness corrected

- The first assigned leader task started the local wrapper but produced no run
  messages and no real Claude child. Root cause: the wrapper read stdin to EOF,
  while Multica keeps stream-json stdin open for later control responses. The
  task was cancelled; provider inference and report writes remained zero.
- The task row also advertised two attempts despite the packet's zero-retry
  rule. The isolated pilot database task default and current row were narrowed
  to one before any failure or retry. Repository migrations were not changed.
- Consolidated correction: read one newline-delimited frame; handle only local
  `list_models`; otherwise launch Claude immediately and stream the first frame
  plus remaining stdin. Fake-child delegation, fake model discovery, and local
  `--version` delegation passed. Wrapper SHA-256 is
  `69fa2c68bbbcc7b62c40cda78c79f04c53ec07fdb69eaa53b4176fcaceccdd0e`.
- The cancelled task is preserved as a provider-free precursor intervention.
  A single replacement leader run is needed to start the intended workflow and
  is not a provider retry; the five provider-backed task cap is unchanged.

Next: trigger exactly one replacement leader run on `CEF-1`, then monitor task
count, actual model observations, structured handoffs, and report confinement.

## 2026-09-14 20:35 -04:00 - Independent review rejected initial inventory

- The replacement intake leader completed and delegated once to the temporary
  executor. The executor created the three report artifacts and recorded an
  `implementation_ready` workflow event.
- The distinct reviewer independently recomputed the inventory and recorded
  `review_failed`. Its correction packet identified swapped CRM/Browser
  detached counts and two roots whose untracked files were inconsistent with
  the stated dirty definition. The workflow advanced to its one allowed
  correction.
- The initial executor and reviewer both exhausted their fixed 18-turn Claude
  Code budgets after their durable workflow event and issue comment were
  accepted. Their task rows consequently show `failed` even though their
  handoffs were delivered. Each terminal failure caused Multica to enqueue an
  additional delegated-failure recovery task.
- Both unintended recovery tasks were cancelled while still queued and their
  recovery signals were marked delivered. Neither was claimed and neither
  contacted Claude. The intended correction task is the only active task and
  is provider-backed task four of the five-task cap.

Next: let the single correction finish, contain any redundant recovery enqueue,
then permit only the workflow-generated re-review and independently validate the
full report contract.

## 2026-09-14 20:41 -04:00 - Hard task ceiling breached; daemon stopped

- The single correction task edited `inventory.json` and the owner report, but
  Claude returned an undetailed terminal error before `correction_ready` was
  recorded. The plan stayed in `correction` with correction count 1/1.
- Multica immediately consumed task five on a delegated-failure follow-up to the
  reviewer, rather than the template's re-review. That task completed after the
  reviewer declined controller recovery work and emitted no workflow event.
- Multica then claimed task six, an intake-leader recovery for the missing
  workflow event. On detection the controller force-stopped exact daemon PID
  54112, verified that its descendant process tree and all pilot executables
  were absent, and cancelled the task with
  `workflow_task_ceiling_exceeded`. The task had eight stored messages and no
  finalized usage row; it is counted as a provider-backed invocation.
- The pilot therefore failed its hard five-task boundary. No further provider
  execution is permitted, and the workflow cannot be repaired or completed
  under the consumed one-correction budget.
- Provider-free independent validation parsed all three artifacts and the
  original 93,160-byte Claude collection output. The collection has 434
  snapshot worktree records (SHA-256
  `c35bb4fabcf70092298d7eb6cddf8e1f14b58135c467170ba9e0b82dd17a8f11`),
  but `inventory.json` has no per-worktree arrays and the report admits only
  three path checks. Current validation sees 428/434 snapshot paths present and
  six absent; current Git lists 433 records after one CRM path disappeared.
- Current artifact hashes are `inventory.json`
  `a6b486f6ccbb01f2df090b734a0fdeb8311a9ca4d2bd3af341f53a3c69944bd6`,
  owner report `750cf7d03a6b22cdc964d64f83cb4634d56190d946d044a17650adfb6f890406`,
  and `EVIDENCE.md`
  `e72d2f7217efb39992ee0e03f2ee94469685d1833e2c7b56b54856de644e4349`.
  The first two do not match the hashes recorded before the partial correction;
  `EVIDENCE.md` also retains the rejected CRM/Browser attribution and 7/10 dirty
  calculation.

Next: preserve the failed workflow state, observe Board/Issue/Inbox through the
background built-in Browser, then finish provider-free accounting and original-
checkout preservation checks. Do not restart the daemon.

## 2026-09-14 20:50 -04:00 - Background UI evidence confirms failed projection

- The first hidden in-app Browser login failed because the frontend advertised
  cross-port `http://localhost:18080` API and WebSocket endpoints. Network
  evidence showed HTTP 200 preflights with no `Access-Control-Allow-Origin`,
  followed by failed fetches.
- Added an isolated compose override that leaves `NEXT_PUBLIC_API_URL` and
  `NEXT_PUBLIC_WS_URL` empty. Stopped—but did not delete—the old stateless
  frontend container and started a uniquely named same-image frontend on the
  same loopback port using the existing `backend:8080` Next proxy. Login then
  succeeded in the background Browser. No app source, database, provider, or
  production state changed.
- Board rendered `CEF-1` as the sole `Status unavailable` item; all normal
  operational lanes, including Done, were empty. The card said no verified
  progress and no recorded next step.
- Issue detail independently showed Status unavailable, no verified progress,
  no next step, In Progress property, offline executor, nine past runs, and UI
  usage estimate `6.3M · $3.57`.
- Inbox rendered zero Needs You, incidents, meaningful updates, and verified
  completions. It showed `Backup due`, no current verified recovery point, and
  an otherwise empty inbox. Thus the required meaningful Inbox projection did
  not occur.

Next: complete provider-free task/usage accounting, focused local tests, and
initial-versus-final preservation checks; then freeze the failed-pilot handoff.

## 2026-09-14 20:58 -04:00 - Failed pilot closed and preserved

- Final database accounting: nine task rows; four cancelled, two completed,
  three failed. One cancelled row is the zero-provider wrapper precursor; two
  recovery rows never started; six provider-backed tasks started. Every row is
  attempt 1/max 1, with zero retries.
- Five finalized Claude receipts all name `claude-sonnet-4-6` and total 72 input,
  51,183 output, 5,995,475 cache-read, and 267,527 cache-write tokens
  (6,314,257 combined). `cost_usd_ticks` is absent; the UI's `$3.57` is only its
  estimate. Task six has eight messages but no final usage/model receipt because
  the daemon was stopped.
- The runtime is offline and no pilot executable process remains. The issue is
  still In Progress; workflow state is `correction`, correction budget 1/1,
  revision 3. No review-pass or meaningful Inbox event exists.
- Fake wrapper delegation and local discovery passed with unchanged SHA-256
  `69fa2c68bbbcc7b62c40cda78c79f04c53ec07fdb69eaa53b4176fcaceccdd0e`.
  Focused recovery, workflow-control, review-effect, and projection Go tests
  passed; focused `go vet` was clean. The tests do not cover the observed chain
  across correction failure, delegated-failure recovery, and missing-event
  recovery. Compose overlay rendering passed.
- The focused web runtime-URL test could not start because `apps/web` has no
  `node_modules`. No dependency installation or network access was attempted.
  The actual background Browser flow verified the isolated same-origin runtime.
- Initial-versus-final comparison across all ten roots found no branch/HEAD or
  tracked/untracked count changes. CRM worktree metadata alone changed 271 to
  270 because `C:\Dev\CRM-navigation-latency-20260914` disappeared during the
  window; attribution is unknown and no pilot command removed it. The original
  dirty Multica checkout remains on its original branch/HEAD with 494 tracked
  and 459 untracked rows. The report path contains exactly the three requested
  untracked files.
- No Phone Farm technical access, SSH, Mac/device work, production, deployment,
  outbound action, credential/access change, fallback, Anthropic API, or
  OpenRouter action occurred.

Natural stop / owner decision: preserve the failed pilot and decide whether to
authorize a separate provider-free Multica defect/fix milestone for global task-
cap enforcement, terminal event/status reconciliation, artifact-schema
validation, and Inbox projection. Do not replay a provider workflow without a
new bounded grant, and do not activate or permanently select the temporary
agents, squad, model, provider, or seat.

## 2026-09-14 23:01 -04:00 - Provider-free defect correction focused proof passed

- Continued only in the isolated A: worktree. The original dirty checkout and
  the failed-pilot evidence remain out of scope; no provider, credential,
  production, outbound, Phone Farm, Browser, or Orca surface was accessed.
- One independent read-only review reported six High and two Medium boundary
  defects. One consolidated correction now binds actual inventory records,
  accepted-event finality, all workflow recovery paths, role-mutation-resistant
  start ceilings, the complete assignment/runtime/model/skill snapshot, an
  exact child environment with no tools or repository, and atomic technical-
  stop plus owner-Inbox admission.
- Focused PostgreSQL tests pass for accepted-event finality without a successor
  row, the role-mutation ceiling, missing transaction support, sweeper failure
  containment, and injected owner-Inbox rollback. Artifact and claim mutation
  matrices pass.
- Exact child-environment/skill tests pass. The complete controller/store,
  claim, daemon, staged executable, fake credential, and fake-provider shadow
  passed with one fixture launch and zero tools. No real provider was called.

Next: finish the broader affected-package and vet gates, recheck frozen hashes,
stop the disposable database, and update the canonical continuation status.

## 2026-09-14 23:21 -04:00 - Provider-free defect correction closed

- Removed two test-only lock copies found by `go vet` and made the claim matrix
  use one isolated runtime per case, eliminating its dependence on mutable
  package-global runtime order. These were folded into the same consolidated
  correction; no additional review/correction cycle or provider call occurred.
- The complete 11-test controller/claim group passes, including every nested
  artifact and packet mutation. Four exact daemon child-boundary tests pass.
  The complete guarded fake-provider shadow passes in 28.35 seconds with one
  staged fixture-provider launch and zero tools.
- Full `workflowguard` and `workflowprojection` packages pass. Matching `go
  vet` across the five affected packages is clean, and `git diff --check`
  reports no whitespace errors. Broader service, handler, and daemon package
  runs remain non-green only on documented out-of-scope repository baseline
  and Windows/profile-dependent tests; the exact changed surfaces pass.
- Neither root nor web frontend dependencies are installed, so no dependency
  installation or frontend gate was introduced for this Go-only correction.
- Before shutdown, no Item 14/guarded/shadow test schema remained. The exact
  task-local PostgreSQL container was stopped and is absent.
- Rehashed frozen failed-pilot evidence still matches all three recorded
  SHA-256 values. The original dirty checkout remains on its original branch
  and HEAD and was not used for implementation.

Natural stop / owner decision: preserve and close Item 14 at this local proof
boundary, or authorize a new separately bounded real workflow replay. No real
provider call, credential access, permanent selection, production action,
outbound action, Phone Farm access, Browser use, or Orca use occurred in this
milestone.

## 2026-09-14 23:39 -04:00 - Bounded replay authorized; provider-free broker preflight opened

- Owner accepted the failed-pilot defect correction and authorized one replay
  through the existing Claude Code subscription, with a five-provider-task
  global ceiling, no retries/fallbacks, and an immediate stop before any sixth
  provider launch.
- Preserved the original dirty C: checkout, the old failed database/runtime,
  and the three immutable failed-pilot hub artifacts. The replay will use a
  fresh loopback stack and a new additive report path.
- Preflight found one real execution gap before provider use: successor tasks
  are correctly isolated and tool-free, but therefore cannot emit the frozen
  artifact/review events. The bounded correction now being prepared keeps the
  Claude child tool-free and delegates only the exact report and task-scoped
  workflow operations to a hashed local parent broker.
- No provider call, credential-file read, production/outbound action, Phone
  Farm technical access, Browser action, Orca action, or permanent selection
  occurred in this checkpoint.

## 2026-09-15 00:15 -04:00 - Replay broker fake and prepare-only gates passed

- Implemented controller-owned broker issuance from one exact acceptance
  marker, immutable assignment binding, and daemon digest validation before and
  after staging. The task kind is passed to the broker so review and correction
  successors cannot be misrouted.
- The broker keeps the task-scoped Multica capability parent-only and launches
  one tool-free Sonnet child from a fresh environment. Fake tests prove that
  Multica identity, API keys, repository/MCP/tool capability, fallbacks, and
  unsafe control/tool frames do not cross that child boundary.
- Added and passed fake leader/executor/reviewer flow coverage, exact three-file
  artifact evidence, altered-artifact denial, loopback-only transport, and the
  Windows in-place correction replacement. The handler now treats worktree
  identity as `(root_path, path)`, permitting the same linked worktree to be
  truthfully reported under two authorized roots.
- Started a fresh disposable PostgreSQL 17 test container on loopback 15433,
  migrated it, and passed exact broker issuance, claim mutation, digest
  mutation, and cross-root worktree tests. Focused workflowguard/daemon/broker
  tests and matching vet/diff checks pass.
- All four broker prepare-only cases returned `provider_started=false`; the new
  report directory remains absent. Pre-review broker SHA-256 is
  `496458800a2ecae5e4bfe5e5f04325c3c873f364b962ce757eac4bc84c458d57`.
- A broad daemon run retained unrelated Windows/profile test failures, and one
  broad workflow run retained an unrelated recovery-timing failure. These were
  not folded into this bounded change; all exact changed tests pass.

Next: perform the one authorized independent read-only review, apply at most
one consolidated correction, rerun every fake/prepare-only gate, then build and
freeze the exact replay broker before any subscription task starts.

## 2026-09-15 00:31 -04:00 - Replay broker review returned REVISE

- The one authorized independent read-only review completed without edits or
  provider, credential, network, production, Phone Farm, Browser, or Orca
  access.
- Verdict `REVISE`: the initial leader/executor are not broker-bound; editable
  acceptance data is not controller authority; the provider child bytes are
  not independently bound; transcript and artifact admission are too
  permissive; the fake precursor skips the full task path; and the worktree
  duplicate test does not exercise a true duplicate `(root_path, path)` key.
- Provider start remains prohibited. The single consolidated correction now
  in progress moves every role task onto durable expiring/revocable controller
  receipts plus exactly-once start consumption, binds and stages both
  executables, makes transcript/artifact verification strict, corrects the
  duplicate test, and adds one complete real-path fake workflow.
- The pre-review broker digest is obsolete. Provider calls under the replay
  grant remain zero.

Next: complete the one consolidated provider-free correction and pass every
focused, prepare-only, and complete-path fake gate before freezing any replay
authority.

## 2026-09-15 01:13 -04:00 - Consolidated correction complete-path fake proof passed

- Replaced editable acceptance promotion with durable, expiring, revocable
  `qualified-runtime-binding.v2` workflow-broker receipts. All initial and
  successor task paths now issue an exact task capability in the enqueue
  transaction and consume it once at provider start.
- Bound and staged both the broker and its provider child by exact path and
  SHA-256. The child environment is allowlisted and excludes Multica,
  repository, MCP, tools, provider API keys, and ambient task identity.
- Tightened transcript admission to one response-bearing exact-model assistant
  result and fail-closed handling for missing/usage-only models, unknown frames
  or content, redirect/fallback behavior, and every tool result.
- Tightened artifact admission to canonical bytes and totals, correction-time
  recollection, and a final hash read immediately before workflow-event
  emission. Corrected the exact composite worktree-duplicate test.
- Passed a complete fake-provider workflow on fresh PostgreSQL 17 through real
  service enqueue, HTTP claim/start/progress/messages/complete, isolated daemon,
  exact broker, authenticated task-scoped parent calls, and exact fake child.
  Leader, implementation, and review completed; three grants were each
  consumed once; exactly three artifacts were admitted; no legacy packet or
  ambient task context crossed the boundary.
- Re-passed the original fake credential/controller shadow plus broker and
  handler adversarial tests. No real provider, credential, production,
  outbound, Phone Farm, Browser, or Orca access occurred. Provider calls: zero.

Next: run the broader affected regression and vet/diff/integrity gates, execute
all four prepare-only cases with the corrected binaries, then freeze the exact
replay authority if every provider-free gate remains green.

## 2026-09-15 01:24 -04:00 - Corrected replay authority frozen provider-free

- Exact authority/receipt/capability, broker, daemon, handler, service, and both
  database-backed fake workflow groups pass. The real-path fake workflow and
  original credential-bound fake shadow pass together in 85.955 seconds.
- The broad qualification package retains known sealed-source drift failures.
  Its exact changed authority group passes. One pre-review test's attempted
  digest mutation used `jsonb_set` with `create_missing=false` after the field
  was intentionally removed; the test now creates the altered field and its
  denial passes. No runtime control was weakened.
- Affected-package `go vet` and `git diff --check` are clean. All four
  prepare-only cases reported `provider_started=false`.
- The repeated trimmed broker build reproduced SHA-256
  `366718905d210c833d10a1116e4ef4033613be65046fa647625be88e09b4d4da`;
  fake child SHA-256 is
  `ed4c842bd98f38b08ef444964edc7dc5977199aad90346d6f22ec4cb7cfd1f0f`.
- Frozen failed-pilot artifact hashes remain unchanged. The additive replay
  report is absent. The original owner checkout remains on its original branch
  and HEAD with 494 tracked plus 459 untracked rows.
- No disposable test schema remained. The exact PostgreSQL test container is
  stopped and absent from the running set. No provider, credential,
  production, outbound, Phone Farm, Browser, or Orca access occurred.

Next: prepare a fresh loopback replay stack, task-local profile, exact temporary
runtime/agents/squad/issue/plan, and controller receipts. Do not reuse the old
failed stack or evidence; do not permit a provider start until the live claim
readback matches the frozen broker and child boundary exactly.

## 2026-09-15 01:47 -04:00 - Fresh replay prepare-only gate passed

- Started a new loopback-only replay stack on PostgreSQL `15434`, backend
  `18081`, and web `13001`, with fresh data and no reuse of the failed pilot.
- Created a task-local CLI profile and only the private temporary replay
  runtime, three agents, skill, squad, issue `CEF-1`, and workflow plan. The
  new issue has zero runs and zero task rows; the additive report is absent.
- Added a controller-only authority command that hard-codes and validates the
  exact replay identifiers, paths, broker/provider executable digests, live
  topology, private agent settings, model/effort, skill, workflow plan,
  pre-launch state, report absence, and task absence. Focused tests and vet
  pass.
- Ran its non-writing prepare mode against the live replay using a fake
  credential marker. It prepared three exact receipts, stored zero, started no
  provider, and database readback remained zero receipts and zero tasks.
- No real credential was read or copied. No provider, production, outbound,
  Phone Farm, Browser, or Orca surface was contacted.

Next: remove the fake marker, perform the one authorized opaque credential
copy into the exact task-local home, re-run prepare, issue the three expiring
controller receipts idempotently, and verify the pre-dispatch readback before
the single bounded replay launch.

## 2026-09-15 01:51 -04:00 - Real-profile authority issued; provider closed

- Deleted the fake credential marker, then used the existing bounded grant for
  one opaque copy of the approved Claude credential file into the exact fresh
  task-local profile. Only regular-file presence was inspected; no credential
  content, size, hash, or derived value was read or emitted.
- The real-profile non-writing prepare passed: three receipts prepared, zero
  stored, provider not started.
- Issued the three fixed controller receipts twice with identical arguments.
  The append-only store retained exactly three records, proving idempotent
  replay. Each is active until `2026-09-15T08:47:00Z`.
- Exact receipt readback confirms the temporary Sonnet/Medium Claude Code
  subscription, frozen broker and native provider-child digests, tool-free
  allowlisted boundary, exact agent and profile bindings, and only the intended
  leader, implementation/correction, and review kinds.
- Pre-dispatch counts: three receipts, zero revocations, zero grants, zero
  launch consumptions, zero tasks, no report, and zero provider starts.

Next: dispatch the frozen issue workflow exactly once, monitor without manual
retry, then revoke all three receipts and delete/verify absence of the
task-local credential regardless of workflow outcome.

## 2026-09-15 02:05 -04:00 - Authorized replay failed closed before provider

- Sent the one authorized `CEF-1` workflow dispatch once. It returned
  `guarded_configuration_unbound` during controller admission. The enqueue
  transaction rolled back with zero task rows, task grants, launch
  consumptions, daemon claims, provider starts, provider calls, usage, cost, or
  report writes. No retry was made.
- Revoked all three controller receipts immediately. Status moved from three
  active / zero revoked to zero active / three revoked.
- Deleted the task-local credential without reading it, removed the empty
  profile directories, and verified both credential and profile home absent.
- Root cause: the Linux controller used Linux-native `filepath` semantics to
  validate receipt paths attested for the Windows runtime, so the correct
  Windows drive paths failed closed.
- Replaced native-host path validation with attested-host validation. Windows
  syntax is canonicalized fail-closed for slash drift, traversal, duplicate
  separators, invalid characters, and unknown hosts; Linux/macOS use canonical
  POSIX paths.
- Focused lifecycle tests pass on Windows and inside the Linux builder with
  Windows receipt paths. The full target-Windows database-backed fake workflow
  passes in 59.731 seconds and its schema is absent afterward.
- An extra non-authoritative Linux-runtime run of the Windows inventory broker
  admitted its fake leader and then failed at Windows-only inventory, which is
  outside the Alienware target and involved no real provider.

Next: complete broader provider-free regression, rebuild and freeze the
corrected Linux controller image while preserving the exact Windows broker,
verify documentation/integrity and original dirty checkout preservation, then
stop at the owner gate. A new credential copy or real dispatch requires new
separately bounded authority.

## 2026-09-15 02:13 -04:00 - Provider-free closure; genuine owner gate

- Re-passed the exact authority/receipt/capability group, handler claim matrix,
  controller-helper tests, affected vet, and diff check. The complete Windows
  fake workflow passed in 59.731 seconds. The Linux-builder lifecycle test
  accepted exact Windows paths and rejected drift; no test schema remained.
- A post-fix broker rebuild reproduced the frozen SHA-256
  `366718905d210c833d10a1116e4ef4033613be65046fa647625be88e09b4d4da`.
- Built the corrected controller image under the new tag
  `smk-multica-integration-replay-backend:20260915-crossos-v2`, image ID
  `sha256:18ff4edf2e3509c0e8a9f8e2f7a48c325383ea02b7d4e7d9f6e959ea2c19e3b3`.
  Updated only the replay overlay; the retained running evidence stack was not
  recreated with it.
- Background built-in Browser verification showed `CEF-1` visibly Blocked and
  its activity transition recorded. The board had zero agents working; the
  issue claimed no verified progress and no release. The Now projection remains
  unavailable because there was never a task, and no Inbox/workflow event was
  fabricated.
- Final replay accounting: one controller dispatch request, zero task rows,
  task grants, launch consumptions, daemon claims, provider processes,
  provider requests, tokens, cost, or additive report files. Three receipts
  are revoked; credential and profile home are absent.
- Original checkout and failed evidence are unchanged: branch
  `codex/selfhost-pilot-20260909`, HEAD
  `60fea34b9770bd8efbf6f6f0854d4b3ec26b3e00`, 494 tracked and 459 untracked
  rows, and hashes `e72d2f...e4349`, `a6b486...44bd6`, `750cf7...90406`.
- Stopped fresh daemon PID 42816. Stopped—but did not remove—the fresh
  frontend, backend, and PostgreSQL containers, preserving their database and
  volumes. The old failed-pilot backend remains running on loopback `18080`.

Natural stop: no meaningful authorized work remains. A new real workflow needs
a fresh fixture plus separately bounded authority for one opaque credential
copy and one dispatch. Never reuse the revoked receipts/profile, call a
provider without that grant, or infer a permanent role/model/provider/seat
selection.

## 2026-09-15 03:02 -04:00 - Fresh v2 replay provider-free gate complete

- Started a fresh loopback-only fixture on PostgreSQL `15435`, backend `18082`,
  and web `13002` using the corrected controller image. Created only fresh
  private replay objects and an isolated daemon; the stopped prior replay and
  original failed pilot were not reused or modified.
- Preserved the accepted five-task topology and zero-retry contract. A proposed
  three-task simplification was revoked before any credential or provider
  access and was never adopted.
- Passed the complete dispatch -> claim -> daemon -> broker -> fake-provider
  leader/implementation/review workflow and the focused receipt lifecycle,
  cross-platform path, handler claim, and editable-authority denial gates on a
  disposable PostgreSQL 17 instance. Removed that disposable container.
- One independent read-only review returned `REVISE`, no P0, and one P1: the
  authority helper did not reject drift in the database task retry default.
- Applied the sole consolidated correction. Authority prepare/issue now
  requires `agent_task_queue.max_attempts` to default exactly to `1`. The new
  denial test, focused command tests, and vet pass. Rebuilt v2 authority helper
  SHA-256:
  `098709fd94fd79907ebaaddc80dd0d5fdafec0faf38ada5634674f5fdb2982bd`.
- The final fake-marker prepare reported
  `prepared=3 stored=0 provider_started=false`. Live state remains zero tasks,
  receipts, revocations, grants, and launch consumptions; the additive report
  is absent. No real credential was copied and no provider started.

Next: under the existing bounded grant, replace the fake marker with one opaque
credential copy, pass real-profile prepare, issue the three controller
receipts, dispatch once, and revoke/delete/verify cleanup regardless of result.

## 2026-09-15 03:05 -04:00 - Fresh v2 real-profile receipt gate complete

- Deleted the fake marker and copied the approved Claude credential opaquely
  once into the exact task-local profile. Checked presence only; no content,
  size, hash, or derived credential value was inspected or recorded.
- Real-profile prepare returned
  `prepared=3 stored=0 provider_started=false`.
- Issued the three fixed controller receipts once. Readback is three active,
  exact Claude/Sonnet/Medium bindings, issued `2026-09-15T07:01:00Z` and
  expiring `2026-09-15T10:01:00Z`.
- Pre-dispatch state remains zero task rows, revocations, grants, launch
  consumptions, report files, and provider starts. The database task default is
  exactly one.

Next: send the one authorized workflow dispatch, then perform terminal receipt
revocation and opaque credential deletion regardless of outcome. Do not retry.

## 2026-09-15 03:18 -04:00 - Fresh v2 replay failed closed and cleaned up

- Sent the one authorized workflow dispatch exactly once. It created leader
  task `01a0a3e1-2e02-7c7d-9d65-83b559b21218`, attempt 1/1, and one task grant.
- The isolated daemon rejected the task before skill, environment, credential,
  staging, or provider exposure with
  `guarded_dispatch_invalid: incomplete guarded execution facts`.
- No retry or successor was created. Launch consumptions, provider processes,
  provider requests, usage/token rows, cost, and additive report files are all
  zero.
- Immediately revoked all three receipts and verified zero active / three
  revoked. Deleted the credential opaquely, removed the empty profile, and
  verified credential/profile/report absence. Stopped the daemon.
- Confirmed that workspace, issue, executor, runtime, provider, model, and
  effort were populated. The concrete omitted fact was the manual daemon
  launch's parent-only `MULTICA_HOST_ATTESTATION=alienware_windows`.
- Added the exact fixture launcher
  `scripts/item14-integration-replay-v2-daemon.ps1`, SHA-256
  `e9f62e521a615b08e88ad2bc92146c8492d70110c4e573b2cf46f2af7ed8e896`.
  It pins the host attestation and frozen broker and denies discovery of every
  non-Claude provider through one absent sentinel; it contains no credential.
- Focused daemon-config, guarded-fact, substitution-denial, and child-
  environment tests pass. A provider-free launch/stop through the corrected
  harness registered only Claude, with zero active tasks, zero Claude child
  processes, and the credential absent.
- Built-in Browser verification shows `CEF-1` under Status unavailable; issue
  detail is Blocked with one failed run and no verified progress, next step,
  owner need, or release. Inbox contains one truthful Important incident
  (`workflow_incident`, `execution_changed`) and no meaningful update or
  verified completion.
- `git diff --check` passes. The original owner checkout remains on
  `codex/selfhost-pilot-20260909` at
  `60fea34b9770bd8efbf6f6f0854d4b3ec26b3e00`, with 494 tracked-dirty and 459
  untracked rows. Failed-pilot evidence hashes remain exact:
  `e72d2f...e4349`, `a6b486...44bd6`, and `750cf7...90406`.
- A redundant final Go re-run could not start because Windows reported an
  exhausted paging file; the one low-concurrency attempt then ended in a host
  out-of-memory exception. It was not retried again. Earlier focused tests and
  vet had completed green, so no code-test failure is inferred.
- Stopped the v2 frontend, backend, and PostgreSQL containers without removing
  their evidence volumes. No production, outbound, Phone Farm, Orca, OpenRouter,
  Anthropic API, permanent selection, or business action occurred.

Natural stop: the one dispatch and credential-custody authority are exhausted.
The concrete harness defect is corrected provider-free. A new provider attempt
requires a fresh, separately bounded owner grant and may not reuse this task,
receipts, or profile.

## 2026-09-15 04:20 -04:00 - Exact fake execution passed; v2 projection failed

- Started no real provider work. The authorized real issue `CEF-3` remains at
  zero tasks and zero receipts, and the Claude credential was never accessed.
- Froze the full launch input as
  `EXACT-LAUNCH-CONTRACT.json` (SHA-256
  `39787d7136b3575172689c8e9e95c21980f6da71c75271d8272d381bf04e064e`)
  and an exact launcher (SHA-256
  `8a2fdf58c39c072b711990af135f0e0009c5732eeb9eeeede38eac3cc5dfccf60`).
  Preflight checks controller image/identity, ports, executable digests,
  workspace, daemon identity/auth, Windows attestation, provider sentinels,
  tool/retry/task ceilings, report absence, and credential shape.
- Preserved `CEF-2` task `01a0a40c-ffa8-77f3-a7c5-d0e10db21d11`
  failing at start because a user PAT is not an authenticated daemon identity.
  It was attempt 1/1 and created no launch consumption or provider process.
- Corrected only the fixture. The launcher now provisions one random opaque
  expiring `mdt_` token in the isolated database and unique task-local Multica
  profile, verifies the exact workspace/daemon binding, and revokes/deletes it
  on cleanup. The raw token and all derived credential values were never
  displayed or documented.
- A deliberately bad authority-helper build with one linked fake-provider
  digest typo failed prepare with no receipt, task, or provider start. The
  rejected helper SHA-256 is
  `289b05ae692335adfad8eaa195b58a8edc8ecd1937f9b667bace6b6092b4b274`.
  The corrected fake helper
  (`ffe7d8fad6f8aa3976866002cdf2274f2bc7540e4df59826ad1478479b7a0b23`)
  then issued three exact receipts. The prepared real-profile helper
  (`f2010fdc2b84e1071ad40290b39b5dcb8774cdb8d30292f6355e0fa26883b0a6`)
  was never executed.
- Dispatched fresh issue `CEF-4`
  (`01a0a41a-06e6-7092-9800-ec4dad7f014f`). Leader task
  `01a0a41b-faa5-786d-8482-5d64cb13d7d8`, implementation task
  `01a0a41c-032f-7b69-803b-f97e68a2df74`, and review task
  `01a0a41c-3a6b-7099-8051-94eeaf87d7dc` completed at attempt 1/1.
  Readback: three tasks, receipts, grants, and launch consumptions; zero attempt
  or daemon drift; exactly three fake-provider processes; no tools or
  repository in the child.
- Captured the additive inventory report under the Item 14 evidence directory:
  10 roots, 443 records, 438 present and 5 absent, 9 dirty roots, 1 clean, 4
  locked, 1 prunable, and 11 detached. Hashes are
  `171ef6909eabf1de23302e15b93c6cfbfc9e3331b6cd35b6bbed4fd3dc969c3`,
  `cef275759e23e3ea9645e629b2bd257078dec31c9727fa47192999f3a2988627`,
  and `4b15c6bd95d5289730ba6559b8bcceb4887a1e3f02ad32a22373f906901331fe`.
- The complete acceptance failed at projection. The broker emitted a legacy
  `issue_workflow_event`, but the current owner surface requires an
  authoritative Current Picture and owner-relevant Inbox projection. `CEF-4`
  has zero Current Picture rows. Its three Inbox rows are routine
  `new_comment` items with no owner category, change/result code, or completion
  evidence. Built-in Browser verification showed Done in the property panel
  but Status unavailable on Board/Now, no verified progress or next step, and
  no completion Inbox item. The separately empty outbound-notification tables
  have no binding and are not used as evidence for this local Inbox result.
- Focused `agentqualification`, `daemon`, and `handler` guarded tests pass. The
  authority helper's command tests and vet and the launcher parser pass. The
  expected post-cleanup preflight fails at the absent credential boundary with
  `provider_started=false`.
- Revoked all three binding receipts, removed the daemon config/token and fake
  provider home/marker, left zero daemon tokens, stopped the daemon, and
  stopped the three v2 containers without removing containers or volumes.
- Reverified the original checkout at branch
  `codex/selfhost-pilot-20260909`, HEAD
  `60fea34b9770bd8efbf6f6f0854d4b3ec26b3e00`, 494 tracked-dirty and 459
  untracked paths. The three old failed-pilot hashes still match exactly.

Read-only route tracing found no task-scoped v2 projection write command or API.
The controller already atomically adapts legacy task failures into a technical-
stop Current Picture plus owner Inbox item, but accepted legacy success events
have no matching adapter. Owner gate: authorize the smallest provider-free
controller correction to add that compatibility projection while preserving
the byte-identical broker, followed by another complete fake rehearsal. Do not
make a real provider call until it passes; the alternative is an explicit
owner decision to relax the UI acceptance contract.

## 2026-09-15 05:22 -04:00 - Legacy-success projection and complete fake workflow passed

- Implemented the controller-only legacy-success compatibility path. Only an
  accepted non-deployment `review_passed -> done` transition can create it, and
  the workflow event/status, revision-1 Current Picture, and owner completion
  Inbox row share one transaction. Expected revision zero prevents overwriting
  normalized truth or an existing technical stop.
- Focused constructor and database-backed tests prove complete owner summary,
  rollback when Inbox admission fails, existing-picture non-replacement, and
  idempotent duplicate event behavior.
- Ran a first v3 fake workflow as `CEF-5`. Leader, executor, and reviewer all
  completed once and the compatibility records existed, but the detail
  validator rejected top-level `scope_revision=null` against scope document
  revision one. Preserved its 10-root/444-path report under
  `pre-scope-revision-correction-report`; hashes are `98e50472...eddf`,
  `75099713...3c0`, and `e454cb01...82dd`.
- Applied the narrow field correction inside the same consolidation and added
  the real detail-handler assertion. Rebuilt isolated controller image
  `sha256:53ce0736da07fbc00c0fc7c7be8c2b8414e5afea63100aa921fee881b5f37080`.
  The v3 contract, launcher, and fake authority helper hash to
  `837411cf...77469`, `6f943358...1eebb`, and `4c9696fb...b5f63`.
- Ran fresh `CEF-6` (`01a0a453-f28e-776b-9865-9258a36103a6`) through normal
  dispatch, guarded claim/start, authenticated isolated daemon, byte-identical
  broker, and fake provider. Tasks `01a0a455-61e9-79db-9541-b9a7f310bcf9`,
  `01a0a455-6b0f-78be-9ec5-56d304dfcda5`, and
  `01a0a455-9cbc-7b01-a17a-80960d700488` completed once at attempt 1/1.
- Exact readback is three receipts, three grants, three one-shot launch
  consumptions, three distinct task/package hashes, one runtime/daemon, two
  accepted events, zero task messages/tools, and Current Picture revision and
  scope revision one. Six synthetic fake usage rows total six input and six
  output tokens at zero cost; no external provider request occurred.
- Built-in Browser acceptance passed: Board places `CEF-6` in Done; issue Now
  says Review recorded / no next step / nothing needed / no active release;
  Inbox shows one Verified completion with the exact independent-review result.
- Preserved the final report under the v3 `exact-path-rehearsal-report`: 10/10
  roots, 444/444 path checks, 439 present and five absent. Exact inventory,
  Windows-report, and evidence hashes are `9d3087a4...7336`,
  `0fc9d359...23c71`, and `d1140a43...26b72`.
- One adjacent issue-page `meaningful-activity` request still returns 500 in
  this retained dirty baseline because `dr.revision` is absent. Current Picture
  and the required Board/Issue/Inbox paths are independent and passed; this
  limitation was recorded without broadening the adapter slice.
- Terminal cleanup passed: zero active / three revoked receipts, offline
  runtime, zero daemon tokens, absent daemon config and fake provider profile,
  zero broker/fake-provider processes, absent additive report source, and v3
  frontend/backend/PostgreSQL stopped with their two evidence volumes retained.
- Full `workflowprojection` and `workflowstate/v2`, focused database handler
  atomicity, authority command tests, focused vet, launcher parse, and diff
  integrity pass. The disposable PostgreSQL 17 test container was removed.
  Broker/fake-provider/CLI hashes and the original owner checkout branch, HEAD,
  and 494 tracked / 459 untracked counts remain exact.

No second independent review was started; the prior review and single
consolidated correction boundary remain governing. No credential, real
provider, production, outbound action, Phone Farm technical access, Orca, or
permanent role/model/provider/seat selection occurred.

Current stop: the provider-free integration acceptance is complete. The next
unresolved Item 14 step is the existing owner-only final configuration and
permanent-seat choice in `ITEM-14-FINAL-CONFIGURATION-DECISION.md`.

## 2026-09-15 08:54 -04:00 - Fresh real integration replay authority received

- Accepted a new owner-supplied task handoff for one fresh Claude Code
  subscription replay of the verified `CEF-6` workflow. This supersedes the
  provider-use stop only for this temporary integration observation; the final
  configuration and permanent-seat decision remains unresolved.
- Preserved the frozen topology and ceilings: leader, executor, independent
  reviewer, and only after rejection one correction plus one re-review; at most
  five provider-backed tasks; one attempt per task; zero provider retries,
  redirects, or fallbacks.
- Confirmed that the replay accepts Claude Code's already-documented possible
  background-Haiku activity and therefore is not exact-single-model proof. The
  temporary Sonnet Medium fixture is not a model, role, provider, squad, or seat
  selection.
- Re-read the canonical authority and isolated v3 launch contract. At this
  checkpoint the v3 stack remains stopped, the prior receipts are revoked, the
  prior task-local profiles are absent, and no credential or provider process
  has been accessed.
- Next: create a fresh v3 issue and exact receipts, complete prepare-only gates,
  perform the single opaque credential copy, start the isolated daemon, dispatch
  once, monitor within the five-task ceiling, verify owner surfaces/evidence,
  and perform unconditional terminal cleanup.

## 2026-09-15 09:06 -04:00 - Fresh real replay prepare-only passed

- Started the retained v3 frontend/backend/PostgreSQL stack on its original
  loopback-only ports. No daemon or provider started during stack startup.
- Created only fresh issue `CEF-7`
  (`8f3111b6-c9b5-4919-986a-6a043837cdb3`) with the unchanged frozen packet,
  temporary squad, working plan, zero of one corrections, and no deployment or
  owner gate.
- Compiled a non-configurable real authority helper for the fresh issue. SHA-256
  is `26bde00be5ed07d7a0692dd51a6625039e798ba9a343df6f1ee925e2246b3d0b`;
  the provider executable and v3 launch-contract digests matched their frozen
  values.
- Passed the focused authority command suite and guarded broker/child boundary
  tests. Pre-custody database readback was zero tasks, receipts, revocations,
  grants, launches, and daemon tokens; task default remained exactly one.
- Copied the approved credential opaquely once. No contents, size, hash, or
  derived values were read or recorded. Provisioned one task-local daemon
  identity and started the exact isolated daemon; it registered only the
  frozen Claude runtime.
- Real-profile prepare returned `prepared=3 stored=0 provider_started=false`.
  CEF-7 still has no task and no provider-backed execution. Next is one issuance
  call and one workflow dispatch.

## 2026-09-15 09:15 -04:00 - Fresh real integration replay passed and cleaned

- Issued the three fresh controller receipts exactly once, then performed one
  normal issue assignment. The controller created leader
  `01a0a52c-fe4a-79a4-8cd7-6ce364016ef2`, executor
  `01a0a52d-27ac-7dab-99ee-8dc30d819583`, and independent reviewer
  `01a0a52d-78a9-7544-af88-e58fa8e47398`. Each task completed at attempt 1/1.
  Review returned `ITEM14_REVIEW_PASS`, so the conditional correction and
  re-review were not created. Three of the global five provider-task allowance
  were used; there was no automatic retry, redirect, or fallback.
- Exact controller readback is three receipts, three grants, three one-shot
  launch consumptions, three distinct task/package hashes, one daemon identity,
  two accepted events, zero task messages/tools, zero attempt drift, and no
  queued or running task. The issue and plan are Done with correction count
  zero; Current Picture is revision/scope revision one and the owner Inbox has
  one successful verified-completion row.
- Claude Code reported three `claude-sonnet-4-6` and three
  `claude-haiku-4-5-20251001` usage rows totaling 2,887 input, 88 output, 7,763
  cache-read, and 2,958 cache-write tokens (13,696 accounted tokens). Database
  cost is null; the Issue UI displayed `$0.02`. The evidence unit is three
  Claude Code task invocations. The accepted background-Haiku behavior means
  this replay does not prove an exact-single-model lane or exact underlying
  provider HTTP-request count.
- The frozen report passed 10/10 roots and 444/444 required path checks: 439
  present, five absent, four locked, one prunable, and eleven detached. Exact
  SHA-256 values are
  `4f6309ba8d864d513c4c2a70e6f7e0f599051a9ac1681a5e08eb376fa48ea4b9`,
  `48382cc93f8d178bbc737b63bbe74b591b00a40cf7fb3f338fed801dec01ce7e`, and
  `0310bf784a2f5da817921cd638ac531b9f04fecce3969875ec5d05c3f3d9e87f`.
  The source directory was moved exactly into the v3 evidence root and is now
  absent at its original hub path. Phone Farm was not accessed.
- Built-in Browser acceptance passed: Board places `CEF-7` in Done; Issue Now
  reports Review recorded / succeeded, no next step, nothing needed, and no
  active release; Issue detail shows three past runs and `13.7K · $0.02`; Inbox
  reports independently verified completion with the 10/10 and 444/444 result.
- Revoked all three receipts exactly once and stopped the daemon. The runtime is
  offline with zero daemon tokens. The opaque credential copy, task-local
  Claude profile artifacts, daemon profile, and exact child temporary roots are
  absent; no matching Claude/broker/replay process remains. The v3 stack is
  stopped with three containers and two evidence volumes retained.
- The original owner checkout remains on `codex/selfhost-pilot-20260909` at
  `60fea34b9770bd8efbf6f6f0854d4b3ec26b3e00` with 494 tracked and 459
  untracked entries. No production, outbound action, Phone Farm technical
  access, Anthropic API, OpenRouter, permanent selection, or additional provider
  call occurred. This replay's provider authority is exhausted.
- Final provider-free regression passes: authority command tests; exact
  guarded-daemon child, credential-capsule, host/executable, prompt/skill/env,
  and broker full-path tests; full `workflowprojection` and
  `workflowstate/v2`; focused `go vet`; PowerShell launcher parsing; artifact
  hashes; cleanup absence; and `git diff --check`. The hub validator returns OK
  with 146 inherited claims past review and its dated registry/current-state
  warnings.
- The first real stewardship check failed closed on the expected new SHA-256 of
  the mutable current-continuation document. Only the three manifest-designated
  mutable status-source hashes will be rebound after this final record; no
  immutable authority, contract, or accepted-evidence hash is eligible to
  change.
- Rebound only those three mutable hashes. The embedded stewardship test and
  real executable checker both pass with `valid:true`, 47 total sources, 31
  immutable predecessor sources, 16 v2 extension sources, and three mutable
  current-status sources. Stewardship itself remained provider-free and records
  zero provider/credential/production/selection/retry actions; the separate
  `CEF-7` integration observation is documented above.

## 2026-09-15 09:32 -04:00 - Reversible local-default slice resumed

- Reconciled the latest delegation with the terminal `CEF-7` result. The real
  replay remains sealed, successful, and non-retriable; no further provider or
  credential access is authorized or needed for this slice.
- Corrected the remaining-work interpretation: deciding a narrow, reversible,
  provider-free local operating default is an engineering decision, not the
  permanent-seat/model business gate. The evidence must still leave provider,
  model, permanent role/seat, squad activation, production, and automatic
  dispatch unselected.
- Next: evaluate the existing reliability, topology, role-stage, readiness, and
  CEF-7 evidence; freeze the smallest supported R01-like local binding; prove
  exact allow/deny, drift, replay, supersession, and retirement behavior; then
  run the complete local integration and stewardship gates.

## 2026-09-15 10:06 -04:00 - Reversible local operating default frozen

- Reconciled the primary reliability v2, topology, role-stage, role/squad
  operating-contract, readiness, and sealed CEF-7 evidence. The smallest
  supported configuration is R01-like controller-bound read-only Windows
  repository inventory with a shared Windows executor, inline architecture
  assurance, a task-scoped intake leader, and a distinct task-scoped reviewer.
  R02, permanent roles/seats/squads, production, and automatic dispatch remain
  held.
- Froze configuration v2 at SHA-256
  `e3a68b16ce1f6975639229e82f1a251e7e11800347738534afa6441adf41e3a2`.
  Claude Code is an observed composite integration lane, not exact-single-model
  qualification. The requested Sonnet fixture and observed background Haiku do
  not establish a permanent model/provider decision.
- Added the controller-owned lifecycle registry at SHA-256
  `53d0a16b2d5fd26b0549fc94f148c739944362787d5db30db8fcec4f4471fc42`.
  It uses controller time, has an exclusive 2026-09-22 13:15Z expiry, supports
  revocation/retirement, and tombstones v1. The current v1 evaluator now denies
  as superseded while its historical evidence still verifies.
- Ran one independent read-only review. It returned REVISE because lifecycle
  truth was requester-supplied, v1 remained executable, and receipt validation
  did not bind the exact request and ordered scenario set. Applied all three
  findings in one consolidated correction.
- The corrected provider-free rehearsal passes 50 scenarios: two exact
  preparation permits and 48 denials, zero mismatches, and zero provider,
  credential, native-tool, provider-child repository, production, outbound,
  Phone Farm, or permanent-selection actions. Receipts now bind ordinal, nonce,
  configuration, canonical request, lifecycle, and evaluation time; altered,
  renamed, missing, duplicate, reordered, replayed, or aggregate-drifted
  receipts fail closed. Rehearsal SHA-256 is
  `d93daf50b962e5de98640ab2538568239e7c29873eba01f763d0aca54342fe4c`.
- Readiness v3 SHA-256 is
  `5408754c547c00c25a32cb4ef3356d944276f1e5c4dd56f9c25690317f7611a1`.
  Quality Checker is structural-only; CEF-7 proved a temporary distinct
  reviewer function, not permanent role qualification.
- Focused configuration/lifecycle/replay/readiness tests and vet pass. No real
  provider, credential, production, outbound action, Phone Farm technical
  surface, permanent selection, or original-checkout mutation occurred.
- Next: run the broader provider-free integration gates, bind the v3 stewardship
  successor, validate the knowledge hub, and recheck cleanup/preservation.

## 2026-09-15 10:20 -04:00 - Reversible local default accepted and closed

- The exact current-slice suite passed 20 consecutive runs in 122.081 seconds.
  Full workflow projection and workflow-state v2 packages, the focused guarded
  daemon boundary, three database-backed controller success-adapter cases, the
  authority command, configuration/stewardship commands, vet, and launcher
  parsing pass.
- Stewardship v3 passed with 60 sources: 44 immutable sources inherited through
  v1/v2 and 16 v3 sources. Counts are two frozen authorities, 28 machine
  contracts, 25 accepted-evidence files, and five current-status files. It
  records one provisional local operating default, zero permanent
  configurations, and zero provider, credential, production, permanent-
  selection, automatic-dispatch, retry, fallback, or Phone Farm actions.
- The first v3 attempt correctly failed when it detected that the historical
  final-decision document was immutable v1 evidence. Restored that file to its
  exact SHA-256 `cc19141d...c677` and created the current decision as a separate
  v2 successor. No historical manifest or immutable evidence was rebound.
- The full historical qualification package remains inherited-red: v2's old
  mutable status hashes and earlier frozen source snapshots drift from the
  accepted current tree, and an existing mutation test panics after source
  loading fails. Preserved those failures honestly; the v3 successor and
  focused current slice are green.
- Knowledge-hub validation returned OK with 7 projects, 287 claims, 156 active
  claims, and the existing 146 past-review plus dated snapshot warnings. The hub
  now records the narrow reversible default without claiming permanence.
- All v3 containers remain stopped; retained evidence volumes remain. Task-
  local daemon profile, provider home, provider-child temporary roots, sentinel,
  and matching provider/broker/replay processes are absent. The retained broker
  `runtime-workspaces` tree contains task execution evidence and remains
  intentionally preserved. The original checkout remains at
  branch `codex/selfhost-pilot-20260909`, HEAD
  `60fea34b9770bd8efbf6f6f0854d4b3ec26b3e00`, with 494 tracked-dirty and 459
  untracked paths.
- No provider call, credential access, production, outbound action, Phone Farm
  technical access, permanent selection, or original-checkout mutation occurred
  in this slice. No meaningful provider-free Item 14 implementation remains
  under current authority.

## 2026-09-15 15:24Z - Accepted adoption reviewed; browser-readiness repair verified

Evidence stage: local code and isolated test database only.

- The independent upstream audit supports retaining pilot HEAD 60fea34b9770;
  86 material accepted paths were adopted with exact preimage checks. The A:
  source and owner checkout remain untouched. All 1,380 inherited dirty files
  and the accepted 60-source/three-manifest chain are preserved.
- One independent adoption review found two P2 verification defects. The
  consolidated correction routes the frozen check to its archive and aligns
  technical-stop/owner-response assertions with accepted copy. Verification
  exposed the second stale label in the same test; no product copy changed.
- Focused results: 204 Core and 23 Views tests pass; Go workflow guard,
  projection, state, bounded daemon, and three controller adapter tests pass.
  The archived stewardship checker reports valid with 60 sources. `make` is
  unavailable on this Windows host; its exact Go command passed directly.
- Reproduced the meaningful-activity HTTP 500 and malformed cursor panic.
  Immutable record revisions now match the canonical reader's constant one,
  and malformed UUIDs return a validation error. Six focused regressions pass
  on fresh PostgreSQL 17, migrated through 574. Generated newline churn was
  restored to exact inherited/accepted preimages.
- Applied isolated upstream Next.js and web OpenSSL hardening patches. The
  source-bound, loopback-only preview build is active; Browser acceptance and
  independent review of this separate slice are still pending.
- Evidence: .qualification-evidence/local-integration-20260915/; reports in
  docs/silver-mediums/UPSTREAM-INTEGRATION-AUDIT-20260915.md and
  ADOPTION-REVIEW-20260915.md. Active successor: LOCAL-INTEGRATION-PROGRESS.md.
- No provider/model trial, fake workflow replay, existing credential operation,
  production push/deploy, outbound message, Orca or Phone Farm action occurred.

## 2026-09-15 15:49Z - Reviewed local candidate passes Browser acceptance

Evidence stage: local build and isolated local runtime; no production release.

- Candidate 60fea34b9770-local-6f0af10d34dd binds 5,865 build inputs. Final
  backend/web/gateway image IDs match the build receipt and the running backend
  health response matches that source ID. PostgreSQL 17.11 has all 603 embedded
  migrations applied; all four preview services are healthy.
- The single browser-readiness review and consolidated correction are complete:
  fixed the sample table/counter defects and gateway queued-write cleanup.
  App/API/database remain internal; the fixed gateway publishes loopback only.
- Built-in Browser verified sign-in, truthful Board/Now states, issue refresh,
  incident Inbox selection/unread change, Chat's agent requirement, and manual
  creation of SAMPLE-4. Data is explicitly synthetic; no workflow was executed.
- Runtime readback paged all four sample activity records once with revision
  one, rejected a malformed cursor with HTTP 400, and counted zero agents,
  runtimes, tasks, chat sessions and owner notifications. The interactive Inbox
  read-state snapshot is recorded without assuming exclusive Browser use.
- Core 204/204, operational Views 23/23, other selected consumers 26/26,
  activity regressions 6/6, controller adapters 3/3, selected Go contracts/daemon,
  focused vet and both application builds pass. Next 16.3.4 and OpenSSL runtime
  libraries 3.5.8-r0 were read back. The separate test database was stopped by
  exact container ID; preview services remain available with stop instructions.
- Acceptance and limitations: LOCAL-CANDIDATE-ACCEPTANCE-20260915.md. Runbook:
  LOCAL-CANDIDATE-README.md. Current progress: LOCAL-INTEGRATION-PROGRESS.md.
- Remaining owner boundary is production destination/access/recovery and
  applicable runtime/credential authority. No push/deploy or provider replay.

Final closure at 15:55:26Z: source digest unchanged, all four preview services
healthy with loopback mappings, and focused-test PostgreSQL stopped. Focused
tracked-file diff checks and knowledge-hub validation pass; inherited stale
hub claim/snapshot warnings remain dated and visible. Preview tab retained.

### Controlled production recovery and access preparation — 2026-09-15 16:35Z

- Verified the owner go-live instruction in the source task and actual Alienware/Tailscale host. Started PRODUCTION-GO-LIVE-PROGRESS-20260915.md as the active successor.
- Owner directly confirmed phone password-manager recovery-key storage and retrieval; recorded custody using the accepted script. Verified matching morning restore and exact OneDrive artifact/key custody.
- Completed one bounded independent release/recovery review and its consolidated operational correction: split source/env/base+overlay inputs, additional checkpoint/image components, all-component restore verification and manual-start preflight. The reviewed application images and accepted checkpoint remain unchanged.
- Produced recovery point 20260915T162533Z-d8ea58f8 (SHA-256 332e92cdac00d33678edc06eec2b532f9404a04f4e38bf046382255d021f32f4), including 5 exact candidate/rollback/database images. Its isolated restore and OneDrive provider readback passed; temporary restore resources are gone.
- Updated the existing hidden backup/watchdog actions to the current release tuple while preserving their prior XML definitions and common state directory. Notifications are off. Production service images and route remain unchanged.
- Verified actual owner-authenticated Tailscale HTTPS access using the existing one-time local-code sign-in. No email-provider delivery is claimed, and no new mail credentials or development-code bypass were enabled.
- The isolated application migration/old-image compatibility rehearsal is running against restored data. Cutover and genuine issue execution remain pending that result.

### Private production release verified — 2026-09-15 16:41Z

- Application recovery/upgrade and prior-image compatibility passed against actual restored data. Recovered candidate image IDs and source match the reviewed build. All 31 pending migrations applied; 1 user, 1 workspace, 48 issues, 132 comments and 110 terminal tasks were preserved. Temporary containers/volumes/network/decrypted staging are absent.
- The first rehearsal reached candidate health/authenticated reads, then its fresh rollback login hit the existing resend window. Preserved that failure, corrected the harness to retain the same test session/JWT across image switches, and completed the required compatibility proof. No production or provider experiment was replayed.
- Closed exact pre-cutover backup/restore/cloud/key/watchdog gates and deployed through the supported self-host base plus pinned overlay. Deployment receipt verifies all three healthy service image IDs at 16:40:03Z; /health commit is 60fea34b9770-local-6f0af10d34dd and /healthz database/migrations are OK.
- Tailscale HTTPS remains private and routes to the existing loopback listener. The built-in Browser kept the authenticated owner session after reload; production Inbox visibly shows Recovery protected. Signup/plugins are disabled, and inherited outbound integration credentials are absent from the effective backend environment.
- Wrote the concrete release/recovery runbook and pinned old-image rollback overlay. No database down migration or destructive restore was performed. The separate sample preview remains available.
- Next: one genuine low-risk issue under fresh bounded manual runtime/capability authority, then native workflow visibility, terminal cleanup and a post-pilot recovery point. Deployment/runtime evidence does not substitute for that business acceptance.


### Operator-assisted SILV-38 delivered; native visibility remains open — 2026-09-15 19:20Z

- The real handoff was written by Claude, corrected by the operator for bounded factual wording, independently reviewed at 19:04:26Z and published as three native attachments with byte-for-byte readback. Reviewed SHA256: 2df9d83d52cf882f7b399b4b823d6d0cc975d41be6856b8838b276d6f494541c.
- Preserved seven native tasks: six failed and one independent review completed. The one zero-contact Windows launch, exact HTTP 409 reply-parent failure and rejected manual-review enqueue are retained; no failed slot or provider result was recycled.
- Source coordination explicitly approved attributed operator-assisted closure and the bounded launch/routing continuations. No automatic handoff or permanent model/role selection is claimed.
- Supported owner Browser control saved Done, but Current Picture revision 1 stayed open/failed/technical_stop; Now and Inbox retain that incident and Board groups the card In review. Native visibility acceptance failed. The missing current reply parent and supported manual-settlement gap are documented for provider-free correction before a future authorized automatic run.
- Archived exactly 21 task-created temporary agents, revoked all 21 role receipts, removed seven private provider/daemon/session profiles and all scoped daemon/task tokens. Active tasks and online runtimes are zero; original team, installed daemon, private route and reviewed application images are preserved.
- Sanitized 78 receipts/artifacts plus their manifest entered the final recovery capsule. Backup 20260915T191656Z-9fe8806c and matching isolated restore passed; final cloud readback is pending.
- Detailed outcome: SILV-38-OPERATOR-ASSISTED-OUTCOME-20260915.md. No direct workflow-state edit, forged event, public exposure, outbound delivery or Phone Farm action occurred.


### Final matching recovery verified — 2026-09-15 19:28Z

- Recovery run 20260915T191656Z-9fe8806c passed backup, isolated restore 3ae53ee64f, complete cleanup, and OneDrive dehydrate/rehydrate hash readback at 2026-09-15T19:23:53.8376871+00:00. The artifact has 377830072 bytes and SHA256 56d7f8b1391e0890daa1b143f97369740608ef1d807782aade9a5ed1e5e02deb. Key custody is verified and the watchdog reports available.
- Exact source/image/private-route/manual-start preflight passed again. No further provider call or application deployment ran.
- Final result: PRODUCTION-GO-LIVE-RESULT-20260915.md. Operator-assisted handoff delivery is verified; native Now/Inbox/Board completion remains unaccepted for the documented repair.

## Current correction update — 2026-09-15T22:52:33.049245+00:00

Source `60fea34b9770-local-9eaa711d97b1` is now deployed and healthy privately. SILV-38's supported operator-assisted completion is verified across Issue, Current Picture, Now, Inbox and Board, with all seven historical tasks unchanged and the old incident archived.

The separate SILV-39 native acceptance failed before publication: its one publisher returned pass, but a 926-byte summary exceeded the strict 600-byte limit. No independent reviewer ran. Its failed task and consumed slot remain preserved; automatic handoff is not accepted. All new temporary authority, agents, runtime and credentials are cleaned. An isolated explicit-limit prompt proposal passed provider-free tests; a fresh live attempt requires a new owner grant. Final post-cleanup recovery is verified: backup 20260915T223229Z-50923941, matching isolated restore, application/rollback rehearsal, OneDrive hash readback at 22:45:45Z and healthy watchdog; the scheduled job exited 0.

Current report: [COMPLETION-CORRECTION-RESULT-20260915.md](C:/Users/a9sho/.codex/worktrees/completion-20260915/SMK-multica-selfhost-pilot/docs/silver-mediums/COMPLETION-CORRECTION-RESULT-20260915.md). The earlier dated result below remains historical evidence.

### 32GB host restore and private-route rehearsal accepted — 2026-10-01 02:47Z

- Production on Alienware remained online and unchanged throughout the work.
  Fresh encrypted recovery run `20261001T022236Z-1154d15c` was verified and
  copied to the 32GB before any restore.
- Isolated rehearsal `r20261001a` matched revision
  `de2e901a748010ac9093586c16238465db918a8d`, its migration ledger,
  representative database counts, and all 246 restored upload path/hash
  records. Authenticated API, web, WebSocket, issue history, and an actual
  attachment byte range passed.
- The Mac created recovery run `20261001T023645Z-9113b760`; independent
  rehearsal `r20261001b` restored that host-created backup and passed the same
  application checks. Both isolated volume sets are preserved; the second
  stack remains running on loopback ports 18080/13000 for owner testing.
- The temporary tailnet-only URL
  `https://32s-mac-mini.tailda9924.ts.net:8443/` passed Alienware HTTPS and
  WebSocket transport. Its off/on rollback command was rehearsed without
  changing the Alienware port-443 production route.
- Package v10 is hash-verified on the 32GB. It removes the unused
  `SMI_DATABASE_URL` requirement after source inspection proved Multica does
  not consume that variable. The protected Intelligence environment remains
  separate; incident delivery uses the authenticated HTTP contract.
- Remaining gate: owner phone access, final no-active-work check, fresh backup,
  protected production environment creation, and explicit cutover review.

