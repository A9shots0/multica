# Independent integration review - 2026-09-24

Final checkpoint: scoped transport, history/evidence retrieval, installed-provider
context/read/handoff, and independent other-device readback passed. The original
failed provider attempt remains documented below. Multica production release and
owner-facing Chat acceptance are still pending; this review does not approve them.

## Scope and source

Reviewed the shared-brain integration in an isolated worktree at
`C:/Dev/_work/multica-brain-owner-review-20260924`, based on owner Chat checkpoint
`a568d6daf65d3505e7b736a74f43413c926f080a`. Integration `844e23e27` applied as
`816604b64`; receipt correction `518a0372` applied as `b51dd4648`.
The owner checkout, backend3220, web preview3221 and database55438 were preserved.
No push, deployment, owner database migration, or production change occurred.

Fresh PostgreSQL18 database55440 and backend3230 supplied local runtime evidence.
The protected service mapping allowed only workspace
`99728d42-73bc-47e0-b51c-c4394320f8f0`, project
`dc392fac-9488-4db1-93d0-519797f876f6` to brain project `multica`, without a default.
The review path is not enrolled in the brain; exact project resolution used the
enrolled owner checkout. Immutable brain prose was reference, not authority.

## Findings and checks

- Reproduced a delivery-reporting defect independently: a bare versioned HTTP200
  response was accepted without a durable event receipt. The new regression
  failed before correction and passed afterward. Delivery now requires a valid
  recorded task-observation receipt with acceptance not established.
- Fresh migrations583-586 passed. Focused client, real-database lifecycle/replay,
  handler, daemon MCP and owner Chat target/readback regressions passed. `go vet`
  passed for brain, handler and daemon packages.
- Selected-issue regression passed: a Chat message targeting a linked issue uses
  that issue's project instead of the conversation project. Changing the issue
  revision before claim withholds context and makes no further service call.
- Actual HTTP claim, daemon task-scoped MCP context, complete immutable read,
  start and completion were exercised against the live shared service. Repeating
  each logical handoff twice produced one outbox row per event identity.

Transport task: `c313c2c0-b811-4399-bf8b-d8ec482a8074`. Its four outbox rows
(start, completion, one pre-correction handoff and one post-correction handoff)
were delivered without retry or blocked rows. Handoff source UUIDs were
`351931a9-bcde-4efc-ac6e-58f206158614` and
`04e0ef9c-1605-499e-8e02-1a9696b5451e`; these are distinct test runs, not duplicates.

Independent Drew Mini readback at `2026-09-24T06:54:32.657420Z` found the same four
events under its enrolled identity. Completion receipt:
`b4c9cac6-5ff5-4b8d-9bf5-18db3f43ec4b`. Every record explicitly leaves acceptance
unestablished.

## Initial real-provider acceptance blocker

Historical result below is preserved. A subsequent correction and passing run
are recorded in the follow-up section; the first failed run is not reclassified.

An opt-in installed Codex test used the actual Multica provider adapter, fresh
temporary Codex home, task-scoped MCP broker and existing authentication copied
as opaque bytes into the temporary home. Credentials/config were not printed.
An initial Windows symlink setup attempt failed before any provider launch.

The subsequent provider turn ran for12.75 seconds and discovered `brain_context`,
but reported that the tool required approval while the test's approval policy
was `never`. It did not call `brain_read` or `brain_record_event`. The test failed
as intended and the fixture recorded that blocker. No approval gate was bypassed.

Provider task: `2b57dc4e-0f5a-410e-a81b-6fcd57d655d8`. This demonstrates a blocker
in the smoke configuration (`read-only` plus `never`), not proof that the ordinary
deployed runtime has the same defect. The full daemon execution-environment and
its managed MCP approval policy still require acceptance with a genuine provider.
No claim of a working Mika brain conversation, issue acceptance or deployment.

## Evidence and next action

Evidence directory: `C:/Dev/_work/brain-completion-evidence`:

- `owner-review-tests.log`, `owner-review-selected-target.log`
- `owner-review-final-tests.log`, `owner-review-final-vet.log`
- `owner-review-live-transport-fixed.log`
- `multica-cross-device-readback.json`
- `owner-review-provider.log`

Next: resolve the scoped MCP approval policy in the authorized ordinary runtime,
then rerun one bounded provider task and independently read its handoff back.
Integrate/release only after the owner accepts the local Chat changes and the
remaining runtime gate is satisfied. Cloud runtimes without daemon identity
remain outside this transport's acceptance scope.

## Follow-up: history retrieval and tool semantics

Added project-bound `brain_events` and immutable `brain_evidence` retrieval with
bounded pagination, optional exact task filter and returned-identity validation.
Historical events may omit redundant payload identity fields; conflicting fields
are rejected. Evidence requires the exact revision and a complete observation.

The local MCP broker omitted tool annotations. Accurate local-only annotations
now describe the four reads as read-only, non-destructive, closed-world and
idempotent. Handoff remains an idempotent append with read-only false. Claim
payloads cannot supply these annotations. Global/provider approval settings and
sandbox policy were not changed.

Fresh provider task `5e45b438-a718-46a4-bf94-4d4b62b64425` passed using the same
`read-only`/`never` settings. The actual installed Codex adapter observed exactly
one call each to `brain_context`, `brain_read` and `brain_record_event`, completed
in24.65 seconds and returned one usage record. ProviderModels was empty; no exact
model is inferred. This proves that the corrected scoped broker is usable by a
real provider. It does not exercise every daemon execution-environment branch or
the owner-facing Mika chat flow. Completion was recorded through the real local
daemon API; independent handoff readback is tracked separately.

Evidence: `owner-review-provider-hints.log`, `owner-history-tests.log`,
`owner-history-handler-tests.log` in the evidence directory above. Local history
client tests, broker tool semantics, handler scope tests and affected package
vet passed. The local review backend build used `-buildvcs=false` after Go VCS
stamping failed; this is not a release binary. No migration was added.

## Final live and cross-device readback

After the integration task confirmed shared-service release
`5b6cb7af19ee693e6a7a9c4accb585bdc4a74099`, the live client test paginated the
exact transport task at one event per page: all four unique events returned,
without duplication. Evidence `3cb84e91-cfad-418c-a25e-f93fe67de84d` revision
`47791dc97ed78b1550348397ad8c222942c670927c6f9c93b052307566c41df6`
read successfully; an all-zero wrong revision was rejected. This observation is
unreviewed local-code/test evidence, not production proof.

Drew Mini independently read the successful provider task's three records at
`2026-09-24T07:14:22.207983Z` through its enrolled identity, with exact task filter,
online connection and no further pages:

- Start: `a9e533ed-2cb3-41d3-89cf-0131abc48693`.
- Provider handoff: `d7a057d0-b93a-43a2-b767-540b826c798e`.
- Completion: `f34a5588-7262-480b-9aa0-9dc66d232b4e`.

All retain acceptance `not_established`. Evidence files:
`owner-live-history-evidence.log` and `multica-provider-cross-device-readback.json`
in the evidence directory. Six disposable daemon tokens were revoked and three
protected claim files removed after completion. Provider temporary homes were
test-cleaned; service mapping and durable evidence remain. Future explicit
smoke tests need a new claim, not a completed task's credentials.

Review/history commits `3f05e919d` and `9f897a80d` were integrated by the parent
task into its source branch as `ce2d75376` and `78a48c1e4`. Owner Chat checkout
and its pre-existing dirty files remain unchanged. Next decision is the owner's
local Chat acceptance and a separately authorized Multica release; no further
provider run is needed for this bounded review.
