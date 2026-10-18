# Item 14 OpenCode structured-output v33 corrected prevalidation

Date: 2026-09-13  
Status: frozen provider-ready capsule; this correction pass made **no provider calls**

## Preserved zero-call execution attempt

The first paid-command invocation made **zero provider calls** and created no
run artifacts. It failed locally before fixture preparation because the parent
`artifacts/protocol-ablation-v33/workspaces` directory did not yet exist. This
is preserved as execution-attempt evidence, not as a model observation. The
v33-only command now creates the exact per-binding fixture parent beneath that
directory before copying the frozen R05 fixture; a provider-free regression
test covers a fully missing artifact-root path.

## Preserved one-call parser halt and provider-free continuation

The next paid invocation consumed exactly one Claude loop call in OpenCode
session `ses_f657bcb13ffeLZ26HslEiUpTud`. OpenCode returned a zero-retry
`StructuredOutputError` with documented total 6,036 tokens: input 2, cache
write 4,761, output 808, reasoning 465, cache read 0; exact provider-reported
cost was `$0.0246365`. The old v33 parser rejected the documented `tokens.total`
field before sealing raw evidence, leaving the original root globally halted
with a pending claim. That original root is immutable recovery input and the
cell must not be retried.

Corrected v33 accepts `tokens.total` only when it exactly equals input, cache
read, cache write, output, and reasoning in both the assistant envelope and
step-finish part. The provider-free recovery command exports that exact local
session, validates prompt/schema/model/effort/no-retry and reconciled usage,
then atomically seeds a separate continuation root with the Claude loop cell
completed as a candidate-local `response_or_tool_protocol_failure`. Replaying
the same export reuses the identical ledger and makes no provider call.

## Preserved timeout halt and second provider-free continuation

The next paid command consumed the Claude one-shot request in OpenCode session
`ses_f656f114fffeJ3HtrPyWe2up7U`, but OpenCode exported a
`MessageAbortedError: Aborted` before recording any provider usage: zero input,
output, reasoning, cache, provider turns, and cost. The cell is not retried and
is not scored as Claude competence evidence. Its exact prompt, schema, model,
effort, denied-tools configuration, single `step-start`, absence of a completed
provider step, and zero accounting are imported provider-free as an
observation-local `transport_timeout` invalidity.

The original continuation-01 remains immutable. Recovery atomically seeds a
new continuation-02 containing both closed Claude cells: the loop cell remains
a valid candidate-local protocol failure, while the one-shot cell is
invalid/unscored transport evidence. Replaying the timeout export reuses the
same continuation and makes no provider call.

## Preserved GPT preflight halt and third provider-free continuation

The first continuation-02 execution reached GPT fixture preparation but failed
inside local OpenCode readiness before preflight sealing, claim creation,
session creation, or a provider request. The ledger and OpenCode session list
therefore prove zero GPT provider turns, tokens, and cost. Continuation-02 is
preserved as the halted source; continuation-03 carries forward only the two
closed Claude cells and no invented GPT observation.

The failure was a local readiness-classification bug. The readiness window was
15 seconds, but `/doc` used the separate 120-second provider client and ignored
GET/body errors. A single hung readiness request could therefore take roughly
120 seconds, leave no document bytes, and be mislabeled a hash mismatch. The
corrected path uses a separate two-second readiness client and request context,
checks transport, status, body-read, and empty-body errors, and reports a hash
mismatch only for successfully read bytes. Any true mismatch preserves the
observed hash, expected hash, and byte length in durable halt evidence.

## Corrected boundary

v33 now models OpenCode 1.18.18's synchronous loopback server API. Each
provider step sends the v32 prompt bytes and the exact A/B schema separately:

```json
{"format":{"type":"json_schema","schema":{},"retryCount":0}}
```

The request also binds its session ID, request-message ID, exact OpenRouter
model, effort, qualification agent, denied tools, isolated config, resolved
config hash, and `enforced_single_route_no_fallback`. It uses `serve --pure`;
CLI `run --format json` is explicitly not represented as schema enforcement.
The single-shot HTTP client has a hash-bound 120-second timeout. There is no
HTTP or model retry after that bounded request returns or aborts.

## Evidence gate

No successful observation receipt exists until all of these pass:

- HTTP 200 with JSON content type;
- complete assistant envelope, fresh-session relationship, completed time,
  finish reason, exact provider/model/effort, and session/message IDs;
- exactly one `step-start` and one `step-finish` for that assistant;
- assistant and step usage/cost reconcile exactly;
- no retry, tool, subtask, patch, duplicate, unrelated, or unknown part;
- `structured` validates against the exact schema carried by the request, or
  the result is a distinct zero-retry `StructuredOutputError`.

The raw response and all parts are preserved. Receipt validation reparses the
raw response and compares every derived field, including usage, costs, part
counts, outcomes, IDs, and capsule/request/config/prompt/schema hashes.

## Lifecycle and accounting

The provider-free lifecycle contract pins:

- OpenCode version `1.18.18`;
- executable SHA-256
  `b6efa9eb3ee1d5c25438f3cba03c471e5d8661a6121abc47f209db3b3f05f415`;
- local OpenAPI SHA-256
  `c3a9f94af0c3324d97b482b14c692e810ce7ccac3136319ba46334de972b4cf1`;
- loopback readiness and guaranteed shutdown;
- durable order: preflight, claim, raw response, observation, ledger;
- no retry after an ambiguous claimed request;
- overflow-safe admission against the unchanged v32 aggregate token, cost,
  and provider-turn caps.

Before every provider step, the plan requires a separately evidenced upper
bound for the exact next input and reduces the server output limit to the lower
of the remaining combined and generated allowances. A call is not admitted if
that bound plus its maximum output could cross 75,000 combined or 20,000
generated tokens.

Candidate competence, response-protocol, and candidate resource exhaustion
remain local. Credential, runtime, configuration, credits, transport,
evidence-integrity, unsafe-action, and actual cumulative campaign-budget
failures are global halts.

## Provider-free proof

Focused tests exercise all four v32 bindings, exact schema/prompt/config/timeout hashes,
valid and schema-invalid structured output, structured-output errors, forbidden
parts, multiple steps, usage/cost disagreement, receipt tampering, HTTP status
and content type, request parentage, one HTTP call, loopback-only routing,
fresh session creation, bounded 120-second client behavior, no HTTP/model retry,
lifecycle shutdown, both durable recovery paths, failure scope,
aggregate boundaries, and integer overflow.

No provider, production, Browser, Phone Farm, or outbound call was made. v32 is
unchanged. The corrected source, tests, command paths, document, inherited
R05/v32 identities, and 120-second transport boundary are atomically frozen
before any further paid observation.
