# Item 14 real model-only shadow authority record

Status: **authority exhausted; sole real call failed closed; no retry authorized**.

Prepared: 2026-09-13 22:36 -04:00.

## Decision

The safest already-supported provider route for the one-call boundary proof is
the existing Claude Code subscription transport. It already has a guarded
Claude adapter with one turn, restricted mode, an empty tool list, strict empty
MCP configuration, no session persistence, no permission prompts, no service
tier, and no automatic retry. The test-only route is:

- runtime provider: `claude`;
- serving provider: `anthropic`;
- model fixture: `claude-sonnet-4-6`;
- effort fixture: `medium`;
- provider calls: exactly one;
- retries: zero, including authentication or timeout failures.

This is a temporary test cell chosen because that exact subscription transport
has prior local evidence. It does not qualify, rank, or permanently select the
model, role, provider, agent, or production runtime.

## Why credential custody needs a separate gate

The accepted guarded child creates a fresh task-local `CLAUDE_CONFIG_DIR` and
does not inherit provider credentials. Anthropic documents that Windows Claude
Code credentials live in `%USERPROFILE%\.claude\.credentials.json`, and that a
different `CLAUDE_CONFIG_DIR` reads a different credential file. The current
isolated child therefore cannot silently reuse the signed-in subscription.

The least-expansive bridge is a controller-bound, one-use credential capsule:

1. The daemon parent opens only
   `C:\Users\a9sho\.claude\.credentials.json` for reading after one exact
   capsule receipt has been consumed.
2. It byte-copies that file, without parsing, displaying, logging, hashing, or
   serializing its contents, to the fresh task-owned
   `CLAUDE_CONFIG_DIR\.credentials.json`.
3. Only the staged, SHA-256-bound Claude executable receives that isolated
   profile, for one process and one turn.
4. The temporary credential file is removed immediately after process exit and
   its absence is verified before the shadow can pass. The original file is
   never modified. Failure to copy, authenticate, launch, clean up, or prove
   absence fails the run with no retry.

The capsule is process-lifetime containment, not a provider-minted one-call
credential: the copied subscription login remains renewable credential material
while the child is alive. That limitation must remain explicit in the owner
authorization and evidence.

This is narrower than the available alternatives. An API key would introduce a
new metered billing lane; `claude setup-token` would mint a new one-year token;
an interactive login would create new external auth state; and pointing the
child at the normal Claude directory would expose the rest of the user profile
and break the accepted isolation boundary. Anthropic's current authentication
reference documents the storage locations, precedence, and long-lived token
behavior: <https://code.claude.com/docs/en/authentication>.

## Temporary controller binding

The disposable local PostgreSQL shadow will use this non-promotable identity:

| Field | Exact temporary value |
| --- | --- |
| Executor identity | `item14-model-boundary-shadow-once` |
| Configuration | `item14.claude-subscription.boundary-shadow.once.v1` |
| Existing role artifact | `silver-mediums.role-artifact.v2.data_sync_integrity` |
| Role use | Harness fixture only; no production role assignment |
| Model / effort | `claude-sonnet-4-6` / `medium`, test fixture only |
| Runtime | Freshly resolved Claude executable, staged and SHA-256-bound at issuance |
| Workspace | Fresh empty guarded workspace; no repository or project resource |
| Provider access | One Claude Code subscription request; no API key, gateway, fallback, or service tier |
| Lifetime | Ten minutes maximum, consumed on the first credential-file open |
| Retry / resume | Zero retries; no session persistence or resume |
| Output | One provider result, provider usage metadata, and nonsecret receipt hashes only |

The existing role artifact is reused only because it is already immutable,
accepted, and exercised by the provider-free path. The executor identity and
configuration are disposable and cannot be promoted by this observation.

## Exact harmless prompt

The guarded path must not use the ordinary issue prompt, because that prompt
tells a normal agent to use the Multica CLI. Before credential access, a
provider-free correction must add a controller-bound guarded prompt whose final
rendered SHA-256 is included in the one-shot package and rechecked immediately
before child launch.

The sole user payload is:

> This is an execution-boundary probe. No repository, tools, evidence source,
> production system, or outbound action is available or authorized. Return
> exactly the twelve-field JSON object required by the bound Data Sync and
> Integrity v2 instructions. Set `named_boundary` to
> `item14_execution_boundary`, `route_settled` to `false`, and `stop_reason` to
> `insufficient supplied evidence`. Use empty strings and arrays for all other
> fields, except `comparison`, which must use zero numerator, zero denominator,
> and definition `no evidence supplied`. Do not request or attempt any action.

The provider-free preflight must prove that changing this payload, its rendered
prompt, its credential purpose, task identity, model, effort, executable,
package digest, or capsule receipt fails before credential access or child
launch.

## Pass and stop conditions

The shadow passes only if the real dispatch/claim/daemon path records exactly
one guarded consumption, the staged Claude child starts exactly once, no tool
event occurs, the returned provider receipt identifies the requested model and
contains usage, the output matches the fixed response contract, and the
temporary credential file is absent afterward.

Any auth failure, model mismatch, tool event, malformed output, timeout,
unexpected billing route, cleanup failure, second launch, or replay is terminal.
There is no retry. A pass proves only that the guarded model-only provider
boundary works; it is not model qualification or production readiness.

After a pass, Item 14 may continue into complete local shadow workflows. No
migration or production action follows automatically. Claude's upstream/fork-
maintenance audit remains a separate pre-migration gate.

## Exact owner authority granted

Owner authority was granted on 2026-09-13 for exactly the bounded operation in
this document: one opaque read-only copy from the named existing credential
file, one `claude-sonnet-4-6` Medium subscription inference, zero retries, and
immediate deletion with verified absence. This grant does not extend to any
other credential, provider call, model/role selection, tool, repository,
production system, or outbound action.

## Provider-free precursor result

The precursor is green before credential access. A versioned
`task-capability-scope.v2` now binds the exact fixed payload, final rendered
role-plus-prompt SHA-256, one provider-call limit, zero retries, one turn, exact
credential source path, copy purpose/policy, and immediate verified-cleanup
policy. Version 1 scopes cannot carry this authority, and version 2 scopes must.

Focused tests prove a valid fake capsule opens its source exactly once, copies
opaque bytes into a fresh Claude profile, deletes the copy immediately, and
verifies absence. Altered prompt content/hash, source, purpose, copy policy,
call limit, or retry limit is denied. The full disposable-database shadow then
crossed real issuance, claim, start/consumption, daemon preparation, native
executable staging, isolated environment, and the Claude stream adapter. The
exact fake package executed once with zero tools and one consumption; altered
model, prompt, task identity, executable, credential source, package digest,
revocation, expiry, duplicate start, and replay produced no extra launch.

The Windows npm launcher audit also found that isolated staging must bind the
bundled `claude.exe`, not copy only its relative `.cmd`/`.ps1` shim. The guarded
resolver now selects that native executable. All of this evidence is local and
provider-free. The real credential has not yet been opened and Claude has not
yet been started.

At the provider-free checkpoint no credential had been inspected and no
provider contacted. The granted scope consists of all and only the following:

- one read of `C:\Users\a9sho\.claude\.credentials.json` for the byte-copy
  described above;
- temporary custody of that copy for one guarded Claude Code process;
- one `claude-sonnet-4-6` Medium subscription inference, with zero retries and
  no API-key spend;
- immediate deletion and absence verification of the temporary copy; and
- recording only nonsecret runtime, package, usage, result, and cleanup
  evidence.

The required credential-capsule and prompt-binding fake precursor completed
before the real attempt. The historical permission to perform one call was
consumed by the terminal execution recorded below and is no longer actionable.

The single independent review returned REVISE with no P0 and four P1 boundary
findings. One consolidated correction now requires an explicit loopback-only
disposable database URL; denies and kills any guarded Claude control request;
suppresses credential-bearing stderr and pre-validation transcript/log events;
preserves provider-reported model identity and turn count; and sets the
official `CLAUDE_CODE_MAX_RETRIES=0` control inside the exact child environment.
The installed Claude Code 2.1.270 binary contains that control. A focused
control-request-first kill test, zero-retry-environment tests, the corrected
full fake shadow, and an exact real-harness prepare-only run all pass. The
prepare-only run issued and claimed the real package but returned before
`runTask`; it recorded `credential_opened=false` and `provider_started=false`.

## One authorized real execution result

The sole authorized real execution ran once on 2026-09-13 and is terminal. The
guarded task was claimed and started once, one Claude child completed in one
reported turn, `CLAUDE_CODE_MAX_RETRIES=0` was bound, and no tool event was
observed. The provider terminal receipt nevertheless reported two model
identities: `claude-sonnet-4-6` and `claude-haiku-4-5-20251001`. Because the
approved package allowed exactly `claude-sonnet-4-6`, the harness failed closed
at the provider-receipt gate. No retry or second controller/CLI invocation
occurred. The two observed identities mean the one Claude CLI process cannot be
certified as only one underlying provider inference; the Haiku identity is
treated as unapproved background provider activity.

The failure happened before output-contract and usage acceptance, so neither is
claimed as passing evidence. The temporary credential copy was deleted after
the child exited and its exact path was verified absent. The fresh task-local
temporary root and exact-name loopback database proxy were also verified
absent. The original credential was not displayed, parsed, hashed, logged, or
modified.

This is evidence that the implemented gate detected provider-side background
model activity; it is not a positive single-model boundary proof. It selects no
model or role and does not authorize complete local shadow workflows. Any
follow-up real-provider experiment requires a new, separately bounded owner
grant and a design that can prove one exact provider model without relying on
this failed Claude Code transport behavior. Claude's upstream/fork-maintenance
audit remains a separate pre-migration gate.

Final provider-free verification passes: the guarded qualification, agent, and
daemon suites; the build-tagged integration compile-only gate; focused vet;
system-graph/stewardship tests; and the real 20-source stewardship validation.
The hub `CURRENT_STATE.md` receipt is
`a2dda25ddb3aaf420626d33fde6276d1b7c0c985aefd7e9a5aea91d01782a9cc`.

## Subsequent provider-free transport determination

The terminal observation remains valid and the exact-model guardrail remains
unchanged. Provider-free review determined that Claude Code cannot satisfy an
exactly-one-request/exactly-one-model authority: its model selection does not
govern every documented background-model path, while its per-model receipt is
post-execution detection. Claude Code is therefore unsuitable for this
boundary; no retry is permitted.

The separately versioned direct Messages candidate and its fake-only evidence
are recorded in `ITEM-14-EXACT-MODEL-TRANSPORT-V1.md`. It is not active dispatch
authority and requires a new owner grant before any credential source, network
transport, or real provider request is supplied.
