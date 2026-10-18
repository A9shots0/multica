# Item 14 v22 survivor reliability prevalidation

Date: 2026-09-13  
Status: frozen after one independent blocker-only PASS; provider calls enabled

This batch repeats only the two v21 five-case survivors: Qwen3.8 Flash Default
pinned to Alibaba and GPT-5.6 Luna Pro Medium pinned to OpenAI. It reuses the
exact frozen T11 v4, T08 v3, T16 v3, T04 v1, and T09 v1 packets and graders.
Nothing is tuned to either candidate.

The matrix is two additional repeats per candidate: 20 assignments maximum,
10,000 reserved tokens per launch, 240,000 tokens total, and a $0.25
incremental OpenRouter ceiling above the pre-freeze $110.142845751 account
snapshot. Exact providers are pinned and fallbacks are disabled.

Candidate-specific competence or protocol failure eliminates only that
candidate. A non-safety harness imperfection invalidates only the affected
observation and is preserved without retry. The batch halts only for credential
failure, uncontrolled/budgeted spending, unsafe external action, or unusable
evidence corruption. No tools, Browser, repositories, production, messages,
Phone Farm operations, role selection, or effort escalation are allowed.

Launch requires focused tests, vet, isolated resolved-config preflight, one
independent blocker-only review, and a frozen batch hash. There will be no
reviewer loop.

## Frozen gate

- focused package tests: PASS
- Go vet: PASS
- exact Qwen and Luna Pro resolved-config preflight: PASS
- sole independent blocker-only review: PASS; no correction required
- batch SHA-256:
  `02f47b1e000b89ead6c32326a2ff745972b5e2baa312bf34b613f267bea1ea46`

This hash is the execution boundary. No case, grader, model, effort, route,
provider, budget, or stop-policy change is allowed during the batch.
