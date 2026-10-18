# Item 14 second screening result

Date: 2026-09-11 17:56 ET  
State: `stopped_invalid_transport`  
Evidence lane: one isolated local Codex candidate run; no production action,
external message, repository checkout, or Phone Farm operation

## Result

The corrected response-shape contract worked, but the direct issue-assignment
transport did not place the issue description in the model prompt. The runtime
prompt was 620 bytes and told the candidate to fetch `SILV-34` with a tool.
The candidate's read-only contract correctly prohibited tools, so it returned:

```json
{"decision":"insufficient_evidence","reason":"No case facts or response contract were supplied."}
```

The full packet remained visibly present in the Multica issue. This proves a
transport mismatch between issue storage and the isolated no-tools prompt, not
a Terra capability result. The batch stopped immediately. Luna was not
launched, Terra was not retried, no grader ran, and zero configurations were
qualified. `SILV-34` was moved to `Cancelled` without starting another run.

## Exact receipt

| Field | Value |
|---|---|
| Issue | `SILV-34` / `01a09278-e86b-7464-9c6f-91a719501efa` |
| Run | `01a09278-e882-7ed7-9dea-b65a3df1ac2a` |
| Agent | `3298a469-0de3-474e-aa64-bc6876f16289` |
| Reported model | `gpt-5.6-terra` |
| Started | `2026-09-11T21:56:33Z` |
| Completed | `2026-09-11T21:56:43Z` |
| Fresh input | 1,408 tokens (`12,416 input - 11,008 cache read`) |
| Cached input | 11,008 tokens |
| Output | 282 tokens |
| Counted total | 12,698 tokens |
| Owner interventions | 0 |

## Correction

The current v3 batch pins `comment_trigger_inline_v1`. The daemon's
comment-trigger prompt embeds the complete triggering comment directly, so an
isolated candidate receives the case facts without reading the issue or using
tools. A focused regression test proves the T11 owner message, case id, and
response contract are inline; the direct-assignment prompt is also proven not
to contain them.

V1 and v2 are now rejected by the same fail-closed launch gate. V3 is defined
but has not launched. No more paid work is allowed until the operator creates a
non-running issue first, uses the exact candidate packet as the triggering
comment, verifies one and only one run was created, and takes a fresh account
capacity reading.

