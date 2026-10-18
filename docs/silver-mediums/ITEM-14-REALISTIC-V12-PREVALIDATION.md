# Item 14 — Realistic v12 prevalidation

Status: frozen after one independent PASS review; provider execution is unlocked.

## Purpose

Advance the sole v22 static reliability survivor, GPT-5.6 Luna Pro at medium effort, into the unchanged frozen realistic capability sequence. This is testing evidence only. It does not select a role, squad, topology, or production configuration.

## Frozen proposal

- Matrix: `realistic-foundation-screen-v12`
- Candidate: `openrouter/openai/gpt-5.6-luna-pro`
- Actual provider required: `openai`
- Effort: `medium`
- Order: R01, then R03 only after a valid R01 pass, then R04 only after a valid R03 pass
- R02 remains deferred
- Retries: zero
- Provider fallbacks: disabled
- Owner interventions: zero
- Maximum generated tokens per observation: 20,000
- Maximum reserved total tokens per observation: 75,000
- Batch token ceiling: 225,000
- Incremental spend ceiling: $5.00

## Stop behavior

Candidate competence, response-protocol, or hard-disqualifier failure removes only this candidate and does not rewrite or retry the case. Global stopping remains limited to the frozen harness classes: test invalidity, evidence-integrity failure, unsafe execution, provider transport failure, or budget breach. Any non-safety imperfection is preserved for the post-batch correction list; no mid-batch harness change is allowed.

## Independent review receipt

One independent reviewer verified candidate identity, the unchanged R01/R03/R04 fixtures and grader path, provider lock, budgets, zero retries, sequential advancement, evidence integrity, and the no-promotion boundary. Result: PASS with no corrections. No second reviewer cycle was opened.
