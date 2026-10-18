# Item 14 T09 v2 affected-only retest result

Date: 2026-09-13  
Status: complete; two valid passes; no role selection

The post-v23 audit found that T09 v1 ambiguously represented a count as a
numeric-looking string enum. Raw v23 evidence remains immutable and unscored.
T09 v2 changes only `canonical_total` to an explicit JSON integer with hidden
truth `47`. Calibration proves reordered correct sets pass and quoted `"47"`,
integer `52`, missing/extra/wrong-type fields, missing responsive coverage, and
premature release answers fail. One independent review passed the corrected
case and exact two-candidate run matrix before either provider call.

| Configuration | Result | Calls | Tokens | Cost | Provider time | Static status |
|---|---|---:|---:|---:|---:|---|
| Qwen3 Coder Next / Alibaba | PASS | 1 | 2,232 | $0.000350 | 0.469s | Valid 5/5 combined with four v23 passes |
| Gemini 3.5 Flash Lite / Google AI Studio | PASS | 1 | 4,830 | $0.006932 | 5.308s | Valid 5/5 combined with four v23 passes |

Batch total: two calls, 7,062 tokens, $0.007282, 5.777 seconds of provider
event time, zero retries, zero tools, zero owner interventions, and zero
external effects. Both configurations advance only to a separately frozen
reliability screen. No model, role, squad, topology, or production permission
is selected.

Evidence:

- `.qualification-evidence/openrouter-v24`
- `server/internal/agentqualification/screening_batch_openrouter_v24_result.json`
- result SHA-256: `f04053b76f1634c5f08316188a1bf0eec001bb5a85e4d2a12f30141331e3f22d`
