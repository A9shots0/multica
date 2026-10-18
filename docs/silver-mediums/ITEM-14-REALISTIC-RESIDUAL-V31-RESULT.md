# Item 14 realistic residual capability batch v31 — complete result

Date: 2026-09-13

Outcome: the one still-unobserved GPT-5.4 Medium binding produced a valid R05
failure. R06 was then skipped by the frozen advancement rule. No model, role,
squad, topology, permission, or production configuration was selected.

## Result

| Candidate | Case | Result | Failure | Calls | Tokens | Cost | Artifact wall time | Retry |
|---|---|---|---|---:|---:|---:|---:|---:|
| GPT-5.4 Medium, OpenRouter requested OpenAI | R05 | Failed; does not advance | Response/tool protocol failure | 20 | 64,706 | $0.211099 | 166.061s | 0 |
| GPT-5.4 Medium, OpenRouter requested OpenAI | R06 | Skipped | Candidate did not produce a valid R05 pass | 0 | 0 | $0 | — | 0 |

Token detail: 60,760 input, 414 output, and 3,532 reasoning tokens. The model
used all 20 permitted actions reading the sealed workspace, repeatedly returned
to `expected/final-state.json`, never edited the required files, never ran the
allowlisted simulator checks, and reached the bounded step ceiling. The
starting and final workspace hashes are identical. This is valid evidence of a
failure under this exact action protocol; it is not evidence that the provider
transport failed.

## Evidence integrity

- Campaign status: `complete`.
- Campaign result SHA-256:
  `e0899c9a7ddc360b8eed3071b7858a663d083b0f08e2035ea5f40dcc8fcdc2e5`.
- R05 sealed receipt SHA-256:
  `2db93bfa3a68e3811a719f8ea07f2f9101390698daa6298070a6b6e5b4b5d529`.
- Freeze SHA-256:
  `2d2b80b0b8eac1bbf5719a9bf671a94574327dd8c83185f16da4503578b5cf2c`.
- Evidence root: `.qualification-evidence/realistic-residual-v31`.
- Reopening the completed campaign preserved the exact result hash and made no
  additional provider calls.
- Post-run focused tests and `go vet` passed.

The batch made no production, Browser, outbound-message, owner, or Phone Farm
action. Claude's v30 observation remains unchanged and invalid/unscored; it was
not retried merely because subscription usage reset.

## Interpretation boundary

R05 now contains two materially different observations: Claude Sonnet 5 Medium
exceeded v30's frozen reservation after attempting edits and checks, while
GPT-5.4 Medium exhausted the action protocol through repeated reads. Neither
reached R06. These results support investigating the interaction between model
behavior and the constrained action protocol before spending on more repeats;
they do not justify changing the frozen case, tuning its grader to accept an
observed answer, or selecting a production role.
