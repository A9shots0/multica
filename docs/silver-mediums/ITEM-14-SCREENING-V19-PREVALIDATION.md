# Item 14 static screening v19 prevalidation

Date: 2026-09-12  
Status: `frozen_reviewed`; provider calls not yet made

## Frozen group

V19 reuses the already frozen and adversarially calibrated five-case static
capability group without changing a case, grader, instruction, or skill:
T11 v4, T08 v3, T16 v3, T04 v1, and T09 v1.

The one-pass candidates are:

- `openrouter/openai/gpt-5.6-luna`, Medium effort, provider `openai`
- `openrouter/google/gemini-3.8-flash`, provider-default effort, provider
  `google-ai-studio`

OpenRouter fallbacks are disabled. The plan permits at most ten calls and
100,000 reserved tokens. Each case has one observation and no retry. A valid
candidate failure skips only that candidate. A non-safety invalid observation
is preserved, stops only the affected candidate in this batch, and allows the
unaffected candidate to continue. No tools, Browser, repositories, external
messages, Phone Farm, production, role selection, or owner intervention are
allowed.

## Independent review and consolidated correction

The single independent blocker-only review found two pre-call defects:

1. Gemini used an invalid provider-order pin.
2. The admission gate still stopped the entire batch after an invalid or
   inconclusive observation.

One consolidated correction pinned current OpenRouter provider metadata to
`google-ai-studio`, made invalid/inconclusive evidence candidate-local without
turning it into competence evidence, updated terminal-assignment accounting,
and made runtime preflight resolve both exact candidates. Luna was pinned to
the planned Medium effort. No second review cycle is authorized or required.

## Frozen proof

- batch SHA-256:
  `821b8dc813acd0b28236557a2b532949226aa7b2723f19b417e8965ff247a8c8`
- capability-case group SHA-256:
  `114bd5918ea79fe0024e1f7ccd8d04b777717ad3f6e7afaf1878a1128aeefc2e`
- candidate instructions SHA-256:
  `b77e91b1734a72c15080fe9a1092d89b79e040e318056db4d9719364a4709745`
- qualification skill SHA-256:
  `abc31ad2b3bbd5260859a863965bf77535cbd05c6ed9e5c3ef22489879d2003f`
- Luna resolved-config SHA-256:
  `c34b0c7218095033a762a3b4108b3d75057bc1a92a6897b8c26f5098c29f731b`
- Gemini resolved-config SHA-256:
  `2bd0a426b69dd299e8518debb275a31723cc56903a6ab728a3a67c35599f6702`

Provider-free verification passed:

```text
go test ./internal/agentqualification ./cmd/screening-run -count=1 -timeout 240s
go vet ./internal/agentqualification ./cmd/screening-run
go run ./cmd/screening-run --openrouter-plan --runtime-preflight
```

The test group is frozen for this batch. Post-call imperfections may be logged
but may not rewrite these inputs or trigger an observation retry.
