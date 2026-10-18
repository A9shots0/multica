# Mika conversations as the work-management surface

Approved design, 2026-09-23. Local implementation is in progress; not deployed. See mika-chat-implementation-progress.md for the verified subset. Owner approved the direction of improving actual Multica while preserving its global navigation, font, colors and styling. This document describes the intended full workflow, not a claim that every interaction is implemented.

## Owner requirements

Use Mika for talking, brainstorming, auditing, dispatching work, receiving progress, discussing evidence, and deciding. Board is optional overview. Inbox notifies and supports quick replies; its actionable items must also be available in Chat. No repeated update chasing, forced issue-detail navigation, giant media, repeated verbose issue chips, or annotation forms that grow the page. Conversations need Recent, Projects, custom groups, pins and search, with mobile parity.

## Evidence boundary

Read local source at base 8080a0e44 plus the existing local review changes. No current worker/provider/deployment claim is made here.

- packages/core/chat/queries.ts: pendingChatTasksOptions aggregates in-flight CHAT tasks. It cannot establish whether linked ISSUE work is running. The current local review Working filter needs this distinction corrected.
- server/internal/handler/chat_issue_links.go and packages/core/types/chat.ts: explicit many-to-many chat/issue links already exist. Extend and repair coverage rather than replace this relationship or infer it from issue mentions.
- packages/core/inbox/owner-inbox-model.ts: owner-facing grouping already distinguishes decisions, stopped work, failures and news. Reuse the underlying actionable records, not independent copies of approval state.
- packages/core/realtime/use-realtime-sync.ts: issue/task/inbox event infrastructure exists. Its existence does not establish durable issue-to-chat delivery or recoverable catch-up.
- The local visual review reads the existing backend through a gateway that blocks writes and WebSockets. End-to-end workflow trials require an isolated backend/database with test agents and scratch work before enabling writes.

## Relationships and organization

An issue has a project and a stable identity. A conversation can manage zero, one or many issues across projects. An agent is assigned work; a conversation is linked to work. These are different relationships.

Create an origin link as part of successful issue creation from Chat, before reporting success. Propagate the origin to delegated child work without flooding the owner with every internal child issue. The Work panel may expand an outcome into children when useful. Merely mentioning or pasting an issue link does not subscribe a conversation. Offer explicit Add to this conversation and Watch here for existing issues. Changes from Board/Inbox/another authorized conversation refresh the same issue everywhere.

Use an origin conversation as the default delivery destination. Additional subscribed conversations may show relevant updates; a plain reference does not. Decisions have one canonical request regardless of how many conversations display them. Unlinking or moving a chat changes organization/subscription only, never cancels the work. Archive must explain that active work continues; unresolved/new needs-you items remain discoverable globally, even for archived chats, with a route to reopen the conversation.

Recent, Projects and My groups are alternate library views, not new boards. Recent uses meaningful conversation activity, never heartbeat churn. Projects group by an explicitly chosen home project, not by the first issue mentioned; cross-project conversations can have a visible multi-project indicator. Custom sections are personal organization independent of actual issue project ownership. Start with one home group per conversation, pins independent of group, and move/rename/archive actions. Preserve the selected view and collapsed groups per user. No-project conversations stay General until deliberately categorized; no inferred silent migration of old data.

## Surface design

Preserve the existing Multica shell. Inside Chat add a compact, persistent work summary below the conversation heading, only when linked work exists. Example: 4 issues / 2 working / 1 needs you / 1 waiting. Counts describe mutually exclusive current issue states; an issue awaiting review is not also counted as actively working. Completed work is secondary and collapsed by default. Mika thinking is shown separately near her response and never stands in for issue progress.

Click the summary or Work control to open a bounded panel within Chat. On desktop it uses a temporary side panel with the conversation still available; on mobile it is a sheet with its own scrolling. Closing it restores the same chat position and draft. Avoid permanently shrinking conversation width or adding another global sidebar. With one issue show its short title and current step directly. With many, show the summary and at most the most urgent next action; do not pin ten large issue cards above the messages.

Each work row shows a short title, state, current verified step, latest meaningful update time, and an optional actual count. Details can show agent, milestones, dependencies, artifacts and activity. Needs You first, unexpected stopped/blocked work next, active/queued next, completed collapsed. Avoid reordering rows on every heartbeat. Provide filters when volume warrants it; no fake global percentage across unrelated issues.

Selecting a row opens its details in Chat and offers Discuss with Mika. That action adds an explicit reply target above the composer, e.g. About: Source account audit. The user can remove it, switch targets, or return to the whole conversation. A response to a particular question carries the canonical issue/request/version identity. If “approve it” could refer to several open decisions, ask one compact disambiguating question. Do not silently choose the most recent or highest-priority issue. Never infer approval from positive brainstorming language.

## Truthful progress

Display owner-understandable states backed by server facts:

- Queued: accepted and waiting, with a reason if known; no running animation.
- Working: an active execution with a fresh signal; show its actual current stage.
- Checking: validation/review is in progress; not complete yet.
- Needs you: a specific actionable request, what is needed and what resumes afterward.
- Waiting: dependency or external wait, name the dependency and who can act.
- Paused: intentionally paused, with who/why when available.
- Stopped: failed/interrupted execution with no active recovery; show recovery route.
- Done: the agreed outcome and supporting evidence are available.
- Status unavailable: connectivity or stale evidence prevents confirmation; retain last verified state and time without pretending it is still working.

Keep execution freshness, meaningful progress time and delivery/readiness separate. A healthy heartbeat is not proof of progress; a stopped response is not proof of finished issue work. No made-up percentages or ETAs. Counts are useful only with known denominators and named scope (e.g. 8 of 12 files checked); changes to scope must be visible. Distinguish preview ready, review passed and deployed when those apply. Done must not mean merely that an agent sent its final message.

Retries update the same issue's current state and retain attempt history. Working elsewhere can continue while one issue is blocked. Dependency waits must not ask the owner to fix something another agent can handle. Stale/lost contact thresholds must come from runtime signal contracts and be tested; do not invent a universal timer in the UI. Recovery actions respect the already authorized scope and report failures rather than loop silently.

## Updates without chat clutter

Use the persistent work summary/panel for the changing present. Use conversation events for meaningful history. Status UI is data-driven and does not depend on Mika composing a message.

Publish durable events for accepted/queued work, real start, substantive milestones, owner questions, meaningful failures/recovery, review ready, revision requested, completion and reopen. Tiny tool steps/logs stay behind activity details. Coalesce routine bursts into one compact update, while owner decisions appear promptly. Every event is visibly tied to its short issue name; do not append full ID/title chips repeatedly.

Current progress rows update in place. Delivered history remains timestamped and attributable; do not silently rewrite what the owner previously read. When an actionable card resolves, show a clear resolved/superseded state rather than deleting its history. No auto-scroll away from someone reading earlier messages or typing. Show a compact New updates control, allow jumping to the next unresolved decision, and preserve the draft. Do not let every heartbeat bump Recents or create unread notifications.

On return after absence, show a concise Since you were here summary from persisted events: completed, still working, stopped, and needs you. Expand for details. Fetch missed changes after refresh/reconnect, not only live WebSocket events. If Mika adds interpretation, it must refer to the retrieved evidence and its freshness; the basic summary should render even if the model provider is unavailable.

## Decisions, media and Inbox

The same decision request appears in Inbox and relevant Chat. Read/unread and unresolved/resolved are distinct: opening a request does not resolve it. Simple decisions can be answered directly. Discuss with Mika focuses that request with its evidence and leaves the request pending during discussion. When the owner gives an unambiguous authorized decision, Mika submits it and waits for a server receipt before saying it was submitted. No second confirmation solely because the response came through Chat.

Submission states are pending, submitting, accepted or failed. Use the request's current revision: a stale preview cannot approve a newer revision. If someone answers in Inbox while Chat is open, the Chat action disables and shows the actual answer. Retries/double clicks/reconnects must not apply a decision twice. A failed submission stays visibly unresolved with the draft and a safe retry. Submission receipt is distinct from execution resuming; show Resuming/Queued until an actual running event arrives.

Compact evidence cards include version and meaningful label. Clicking media opens viewing/annotation only; approvals stay in the conversation/review context. Local annotation drafts stay attached to that artifact version, survive closing the viewer, and are submitted with the intended response. No submission merely from Done marking. Preserve multiple-image notes and show a compact note count; 12+ notes do not expand the transcript into 12 textareas. New artifact versions supersede rather than overwrite the evidence the owner commented on.

## Mobile, attention and controls

Mobile has the same work summary and decisions; the Work sheet should not hide or reset the conversation draft. Compact single-column rows, readable preview controls, keyboard-safe composer, no horizontal page overflow. Return navigation preserves scroll position and focus. Verify 390px, narrower widths and keyboard-open states, not merely desktop shrinkage.

The conversation list shows separate unread-update and needs-you indicators. A chat with a decision waiting stays Needs You even when another linked issue is running. Opening the chat can mark updates read but must not clear unresolved requests. The Working filter uses linked issue execution state, not Mika chat task state. Provide a separate Needs You filter. Inbox remains available for triage, but navigating there is never required to finish work.

Stop generating stops only Mika's current answer. Pause/cancel issue work is an explicitly targeted work action; broad instructions like pause everything must reveal the affected set and respect existing permissions. Archiving a chat is not cancellation. Ordinary questions and brainstorming do not automatically create or redirect issues. Chat commands that change scope/priority/pause state receive a real receipt or clear failure.

## Reliability and authorization

Persist origin links, issue events and decision references server-side, scoped to authorized users/workspaces. Creation and notification delivery must have transaction/outbox semantics or equivalent retryable guarantees; a closed browser cannot stop routing. Model-produced text is not canonical execution state. Reuse existing issue ownership and decisions rather than creating a second work engine in Chat.

Use stable event IDs/sequences, replay cursors and deduplication. Request mutations must be idempotent and reject stale revisions. Refresh on reconnect, merge ordered events without regression, and reconcile counts against the authoritative current state. Access revocation/removal must hide restricted issue details rather than retain private previews indefinitely. Distinguish a human-facing original chat from agent-to-agent execution transcripts.

## Implementation sequence and acceptance

1. Establish the shared server-side conversation work summary, origin/subscription coverage and reliable durable event delivery. Keep existing issue lifecycle semantics. Add truth/freshness mapping and decision identity.
2. Wire summary, bounded Work panel and compact timeline events using real test executions. Correct library filters and preserve existing shell/media improvements.
3. Deliver and discuss decisions inside Chat, with synchronized Inbox resolution, versioned artifacts and submitted/resumed receipts.
4. Add custom grouping/persisted preferences and catch-up polish, then run the full mobile and failure matrix. Organization must not alter execution ownership.

Use an isolated test backend and scratch repository. One test conversation should manage at least three issues simultaneously: an audit progressing, a UI proposal waiting for review, and work waiting on the proposal. Verify each row, aggregate count, explicit reply target, decline/revise/approve, evidence version, actual resume, and final outcomes. Also test a plain brainstorming conversation with zero issues.

Additional acceptance: close Chat and return; lose connectivity and replay; repeated/out-of-order events; worker stop/retry; failure to submit a decision; answer elsewhere first; multiple eligible targets; artifact replaced before approval; cross-project chat; additional subscribed chat; chat archived while work is active; access revoked; many issues and 12+ annotations; screen reader/keyboard and phone layouts. No false running/complete state, duplicate work/decision, lost draft, or forced visit to issue detail. Owner review precedes any deployment.
