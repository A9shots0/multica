# Multica 32GB host implementation

This plan moves the Multica control plane to the 32GB Mac without moving its
coding workers or disturbing active development worktrees. Production remains
private, pinned, recoverable, and separate from UI/backend development.

## 1. Build the immutable host package

**Depends on:** -

**Tasks:**

- [x] Add a 32GB Compose profile pinned to the exact accepted production images.
- [x] Add protected host configuration, preflight, start, stop, and status tooling.
- [x] Add a user LaunchAgent installer for terminal-free startup after login.
- [x] Add an interactive SSH runtime bootstrap for the one admin-password step.
- [x] Add an Alienware exporter that packages images without exporting secrets or data.
- [x] Verify syntax, Compose rendering, manifest identity, and fail-closed behavior.

**Acceptance:** The package can be built from the current production images,
contains no credentials, and cannot start with missing configuration or images.

## 2. Build and prove rehearsal restore

**Depends on:** 1

**Tasks:**

- [x] Produce a fresh encrypted production backup and verified off-host copy.
- [x] Add a bounded transfer format for the database dump and uploads archive.
- [x] Restore into isolated rehearsal volumes and alternate loopback ports.
- [x] Compare build identity, migration ledger, business table counts, and upload hashes.
- [x] Prove authenticated web, WebSocket, attachment, and issue-history flows.

**Acceptance:** The rehearsal matches the source snapshot and a backup taken on
the 32GB restores into a second isolated stack.

## 3. Prepare private routing and rollback

**Depends on:** 2

**Tasks:**

- [x] Configure a temporary 32GB host-specific, tailnet-only Tailscale URL.
- [x] Record the exact old endpoint, temporary new endpoint, and rollback commands.
- [ ] Prove phone access without public exposure. Alienware access is proven.
- [x] Rehearse endpoint reversal while the Alienware stack remains intact.

**Acceptance:** Routing can move in either direction without deleting either
host's data or release package.

Current routing evidence:

- Existing production remains `https://a9drew.tailda9924.ts.net/` on the Alienware.
- The 32GB rehearsal is available only inside the tailnet at
  `https://32s-mac-mini.tailda9924.ts.net:8443/`.
- The temporary route proxies to loopback port `13000`; it does not replace the
  production port-443 route.
- Disable the temporary route with
  `/usr/local/bin/tailscale serve --https=8443 off`.
- Re-enable it with
  `/usr/local/bin/tailscale serve --bg --https=8443 http://127.0.0.1:13000`.
- The off/on reversal and Alienware HTTPS/WebSocket access were both proven.
- Phone access and the permanent production route are intentionally still open.

## Verified rehearsal evidence (2026-10-01 UTC)

- Source backup: `20261001T022236Z-1154d15c`, encrypted and copied to the 32GB
  before restore.
- Source revision: `de2e901a748010ac9093586c16238465db918a8d`.
- First restore: `r20261001a`; database ledger, representative counts, 246
  upload paths/hashes, authenticated API, web shell, WebSocket, issue history,
  and attachment-byte read all passed.
- Host-side backup: `20261001T023645Z-9113b760`.
- Second restore: `r20261001b`; the same restore and authenticated application
  checks passed from the backup created on the 32GB.
- `~/.smi/env` remains a separate mode-600 Intelligence environment. Multica
  receives Intelligence incidents through the authenticated HTTP contract;
  current Multica source does not consume `SMI_DATABASE_URL`, so the host
  package does not copy or require that database credential.
- Production Multica on the Alienware was not stopped, modified, or rerouted.

## 4. Cut over the control plane

**Depends on:** 3

**Tasks:**

- [ ] Confirm there is no active or queued agent work.
- [ ] Take and verify the final encrypted backup and off-host copy.
- [ ] Stop, but do not delete, the Alienware application containers.
- [ ] Restore the final delta on the 32GB and switch the private endpoint.
- [ ] Verify health, identity, data, uploads, login, WebSockets, and incident intake.

**Acceptance:** The 32GB is the verified private production host and the untouched
Alienware stack remains a rollback target for seven days.

## 5. Reconnect coding workers

**Depends on:** 4

**Tasks:**

- [ ] Replace the Alienware watchdog's incorrect cloud profile with the self-hosted profile.
- [ ] Prove bounded restart, stale-heartbeat reporting, and one harmless test task.
- [ ] Route Mac Studio daemon setup through a dedicated Mac Studio task.
- [ ] Keep the separate Chat review daemon isolated.

**Acceptance:** Each coding task routes to the explicitly registered machine and
repository; Multica itself does not execute code on the 32GB or EVO.

## 6. Complete the intelligence path

**Depends on:** 4

**Tasks:**

- [ ] Finish trusted incident intake and the 8GB watcher producer after its acceptance gate.
- [ ] Complete durable cross-repository development memory and Multica context packets.
- [ ] Add EVO later as a cited analysis layer, never as a fact store or coding worker.
- [ ] Keep owner review and explicit deployment approval as permanent gates.

**Acceptance:** A deterministic incident creates one durable Multica issue, an
agent receives the correct facts and development history, and any patch returns
for review without silently changing production.
