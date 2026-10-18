# Intelligence external incident ingestion handoff

Date: 2026-09-30
Source base: `origin/main` at `8fac9a705dac91b600b26c14f2f00f4490f6218b`
Status: local source and disposable-database proof only; not pushed or deployed

## What this slice does

This slice gives Silver Mediums Intelligence a narrow authenticated path to
open and maintain Multica issues from deterministic lifecycle findings. It does
not make Multica run the detector, infer incidents, or send alerts itself.

One source finding and episode generation own one Multica issue. New occurrence
records update that issue through these lifecycle states:

| Intelligence state | Multica issue state |
|---|---|
| `new`, `worsened`, `recovering` | `in_progress` |
| `needs_human` | `blocked` |
| `suppressed` | `cancelled` |
| `recovered`, `retired` | `done` |

Severity maps mechanically to issue priority: `low` to `low`, `medium` to
`medium`, `high` to `high`, and `critical` to `urgent`.

## Trust and identity boundary

An owner or administrator first configures a source binding:

```text
PUT /api/workspaces/{workspaceId}/external-incident-sources/{sourceKey}
```

The route requires workspace owner/admin authorization and the human-actor
guard. Its body binds the source to an existing project and a real, active
Multica agent that will be shown as the issue creator. A first configuration or
explicit rotation returns one `mei_` machine credential once. Multica stores
only its SHA-256 digest and a short display hint. Binding changes and rotations
use the current `expected_revision`; concurrent stale rotations fail rather
than returning a credential another request already invalidated.

Intelligence submits occurrences to:

```text
POST /api/workspaces/{workspaceId}/external-incident-sources/{sourceKey}/occurrences
Authorization: Bearer mei_...
```

This endpoint does not enter Multica's human JWT/PAT middleware. It validates
the source-scoped machine credential and exact workspace/source binding before
reading the body. Human PATs, browser sessions, task tokens, daemon tokens,
plugin tokens, and cloud-node tokens cannot cross this boundary. The request
body is capped at 256 KiB, strict JSON is enforced, and caller-supplied keys and
evidence references are bounded.

The occurrence contract is:

```json
{
  "event_key": "incident:multica:<outbox-sha256>",
  "finding_key": "<64-character Intelligence finding hash>",
  "incident_key": "<64-character Intelligence incident hash>",
  "source_sequence": 17,
  "predecessor_event_key": "incident:multica:<prior-outbox-sha256>",
  "episode_generation": 1,
  "transition": "new",
  "severity": "high",
  "action_required": true,
  "summary": "The deterministic probe failed.",
  "next_step": "Open the retained evidence and correct the first deterministic error.",
  "observed_at": "2026-09-30T12:00:00Z",
  "evidence_refs": ["railway-log-window:123"]
}
```

The lifecycle fields come from Intelligence incident-registry commit
`ee14d9bf4777b5a04fc9ebb7e8ea9efdd7912fd8`, and the outbox row's existing
`idempotency_key` becomes `event_key`. Before delivery can be enabled,
Intelligence must also add the producer-owned `source_sequence` and
`predecessor_event_key` to the Multica delivery. The predecessor is the prior
Multica outbox idempotency key for that finding, or null for the first Multica
delivery. This is derivable from Intelligence's retained event/outbox rows; it
is not inferred by Multica. Multica copies `summary` to the issue title and
puts the exact `summary` and `next_step` in the description.

No authorization header, raw machine credential, cookie, or other secret is stored in an
occurrence receipt.

## Idempotency and ordering

- `(workspace, source_key, event_key)` is the receipt identity.
- A canonical request SHA-256 distinguishes an exact replay from reuse of the
  same event key with changed content.
- Exact applied replays return the stored result and do not update the issue.
- Changed content under a used event key is rejected and retained in the
  quarantine table for diagnosis.
- A lease prevents concurrent exact retries from both applying.
- A per-finding advisory transaction lock orders separate events.
- `source_sequence` must increase but need not be contiguous because
  incomplete, invalid, and `continuing` producer events may intentionally have
  no Multica delivery. The explicit predecessor key supplies the delivery
  chain across gaps and episode generations.
- A delivery received before its predecessor is retained as
  `pending_predecessor` and returns HTTP 202 with a stable pending result. HTTP
  202 is not delivery success: the sender must retain and retry that exact
  outbox delivery. Once its predecessor is applied, the retry claims and
  applies the retained receipt. This durable sender/receiver handshake avoids
  relying on interruptible post-commit background work.
- The issue identity is deterministic from workspace, source, finding,
  generation. The issue update, episode update, and terminal receipt result
  commit in one transaction.
- New distinct events may have an equal `observed_at`; they cannot move the
  finding backward to an older observation. A terminal episode cannot reopen;
  the next episode must use exactly the next generation.
- Deterministic lifecycle rejections are themselves retained and replay-safe.

The issue starts machine-owned. On later transitions Multica compares the live
title, description, status, and priority with the prior machine projection.
Any diverged field becomes permanently human-owned for that episode and later
machine transitions preserve it, including preserving a human status through
source recovery. Untouched fields continue to follow the machine projection.

## Deliberate isolation from Workflow v2 and SMS

The endpoint writes only the external incident receipt/binding/episode tables
and the ordinary `issue` row. It does not create or mutate Workflow v2 concern
records, action receipts, Current Picture projections, owner-notification
outbox rows, Telnyx attempts, or SMS configuration.

This is intentional. An external detector occurrence is not automatically an
owner interruption or execution authorization. Any later policy that promotes
a Multica issue into Current Picture or SMS must be a separate reviewed slice.
The ordinary issue realtime event carries a complete canonical issue snapshot
so open clients update immediately. It deliberately uses the background map
shape: realtime/subscriber reconciliation runs, while the existing
`IssueResponse`-only activity and notification listeners do not create Inbox or
SMS noise from machine lifecycle updates.

## Database additions

- `external_incident_source_binding`
- `external_incident_occurrence`
- `external_incident_episode`
- `external_incident_quarantine`
- `issue.origin_type = 'external_incident'`

All uniqueness is installed through separate concurrent-index migrations. No
foreign keys or cascades were added.

## Local proof

A fresh disposable PostgreSQL 16 database applied migrations 001 through 602.
Focused handler/database tests proved:

- machine-auth-first and strict-payload rejection;
- human PAT rejection and source-credential enforcement before receipt creation;
- one-time credential issuance and rotation invalidating the prior credential;
- non-human `agent` issue attribution;
- `new -> worsened -> needs_human -> recovering -> recovered` on one issue;
- `new -> retired` on one issue;
- acceptance of the planned expanded Intelligence delivery contract, including
  `suppressed` and producer-derived ordering fields;
- exact replay and deterministic rejected-receipt replay;
- changed-event-key payload quarantine;
- concurrent exact retries converge to one receipt and one issue;
- out-of-order and concurrent distinct deliveries reconcile through their
  explicit predecessor chain and durable 202 retry, including across episode
  generations and after the predecessor commits without any post-commit drain;
- human changes to title, description, status, and priority remain untouched;
- realtime events carry the canonical issue snapshot and change flags;
- concurrent binding creation returns one live credential, and concurrent
  rotations return one live credential plus one revision conflict;
- zero change to owner-notification and Workflow v2 action-receipt counts.

Rollback is fail-closed on every migration from 602 down through 591 before an
index or table is removed whenever a binding, receipt, episode, quarantine row,
or external-incident issue exists. The used-data proof preserved both new
indexes and all rows; after explicit fixture cleanup, the focused 602-to-591
rollback completed. An operational application rollback should disable the
source/sender and leave the additive schema and captured evidence in place.

## Remaining integration boundaries

1. Intelligence still needs the delivery worker that sends each pending
   Multica outbox payload and its existing idempotency key to this endpoint,
   then records the returned issue binding/outcome. It must add
   `source_sequence` from the transition's source event and
   `predecessor_event_key` from the prior Multica outbox delivery for the same
   finding. Delivery must remain disabled until that exact producer contract is
   implemented and tested. The dispatcher must treat HTTP 202
   `pending_predecessor` as retained-but-undelivered, keep the outbox row
   retryable, and mark it delivered only after Multica returns an applied or
   exact-replay result.
2. A Multica owner must choose the target workspace project and a dedicated
   active agent, create the source binding, and securely install the returned
   one-time `mei_` credential in the Intelligence sender. No production binding
   or credential exists from this work. A workspace with the optional
   member-only issue-creation policy enabled will intentionally reject this
   agent-created issue and must make that policy choice explicitly.
3. Deployment requires the normal Multica migration, release, rollback, and
   authenticated runtime acceptance process. Nothing here is deployed.
4. Decide separately whether issue creation should ever qualify for an Inbox or
   SMS alert. This slice deliberately makes no such policy decision.
