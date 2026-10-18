# Item 14 topology-v4 result

Status: halted, incomplete replication evidence. Preserve the campaign and raw
receipts exactly; do not use v4 to select a topology, model, role, or squad.

Topology-v4 attempted the same frozen R03 v2 Direct/Reviewed comparison with
GPT-5.4 Medium through the exact OpenAI route. Pair one Direct completed and
passed. Pair one Reviewed stopped after the executor produced two concatenated
JSON actions, a candidate-local `response_or_tool_protocol_failure`.

The frozen v4 runner then exposed a separate harness defect: it continued a
terminally failed executor into review, and the reviewer planner compared the
new GPT-5.4 reviewer identity against the older topology-v1 Claude definition.
That pre-call rejection was incorrectly recorded as `evidence_corruption` and
halted the campaign. No reviewer call occurred.

| Observation | Arm | Recorded outcome | Scientific interpretation | Calls | Tokens | Cost | Time |
|---|---|---|---|---:|---:|---:|---:|
| `v4-r03-p1-direct` | Direct | PASS | Valid PASS | 12 | 32,468 | $0.130188 | 112.609s |
| `v4-r03-p1-reviewed` | Reviewed | `global_halt/evidence_corruption` | Executor validly failed protocol; subsequent halt classification is a harness defect | 6 | 13,131 | $0.042393 | 54.600s |
| `v4-r03-p2-reviewed` | Reviewed | Not run | Precluded by the preserved global halt | 0 | 0 | $0 | - |
| `v4-r03-p2-direct` | Direct | Not run | Precluded by the preserved global halt | 0 | 0 | $0 | - |

Campaign total: 18 provider calls, 45,599 tokens, $0.172581, and 167.209
seconds of summed observation time. Accounting is exact. The result is not a
valid matched-pair topology comparison.

The next fresh harness version must make two bounded corrections before any
provider call:

1. A terminal executor protocol/competence failure ends only that observation;
   it must not invoke a reviewer or halt the batch.
2. The reviewer plan must validate against the current frozen experiment's
   reviewer identity, not topology-v1.

Canonical machine evidence:

- `.qualification-evidence/topology-v4/campaign.json`
- `.qualification-evidence/topology-v4/observations/v4-r03-p1-direct/result.json`
- `.qualification-evidence/topology-v4/observations/v4-r03-p1-reviewed/result.json`
- `.qualification-evidence/topology-v4/observations/v4-r03-p1-reviewed/run-state.json`
