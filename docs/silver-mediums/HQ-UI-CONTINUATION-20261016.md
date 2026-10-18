# HQ UI continuation handoff

Date: 2026-10-16 America/New_York

## Outcome of this stopping slice

The rejected floating-card prototype was not polished further. Its basic model
was wrong: grouped issue cards did not give a disorganized, high-concurrency
owner a useful company operating picture.

The durable continuation is:

- `HQ-UI-DEFINITION.md` — page job, objects, hierarchy, states, relationships,
  source mapping, scale rules, first build slice and acceptance scenarios.
- `HQ-PRODUCT-MAP.md` — where HQ sits among Chat, Inbox, Projects and History.
- `HQ-INTERFACE-LEDGER.md` — owner decisions and rejected directions.
- `HQ-COMPANY-INTEGRATION-AUDIT.md` — upstream systems and evidence boundaries.

These repository files, not Task Control prose, are the continuation record.

## Current source state

- Base: protected Multica main `5235c3cb04ead5e24bd346ddb9c3d47538ca1a55`.
- Worktree: `/private/tmp/multica-hq-ui-continuation-20261016`.
- Branch: `codex/hq-ui-continuation-20261016`.
- This stopping slice changes documentation only. It does not claim that HQ is
  implemented or accepted.
- The company-operations foundation provides typed services, signals, incidents
  and an operating-picture endpoint. Missing source credentials/mappings do not
  block the first UI slice; the UI shows unknown/unconnected honestly.

## What remains valid

- Chat is the Mika/orchestrator conversation surface.
- Inbox Needs you is the exception queue.
- Inbox All work and semantic search remain the compact list.
- Existing issue records, details, actions and permissions back the work.
- Agor's spatial grouping, persistent placement and selected relationship
  reveal remain reference behaviors.
- Event Horizon's timeline remains a later History reference, not an HQ canvas
  element.

## What must not be resumed

- Do not restore the removed Project map, Work lanes or Timeline prototypes.
- Do not build loose cards on a blank canvas and call them a map.
- Do not put new HQ surfaces under Inbox All work.
- Do not represent chats as workers or work records.
- Do not duplicate work for each affected project.
- Do not use fake records, standalone HTML or light-mode screenshots as owner
  evidence.
- Do not expand backend integrations before the first UI slice unless a missing
  contract directly blocks it.

## Exact next step

Implement the “First build slice” from `HQ-UI-DEFINITION.md` in the real Multica
Issues route, preserving classic views as a secondary escape hatch. Add the
observable acceptance table to `ui-review/hq-interface-acceptance.md`, then use
real project, issue, working-agent and company-operating-picture data.

Run focused tests while iterating. When a working preview exists, use the
`ui-acceptance-review` workflow with a separate current-task reviewer. Review
dark mode only and use the real signed-in workspace. Correct confirmed failures
before presenting it as ready. The owner makes the final visual decision.

## Next safe stopping point

The next stopping point is a real dark Multica HQ where the seven review
scenarios can be exercised with real data. Code, tests or an attractive canvas
alone are not completion.
