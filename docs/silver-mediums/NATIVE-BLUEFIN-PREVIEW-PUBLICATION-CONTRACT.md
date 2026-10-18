# Native Bluefin preview publication contract

Status: controller publication and scoped Shell read transport implemented.

## Current repository truth

- Normalized candidates are durable `issue_workflow_candidate` rows. Their
  immutable identity, source revision, artifact and build contract are created
  in `server/internal/workflowcommand/service.go`.
- A correction is admitted only from the typed `workflow_result` on the exact
  completed `review_correction` task. Ordinary output, comments and PR URLs
  cannot create a candidate (`server/internal/workflowcommand/task_result.go`).
- The result route binds task/agent/workspace/issue/action/operation plus the
  controller-selected runtime, runtime daemon, `work_dir`,
  `durable_work_dir`, and `branch_name`.
- Scoped `msr_` Bluefin reader sessions are present and the allowlist is
  extended only for the issue preview publication GET.
- The controller stores immutable publications. The producer still owns the
  loopback preview server, identity endpoint, policy enforcement, and shutdown.

## Fixed v1 contract

`server/internal/previewpublication` is the machine-checked definition.
The controller, never the worker result, supplies the complete binding:

```json
{
  "workspace_id": "uuid",
  "issue_id": "uuid",
  "agent_id": "uuid",
  "agent_run_id": "task uuid",
  "candidate_id": "uuid",
  "candidate_revision": 1,
  "source_revision": "immutable producer revision",
  "worktree": {
    "daemon_id": "registered daemon identity",
    "runtime_id": "uuid",
    "path": "absolute task worktree path",
    "branch": "recorded task branch"
  }
}
```

The producer may propose only `preview_url`, `identity_url`, the exact policy
below, and `expires_at`. The controller seals those values with the binding and
`published_at`. Candidate revision v1 is immutable; replacement requires a new
candidate ID and publication.

All object keys shown below are required. IDs identified as UUIDs are lowercase
canonical UUID strings on the wire and are compared exactly. `daemon_id`,
`source_revision`, worktree path and branch are non-empty strings;
`candidate_revision` is the JSON integer `1`; timestamps are UTC RFC 3339
strings. There is no `identifier` or `title` field in either payload.

Controller publication:

```json
{
  "schema_version": "multica.preview-publication.v1",
  "binding": {
    "workspace_id": "11111111-1111-4111-8111-111111111111",
    "issue_id": "22222222-2222-4222-8222-222222222222",
    "agent_id": "33333333-3333-4333-8333-333333333333",
    "agent_run_id": "44444444-4444-4444-8444-444444444444",
    "candidate_id": "55555555-5555-4555-8555-555555555555",
    "candidate_revision": 1,
    "source_revision": "git:0123456789abcdef",
    "worktree": {
      "daemon_id": "bluefin-host",
      "runtime_id": "66666666-6666-4666-8666-666666666666",
      "path": "/workspace/issue-42",
      "branch": "agent/issue-42"
    }
  },
  "preview_url": "http://127.0.0.1:4173/",
  "identity_url": "http://127.0.0.1:4173/.well-known/multica-preview-identity",
  "data_policy": {
    "id": "synthetic-or-redacted-read-only-v1",
    "data_class": "synthetic_or_redacted",
    "mutations": "denied",
    "external_network": "denied",
    "credentials": "denied"
  },
  "published_at": "2026-09-21T00:00:00Z",
  "expires_at": "2026-09-21T00:05:00Z"
}
```

Preview-origin identity:

```json
{
  "schema_version": "multica.preview-identity.v1",
  "binding": {
    "workspace_id": "11111111-1111-4111-8111-111111111111",
    "issue_id": "22222222-2222-4222-8222-222222222222",
    "agent_id": "33333333-3333-4333-8333-333333333333",
    "agent_run_id": "44444444-4444-4444-8444-444444444444",
    "candidate_id": "55555555-5555-4555-8555-555555555555",
    "candidate_revision": 1,
    "source_revision": "git:0123456789abcdef",
    "worktree": {
      "daemon_id": "bluefin-host",
      "runtime_id": "66666666-6666-4666-8666-666666666666",
      "path": "/workspace/issue-42",
      "branch": "agent/issue-42"
    }
  },
  "data_policy": {
    "id": "synthetic-or-redacted-read-only-v1",
    "data_class": "synthetic_or_redacted",
    "mutations": "denied",
    "external_network": "denied",
    "credentials": "denied"
  },
  "expires_at": "2026-09-21T00:05:00Z"
}
```

URLs must be absolute loopback HTTP (`localhost`, `127.0.0.0/8`, or `::1`);
remote origins, userinfo and fragments are forbidden. Identity is exactly:

`GET {preview-origin}/.well-known/multica-preview-identity`

It is same scheme, host and port as the preview. A successful response is
`200 application/json`, `Cache-Control: no-store`, schema
`multica.preview-identity.v1`, and repeats byte-equivalent decoded values for
the complete binding, data policy and expiry. The Shell must fetch and validate
identity immediately before rendering. A reachable page with missing,
expired, stale, partial, or mismatched identity is not a preview.

The producer endpoint uses no credential; access is host-local and its content
must satisfy the non-sensitive policy below. The `msr_` Bluefin reader token is
sent only to the controller and must never be forwarded to preview origin.

The only accepted producer-enforced policy is:

```json
{
  "id": "synthetic-or-redacted-read-only-v1",
  "data_class": "synthetic_or_redacted",
  "mutations": "denied",
  "external_network": "denied",
  "credentials": "denied"
}
```

The producer must enforce it, not merely label the response: no production
credentials or unredacted production data, no mutating methods or background
writes, and no external network. The Shell treats policy drift as fail-closed.

Expiry is strictly after publication and at most five minutes later. At expiry,
the controller stops returning the publication, the Shell stops rendering it,
and the producer must return `410 Gone` and terminate the preview. Identity at
the exact expiry instant is expired. The Shell validates
`expires_at <= published_at + 5m` from the controller publication, then requires
the identity `expires_at` to match exactly. Identity deliberately omits
`published_at`; checking only `expires_at <= fetch_time + 5m` is allowed as an
additional conservative bound but is not proof of issuance and cannot replace
the publication-timestamp and exact-match checks.

## Implemented controller endpoints

- `POST /api/daemon/tasks/{taskId}/preview-publication` — existing
  daemon/task-scoped authentication only. The controller derives every binding
  and provenance field from the locked task, action, candidate, runtime and
  daemon rows; request identity fields are rejected, not trusted.
- `GET /api/shell/issues/{issueId}/preview-publication` — existing scoped
  Bluefin reader token only (`audience=bluefin-work`, workspace membership
  rechecked). Exact issue path and token workspace must match. Response is
  `404` before publication, `200` while current, and `410` after expiry.
- `GET /.well-known/multica-preview-identity` on the preview origin — no
  Multica credential; read-only identity described above. Every non-GET/HEAD
  method returns `405`.

All three responses use `Cache-Control: no-store`.

The POST body contains exactly `preview_url`, `identity_url`, `data_policy`,
and `expires_at`; unknown fields (including any binding or provenance field)
are rejected. A first registration returns `201`; an authority-identical,
semantically identical replay returns the stored object with `200`; any other
replay returns `409`.

The controller preallocates `correction_candidate_id`, revision `1`, and an
opaque immutable `preview_source_revision` in the settled `review_correction`
action before its task is dispatched. The source revision is the
controller-allocated candidate source namespace
`candidate:{candidate_id}:1`; it is a protocol revision, not a claim that the
worktree has a Git commit. The producer must bind the exact served snapshot to
that revision in its identity response. Registration permits dispatched,
running, or completed tasks, but only while the exact failed-review/action
authority, runtime/daemon, absolute persisted worktree path, and persisted
branch are present. It never consumes those values from the publication request
or task result.

The Shell read returns only the newest exact-issue publication. It returns
`404` when absent or ineligible, `200` while eligible and unexpired, and `410`
at or after expiry. Eligibility is derived on every read: cancellation/failure,
candidate supersession, runtime/daemon disconnect, action drift, or membership
loss fails closed. Rows are retained as immutable records; availability is not
implemented by mutation.

## Producer responsibility

The daemon-side Bluefin producer must persist its worktree and branch on the
task before registration, serve the exact identity response, enforce the
read-only/no-network/no-credentials policy, and terminate on expiry or when its
task is cancelled. The native Shell must fetch `identity_url` immediately
before rendering and call `VerifyIdentity`; the controller response alone is
not sufficient authority to render.

Implementation is based on accessible Git source
`4475751eaa761dac6187326289791887d6e07cf1`. The noted deployed identity
`4475751ea22b...` is an unavailable local overlay, so behavior unique to that
overlay could not be inspected or integrated.
