# Item 14 responsibility audit

- Status: consolidated placement audit complete; production seats and model
  configurations remain unqualified
- Evidence stage: `local-only`
- Inputs: current role packets, system coverage map, accepted boundary contract,
  hosted interaction audit, and accepted Items 1-13
- Constraint: this document does not rename, add, remove, or configure agents

## Audit method

Each existing role was examined as a bundle of responsibilities rather than as
an assumed permanent seat. The audit asks:

1. What business outcome or decision does it own?
2. What systems, repositories, providers, and hosts must it understand?
3. Is its work deterministic coordination, persistent judgment, dynamically
   loadable expertise, host-bound execution, or independent assurance?
4. Where can it collide with another authority?
5. What evidence would prove the role deserves a permanent agent?

Labels below mean:

- **Keep testing** — coherent responsibility worth evaluating as a persistent
  role; not yet approved or model-qualified.
- **Reshape test** — necessary responsibility, but the existing bundle or name
  may be wrong.
- **Controller** — responsibility should be enforced by deterministic state and
  not depend on an LLM remembering it.
- **Skill candidate** — may be better loaded into a qualified executor than
  represented by another permanent agent.
- **Assurance** — must remain independent from the implementation it judges.

## Current responsibility matrix

| Existing role | Actual responsibility in its packet | Preliminary class | Main overlap or risk | Evidence required before final form |
| --- | --- | --- | --- | --- |
| Mika | Preserve one owner outcome, understand intent, route exceptions, explain verified truth | Owner interface; reshape test | Older packet includes routing/approval continuity now owned partly by the controller; overlaps Company Guide and Chat | Intake and exception cases show accurate routing without repeated workflow wakeups or owner correction |
| Company Guide | Conversational knowledge retrieval and planning without starting work | Owner interface; reshape test | Chat already owns brainstorming, explanations, and concrete-goal binding; may duplicate Mika/context | Real chat cases show a separate identity materially improves accuracy or context cost over one bounded interface |
| Company Auditor | Independent read-only truth/completeness/freshness audits | Assurance; keep testing | Can overlap QC on evidence and Data Sync on reconciliation; must not become a second implementer | Audit cases require breadth and independence not supplied by issue QC or deterministic freshness checks |
| Quality Checker | Independent verdict on exact acceptance outcome and evidence stage | Assurance; keep testing | Can collide with Product/UI taste, Release runtime truth, or Auditor breadth if verdict authority is vague | Cases prove one decisive verdict, correct evidence boundary, no implementation, and no redundant reviewer loop |
| Windows Builder | Execute approved Windows-specific implementation while protecting occupied work | Host-bound executor; skill/shared-pool candidate | Repository/domain expertise may be too broad; implementation ownership can collide with domain owner | Compare persistent seat against qualified builder plus repo/Windows skill packs on real Windows tasks |
| Product & UI | Diagnose, design, implement, and verify user-facing experiences | Persistent capability or enabling role; reshape test | Combines product judgment, design, implementation, and verification; owner taste and QC authority must remain distinct | Visual cases across products prove consistent quality without becoming a mandatory bottleneck for trivial UI work |
| Release & Runtime Reliability | Trace approved revision through deployment/runtime/rollback without claiming business success | Assurance plus consequential executor; reshape test | Crosses every repo/host; Phone Farm host boundary; can become central bottleneck; overlaps QC and domain operators | Release cases prove exact revision/runtime truth, bounded host delegation, recovery, and low handoff cost |
| CRM Identity & Access configuration | Canonical identity, access, shared contracts, and CRM-owned cross-app rules only | Capability-specific configuration; keep testing | Must not own Calendar, subscriptions, Jobs, reporting, or every CRM feature | T01/T12 prove identity authority; T13 proves ordinary CRM product work binds to `crm_product_execution` instead |
| Data Sync & Integrity | Find first divergence across allowed system boundaries; own reconciliation problem, not every app | Persistent cross-boundary capability; keep testing | Can become universal investigator or collide with application owners; may require large context | Boundary cases prove fast first-divergence diagnosis, bounded contributors, and no ownership of unrelated local defects |
| Media Pipeline | Preserve media identity and movement across Drive, conversion, storage, catalog, and consumer availability | Persistent end-to-end capability; keep testing | Crosses CRM/Canopy/R2 and overlaps Intelligence, Publishing, Desktop, and Data Sync at downstream use | Media cases prove a coherent failure model and context advantage over repo ownership or temporary collaboration |
| Phone Farm Application | Own application, jobs, state, contracts, authorization, UI, and Mac application release | Persistent domain plus Mac host-bound executor; keep testing | Very broad product surface; Release and Product/UI support cannot violate Mac-only operation | Mac-only cases separate application cause from fleet cause and complete releases without Windows technical access |
| Phone Fleet Reliability | Own physical devices, bridges, sessions, connectivity, and fleet recovery | Persistent complicated-subsystem authority; keep testing | Boundary with Phone Farm application can be ambiguous at intake; operational actions are consequential | Ambiguous symptom cases correctly isolate app versus fleet with healthy controls and safe per-device recovery |
| Content Inspiration & Telnyx | Own Viral Queue/Content Inspo plus approved Telnyx messaging | Reshape test | Product/data pipeline and company messaging provider may not share expertise, evidence, tools, or risk | Scenario comparison must show whether the combined role reduces handoffs without weakening provider safety or product depth |
| LinkHub & Domains | Own LinkHub pages, redirects, destinations, attribution, domain safety, and public customer journey | Persistent product/domain authority; keep testing | Name may imply repository ownership; CRM authority and Data Sync must remain distinct at the CRM-LinkHub edge | Cross-boundary and public-journey cases prove cohesive expertise and correct deference to CRM identity authority |
| Mac Alert Investigator | Enrich deterministic Mac Mini watcher events without suppressing critical alerts | Skill candidate or optional bounded analyst | Deterministic watcher owns critical detection and wording; prior local candidates did not qualify; may not justify a seat | Only retain if a candidate adds measurable grouping/diagnosis value without missed signals, extra noise, or high cost |
| Canopy Accounts & Connections | Canopy account identity, OAuth/session, assignment, restriction, and lifecycle | Persistent product subdomain; keep testing | CRM identity authority and Data Sync boundaries; credential/account actions are high risk | Cross-system identity and provider cases plus shadow evidence qualify the complete configuration |
| Canopy Publishing & Automation | Queues, workers, outreach, notifications, retries, idempotency, and provider action | Persistent product subdomain; keep testing | Broad action surface; overlaps Telnyx, Accounts, Release, and Data Sync | Queue-to-provider cases prove exactly-once behavior, failure recovery, and authorization discipline |
| Canopy Intelligence & Reporting | Metrics, analytics, semantic systems, recommendations, reports, and AI behavior | Reshape test | Analytics truth, AI evaluation, reporting UX, and recommendation behavior may be too heterogeneous | Scenario clusters must show common evidence/tool needs; otherwise split by skill or responsibility without multiplying seats prematurely |
| Canopy Desktop Application | Installed desktop UI, local state, backend transport, OS integration, and updater | Persistent product/host domain; keep testing | Product/UI, Windows Builder, backend ownership, and Release can create four-way ambiguity | Desktop journey and updater cases prove one accountable lead with bounded shared assistance |
| Social Publishing | Legacy packet spanning Viral Queue and Canopy actions | Exclude from active roster | Contradicts the later Content Inspiration boundary and duplicates Canopy Publishing | Preserve as historical evidence only unless an active outcome proves a missing responsibility |

## Authority conflicts requiring resolution

### Mika versus controller

The older Mika packet owns routine routing, approval continuity, and movement.
Items 1-13 moved deterministic transitions, retries, decision binding, and
recovery into the controller. Final Mika must own intent interpretation,
exceptions, and owner explanation only where judgment is actually required.

### Product/UI versus application owner versus Quality Checker

The application/domain owner understands workflow truth. Product/UI understands
experience design. Drew owns business taste and consequential direction.
Quality Checker owns the evidence verdict. A final topology must state who leads
each kind of UI problem without turning every visible change into four agents
repeating the same inspection.

### Release/Runtime versus host-bound domain owner

Release authority must prove the intended revision is running, but it cannot
grant a Windows agent technical access to Phone Farm. The release stage needs a
host-capability contract that delegates execution to the approved host while
preserving one release receipt and one outcome.

### CRM authority versus satellite behavior

The accepted boundary contract makes CRM authoritative for identity, roster,
access, and cross-app coordination. LinkHub, Canopy, Viral Queue, and Phone Farm
own their local behavior and provider edges. Cross-system work must identify the
first disagreement without allowing a satellite to redefine CRM truth or CRM to
claim the satellite's business behavior is proven.

### Data Sync versus domain ownership

Data Sync should lead only when expected and observed state disagree across a
boundary or reconciliation mechanism. Once the first failing boundary is
proven, the relevant domain executor repairs it while Data Sync preserves the
end-to-end contract evidence. It must not become a general debugging manager.
This boundary remains a hypothesis until the locked scenarios validate it.

### Company Auditor versus Quality Checker

Quality Checker answers whether this issue's exact outcome passed. Company
Auditor answers broader truth/completeness/freshness questions across a named
scope. If the scenario bank cannot distinguish their inputs and verdicts, the
separate permanent role is not justified.

## Coverage gaps found

These are responsibility gaps, not proposed new agents.

1. **System-graph stewardship.** No current role clearly owns maintaining the
   machine-readable map of business journeys, nodes, edges, contracts, and
   regression blast radius. Deterministic drift checks and domain-authored edge
   updates are likely required; final accountability remains to be selected.
2. **Issue-level technical lead selection.** Corrected: the controller owns one
   concrete accountable-actor binding per journey. Ambiguous intake uses a
   bounded read-only triage configuration and can rebind only from a first-
   divergence receipt; Mika and contributors cannot claim ownership.
3. **Permanent regression ownership.** Release and QC execute gates, but the
   domain owner must identify which historical behaviors travel with every
   future release. This needs a durable regression registry rather than prompt
   memory.
4. **Repository execution knowledge.** Existing role packets name repositories,
   but there is no final rule deciding when repo knowledge is a permanent role
   versus a selectively loaded skill.
5. **Cross-issue blast-radius reconciliation.** A shared contract change can
   affect several existing outcomes. The controller needs a deterministic way
   to attach consequences and verification requirements without creating
   duplicate issues or letting one issue silently mutate another.

## First audit result

The current roster should not be preserved wholesale, replaced with one agent
per repository, or converted to a matrix yet. It contains several coherent
specialist candidates, several likely skill/controller responsibilities, and
several unresolved authority collisions. The next evidence gate is a locked
scenario bank and topology comparison; no role rewrite is justified before it.

## Consolidated responsibility placement

This closes the placement question after the locked topology scenarios,
accepted Items 1-13, role-contract work, and two fresh independent read-only
challenges. It classifies responsibilities; it does **not** create permanent
agent seats, choose models, authorize dispatch, or qualify production.

Three properties must not be conflated:

1. **Standing responsibility** — the system always knows what owns the function.
2. **Maintained configuration** — an instruction/skill/model package may be
   versioned even when it is rarely invoked.
3. **Invocation frequency** — a standing responsibility may run only when a
   named condition is present.

This resolves the earlier `persistent` versus `on_demand` wording conflict.
Release truth, for example, is a standing responsibility with a maintained
capability, but release execution is conditional on consequential runtime work.

### Deterministic controller and infrastructure

The following belong to code and durable data, never prompt memory:

- issue identity, one accountable outcome binding, and first-divergence rebind;
- transitions, leases, waits, retries, correction/contributor ceilings,
  interruption recovery, cancellation, supersession, and deduplication;
- typed owner decisions, unchanged-authority reuse, revocation, and expiry;
- capability, host, target, tool, credential-purpose, and independence receipts;
- bounded graph queries, evidence freshness, drift/conflict detection, and
  consequential-action admission;
- cross-issue consequence relationships with exactly-once receipts;
- the versioned permanent regression registry and its contribution history;
- Inbox/SMS admission, delivery dedupe, acknowledgement, and reconciliation;
- backup, watchdog, and recovery state.

System-graph and documentation stewardship is therefore infrastructure work.
Domain evidence may propose a graph or regression change, and a model may help
draft or audit it, but reviewed versioned sources plus deterministic checks are
authoritative. A permanent documentation agent is not the source of truth.

### Reusable skills

Repository navigation, safe commands, domain invariants, provider procedures,
journey evidence, contract reconciliation, UI verification, release checklists,
audit/report formats, and bounded documentation drafting are selectively loaded
skills. A repository is an execution context, not an issue owner.

### Standing responsibility boundaries

- **Mika owner interface:** intake, serious ambiguity, genuine exceptions, and
  verified explanation; never workflow orchestration or status polling.
- **Independent outcome assurance:** one fresh independent verdict for a
  completed outcome. One universal Quality Checker configuration is not yet
  proven.
- **Phone Farm Application and Phone Fleet Reliability:** distinct Mac-only
  technical boundaries. They collaborate inside one issue when needed; Windows
  never inspects or operates them.
- **Accountable outcome registry:** one controller-bound accountability identity
  per journey. These preserve acceptance and regression continuity; they are
  not automatically conversational agent seats.
- **Shared Windows execution and release truth:** standing capability classes
  whose exact permanent configuration shape remains unqualified.

### Conditional participants

Invoke these only when a named graph edge, risk, evidence gap, or judgment
requires unique work:

- Data Sync while a cross-boundary first divergence remains unresolved;
- Product/UI for genuine behavior, interaction, information architecture,
  responsive experience, or owner-taste questions—not routine UI craft;
- Architecture Assurance for truth, identity, schema, credential, lifecycle,
  or host-boundary changes;
- Security/Privacy after deterministic redaction exposes a relevant risk;
- Release/Runtime for a consequential deployment, restart, migration, updater,
  provider configuration, degradation, or rollback;
- Company Auditor for a named broad read-only audit;
- Phone Fleet evidence for physical device, bridge, connectivity, or session
  questions;
- Mac alert analysis when deterministic monitoring cannot supply the required
  non-critical diagnosis.

Product/UI invocation is explicit: exact accepted behavior uses the primary
executor plus the UI skill; ambiguous product behavior invokes Product/UI; a
consequential taste choice becomes one bounded owner decision; independent QC
then judges the locked outcome. These are not four automatic participants.

Content Inspiration and Telnyx are separate responsibility identities because
their permissions, evidence, failure modes, and risk differ. They may later use
the same qualified executor core, but they may not share one unrestricted
context or capability receipt merely because an older label combined them.

The machine catalog now reflects that decision with separate configuration-only
`content_inspiration_outcome` and `telnyx_messaging_outcome` identities. This is
an authority split, not two new agent seats.

Mac alert analysis is retained as a skill-only candidate for sanitized,
non-critical watcher evidence. It cannot monitor, suppress, acknowledge, or
operate alerts and creates no seat. Phone Farm alerts remain in the dedicated
Mac roles.

### Minimal cross-repository route

One business result remains one issue. The controller builds the relevant
subgraph and binds one outcome. One primary Windows executor may work across
CRM, LinkHub, and Canopy using only the required skills. A Mac participant joins
that issue only when Phone Farm evidence or work is required. A contributor
returns only unique named-boundary evidence and stops when the route settles.
Release is optional. One independent verdict covers the complete journey and
unaffected controls. Mika does not relay routine evidence.

### Remaining implementation gates

The placement decision is complete, but production organization is still held:

1. **complete locally:** connect cross-issue consequence receipts to an
   append-only durable store, current workflow snapshots, deterministic issue
   projection, and owner meaningful activity;
2. **complete locally:** connect regression contribution, approval,
   supersession, retirement, and rejected-attempt audit to current durable
   workflow evidence and restart-safe selection;
3. **complete locally:** run the graph/catalog/maintained-source stewardship
   checker before the Item 14 provider-free acceptance tests so drift fails
   closed;
4. **complete for the opt-in guarded local path:** enforce immutable role,
   configuration, capability, claim, daemon, skill, tool, environment,
   credential-purpose, and consequential-action boundaries while preserving
   legacy nil-envelope behavior;
5. qualify complete model/instruction/skill/tool configurations and repeats;
6. prove each proposed permanent seat beats a shared executor plus exact skills
   on repeated real shadow work.

Until those gates pass, the catalog remains a local responsibility and testing
map, not a production org chart.

## Safeguard implementation result

The provider-free foundations for gates 1-3 were built and replay-tested. Final
independent review found four P1 trust gaps and no P0. The one allowed
consolidated correction now requires authenticated controller authority plus
durable current snapshots for consequence links, exact full-journey controller
scope for regression admission, durable hash-chained replay of rejected
regression attempts, and a registry-derived 20-file stewardship authority set.

That original library/checker boundary is historical. Migrations 558-574 now
persist consequence receipts plus accepted and rejected regression histories in
append-only workflow tables. Controller-only transactions lock the workspace,
issue owners, current outcome snapshots, and exact current workflow evidence;
deterministic projection can rebuild all three record types. A disposable
PostgreSQL 17 run proves exact and concurrent replay, altered/stale/untyped
denial, hash-chain restart recovery, and UPDATE/DELETE rejection.

Gate 3 is now part of `silver-mediums-item14-acceptance`; a real maintained-file
change stopped the check until the current source bytes were validated and
rebound. Gate 4 is implemented and fake-tested through the opt-in guarded
dispatch/claim/daemon path, including the isolated child and one-shot authority
lifecycle. These are local controller/application-path proofs only. Production
remains untouched, zero configurations are production-qualified, and legacy
tasks without the guarded envelope remain unchanged.

The remaining seven LLM-executed responsibility artifacts are also complete as
an accepted provider-free candidate set. Their hash-bound manifest rejects
missing, changed, or unlisted files and records one independent review plus one
consolidated correction. Live configuration binding is deliberately denied
until the next gate; no model or permanent seat is selected.

The only remaining implementation gates are configuration qualification and
the comparative permanent-seat decision (gates 5-6). Both require valid real
configuration evidence and owner authority; neither can be resolved by more
provider-free implementation or by promoting the terminal OpenRouter failure.
