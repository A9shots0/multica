# Current review

The functional local review is now http://127.0.0.1:3221/mika-local-review/chat, backed by an isolated database and Go server. See mika-chat-implementation-progress.md for current evidence and remaining work. The 3218 review below is historical and does not include the functional backend changes.

# Mika Chat local review

Stage: local-only. Recorded 2026-09-23 (America/New_York).

The owner rejected the standalone redesign and requested changes in the actual Multica app, preserving all navigation, fonts, colors and style. This branch is `codex/mika-chat-local-review`, based on `8080a0e44`, in `C:\Dev\multica-mika-chat-local-review`. The source checkout and its dirty auth.go were not changed.

## Review

Open http://127.0.0.1:3218/silver-mediums-pilot/chat . This is the actual Next.js app using the existing local backend through a read-only gateway at 3219. GET/HEAD reads pass to 8080; writes and WebSockets are blocked. Sending messages, pinning/renaming, linking issues and submitting decisions are therefore disabled for this review. No push, deployment, migration, or real issue mutation was performed. The rejected standalone preview on 3217 was stopped.

Changed: compact Chat list with search, All/Unread/Working filters and Project/Recent grouping; existing pins and row actions retained; compact attachment thumbnails and adjacent image paragraphs; quiet issue ID links; a Work menu using explicit issue associations; click-to-open Inbox images; media-only fullscreen viewer; one contextual annotation editor, saved pins and bounded notes list. Original global shell and semantic theme remain in use. Two unrelated unused imports/variables in subscriptions-tab were removed to clear baseline TypeScript errors, without changing behavior.

## Verification

- Views TypeScript check passed after final edits.
- 41 tests passed across conversation-library, preview-viewer and editor attachment suites. Covers 12 saved notes, editing one note, retaining focus while typing, drawing/removing a note, and no decision buttons in the viewer.
- Built-in Browser: actual owner conversation on desktop; three screenshots measured at the same Y coordinate in a compact row. Quiet SILV-74 links verified. Existing global navigation and Inter/theme tokens retained.
- Built-in Browser at 390 x 844: Chat library width and document scrollWidth both 390; conversation header and composer visible. Viewport reset after testing.
- Built-in Browser: SILV-55 existing preview opened by clicking media; drawing a local draft opened one comment editor; Done dismissed the editor and left one pin; fullscreen viewer had no approval controls. No answer submitted.
- git diff --check passed. Existing Inbox i18n lint debt is compared against HEAD in C:\Dev\_work\mika-chat-prototype-20260924\lint-comparison.json; do not claim a clean repository-wide lint gate.

## Remaining work

This is an initial real-app visual review, not completion of the chat-managed work lifecycle. Auto-associating newly created issues, proactive issue progress and blockers in Chat, synchronized Inbox/Chat decisions, and Mika submitting the owner's agreed response remain to implement. The existing SILV-74 conversation has zero explicit issue links and old chats have no project IDs; the UI does not fabricate those associations. Backend write paths were not tested. Production build/release checks and owner design acceptance are still pending.

Dev runner: from apps/web set REMOTE_API_URL=http://127.0.0.1:3219, NEXT_TELEMETRY_DISABLED=1, NODE_OPTIONS=--max-old-space-size=4096, then node node_modules/next/dist/bin/next dev --webpack --hostname 127.0.0.1 --port 3218. Gateway: node C:\Dev\_work\mika-chat-prototype-20260924\review-gateway.mjs. Next dev can briefly restart when its memory threshold is reached. Generated apps/web AGENTS.md, CLAUDE.md and next-env type references came from Next dev, not product changes.

## Final check notes

- Focused lint comparison: zero newly introduced diagnostics in the five compared components; existing Inbox diagnostics remain (11 viewer, 17 detail, 19 legacy card).
- Hub validator ran and failed on the unrelated intelligence registry entry: intelligence.remote.main has zero active remote claims instead of one. This change did not modify that registry/claim.
- Final rendered Chat screenshot: C:\Dev\_work\mika-chat-prototype-20260924\actual-multica-chat-review.png.
- Local review runners restarted after a session interruption; Chat loaded again in the built-in Browser and was marked as the deliverable.

## Next behavior design
Owner requested a complete multi-issue Chat design. See mika-chat-workflow-design.md in this directory. Proposed, not implemented. Next local work should prove real issue progress, decision targeting and Inbox/Chat synchronization with an isolated test backend before deployment.
