# Item 14 Claude Subscription Realistic v38 — Result

Status: **complete valid candidate-local R04 failure; testing only**.

The separately frozen v38 phased controller tested Claude Code subscription
`claude-sonnet-4-6` Medium on R04 v1. The candidate completed the required
workflow work by phase 2: it resumed the missing continuation, produced the
single local output, and updated the handoff. It did not finish from that
current truth.

Instead, later phases repeatedly reread stale evidence. In phase 5 the
candidate requested `python resume.py` again. The deterministic controller
rejected that duplicate continuation with `resume action journal identity
mismatch`, so the observation validly failed
`response_or_tool_protocol_failure`.

| Case | Result | Calls | Input | Output | Cost-equivalent | Retries | Ambiguous calls |
|---|---|---:|---:|---:|---:|---:|---:|
| R04 v1 | FAIL — duplicate resume after completed workflow | 5 | 26,129 | 3,098 | 105,944 micros | 0 | 0 |

Evidence:

- Evidence root: `.qualification-evidence/claude-subscription-realistic-v38`
- Freeze SHA-256: `7f73325a3e64fe207d465beca3d54a465e5a28a9f1271441d8576b381f2c67eb`
- Definition SHA-256: `2139ac234382283c0c92268381e7d56ef4a19b793c6370e4ec8ff1417dd3369f`
- Ledger SHA-256: `540a3777895cf4a3e332358b8c890d186579c8d2de88b7a06721db7eb66adf49`
- Result SHA-256: `b5baf56b9b587d35c642b25db9542c882a54a3018351dd9b4a0d5babcfbe4c99`
- `candidate_eliminated=true`; `global_halt=false`

The 105,944-micro value is provider-reported subscription accounting metadata,
not marginal cash spend. The result is a candidate-local failure of this exact
configuration on R04. It is not test invalidity and does not halt unrelated
candidates.

No role, squad, permanent model, permission, deployment, production, Browser,
outbound-message, or Phone Farm selection or action occurred.
