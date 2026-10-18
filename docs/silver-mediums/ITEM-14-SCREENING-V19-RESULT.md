# Item 14 static screening v19 result

Date: 2026-09-12  
Status: `complete_with_one_survivor`; testing evidence only; no role selection

## Result

| Candidate | Result | Failure | Calls | Tokens | Cost | Time | Advances |
|---|---|---|---:|---:|---:|---:|---|
| GPT-5.6 Luna, Medium | PASS 5/5 | - | 5 | 11,938 | $0.004264 | 4.641s | Reliability screen only |
| Gemini 3.8 Flash, default | FAIL on T11 | `invalid_response_format` | 1 | 10,397 | $0.031564 | 21.372s | No |
| **Total** | **1 static survivor** | - | **6** | **22,335** | **$0.035828** | **26.013s** | - |

Four later Gemini assignments were policy-skipped after its valid
candidate-specific failure. Six calls plus four skips reconcile all ten frozen
assignments. Retries, tool events, owner interventions, and external effects
were zero. Both actual serving providers matched their pins and fallbacks were
disabled.

## Interpretation

Luna produced one clean pass across owner-truth handling, rollback truth,
credential boundaries, cross-system diagnosis, and documentation/UI planning.
That is sufficient only to enter a separate reliability repeat; it is not a
role, squad, topology, or production selection. Gemini is eliminated from this
batch, not declared universally incapable.

Canonical evidence:

- `.qualification-evidence/openrouter-v19`
- `server/internal/agentqualification/screening_batch_openrouter_v19_result.json`
- result SHA-256:
  `82c8d877583bd36f3b4ca9afca653b3d7f325a0310afc413e1a1f1b84f7c22ec`

Focused tests and vet passed after result sealing. The final paid launch attempt
returned `screening_batch_complete_with_candidate_eliminations` before any
provider call.
