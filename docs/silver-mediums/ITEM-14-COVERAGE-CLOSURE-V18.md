# Item 14 valid-coverage closure v18

Status: complete; one valid candidate failure, one preserved invalid observation, and no survivor.

## Purpose

Close the only two cheap-candidate coverage gaps left by invalid T12 v3 calibration evidence. This batch does not select models or assign production roles.

## Frozen matrix

- DeepSeek V4 Flash, high, exact OpenRouter model `openrouter/deepseek/deepseek-v4-flash`, pinned to `deepinfra`.
- Qwen3.7 Flash, high, exact OpenRouter model `openrouter/qwen/qwen3.7-flash`, pinned to `alibaba`.
- T12 v4 is the first gate for both candidates.
- Only candidates that pass T12 continue through the already frozen T11 v4, T08 v3, T16 v3, T04 v1, and T09 v1 cases.
- Maximum: 12 provider calls, 150,000 tokens, and $0.25 above the authoritative pre-batch OpenRouter usage snapshot.
- One observation per candidate/case; no retries, effort changes, tools, repository access, external actions, or role promotion.

## Independent review and correction

The single independent review returned NO-GO only because provider-reported routing metadata was recorded but not compared with the admitted provider pin. The consolidated correction makes receipt normalization reject any non-equivalent reported provider and adds regression coverage, including case-insensitive comparison for OpenRouter display-name capitalization. All other matrix, grader, ordering, accounting, and stop-policy checks passed review.

No second reviewer cycle was opened. The accepted correction was verified by focused tests and `go vet`. The embedded definition retains the harness-required `planned_not_launched` lifecycle value, while this review receipt freezes its matrix, cases, graders, routes, and policies against further changes before execution.

## Completed observations

| Configuration | Result | Failure | Tokens | Cost | Time | Advances |
|---|---|---|---:|---:|---:|---|
| DeepSeek V4 Flash High | FAIL | T12 response was not valid bare JSON | 3,303 | $0.000275 | 21.535s | No |
| Qwen3.7 Flash High | INVALID | Alibaba rejected the request before generation because the 8,000 completion cap was below the 16,000 thinking budget | 0 reported | $0 reported | 72.59s | No |

DeepSeek is eliminated from this configuration by a valid candidate-local protocol failure. Qwen is not scored and was not retried. Because no candidate passed T12, the frozen breadth cases were correctly skipped.

The authoritative OpenRouter account reading moved from `$106.250792781` to `$106.253409941`. That `$0.002617160` account delta is preserved separately and is not forced onto either row because delayed or concurrent posting cannot be excluded. The DeepSeek execution receipt attributes `$0.000275`; the rejected Qwen request exposed no billable token or cost receipt.

Post-batch correction list: make high-effort output and thinking-budget caps mutually compatible before any future Qwen3.7 High observation. This does not authorize a retry of the invalid v18 observation.

Machine result: `server/internal/agentqualification/screening_batch_openrouter_v18_result.json`. Raw immutable evidence: `.qualification-evidence/openrouter-v18`.
