# Item 14 fifth capped screening batch

Date: 2026-09-11  
State: `dry_run_valid_no_go`; no paid model call

## Purpose

V5 separates cheap model capability screening from full Multica workflow
testing. The candidate receives one direct prompt containing the versioned
policy and complete public T12 packet. Its final JSON is captured from Codex
JSONL directly. It does not need to read an issue, load a workspace skill from
disk, stage a file, or post a comment.

## Locked comparison

| Setting | Value |
|---|---|
| Batch | `screen-blast-radius-t12-codex-v5` |
| Case | T12 v1 |
| Candidates | Luna medium and Terra medium |
| Difference | model only |
| Transport | `direct_codex_exec_jsonl_v1` |
| Prompt SHA-256 | `b6ce827db58f2b6a181a32b0071792da026cb0496efaa5b57f67c417bfb1f34c` |
| Output-schema SHA-256 | `9fa32e64810da3de76745cfbbc23684b0b4c65ce87395ad73050d0c39511dc4c` |
| Prompt size | 5,106 bytes |
| Seeded first candidate | Terra medium |
| Per-launch reservation | 35,000 counted tokens |
| Batch ceiling | 70,000 counted tokens |
| Retries / owner interventions | zero / zero |

The higher reservation is not a target. It is a fail-closed bound based on the
known Codex base-instruction overhead and must be replaced by measured direct
turn usage after the first valid observation.

## Plan-only runner contract

`server/cmd/screening-run` builds and hashes the exact direct prompt, schema,
model, effort, and seeded candidate without invoking a model. Paid execution is
intentionally disabled. Passing `--execute` fails closed until a separate
executor proves all of these before it can exist:

- it consumes `PrepareNextScreeningLaunch`, the single authoritative capacity,
  ledger, seeded-order, repeat, and prior-failure admission path;
- it prevents tools and external effects rather than detecting them afterward;
- it passes only a minimal allowlisted environment and cannot mutate the
  owner's credential file;
- it writes an atomic receipt binding provider/model version, requested and
  reported configuration, account lane, timestamps, thread/run identity,
  immutable raw-trace hash, final output, exact usage, and admission evidence.

Any command, file, MCP, web, computer, or other tool event makes the
observation `INVALID`. Missing output, malformed JSON, missing usage, broken
token accounting, or a reservation overrun also invalidates it. The runner
never prints credentials or copies the owner's config, skills, memories,
plugins, or project instructions.

## Proven dry run

The zero-model dry run completed with `paid_model_called: false` and
`paid_execution_enabled: false`, selected the
seeded Terra configuration, reproduced both hashes above, and passed focused
tests for prompt isolation, schema generation, valid one-turn parsing,
tool-event rejection, raw-trace hashing, and fail-closed paid-execution refusal.

## Paid boundary

The independent V5 review is `NO-GO` for paid work until the three executor
gates above are implemented and reviewed. Once they pass, admit only seeded
Terra once through the authoritative gate, reconcile its complete immutable
receipt, and stop before Luna if the observation is invalid, the reservation is
exceeded, or weekly account usage rises by two points.

A valid direct screen establishes only isolated capability. Finalists must
later pass representative Multica issue, tool, review, recovery, release, and
owner-burden workflows before any production role is assigned.
