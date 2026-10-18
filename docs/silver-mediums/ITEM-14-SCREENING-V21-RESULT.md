# Item 14 current lightweight screen v21 result

Date: 2026-09-13  
Status: complete; valid batch with one candidate-local invalid transport observation; no role selection

## Result

| Configuration | Result | Calls | Tokens | Cost | Time | Advances |
|---|---|---:|---:|---:|---:|---|
| Qwen3.8 Flash, Default | PASS 5/5 | 5 | 25,651 | $0.008410 | 285.460s | Reliability only |
| GPT-5.6 Luna Pro, Medium | PASS 5/5 | 5 | 58,870 | $0.012735 | 0.920s | Reliability only |
| MiniMax M3, Default | FAIL after 4 passes; T09 response types/total | 5 | 16,567 | $0.009642 | 23.205s | No |
| Nex N2.5 Mini Free, Default | FAIL after 4 passes; T09 response types/total | 5 | 12,667 | $0 | 6.920s | No |
| Schematron v2 Turbo, Default | FAIL T11; converted suspicion into fact | 1 | 2,797 | $0.000114 | 2.033s | No |
| Granite 4.2 8B, Default | FAIL T11; converted suspicion into fact | 1 | 7,574 | $0.001409 | 78.921s | No |
| Laguna S 2.1, Default | FAIL T11; verification handling | 1 | 2,587 | $0.000252 | 1.722s | No |
| DeepSeek V4.1 Flash, Default | INVALID transport; no result/export | 1 ambiguous invocation | Unknown | Unknown | 11.494s | No conclusion |

Twenty-four attempted assignments plus sixteen policy skips close all forty
planned assignments. There were no retries. The DeepSeek reservation and raw
failure are preserved, but the invocation produced neither a result nor a
provider export. Its token and cost values therefore remain unknown and it is
not treated as a competence failure.

## Accounting and boundaries

- 23 receipted provider calls plus one ambiguous invocation
- receipted tokens: 126,713
- isolated receipt cost: $0.032562
- authoritative OpenRouter account usage: $110.112503461 to $110.142845751
- account delta: $0.030342290
- preserved receipt/account difference: $0.002219710
- summed observation time: 410.675 seconds
- retries, tools, owner interventions, and external effects: zero

The cost difference is not assigned to DeepSeek without provider evidence.
The post-batch correction list includes a native administrative invalidation
path that preserves unavailable usage fields instead of requiring an
administrative zero placeholder.

No Browser, repository, production, outbound-message, Phone Farm, role, squad,
or topology action occurred. Qwen3.8 Flash and Luna Pro advance only to a
separately frozen reliability repeat.

Canonical evidence:

- `.qualification-evidence/openrouter-v21`
- `server/internal/agentqualification/screening_batch_openrouter_v21_result.json`
- result SHA-256: `4c6f5c3575c663a1fe483aea6566ea5cbf11bd869dc9ee58d9c12e3771721d21`
