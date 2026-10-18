# Controlled production go-live

## Current correction update — 2026-09-15T22:52:33.049245+00:00

Source `60fea34b9770-local-9eaa711d97b1` is now deployed and healthy privately. SILV-38's supported operator-assisted completion is verified across Issue, Current Picture, Now, Inbox and Board, with all seven historical tasks unchanged and the old incident archived.

The separate SILV-39 native acceptance failed before publication: its one publisher returned pass, but a 926-byte summary exceeded the strict 600-byte limit. No independent reviewer ran. Its failed task and consumed slot remain preserved; automatic handoff is not accepted. All new temporary authority, agents, runtime and credentials are cleaned. An isolated explicit-limit prompt proposal passed provider-free tests; a fresh live attempt requires a new owner grant. Final post-cleanup recovery is verified: backup 20260915T223229Z-50923941, matching isolated restore, application/rollback rehearsal, OneDrive hash readback at 22:45:45Z and healthy watchdog; the scheduled job exited 0.

Current report: [COMPLETION-CORRECTION-RESULT-20260915.md](C:/Users/a9sho/.codex/worktrees/completion-20260915/SMK-multica-selfhost-pilot/docs/silver-mediums/COMPLETION-CORRECTION-RESULT-20260915.md). The earlier dated result below remains historical evidence.

Updated: 2026-09-15 17:20Z
State: reviewed build live privately; controlled genuine issue remains

## Authority and outcome

The owner authorized go-live at 2026-09-15 16:00:55Z in task
01a06e85-9068-74f1-9c2d-64fc61dd7bd8. The task's delegated continuation selects
the Alienware host, private Tailscale access, and one genuine low-risk Silver
Mediums issue with controlled manual dispatch. Verify backup, isolated restore,
off-machine artifact custody and separate key custody before cutover. Preserve
Items 1–14, the reviewed candidate and the existing owner checkout.

No public exposure, unrestricted automatic execution, real-person messages/SMS,
new credential source/exposure, Orca, or Windows Phone Farm technical access.

## Exact candidate

- Git base: 60fea34b9770bd8efbf6f6f0854d4b3ec26b3e00 plus preserved local changes.
- Reviewed build: 60fea34b9770-local-6f0af10d34dd.
- Build inputs: 6f0af10d34dd77de23dc5ff4a67ef1a79270b33671a8f9a89fe694b5303de16e.
- Acceptance and image identities: LOCAL-CANDIDATE-ACCEPTANCE-20260915.md.
- The sample preview at loopback port 13004 remains available and separate.

## Fresh host and recovery evidence

- A9DREW identifies as Alienware m16 R1. Tailscale is running and online.
- Private hostname: a9drew.tailda9924.ts.net. Existing HTTPS Serve route points
  to loopback port 3000; no AllowFunnel entry appears in current Serve config.
- Existing `multica` containers use the original C: checkout's compose files,
  volumes `multica_pgdata` and `multica_backend_uploads`, and loopback ports
  3000/8080. They have not been changed by this release phase.
- Latest scheduled backup: 20260915T103923Z-04a0a00a, 21,799,976 bytes,
  SHA-256 7cad3605097dfd386a5c8dbe7eb302bb42ecf9f4810d4046ef6eb8e22edeb125.
- OneDrive receipt verifies exact dehydrate/rehydrate/hash readback at
  2026-09-15 10:52:30Z. This closes the earlier artifact-custody gap for that run.
- Matching restore remains unproven; the previous successful restore belongs
  to an older September 11 backup. Fresh watchdog reports platform available.
- Separate key custody remains pending. The owner has been asked to confirm
  phone password-manager storage and retrieval without sharing the key.

## Checklist

- [x] Confirm go-live authority and actual host/private route
- [x] Identify current backup, artifact-custody and watchdog receipts
- [x] Verify a matching isolated restore and cleanup
- [x] Confirm separate recovery-key custody
- [x] Prepare and review exact deployment configuration and rollback
- [x] Close current pre-cutover recovery point and candidate recovery coverage
- [x] Deploy exact reviewed images through the supported self-host compose path
- [x] Verify health, private Browser route and production authentication
- [ ] Run one genuine issue through assignment, progress, review and completion
- [ ] Verify Inbox/Issue/Board visibility and bounded recovery behavior
- [ ] Record release, recovery, runtime and business acceptance separately

## Next work

Prepare one genuine low-risk issue and current per-run runtime/capability
receipts. Execute through controlled manual dispatch, verify review/completion
in Inbox/Issue/Board, clean temporary authority, and capture a post-pilot backup.

Evidence will be retained under .qualification-evidence/production-go-live-20260915/.

## Recovery/access milestone at 16:35Z

- The owner saved the existing recovery key in the phone password manager and
  confirmed retrieval. The accepted custody script recorded that confirmation;
  neither the recovery key nor a derived key value was displayed or changed.
- The morning backup restored 137 tables and 6,847 source files in 18.236s;
  migration/count/upload checks passed and temporary resources were removed.
- One independent release/recovery review is complete. Its consolidated
  correction adds explicit source/env/Compose-overlay backup inputs, verification
  of optional archived components, exact candidate/rollback image recovery, and
  a startup inventory gate. Accepted Items 1–14 semantics remain unchanged.
- The image archive contains 5 exact Linux/amd64 candidate/rollback/database
  image indexes and 83 verified content blobs. Its SHA-256 is
  bd31b7a69a6882a1a48a2df4fcc2c3b9371072d15bd95c9e1608d89d55a8bbd4.
- New recovery point: 20260915T162533Z-d8ea58f8, 377,246,168 bytes, SHA-256
  332e92cdac00d33678edc06eec2b532f9404a04f4e38bf046382255d021f32f4.
  It includes current source, 60-source accepted checkpoint, build receipts,
  exact image bytes, deployment overlay, encrypted configuration, data/uploads.
- Its matching isolated restore passed in 18.556s with 137 tables and 6,978
  source entries. OneDrive dehydrate/rehydrate/hash proof passed at 16:30:01Z
  with verified key custody. The external watchdog is healthy.
- The backup task now uses the reviewed task's release wrapper every six hours;
  the next scheduled run is 22:30Z. The watchdog uses the common recovery state
  and /healthz with notifications off. Original scheduled definitions are saved.
- Preflight preserves existing external volumes, loopback ports 3000/8080 and
  the existing JWT from the owner .env. It removes inherited integration secrets
  from the effective backend environment and pins all three production images.
  Inventory: zero runnable tasks, Autopilots, triggers, webhook deliveries,
  channel installations/outbound messages, or online runtimes.
- The private Tailscale HTTPS route was authenticated through the existing
  owner account's normal, newly generated one-time-code flow. The one-use code
  was consumed; no fixed bypass or mail provider was enabled. This is the
  existing local-code access mode, not production email-delivery acceptance.
  The in-app Browser shows the existing 48-issue workspace before cutover.

Exact release/rollback commands and evidence will be recorded in the release
runbook after the application compatibility result. No production image or
Tailscale route has changed at this milestone.

## Private release verified at 16:40–16:41Z

- Corrected application rehearsal passed at 16:37:10Z. It recovered actual image
  bytes from the encrypted bundle, restored the database, ran 31 additive
  migrations (572 -> 603 ledger entries) and verified authenticated existing
  workspace/issue reads with both candidate and prior images. The same ephemeral
  session was retained across the image switch, matching production behavior.
  The initial harness's immediate second code request conflicted with the
  existing 60-second resend limit; its failure receipt is retained.
- Before/after counts match: 1 user, 1 workspace, 48 issues, 132 comments,
  110 terminal tasks and zero runnable tasks. All isolated containers, volumes,
  network and decrypted staging were removed before success was recorded.
- `production-release.ps1 Deploy` passed recovery and startup gates, then used
  the supported self-host base/overlay with exact pinned images. Deployment
  started at 16:39:46Z and all three services were healthy at 16:40:03Z.
- `/health` reports commit `60fea34b9770-local-6f0af10d34dd`; `/healthz` reports
  database/migrations OK. Production has all 603 migration entries and unchanged
  business counts. The private config route returns HTTP 200 with signup and
  plugins_v1 disabled; no Tailscale route or public-access change was necessary.
- Built-in Browser reload preserved the owner session at the private HTTPS
  workspace. Production Inbox visibly reports `Recovery protected`, verified
  backup/restore and automatic health checks. Historical fixtures remain
  historical; they are not counted as the new genuine issue's business result.
- Exact configuration, rollback scope and commands are in
  PRODUCTION-RELEASE-RUNBOOK-20260915.md. Current evidence:
  deployment-receipt.json, application-recovery-rehearsal.json and deployment.log
  under the production evidence directory.

No new production agent dispatch or provider invocation has occurred yet.

## Genuine pilot preparation at 17:20Z

- Created SILV-38, `Prepare the current CRM engineering handoff`, through the
  authenticated production Browser. UUID: 01a0a5fc-a49d-766b-b483-cf85a129c38c.
  It is Todo under the owner member. Database readback confirms zero new tasks
  and zero runnable tasks. The create form retained a stale collapsed Mika hint;
  submitted member ownership and the zero-task result were independently checked.
- The scoped host harness is in scripts/production-pilot. Its real deliverable
  will be model-written analysis of one dated CRM snapshot, followed by a
  separate review. The provider-free collector sees 1 root and 281 registered
  worktrees. Full records are retained; model input includes totals and eight
  representative locations. No CRM application or provider was invoked.
- Six focused broker checks pass, including a real fake-child process that
  verifies exact flags, one invocation, primary/result binding, disclosed
  composite model usage and exclusion of broker credentials/API keys. Wrong
  issue/role/model/tool output, duplicate launch and Phone Farm/UNC path cases
  are rejected. Application build-input fingerprint remains unchanged.
- One independent runtime review is active. Early findings cover partial setup
  recovery, planner-to-writer context continuity and fresh provider working
  directories. One consolidated correction will precede provider authority.
- A new controller UUID is prepared in the pinned overlay, but has not been
  applied. Fresh backup 20260915T171209Z-7ae7a897 and isolated restore
  4c2374403c passed. The encrypted cloud copy did not reach confirmed cloud-only
  state within the first 180-second verifier window. The same artifact is being
  checked with a 600-second window; no replacement backup or weaker custody
  claim was substituted. Existing deployed services are unchanged and healthy.
- Owner recovery-key retrieval on the phone remains verified; no repeat key
  action or disclosure is needed.

## Active controlled pilot continuation

- The same controller-config backup passed full OneDrive dehydrate/rehydrate
  hash readback at 17:44:10Z; key custody is verified. The watchdog is healthy.
- The consolidated setup/cleanup, planner-input, unique-workspace and runtime
  owner corrections passed eight Go and five Python tests. The same reviewed
  application images with the prepared controller setting deployed successfully
  at 17:52:02Z. Source fingerprint remains exact at 18:13:34Z.
- Three native planner tasks are preserved as failed. First:
  01a0a634-ff16-700a-b631-0726ac8e2229, 17:54:58Z. First manual recovery:
  01a0a642-564f-7946-a7a2-90bb8183ed47, 18:09:34Z. Its safe telemetry identified
  Claude Code 2.1.272's system/thinking_tokens event as the rejected frame.
  The bounded correction accepts only that observed progress event and discards
  reasoning blocks. Final planner 01a0a647-e6c4-7b38-acf3-23930226f396 completed
  its provider response but failed the strict plan check at 18:16:03Z.
- No provider failure was silently retried and no failed slot was reopened.
  The first two runtimes were stopped, all six earlier role receipts revoked,
  and both sets of temporary credential/token copies removed with verification.
- The final recovery harness passes nine Go and five Python checks. Its exact
  runtime, agents and executable digests have fresh native authority. Evidence
  is in .qualification-evidence/production-pilot-SILV-38-recovery2. Two provider
  calls remain within the five-call overall ceiling; correction calls are denied.
- The Codex operator has supplied a clearly labeled manual plan from the real
  captured CRM snapshot and explicitly mentioned the qualified writer in the
  production issue. The three planner tasks remain failed. Writing and
  independent review are the current live steps; no business completion is
  claimed yet. The application and private route remain healthy and unchanged.


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
