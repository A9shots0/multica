# Item 14 v22 survivor reliability result

Date: 2026-09-13  
Status: complete; valid testing evidence; one survivor; no role selection

| Configuration | Result | Calls | Tokens | Cost | Time | Advances |
|---|---|---:|---:|---:|---:|---|
| GPT-5.6 Luna Pro, Medium | PASS 10/10 | 10 | 118,365 | $0.026380 | 3.440s | Realistic capability testing only |
| Qwen3.8 Flash, Default | FAIL repeated T11 | 1 | 6,853 | $0.002445 | 73.860s | No |

Qwen treated the owner's suspected cause as established fact on its first
reliability observation. It was eliminated without retry and its nine remaining
assignments were skipped. Luna Pro passed both complete five-case repeats.
Combined with v21, Luna Pro now has fifteen consecutive static passes. This is
enough to advance it into realistic capability testing, not enough to select a
role.

Accounting:

- 11 calls plus 9 policy skips = 20 planned assignments
- 125,218 tokens
- isolated receipts: $0.028825
- authoritative account delta: $0.026493110
- preserved difference: $0.002331890
- summed observation time: 77.300 seconds
- retries, tools, owner interventions, and external effects: zero

Canonical evidence:

- `.qualification-evidence/openrouter-v22`
- `server/internal/agentqualification/screening_batch_openrouter_v22_result.json`

Next: one separately frozen Luna Pro realistic capability group covering the
already-validated R01, R03, and R04 scenarios. No role selection.
