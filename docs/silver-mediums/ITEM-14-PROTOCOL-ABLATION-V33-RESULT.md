# Item 14 — structured-output protocol ablation v33 result

Date: 2026-09-13

Status: complete; no candidate or protocol advances. No model, agent, role,
squad, topology, permission, or production configuration was selected.

Freeze SHA-256: `c99506b60f225de2f2b91563698657226100aa8252ba0c8633d60d0711cf155f`  
Final ledger SHA-256: `e1f336da03c2c62419911fde6196887b6278d828471bd39b5791aefcd790b835`

## Result

| Candidate | Protocol | Result | Exact reason | Recorded provider turns | Input | Output | Reasoning | Cost | Time |
|---|---|---|---|---:|---:|---:|---:|---:|---:|
| Claude Sonnet 5 Medium | one-action loop | Failed | OpenCode returned `StructuredOutputError`; zero retry | 1 | 4,763 | 808 | 465 | $0.024637 | 12.874s |
| Claude Sonnet 5 Medium | one-shot plan | Invalid / unscored | Client-aborted transport timeout; provider settlement remains uncertain | 0 | 0 | 0 | 0 | $0 recorded | ~14.376s |
| GPT-5.4 Medium | one-action loop | Failed | OpenCode returned `StructuredOutputError`; zero retry | 1 | 2,776 | 1,066 | 2,070 | $0.053980 | 24.265s |
| GPT-5.4 Medium | one-shot plan | Failed | OpenCode returned `StructuredOutputError`; zero retry | 1 | 2,781 | 1,430 | 1,996 | $0.058343 | 25.654s |

Recorded batch total: 3 billed provider turns, 10,320 input tokens, 3,304
output tokens, 4,531 reasoning tokens, and $0.136960. The Claude one-shot
timeout has zero recorded usage/cost and is excluded from those totals; its
provider settlement remains uncertain. The final ledger records
`global_halt=false`.

## Interpretation boundary

The completed responses looked substantively useful: they identified the
owner-decision authority, bounded scope, required document reconciliation,
verification checks, and the prohibition on unsupported deployment or Live
claims. However, each completed response violated the exact structured-output
contract delivered through the frozen OpenCode server request. Useful prose or
differently shaped JSON is not a schema-valid result and therefore cannot be
scored as a pass.

The Claude one-shot timeout is transport-invalid and supplies no competence or
protocol conclusion. The three completed cells are valid zero-retry protocol
failures for their exact model, effort, route, case, and interface. With no
valid R05 pass, neither candidate nor protocol advances and R06 remains skipped
under the existing frozen advancement rule.

## Evidence boundary and next step

The immutable final root is
`.qualification-evidence/protocol-ablation-v33-continuation-03`. No production,
Browser, outbound, owner, or Phone Farm action occurred. No additional review
or provider call was made during documentation.

The existing plan does not authorize R06 without a valid R05 pass. The next
recorded testing step is to scope the next smallest bounded realistic group
from the remaining documented capability gaps; do not search more long-tail
models or select production roles.
