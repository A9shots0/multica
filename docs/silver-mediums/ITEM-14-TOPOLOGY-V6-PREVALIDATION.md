# Item 14 topology-v6 prevalidation

Status: `frozen_reviewed`. Provider calls are admitted by the atomic reviewed freeze. This is a testing-only comparison; it does not select a production role or topology.

## Frozen comparison

- Case: unchanged frozen R03 v2.
- Executor: fresh `realistic.mercury25.screening-default.executor.topology-v6`, exact model `openrouter/inception/mercury-2.5`, effort `screening_default`, serving provider `inception`.
- Reviewer: fresh `realistic.claude-sonnet5.medium.reviewer.topology-v6`, exact model `openrouter/anthropic/claude-sonnet-5`, effort `medium`, serving provider `anthropic`.
- Pair 1: Direct, then Reviewed. Pair 2: Reviewed, then Direct.
- Each Reviewed observation permits at most one correction. Retries, fallbacks, and owner interventions are zero.

## Hard limits

| Scope | Tokens | Cost |
|---|---:|---:|
| Direct observation | 75,000 | $0.20 |
| Reviewed observation | 140,000 | $0.45 |
| Each reviewer stage | 16,000 | included above |
| Entire campaign | 430,000 | $1.30 |

The Reviewed arm covers execution, review 1, one possible correction, and review 2 inside its single arm cap. Paid responses are sealed and counted before any next-call decision.

## Stop and invalidity policy

Global halt is limited to credential failure, uncontrolled spending, unsafe external action, or evidence corruption. Executor/reviewer competence and response/tool protocol failures remain candidate-local. A non-safety harness imperfection preserves and invalidates only its affected observation, is added to the post-batch correction list, and does not stop unaffected observations.

## Offline validation completed before review

The whole group is provider-locked and exercises:

- exact executor and reviewer model, effort, provider, and fresh identities;
- a named v6 reviewer accepted while a wrong reviewer is rejected;
- R03 v2 reference success and a semantically equivalent paraphrase;
- subtle missing evidence rejected as incomplete;
- an adversarial prohibited external action rejected;
- malformed initial and correction-stage responses short-circuited locally without invoking the next reviewer stage;
- genuine budget breach and evidence corruption globally halt;
- a harness imperfection invalidates one observation and continues the campaign;
- exclusive claims, exact receipts, counterbalancing, one-correction ceiling, crash-without-retry, and recursive v5-v1 source admission.
- fail-closed admission on any drift in the complete upstream realistic freeze;
- reviewer CLI-completion or export failure after a paid `step_finish` seals the raw receipt and trustworthy usage/cost, while missing or untrustworthy usage remains explicitly ambiguous.

## Review gate

The one independent review and its consolidated correction are complete. The experiment and freeze were atomically changed to `frozen_reviewed` with `provider_calls_allowed: true`; all dependent hashes and provider-free checks were refreshed before any provider work. This document records admission but does not itself execute a provider call.
