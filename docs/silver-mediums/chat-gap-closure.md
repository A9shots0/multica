# Chat gap closure - 2026-09-24

Scope: finish the previously identified local Chat gaps, preserve Multica's
existing shell, and restore the owner's preview. No push or deployment.

- [x] Restore API, web and dedicated review worker; verify Chat visibly loads.
- [x] Expose recorded stage, responsible actor and dependency reasons in the
  existing bounded work area, without inventing progress for missing evidence.
- [x] Browse older workflow activity inside Chat with scoped, stable cursors,
  retries and no replacement of the current status snapshot.
- [x] Verify multiple issues plus versioned approval/rejection, changed evidence,
  and unrelated pending questions remaining untouched.
- [x] Verify desktop/phone, preserved draft, and failure behavior in Browser.
- [x] Run focused gates and review; record broader release blockers below.

Files: core Chat/API/query modules; shared Chat work components and English
locale; canonical review response components if required by stale-review checks;
focused tests; local acceptance seed/helper; this record and project changelog.

Current source baseline: 0b3e57f44. Preview database remains isolated on 55438;
test database on 55440. Source refresh in the shared brain remains pending.

## Observed results at 2026-09-24 20:30 UTC

The built-in Browser verified the existing desktop shell and a true 393x852
phone viewport (document width 393). Work remains bounded to 35dvh, leaving the
composer available. An unsent draft survived opening history and changing the
viewport. History loaded 50 then 60 fixture events; the final page had no older
button. Cursors retain microseconds and the event ID; unlinked issue history is
excluded, including requests using previously captured cursors.

Current detail displays the recorded step, the responsible agent/member name,
dependency reason, and known next event. Missing or unknown contracts do not
invent a stage or actor. Existing fonts, colors and global navigation remain.

The canonical review packet permits an empty custom-choice list: approve/reject
is described by its response schema. The UI previously rejected that valid
packet. Review text and version are now held together until the owner explicitly
loads a changed request. Tests and Browser confirmed stale approval is disabled.
Failed submission keeps the typed change request. Responses use the captured
revision and preserve the note once, regardless of option ordering.

Final disposable acceptance session: f8ddea4f-c087-40ae-a3bd-45c648fab603.
Browser submitted approve to decision b35ad9d1-5a5d-5e0a-9d33-e56021c97ab4,
then reject with a note to 2c343f7d-3b16-5982-86bf-dc7e360b218e. Database readback
confirmed one response each and the exact note. The other decision stayed
unanswered after the first response. Answering now archives only that owner's
matching decision notification in the response transaction; it no longer leaves
Chat asking the same question. Raw event-code fallback and stale question badges
were also corrected. These synthetic records deliberately do not dispatch a real
continuation; their unavailable execution state is not completion evidence.

The earlier fixture session 11bdd694-5602-43db-8373-dae2e615177d contains a
deliberately inconsistent revision injection used to verify conflict handling.
Its retained failed note is not a production error. The guarded local helper is
`server/cmd/chat-gap-fixture`; it accepts only loopback database 55438 named
multica_review. No production data or provider was used.

## Crash isolation and running preview

Windows Application event 1000 at 2026-09-24 15:51:49 EDT records bundled pwsh.exe
failing in coreclr.dll with 0xc00000fd (stack overflow). The tool shell and its
children disappeared. This is evidence of a shell crash, not proof of a Chat
application fault or a resolved host root cause.

Web, API and dedicated worker were relaunched hidden through Win32_Process.Create,
outside the tool session's process tree. This is best-effort persistence, not an
OS startup service. The API launcher now supplies the isolated workflow controller
identity; without it, canonical responses returned HTTP 503. Other running apps,
the global daemon, and the unrelated port-3000 server were preserved.

At this snapshot: web PID 50948 (parent 28616), API PID 44880 (parent 52280),
worker PID 45472 (parent 14900), profile mika-chat-review-20260924. API health and
Chat work routes returned 200; the review worker heartbeat was online. Before
stopping a preview, recheck the exact PID/path/profile, never stop all node or
multica processes. No autostart or machine-wide setting was changed.

Preview: http://127.0.0.1:3221/mika-local-review/chat?session=01a0d454-5dfb-743b-918d-055136d981a8

Hidden launchers and logs: `C:/Dev/_work/mika-chat-prototype-20260924/`:
start-catch-up-web.ps1, start-followup-api.ps1, start-followup-worker.ps1.
Loaded backend is mika-chat-gap-v3-server.exe built from this working tree with
`-buildvcs=false`; it does not carry a verified embedded revision stamp.

## Verification and remaining boundary

- Core: 16 files, 358 tests passed (Chat, schemas, owner Inbox model).
- Views: 34 files, 325 tests passed (Chat, versioned response, media viewer).
- Views TypeScript passed. New Chat component ESLint passed. Touched legacy Inbox
  components have 35 existing localization errors, also reproduced from HEAD
  (18 card, 17 detail); this pass introduced no additional lint errors there.
- Go: Chat work/history, exact decision/idempotency, independent question and
  operator-completion contention tests passed against isolated PostgreSQL 55440.
  Broader decision tests in workflowcommand passed. Server build passed.
- The contention check found completion waiting on the issue lock before its
  existing nonblocking plan lock. The issue lock is now also NOWAIT; concurrent
  legacy transitions get a conflict instead of hanging. All three contention
  scenarios passed afterward. Generated sqlc source was refreshed.
- `git diff --check` passed. Browser returned to the real-worker acceptance
  conversation, still displaying one Done issue. Viewport override reset.

Release is **not certified**: TestApplicationV2WritesUseAdmissionTransaction
still identifies pre-existing direct projection writes in workflow_control.go,
owner_runtime_projection.go and workflow_legacy_guard.go. Those sources are
unchanged from 0b3e57f44. Follow-up must bring legacy projection writes under the
admitted write boundary without weakening the guard, then rerun the broader
workflow suite. This supersedes the prior note's blanket no-blocking-finding
claim. Production-scale qualification and host-shell root-cause resolution are
also unproven. No push, merge, deployment or production mutation occurred.

Evidence logs: chat-gap-core-final.log, chat-gap-views-all.log,
chat-gap-typecheck-final.log, chat-gap-new-lint.log, chat-gap-lint-baseline.json,
chat-gap-handler-final.log, chat-gap-backend-final.log, catch-up-api.log and
catch-up-web.log under the artifact directory above. The broad backend log
contains the remaining admission-guard failures and the initial contention
failure; the final handler log records the corrected contention result.

### Admission follow-up, 2026-09-24 21:56 UTC

Local code checkpoint `9f47cb811ef336d362a754711763abf513318891` supersedes the
three-path admission blocker above. The unchanged architectural guard passes;
concurrent exact completion replays now serialize without losing the legacy
contention protection. See `chat-admission-progress.md` for 971 passing backend
test/subtest events, 18 optional Redis skips, build/vet evidence and exact local
preview runtime readback. Production qualification and Windows crash root-cause
resolution remain unproven.
