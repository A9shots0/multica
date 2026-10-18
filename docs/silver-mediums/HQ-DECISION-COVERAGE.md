# AI Workforce HQ decision coverage

Status: superseded as an implementation gate by
`docs/silver-mediums/HQ-DATA-READINESS.md`. This remains the owner-decision and
research record, not approval for another card-based wireframe.

This document prevents a convenient implementation slice from replacing the
owner's complete product decision. It is the coverage source for the next HQ
wireframes and their independent review.

## The owner's real task

The owner is coordinating many simultaneous company work streams through
Multica, ChatGPT, Codex, Claude, local machines, hosted services, and watchers.
They work conversationally, often keep many chats pinned, and do not want to
maintain a project-management system by hand.

The HQ must answer, in roughly ten seconds:

1. What important work is happening across the company?
2. What is moving, waiting, blocked, stale, or needs me?
3. What business outcome does each piece of work serve?
4. What else will be affected if this changes or fails?
5. Where should I focus, without reading every chat or internal engineering note?

The HQ fails if it becomes another list of projects, another issue table, a pile
of cards on empty space, or a diagram the owner must organize before it is useful.

## Responsibilities of surrounding Multica surfaces

| Surface | The question it answers | What it must not become |
| --- | --- | --- |
| Chat | What do I want Mika to understand, coordinate, or explain? | The full operational map. |
| Inbox — Needs You | What genuine decision or exception requires me now? | A feed of routine agent progress. |
| Inbox — All Work | What current work exists, and can I find it semantically? | The company relationship map. |
| Projects | What business outcomes, milestones, resources, and ownership exist? | A live view of every worker and dependency. |
| HQ (replaces Issues) | How is live work connected to company outcomes, systems, incidents, and each other right now? | Projects with larger cards, or Inbox spread across a canvas. |
| History | What happened, when, and in what sequence? | The default live-work surface. |
| Analytics | What are usage, cost, capacity, and model-efficiency trends? | The operational work map. |
| Agents | How is the workforce configured and governed? | A list of current company work. |
| Brain | What durable evidence and accepted knowledge can be retrieved? | The live controller or unquestioned source of current status. |

Task Control and Dot are temporary bridges. They are not the long-term owner
interface once Multica is trusted.

## Accepted operating model

- Mika/chat sessions are orchestrators and owner-facing communicators. A chat is
  not automatically a work object on the HQ map.
- A business outcome may have several issues, workers, systems, incidents, and
  verification steps.
- A work item exists once even when it affects several projects or systems.
- Multiple agents are not spawned by default. Parallel workers are justified by
  independent work, risk, or elapsed-time benefit.
- The map must incorporate work and evidence from Multica, outside ChatGPT/Codex
  and Claude sessions, the 8 GB Railway watcher, Phone Farm support, deployments,
  runtime health, and Brain receipts where adapters exist.
- Outside-agent reports are claims until Multica verifies the applicable result.
- Brain evidence may be durable context, but current source, runtime, test, and
  provider evidence can be more authoritative for a live state.
- No guessed completion percentages. Progress is expressed through evidence-backed
  milestones, state changes, and verified outcomes.

## Information model shown by the HQ

### Primary hierarchy

`Company area -> business outcome -> work item -> worker / run / evidence`

Incidents, systems, releases, support signals, and dependencies may cross that
hierarchy. They must not be duplicated merely to appear under two areas.

### Status language

The visible state vocabulary is:

- Not started
- Queued
- Working
- Waiting
- Needs you
- Blocked
- Verifying
- Reported done
- Accepted done
- Unknown

The UI should emphasize exceptions and change, not decorate every healthy item.
Staleness is a factual age/evidence condition, not a synonym for failure.

### Relationship rules

- Only typed, evidenced relations become visible connectors.
- A selected object reveals its local dependency and impact path.
- Company overview does not draw every possible line.
- Example relation types include: advances, depends on, blocks, affects, runs on,
  reported by, verifies, deployed as, and raised from support.
- Layout changes never change operational truth.

## Accepted interaction requirements

- The default layout must be useful without manual organization.
- The owner may drag, group, pin, and resize meaningful regions or objects.
- Owner placement persists.
- Automatic layout must respect pinned/manual placement and accommodate new work.
- Selection reveals connections and concise operational meaning in context.
- Semantic search narrows or highlights relevant work without replacing the map.
- Progressive disclosure keeps the company view readable: company areas and
  outcomes first, work and evidence on focus.
- Dense state must be designed deliberately; the normal case is dozens of active
  streams, not seven cards.
- A narrow/mobile view becomes a hierarchical navigator or focused local map. It
  must not shrink an infinite canvas into illegibility.
- Owner review is dark mode only.

## Owner preferences and rejections

### Accepted or strongly preferred

- Agor's fluid spatial board, regions/containers, movement, grouping, persistent
  organization, and connections revealed through selection.
- Event Horizon's event timeline as a useful history pattern, without its planet
  metaphor.
- Semantic retrieval of prior work, such as finding conversations about a
  specific LinkHub 403 error and the days they happened.
- A dedicated usage/cost/efficiency view in Analytics.
- Concise owner-facing language; technical details remain available on demand.
- A control center that ties together work performed inside and outside Multica.

### Explicitly rejected

- Pixel agents or a pixel office as the primary operating interface.
- Generic AI-generated decorative art, random 3D scenes, planets, org charts, or
  TUI aesthetics.
- Work lanes that reproduce the old Issues page.
- A project-card grid, including cards floating on an otherwise empty board.
- A timeline that is only a sorted list of latest timestamps.
- A text-heavy detail panel that exposes internal repo or agent instructions.
- The static HQ candidate based on grouped project cards and a permanent details
  panel.
- Sparse mock data or fake evidence presented as if it proves the product works.

## External references and their limits

| Reference | Useful pattern | Why it fits | Limit for Multica |
| --- | --- | --- | --- |
| [Agor](https://github.com/preset-io/agor) | Spatial board, movable work, zones, sessions, local relationships | Gives a messy, conversational operator a persistent visual workspace | Agor is a collaboration canvas, not a whole-company operational truth model; its styling and data assumptions cannot be copied wholesale. |
| [Event Horizon](https://github.com/HeytalePazguato/event-horizon) | Cross-agent event timeline | Makes long-running activity and handoffs inspectable | Timeline belongs in History or a temporary focus tool; planets and terminal styling were rejected. |
| [Trajectory](https://github.com/datadog-labs/trajectory) | Reading Codex/Claude session history and searching work over time | Supports outside-work history and semantic retrieval | Session logs are evidence sources, not the company data model or primary HQ layout. |
| [Flightdeck](https://github.com/flightdeckhq/flightdeck) | Model/runtime usage awareness | Helps spend subsidized subscriptions and API capacity deliberately | Cost and utilization belong mainly in Analytics; do not crowd the HQ overview. |
| [Backstage catalog graph](https://backstage.io/docs/features/software-catalog/creating-the-catalog-graph/) | Typed entities and relations; graph as a mental model | Supports truthful cross-project/system dependencies | The catalog graph is not a live activity UI and is not a visual template. |
| [Linear Inbox](https://linear.app/docs/inbox) | Separating notifications and required attention from planning | Supports the Needs You/All Work boundary | It does not solve spatial company comprehension. |
| [Linear Projects](https://linear.app/docs/projects) | Outcome-oriented planning and milestones | Clarifies the role of Projects | It is not the live workforce HQ. |

Other researched directions such as Agent Deck, aquarium/character scenes, TUIs,
and org-chart-style views did not fit the owner's workflow or taste.

## What remains useful from the rejected implementation

The visual composition is not a foundation. The following non-visual work may be
reused only after verification:

- truthful status normalization;
- typed relationship adapters;
- outside-work report ingestion;
- real data loading and authentication integration;
- existing Multica shell/theme primitives;
- semantic search behavior;
- tests that assert data provenance and prohibit fake production records.

Card layout, grouping, labels, details-panel copy, hierarchy, and interaction
model are rejected and must not constrain the wireframes.

## Scale and failure states every direction must show

Each wireframe must demonstrate all of these with representative data:

1. At least 25 simultaneous work items across several company areas.
2. Several issues/workers advancing one outcome.
3. One work item affecting more than one project or system without duplication.
4. Healthy active work that stays visually quiet.
5. A true owner decision.
6. A blocker and its downstream impact.
7. A stale or unknown report distinguished from a failure.
8. A support signal tied to Phone Farm work.
9. A watcher/runtime/deployment signal tied to relevant work.
10. Outside work reported but not yet accepted.
11. Selection that exposes a local path without turning the whole map into a web.
12. New unplaced work entering an already owner-organized workspace.

## Historical wireframe directions — rejected

These hypotheses were implemented and rejected on 2026-10-18. They shared the
same project/work-card hierarchy and therefore did not represent materially
different information models. Do not iterate on them.

### Direction A — spatial operating canvas

Company areas are movable/resizable regions with auto-placed outcomes and work.
The owner can drag, group, and pin. Selection reveals only the local operational
path. This is closest to Agor's useful interaction model.

Falsifier: if the default view becomes empty-space card storage or requires
regular owner tidying, reject it.

### Direction B — semantic focus field

The default is a dense company-level field showing pressure, movement, and areas
that need attention. Semantic zoom or selection transitions into an outcome's
local work graph. Manual pinning preserves a few owner landmarks, but automatic
layout does most organization.

Falsifier: if it hides the identity of work or feels like a decorative heat map,
reject it.

### Direction C — spatial operations desk

Persistent regions hold live outcomes while a temporary contextual strip shows
recent events, exceptions, and the selected impact path. It combines a spatial
workspace with Event Horizon's useful time signal without making History the HQ.

Falsifier: if the strip becomes another inbox/timeline or the regions reduce to
fixed columns, reject it.

## Replacement decision gate

The production UI stays paused. The next candidate must satisfy the verified
data-readiness gate, fixed comparative benchmark and automatic rejection rules
in `docs/silver-mediums/HQ-DATA-READINESS.md`. The implementing agent cannot
author its own acceptance result.
