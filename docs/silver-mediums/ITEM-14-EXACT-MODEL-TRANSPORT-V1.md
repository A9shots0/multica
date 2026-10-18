# Item 14 Exact-Model Transport v1

Status: direct real authority exhausted; attempt failed closed before provider
contact; provider-free transport evidence remains valid.

## Preserved observation

The one authorized Claude Code subscription shadow remains a valid fail-closed
observation. One controller/CLI launch reported both
`claude-sonnet-4-6` and `claude-haiku-4-5-20251001`; the exact-model gate
rejected it. It is not retried, reclassified, or weakened by this document.

## Claude Code boundary decision

Claude Code is unsuitable for an authority that permits exactly one underlying
provider request to exactly one model. Anthropic documents `--model` as the
session's starting selection rather than full enforcement, and documents
separate default model controls for Sonnet, Opus, and Haiku/background
functionality. Its JSON result provides per-model usage after execution, which
is useful detection evidence but cannot prevent an already-issued background
request.

Pinning every documented model alias to the requested model would not prove
that only one request occurred. Accepting a one-model aggregate after doing so
would therefore weaken the existing one-request guardrail. An
`ANTHROPIC_BASE_URL` relay would also be insufficient by itself because the
accepted child boundary is not an OS egress sandbox and could not prove that
all closed-source CLI traffic traversed that relay.

Sources:

- https://code.claude.com/docs/en/model-config
- https://code.claude.com/docs/en/headless
- https://code.claude.com/docs/en/env-vars

## Safest measurable alternative

The provider-free candidate is a controller-owned, single-use direct Anthropic
Messages request. It does not launch Claude Code. Its v1 policy freezes:

- endpoint `https://api.anthropic.com/v1/messages`;
- one exact model value and exact prompt SHA-256;
- exactly one provider request and zero retries;
- HTTP POST with a non-replayable body and denied redirects;
- a fixed 60-second request timeout;
- exact JSON fields `model`, `max_tokens`, and one user message, with no tool
  field or tool choice;
- a 1,024-token output ceiling;
- one opaque API-key read into daemon memory for this request only, with no
  persisted credential copy; and
- required response-body model identity, positive usage, end-turn stop, and
  text-only content before an output can be accepted.

The executor is single-use even when validation or transport fails. Prompt or
policy alteration fails before fake credential access. Its HTTP gate permits a
single `RoundTrip`, validates the exact request body before handing it to the
fake provider, disables request-body replay, and denies redirect following.
Provider model drift, a tool response, duplicate invocation, and a synthetic
transport failure all fail closed without a second fake-provider request.
Strict response decoding also rejects unknown response fields and any nonzero
server-tool-use count. Redirect coverage supplies a real `Location` target and
proves the client returns the original redirect response without a second base
`RoundTrip`.

## Evidence and remaining gate

This slice uses only fake credential bytes and an in-memory fake HTTP provider.
It does not read a credential path, start Claude Code, resolve an installed
provider CLI, open a socket, or contact Anthropic or any other provider. The
candidate is not yet wired as active dispatch authority and does not select a
role or model.

The injected `RoundTripper` proves the controller gate against a fake provider;
it does not prove that an arbitrary custom transport makes only one network
request. Before real wiring, the implementation must construct and bind a fresh
known Go HTTP transport with no ambient proxy or custom retry wrapper and test
that exact transport under the new authority. Supplying the default network
transport remains intentionally impossible from the active dispatch path.

A real call would require a separately bounded owner grant covering an exact
temporary model binding, an Anthropic API-key source and in-memory custody,
metered API spend, and one direct request. That future grant
must be issued before any credential source is inspected or the default HTTP
transport is supplied.

## Final provider-free verification

The slice closed provider-free on 2026-09-14. Focused qualification tests,
20 repetitions of the guarded exact-model daemon suite (including the
concurrent duplicate case), the preceding guarded credential/child/authority
regressions, guarded agent tests, the build-tagged integration compile gate,
and focused `go vet` all pass. The platform cannot run Go's race detector
because CGO is disabled, so concurrency was exercised repeatedly without
claiming race-detector evidence.

The hub validator passes with its pre-existing stale-claim warnings. The real
20-source stewardship check passes with graph SHA-256
`ff4119b1d70ced0a943f8dd056a1f3851e24c975ebacfad37dfc76d9aa75d1fa`;
the bound hub `CURRENT_STATE.md` SHA-256 is
`16ae1ab2afab0ee6d53ef69e376d2214c6d095412ac394e21d29478543d40744`.

This provider-free slice is complete. The stop boundary is deliberate: no
credential source, real HTTP transport, dispatch activation, or provider call
may be added without a new separately bounded owner grant.

## Authorized direct-proof preflight (2026-09-14)

The owner subsequently granted exactly one direct metered proof with temporary
fixture `claude-sonnet-4-6` and a `$1` ceiling. The frozen authority is in
`ITEM-14-DIRECT-MESSAGES-REAL-PROOF-AUTHORITY.md`.

Before credential access, v1 was tightened to a 64-token output cap, explicit
`standard_only` service, explicit `global` inference, and an exact list-price
cost contract. A fresh Go HTTP transport disables ambient proxies, HTTP/2
forcing, keep-alive reuse, compression, redirects, and custom retry wrappers.
The non-idempotent POST remains non-replayable.

The complete fake suite passed 20 repetitions, including concurrent duplicate
denial and exact cost evidence. The build-tagged prepare-only harness then
passed with zero requests, no credential open, and no provider start. One
independent provider-free review returned REVISE; one consolidated correction
added current documented response metadata handling and wiped temporary
credential bytes on every error path. The corrected fake suite, prepare-only
gate, and focused vet pass. The sole real attempt is now eligible.

The sole authorized real invocation then failed before provider contact because
the inherited API-key mechanism was unavailable in its test process. It
recorded zero requests, tokens, and cost; no model was provider-observed and no
credential bytes or artifact existed to clean up. The authority is exhausted
and the transport is not retried. This leaves the positive real exact-model
boundary unproved while preserving every provider-free result.
