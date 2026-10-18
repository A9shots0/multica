# Item 14 third screening result — invalidated comparison

Date: 2026-09-11 18:03-18:07 ET  
State: `complete_invalid_harness`  
Evidence lane: two isolated local Codex candidate runs; no production action,
external message, repository checkout, tool call, or Phone Farm operation

## Corrected outcome

| Candidate | Model verdict | Why |
|---|---|---|
| Luna medium | INVALID / not scored | Its response happened to use the hidden grader's preferred field types, but the candidate received conflicting public response instructions |
| Terra medium | INVALID / not scored | The grader failed three field types that the public case packet never disclosed, while the assigned skill simultaneously required a different field set |

Neither run can qualify, eliminate, or rank a model. The paid execution and
usage receipts remain valid cost evidence, but V3 is not a valid model
comparison.

## Harness defect

The candidate instruction said to follow the case `response_contract`. That
public T11 contract named eight fields but did not declare their JSON types.
The assigned `Item 14 Qualification Evidence v1` skill separately required an
incompatible eleven-field response. The hidden grader then required arrays for
three T11 fields even though those types were never disclosed publicly.

Hash pinning proved which instruction, skill, and packets ran. It did not prove
that their semantic contracts agreed. Therefore:

- Luna following the packet instead of the skill conflict is not a valid pass.
- Terra returning strings instead of undisclosed arrays is not a valid failure.
- The earlier claim that Luna advanced and Terra failed is retracted.

## Durable receipts

| Field | Luna | Terra |
|---|---:|---:|
| Issue | `SILV-35` | `SILV-36` |
| Run | `01a0927f-9e49-7e46-854b-242b10615260` | `01a09282-4c05-70e7-8b4b-0283f59d1183` |
| Fresh input | 2,572 | 2,081 |
| Cached input | 9,984 | 11,008 |
| Output | 827 | 766 |
| Counted total | 13,383 | 13,855 |
| Started | `2026-09-11T22:03:53Z` | `2026-09-11T22:06:49Z` |
| Completed | `2026-09-11T22:04:11Z` | `2026-09-11T22:07:07Z` |

Batch cost was 27,238 counted tokens. The authoritative weekly account reading
moved from 52% to 53%. There were zero retries, owner interventions, tools, or
external effects. Both evidence issues remain Done and Unassigned because the
runs themselves completed; their Done state is not a qualification verdict.

The exact identities, output hashes, invalid verdicts, usage, and capacity
readings are sealed in
`server/internal/agentqualification/screening_batch_v3_result.json`.

## Correction

V4 uses a new skill whose response rule explicitly defers to the public case
packet. The T12 public packet declares every field and JSON type, and its hidden
grader is regression-tested to enforce only disclosed structure. The
compatibility gate now rejects the V3 skill/packet combination.

V4 remains `planned_not_launched`. Zero configurations are production-qualified
and V3 provides no basis for selecting Luna over Terra.
