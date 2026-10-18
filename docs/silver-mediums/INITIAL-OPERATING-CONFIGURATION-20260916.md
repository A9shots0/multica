# Initial operating configuration — 2026-09-16

- Configuration ID: `silver-mediums-initial-operating-configuration-20260916`
- Status: accepted, reversible, provisional
- Active: 2026-09-16T00:00:00Z through, but not including, 2026-09-30T00:00:00Z
- Machine record: `server/internal/agentqualification/initial_operating_configuration_v1.json`
- Configuration SHA-256:
  `dadf6a215f9ed8fb6d4ed5b957a6be143e3226dab080452c4c9d63454aaffa6d`
- Controller lifecycle registry:
  `server/internal/agentqualification/initial_operating_configuration_lifecycle_v1.json`
- Provider, credential, production, deployment, recovery, outbound, Phone Farm
  and permanent-selection authority granted by this document: **none**

## What it is, in one paragraph

The accepted automatic workflow — one worker turn, then an independent reviewer
the workflow routes on its own — may now be used for ordinary internal issues in
this repository, one at a time, started by the owner. Everything the accepted run
did not demonstrate stays switched off. The configuration expires by itself in
fourteen days and can be revoked instantly in the controller's registry.

## Eligible work

An issue is eligible only when every one of these is true:

| Requirement | Value |
|---|---|
| Repository | `SMK-multica-selfhost-pilot` |
| Host | `alienware_windows` |
| Workspace | internal only |
| Issue state | `received` or `queued` |
| Started by | the owner, per issue |
| Execution plan | names a distinct reviewer |
| Declared release gate | none — non-live work only |
| Existing provider tasks on the issue | zero |
| Issues already in flight | zero |

Anything else is denied by name rather than ignored.

## What stops and comes to the owner

These twelve conditions emit `owner_decision_required` and move the issue to
`needs_owner`. The list is closed: a condition the configuration does not
recognise also stops the work rather than being skipped.

- requires a business or product decision
- requires a broad or undefined scope change
- requires a credential or identity change
- requires a destructive or irreversible data change
- requires a new credential source or spending
- requires outbound communication
- requires Phone Farm access
- requires production deployment or rollback
- requires a public exposure change
- requires a recovery or restore action
- touches another repository or host
- unresolved high-risk ambiguity

Routine engineering inside the eligible class runs without asking.

## The route

```text
controller intake -> task-scoped executor session -> task-scoped independent reviewer session
```

- Execution and review are **separate sessions**. The reviewer is never the
  implementer.
- Permanent agent seats created: **0**. All three roles are task-scoped and
  archived after the run, exactly as the accepted run did.
- The controller owns every transition, using the states in
  `WORKFLOW-STATE-CONTRACT.md`.

## Bounds

| Bound | Value | Why this number |
|---|---|---|
| Provider tasks per issue | 2 | one publisher turn + one reviewer turn, the exact accepted shape |
| Automatic corrections | 0 | the accepted run needed none; the correction loop is implemented and provider-free tested but is not enabled here |
| Attempts per task | 1 | as accepted |
| Retries | 0 | as accepted |
| Fallbacks | none | no silent provider substitution |
| Concurrent eligible issues | 1 | the evidence covers one issue at a time |

## Lane assignment, and why it is not a permanent choice

| Lane | Provider / transport | Basis |
|---|---|---|
| Execution | Claude Code subscription composite, requesting `claude-sonnet-4-6`, medium effort | the only configuration with an accepted end-to-end native run |
| Independent review | the same lane, in a **distinct task-scoped session** | no second provider is authorized on this host, so independence is session-level only |

This is deliberately **not** a statement that Claude should permanently fill
every role. The record states in machine-checked form that:

- review independence is `single_provider_distinct_session`, not
  `distinct_provider`. The validator rejects the configuration if it claims
  otherwise while the acceptance record says the reviewer was not a distinct
  provider.
- the exact served model is not proven. Both accepted turns observed a
  background `claude-haiku-4-5-20251001` alongside the requested
  `claude-sonnet-4-6`, so `exact_single_model_claim_allowed` is false.
- **recovery-sensitive authority is withheld**, because this exact configuration
  validly failed the R04 exact-once recovery case
  (`ITEM-14-CLAUDE-SUBSCRIPTION-REALISTIC-V38-RESULT.md`, pinned by hash as
  `recovery_sensitive_exclusion_evidence`). Release, rollback, recovery and
  production verification remain owner-operated.
- `permanent_role_or_model_selection_allowed` is false and
  `permanent_squad_activation_allowed` is false.

## Disabled, and enforced by the record

`automatic_dispatch_allowed`, `public_access_allowed`,
`production_access_allowed`, `other_production_system_access_allowed`,
`other_repository_access_allowed`, `outbound_actions_allowed`,
`phone_farm_windows_access_allowed`, `provider_child_tools_allowed`,
`provider_child_repository_access_allowed`,
`configuration_authorizes_provider_call`,
`configuration_authorizes_credential_read` — all false. Each run still needs its
own separate authority, runtime binding and capability scope, exactly as before.

## Evidence this configuration is bound to

Every hash below is recomputed at decode time; any drift fails the configuration
closed rather than degrading it.

| Bound source | SHA-256 |
|---|---|
| Native publication acceptance | `8c096d747e0e2a4e151fff36ccb5a484def9a712bffff2588b49c766f5bc10f0` |
| Provisional configuration v2 | `e3a68b16ce1f6975639229e82f1a251e7e11800347738534afa6441adf41e3a2` |
| Integration replay result | `b4eed5453e1098ac9faed668961d1e6db2244fb7bf53ec39da1cf1fb005ab5a2` |
| Reliability trial result | `9f99664568ff701cadddbac537a7548cebfd88fbd2bc7d3d1415176ddb1f1f1a` |
| Role/squad operating contract | `b3f88549e6a92df5708c7a89c53cb31b0b034cfbf7639f52c96e76659ae8fe09` |
| Role/squad readiness v3 | `5408754c547c00c25a32cb4ef3356d944276f1e5c4dd56f9c25690317f7611a1` |
| Recovery-sensitive exclusion evidence | `7f73325a3e64fe207d465beca3d54a465e5a28a9f1271441d8576b381f2c67eb` |

The configuration also cross-checks the acceptance record directly: it may not
allow more provider tasks, corrections or retries than the accepted run used, may
not name a transport or model the accepted run did not use, and may not claim
review independence or recovery authority the accepted run did not demonstrate.

## What stays provisional

1. Both lanes run on the same provider, so review independence is session-level,
   not provider-level.
2. The exact served model is not proven.
3. One automatic correction cycle exists in code and passes provider-free tests,
   but is not enabled here.
4. The eligible class is one bounded internal engineering issue at a time, on one
   repository and one host.
5. Recovery, release, rollback and production verification remain owner-operated.

## How to change it later

- **To stop it now:** set the configuration `revoked` in
  `initial_operating_configuration_lifecycle_v1.json`. Admission is denied
  immediately and every reason string says why.
- **To change any bound:** publish a successor record, tombstone this one in the
  registry, and record the new evidence. This record is never edited in place.
- **It retires itself** on evidence drift, transport drift, scope expansion, or
  when 2026-09-30T00:00:00Z passes.

Re-evaluation triggers already recorded: a second authorized provider becomes
available for the review lane; an accepted recovery-sensitive trial changes the
withheld authority; evidence drift in any bound source; transport drift in the
Claude Code subscription lane; any request to widen the eligible work class.

## How to check it without contacting a provider

```bash
cd server && go run ./cmd/initial-operating-configuration-check
```

It prints the configuration and lifecycle hashes, the live decision, and an
admission matrix over representative issue facts, including every owner-gated
condition. It makes zero provider calls and reads zero credentials. Exit code 3
means the configuration is not currently admitting work.

## Coexistence

This record does **not** retire
`item-14-local-r01-guarded-claude-composite-20260915`. That configuration covers
a different case family (R01-like read-only Windows repository inventory), keeps
its own window and its own lifecycle registry, and is untouched. The validator
rejects this record if it claims to supersede anything.

## Verification performed

- 9 focused Go tests: accepted-run shape, unknown-field rejection, 38 mutation
  denials, admissible-state coverage, 9 out-of-scope denials, all 12 owner gates
  plus unrecognised-condition handling, lifecycle window/revocation/tombstone
  behaviour, and end-to-end decision inside and outside the window.
- `go vet` clean on the package and the new command.
- The whole `agentqualification` package was run before and after these files
  were added; the failing-test set is byte-identical, so no new failure was
  introduced. Nine tests in that package were already failing before this change
  because historical frozen manifests pin superseded document and source hashes;
  that preservation is deliberate and is not repaired here.
