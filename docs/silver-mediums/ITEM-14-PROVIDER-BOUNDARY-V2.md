# Item 14 Provider Boundary v2

Date: 2026-09-14  
Status: terminal fail-closed after one authorized provider call; authority exhausted  
Scope: Item 14 only

## Corrected provider ownership

- Claude and every Anthropic model may run only through the existing Claude
  subscription in Claude Code. OpenRouter is forbidden for Claude/Anthropic.
- The prior Claude Code observation remains valid evidence that the selected
  Sonnet session also produced background Haiku usage. It is a documented
  exact-model limitation, not permission to retry, bypass, or weaken the gate.
- OpenAI/Codex work uses the two Codex subscriptions. OpenRouter is forbidden
  for OpenAI models in this Item 14 lane.
- OpenRouter may be used only for the already-established small non-Claude
  v26 survivor pool and only within the existing cumulative `$40` testing
  ceiling. Historical evidence is not rewritten.

The additive Item 14 v2 lane rejects any OpenRouter route outside the exact v26
survivor allowlist before executable lookup, credential access, or inference.
That includes Claude/Anthropic, OpenAI, altered aliases, provider mismatches,
and previously screened-but-eliminated routes. Historical frozen runners were
not rewritten and are not authorized by this lane.

## Temporary fixture and frozen real-shadow bound

`openrouter/qwen/qwen3-coder-next` through serving provider `alibaba` is frozen
as a temporary fixture because its v26 reliability result was `PASS_10_OF_10`.
This is not a role, squad, or final model selection.

The frozen definition is
`silver-mediums.openrouter-local-shadow-definition.v1`, SHA-256
`00a59b865276362a47c9c73777624ef64eb7fda55cf0f44393b1ba88235fce4a`.
It binds the existing provider-free local-shadow suite SHA-256
`9c8803d41b12b99fa7286ecd379d82ca3f2497bae8f181f1d4c524e8912e32be`
and permits at most four real inference calls: exactly one for each frozen
workflow, zero retries, no fallback, and a 1,200 generated-token ceiling per
call. Any identity, serving-provider, response-contract, transport, cleanup,
or evidence failure halts the remaining run and is never retried.

The existing cumulative OpenRouter authority remains exactly `$40`: baseline
`102,104,668` micros and absolute ceiling `142,104,668` micros. This slice also
adds a stricter `$1` incremental reservation. The published Alibaba route price
used for preventive admission was `$0.30`/million input and `$1.50`/million
generated tokens, producing an `80,444`-microdollar per-call reservation and a
`321,776`-microdollar four-call reservation. Both the authoritative account
usage and provider-reported receipt cost must remain inside the frozen bounds.

## Provider-free gates

The following completed without reading a credential or contacting a model:

- Exact provider-policy tests accept only the four v26 survivors and reject
  Claude/Anthropic, OpenAI, unknown, altered, and provider-mismatched routes.
- The fake workflow run produced four simulated exact-model/provider receipts,
  zero real provider requests, zero retries, and zero tool, repository,
  production, outbound, Phone Farm, or final-selection actions.
- Altered model/provider/response evidence, duplicate observations, replayed
  provider response IDs, retries, tools, and excess spend fail closed.
- Prepare-only validated all four fully merged OpenCode configurations at
  SHA-256
  `b56f1077f1a76772ab939bbdfa414267a5db98530804706d676c92420f257a29`.
  It made zero provider calls and zero credential reads, then removed four
  fresh task-local runtime roots and verified all four absent.

The real path writes an exclusive run claim and one exclusive workflow claim
before each possible inference. Existing roots or duplicate evidence writes
are rejected. A claim without a complete receipt is ambiguous and cannot be
replayed. Raw run/export evidence, exact actual model and selected serving-
provider metadata, tokens, provider cost, timestamps, cleanup, and before/after
account usage are sealed locally.

## Independent review and consolidated correction

The one independent provider-free review returned REVISE. It found that the
initial candidate checked only the terminal assistant, allowed an alternate
artifact root to consume the same logical authority, enforced the `$1` limit
post hoc, let prepare-only inherit the real profile, let the fake path bypass
the real state machine, and did not bind the exact v26 eligibility artifact.

One consolidated correction addressed all findings: full-session assistant,
model, tool, and selected-provider inspection; one fixed global authority
claim; price-derived preventive reservations plus post-exposure accounting;
fresh empty prepare-only profiles; dependency-injected fake and real paths
through the same claims and lifecycle; and the exact v26 eligibility SHA-256.
The final definition and session-inspection tests passed before the authorized
call. No second review ran.

## Authorized real result

The frozen real command was invoked once. The first workflow made exactly one
provider request, then failed closed and did not call the remaining three:

- requested and observed model:
  `openrouter/qwen/qwen3-coder-next`;
- requested serving provider: `alibaba`, with fallbacks disabled;
- actual serving provider: unobserved because the sealed OpenCode/OpenRouter
  session contained no selected-provider metadata;
- full session: one assistant, one model, zero summary/background assistants,
  and zero tool parts;
- strict response: rejected because the returned JSON added unapproved fields
  and therefore did not satisfy the frozen evidence contract;
- usage: 1,133 input, 0 cached input, 265 output, 0 reasoning tokens;
- provider receipt cost: `$0.00034796` (`348` normalized micros);
  authoritative before/after account snapshots both read `111,578,819`
  micros, so snapshot delta was `0`;
- trace duration: `989 ms`; accounting window: `8,276 ms`;
- cleanup: verified; retry: forbidden; authority: exhausted.

The write-once terminal receipt is
`.qualification-evidence/item14-openrouter-local-shadow-v1/terminal-failure-receipt.json`.
Provider-free reconciliation also sealed `accounting.json`; a second
reconciliation or a second real run cannot overwrite or replay the claims.

## Final provider-free verification

- Fake execution: four simulated responses, zero real requests, zero retries,
  and every forbidden counter zero.
- Prepare-only: four exact resolved configurations, zero credential reads,
  zero provider calls, and four removed/absent task-local roots.
- Exact boundary, hidden-assistant, mutation, spend, claim, duplicate, and
  replay tests passed 20 repetitions.
- Full `agentqualification` and `screening-run` packages pass; focused vet
  passes.
- A second reconciliation against the actual evidence root failed on the
  existing write-once terminal receipt. Only workflow 1 exists; `run.json` is
  absent, so no aggregate success can be inferred.
- The Silver Mediums hub validator passes with its existing stale-claim and
  dated-snapshot warnings. The real stewardship check passes all 20 sources;
  graph SHA-256 is
  `ff4119b1d70ced0a943f8dd056a1f3851e24c975ebacfad37dfc76d9aa75d1fa`,
  role-catalog SHA-256 is
  `c89c5f619c5f1a724e255b9014bfc882295a2c00504191bfc48569a5bbcc63e2`,
  and the bound hub `CURRENT_STATE.md` SHA-256 is
  `ac0f65d6683acbc60878bc249031968985d1d0bf6b0b39e7b96475edbef5a413`.

## Boundary

The one-call observation is useful boundary evidence, but it is not a positive
exact-model-and-serving-provider workflow proof and it is not model
qualification. The three remaining workflows were never called. No retry or
additional provider call is authorized. Do not use this artifact to select a
permanent role/model, call Claude or OpenAI through OpenRouter, access
production, enable tools, touch Phone Farm, or start the upstream audit.
