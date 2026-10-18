# Multica AI Workforce HQ UI definition

Updated 2026-10-16 America/New_York.

## The owner's job on this page

HQ replaces the current Issues destination. In one glance, it should answer:

1. What company outcomes are being worked on?
2. Where is effort going?
3. What is moving, waiting, broken or unverified?
4. What depends on what, and what would be affected by a failure?
5. Is anything important being ignored or wasting time?

HQ is not another Inbox, issue board, infrastructure dashboard or decorative
agent office. It is the visual operating picture connecting those systems.

## What appears on the map

The map uses five real object types:

- **Outcome** — the result the company wants, usually backed by a Multica
  project or major milestone. Outcomes are the primary objects.
- **Work** — an issue, incident, deployment or bounded work stream contributing
  to an outcome. Work is revealed inside a selected outcome, not dumped onto
  the company-wide view by default.
- **Worker** — the Multica agent or acknowledged outside provider currently
  responsible for work. Mika chats coordinate work; they are not map cards.
- **System** — a service or operational capability such as Phone Farm, CRM,
  Brain, Multica, Railway or the 8 GB watcher.
- **Evidence** — receipts, support impact, deployments, health observations and
  accepted verification. Evidence appears in details and on meaningful
  connections rather than as hundreds of independent nodes.

Every displayed object must resolve to an authoritative record or a clearly
labelled provider report. Nothing is generated merely to fill the canvas.

## Default view: useful without manual organization

The company-wide view is a stable, automatically arranged map. It remains useful
for an owner who never tidies it.

- Each major outcome is a compact region, sized by current work volume within
  bounded limits. Large projects do not consume the whole screen.
- Outcomes are grouped by company area, but an area is a quiet spatial anchor,
  not a giant card saying “open area.”
- Shared work appears once in a neutral shared position with connections to
  every affected outcome or system.
- Active effort is visible through small worker/activity markers attached to
  the work they own. There is no separate row of cartoon employees.
- Only exceptional connections are visible at rest. Selecting an object reveals
  its local dependency and impact path; a deliberate control can reveal all.
- The system provides the layout. The owner may drag, group and pin important
  objects as an optional aid. Manual placement persists per owner and never
  changes the underlying relationships.

This borrows Agor's useful spatial behaviors—regions, persistent placement,
grouping and reveal-on-selection—without copying its Git workflow or styling.

## Three levels of detail

HQ cannot show every issue at once. It uses progressive disclosure.

### Company level

Shows outcomes, urgent incidents, active effort, stale areas and cross-company
dependencies. This is the default view. A busy company stays readable because
individual issues are summarized inside their outcome.

### Outcome level

Selecting or zooming into an outcome reveals its work streams, current workers,
important dependencies, support impact and next milestone. This answers “what
is happening here?” without leaving HQ.

### Work level

Selecting one item opens Multica's real detail panel with goal, current state,
latest meaningful progress, next step, blockers, evidence, linked support or
deployment activity, responsible agent and History link. Existing issue actions,
comments and permissions remain authoritative.

## State and urgency

Status and urgency are separate. A normal task can be blocked without being an
emergency; an outage can be actively recovering and still be urgent.

Allowed work states: Not started, Queued, Working, Waiting, Needs you, Blocked,
Verifying, Reported done, Accepted done and Unknown.

Urgency uses business impact:

- **Critical** — a core workflow is unavailable, active data is at risk, or
  production recovery is required now.
- **High** — materially delays a company outcome or several people/systems.
- **Normal** — ordinary planned work.
- **Background** — research, maintenance or optional improvement with no current
  impact.

The page never shows guessed completion percentages. Progress is expressed as
evidence-backed milestones.

## Relationships that earn a line

Connections appear only for typed relationships: depends on/blocks, contributes
to outcome, executed by, affects system or customer workflow, verified by,
deployed through, duplicate/same incident, or shared work across areas.

Selecting a connection explains it in ordinary language. Decorative proximity
or AI-inferred similarity does not become a relationship without an explicit
suggestion/confirmation state.

## Cross-area work

A task affecting CRM and Phone Farm is one work object with two typed area or
system relationships. Its status, worker and evidence have one source of truth.
At company level it sits between the relevant outcomes or in a shared-work band.
At outcome level each area can reveal it, but both routes open the same record.

## How the other Multica pages fit

- **Chat** is where the owner tells Mika what they want and asks questions.
- **Inbox — Needs you** contains only decisions and exceptions requiring the
  owner.
- **Inbox — All work** is the fast compact list and semantic search.
- **HQ** is the visual company operating picture and replaces Issues.
- **Projects** define outcomes, milestones and ownership.
- **History** is the optional evidence timeline opened from search or a selected
  object.
- **Deployments**, **Analytics**, **Agents**, **Knowledge** and **Autopilot** are
  focused operational/configuration surfaces, not competing dashboards.

## Source mapping for the first real version

| HQ concept | Current source |
| --- | --- |
| Outcomes and work | Multica projects, issues, milestones and relationships |
| Current worker | Multica working-agent summaries and admitted outside-work receipts |
| Service/system | Company operating-picture services plus the owner-only typed system graph |
| Incident and business impact | Company operating-picture incidents plus linked issue/support receipt |
| Support impact | Restricted Phone Farm support projection; raw support text stays in Phone Farm |
| Runtime/deployment evidence | Railway snapshots, 8 GB watcher, uptime and deployment receipts |
| History | Brain-backed recollection plus structured evidence events |
| Owner action | Existing Inbox requests and issue actions |

Missing or stale source data is shown as unknown. It is never converted to a
healthy state or a zero count.

The typed system graph is topology, authority and evidence coverage, not live
health. HQ attaches its nodes and edges only through explicit current service,
incident or affected-reference identities. It must not match a project to a
system by similar names or present documented topology as proof that a runtime
is healthy.

## Scale rules

- The company view has an object budget; overflow is summarized within its
  outcome rather than rendered as more floating cards.
- Search and filters change emphasis before they hide context.
- Layout is deterministic for the same data unless the owner saved a position.
- New events update badges and details without reshuffling the whole canvas.
- Labels remain complete enough to distinguish work; hover-only meaning is not
  acceptable.
- Accepted work leaves the live map and remains in History.
- On phones or narrow panels, HQ becomes a hierarchical navigator with the same
  objects and relationships. It does not shrink the canvas into illegibility.

## First build slice

This section is paused and must not be implemented from the current live data.
The A32 readiness audit found no cataloged company services, no company
incidents, no live workers and no recorded cross-issue consequences. Building
the page below now would reproduce Projects or invent relationships.

After the minimum evidence bridge in
`docs/silver-mediums/HQ-DATA-READINESS.md` is demonstrated, the next
implementation is one real dark-mode Multica page using real workspace data. It
builds only this vertical slice:

1. Rename the primary Issues destination to HQ while keeping classic views
   available behind a secondary control.
2. Add a deterministic company-level layout of real outcomes, with compact
   state, urgency and active-effort summaries.
3. Select one outcome to reveal its real work and typed local relationships.
4. Open one work item in the existing real detail experience.
5. Project available company incidents/services onto affected work; show
   unknown when no source is connected.
6. Support desktop and a narrow/phone navigator using the same information.

Do not add freeform notes, multiplayer cursors, decorative avatars, global
timeline UI, cost charts or new orchestration behavior in this slice.

## Review scenarios

The first candidate is not ready until an independent reviewer can perform
these tasks in the signed-in dark Multica shell with real data:

1. In under ten seconds, identify the most urgent company outcome and what is
   happening next.
2. Identify which outcomes have active work and which have gone stale.
3. Open Phone Farm modernization and distinguish its outcome, underlying work,
   responsible worker(s), blocker and latest evidence.
4. Follow one cross-area item between CRM and Phone Farm without finding two
   conflicting copies.
5. Find an operational incident from the Railway watcher or Phone Farm support
   and see its impact without exposing raw private support content.
6. Open the existing issue/work detail and return without losing map position.
7. Repeat the core inspection at desktop, narrow panel and phone widths without
   horizontal overflow or illegible mini-cards.

A screenshot or passing component test is not enough. These scenarios are
subordinate to the comparative benchmark in
`docs/silver-mediums/HQ-DATA-READINESS.md`; record pass, fail or untested for
each scenario, and do not let the implementing agent score its own acceptance.
