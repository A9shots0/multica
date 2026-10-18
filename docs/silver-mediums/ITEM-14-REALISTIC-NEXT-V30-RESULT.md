# Item 14 realistic next capability batch v30 — halted result

Date: 2026-09-13

Outcome: valid global halt on the frozen per-observation token budget. No role,
squad, topology, permission, or production configuration was selected.

## Result

| Candidate | Case | Result | Failure | Calls | Tokens | Cost | Retry |
|---|---|---|---|---:|---:|---:|---:|
| Claude Sonnet 5 Medium, OpenRouter requested Anthropic | R05 | Invalid; does not advance | Frozen 75,000-token observation ceiling exceeded | 20 | 92,508 | $0.321781 | 0 |
| GPT-5.4 Medium, OpenRouter requested OpenAI | R05 | Not run | Global budget halt occurred first | 0 | 0 | $0 | 0 |
| Both candidates | R06 | Not run | R05 stage did not complete | 0 | 0 | $0 | 0 |

Token detail for the one observation: 80,439 input, 3,512 output, and 8,557
reasoning tokens. The model reached the 20-action ceiling after repeatedly
rewriting the declarative policy. It eventually passed the harness-owned apply
check, failed the harness-owned verify check, and honestly finished without
claiming success. The controller then classified the observation as invalid
for exceeding the frozen total-token reservation. This is not a valid
competence score and is not retried.

## Evidence integrity

- Campaign status: `global_halt`; reason: `budget_breach`.
- Campaign result SHA-256:
  `0e32bf1eb6b3be18372599135708cece7318dd8296cdeb79b9cdae2b14ab6446`.
- Sealed observation receipt SHA-256:
  `f17db2c3deef1b35341189922fde6d77429ff032c0fdf9cc8621a389fa05d95c`.
- Evidence root: `.qualification-evidence/realistic-next-v30`.
- Reopening the completed campaign validated the sealed result and returned
  without new provider work.
- Post-run focused tests and vet passed.

The batch performed no production, Browser, outbound-message, owner, or Phone
Farm action. The v30 fixtures, raw responses, claim, workspace, and receipts
remain immutable. The unobserved GPT binding must not be described as a pass or
failure.

## Post-batch correction note

V30 correctly obeyed its frozen rule that any observation-level token breach
halts the campaign. The result also exposes that this rule is too broad for
future batches: a candidate exhausting its own reserved observation allowance
is a candidate-local failure, while an actual campaign spend/token cap breach
must remain a global halt. Any later matrix may make that distinction, but v30
itself is not changed or resumed.
