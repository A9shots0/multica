# Current handoff

## 2026-10-05 01:08 EDT — Unified Chat reply and deliverable release live

- The approved Chat work is live on the 32GB Mac Mini from source
  `d97b28019c60c3521936b3d0649ed24af132b1db` in release package
  `/Users/a32/.silver-mediums/multica/releases/multica-32gb-host-20261005-v29-r2`.
  The package/tooling head is `2e90742650c9aca435e0d10e3af6ebbe300e794b`.
- Production is pinned to backend image
  `sha256:59229cb8c665c75b334b1c5dee0cdacfb9370a181e5b0d9fb2dd561f8dd6cc60`,
  frontend image
  `sha256:b1fa0575c44d441ddd3088d547c6b562f2a55f70c138264d9598d48bc3c59547`,
  and unchanged PostgreSQL image
  `sha256:cf134a767f474095eeba57e0117be8e568e011a63f33fbf252f14c9b760f8e6f`.
  Health reports the exact source revision, 649 migrations are applied, and
  both cutover-mode and live-mode authenticated production verification pass.
- Upgrade rehearsal `v2920261005` restored production data, booted the exact
  candidate with dispatch disabled and zero active work, passed authenticated
  API/WebSocket/history/attachment reads, and produced verified backup
  `20261005T045916Z-b74b0ef5`. Rollback rehearsal `v28rollback1005` separately
  restored and booted the exact v28 release against the same source snapshot.
- Final pre-cutover backup `20261005T050117Z-9ee6fcf2` and post-cutover backup
  `20261005T050441Z-5ad1366d` both passed local, off-box, and private-R2
  verification. The accepted rollback snapshot is retained at
  `/Users/a32/.silver-mediums/multica/recovery/manual/pre-chat-v29-retry-20261005T0503Z`.
- The first cutover invocation restored v28 after noninteractive SSH could not
  find `/usr/local/bin/tailscale`; production returned healthy before the retry.
  The retry used the complete host PATH, passed all gates, and rebound both the
  application and nightly-backup LaunchAgents to v29.
- The Alienware production daemon now runs
  `v0.4.35-silver-chat.1.gd97b28019`, with its v28 binary and scheduled-task
  definition retained at
  `C:\Users\a9sho\.multica\recovery\pre-chat-v29-20261005T0507Z`. Its scheduled
  task is running and the expected local runtimes reconnected.
- Signed-in built-in Browser readback passed at
  `https://32s-mac-mini.tailda9924.ts.net/silver-mediums-pilot/chat` for real
  active and archived conversations. No message or production record was
  created merely for acceptance; a fresh provider-backed multi-answer send
  therefore remains unexercised in this release check.

## 2026-10-04 23:56 EDT — Deliverables read as Chat attachments, not a dashboard (local only)

- The former result block repeated the same set three times: a deliverable
  count, a large tiled gallery plus file row, and a `Review N deliverables`
  footer with a generic `Reply` action. In a conversation, that hierarchy
  looked like an embedded inbox and consumed more space than the message
  justified.
- Results now use one conversational attachment grammar across a single image,
  image sets, full-page prototypes, PDFs, and mixed sets. The newest/primary
  visual is one calm preview; every additional item appears in a compact
  horizontally scrollable attachment rail. A file-only batch uses the same
  rail instead of becoming a separate panel.
- Each image or page opens the full-screen reviewer at that exact artifact;
  PDFs remain direct downloads. The generic written reply footer is gone. The
  exact-artifact reply action remains available as a small preview overlay,
  and marked-note count remains a compact badge rather than another row.
- The component no longer caps the visible attachment inventory at three or
  hides the rest behind `+N`. Large sets remain bounded by one horizontal rail,
  while every item keeps its own stable identity and action.
- Verification: Views typecheck, changed-file ESLint, locale JSON parsing, and
  16 focused Chat tests pass. Built-in Browser verification covered the mixed
  desktop/mobile/full-page/PDF fixture at the normal viewport and at 390x844,
  opened the exact full-page artifact as result 3 of 3, and found zero console
  errors.
- Preview:
  `http://127.0.0.1:13120/local-candidate-samples/chat?session=89ad54d5-1a65-4f68-aeb0-b1ad5e9e976e`.
  Local-only: no file was downloaded, no reply was sent, no provider ran, and
  nothing was pushed, deployed, or written to production.

## 2026-10-04 23:40 EDT — One Chat turn can answer several Mika questions (local only)

- The previous local design remained singular end to end: one stored work
  topic, one reply target, one large composer quote, and one guarded workflow
  response. Two questions therefore required two sends and two separate Mika
  turns even though the questions arrived together.
- Workflow suggestions now compose rather than navigate. Choices stay checked
  inline on their own question; selections from different issues accumulate in
  one compact `N answers` strip above the ordinary composer. There is no focus
  jump, permanent `Replying to Mika` banner, per-question reply button, or
  requirement to send after every choice. The owner may add one optional note
  and send all selected answers in one turn.
- This is not a UI-only batch. The API and task service preserve each answer's
  exact issue, request, owner, and revision as an independently validated frozen
  response. Mika receives the compound set and must submit each response by its
  own deterministic response ID, so simultaneous issues cannot overwrite or
  redirect one another. Duplicate and stale targets fail the send transaction
  without saving a partial message.
- Selected answers persist across a full page refresh and clear only when
  individually removed, explicitly cleared, or accepted by Send. A stale
  question leaves the batch selected and reports that the questions must be
  reviewed before retrying.
- Verification: Core and Views typechecks, changed Views ESLint, 21 focused
  frontend tests, Core work-response tests, Go service tests, the focused
  handler compound/rollback test, daemon prompt test, CLI compilation, and
  `git diff --check` pass. Built-in Browser verification confirmed two
  simultaneously checked issue answers, the compact two-answer composer strip,
  refresh persistence, and zero console errors.
- Preview:
  `http://127.0.0.1:13120/local-candidate-samples/chat?session=89ad54d5-1a65-4f68-aeb0-b1ad5e9e976e`.
  Local-only: the preview uses the existing fixture API; no message or owner
  decision was submitted, no model/provider ran, and nothing was pushed,
  deployed, or written to production.

## 2026-10-04 — Chat decisions use one conventional reply flow (local only)

- The prior exact-reply milestone left the old workflow-question UI intact:
  every question still exposed its own written `Write a reply` control,
  suggested choices looked like disconnected buttons, and unread work added a
  separate `Mark seen` row. That preserved two reply models inside one
  conversation.
- Workflow questions now use the same reply grammar as ordinary Mika messages.
  A small conventional reply icon targets the exact question. Selecting an
  authored choice checks it in place, seeds the single global composer, and
  leaves the text editable so the owner can add context before sending.
  Switching choices replaces only the authored choice and preserves the
  owner's added comment.
- Simultaneous questions are composed as one bounded, divided decision section
  with a restrained leading rail and issue labels only when disambiguation is
  needed. Routine worker activity is one quiet expandable row with an unread
  dot; opening it acknowledges only the visible events. The duplicate
  `Write a reply` and `Mark seen` controls are gone.
- Exact reply context in the composer is a quote strip rather than a topic
  pill. It keeps the durable message/request target and guarded work target
  together; cancel and successful send clear both.
- Verification: Core and Views typechecks pass; 93 focused Views tests and 11
  focused Core tests pass; `git diff --check` and the production-style local candidate build
  pass. Built-in Browser verification confirmed two simultaneous issue
  questions, visible selected state, the editable exact-question composer
  quote, and the absence of the old duplicate controls.
- Preview:
  `http://127.0.0.1:13004/local-candidate-samples/chat?session=89ad54d5-1a65-4f68-aeb0-b1ad5e9e976e`.
  Local-only: no message was sent, no decision was submitted, and nothing was
  pushed, deployed, or written to production.



## 2026-10-04 — Mixed deliverable sets in multi-issue Chat (local only)

- Chat results now represent a bounded deliverable set instead of assuming one
  screenshot. The synthetic conversation sends two independent four-file
  results: checkout has desktop, mobile, a complete HTML page, and a handoff
  PDF; creator profile has desktop, mobile, a tall full-page capture, and a
  separate handoff PDF.
- Each result stays attached to its own issue and message even when two results
  arrive consecutively. Inline Chat shows one fixed-height preview card, at
  most three visual tiles, an overflow count, file rows, and one review action;
  it does not expand into an Inbox or dump every attachment into the transcript.
  Large file-only batches remain individually downloadable inside a bounded,
  scrollable file rail.
- Full-screen review includes only visual/page sources in its carousel. PDFs
  remain explicit download rows. Tall captures use contained sizing, and the
  owner can enter the set from any visible tile.
- Browser verification found and fixed a real attached-page failure. Multica
  had framed its own Next.js preview route inside an opaque-origin sandbox, so
  the app shell's storage access crashed. Commit
  `d4579ce3575b9c691df24aa7f359eb5efaae1523` now detects only canonical
  same-origin attachment preview routes, fetches their HTML, and renders it as
  sandboxed `srcDoc`; look-alike external URLs are rejected and the sandbox is
  not weakened.
- Focused Core tests pass 45/45; focused Views tests pass 20/20; the conversation
  scenario passes 1/1; Core and Views typechecks, changed Views ESLint, and
  `git diff --check` pass. The production-style candidate build is exact to
  final implementation commit is `442c6b71c`; the exact rebuilt candidate is
  `31befa25401a-local-992b84c66930`. All four isolated services are healthy.
- Built-in Browser acceptance confirmed both four-file sets, correct
  issue/message separation, working desktop/mobile/tall-image navigation, the
  secure scrollable HTML page, visible PDF rows, and zero console errors. No
  message or owner decision was submitted, and no push, deployment, production
  write, provider action, or Mac Studio operation occurred.

## 2026-10-04 — Multi-issue design conversation acceptance fixture (local only)

- The local acceptance session now exercises the actual conversation model
  instead of a single isolated result: 13 chronological user/Mika/work entries
  span two independently linked issues, two checkout iterations, one creator
  profile iteration, and two simultaneous unresolved owner decisions.
- Older checkout work collapses behind one quiet `background update`; the
  revised checkout and creator-profile mockups remain in conversational order.
  Each pending decision keeps its own server-authored choices, request target,
  and draft, so switching issues cannot redirect or duplicate the other reply.
- The fixture is synthetic and rerunnable. Stable message/request IDs preserve
  drafts, cleanup is limited to fixture-owned records, and repeated runs retain
  exactly 13 messages, two requests, and two linked issues. It does not start a
  task, bind a runtime, call a model/provider, or submit an owner response.
- The scenario regression test passes 1/1; focused Chat tests pass 19/19;
  changed-file ESLint, Node syntax checks, `git diff --check`, and the
  production-style candidate build pass. The candidate was rebuilt from
  implementation commit `039b8a623a0cd07d2aa3a74c7297ae195c1b0b93`, and all
  four isolated services are healthy. Direct Views typecheck retains only the
  known unrelated `preview-viewer.test.tsx:123` optional fixture error.
- Built-in Browser acceptance confirmed the transcript, collapsed older pass,
  three reviewable mockups, two independent decision targets, draft switching,
  and zero console errors. No message was sent and no push, deployment,
  production write, provider action, or Mac Studio operation occurred.

## 2026-10-04 — Owner questions stay in conversation; concurrent requests route exactly (local only)

- The latest owner feedback identified an interaction-model failure, not a
  styling problem: Chat was directly mounting Inbox's decision form and
  mutation path, so the conversation stopped at a second Inbox.
- Chat now keeps each owner request as an ordinary Mika/work message. Its
  server-authored choices are compact suggested replies; choosing one only
  targets the exact request and seeds the normal composer. The owner can edit
  the words before sending or choose `Write a reply` for a blank response.
- Multiple linked issues and multiple pending questions are independent. Each
  question retains its stable issue/request target, shows the issue title only
  when needed to disambiguate concurrent work, and has its own draft slot.
  Sending one reply can resolve only that request; unanswered siblings remain
  visible and pending. Returning to General chat cannot overwrite a request
  draft, and reselecting a question cannot duplicate its suggested answer.
- The prior direct Chat decision component and its Inbox mutation were removed.
  Mika's prompt and platform workflow reference now describe message-level
  replies rather than a global topic picker or embedded form.
- Focused Core tests pass 10/10 and focused Chat tests pass 12/12. Changed
  Views ESLint and `git diff --check` pass. The production-style candidate
  build for implementation commit `de335aaafc04ed50e72e8cc77408de494ac3d0ba`
  passed and all four isolated services are healthy. The focused daemon prompt
  test also passes. Direct Views typecheck is blocked by the untouched existing
  `preview-viewer.test.tsx` optional fixture error; the broader daemon suite
  retains unrelated environment/baseline failures.
- Built-in Browser acceptance passed on desktop and exact 390×844. The Inbox
  radio form and Continue button are absent; all three authored replies and a
  freeform action remain in the transcript; selecting an answer fills the
  normal composer with visible reply context; clearing/reselecting preserves
  one exact draft; and the browser logged zero console errors. No message,
  owner decision, push, deployment, production write, provider action, or Mac
  Studio operation occurred.

## 2026-10-04 — Chat work requests are actions, not navigation chrome (local only)

- Owner feedback showed an architectural failure rather than a styling problem:
  one pending request was repeated as a full-width work bar, an update row, and
  an implicit composer destination, while none of those surfaces completed the
  decision at the point where it was asked.
- Chat now renders the exact owner decision inline with Mika's question. The
  owner can select one of the server-provided choices, optionally add context,
  and continue without leaving the conversation. `Discuss instead` creates one
  visible, removable reply context; ordinary messages remain General chat.
- The persistent work bar, catch-up panel, global destination picker, automatic
  latest-question routing, and their duplicate labels were removed. A compact
  header manager appears only when more than one issue is linked, and update
  acknowledgement is scoped to the visible group instead of clearing unrelated
  work.
- Browser verification exposed a server contract defect: owner-request
  `details` were serialized as base64, so the client silently substituted
  generic choices. The response now emits normalized request records with raw
  JSON details, and a regression test proves the exact options survive.
- Focused Core tests pass 11/11; focused Chat tests pass 41/41; Core typecheck,
  changed-file ESLint, Go service/handler tests, `git diff --check`, and the
  production web build pass. The full Core suite passes 2,004 tests with one
  unrelated existing `/knowledge` route-coverage failure. Direct Views
  typecheck retains the existing `preview-viewer.test.tsx` fixture error, and
  repository-wide Views lint retains unrelated baseline errors.
- Built-in Browser acceptance passed on desktop and exact 390x844: the exact
  three server choices render, selection enables Continue, optional context
  stays attached to the choice, Discuss shows its exact reply context, and X
  returns to General chat. No decision or message was submitted. No push,
  deployment, production write, provider run, or Mac Studio operation occurred.

## 2026-10-04 — Chat work context is compact and owner-selected (local only)

- Owner feedback rejected the expanded work panel as repetitive workflow
  metadata and the composer topic picker as oversized internal routing UI.
- Investigation found three distinct causes: the picker used the full pending
  question as its trigger label, parent and child rows could both match the
  same request, and `undefined` represented both untouched state and an
  explicit General chat choice.
- The composer now displays the short issue title, treats pending questions as
  children of a non-selectable issue label, and marks exactly one destination.
  An explicit General chat choice is persisted for that exact request; a newly
  arriving request can still auto-target once.
- The work header is one line with a subtle unread dot. Its expanded state now
  contains compact New updates/Clear controls and details on demand, without
  repeating the issue title/status or adding a redundant Discuss button.
- Focused Core tests pass 11/11; focused Views tests pass 7/7; Core and Views
  typechecks, changed Views ESLint, diff checks, and the production web build
  pass. The isolated Docker frontend/gateway are healthy. Built-in Browser
  acceptance passed on desktop and exact 390x844, including persistent General
  chat selection, one exact-question selection, compact expansion, and zero
  console errors.
- The broader suites retain unrelated baseline failures: Core diagnostic route
  coverage for `/knowledge`, incomplete non-English locale parity, and existing
  repository-wide Views lint errors. Core's package-local ESLint executable is
  unavailable. No message was sent and no push, deployment, production write,
  owner decision, or shared-data change occurred.

## 2026-10-04 — Chat results now read as Mika, not a workflow dashboard (local only)

- Owner feedback identified a composition failure: work-update counts led the
  conversation, visual results were nested dashboard panels, questions repeated
  issue/status chrome, and deduplication failed across interrupted event groups.
- Chat now leads with Mika's plain-language result, presents the screenshot as
  one compact media attachment, follows with one natural question and `Reply to
  Mika`, and moves background evidence to a collapsed disclosure at the end.
- Session-level promotion keeps only the newest result and newest copy of one
  owner question visible while preserving older records as background evidence.
  The local acceptance fixture updates its existing synthetic records with
  conversational copy instead of exposing system-acceptance terminology, and
  uses a neutral visual-review sample instead of another three-card dashboard.
- Focused Chat tests pass 14/14, changed-file ESLint and `git diff --check`
  pass, the production-style candidate build passes, and all four isolated
  services are healthy. Built-in Browser acceptance passed on desktop and exact
  390x844, including full-screen `Annotate` and question-to-composer handoff.
- The broader Views run passed 5,099/5,132; 33 failures remain in unrelated
  baseline-wide locale, browser-environment, billing-date, and timeout suites.
  Direct Views typecheck retains an untouched test-fixture type error while the
  production web build's TypeScript gate passes.
- Local-only; no push, deployment, production write, owner decision, provider
  run, or Mac Studio operation occurred.

## 2026-10-04 — Chat visual results are conversational and annotatable (local only)

- Owner feedback exposed an architectural UI defect: Chat rendered visual
  results as duplicated system-status/dashboard chrome, gave each attachment
  most of the conversation, promoted repeated historical results, and opened a
  viewer where annotation was explicitly disabled.
- A visual result now reads as Mika's normal result prose followed by one
  compact attachment with one clear `Review and annotate` action. Only the
  newest visual result per issue stays prominent; older results fold into work
  history.
- Chat now reuses the accepted owner-review viewer. Dragging opens the anchored
  note editor, Enter saves, completed notes collapse to numbered pins, and
  clicking a pin reveals only that note's rectangle/editor. An explicit
  `Add notes to chat` action persists marked evidence before placing the owner
  feedback in the correct issue-scoped Mika draft.
- Focused interaction tests pass 13/13; Views typecheck, changed-file ESLint,
  locale JSON parsing, `git diff --check`, the production-style local candidate
  build, and isolated container health pass. Built-in Browser acceptance passed
  on desktop and exact 390x844 phone size.
- Local-only; no note was submitted, and no push, deployment, production write,
  owner decision, or Mac Studio operation occurred.

## 2026-10-04 — Choice plus optional context and quieter decision actions (local only)

- Owner feedback identified a workflow-model defect rather than a styling
  defect: choosing a proposed next step and adding context were mutually
  exclusive. A selected option can now reveal one compact optional-comment
  field and sends that context through the existing directive path only after
  the owner presses Continue.
- `Talk to Mika` remains visible as the discussion route. Different direction,
  work history, and linked Chat moved under one `More options` menu. Choosing a
  different direction replaces the choice UI with one focused editor instead
  of stacking both modes.
- Focused decision/answer/markup/Mika tests pass 9/9; Views typecheck,
  changed-file ESLint, `git diff --check`, the production-style frontend build,
  and isolated container health pass. Inbox locale coverage includes English,
  Japanese, Korean, and Simplified Chinese. The repository-wide locale test
  still has ten inherited failures in unrelated Issues, Editor, and Chat keys.
- Built-in Browser acceptance passed on desktop and exact 390x844 phone width:
  selection enables Continue without submitting, optional context stays bound
  to the selected choice, secondary actions stay behind one menu, and custom
  direction replaces rather than stacks over the normal decision flow.
- Local-only; no Continue/Send action, push, deployment, production write, or
  Mac Studio operation occurred.

## 2026-10-04 — Explicit owner selection and reliable annotation save (local only)

- Owner feedback exposed two coupled UX failures: recommendation styling was
  presented as selected state, and clicking the recommended row immediately
  submitted the decision. The decision surface now starts unselected, gives
  each row real radio state, and requires a separate Continue action.
- Annotation editing now follows the requested keyboard contract: Enter saves,
  Shift+Enter adds a line, and Tab moves focus. IME composition is preserved.
- Marked evidence now falls back from a failed `createImageBitmap()` decode to
  `Image.decode()`, so SVG review sources can be rendered into the marked PNG
  instead of failing with “The source image could not be decoded.”
- Verification passed: 8/8 focused tests, Views typecheck, changed-file ESLint,
  production frontend build, and healthy local containers. Built-in Browser
  acceptance passed on desktop and exact 390x844 phone width for selection,
  Continue gating, Enter, Shift+Enter, Tab, and responsive layout.
- Wider Inbox tests passed 132/134; the two failures are in an untouched legacy
  panel test that expects combined option text which that panel no longer
  renders. Repository-wide Views lint still has unrelated existing violations.
- Local-only; no Continue action, push, deployment, production write, or Mac
  Studio operation occurred.

## 2026-10-04 — Collapsed annotation pins and contextual editor (local only)

- This supersedes the prior “one active annotation” correction. That revision
  solved the all-rectangles state by hiding every completed annotation, which
  made saved feedback undiscoverable and left note editing in a detached list.
- Root cause: the component treated the rectangle as the annotation's only
  visual representation. It had no collapsed representation and no explicit
  active-versus-collapsed state machine.
- Every saved annotation now remains visible as a numbered circle. Dragging a
  new region opens one compact editor beside it. Save or outside click collapses
  it to its circle; selecting that circle reopens only its rectangle and note.
  Blank new notes are removed on outside click. Multiple saved notes therefore
  mean multiple circles, never a wall of rectangles.
- The editor is bounded inside the image surface and stays fully visible for a
  right-edge annotation at 390px width. The optional annotation list remains a
  separate overview, not the primary editing surface.
- Regression tests pass 5/5. Views/Core type checks, changed-file ESLint,
  `git diff --check`, the production-style frontend build, and isolated
  container health passed. Repository-wide Views lint still reports unrelated
  existing failures outside these files.
- Built-in Browser acceptance used real pointer drags at desktop and exact
  390px width. Two saved notes produced two pins, one active rectangle, and one
  editor; click-out produced two pins and zero rectangles. A blank draft was
  removed on click-out. A saved mobile note reopened from its pin and collapsed
  back to one pin with zero rectangles and zero editors.
- Local-only; no push, deployment, production write, or owner decision
  submission occurred.

## 2026-10-03 — Owner review annotation and decision hierarchy correction (local only)

- Owner feedback rejected the attachment filename-pill wall, floating annotation editor, detached note markers, and oversized lower action form. Investigation traced those failures to three structural causes: generated marked screenshots were presented as peer deliverables, mark geometry and note editing were coupled in one image overlay, and seven decision/action blocks were composed without one owner-facing hierarchy.
- Inline and full-screen review now show compact previous/next navigation with `Result X of Y` instead of filenames. Notes use one dedicated inspector beside the image on wide screens and below it on narrow screens. Saved marks remain visible as translucent outlined regions with numbered badges; the editor no longer covers the work being judged.
- The lower Inbox action area is now one decision surface containing the recommendation, concise alternatives and consequences, custom direction, `Talk to Mika`, history, and linked Chat. No owner action was submitted during qualification.
- Verification passed: Views and Web type checks, 53 focused Inbox/preview tests, changed-file ESLint, `git diff --check`, full Docker candidate build, and healthy local containers. Built-in Browser interaction passed on desktop and an exact 390px CSS viewport, including create/edit/reopen/remove annotation behavior. Repository-wide Views lint still has unrelated pre-existing errors outside these files.
- This work is uncommitted and local-only in `codex/owner-review-system-20261003`; nothing was pushed, deployed, or applied to production.

## 2026-10-03 — Owner review and AI-failure learning system (local candidate only)

- Work is isolated on `codex/owner-review-system-20261003`; nothing in this
  section has been pushed, deployed, or applied to the live 32GB host.
- Chat-created work preserves its durable Chat-to-Issue origin through child
  tasks. Review artifacts now appear inline in Chat and open in the existing
  full-screen visual viewer instead of requiring an Issue-page detour.
- Inbox now contains only owner-actionable decisions and recovery controls.
  A contextual `Talk to Mika` path finds or creates the linked Chat, preserves
  the Issue relationship, and prepares an editable plain-English message
  without sending it automatically.
- Owner markup is append-only evidence: the exact source screenshot,
  normalized annotations/notes, generated marked screenshot, and receipt must
  all persist before the direction is sent. Markup is limited to saved image
  evidence rather than a mutable live iframe.
- Terminal AI task failures, owner-declared rejections, and QA rejections now
  create one idempotent `ai_failure_investigation`. A QA or owner rejection no
  longer immediately repeats the implementation. The investigator must
  reconstruct evidence across company/product context, model behavior,
  prompts/memory/context, tools/runtime/data, human factors, and the
  workflow/review/prevention system. Investigator failure is terminal instead
  of recursively launching another investigation.
- Product & UI completion now requires three different reviewer agents and
  child tasks: Product Designer, Data Visualization Designer, and Creative
  Director. Each must inspect every distinct delivered screenshot, apply its
  full role rubric with task-specific evidence, name professional standards,
  and prove an absolute professional bar rather than merely `better than the
  current page`. The server reads and hashes the real child result; generic or
  fabricated receipts fail closed.
- Local commits: `55098771e`, `286a2f1d9`, `3dd1f61f7`, `3113fc299`,
  `f40d86bd1`, `a34e93457`, and `69c1fa69f`. The complete service,
  design-gate, and handler Go packages pass; 100 focused core/views tests,
  sqlc regeneration, and a complete disposable PostgreSQL migration through
  migration 618 pass. A signed-in Browser fixture proved inline Chat review,
  full-screen visual review, one actionable Inbox decision, and the linked
  unsent Talk-to-Mika draft. The broad daemon package still has unrelated
  Windows/environment-sensitive executable discovery, skill/MCP inventory,
  symlink privilege, and worktree failures; touched daemon prompt tests pass.
  Independent review and any push/deployment remain separate decisions.
- The independent review is now complete on the local candidate. It found and
  corrected seven structural faults before release: Inbox views were bypassing
  the Core mutation boundary; owner markup could name an unrelated Inbox row;
  `Talk to Mika` could reuse an unrelated active conversation; any investigator
  prose could previously be accepted as a completed investigation; an
  investigator's owner question was not durably surfaced; answering that
  question could recursively declare another AI failure; and new review copy
  was missing non-English locale coverage. The corrected implementation uses
  one shared feedback mutation/hook, verifies workspace/recipient/issue
  ownership server-side, creates or reuses only the exact linked Mika chat,
  validates a strict six-domain evidence receipt, writes investigation
  completion plus any owner question atomically, treats the answer as new
  direction rather than another rejection, and includes all affected locale
  keys.
- Final local evidence after those corrections: `internal/service`,
  `internal/designgate`, and `internal/handler` pass; Core typecheck and 140
  Inbox tests pass; Views typecheck and 57 focused Chat/Inbox/markup tests pass;
  changed-file ESLint and `git diff --check` pass. Repository-wide Views lint
  and locale-parity still have unrelated pre-existing failures outside the
  changed namespaces. This remains `local-only`: no push, deployment, live
  migration, provider run, production data change, or mockup rerun occurred.
- Browser acceptance exposed and then closed one further transactional gap:
  marked evidence and the owner directive could succeed while the exact Inbox
  question remained active. The directive request now carries its Inbox item
  identity, and the server validates workspace, recipient, issue, and active
  state before committing the comment, directive, and Inbox closure together.
  A stale or unrelated Inbox identity rolls the whole transaction back.
  Focused handler tests cover both outcomes. The rebuilt isolated candidate
  (`20eb77913c87-local-dc7db172ef24`) passed the complete 390px Browser flow:
  one PNG mark and note were saved, the direction was recorded, the page
  changed to `Nothing needs you`, and that state survived reload. Database
  readback bound the feedback, marked PNG, directive, and archived/read Inbox
  row to SAMPLE-5, with zero task rows and no runtime. The fixture is
  idempotent while active (two initializer runs left exactly one question),
  and desktop readback at 1280px had no horizontal overflow.
- Independent-review correction commit: `b49b9c976`. The documentation-only
  commit that records this final evidence follows it on the same local branch.
- Expanded qualification then ran the complete Core and Views test suites.
  Core passed 2,002/2,003 and Views passed 5,091/5,122; the remaining failures
  are repository-wide route coverage, legacy Inbox-panel assertions, locale
  parity, browser-environment rendering/navigation, date-zone expectations,
  and overloaded-suite timeouts outside this change. The exact Core route test
  and both Inbox-panel failures were rerun in a disposable checkout at the
  pre-change base `3703297df` with the same dependency tree and failed
  identically, proving they are inherited rather than regressions from this
  branch. The comparison checkout was removed afterward. Go vet and both
  production binaries pass with local VCS stamping disabled; all changed-path
  tests remain green.

## 2026-10-03 — Owner Inbox and Codex startup recovery live

- Owner Inbox now translates the observed Codex app-server launch failure into
  plain English, keeps raw diagnostics behind `Technical details`, uses
  `Stopped` instead of `Broke`, removes duplicated decision prose, and gives
  durable memory the owner-facing choice `Save rule`, `Edit first`, or
  `Don't save` with an explicit explanation of what future agents receive.
- The Inbox list and detail layouts were verified in the built-in Browser at a
  430x932 viewport with `scrollWidth == clientWidth`; raw `--sandbox` stderr is
  not visible. Desktop live readback also passed.
- Root cause of the Codex failure was a CLI contract mismatch: Multica passed
  the `exec`-only `--sandbox` flag to `codex app-server`. Source
  `163ed055a9acc9a0f328336dcbfd69ef4855de5c` now translates that setting to
  app-server's supported `-c sandbox_mode=...` configuration while preserving
  normal configuration precedence.
- The exact source is live on the 32GB host in release v28. Backend image
  `sha256:ce90fdba1a92c3596fefd0a005d688774e645b493573b9e5f0a9f51101a51cea`
  and frontend image
  `sha256:f9051cfd54bddbc9eef9947d0060b48e36f8ca0080a3b27a78536b6f4a819ce1`
  passed live production verification. The first cutover safely rolled back
  when frontend startup lagged backend readiness; the accepted retry waited
  for both healthy containers.
- Pre-cutover backup `20261003T212909Z-2e19a119` passed local, off-box, and
  private-R2 verification. Rollback state is retained at
  `/Users/a32/.silver-mediums/multica/recovery/manual/pre-owner-inbox-v28-retry-20261003T2134Z`.
- The Alienware production daemon is also running source `163ed055a...` as
  `v0.4.34-silver-owner-inbox.1.g163ed055a`; its recovery copy is retained at
  `C:\Users\a9sho\.multica\recovery\pre-owner-inbox-v28-20261003T2137Z`.
  A real SILV-83 continuation entered `running` immediately, proving the fixed
  Codex initialization path rather than only a unit-test path.
- SILV-83 is correcting the two independent UI review failures and its strict
  completion receipt. SILV-85 had been misassigned to the general Alienware
  Builder, whose own contract forbids Phone Farm work. That wrong run was
  cancelled and the existing issue was reassigned to the registered `Phone
  Farm Application Specialist` on `SilversacStudio.lan`. The correct runtime
  is offline, so the work is truthfully queued until that Mac Studio daemon
  returns. No Windows Phone Farm inspection or operation occurred.

## 2026-10-02 — Project-faithful UI mockup contract and Chat HTML previews live

- Owner feedback on SILV-81 established a permanent visible-product rule:
  mockups must reuse the inspected project's real shell, tokens, shared
  components, responsive behavior, and current design references instead of
  inventing standalone global chrome. Every changed element needs a recorded
  user/workflow reason and visual work requires an independent viewport review
  before owner delivery.
- Source `50ed771606b98d2fe2cc4b656bca8c334e0d1087` adds
  `docs/silver-mediums/UI-PRODUCT-FIDELITY-CONTRACT.md` and strengthens the
  Product & UI, Builder, and Quality Checker role contracts. The corresponding
  live workspace, CRM project, and three agent instructions were also updated.
- SILV-81 was rerun against CRM source
  `4383458586b23b52754c0fbc76e66c5df3ec55d9`. Built-in Browser review rejected
  the first corrected draft, returned five exact must-fix findings, and then
  verified the revision at 1920x1080, 1440x900, 1024x900, and 390x844. The
  revised draft uses the current plum shell and full navigation, preserves the
  working Switch View surface, keeps tabs and controls readable, exposes phone
  filters, and handles minimum, typical, and dense data without page overflow.
  SILV-81 is in review for owner taste; this remains a mockup, not CRM product
  code.
- Chat HTML attachments now use the authenticated full-page
  `/attachments/{id}/preview` route instead of an iframe-incompatible API URL.
  Focused handler and system-graph tests passed.
- The exact v26 backend image
  `sha256:6915d9f2ad4bc6b9fab720934ef8a76fada2cccc760258b894bc1f1b06ba8ef1`
  is live on the 32GB host and reports source
  `50ed771606b98d2fe2cc4b656bca8c334e0d1087`. Live production verification
  passed. Pre-cutover backup `20261003T033923Z-129a6e1b` passed local,
  off-box, and private-R2 verification. Rollback state is retained at
  `/Users/a32/.silver-mediums/multica/recovery/manual/pre-chat-preview-v26-20261003T0318Z`.

## 2026-10-02 — Correction: delivered mockup now wins over later stopped task state

- The prior v24 owner-facing claim was incomplete. Product & UI had durably
  posted its final SILV-81 comment and `clients-page-mockup.html`, but Chat's
  reconciliation query classified every later-cancelled worker task as
  `stopped` without checking its source-task-bound comment or attachment.
- Source fix `5a95668ff87194684d08708d08711b29d10675a5` makes the durable
  agent-authored delivery authoritative. A cancelled or failed task that
  already posted a reviewable result is delivered to Chat as a result, with
  the actual worker comment and stable attachment links, while still stating
  that the run stopped afterward.
- Regression coverage recreates a cancelled task with a durable comment and
  attached HTML. Focused Chat delivery/follow-up tests, `internal/service`, the
  migration cleanup check, server/migrate builds, and the host profile check
  passed. The broad daemon package retains unrelated Windows fixture failures.
- The exact fix is live in 32GB-host release v25 at
  `/Users/a32/.silver-mediums/multica/releases/multica-32gb-host-20261002-v25`.
  Backend image
  `sha256:9b3ed7fad9d8d674a94250fcfa550bd343e08ceb0ecdf3bd89940c7b049bd66c`
  reports the exact source commit. Migration
  `609_comment_source_task_id_index` is recorded and its index is valid and
  ready. Live production verification passed.
- Pre-cutover backup `20261003T023643Z-7badeeb5` passed local, off-box, and
  private-R2 verification. Rollback state is retained at
  `/Users/a32/.silver-mediums/multica/recovery/manual/pre-chat-delivery-v25-final-20261003T0244Z`;
  v24 remains installed.
- The Alienware Mika daemon had stopped with Windows result `0xC000013A`, so
  the first corrective follow-up remained queued. The existing scheduled task
  was restarted without cancelling work; Mika returned online and completed
  the queued follow-up.
- Built-in Browser acceptance on owner Chat session
  `01a0fed1-83eb-7591-b724-18a136096fb1` now shows the latest update as a
  reviewable Clients-page mockup with a working attachment link and accurate
  implementation/deployment boundaries. Older false failure messages remain
  as retained history; the latest owner-facing state corrects them.

## 2026-10-02 — Mika CRM mockup dispatch correction live; SILV-81 mockup delivered

- SILV-81 proved that issue creation and dispatch worked, but Product & UI was
  bound to a Claude runtime whose OAuth session had expired. The task failed in
  seven seconds with `Failed to authenticate: OAuth session expired and could
  not be refreshed`; daemon `online` status had not proven provider auth.
- Product & UI was rebound through the live UI to the authenticated Codex
  runtime. SILV-81's stale `design-system/` instructions were corrected to use
  the CRM project's bound repository, `docs/design/UI_PLAYBOOK.md`, and
  `docs/design/reference/`.
- Mika had searched a similarly named stale checkout before using the CRM
  project resource, checked only active runs after a successful bind, then
  misreported dispatch as unavailable and attempted a forbidden direct
  handoff. `buildChatPrompt` now requires project-resource source authority and
  treats a successful bind receipt as dispatch evidence while the controller
  returns terminal results.
- Product & UI inspected CRM revision
  `4383458586b23b52754c0fbc76e66c5df3ec55d9`, built and attached the
  54,870-byte `clients-page-mockup.html`, and documented the exact source files
  and preserved workflows. Attachment ID:
  `01a0ff06-8075-7860-aff2-60d1384b170e`.
- The worker obeyed the no-browser boundary. The owner-side built-in Browser
  verified the delivered mockup at 390x844, 900x900, and 1440x1000. Plan and
  Group remain reachable through the visible medium/phone Filters sheet;
  desktop preserves the inline controls. Inline JavaScript also passed an
  independent syntax check.
- The worker posted the attachment and full review comment before a redundant
  final `node --check` tool call stalled. That run was cancelled only after the
  attachment was durably present and independently verified; SILV-81 is In
  Review. Its terminal task status therefore remains `cancelled`, not a false
  `completed` claim.
- Permanent source fix `ce7eb1add2f6121c12bb2d9891a73f7eb16617af`
  passed focused prompt tests and is live in 32GB-host release v24. Backend
  image `sha256:a9c00a0dc93445c774094c3bcd0917498d12e15ce4c63a5a21f8a4adfe5ee240`
  reports the exact source revision with workflow dispatch enabled. Live
  production verification passed and the Tailscale issue page loaded after
  cutover. Rollback state is retained at
  `/Users/a32/.silver-mediums/multica/recovery/manual/pre-mika-dispatch-v24-20261002T235257Z`.

## 2026-10-02 — Correction: full navigation restored

- The earlier owner-mode interpretation was wrong: disliking the issue-board
  workflow did not authorize hiding Multica's other pages.
- The full sidebar is restored. Chat remains the default landing page, and
  Chat/Inbox retain the simplified memory-review workflow, but Issues,
  Projects, Autopilot, Deployments, Agents, Squads, Skills, Runtimes,
  Analytics, Knowledge, and Settings remain visible and usable.
- Corrective source commit:
  `be0940cd6c1401507dea290593e6ef05eacec5e1` on
  `codex/owner-chat-inbox-20261002`.
- Frontend-only hotfix image:
  `multica-owner-frontend:be0940cd6c14`
  (`sha256:1457d321807b97c0cd3fb03b0b2fc7d0455f6708cab82e8bc40c4c3ff29d5a8a`).
  The backend and database were not changed for this correction.
- Built-in Browser acceptance passed on the live Tailscale Chat URL and showed
  the complete navigation. Focused sidebar tests passed 23/23, ESLint passed,
  Views typecheck passed, and the live health endpoint reported the database
  and migrations healthy.
- Rollback state is preserved at
  `/Users/a32/.silver-mediums/multica/recovery/manual/pre-sidebar-restore-20261002T2224Z`.

## 2026-10-02 — Owner mode: Chat + Inbox, durable memory cleanup, live v23

- Working branch: `codex/owner-chat-inbox-20261002`; application source
  `ff38e4d106275ad751e959a63370454e680313e6`; release-package commit
  `927569fd7`.
- Superseded navigation statement: the full navigation is restored by
  `be0940cd6`; only Chat remaining the default destination is still current.
- Workspace entry, sign-in, onboarding, workspace creation/join, and desktop
  defaults now land in Chat.
- Durable development memory is now filtered before proposal creation. A task
  result creates owner attention only when it contains an actual durable
  decision, requirement, fact, constraint, design, or user preference. Generic
  run output, canaries, hashes, findings, next actions, and empty fallback
  summaries remain task evidence and do not become memory reviews.
- Chat-linked work and Inbox show the same plain owner decision: Remember,
  Edit, or Ignore. The technical proposal dialog is no longer the required
  owner surface.
- Live release: `/Users/a32/.silver-mediums/multica/releases/multica-32gb-host-20261002-v23`.
  Backend image `sha256:60fce716f0cb79d456175d13211c171a569e4a1043b5ee72a6f4ee2e5ed0e044`;
  frontend image `sha256:21f807445370b357decba4da8fb18b442b21c1a9ffbdda900a4c24f0d725d11c`.
- Runtime verification passed in live mode. The LaunchAgent points to v23;
  backend, frontend, and PostgreSQL are healthy; the authenticated browser
  reported client version `ff38e4d106275ad751e959a63370454e680313e6`.
- The three old SILV-80 test proposals were rejected through the normal Ignore
  workflow. Their history remains retained, but no memory review remains on the
  issue or in Inbox.
- The Alienware production daemon had exhausted its five restart attempts and
  left Mika offline. It was restarted successfully and its existing scheduled
  task now retries once per minute up to 999 times. A fresh owner Chat session
  returned `OWNER_MODE_LIVE_OK` in 14 seconds.
- Pre-release backup: `20261002T215736Z-728e65e3`. Post-release backup:
  `20261002T220050Z-6bfe2d3b`. Both verified local, off-box, and private R2
  copies. Rollback state is preserved under
  `/Users/a32/.silver-mediums/multica/recovery/manual/pre-owner-mode-20261002T2159Z`
  and the previous v22 release remains installed.
- Focused Go, Core, Views, Web, and Desktop tests/typechecks passed, as did the
  production image builds and host profile check. The repository-wide Go suite
  remains non-green for pre-existing Claude-version evidence drift, legacy
  fake-provider VCS stamping, and old migration cleanup fixtures; none are in
  this release's changed paths.

## 2026-10-02 — Durable development memory proven through owner review

- Remote working branch: `codex/restore-chat-ui-20261001` at `6a90ca201`.
- Multica Platform's live project resource now targets that branch instead of
  `codex/multica-recovery-lead-20260919`.
- Alienware production daemon: scheduled task
  `SilverMediums-Multica-Daemon`, exact CLI
  `v0.4.34-silver-live-verify.1.g4d9567b4c-47-g6a90ca201`, SHA-256
  `109704D9A951AC2D42F16CB2A8419E3729A2835A3EC4EBF3EC09678D2F14352F`.
- `SILV-80` first caught an old-daemon prompt gap, then a nested proposal-schema
  ambiguity. Commit `6a90ca201` fixes the schema instruction and focused prompt
  tests pass.
- Final runtime evidence: task `01a0faf6-b9b2-7007-b471-b0448885708c` received
  durable Intelligence context and produced one pending proposal containing one
  valid workstream memory item. It is unreviewed, undelivered, and has zero
  delivery attempts.
- Browser evidence: the issue is `In Review`; Development memory shows three
  review candidates, and the final candidate opens an editable owner dialog
  with repository evidence plus Approve and Reject controls.
- Do not auto-approve the canary. Owner approval is the intended boundary; only
  after approval should accepted delivery and cross-issue recall be verified.

## 2026-10-01 — Restore approved Chat UI on the 32GB host

- Root cause: the live v19 private-login release was built from current `main`, while the approved Chat/Knowledge UI remained on `codex/brain-owner-review-20260924`. The UI commit was never an ancestor of the live release.
- Integration branch: `codex/restore-chat-ui-20261001`.
- UI/backend source identity: `9768573eb03a3be2b78af11f4ba7ed5ead42edd3`.
- Host-package head: `23038c683`.
- Live images: backend `sha256:d1eba9b88e13af41485dfeda447e8a215fee58620811710721cdde6166c6254e`; frontend `sha256:b8985c15ab4d356dd1444b9d93a5c55cece2062faecd92787db2d3aced5aa431`.
- Live release package: `/Users/a32/.silver-mediums/multica/releases/multica-32gb-host-20261001-v22`.
- LaunchAgent now points to v22 and its last exit was 0.
- Production verification: health commit matches, workflow dispatch preserved as enabled, 637 migrations recorded, all ten Chat migrations present, zero active tasks, and the intentionally cleared issue board remains at zero issues.
- Browser acceptance: the exact owner conversation `01a0d0af-0492-7442-83a2-82fd697b240c` loaded on the Tailscale URL with the approved Chat library, grouping/search controls, conversation view, work-and-updates header, topic selector, messages, media, and Knowledge navigation.
- Pre-cutover rollback snapshot: `/Users/a32/.silver-mediums/multica/recovery/manual/pre-chat-ui-20261001T6pK3Hj`.
- Post-cutover backup: `/Users/a32/.silver-mediums/multica/recovery/production-backups/20261001T221435Z-a8981c75`; off-box and private R2 copies both succeeded.
- The integration branch was subsequently pushed and now points to
  `6a90ca201`; it has not been merged into `main`.

### Remaining boundary

- Reboot recovery is configured through the v22 LaunchAgent and passed a run-at-load invocation, but a physical Mac reboot was not performed because that would interrupt the host beyond the requested UI restoration.

## 2026-10-03 — Product-specific design pack and owner-delivery gate (local only)

- Product & UI work is identified from the configured agent/skill identity, not guessed from title, description, or prompt keywords. Ordinary backend work mentioning UI remains ordinary work.
- On claim, an ordinary repository-backed Product & UI task now requires exactly one approved product design pack from the Intelligence development-context response matching one of the task repositories. Multica persists the exact product ID, canonical repository identity, source revision, schema version, canonical pack SHA-256, approval state, and full pack payload with the task and sends that binding to the daemon.
- Missing, malformed, unapproved, or ambiguous packs fail closed before provider launch. The run prompt requires source validation, real shell/token/component reuse, workflow/data/component/responsive reasons, the four required viewport proofs, the three required data states, all quality checks, and actual independent Product Designer, Data Visualization Designer, and Creative Director reviews.
- Successful completion is server-gated before task result, issue comment, or owner Chat publication. The server requires exactly one strict `design-task-delivery.v1` receipt bound to the persisted pack. It now verifies every claimed review against a real completed child task on the same issue, performed by an agent other than the design agent and carrying a non-empty result. Self-review, nonexistent reviewer tasks, wrong parents/agents, missing receipts, invented pack references, source drift, redrawn shell, missing view/state evidence, failed quality checks, duplicate/unknown/missing review lenses, and non-ready owner delivery return a permanent `422`; the daemon's existing permanent-completion fallback records a visible failed run instead of success.
- Go and Intelligence now use the same canonical JSON hash semantics, including literal `&`, `<`, and `>` characters. The real approved CRM pack produced the same `3972df803f7f3ceb31279b87f315f904ca0fa0c9e4dbbacd321f7d81d5a318ee` identity in both services and bound successfully in Multica's external-pack acceptance test.
- The run prompt embeds the exact receipt template, including reviewer task/agent provenance fields, so a Product & UI task in CRM does not need access to the Intelligence repository to satisfy the gate.
- Focused qualification passed: design-gate package tests, development-memory store tests, Product & UI prompt tests, permanent-completion fallback test, and database-backed claim/persist/self-review-reject/missing-review-reject/accept handler tests. The broader daemon suite was also attempted earlier and remained non-green only on pre-existing host-specific CLI fixtures, symlink permissions, local skill discovery, fake provider binaries, path escaping, and temp-directory locks; changed packages compiled.
- No push, deployment, database migration, live approved-pack registration, runtime restart, production data change, Mac Studio access, or Phone Farm work occurred.

## 2026-10-04 03:58 UTC — Correction: evidence is not a result; annotation is a session (local only)

- The previous local owner-review correction remained structurally wrong. The sample issue contained two real source images plus three generated marked PNGs. Existing owner-review feedback rows identified those PNGs as immutable evidence, but both Inbox layouts mapped every image attachment into the result carousel.
- The editor also showed draft geometry during ordinary review, exited marking after one rectangle, split mouse and touch handlers, completed on mouse leave, offered 14px resize handles, and had no undo.
- The isolated candidate now excludes exact `marked_screenshot_attachment_id` values from reviewable sources without filename heuristics. Marked PNGs remain in issue history. Normal review is clean; annotations appear only during an explicit annotation or inspection session.
- Annotation stays active across marks and note completion, validates blank notes before Finish, includes Undo, exits annotation state before closing on Escape, uses Pointer Events with pointer capture, and uses 24px resize handles. The whole-image amber ring is replaced by one small drag instruction.
- Focused tests passed 4/4; both affected package type checks and changed Views lint passed; the production-style frontend built; local frontend/gateway are healthy. Built-in Browser readback showed `Result 1 of 2` (previously 5), clean full-screen controls, and matching controls at exact 390x844.
- The Browser automation backend timed out while synthesizing a held drag, so an actual Browser drag is not claimed; the pointer lifecycle is covered by the component test. The broader Inbox run passed 129/131 with two unrelated owner-panel copy assertion failures. Core's package lint command still cannot resolve its ESLint binary.
- No push, deployment, production write, or owner decision submission occurred.

## 2026-10-04 13:12 EDT — Direct Mika questions are ordinary Chat replies (local only)

- Owner feedback rejected the `Reply to Mika` button because the question is
  already a Chat message and the button only selected the same question topic
  and focused the composer.
- Investigation found that the exact pending-request target was wired only to
  that button on the full Chat page; the floating Chat window omitted both the
  button and automatic question routing.
- Both Chat hosts now derive the newest displayed question from conversation
  history, validate it against the live pending-request snapshot, and attach
  its exact issue revision/request ID to an otherwise unscoped composer. An
  explicit owner-selected topic is not overwritten, and resolving the newest
  question cannot silently retarget an older question.
- The direct-question button is removed. Mika's question remains ordinary
  dialogue; existing result discussion and manual topic controls remain.
- Core routing tests passed 8/8; focused Chat tests passed 19/19; Core
  typecheck and the production web build passed. Views typecheck retains one
  pre-existing unrelated error in `preview-viewer.test.tsx:123`.
- The loopback Docker frontend was rebuilt and is healthy. Built-in Browser
  verification on desktop and exact 390x844 showed no redundant reply button;
  accessibility readback confirmed the composer topic is the complete Mika
  question.
- Local-only. No message, owner decision, push, deployment, production write,
  or shared-data change occurred.

## 2026-10-04 — Chat work tray composition correction (local only)

- Owner feedback rejected the Chat work area and message-destination menu as
  oversized, bold, visually unstructured, and unable to scale cleanly when a
  conversation links several issues.
- Investigation found the primary structural defect: `ChatWorkPanel` mounted
  the complete `OwnerInboxDetail` decision surface inside Chat. Opening one
  worker update therefore created a nested Inbox page with its question,
  evidence carousel, choices, and actions inside the conversation.
- The nested Inbox surface and its extra Inbox polling were removed. The work
  area is now a bounded, independently scrolling tray: one thin updates row,
  compact 40px issue rows, one selected detail at a time, a restrained leading
  rail, and separated metadata. Five issues remain a dense switcher instead of
  five cards or five expanded decision forms.
- The composer destination menu is now a composed selector with an explanatory
  header, separators, compact icon-led rows, one selected circle, the short
  issue title, and a single truncated question preview.
- Focused regression tests passed 7/7; changed-file ESLint and `git diff
  --check` passed; the production web build and isolated candidate image build
  passed; all four local containers are healthy. Built-in Browser verification
  passed at desktop and exact 390x844 with no horizontal overflow and zero
  console errors.
- Views typecheck still reports the pre-existing unrelated
  `preview-viewer.test.tsx:123` optional `Mark` fixture error. Local preview:
  `http://127.0.0.1:13004/local-candidate-samples/chat?session=89ad54d5-1a65-4f68-aeb0-b1ad5e9e976e`.
- No message, owner decision, push, deployment, production write, provider run,
  or shared-data change occurred. Owner visual acceptance is not established.

## 2026-10-04 — Chat work controls become overlays, not content (local only)

- Correction to the preceding Chat tray work: the tray remained the wrong
  product model. Even with compact rows, expanding it still made routing and
  status metadata occupy the conversation as if it were primary Chat content.
- The work control is now a single 36px status row. Opening it produces an
  anchored overlay instead of increasing Chat height. Each linked issue is one
  32px title/state/unlink row; the checked timestamp, work-detail expansion,
  development-memory block, activity history, and empty detail chrome are no
  longer rendered in Chat.
- The composer topic picker is now only General chat plus compact routable
  destinations. Its tutorial header, helper copy, section label, duplicated
  issue/question preview, and selected-circle chrome were removed. One pending
  request maps to its issue row; multiple pending requests remain individually
  selectable.
- Focused tests passed 7/7 and include five linked issues. Views typecheck,
  changed-file ESLint, diff checks, production web build, candidate image build,
  and all four local container health checks passed. Built-in Browser verification
  passed at desktop and exact 390x844; both controls float without moving the
  conversation and browser console errors were zero.
- Local-only. No message, owner decision, push, deployment, production write,
  provider run, or shared-data change occurred. Owner visual acceptance is not
  established.

## 2026-10-04 — Needs-you rows now enter the reply flow (local only)

- Owner feedback found that the compact work overlay reported `Needs you` but
  its issue row was static. The adjacent chain-shaped control was an immediate
  unlink action with no visible explanation.
- The issue row is now the primary action. For one pending question it captures
  that exact request target, closes the overlay, and invokes Chat's existing
  composer-focus path. If an issue has several pending questions, it expands
  them for explicit selection instead of silently choosing the first.
- Unlink moved behind a conventional `...` menu whose item is explicitly
  labelled `Unlink issue N`; it cannot be confused with the reply action.
- Focused interaction tests passed 4/4, including exact-request routing,
  explicit multi-question selection, and unlink isolation. Changed-file ESLint,
  diff checks, and the production web/candidate build passed. Built-in Browser
  readback confirmed that the row closes the overlay and selects the matching
  composer topic, while the overflow menu exposes the labelled unlink action.
- Views typecheck still reports the pre-existing unrelated optional-`Mark`
  fixture error at `preview-viewer.test.tsx:123`. The synthetic preview Mika has
  no runtime, so the disabled composer cannot accept keyboard focus; topic
  selection itself is visible and verified.
- Local-only. No message, owner decision, unlink, push, deployment, production
  write, provider run, or shared-data change occurred. Owner acceptance is not
  established.

## 2026-10-04 22:08 EDT — Exact conversational and artifact replies reach Mika (local only)

- The first implementation milestone for the unified Chat event stream is
  complete in the isolated worktree. A member can now reply to an ordinary
  user/Mika turn or to one exact review artifact without opening an Inbox-like
  destination surface.
- The target is a durable `chat_message.id` plus optional block/artifact ID.
  The server validates same-session visibility and sub-target membership in the
  send transaction, then stores an immutable context snapshot beside the new
  turn. Missing, cross-chat, hidden, or stale targets fail closed with `409` and
  do not partially create a message or task.
- The claimed agent task carries that frozen snapshot into Mika's prompt ahead
  of the current user message. It is explicitly labelled untrusted quoted
  context, cannot authorize work, and tells the model to answer in relation to
  the exact selected event instead of a newer nearby message. Editing a reply
  revalidates and preserves the same target.
- Chat presents one compact composer quote and a small persisted reference on
  the sent turn. Workflow questions retain their existing single `Write a
  reply` action; the visual pass removed a duplicate generic reply action.
- Commits: `5ba2dab27`, `6423d660f`, `e3ec178bb`, `1b0c42869`,
  `1b86ac45b`, `5b7148973` on `codex/owner-review-system-20261003`.
- Checks: Core and Views typechecks; 190 focused frontend tests; Go service and
  handler packages; changed-file ESLint; diff checks; production web build;
  candidate image build; four healthy local containers. Built-in Browser
  interaction verified ordinary-message and artifact selection replace the
  old topic chip with one exact quote. The full suites retain unrelated
  baseline failures in diagnostics routing, locale parity, Mermaid sizing,
  billing dates, environment-sensitive UI tests, and pre-existing literal
  string lint.
- Preview: `http://127.0.0.1:13004/local-candidate-samples/chat?session=89ad54d5-1a65-4f68-aeb0-b1ad5e9e976e`.
  Local-only: no message was sent, no model/provider ran, and nothing was
  pushed, deployed, or written to production. Owner acceptance is not yet
  established; grouped thread context and reply-aware retrieval remain later
  planned milestones.
