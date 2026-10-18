# Item 13 — Backup and disaster recovery

Updated: 2026-09-11

## Outcome

Silver Mediums can recover the local Multica pilot without overwriting the
running system, silently omitting owner work, exposing secrets, or claiming
recovery from an untested backup.

## Progress

- [x] Inventory authoritative recovery state and existing gaps
- [x] Define the backup, restore, retention, and outage-alert contracts
- [x] Implement atomic encrypted backup with a durable receipt
- [x] Implement fail-closed isolated restore and verification
- [x] Implement an external watchdog and deduplicated failure receipts
- [x] Add focused automated tests
- [x] Run an isolated restore rehearsal and measure recovery evidence
- [x] Expose owner-facing recovery health in the local app
- [x] Verify the owner-facing state in the built-in Browser
- [x] Run one final independent review and one consolidated correction pass
- [x] Record the verified checkpoint in the initiative and local handoff
- [x] Select and configure separate-drive plus encrypted OneDrive destinations
- [x] Copy the exact encrypted artifact and receipt to both destinations
- [ ] Complete OneDrive cloud-only/provider-readback hash proof
- [ ] Confirm separate recovery-key custody in the phone password manager

## Current boundary

The local encrypted-backup and isolated-restore path is implemented and
rehearsed. The corrected drill restored 137 user tables, matched migration and
representative row-count evidence, matched the complete upload checksum
manifest, and removed every disposable container and volume before writing its
success receipt.

The newest scheduled backup is `20260911T163922Z-bf49f066`: 19,853,528 bytes,
SHA-256 `fb240d4f0296c0378d50b159d4d458217f115edd6f6bf32ea49903e8a4b41d2d`.
Its exact encrypted artifact and receipt were hash-verified on the separate
Samsung A: SSD and copied into the configured OneDrive sync root. An isolated
restore of that exact run again restored 137 user tables and removed every
disposable recovery resource.

OneDrive has not yet acknowledged cloud-only custody; its bounded verifier
wrote `sync_pending` for the exact run/hash rather than treating the local sync
folder as off-machine proof. The built-in Browser visibly showed `Secondary
copy needed` and the exact automatic next action: OneDrive is uploading and the
verification will retry.

The owner selected OneDrive for encrypted off-machine artifacts and the phone
password manager for separate key custody. Two acceptance gates remain:
provider readback must pass, and the owner must actually save and confirm the
key on the phone. A sync-root copy alone is deliberately not called protected.

Item 14 is active. Its current role/squad qualification state and continuation
route live in `CURRENT-CONTINUATION.md` and
`ITEM-14-ROLE-BUILD-PROGRESS.md`; Item 13's two external custody gates remain
independent and must not be inferred complete from Item 14 progress.
