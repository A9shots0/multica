# Multica AI Workforce HQ product map

Updated 2026-10-16 America/New_York.

## The simple model

Multica should have **one visual control center**, not several dashboards that
repeat the same work.

The existing **Issues** destination will be replaced by **HQ**. HQ is where the
owner sees the company’s live work, how projects relate, who or what is working,
what is urgent, and where work is stuck. Issues remain the underlying work
records, but the owner should not have to manage the company through an issue
tracker.

The other pages support HQ and each have one distinct job.

## Where everything lives

| Surface | The question it answers | What lives there | What does not live there |
| --- | --- | --- | --- |
| **Chat** | “What do I want Multica to do or explain?” | Start work, give direction, ask company-wide questions, and search work history conversationally. | A permanent status dashboard or a long feed to monitor. |
| **Inbox — Needs you** | “What genuinely requires me?” | Decisions, business or UI choices, failures, approvals, and exceptions that automation cannot safely resolve. | Healthy routine updates or every agent action. |
| **Inbox — All work** | “What is moving right now?” | A compact list of current work, latest meaningful update, status, and semantic search. | A spatial map, full issue board, or detailed event timeline. |
| **HQ** *(replaces Issues)* | “How is the whole AI workforce operating?” | A spatial live-work map: projects as regions, work as connected objects, dependencies, urgency, ownership, agent activity, and blockers. Opening an object reveals its real issue details and controls. | A second copy of Inbox or decorative fake employees. |
| **Projects** | “What company outcome are we trying to reach?” | Outcomes, milestones, resources, important decisions, ownership, and rollups. A project opens HQ filtered to its work and History filtered to its evidence. | Minute-by-minute agent activity. |
| **History** | “What actually happened?” | A searchable event timeline across Multica, Codex, Claude, ChatGPT, deployments, and accepted outcomes. It shows attempts, failures, retries, decisions, tool milestones, time, and usage when available. | Another page the owner must constantly watch. |
| **Analytics** | “Are we using models, subscriptions, time, and money well?” | Usage, cost, capacity, provider/model comparisons, repeated loops, failed work, and efficiency trends. | Live task management. |
| **Autopilot** | “What repeats automatically?” | Scheduled and recurring workflows, their next run, health, and exceptions. | One-off project work. |
| **Deployments** | “What is going live and can it recover?” | Release queue, environments, deployment evidence, failures, rollback, and production health. | Product planning or general agent supervision. |
| **Agents** | “Who can do the work?” | Agent roles, instructions, permissions, connected accounts, and current availability. | A duplicate list of every task an agent is doing. |
| **Knowledge** | “What does the company know?” | Brain-backed evidence, policies, durable decisions, source material, and retrieval. | Current task status or unreviewed memory presented as fact. |
| **Squads, Skills, Runtimes, Settings** | “How is the workforce configured?” | Administrative setup and reusable operating rules. These should be grouped and visually quieter than daily work. | Daily owner oversight. |

## How the pages connect

1. The owner tells **Chat** the outcome.
2. Multica creates or attaches the real work records and shows them spatially in
   **HQ**.
3. Routine work stays quiet. Only genuine decisions or failures appear in
   **Inbox — Needs you**.
4. **Inbox — All work** gives a fast list and semantic search without requiring
   the owner to enter HQ.
5. Selecting work in HQ opens its real detail. Selecting its history opens the
   same item in **History**, not a second invented record.
6. **Projects** group the work around business outcomes. **Analytics** measures
   efficiency. **Deployments** handles releases. **Knowledge** preserves accepted
   learning and evidence.

## HQ: the replacement for Issues

HQ is the product’s main visual face. It should use the useful part of Agor’s
spatial model without copying its Git-branch workflow or turning Multica into a
game.

- Projects or major outcomes form stable regions.
- Work items are compact objects placed inside or between those regions.
- Lines show real dependencies only; decoration never creates fake meaning.
- Urgent incidents, blocked work, and owner decisions are visually distinct.
- Agents are visible through the work they own or are running, not as a separate
  cartoon office.
- Zooming out answers “where is the company’s effort going?”
- Zooming in answers “what is happening, what is next, and what evidence exists?”
- Existing issue detail, comments, actions, and history remain the real backing
  data rather than being rebuilt as a second system.

The current board, list, table, swimlane, and Gantt views are not the new HQ.
They may remain as secondary “classic views” while the spatial view proves
itself, but they should not compete with HQ in the primary navigation.

The interaction and scale contract is defined in
`docs/silver-mediums/HQ-UI-DEFINITION.md`. The company-wide view shows outcomes
rather than every issue; selection reveals local work and typed relationships;
shared work appears once; and the default layout is useful without the owner
organizing it manually.

## History: the real timeline

History is not a row of recent timestamps. It is an evidence-backed event
timeline inspired by Event Horizon’s useful timeline and by Trajectory’s agent
session observability.

It can be opened globally or filtered from a project, work item, agent, search
result, deployment, or chat answer. A query such as `LinkHub 403` should find
related work semantically, group it by day and project, and show the sequence of
attempts and results. The owner can inspect it when needed without monitoring it
all day.

## Navigation recommendation

The daily navigation should be short:

1. **Chat**
2. **Inbox**
3. **HQ** *(replaces Issues)*
4. **Projects**
5. **History**

Operational destinations remain available but secondary:

- Autopilot
- Deployments
- Agents
- Analytics
- Knowledge

Configuration should be grouped below those:

- Squads
- Skills
- Runtimes
- Settings

This is an information-architecture plan. It does not authorize another visual
implementation before the owner chooses the first HQ visual direction in the
real Multica shell.

## Why this structure

- [Agor](https://github.com/preset-io/agor) demonstrates that a spatial canvas
  and named regions can make concurrent work and relationships easier to scan.
  Multica should borrow the spatial organization, not Agor’s branch-centric
  runtime.
- [Linear Inbox](https://linear.app/docs/inbox) separates items requiring
  attention from broader work. That supports keeping Inbox focused instead of
  turning it into the whole product.
- [Linear Projects](https://linear.app/docs/projects) separates outcome-level
  planning from issue-level execution, which fits Multica’s Projects/HQ split.
- [Event Horizon](https://github.com/HeytalePazguato/event-horizon) supplies the
  useful multi-agent timeline pattern; its planets and theme are not a fit.
- [Trajectory](https://github.com/datadog-labs/trajectory) shows how session
  timelines can expose failures, loops, tool thrashing, context churn, and cost
  without presenting raw logs as the primary UI.
- [Flightdeck](https://github.com/flightdeckhq/flightdeck) reinforces keeping
  fleet usage and performance analysis in Analytics rather than the live-work
  surface.
- [Claude History](https://github.com/raine/claude-history) demonstrates the
  value of combined semantic and lexical search over agent sessions while
  preserving links back to the source.

## Build order

1. Keep the accepted Inbox All Work view and semantic search.
2. Replace the Issues destination with an HQ route and reuse the existing issue
   data/detail system underneath it.
3. Build the bounded real-data vertical slice in `HQ-UI-DEFINITION.md` inside
   the signed-in dark Multica shell. It uses structured regions, progressive
   disclosure and typed relationships rather than more loose-card directions.
4. Refine the owner-accepted HQ foundation and keep classic issue views available during
   the transition.
5. Build History from real indexed events and session references, then connect
   it from HQ, Projects, Chat search, Deployments, and Agents.
6. Add Analytics views for provider/model usage and wasted-work signals after
   the underlying evidence is reliable.
7. Simplify navigation only after the replacement surfaces work end to end.

## Acceptance boundaries

- No fake data, fake agents, fake PR links, or standalone test-page screenshots
  may be used as owner evidence.
- Visual review happens in the real signed-in Multica shell with real work.
- Percentages are shown only when the system has a real measurable denominator.
- Outside-agent reports remain claims until Multica verifies the business
  result.
- Semantic search remains available in Inbox All Work even after History is
  added.
- The same work item must not become separate competing records across Inbox,
  HQ, Projects, and History.
