# Item 14 Claude Sonnet 5 High R04 effort result

Date: 2026-09-12  
Status: `complete_failed`; testing evidence only; no role or production selection

## Frozen question

Does increasing the same Claude Sonnet 5 executor from Medium to High effort
make its forced-interruption and exact-once continuation behavior reliable on
the unchanged frozen R04 v1 case?

The direct-only configuration was
`realistic.claudesonnet5.high.executor.r04-effort-v1`, exact model
`openrouter/anthropic/claude-sonnet-5`, High effort, Anthropic serving provider,
fallbacks disabled. Two clean isolated repeats were fixed before calls. Review,
correction, retry, owner, Browser, and external-action paths were disabled.

## Pre-call integrity correction

The one independent review found two admission defects before any provider
call: configured global halt reasons were compared as an exact set instead of
an allowlist, and direct-only crash recovery could discard authoritative
ambiguous spend. One consolidated correction added allowlist membership,
conservative execute-stage spend attribution, and adversarial tests. Focused
tests, vet, and the provider-free plan passed; no second review cycle occurred.

## Results

| Observation | Result | Failure | Calls | Tokens | Cost | Time |
|---|---|---|---:|---:|---:|---:|
| `r04-high-repeat-1` | FAIL | `competence_failure` | 9 | 29,468 | $0.087274 | 88.416s |
| `r04-high-repeat-2` | FAIL | `response_or_tool_protocol_failure` | 7 | 21,618 | $0.067664 | 70.990s |
| **Total** | **0/2 pass** | - | **16** | **51,086** | **$0.154938** | **159.406s** |

Both observations have exact receipt accounting, zero retries, zero owner
interventions, and zero external side effects. The campaign is terminal and
has no next observation.

## Interpretation

High effort did not rescue this R04 configuration and did not demonstrate
reliable interruption recovery. The configuration is eliminated for this
tested capability. This does not claim that the model is universally
incapable, select another model, or assign any role.

Canonical evidence:

- `.qualification-evidence/r04-effort-v1/campaign.json`
- campaign SHA-256:
  `dff4f047c4408687f5b4dd7fec2e251c67966c6c2e0242bc894d3569beed4b00`

Final verification:

```text
go test ./internal/agentqualification ./cmd/r04-effort-run-v1 -count=1 -timeout 240s
go vet ./internal/agentqualification ./cmd/r04-effort-run-v1
go run ./cmd/r04-effort-run-v1 --plan
```

The final plan returned `next_observation: null` without a provider call.
