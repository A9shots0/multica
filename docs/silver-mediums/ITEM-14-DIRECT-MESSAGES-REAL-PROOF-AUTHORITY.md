# Item 14 direct Messages real-proof authority

Status: **authority exhausted; sole real attempt failed closed before provider
contact; no retry authorized**.

Authorized: 2026-09-14.

## Exact bounded operation

The owner authorized one direct Anthropic Messages request solely to prove the
exact-model execution boundary. The temporary fixture is
`claude-sonnet-4-6`; it is not a final model, role, agent, provider, or runtime
selection.

The sole request is bound to:

- `POST https://api.anthropic.com/v1/messages`;
- exact model `claude-sonnet-4-6`;
- one fixed harmless prompt whose required response is
  `ITEM14_EXACT_MODEL_BOUNDARY_OK`;
- `max_tokens: 64`, `service_tier: standard_only`, and
  `inference_geo: global`;
- no tools, system prompt, metadata, files, repository content, production
  context, or outbound business action;
- one request, zero retries, redirects, provider/model fallbacks, proxy use,
  connection reuse, or custom retry wrapper; and
- a 60-second terminal timeout.

The only eligible network implementation is the freshly constructed Go HTTP
transport in `exact_model_transport_v1.go`. Ambient proxy lookup is disabled,
HTTP/2 forcing is disabled, keep-alive reuse and compression are disabled, and
the guarded non-idempotent POST has neither `GetBody` nor an idempotency key.
The single-use executor is consumed before credential access.

## Cost ceiling

The owner ceiling is exactly `$1.00` (`1,000,000` USD microdollars). The policy
uses the official 2026-09-14 Claude Sonnet 4.6 list prices of `$3` per million
input tokens and `$15` per million output tokens. Before credential access it
reserves a deliberately conservative 100,000 input tokens plus the 64-token
output cap: `300,960` USD microdollars. A response is rejected if its reported
usage or calculated list-price cost exceeds those bounds. No prompt caching,
US-only multiplier, batch pricing, priority tier, or fast mode is requested.

Official sources:

- https://platform.claude.com/docs/en/models/sonnet-4-6/overview
- https://platform.claude.com/docs/en/about-claude/pricing
- https://platform.claude.com/docs/en/api/messages/create

## Credential custody

The approved mechanism is the already-configured `ANTHROPIC_API_KEY` inherited
by the one test process. Prepare-only mode never reads or tests that value. The
real call site may read it exactly once, immediately removes its inherited copy
from that process environment, supplies it only as the one request's
`x-api-key` header, and zeroes its mutable byte buffers when the source and
executor close. It never reads `.env`, searches for another credential source,
copies the value to a file, prints it, hashes it, serializes it, or includes it
in a receipt. The owner's original configured source is not modified.

Cleanup passes only when the one credential source was opened and closed once,
the inherited environment entry is absent from the test process after use, and
no task-local credential artifact was created or retained.

## Pass, failure, and stop rules

A pass requires exactly one guarded transport request, exact response model
`claude-sonnet-4-6`, positive bounded input/output usage, standard/global
serving evidence, zero cache and server-tool use, the fixed harmless result,
calculated cost at or below `$1`, and verified custody cleanup.

Any missing credential, authentication/provider error, redirect, timeout,
model drift, extra/unknown response shape, tool evidence, output mismatch,
usage/cost breach, or cleanup failure is terminal and receives no retry. The
receipt records only the actual model, request count, tokens, calculated
list-price cost, timing, harmless result, request/response body hashes, and
cleanup booleans.

The authority is exhausted by the first real execution attempt regardless of
outcome. No additional provider call follows automatically.

## Provider-free precursor

The full fake suite passed 20 repetitions after the cost and standard/global
request tightening. It covers exact success, model/prompt/policy drift,
sequential and concurrent duplicate use, transport failure, redirect, tool and
server-tool evidence, unknown response fields, exact cost calculation, and the
pinned no-proxy real-transport constructor. The build-tagged prepare-only test
then passed with `requests=0`, `credential_opened=false`, and
`provider_started=false`.

## Independent pre-call review and correction

One independent provider-free review returned REVISE. Its reported authority
mismatch applied to the exhausted 2026-09-13 subscription-file grant, not the
owner's new 2026-09-14 API-key and metered-request grant recorded here; no
authority expansion was made.

The two applicable P1 findings were corrected in one consolidated pass. The
strict response decoder now recognizes current documented nullable
`container` and `stop_details` fields, cache detail, citations, and nested
`output_tokens_details.thinking_tokens` while rejecting any non-null container
or stop detail, non-empty citations, nonzero cache/tool usage, or impossible
thinking count. Every local credential byte buffer is now wiped on success,
malformed input, read failure, and close failure. Positive and negative fake
cases pass 20 repetitions, focused vet passes, and prepare-only again reports
zero requests with no credential open or provider start.

## Sole real-attempt result

The authorized harness was invoked exactly once on 2026-09-14 with the real-run
gate enabled. The approved inherited `ANTHROPIC_API_KEY` mechanism was
unavailable inside that one test process. The single-use executor failed before
it obtained credential bytes, constructed a request, or entered the HTTP
transport.

Terminal receipt:

| Field | Observed result |
| --- | --- |
| Requested temporary model | `claude-sonnet-4-6` |
| Provider-observed model | unobserved |
| HTTP provider requests | `0` |
| Retries / redirects / fallbacks / proxies | `0 / 0 / 0 / 0` |
| Input / output / thinking tokens | `0 / 0 / 0` |
| Calculated list-price cost | `$0.000000` |
| Harness execution timing | `0 ms` |
| Result | `approved credential mechanism is unavailable` |
| Credential bytes obtained | no |
| Credential artifact created or retained | no |
| Harness cleanup flag | `false` because no source opened or closed |

The false cleanup flag is preserved exactly and is not reclassified as a pass.
It means the expected one-open/one-close lifecycle never began; it does not
mean credential material was left behind. The harness has no file-copy path,
and no credential value was displayed, logged, hashed, persisted, or
documented.

This is a valid fail-closed credential-admission observation, not a real
provider or exact-model proof. The owner authority is exhausted by this attempt.
The code must not be rerun with a newly loaded `.env`, alternate source, or
credential substitution without a new separately bounded grant.
