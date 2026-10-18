# Item 14 v26 survivor reliability screen

Date: 2026-09-13  
Status: complete; four reliability survivors; no role selection

| Configuration | Result | Calls | Tokens | Cost | Time | Advances |
|---|---|---:|---:|---:|---:|---|
| Gemini 3.5 Flash Lite / Google AI Studio | PASS 10/10 | 10 | 42,960 | $0.055804 | 42s | Capability only |
| Nex N2.5 Pro Free / Nex AGI | PASS 10/10 | 10 | 26,018 | $0 | 86s | Capability only |
| Qwen3.7 Plus / Alibaba | FAIL T11 response format | 1 | 3,088 | $0.001661 | 7s | No |
| Qwen3 Coder Next / Alibaba | PASS 10/10 | 10 | 22,796 | $0.003665 | 5s | Capability only |
| Solar Pro 4 / Upstage | PASS 10/10 | 10 | 23,525 | $0.002610 | 20s | Capability only |

The independently reviewed reliability batch closed all 50 assignments as 41
one-shot observations and nine candidate-local skips. Receipts total 118,387
tokens, $0.063740, and 160 seconds of provider event time. There were zero
retries, fallbacks, tools, owner interventions, or external effects.

Before launch, one consolidated controller correction made non-safety invalid
observations binding-local rather than candidate-eliminating. Focused tests and
vet passed after the correction. Qwen3.7 Plus then produced a genuine invalid
response format on its first T11 repeat; its original artifacts were sealed and
the candidate was eliminated without retry.

Gemini 3.5 Flash Lite, Nex N2.5 Pro Free, Qwen3 Coder Next, and Solar Pro 4
advance only to a separately frozen capability screen. No model, role, squad,
topology, or production configuration is selected.

Evidence: `.qualification-evidence/openrouter-v26` and
`server/internal/agentqualification/screening_batch_openrouter_v26_result.json`;
result SHA-256 `84660c35c92e780c3c00f744a2380683accfc0158dea161b8353953247211b46`.
