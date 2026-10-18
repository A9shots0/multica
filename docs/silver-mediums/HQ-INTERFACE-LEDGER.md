# AI Workforce HQ interface ledger

Updated 2026-10-17 America/New_York.

## Owner contract

The owner uses **Chat** to direct work and **Inbox** to handle things that need
attention. They do not intend to manage the company from the taskboard. The HQ
interface must therefore make those two surfaces sufficient for everyday
oversight while preserving the taskboard and issue pages as deeper records.

The interface must:

- show work performed inside Multica and work reported by outside AI systems;
- distinguish urgent or stopped work from ordinary progress;
- show evidence-backed milestones and next actions, never guessed percentages;
- use plain English and compact layouts on desktop, narrow panels and phones;
- keep routine healthy activity quiet while making owner decisions actionable;
- preserve the difference between an agent reporting completion and Multica
  independently accepting the business result;
- let the owner discuss a selected piece of work through Chat without implying
  that Multica controls an outside ChatGPT, Claude or Codex conversation;
- use deterministic data and controls by default rather than model-generated
  summaries for routine display.

## Design direction to validate in the real app

This is a working hypothesis, not an approved screen or final layout. It must
be implemented and reviewed inside the signed-in Multica shell with real work
data before it can become the product decision. Standalone cards, static HTML,
test fixtures and component-only screenshots are not owner-review evidence.

The product map now gives HQ one concrete job rather than treating every owner
surface as a combined dashboard. See `docs/silver-mediums/HQ-PRODUCT-MAP.md`.

The owner workflow is:

1. **Inbox / Needs you** remains the default exception queue.
2. **Inbox / All work** adds a separate, compact view of open company work.
3. **Chat** remains the place to ask questions, give direction and start work.
4. **HQ replaces Issues** as the visual live-work control center while reusing
   existing issue records, details and controls underneath.
5. **History** is the searchable event timeline and evidence drill-down.
6. Projects, Analytics, Deployments, Agents and Knowledge retain distinct jobs
   defined in the product map instead of duplicating HQ.

This separation follows two relevant external patterns:

- Linear keeps work requiring attention in Inbox and broader progress in Pulse,
  rather than mixing every update into one notification list:
  https://linear.app/docs/inbox and https://linear.app/docs/pulse
- WCAG status-message guidance recommends exposing meaningful state changes
  without needlessly interrupting the current task:
  https://www.w3.org/WAI/WCAG21/Understanding/status-messages.html

Multica differs from Linear because the owner wants fewer destinations. Inbox
therefore keeps only its two accepted modes. A separate History destination is
justified because it is an on-demand investigation tool, not another routine
status feed.

## Prototype question

Can the owner open Inbox and understand, without developer knowledge:

- what needs them now;
- what is actively moving;
- what is stopped or uncertain;
- what changed most recently;
- what happens next; and
- which system or agent reported the state?

The prototype is successful only if those answers remain clear with realistic
dense data at 1920x1080, 1440x900, a narrow desktop panel and a phone-sized
viewport.

## Owner messages carried into this round

- 2026-10-17 22:15 EDT: "Okay awesome, i stopped it. Lets pick up where we
  left off!" Resume the documented HQ replacement for Issues from the protected
  handoff branch. The stopped Bluefin Copilot chat is not authority and none of
  its unreviewed local work is part of this continuation.

- 2026-10-16: "Yes discard those 3 protoypes. Then ye product map is needed!!!
  like we did so much research, we have good plans, we intended on fulyl
  replace issues page and redoing it, where will everything else live?" The
  rejected Project map, Work lanes and Timeline prototypes must be removed.
  New pages are allowed when they have a clear job. Build and approve the
  product map before any further HQ UI. Inbox / All work keeps semantic search.
- 2026-10-16: "worklanes is the same as existing issues". Do not duplicate the
  current Issues board inside Inbox or elsewhere.
- 2026-10-16: "okay.... then continue working. That is your job. your job was
  not to stop at search and show me, it was to get all of that done then show
  me, asking me questions along the way on design." The search slice is not a
  stopping point. Continue the full AI Workforce HQ inside real Multica and
  interrupt the owner only for consequential visual/product choices.

- 2026-10-16: "yes continue" — continue the AI Workforce HQ after the
  searchable-history foundation is proven; the next accepted slice is the real
  Multica Inbox/HQ integration, not another standalone prototype.
- 2026-10-16: one owner-approved LinkHub Codex conversation was imported into
  production as 44 bounded recollection fragments. Exact and semantic search,
  day/project grouping, local source resolution and zero-duplicate replay all
  passed. Brain's RFC3339 compatibility fix is live at
  `a2079c3d08170e4324f294f8fc630894cf36c71e`.

- 2026-10-15: "New goal continue with HQ interface"
- 2026-10-15: "dont do any UI work if it isnt actual using multica UI"
- 2026-10-15: plain-text test images are not practical evidence of proper UI
  work; show only the real Multica interface.
- 2026-10-15: the owner questioned why the review image was light and whether
  All work actually fits the existing Inbox behaviors. Review evidence must use
  Multica's real theme behavior and explain the integration with existing owner
  decisions, recovery actions, quota notices, and memory reviews.
- 2026-10-15: after reviewing the real dark-mode Multica screen and the
  `Needs you` / `All work` separation, the owner accepted the interface: "Ahh i
  like that!! i didnt notice all work. yea this is great goo job." This is the
  owner UI approval for the unchanged candidate.
- Earlier accepted direction: the HQ is the control center coordinating the
  whole AI workforce, including outside ChatGPT/Claude/Codex work.
- Earlier accepted direction: Chat and Inbox are the owner's primary Multica
  surfaces; the taskboard is not a page they expect to manage routinely.
- Earlier rejection: giant cards, developer language, guessed progress and
  non-actionable "needs attention" content are not useful.
- 2026-10-15 21:46 EDT: "I thought you built the brain codex claude inbox
  connection? but sure eya lets audit, make sure youre keeping documentation
  of all we're planning! I love this idea and how you described it to fit in"
- 2026-10-15: the owner rejected decorative agent worlds, TUIs, generic org
  charts, planets and feature-heavy dashboards as the HQ direction. Agor's
  spatial organization remains interesting for a future Issues replacement;
  Event Horizon's timeline remains interesting without its planets.
- 2026-10-15: the owner approved the concept of a searchable company work
  history. A query such as `LinkHub 403` should find related Codex, Claude and
  Multica work, group it by day/project, and let the owner inspect the attempts,
  failures and result through a timeline only when desired.
- 2026-10-15: the owner asked that the separate `Audit HELIX agent runtime`
  task be checked before it builds an ambitious Brain-adjacent system. Useful
  findings should be incorporated into the HQ plan; duplicate runtime or Brain
  work should not be created.
- 2026-10-16: the owner asked that `mksglu/context-mode` be inspected as a
  professional reference for context handling and compared with the Brain,
  Multica and HQ work already underway.

## Outside-work connection audit

The connection was built and merged. It is narrower than full conversation
ingestion, which is intentional and important to preserve in later work.

### What exists now

- `multica issue receipt observe-codex` observes one explicitly linked Codex
  rollout file and one exact Task Control task. It converts completed structured
  task updates into Outcome Receipt v0.2 records without a model call.
- `multica issue receipt observe-stream` provides the same deterministic bridge
  for Claude, ChatGPT, Copilot, EVO and other approved producers that emit the
  structured receipt contract.
- Admitted receipts are stored against the exact workspace, issue, provider and
  external work ID. They appear in issue history and can produce useful Inbox
  state while remaining only a provider-reported claim until Multica accepts the
  business outcome independently.
- Linked outside work can request a bounded, read-only Brain context packet.
  Workspace/project authority comes from the selected Multica issue rather than
  request text. The deployed Brain client supports a query string and immutable
  references; workflow events also use a durable outbox for Brain delivery.

### What does not exist now

- The Codex observer deliberately does not discover sessions or read ordinary
  conversation prose.
- The provider-neutral observer accepts structured receipts; it is not a Claude
  or ChatGPT transcript parser.
- Multica does not yet ingest complete Codex/Claude conversation history into a
  semantic index, group results by day/project, or answer broad searches such as
  `LinkHub 403` from those external transcripts.
- A receipt can prove that meaningful work happened, but it cannot reconstruct
  every exploration, error message or earlier conversation turn.

A live Brain check at 2026-10-15 21:46 EDT confirmed the distinction. Brain was
online and `brain_search("LinkHub 403")` returned general LinkHub reference
documents, but no cited 403 conversation or dated session match. The separate
maintained-company search returned no result and explicitly reported an exact/
term-prefix policy with **no semantic search**. This is useful project knowledge
search, not yet the owner-requested cross-agent conversation memory.

The existing connection is therefore the correct status and Inbox foundation.
The proposed searchable history is a compatible second layer, not a replacement
or duplicate importer.

Audit evidence: `CLI_AND_DAEMON.md`,
`server/cmd/multica/cmd_issue_receipt_codex.go`,
`server/cmd/multica/cmd_issue_receipt_stream.go`,
`server/migrations/621_external_work_receipt.up.sql`,
`server/internal/handler/context_request.go`, `server/internal/brain/client.go`,
`server/internal/handler/workflow_owner_inbox.go`, and
`packages/views/issues/components/external-work-history.tsx` on protected main
`06e309f51c5fe03f43832867b13b93c158b00b5c`.

## Approved HQ information model

Do not add independent dashboards for each borrowed feature. The owner-facing
surfaces have distinct jobs:

1. **Chat** is the command and question surface. Natural-language history search
   can begin here.
2. **Inbox / Needs you** contains only decisions, failures and exceptions that
   require the owner.
3. **Inbox / All work** is the compact current-company view with semantic search.
4. **HQ replaces Issues** as the Agor-inspired spatial view for current work,
   projects, dependencies, urgency and agent ownership. Existing issues remain
   the underlying records and detail controls.
5. **History** is a dedicated on-demand timeline reached globally or from search
   results, a project, work item, agent, deployment or chat. It groups work by
   day and project and shows attempts, milestones, decisions, deployments,
   failures and accepted results.
6. **Projects** own business outcomes and milestones; **Analytics** owns model,
   subscription, cost and efficiency analysis; **Deployments** owns releases and
   recovery; **Agents** owns workforce configuration.
7. **Brain** indexes the durable searchable record and relates external session
   material to Multica projects/issues without treating retrieved text as
   authority or completion evidence.

Emergency work is not a separate dashboard. Urgency, impact and recovery events
are emphasized inside the same current-work and timeline models. Execution
controls such as retrying one failed step belong behind a task/issue detail and
should normally be handled automatically. Timing, repeated loops and usage are
machine-analyzed; the owner sees a concise warning or recommendation rather than
being required to study trace charts.

## Searchable history acceptance shape

The next planning or implementation slice must prove all of the following before
adding UI:

- one bounded importer can read supported local Codex and Claude session records
  without controlling or messaging those sessions;
- secrets, credentials, raw hidden system instructions and unnecessary private
  conversation content are excluded or redacted before durable indexing;
- every indexed fragment retains provider, session, project, timestamp and a
  resolvable source reference;
- semantic search plus exact text search can find a realistic query such as
  `LinkHub 403` and group the matches by day/project;
- results clearly separate conversation recollection, structured work receipts,
  direct evidence and accepted Multica outcomes;
- repeated imports are incremental and deterministic, with no model call needed
  when source data has not changed;
- the timeline is optional drill-down, not another destination the owner must
  monitor.

No UI for this slice is approved yet. First audit the session formats, existing
Brain indexing/search contracts, privacy boundary, storage volume and incremental
update path. Then test retrieval quality on bounded real history before deciding
where the final search control and timeline live.

## HELIX comparison and overlap boundary

The HELIX 1.0.1 source and published benchmark were independently inspected at
commit `b3e1b2bc0715e196f545c6f23230953ef8276943`. HELIX is not a suitable runtime
replacement for Multica or Brain. Its own 50-task comparison reports 77.73%
fewer known provider tokens, but also four fewer verified solves, 79 more model
requests and 37.14% more output tokens. The comparison reuses a known cohort and
a historical baseline, so it is promising evidence rather than a clean causal
trial. HELIX itself documents those limitations.

The transferable patterns are narrower:

- preserve exact source evidence while presenting small typed receipts;
- retrieve originals on demand instead of replaying all old tool output;
- execute only previously authorized mechanical steps deterministically;
- treat stored memory and conversation history as retrieval evidence, never as
  authority, a task queue or proof of completion;
- measure token savings together with task quality, model-call count and failed
  work rather than celebrating token reduction alone.

These patterns reinforce the HQ design already chosen. Brain remains the shared
search and durable-reference layer; Multica remains the work coordinator and
acceptance authority. A second HELIX memory, goal engine, execution runtime or
monitor would duplicate those systems and add another source of truth.

The separate HELIX task has already produced two bounded results:

1. PhoneFarm PR `A9shots0/SMC#899` merged research and a pilot specification.
   Its measured Serena experiment reduced one matched task's input from 34,119
   to 18,692 tokens without changing the answer. This is a context-efficiency
   experiment, not proof of company-wide savings.
2. Brain PR `A9shots0/silver-mediums-brain#30` merged as `4a718343...` and is the
   current Brain production release. It adds an exact status filter so routine
   writer and reviewer automations do not reread completed refresh requests.
   Deployment-host tests and isolated corpus validation passed; Mac Studio's
   filtered client read was verified. The separate Windows writer/reviewer
   measurement was stopped at the owner's request before any model tokens or
   Brain writes, so the production feature is live but its measured savings on
   that host remain unproven. Both schedules were left paused at that checkpoint.

The HQ searchable-history slice must not duplicate either experiment. It should
adopt HELIX's exact-original/compact-receipt principle when designing stored
conversation fragments and consume Brain's protected filtered API without
claiming savings that the stopped Windows measurement did not establish.

## Context Mode comparison and adoption boundary

`mksglu/context-mode` was inspected at commit
`40813fe886798bad07c69a43d2ae00a52b4b934d`. It is a polished local
agent-context utility, not a company Brain or work coordinator. Its strongest
idea is to keep large tool output out of an agent's prompt: process data in a
local sandbox, store exact chunks in SQLite FTS5, return only a small result,
and retrieve exact originals on demand. Hooks also record bounded per-project
session events, deduplicate recent repeats, evict low-priority old events, keep
incremental usage cursors and build a compact resume index after compaction.
Source: https://github.com/mksglu/context-mode and its reproducible benchmark at
https://github.com/mksglu/context-mode/blob/main/BENCHMARK.md.

The transferable patterns are:

- capture typed events at the source when a provider offers trustworthy hooks,
  while retaining the deterministic JSONL importer as the fallback;
- account for bytes avoided and bytes returned by provider, tool, session and
  project, but judge savings alongside task success and retries;
- keep bounded local raw material and inject a compact table of contents with
  exact retrieval references rather than replaying whole histories;
- retain per-project attribution confidence, deduplication, retention limits
  and explicit purge behavior;
- apply large-output routing selectively to logs, builds and fetched documents,
  not to every command or every worker by default.

It must not be adopted wholesale. Its published reduction benchmark compares
fixture byte sizes before and after filtering; it does not measure end-to-end
task quality, retries or successful business outcomes. Session timeline search
is exact/LIKE-based rather than semantic. More importantly, its hook stores raw
user prompts and restores the most recent prompts verbatim; it filters only a
small set of system-message prefixes and does not provide the durable trust and
privacy boundary required for shared company memory. Its Elastic License 2.0
also prevents treating copied code as the basis of a hosted or managed Multica
feature without separate legal review.
License: https://github.com/mksglu/context-mode/blob/main/LICENSE.

Brain therefore remains the private exact-plus-semantic shared retrieval layer,
and Multica remains the coordinator and acceptance authority. Context Mode's
event-capture and accounting designs are reference patterns for a later measured
optimization. They do not replace the existing importer, Brain, receipts,
skills or HQ surfaces.

## Searchable history foundation candidate — 2026-10-15

The non-UI foundation is merged on protected Multica and Brain main. Brain is
live. Multica production activation is pending a privacy correction discovered
during the required real-session proof.

- Multica has a deterministic Codex/Claude JSONL reader that accepts one exact
  file under one explicit root, keeps only visible user/assistant text, chunks
  long messages for local semantic retrieval, and redacts credentials, email,
  phone numbers and workstation usernames before transport.
- Imports are bounded and append-only. A private checkpoint is advanced only
  after Brain accepts the batch. Unchanged files cause no network or embedding
  call; truncation or changes to the source identity or saved boundary fail
  closed.
- Every fragment carries provider, session, exact Multica-to-Brain project,
  time, role, text hash, record position and an opaque source reference. The
  original private path remains only in a mode-0600 local checkpoint and can be
  resolved on demand on the importing machine.
- Brain stores this data in separate recollection/FTS/vector tables rather than
  documents, evidence or claims. All returned hits are explicitly unassured,
  unaccepted and non-authoritative. Multica derives the allowed Brain projects
  from the signed-in workspace instead of accepting arbitrary search scope.
- Exact SQLite FTS remains available without a model. Optional semantic ranking
  uses only a configured loopback Ollama endpoint and caches query embeddings.
  It never sends history to a cloud model.
- A frozen evaluation using local `all-minilm:latest` found the exact
  `LinkHub 403` record and found a differently worded Claude explanation with no
  exact keyword overlap, while excluding an unrelated PhoneFarm record. Results
  were grouped by day/project and remained labelled recollection.
- Brain's full 117-test suite passed. Focused Multica importer, CLI and Brain
  transport tests passed; full Multica CI and protected integration remain.

The first real Codex dry run found an additional privacy boundary that fixtures
had missed: Codex can serialize hidden application context as a user-role text
message. The canceled release candidate was never activated. The importer now
removes reserved goal, browser, app, environment, generated AGENTS and
attachment-wrapper material while preserving the actual request outside those
wrappers. Focused package and CLI tests pass. Re-running the same 8 MiB source
slice produced 36 visible fragments, ignored 1,318 non-conversation records and
sent nothing because the run was dry-only. This corrected code still needs full
protected CI and a replacement sealed release before activation.

This proves the retrieval and trust model, not the final owner experience. No
history UI is approved or implemented in this slice. Automatic multi-file
discovery and scheduling are also intentionally deferred until the exact-file
foundation is protected and its real storage/quality behavior is observed.

## Acceptance boundary

Before production release, the exact candidate needs protected-main integration,
full repository CI, Brain-first installation, Multica release, and a bounded
end-to-end import/search proof on inspected redacted data. This slice has no UI,
so visual preview and owner design approval are not release gates.

## Implemented candidate

The candidate now runs inside the signed-in Multica shell and uses the real
Silver Mediums workspace. Inbox has two compact modes:

- **Needs you** keeps the existing owner decision queue as the default.
- **All work** shows every open issue, sorted by evidence-backed attention and
  urgency. It opens the most important item automatically on desktop and keeps
  the list first on phones.

Each selected item shows only the latest progress, what happens next, relevant
context, and where an outside update came from. Unknown work is explicitly
labelled **No recent update**. An outside worker reporting completion remains
**Reported done** until Multica independently accepts the business result.

The live SILV-98 evidence disproved the previous single-state interpretation:
one branch was waiting, but the primary execution and Claude report were still
working. The candidate therefore shows **Working** plus the secondary note
that another part is waiting, rather than the false **Stopped** label.

Verified locally against the production API at 1920×1080, 1440×900, 1280×720,
and 390×844. A separate reviewer then rechecked the corrected real-data screen
at 1440×900 and 390×844 and passed the candidate: `Needs you` contained no
routine SILV-98 report, `All work` contained all five open tasks, SILV-98 was
`Working`, and unknown work stayed `No recent update`. The remaining acceptance
boundary is owner approval, protected-main integration, deployment, and live
verification.

The candidate inherits Multica's existing system/light/dark theme rather than
introducing its own colors. The first owner-review capture came from a clean
browser whose system preference was light; a follow-up capture verified the
same real-data screen in dark mode. The existing `Needs you` components and
actions are unchanged. `All work` is a separate sibling mode, so ordinary
questions, scoped approvals, retry controls, quota recovery, and development
memory reviews continue through their original Inbox paths.

## 2026-10-16 — searchable outside-work history in Inbox

The current candidate adds one compact, submitted search inside `All work`.
Typing alone makes no request. A submitted query uses the existing signed-in
`POST /api/work-history/search` boundary and groups conversation recollection by
day and project. Current work returns unchanged when the search is cleared.

The detail deliberately says the result is memory rather than proof. It does
not expose the private session id, source reference, local path, text hash,
checkpoint or source offsets. It distinguishes the owner's message from an AI
reply instead of labelling both as the provider's claim. Malformed responses
become a recoverable error rather than a false empty result.

Focused Core and Views tests pass, locale parity passes, both package
typechecks pass, and the production web build succeeds. A real-data preview is
running at `http://localhost:33664/silver-mediums-pilot/inbox?mode=work` against
the existing Multica API. Final interactive acceptance remains blocked only
because this Codex task currently has no built-in Browser surface. Resume when
the owner opens that preview in the in-app Browser; then review actual
`LinkHub 403` results at desktop, narrow and phone widths, run the independent
review, correct any observed defect, and request the owner's visual decision.

## 2026-10-16 — rejected HQ visual direction prototype

The Project map, Work lanes and Timeline prototypes were rejected and removed.
They were not faithful to the approved product map or the researched reference
patterns:

- Work lanes duplicated Multica's existing Issues board.
- Project map was a grid of grouped cards, not the Agor-like spatial replacement
  considered for Issues.
- Timeline was a list of each issue's latest timestamp, not the optional
  Event-Horizon-inspired history of attempts, milestones, decisions, failures,
  deployments and accepted outcomes.
- Putting all three under Inbox / All work incorrectly treated the owner's
  preference for Chat and Inbox as a prohibition on new pages.

No further HQ UI should be implemented until the whole Multica product map
assigns one clear job to Chat, Inbox, the Issues replacement, History and any
new top-level HQ surface. The existing compact All work view and its semantic
search remain valid.

## 2026-10-16 — owner correction after the spatial prototype

These decisions supersede the idea that the next step is merely polishing the
floating-card prototype.

- Loose project and issue cards on an empty canvas do not explain the company.
- The useful part of Agor is its regions, persistent placement, grouping and
  connections that reveal meaning when selected—not its styling.
- HQ must organize itself for an owner who may have dozens of chats and work
  streams open and does not maintain them carefully.
- Chats are orchestrators and the owner's communication surface, not the work
  objects. HQ shows the outcomes, work, agents, systems, incidents and evidence
  those conversations coordinate.
- One outcome can contain several issues and workers, but Multica should not
  spawn several agents by default.
- Work spanning CRM and Phone Farm appears once and connects to both areas.
- The 8 GB Railway watcher, Railway/uptime evidence and Phone Farm support
  reports belong in the same operating picture as evidence and impact lanes.
- Dark mode is the owner-review target. New HQ work uses Multica's shell,
  typography, icons and semantic tokens instead of a second visual system.
- The future control plane will change. The UI must depend on typed objects and
  relationships with replaceable adapters, not today's exact source list.

The accepted interaction and scale definition is now in
`docs/silver-mediums/HQ-UI-DEFINITION.md`. Future UI work starts there instead
of iterating on screenshots of the rejected card canvas.

## 2026-10-16 — stopping instruction

The owner asked that Task Control not be used for this continuation. The durable
continuation is repository documentation and Git history. The requested stopping
slice is the HQ UI definition and handoff; it must not expand into another
backend integration before the current model budget expires.

## 2026-10-17 — A32-only continuation

The owner corrected the host boundary after an earlier preview was started on
the PhoneFarm Mac Studio: the Studio is production PhoneFarm infrastructure and
must not run Multica development services. HQ development and preview now run
only on the 32 GB Mac mini named **A32**. The isolated checkout is
`/Users/a32/Worktrees/multica-hq-ui-continuation-20261016`; the development web
preview uses port `33664` and proxies to the existing Multica API. The PhoneFarm
Studio is not part of this continuation.

The first A32 continuation removed the rejected three-column `Needs you /
Moving / Waiting` board. That structure was another issue board and contradicted
the accepted HQ definition. The replacement candidate now:

- automatically sorts bounded outcome regions by evidence-backed business
  urgency, state and last update;
- keeps status and urgency separate and never shows a guessed percentage;
- summarizes at most four underlying work items per outcome with an explicit
  overflow count;
- exposes active workers, affected systems, incidents and freshness without
  dumping every issue onto the company view;
- opens an in-place outcome panel that explains the next step and connects the
  outcome to its actual work records, workers and operational impact;
- uses the existing Multica shell, components and semantic theme tokens; and
- becomes a hierarchical single-outcome navigator at phone widths rather than
  shrinking a canvas.

Focused model tests, the Views TypeScript check and the full production web
build pass on A32. Interactive acceptance is still open: the A32 preview's old
saved login configuration is not a valid account for the active backend, and no
credential or production-auth change was made merely to obtain a screenshot.
Resume by using a normal authenticated owner session, then run the independent
review scenarios in `HQ-UI-DEFINITION.md` before presenting the candidate as
ready. No fake data, standalone mockup or light-mode capture is acceptable.

## 2026-10-17 — project and topology correction on A32

Independent review rejected the first A32 card-grid candidate before owner
review. The confirmed defects were structural, not cosmetic:

- every root issue was incorrectly promoted to a company outcome;
- the full set of outcomes could still become an endless grid;
- search and project filters hid context instead of emphasizing it;
- the page had no typed shared-work or cross-system relationship source; and
- the A32 checkout did not yet match the exact reviewed Git candidate.

The replacement source now uses active Multica Projects as the primary company
outcomes. Unprojected work is one bounded company-wide group rather than one
outcome per root issue. The company view has a global eight-outcome budget,
with overflow summarized and search bringing matching work into that budget
while dimming unrelated context.

The existing owner-only `system-graph` API is now consumed through a parsed Core
contract. Current company services, incident `affected_refs`, linked issues and
project identities are the only sources allowed to attach work or impact across
outcomes. The static graph supplies typed system nodes and edges only when a
current service or incident explicitly references them; titles are not used to
invent relationships. One incident can therefore appear once as shared impact
while linking to every explicitly affected outcome.

The page is organized into quiet company-area regions, each containing its
actual project outcomes. Selecting an outcome reveals the outcome, its real work
records, current workers, affected systems, known system links, incidents and
next step in one local relationship view. This is the bounded first slice from
`HQ-UI-DEFINITION.md`; freeform placement and user-created containers remain a
later interaction after the automatic operating picture is accepted.

Focused HQ/Core tests, both affected package typechecks, locale parity, Views
lint (no new errors) and the production web build pass on A32. Signed-in visual
acceptance and exact-candidate checkout alignment remain required before this
can be called ready.

## 2026-10-17 — resume and first authenticated A32 review

- Owner direction: "Okay. Please get back to work on a32". Multica HQ work
  resumes only on A32; the PhoneFarm Studio remains out of scope.
- The owner supplied the existing private-preview login details in chat. Those
  credentials are operational input only and are not recorded in repository
  documentation.
- The first authenticated dark-mode capture used real Silver Mediums Pilot
  records. It exposed two confirmed product defects before owner review:
  1. Projects without an explicit live-service link all collapsed into one
     narrow `Company work` column, leaving most of the page empty and reading
     as another stacked list rather than an operating picture.
  2. A project marked `in_progress` was labelled `Moving` even when it had no
     active worker and no recent work evidence. Project intent is not evidence
     of current motion.
- The next candidate separates unprojected shared work from project outcomes,
  lets each region use the available width, labels work state in words, and
  derives `Moving` only from current work evidence. The already accepted
  outcome-detail interaction remains intact.

## 2026-10-18 — authenticated correction and independent acceptance

- The real A32 workspace showed that a two-day-old outside report was being
  presented as `Working now` without a live task or worker. Current movement
  now requires live execution evidence; stale claims remain visible as history.
- Phone Farm now exposes the actual failed queued run, its assigned Application
  Specialist and a plain next step instead of hiding those facts behind `No
  recent update`.
- Ranked regions bring the strongest current evidence forward, while responsive
  auto-fit cards avoid both a narrow endless list and loose cards stranded on a
  mostly empty board.
- Independent dark-mode review passed the corrected desktop and 390px flows,
  real Phone Farm inspection, stale/active distinction, and open-record/return
  behavior. Cross-area and operational-incident scenarios remain untested
  because the live workspace had no typed examples; no relationship was
  inferred from SILV-79's prose.
- The private A32 development preview still receives HTTP 403 on its WebSocket
  upgrade. This means live push refresh is not yet proven by the preview and
  must be checked separately before a release is accepted.

## 2026-10-18 — owner rejection of the static HQ candidate

- The owner fully rejected the authenticated HQ candidate. It must not be
  merged, deployed, polished or used as the visual foundation for the next
  direction.
- The candidate reproduced a Projects-style card summary with a text-heavy
  detail panel. It did not deliver the accepted spatial behavior: fluid
  placement, owner grouping and pinning, persistent organization, or useful
  reveal-on-selection relationships.
- The CRM selection also exposed repository and agent-facing implementation
  instructions as owner-facing copy. That content is not appropriate for the
  HQ surface.
- The earlier review established data truthfulness and responsive rendering,
  but did not establish that HQ performed a unique job beyond Projects. Those
  passes do not constitute product or owner acceptance.
- Production UI implementation is paused. The next design work is throwaway,
  low-fidelity wireframes and interactive mockups derived from the complete
  research and decision history. The owner chooses or combines a direction
  before another production implementation begins.

## 2026-10-18 — decision coverage and interactive wireframes

- The complete product decision was reconstructed in
  `docs/silver-mediums/HQ-DECISION-COVERAGE.md`. It separates Chat, Inbox,
  Projects, HQ, History, Analytics, Agents and Brain; records the owner's
  accepted and rejected directions; maps every researched reference to its
  useful pattern and limit; and defines realistic scale and failure states.
- A reusable automatic `product-decision-integrity` skill was installed so a
  future agent must discover the full decision corpus, resolve later owner
  decisions over older handoffs or reviewer passes, prove a new surface's
  unique job, and prototype defining interaction before production code.
- Three throwaway dark-mode directions now run on A32 only:
  1. a persistent operating canvas with movable regions and work;
  2. an automatically organized semantic focus field;
  3. a spatial operations desk with a temporary event sequence.
- The candidate uses 27 representative work streams across Phone Farm,
  Multica, Brain, CRM and company systems. It includes support, watcher,
  deployment, runtime and outside-work signals. It contains no production
  records and is explicitly labelled as a wireframe.
- Implemented interactions include region movement, cross-region work movement,
  persistent placement, reset, pinning, semantic highlight, selected local
  relationship paths and concise inspection details.
- Production implementation remains paused. These wireframes exist only for
  the owner's choose/reject/combine decision. No direction is accepted yet.
- Independent interaction review passed the final wireframe candidate. It
  verified all 27 work objects, area → outcome → work hierarchy, truthful
  regrouping, contextual search, selected event targeting, separate status and
  urgency, mobile 390px behavior, immediate reset, and a temporary rather than
  permanent event sequence. The pass makes the prototype suitable for owner
  comparison; it does not accept a direction or resume production UI work.

## 2026-10-18 — owner rejection of all card-first wireframes and upstream audit

- The owner rejected all three interactive wireframes. Although their layouts
  differed, they were still variations of the old Issues/Projects experience:
  cards on a board that required reading and organizing records rather than
  understanding company state.
- A separate product evaluator reviewed the raw owner conversation, actual
  Multica surfaces and primary references rather than the implementing agent's
  summary. It found that all three prototypes shared the same work-card
  renderer and hierarchy, so the earlier independent interaction pass tested
  mechanics rather than the HQ's unique job.
- The HQ's unique job is now bounded to an evidence-backed company operating
  model: where effort is concentrated, which outcomes/systems are under
  pressure or neglected, and how one material situation affects the rest of
  the company. Individual work records are drill-down evidence, not the default
  overview.
- Read-only source and A32 API inspection is recorded in
  `docs/silver-mediums/HQ-DATA-READINESS.md`. The codebase has a capable typed
  graph, bounded blast-radius queries, company-service/incident contracts,
  outside-agent receipts and cross-issue consequence storage. The current
  Silver Mediums Pilot workspace, however, has zero cataloged services, zero
  company incidents, zero live workers and zero recorded cross-issue
  consequences.
- Operating topology is the first concept to validate because the typed graph
  already exists. Effort-to-impact flow is deferred because current live data
  does not evidence capacity allocation precisely enough. UI implementation is
  paused until Phone Farm support, Railway watcher, live service/incident,
  work-to-system and consequence evidence satisfy the minimum bridge.
- The replacement gate is comparative and timed. A fresh independent reviewer
  must show that the candidate beats current Projects or classic Issues on
  cross-company comprehension. Any default overview with one card per
  project/outcome/issue is rejected automatically.
