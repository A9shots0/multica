# Item 14 — realistic v12 result

Date: 2026-09-13  
Status: complete; valid candidate failure; no survivor; no role selection

| Configuration | Case | Result | Calls | Tokens | Receipt cost | Time | Advances |
|---|---|---|---:|---:|---:|---:|---|
| GPT-5.6 Luna Pro, Medium | R01 | FAIL — response/tool protocol | 8 | 78,489 | $0.014371 | about 80s | No |

Luna Pro correctly found the deliberately faulty consumer, changed only that
consumer, preserved the healthy and Phone Farm fixtures, ran the required
pre-change failure and post-change passing checks, and wrote `EVIDENCE.md`.
It then failed to emit a valid next action or finish receipt on call eight and
produced no final summary. The controller recorded this as the frozen
`response_or_tool_protocol_failure`; it is valid candidate evidence, not a
test-invalidity finding.

Because v12 had one candidate, that failure closed the batch. R03 and R04 were
policy-skipped and received no calls. There was no retry, harness change,
second reviewer cycle, Browser action, production action, owner intervention,
role selection, or Phone Farm operation.

Accounting:

- input: 74,691 tokens
- output: 1,273 tokens
- reasoning: 2,525 tokens
- total: 78,489 tokens
- isolated receipt cost: $0.014371
- authoritative OpenRouter delta: $0.009203
- preserved difference: $0.005168

The final total exceeded the 75,000-token prelaunch reservation by 3,489
tokens. This did not breach the 225,000-token campaign ceiling, but the
reservation miss is logged for post-batch correction. This observation remains
unchanged and will not be retried.

Canonical evidence:

- `.qualification-evidence/realistic-v24`
- `server/internal/agentqualification/realistic_matrix_v12_result.json`

Next: prepare the next bounded model-testing group. Do not select roles.
