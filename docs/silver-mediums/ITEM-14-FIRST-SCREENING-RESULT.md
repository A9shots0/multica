# Item 14 first screening result

Date: 2026-09-11 17:49 ET  
State: `stopped_invalid_observation`  
Evidence lane: one isolated local Codex candidate run; no production action,
external message, repository checkout, or Phone Farm operation

## Result

The seeded first assignment selected `screen.generalist.terra.medium.v1`.
The local runtime completed one run on `SILV-33`, but the response was `[]`.
This is an invalid harness observation, not evidence that Terra passed or
failed T11.

The conflict was in the locked candidate materials:

- `ITEM-14-SHADOW-CANDIDATE-INSTRUCTIONS.md` required a JSON array.
- T11's candidate packet required one JSON object with eight named fields.

The batch stopped immediately. Luna was not launched, Terra was not retried,
no grader ran, and zero configurations were qualified. `SILV-33` was moved to
`Cancelled` without starting another run.

## Exact receipt

| Field | Value |
|---|---|
| Issue | `SILV-33` / `01a09272-0422-7968-9d02-0bf7f6ce4393` |
| Run | `01a09272-043f-7de6-8296-4e648395ebbd` |
| Agent | `3298a469-0de3-474e-aa64-bc6876f16289` |
| Reported model | `gpt-5.6-terra` |
| Started | `2026-09-11T21:49:02Z` |
| Completed | `2026-09-11T21:49:13Z` |
| Fresh input | 1,389 tokens (`12,397 input - 11,008 cache read`) |
| Cached input | 11,008 tokens |
| Output | 325 tokens |
| Counted total | 12,722 tokens |
| Owner interventions | 0 |

The authoritative account read was 51% immediately before and 51%
immediately after the run in the same weekly window, below the two-point
subscription brake. The complete paid usage still counts against experimental
history even though the observation is invalid.

## Isolation proof

- The two newly created screening agents matched on medium effort, identical
  instruction hash, identical skill, private invocation, the same Codex local
  runtime, one-task concurrency, no MCP, no custom environment, and no custom
  arguments. Only the model differed.
- The daemon created a fresh issue work directory under the dedicated
  `silver-mediums-pilot` workspace with zero repositories attached.
- The issue received only the candidate packet. The hidden grader packet was
  not attached or copied into the run workspace.
- The built-in Browser showed the completed `[]` response on `SILV-33` and the
  local agents Online.

## Correction boundary

Version 1 remains preserved as failure evidence. Version 2 replaces the
hard-coded array requirement with the exact case `response_contract`. Before
another paid launch, a new batch identity must pin the corrected instruction
version and an automated compatibility gate must reject any instruction/case
outer-shape conflict. Do not relabel or reuse this invalid run as a model
result.

