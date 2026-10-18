# Item 14 GPT-5.4 High R04 prevalidation

Date: 2026-09-12  
Status: `pending_independent_review`; provider locked; zero calls

## Fixed experiment

- Candidate: `realistic.gpt54.high.executor.r04-effort-v2`
- Exact model: `openrouter/openai/gpt-5.4`
- Serving provider: `openai`; fallbacks disabled
- Effort: `high`
- Case: unchanged frozen `R04 v1`
- Order: `r04-gpt54-high-repeat-1`, then `r04-gpt54-high-repeat-2`
- Topology: direct executor only; no reviewer or correction
- Per observation: 15 calls, 75,000 total tokens, 30,000
  output-plus-reasoning tokens, $0.35, eight minutes
- Campaign: 30 calls, 150,000 tokens, $0.70
- Retries, owner interventions, Browser calls, external effects: zero

Both repeats run even if the first has a candidate-specific competence or
response/tool-protocol failure. A non-safety harness or transport imperfection
invalidates only the affected observation, is preserved without retry, and
does not block the unaffected repeat. Only credential failure, uncontrolled
spending or budget breach, unsafe external action, or unusable evidence
corruption halts the group.

The new namespace preserves the completed Claude High effort-v1 freeze and all
topology-v9 through v1 freezes. It changes no case, task, fixture, grader,
declaration, controller, or shared accounting source.

Provider-free gates passed before review:

```text
go test ./internal/agentqualification ./cmd/r04-effort-run-v2 -count=1 -timeout 240s
go vet ./internal/agentqualification ./cmd/r04-effort-run-v2
go run ./cmd/r04-effort-run-v2 --plan
```

The plan resolved the exact candidate/provider route, returned
`provider_ready:false`, selected only repeat 1, and made no provider call.
