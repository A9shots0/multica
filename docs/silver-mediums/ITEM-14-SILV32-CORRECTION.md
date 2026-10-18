# Item 14 SILV-32 correction and closure record

Date: 2026-09-11  
Scope: local Silver Mediums Multica pilot only

## Review boundary

The final independent review on `SILV-32` returned `REVISE ITEM 14` with
P0=0, P1=6, and P2=1. The result was not overridden. This document records the
single consolidated correction pass required before a bounded closure recheck.

## Corrections

| Review finding | Correction | Proof |
|---|---|---|
| Locked-case routing was incomplete | Every T01-T19 case is table-tested; T05 includes Phone Fleet, T18 Product/UI, T19 Data Sync and Security, and T10 describes deterministic recovery rather than an invented restore path. | `query_test.go`; trace ledger |
| Three maintained system paths were reversed or stale | Canopy owns Drive ingestion, conversion, R2/catalog and X outreach; Viral Queue supplies CRM Content Inspiration; Photobot is explicitly legacy. | `default_graph.json`; authoritative-direction test |
| Graph evidence had no authority/freshness contract | Nodes and edges now expose authority, technical boundary, active/allowed state, observed/review times, unknowns, and regression references. Queries cover stale evidence, authority, release requirements, conflicting claims, related outcomes, and edge-seeded blast radius. | `types.go`; `query.go`; graph tests |
| Capability receipts could bypass the route | Receipt validation is bound to exact issue, authority revision, route version, role/configuration, action, targets, host, tool, credential purpose, route coverage, and a fresh immutable architecture-assurance receipt. | `roles.go`; authority-bypass tests |
| Same-family review substitute was caller-asserted | Work provenance must be immutable and controller-sourced; deterministic substitute receipts bind candidate receipt, acceptance revision, run, pass result, and freshness. | `roles.go`; reviewer-independence tests |
| Qualification scope could be cherry-picked and generic trials were overstated | Applicable cases are derived internally from the locked case bank. All selections remain `provisional_shadow_only`, `held_pending_qualification`, or Mac-owned external holds. No configuration is `qualified_bounded`. | `qualification.go`; `decisions.go`; configuration tests |
| Outcome ownership was conflated with repository execution | Each business journey binds one accountable domain outcome role. The controller separately assigns a repository executor configuration and independent assurance functions. Roles are capabilities, not automatic permanent seats. | `roles.json`; `BuildExecutionPlan`; separation test |
| Scenario comparison ended at T16 | The comparison and trace artifacts cover T01-T19. | topology comparison and trace ledger |
| Closure review: authority query omitted enforcement boundaries | Authority results now include technical boundary, allowed executor, approval requirement, allowed execution hosts, and prohibited hosts; Phone Farm is explicitly Mac-only/Windows-prohibited. | `query.go`; authority-query tests |
| Closure review: architecture assurance could be self-issued | Assurance must be controller-recorded, use the architecture-assurance role and exact expected configuration, be immutable and fresh, and come from an agent other than the executor. | `roles.go`; self-assurance bypass tests |
| Closure review: explicit owner deployment authority was not typed | Durable owner authorization now binds exact issue, authority revision, candidate, action, targets, scope, owner issuer, and controller record. It can be reused without asking again, but cannot authorize another candidate or a silent deployment-path change. | `roles.go`; durable authorization/no-reroute tests |

## Newly captured authority regression

The owner reported a separate Codex host-command defect in which an explicit
`git push`/deploy instruction was rejected repeatedly. Item 14 does not control
that host launcher, so causation is unproved. The architecture nevertheless
treats the incident as a mandatory regression: a recorded, in-scope owner
authorization is durable authority. An agent may verify its exact scope and
candidate identity, but may not reinterpret it as absent, demand repeated
permission, silently switch deployment paths, or mutate unrelated provider
state. Genuine higher-priority safety restrictions must be reported as the
specific blocked action, not as invented owner indecision.

## Acceptance boundary

These corrections establish a fail-closed local control-plane foundation.
They do not promote any model configuration, dispatch production work, operate
Phone Farm from Windows, or claim a live business outcome. Promotion requires
the locked applicable benchmark cases and repeats for the complete versioned
configuration.
