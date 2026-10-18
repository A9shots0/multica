# Item 14 local shadow workflow suite v1

Status: prepared, frozen, and provider-free fake-tested on 2026-09-14.

This suite is the next safe Item 14 layer after the sole direct Messages
attempt failed closed before provider contact. That real observation remains
terminal: provider model unobserved, zero HTTP requests, retries, tokens, and
cost, with no credential bytes obtained or artifact created. This suite does
not retry it and does not qualify or select a provider, model, role, squad, or
production configuration.

## Frozen boundary

The authoritative manifest is
`server/internal/agentqualification/local_shadow_workflow_suite_v1.json`,
schema `silver-mediums.local-shadow-workflow-suite.v1`, SHA-256
`9c8803d41b12b99fa7286ecd379d82ca3f2497bae8f181f1d4c524e8912e32be`.
It permits no provider call, credential read, tool, repository access,
production access, outbound action, or Phone Farm access. Role/model selection
is explicitly forbidden. Its fake executor is deterministic and has no network
or provider implementation.

The suite freezes nine required surfaces into four overlapping end-to-end
journeys:

| Journey | Surfaces | Required local outcome |
|---|---|---|
| Intake and decision coordination | intake, Chat, Inbox, mobile, notifications | Plan precedes delegation; one typed owner decision resumes the exact worker once; Chat-originated operational mutations fail closed; the owner-facing state is consistent. |
| Implementation and review | implementation, review, Inbox, mobile, notifications | A distinct reviewer rejects with structured evidence; exactly one automatic consolidated correction is allowed; a passing review binds the current candidate; completion remains non-live. |
| Release, regression, and rollback | release truth, Inbox, mobile, notifications | Deployed is not Live; exact release identities remain separate; regression opens an incident; rollback requires exact known-good proof and leaves the business outcome open pending correction. |
| Interruption and recovery | recovery, Inbox, notifications | Committed work resumes without repetition; incomplete work remains replayable; recovery is bounded, settles once, and cannot recursively wake itself. |

Each journey binds the exact SHA-256 of every supporting test file. Altering a
path, hash, surface, squad, capability, accountable identity, stage order,
role, artifact, correction/review limit, evidence contract, or forbidden
authority field invalidates the suite.

## Role and squad contract

The suite uses the existing controller-owned `standard_outcome_delivery` and
`cross_boundary_change` templates. The role catalog remains authoritative for
accountable outcome identities. Every execution stage resolves an accepted
frozen role artifact; verification is assigned to `quality_checker` and cannot
share the implementation role.

The four accepted v1 fixtures—Mika, Shared Windows, Product UI, and Quality
Checker—resolve only through `ShadowRoleArtifactRefV1`. That record contains
the role and immutable instruction/skill artifact identities, but has no
provider, model, reasoning, runtime, configuration, credential, or tool field.
It is a local orchestration fixture and is not accepted by guarded runtime
dispatch. V2 artifacts are validated through their existing freeze before the
same model-neutral shadow reference is produced. This is artifact and workflow
contract evidence, not runtime role qualification.

Review policy is frozen as one independent review, one consolidated correction,
no second review, one automatic correction, and at most two review attempts.
The squad catalog's historical `max_correction_cycles: 2` is not interpreted as
permission for two automatic corrections.

## Evidence contract

Twelve typed evidence contracts keep these proof lanes distinct:

- controller state;
- local code;
- remote Git;
- deployed revision;
- observed runtime;
- business outcome.

The fake suite fills required fields only with deterministic fixture values.
It never upgrades local evidence into remote, deployed, runtime, or business
truth. Item 13 external custody remains `secondary_copy_needed`; no Protected
state is fabricated.

One fake run produces 22 exact, ordered stage receipts. Receipt IDs and replay
keys are unique, the suite hash and all role/squad/stage/evidence identities
must match, and side effects plus all seven forbidden counters must remain
zero. Altered, missing, duplicate, or replayed receipts fail closed. The focused
suite, including concurrent role-artifact and exact manifest validation, passed
20 repetitions.

## Real local harness evidence

A fresh disposable PostgreSQL 17 database was migrated through 557. Against
that isolated database:

- all workflow-controller tests passed, including plan-before-delegation,
  exact owner-decision resume, independent review, bounded correction, and
  no Mika relay;
- Chat task attempts to update an issue, create a comment, or post a workflow
  event returned conflict and left the issue, comments, and events unchanged;
- one Current Picture notification admission created one durable outbox item,
  its semantic replay created none, dry-run delivery recorded exactly the
  started and delivered attempts, and a second worker pass found no work;
- delegated-failure recovery and settlement tests passed, including bounded
  retries, replay, terminal settlement, pending preservation, and no recursive
  recovery wake.

The disposable container was then removed and its exact-name absence was
verified. It contained test-only data and is not recoverable.

The pure workflow-state, command-loader, projection, and notification-policy
packages passed. The bound owner-control tests passed in Core (2), Views (3),
and Mobile (12). Focused Go vet and scoped diff checks passed. An initial root
Vitest invocation used the wrong package setup; it was corrected by running
each package from its own configured workspace and is not treated as a product
failure.

## Independent review and correction

One independent read-only review returned REVISE. It found that nine isolated
surface labels would not prove complete workflows, that v1 artifacts could not
be treated as guarded runtime bindings, and that direct Chat-mutation plus
notification admission-to-dry-run coverage was missing. It also clarified the
single-correction limit and preserved Item 13's open external-custody state.

One consolidated correction replaced the label-only design with the four
journeys above, added the model-neutral shadow artifact bridge, froze each
journey to exact harness hashes, added the two missing database-backed guards,
and expanded fail-closed mutation tests. No second review loop ran.

## Evidence boundary and next gate

This proves that the complete local shadow suite, role/squad assignments,
evidence fields, replay rules, and supporting provider-free application paths
are prepared and internally consistent. It does not prove a real provider
model boundary, deployment, observed production runtime, or business outcome.

The positive exact-model real proof remains owner-dependent. The prior grant
is exhausted, so no credential source, network transport, or provider may be
used without a new separately bounded authorization. Other provider-free Item
14 work may continue. The upstream/fork-maintenance audit remains separate and
was not started.
