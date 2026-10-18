# Chat response format selections

Local-only continuation, 2026-09-24. User rejects composer actions inserting
boilerplate sentences into their draft. Preserve Multica styles and navigation.

- [x] Inspect composer, send, persisted task context, history and runtime paths.
- [x] Replace prompt insertion with one removable, draft-scoped format selection.
- [x] Send structured format metadata and deliver it to Mika separately.
- [x] Preserve format labels on history, editing and draft recovery.
- [x] Verify focused tests, typecheck, lint and actual local Browser behavior.
- [x] Review and commit code; record bounded evidence/handoff.

## Evidence: 2026-09-24 23:59 UTC

Code commit `77431292149dc57e3f50220770da81f7324d7f19`, local only.
API 3220 `/health` reports that exact commit, PID 39368, started 23:57:41Z.
`/healthz` and `/readyz` report database and migrations OK. Web 3221 uses this
worktree. Only the isolated review API was restarted; other services preserved.

Behavior: one of Plan / Compare / Checklist / Diagram is stored separately from
the draft. Selection replaces selection, removal leaves text unchanged, failure
retains it, and successful send clears it. History carries a small format label.
Backend validates the enum, persists per-task context, and adds fixed format
guidance to the claimed worker input. Original message content and title remain
unchanged. Selection is presentation intent, not authorization to execute work.
Agent Builder does not advertise formats because its endpoint does not support
this metadata.

Verification artifacts in `C:/Dev/_work/mika-chat-prototype-20260924/`:

- `response-format-views.log`: 111 passing tests across composer, controller,
  draft isolation and restoration.
- `response-format-message-final.log`: 25 passing message-list tests.
- `response-format-core-final.log`: 189 passing API/store/schema/realtime tests.
- `response-format-handler-final.jsonl`: 28 passing Go test/subtest events on
  isolated PostgreSQL 55440, including actual task claim, exact original text,
  metadata scope, edited turn, durable draft restore and next-turn isolation.
- Views typecheck, focused Views/Core lint, Go vet and backend build pass.
- Core-wide typecheck remains blocked by an unchanged pre-existing fixture in
  `packages/core/inbox/owner-inbox-model.test.ts:40` (`workflow_update` is not an
  allowed InboxItem type). This fixture matches the prior HEAD. No global clean
  typecheck claim is made.

Built-in Browser local worker acceptance:
`http://127.0.0.1:3221/mika-local-review/chat?session=01a0d5db-68ea-7975-a2b7-c684d655f8eb`

Selected Compare options and sent an ordinary question about immediate alerts
versus a daily digest. Mika returned a comparison table in 29 seconds. Original
message text and small format label remained after reload; next composer had no
format selected. A separate Checklist test draft and its selection both survived
reload; removing the chip preserved the text. Test draft cleared afterward.
Measured phone viewport 393 by 852 CSS pixels, composer within bounds with no
document horizontal overflow; temporary viewport overrides reset. Browser's
screenshot helper scaled the image incorrectly, so direct in-tab CDP screenshot
was used to inspect the actual rendering. No separate browser or desktop control.

Only Comparison was exercised through a real worker response. All four enum
values and invalid values are covered by structured-path tests; visual quality
of every generated diagram/plan/checklist is not certified by this check.

Owner changes under `apps/web` remain excluded. No push, deployment or production
mutation. Shared brain source refresh remains pending; this is new bounded local
evidence, not certification of the broader Chat workflow.
