# Continuous-work A/B pilot v1

## What this pilot answers

This pilot answers one narrow question: does replacing a long provider session
with three bounded work packages and two fresh-session resumes reduce paid token
processing without making the result worse?

Codex and Claude are separate experiments. A good result from one provider does
not authorize the other. The model, tools, task input, quality checks, and normal
execution settings stay fixed within every matched pair.

The repository harness is deliberately plan-only. It generates the experiment
schedule and evaluates completed receipts. It has no provider client and cannot
start paid work.

## Before paid pairs

For each provider:

1. Supply at least 20 already-completed shadow runs with authoritative terminal
   usage, turn, and wall-time measurements. Collecting these observations does
   not create extra work.
2. Supply 12 representative work items. Each item has an immutable input hash.
3. Declare the hard quality gates before seeing pilot outcomes.
4. Define a blinded 0–100 secondary quality rubric and have reviewers score
   outputs without knowing which condition produced them.
5. Run the contract preflight on one representative item: validate inputs,
   receipt capture, checkpoint byte counting, fresh-session identity, and the
   quality evaluator. A preflight is not an experimental pair and is excluded
   from the result.

Historical p95 measurements set each provider's per-run ceilings:

- processed-token ceiling: `1.25 × p95`, rounded up;
- turn ceiling: `p95 + 2`;
- wall-time ceiling: `1.5 × p95`, rounded up.

At 80% of a run ceiling the runner requests a wrap-up or checkpoint. At 100% it
interrupts the run. These are run-budget percentages, not estimates of how full
the provider's context window is.

## Pair protocol

Every work item produces one matched pair. The generated schedule assigns six
pairs control-first and six treatment-first using a deterministic hash rank.
This controls order effects while making the plan reproducible.

The control is one uninterrupted, provider-native session.

The treatment is exactly three declared work packages. At the two boundaries it
writes a checkpoint no larger than 2 KiB and starts a fresh session with that
checkpoint plus current workspace evidence. It does not carry the old transcript.

Subagents are disabled in both conditions. Current Codex usage telemetry is
thread-scoped, so enabling them would mix work that cannot yet be attributed
reliably to a pair.

No action may use a context-window percentage unless both the used-token
numerator and context-window denominator are exact. Codex app-server
`thread/tokenUsage/updated.last.totalTokens` is accepted as exact active context
when paired with its provider-reported `modelContextWindow`; cumulative
`tokenUsage.total` remains usage traffic and can never trigger rotation. Claude
does not yet expose exact live occupancy on this runtime path and remains
observation-only until its separate provider gate passes.

## Measurement

Only an authoritative terminal receipt counts. For this pilot:

`processed tokens = uncached input + cache read + cache write + output`

The evaluator rejects non-terminal receipts, negative or empty usage, unplanned
runs, subagent use, oversize checkpoints, treatment transcript carryover, too few
fresh rotations, missing or unblinded 0–100 quality scores, context-percentage
enforcement based on inexact values, or a token, turn, or wall-time ceiling breach.

Every declared quality gate must be present for every run. A failed gate rejects
the provider result even when token savings are large.

## Stages and decisions

- Wave 1: four pairs per provider.
- Wave 2: four more pairs, only after a promising Wave 1.
- Normal decision: eight complete pairs per provider.
- Extension: up to four more pairs only when the eight-pair result is
  inconclusive. Twelve is the hard cap.

Stop for futility after Wave 1 when the median treatment/control processed-token
ratio is at least `0.95`, or when fewer than three of four pairs save tokens.

Adopt for a provider only when all hard quality gates pass and all of these are
true:

- median processed-token ratio is at most `0.80`;
- geometric-mean processed-token ratio is at most `0.85`;
- at least seven of eight runs save tokens (the same 87.5% rule applies to an
  extension sample);
- p90 treatment/control wall-time ratio is at most `1.25`.
- median blinded treatment-minus-control quality difference is at least `-5`;
- no individual treatment loses more than 10 blinded quality points against its
  matched control.

An eight-pair result that does not pass may extend within the declared cap. A
result that still does not pass at 12 pairs is rejected. Providers are reported
separately; ratios are never pooled across Codex and Claude.

## Offline harness

From `server/`, generate a plan with:

```sh
go run ./cmd/continuity-pilot -mode plan -input config.json > plan.json
```

After external provider-native runs have produced terminal receipts, evaluate
them with:

```sh
go run ./cmd/continuity-pilot -mode evaluate -input evaluation.json > result.json
```

Both inputs use strict JSON. Unknown fields and trailing values fail closed. The
generated plan is also revalidated during evaluation so edited safety rules or
thresholds cannot silently change the experiment.

## Evidence behind the constraints

- OpenAI documents provider-native compaction and requires opaque compacted
  state to be replayed unchanged: <https://developers.openai.com/api/docs/guides/compaction>
- OpenAI's Codex app-server protocol and official TUI implementation expose and
  consume `last`, cumulative `total`, and `modelContextWindow` with their
  distinct meanings:
  <https://github.com/openai/codex/blob/main/codex-rs/app-server-protocol/src/protocol/common.rs>
  and <https://github.com/openai/codex/blob/main/codex-rs/tui/src/token_usage.rs>
- Anthropic documents context-window behavior and prompt-cache accounting:
  <https://platform.claude.com/docs/en/build-with-claude/context-windows> and
  <https://platform.claude.com/docs/en/build-with-claude/prompt-caching>
- Anthropic's compaction and on-demand compaction controls informed the decision
  to keep provider compaction distinct from Multica's verified work-package
  boundaries: <https://platform.claude.com/docs/en/build-with-claude/compaction>
  and <https://platform.claude.com/docs/en/build-with-claude/compaction-on-demand>

Those provider features are useful building blocks, but none proves exact live
context occupancy or the quality of a resumed business task. This pilot measures
the end-to-end outcome instead of assuming either.
