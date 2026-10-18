# Item 14 Claude Subscription Realistic v37 — Result

Status: **complete two-observation R03 pass; testing only**.

The separately versioned phased controller tested the same Claude Code
subscription `claude-sonnet-4-6` Medium configuration on the unchanged frozen
R03 v2 case. The first observation and one exact reliability repeat both
passed with zero retries in separate fresh evidence roots.

| Observation | Result | Calls | Input | Output | Cost-equivalent | Retries | Evidence root |
|---|---|---:|---:|---:|---:|---:|---|
| v37 run 1 | PASS | 3 | 16,465 | 2,153 | 92,159 micros | 0 | `.qualification-evidence/claude-subscription-realistic-v37` |
| v37 repeat 1 | PASS | 4 | 22,185 | 2,627 | 94,675 micros | 0 | `.qualification-evidence/claude-subscription-realistic-v37-repeat-01` |

Both observations used the same frozen case, configuration, deterministic
checks, grader, and phased-controller definition. They are separate executions
with separate raw receipts, runtime state, execution receipts, results, and
ledgers. The provider-reported cost-equivalent values are subscription
accounting metadata, not marginal cash spend.

For the narrow controller comparison, v36 used the same model, effort, and R03
case through its one-action controller. It failed the resource ceiling after
10 calls, 57,508 input, 2,371 output, 206,700 cost-equivalent micros, and zero
retries. V37 therefore supplies two successful observations that materially
reduced provider calls and input use while preserving the frozen business
outcome and evidence gates. This is evidence for the phased controller on R03;
it is not a general model, role, topology, or production selection.

Evidence bindings:

- V37 freeze SHA-256: `6cd1d07a662750061e6e0e218c07fcfd11d35aa5c1482a4c9b665b0df72422c5`
- V37 definition SHA-256: `dc86053057f4a57c17e9acac67561a5c4d62f60573744f966a9498dabece0677`
- Run 1 ledger SHA-256: `87f2188c99b12adc2becf2f53320f408e3f9262e544304cffed5845fbc96c20f`
- Repeat ledger SHA-256: `573c7e771699fa2a0c6dcc7d3e684e474ce32f6caa260cbcd2727a6f6619f018`
- Both grades SHA-256: `4d8631dbde111224e41ee37942bff11692e8eb9f123a0488d3545c3248e0bf3a`
- Run 1 result SHA-256: `ea32d75c32b7c1953f27bbdb3212fbeed4606384d3d2027c5408613d42140cef`
- Repeat result SHA-256: `829314ccb7c293d7640385f81f1c76c9f1b0577a194c408cd5c6e3b37f0e7578`

No role, squad, permanent model, permission, deployment, production, Browser,
outbound-message, or Phone Farm selection or action occurred.
