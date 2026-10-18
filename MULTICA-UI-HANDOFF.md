# Multica UI handoff: use Chat as the product standard

Updated 2026-09-25 UTC. Read this before starting another Multica page.
This is the owner's local UI continuation, not the Intelligence repository.
Read `AGENTS.md` and `CLAUDE.md` for architecture and implementation rules.

## Deployment authorized; release still blocked - 2026-09-25

The latest owner instruction is: "Give me a message to send to next agent. Is
all of your changes live? if not they need to be". This authorizes releasing
the prepared changes and supersedes older no-deployment statements below.
Do not ask for deployment permission again. It does not turn local evidence
into production evidence or remove recovery requirements. Read
`docs/silver-mediums/chat-release-status-20260925.md` first for the fresh runtime
snapshot, missing recovery configuration, key-custody gap and release work.
Fresh backup and isolated restore subsequently passed, and the owner confirmed
the key is on their phone; see the follow-up in the release note. The release
tool still rejects the unrecorded running-image/source baseline. No push,
production restart or migration occurred. `NEXT-AGENT-MESSAGE.md` is ready to send.

## Latest correction: diagram review UX rejected

After the wrap-up, the owner rejected the whole diagram annotation experience:
"this entire thing is just such bad UX. Ugly on mobile too." This supersedes
the earlier positive annotation feedback below. The broader Chat visual approval
does not approve this viewer. Do not copy the old stacked toolbars/comment form
as the standard for another page, or represent the diagram viewer as accepted.

The local revision simplifies Chat's viewer to one icon toolbar, defaults to
Move, hides source/download/zoom actions under options, opens only one selected
pin comment at a time, and collapses the general question behind its button.
Ordinary wheel scroll pans rather than unexpectedly zooming; deliberate
Ctrl/pinch, menu and keyboard zoom remain. Saved marks and comments are retained.
An incomplete pin opens for editing when Add to chat is pressed. Erase and Undo
remain available. This revision requires owner visual review before declaring
the diagram UX settled; the source screenshot is in `ui-reference/` below.

Owner then rejected the cramped comment editor/clipped action in the first
simplification too. Latest correction uses a full-width textarea with a labeled
Done action above it; other footer actions are hidden during editing. The
collapsed footer has two constrained columns, with general questions inside
the comments menu. A container query switches narrow toolbars to a labeled tool
picker below 340px. Test the actual narrow panel and editing state, not just a
393px viewport with fields closed. Both rejection screenshots are retained.

## Owner direction and acceptance

The owner wants the actual Multica app improved locally, then reviewed before
deployment. They explicitly rejected replacing its layout with a separate
prototype. Preserve its navigation, font, colors, density, component language
and functionality. This is an extension of Multica, not a new design system.

The owner said Chat received a "100% on visual pass" except for an oversized
diagram. We bounded that preview and added drawing/comment annotations. Latest
feedback: "Not bad tho i like it" with a missing eraser, followed by a request
to wrap up and document the standard so another agent can tackle other pages.
The eraser is the final small correction in this handoff. This approval is for
the reviewed local visuals, not production readiness or every backend edge case.

User request: preserve continuity and conserve remaining usage. Do not begin a
new large page redesign in this wrap-up. No new agent has been started. The
later explicit deployment instruction above supersedes the earlier local-only
scope; the visual acceptance itself was narrower than operational completion.

## Non-negotiable product standard

| Principle | Apply it on every page | Rejected pattern |
| --- | --- | --- |
| Preserve Multica | Keep workspace switcher, Search, New Issue, Inbox, Chat, Work, AI Team, Analytics, Knowledge, Settings and existing destinations. Reuse semantic tokens, role-based font sizes and Lucide icons. | Replacing the shell, new font/color palette, emoji folder icons, generic dashboard redesign. |
| Compact and readable | Support a narrow desktop panel as well as a real phone viewport. Use bounded details, concise rows, clear hierarchy and deliberate overflow behavior. | Wrapping filter labels, clipped controls, tiny desktop layouts masquerading as phone views, giant inline diagrams. |
| Calm default view | Show current meaning and the next useful action; put technical evidence/history behind disclosure. | Repeated automatic status cards, duplicate Work buttons, raw commands/JSON/path dumps, unnecessary explanatory boilerplate. |
| Keep context | Preserve draft, selection, scroll and return position. Let the user inspect or discuss work with few steps. | A new full-screen page for routine phone work details; navigating Work -> another page -> chat just to ask a question. |
| Visible targeting | Show what issue/request a message concerns before and after sending. General chat remains available. | Context known only to AI; silently retargeting a stale reply; treating selecting a topic as approval. |
| Real capabilities | UI actions must use actual persisted contracts that Mika can read or act on with the same scoped identity. Demonstrate the round trip. | Decorative buttons, pretend progress, UI-only state Mika cannot see, unsupported capabilities disguised as working. |
| Intentional composer | Attachments/context/formats are explicit selections. A response-format chip does not alter typed words; successful send consumes that selection and history retains its label. | Menu actions that paste canned prompts into the user's message. |
| Honest outcomes | Distinguish not started, queued, working, needs input, result ready, stopped, unknown and accepted Done from evidence. | Finished worker = approved result, healthy API = finished product, missing data = zero. |
| Useful rich content | Tables, lists, plans, files and diagrams support the conversation. Compact previews expand on demand. Annotation sends marked image plus questions. | Giant photo-like graph occupying the whole conversation or decorative embeds with no useful interaction. |
| Reversible interaction | Save drafts, preserve input on error, give retries/Undo, respect permission and runtime gates. | Losing a note after error, duplicate submissions, unannounced action or empty success claims. |

The owner is not a developer. Use ordinary language for activity and statuses;
keep technical details available on demand. Do not hide a real capability error
just to remove an ugly warning. Resolve it or state a concise actionable limit.

## What Chat already demonstrates

- Recent, Projects and personal custom groups; server-backed organization and
  Mika `chat organize` controls. Earlier notes saying groups are device-only
  are superseded by `docs/silver-mediums/chat-continuation-progress.md`.
- Multiple issues linked to one conversation, independent states, compact
  shared work summary and bounded details/history. Automatic Mika reports carry
  reviewable results; avoid adding another stream of redundant inline cards.
- Explicit issue/question/version targets with visible sent-message context.
  Discussing a choice does not answer it. Exact answers retire only the matching
  Inbox request. Stale/changed review evidence must be recaptured explicitly.
- Request changes and accept the exact eligible result; acceptance does not
  start another run. Idempotency and unrelated issue/request isolation matter.
- Persistent drafts/topics, retry behavior, reconnect recovery and floating
  Chat continuity. Project context follows actual worker capability support.
- Plan, Comparison, Checklist and Diagram as structured per-turn selections.
  All four produced real worker replies in the local owner-flow rehearsal.
- Chat Mermaid preview capped at 240px with app font/background, full viewer,
  drawing, numbered comments, Undo, saved annotation drafts and marked PNG
  attachment through the regular upload pipeline. Erase removes one whole
  stroke or pin; Undo restores it. It is not a pixel/partial-stroke eraser.

Do not infer every desired Codex-like feature is implemented. Examples such as
plugins, goals, arbitrary document annotation or voice are not promises in this
handoff. Scope the next feature against actual backend/provider capabilities.

## Where to resume safely

| Item | Location / boundary |
| --- | --- |
| Current isolated source | `C:/Dev/_work/multica-brain-owner-review-20260924` |
| Branch | `codex/brain-owner-review-20260924`; run fresh `git status --short --branch` and `git log -5 --oneline` |
| Local Chat | `http://127.0.0.1:3221/mika-local-review/chat?session=01a0d661-e5c8-74a8-809c-b461300d57f3` (Chat workflow rehearsal) |
| Web | 3221, source above; launcher `C:/Dev/_work/mika-chat-prototype-20260924/start-catch-up-web.ps1`, log `catch-up-web.log` |
| API | 3220; last verified revision `77431292149dc57e3f50220770da81f7324d7f19`; recheck `/health` and `/readyz` before relying on it |
| Dedicated review worker | Last verified `4bfc2a26d501b334bd9e8321d5f488158f40420e`; profile `mika-chat-review-20260924`; do not replace the global daemon |
| Review database | Dedicated local PostgreSQL 55438, container `multica-mika-review-pg`; do not run destructive automated tests here |
| Automated-test database | Separate local PostgreSQL 55440; existing test helpers/docs specify configuration |
| Launcher/evidence directory | `C:/Dev/_work/mika-chat-prototype-20260924/`; contains hidden launchers and owner-flow evidence; inspect current script/path/process before restarting |
| Owner files to preserve | `apps/web/next-env.d.ts`, `apps/web/tsconfig.json`, untracked `apps/web/AGENTS.md`, `apps/web/CLAUDE.md` |
| Shared knowledge | `C:/Dev/Silver-Mediums-Knowledge/projects/multica/`; brain project ID `multica` |

Different API/worker/web revisions are intentional; later changes affected
worker guidance and web rendering without API changes. Never stamp the source
HEAD as the loaded backend revision. These are dated snapshots, not ongoing health.

If the preview is missing: inspect the named ports/process paths and log first.
Never kill all Node/Multica processes, restart shared Docker, or reset a database.
Only start a missing verified local component using its existing launcher and
hidden window. Do not blindly invoke an old launcher that points to an obsolete
binary. Development rebuilds can temporarily blank the browser or delay sends;
check whether a send completed before retrying it. Unrelated port 3000 and other
agents are not part of this preview. Do not change system zoom or take over the
desktop. Use the built-in in-app Browser exclusively; no standalone Playwright,
Chrome, desktop automation or alternate browser fallback without user direction.

For another agent: inspect occupancy. Continue this worktree only with exclusive
ownership, or create an isolated task worktree from the full current local branch
tip. Starting from an older main/upstream tip can discard the accepted Chat work.
Do not clean/reset/stash owner changes. Do not assume these local commits are
available on another machine; remote publication needs separate authorization.

## Source map and evidence to read

| Need | Read |
| --- | --- |
| Final real worker rehearsal and acceptance receipts | `docs/silver-mediums/chat-owner-flow-acceptance.md` |
| Diagram annotation behavior, test proof and limits | `docs/silver-mediums/chat-diagram-annotations.md` |
| Response format contract | `docs/silver-mediums/chat-response-format-progress.md`; `packages/core/chat/response-format.ts`; handler/service `chat_response_format.go` |
| Recovery/history/stale decisions | `docs/silver-mediums/chat-gap-closure.md` |
| Later admission correction superseding its old blocker | `docs/silver-mediums/chat-admission-progress.md` |
| Groups, delivery and earlier continuation | `docs/silver-mediums/chat-continuation-progress.md` |
| Chat composition | `packages/views/chat/chat-page.tsx`, `components/chat-window.tsx`, `chat-input.tsx`, `chat-topic-picker.tsx` |
| Compact work UI | `components/chat-work-panel.tsx`, `chat-work-details.tsx`, `chat-work-history.tsx`, `chat-work-update-group.tsx` under `packages/views/chat/` |
| Work and grouping contracts | `packages/core/chat/{work,library,store}.ts`; handler/service `chat_work*.go`, `chat_library.go` |
| Diagrams | `packages/views/editor/{mermaid-diagram,mermaid-viewer,diagram-review}.tsx`; `utils/diagram-review-export.ts`; Core `chat/diagram-review.ts` |
| Design tokens and components | `packages/ui/styles/tokens.css`, existing `packages/ui/components/ui/`, Chat's current rendered behavior |

Chronology matters: old progress files include superseded blockers, paths and
"remaining" lists. Read their later entries and this handoff before restarting
old work. Original implementation plan is historical intent, not current truth.
User reference screenshots are saved under `docs/silver-mediums/ui-reference/`.
They are feedback evidence, not perfect desired mockups: the annotation screenshot
specifically demonstrates the missing eraser and the shell screenshot preserves
the original navigation/style expectation.

## Known limits to carry forward

1. Physical finger/stylus and real phone keyboard behavior are not certified.
   Built-in Browser verified phone CSS geometry; drawing used pointer input.
2. Production deployment, volume/concurrency/load qualification, broad provider
   and upload matrix, and optional Redis tests are separate unperformed lanes.
3. Views typecheck and affected tests pass. Full Core typecheck has a previously
   recorded unrelated Inbox fixture mismatch; do not claim every repo gate green.
4. Windows shell crash root cause remains unresolved. Hidden local launch reduces
   tool-session coupling; it is not an autostart service or reliability guarantee.
5. Mika can still phrase things badly, omit details or repeat sibling status.
   A sentence-count error was corrected through the real review/revision flow.
   Prompt instructions do not guarantee model output.
6. Mika's CLI cannot rename Chat conversations; UI renaming works. Do not invent
   a tool. Current diagram annotations do not cover arbitrary images/PDFs.
7. Brain source refresh generation 17 is pending (11 sources, zero submissions
   at last read); historic hub runtime data is not current deployment evidence.

## Suggested next page and working method

Recommended next slice is **Inbox**, then **Issues**, then **Projects**: those
complete the same question -> decision -> work -> result loop already exercised
in Chat. This order is a recommendation, not an owner-approved page redesign.
Keep other pages (Agents/Squads/Skills/Runtimes/Autopilot/Deployments/Analytics/
Knowledge/Settings) discoverable; address them individually after core workflow.

For each page:

1. Read this handoff and its local instructions; inspect the current page and
   existing API/CLI/Mika capabilities before proposing changes.
2. State the owner's job in one sentence and identify the actual workflow gaps.
   Capture current desktop, narrow-panel and phone behavior as evidence.
3. Implement one coherent workflow in the actual app, preserving the shell.
   Use existing authoritative records; trace UI -> API -> worker -> observed
   result -> UI. Add/update Mika capabilities where that workflow requires them.
4. Cover loading, empty, unavailable, failed, stale and completed states. Preserve
   unsent input, access boundaries and exact target/version on every action.
5. Run relevant regression checks and inspect Browser at a measured ~393x852
   CSS phone viewport, a narrow desktop panel and full desktop. Check long names,
   keyboard/focus, overflow, expand/return, retries and reconnect as applicable.
   Reset temporary viewport/network overrides. Do not confuse screenshot scale
   with the real CSS viewport or claim simulated layout is physical-phone QA.
   Use the Browser viewport capability and reset it afterwards. Ad hoc CDP
   metrics produced scaled/clipped captures on this host. Also test ~280px
   narrow panels with comment/form editing open; a closed-field fit check is
   insufficient and was explicitly rejected by the owner.
6. Exercise a safe local end-to-end example autonomously; the owner previously
   asked agents to supply test scenarios while they are busy. Keep fixtures
   clearly labeled and avoid showing internal acceptance-test jargon as product
   content. Never use real business work/production as a disposable fixture.
7. Finish the authorized slice, record evidence and unresolved limits, commit
   only owned files locally, and update hub + brain. Present a working local
   review with a short explanation, not another abstract prototype.

Start prompt for the next agent:

> Continue Multica from C:/Dev/_work/multica-brain-owner-review-20260924.
> Read MULTICA-UI-HANDOFF.md and AGENTS.md first, inspect Git and the local app,
> and preserve the accepted Chat design and behavior. The owner wants other
> pages to meet those standards. Begin by assessing Inbox against its real
> workflow and Mika capabilities; the suggested order is Inbox, Issues, Projects.
> Work locally, use the built-in Browser, preserve owner changes, and do not
> push/deploy. Use the handoff's evidence and limits instead of starting over.
