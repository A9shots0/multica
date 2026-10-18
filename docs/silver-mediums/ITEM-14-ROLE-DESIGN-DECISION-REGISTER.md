# Item 14 role-design decision register

- Status: active decision register; local foundation accepted, production role design not selected
- Scope: role, squad, topology, instruction, and ownership decisions only
- Evidence boundary: local contracts, machine catalogs, locked scenarios, and completed qualification/topology observations
- Explicit exclusions: model assignment, production authority, hosted changes, and Windows Phone Farm technical work

## How to read this register

This register separates an accepted control-plane rule from a candidate role or
configuration. The accepted local foundation defines how work must be bounded,
routed, authorized, reviewed, and evidenced. It does **not** qualify a model,
make every catalog entry permanent, or authorize real dispatch.

Current machine evidence remains fail-closed: the catalog contains 20 role
definitions, 13 accountable-actor bindings, and 3 bounded squad templates, but
exactly zero configurations are `qualified_bounded`. A status of **resolved in
the local contract** below therefore means the responsibility boundary is
settled; it does not mean an agent configuration is production-ready.

Status values:

- **resolved in the local contract** — the narrow responsibility boundary is
  accepted and must be preserved;
- **held for configuration trials** — the boundary is known, but the permanent
  seat, configuration, or operational fitness is not proven;
- **held for deterministic implementation evidence** — this is control-plane
  or infrastructure responsibility, not a model-selection question;
- **documentation reconciliation pending** — authority is identifiable, but
  older labels still describe an earlier stage.

## Accepted baseline

The following rules are not reopened by the remaining decisions:

1. One issue carries one accountable business outcome. A repository is an
   execution context, not automatic ownership.
2. The deterministic controller owns transitions, retries, waits, correction
   ceilings, decision binding, notification, cancellation, and completion
   receipts. Mika does not act as the workflow engine.
3. Ambiguous intake uses bounded read-only first-divergence triage. The proven
   divergence selects the primary executor; contributors add only evidence for
   named boundaries.
4. Release/runtime work and independent outcome verification are distinct from
   implementation and from each other. Reviewer independence is fail-closed.
5. Exact unchanged owner authority can be reused. A changed candidate, scope,
   target, or release path requires a new valid authority receipt.
6. CRM remains authoritative for identity, roster, access, and cross-app
   coordination. Satellite applications own their local behavior and provider
   edges.
7. All Phone Farm technical inspection, implementation, device work, and
   release execution remains on the dedicated Mac Studio lane. Windows may
   route work and consume sanitized returned evidence only.

Primary evidence:

- `ITEM-14-FINAL-REVIEW-RESULT.md`, accepted result and deliberate remaining
  boundary;
- `ITEM-14-ROLE-INSTRUCTION-CONTRACT.md`, fixed organization, permissions, and
  squad construction;
- `ITEM-14-SYSTEM-GRAPH-CONTRACT.md`, drift, authority, host, and receipt
  contracts;
- `server/internal/systemgraph/roles.json`, local candidate catalog and
  accountable bindings;
- `server/internal/agentqualification/accepted_configurations.json`, current
  held and shadow configurations.

## P0 decisions

### P0-1 — Canonical status and document authority

**Accepted evidence.** `ITEM-14-FINAL-REVIEW-RESULT.md` explicitly approves the
Item 14 local foundation. `ITEM-14-EVALUATION-AND-USAGE-POLICY.md` and the
configuration catalog explicitly preserve zero production-qualified
configurations. The previously stale headings in
`ITEM-14-ROLE-INSTRUCTION-CONTRACT.md` and
`ITEM-14-PROVISIONAL-TOPOLOGY.md` now state the same accepted-local,
production-unqualified boundary.

**Concrete decision.** Treat the final-review result, machine catalogs,
and current evaluation policy as current authority. Treat evaluation,
comparison, audit, and provisional-topology documents as design history unless
a current accepted contract cites a still-open question from them. This
register records that interpretation but does not rewrite those source files.

**Acceptance/falsification test.** A fresh reader must identify, without oral
context, that the local control-plane contract is accepted while every
production configuration remains held. The decision is falsified if any active
entrypoint or current handoff treats a provisional design heading as production
authority or treats local acceptance as role qualification.

**Current status.** **resolved in the local contract**. The authority order and
active source headings now agree: local architecture accepted, production role
configurations unqualified.

### P0-2 — Documentation and system-graph stewardship

**Accepted evidence.** `ITEM-14-RESPONSIBILITY-AUDIT.md` finds no accountable
owner for the machine-readable journeys, edges, contracts, and regression blast
radius. `ITEM-14-SYSTEM-GRAPH-CONTRACT.md` requires repository manifests,
drift checks, freshness, domain-authored edge evidence, and fail-closed partial
coverage, but several mechanisms are still future requirements.

**Concrete decision needed.** Record graph/catalog/version maintenance,
evidence freshness, drift detection, and knowledge-hub synchronization as an
explicit infrastructure responsibility with durable inputs, checks, and change
receipts. Do not infer that this responsibility requires a permanent
documentation agent. Whether any model assists with bounded drafting or audit
remains a later configuration question; architectural authority must stay in
reviewed sources and controller-enforced receipts.

**Acceptance/falsification test.** For a seeded contract or repository-edge
change, the system must identify the affected graph version and regressions,
show the accountable domain evidence, reject an undeclared high-risk edge, and
leave a replayable change receipt. It must also detect an intentionally stale
hub or graph artifact. The decision is falsified if prompt memory can silently
become architectural truth or if one agent can update authority without an
independent, durable check.

**Current status.** **held for deterministic implementation evidence**. This is
infrastructure ownership, not an automatic permanent seat.

### P0-3 — Mika, controller, and Company Guide boundary

**Accepted evidence.** Items 1–13 moved routine routing, movement, retries,
decision binding, and recovery into the controller.
`ITEM-14-ROLE-INSTRUCTION-CONTRACT.md` limits Mika to owner intent, exceptions,
and verified explanation, and keeps Company Guide out of the active seat list
unless blind evidence shows a material benefit.

**Concrete decision needed.** Preserve the narrow resolution: controller for
deterministic workflow; Mika for owner-facing interpretation and genuine
exceptions; no separate Company Guide configuration without comparative
evidence. Trials still must determine whether a Mika configuration can perform
that bounded interface accurately and cheaply.

**Acceptance/falsification test.** Locked intake, correction, waiting, release,
and status-question cases must complete routine transitions with zero Mika
wakeups. Mika must accurately bind ambiguous owner intent and explain current
evidence without claiming status from chat memory. A separate Guide advances
only if blinded repeated cases materially reduce errors or owner burden after
counting the added context surface and cost.

**Current status.** Responsibility collision **resolved in the local contract**;
Mika configuration **held for configuration trials**.

### P0-4 — Product/UI, domain owner, Quality Checker, and owner taste

**Accepted evidence.** The domain owner holds workflow truth; Product/UI holds
experience-design judgment when that judgment is genuinely in scope; the owner
holds consequential business taste; Quality Checker judges the locked outcome
and cannot implement or redefine it. `roles.json` currently keeps Product/UI
on demand, but the responsibility audit records four-way ambiguity for visible
changes.

**Concrete decision needed.** Determine the exact invocation and lead rules
that distinguish routine UI implementation, product-design uncertainty,
business-taste decisions, and independent acceptance. Do not make every UI
change invoke all four surfaces, and do not let Product/UI or QC silently make
an owner decision.

**Acceptance/falsification test.** Use locked UI cases covering: exact supplied
acceptance, ambiguous interaction design, consequential owner preference,
responsive/minimum/maximum data states, and a visually plausible but
functionally wrong result. A viable rule must route each case to the minimum
necessary participants, preserve independent verification, and avoid both an
unnecessary owner interruption and an inferred taste decision.

**Current status.** **held for configuration trials**.

### P0-5 — Release execution, runtime assurance, and host-bound domain work

**Accepted evidence.** Release is a consequential stage, not a business-outcome
owner. It must bind exact candidate/tree/configuration, target, deployed
revision, health, rollback, and the still-open business outcome. The machine
catalog separates Release & Runtime from primary executors and Quality Checker.
Phone Farm is the explicit exception in execution placement: its Mac role must
perform all technical release actions and return the release receipt.

**Concrete decision needed.** Determine whether release execution and runtime
assurance use one qualified configuration or separately qualified
configurations for each allowed host/domain, and define the provenance needed
when the Phone Farm application specialist executes the release. The decision
may not grant a Windows role Phone Farm access or let a builder self-approve.

**Acceptance/falsification test.** Repeated locked release, failed deployment,
deployed-but-unverified, rollback, crash-recovery, and later-regression cases
must prove exact candidate/runtime identity with no false Live/Done claim. A
Phone Farm case must produce one sanitized Mac receipt and independent outcome
verification with zero Windows technical access. The design fails if the same
mutable implementation context can manufacture its own release or review
evidence, or if separation creates unbounded handoffs/cost.

**Current status.** Separation and Phone Farm host boundary **resolved in the
local contract**; production configuration shape **held for configuration
trials**.

### P0-6 — Quality Checker scope and independence

**Accepted evidence.** Quality Checker owns the exact issue-outcome verdict,
receives fresh locked outcome/evidence, has no implementation authority, and
returns structured Pass, Fail, Partial/Needs Owner, or Invalid Evidence.
Controller-proven independence is mandatory; fresh context alone is not
independence. Current topology evidence is mixed and descriptive: reviewed
paths sometimes passed, sometimes failed or became invalid, and consistently
added calls, tokens, cost, or time. No topology advanced.

**Concrete decision needed.** Determine whether one persistent Quality Checker
configuration can validly review every domain or whether independently
qualified domain configurations are required, and identify which action classes
require model review beyond deterministic gates. No decision may weaken the
accepted independent-verdict boundary merely to reduce cost.

**Acceptance/falsification test.** Run repeated, counterbalanced locked cases
across domains and risk classes with false-completion, stale-evidence,
regression, UI, deployment, cross-system, and adversarial acceptance failures.
Measure verdict accuracy, independence violations, correction quality, owner
burden, calls, tokens, cost, and time. A universal configuration is falsified
by systematic domain misses; domain configurations are falsified if shared
instructions/context destroy independence or add cost without accuracy gain.

**Current status.** Verdict responsibility **resolved in the local contract**;
scope and configuration **held for configuration trials**.

### P0-7 — Data Sync versus domain diagnosis

**Accepted evidence.** Cross-repository handling preserves one business
outcome. Bounded triage finds the first divergence. Data Sync may compare
expected and observed state and preserve a reconciliation receipt, but the
proven domain executor repairs the fault. It cannot become a universal
debugging manager or redefine CRM authority.

**Concrete decision needed.** Preserve that narrow on-demand boundary and test
whether a distinct Data Sync configuration adds reliable cross-boundary
evidence beyond the selected domain executor plus graph/skill packet.

**Acceptance/falsification test.** In repeated identity, media, outreach, and
Phone Farm boundary cases, Data Sync must localize the first divergence without
editing, assuming ownership, accessing Phone Farm from Windows, or inspecting
unrelated systems. The separate configuration is falsified if it duplicates
domain investigation, increases handoffs, or performs no uniquely attributable
evidence work.

**Current status.** Authority boundary **resolved in the local contract**;
separate configuration value **held for configuration trials**.

### P0-8 — Company Auditor versus issue Quality Checker

**Accepted evidence.** Quality Checker decides whether one issue's exact locked
outcome passed. Company Auditor performs a broader, named, read-only
truth/completeness/freshness audit and cannot replace issue verification.
`roles.json` keeps Company Auditor on demand.

**Concrete decision needed.** Preserve the scope distinction and determine
whether Company Auditor needs a separately maintained configuration or can be a
qualified on-demand audit packet without persistent identity.

**Acceptance/falsification test.** Paired issue-verdict and broad-audit cases
must yield materially different scopes, evidence contracts, and outputs while
remaining read-only. A separate maintained configuration is falsified if the
same bounded configuration and audit skill performs both correctly without
authority confusion or added owner burden.

**Current status.** Responsibility distinction **resolved in the local
contract**; configuration lifecycle **held for configuration trials**.

### P0-9 — Content Inspiration and Telnyx responsibility coherence

**Accepted evidence.** The machine catalog currently binds both content-supply
and Telnyx-messaging outcomes to one persistent-candidate role but uses distinct
configuration and skill identifiers. The responsibility audit finds that the
product/data pipeline and the consent-sensitive external messaging provider may
not share expertise, evidence, tools, failure modes, or risk.

**Concrete decision needed.** Determine whether the two outcomes justify one
maintained role core, distinct configurations under a shared label, or separate
responsibility identities. No option is accepted merely because the current
catalog combines them.

**Acceptance/falsification test.** Compare repeated content-supply and
consent-to-delivery cases for routing accuracy, prohibited-action compliance,
context/tool overlap, cross-domain errors, maintenance burden, and total cost.
The combined responsibility is falsified if context or permissions needed by
one outcome weakens the other, or if there is no recurring shared evidence that
reduces handoffs or cost.

**Current status.** **held for configuration trials**.

## P1 decisions

### P1-1 — Cross-issue blast-radius reconciliation

**Accepted evidence.** Graph-bounded per-issue routing and related-outcome
queries are accepted. A shared contract or release may affect several existing
outcomes, but each outcome keeps distinct acceptance. The audit records no
final deterministic rule for attaching consequences without duplicate issues
or silent mutation.

**Concrete decision needed.** Define the controller-owned relationship and
receipt rules for one change affecting multiple existing issues: shared
evidence may be referenced, but outcome scope, owner directives, status, and
acceptance may not be silently merged or overwritten.

**Acceptance/falsification test.** Seed one cross-app contract change affecting
multiple existing outcomes plus an unrelated control. The controller must
attach the correct consequence and verification requirements exactly once,
reuse safe evidence, preserve each outcome's scope/status/authority, and exclude
the unrelated issue. Crash/replay must not duplicate links or notifications.

**Current status.** **held for deterministic implementation evidence**.

### P1-2 — Permanent regression ownership

**Accepted evidence.** Release and Quality Checker execute gates, but the
accountable domain must identify the historical behaviors that travel with a
future change. The accepted graph contains permanent regression relationships;
prompt memory is explicitly insufficient authority.

**Concrete decision needed.** Define the durable, versioned contribution and
approval path by which domain evidence adds, updates, supersedes, or retires a
regression obligation. Execution and verification roles consume that registry;
they do not silently author it during a release.

**Acceptance/falsification test.** A seeded fix must add its accepted regression
to later affected release packets, survive restart and instruction changes, and
remain absent from unrelated releases. Unauthorized deletion, stale evidence,
or conflicting domain claims must fail closed and be visible.

**Current status.** **held for deterministic implementation evidence**.

### P1-3 — Permanent specialist versus selectively loaded repository skill

**Accepted evidence.** Repositories are execution contexts, and the shared
Windows executor loads only selected repository/journey/provider/verification
skills. A permanent specialist is permitted only for a coherent recurring
outcome or complicated subsystem with shared evidence, tools, permissions, and
failure modes; persistent context must measurably outperform a qualified shared
executor plus skill packet at sufficient volume and without duplicate
authority. Phone Farm's Mac-only application and fleet responsibilities remain
the accepted host-bound exception under test.

**Concrete decision needed.** Apply the accepted six-part persistent-specialist
test to every candidate domain responsibility using repeated evidence. Do not
convert the provisional catalog wholesale into permanent seats and do not
collapse the Phone Farm host/expertise boundary into a Windows shared executor.

**Acceptance/falsification test.** For each candidate, compare the same locked
cases using the shared executor plus exact skills versus the persistent-domain
configuration. Require repeated gains in competence, recovery, or total cost,
enough real workload to justify maintenance, and no authority duplication. A
permanent seat is falsified when performance is equivalent or worse after full
context, handoff, and maintenance cost.

**Current status.** Rule **resolved in the local contract**; every persistent or
persistent-candidate seat remains **held for configuration trials**. The Phone
Farm responsibility split and host boundary are accepted, but its configuration
must still be qualified only through the dedicated Mac evidence lane.

### P1-4 — Owner burden and operational communication

**Accepted evidence.** The controller must reuse unchanged valid authority,
avoid waking Mika or the owner during routine transitions, issue bounded
correction packets, and surface genuine decisions rather than agent/run noise.
Chat owns brainstorming, new goals, serious ambiguity, cross-issue questions,
and explanations; concrete operational interaction moves to the bound issue and
Inbox. Owner burden is a required qualification metric.

**Concrete decision needed.** Determine whether the bounded Mika and candidate
role configurations can honor those accepted interaction surfaces without
status polling, repetitive summaries, unnecessary approvals, or hidden
consequences. Only a genuine business/safety/authority decision may become
Needs You; verified completion and important incidents must remain visible
through the accepted notification surfaces.

**Acceptance/falsification test.** Replay representative owner conversations
and issue histories, including corrections, partial acceptance, changed scope,
UI evidence, automatic waiting, failed release, rollback, regression, and
verified completion. Measure unnecessary owner prompts, repeated explanations,
time-to-understanding, wrong surface, missed material alerts, and owner
corrections. Any configuration that shifts routing, recovery, or routine status
reconstruction back to the owner fails regardless of raw task completion.

**Current status.** Interaction and escalation boundary **resolved in the local
contract**; configuration fitness **held for configuration trials**.

## Decision gate for final role design

Final role design remains held until:

1. P0-1 has one unambiguous current-authority reading;
2. P0-2, P1-1, and P1-2 have replayable deterministic responsibility evidence;
3. each P0 configuration collision has passed its stated comparison or retained
   only the already accepted narrow boundary;
4. every proposed permanent seat passes the P1-3 test;
5. review/release independence and the Phone Farm Mac boundary pass their
   adversarial cases;
6. owner burden is measured across the complete workflow, not inferred from a
   successful isolated response; and
7. a complete versioned configuration—not merely a model name—passes all
   applicable locked cases and repeats.

Until then, the role catalog is a local executable candidate and testing map,
not a production org chart.

## Runtime enforcement finding

A read-only trace found that `ValidatedConfigurationArtifacts`,
`RoleCatalog.Authorize`, and `ValidateCapabilityReceipt` are currently
library-only. No live task creation, claim, tool, skill, environment, or
credential path invokes them yet. Therefore no candidate configuration is
authorized for real execution even if its static artifact validation passes.

The bounded implementation sequence is fixed for the later runtime gate:

1. add an explicit immutable guarded-task envelope;
2. require qualified artifact and role admission immediately before task
   creation, including recovery and normalized workflow effects;
3. revalidate at the shared claim builder before any tool, skill, MCP, custom
   environment, credential, or task token is exposed;
4. validate capability receipts immediately before mediated remote MCP and
   plugin calls; and
5. keep consequential guarded configurations broker-only until raw shell,
   native provider, Git, and direct deployment actions have an equivalent
   provider-independent interceptor.

The envelope is opt-in: its absence preserves legacy behavior; its presence
fails closed. This finding narrows the implementation gate and does not qualify
any role, model, or production configuration.
