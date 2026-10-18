# Item 14 architecture preflight

- Status: author preflight complete; independent review still required
- Evidence stage: `local-only`
- Scope: internal consistency and omitted-boundary check before review lock

## Findings

### P1 — Ambiguous issues need a bounded triage stage before outcome ownership

The provisional topology assumed one accountable outcome could be selected at
intake. Several real reports do not reveal whether the problem is application,
data boundary, runtime, provider, UI, or device related. Selecting a permanent
domain owner too early can recreate anchoring and rerouting.

**Correction required:** make time-bounded first-divergence triage a controller
stage. It keeps the same issue, receives read-only evidence and the relevant
graph candidates, and ends by binding one accountable outcome or producing one
explicit unresolved-boundary escalation. It is not a permanent issue owner.

### P1 — Fixed contributor count is too blunt

One primary executor plus one contributor is a useful normal path, but complex
incidents can require multiple independent read-only checks. A raw numerical cap
could force serial work or conceal a required boundary.

**Correction required:** constrain contributors by named graph boundaries and a
predeclared route DAG, while separately limiting concurrency, retries, and total
context. Additional work must add unique evidence rather than repeat prior
inspection. The controller stops fan-out when the first divergence settles the
route.

### P1 — System graph completeness cannot be assumed

The current maintained map is application-level and contains dated/contradicted
edges. Silver Mediums has far more feature and job detail than the initial map.
A bounded query can create false confidence if the underlying catalog is
incomplete.

**Correction required:** every journey/subgraph carries coverage state
(`complete`, `partial`, `unknown`, `contradicted`) and evidence freshness.
High-risk changes with partial/unknown coverage require targeted source/runtime
expansion before implementation. Low-risk local work may continue only when the
unknown area cannot affect the named journey or authority boundary.

### P1 — Skills and role packets can become stale architecture

Loading a repository skill is not proof of current source, runtime, or provider
behavior. Persistent specialists can also anchor on remembered history.

**Correction required:** skills contain navigation, invariants, safe commands,
and evidence methods, while current facts are fetched at task time. Packets and
graph snapshots are versioned in every run receipt. Expired evidence is visibly
refreshed before consequential action.

### P1 — New cross-app design needs independent architecture assurance

The topology covered implementation, release, and QC, but an allowed new
business journey can still encode a poor source-of-truth or edge design before
implementation starts. QC after implementation is too late.

**Correction required:** consequential changes to system edges, identity
authority, schemas/contracts, credentials, data lifecycle, or host boundaries
invoke a read-only architecture-assurance stage before implementation. This is
an on-demand configuration/skill unless workload and qualification later justify
a permanent seat.

### P1 — Security/privacy concerns need deterministic containment first

The first topology did not explicitly separate redaction/capability containment
from model judgment.

**Correction required:** tools redact secrets, credentials remain scoped, and
dangerous actions are unavailable or gated mechanically. A bounded security
review can classify evidence, but no unrestricted "security agent" receives
company-wide secrets or automatic rotation authority.

### P2 — CRM is larger than the existing `CRM & Access` role name

The existing packet correctly specializes in identity, access, and contracts,
but many CRM-native product, job, reporting, calendar, subscription, and client
workflows do not fit it.

**Correction required:** retain the specialization question and test ordinary
CRM work against a shared executor with CRM product/repository skills before
creating a broad `CRM` permanent agent.

### P2 — Graph stewardship must not become another prose-maintenance job

No permanent role should manually keep a giant map current.

**Correction required:** repository manifests and static/runtime reconciliation
produce proposed graph changes; responsible domains attest their own boundaries;
cross-app authority changes use the accepted decision process; deterministic
checks surface drift. A human-readable visual is generated from the same data.

### P2 — Cross-issue blast radius needs dependency links without issue merging

A shared contract release may affect several outcomes. One-issue-per-outcome
does not mean issues are isolated.

**Correction required:** the controller records typed dependency/impact links,
attaches shared release and regression receipts, and prevents one issue from
closing or mutating another silently. Issues merge only when they are the same
business outcome.

## Preflight verdict

`REVISE BEFORE INDEPENDENT REVIEW`

The bounded-layer direction remains viable, but the concrete candidate must add
ambiguous triage, graph coverage states, route-DAG limits, architecture
assurance, deterministic security containment, skill freshness, and cross-issue
dependency semantics before the independent review lock.

