# Item 14 topology-v7 prevalidation

Status: `frozen_reviewed`. The atomic reviewed freeze admits provider calls, but this prevalidation step made none. This is a testing-only comparison and does not select a model, production role, squad, or topology.

## Frozen comparison candidate

- Case: unchanged frozen R03 v2.
- Executor: fresh `realistic.mercury25.screening-default.executor.topology-v7`, exact model `openrouter/inception/mercury-2.5`, effort `screening_default`, serving provider `inception`.
- Reviewer: fresh `realistic.claude-sonnet5.medium.reviewer.topology-v7`, exact model `openrouter/anthropic/claude-sonnet-5`, effort `medium`, serving provider `anthropic`.
- Pair 1: Direct, then Reviewed. Pair 2: Reviewed, then Direct.
- Each Reviewed observation permits at most one correction. Retries, fallbacks, and owner interventions are zero.

## Hard limits

| Scope | Tokens | Cost |
|---|---:|---:|
| Direct observation | 75,000 | $0.20 |
| Reviewed observation | 140,000 | $0.45 |
| Each reviewer stage | 16,000 | included above |
| Entire campaign | 430,000 | $1.30 |

The Reviewed arm includes execution, review 1, one possible correction, and review 2 inside one arm cap. Paid responses must be sealed and counted before another call can be admitted.

## Generation-boundary correction

Topology-v6 remains preserved as a zero-call invalid calibration. V7 does not weaken or edit the accepted realistic matrices. Instead it owns exact generation-specific boundaries for:

- executor plan construction;
- executor receipt sealing after execution and correction;
- executor execution for both initial work and correction;
- reviewer plan construction and resolved-config validation;
- provider source and runtime hashes;
- provider-free preclaim route construction and validation.

Every boundary requires exact equality across configuration ID, model, effort, and serving provider. The executor and reviewer each use a unique v7 transport; no transport alias is used to bypass validation. `RequireCompleteRealisticFreeze` remains an upstream admission gate, preserving the accepted R03 fixture, controller, evidence, and hidden grader.

## Stop and invalidity policy

Global halt is limited to credential failure, uncontrolled spending, unsafe external action, or evidence corruption. Candidate competence and response/tool protocol failures are local. A correction-stage protocol failure stops before review 2. A non-safety harness imperfection invalidates only the affected observation and records a post-batch correction.

A genuine zero-call runtime preflight invalidity has its own exact durable result shape: one zero-call execute receipt, zero tokens and cost, valid evidence hashes, an explicit correction note, claim clearance, and continuation to the next unaffected observation. Malformed zero-call claims are rejected.

## Offline validation completed

The provider-free A-I adversarial matrix proves:

1. executor configuration-ID, model, effort, and provider mutations are rejected;
2. the v7 planner and v7 sealer admit the same exact fresh executor, while the historical matrix-bound planner and sealer remain unchanged and reject it;
3. the named v7 reviewer is accepted, reviewer identity mutations are rejected, and fallback, provider-order, tool, or inherited-instruction drift fails closed;
4. preclaim route failure creates no claim and no provider call;
5. exact zero-call runtime invalidity is recorded locally, clears the claim, and permits the next observation;
6. malformed zero-call invalid receipts are rejected;
7. topology-v6 bytes and the recursive v6-to-v1 command/source chain are preserved and drift-tested;
8. paid partial reviewer accounting, protocol short-circuits, arm/campaign caps, and crash receipt reconstruction are preserved;
9. initial execution and correction both call the generation-bound v7 runner.

The single independent-review correction is now applied. The v7 stage wrapper rejects an already exhausted call allowance as provider preflight, so the controller removes its virtual in-flight call. After every paid response, the wrapper's durable checkpoint checks exact total and generated-token caps; over-cap finish and non-finish responses remain fully counted and return local `stage_budget_exhaustion` without invoking a reviewer. The zero-call and paid harness-invalid validators now share explicit safety, duration, and evidence-hash gates, require the exact execute run identity with `Correction=false`, and require an unchanged tree for a zero-call preflight invalidity.

The R03 v2 reference pass, equivalent paraphrase, subtle incomplete evidence, and adversarial prohibited-action case remain validated. Focused tests and vet must pass again after any correction because all relevant sources are content-hashed.

## Review gate

The one independent review and its single consolidated correction are complete; no second review was requested. Experiment and freeze were atomically activated as `frozen_reviewed`, `provider_calls_allowed` is enabled, dependent hashes were refreshed, and the provider-free plan proved `provider_ready=true` with zero provider calls. This document records admission only; it does not record a candidate observation.
