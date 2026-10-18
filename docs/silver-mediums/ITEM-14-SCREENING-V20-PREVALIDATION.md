# Item 14 Luna reliability v20 prevalidation

Date: 2026-09-12  
Status: `frozen_reviewed`; provider calls not yet made

## Frozen reliability group

V20 repeats the exact GPT-5.6 Luna Medium configuration that passed the v19
broad screen. It changes no case, grader, candidate instruction, skill,
provider route, fallback behavior, or effort setting. The frozen cases remain
T11 v4, T08 v3, T16 v3, T04 v1, and T09 v1.

The only candidate is:

- `openrouter/openai/gpt-5.6-luna`, Medium effort, provider `openai`

The group schedules three new full-suite repeats: fifteen observations total.
OpenRouter fallbacks are disabled. The plan permits at most fifteen calls,
150,000 reserved tokens, and exactly $0.50 of additional account spend above
the v19 closing reference. Each observation has no retry. A competence failure
eliminates this candidate. A non-safety harness imperfection is preserved as
invalid and unscored. No tools, Browser, repositories, external messages,
Phone Farm, production, role selection, or owner intervention are allowed.

## Independent review

The single independent blocker-only review passed the complete frozen group.
It confirmed the unchanged v19 cases and packets, exact Luna route and effort,
provider pin, disabled fallbacks, fifteen-observation geometry, token and spend
ceilings, no-retry rule, candidate-local stop behavior, and absence of role or
production promotion. No consolidated correction and no second review cycle
were required.

The reviewer also observed an unrelated Windows temporary lock-file failure in
a broader command-suite run. Focused package tests and the v20 runtime preflight
were unaffected, so this did not invalidate or change the frozen batch.

## Frozen proof

- batch SHA-256:
  `d6228daa4216066bbd40a470ad07e33c04a75b9d530dd951a69c368f130d7266`
- capability-case group SHA-256:
  `114bd5918ea79fe0024e1f7ccd8d04b777717ad3f6e7afaf1878a1128aeefc2e`
- candidate instructions SHA-256:
  `b77e91b1734a72c15080fe9a1092d89b79e040e318056db4d9719364a4709745`
- qualification skill SHA-256:
  `abc31ad2b3bbd5260859a863965bf77535cbd05c6ed9e5c3ef22489879d2003f`
- Luna resolved-config SHA-256:
  `c34b0c7218095033a762a3b4108b3d75057bc1a92a6897b8c26f5098c29f731b`
- absolute OpenRouter account-spend ceiling:
  `$110.605537`

Provider-free verification passed:

```text
go test ./internal/agentqualification ./cmd/screening-run -count=1 -timeout 240s
go vet ./internal/agentqualification ./cmd/screening-run
go run ./cmd/screening-run --openrouter-plan --runtime-preflight
```

The reliability group is frozen for this batch. Post-call imperfections may be
logged but may not rewrite these inputs or trigger an observation retry.
