# Item 13 — Recovery contract

Updated: 2026-09-11

## Authoritative state

A complete recovery point contains all of the following from one bounded run:

1. a PostgreSQL custom-format logical dump;
2. the backend uploads volume;
3. the current source tree, including uncommitted owner work but excluding
   dependencies, build outputs, Git internals, and plaintext environment files;
4. encrypted runtime configuration required to rebuild the same deployment;
5. a manifest containing timestamps, checksums, schema version, source and
   container fingerprints, and backup-tool versions;
6. a durable, non-secret receipt that records success or the exact failed stage.

Omitting any required component fails the backup. A configured schedule or an
artifact that has never been restored is not recovery proof.

## Safety boundaries

- Backup assembly happens in a private staging directory and is published by
  atomic rename only after checksums and encryption succeed.
- Plaintext secrets exist only inside that staging directory and are removed
  on every exit path. Logs and receipts never contain secret values.
- The default restore target is isolated. Restoring over the active pilot
  requires a separate explicit destructive flag and is never part of the
  automated rehearsal.
- A restore accepts only a supported manifest version, authenticated encrypted
  archive, and matching checksums. Any mismatch stops before database import.
- Source, database, uploads, and configuration are verified independently.
- Backup and restore locks prevent overlapping runs.

## Encryption and key custody

The backup is encrypted with an operator-supplied recovery passphrase. The
passphrase is read from an environment variable or protected file, is never
written into the artifact or receipt, and must be stored separately from both
the workstation and the secondary backup copy. Machine-only encryption is not
acceptable because it cannot recover a lost workstation.

The local pilot may keep a user-readable-only key file for unattended backups,
but that proves recovery only while the workstation survives. The owner-facing
state remains `Secondary copy needed` until both the encrypted artifact and an
offline copy of the key have verified custody outside the workstation.

## Retention and second copy

Successful backups may be copied to an explicitly configured secondary path.
A matching hash proves only `copy_verified`; it never proves that the path is
off-machine. `Protected` additionally requires an independently supplied
off-machine custody receipt bound to the exact backup run and artifact hash,
plus separately verified recovery-key custody.
Retention applies only to verified, closed artifacts and never deletes the
newest verified local or secondary recovery point.

The selected local layout is the separate Samsung A: SSD for the fast recovery
copy and the signed-in personal OneDrive account for the encrypted off-machine
copy. The selected separate key location is the owner's phone password
manager. OneDrive counts only after Files-on-Demand confirms cloud-only custody
and the exact encrypted artifact is rehydrated and hash-matched. A file merely
present in the local OneDrive directory does not count.

The OneDrive verifier is bounded and durable. Pending upload writes
`sync_pending` with the exact backup run and artifact hash; it never hangs a
scheduled task or promotes `Protected`. Provider readback failure writes a
failed receipt and remains owner-visible.

## Recovery proof

Acceptance requires a disposable rehearsal that:

1. decrypts and validates the artifact;
2. creates isolated database and uploads targets;
3. restores the dump and uploads without touching the active pilot;
4. verifies schema reachability, representative row counts, upload checksums,
   source/config presence, and migration identity;
5. records elapsed time, recovery-point age, and exact evidence in a receipt;
6. removes only explicitly named disposable rehearsal resources after their
   identity is checked, proves they are absent, and only then publishes the
   successful restore receipt.

RPO and RTO are reported from this rehearsal, not guessed beforehand.

## External failure detection

The watchdog runs outside the Multica backend. It records backup staleness,
backup failure, failed rehearsal, and platform unavailability in a durable
local receipt with semantic deduplication. Owner SMS remains dry-run unless a
separate live-send authorization exists. A live alert is durably admitted
before the provider call; a crash-ambiguous submission is never automatically
resent. Recovery reconciliation updates the same incident instead of creating
duplicate alerts.

## Owner-facing truth

The app may show only evidence-backed states:

- `Protected` — the latest verified recovery point has a matching successful
  rehearsal, verified off-machine artifact and key custody, and a fresh healthy
  external-watchdog receipt;
- `Backup due` — the latest verified recovery point exceeds the configured age;
- `Recovery unproven` — backups exist but no successful rehearsal exists;
- `Secondary copy needed` — local backup and rehearsal passed, but no verified
  off-machine copy protects against loss of the workstation;
- `Backup failed` — the latest bounded backup attempt failed;
- `Platform unavailable` — the external watchdog cannot reach the required
  health endpoint after its threshold.

Each state includes the last verified time, recovery-point age, latest test,
replication state, and automatic next action. It must not expose paths, phone
numbers, provider ids, or secrets.
