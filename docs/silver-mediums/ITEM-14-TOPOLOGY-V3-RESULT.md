# Item 14 topology-v3 result

Status: completed `local-qualification-evidence`; four of four frozen R03
observations executed with zero retries, zero owner interventions, and zero
external side effects. This result does not select a topology, model, role, or
squad.

The comparison used Claude Sonnet 5 Medium through the exact Anthropic route.
Pair one ran Direct then Reviewed; pair two ran Reviewed then Direct. Reviewed
allowed one correction but neither reviewed observation used one.

| Observation | Pair | Arm | Outcome | Failure | Calls | Tokens | Cost | Time |
|---|---:|---|---|---|---:|---:|---:|---:|
| `v3-r03-p1-direct` | 1 | Direct | PASS | - | 13 | 54,083 | $0.150565 | 101.517s |
| `v3-r03-p1-reviewed` | 1 | Reviewed | FAIL | `competence_failure` | 14 | 63,015 | $0.185674 | 126.228s |
| `v3-r03-p2-reviewed` | 2 | Reviewed | PASS | - | 13 | 61,020 | $0.178752 | 121.735s |
| `v3-r03-p2-direct` | 2 | Direct | PASS | - | 12 | 48,680 | $0.139181 | 96.902s |

Campaign totals: 52 provider calls, 226,798 tokens, $0.654172, and 446.382
seconds of summed observation time. Accounting is exact.

## Matched-pair interpretation

- Pair one: Direct passed and Reviewed failed. Reviewed used one more call,
  8,932 more tokens, $0.035109 more, and 24.711 seconds more.
- Pair two: both arms passed. Reviewed used one more call, 12,340 more tokens,
  $0.039571 more, and 24.833 seconds more.
- Across the two pairs, Direct passed two of two and Reviewed passed one of
  two. Reviewed added 2 calls, 21,272 tokens, $0.074680, and 49.544 seconds.
- Two matched pairs are descriptive evidence, not enough to select a topology.

## Topology-v2 calibration record

Topology-v2 stopped after two of four observations. Its Direct observation
passed at 12 calls, 49,046 tokens, $0.137883, and 100.090 seconds. Its Reviewed
observation halted as `uncontrolled_spending` at 13 calls, 60,990 tokens,
$0.181220, and 144.844 seconds. The paid first review used 12,113 tokens, above
the predeclared 12,000 reviewer-stage ceiling even though the observation was
well below its 120,000-token arm ceiling. The campaign therefore halted at 25
calls, 110,036 tokens, and $0.319103. V3 corrected only that invalid budget
geometry before collecting four fresh observations; v2 remains preserved and
is not treated as topology evidence.

Canonical machine evidence:

- `.qualification-evidence/topology-v3/campaign.json`
- `.qualification-evidence/topology-v3/observations/*/result.json`
- `.qualification-evidence/topology-v2/campaign.json`
- `.qualification-evidence/topology-v2/observations/*/result.json`
