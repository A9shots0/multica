# Item 14 qualification execution transport boundary

Date: 2026-09-11  
State: `tool_free_dry_run_admission_proved_real_execution_held`

## Question

Can the next isolated capability trial prevent tool use, preserve complete
scientific provenance, and still consume the owner's Codex subscription rather
than an unrelated API billing lane?

## Verified evidence

- Local `codex-cli 0.153.2` exposes sandboxing, ignored user configuration and
  rules, ephemeral history, model/effort selection, JSONL, and an output schema.
  Its documented `codex exec --help` surface does not expose a disable-all-tools
  or `tool_choice=none` option.
- Official OpenAI Responses API documentation exposes `tool_choice: "none"`,
  an optional empty tools list, structured JSON output, `store: false`, model
  identity, response identity/timestamps, and usage fields.
- No evidence gathered here establishes that direct Responses API requests can
  use either ChatGPT/Codex subscription. API credentials, spend, and usage are
  therefore a separate unapproved capacity lane until proven otherwise.
- Installed Claude Code 2.1.263 exposes non-interactive JSON/stream-JSON,
  JSON Schema, no-session persistence, `--tools ""`, bare/restricted modes,
  strict MCP configuration, and usage/cost metadata. That makes it a plausible
  preventive tool-free subscription adapter. Its command plan and fabricated
  JSON fixture now pass the common normalized receipt path; one real capped
  zero-repository receipt and authoritative subscription-capacity evidence
  remain outstanding.
- A redacted `claude auth status` preflight confirms this host is logged in via
  `claude.ai` with a Max subscription. The CLI does not expose a non-inference
  usage/quota command in its documented help, so authentication is proved but
  authoritative remaining capacity is not.
- Installed OpenCode 1.18.18 exposes pure one-turn JSON runs, inline runtime
  configuration, exact model/variant selection, and session export. Official
  current OpenCode documentation supports deny permissions for built-in,
  custom, and MCP tools. A hash-pinned empty-workspace plan and fabricated
  JSONL-plus-export receipt now pass the common normalizer without inference.
  A redacted resolved-config preflight confirms deny-all settings and zero MCP
  entries; a separate negative fixture proves inherited MCP state is rejected.
- A read-only `opencode auth list` on this host reports one OpenRouter
  credential and no OpenCode Zen or OpenCode Go credential. OpenCode is the
  client transport, not proof of a particular subscription or billing lane.
  Current official OpenCode documentation distinguishes Zen pay-as-you-go,
  Go subscription, provider credentials such as OpenRouter, and temporary free
  models. Those lanes must remain separate in every result.

Official source:
https://developers.openai.com/api/reference/cli/resources/responses/methods/create

Additional primary sources:

- https://code.claude.com/docs/en/cli-usage
- https://code.claude.com/docs/en/headless
- https://opencode.ai/docs/agents/
- https://opencode.ai/docs/cli/
- https://opencode.ai/docs/config/
- https://opencode.ai/docs/zen/
- https://opencode.ai/config.json
- https://github.com/anomalyco/opencode/blob/dev/packages/opencode/src/cli/cmd/run.ts
- https://github.com/anomalyco/opencode/blob/dev/packages/opencode/src/cli/cmd/export.ts

## Decision

Do not weaken the benchmark merely to fit the Codex CLI. Do not silently move
the trial onto API billing. Keep V5 plan-only.

The eventual executor may be admitted only after its transport proves:

1. tools are unavailable by construction, not only rejected after a call;
2. the immutable prompt/schema/configuration and complete response trace bind
   to provider-reported model, response ID, timestamps, and exact usage;
3. it consumes `PrepareNextScreeningLaunch` immediately before the call and
   atomically seals the receipt so the admission cannot be replayed;
4. its credential and billing/account lane is explicit, isolated, and
   authorized;
5. Codex, Claude Code, OpenCode, and local-model trials use separate transport
   adapters while producing the same normalized observation contract.

## Available-lane register

`server/internal/agentqualification/transport_catalog_v1.json` now preserves
all six lanes the owner named instead of silently dropping or conflating them:
the two Codex subscriptions, OpenAI API, Claude Code subscription, OpenCode
provider lanes, Mac Mini Ollama utility, and the separate Mac Studio Phone Farm
boundary. The two Codex subscriptions are capacity lanes for the same
transport, not automatically different model candidates.

The catalog currently qualifies none of them. Codex CLI lacks proven
preventive tool isolation; Responses API lacks an authorized and
authoritatively measured API budget lane; Claude Code has a hash-pinned
restricted tool-free plan and fixture-calibrated receipt but lacks one real
capped receipt and authoritative subscription-capacity evidence;
OpenCode CLI now has a fixture-calibrated plan, resolved-config preflight, and
receipt, but the host currently exposes OpenRouter rather than a verified
OpenCode subscription. It still lacks one real capped receipt, authoritative
lane-specific capacity evidence, and Multica calibration. Ollama
remains a bounded utility; Phone Farm remains external and Mac Studio-only.
Unknown compatibility is recorded as unknown, not treated as failure or
success.

This is not an owner decision yet because no spend or provider migration is
being requested. The model-independent ledger now stops after invalid evidence
or a hard disqualifier, binds completion to the seeded assignment, atomically
reserves a launch across processes, keeps API-dollar and subscription capacity
separate, and seals a provenance-complete receipt exactly once. Raise the
billing-lane choice only when a provider adapter enforces these contracts and
is ready for one bounded reviewed observation.

The original Claude dry-run plan pinned bare/restricted mode, but live CLI help
proved `--bare` disables OAuth and keychain reads and is therefore incompatible
with the signed-in subscription lane. The corrected subscription plan omits
bare mode and pins restricted mode, an explicitly empty tools list, strict
empty MCP configuration, no session persistence, no permission prompt handler,
one turn, JSON output, and the case schema. Its parser rejects failed
or nonterminal results, missing or multiple provider-reported models,
malformed structured output, incomplete token data, and costs that cannot be
represented exactly in integer micro-dollars. Reasoning-token detail the
provider does not expose remains unavailable; it is never invented as zero.

The OpenCode plan pins `--pure`, a fresh one-turn session with an explicit title
and empty working/config directories, exact provider/model and variant, disabled
sharing/plugins/snapshots/compaction, and global plus agent-level deny-all tool
policy. Its parser requires exactly one start, JSON text result, and finish;
then binds that stream to the matching exported session, provider/model,
tokens, cache, reasoning, cost, and output. Tools, unknown events, duplicate
terminal parts, model drift, usage drift, or export disagreement invalidate the
receipt. The resolved-config preflight additionally requires zero MCP entries;
the redacted local check passed that condition. No OpenCode request was made.

## Dry-run admission proof

OpenCode is now connected to the same central instruction-compatibility,
ledger, seeded-assignment, and atomic-reservation path as every other
screening transport. `ReserveOpenCodeLaunchCapsule` performs the following
without invoking a provider:

1. evaluates the complete fail-closed launch gate, including a fresh
   authoritative metered-API spend snapshot rather than a subscription
   percentage;
2. selects the exact seeded assignment;
3. reconstructs and hash-checks its prompt, schema, inline configuration,
   model, effort, and public packet;
4. validates the fully resolved OpenCode configuration before reservation;
5. atomically reserves the assignment across processes; and
6. writes one immutable `ready_no_inference` launch capsule beside the
   reservation.

The following paragraph records the original dry-run checkpoint and is
superseded for OpenRouter by the dated live-receipt proof below. The capsule
explicitly records `provider_call_allowed: false`. It contains
only hashes and placeholder command/environment templates, not credentials or
the prompt body. Unsafe inherited configuration fails before a reservation is
created, while a second claimant is rejected as ambiguous rather than retried.

Subscription percentages and API-dollar budgets are now separate batch
families and cannot be supplied together or substituted for one another. This
closes the adapter-to-ledger wiring gap but does **not** authorize a real
request. The currently configured OpenRouter lane has no authoritative budget
or capacity read in this evidence set, and the owner-reported OpenCode
subscription is not configured as Zen or Go on this host. The future executor
must also revalidate that the
resolved configuration matches the capsule immediately before submission and
then normalize and seal the real provider receipt. No model call was made.

Claude Code now has the parallel subscription-lane dry-run proof. Its exact
tool-free plan is validated before the central subscription-capacity gate
atomically reserves a seeded assignment and seals a write-once
`ready_no_inference` capsule with `provider_call_allowed: false`. A stale or
missing authoritative capacity snapshot produces no reservation, and duplicate
claims remain ambiguous. This proves orchestration wiring, not quota. No Claude
request was made, and the provider call remains disabled until authoritative
capacity and final environment isolation are both available.

## 2026-09-12 OpenRouter live-receipt supersession

The OpenCode/OpenRouter lane now has a separate narrow provider-call grant
after the dry-run capsule. The executor re-reads authoritative OpenRouter
spend, validates the current resolved deny-all configuration, atomically
reserves one exact assignment, seals `provider-call-grant.json`, invokes once,
never retries ambiguity, and seals raw JSONL, session export, normalized
receipt, blind grade, and final observation. The v5 transport additionally
pins `OPENCODE_EXPERIMENTAL_OUTPUT_TOKEN_MAX=20000`.

This proves only the OpenCode client plus configured OpenRouter metered lane
for isolated qualification. Claude remains uncalibrated: v29 stopped locally
with zero provider calls when Windows rejected the oversized command line; the
prompt-to-stdin correction is tested but unexecuted. OpenCode Zen/Go,
Multica integration, production authority, repositories, Browser, messaging,
and Phone Farm remain outside this proof.
