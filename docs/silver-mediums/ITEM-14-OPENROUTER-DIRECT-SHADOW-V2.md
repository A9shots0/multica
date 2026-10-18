# Item 14 OpenRouter direct shadow v2

Status: terminal fail-closed; one router request, zero provider inference

Snapshot: 2026-09-14 13:26 America/New_York

## Why this is a new generation

The terminal v1 OpenCode/OpenRouter observation remains immutable. It proved
one requested Qwen model and no hidden assistants or tools, but OpenCode did
not expose the actual serving provider and the model missed the response
contract. V2 neither edits nor reuses that evidence root. It replaces only the
transport with a direct OpenRouter Chat Completions request in a new write-once
root.

OpenRouter's current router-metadata contract states that the opt-in
`X-OpenRouter-Metadata: enabled` response includes the requested model,
available/selected endpoints, and every provider attempt. Its routing contract
also supports exact `order` plus `allow_fallbacks:false`; V2 additionally uses
the exact `only` allowlist. Sources:

- https://openrouter.ai/docs/guides/features/router-metadata
- https://openrouter.ai/docs/guides/routing/provider-selection
- https://openrouter.ai/docs/api/api-reference/chat/send-chat-completion-request

## Frozen boundary

- Definition: `silver-mediums.openrouter-direct-shadow-definition.v2`
- Definition SHA-256:
  `fbe314bc6d779d0ad57bb6150f200e7a9db6af8e9a7c0b4ca6e814007c9441b4`
- Frozen local-shadow suite SHA-256:
  `9c8803d41b12b99fa7286ecd379d82ca3f2497bae8f181f1d4c524e8912e32be`
- Eligibility evidence SHA-256:
  `84660c35c92e780c3c00f744a2380683accfc0158dea161b8353953247211b46`
- Temporary test configuration only:
  `openrouter/qwen/qwen3-coder-next`, direct API model
  `qwen/qwen3-coder-next`, serving provider `alibaba`.
- Four frozen workflows, one request per workflow, zero retries, zero
  redirects, no model fallbacks, exact provider `order` and `only`, required
  parameter support, strict JSON schema, and mandatory router metadata.
- Per-call generated-token ceiling: 1,200. Price-derived four-call reservation:
  321,776 micros. Slice ceiling: $1. The cumulative OpenRouter ceiling remains
  the established $40 above the recorded baseline; no new budget was created.
- Tools, repositories, production, outbound actions, Phone Farm, and role/model
  selection are forbidden.
- A schema/response miss is a candidate-local failure and does not prevent the
  remaining frozen workflows from running. Any model, endpoint, selected
  provider, provider-attempt, tool/choice, transport, accounting, or evidence
  ambiguity globally halts without retry.

## Provider-free evidence

Plan, fake-provider lifecycle, and prepare-only all pass with zero credential
reads and zero real provider calls. The fake lifecycle produces four exact
model/provider observations and four valid response contracts. Focused tests
prove altered routes, Claude/OpenAI substitution, fallback/retry/redirect/tool
expansion, duplicate claims, alternate-root replay, duplicate transport use,
provider/model/attempt drift, and hidden tool activity fail closed. They also
prove one response-contract failure is sealed locally while the other three
workflows continue.

The real path reads the existing OpenRouter credential through the established
local mechanism once into memory, never writes or hashes it, uses a direct
no-proxy/no-retry HTTP transport, clears request headers, checks that no
evidence file contains the credential bytes, zeroes the in-memory byte slice,
and records only non-secret accounting and cleanup facts.

## Next atomic action

Run the complete focused and package gates once more, then launch this exact
new V2 root. Do not rerun v1. Preserve every response, provider-attempt,
accounting, and failure receipt. A model response failure continues the suite;
an identity/provider/evidence failure stops it without retry.

## Terminal result

The exact V2 command ran once. OpenRouter returned HTTP 404 at parameter
filtering because Alibaba could not satisfy the strict structured-output
parameter. Router metadata reported the requested Qwen alias, four available
endpoints including Alibaba's exact
`qwen/qwen3-coder-next-2025-02-03` revision, no selected endpoint, router
attempt `0`, and no provider-attempt array. Account usage remained exactly
`111582154` micros before and after. Therefore this was one router request,
zero model inference attempts, zero spend, zero retries, and three workflows
not called.

The terminal response SHA-256 is
`611dfac19c079fcd560ebf89f042049646a53312b0b9eac2bc14b4991aa04aa6`.
Provider-free reconciliation sealed `terminal-failure-receipt.json` and
`accounting.json`. V2 authority is exhausted and must not be rerun. The result
justifies only a new version that removes the unsupported transport parameter;
it does not justify weakening the exact model, selected provider, attempt,
tool, budget, or response validation gates.
