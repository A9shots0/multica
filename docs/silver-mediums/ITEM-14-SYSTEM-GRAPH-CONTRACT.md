# Item 14 company system graph contract

- Status: machine-readable local foundation implemented; runtime gateway
  integration remains fail-closed until qualification receipts are accepted
- Evidence stage: `local-only`
- Purpose: give agents bounded, current blast-radius context and later provide
  an owner-facing Multica system/data visualizer from the same source

## Product outcome

Drew should be able to open a visual company map and understand how a feature,
system, repository, provider, host, or data flow connects to the rest of Silver
Mediums. Agents should query the same graph before diagnosing or changing work.
The owner visual and the agent context must not drift into separate stories.

## Canonical layers

The graph stores connected but distinct layers:

```text
Business journeys and features
              |
Applications and responsibility boundaries
              |
Repositories, services, jobs, APIs, and contracts
              |
Datastores, queues, files, providers, and caches
              |
Hosts, devices, deployments, and monitored runtime
```

An edge between layers is explicit. A repository relationship cannot be
silently treated as a live runtime relationship, and a runtime relationship
cannot be treated as a verified business outcome.

## Node contract

Every node has:

| Field | Meaning |
| --- | --- |
| `id` | Stable non-secret identifier |
| `kind` | Journey, feature, app, repo, service, job, contract, datastore, provider, cache, host, device group, deployment, role, or regression |
| `name` | Owner-readable label |
| `active_state` | Active, legacy, planned, held, or unknown |
| `business_purpose` | Why the company depends on it |
| `truth_owner` | System authoritative for the represented fact |
| `technical_boundary` | Domain or host allowed to operate it |
| `evidence_stage` | Documented, local, remote Git, deployed, runtime, or business verified |
| `observed_at` | Time of the named evidence |
| `review_after` | Freshness boundary |
| `source_refs` | Safe links to maintained evidence; no credentials |
| `known_unknowns` | Unresolved claims that must remain visible |
| `coverage_state` | Complete, partial, unknown, or contradicted for the named journey/scope |

## Edge contract

Every edge has:

| Field | Meaning |
| --- | --- |
| `id` | Stable edge identifier |
| `from` / `to` | Stable node identifiers |
| `edge_kind` | Data, command, provider, deploys-to, reads, writes, schedules, monitors, renders, authorizes, or verifies |
| `direction` | One-way or explicit bidirectional behavior |
| `allowed_state` | Allowed, forbidden, legacy exception, proposed, or unknown |
| `payload_or_effect` | Business data, command, artifact, or consequence carried |
| `authority_key` | Stable identity/ownership key when applicable |
| `contract_ref` | Schema, version, route, table, queue, or file contract |
| `execution_mode` | Request-time, asynchronous, scheduled, manual, or provider callback |
| `credential_owner` | Owning application/purpose without secret material |
| `fallback` | Durable last-good or declared fail-closed behavior |
| `failure_consequence` | Employee/client/business impact if the edge fails |
| `monitor` | Detection and stale threshold |
| `recovery` | Retry, replay, rollback, or manual recovery boundary |
| `regression_refs` | Behaviors every affected release must recheck |
| `evidence_stage` / `observed_at` / `review_after` | Exact truth and freshness lane |

## Required graph queries

The controller and agents need bounded deterministic queries rather than a full
company dump:

1. `journey_subgraph(journey_id)` — every required node/edge for the business
   outcome.
2. `blast_radius(changed_nodes_or_edges)` — upstream authorities, downstream
   consumers, employee/client journeys, and regression checks affected.
3. `authority_for(fact_or_action)` — truth owner, allowed executor, approval
   requirement, and prohibited hosts.
4. `first_divergence_path(expected, observed)` — ordered evidence boundaries to
   compare without guessing a cause.
5. `release_requirements(candidate)` — target runtimes, migrations/config/assets,
   rollback evidence, and permanent regression set.
6. `stale_evidence(subgraph)` — claims that must be refreshed before action.
7. `issue_route(subgraph, symptom)` — eligible accountable outcomes and bounded
   contributors; this returns candidates and evidence needs, not an unreviewed
   LLM conclusion.
8. `conflicting_claims(subgraph)` — preserved contradictions and the check that
   can settle them.
9. `related_outcomes(changed_nodes_or_edges)` — other issues whose acceptance or
   regression evidence depends on the same contract, release, or system edge.

## Agent use contract

Before diagnosis or implementation, the issue receives:

- the owner's exact outcome and observations;
- the current issue picture and valid decisions;
- only the relevant business-journey subgraph;
- fresh/stale/unknown markers;
- allowed actions and host boundaries;
- historical regressions linked to affected nodes/edges.

If the agent discovers an undeclared edge, it records evidence and stops any
consequential use of that edge until the boundary is classified. It does not
silently expand company architecture from an observed call.

## Drift prevention

The graph cannot rely on prose alone.

- Each repository should eventually declare its outbound hosts, storage
  buckets/prefix purposes, cross-app routes, scheduled jobs, and owned contracts
  in a machine-readable manifest.
- CI compares declared edges with detectable source/config use and blocks a new
  undeclared cross-app edge.
- Runtime/provider reconciliation updates evidence stages and freshness; it
  never rewrites architectural authority from runtime behavior.
- Domain owners propose edge changes with source evidence. Cross-app authority
  changes require the accepted boundary decision process.
- Removed systems become legacy nodes so historical issues and regression
  evidence remain understandable.
- The visualizer displays stale, unknown, forbidden, and contradicted edges
  rather than hiding them.
- Every bounded query returns its coverage state. High-risk action fails closed
  when relevant coverage is partial, unknown, or contradicted until targeted
  evidence expansion resolves the boundary.

## Owner-facing visualizer views

The same graph can provide:

- **Company map:** major apps, providers, hosts, and business journeys.
- **Feature journey:** what happens from owner/employee action to business
  outcome.
- **Data lineage:** where a record originates, copies, transforms, and appears.
- **Blast radius:** everything a proposed change could affect.
- **Incident view:** first failing edge, current impact, recovery path, and
  verified controls.
- **Ownership view:** accountable outcome owner, technical executors, review,
  release, and owner gates.
- **Evidence view:** local, Git, deployment, runtime, and business-proof stages
  with freshness.

The default view must remain simple. Depth appears progressively when Drew or
an agent opens a node or follows a journey.

## Safety and privacy

- Never store passwords, tokens, private keys, raw environment values, customer
  personal data, or sensitive logs in the graph.
- Credential nodes identify ownership and purpose only.
- Provider and runtime evidence is sanitized before graph ingestion.
- Phone Farm nodes can expose owner-readable health and returned evidence while
  keeping technical inspection and action on the Mac Studio.
- A graph edge is not authorization to invoke it.

## Executable authority contract

The controller, not an agent, issues a typed capability receipt only after a
route is admitted. The execution gateway validates graph and route version,
role/configuration, action and targets, host attestation, allowed tools,
credential purpose, authority revision, expiry, revocation, and any mandatory
architecture-assurance receipt. It then returns an execution receipt.

Consequential actions also require a durable owner-authorization receipt bound
to the exact issue, authority revision, candidate, action, target set, and
authorized scope. The receipt remains valid for that unchanged scope without
asking the owner again. It cannot be consumed by an agent, reused for another
candidate, or silently redirected from the authorized release path. An
architecture-assurance receipt must be controller-recorded, fresh, immutable,
issued by the exact allowed assurance configuration, and independent from the
executor.

Runtime placement and agent execution authority are separate. A journey's
`allowed_execution_hosts` is explicit and validated; it is never inferred from
the runtime hosts of every node in that business journey. This prevents a
Phone Farm route from inheriting Railway authority merely because it reads CRM
identity hosted there.

Prompts and role prose can narrow an issued receipt but cannot widen it. The
initial Go foundation validates exact executor and configuration identity,
role/action/host, candidate and owner scope, expiry/revocation, target/tool and
credential-purpose equality, independent coverage assurance, and durable owner
authorization. Real dispatch remains denied until the complete receipt is
connected to the tool gateway.

## Implementation dependency

Do not build the visualizer first. The machine-readable graph, source manifests,
drift checks, bounded query interface, and authority semantics must be accepted
and tested before a UI projects them. Otherwise Multica would gain another
attractive but stale diagram that agents cannot safely rely on.
