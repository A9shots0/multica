# Item 14 topology-v8 R04 result

Status: complete, mixed descriptive topology evidence. This batch selects no
topology, model, role, squad, or production permission.

Topology-v8 ran the unchanged frozen R04 v1
`synthetic-exact-once-resume-v1` case with GPT-5.4 Medium as executor and
Claude Sonnet 5 Medium as the independent reviewer. It completed two
counterbalanced Direct/Reviewed pairs with zero retries, zero owner
interventions, and zero external side effects.

| Observation | Pair | Arm | Result | Calls | Tokens | Cost | Time |
|---|---:|---|---|---:|---:|---:|---:|
| `v8-r04-p1-direct` | 1 | Direct | FAIL — `competence_failure` | 11 | 25,745 | $0.105927 | 105.023s |
| `v8-r04-p1-reviewed` | 1 | Reviewed | INVALID — `transport_or_harness_imperfection`; no retry | 20 | 59,377 | $0.232759 | recovered; exact stage receipts |
| `v8-r04-p2-reviewed` | 2 | Reviewed | FAIL — `competence_failure` | 11 | 32,476 | $0.132352 | 110.390s |
| `v8-r04-p2-direct` | 2 | Direct | FAIL — `competence_failure` | 9 | 19,189 | $0.083000 | 81.230s |

Exact paid stage receipts:

| Observation | Stage | Calls | Tokens | Cost | Time |
|---|---|---:|---:|---:|---:|
| p1 Direct | Execute | 11 | 25,745 | $0.105927 | 104.877s |
| p1 Reviewed | Execute | 11 | 25,825 | $0.109641 | 99.884s |
| p1 Reviewed | Review 1 | 1 | 11,111 | $0.045552 | 32.899s |
| p1 Reviewed | Correction | 8 | 22,441 | $0.077566 | 61.493s |
| p2 Reviewed | Execute | 10 | 22,887 | $0.100857 | 94.868s |
| p2 Reviewed | Review 1 | 1 | 9,589 | $0.031495 | 15.152s |
| p2 Direct | Execute | 9 | 19,189 | $0.083000 | 80.971s |

Pair one is not a valid topology comparison because its Reviewed observation
is invalid. Its exact receipts are retained, but it is excluded and was not
retried. Pair two is valid: Direct and Reviewed both failed for competence.
The reviewer added two calls, 13,287 tokens, $0.049352, and 29.160 seconds
without changing the outcome. The reviewer therefore did not rescue R04, and
this batch shows no topology advantage.

Campaign total: 51 calls, 136,787 tokens, and $0.554038. Accounting is exact.

## Post-call harness record

Two harness imperfections were found after paid work and corrected without any
retry or evidence rewrite:

- p1 Reviewed used exactly all eight allowed correction calls and returned an
  invalid controller state, but the stage validator initially recognized only
  an over-cap value as stage exhaustion. Exact call-ceiling exhaustion is now
  observation-local invalid.
- R04 run-family binding intentionally gives each observation a unique initial
  tree. The pair-integrity check initially compared that run-bound tree and
  falsely set a global evidence-corruption halt even though canonical task and
  fixture hashes matched. Pair integrity now uses those canonical hashes. The
  false halt alone was cleared; the p1 result and receipts remained unchanged.

Canonical machine evidence:

- `.qualification-evidence/topology-v8/campaign.json`
- `.qualification-evidence/topology-v8/observations/*/result.json`
- `.qualification-evidence/topology-v8/observations/*/run-state.json`
- `.qualification-evidence/topology-v8/observations/*/hidden-grade.json` where produced

No topology advantage, production model, or role was selected.
