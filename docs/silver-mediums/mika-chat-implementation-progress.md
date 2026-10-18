# Mika work management implementation

Owner authorized local implementation on 2026-09-23. Preserve Multica's shell and styling. No push, deployment, or mutation of live work. Continue in `codex/mika-chat-local-review`.

## 1. Shared conversation work capabilities
- Depends on: -
- Tasks:
  - Read existing issue, Chat, Inbox and execution contracts.
  - Link newly created work atomically to its originating conversation.
  - Share authoritative work snapshots and durable activity between Mika and Chat.
  - Support multiple issue dispatches in one Chat turn without duplicate execution.
  - Give Mika targeted, owner-authorized decision and work controls with receipts.
- Files: server handlers, services, queries, CLI, daemon prompt and their tests; core Chat API, types, queries and tests.
- Acceptance criteria: same scope and facts for UI and Mika; ownership, stale state, replay, ambiguity and duplicate-action tests pass.

## 2. Chat interaction and organization
- Depends on: 1
- Tasks:
  - Add compact work summary, bounded details, updates and explicit discussion target.
  - Reuse canonical Inbox decisions and versioned evidence inside Chat.
  - Correct Working/Needs You filters; persist personal grouping and view preferences.
- Files: packages/core/chat, packages/views/chat, shared Inbox components, locales, tests.
- Acceptance criteria: preserve existing visual system; drafts and scrolling survive updates; multiple issues and mobile layouts work.

## 3. Local acceptance
- Depends on: 1, 2
- Tasks:
  - Run an isolated database/backend and scratch workflow; keep existing services intact.
  - Verify three issues in one conversation, owner decision, dependent work and reconnect.
  - Run typecheck, relevant tests, lint and independent review.
  - Record local evidence, limitations and handoff; commit locally only.
- Files: test fixtures, local verification scripts, this checklist and local review evidence.
- Acceptance criteria: no false working/done/submitted claims; actual receipts and browser evidence support each demonstrated capability.

## Progress — local milestone, 2026-09-24 UTC
- [x] Read approved behavior, source, ownership and authorization paths.
- [x] Atomic origin/child links and multiple dispatch receipts per Chat turn.
- [x] Shared UI/Mika work projection, canonical pending requests, latest durable activity and CLI refresh.
- [x] Explicit owner-selected issue/revision, exact current owner input, canonical response handlers and replay checks.
- [x] Compact work panel and About target; canonical Inbox response inside Chat.
- [x] Personal custom groups, Recent/Projects and issue-work filters. Groups currently persist on this device only.
- [x] Isolated database/backend with three scratch issues; actual browser answer archived the Inbox question and queued exactly one continuation.
- [x] Focused backend tests, core/view tests, views typecheck, Chat lint and diff checks.
- [x] Sequential source review corrected shared-runtime owner identity, invalid child-link source, duplicate UI keys, test helper usage, stale target capture and Inbox/Chat cache invalidation.
- [x] Owner's narrow-panel feedback: container-sized filter dropdown instead of wrapping labels; group titles truncate without losing counts.
- [x] Corrected phone test: browser zoom made nominal 393px render at 206px. Measured corrected page is 393x852 CSS pixels with no horizontal overflow. Earlier nominal-size checks are insufficient evidence.
- [x] Connected Codex-backed Mika trial: read shared state, created/auto-linked and dispatched scratch work, discussed a pending question without answering, submitted an explicit answer, and resumed the worker exactly once. Downloaded output contains Compact. This proves the ordinary-question path, not versioned approval/release acceptance.
- [ ] Full approved design: server-synced grouping and Mika organization tools; proactive narrative delivery/read cursors; older event paging UI; dependency/milestone detail and all work controls.
- [x] Multiple pending questions require a specific owner-selected request; regression coverage rejects ambiguous/resolved selections. Browser verified the specific-question About label and ordinary discussion/answer path.
- [ ] Versioned approval and simultaneous-question browser acceptance remain to be exercised.
- [ ] Full release/build checks, independent review and owner acceptance. No deployment authorized.
- [x] Narrow-toolbar correction passed views typecheck and focused Chat lint. New phone preview stays open at the owner-requested phone size.

## Evidence and scope
Review URL: http://127.0.0.1:3221/mika-local-review/chat
Worktree: C:\Dev\multica-mika-chat-local-review, codex/mika-chat-local-review, base 8080a0e44.
Evidence directory: C:\Dev\_work\mika-chat-prototype-20260924.

- backend-final-tests.log: handler/service/daemon focused tests passed, including exactly-once answer, no target, stale target before/after send, unlink, wrong owner/agent, shared runtime, cancel replay, origin/child links and inherited Chat editing/prompt tests.
- core-tests.log: 32 tests across work/library/store. view-tests.log: 5 library/preview tests. Earlier media/annotation suite result remains in mika-chat-local-review.md.
- typecheck-final.log and chat-lint-final.log: passed after responsive toolbar correction. Repository-wide lint is not green; existing Inbox debt is separately recorded in lint-comparison.json.
- phone-library-corrected.png and phone-conversation-corrected.png: corrected full-width phone library and conversation, measured 393x852 CSS pixels in a fresh Browser tab. Browser zoom requires checking actual document width, not trusting viewport control labels.
- Browser answer receipt: intake continued, Inbox archived, continuation 01a0d159-aaac-7e31-9802-1008bdafa529 queued once. No real business issue or live database changed.

Local prerequisite repairs: missing Chat editing service implementation (existing six focused tests pass), optional IntelligenceDB handler field required by inherited source, optional loopback bind for isolated backend, and optional .next-review build directory. Keep these visible during review.

## Local services
Next.js 3221 forwards API to isolated Go 3220; both loopback-only. PostgreSQL container multica-mika-review-pg is bound to 127.0.0.1:55438 and contains disposable multica_review fixtures. Seed script scripts/mika-local-review-seed.mjs refuses other database targets. One Codex worker is connected through the disposable mika-chat-review-20260924 profile. The original offline fixture agent remains separate. The named review worker uses only the checkout CLI, Codex provider, max concurrency 1 and scratch workspaces; other installed providers are disabled before probing.

Do not point this preview at the live writable backend as a shortcut to acceptance. Existing owner checkouts, containers, and unrelated hub changes were preserved.

Local checkpoint: `13493064b` (not pushed). Next-generated apps/web next-env.d.ts, tsconfig.json, AGENTS.md and CLAUDE.md remain outside the product commit. Hub validation still reports the pre-existing Intelligence remote-claim gap (zero active intelligence.remote.main claims); this task did not alter that registry.


## Connected acceptance continuation — 2026-09-24 UTC

Local-only. Connected session `01a0d16f-ccac-7d02-97b9-1c7940e0ffdb` is titled Mika connected acceptance. Project `3ed4cfb2-7e46-4a61-9bca-83cfdfecaa57` and issue DEMO-4 (`01a0d17a-41ab-7cd8-b12e-67003b0ffff2`) contain only disposable synthetic work.

Observed results:
- Fresh `multica chat work` matched the UI: two queued fixtures, one waiting fixture. Status-only request did not start work.
- Chat created DEMO-4, automatically linked it and queued worker `01a0d17a-6ac2-7d8a-81a5-7a4a054f2d55` through chat bind. Actual execution was observed separately.
- Worker reported canonical request `e43a4a21-af1f-59d9-871b-76b1dd3b0f68`. Work in Chat displayed the question without an issue-page visit.
- Browser selected Discuss with Mika. About displayed the actual question. Discussion left Inbox pending/unarchived and the issue with one task.
- Browser explicitly answered Compact. Mika called chat respond; Inbox became continued/archived. Exactly one continuation `01a0d17e-ed40-7225-aa16-368c0f72384a` ran and completed.
- Attachment `01a0d17f-ed53-7525-86f0-72a2a04c472d` downloaded through the owner API and contains exactly Compact after trimming its newline. The issue remains in_review, not accepted/deployed.

Corrections found by the real trial:
- An older globally installed multica CLI could not run chat work. The review launcher now prepends this checkout's binary as multica.exe to the worker PATH. No global CLI was replaced.
- The hand-written seed issue numbers had not advanced workspace.issue_counter. Seeder and existing disposable fixture counter corrected.
- Header linked-issue count was stale after agent creation; it now refreshes on the same cadence as Work and after reconnect.
- Multiple questions on one issue no longer silently choose the first. Exact request targets are recaptured at send time; ambiguous submissions and resolved requests fail closed.
- Resolved About targets clear so later ordinary messages are not blocked by an obsolete request. Unavailable snapshots and revision-only changes do not silently retarget.
- Completed worker plus in_review means waiting for review, not working/done or unavailable.

Evidence: question-selection-tests.log (focused handler/service/daemon suites); connected-answer-receipts.jsonl; connected-work-receipt.json; connected-acceptance-note.txt. Core work suite: six tests passed. Views typecheck and focused Chat lint passed. No production database, live issue or external business service was used.

Launch helpers: scripts/mika-local-review-worker.mjs bootstraps only the dedicated DB/profile and omits token output; scripts/mika-local-review-worker.ps1 takes an explicit installed Codex executable. Stop only this named review daemon before replacing its binary. Initial default provider detection ran installed CLI version probes; it was stopped and replaced by explicit Codex-only configuration before any worker task. No non-Codex task was launched.

Remaining: proactive narrative/result delivery and read cursors, server-synced groups/Mika organization controls, versioned approval acceptance, older history UI and dependency controls. The worker's issue result does not yet become an automatic Mika transcript update; do not claim that loop is complete.

Current review binaries: server/bin/mika-review-server-connected.exe on 3220, and server/bin/mika-review-multica.exe copied by the launcher to the isolated worker bin directory. Backend log: review-backend-connected.log; worker profile daemon.log holds task traces.

Final connected readback: Mika retrieved the worker attachment, checked its contents and attached it directly to Chat without approving or dispatching more work. Browser evidence connected-phone-result.png measures 430x932 CSS pixels with scrollWidth 430. The compact file card and enabled composer are visible. The status endpoint now reports the completed scratch task as waiting for review. Hub validator still fails only on the previously recorded Intelligence remote-claim gap.

## Chat workflow clarity — 2026-09-24

Stage: local-only, owner review pending. Implements the owner's ten Browser annotations without changing Multica navigation, typography or theme.

- Removed the duplicate header Work button on the Chat page. The single status strip expands a bounded inline section (35dvh maximum), preserving conversation and composer on phone.
- Added a composer topic picker with General chat, linked issues and exact pending questions. Discuss selects the same target and focuses the composer. Selecting a topic does not submit a decision or launch work. Captured revisions and request IDs remain unchanged until explicit reselection; unavailable snapshots do not retarget.
- Issue mentions use quiet title links instead of unexplained issue IDs. Full identifier remains in the hover details. Historical scratch text (DEMO-4, acceptance-note.txt, Compact) remains authentic acceptance evidence, not a proposed product control.
- Activity groups actual tool requests into readable categories and counts each request once. Commands, output and internal traces remain under Technical details. Categories describe requested actions, not successful outcomes; this is not yet a business-level progress narrative.
- Loading work no longer briefly claims zero issues; empty state and nonzero status counts are clearer.

Checks: Views TypeScript passed; 34 tests passed across activity summary, live transcript and topic picker; focused Chat ESLint passed; diff whitespace check passed. Manual review checked target immutability, unavailable-data behavior, existing decision paths, locale fallback and preservation of live transcript identity.

Built-in Browser: desktop 1280x720 and actual phone 393x852. Phone document scrollWidth 393, inline Work height 287px, composer visible. Direct topic selection retained a test draft and focused the editor; test draft was cleared without sending. Technical trace collapsed by default. Evidence: C:\Dev\_work\mika-chat-prototype-20260924\work-inline-phone.png and activity-phone.png.

The local frontend/backend had stopped between sessions and were restarted on the same loopback ports. Dedicated Codex-only review worker restored via the existing isolated launcher; no global worker or real issue changed. No push or deployment. Remaining broader delivery work is unchanged: proactive worker result delivery, synchronized organization, versioned approval acceptance and release validation.


## Automatic worker updates and connected handoff — 2026-09-24 05:20 UTC

Stage: local-only code and isolated runtime verification. No push, production data change or deployment.

- Worker starts, terminal run results/stops, and canonical owner questions now arrive as durable work-update entries in linked web conversations. Deterministic source identities prevent duplicate deliveries across retries and concurrent servers. Archived chats and outbound channel sessions do not receive new deliveries; unlinking hides prior issue updates from transcript and unread/preview queries.
- Updates are small text rows with an issue title and timestamp. Longer reports fold under Read worker update. Discuss and answer selects the exact current question directly in the composer; resolved historical questions retain evidence but lose the submission action.
- Delivery uses a creator-private invalidation plus 30-second/focus/reconnect refresh fallback. Reading older messages preserves scroll position and an unsent draft; a New work updates button returns to the latest entries. Existing conversation-open read semantics remain; per-item visible/read cursors are not implemented.
- Each new/resumed Mika turn and chat work CLI receive the same current work state plus the last 30 structured updates. Worker completion means a result is available, not issue acceptance or deployment.
- Real acceptance found an ambiguous continuation brief: the worker tried to resubmit an answer Mika had already saved. The admitted continuation now includes the canonical receipt, exact owner words, and an explicit instruction to apply the recorded choice without resubmitting it. Authorization remains unchanged.

Connected trial: session 01a0d1c4-5b1c-7d57-b5ad-7fa9618df519, two disposable issues. On the corrected second issue 01a0d1d3-8872-7047-9112-09ed47d30f7f, question 96765c56-b995-50f5-b1c4-ab811c30e0c5 appeared automatically. Owner answer through the phone composer produced one applied answer receipt 01a0d1d5-1ddb-7ae1-8cdf-8669ca59096d and one continuation 01a0d1d5-1de6-732c-9a0c-02c3a94a4560. It returned Chosen update label: Awaiting your review. That result arrived without a chat status request. The issue has exactly two worker tasks (initial plus continuation); the two total owner directives are initial dispatch plus this answer, not duplicate answers. Its issue status remains backlog, not accepted or done.

Browser evidence: actual 393x852 phone with document scrollWidth 393, compact start row 96px, pending question row 116px; desktop 1280x720 without horizontal overflow. Earlier first-trial arrival preserved scrollTop 233.3333282470703 and the exact unsent test draft, then exposed New work updates. All test drafts were cleared or intentionally submitted; none left behind. Files in C:\Dev\_work\mika-chat-prototype-20260924: automatic-question-phone.png, automatic-result-phone.png, automatic-result-desktop.png, automatic-delivery-transcript.json and automatic-continuation-receipt.json. Historical first-trial failure remains visible rather than being rewritten as success.

Verification: focused Go handler/service/listener/migration suites passed (TestChatWork, TestOrdinaryTaskQuestion, TestRegisterListeners, TestMigration); covers concurrent delivery, retry, canonical question coalescing, answered/archived/unlinked/stale cases, structured Mika context and exactly-once continuation receipt. 30 Views tests passed for update rows, exact topic selection and scroll behavior; 51 Core tests passed for API parsing and realtime cache behavior. Views/Core TypeScript and focused ESLint passed. Final Go binary built successfully. Query regeneration exposed a pre-existing generated-only FindActiveProjectDeploy query; its original SQL and named nullable project parameter were restored to query source, preserving the generated interface.

Migration 582 adds nullable work_issue_id/work_update fields to chat_message; no new foreign keys or indexes. Applied only to the isolated review DB. Older clients retain readable message content. Downgrade removes the generated work-update entries and new columns; original worker, issue, Inbox and answer records remain authoritative. The reconciler scans durable linked sources every five seconds with bounded batches; production-volume performance has not been established.

Current backend: server/bin/mika-review-server-delivery.exe, loopback 3220; frontend loopback 3221; isolated PostgreSQL 55438; existing dedicated Codex review worker. Pending: shared/server-synced organization and Mika organization tools, versioned approval acceptance, richer milestone/dependency events, per-item read cursors and release qualification. These automatic updates are sourced worker evidence; they do not yet trigger an autonomous Mika synthesis run.

Final Mika readback on the rebuilt backend: selected Review the update wording from its result entry, requested explanation only, and Mika used one read action to quote the saved chosen-label result, note no pending questions and preserve the unaccepted outcome. Browser console had no captured errors. The final composer is empty; desktop viewport override reset.

## Composer and update polish — 2026-09-24

Stage: local-only code and isolated runtime verification; owner visual acceptance pending. Addresses the five latest Browser annotations while retaining the approved Multica shell.

- Chat project controls now use the existing outline FolderKanban icon instead of the folder emoji. Other project surfaces retain their original icon behavior.
- The project-description warning was caused by an unstamped review CLI reporting `dev`, which fails the existing minimum-version check. The current source passed `TestChatProjectContextInjectedIntoRuntimeBrief` for Codex and Claude. Rebuilt the dedicated review CLI with the repository version/commit ldflags and restarted only its idle review daemon. Fresh runtime metadata reports v0.4.34-silver-recovery.14.g0a1a5c8d8-7-g3d6d3acbe-dirty, and Browser no longer shows the warning for selected project context. The launcher rejects unstamped binaries; genuinely unsupported runtimes still warn.
- Activity uses aligned labels/count badges and optional technical details without the redundant explanatory paragraph. Labels describe tool actions without claiming successful outcomes.
- Routine work updates are expandable single-line status rows. Results preview the actual worker text; full original text and timestamps remain in disclosure. Pending questions keep their answer action visible; historical answered questions remain readable without a submission action. No delivery, targeting or approval behavior changed.
- Removed the topic-picker explanation. Disabled only the isolated review's Next development indicator because it overlapped the phone Add button.

Checks: 80 Views tests passed across update rendering, topic selection, composer, scroll behavior and activity classification; Views typecheck and focused ESLint passed. Go project-context injection test and versioned CLI build passed. Diff whitespace check passed. Built-in Browser verified 393x852 phone, 474x544 small window and 1280x720 desktop with matching document scroll widths and no captured console errors. At phone width, routine status rows measured 16px and the compact result measured 42px, excluding shared transcript spacing.

Evidence: C:\Dev\_work\mika-chat-prototype-20260924\polish-project-phone.png, polish-activity-phone.png, polish-topic-phone.png, polish-small-window.png and polish-desktop.png. Test-only project selection was cleared and the original conversation restored; no chat message was sent. Viewport override reset. Selecting a different project currently opens a fresh chat by existing explicit controller policy; this remains a separate workflow question. Broader delivery/organization/release limits above remain unchanged. No push or deployment.

Shared brain: resolved this enrolled worktree to project `multica` on host `windows`; read immutable working/systems-chat-and-issue-behavior.md revision f7c8f5bb1d8d9c0bce8c3da9411c7ca2df2d363823a1ba678cb7267b8367160b. Coverage is incomplete and reference-only; fresh code and local Browser evidence establish this change, not the older reference document.

## Grouped updates, reply context and composer tools — 2026-09-24

Stage: local code and isolated runtime verification; owner visual acceptance pending. No push or deployment. Addresses the owner's update-spam, invisible issue-reply context and richer composer annotations.

- [x] Collapse consecutive routine work events into one disclosure with event/issue counts; retain all original evidence and chronological boundaries between messages. Pending questions remain outside the disclosure, including readable question text when refresh fails; submission still requires current canonical target data.
- [x] Return display-only reply context from each message's saved task target on send and history reads. The user bubble shows `Re: <issue title>` after refresh. General messages do not inherit neighboring targets. Session/workspace scoping remains enforced; missing issue titles use an unavailable label. Titles are current display names, not historical title snapshots. The edit service preserves the original target and still revalidates it.
- [x] Add Draft a plan, Compare options, Make a checklist and Draw a diagram to the composer. These append editable prompts, preserve existing text and never send automatically. They remain available when uploads are disabled. Empty-editor insertion replaces its starter paragraph to avoid a leading blank line. These are request shortcuts, not persistent execution modes or new external integrations.
- [x] Verify code and the built-in Browser. Views/Core typechecks, 1,470 Views tests across Chat/editor, 210 Core API tests, focused Go handler integration tests, Go build/vet and focused ESLint passed. After the final menu-availability adjustment, all 49 composer tests passed again. Diff whitespace check passed. Focused manual review covered chronological grouping and pagination anchoring, live-row identity, stale questions, cache merge behavior, saved target readback, cross-session isolation and preservation on edit. No blocking findings remain in this change.
- [x] Verify actual 393x852 phone and 1280x720 desktop: matching document scroll widths, collapsed four-update burst, saved issue reply label, menu request insertion without extra paragraphs, expansion retaining evidence, empty composer after refresh and no captured console errors. No message was sent during these UI checks. Test draft cleared; viewport override reset.

Evidence: C:\Dev\_work\mika-chat-prototype-20260924\chat-tools-menu-phone.png, grouped-updates-reply-phone.png and grouped-updates-reply-desktop.png. Preview: http://127.0.0.1:3221/mika-local-review/chat?session=01a0d16f-ccac-7d02-97b9-1c7940e0ffdb. Backend rebuilt as server/bin/mika-review-server-chat-tools.exe and restarted only after confirming no active dedicated review tasks. No migration added. Next's dev server restarted itself at its memory threshold during verification; the preview recovered and final checks passed.

Existing renderer supports tables, checklists, Mermaid diagrams, formatted content and attachment previews. Persistent goals, enforced plan mode, voice/sketch tools, connected apps, versioned review surfaces, server-synced organization and autonomous Mika synthesis remain separate work; a menu item alone does not establish those capabilities. This checkpoint does not certify the full business workflow or release readiness. The separate brain integration commit 844e23e27e987224084f3b9d6dc56026c010bdc1 remains for isolated integration review; migrations 583–586 are reserved for it.

Correction to the preceding handoff: shared-brain delivery for 8ee884137 has now succeeded after the service timestamp fix; receipt 784a32ad-3839-4512-9914-efb164bf2113 is retained at C:\Dev\_work\brain-completion-evidence\multica-owner-event-replay.json. It is a task observation, not issue acceptance or deployment certification.


## Completion of the Chat continuation - 2026-09-24

The later isolated combined checkout closes autonomous Mika reports, server-synced organization, exact versioned response replay and ordinary result acceptance. See [chat-continuation-progress.md](chat-continuation-progress.md) for the connected create/question/discuss/answer/result/accept trial, full Chat regression results and current preview. Earlier pending statements above describe their checkpoint, not the final local Chat state. Production deployment remains unperformed.
