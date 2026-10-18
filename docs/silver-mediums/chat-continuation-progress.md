# Chat continuation

Local implementation authorized 2026-09-24. Preserve Multica styling and the
existing isolated preview. No push or production deployment.

- [x] Durable catch-up: explicit acknowledgement of exact delivered events,
  independent of opening Chat or resolving an owner decision.
- [x] Compact catch-up in existing Work details; same evidence in Mika context.
- [x] Verify concurrency, access, stale responses, mobile and reconnect behavior.
- [x] Review and commit the local slice.
- [x] Autonomous Mika interpretation and continuation using existing execution
  authority (never replay the last user message as a new instruction).
- [x] Versioned approvals and a realistic multi-issue end-to-end workflow.
- [x] Cross-device organization and Mika organization tools.

The first slice deliberately distinguishes unseen work from ordinary chat unread
state. Reading a conversation must not silently discard unseen work. Exact event
IDs prevent a newly arriving event from being acknowledged by an older screen.

## Verified local slice, 2026-09-24

Preview: http://127.0.0.1:3221/mika-local-review/chat. The combined checkout
`C:/Dev/_work/multica-brain-owner-review-20260924` supplies API 3220 and web 3221.
The existing review database remains on 55438; migrations 583-587 were applied
after a `pg_dump` backup. No production database, push or deployment was touched.
Hidden background launchers and logs are under
`C:/Dev/_work/mika-chat-prototype-20260924/start-catch-up-{api,web}.ps1`.

- Opening Chat no longer implicitly clears the new work catch-up state. An
  explicit Mark seen acknowledges an exact batch, independently of Inbox decisions.
- A quiet New badge opens the existing bounded work area. Each issue appears
  once, with current status plus its latest unseen event; report excerpts are
  available by expanding that issue. No new transcript cards or mobile screen.
- `multica chat work` and the claim-time Mika snapshot share this persisted
  evidence. Mika's prompt distinguishes historical updates from current state.
- Catch-up is bounded at 100 deliveries, with a visible partial-batch indication.

Checks passed: focused Go ChatWork/ChatCatchUp handler suites against isolated
test DB 55440; daemon prompt suite; core work tests (7); catch-up component tests
(3); shared views typecheck and focused ESLint; Go build and handler/service vet.
The local binary uses `-buildvcs=false`; this is not release provenance.

Built-in Browser verification: actual DOM dimensions 393x852, scroll width 393;
expanded work stays within 35dvh and the composer remains accessible. Clicking
Mark seen removed only the New markers, retaining 4 issues (1 Waiting, 3 Stopped).
A process restart and fresh tab at actual 1280x850 preserved that acknowledgement
and retained the transcript/results. The preview process exit during a tooling
reconnect was recovered using hidden background launchers.

Focused sequential review covered ownership/access, exact-ID race handling,
bounded payloads, backward-compatible schemas, Query ownership, package/style
rules and test coverage. Browser review caught duplicated issue rows and a file
encoding regression; both were corrected before final checks. Specialized review
subskills other than AGENTS adherence were unavailable; those checks were done
directly. No outstanding blocking finding in this slice.

At that earlier checkpoint the connected follow-up, grouping and approval work
was incomplete. The continuation below closes those local implementation gaps.
The separate brain-upkeep agent and global worker were not modified.


## Completed local Chat continuation - 2026-09-24

The remaining agreed Chat workflow is implemented and verified locally. This is
not production deployment or a claim that every future Chat feature exists.
Existing Multica shell, typography and colors are preserved.

### Behavior and source boundaries

- Groups, membership, grouping mode and filters now live in an owner/workspace
  scoped server library. Locked merge operations preserve unrelated concurrent
  changes. Device reads cannot replace a newer revision. Previous local groups
  import once, with an explicit retry if disconnected. Mika can read/create,
  rename, remove and move conversations using `multica chat organize`.
- New questions, results and stopped runs trigger a short automatic Mika reply.
  Nearby events coalesce; historical events are not replayed on upgrade. The
  report receives current linked work and new evidence, has no mutation tools,
  and cannot reuse an old human request as fresh authority. The human provider
  conversation remains available for its next reply. Reporting currently uses
  the capability-advertising Codex worker; other runtimes still receive durable
  work updates but do not silently claim this automatic narration capability.
- Questions can be discussed before deciding. Explicit answers retain the exact
  owner message, question identity and revision, then queue one continuation.
  Versioned workflow approval uses its canonical decision/evidence gates.
  Response loss can replay the same receipt, but not change approve to reject.
- Ordinary finished work can be accepted in Chat. Acceptance binds the captured
  task, result digest, owner and issue revision. A replacement result, new run,
  new question or workflow-controlled issue rejects that ordinary acceptance.
  Accepted ordinary work becomes Done without dispatching another worker.
- Compact routine updates, durable catch-up, visible Re: reply labels, issue
  topic selection, readable Activity and composer request shortcuts remain in
  the existing Chat surface. A no-text-reply state remains visible in the list;
  normal message previews and repeated agent names do not crowd every row.

### Actual connected acceptance

Conversation: `01a0d454-5dfb-743b-918d-055136d981a8`, now named
**Welcome message review**, in **Content reviews**.
Issue: `01a0d456-2f51-790c-af3d-dc64ae217f1b`, **Improve the welcome message**.

The installed Codex worker created, linked and dispatched the issue from Chat.
An installed-Codex path change interrupted its first run; the real failed result
was reported automatically, retained in history, and successfully retried after
correcting only the dedicated review launcher. Current provider version:
0.155.0-alpha.16.4. The unrelated global daemon and offline brain-upkeep agent
were not changed.

The worker asked friendly vs concise. Mika explained its recommendation without
answering or resuming work. The explicit choice was then submitted through Chat;
the worker returned the chosen wording and Mika reported it automatically.
The final owner message, `I accept this finished welcome wording. Mark this issue
complete.`, produced one applied accept_result receipt and issue status Done.
Database readback found zero worker runs after acceptance and three completed
automatic reports (failure, question, result). No issue-board visit or Inbox
response was used to complete this acceptance flow.

### Verification

- Backend Chat, report isolation, decision and workflow suites passed across six
  packages against isolated PostgreSQL 55440. Additional acceptance regression
  covers same-receipt retry, changed result, newer run, newly arrived question,
  workflow exclusion and conflicting replay. Multi-issue tests cover independent
  working, waiting and needs-owner states plus exactly-once answer continuation.
- All 318 Views Chat tests (31 files) passed; 12 Core library/work tests passed.
  Views typecheck, focused ESLint, backend vet and server/CLI builds passed.
  Existing stale test fixtures were updated to the workspace/query boundaries
  and owner-approved compact list behavior, preserving archive/keyboard tests.
- Built-in Browser verified true 393x852 and 360x800 at scale 1 with matching
  document widths. The work section remains bounded and the composer visible;
  saved reply context and completed status persist. No new mobile screen.
- Group rename through the phone UI persisted into a fresh desktop tab. Mark
  seen cleared exact unseen updates without removing the completed issue.
  Viewport override reset; preview remains available for owner review.

Preview: http://127.0.0.1:3221/mika-local-review/chat?session=01a0d454-5dfb-743b-918d-055136d981a8
Evidence logs and hidden launchers: `C:/Dev/_work/mika-chat-prototype-20260924/`.
`chat-complete-{backend,core,views,vet}.log`, `chat-final-acceptance.log`,
`chat-final-typecheck.log`, and `chat-final-lint.log` retain focused results.
Review DB 55438 only: migrations 588-590, backed up before each material phase;
latest backup `pre-result-acceptance.dump` (689153 bytes). Test DB 55440 is separate.

Sequential review covered scoped access, immutable inputs, coalescing, stale
requests, idempotency, unsupported runtimes, server-state ownership, restart and
mobile behavior. No known blocking finding remains in this local Chat scope.
Production-volume qualification and deployment are separate, unperformed lanes.
Subsequent 2026-09-24 verification found additional response, notification and
lock-contention defects, corrected in `chat-gap-closure.md`, plus an unresolved
legacy projection admission guard failure. That newer evidence supersedes the
no-known-blocking-finding statement above; the release is not certified.
Shared-brain source refresh remains pending; fresh source and isolated runtime
observations establish these results, not an older brain guide.
