# Item 14 — Claude subscription static reliability repeat v35 prevalidation

Status: `independent_hold_corrected_once_pending_atomic_freeze`

Evidence stage: `local-only`

Provider calls: **locked**. This preparation made no Claude/provider call, spent no subscription usage, created no evidence root, selected no role, and changed no production system.

## Purpose

V35 is one exact reliability repeat of the accepted v34 Claude Code subscription static breadth batch. It reuses the same candidate, model, effort, transport, six frozen case versions, execution order, and caps. It does not add or tune a case or grader.

Exact order:

1. `T12:v4`
2. `T11:v4`
3. `T08:v3`
4. `T16:v3`
5. `T04:v1`
6. `T09:v2`

Exact route: `claude-sonnet-4-6`, Medium effort, Claude Code subscription, Anthropic serving boundary, no provider fallback, restricted mode, empty tools, strict empty MCP configuration, no session persistence, one turn, prompt on stdin.

Caps remain identical to v34: six provider invocations, zero retries, 15,000 reported tokens per cell, 2,000 output tokens per cell, 90,000 campaign-reported tokens, and 900 seconds campaign wall time.

## Prior evidence binding

The v35 antecedent manifest binds the repeat to completed v34 evidence:

- v34 freeze: `424041cd761a56b72db4bb92c48f7518901f0975f608d37e3cb687ba0a92f9a3`
- v34 ledger: `21c3089b90b841513d3107477df339fe874aec9c93d71e3d8fbe37aac8207163`
- v34 result: six exact static passes, no candidate elimination, no global halt

The manifest also preserves each prior case result, receipt hash, grade hash, usage totals, and evidence-root identity. V34 sources and evidence were not edited.

Prepared definition SHA-256: `584a8f0848c40466db03457678a5fed09c063cced3ddd3ca98ddb27fc575c1d1`

Prepared antecedent SHA-256: `70d47c9287a0a01dab0966298d3563a4df28af531227389c632a50cbb5d030ee`

Prepared review binding reported by the provider-free plan: `b54a7dec0b2dc36a73d5c8d540616ee61c3b1703cbeb067d38d5df39e33026b2`

These are the immutable prepared review inputs. The independent review returned a HOLD. Per the owner's instruction, one consolidated correction was applied with no second review: the atomic freeze now binds these prepared inputs and all corrected execution sources without mutating the prepared definition.

## Execution and evidence controls

The prepared command implements:

- write-once initial claims, launch capsules, provider raw output, candidate response, execution receipt, blind grade, and result envelopes;
- an atomic canonical ledger with exact or conservatively reserved every-launch accounting and integer-overflow checks;
- provider-free stranded-claim recovery that closes an ambiguous launch once and never retries it;
- candidate-local elimination for competence/protocol/disqualifier failure while preserving unrelated global-stop semantics;
- global halt only for credential failure, unsafe external action, evidence corruption, or uncontrolled campaign-cap breach;
- full provider-free replay of identities, hashes, routing, receipts, usage, responses, grades, results, and zero-retry semantics;
- semantic grade comparison that treats omitted empty slices and explicit empty slices as equivalent while still rejecting changed checks.

The prepared definition remains `prepared_for_independent_review` with `provider_calls_allowed=false`. Execution is authorized only by a separately validated atomic freeze receipt; neither the prepared definition nor an unvalidated receipt can unlock calls.

Before every new provider invocation, the executor now performs a complete provider-free replay of every previously completed claim and all bound claim/result/capsule/raw/receipt/response/grade artifacts. A stranded pending claim is reconciled once with conservative usage and no retry, then replayed before any later cell may launch. Any drift globally halts before the next call.

Claude Code authentication loss is classified only from its structured failed terminal JSON envelope. A real-shape zero-turn, zero-provider-time signed-out fixture and a typed `authentication_error` fixture both globally halt as credential failures. Plain error strings, successful result envelopes containing auth-like text, and post-provider/model-shaped failures cannot spoof that classification; ordinary nonzero process failures remain observation-local transport invalidity.

## Verification completed

From `server/`:

```text
go test ./internal/agentqualification ./cmd/claude-subscription-static-v35 -count=1
```

Result: both packages passed.

```text
go run ./cmd/claude-subscription-static-v35
```

Initial result before the consolidated correction: provider-free plan emitted the exact six-case order, route, caps, `provider_calls_allowed=false`, `retries=0`, and the prepared review binding.

```text
go run ./cmd/claude-subscription-static-v35 -execute -artifact-root C:\Dev\SMK-multica-selfhost-pilot\.qualification-evidence\SHOULD-NOT-EXIST-v35
```

Initial result before the consolidated correction: failed closed with `v35 provider calls remain locked pending independent review`; the requested directory does not exist.

## Review request

The independent review completed once and returned HOLD with three consolidated findings. The correction described above addresses that single review. No second reviewer cycle is requested. Final focused tests, vet, atomic receipt validation, and a provider-free execute-ready plan are the remaining freeze checks; no provider call is part of this correction.
