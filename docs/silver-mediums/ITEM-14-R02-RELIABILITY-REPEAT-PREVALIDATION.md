# Item 14 R02 reliability repeat prevalidation

Date: 2026-09-12  
Evidence stage: `frozen-reviewed`; provider calls are enabled only for this exact admission

## Frozen observation

This is one reliability repeat of the already frozen R02 responsive-report
case. It does not change or refreeze `realistic_matrix_v11.json`, the R02
fixture, hidden verifier, Browser bridge, candidate instructions, or tool
contract.

| Field | Frozen value |
|---|---|
| Configuration | `realistic.claudesonnet5.medium.anchor.r02.repeat1.v11` |
| Model | `openrouter/anthropic/claude-sonnet-5` |
| Effort / provider | `medium` / `anthropic` |
| Case | `R02` |
| Actions | 18 maximum |
| Tokens | 75,000 total; 20,000 generated |
| Spend | $0.35 maximum for this generation |
| Retries / fallbacks / owner interventions | 0 / 0 / 0 |

The generation-owned admission record is
`server/internal/agentqualification/realistic_r02_repeat_admission_v1.json`.
Its status is `frozen_reviewed` and `provider_calls_allowed` is true. The new
$0.35 envelope only tightens V11's older $5.00 campaign ceiling in the plan;
the frozen V11 file remains byte-for-byte unchanged.

## Evidence admission

The plan hash-binds the accepted R01, R03, R04, and R02 Browser evidence from
their original immutable roots:

- R01: `realistic-v14`, accepted PASS receipt.
- R03: `realistic-v17`, immutable original FAIL retained; the later accepted
  post-batch semantic correction is explicit and no provider result is
  rewritten or fabricated.
- R04: `realistic-v18`, accepted PASS receipt.
- R02 Browser: `realistic-v23`, accepted PASS receipt with narrow, medium, and
  wide proof, 47 total, four cards, and no horizontal overflow.

This replaces the unusable assumption that one fresh campaign ledger must
contain synthetic V10 rows. No legacy V10 prerequisite is created.

## Stop behavior

- Global halt only: credential failure, uncontrolled spending, unsafe external
  action, or evidence corruption that makes further results unusable.
- Candidate-local failure: competence failure, hard disqualifier, or response
  / tool protocol failure.
- Observation-local invalid, no retry: non-safety harness imperfection or
  provider transport imperfection.

The admission record is the complete review surface for these
generation-specific rules. The single independent review and consolidated
correction are complete, so the exact frozen observation is now activated.

## Provider-free proof

```powershell
cd C:\Dev\SMK-multica-selfhost-pilot\server
go test ./internal/agentqualification ./cmd/screening-run -run 'R02Repeat|RealisticFreeze|ReliabilityBrowser|RealisticMatrix|RealisticOpenCodeActionPlanAllowsOnlyR02|RealisticBrowser' -count=1
go run ./cmd/screening-run --realistic-r02-repeat-plan
go vet ./internal/agentqualification ./cmd/screening-run
```

Expected plan facts after activation: all accepted evidence hashes validate,
complete frozen foundation validation passes, the exact V11 tuple is returned,
provider readiness is true, and the plan command itself makes no provider call.

## Independent review and consolidated correction receipt

The one independent review returned `NO-GO`. No provider was called and no
accepted artifact was rewritten. Its three findings were corrected together:

1. R03 remains the immutable original `candidate_fail` / `FAIL`, while
   admission now loads its hash-bound original controller result and actually
   reruns the corrected frozen declaration verifier against that final summary.
2. The paid command is now a dedicated R02-repeat path. Its first operation is
   `RequireActiveR02RepeatAdmission`; it then consumes only
   `R02RepeatBoundedMatrix`, a dedicated campaign ledger, and the exact admitted
   candidate/case. The generic historical campaign prerequisite and its $5
   ceiling cannot admit this run.
3. Every prerequisite provider receipt is now independently hash-checked,
   decoded, and verified as Claude Sonnet 5 / medium over direct OpenRouter with
   Anthropic as the sole requested upstream and fallbacks disabled. Token and
   cost totals are recomputed from raw receipts and reconciled with both the
   execution receipt and controller.

The dedicated recorder maps ordinary competence and protocol failures to the
candidate only, transport or non-safety harness defects to this observation
only, and only credentials, uncontrolled spend, unsafe external action, or
evidence corruption to a global halt. It enforces 18 actions, 75,000 total
tokens, 20,000 generated tokens, $0.35, no retries, no fallback, and no owner
intervention. Model, provider, receipt hash, accounting, cap, and corrected R03
semantic mutations are covered by focused tests.

Per instruction, this is the single consolidated correction pass and there is
no second reviewer cycle. The exact corrected admission is now activated; no
broader matrix, case, grader, model, or retry authority changed.

## Post-batch Browser capability correction

The R02 repeat reached `awaiting_browser_evidence` with eight exact provider
calls, 24,792 input tokens, 507 output tokens, 180 reasoning tokens, and
$0.068849 cost. The current ChatGPT in-app Browser control surface could not
create the frozen 390/900/1440 viewports or capture the required DOM evidence,
and its security policy prohibited raw CDP or workaround access. The frozen
Browser contract therefore could not be completed.

The observation is sealed as `invalid/non_safety_harness_imperfection`, not
scored against Claude. Its original run state, eight provider receipts, 25,479
cumulative tokens, exact cost, and final workspace tree remain hash-bound. The
administrative correction made no provider call and performed no retry.

An activated-test defect also caused the former provider-lock-only unit test
to enter the executor twice before interruption. They remain separate:

- `...-10adde92fbac094a`: seven completed calls; 20,716 input tokens including
  cache, 502 output, 225 reasoning; $0.0590565.
- `...-61d0c2e3fd5d1cb9`: seven completed receipted calls; 20,152 input, 468
  output, 521 reasoning; $0.060269, plus one eighth pending session that
  recorded zero tokens and zero cost before interruption.

Total accidental cost was $0.1193255. All 14 completed OpenCode session IDs are
preserved in the post-batch correction receipt, and the second interrupted
run-state is path- and hash-bound. None of that accounting was merged into the
intended R02 result. The unsafe test was replaced by a provider-free
admission/envelope assertion.
