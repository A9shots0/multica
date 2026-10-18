# Item 14 R03 topology recovery group v2

Status: planned, not reviewed, not frozen, no provider calls authorized yet.

## Question

Does one independent review plus at most one bounded correction improve the verified R03 regression/rollback outcome enough to justify its added runs, tokens, cost, and time compared with one capable direct executor?

This is the unresolved question from topology v1. It does not select a topology, model, role, squad, or production configuration.

## Proposed fixed matrix

- Case: unchanged realistic R03 v2 only.
- Model for executor and reviewer: exact `openrouter/anthropic/claude-sonnet-5`, medium effort, Anthropic route, fallbacks disabled.
- Two fresh matched pairs; neither reuses or overwrites topology-v1 outcomes.
- Counterbalanced order: pair 1 Direct, pair 1 Reviewed, pair 2 Reviewed, pair 2 Direct.
- Direct arm: at most 15 provider turns, 75,000 tokens, 20,000 generated tokens, and $0.30 per observation.
- Reviewed arm: at most 25 provider turns, 120,000 tokens, 30,000 generated tokens, and $0.60 per observation, including initial execution, one review, at most one correction, and one final review.
- Group ceilings: four observations, 390,000 tokens, and $1.80 incremental OpenRouter spend.
- Retries: zero. Owner interventions and external side effects: zero.

## Evidence and stop rules

- Preserve topology v1 exactly as historical evidence.
- Every paid response must be durably sealed and counted before a stage-budget decision.
- A candidate/arm failure ends only that observation. A non-safety harness defect invalidates only the affected observation and is logged for post-batch correction.
- Halt the group only for credential failure, uncontrolled spending, unsafe external action, or evidence corruption that makes later results unusable.
- Each matched pair must start from byte-identical isolated R03 fixtures and use the unchanged hidden grader.
- No conclusion is allowed unless both matched pairs are valid. Mixed outcomes remain inconclusive.

## Pre-provider gate

Implementation must preserve v1 and introduce a separate v2 identity/evidence root. Run focused adversarial tests for paid-response sealing, stage accounting, crash recovery, matched-pair hashes, correction ceiling, candidate-local continuation, and global halt behavior. Then obtain one independent review, apply at most one consolidated correction, and atomically freeze the complete group before any provider call.
