# AI Workforce HQ data readiness and evaluation gate

Updated 2026-10-18 America/New_York.

Status: authoritative gate for the next HQ concept. No production HQ UI is
approved by this document.

## Product conclusion

The HQ has one job that no existing Multica page performs: give the owner an
evidence-backed mental model of the company operating state. It must show where
effort is going, which outcomes or systems are under pressure, what is being
neglected, and how one material problem affects the rest of the company without
requiring the owner to read or organize individual work records.

Chat remains the command surface. Inbox remains the attention and searchable
work surface. Projects remains the outcome-planning surface. Classic Issues
remains the record-management surface. History remains the temporal evidence
surface. Analytics remains the cost and capacity surface.

The default HQ overview therefore must not render one card, row, or rectangle
per project, outcome, issue, chat, or agent. Those objects appear only after
focus or drill-down.

## Independent product evaluation

An evaluator working from the raw owner conversation, actual Multica surfaces,
and the researched references rejected every card-first direction. The three
interactive wireframes were not materially different product hypotheses: they
shared the same work-card renderer and project/work hierarchy with different
layout rules. Interaction tests proved that cards could move, not that the
owner could understand the company faster.

Two genuinely different information models remain plausible:

1. **Operating topology** — 8–12 aggregate systems, capabilities, or outcomes
   connected by typed dependencies. Current incidents, effort, evidence age,
   and acceptance state alter the topology. Selection reveals one bounded
   impact path and supporting records.
2. **Effort-to-impact flow** — evidenced capacity and activity flow from
   providers, machines, and workers through outcomes to employee/customer
   impact. Blocked or unverified flows break visibly; reported completion and
   accepted completion remain distinct.

A nested company terrain is a fallback only if relationship coverage is too
weak for either model. It is not permission to return to project cards.

## Verified implementation capability

Source inspection confirms that Multica already contains:

- an owner-only canonical system graph with 32 typed nodes, 39 typed edges and
  13 company journeys;
- bounded queries for journey subgraphs, blast radius, stale evidence,
  authority, release requirements, related outcomes and issue routing;
- a company operating-picture contract for cataloged services, correlated
  incidents, evidence lanes, affected references and verification state;
- append-only cross-issue consequence receipts;
- outside-agent receipts that distinguish provider claims, evidence,
  acceptance stage/status, resource use and owner decisions; and
- current issues, projects, agent tasks, current-picture evidence and Inbox
  projections.

The browser client currently fetches the complete system graph but does not
expose the bounded query endpoint as a typed client method. The rejected HQ
model projects most live information back into Projects and work cards and does
not consume recorded cross-issue consequence relationships as company-level
topology.

## Collector and activation audit

The missing operational overlay is not primarily a collector-development
problem. Source and runtime inspection on 2026-10-18 established:

- the Phone Farm Sidecar already contains the merged privacy-safe support
  signal outbox, and that code is present in the healthy live release;
- the live Phone Farm runtime has no Multica endpoint, workspace, source
  credential, or reference-salt configuration, so the outbox is intentionally
  inactive;
- the protected Railway watcher repository contains a durable Multica signal
  outbox, and the actually running 8 GB watcher is a later clean branch that
  also contains that exporter, but it has no Multica source configuration; and
- the live Railway watcher has a large pre-existing incident backlog across
  several products. Importing that backlog without a scoped mapping would turn
  historical detector noise into false current company pressure.

The next step is therefore bounded activation and proof, not a new UI and not
a new generic ingestion framework.

### Trust boundary discovered during the audit

A Multica company-signal source binding is scoped to one project. Cataloged
services written with that credential must belong to the same project. The
current Railway watcher exporter accepts one source key and credential, so one
instance cannot safely publish arbitrary services across several Multica
projects.

The first production pilot is deliberately restricted to the Phone Farm
project. Phone Farm support and Railway runtime evidence use separate source
bindings. They may be correlated only when they carry the same verified
operational reference and deterministic failure family; shared naming or
simultaneous timestamps are not enough.
The four provider-confirmed services in the current PhoneFarm-Uptime Railway
project are `phonefarm-logs`, `phonefarm-stream-probe-singapore`,
`phonefarm-stream-probe-us-east`, and `uptime-monitor`. All reported a successful
deployment with one of one configured replicas on 2026-10-18, but the provider
receipt marked them `source_unbound`; their semantic relationship to the
Phone Farm Sidecar must be explicitly modeled before any incident is presented
as a shared support/runtime situation.

The current 8 GB watcher target catalog does not contain the PhoneFarm-Uptime
project. It currently observes selected LinkHub, CRM, AdminCRM, VCP and
Williams Media services. Its health endpoint reported current Railway log
collection on 2026-10-18, but a fresh target-discovery command was refused by
the current Railway credential. PhoneFarm-Uptime must not be added from stale
configuration or Brain names alone; renewed provider discovery and one observed
target receipt are required first.

The LinkHub and CRM watcher backlog is excluded from this pilot. A future
company-wide watcher needs either one source binding per project or a reviewed
contract extension with an explicit service allowlist. It must not obtain broad
cross-project authority by leaving services unowned.

## Verified A32 workspace snapshot

Read-only authenticated API inspection of the Silver Mediums Pilot workspace on
2026-10-18 found:

| Source | Observed live data |
| --- | --- |
| System graph | 32 nodes, 39 edges, 13 journeys |
| System graph coverage | nodes: 7 complete, 23 partial, 2 unknown; edges: 7 complete, 27 partial, 5 unknown; journeys: 10 partial, 3 unknown |
| Projects | 10 total: 8 in progress, 1 planned, 1 completed |
| Issues | 7 total: 6 todo, 1 in review; 6 linked to projects |
| Outside-agent receipts | 10 total: 9 Codex, 1 Claude; 5 working, 4 waiting externally, 1 reported complete |
| Outside-agent acceptance | 1 accepted; 9 partial; stages range from local to production verified |
| Agent task snapshot | 8 terminal tasks: 5 completed, 3 failed |
| Live working agents | 0 |
| Cataloged company services | 0 |
| Company incidents | 0 |
| Recorded cross-issue consequences in current issue activity | 0 |

This snapshot proves that the static company topology and external-work
acceptance model are real, but the live operational overlays needed for an HQ
are not populated. A visually rich topology built from the current workspace
would either show mostly documented architecture or invent links from titles
and descriptions. Both would misrepresent live company state.

## Decision

**Operating topology is the first concept to validate, but production UI work
is blocked on a minimum evidence bridge.** It is better supported than
effort-to-impact flow because Multica already has typed topology and bounded
blast-radius behavior. Effort-to-impact flow would currently imply capacity and
allocation precision that the live data does not provide.

The first concept may use the canonical graph as context, but it may label a
node or edge as current only when a live source explicitly binds to it. Partial
and unknown coverage remain visible. Unmapped work is one aggregate count and a
drill-down list, never an invented edge.

## Minimum evidence bridge before another HQ UI

The next implementation must first provide and verify these contracts:

1. A typed browser client for bounded system-graph queries, especially journey
   subgraph and blast radius. **Implemented and covered by focused client tests
   on the current HQ branch.**
2. Explicit mapping from current services and incidents to system-graph nodes
   and journeys. **Implemented as first-class validated `system_node_ids` and
   `journey_ids` on cataloged services; incidents inherit the exact cataloged
   service mapping in the operating picture.**
3. Explicit mapping from work/outcomes to affected system nodes or journeys;
   title similarity and free-text descriptions are prohibited. **Implemented
   for work as a versioned `company_work_topology_binding`: each issue binds to
   validated canonical node/journey IDs with owner-or-cited authority,
   evidence, freshness and graph version. Current effort inherits only current
   mappings. This deliberately does not decide whether a future business
   Outcome is a dedicated record or an issue role.**
4. Company-signal adapters for at least Phone Farm support and the Railway
   watcher, activated through project-scoped source bindings and producing
   cataloged services, incidents, affected references and evidence lanes.
5. Company-level projection of admitted cross-issue consequence receipts.
   **Implemented in the operating-picture contract as typed, bounded
   `work_relationships`; raw receipt internals remain controller evidence.**
6. A current-effort projection that distinguishes active execution from stale
   provider reports and terminal task history. **Implemented as bounded
   `current_effort`: live Multica tasks remain native work; outside receipts
   remain explicitly provider-report-only; expired working/waiting receipts
   become stale; provider-complete work stays visible only while the native
   outcome is still open. Terminal task history and natively accepted outcomes
   are excluded and remain available through History.**

Minimum representative evidence for concept validation is:

- at least four cataloged services across two company areas;
- at least two live or recently resolved incidents with typed affected refs;
- one Phone Farm support signal and one Railway watcher signal admitted to
  their correct, explicitly bound services;
- one correlation fixture proving that separate sources merge only when the
  verified service, environment and deterministic failure family match;
- one real cross-area work relationship;
- one admitted cross-issue consequence;
- one active execution, one stale report, one reported-done-but-unaccepted
  receipt and one accepted result; and
- one shared Brain dependency whose evidence coverage is explicitly partial or
  unknown.

This evidence may be captured in an isolated evaluation fixture for product
testing, but production screens must never silently present fixture records.

## Fixed evaluation benchmark

The independent evaluator, not the implementing agent, owns scoring. Use one
fixed snapshot containing six company areas, nine outcomes, forty work records,
twelve sessions, four machines/runtimes, three deployment states, two Phone
Farm support incidents, Railway watcher signals, four owner decisions, five
blocked items, six stale/unknown items, three outside-agent results that are
reported done but not accepted, a CRM–Phone Farm workflow, a shared Brain
dependency, and multiple chats coordinating one outcome.

The candidate must let a fresh reviewer:

- identify the two most important risks and their affected outcomes/systems in
  10 seconds;
- identify effort concentration and one neglected important outcome in 15
  seconds;
- trace the CRM–Phone Farm blast radius to one supporting record in 20 seconds;
- distinguish reported completion from accepted completion in 20 seconds;
- identify stalled or looping work and its blocked outcome in 20 seconds;
- connect Phone Farm support and Railway watcher evidence to one situation in
  20 seconds;
- drill to the canonical work record and return without losing context in 30
  seconds;
- detect one injected incident change without a total layout reshuffle in 10
  seconds; and
- identify the highest risk and required owner action on mobile in 20 seconds.

Repeat at twice the volume. Pass requires at least 90% correct answers and a
30% time improvement over the better of current Projects or classic Issues on
the cross-company tasks. The default overview must contain no individual work
cards and no more than 8–12 aggregate situations/entities.

## Automatic rejection rules

Reject a candidate without further polishing when any of these is true:

- the overview has one card/row/rectangle per project, outcome or issue;
- understanding requires reading work titles;
- regions contain the same grids under new labels;
- compared directions share the same renderer and information hierarchy;
- dragging changes no meaningful owner organization and is required for basic
  readability;
- the owner must maintain layout as new work arrives;
- selection opens a project-summary drawer rather than a bounded operational
  relationship or impact path;
- cross-area work is duplicated;
- relationships are guessed from names or prose;
- unknown evidence is presented as healthy or inactive;
- the concept works only with sparse demo data or a shrunken mobile canvas; or
- it does not beat current Projects or classic Issues in the fixed benchmark.

## Resume condition

UI concept implementation resumes only after the minimum evidence bridge is
demonstrated and an independent evaluator confirms that the operating-topology
prototype is meaningfully different from Projects, Issues and Inbox. A passing
component test, screenshot review, or interaction checklist is insufficient.
