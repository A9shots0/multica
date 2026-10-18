# Item 14 new lightweight screen v21 prevalidation

Date: 2026-09-13  
Status: `frozen_reviewed`; provider calls not yet made

## Frozen group

V21 reuses the unchanged, independently calibrated five-case static group:
T11 v4, T08 v3, T16 v3, T04 v1, and T09 v1. It changes no case, grader,
candidate instruction, or qualification skill.

The one-pass candidate pool is:

| Candidate | Effort | Pinned provider |
|---|---|---|
| `openrouter/inference-net/schematron-v2-turbo` | provider default | `inference-net` |
| `openrouter/deepseek/deepseek-v4.1-flash` | provider default | `deepseek` |
| `openrouter/qwen/qwen3.8-flash` | provider default | `alibaba` |
| `openrouter/ibm-granite/granite-4.2-8b` | provider default | `deepinfra` |
| `openrouter/minimax/minimax-m3` | provider default | `coreweave` |
| `openrouter/nex-agi/nex-n2.5-mini:free` | provider default | `nex-agi` |
| `openrouter/openai/gpt-5.6-luna-pro` | Medium | `openai` |
| `openrouter/poolside/laguna-s-2.1` | provider default | `poolside` |

These exact configurations have no prior v1-v20 static observation. They add
new model generations, family diversity, one free baseline, and the direct
Luna-Pro follow-up without repeating an already validly eliminated exact
configuration.

OpenRouter fallbacks are disabled. The plan permits at most forty calls,
400,000 reserved tokens, and $0.50 of additional account spend above the
fresh $110.112503461 pre-freeze reference. Each candidate/case has one
observation and no retry. A valid failure eliminates only that candidate. A
non-safety invalid observation is preserved and unscored, stops only its
candidate in this batch, and does not interrupt unaffected candidates. Shared
failures are logged for a post-batch test audit rather than changing the frozen
grader during execution.

## Independent review

The single completed independent blocker-only review returned PASS. It
confirmed the current loader selects v21; all five cases byte-match the frozen
reviewed group; all eight exact routes resolve with pinned providers and no
fallback; and the controller enforces 40 observations, 400,000 reserved tokens,
an 8,000 generated-token cap, a $0.025 per-call spend reservation, and the
absolute $110.612504 account ceiling. It also confirmed candidate-local failure
and invalidity, global safety/evidence/spend stops, and the exclusion of tools,
repositories, production, external effects, owner intervention, retries,
effort escalation, and role promotion. No correction or second review cycle
was required.

## Frozen proof

- batch SHA-256:
  `0f8b4b9cdf59e362ff398db8dbecf95e84c4ce0cf269e8eb863af628c06d0140`
- capability-case group SHA-256:
  `114bd5918ea79fe0024e1f7ccd8d04b777717ad3f6e7afaf1878a1128aeefc2e`
- candidate instructions SHA-256:
  `b77e91b1734a72c15080fe9a1092d89b79e040e318056db4d9719364a4709745`
- qualification skill SHA-256:
  `abc31ad2b3bbd5260859a863965bf77535cbd05c6ed9e5c3ef22489879d2003f`
- absolute OpenRouter account-spend ceiling:
  `$110.612504`

Provider-free verification passed:

```text
go test ./internal/agentqualification ./cmd/screening-run -count=1 -timeout 240s
go vet ./internal/agentqualification ./cmd/screening-run
go run ./cmd/screening-run --openrouter-plan --runtime-preflight
```

The batch is frozen. Provider execution may not modify or refreeze its cases,
graders, candidate definitions, or stop policy and may not retry an observation.
