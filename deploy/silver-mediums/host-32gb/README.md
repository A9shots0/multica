# Multica on the 32GB Mac

This profile prepares the 32GB Intel Mac mini to run the pinned Silver Mediums
Multica control plane. It does not move Claude/Codex workers to the Mac, expose
Multica publicly, migrate production data, or perform a cutover by itself.

## Safety boundary

- The base and host Compose files publish only to loopback.
- The real host environment lives outside Git with mode `600`.
- All three images are exact image IDs and `pull_policy: never` prevents drift.
- Start and stop preserve PostgreSQL and upload volumes.
- Start refuses to create empty production volumes. It requires both restored
  volumes and a mode-600 accepted data-ready receipt produced by the rehearsal.
- The checked-in LaunchAgent is a user agent. It starts after the user session
  begins; true reboot recovery is not accepted until tested on the real Mac.
- The Alienware stack remains untouched until rehearsal and owner review pass.

## Package on Alienware

Run from PowerShell with an empty destination:

```powershell
.\deploy\silver-mediums\host-32gb\scripts\windows\Export-Host32Release.ps1 `
  -OutputDirectory C:\Dev\_artifacts\multica-32gb-host
```

The export contains the exact images and deployment files, but no database,
uploads, environment file, credentials, or recovery key.

## One-time Mac prerequisites

The owner runs `scripts/bootstrap-runtime.sh` through an interactive SSH TTY.
It first verifies every copied file against the package's `SHA256SUMS`. It then
asks for the Mac login password inside that SSH session, verifies and installs
the pinned MacPorts package for Intel macOS 15, then installs Colima, the Docker
CLI, and the Compose v2 plugin. It starts a 4 CPU / 8 GiB / 64 GiB Colima
runtime and loads the pinned images. It does not start Multica, restore data,
or change Tailscale routing. Homebrew is deliberately not used because its
current macOS installer rejects Intel processors.

## Configure and preflight manually

These steps are for a manual host setup only. The production migration uses
`restore-production.sh`, which creates the protected environment after it has
verified the final data bundle. Do not create `host.env` before that restore.

1. Load `deployment-images.tar` with `docker image load`.
2. Copy `host.env.example` to
   `~/.config/silver-mediums/multica/host.env` and replace every placeholder.
3. Run `chmod 600 ~/.config/silver-mediums/multica/host.env`.
4. Copy `feature-flags.yml` to the protected path named by
   `MULTICA_FEATURE_FLAGS_PATH`, keep it owner-readable only, and review the
   bounded cohort before each rollout. Do not add global
   `FF_CONTINUOUS_WORK_OBSERVATION_V1` or
   `FF_CONTINUOUS_WORK_CHECKPOINT_V1` overrides while a targeted pilot is in
   progress; either override takes precedence over the task-targeted file.
4. Run `chmod 755 deploy/silver-mediums/host-32gb/scripts/*.sh` in the unpacked
   package. The Windows export filesystem does not preserve Unix executable bits.
5. Create the configured recovery-state directory.
6. Run `bash deploy/silver-mediums/host-32gb/scripts/preflight.sh`.

The shared company brain remains the separate authenticated service already
running on this Mac. The Multica backend does not consume a filesystem
`KNOWLEDGE_HUB_PATH`; coding daemons access the brain through their enrolled
client instead of mounting a second, potentially stale checkout.

Before preflight, place the reviewed Multica Brain mapping at
`~/.config/silver-mediums/multica/brain.json` and its dedicated bearer
credential at `~/.config/silver-mediums/multica/brain.token`, both with mode
`600`. The host profile mounts only those two files read-only into the backend;
it does not mount `host.env`, an enrollment directory, or the Brain data store.
The JSON must use `/run/secrets/multica-brain/token` as `tokenFile`. Recovery
backups intentionally exclude both protected files, so a restored host must
re-enroll or restore them through the credential process before preflight.

`FF_PLUGINS_V1` stays false by default. Set it to true in the protected
`host.env` only when this private host intentionally enables reviewed Plugin
or Outcome Receipt integrations; the Compose profile now passes that explicit
choice into the backend instead of silently ignoring it.

Do not copy the current Alienware environment file. The migration rotates the
database password, JWT signing secret, and any credential previously present in
the release configuration.

## Commands

```bash
bash ./scripts/preflight.sh
bash ./scripts/start.sh
bash ./scripts/status.sh
bash ./scripts/stop.sh
bash ./scripts/install-launch-agent.sh
bash ./scripts/backup-production.sh
bash ./scripts/install-backup-launch-agent.sh
```

`start.sh` cannot create missing volumes and cannot run against an unaccepted
receipt. The restore tooling in the next implementation chunk owns volume
creation and emits the receipt only after the database and upload checks pass.

## Isolated restore rehearsal

The Windows exporter converts one verified encrypted recovery artifact into a
bounded transfer directory containing only the database dump, uploads archive,
and their verification evidence. It deliberately excludes the production
environment, credentials, source archive, and image archive.

```powershell
.\scripts\windows\Export-Host32DataBundle.ps1 `
  -ArtifactPath C:\path\to\multica-recovery.smkdr `
  -PassphraseFile C:\protected\recovery.key `
  -OutputDirectory C:\empty\transfer-directory
```

After copying the encrypted source artifact, its receipt, and the bounded data
directory to the 32GB Mac over SSH, restore with a unique rehearsal ID:

```bash
bash ./scripts/restore-rehearsal.sh /path/to/data-bundle rehearsal01
bash ./scripts/start-rehearsal.sh rehearsal01
bash ./scripts/verify-rehearsal.sh rehearsal01
bash ./scripts/backup-rehearsal.sh rehearsal01
bash ./scripts/stop-rehearsal.sh rehearsal01
```

The rehearsal uses separately named external volumes and alternate loopback
ports 18080/13000. It generates fresh database and JWT secrets, disables SMS,
LLM calls, VCS integration, and the workflow driver, and never copies the live
production environment. Stopping the rehearsal preserves its isolated volumes.
The verifier performs read-only authenticated API and WebSocket checks plus
frontend routing, issue history, attachment-byte, dispatch-fence, and
zero-active-work checks against restored records. It emits a separate mode-600
acceptance receipt without storing its short-lived token. Human sign-in remains
a separate owner acceptance check.
The rehearsal backup command takes a second online database/uploads snapshot on
the 32GB Mac and emits the same bounded, credential-free transfer format. Feed
that directory back to `restore-rehearsal.sh` under a new ID to prove the Mac's
own recovery path without overwriting either restored volume.

## Production restore and acceptance

After a final quiesced source backup has been exported into the bounded data
bundle, restore it into the production-named volumes with:

```bash
MULTICA_MIGRATION_LLM_BASE_URL=http://100.85.0.90:11434/v1 \
MULTICA_MIGRATION_LLM_DEFAULT_MODEL=qwen2.5-coder:3b \
bash ./scripts/restore-production.sh /path/to/final-data-bundle EXISTING_CONTROLLER_UUID
bash ./scripts/start.sh
bash ./scripts/verify-production.sh
```

The default verifier mode is the first-cutover safety gate and therefore
requires workflow dispatch to remain disabled with no active work. After the
remote coding workers are connected and dispatch is intentionally enabled, use
the same authenticated checks in live mode:

```bash
MULTICA_VERIFY_MODE=live bash ./scripts/verify-production.sh
```

Live mode requires the configured and observed dispatch state to be enabled. It
records the current active-work count without treating legitimate in-progress
work as a failed host verification.

The restore refuses an existing production environment, container, or volume.
It rotates the database and JWT secrets, verifies the database and every upload
before writing an accepted mode-600 receipt, and leaves workflow dispatch off.
The verifier checks the exact build, authenticated API and WebSocket paths,
frontend routing, issue history, attachment bytes, the real dispatch fence, and
a zero active/queued task boundary. Human sign-in remains a separate owner
acceptance check. Enable the workflow driver only after the remote coding
workers are reconnected.

## Production backups

`backup-production.sh` creates a credential-free database/uploads bundle under
`~/.silver-mediums/multica/recovery/production-backups`, verifies every local
component hash, then copies the same bounded bundle to the 8GB Mac and the
private R2 `multica-control-plane-backups` prefix. Any failed off-box or R2 copy
is a failed run. `install-backup-launch-agent.sh` schedules it daily at 4:15 AM,
after the Intelligence backup window. Recovery qualification still requires a
separate restore rehearsal; creating a bundle alone is not restore proof.
