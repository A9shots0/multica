# Item 12 — SMS notifications

Accepted local-pilot implementation: 2026-09-11.

## Owner contract

SMS is an interruption channel, not a second workflow engine. The authenticated
Inbox remains the authority for evidence and consequential responses.

Eligible events are deliberately narrow:

- a genuine owner decision;
- an important incident, production regression, rollback, or consequentially
  stalled release;
- exactly one independently verified completed business outcome.

Agent starts, comments, handoffs, builds, reviews, deployments, intermediate
recovery, and routine progress never qualify by themselves. There are no
system quiet hours. The explicitly bound workspace owner is the only recipient.

```text
Authoritative Current Picture (same transaction)
                  |
          deterministic policy
                  |
      durable deduplicated outbox
                  |
   dry_run | live_test | explicit live
                  |
      Telnyx accepted -> delivered
                  |
    bounded receipt in secure Inbox
```

## Safety and truth

- Admission happens inside the same database transaction that stores the
  Current Picture. A failed admission rolls the projection back.
- `(workspace, delivery mode, event key)` is unique. The event key uses the
  stable decision, incident, or outcome identity rather than its mutable
  revision, so later evidence cannot resend the same owner event. Dry-run,
  live-test, and live namespaces cannot suppress one another.
- The phone number remains environment-only. The database stores a destination
  key and SHA-256 fingerprint, never the number.
- Delivery state distinguishes admitted, claimed, provider accepted, delivered,
  ambiguous, retryable, rejected, failed, acknowledged, and resolved.
- A network/server outcome that might have sent is `ambiguous` and is never
  blindly resent. Expired claims recover to `ambiguous`.
- Provider delivery checks have a hard ceiling. Telnyx final
  `delivery_unconfirmed`/`dlr_timeout` remains unknown delivery, not success.
- Live-test and live require separate explicit gates plus an explicitly
  verified owner destination. Only the workspace `owner` role may be bound;
  an administrator cannot become a substitute recipient. The accepted pilot
  was returned to `dry_run` after the real test.
- Inbox exposes only bounded delivery truth: event, mode, state, and timestamps.
  It omits destination, message body, credentials, and provider message id.

## Proof completed

| Proof | Result |
|---|---|
| Deterministic policy tests | Routine workflow noise suppressed; decisions, regressions, rollbacks, and verified completion admitted from exact references |
| Go safeguards | Owner-notification, workflow-store, workflow-command, and server configuration focused tests pass |
| UI contract | Core and Views typecheck; focused Inbox suite passes |
| Database migration | Fresh database and existing pilot both reached migration 543; table migration 523 plus corrective/split concurrent-index and primary-key migrations 524–543 leave no notification foreign keys or cascades |
| Binding privacy | Explicit owner binding stores a 64-character fingerprint and no destination number |
| Dry-run | One notification, one claim, two durable attempt receipts, terminal `dry_run_delivered` |
| Deduplication | Re-admitting the same dry-run event inserted zero rows; row count remained one |
| Namespace isolation | The same event key admitted separately in live-test without suppressing dry-run |
| Crash uncertainty | An expired claimed send became `ambiguous` and did not retry |
| Real provider test | Exactly one clearly labeled `live_test` SMS; one send, one provider check, Telnyx state `delivered` |
| Browser | Selected Inbox record visibly shows `SMS delivery`, `Live test delivered`, and `Dry-run recorded` |

The real message proves the transport and provider readback for one bounded
test. It does not yet prove a real owner decision link/resume or a real business
completion event on a physical phone. Those remain scenario acceptance work,
not permission to enable unrestricted live sending.

## Operator configuration

Configuration is fail-closed through environment variables documented in
`.env.example`. An enabled worker requires an explicit workspace slug,
recipient email, destination, and delivery mode. `live_test` additionally
requires `MULTICA_OWNER_SMS_LIVE_TEST_ENABLED=true`; `live` requires the
separate `MULTICA_OWNER_SMS_LIVE_ENABLED=true` gate. Either sending mode also
requires `MULTICA_OWNER_SMS_DESTINATION_VERIFIED=true`, and startup rejects a
recipient whose current workspace role is not exactly `owner`.

The local pilot currently remains in `dry_run` mode.

## Independent review and correction

The one final independent review returned NO-GO with three P1 findings and one
important P2. One consolidated correction pass removed prohibited foreign
keys/cascades and split every notification index/primary key into compliant
migrations; made event dedupe revision-insensitive and suppressed duplicate
technical-stop/current-incident alerts; restricted binding to the exact owner
role with an explicit verified-destination gate; and added checked-in tests for
ambiguous sends, retry ceilings, and bounded delivery reconciliation. Focused
tests, the running migration ledger/schema, service health, and the built-in
Browser proof passed afterward. No second reviewer loop was started.
