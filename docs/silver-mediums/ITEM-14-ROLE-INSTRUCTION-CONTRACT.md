# Item 14 role, skill, permission, and squad contract

- Status: accepted local contract; production role configurations remain unqualified
- Evidence stage: `local-only`
- Machine authority: `server/internal/systemgraph/roles.json`

## What is fixed in code

The organization is capability-led and graph-routed. Repositories are execution
contexts, not automatic permanent agents. A real issue receives one accountable
business outcome, one primary executor, only named boundary contributors, a
release/runtime stage when consequential state changes, and a separate outcome
reviewer. The deterministic controller owns every transition, retry, wait,
decision, notification, cancellation, and completion receipt.

The machine catalog currently defines:

- one owner interface candidate: Mika;
- one shared Windows executor candidate with dynamically loaded repository and
  journey skills;
- two persistent Mac-only Phone Farm responsibilities because application and
  fleet evidence require different authority and expertise;
- persistent-candidate Release & Runtime assurance;
- persistent independent Quality Checker;
- on-demand Data Sync, Product/UI, Company Audit, Architecture Assurance, and
  Security/Privacy configurations;
- three bounded controller-owned squad templates.

`Company Guide` is not a separate seat. Its useful explanation responsibility
is part of Mika's Chat mode until blind evaluation proves a separate identity
improves accuracy or owner clarity enough to justify another context surface.

`Windows Builder` is not a repository owner. It becomes the shared Windows
executor and loads the selected repository, business-journey, provider, and
verification skills for the current graph route.

## Four instruction layers

Every complete agent configuration consists of all four layers below. A model
name by itself is never a candidate.

1. **Short role core** — durable purpose, allowed decisions, response contract,
   and explicit non-responsibilities.
2. **Selected skills** — navigation, invariants, safe commands, boundary
   contracts, and current-evidence methods for only the named subgraph.
3. **Controller packet** — original owner outcome, current milestone, route DAG,
   authority revision, evidence already gathered, unknowns, prohibitions,
   retry/time/token ceilings, and exact output schema.
4. **Mechanical capabilities** — the actual tools, host, repository worktree,
   provider scopes, and action allowlist. Prompt text cannot expand these.

Current runtime facts never live permanently in role instructions. Each run
records role-catalog version, graph version, instruction version, skill
versions, permission version, model, effort, provider/runtime, and evidence
freshness.

## Permission rules

`RoleCatalog.Authorize(role, action, host)` fails closed. The first implemented
guards prove:

- Quality Checker cannot edit, deploy, change acceptance, or approve its own
  work;
- Mika cannot move workflow state, deploy, or approve releases;
- Windows cannot inspect or edit Phone Farm;
- the Mac Phone Farm application specialist may perform only its explicitly
  allowed Mac-host actions;
- a role packet may narrow authority, never widen the machine allowlist.

The API exposes the redacted architecture only to the workspace owner. Secrets,
raw credentials, phone numbers, and provider tokens are not graph fields.

## Squad construction

The controller builds the smallest valid route:

```text
unambiguous issue
  -> one primary executor
  -> optional named-boundary evidence
  -> optional release/runtime assurance
  -> independent Quality Checker

ambiguous issue
  -> bounded read-only first-divergence triage
  -> bind one accountable outcome
  -> same route as above
```

Normal concurrency is two workers. Correction is capped at two cycles. A failed
review creates one structured correction packet; it does not wake Mika or start
another open-ended discussion. Additional contributors must exist in the route
DAG and add unique evidence for a named edge, host, provider, or separation-of-
duty boundary.

## Cross-repository example

If CRM says LinkHub access is enabled but LinkHub hides the page:

1. The issue remains one customer-access outcome.
2. Read-only triage compares CRM authority, durable propagation, LinkHub's local
   projection, runtime authorization, and UI rendering.
3. The first divergence selects the primary executor and loads only the CRM or
   LinkHub repository skill actually needed.
4. Data Sync may contribute the cross-boundary reconciliation receipt; it never
   becomes a second owner.
5. Release assurance proves exact deployment if a release is needed.
6. Quality Checker verifies the employee journey and unaffected access roles.

This preserves deep repository knowledge without making a repo agent defend its
component instead of the company's outcome.

## Concern and bug intake

Every owner concern is recorded as an observation with source and time. The
controller must preserve separately:

- what the owner observed;
- the owner's proposed cause, if any;
- verified facts;
- hypotheses;
- first-divergence checks;
- affected journey and blast radius;
- root correction and permanent regression evidence.

Agreement is not evidence. A fast answer is not permission to replace the
architecture. A concern changes the system only after the same evidence and
review gates as any other claim.
