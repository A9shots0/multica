# Item 14 Claude Subscription Static Breadth/Reliability v34 — Prevalidation

Status: **frozen and provider-free after the independent review plus one consolidated correction**. The exact capped batch is now authorized to execute, but no Claude/provider call was made while preparing, reviewing, or correcting it. This batch gathers testing evidence only; it cannot select a role or promote a production configuration.

## Fixed candidate and transport

- Candidate: `screen.generalist.claude46.medium.subscription.calibration.v29`
- Runtime/model/effort: exact local Claude Code `2.1.270 (Claude Code)` executable, SHA-256 `7999fba95dbffe167d9e0a043f29057979a0518ebe89b60c4fcfc6401ea8c424`, `claude-sonnet-4-6`, medium
- Transport: exact prompt bytes over stdin; restricted permission mode; `--tools ""`; strict empty MCP config; no session persistence; JSON Schema output; no retries
- Billing/provider boundary: the owner's direct Anthropic Claude Code subscription, explicitly pinned as `anthropic` with fallbacks disabled. Provider-reported cost is retained as a cost-equivalent measurement, not represented as cash spend.
- The original v29 Windows argument-length attempt remains preserved as invalid evidence. The corrected stdin T11 v4 calibration is a valid PASS, but the transport lane remains held until breadth/reliability evidence exists.

## Frozen order and stop behavior

The batch reuses the existing frozen candidate packets and blind graders byte-for-byte, in this order:

1. T12 v4
2. T11 v4 repeat
3. T08 v3
4. T16 v3
5. T04 v1
6. T09 v2

T09 v1 is not reused because its numeric count contract was proven ambiguous. T09 v2 is the already independently validated, minimally corrected version. A competence, output-protocol, or disqualifier failure eliminates this candidate and records later cells as skipped without calls. An observation-local transport invalidity records only that cell invalid/unscored and continues unaffected cells. Typed credential failure, unsafe external action, evidence corruption, campaign-token overflow, or the 15-minute wall deadline halts globally; error-message wording cannot change those classes.

## Caps and evidence

- At most six provider invocations; zero retries
- At most 15,000 provider-reported tokens and 2,000 output tokens per cell
- At most 90,000 exact or conservatively reserved reported tokens for the campaign
- One 15-minute wall-clock campaign context
- Every launched CLI attempt counts once. A launched response whose usage cannot be normalized is marked unknown and conservatively reserves the full 15,000-token cell ceiling; it is never reported as zero usage.
- Write-once claim, fully bound launch capsule, raw response, normalized receipt, candidate response, blind grade, result, and aggregate ledger
- Provider-free recovery closes a stranded pending claim as invalid/unscored with one unknown, conservatively reserved invocation. It never repeats the provider call.
- Replay verifies the complete frozen claim set and the hashes plus semantic bindings of claims, capsules, raw responses, receipts, candidate responses, grades, results, routing, and accounting without invoking the provider.
- The freeze hash-binds the shared transport, parser, receipt normalizer/validator, case/schema builder, grader, adversarial tests, and an antecedent manifest whose hashes are checked against both the original invalid v29 attempt and the corrected stdin PASS evidence.

## Provider-free acceptance gate

Before any provider execution: the corrected definition, transport catalog, execution-critical shared sources, adversarial tests, v29 antecedent manifest, executable lifecycle, focused tests, and this document are hash-bound in one atomic freeze; every inherited candidate/grader hash must match; the antecedent files must remain byte-identical; all frozen grader and lifecycle tests must pass; `go vet` must pass; and the plan must report the reviewed freeze with `provider_calls_allowed: true` while still reporting `provider_called: false`. This authorization is limited to the six frozen calls and grants no role, production, Browser, tool, external-effect, or Phone Farm authority.
