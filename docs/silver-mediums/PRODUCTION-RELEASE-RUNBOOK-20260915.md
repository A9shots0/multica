# Private Alienware release and recovery

## Deployment identity

Release source: C:\Users\a9sho\.codex\worktrees\completion-20260915\SMK-multica-selfhost-pilot.
Keep this source/evidence directory while its scheduled recovery action is active.
Current build: `60fea34b9770-local-9eaa711d97b1`; Git base plus exact local content
hash. Deployment was verified at 2026-09-15T22:12:52Z. The independent review and
later targeted correction have separate evidence in the current result report.
Backend/web image bytes are pinned in
`deploy/silver-mediums/completion-20260915/compose.yml`. The previous
`60fea34b9770-local-6f0af10d34dd` release remains the pinned rollback target.

Private entry: https://a9drew.tailda9924.ts.net/silver-mediums-pilot/issues.
Tailscale Serve continues to proxy HTTPS to 127.0.0.1:3000. Funnel is absent.
The supported self-host Compose base plus release overlay retains project
`multica`, existing volumes `multica_pgdata` and `multica_backend_uploads`, and
loopback ports 3000/8080. Local previews remain separate from production.

The owner .env stays at C:\Dev\SMK-multica-selfhost-pilot\.env. The launcher
removes ambient Compose variables before rendering that explicit file, verifies
the running JWT matches, and uses an allowlisted backend environment. The
development code, LLM, SMS and external integration credentials are absent.
Signup and workspace creation are closed for this controlled existing-owner pilot.

## Commands

Run from the release source in PowerShell:

```powershell
Set-Location -LiteralPath 'C:\Users\a9sho\.codex\worktrees\completion-20260915\SMK-multica-selfhost-pilot'
./scripts/completion-release.ps1 -Action Preflight
./scripts/completion-release.ps1 -Action Status
./scripts/completion-release.ps1 -Action Backup
```

For an explicitly authorized cutover, use
`./scripts/completion-release.ps1 -Action Deploy` from that same directory.
Deploy rejects changed build inputs/images, unexpected listeners/volumes, active
or queued work, online runtimes, Autopilot/channel/webhook sources, missing
matching backup/restore/cloud/key/watchdog evidence, or a missing application
recovery rehearsal. It never rebuilds, pulls a new image, or deletes a volume.

## Recovery and rollback

Final post-cleanup recovery point: `20260915T223229Z-50923941`, SHA-256
`0e9d5787f6e4f55292f646d4f0713607a8cb855c2f428d2fb742f5079e466ed6`,
378,588,136 bytes. The encrypted artifact is on A:\Multica-Recovery\Backups
and copied to the selected OneDrive recovery directory. Matching backup,
isolated restore and application/rollback rehearsal are verified. OneDrive
cloud hash readback matched at 2026-09-15T22:45:45Z; the watchdog is available
and the existing scheduled backup finished with exit code 0.
The owner verified the separate key on the phone. Never paste it into chat.

The artifact contains data, uploads, current source/accepted checkpoint,
configuration and five exact Linux/amd64 image indexes. The existing hidden
backup task now runs `completion-release.ps1 -Action Backup` every six hours.
That cycle includes a matching isolated restore and cloud verification. The
external five-minute watchdog uses the same state directory and sends no SMS.
Only the backup action changed; its schedule, identity, settings and other
fields are unchanged. Prior task definitions and the exact action comparison
are retained in the release evidence directory.

The application recovery rehearsal loaded image bytes from the encrypted
artifact and verified both the corrected and pinned rollback applications
against the final restored database. Rehearsal `appf8263348b3` completed at
2026-09-15T22:40:40Z. All 50 issues, 146 comments, 118 terminal tasks and 603
migrations were preserved; both versions passed health and authenticated reads.
All rehearsal resources were removed. This proves the tested idle-pilot read
compatibility; old-version workflow writes have not been accepted.

If an image rollback is needed, first stop the controlled pilot runtime and
ensure no active/queued work remains. Preserve new business evidence, then run:

```powershell
./scripts/completion-release.ps1 -Action Rollback
```

This starts the two pinned previous images with the same private configuration
and retains the upgraded database/uploads. It does not run down migrations or
overwrite data. Restoring a database over the active system is a separate
destructive recovery decision; use the isolated restore tool to inspect an
artifact first. The exact application rehearsal receipt and all cleanup proof
are in `.qualification-evidence/production-completion-20260915/` within the
current release source. Earlier release evidence remains preserved separately.

## Owner access and operational boundary

The existing setup has no email delivery configured. It uses a newly generated,
one-use login code available locally to the operator; the fixed development
bypass is off. The owner-authenticated private Browser session was verified
before cutover. Preserve that session and JWT. The UI's generic email wording
does not establish email delivery. Broader user access or a mail provider needs
its own deliberate setup; neither is part of this one-owner launch.

SILV-38 is consistently Done with recorded operator assistance and unchanged
original task history. The separate SILV-39 automatic-publication test stopped
after its one publisher returned a summary over the strict byte limit. It made
no publication or reviewer run. Its consumed authorization cannot be reused.

All temporary execution authority, profiles and credentials are cleaned. The
isolated prompt proposal passed provider-free tests. A further live attempt
requires a new explicit owner grant and fresh issue/state/profile/binary and
authority bindings. No permanent role/model choice or unrestricted execution
is granted. Read `COMPLETION-CORRECTION-RESULT-20260915.md` in the current release
source before continuing.
