# Item 14 v27 realistic capability screen

Date: 2026-09-13  
Status: complete; valid test; no survivor; no role selection

| Configuration | Result | Calls | Tokens | Cost | Provider event time | Advances |
|---|---|---:|---:|---:|---:|---|
| Gemini 3.5 Flash Lite / requested Google AI Studio | FAIL R01 protocol | 3 | 6,243 | $0.004820 | 16.953s | No |
| Nex N2.5 Pro Free / requested Nex AGI | PASS R01, R03; FAIL R04 | 35 | 95,569 | $0 | 547.372s | No |
| Qwen3 Coder Next / requested Alibaba | FAIL R01 competence | 9 | 19,850 | $0.002617 | 57.565s | No |
| Solar Pro 4 / requested Upstage | FAIL R01 protocol | 3 | 4,896 | $0.000459 | 16.417s | No |

The frozen v27 capability screen completed six attempted observations and six
policy skips. It used 50 calls, 126,558 tokens, $0.007896 in isolated receipt
cost, 638.307 seconds of provider event time, and zero retries or owner
interventions.

Gemini and Solar failed the controller action protocol in R01. Qwen completed
the local repair but failed the hidden evidence contract because the required
current-truth receipt was missing. Nex passed the cross-system authority case
R01 and release-regression case R03, then failed interruption recovery R04: its
canonical handoff did not reject and record the superseded pre-interruption
claim. The forced R04 interruption made no provider call and resumed the exact
same run family, so it was not a retry.

Before provider calls, the one independent review found one narrow stop-policy
defect. The single consolidated correction made credential failures exact
zero-call global halts while keeping ordinary non-safety harness invalidity
local. Focused tests and vet passed, the matrix was frozen, and no further
review cycle ran.

The OpenRouter evidence independently proves the direct OpenRouter route and
the requested upstream provider. Its metadata did not expose the actual
upstream provider for these observations, so that remains explicitly
unverified. The account-wide usage delta was $0.011638, $0.003742 above the
isolated receipt total; both figures are preserved for later reconciliation.

No candidate advances from v27. This selects no model, effort, role, squad,
topology, or production configuration. No browser, production, outbound,
owner, or Phone Farm action occurred.

Evidence: `.qualification-evidence/realistic-v27` and
`server/internal/agentqualification/realistic_matrix_v13_result.json`; result
SHA-256 `029302f86088f11d6d3cc0179fdff22dca810cee2096f6f5738f59f600366f34`.
