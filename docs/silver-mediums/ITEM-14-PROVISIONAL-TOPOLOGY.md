# Item 14 provisional bounded-layer topology

- Status: accepted local architecture foundation; production role configurations remain unqualified
- Evidence stage: `local-only`
- Derived from: responsibility audit and locked 19-case comparison
- Scenario bank: `silver-mediums.topology-cases.v2`,
  `sha256:d819bb8dd5d71bb4234ebdcf9707a45652fabb0a1a4a4301a59a421544fb25a3`
- Purpose: preserve the reviewed architecture foundation used for role and
  configuration qualification; this document does not authorize production dispatch

## Design target

The organization must combine deep company understanding with bounded context
and explicit authority. It must not become either:

- isolated repository agents that each prove only their component; or
- a free-form swarm where many specialists talk until someone appears confident.

## Five layers

```text
1. OWNER INTERFACE
   Understand Drew, preserve the requested outcome, explain company truth
                         |
                         v
2. DETERMINISTIC CONTROL PLANE
   Issue identity, decisions, route plan, state, timers, retries, recovery,
   attention, notifications, and evidence receipts
                         |
                         v
3. ACCOUNTABLE OUTCOME + EXECUTION
   One outcome lead, one primary executor, only named boundary contributors
                         |
                         v
4. RELEASE / RUNTIME ASSURANCE
   Exact candidate, target, deployment, exposure, health, rollback
                         |
                         v
5. INDEPENDENT OUTCOME ASSURANCE
   Original acceptance result, business journey, regressions, verdict
```

The layers are responsibilities. They are not automatically five permanent
agents.

## Layer 1 — Owner interface

One owner-facing interface handles:

- brainstorming and explanations in Chat;
- turning concrete work into or binding it to one issue;
- preserving Drew's exact observations, desired result, decisions, objections,
  and constraints;
- presenting only genuine decisions and verified meaningful outcomes.

It does not operate the workflow, poll agents, restate screenshots, invent
technical causes, or ask Drew to choose engineering roles. The Company Guide
and Mika responsibilities are tested first as one interface configuration with
different Chat/Issue modes. A second permanent identity survives only if blind
tests show a material accuracy, clarity, or context-cost advantage.

## Layer 2 — Deterministic control plane

Code, not an LLM, owns:

- one issue per business outcome;
- structured owner decisions and authority revisions;
- issue route and role/run receipts;
- leases, waits, timeouts, retries, and interruption recovery;
- worker -> review -> correction -> release -> verification transitions;
- retry and contributor ceilings;
- stale-evidence and regression requirements;
- Inbox admission and SMS policy;
- dedupe, cancellation, supersession, and reconciliation.

The controller queries the company graph to identify allowed roles, hosts,
system boundaries, and required evidence. A model can recommend among eligible
routes when the symptom is ambiguous; it cannot create a route outside the
mechanically allowed set.

When the first failing boundary is unknown, the controller starts a bounded,
read-only triage stage rather than prematurely assigning permanent outcome
ownership. Triage receives eligible graph paths and ends when current evidence
binds one accountable outcome or when one explicit unresolved boundary requires
escalation. It never becomes a second issue or recurring coordinator.

## Layer 3 — Accountable outcome and execution

### Accountable outcome

Each issue has one accountable outcome classification and one controller-owned
`accountable_actor` binding. The binding names a concrete role and qualified
configuration, the acceptance authority it preserves, its precedence over
contributors, a bounded fallback, and the evidence required before rebinding.
Classification alone never owns work. Examples include:

- identity/access authority;
- LinkHub customer journey;
- media availability;
- content supply;
- outreach conversation;
- Phone Farm operation;
- reporting/client communication;
- employee product experience;
- platform/recovery/monitoring reliability.

The bound actor owns continuity of the acceptance result and historical
regression set. It is not necessarily a permanent conversational agent. It may
be a capability-specific configuration of a shared executor. A contributor or
repository label cannot compete with this binding. Ambiguous intake receives a
read-only triage binding; the controller rebinds once, only from a first-
divergence receipt.

### Primary executor

One primary executor receives:

- the exact issue outcome and current milestone;
- the bounded system subgraph and evidence freshness;
- the repository/host skill required for the first failing boundary;
- valid owner authority and explicit prohibitions;
- previous repair/regression requirements;
- what earlier contributors checked and must not be repeated.

Repository expertise normally lives here as scoped skills. A repository earns
a permanent specialist only if repeated tests show that dynamically loaded
knowledge cannot reach the competence threshold reliably or economically.

### Boundary contributors

An additional contributor is admitted only because a named graph edge, host,
provider, or assurance separation requires different expertise or authority.
The contributor returns structured evidence; it does not become a second issue
owner.

The normal execution route starts with one primary executor. Boundary
contributors must appear in a controller-owned route DAG and each must add
unique evidence for a named graph edge, host, provider, or separation-of-duty
requirement. Independent read-only checks may run in parallel within configured
concurrency, token, and time ceilings. The controller cancels unnecessary
fan-out once the first divergence settles the route. Adding a contributor does
not require Drew unless the business scope or authority changes.

## Layer 4 — Release and runtime assurance

Release is a controlled stage, not a general-purpose manager. It owns:

- exact source/candidate identity;
- reviewed-tree and configuration/build-asset identity;
- target service/host and capability permission;
- migration/predeploy and rollback requirements;
- deployed revision and exposed population;
- runtime health and release consequence;
- preservation of the original business outcome after rollback.

Execution is delegated to the approved host-bound lane. For Phone Farm, the Mac
Studio role performs every technical release action and returns the release
receipt; a Windows release role cannot inspect or operate it.

Not every local change invokes this layer. It appears only when a consequential
release, restart, migration, updater, provider configuration, or rollback is in
scope.

## Layer 5 — Independent outcome assurance

A fresh reviewer configuration receives:

- the original owner outcome and accepted scope;
- the exact candidate/release receipts;
- the business-journey subgraph;
- required regressions and unaffected controls;
- current evidence sources and known unknowns;
- no implementation authority.

It returns a structured Pass, Fail, Partial/Needs Owner, or Invalid Evidence
verdict. A Fail creates one bounded correction packet through the controller.
The reviewer cannot waive acceptance criteria or silently redefine success.

The controller records immutable implementation and reviewer provenance. It
rejects verdicts from the same agent or configuration lineage, rejects reviewer
access to mutable implementation channels, and for high-risk work requires a
different model family unless a named deterministic check supplies the missing
independence. Fresh context alone is not independence.

Quality Checker is the likely owner-facing name for this function. Whether one
permanent model configuration can review every domain or whether fresh domain-
qualified reviewer configurations are required is a benchmark question.

## On-demand assurance and specialist functions

These functions do not join every issue:

| Function | Invoked when | Why it is separate |
| --- | --- | --- |
| Data Sync & Integrity | Expected and observed state disagree across a declared boundary | Finds the first divergence without owning every participating app |
| Product & UI | Intended user behavior, information architecture, visual design, or broad responsive experience is genuinely in question | Product judgment differs from routine implementation and final evidence verdict |
| Company Auditor | The requested outcome is a broad independent truth/completeness/freshness audit | Broader scope than issue acceptance review; remains read-only |
| Release & Runtime | Consequential runtime state changes | Deployment truth differs from business success |
| Mac alert analysis | A sanitized non-critical event needs grouping or investigation beyond deterministic policy | Critical detection and wording remain deterministic |
| Architecture assurance | A change alters a cross-app edge, truth owner, identity key, schema/contract, credential purpose, data lifecycle, or host boundary | Prevents an internally consistent but unsafe design from reaching implementation |
| Security/privacy review | A sanitized finding suggests exposure or a consequential permission/privacy change | Independent classification after deterministic redaction and capability containment |

## Persistent specialist test

A responsibility receives a permanent named agent only when all are true:

1. It represents a coherent recurring business outcome or complicated
   subsystem, not merely a repository name.
2. Its tasks share evidence, tools, permissions, failure modes, and refusal
   boundaries.
3. Persistent context materially improves repeated accuracy, recovery, or cost
   compared with a qualified shared executor plus skill pack.
4. It receives enough real work to justify evaluation and maintenance.
5. Its authority does not duplicate another role or deterministic controller
   function.
6. A complete candidate configuration passes locked role tests and shadow work.

Failure of any condition means the responsibility remains a skill, controller
stage, on-demand configuration, or member of a broader coherent role.

## Concern and bug handling

A small report does not trigger a small-minded process. It triggers a bounded
version of the same disciplined process:

```text
Symptom
  -> exact affected journey
  -> bounded graph expansion
  -> current evidence and prior regressions
  -> first failing boundary
  -> root correction
  -> blast-radius tests
  -> full requested outcome
```

"Bounded" means the system inspects every relevant connection without loading
or auditing unrelated company systems. This supplies diligence without spending
company-wide context on every task.

A bounded subgraph must declare whether its feature/edge coverage is complete,
partial, unknown, or contradicted. Partial or unknown coverage is not silently
treated as safety. Consequential work expands the relevant source/runtime
evidence until the affected boundary is understood. Executors cannot waive
partial, unknown, or contradicted coverage. Risk is derived by the controller
from changed node/edge kinds, authority and credential ownership, host
boundaries, manifest drift, and detected startup, migration, scheduler,
provider, or deployment effects. Consequential execution requires a separately
attributable architecture-assurance receipt.

Every consequential action is checked at the tool gateway against a typed,
expiring capability receipt containing route version, role/configuration,
action, target nodes, host attestation, allowed tools, credential purposes, and
coverage-assurance receipt. Missing, expired, revoked, host-mismatched, or
scope-mismatched receipts are denied regardless of graph text or prompt claims.

Role packets and skills provide navigation, invariants, safe commands, and
evidence methods. They are not current system truth. Every run receipt records
their versions and refreshes expired source, deployment, runtime, or provider
evidence before relying on it.

## Expected examples

### CRM -> LinkHub disagreement

- Accountable outcome: LinkHub customer journey remains broken.
- Boundary analysis: CRM owns identity/distribution truth; LinkHub owns its
  local application and public behavior; Data Sync is invoked only while the
  first divergence is unknown.
- Executor: selected after the failing edge is proven, with the matching repo
  skill.
- Release: only for the changed system.
- Verification: CRM authority through public LinkHub behavior plus controls.

### Later deployment restores an old Phone Farm bug

- Accountable outcome: employee Phone Farm workflow.
- Technical execution: Mac Studio only; application and fleet evidence separate.
- Release stage: exact code, bundled assets, configuration, and cut release.
- Regression registry: every prior employee-critical behavior affected by the
  release.
- Verification: new feature plus all affected historical journeys on the actual
  Mac/phone path.

## Cost controls

- Query a bounded graph subview rather than attaching the entire company map.
- Load repository/provider skills only for the current boundary.
- Do not wake the owner interface during routine transitions.
- Do not run assurance stages that the issue does not require.
- Stop topology/model candidates immediately on automatic disqualification.
- Measure all contributor, retry, cache, and owner-correction costs against the
  accepted outcome.

## Known risks requiring independent attack

1. Outcome classifications could become vague coordinators rather than real
   expertise.
2. Shared executors could lack persistent context for subtle systems.
3. The company graph could become stale and create false confidence.
4. A two-contributor normal cap could be too restrictive for genuine incidents.
5. Separate release and QC stages could become token-heavy bottlenecks.
6. One owner interface could be weaker at either brainstorming or concise issue
   communication.
7. Domain-qualified reviewer configurations could accidentally lose
   independence through shared instructions or context.
8. Related issues could share one contract or release while their acceptance
   outcomes remain distinct.

The independent review must provide concrete counterexamples for these risks,
not merely stylistic feedback.
