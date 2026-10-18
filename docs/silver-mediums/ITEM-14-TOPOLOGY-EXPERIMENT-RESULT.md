# Item 14 execution-topology experiment result

Date: 2026-09-12  
Status: completed with one valid matched pair and three preserved invalid observations  
Decision boundary: testing evidence only; no topology, model, role, squad, or production permission selected

## Result

| Observation | Arm | Result | Recorded calls | Recorded tokens | Reconciled cost | Time | Interpretation |
|---|---|---|---:|---:|---:|---:|---|
| R03 pair 1 | Direct | INVALID | 10 | 38,413 | $0.101370 | 74.1s | Pre-call reservation was incorrectly treated as actual arm spend |
| R03 pair 1 | Reviewed | INVALID | 12 | 49,311 | $0.140901 | 109.6s | New reviewer transport name was rejected before any reviewer call |
| R01 pair 1 | Reviewed | PASS | 10 | 37,786 | $0.116310 | 95.7s | Executor passed; one independent review call also passed |
| R01 pair 1 | Direct | PASS | 10 | 33,290 | $0.091982 | 73.1s | Direct executor passed the identical frozen case |
| R03 pair 2 | Reviewed | INVALID | at least 13 durable | at least 54,883 durable | $0.169352 | not comparable | Initial executor crossed its 60k reservation on a paid response that the wrapper failed to seal; no review occurred |
| R03 pair 2 | Direct | PASS | 12 | 49,094 | $0.140365 | 97.5s | Direct executor passed the frozen R03 case |

Campaign ledger: 54 fully attributed calls, 207,894 fully attributed tokens,
and $0.760280 reconciled provider spend. The R03 pair-two reviewed row retains
its durable 13-call partial controller receipt separately; the provider-spend
delta includes the unsealed final paid response. It is excluded from token,
call, time, and outcome comparisons.

## What the valid evidence says

R01 is the only valid matched pair. Both arms passed. Review added 4,496 tokens,
$0.024328, and 22.6 seconds without changing the verified outcome. This is one
clean-path overhead observation, not evidence that review is generally useless.

No R03 pair is valid. Pair one was invalidated by two unrelated harness defects;
pair two lost the reviewed-arm accounting boundary before review began. The
experiment therefore cannot measure whether review improves regression and
rollback outcomes. No topology winner can be named.

## Preserved evidence

- Campaign ledger: `.qualification-evidence/topology-v1/campaign.json`
- Per-observation immutable results and corrections:
  `.qualification-evidence/topology-v1/observations/`
- Frozen definition and hashes:
  `server/internal/agentqualification/topology_experiment_v1.json` and
  `server/internal/agentqualification/topology_freeze_v1.json`

No observation was retried. No owner intervention, external side effect,
production action, Browser action, or Phone Farm action occurred.

## Required correction before reuse

The stage-budget wrapper must never discard a provider response after it has
been paid. An over-ceiling response must be sealed and counted first, then end
only that observation with exact accounting. This is an infrastructure fix,
not a grader or case change. The frozen historical records remain unchanged.

