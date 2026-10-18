# Controlled release and recovery: independent review

## Verdict and scope

**Continue release preparation; exact candidate recovery and rollback coverage
must be completed before cutover.** The existing backup/restore helpers can
recover the old database and uploads. They do not yet bind the proposed split
source/configuration deployment or prove the reviewed application can be
recovered. No change to accepted Items 1–14 semantics is required.

This is the single bounded independent release/recovery review. Snapshot copied
at **2026-09-15 16:12:48.0367619 UTC** into
`A:\Silver-Mediums-Workspace\audits\multica\browser-readiness-review-20260915`.
`RELEASE-RECOVERY-SLICE.json` records SHA-256 hashes for **15/15** copied nonsecret
files: ten recovery scripts/module/test files, self-host Compose, Dockerfile,
entrypoint, production progress, and local acceptance. Startup readers were
inspected in the existing accepted review snapshot. No active file, container,
database, scheduled task, credential, secret source or provider was changed or
opened by this reviewer.

The production progress snapshot records owner authorization for the Alienware
host/private Tailscale route, recovery and custody gates before cutover, and one
genuine low-risk issue under controlled manual dispatch. Public exposure,
unrestricted automatic execution and real-person messages remain excluded.

Definitions: P1 is a release-blocking recovery/access gap if left unaddressed;
P2 is a narrower operational defect or missing release guard. “Verified” below
distinguishes this review's source inspection from runtime evidence supplied by
the parent. This is not an exhaustive security or repository audit.

## Findings and smallest release-only adaptations

### P1 — Backup currently cannot bind the proposed deployment tuple

`scripts/disaster-recovery/Backup-Multica.ps1:4–5` accepts one repository root
and one Compose file. The same file drives service discovery, dump, resolved
configuration and image listing (lines 91–114, 145–156). Source is archived from
that root (lines 40–49, 132–139), and the secret configuration must be that
root's `.env` (lines 142–144). The upload lookup is explicitly project `multica`
(lines 28–37).

The proposed deployment uses the reviewed task source, an existing owner-checkout
secret file and an additional deployment overlay. Running the old scheduled
backup can therefore archive old source/configuration while dumping the new
database. Pointing it at the task root instead cannot represent the old secret
source or additional Compose overlay. An image name/ID listing is evidence about
an image, not image recovery bytes. The ignored accepted checkpoint is absent
from Git-visible source enumeration.

**Required minimal adaptation:** preserve backward-compatible defaults but accept
an explicit source root, environment file and Compose override; apply the same
project/base/overlay/env arguments to every Compose operation. Keeping project
`multica` is appropriate for this release: verify the expected live volume names
and container mounts, rather than generalizing every project convention.

Archive the exact approved overlay/configuration, source manifest/build receipt,
and specifically allowlisted accepted checkpoint sources/manifests as encrypted,
hashed components. Include exact candidate image bytes, or establish an equally
durable verified immutable image source. Do not copy the whole ignored evidence
tree or introduce a new secret source.

Scheduled actions also need the new deployment tuple. The installer currently
passes only RepositoryRoot/output/passphrase path and uses scripts under its own
`$PSScriptRoot` (`Install-MulticaRecoveryTasks.ps1:21–30,40–47`). Parent inspection
reports existing tasks point to the C:/Dev owner checkout. A release wrapper and
updated scheduled arguments must select the reviewed source while retaining the
existing secret path, backup destinations, key source and common recovery-state
directory. The backend's read-only state mount must select that same absolute
directory (`docker-compose.selfhost.yml:64,172`). Otherwise the new UI can show
different or empty receipts while the old scheduled task keeps succeeding.

### P1 — Successful isolated data restore does not prove application rebuild or rollback

`Restore-MulticaIsolated.ps1:79–91` validates eight required archive components;
lines 125–165 restore/check the database and uploads. Lines 167–172 only list the
source tar and compare its entry count. No backend/web image is loaded, rebuilt
or started, and no authenticated application route is exercised. The accepted
local preview started with an empty database and is not an upgrade rehearsal of
the existing production data.

**Required minimal adaptation:** retain the proven old-artifact drill, then
perform a separate isolated application recovery/upgrade rehearsal from the
fresh pre-cutover artifact plus exact candidate recovery bundle. Verify every
new optional component when present, load/check exact image identities, use
disposable database/uploads with outbound authority disabled, run migrations,
and inspect health plus representative existing data through the application.
Never attach rehearsal services to production volumes.

Capture both current and candidate backend/web image IDs and preserve their
recoverable bytes. Record the exact pre-cutover migration set, not only the
largest version name. The entrypoint always runs `./migrate up` before the server
(`docker/entrypoint.sh:26–44`); changing an image back does not reverse migrated
data. Rollback must specify either demonstrated old-image compatibility with the
new schema or restoration of the quiesced pre-cutover database/uploads. Do not
infer compatibility or automatically run down migrations. Keep the production
volume names and old artifact intact throughout preparation.

### P2 — Startup has automatic dispatch paths; credential removal is not a dispatch gate

`server/cmd/server/main.go:749–750,795–814` starts the webhook queue worker and
registers Autopilot/plugin schedule jobs. The scheduler runs its first tick
immediately (`server/internal/scheduler/manager.go:100–104`). Autopilot triggers
are reconstructed from durable enabled/active records
(`jobs_autopilot.go:76–88,175–178`). The webhook worker claims persisted deliveries
and dispatches them (`server/internal/handler/webhook_delivery_worker.go:113–119,191`).
Existing runtimes also retain normal claim routes (`server/cmd/server/router.go:1462–1467`).
Clearing a server-side LLM API key cannot prevent a separate registered daemon
from claiming already queued work with its own runtime authority.

**Fresh evidence narrows this:** the parent reports **0/110 nonterminal task
rows** (105 completed, 3 cancelled, 2 failed), zero Autopilot rules/triggers, zero
webhook deliveries, and four offline runtimes. There is therefore no demonstrated
startup dispatch backlog in that snapshot. Do not claim startup will necessarily
launch work, and do not add a blanket execution redesign for this release.

**Smallest guard:** preserve those observations as an explicit fail-closed
pre-cutover/start preflight. Recheck runnable queue, scheduled/event sources and
runtime liveness immediately before activation. Keep plugin schedules disabled
or prove none are enabled; `plugins_v1` defaults false
(`server/internal/featureflags/keys.go:57–58`, `jobs_plugin_hook.go:123–129`).
Explicitly clear all inherited outbound integration settings in the effective
overlay and verify presence/absence without values. If unexpected work or live
runtimes appear, stop cutover and resolve the exact source; do not mark tasks
complete or delete history to obtain zero counts. Enable only the authorized
manual issue path after startup readback.

### P2 — Mail settings cleared in production do not establish fresh owner sign-in

`APP_ENV=production` disables the fixed-code bypass
(`server/internal/handler/auth.go:127–146`), but absent SMTP/Resend still makes
`EmailService.SendVerificationCode` print the generated login code to stdout
and return success (`server/internal/service/email.go:349–354`). That fallback
does not check APP_ENV. Thus “production env + no outbound settings” is not, by
itself, evidence of a usable production email authentication route.

**Release action:** preserve the existing JWT/identity source and verify the
actual owner-authenticated private route. An existing valid owner session can be
used if available. If fresh sign-in is needed, arrange an explicit owner-operated
authentication path consistent with the no-message/no-new-secret boundary before
claiming access acceptance. Do not retrieve generated codes from logs, re-enable
the development bypass, silently restore mail credentials, or mistake private
Tailscale transport alone for application authentication. This is an access
acceptance gate, not a request to redesign accepted authentication semantics.

## Evidence already closed versus still required

| Evidence lane | Status and denominator/identity |
| --- | --- |
| Reviewed local candidate | Acceptance snapshot 15:49:43 UTC/source recheck 15:50:56; source ID `60fea34b9770-local-6f0af10d34dd`, 5,865 hashed build inputs. Source/build/runtime/Browser sample acceptance remain local facts. |
| Accepted preservation | Prior independent adoption: 86/86 deltas, 1,380/1,380 inherited backups, 60/60 frozen sources, 3/3 manifests. This review does not reopen them. |
| Existing recovery artifact | Parent reports `20260915T103923Z-04a0a00a`, SHA-256 `7cad3605097dfd386a5c8dbe7eb302bb42ecf9f4810d4046ef6eb8e22edeb125`; OneDrive dehydrate/rehydrate/hash readback verified. |
| Matching old-artifact restore | Parent reports success in 18.236 seconds, 137 user tables, 6,847 source entries and no retained resources. This closes the previously stale matching-restore gate for that artifact. |
| Separate key custody | Parent reports direct owner confirmation during this review. Do not ask for or expose the key. The script requires that explicit confirmation (`Confirm-RecoveryKeyCustody.ps1:13`). |
| Current production data | Parent reports 1 owner, 1 workspace, 48 issues, 10 idle agents, 4 offline runtimes, 572 applied migration rows. Candidate preview reports 603 embedded migration files; obtain exact set differences before migration rather than equating row counts with numeric prefixes. |
| Candidate/rollback recovery | Open until the exact deployment bundle and isolated migration/application rehearsal pass. Old artifact custody cannot establish custody of the new release automatically. |
| Production/runtime/business result | Open: no cutover, production Browser acceptance or genuine issue execution was performed by this reviewer. |

## Consolidated correction and completion order

1. Apply the narrow backup/wrapper/scheduled-argument adaptations and write the
   concrete deployment/rollback overlay. Pin reviewed images, preserve
   `multica_pgdata` and `multica_backend_uploads`, retain loopback ports, and set
   private HTTPS origins. Keep the original owner checkout and secret file exact.
2. Capture a quiesced pre-cutover recovery point and durable candidate/image/
   checkpoint bundle; prove matching restore and candidate migration/application
   recovery in isolated resources. Retain artifact/key custody evidence.
3. Pass the dispatch preflight and verify the owner authentication route; then
   perform the authorized cutover and one controlled genuine issue, recording
   deployment, runtime and business results separately.

No second independent review round is requested. The parent owns the consolidated
correction and its focused verification. Source inspection is high confidence;
runtime/custody counts above are explicitly parent-supplied and were not queried
again. No secrets were read, encrypted archives opened, recovery scripts run,
remote service contacted or production stack changed in this review.
