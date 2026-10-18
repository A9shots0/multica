# Item 14 GPT-5.4 High R04 effort result

Date: 2026-09-12  
Status: `complete_invalid_unscored`; testing evidence only; no role or production selection

## Frozen question

Does increasing the same GPT-5.4 executor from Medium to High effort make its
forced-interruption and exact-once continuation behavior reliable on the
unchanged frozen R04 v1 case?

The direct-only configuration was
`realistic.gpt54.high.executor.r04-effort-v2`, exact model
`openrouter/openai/gpt-5.4`, High effort, OpenAI serving provider, fallbacks
disabled. Two clean isolated repeats were fixed before calls. Review,
correction, retry, owner, Browser, and external-action paths were disabled.

## Pre-call integrity

One independent blocker-only review passed the complete case, run matrix,
accounting, stop behavior, and recursive freeze bindings. Focused tests, vet,
and the provider-free plan passed. No correction or second review cycle ran.

## Results

| Observation | Result | Failure | Calls | Tokens | Cost | Time |
|---|---|---|---:|---:|---:|---:|
| `r04-gpt54-high-repeat-1` | INVALID | `transport_or_harness_imperfection` | 1 | 0 | $0.000000 | 0.109s |
| `r04-gpt54-high-repeat-2` | INVALID | `transport_or_harness_imperfection` | 1 | 0 | $0.000000 | 0.270s |
| **Total** | **0 scored** | shared zero-token transport incompatibility | **2** | **0** | **$0.000000** | **0.379s** |

Both observations were preserved without retry. Campaign account-spend
reconciliation is exact; the individual zero-token calls remain conservatively
marked ambiguous. Owner interventions and external side effects were zero.

## Interpretation

This batch provides no competence evidence for GPT-5.4 High. Both frozen
observations failed before producing a provider receipt or tokens. The precise
transport incompatibility is intentionally left for the post-batch correction
list rather than tuning or rerunning this result. No model, role, squad,
topology, or production permission was selected.

Canonical evidence:

- `.qualification-evidence/r04-effort-v2/campaign.json`
- campaign SHA-256:
  `52210eb1e9a4d001e042b3d962fc42148e133f531e7bfff5a680ac29da149f2e`

Final verification:

```text
go test ./internal/agentqualification ./cmd/r04-effort-run-v2 -count=1 -timeout 240s
go vet ./internal/agentqualification ./cmd/r04-effort-run-v2
go run ./cmd/r04-effort-run-v2 --plan
```

The final plan returned `next_observation: null` without a provider call.
