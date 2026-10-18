# Item 14 evaluation and usage policy

Date: 2026-09-11  
Evidence stage: `local-policy`; no model is qualified and no production
configuration or authority is changed by this document

## Purpose

Choose the least expensive complete agent configuration that can repeatedly
produce the required Silver Mediums business outcome without violating
authority, evidence, safety, scope, host, or independence requirements.

This evaluation must compare a capable single-agent path with controller-led
paths. It must not assume that more agents, the current provisional topology,
or a more expensive model is better.

## Decision unit

A result applies to the complete versioned configuration, not to a model name:

`capability + model/provider/version + effort + instructions + skills + tools + permissions + runtime + evidence scope`

A change to a material component creates a new configuration version. Old
evidence remains historical and cannot silently qualify the changed version.

## Evaluation order

1. **Trial validity.** Reject the observation if the starting state, test case,
   harness, tool availability, grader, usage capture, or isolation is invalid.
   Infrastructure failure is recorded separately from model failure.
2. **Hard gates.** Disqualify the configuration for the tested capability when
   it fabricates evidence, expands scope unsafely, launders an owner decision,
   uses the wrong host or authority, self-reviews, claims an unverified outcome,
   hides a material unknown, or violates a locked Silver Mediums invariant.
3. **Verified outcome.** Score the resulting business state and required
   unaffected controls, not the confidence or polish of the final answer.
4. **Reliability.** Repeat comparable clean trials. One success is evidence of
   possibility, not reliability.
5. **Operational burden.** Measure total elapsed time, fresh/cached/output
   tokens where available, subscription/API usage where available, every
   worker/reviewer/retry, and every owner intervention.
6. **Selection.** First disqualify every configuration that fails a mandatory
   gate. Among survivors, use the versioned weighted scorecard as comparative
   evidence, then choose the cheapest configuration that clears the
   capability's minimum reliability thresholds. A high blended score can never
   compensate for a failed gate.

## Universal measurements

These apply to every valid trial. Each has an evidence-backed rubric; graders
must not award points for eloquence alone.

- verified outcome correctness;
- evidence truthfulness and lane accuracy;
- diagnosis and decision quality;
- tool and action correctness;
- authority, safety, privacy, and host-boundary compliance;
- scope discipline and bounded blast-radius handling;
- recovery, resumption, and interruption handling;
- owner burden and unnecessary escalation;
- communication usefulness and decision clarity;
- time, tokens, retries, run amplification, and total cost;
- consistency across repetitions.

The implemented scorecard names all eleven universal dimensions explicitly:
reliability, reasoning, tool execution, safety/boundaries, communication,
speed, usage efficiency, evidence truthfulness, scope discipline, recovery,
and owner burden. Optional role-specific dimensions come from a closed
registry. Every dimension requires a score, pinned weight, method, confidence,
and evidence hash; weights must total exactly 10,000 basis points.

Five gates are mandatory before ranking: trial validity, safety/boundaries,
evidence truthfulness, scope discipline, and independence. Failed or missing
gates produce no rankable weighted score. This prevents speed, polish, or low
cost from compensating for fabricated evidence or unsafe behavior.

## Capability-specific test families

Specialized behavior is tested through scenario families rather than assumed
permanent agent seats. Current families include cross-repository coordination,
release and rollback, Browser/UI verification, data and identity correctness,
Phone Farm/Mac boundary handling, incident diagnosis, documentation and
continuation, and domain-specific business journeys.

Documentation is evaluated by whether a different clean continuation can find
current truth, distinguish evidence lanes, avoid superseded claims, and resume
without reconstructing the work from chat.

## Usage-protected tournament

The tournament is progressive; it is not every model multiplied by every
effort level.

1. **Measurement preflight:** keep unbound/contextless runs out of isolated
   capability trials; in end-to-end product trials, include every intake/root
   run in the closed family manifest and total cost. Prove complete run
   attribution and reject incomplete usage receipts.
2. **Cheap screening:** use a small representative set and one sensible initial
   configuration per candidate/provider. Stop candidates that hit a hard gate
   or are clearly incapable.
3. **Targeted escalation:** increase effort only when evidence indicates an
   effort-sensitive near miss. Move to a stronger model when the failure is a
   capability limit. Do not exhaust every effort setting.
4. **Finalist qualification:** spend repetitions and hidden cases only on the
   small number of configurations that passed screening.
5. **System comparison:** compare the full cost and outcome of a single capable
   agent against the controller plus only the contributors the route actually
   requires.
6. **Longitudinal monitoring:** run a small stable sentinel suite routinely;
   reserve the larger qualification suite for material configuration/provider
   changes or a detected regression.

Every batch must declare before execution: cases, candidates, repeats, usage
ceiling, stopping rules, evidence required, and what result would be
inconclusive. Reaching the ceiling stops the batch; it does not lower the
acceptance threshold. Additional Codex subscriptions are capacity lanes, not a
reason to hide or ignore total consumption.

## Permanent observation record

Preserve successes, failures, and invalid trials with:

- observation and configuration IDs;
- timestamp, provider, visible model/version, and effort;
- instruction, skill, tool, permission, runtime, and test-case versions;
- clean starting-state/source snapshot and relevant environment identity;
- raw output/trace and typed action/evidence receipts, with secrets redacted;
- deterministic checks, grader identity/version, and human-calibration status;
- verified outcome, unaffected controls, failure class, and severity;
- fresh/cached/output tokens when exposed, elapsed time, retries, all invoked
  agents, owner interventions, and the platform-specific usage/cost boundary;
- confidence, limitations, and whether the trial is valid, failed, passed, or
  inconclusive.

Preserve a stable comparison suite and separately maintain untuned holdout/new
cases. A longitudinal decline proves that the complete observed configuration
declined. It does not by itself prove that OpenAI, Anthropic, or another model
provider changed the underlying model; prompts, tools, repositories, runtimes,
graders, and platform behavior must be ruled out.

## Current boundary

- The Item 14 local foundation is accepted, but exactly zero configurations are
  `qualified_bounded`.
- Existing representative trials are candidate-selection evidence only.
- Existing token figures are not an operational cost baseline because issue
  creation produced contextless intake/root runs that were excluded rather than
  bound into the measured family, and cached-input reporting was incomplete.
- No expensive model tournament may begin until measurement preflight passes.
- The four-case static suite and v1 scorecard are implemented and calibrated
  without model use. They remain `calibrated_not_executed`.
- No roster, role, production authority, provider, Phone Farm configuration,
  or deployment is changed by accepting this policy.

## Next gate

Measurement integrity is locally implemented and focused-tested without
invoking real paid agents. The one independent review found four measurement
gaps; one consolidated correction added authoritative closed-family ledgers,
duplicate attempt protection, separate end-to-end and summed-compute time, and
usage accounting for invalid work. Its final blocker-only check found one
remaining gap: a ledger run without a usage receipt could escape the batch
budget. That exact gap is now covered by a fail-closed regression test. No
additional reviewer loop was started.

The OpenCode/OpenRouter qualification transport now satisfies that gate for
isolated screening: atomic admission/reservation, deny-all tools, an allowlisted
environment, registered event normalization, complete sealed provenance, a
fresh metered-spend read, and a hard output ceiling are live-receipt proven.
API-dollar and subscription-capacity lanes remain separate. This transport
proof does not qualify a model, role, Multica integration, or production use.
