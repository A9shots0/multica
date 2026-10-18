# Item 14 v25 next-lightweight static screen

Date: 2026-09-13  
Status: complete; three static survivors; no role selection

| Configuration | Result | Calls | Tokens | Cost | Time | Advances |
|---|---|---:|---:|---:|---:|---|
| Solar Pro 4 / Upstage | PASS 5/5 | 5 | 11,744 | $0.001298 | 8.743s | Reliability only |
| North Mini Code Free / Cohere | FAIL T08 owner burden | 2 | 6,569 | $0 | 24.133s | No |
| Nex N2.5 Pro Free / Nex AGI | PASS 5/5 | 5 | 12,682 | $0 | 31.857s | Reliability only |
| Nemotron 3 Ultra Free / NVIDIA | FAIL T16 preparation | 3 | 9,920 | $0 | 56.069s | No |
| Seed 2.1 Turbo / Seed | FAIL T11 response format | 1 | 10,495 | $0.021248 | 146.094s | No |
| Qwen3.7 Plus / Alibaba | PASS 5/5 | 5 | 16,408 | $0.010239 | 68.563s | Reliability only |

The independently reviewed batch closed all 30 assignments as 21 one-shot
observations and nine candidate-local skips. Receipts total 67,818 tokens,
$0.032785, and 335.459 seconds of provider event time. There were zero retries,
fallbacks, tools, owner interventions, or external effects.

Solar Pro 4, Nex N2.5 Pro Free, and Qwen3.7 Plus advance only to a separately
frozen reliability screen. No model, role, squad, topology, or production
configuration is selected.

Evidence: `.qualification-evidence/openrouter-v25` and
`server/internal/agentqualification/screening_batch_openrouter_v25_result.json`;
result SHA-256 `fac8232fb9bd00438ef4d975f96036d65fc934a97ce72cf140338d5ed31d87aa`.
