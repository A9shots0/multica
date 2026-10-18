# Current continuation

## Live repository task and durable write-back accepted — 2026-10-02T04:56Z

- The Multica Platform project resource no longer targets the stale September
  recovery branch. It now targets remote branch
  `codex/restore-chat-ui-20261001`, currently at `6a90ca201`.
- Production issue `SILV-80` exercised a real repository-backed Codex task on
  the Alienware. The first run exposed that the otherwise healthy daemon was
  built before durable development memory reached provider prompts.
- The scheduled Alienware daemon now runs exact CLI
  `v0.4.34-silver-live-verify.1.g4d9567b4c-47-g6a90ca201`, SHA-256
  `109704D9A951AC2D42F16CB2A8419E3729A2835A3EC4EBF3EC09678D2F14352F`,
  from the existing `SilverMediums-Multica-Daemon` task. The prior binaries
  remain beside it as rollback copies.
- A follow-up run proved the `Durable development memory` block reached the
  actual Codex input. That run then exposed an ambiguous nested JSON contract:
  the agent used `repository_identity` for repository evidence, so Multica
  safely created an empty fallback instead of accepting malformed memory.
- Commit `6a90ca201` now gives agents the exact review-candidate schema and
  distinguishes `memory_items[].repository_identity` from
  `repository_evidence[].canonical_identity`. Focused daemon prompt tests pass.
- Final task `01a0faf6-b9b2-7007-b471-b0448885708c` completed with a live
  Intelligence binding and created exactly one proposal with one memory item.
  Database readback shows `pending`, unreviewed, undelivered, and zero delivery
  attempts. The issue is `In Review`, and the normal UI displays the editable
  proposal with Approve and Reject controls.
- No proposal was approved on the owner's behalf. Therefore the execution,
  context injection, candidate parsing, and owner review surface are
  runtime-verified; accepted-memory delivery and cross-issue recall remain the
  next explicit owner-review boundary.

## Mika execution restored and made durable — 2026-10-01T23:31Z

- The live Chat UI correctly exposed a real execution failure: Mika's Alienware
  runtime was offline. The production daemon had stopped at 22:16Z, while an
  unrelated isolated review daemon remained connected only to localhost.
- The existing Alienware Multica profile retained a valid credential. Its
  server and app URLs now point to the 32GB Mac, and exact daemon
  `01a06d21-3686-73c9-9b42-9e67d8b22739` re-registered the same seven runtime
  IDs against the migrated database.
- Claude Code on Alienware is logged out, so Mika's first live probe reached
  the daemon but failed provider authentication. Mika is temporarily bound to
  the already-authenticated primary Codex runtime. Mika's instructions, skills,
  permissions, and issue state were not changed.
- The verified CLI binary is installed at
  `C:/Users/a9sho/.multica/bin/multica-production.exe` and owned by Windows
  Scheduled Task `SilverMediums-Multica-Daemon`, which starts at logon and
  restarts on failure. Auto-update and auto-reload remain disabled.
- Browser acceptance passed twice: `MIKA_RUNTIME_OK` returned in 8 seconds,
  then `MIKA_DURABLE_OK` returned in 6 seconds after transferring daemon
  ownership to the scheduled task. Final readback showed Mika online with a
  fresh heartbeat, zero active tasks, and zero issues.
- Claude can be restored as Mika's provider after a one-time owner Claude Code
  login; no new credential was created in this recovery.

## 32GB Multica production and durable development memory live — 2026-10-01T16:00Z

- Production moved from Alienware to the 32GB Intel Mac. The final quiesced
  source backup was `20261001T154854Z-46aa966c`; the old Alienware PostgreSQL
  container and data remain stopped-but-intact as the rollback copy.
- The Mac runs exact backend image
  `sha256:e6ef554b649c82312a4f9213a2cb8bd412ff903537e8f0d8d5987cdf748e9019`
  with embedded source `f1f73f56a568e915d73449a4d3b02826b0073f24`, exact frontend and PostgreSQL
  images from the release manifest, and migrations through
  `608_development_memory_proposal_due_index`.
- Production acceptance passed authenticated API and WebSocket reads, restored
  issue history and attachment bytes, exact build identity, database health,
  zero active work at cutover, and the first-start workflow dispatch fence.
  Tailnet-only access is now `https://32s-mac-mini.tailda9924.ts.net/`.
- The Alienware daemon reauthenticated against the migrated database and is
  online with Claude, Codex, OpenCode, Copilot, and Grok runtimes. The workflow
  controller is enabled again. The historical Mac Studio runtime remains
  offline; this Windows task did not inspect or operate the Mac Studio.
- Durable development memory is active server-side. Repository-backed tasks
  receive bounded Intelligence context before dispatch; Intelligence
  credentials remain in the Multica backend. Terminal tasks create editable
  pending proposals. Only human-approved proposals enter the leased,
  idempotent delivery worker, and project-scoped reviewed rules can cross issue
  boundaries. A real disposable-stack canary proved cross-repository context,
  cross-issue project memory, and replay without duplicate records.
- The production service and nightly backup each have a user LaunchAgent. The
  first Mac production backup, `20261001T155809Z-2f8af5d9`, verified all local
  component hashes, copied to the 8GB Mac, matched six files in private R2, and
  restored successfully into isolated rehearsal `prodback01`. Nightly backups
  run at 04:15 after the Intelligence backup window.
- Remaining acceptance is owner sign-in/use from the normal Multica UI and
  reconnecting the Mac Studio daemon from its dedicated Mac task. Neither is a
  database, host, backup, or Alienware-runtime blocker.

## release-20260918 live; Mac Studio joined; workspace is the owner's own - 2026-09-18T02:05Z

- **Live:** backend `48cb21860ad2-local-dc1f6d3114e6`, healthy, readiness ok, 603 migrations
  unchanged, database container never restarted. Rollback target is release-20260917, archived.
- **What shipped:** an issue nobody has started now reads "Not started yet / Nothing has happened
  yet / Assign it, or ask Mika to route it" instead of "Status unavailable". The old behaviour was
  the product reporting a fault where the real state was that nothing had run.
- **Mac Studio is registered.** CLI cross-compiled from this source, installed over SSH, authenticated
  with a scoped access token (browser OAuth cannot complete over SSH), daemon running. Three Mac
  runtimes online. `phonefarm_application_specialist` and `phone_fleet_reliability` exist and are
  bound to the Mac runtime, so mac-only is enforced by where they run. SILV-41 is assigned to the
  application specialist with --no-start. No Phone Farm code or device was touched from Windows.
- **Owner's rule change:** the SSH prohibition in `C:/Dev/AGENTS.md` was removed at his instruction;
  the file now says SSH and Tailscale to the Mac are expected. The stale `C:/Dev/SMC` note stands.
- **Workspace:** his own issues only. 48 pilot/Item 14 issues and the three CRM handoff issues were
  permanently deleted at his explicit instruction; evidence remains on disk and in the backups.
  19 roles installed from roles.json, 2 more on the Mac, 3 squads from the accepted templates,
  10 company skills, 4+3 runtimes after 9 dead registrations were removed.
- **LinkHub skill rebuilt from the repo** (141 lines, from 9): layout, real table and route names,
  config names without values, invariants, safe commands, the money-spending `npm start` hazard,
  cross-app blast radius, deploy-is-not-a-working-page, failure modes. The other thin skills are
  still hub cards and need the same treatment.
- **Five defects found and fixed during the release:** an optional-field regression in the status fix
  (caught by the independent review); a hard-coded previously-deployed release that made the second
  deploy impossible; `go vet` failing a frontend-only release; the rollback rehearsal pinned to a
  deleted issue id; and the deploy gate requiring an instance nobody had used.
- **Host:** Docker's VM had no memory ceiling and had grown to 6.8 GB, killing two release runs. It is
  capped at 8 GB in `.wslconfig` and the test run is capped at four workers.

## release-20260917 deployed; option B proven end to end - 2026-09-17T22:15Z

- **Deployed and verified.** Backend `2e7bed89`, frontend `ba6bf43f`, both healthy
  and serving the qualified build `6389f0d062be-local-dffeaef888c4`. Readiness ok
  (db, migrations). Data intact: 51 issues, 149 comments, 120 tasks, 1 user,
  1 workspace. Schema unchanged at 603 migrations - this release adds none.
  Ports remain loopback-only; the database publishes no port. The database
  container was not restarted.
- **Rollback stays one step away:** the previous images (`108a608b`, `8fe461d2`)
  are in the verified archive, and the downgrade was rehearsed on a restored copy
  of real data before the deploy.
- **Qualification:** every check green on the shipped source; fresh backup with
  isolated restore and OneDrive readback; rollback rehearsal verified; preflight
  verified. The review's two findings were waived as pre-existing under an
  owner decision, and the waiver mechanically proves both files are outside this
  release's build-input delta.
- **Option B proof: passed on the first run.** Issue
  `01a0b167-c136-7a6d-a57d-76f51682b6f8` reached `done` with four completed
  tasks; Issue, Current Picture, Now, Inbox and Board all agree. Grant revoked,
  daemon profile and sessions removed, local instance stopped.
- **Host fixes this session:** PowerShell 7 was missing and the recovery tooling
  requires it (portable build at `C:/Users/a9sho/.multica/pwsh7`, pass it with
  `MULTICA_POWERSHELL7`); Docker had exhausted its address pools with 27 stale
  networks, which broke the first rehearsal.
- **Open, recorded, not urgent:** issue deletion leaves owner notification rows
  behind; migration 522 uses cascading foreign keys and a non-concurrent index.

## release-20260917 built and tested; blocked at the review gate - 2026-09-17T20:50Z

- **Candidate:** source `6389f0d06`, backend image `2e7bed89`, frontend `ba6bf43f`,
  database image unchanged. Archive verified (87 blobs) and it contains the two
  currently running images, so rollback needs no rebuild.
- **All checks pass** on the packaged source: every changed Go package
  (handler 130.5s), both type checks, core and views unit tests (5,015 views
  tests). Run now takes ~6.5 min at full parallelism, in UTC like CI.
- **Fixed this session:** workflow hand-off recovery now stops visibly instead of
  returning silently; the self-leave mark is lifted on rejoin (new regression
  test); owner notification rows are deleted with their workspace and eight
  append-only evidence tables are classified as retained; slash-absolute skill
  paths rejected on every host; five stale tests aligned with deployed contracts;
  one host-separator test made portable.
- **Review verdict: fail, on two pre-existing conditions** that are NOT in this
  release's delta and are already running in production:
  1. `DeleteIssue` does not remove `owner_notification` /
     `owner_notification_attempt`, and `ClaimNextOwnerNotification` does not
     check issue existence, so a deleted issue can still deliver a stale
     notification.
  2. Migration 522 (`issue_owner_directive`) uses cascading foreign keys and a
     non-concurrent index, against the repository migration contract. It is
     already applied on the live database.
- **Consequence:** `release-qualification.json` is `failed`, and the backup,
  restore, rehearsal and preflight actions all refuse without a verified
  qualification. Deploy readiness is blocked on an owner decision: fix the two
  pre-existing issues in this release, or record an explicit waiver that they
  predate it.
- **Not deployed. Live instance untouched** (backend `108a608b`, frontend
  `8fe461d2`).

## Owner asked to continue; host memory stops even light work — 2026-09-17T16:25Z

- **Closed:** the inactive 2026-09-14 integration pilot containers (stopped,
  not removed) and a stray log tail. MongoDB has live connections (`gads`), so
  it stays up. Memory still went from 94% to 95.7% (1.4 GB free).
- **Release `release-20260917`:** re-prepared at `c8b0adde2` (source
  `b85a7bfbc94c`, 5,914 inputs).
  - The exact delta from the deployed build: 28 added, 36 changed. The deployed
    `packages/core/api/schemas.ts` was uncommitted, so that file is reviewed in
    full.
  - The review and the operating check now run from source, before the images.
- **Stopped by the memory guard:** the Codex release review, the lightest
  remaining step. Its temporary Codex home, holding a sign-in copy, was removed
  and its authority revoked.
- **Nothing more can run here** until memory is freed: idle Claude terminals,
  Task Manager, or a reboot for the kernel-pool leak.

## Exception proof run stopped by host memory; cleaned up — 2026-09-17T08:35Z

- **Daemon identity fix:** `b9fc52f38` (scoped `mdt_` token, as #40 did).
- **The one exception run:** killed by the host memory guard during the lane
  executor, before any verdict.
- **Cleanup verified:** grant revoked, copied sign-in and daemon profile removed,
  proof database stopped.
- **Interrupted lane worktree:** its partial edits are saved as a patch and git
  status record under its evidence directory, and the worktree is removed
  (junctions unlinked first; warm install intact).
- **No further proof runs without a design review.** Host memory now blocks
  both the proof and the release web build.
- **Candidate:** blocked on GitHub Actions billing (with the owner).

## Option B built; single proof stopped at daemon identity — 2026-09-17T08:15Z

- **Option B is built and pushed** (up to `ebbbf8f39`): review broker, one-receipt
  authority helper, shared lane evidence and the harness grant flow, all with
  provider-free tests. The setup rehearsal passed.
- **The single proof run:**
  - lane passed and `implementation_ready` accepted;
  - the review grant turned the reviewer into a guarded broker execution;
  - the daemon's start was refused because the proof daemon uses a PAT, not a
    scoped `mdt_` daemon token.
- **Recorded fix:** provision a scoped daemon token as #40 did (details in
  `ISSUE-BRIDGE-ROOT-CAUSE-20260917.md`). No rerun under the single-run rule.
- **Candidate `release-20260917`:** still blocked on GitHub Actions billing (with
  the owner).

## NEEDS YOU: GitHub Actions blocked; candidate waits — 2026-09-17T08:10Z

- **Off-host build workflow:** added and registered (`f18d29ccf`):
  `.github/workflows/release-candidate-images.yml` with
  `.github/release-build/request.json`, pinning `41fe0c670` and source
  `db58ffe92626`. No secrets; 3-day private artifact.
- **Blocked:** every Actions run on the owner's private repos (this one and
  SMCrm, since at least 2026-09-16 01:36Z) ends `startup_failure` in 0 s with no
  jobs. The workflow YAML is valid, which points to an account billing block
  (minutes, spending limit or payment). The gh token cannot read billing.
  Minutes used: 0.
- **Owner options:**
  - restore Actions under GitHub Billing & plans (may cost money);
  - authorize another build host;
  - wait for free memory on this host.
- **Option B for the bridge:** chosen and scoped in
  `ISSUE-BRIDGE-ROOT-CAUSE-20260917.md`. It is built after the candidate.

## Release candidate blocked by host memory; bridge root cause found — 2026-09-17T07:55Z

- **Live instance:** unchanged and healthy. It runs the reviewed 2026-09-16 build.
- **Release tooling for a new candidate is built and committed:**
  - `scripts/release-candidate.py`, `scripts/release-qualify.py`,
    `scripts/release-recovery-rehearsal.py`;
  - `adoption-release.ps1 -Release`;
  - build-input content check with line endings normalized.
- **Candidate `release-20260917`:**
  - the clone and source manifest are prepared;
  - the backend image built;
  - **the web image build was stopped three times for low host memory**, the
    last with a single Next worker and a capped heap. The host is at about 90%
    of 31.7 GB with about 20 GB unaccounted (the known kernel-pool leak).
  - Identity, archive, qualification, backup, rehearsal and preflight wait on
    the web image.
  - To continue once memory allows, from `C:/Dev/multica`:
    - `python scripts/release-candidate.py prepare release-20260917`, then the
      image steps
    - `python scripts/release-candidate.py image release-20260917 frontend`
    - `python scripts/release-candidate.py image release-20260917 backend`
- **Issue bridge:** root cause in `ISSUE-BRIDGE-ROOT-CAUSE-20260917.md`. The bridge
  impersonates a provider CLI, but reviewer turns run as bounded, tool-free broker
  turns. Recommendation B: keep the lane as implementer and use the native bounded
  reviewer under a broker grant. No more proof runs until that is built.

## Pulse owner decisions applied — 2026-09-17T05:25Z

- **Summary cap:** kept as a known future limit; revisit soon after go-live.
- **Impersonation:** not counted. Commit `498a134` on
  `codex/crm-pulse-batch-integrity-20260917` turns collection off while
  impersonating.
  - The server already refused those writes (the first audit said otherwise;
    corrected).
  - Residual: the impersonated user's `last_activity` still refreshes.
- Patches `0001` and `0002`, plus the bundle, are under
  `.qualification-evidence/crm-pulse-batch-integrity-20260917/`. No CRM push or
  deploy.
- The issue proof still needs the owner's re-run:
  `python scripts/implementation-lane/issue-local/local_instance.py prove`

## Pulse fixed on a CRM branch; issue proof needs a re-run — 2026-09-17T00:50Z

- CRM Pulse audit and fix are in `CRM-PULSE-AUDIT-20260917.md`.
  - Branch `codex/crm-pulse-batch-integrity-20260917` is saved as a patch and
    bundle under
    `.qualification-evidence/crm-pulse-batch-integrity-20260917/`.
  - No CRM push or deploy.
  - Two business questions for the owner: the 50,000-event summary cap, and
    how to handle impersonation.
- The owner's issue proof failed because a CRM lane run was active at the same
  time. The script is fixed and lane v11 now allows one run at a time. Re-run
  from `C:/Dev/multica`:
  `python scripts/implementation-lane/issue-local/local_instance.py prove`
- Multica commits since `80a375542` are local; the push was blocked by the
  permission classifier:
  `git -C C:/Dev/multica push origin codex/workflow-completion-20260915`

## Stalled hand-off fixed (recover once, else owner incident) — 2026-09-17T00:10Z (system clock)

- `539187f5e`: a hand-off lost after task completion is now recovered to exactly
  one reviewer task, or raised as a technical stop plus an Inbox incident. Tests
  62/62 handler and 22/22 service. Details in `IMPLEMENTATION-LANE-20260916.md`.
- Push blocked locally; the owner can run
  `git -C C:/Dev/multica push origin codex/workflow-completion-20260915`.
- Next: the owner's first CRM job, auditing the Pulse page.

## Workflow test fixed; stalled-hand-off finding recorded — 2026-09-17T00:40Z

- Always-failing workflow test aligned with the accepted replay rule; the new
  test pins a finding: a hand-off whose enqueue fails and whose task then
  completes can never recover. Details in `IMPLEMENTATION-LANE-20260916.md`.
- Still parked for the owner: the daemon start for the one-step issue proof, and
  the CRM job.

## Parked on the daemon start; proof is one command — 2026-09-17T00:15Z

- **NEEDS YOU (parked):** approve starting the local daemon that lets issues
  launch the lane. Then run from `C:/Dev/multica`:
  `python scripts/implementation-lane/issue-local/local_instance.py prove`
- Always-failing workflow test: diagnosed (the test encodes the design replaced
  in `ffec7d703`), not edited; it needs a workflow recovery decision.
- First CRM job: no current, reproducible product defect fit the limits; what
  was checked is recorded in `IMPLEMENTATION-LANE-20260916.md`.

## Codex reviewer working; issue-started lane built but blocked at daemon start — 2026-09-16T23:25Z

- **Codex reviewer (lane v9):** working and proven on the real task the Claude
  reviewer failed in trial 2. `gpt-5.6-luna` at high effort caught a type error
  and a rule violation, verified server fields by reading the Go code, and
  needed no fallback. The change was applied; tests and `tsc` pass.
- **Issue bridge + local instance:** built and committed. The local server and
  owner sign-in work.
- **Blocked:** this session's safety check refused to start the local daemon
  that would let issues launch the lane. It needs the owner's own approval. The
  exact command is `python scripts/implementation-lane/issue-local/local_instance.py bootstrap`
  from `C:/Dev/multica` after `... up`.
- **CRM job:** skipped, because it depends on the blocked step.

## Lane v8 fits the memory-tight host; trial 4 meets the clean-trial rule — 2026-09-16T22:45Z

- The host stays near 90% memory (kernel pool leak, no reboot). Lane v8 reuses a
  warm dependency install via junctions, fingerprints it, waits for memory
  instead of refusing, and records its own peak (`508b86d89`).
- **Trial 4:** four clean passes and one pass after its single correction, $6.54,
  no kills and no memory waits. The lane peaks around 800 MB (1.2 GB worst case).
  By the definition written before trial 2 this is a clean trial. The only caveat
  is one false comment both reviews missed, corrected when applied.
- Applied in `91ef80938`: core API 535/535, core inbox/issues/agents 556/556,
  `tsc` clean. Not deployed.
- **Adoption:** the lane now meets its own bar for use as the standard
  **supervised** workflow on this machine. Its patches still get a human check
  before they are applied.
- Open: the pre-existing failing test
  `TestWorkflowControlCommentBeforeMissingDispatchDoesNotRaceLeaderFallback`.

## Lane v7 frozen; trial 3 cut short by host memory; crash fixed; old checkout compared — 2026-09-16T21:00Z

Main location `C:/Dev/multica`, pushed to the private remote.

- **Cancel-task crash fixed** (`f7ef89c17`), plus the same panic class in the
  workflow leader check. Verified on a disposable database.
- **Lane v6 then v7.** v6 added the server-contract check and byte-exact
  patches; v7 adds regression checks and narrows the contract check. 42 tests.
  Details in `IMPLEMENTATION-LANE-20260916.md`.
- **Trial 3 incomplete.** Runs 1-2 passed their gates but carried defects found
  at apply time (an existing test broken, a test that did not typecheck), fixed
  and applied in `480f3f4e4`. Runs 3-4 were killed by the session for low memory
  while other Claude sessions used about 4.4 GB; run 5 never started.
- **Trial 4 pre-registered** (`1c93a0d64`) on v7 and not yet run. It needs the
  host to keep memory headroom through a run; both kills came after admission.
- **Old checkout compared read-only.** 1,696 of 1,749 files are identical, 51
  unique files are archived on `A:`. Report: `OLD-CHECKOUT-COMPARISON-20260916.md`.
- Pre-existing failing test to investigate:
  `TestWorkflowControlCommentBeforeMissingDispatchDoesNotRaceLeaderFallback`.

## Main location is now C:/Dev/multica — 2026-09-16T20:05Z

- Private repository `A9shots0/silver-mediums-agent-control-plane` created by the
  owner; only `codex/workflow-completion-20260915` pushed, at `80a375542`
  (`ls-remote` matches, visibility PRIVATE).
- Cloned to `C:/Dev/multica` with `upstream` = `multica-ai/multica`. Bundle:
  `A:/Multica-Recovery/multica-canonical-20260916T195744Z.bundle` (verified).
- In the clone: Go server and CLI build; `@multica/core` typecheck clean; 467/467
  core API tests; lane 36/36, freeze v5 intact.
- Registered in the knowledge hub as the primary path. The temporary `.codex`
  worktree is superseded. The public fork `A9shots0/multica`, the dirty
  `C:/Dev/SMK-multica-selfhost-pilot` checkout and the private deployment were
  not touched.
- Still open: owner approval of the lane change before trial 3; the `CancelTask`
  malformed-id panic; a read-only comparison of the old dirty checkout.

## Trial 2 complete: not clean; lane stays supervised-only — 2026-09-16T19:50Z

**Current outcome: five real code changes ran through lane v5. All passed every
mechanical gate first time with no correction, but the human check found a real
defect in run 5 that the tests and reviewer missed. Under the definition written
before the trial, the trial is not clean, so the lane is not adopted as the
standard workflow.** It remains GO for supervised use. Details:
`IMPLEMENTATION-LANE-20260916.md`, "Trial 2 result on lane v5".

- Applied after human check: runs 1-4 (`c5c2aec0f`, `f4369f279`, `b6990ef29`,
  `3d97bd9a0`); core API tests 467/467, core typecheck clean. Not deployed.
- Not applied: run 5 (chat task messages would render empty).
- Total $3.92.

Open items:

1. **Online copy blocked by this session's safety check.** The owner said "do
   it now"; `gh repo create` was still refused. Nothing created or pushed. The
   owner can run it directly (commands in the final report). After that: clone
   to `C:/Dev/multica`, bundle to `A:/Multica-Recovery`, verify the build, and
   record it as the main location.
2. **Lane change needed before trial 3 (owner decision):** a deterministic check
   of claims against the real system, such as schema fields against server JSON
   tags, and byte-exact patch evidence.
3. Real bug found on the side: `CancelTask` panics on a malformed task id
   (`server/internal/handler/daemon.go:5434`).

## Lane revision 2 built and frozen; second trial not started — 2026-09-16T18:30Z

**Current outcome: the owner-approved lane revision is built, tested (36/36) and
frozen as v5, with a clean-pass definition written before any run. No provider
run has started.** Details: `IMPLEMENTATION-LANE-20260916.md`, section "Lane
revision 2".

Waiting on, in order:

1. **Online copy (owner's own confirmation needed).** Requested through another
   session: private repository `A9shots0/silver-mediums-agent-control-plane`,
   push only `codex/workflow-completion-20260915`, then clone to
   `C:/Dev/multica`, bundle to `A:/Multica-Recovery`, verify build, record as the
   main location. Read-only preparation done: the name is free, `gh` is
   authenticated as `A9shots0` with `repo` scope, the public fork
   `A9shots0/multica` exists and is untouched, and the branch tip is now `8ce14f56f` (the revision commits added since the check at `8e655b9df`).
   Nothing has been created or pushed. A relayed approval is not used for this.
2. **Host memory.** 91% load at 18:26Z; the lane refuses below 4 GB available
   physical or commit memory.
3. **Five task specs.** Selection started; see the lane document for the
   handler-package constraint and upstream bugs already checked.

Found and fixed while building: agents given `go test` could have written into
the native PostgreSQL on `localhost:5432`; and credential-shaped environment
variables for four other apps, plus this session's messaging token, were
readable by the first trial's executors (no evidence any did). Both are now
withheld.

## Implementation-lane repeatability trial COMPLETE — 2026-09-16T09:05Z

**Current outcome: five real code-changing tasks completed through the new lane.
Each passed on the first attempt of its final run with no correction. The
recommendation is GO for supervised use and NO-GO as the unsupervised standard
workflow yet.** Full result and reasoning: `IMPLEMENTATION-LANE-20260916.md`.

### What changed in the product

| Commit | Change |
|---|---|
| `243c5a6df` | missing chat page-purpose translations (ja, ko, zh-Hans) |
| `7945896ae` | 180 missing inbox owner-control translations; parity suite 160/160 |
| `3cb3f1b27` | workspace leave awaits the server before navigating, with a pending-leave guard |
| `554a48546`, `ccdb8466e` | text-preview allowlist generated from one JSON source; inaccurate CI claim corrected by hand |
| `1298a8660` | slug lookup documented as unsafe to cache; slug header now wins over query, matching the sibling resolver |

None of these is deployed. The deployed build remains
`60fea34b9770-local-24b0121ef7a0`. Nothing is pushed; there is no Silver Mediums
remote.

### The three answers

- **Repeats reliably?** Not yet proven. The agent repeated; the lane was fixed
  four times mid-trial and only three runs used its final version.
- **Work good?** Yes, with human verification. Two non-obvious correct
  judgments, but the same-provider reviewer approved one false premise, missed
  one false guarantee and passed one unrequested change.
- **Worth the cost?** Yes on money: $3.29 for the five final runs, $7.77
  including all failures. Unclear on operator time: setup takes 1-5 minutes per
  run and was killed twice by host memory, and Go changes needed hand
  verification.

### Next

Revise the lane: orphan sweep, `go` access for Go tasks, a mechanical
test-fails-before check, no JavaScript install for Go tasks, and a reviewer that
must list out-of-scope changes. Then run five more tasks on that unchanged
version. Adopt it as standard only if that run is clean.

### Reconciliation into a durable canonical checkout — identified, not performed

- The accepted branch `codex/workflow-completion-20260915` (18 commits ahead of
  the canonical checkout's `60fea34b9`) already lives in the shared object store
  at `C:/Dev/SMK-multica-selfhost-pilot/.git`.
- `C:/Dev/SMK-multica-selfhost-pilot` itself is on
  `codex/selfhost-pilot-20260909` with 125 modified and 1,255 untracked files of
  uncommitted owner work. It must not be reset, overwritten or checked out over.
- There is no Silver Mediums remote, so the accepted branch exists on this disk
  only.

**Exact next step:** from the shared repository, create a stable branch at the
accepted commit and a new canonical checkout for it, without touching the dirty
checkout:

```text
git -C C:/Dev/SMK-multica-selfhost-pilot branch silver-mediums/canonical <accepted tip>
git -C C:/Dev/SMK-multica-selfhost-pilot worktree add C:/Dev/multica silver-mediums/canonical
git -C C:/Dev/SMK-multica-selfhost-pilot bundle create A:/Multica-Recovery/multica-canonical.bundle silver-mediums/canonical
```

Then verify the build in `C:/Dev/multica`, and only after that record it as
Multica's primary path in the knowledge-hub registry. The gitignored evidence
tree stays backup-only; its verified archive is at
`A:/Multica-Recovery/Evidence-Archive/`. The 125 + 1,255 uncommitted files in the
old checkout need a separate read-only comparison against the accepted branch
before anything there is discarded. Creating a real Silver Mediums remote is an
owner decision.

## Implementation lane built, proved and frozen — 2026-09-16T05:50Z

**Current outcome: an agent has changed real code under full safeguards, once,
successfully.** The document-only repeatability trial was paused on the owner's
decision after this session established that the accepted publication workflow
cannot change code at all. A five-task repeatability trial on the new lane is in
progress.

Records: `IMPLEMENTATION-LANE-20260916.md` (lane, first proof, trial plan),
`ADOPTION-MILESTONE-RESULT-20260916.md`, `INITIAL-OPERATING-CONFIGURATION-20260916.md`.
Evidence: `.qualification-evidence/implementation-lane/`.

### The finding that changed the plan

The accepted publication workflow's provider child runs with no tools and no
repository access, and its broker may only publish a frozen pre-approved
document or record a review of that publication. Every accepted end-to-end
"implementation → review" proof used a **fake** provider. It could never have
performed day-to-day engineering work. Repeating it five times would have
measured document verification, not getting work done.

### Accepted state is now preserved

- 1,635 files committed across four commits on `codex/workflow-completion-20260915`;
  working tree clean; nothing pushed; the other three checkouts untouched.
- The gitignored 1.7 GB `.qualification-evidence/` tree is archived to
  `A:/Multica-Recovery/Evidence-Archive/` with a per-file SHA-256 manifest,
  verified by reading all 340 members back out of the archive. The 28 existing
  encrypted backups are untouched. Tool: `scripts/archive-qualification-evidence.py`.

### The lane

`scripts/implementation-lane/`, frozen in `FROZEN.json`. One run is an isolated
detached worktree, offline lockfile-pinned dependencies, an executor session
confined by the CLI's `--restricted` mode and a tool allowlist, the **harness**
running the task's own verification command, an independent read-only reviewer
session, at most one bounded correction, then cleanup. Authority is minted fresh
per run, expires, is revoked at cleanup, and grants no production, deployment,
outbound, Phone Farm, cross-repository or permanent-selection capability. A
containment check proves after the fact that nothing outside the allowed paths
was touched. 16 provider-free tests.

**Authority here is reusable eligibility policy, never reusable execution
authority.** Every run mints its own.

### First proof

Run `20260916T054524Z-3f9f7e6f`: six missing chat translations in ja, ko and
zh-Hans. Passed on the first attempt, no correction, 49.2 s executor and 4.4 s
reviewer, $0.3521 cost-equivalent, 3 files touched and 0 outside scope, cleanup
verified. Committed as `243c5a6df`.

Two harness failures are recorded before it and were never retried into a
success: `pnpm` unresolvable without a shell, and cp1252 subprocess decoding.
Both times authority revocation and worktree removal worked correctly.

### Trial in progress

Task 1 chat parity (done). Task 2 inbox parity, 180 keys — volume and long
context. Task 3 the workspace-leave known debt at `CLAUDE.md:179`, a **judgment**
task with no existing test, where the executor must decide what correct is,
write the test, then fix it. Tasks 4 and 5 to be selected from recorded evidence.

The go/no-go will answer three questions separately: does it repeat, is the work
good, is it worth the cost.

### Boundaries carried forward

The lane produces a reviewed patch; it never commits, pushes or applies its own
work. It is not yet integrated with the Multica server, task queue or workflow
states. Reconciling the temporary worktree into a durable canonical checkout is
identified but deliberately not executed: committed history already shares the
object store with `C:/Dev/SMK-multica-selfhost-pilot`, and the remaining step is
deciding whether the gitignored evidence tree becomes a tracked archive or stays
backup-only. Orca is retired and is not used or proposed anywhere.

## Controlled operational adoption DEPLOYED and verified — 2026-09-16T03:12Z

**Current outcome: implemented, independently reviewed, corrected, privately
deployed and verified. Nothing is outstanding.** Source
`60fea34b9770-local-24b0121ef7a0` runs on the private Alienware/Tailscale
instance. Zero provider calls, zero credential reads, zero dispatched issues.

Full report: `ADOPTION-MILESTONE-RESULT-20260916.md`. Configuration record:
`INITIAL-OPERATING-CONFIGURATION-20260916.md`. Evidence:
`.qualification-evidence/adoption-20260916/` and
`.qualification-evidence/production-adoption-20260916/`.

### What is now true

- One initial operating configuration is active and revocable:
  `silver-mediums-initial-operating-configuration-20260916`, SHA-256
  `dadf6a215f9ed8fb6d4ed5b957a6be143e3226dab080452c4c9d63454aaffa6d`, valid
  2026-09-16T00:00:00Z to 2026-09-30T00:00:00Z. It admits one owner-started
  internal engineering issue at a time on this repository and host, routed
  through the controller with separate execution and independent-review
  sessions, two provider tasks, zero corrections, zero retries, zero permanent
  agent seats.
- Twelve owner-gated conditions stop work and wake the owner; an unrecognised
  condition also stops it. Automatic dispatch, public access, production access,
  other production systems, other repositories, outbound actions and Phone Farm
  access are all false and validator-enforced.
- Claude is **not** permanently selected. Both lanes share one provider, so
  review independence is session-level only; the exact served model is unproven;
  the requested effort was never observed; recovery-sensitive authority is
  withheld on the R04 failure; release, rollback, recovery and production
  verification stay owner-operated.
- The record is a declarative contract checked out of band, **not** a runtime
  interlock. No server, controller or daemon code path consults it before work
  starts. Verify it with `cd server && go run ./cmd/initial-operating-configuration-check`.
- The two recorded defects are fixed. PowerShell 7 is resolved without relying
  on PATH, cannot be satisfied by PowerShell 5 or a Store stub, and fails loudly
  on a bad override; no identity guard changed. The OneDrive verifier waits for
  the artifact to settle before hashing and re-issues the dehydrate verb, with
  the exact-hash requirement unchanged — a settled, complete, non-matching copy
  still fails, promptly.
- The app shows a secondary identity: "Silver Mediums Agent Control Plane /
  Built on Multica" in the sidebar footer and on all three sign-in steps, with an
  About surface. Multica's mark, name, copyright and attribution are unmodified
  and now more visible than before; the signed-in app previously showed no
  Multica copyright at all.

### State after this run

- Deployed and verified: health ok, readiness `db: ok` / `migrations: ok`, data
  preserved exactly (1 user, 1 workspace, 51 issues, 149 comments, 120 tasks,
  0 runnable, 603 migrations, 10 active agents, 0 online runtimes).
- Boundaries intact: loopback-only listeners, private Tailscale route, no public
  Funnel, signup disabled, workspace creation disabled, SMS off.
- Rollback was proved **before** deploying: backup `20260916T025449Z-ea0d859d`,
  isolated restore `ae3e246f4e`, and rehearsal `app8701907fc9` booting both the
  new and the rollback image against the restored database with 603 migrations
  unchanged and identical row counts. Rollback target is
  `60fea34b9770-local-9eaa711d97b1` via `scripts/adoption-release.ps1 -Action Rollback`.
- Closing recovery point `20260916T030645Z-89971836` is verified end to end,
  including OneDrive readback and a healthy watchdog.
- One independent review returned REVISE with 15 findings; all 15 were addressed
  in one consolidated correction. No second review is claimed.
  `provider-unavailable: orca runtime_unavailable | fallback: in-session
  independent agent instance | lane: independent review`.
- Item 14 acceptance evidence, SILV-38, SILV-39 and every historical failure are
  untouched. Pre-correction harness bytes are preserved and hash-verified.

### Known and recorded, not outstanding

- Nine `agentqualification` tests and the `inbox`/`chat` locale-parity tests
  were already failing before this change; the failing set was captured with and
  without the new files and is byte-identical. Historical frozen manifests pin
  superseded hashes on purpose.
- A probe that imported `scripts/completion-recovery-rehearsal.py` executed it
  (these scripts run on import). It passed, cleaned up, and left production
  untouched, but it overwrote one prior rehearsal receipt. Run these scripts;
  never import them.

### Boundaries carried forward

Starting a real issue under this configuration is a separate owner action. A
permanent role/model selection, a wider eligible work class, the automatic
correction cycle, a second review provider, recovery-sensitive authority, public
exposure, outbound messages, another repository or host, and any Phone Farm work
from Windows each need separate explicit authorization and their own evidence.

## Native publication acceptance PASSED — 2026-09-16T01:24:29+00:00

**Current outcome: accepted and closed.** One publisher turn and one
automatically routed independent reviewer turn completed natively on the exact
private release. Issue #40 `01a0a7a4-2415-7ada-9a47-667a5bc6a094` reads Done
consistently across Issue, Current Picture, Now, Inbox and Board, with no
operator assistance, no technical stop, no retry and no correction. This is the
first accepted fully automatic completion; the Item 14 native-visibility gap is
closed.

The correction worked: the publisher wrote a 269-character ASCII summary and the
reviewer 300, both inside the 10–400 character instruction and far under the
retained 600-byte ceiling. SILV-39's 926-byte failure is what this replaced.

Full report: `NATIVE-ACCEPTANCE-RESULT-20260916.md`. Evidence:
`.qualification-evidence/native-acceptance-v3-20260915/`.

### State after this run

- Fresh bindings were used throughout: harness
  `scripts/production-pilot/completion-acceptance-v3/`, state
  `.qualification-evidence/production-pilot-SILV-38-completion-acceptance-v3/`,
  daemon/profile `silv38-production-pilot-completion-acceptance-v3-20260916`,
  new binaries, new issue, new authority contract. Nothing from the cancelled v2
  trial or the consumed SILV-39 slot was reused.
- Cleanup is complete and independently verified (`fully_clean: true`): 3 of 3
  authority receipts revoked, runtime deregistered, zero daemon tokens, both
  temporary profiles removed, owner session removed, three temporary agents
  archived, zero pilot processes, zero active tasks. The owner's own
  subscription credential file is untouched.
- Recovery closed on run `20260916T011317Z-a6a72487`: backup verified, isolated
  restore verified, application/rollback rehearsal verified against both images
  with 603 migrations unchanged, OneDrive readback verified at 01:23:37Z, and a
  healthy watchdog.
- Production was never restarted: `started_at 2026-09-15T22:12:43Z` before and
  after, all three containers healthy on the same images. Workspace totals moved
  only by this run: 50 → 51 issues, 118 → 120 tasks.
- SILV-38 and SILV-39 are byte-identical before and after. All earlier failed
  evidence and the v2 `stopped_by_owner` record are preserved untouched.

### Two defects found and fixed in passing

- The authority helper pinned the superseded v1 issue title, so no v2 or v3 run
  could ever obtain receipts. Its expected title now names the acceptance issue;
  the check is otherwise unchanged.
- The harness stops its daemon through `pwsh`, which is not on this host's PATH,
  so cleanup reported one failure. The process was stopped separately under the
  same identity guards and verified absent. `pwsh.exe` exists at
  `C:\Users\a9sho\.cache\codex-runtimes\codex-primary-runtime\dependencies\native\powershell\`.

### Boundaries carried forward

- This accepts one bounded run. It does not authorize a permanent role/model
  selection, wider automatic execution, public exposure, or any further provider
  run. Each needs separate explicit authorization.
- Acceptance evidence is API and projection evidence read from the endpoints and
  the lane rule the clients use, not a rendered screenshot. The Orca Browser path
  was excluded by the owner's instruction for this run and the Chrome extension
  was not connected.
- Known follow-up, not urgent: `Verify-OneDriveRecoveryCopy.ps1` hashes the local
  cloud copy without retrying, so it fails once while a ~361 MB artifact is still
  uploading. It passed on retry roughly ten minutes later.

## Fresh bounded native trial v2 STOPPED by owner — 2026-09-15T23:02:18.159389+00:00

**Current outcome: authorization revoked before any live setup or provider call.** Zero publisher/reviewer runs, zero new issues/agents/runtimes/tokens, zero active tasks and zero online runtimes. Production remains 50 issues and 118 historical tasks on source 60fea34b9770-local-9eaa711d97b1. SILV-38 assisted completion and the failed SILV-39 observation are preserved; native automatic success remains unaccepted.

Exact interruption: copied the frozen harness to scripts/production-pilot/completion-acceptance-v2, applied only the approved summary instruction and fresh binding names, and ran provider-free checks. Thirteen Go tests plus four byte-boundary subcases and six mocked controller tests passed. The local test process finished; no temporary execution remains. prepare-resources, build-bound, daemon provisioning, authority issuance, credential copying and launch were never invoked. No temporary runtime, authority, profile or credential required removal; absence was verified. Local qualification is marked stopped_by_owner so the harness's existing qualification gate rejects execution. Preparation and evidence are retained.

### Claude-ready continuation

- Worktree: C:/Users/a9sho/.codex/worktrees/completion-20260915/SMK-multica-selfhost-pilot. Original owner/A: worktrees and all failed histories remain preserved.
- Read .qualification-evidence/native-acceptance-v2-20260915/interruption.json and test-stop.json first. Fresh state is .qualification-evidence/production-pilot-SILV-38-completion-acceptance-v2; it contains no issue, launch, consumed slot, authority or credential binding.
- Treat this trial as cancelled. Do not resume the prepared harness, re-enable its qualification, call a provider, start a reviewer/retry, deploy or begin another slice. Further work requires a new explicit owner instruction. The earlier fresh-run approval has been revoked.
- Latest verified recovery remains 20260915T223229Z-50923941: isolated restore, corrected/rollback application checks, OneDrive hash readback at 22:45:45Z and healthy watchdog. No production change occurred after that closure in this interrupted trial, so no additional recovery run was started.
- The prompt proposal is locally tested only. Automatic completion is still unaccepted. The earlier correction result report and SILV-39 failure evidence remain authoritative for completed work.


Updated: 2026-09-15T22:52:33.049245+00:00
Stage: corrected private release and SILV-38 completion accepted; SILV-39 native acceptance failed; cleaned; final recovery verified; fresh native grant required

## Active controlled completion correction

Active worktree: `C:/Users/a9sho/.codex/worktrees/completion-20260915/SMK-multica-selfhost-pilot`, branch `codex/workflow-completion-20260915`. Preserve all original owner/release/A: worktrees and failed runs.

Production source `60fea34b9770-local-9eaa711d97b1` is deployed and healthy. Supported SILV-38 completion event `01a0a721-e653-7ecd-91a9-e381dcd1eabb` passed API and built-in Browser acceptance across Issue/Current Picture/Now/Inbox/Board, with explicit operator assistance and all seven original tasks unchanged.

SILV-39's one publisher role run returned pass but a 926-byte summary violated the 600-byte guard. No publication or reviewer ran. Its failed task and consumed slot remain immutable; native automatic acceptance is not achieved. The explicit-summary-limit prompt proposal passed local tests in an isolated copy and is not applied to the used harness. A new live attempt requires a fresh owner grant; do not reuse the consumed slot or silently enable retries.

All temporary runtime bindings, task capability, agents, profiles and credentials are cleaned. Terminal counts are zero for active execution, online runtimes and outbound/Autopilot lanes. Final post-cleanup recovery is verified: backup 20260915T223229Z-50923941, matching isolated restore, application/rollback rehearsal, OneDrive hash readback at 22:45:45Z and healthy watchdog; the scheduled job exited 0. All currently authorized release, cleanup and recovery work is complete. Do not ask again for phone key custody and never read/display/hash the key.

Current result: `COMPLETION-CORRECTION-RESULT-20260915.md` in the correction worktree. Coordinating task: `01a06e85-9068-74f1-9c2d-64fc61dd7bd8` (Audit Multica workflow state). Historical material below is superseded by this active state.

## Earlier controlled pilot context

The accepted Items 1–14 implementation is locally integrated and browser-tested.
The owner authorized the first controlled Alienware/Tailscale production release
and one genuine low-risk issue after recovery and custody gates are verified.
The active plan is PRODUCTION-GO-LIVE-PROGRESS-20260915.md. Reviewed preview:
http://127.0.0.1:13004/local-candidate-samples/issues
Acceptance: LOCAL-CANDIDATE-ACCEPTANCE-20260915.md
Run/stop and sample sign-in: LOCAL-CANDIDATE-README.md

## Verified milestone

- The upstream divergence/extension audit is complete. Keep pilot HEAD
  60fea34b9770bd8efbf6f6f0854d4b3ec26b3e00 for this candidate. The full accepted
  source has 21 textual conflict paths and 34 new migration-number collisions
  against upstream a843b44a48b4cb3d94c08e0c56ddef1fc1105257.
- Adopted exactly 86 material paths from the accepted A: source after verifying
  both source and destination preimages. Left 151 newline-only paths untouched.
  All inherited dirty contents are preserved in the task evidence directory.
- The accepted 60-source Item 14 stewardship chain is archived byte-for-byte;
  its real checker passes against that snapshot. The CEF-7 result remains sealed.
- Independent adoption review verified 86/86 paths, 1,380/1,380 inherited
  backups, 60/60 archived sources and 3/3 manifests. Its consolidated correction
  routes the Makefile check to the frozen archive and aligns two stale test
  labels with accepted product copy. The archive and all 23 Views tests pass.
- Go workflow guard/projection/state, bounded daemon, three database-backed
  success-adapter tests, and 204 Core schema tests pass.
- Fresh task-local PostgreSQL 17 is migrated through 574. Meaningful activity
  now uses revision one for immutable records and validates malformed cursor
  UUIDs without panic. Six focused regressions pass after reproducing both bugs.
- The two isolated upstream web hardening patches are applied and read back:
  Next 16.3.4 and libssl3/libcrypto3 3.5.8-r0. Standalone web/TypeScript and Go
  builds pass. Another 26 Inbox/Chat/navigation tests and focused Go vet pass.
- One independent browser-readiness review and its consolidated correction are
  complete. Fixed sample table/counter handling and gateway stream cleanup.
  Candidate 60fea34b9770-local-6f0af10d34dd binds 5,865 build inputs; the three
  built image IDs and running health source match. Four services are healthy;
  the preview database has all 603 required migrations applied.
- Built-in Browser passed sign-in, Board/Now truth, issue refresh, Inbox detail,
  Chat's agent requirement and manual SAMPLE-4 creation. Four activity records
  paginate exactly once and malformed cursor HTTP 400 is verified in runtime.
  Samples are static/local; agents, runtimes, tasks, chats and owner notifications are
  zero. The separate focused-test database is stopped; preview remains available.

## Earlier closure and next actions (historical)

- Private production is deployed and healthy on the exact reviewed images.
  The owner confirmed recovery-key retrieval on the phone. Pre-release recovery,
  application/rollback compatibility and cloud readback passed.
- SILV-38's real CRM handoff received attributed operator factual corrections,
  an independent pass and 3/3 uploaded attachment hash readbacks. Its manual
  status is Done. Seven native runs remain: six failed and one review completed.
- Native completion visibility is NOT accepted. Current Picture/Now and Inbox
  retain the earlier technical stop; Board consequently groups the card In review.
  Do not describe this as a fully automatic successful workflow.
- All 21 temporary bindings are revoked and temporary agents archived. Seven
  profiles and all scoped daemon/task credentials are cleaned; execution is idle.
- Final backup 20260915T191656Z-9fe8806c and its isolated restore passed; matching
  OneDrive readback passed at 19:23:53Z. The final matching recovery receipt is
  recorded in PRODUCTION-GO-LIVE-RESULT-20260915.md.

1. Current release/recovery closure is complete with native visibility explicitly
   unaccepted. Read PRODUCTION-GO-LIVE-RESULT-20260915.md before continuing.
2. Before a future automatic pilot, make provider-free corrections for current
   comment reply-parent binding and supported manual settlement of legacy
   technical stops across Now/Issue/Inbox/Board. Preserve all failed runs.
3. No provider replay or permanent role/model selection is authorized by this
   closure. A future bounded provider experiment needs separate authority.
4. Retain the accepted A: checkpoint read-only. The existing sample preview may
   remain available; do not reset the dirty owner or task checkout.

## Evidence and retained checkpoints

- Active integration: .qualification-evidence/local-integration-20260915/
- Exact accepted checkpoint: .qualification-evidence/item14-accepted-checkpoint-20260915/
- Independent upstream audit: UPSTREAM-INTEGRATION-AUDIT-20260915.md
- Item 14 role-build history: ITEM-14-ROLE-BUILD-PROGRESS.md. Integration progress
  continues in LOCAL-INTEGRATION-PROGRESS.md; historical Next paragraphs are not
  active instructions.
- Authoritative A: source remains read-only:
  A:\Silver-Mediums-Workspace\worktrees\multica\integration-preservation-pilot-20260914T232814Z

## Boundaries

Production deployment and one controlled real issue are now authorized by the
owner's September 15 go-live grant. Close recovery/custody before cutover. Do not
replay completed experiments, choose permanent roles/models, enable unrestricted
automatic execution, use new credential sources, expose secrets or public access,
send real-person messages/SMS, use Orca, or access Phone Farm from Windows.
The provisional R01-like default expires exclusively at 2026-09-22 13:15Z;
preserve its preparation-only semantics. The new pilot needs fresh applicable
runtime/capability receipts within the controlled go-live authority.

Stop only at a genuine unresolved owner-only business, destructive/data,
credential, outbound, or production boundary. Local code, remote Git, deployment,
runtime observation and business acceptance remain separate evidence claims.
