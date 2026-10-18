# Chat release handoff - 2026-09-25

Snapshot: 05:06-05:10 UTC. Read-only production inspection plus documentation.
Owner explicitly authorized making the prepared changes live. No further
permission question is needed for that release. Deployment remains unperformed.

## Recovery follow-up - 05:16 UTC (supersedes initial blockers below)

Final follow-up05:18:03Z: OneDrive provider readback PASSED using
dehydrate/rehydrate and matching SHA256, same backup run, key_custody=verified.
The backup, isolated restore and off-machine receipts now agree. This resolves
the initial manual recovery-point blockers; it does not repair the scheduled
jobs, establish running-source provenance or qualify the candidate application.

- Owner answered: "I have the key on my phone". Recorded a separate-copy
  attestation with method `owner_attested_separate_phone_copy` in the existing
  recovery state directory. This is owner attestation, not a key-value comparison
  or a claim about which phone app stores it. Do not ask for the key itself.
- Created isolated release worktree `C:/Dev/_work/multica-chat-release-20260925`,
  branch `codex/chat-release-20260925`, at
  `e18696e11a0ef6d15c2d3a38fa4c853cdb72f85b`. No owner files moved or reset.
- Prepared clean LF build clone at
  `C:/Users/a9sho/.codex/worktrees/multica-release-20260925`.
  Candidate source ID `e18696e11a0e-local-e73e6f770bb0`, hash
  `e73e6f770bb0714935da72ba11faa698df2dadf042961d0632222db98c34dc19`;
  6,148 build inputs. Manifests are in the release worktree's
  `.qualification-evidence/release-20260925/`.
- Preserved the exact three running images in a Docker archive and reconstructed
  backup-only service configuration from current container metadata. Stored
  under `C:/Users/a9sho/.multica/recovery-release-20260925` with inherited access
  removed and access granted to the current owner and SYSTEM. It contains
  credentials; never print, commit, attach or copy it into knowledge. No
  container was started/replaced from this snapshot.
- New encrypted backup succeeded at05:15:00Z:
  `A:/Multica-Recovery/Backups/multica-recovery-20260925T051340Z-d8ea947e.smkdr`,
  396,727,528 bytes, SHA256
  `19e08078996eb3d62a3755790e9c820d26bc0f02e47d70ad256d0d1bf75ba5bf`.
  Includes current database, uploads, service configuration, running images and
  prepared source. Prepared source is NOT proven running-image source.
  A local OneDrive copy matched; separate provider readback subsequently passed
  as recorded above.
- Isolated restore passed at05:15:57Z: matching database verification (147 user
  tables), 246 upload files and checksums, resources_retained=false. Existing
  production/review databases were untouched. Receipts: backup-latest.json and
  restore-latest.json in the configured state directory.
- After normalizing both sides' line endings, all1,218 migration SQL files
  copied from the running backend match the candidate; candidate adds20 files
  (10 up/down pairs). First raw-hash comparison was invalid due to CRLF in the
  running image; do not repeat it as a content-change finding. Full evidence is
  `.qualification-evidence/release-20260925/migration-file-comparison.json` in
  the original Chat worktree. Ledger/schema compatibility still needs rehearsal.
- Actual release command `release-candidate.py delta release-20260925` exited1:
  "no recorded release identity matches the running production images; rollback
  target unknown". Targeted identity/receipt searches in the source, recovery
  checkout, release config directory and hub found no matching record. Exact
  images are archived, but the gate still lacks a proven deployed source baseline.
  Do not invent a source identity or silently waive that check.
- Scheduled backup/cloud jobs remain pointed at old sources. Their permanent
  repair must follow the qualified release configuration; the successful manual
  backup above does not certify the schedule repaired. No candidate image build,
  push, tag, production migration/restart or deployment has occurred.

## Source and observed runtime are different

- Prepared product commit: `6ac56efcb84ff5cfd33c7a7578a74031e5ec3965` on
  `codex/brain-owner-review-20260924`, isolated source at
  `C:/Dev/_work/multica-brain-owner-review-20260924`.
- Fresh remote HEAD/main: `8fac9a705dac91b600b26c14f2f00f4490f6218b`.
  Source is 34 commits ahead of that SHA; delta is 342 files, 25,763 additions,
  1,361 deletions. This is NOT a measured diff against the deployed image.
- Production backend container: image
  `sha256:24aa70354a236da738fe10a30ae44e98b4b5e6aeee7e300a18aa153a877b9d64`,
  tag `multica-backend:guardrails-v7`, started 2026-09-23T19:03:27Z.
  `/health` returns status ok but commit unknown; source provenance unproven.
- Production frontend: image
  `sha256:b0650fca6c1af749c58d8d1020fb4fe9e5b1336f7fc761ed51801e8ffbaa6268`,
  tag `multica-web:deploy-verified`, started 2026-09-22T18:42:50Z.
- Backend compose labels point to
  `C:/Users/a9sho/.multica/releases/multica-recovery-4475751ea/deploy/silver-mediums/release-20260920/`
  with `compose.yml` and `compose.driver.yml`. The frontend lacks compose
  provenance labels. Preserve the actual runtime configuration rather than
  replacing it from an old release template. Config files can contain secrets;
  inspect through allowlisted/redacted projections, never print or commit them.
- Read-only database aggregate: 610 schema_migrations rows; lexical max
  `581_chat_search_bigm_index` (not an ordering/completeness claim). Queue counts:
  407 completed, 125 failed, 90 cancelled; zero nonterminal rows at this snapshot.
  Recheck immediately before a deployment; do not edit historical task rows.
- Local review remains separate: web3221, API3220, review DB55438. See root
  handoff for exact launcher/source/worker pointers. Do not promote review data,
  credentials or workspace mappings into production.

## Concrete recovery blocker

`SilverMediums-Multica-Backup` failed on 2026-09-25 at 04:30:39Z, exit1.
Its current receipt reports a missing environment file:
`C:/Dev/SMK-multica-selfhost-pilot/.env`. Existence check confirmed absent.
The task still points to release `multica-recovery-a19ac46c2`; this differs
from the running backend's compose source. Do not blindly replay that task.

Configured recovery state directory:
`C:/Dev/SMK-multica-selfhost-pilot/recovery/state`.
It contains backup-latest (failed), watchdog-state and recovery-incident-latest.
Matching restore-latest, off-machine-latest and key-custody.json are absent.
The cloud-verification scheduled task also returned1 today. Existing encrypted
artifacts and receipt filenames in both A:/Multica-Recovery/Backups and the
OneDrive recovery directory reach only 2026-09-20; directory presence does not
prove off-machine readback or separate key custody.

The protected local recovery key file exists. Its value was not read.
Owner was asked whether a separate copy exists and where (location only, never
the key). No answer was available when this note was written. Do not fabricate
an attestation or mark this gate verified merely because the local key exists.

## Release continuation

1. Reconcile the actual running configuration with a protected, current backup
   source. Preserve session secrets, uploads, database identity, worker routing
   and existing integration settings; avoid old templates that erase them.
2. Obtain truthful separate-key-custody evidence; generate a fresh encrypted
   recovery point, prove isolated restore and off-machine provider readback.
   Keep original artifacts and owner work. Restore rehearsal must not touch
   production volumes or the existing review/test databases.
3. Qualify the whole prepared branch and migration compatibility against the
   actual production state. Existing Chat evidence is useful but is not a
   complete combined-release qualification. Core typecheck has a previously
   documented Inbox fixture mismatch; fix or resolve it, do not claim green.
4. Read `shared-brain-release-handoff.md` and exact current release scripts.
   `release-candidate.py` expects clean build inputs; owner web config files
   are dirty in this checkout. Use a separate pinned release worktree and
   preserve them. `release-qualify.py` hardcodes port55436, already occupied by
   `silv59-pg-proof-20260920`, and deletes its fixed-name disposable container:
   make its isolation safe before running. `adoption-release.ps1` insists on
   C:/Dev/multica, an occupied diverged checkout; do not overwrite that checkout
   or bypass its gates. Modernize the execution path with equivalent checks.
5. Pin build receipts/images, independent review, migration/rollback rehearsal,
   compatible CLI release and real workspace brain mappings. Root CLAUDE.md
   requires a patch-version CLI tag on main and release.yml publication.
   The existing release-build/request.json pins an older release: do not trigger
   it unchanged. Do not replay already integrated brain commits.
6. Publish the qualified source and deploy exact artifacts, then verify the
   real signed-in owner flow through the built-in Browser and Mika runtime.
   Check topic routing, choice/acceptance isolation, uploads/annotated image,
   mobile editor/actions, and persisted history. Report actual loaded revision.

No push, tag, build, production configuration change, migration, container
restart or scheduled-task update was performed during this inspection. Only
read-only production queries and documentation changes were made. The latest
diagram correction is not separately visually accepted; the later release
instruction authorizes promotion without requiring another permission ritual.
Keep its rejection history visible and verify the narrow phone editing state.

## Next pages

After completing release, use Chat's documented standards for the next page.
Suggested order is Inbox, Issues, Projects; this is a recommendation. Preserve
the real Multica shell, compact phone UI, actual Mika capabilities, truthful
states and visible reply context. Do not copy the rejected diagram UI patterns.
