# Item 14 Claude Subscription Static Reliability v35 — Result

Status: **complete static reliability repeat; testing only**.

Claude Code subscription `claude-sonnet-4-6` at medium effort repeated the exact frozen v34 six-case sequence and passed T12 v4, T11 v4, T08 v3, T16 v3, T04 v1, and T09 v2 again.

| Result | Calls | Input | Output | Provider cost-equivalent | Retries |
|---|---:|---:|---:|---:|---:|
| 6/6 PASS | 6 | 35,645 | 4,721 | 156,403 micros | 0 |

The cost figure is provider-reported metadata under the Claude subscription, not marginal cash spend. `candidate_eliminated=false` and `global_halt=false`.

Evidence:

- Freeze SHA-256: `b37d6246a1a3f998e5373f7baadaace2fcc8408a8b040a40074b8481a118b4ee`
- Ledger SHA-256: `8ed67c98c885cb9468c8d9d7146a8d8395401ec7c567fea10ba0470d2304b439`
- Evidence root: `.qualification-evidence/claude-subscription-static-v35`
- Provider-free replay: valid, zero provider calls

All six parsed candidate-response hashes exactly match v34. The raw receipts and usage differ, as expected for separate provider invocations. This is perfect observed repeat consistency for the identical frozen static packets and graders. It does not prove realistic action competence, tool use, recovery in a live workflow, or suitability for a production role.

No model, role, squad, topology, permission, or production configuration is selected. Next: evaluate which realistic Claude subscription test is scientifically justified by the remaining documented gaps; do not promote from static evidence alone.
