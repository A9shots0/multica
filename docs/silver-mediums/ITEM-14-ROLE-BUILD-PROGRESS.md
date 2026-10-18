# Item 14 role and squad build progress

## Trial 2: executor strong, same-provider reviewer still misses system facts — 2026-09-16T19:50Z

Executor: five for five on the first attempt, including a correct judgment call
to fail closed on a credential response against the general fallback rule.
Reviewer: listed out-of-scope changes as now required, and caught an inaccurate
comment, but passed a schema that contradicts the server's actual response for
chat tasks. Across both trials the reviewer's misses are claims about the real
system, not about the diff. No role, model or seat selection changed.

## Revision for the second executor/reviewer trial — 2026-09-16T18:30Z

The reviewer role now has to list every unrequested change, because in the first
trial it passed one without saying so. The harness, not the reviewer, now proves
mechanically that each test catches its defect. No role, model or seat selection
changed. The second trial has not started.

## Executor and reviewer roles measured on real work — 2026-09-16T09:05Z

Five real code-changing tasks ran through the implementation lane. The
**executor** role passed all five on the first attempt of its final run, with no
correction, including two judgment tasks where no test defined correct
behaviour. On one it rejected a false premise written into the code's own TODO.
It also made one unrequested change, which was verified and kept.

The **independent reviewer** role, running as a separate session on the same
subscription, caught one real defect no test could see, but approved a change on
a false factual premise, missed a false documentation guarantee, and did not flag
scope creep. Session-level independence is useful and not sufficient; the next
configuration should add deterministic claim checks or a second provider for
review.

No permanent role, model or seat is selected by this result.
Full record: `IMPLEMENTATION-LANE-20260916.md`.

## The role layer can now change code — 2026-09-16T05:50Z

**Item 14's roles were never able to perform engineering work.** Traced in
source this session: the accepted publication workflow's provider child runs
with no tools and no repository access, its broker may only publish a frozen
pre-approved document or record a review of that publication, and every accepted
end-to-end "implementation → review" proof used a **fake** provider. The
executor role existed structurally; it had never executed anything.

A new lane closes that gap without touching the Item 14 safeguard framework.

Role shape in the implementation lane, unchanged from what the evidence
supports:

- **Executor** — one task-scoped session in an isolated detached worktree, with
  `Read`, `Grep`, `Glob`, `Edit`, `Write` and `Bash` restricted to this
  repository's own check commands. Confined by the provider CLI's `--restricted`
  mode and by an after-the-fact containment check against the task's allowed
  paths.
- **Independent reviewer** — a separate session with read-only tools. It cannot
  edit the candidate it is judging.
- **No permanent seats.** Both roles are task-scoped and end with the run.

The forbidden-capability vocabulary is the one the accepted benchmark records
already used: `browser`, `git_push`, `deployment`, `external_messaging`,
`credential_access`, `phone_farm`, `host_files_outside_workspace`. It was
benchmarked in the R01–R04 realistic groups and had never been wired to a real
provider run; it is now.

Authority remains **eligibility policy, never standing permission**: minted
fresh per run with a nonce and expiry, checked before every session, revoked at
cleanup, and recording production, deployment, outbound, Phone Farm,
cross-repository and permanent-selection capability as false.

First proof: run `20260916T054524Z-3f9f7e6f` fixed six genuinely missing chat
translations, passed on the first attempt with no correction, touched 3 files
and none outside scope, and cost $0.3521. Two harness failures before it are
preserved and were not retried into a success.

The lane is frozen in `scripts/implementation-lane/FROZEN.json`. A five-task
repeatability trial is in progress, including one **judgment** task — the
workspace-leave known debt at `CLAUDE.md:179` — where no test defines correct
behaviour and the executor must decide, write the test, then fix it. That task
is the first real exercise of reviewer independence: with no test to arbitrate,
the reviewer's opinion is the only check, and both lanes share one subscription.

This selects no permanent role, model or seat, and grants no production,
deployment or outbound authority. Full record:
`IMPLEMENTATION-LANE-20260916.md`.

## Controlled operational adoption — 2026-09-16T03:12Z

**Item 14's role work now has an operating configuration, not just evidence.**
The accepted publisher → independent reviewer workflow is usable for one
owner-started internal engineering issue at a time, under a record the
controller can check and the owner can revoke.

Machine record: `server/internal/agentqualification/initial_operating_configuration_v1.json`
(SHA-256 `dadf6a215f9ed8fb6d4ed5b957a6be143e3226dab080452c4c9d63454aaffa6d`),
with `initial_operating_configuration_lifecycle_v1.json` as the controller-owned
registry and `native_publication_acceptance_v1.json` as the immutable
transcription of the accepted run. Owner-facing record:
`INITIAL-OPERATING-CONFIGURATION-20260916.md`. Result:
`ADOPTION-MILESTONE-RESULT-20260916.md`.

What the role layer gained:

- The accepted run is now a first-class embedded record that states its own
  limits — distinct reviewer *session* but not a distinct provider, no
  recovery-sensitive authority demonstrated, no exact served model proven, and no
  authority to select a permanent role or model. The validator rejects any
  configuration that reads more into it than that.
- The configuration binds seven evidence records by hash and cross-checks the
  acceptance record directly: it may not allow more provider tasks, corrections
  or retries than the accepted run used, may not name a transport or model it did
  not use, and may not claim independence or recovery authority it did not
  demonstrate.
- Role shape is unchanged from what the evidence supports: task-scoped intake,
  a shared Windows executor and a distinct task-scoped quality checker, bound to
  the same `role-artifact.v1` bindings as configuration v2. **Zero permanent
  agent seats** are created; the conditional packets, the Mac-only external roles
  and the skill-only packet are untouched.
- Recovery-sensitive authority is withheld and pinned to the V38 R04 freeze, so
  the benchmark closure's disqualifying finding is now enforced by a record
  rather than remembered in prose.

This does **not** retire `item-14-local-r01-guarded-claude-composite-20260915`.
That configuration covers a different case family, keeps its own window and its
own registry, and the validator rejects this record if it claims to supersede
anything.

One independent review returned REVISE with 15 findings; all were addressed in
one consolidated correction, including four record-honesty defects: an
unenforced gate described as enforcing, a revocation that a two-line edit could
undo, a live configuration with no recorded activation, and a failed review with
no declared destination. `provider-unavailable: orca runtime_unavailable |
fallback: in-session independent agent instance | lane: independent review`.

Deployed to the private instance as `60fea34b9770-local-24b0121ef7a0` and
verified. No provider call, no credential read, no dispatched issue, no
permanent selection.

## Native publication acceptance PASSED — 2026-09-16T01:24:29+00:00

**Item 14's native-visibility gap is closed.** A real issue was completed
automatically end to end: an owner mention started one publisher turn, its pass
emitted `implementation_ready`, the workflow routed one independent reviewer on
its own, its pass emitted `review_passed`, and issue #40
`01a0a7a4-2415-7ada-9a47-667a5bc6a094` settled to Done consistently across Issue,
Current Picture, Now, Inbox and Board. No operator assistance, no technical stop,
no retry, no correction call.

This is what the seven earlier SILV-38 runs and the single SILV-39 run never
reached. The blocking defect was the summary guard, not the topology: SILV-39's
publisher returned a valid pass with a 926-byte summary against a 600-byte
ceiling, so nothing published and no reviewer ran. The corrected instruction asks
for one ASCII sentence of 10–400 characters and keeps the 600-byte hard ceiling.
The publisher wrote 269 characters, the reviewer 300.

Role behaviour observed in this run:

- The three temporary roles (planner, publisher, reviewer) were bound by exact
  ID into the broker, with planner and reviewer suppressed from incidental
  comment routing at launch.
- Only the publisher and reviewer slots are reachable; a leader, correction or
  retry call is rejected by `roleSlot`. Neither was attempted.
- The publisher verified rather than rewrote. The broker published the unchanged
  candidate bytes and digest under the native `parent_id`; the published body is
  byte-identical to the approved candidate.
- Provider frames contain only `thinking`, `text` and `result` — no tool use, no
  MCP, one turn per slot.

Harness: `scripts/production-pilot/completion-acceptance-v3/`, a binding-only
derivation of the reviewed v2 baseline (every file reproduces byte-for-byte by
substitution). One further binding correction was needed: the authority helper
pinned the superseded v1 issue title, which no v2/v3 run could satisfy.

All temporary bindings, profiles, credential copies, runtimes and agents were
revoked and removed afterwards, verified `fully_clean: true`. Matching recovery
closed on run `20260916T011317Z-a6a72487`.

Full report: `NATIVE-ACCEPTANCE-RESULT-20260916.md`. Evidence:
`.qualification-evidence/native-acceptance-v3-20260915/`.

Next: no permanent role or model selection is authorized by this acceptance, and
no further provider run is in scope. Any scope expansion needs separate
applicable authority.

## Fresh bounded native trial v2 STOPPED by owner — 2026-09-15T23:02:18.159389+00:00

**Current outcome: authorization revoked before any live setup or provider call.** Zero publisher/reviewer runs, zero new issues/agents/runtimes/tokens, zero active tasks and zero online runtimes. Production remains 50 issues and 118 historical tasks on source 60fea34b9770-local-9eaa711d97b1. SILV-38 assisted completion and the failed SILV-39 observation are preserved; native automatic success remains unaccepted.

Exact interruption: copied the frozen harness to scripts/production-pilot/completion-acceptance-v2, applied only the approved summary instruction and fresh binding names, and ran provider-free checks. Thirteen Go tests plus four byte-boundary subcases and six mocked controller tests passed. The local test process finished; no temporary execution remains. prepare-resources, build-bound, daemon provisioning, authority issuance, credential copying and launch were never invoked. No temporary runtime, authority, profile or credential required removal; absence was verified. Local qualification is marked stopped_by_owner so the harness's existing qualification gate rejects execution. Preparation and evidence are retained.

### Claude-ready continuation

- Worktree: C:/Users/a9sho/.codex/worktrees/completion-20260915/SMK-multica-selfhost-pilot. Original owner/A: worktrees and all failed histories remain preserved.
- Read .qualification-evidence/native-acceptance-v2-20260915/interruption.json and test-stop.json first. Fresh state is .qualification-evidence/production-pilot-SILV-38-completion-acceptance-v2; it contains no issue, launch, consumed slot, authority or credential binding.
- Treat this trial as cancelled. Do not resume the prepared harness, re-enable its qualification, call a provider, start a reviewer/retry, deploy or begin another slice. Further work requires a new explicit owner instruction. The earlier fresh-run approval has been revoked.
- Latest verified recovery remains 20260915T223229Z-50923941: isolated restore, corrected/rollback application checks, OneDrive hash readback at 22:45:45Z and healthy watchdog. No production change occurred after that closure in this interrupted trial, so no additional recovery run was started.
- The prompt proposal is locally tested only. Automatic completion is still unaccepted. The earlier correction result report and SILV-39 failure evidence remain authoritative for completed work.


Status: the CEF-7 integration replay and the narrow reversible local R01-like
operating configuration v2 are accepted. The real replay used three of five
permitted tasks, passed independent review without correction, and was cleaned.
The new configuration is provider-free, expires 2026-09-22 13:15Z, and grants
no provider or credential authority. Zero production-qualified configurations
or permanent selections exist.

Next: no meaningful provider-free Item 14 implementation remains. Retain the
exact default until expiry; any later scope expansion, extension, permanent
selection, or real run requires separately applicable authority.

This is the short canonical progress view for the final Item 14 phase. Detailed
evidence remains in the linked decision, benchmark, and trial records.

## Reversible local operating configuration v2 (2026-09-15 10:06 -04:00)

- [x] Reconciled CEF-7 with primary reliability v2, topology, role-stage,
  operating-contract, and readiness evidence.
- [x] Froze one R01-like, controller-bound, read-only Windows repository
  inventory route: task-scoped intake leader -> shared Windows executor with
  inline architecture assurance -> distinct task-scoped reviewer.
- [x] Preserved Claude Code as a composite transport observation only. Requested
  Sonnet plus allowed background Haiku is not exact-single-model qualification
  or a permanent model/provider decision.
- [x] Added a controller-owned lifecycle registry with trusted clock, one active
  v2 entry, the v1 tombstone, revocation support, and exclusive expiry.
- [x] Rehearsed 50 provider-free cases: two exact preparation permits and 48
  denials, with zero side effects or external authority.
- [x] Bound every receipt to its ordinal, unique nonce, configuration, canonical
  request, lifecycle, and evaluation time; altered, missing, reordered, renamed,
  duplicate, aggregate-drifted, or replayed evidence fails closed.
- [x] Completed one independent review. Its three material findings were fixed
  together in one consolidated correction: requester-owned lifecycle removal,
  active-config/tombstone enforcement, and exact receipt-set validation.
- [x] Downgraded Quality Checker runtime status to structural-only: CEF-7 proved
  a distinct temporary reviewer function, not permanent Quality Checker role
  qualification.

Frozen SHA-256 values: configuration
`e3a68b16ce1f6975639229e82f1a251e7e11800347738534afa6441adf41e3a2`,
lifecycle `53d0a16b2d5fd26b0549fc94f148c739944362787d5db30db8fcec4f4471fc42`,
rehearsal `d93daf50b962e5de98640ab2538568239e7c29873eba01f763d0aca54342fe4c`,
and readiness v3
`5408754c547c00c25a32cb4ef3356d944276f1e5c4dd56f9c25690317f7611a1`.

Final gates: the exact current-slice suite passed 20 consecutive runs; the
workflow projection/state, guarded daemon, controller success adapter,
authority command, configuration command, v3 stewardship, vet, launcher,
format, JSON, hub, and preservation checks pass. Stewardship v3 covers 60
sources (44 immutable predecessors plus 16 current-slice sources). The full
historical qualification package retains known frozen-source failures and was
not falsely reported green.

## Fresh real integration replay (2026-09-15 08:54 -04:00)

- [x] Preserve `CEF-6` as the complete provider-free precursor.
- [x] Confirm the frozen temporary Claude/Sonnet/Medium integration binding,
  accepted documented background-Haiku limitation, leader/executor/reviewer
  path, conditional correction/re-review, five-task global ceiling, one attempt
  per task, and zero provider retries.
- [x] Create and prepare one fresh v3 replay issue and fresh exact authority.
- [x] Copy the approved credential opaquely once into the isolated task-local
  profile, start the isolated daemon, and pass real-profile prepare-only with
  zero stored receipts and zero provider starts.
- [x] Issue the three exact controller receipts.
- [x] Dispatch the workflow once and monitor the controller-owned path without
  manual provider retry or fallback.
- [x] Verify database accounting, frozen report evidence, Current Picture,
  Board, Issue, and Inbox truth.
- [x] Revoke every receipt, remove the credential and daemon identity, stop the
  isolated runtime, and verify terminal absence regardless of result.
- [x] Update all canonical records and re-run stewardship/integrity checks.

Prepare result: `CEF-7` is `8f3111b6-c9b5-4919-986a-6a043837cdb3`; the
fresh real helper SHA-256 is
`26bde00be5ed07d7a0692dd51a6625039e798ba9a343df6f1ee925e2246b3d0b`.
Pre-custody accounting was zero tasks/receipts/revocations/grants/launches and
zero daemon tokens. The opaque copy and task-local daemon provisioning then
completed, the daemon registered the exact Claude runtime, and prepare returned
three prepared / zero stored / no provider start. Credential bytes and metadata
other than regular-file shape were not inspected.

Terminal result (2026-09-15 09:15 -04:00): `CEF-7` completed leader, executor,
and independent-review tasks once each at attempt 1/1. The reviewer returned
`ITEM14_REVIEW_PASS`; no correction or re-review was created, so three of the
five permitted provider-backed tasks were used. Database readback shows three
receipts/grants/launch consumptions, three distinct package hashes, one daemon,
two accepted events, zero task messages/tools, zero attempt drift, and no
queued/running work. Board, Issue, Current Picture, and owner Inbox agree on
Done and verified completion.

The report passes 10/10 roots and 444/444 path checks and is preserved under
`.qualification-evidence/item14-integration-replay-20260915-v3/real-claude-replay-report`.
Claude Code disclosed three Sonnet and three background-Haiku usage rows totaling
13,696 accounted tokens; database cost is unreported and the Issue UI estimate
was `$0.02`. This is composite subscription-path evidence, not exact-single-model
qualification. All receipts are revoked and credential/profile/runtime/process
absence is verified; no further provider call is authorized by this replay.

Final stewardship passes through both the embedded test and executable checker:
`valid:true`, 47 bound sources, 31 immutable predecessor sources, 16 extension
sources, and three mutable current-status sources. Immutable authority,
machine-contract, and accepted-evidence hashes were not rebound.

- [x] Close open-ended model shopping and preserve the benchmark evidence.
- [x] Identify unresolved role, ownership, documentation, and authority gaps.
- [x] Reconcile the active architecture documents: local foundation accepted;
  production agents remain unqualified.
- [x] Draft immutable candidate instructions and skills for Mika, Quality
  Checker, Product/UI, and the shared Windows executor.
- [x] Hash-bind the first testable instructions, skills, and mechanical
  permission catalog for all four candidate role families.
- [x] Prepare the complete first role-qualification case group before any model
  calls.
- [x] Adversarially validate every case and grader with strong passes, subtle
  failures, paraphrases, stale evidence, false completion, and scope attacks.
- [x] Run one independent review of the complete group, apply at most one
  consolidated correction, then freeze it.
- [x] Compare the three carried configuration classes on the frozen role cases:
  the strong paid anchor validly failed all four role bindings; the bounded
  subscription candidate and cheap/free floor remain incomplete because their
  transports did not yield a complete comparable set. No candidate advanced.
- [x] Repeat only survivors to measure reliability; no configuration survived
  this role screen, so no reliability or higher-effort calls were justified.
- [x] Decide the responsibility placement boundary: deterministic controller,
  reusable skill, standing responsibility, or conditionally invoked function.
  This classifies work but does not create permanent seats or select models.
- [x] Build and replay the provider-free responsibility safeguards, then apply
  the one consolidated correction from their final independent review.
- [x] Create the remaining exact instruction and skill bundles for roles that
  survive the responsibility tests.
- [x] Wire immutable configuration and capability checks into the opt-in guarded
  dispatch, claim, daemon, tool, skill, environment, credential, and
  consequential-action path; prove denied configurations cannot act. Legacy
  nil-envelope behavior remains unchanged and no production configuration is
  qualified.
- [x] Run complete local Multica shadow workflows across intake, implementation,
  review, release truth, recovery, Inbox, Chat, mobile, and notifications.
- [ ] Qualify and select the final agent configurations and permanent-seat
  shape, then publish the final roster/model-effort assignments. The squad,
  budget, documentation-stewardship, rollback, and regression contracts are
  frozen provider-free, but the completed single-run comparison did not
  qualify a configuration. All 12 exact trial calls are consumed. The next
  step is the owner choice in `ITEM-14-FINAL-CONFIGURATION-DECISION.md`; this
  progress file grants no provider, credential, selection, or production
  authority.

Completed comparison gate: the frozen role screen closed without an advancing
configuration. No model or provider call is authorized by this progress file
alone.

Provider-capable preparation: execution v4 preserves v3 and adds only explicit
12-cell configuration-class batches. Its fresh read-only preflight binds the
exact runtime, no-fallback route, credential state, and OpenRouter spend
baseline; OpenCode uses the proven v33 structured loopback path and Claude uses
the proven restricted subscription stdin path.

First live v4 attempt: the free Nex class passed exact-route preflight, then its
first Product/UI cell exceeded the wrapper HTTP timeout before an authoritative
response was sealed. The write-ahead claim is preserved at
`.qualification-evidence/role-qualification-v4`; no observation is scored and
the model is not penalized. V4 correctly halted on the stranded claim, but that
behavior is too broad for the owner's accepted campaign policy: an isolated
transport imperfection must invalidate only that observation and allow
unaffected cells to continue. V4 remains immutable halted evidence. A minimal
versioned transport-receipt correction is required before more provider calls;
it must retain zero retries and must not change the frozen cases or graders.

Live comparison update: v5 freeze
`cbe3d5125cb28b716c0db5423b223db1b192aaea21ec73a7925f9c46e4c90c8b`
correctly recovered the legacy v4 claim without a call and continued through
isolated timeouts. Nex produced one scoreable transport receipt among six
attempted provider calls before a local OpenCode readiness failure halted that
evidence root; Nex remains incomplete, not selected. Claude subscription made
one call whose Claude Code envelope reported both the requested Sonnet model
and auxiliary Haiku usage, so the exact single-model receipt gate rejected it;
that configuration remains incomplete pending transport classification.
Sonnet 5 through the exact OpenRouter route made four calls for 33,048 tokens,
106,269 cash micros, and 95,280 ms, but all four were incorrectly normalized as
protocol failures because the adapter checked for outcome `completed` while the
validated OpenCode parser's successful outcome is `structured`. These are
invalid harness observations, not model failures. V5 evidence remains
immutable. A narrow v6 adapter correction must rerun only affected observations
under new evidence identities; cases, graders, prompts, model routes, and the
zero-retry policy remain frozen.

V6 closure: execution v6 freeze
`f0fd42f0da64ef38ca59090f19e2c07484feffd24c6e122268b49d044211868b`
corrected only that adapter defect and preserved the frozen role cases,
graders, prompts, routes, and policies. The fresh Sonnet 5 rerun made four
exact-route calls for 31,612 tokens, 91,871 cash micros, and 79,138 ms. Each
role-family observation ended in an authoritative OpenCode
`StructuredOutputError`, so all four are valid candidate-local protocol
failures and none advances. The run had no transport receipts, no global halt,
and no retry. Nex remains incomplete from isolated timeouts/readiness failure;
Claude subscription remains held because its otherwise complete response
reported unmodeled auxiliary Haiku usage alongside requested Sonnet usage.
Neither incomplete configuration is scored as a competence failure, and no
additional paid call is justified merely to force a winner.

Responsibility-placement result: two independent read-only challenges agree on
one issue, one accountable outcome, one primary executor per host boundary,
optional named-boundary contributors, conditional release, and one independent
outcome verdict. Repository knowledge stays in selectively loaded skills.
Phone Farm Application and Fleet remain distinct Mac-only standing boundaries.
Mika remains the owner interface, not workflow orchestration. Documentation
truth remains versioned infrastructure, not a permanent documentation agent.

Deterministic responsibility evidence is now implemented and replay-tested:
cross-issue consequence receipts preserve separate issue scope/state/authority;
the permanent regression registry binds contribution, independent approval,
supersession, retirement, and scoped graph evidence; and the stewardship
manifest/checker detects graph, role-catalog, and maintained-source drift. The
real checker currently validates the maintained hub sources. These foundations
are not yet database-backed or connected to live dispatch.

Final safeguard review: REVISE with four P1 findings and no P0. One consolidated
correction bound consequence creation to authenticated controller authority and
current durable snapshots; required complete controller-bound regression impact
scope; made rejected regression attempts restart-replayable and tamper-evident;
and expanded stewardship from 2 summary files to the complete 20-file authority
set derived from the project registry. The combined tests, vet, diff check, and
real 20-source manifest check pass.

Remaining bundle result: seven candidate-only instruction/skill pairs now cover
Phone Farm Application, Phone Fleet Reliability, Data Sync, Release/Runtime,
Company Auditor, Architecture Assurance, and Security/Privacy. Mac alert
analysis is skill-only. Nine accountable outcome identities remain controller
configuration, not agent seats. Content Inspiration and Telnyx now have
separate machine identities and capability contexts. The artifact set is
hash-bound, rejects unlisted files, records one independent REVISE/no-P0 review
and one consolidated correction, and intentionally remains denied by live
configuration resolution until the next separately versioned binding gate.

Current next gate: thread verified guarded scope into daemon task setup, then
close lazy-skill and environment/credential-purpose exposure. This must not
convert historical provisional model labels into selections.

Runtime enforcement checkpoint (2026-09-13 16:49 -04:00): the first half of
that gate is implemented. Exact model-neutral v2 artifact bindings and an
opt-in guarded envelope now fail closed before correction/review/release task
insertion and at the start of the shared single/batch claim builder. Since no
runtime/model binding is qualified, guarded work is denied before tools, MCP,
skills, environment, instructions, or credentials are exposed; rejected claims
are terminally settled. Legacy tasks remain unchanged. Focused tests and vet
pass. The final independent review returned three P1 findings and no P0. One
consolidated correction added exact workspace/issue/agent/runtime binding,
Observe-path rejection, and a typed controller attachment boundary. Normal
tasks are not automatically stamped because no runtime/model configuration is
qualified yet; claiming otherwise would be false. Tool-gateway calls, lazy
skill reads, environment/credential-purpose filters, and the raw-shell
limitation remain, so the parent checkbox stays open. No second review loop was
run.

Tool-gateway checkpoint (2026-09-13 17:16 -04:00): a shared model-neutral
capability validator now guards Remote MCP and plugin-hook execution when a
guard is supplied. Remote MCP denies mismatched tools and derived credential
purposes before secret resolution, discovery, refresh, or upstream calls.
Plugin hooks deny before invocation using independently supplied live
task/runtime facts and a controller-mapped destination for the selected tool.
Nil guards preserve legacy tasks. Consequential raw shell remains fail-closed.
Focused combined tests, vet, and diff checks pass.

The final independent review found one P1 and no P0: plugin actual identity and
targets were initially copied from the authorization receipt. The single
consolidated correction separated receipt authority from gateway facts and
added live-identity and destination-drift coverage. No second review loop was
run. The parent runtime-enforcement gate remains open: positive guarded scope
is not yet threaded through daemon task setup, no runtime/model configuration
is qualified, and lazy-skill plus environment/credential-purpose exposure
still require equivalent checks.

Authenticated transport finding (2026-09-13 17:24 -04:00): a positive guarded
execution package cannot be truthfully issued yet. Durable controller loaders
for qualified runtime and capability receipts, exact plugin target bindings,
and authoritative daemon/host identity are missing, and final model/effort
resolution currently occurs after tool servers start. The next implementation
must add a typed claim-wire package that stays fail-closed, then validate exact
transport identity before tools, skills, custom environment, or credentials
are exposed. It must not convert benchmark evidence into production authority.

Skill/environment mapping confirms four later checks: pre/post-resolution
skill identity and hash; custom-env key to credential-purpose filtering;
guarded rejection of credential-bearing direct MCP; and reduced inherited host
environment/profile access. Consequential native shell/file execution remains
denied until process isolation or OS sandboxing exists.

Typed claim-wire checkpoint (2026-09-13 17:36 -04:00): the model-neutral
`guarded_execution` package is now separate from workflow prose and binds the
dispatch/runtime receipt to the capability scope by exact receipt ID. It
validates independent workspace, issue, agent, runtime, provider, model,
effort, host, role, configuration, and plugin-destination facts. Mirrored
server/daemon fields exist, but no claim producer populates them before durable
controller loaders and a real qualification decision exist.

The final independent review found one P1 and no P0: permissive claim JSON
decoding could discard unknown authority fields. The single consolidated
correction added strict guarded-field decoding and wire tests for unknown,
null, duplicate, malformed, and trailing packages. Focused tests, vet, and diff
checks pass. No second review loop was run. Next: controller-owned durable
receipt/capability loading, then daemon pre-exposure validation and
skill/environment filters.

Durable permission-receipt checkpoint (2026-09-13 18:15 -04:00): immutable
qualified-runtime receipts and per-task capability grants now have
controller-only storage, exact scoped loaders, canonical payload hashes,
separate revocations, and idempotent exact replay. Revocation cannot be used
against an unknown or wrong-workspace target, and an existing tombstone counts
as success only when controller, scope, reason, and timestamp exactly match.
Database triggers deny both UPDATE and DELETE, preventing removal of a
revocation from silently restoring authority.

Final independent review: REVISE with two High findings and one Medium proof
gap. One consolidated correction closed all three; no second reviewer loop was
run. Proof passed in unit/adversarial tests and against a disposable real
PostgreSQL database, including duplicate issuance, one-grant-per-task
uniqueness, update/delete rejection, wrong-scope revocation denial,
two-connection concurrent exact revocation, and active-load exclusion after
revocation. Full package tests, focused migration tests, generated SQL
compilation, vet, and scoped diff checks pass. The Windows build could not run
Go's race detector because CGO is disabled; this is recorded as a tooling
limitation, not silently treated as proof.

The runtime-enforcement checkbox remains open. No qualified runtime/model
receipt has been issued, the claim producer does not yet attach the durable
package, and skills, custom environment, credentials, and native shell/file
access still require the next pre-exposure enforcement slice. No provider,
Browser, production, outbound, credential, model selection, or Phone Farm
action occurred.

Provider-free execution readiness: execution v2 is preserved as rejected
NOT READY evidence. The single consolidated correction is execution v3: a
separately frozen 36-cell counterbalanced plan with typed role schemas,
field-specific hidden grading, conservative pre-grant reservation, raw-first
immutable receipts, raw-derived observations, stranded-claim global blocking,
full evidence/accounting replay, and exact-byte validation of commands and all
four transport antecedents. Only `plan`, `validate`, `status`, and `replay`
exist. Its provider gate remains disabled until a separately authorized fresh
transport preflight; no comparison observation has run yet.

Runtime enforcement inventory: guarded task envelopes, pre-insert/Observe
admission, shared claim-time rejection, and enforcement-capable Remote MCP and
plugin-hook gateways are implemented and tested. Positive guard plumbing into
daemon task setup, lazy skills, and environment/credential-purpose exposure
remain. Consequential execution stays broker-only until raw shell actions have
an equivalent interceptor. Legacy tasks remain unchanged when no guarded
envelope or guard is present.

Primary references:

- `ITEM-14-BENCHMARK-CLOSURE.md`
- `ITEM-14-ROLE-DESIGN-DECISION-REGISTER.md`
- `ITEM-14-ROLE-INSTRUCTION-CONTRACT.md`
- `ITEM-14-FINAL-REVIEW-RESULT.md`

Pre-exposure protection checkpoint (2026-09-13 18:48 -04:00): durable
runtime and capability authority is loaded before the shared claim builder
exposes agent material, rechecked immediately before finalization, and
reloaded inside the final database transaction before any task token, daemon
token, or comment receipt is written. Guarded claims contain only exact
embedded instructions and one inline embedded skill. Mutable or lazy skills
and every unbound environment, argument, MCP, runtime-config, connected-app,
Remote MCP, and plugin surface are denied.

The daemon validates task, runtime, provider, model, effort, and the separately
configured `MULTICA_HOST_ATTESTATION` before repository registration. It then
deliberately returns `guarded_environment_not_isolated`: current provider
processes inherit the daemon HOME/profile and retain native shell/file access,
so positive guarded execution is not yet safe.

One independent review found the transaction window, raw-artifact-versus-
bundle hash mismatch, mutable role construction, lazy-skill route, and missing
pre-repository daemon gate. One consolidated correction addressed them; no
review loop was run. Focused tests, vet, and scoped diff checks pass. At the
owner-reported 3% remaining usage, no new slice may start. Next is a proven
allowlisted child environment and tool-free or sandbox boundary. No final
agent, model, role, or production permission was selected.

Isolated-child implementation checkpoint (started 2026-09-13 20:01 -04:00):

- [x] Re-read the authoritative continuation, this progress record, the latest
  run journal, repository rules, and Silver Mediums hub state without changing
  or reclassifying accepted Items 1-13.
- [x] Map the pre-repository daemon gate, runtime receipt, claim exposure,
  backend environment, and Claude/OpenCode tool controls. This found one
  additional unbound input: guarded `service_tier` must remain empty until a
  future versioned receipt binds it.
- [x] Bind an exact child environment and tool-free launch policy into durable
  runtime authority, with host attestation retained by the daemon parent.
- [x] Enforce the bound policy before repository registration and provider
  setup; preserve nil-package legacy behavior.
- [x] Prove environment non-inheritance and tool denial with local fake-child
  tests only, then run the broader Go verification gates.
- [x] Obtain one independent review and apply at most one consolidated
  correction, with no review loop.
- [x] Update the continuation, run journal, and Silver Mediums hub with exact
  evidence boundaries. No model testing, model/role selection, provider call,
  credential read, Browser/Orca, production, outbound, or Phone Farm action is
  authorized by this slice.

Focused proof checkpoint (2026-09-13 20:35 -04:00): guarded runtime receipts
now bind the executable SHA-256, host OS, exact environment-name SHA-256,
provider adapter, `tool_free_v1`, and `daemon_parent_only_v1` attestation
custody. The daemon verifies those facts before repository registration,
rejects custom runtime commands, skips provider model-catalog probing, and
builds fresh task-local HOME/XDG/config/temp roots without reading a task token
or exporting any `MULTICA_*` or provider credential. Claude uses its proven
restricted/empty-tools/empty-MCP/no-session flags; OpenCode uses `--pure` plus
an exact deny-all inline config. Either adapter fails the task if a tool event
appears despite prevention. Guarded service tier remains explicitly empty.

Local focused tests pass for `pkg/agent`, `internal/agentqualification`,
`internal/daemon`, and `internal/handler`, including an actual fake child
process environment readback. These tests made no provider call and used no
credential. Broader tests and the one independent review remain pending.

Isolated-child accepted checkpoint (2026-09-13 20:56 -04:00): the required
independent read-only review found no P0, three P1 gaps, and one P2 proof gap.
The one consolidated correction moved the child-bound runtime receipt to v2
(legacy v1 is explicitly ineligible), stages and independently rehashes the
opened provider executable into a task-owned read-only path before use, and
hard-stops the owned process tree on the first observed tool event. New fake
Claude and OpenCode children now cross each real adapter `Execute` path and
prove exact environment values and names, empty task-local cwd/profile roots,
preventive argv/config, host-secret non-inheritance, symmetric tool-event
failure, and absence of a delayed second action. No second review loop ran.

This closes the requested isolated allowlist plus model-only/tool-free adapter
slice. It does not claim an OS syscall/filesystem/network sandbox: tool freedom
is prevented by the provider flags/config and enforced observationally with an
immediate process-tree kill if a tool event appears. Consequently no native
shell/file capability or brokered tool/credential is granted. A future positive
path additionally requires a controller-issued `qualified-runtime-binding.v2`
receipt matching the staged executable, host, adapter, exact environment-name
set, model, and effort; none exists and none was selected or issued here.

Post-correction proof: full `internal/agentqualification` and
`internal/handler` packages pass; focused guarded tests pass in `pkg/agent`,
`internal/daemon`, and the migration package; `go vet` passes across
qualification, daemon, handler, service, and agent packages; scoped diff checks
have no errors. The unfiltered daemon, agent, and service packages retain their
documented machine/ambient baseline failures (Windows executable fixtures,
installed CLI/identity/workdir behavior, ambient local skill/MCP discovery,
and four built-in-skill conformance checks). No changed guarded test failed.
No provider/model call, credential read, role/model selection, Browser/Orca,
production/outbound action, or Phone Farm access occurred.

The Silver Mediums hub current-state entry is updated and its maintained-source
receipt is rebound. `python scripts/check_hub.py` passes with its existing stale
claim/dated-snapshot warnings, and the real 20-source stewardship check returns
`valid:true`, graph SHA
`ff4119b1d70ced0a943f8dd056a1f3851e24c975ebacfad37dfc76d9aa75d1fa`,
role SHA
`c89c5f619c5f1a724e255b9014bfc882295a2c00504191bfc48569a5bbcc63e2`.

Controller authority lifecycle and provider-free shadow checkpoint (started
2026-09-13 21:19 -04:00):

- [x] Re-read the authoritative continuation, Item 14 progress, newest run
  journal, repository rules, and Silver Mediums hub without reopening accepted
  Items 1-13 or the completed role/model evaluation.
- [x] Add controller-owned combined v2 issuance plus durable, append-only,
  one-shot execution consumption bound to the exact task, runtime, grant,
  receipt, package digest, and authenticated daemon.
- [x] Make revocation, expiry, alteration, duplicate issuance, duplicate
  launch, and replay fail closed at the real transactional start boundary.
- [x] Run one complete fake-provider execution through controller issuance,
  authenticated dispatch/claim, daemon staging, and the real tool-free adapter.
- [x] Run focused tests, obtain one independent read-only review, and apply one
  consolidated correction without a second review loop.
- [x] Update the canonical continuation, run journal, and hub with the exact
  provider-free evidence boundary. A real model-only shadow is separately
  gated: this slice may not call a provider, inspect credentials, spend funds,
  or select a production role/model.

Provider-free execution proof checkpoint (2026-09-13 21:55 -04:00):

- [x] Controller-owned combined v2 issuance now converges on one exact runtime
  receipt plus one task grant. Partial receipt-only state remains inert and an
  exact retry completes it; conflicts cannot produce an executable package.
- [x] Migrations 554-557 add append-only one-shot launch consumption bound to
  the exact workspace, task, issue, executor, runtime, runtime receipt, task
  grant, authenticated daemon, canonical package SHA-256, and consumption
  time. The start transition and consumption append share one transaction.
- [x] The daemon presents the SHA-256 of the exact in-memory package it will
  launch. The controller independently reconstructs and hashes its immutable
  package; mismatch, wrong daemon, expiry, revocation, or an existing
  consumption rolls the task start back before child launch.
- [x] One fake-provider shadow crossed the real authenticated dispatch/claim,
  handler/client start, daemon preparation and executable staging, and Claude
  tool-free adapter. Exactly one approved package produced exactly one local
  fixture launch and `provider-free-ok`; altered package, wrong package digest,
  expired runtime authority, revoked runtime qualification, and replay
  produced no additional launch. No network/provider implementation or
  credential was available to the fixture.
- [x] Disposable PostgreSQL 17 proof passed through migration head, the real
  permission-store contract, and the fake-provider shadow; its container was
  removed. Full qualification and migration suites pass. The broad unfiltered
  agent package retains its documented unrelated Windows/ambient fixture
  failures; focused guarded adapter tests remain the applicable lane.
- [x] One independent read-only review completed and its findings were handled
  in one consolidated correction with no second review loop, followed by final
  focused verification and canonical/hub acceptance updates.

The separately bounded real model-only shadow remains unstarted. It requires
provider/credential/subscription or spending authority and a model/role choice,
all explicitly prohibited by the current slice. Fixture-only provider, model,
effort, role, and configuration strings are test data and are not selections.

Controller authority lifecycle accepted checkpoint (2026-09-13 22:10 -04:00):
the independent read-only review returned REVISE with no P0, two P1 findings,
and one P2 proof gap. It found that controller configuration loss after claim
could remove the start hook, the consumed package digest was not repeated at
the final child-launch boundary, and the shadow used only an internally
inconsistent alteration while omitting an expired-path zero-consumption
assertion.

The one consolidated correction installed legacy-preserving claim/start gates
even when controller configuration is absent, repeated the canonical package
digest immediately before `executeAndDrain`, added a self-consistent but
unapproved evidence mutation that reaches controller digest denial through
`runTask`, and proved expired authority leaves zero consumption. No second
review loop ran.

Final proof is green: full `internal/agentqualification` and
`internal/handler`; focused guarded migration, service, daemon, and
`pkg/agent` tests; cross-package vet; scoped diff checks; and the final
disposable PostgreSQL contract plus real claim/start/daemon fake-provider
shadow. Exactly one approved package created one immutable consumption and one
local fixture launch. Altered semantics, coherent package drift, wrong digest,
missing controller configuration, wrong daemon, expired runtime authority,
revoked runtime qualification, duplicate launch, and replay fail closed. The
disposable container was removed.

This is provider-free local evidence, not a deployment or business outcome.
The test injects the already-authenticated daemon identity into the real
handlers; token authentication itself is pre-existing and outside this slice.
The child remains the accepted isolated profile plus preventive tool-free
provider boundary, not a general network/filesystem/syscall sandbox. No real
provider, saved credential, tool, selected model/role, Orca/Browser,
production/outbound surface, or Phone Farm system was touched.

The Silver Mediums hub now records this provider-free boundary and its
maintained-source receipt is rebound to SHA-256
`b37a44a438b0edbc6c15cc701059de0aab043fd56f7416cf7c29caa82cfb90d6`.
`check_hub.py` passes with its pre-existing stale-claim and dated-snapshot
warnings. The real 20-source stewardship check returns `valid:true`, unchanged
graph SHA
`ff4119b1d70ced0a943f8dd056a1f3851e24c975ebacfad37dfc76d9aa75d1fa`,
and unchanged role SHA
`c89c5f619c5f1a724e255b9014bfc882295a2c00504191bfc48569a5bbcc63e2`.

Real model-only shadow authority proposal checkpoint (2026-09-13 22:36
-04:00):

- [x] Chose the existing guarded Claude Code subscription transport as the
  narrowest supported route. `claude-sonnet-4-6` Medium is an exact temporary
  test fixture only; it is not qualified, promoted, or selected for a role.
- [x] Defined a disposable executor identity and one harmless, fixed Data Sync
  insufficient-evidence response in
  `ITEM-14-REAL-MODEL-SHADOW-AUTHORITY.md`.
- [x] Confirmed the current isolated child cannot silently reuse the saved
  Windows subscription because its `CLAUDE_CONFIG_DIR` is a new empty
  task-local directory.
- [x] Rejected broader credential routes: no API key or metered API lane, no
  one-year `claude setup-token`, no interactive login, and no exposure of the
  normal Claude profile directory.
- [ ] Before any real call, add and provider-free test a one-use controller
  credential capsule and an exact rendered-prompt binding. Alteration must fail
  before credential access or child launch.
- [ ] Await exact owner authority for one read-only byte-copy of
  `C:\Users\a9sho\.claude\.credentials.json`, one temporary child-lifetime
  copy, one subscription call with zero retries, and verified deletion of the
  copy.
- [ ] After that grant and a green fake preflight, run exactly one real
  dispatch/claim/daemon shadow. A pass is execution-boundary evidence only.

No saved credential was inspected, no provider/runtime process was invoked, no
receipt or grant was issued, and no tool, repository, production/outbound,
Browser/Orca, or Phone Farm surface was touched. Complete local shadow workflows
follow only after a pass. Claude's upstream/fork-maintenance audit remains a
separate pre-migration gate.

The Silver Mediums hub records this locked decision and the maintained
`CURRENT_STATE.md` source receipt is rebound to SHA-256
`af1555f3e7d1e06f08437ab347c869db066fb3f578e4276d650f23fd51cc6a32`.
The hub validator passes with its three existing stale-snapshot warnings;
system-graph and stewardship-command tests pass; the real 20-source check
returns `valid:true` with the unchanged graph and role-catalog hashes; and
scoped diff checks have no errors.

Real model-only shadow authorized precursor checkpoint (2026-09-13 23:00
-04:00):

- [x] Owner granted exactly the one-copy, one-subscription-inference,
  zero-retry, immediate-cleanup authority frozen in
  `ITEM-14-REAL-MODEL-SHADOW-AUTHORITY.md`.
- [x] Added `task-capability-scope.v2`, binding the fixed harmless payload and
  rendered immutable role-plus-prompt SHA-256 to one provider call, one turn,
  zero retries, the exact credential source/purpose/copy policy, and immediate
  deletion with verified absence. V1 cannot carry this authority; V2 must.
- [x] Focused fake-capsule tests prove one source open, opaque byte copy, and
  cleanup/absence. Altered payload/hash/source/purpose/policy/call/retry fields
  fail closed before source access.
- [x] The fake full path passes through the real disposable PostgreSQL
  issuance, claim, task-start consumption, daemon, staged executable, isolated
  environment, and Claude stream adapter. One exact package produced one fake
  launch, zero tools, one consumption, and verified credential-copy absence.
  Altered model, prompt, task identity, executable, credential source, package
  digest, revocation, expiry, duplicate start, and replay produced no extra
  launch.
- [x] Corrected Windows launcher custody provider-free: guarded Claude now
  resolves the bundled native `claude.exe` behind the npm shim before hashing
  and staging, so the isolated child does not depend on an unstaged relative
  `node_modules` target.
- [x] The build-tagged real harness compiles without executing. Its sole
  `runTask` call is one exact model-only shadow, and guarded execution is
  explicitly excluded from the daemon's fresh-session retry branch.
- [x] One independent, read-only, provider-free review returned REVISE with no
  P0 and four P1 boundary findings. One consolidated correction requires an
  explicit loopback-only disposable DSN, kills guarded control requests,
  suppresses credential-bearing diagnostic/transcript channels, preserves the
  provider-reported model/turn receipt, and places the official
  `CLAUDE_CODE_MAX_RETRIES=0` control in the exact child allowlist.
- [x] Focused control-request-first and retry-value tests, the corrected full
  fake shadow, and an exact build-tagged prepare-only real harness pass. The
  prepare-only run issued and claimed the real package, then stopped before
  `runTask` with `credential_opened=false` and `provider_started=false`.
- [x] The sole authorized real controller/CLI invocation ran exactly once. One child
  completed in one provider-reported turn with zero tools and zero configured
  failed-request retries, but the terminal receipt reported both
  `claude-sonnet-4-6` and `claude-haiku-4-5-20251001`. The exact-model gate
  therefore failed closed. No retry or second controller/CLI invocation
  occurred. The two identities prevent certifying that the CLI made only one
  underlying provider inference and are treated as unapproved background
  provider activity.
- [x] The task-local credential copy, its fresh temporary root, and the
  exact-name loopback proxy were verified absent immediately afterward. Output
  schema and usage acceptance were not reached and are not claimed.

Evidence boundary: the fake database tunnel used the existing local development
PostgreSQL only to create and remove a disposable schema; a temporary exact-name
loopback proxy was removed after each run. No shared rows, production, tools,
repository prompt, outbound action, Browser/Orca, or Phone Farm surface was
used. The temporary fixture model and existing role artifact remain
non-promotable test bindings, not permanent selections.

Terminal boundary: the requested positive single-model proof did not pass, so
complete local shadow workflows do not begin from this result. A different
real-provider test or transport change now requires new owner authority. The
separate Claude upstream/fork-maintenance audit remains required before any
migration.

Final provider-free verification is green: qualification, guarded agent and
daemon tests; integration compile-only; focused vet; hub validator;
system-graph/stewardship tests; and the real 20-source validation. The hub
current-state receipt is
`a2dda25ddb3aaf420626d33fde6276d1b7c0c985aefd7e9a5aea91d01782a9cc`.

Exact-model transport v1 provider-free checkpoint (2026-09-14):

- [x] Preserved the one real Claude Code shadow as terminal valid fail-closed
  evidence. The exhausted authority record no longer contains actionable text
  saying the real call may still run.
- [x] Determined Claude Code is unsuitable for an exactly-one-request and
  exactly-one-model boundary. Its CLI model choice is not full background-model
  enforcement, and its multi-model usage evidence arrives only after requests.
  Pinning aliases or accepting a one-model aggregate would not prove one call.
- [x] Added `silver-mediums.exact-model-transport.v1`, a provider-free direct
  Anthropic Messages candidate binding one endpoint, exact temporary model and
  prompt hash, one request, zero retries, denied redirects, no tools, a
  60-second timeout, bounded output, one opaque in-memory credential read, and
  response-body model evidence.
- [x] Added a single-use daemon harness with an exact, non-replayable POST body
  and a one-`RoundTrip` gate. Fake credentials/providers prove the success path
  and fail-closed prompt/policy/model drift, sequential and concurrent
  duplicate attempts, transport failure, redirect, tool content, server-tool
  metadata, and unknown response fields.
- [x] One independent provider-free review returned REVISE with no P0. One
  consolidated correction exhausted stale authority text, exercised a real
  redirect target, made response decoding strict with zero server-tool-use,
  and froze the injected `RoundTripper` as fake-only evidence.

Evidence boundary: the candidate is not wired to claim/dispatch and has no real
credential source or default network transport. A future real path must bind a
fresh known no-proxy/no-custom-retry Go HTTP transport and requires new owner
authority for one API-key read, one metered request, and an exact temporary
model binding. No provider, credential, role/model selection, upstream audit,
production, or Phone Farm surface was used.

- [x] Final provider-free verification passed: focused qualification tests;
  the exact-model daemon suite repeated 20 times, including its concurrent
  single-use case; adjacent guarded regressions; guarded agent tests;
  build-tagged compile-only; focused vet; hub validation; system-graph tests;
  and real 20-source stewardship validation. Go race-detector evidence is not
  claimed because this Windows environment has CGO disabled.
- [x] The final stewardship graph SHA-256 is
  `ff4119b1d70ced0a943f8dd056a1f3851e24c975ebacfad37dfc76d9aa75d1fa`;
  the rebound hub `CURRENT_STATE.md` SHA-256 is
  `16ae1ab2afab0ee6d53ef69e376d2214c6d095412ac394e21d29478543d40744`.

Terminal boundary: this provider-free exact-model transport slice is complete.
Stop before inspecting a credential source, constructing a real network
transport, activating dispatch, or making a provider call. Each requires a new
separately bounded owner grant.

Direct Messages real-proof preflight (2026-09-14):

- [x] Owner authorized one `claude-sonnet-4-6` direct request as a temporary
  fixture, capped at `$1`, with zero retries, redirects, fallbacks, proxies,
  tools, repository/production access, or outbound action.
- [x] Froze the grant in
  `ITEM-14-DIRECT-MESSAGES-REAL-PROOF-AUTHORITY.md`. The only credential source
  is the inherited `ANTHROPIC_API_KEY`; prepare-only cannot inspect it, no
  `.env` or alternate source is eligible, and no value or derivative may be
  logged, hashed, or persisted.
- [x] Tightened the direct request to 64 output tokens, standard-only/global
  serving, and a conservative 300,960-microdollar reservation under the
  1,000,000-microdollar owner ceiling.
- [x] Added the fresh known Go transport: no proxy lookup, retry wrapper,
  redirects, HTTP/2 forcing, connection reuse, compression, replayable body,
  fallback, or second executor use.
- [x] The complete fake suite passed 20 repetitions. The exact prepare-only
  harness passed with `requests=0`, `credential_opened=false`, and
  `provider_started=false`.
- [x] One independent provider-free pre-call review returned REVISE. Its
  authority concern referred to the exhausted subscription-file grant rather
  than the new explicit API-key/metered grant and required no expansion.
- [x] One consolidated correction accepts only the current documented null
  container/stop-detail and nested thinking metadata, rejects unsafe variants,
  and wipes every temporary credential byte copy on success, malformed input,
  read failure, and close failure. Corrected fake tests passed 20 repetitions;
  prepare-only and focused vet pass. The sole real call is now eligible.
- [x] The sole real-run invocation was made once and failed closed before
  provider contact because the inherited API-key mechanism was unavailable.
  Actual receipt: provider model unobserved; requests `0`; retries `0`; input,
  output, and thinking tokens `0`; cost `$0`; timing `0 ms`; no credential
  bytes obtained and no artifact created. The raw cleanup flag is `false`
  because no source opened/closed, not because material remained.

Terminal boundary: this authority is exhausted. Do not load `.env`, substitute
another credential source, or retry. The positive real exact-model boundary
remains unproved; continue only provider-free Item 14 work.

Complete local shadow suite checkpoint (2026-09-14):

- [x] Froze `silver-mediums.local-shadow-workflow-suite.v1` at SHA-256
  `9c8803d41b12b99fa7286ecd379d82ca3f2497bae8f181f1d4c524e8912e32be`.
  Four journeys jointly cover all nine required surfaces: intake,
  implementation, review, release truth, recovery, Inbox, Chat, mobile, and
  notifications.
- [x] Bound 12 typed evidence contracts without collapsing controller state,
  local code, remote Git, deployed revision, observed runtime, or business
  outcome. Item 13 external custody remains `secondary_copy_needed`.
- [x] Added `ShadowRoleArtifactRefV1` for accepted frozen v1/v2 instruction and
  skill artifacts. It contains no provider/model/reasoning/runtime/
  configuration/credential/tool field and cannot enter guarded dispatch.
  Accountable outcome bindings remain exact catalog identities.
- [x] One provider-free fake run produces exactly 22 ordered receipts with
  unique replay keys and zero provider, credential, tool, repository,
  production, outbound, and Phone Farm counters. Alteration, missing evidence,
  duplicate receipt, replay, or authority drift fails closed. The focused fake
  and artifact suite passed 20 repetitions.
- [x] One independent read-only review returned REVISE. One consolidated
  correction replaced nine label-only cases with four complete journeys,
  froze exact supporting harness hashes, added the missing Chat mutation guard
  and notification admission/outbox/dry-run/deduplication proof, and encoded
  one automatic correction with two total review attempts. No second review
  ran.
- [x] Migrated a fresh disposable PostgreSQL 17 database through 557 and passed
  the real local workflow-controller, Chat operational guard, notification
  dry-run/deduplication, delegated-failure recovery, and settlement tests.
  The container was removed and exact-name absence verified.
- [x] Pure workflow-state, command-loader, projection, and notification-policy
  packages pass. Bound Core (2), Views (3), and Mobile (12) tests pass. Focused
  Go vet and scoped diff checks pass. The initial root Vitest invocation was a
  wrong package harness; rerunning from each package's configured workspace
  passed and no product failure is claimed.
- [x] The complete contract and evidence boundary are recorded in
  `ITEM-14-LOCAL-SHADOW-WORKFLOW-SUITE-V1.md`.

Evidence boundary: this is a prepared/frozen provider-free local suite plus
real local application-path tests. It is not a real provider boundary,
deployment, production runtime, business outcome, or runtime role/model
qualification. The direct Messages authority remains exhausted after its
zero-request fail-closed result. No provider, credential, final selection,
upstream audit, production/outbound action, Orca/Browser, or Phone Farm access
occurred in this slice.

- [x] Updated the Silver Mediums hub. `check_hub.py` passes with the existing
  stale-claim and dated-snapshot warnings. The real 20-source check returns
  `valid:true`, graph SHA-256
  `ff4119b1d70ced0a943f8dd056a1f3851e24c975ebacfad37dfc76d9aa75d1fa`,
  role SHA-256
  `c89c5f619c5f1a724e255b9014bfc882295a2c00504191bfc48569a5bbcc63e2`,
  and bound `CURRENT_STATE.md` SHA-256
  `a0f99f3b2e07dcb7e1ff24d0dd2f0e2f135c7d4fbf354d2863e6cc3781bec5f4`.
  A concurrently changed Viral Queue changelog was not edited; its maintained-
  source receipt was rebound to the current file identified by validation.

Security follow-up: a local Docker diagnostic during disposable-database
isolation emitted existing non-Anthropic application environment values into
the task's tool output. No value was reused, persisted in the implementation
or documentation, or used for service access; no Anthropic credential was
obtained. Rotation of the affected local credentials requires separate
credential-change authority and remains open.

Provider boundary v2 and OpenRouter local-shadow precursor (2026-09-14):

- [x] Corrected provider ownership: Claude/Anthropic is Claude Code
  subscription-only; OpenAI/Codex is Codex-subscription-only; neither may use
  OpenRouter. The Claude Code Sonnet-plus-background-Haiku observation remains
  immutable limitation evidence with no retry or bypass.
- [x] Added a runtime-start allowlist for only the four v26 small non-Claude
  survivors. Claude/Anthropic, OpenAI, unknown, altered, and provider-mismatched
  OpenRouter routes through this additive Item 14 lane fail before executable
  lookup or credential access. Historical frozen runners remain unchanged.
- [x] Froze `silver-mediums.openrouter-local-shadow-definition.v1`, SHA-256
  `00a59b865276362a47c9c73777624ef64eb7fda55cf0f44393b1ba88235fce4a`.
  Qwen3 Coder Next via Alibaba is a temporary fixture only, not a selection.
- [x] Bound all four frozen workflows to one call each, zero retries, no
  fallbacks, a 1,200 generated-token ceiling per call, and a `$1` incremental
  cap nested inside the unchanged cumulative `$40` OpenRouter ceiling.
- [x] Fake-provider precursor passed with four simulated receipts, zero real
  provider requests, and all forbidden counters zero. Alteration, duplicate,
  replay, route drift, retry, tool, and spend mutations fail closed.
- [x] Prepare-only passed for all four plans with zero credential reads and
  zero provider calls. The merged config hash is
  `b56f1077f1a76772ab939bbdfa414267a5db98530804706d676c92420f257a29`;
  four task-local runtime roots were removed and verified absent.
- [x] The real path seals exclusive run/workflow claims before inference and
  cannot reuse an existing root or overwrite evidence. Actual model, selected
  serving provider, usage, cost, timing, response, cleanup, and account spend
  must reconcile exactly.
- [x] One independent provider-free review returned REVISE. One consolidated
  correction added full-session hidden-assistant/model/tool/provider checks,
  fixed-root replay denial, preventive price reservations and post-exposure
  accounting, empty prepare profiles, one fake/real lifecycle, and the exact
  v26 eligibility hash. No second review ran.
- [x] The authorized command ran once. The first workflow made one request and
  observed exactly `openrouter/qwen/qwen3-coder-next`, one assistant, no hidden
  assistant, and no tools. It failed closed because actual serving-provider
  metadata was absent and the returned JSON failed the frozen response
  contract. Usage was 1,133 input and 265 output tokens, raw receipt cost
  `$0.00034796` (`348` normalized micros),
  trace time 989 ms, and cleanup verified. Three workflows were not called.
- [x] Provider-free terminal reconciliation sealed write-once failure and
  accounting receipts. Authority is exhausted; retry and additional calls are
  forbidden.
- [x] Final provider-free gates pass: fake and prepare-only commands; 20
  repetitions of boundary, hidden-assistant, mutation, claim, spend, duplicate,
  and replay tests; full affected Go packages; focused vet; hub validation; and
  real 20-source stewardship validation. A second reconciliation was denied by
  the existing terminal receipt. Only workflow 1 exists and `run.json` is
  absent. Bound hub `CURRENT_STATE.md` SHA-256 is
  `ac0f65d6683acbc60878bc249031968985d1d0bf6b0b39e7b96475edbef5a413`.

Evidence boundary: this is a terminal one-request fail-closed observation, not
a positive exact-serving-provider workflow proof or model qualification. No
model/role was promoted or selected, no upstream audit began, and no tools,
repository content, production/outbound action, Orca/Browser, or Phone Farm
surface was given to the model.

Durable responsibility-evidence closure (2026-09-14 12:11 -04:00):

- [x] Added migrations 558-574 for controller-owned, append-only
  cross-issue consequence receipts, accepted regression change records, and
  rejected regression attempts. Repository policy is preserved: no foreign
  keys or cascades, and every index is created concurrently in a separate
  single-statement migration.
- [x] Connected consequence creation to current workflow issue/outcome
  snapshots inside one transaction. The store locks the workspace and both
  issue owners in deterministic order, returns only an exact idempotent replay,
  and rejects altered replay, missing ownership, or stale snapshot input.
- [x] Connected regression add/update/supersede/retire to exact current,
  non-superseded, non-contradicted `workflow-evidence:<uuid>` rows on the
  source issue. Accepted and rejected histories are replayed from their hash
  chains before each append; release/quality selection survives restart.
- [x] Added all three record types to deterministic issue projection rebuilds.
  Accepted consequence and regression changes appear in owner meaningful
  activity; rejected attempts remain rebuildable audit evidence without
  becoming owner-facing progress.
- [x] Added `silver-mediums-stewardship-check` and
  `silver-mediums-item14-acceptance` Make targets. The stewardship check runs
  before the provider-free package tests/vet and fails closed on maintained
  graph, role-catalog, or hub-source drift. GNU Make is unavailable on this
  Windows host, so the target's exact Go commands were run directly; they pass.
- [x] A fresh disposable PostgreSQL 17 test applied migrations 558-574 and
  passed exact replay, altered replay denial, stale snapshot denial, typed and
  current evidence enforcement, concurrent exact convergence, accepted and
  rejected history replay, restart reconstruction, and UPDATE/DELETE denial.
  The exact test container was removed. Full affected Go tests, focused vet,
  and an all-server-package compile-only pass succeed.
- [x] During acceptance, the real stewardship check correctly failed on three
  concurrently updated maintained changelogs. Their current bytes were
  validated, only those manifest bindings were refreshed, and the check then
  passed. This is direct fail-closed acceptance-path evidence, not a simulated
  drift assertion. Final evidence is `source_count:20`, graph SHA-256
  `ff4119b1d70ced0a943f8dd056a1f3851e24c975ebacfad37dfc76d9aa75d1fa`,
  role SHA-256
  `c89c5f619c5f1a724e255b9014bfc882295a2c00504191bfc48569a5bbcc63e2`,
  and bound hub `CURRENT_STATE.md` SHA-256
  `c696f740aabedd78a8e52244d0ed0427b99d306f9a6df08158c6f96333336a37`.
- [x] The final qualification gate separately rejected an attempted status edit
  to the hash-frozen role-decision register. That edit was removed and the
  artifact returned to its exact accepted SHA-256
  `44025c5503da7f0b7047d4133caccfd3ef1ce06aca925df038c19a1d30fd1c94`;
  current closure truth stays in this progress file and the continuation. No
  frozen qualification hash was rewritten.
- [x] One consolidated review correction fixed the row-locking clause,
  protected workspace/issue ownership and evidence rows through commit,
  canonicalized issue lookup identity, rejected untyped evidence UUIDs, and
  made an already-sealed exact regression replay return its original result
  even after the source evidence is later superseded. New mutations still
  require current evidence before append.

Provider-free Item 14 engineering is now exhausted. Gates 1-4 in the
responsibility audit have deterministic implementation evidence, and the
complete local shadow suite is frozen and green. Gates 5-6 require valid
comparative model/configuration runs and a business choice about permanent
seats versus shared execution. The prior OpenRouter request remains terminal
failed-closed evidence and cannot be retried. No provider, credential,
production, outbound, Browser/Orca, upstream-audit, or Phone Farm action was
performed in this closure slice.

OpenRouter direct-shadow v2 precursor (2026-09-14 13:26 -04:00):

- [x] Recorded the owner's authority clarification: remaining configuration
  testing is already authorized inside the established provider lanes and
  budgets; configuration choice for testing is an engineering decision.
- [x] Preserved the terminal v1 root and created a separately versioned direct
  Chat Completions definition, SHA-256
  `fbe314bc6d779d0ad57bb6150f200e7a9db6af8e9a7c0b4ca6e814007c9441b4`.
- [x] Pinned the eligible temporary Qwen/Alibaba route with exact provider
  `order` and `only`, fallbacks/retries/redirects disabled, mandatory router
  metadata, strict schema, no tools, and the unchanged cumulative `$40`
  ceiling plus a `$1` slice cap. This is not a permanent model or role choice.
- [x] Plan, fake lifecycle, and prepare-only pass with zero credential reads
  and zero real calls. Focused tests prove altered model/provider/attempt,
  fallback, retry, redirect, tool, duplicate, and replay paths fail closed.
  Response-contract failure is now candidate-local so remaining frozen
  workflows continue; identity/provider/evidence ambiguity remains a global
  no-retry halt.
- [x] Ran the exact V2 command once. It stopped at router parameter filtering
  before provider inference and was reconciled into terminal zero-spend
  evidence. The V2 authority is exhausted.

Direct-shadow V2 terminal and V3 correction (2026-09-14 13:40 -04:00):

- [x] V2 ran once and failed closed at OpenRouter parameter filtering. The
  router made no provider inference attempt, selected no endpoint, charged
  zero spend, and reported Alibaba's available exact dated Qwen revision.
  Provider-free reconciliation sealed the terminal receipt and accounting;
  V2 is exhausted and will not be rerun.
- [x] Froze V3, SHA-256
  `b3241a97ac57418868c71059a6b443754e61069405025d0f3da56ad913372c5c`.
  It removes only the unsupported provider-side structured-output parameter;
  the controller still applies the identical strict response contract after
  the response. Exact selected provider, exact dated endpoint model, one
  attempt, zero fallback/retry/redirect/tool, and budget gates are unchanged.
- [x] V3 plan, fake lifecycle, prepare-only, and focused mutation tests pass
  with zero credential reads and zero real provider calls. One consolidated
  review correction now allows unrelated non-selected router inventory while
  still requiring exactly one selected and attempted Alibaba endpoint.
- [x] Ran the exact V3 real command once and reconciled it. Exact Qwen alias,
  Alibaba top-level provider, and one selected dated Alibaba endpoint were
  observed with one inference and zero retry. The model response failed the
  frozen contract, and the V3 harness failed closed because the successful
  metadata omitted its expected detailed attempt array. V3 is terminal.

Direct-shadow V4 remaining-workflow continuation (2026-09-14 13:58 -04:00):

- [x] Froze a new generation bound to the V3 definition and response. It omits
  the already-attempted first workflow and contains only the three unattempted
  frozen workflows. The definition SHA-256 is
  `c9368b0dbb9f9bf3a01c6dc5d5a554fa840ab0ce4d144f5d1f62ea2522190af6`.
- [x] Corrected the metadata validator to accept either no detailed attempt
  array or exactly one matching Alibaba/model attempt while still requiring
  attempt value `1`, exact top-level model/provider, and exactly one selected
  dated Alibaba endpoint. Any second or mismatched attempt fails closed.
- [x] Plan, fake, and prepare-only pass with zero credential reads and zero
  real calls. Focused V4 tests pass 20 repetitions; the full affected packages
  and focused vet pass. Fake execution cannot qualify the candidate because
  the bound V3 antecedent response already failed.
- [x] The independent provider-free review returned REVISE. One consolidated
  correction reparses a hash-bound V3 terminal receipt, freezes every plan and
  schema hash plus the exact system prompt, covers all allowed/denied attempt
  shapes, and seals post-exposure partial failures with usage/accounting,
  credential cleanup, exhausted authority, and no retry. Two review comments
  about missing transport/replay enforcement and absent provider proof were
  traced to earlier generations; V4 already contains those controls and V3's
  terminal receipt proves the exact Alibaba route. No second review ran.
- [x] Ran the V4 continuation exactly once. All three requests observed exact
  Qwen/Alibaba alias, top-level provider, selected dated endpoint, and attempt
  `1`, with zero retry. All three model outputs failed the unchanged frozen
  response contract, so the four-workflow V3+V4 candidate result is 0/4 and is
  not qualified. V4 used 988 input and 758 output tokens at 1,434 microdollars
  by receipt; combined V3+V4 cost is 1,933 microdollars. Immediate account
  readback lagged the V4 receipts and is preserved separately. The run,
  accounting, and replay claims are sealed; V4 is exhausted and must not run
  again.

Codex subscription model-only precursor (2026-09-14 14:17 -04:00):

- [x] Used a fresh external work directory, fresh task-local profile, dummy
  credential, loopback fake Responses provider, and zero request/stream
  retries. No real credential or provider was accessed.
- [x] Ignored user config/project rules, disabled project documents and every
  relevant tool-bearing feature, disabled web search, and cleared inherited
  desktop tool/session variables. The captured request pinned exactly
  `gpt-5.6-luna` and made one fake-provider request.
- [x] Held the lane fail closed: Codex CLI 0.153.2 still emitted an
  `additional_tools` namespace containing `functions.exec`,
  `functions.wait`, and `functions.request_user_input`, with
  `tool_choice:"auto"`. A disabled code-mode host prevents execution but does
  not meet the preventive no-tool request boundary.
- [x] Added hash-bound v1 evidence and a current v2 transport catalog while
  preserving historical v1 bytes. Focused mutation tests passed 20
  repetitions. The three disposable exploratory roots were deleted and exact
  absence verified.
- [x] Completed the provider-free role/squad operating-design contract. It
  reconciles 21 roles, 10 controller-only outcome identities, three candidate
  cores, six conditional packets, two external Mac roles, one skill-only
  packet, three squads, all 13 journeys, and all 19 locked cases without
  selecting a model or permanent seat.
- [x] Froze
  `silver-mediums.role-squad-operating-contract.v1`, SHA-256
  `b3f88549e6a92df5708c7a89c53cb31b0b034cfbf7639f52c96e76659ae8fe09`,
  plus its deterministic result, SHA-256
  `5602a14e5e4631a5c2590be0dc37a1ab0d381b906b8465374812b7cd4514f010`.
- [x] Made UI routing explicit: exact acceptance uses the shared executor plus
  Product/UI skill; ambiguous experience invokes Product/UI before the shared
  executor; consequential taste stops at one Mika/owner decision. Quality
  Checker remains the distinct final verdict in executable routes.
- [x] One self-adversarial review returned REVISE with no P0. One consolidated
  correction fixed the external Mac role ID, removed a duplicate graph hash
  helper, proved every journey capability has locked-case coverage, and made
  ordered stages, historical-label non-selection, shared-skill non-independence,
  exact squads, primary uniqueness, and host isolation machine-enforced.
- [x] Focused role/squad tests passed 20 repetitions; full
  `agentqualification` and `systemgraph` packages pass. Provider, credential,
  production, outbound, Phone Farm operation, model selection, and permanent
  seat counts are all zero.

Next: bind this additive operating contract into the Item 14 documentation and
stewardship gate, run the complete provider-free acceptance surface, then
prepare the remaining owner decision without reopening small-model testing.

Local Item 14 documentation stewardship (2026-09-14):

- [x] Added a separate local stewardship manifest for frozen authority,
  machine contracts, accepted evidence, and the three mutable canonical status
  documents. This complements rather than replaces the 20-source Silver
  Mediums knowledge-hub stewardship gate.
- [x] Added a read-only checker and made it a prerequisite of
  `silver-mediums-item14-acceptance`. Missing, extra, reordered, reclassified,
  unsafe-path, duplicate, or hash-drifted sources fail closed.
- [x] Focused structural, content-drift, and real-source checks pass with zero
  provider calls, credential reads, production actions, or model/seat
  selections.

Next: run the complete provider-free Item 14 acceptance command set, rebind the
mutable status sources to its final documented result, and prepare the smallest
remaining owner-only decision.

All-route role/squad rehearsal (2026-09-14):

- [x] Deterministically rehearsed all 19 locked cases plus the independent
  company-audit route through 59 controller-owned ordered participant stages.
- [x] Resolved 69 exact artifact references covering all 11 executable and
  assurance role packets: four accepted v1 packets and seven accepted v2
  packets. Ten shared-skill attachments remained inside the shared executor's
  stage and never became workers or independent reviewers.
- [x] Preserved all 76 locked case-evidence fields and delivered each complete
  case to one distinct final Quality Checker. No activity may follow a final
  verdict and every separate contributor has a unique named evidence contract.
- [x] The hash-bound trace is
  `9f208e29dba1273c342c160c1acf471f50fd61127cc8d590c9437fa5022adce4`;
  the recorded result SHA-256 is
  `d0fef123bab7d08ff591eef7f5633dc9e04f481018190b19a31da9cd25e96a81`.
  Mutation tests deny reordered work, artifact or contribution drift,
  self-review, shared-skill promotion, duplicate evidence, excess concurrency,
  and forbidden authority.
- [x] Observed worker concurrency was one. Provider, credential, tool,
  repository, production, outbound, Phone Farm-access, model-selection, and
  permanent-seat-selection counts were zero.

Primary-subscription role-trial owner gate (2026-09-14):

- [x] Prepared a closed, machine-validated gate for the first real structural
  comparison without contacting a provider or subscription.
- [x] Bound two unchanged synthetic fixtures, two organization arms, Claude
  Code and Codex subscription lanes, and eight exact observations totaling 12
  named participant CLI sessions. A Claude-excluded decision reduces execution
  to the six already-listed Codex participant sessions.
- [x] Provider-native tools remain forbidden. Proposed executor and specialist
  work uses only the controller-mediated action protocol against fresh
  disposable fixtures. R02 Browser evidence is controller-owned, in-app, and
  local-loopback-only; independent verdicts are provider-free hidden-verifier
  results.
- [x] One self-adversarial pass returned REVISE with no P0. One consolidated
  correction removed ambiguous session math, provider-paid reviewer sessions,
  and unbounded Browser wording. Mutations of participants, tools, retry,
  replay, Browser scope, Claude handling, authority flags, or bound source
  hashes fail closed.
- [x] Preserved the exact-model result: Claude Code cannot be described as an
  exact-one-model session because requested Sonnet can report background Haiku.
  The owner must accept this as a disclosed composite configuration or exclude
  Claude. Codex retains exact requested-primary binding and its observed
  `additional_tools` limitation; native tool execution remains disabled.

Next: complete the local stewardship rebind and final provider-free acceptance.
Then stop at the exact owner decision in
`ITEM-14-PRIMARY-SUBSCRIPTION-ROLE-TRIAL-DECISION.md`; do not start a provider,
subscription, credential, fixture, or Browser operation without that decision.

Final provider-free acceptance (2026-09-14 15:20 -04:00):

- [x] Both stewardship layers pass: 20 external Silver Mediums hub sources and
  27 local Item 14 sources (1 frozen authority, 9 machine contracts, 14
  accepted-evidence documents, and 3 mutable status documents).
- [x] The full package test surface used by
  `silver-mediums-item14-acceptance` passes once and its focused `go vet` set is
  clean. GNU Make is unavailable on this Windows host, so the exact underlying
  commands were run directly rather than claiming the wrapper ran.
- [x] Operating-contract, all-route rehearsal, primary-subscription owner-gate,
  and local-stewardship tests pass 20 repetitions together.
- [x] Owner gate SHA-256 is
  `d21f6b459bdf83757f6c7d27ac103309d659f5eb2a0dc8f13e0272439ed4e405`.
  The accepted role-design register remains unchanged at SHA-256
  `44025c5503da7f0b7047d4133caccfd3ef1ce06aca925df038c19a1d30fd1c94`.
  The sealed V4 evidence root remains present and was not rerun.
- [x] No provider request, subscription access, credential read, Browser
  action, fixture execution, production/outbound action, Phone Farm access,
  model selection, or permanent-seat selection occurred in this slice.

Provider-free work on the current role/squad roadmap is exhausted. The next
evidence requires the exact owner authority documented in
`ITEM-14-PRIMARY-SUBSCRIPTION-ROLE-TRIAL-DECISION.md`.

Owner-authorized primary-subscription role trial (2026-09-14 16:05 -04:00):

- [x] Recorded the owner's exact authorization for all 12 frozen participant
  sessions and the trial-only Claude Sonnet-primary plus disclosed-Haiku
  attribution. Permanent model, role, provider, and seat selection remain
  forbidden.
- [x] Froze `silver-mediums.primary-subscription-role-trial.v1` at SHA-256
  `bd412ada00e9dbccd26c42423fed193957afcf516e4e6bf0e347ba1338b9233a`.
  It binds every participant ordinal, lane, case, arm, role/skill packet,
  ceiling, cleanup requirement, and forbidden surface to the accepted owner
  gate.
- [x] Added one-shot structured participant envelopes and a controller-owned
  action executor. Unsafe paths, unlisted commands, missing R01 before/after
  checks, incomplete R02 checks/Browser requests, third Claude models, Codex
  native tool events, retries, fallbacks, duplicate execution, or altered
  authority fail closed.
- [x] Completed the full fake-provider precursor at
  `.qualification-evidence/item14-primary-subscription-role-trial-v1-fake-v3/`.
  It produced 12/12 fake sessions, 8/8 hidden-verifier passes, zero native tool
  events, zero retries/fallbacks, and verified cleanup. Terminal ledger SHA-256:
  `10f79077e8b1393e758fed79fc6d9919e5053431fa4cac0bbb442204909c4cc0`.

Next: prepare the fresh real evidence root, revalidate runtime identities, and
consume exactly the 12 authorized subscription sessions without relaunch or
automatic correction. Capture each R02 result only in the controller-owned
in-app Browser against its own loopback preview, then run the provider-free
hidden verdict and cleanup before advancing.

Primary-subscription live checkpoint and bounded Codex correction
(2026-09-14 16:24 -04:00):

- [x] Consumed all six frozen Claude participant sessions exactly once. All
  six reported requested Sonnet only; three observations passed the hidden
  verifier and the separate R02 observation failed closed on its executor's
  missing `local-static-preview report` action. No Claude session was replayed.
- [x] Preserved Codex ordinal 7 as terminally invalid after user-profile hook
  diagnostics and provider rejection of the original output schema. It was
  not retried. Corrected its evidence classification from two error
  diagnostics to zero actual native-tool executions using the preserved raw
  JSONL hash.
- [x] Applied one provider-free correction only to ordinals 8-12: strict
  all-properties-required Codex schemas, nullable non-applicable action fields,
  a fresh task-local `CODEX_HOME`, an opaque streamed subscription-auth copy,
  and verified whole-profile cleanup. Credential contents are never logged,
  displayed, hashed, persisted as evidence, or documented.
- [x] Added fail-closed total/generated token ceilings and focused fake,
  parser, schema, profile-copy, ceiling, correction, and correction-idempotency
  tests. Correction record SHA-256 is
  `62b067e949f6dc4a23870678a1cea09bfdf1091f8d8efa3dd662ff53708c3edb`;
  provider calls, retries, and replays for the correction are zero.
- [ ] Consume frozen Codex ordinals 8-12 exactly once, complete any valid R02
  local Browser evidence, verify all cleanup and evidence hashes, then perform
  the provider-free comparative and role/squad/instruction/skill analysis.

Current live counters: seven of 12 sessions consumed, seven provider calls,
zero retries, zero fallbacks, zero actual native-tool executions, and five
remaining authorized sessions. No permanent model, role, provider, or seat has
been selected.

Codex R01 separate checkpoint (2026-09-14 16:27 -04:00):

- [x] Consumed ordinals 8 and 9 once. Both pinned `gpt-5.6-luna` at Medium;
  actual model identity remains truthfully unavailable from the CLI receipt.
- [x] The specialist-to-executor R01 observation passed its hidden verdict.
  Provider-profile, fixture, and initial-copy cleanup are verified.
- [ ] Consume ordinal 10 once and, only if its action envelope is valid,
  complete the three-width controller-owned local Browser capture.

Current live counters: nine of 12 sessions and nine provider calls consumed;
zero retries, fallbacks, native tools, repository/production/business/Phone
Farm accesses, or permanent selections.

Codex R02 inline checkpoint (2026-09-14 16:31 -04:00):

- [x] Consumed ordinal 10 once and accepted its bounded R02 action envelope.
- [x] Captured controller-owned in-app Browser evidence at 390/820/1440 pixels;
  every width rendered total 47, four cards, and no horizontal overflow.
- [x] Hidden verdict PASS; capture token, provider profile, fixture, and
  initial-copy removal all verified.
- [ ] Consume final ordinals 11 and 12 exactly once, handling any failure
  terminally, then seal the 12-session ledger.

Current live counters: 10/12 sessions and ten provider calls consumed; all
forbidden-effect and replay counters remain zero.

Primary-subscription trial terminal result (2026-09-14 16:35 -04:00):

- [x] Consumed final ordinals 11 and 12 exactly once. Both provider envelopes
  were structurally valid; the executor's edit payloads were prose rather than
  literal file contents, so the local allowlisted check failed and Browser was
  never opened. The observation was cleaned without replay.
- [x] Sealed 12/12 sessions, 12 provider calls, eight terminal observations,
  five hidden-verifier PASS outcomes, two invalid observations, and one
  controller-action failure.
- [x] Verified every provider workdir and fixture/initial root absent. Zero
  retries, fallbacks, actual native tools, repository/production/business/
  Phone Farm accesses, or permanent selections.
- [x] Terminal ledger SHA-256:
  `5098d32f8781628547dde360f2c8f7b48bfdd733247d16c8b40990d0c3a53713`.
  Evidence-manifest SHA-256:
  `7190de705aa51b213df2d92986fcfb725e9a00fdf0915fc1802f6568e3a12167`.
- [x] Complete one provider-free independent evidence audit and comparative
  interpretation, then continue the next role/squad/instruction/skill contract
  validation without any thirteenth provider call.

Provider boundary is now closed. Claude totals: 43,004 input/12,801 output
tokens, 219.484 seconds, $0.372820 API-equivalent estimate, Sonnet-only in the
six observed receipts. Codex totals: 45,755 input/3,702 output tokens, 95.121
seconds, pinned Luna/Medium with actual identity unavailable and no provider
cash receipt. No permanent decision follows.

Independent trial review and role-stage evidence contract
(2026-09-14 16:49 -04:00):

- [x] Ran one separate provider-free deterministic evidence review. All 13
  integrity checks passed; an altered raw response in a copied evidence root
  was rejected. Review SHA-256:
  `f217c433a147b2aaa745d4c87b04182da4095cd1d5c7742e32f2b7ac43b46bec`.
- [x] Froze the descriptive trial result without selecting: inline 3/4 PASS,
  separate 2/4 PASS, five of eight overall. Canonical machine-result SHA-256:
  `03208af564632ff8a030029c963085f3ffce3c3b1e442714512cacadfabbc41a`.
- [x] Bound all 11 staged role outputs to exact artifact references and JSON
  field sets across 20 routes, 59 participant stages, and 10 shared-skill
  attachments.
- [x] Required controller-only handoff, complete packet reissue, advisory-only
  specialist evidence, literal self-contained edit payloads, exact check
  allowlists, and terminal failure without automatic replay. Mutation tests
  reject every relevant weakening.
- [x] Rehearse the new role-stage contract through all routes and the complete
  provider-free local shadow suite, bind it into stewardship, then run the full
  Item 14 acceptance surface.

Role-stage evidence contract SHA-256:
`f0105e916d826407b499d0de76c39e11fe9c29c96570fa04c5ef18f49ed705f5`.
Provider calls remain closed at exactly 12; no configuration or permanent seat
is selected.

Integrated role-stage rehearsal (2026-09-14 16:57 -04:00):

- [x] Replayed the response/handoff contract through all 20 operating routes
  and all four complete local shadow workflows.
- [x] Produced 91 receipts: 89 exact role-response bindings, two
  controller-only outcome bindings, 79 complete packet reissues, 10 shared
  skill attachments, and 20 self-contained executor action obligations.
- [x] Preserved all 112 role-qualified response fields and both observed
  downstream Product/UI handoff failures without post-hoc repair or rescoring.
- [x] Mutation tests reject response-class, handoff, full-packet,
  advisory-evidence, literal-payload, shared-skill, controller-only, trace,
  provider, and selection drift.
- [x] Add the new immutable sources and mutable documentation hashes to local
  stewardship, then run the full Item 14 acceptance surface.

Combined trace SHA-256:
`7cad59a9b3beef094ba47846f8c5b4adb47d2d1863dd43ec895ee5cd90690054`.
Recorded result SHA-256:
`45c7294b5a15b36a9bd702fc3241aaeaf2697282897d99eb0b362e9a38d02217`.
Every authority/effect counter remains zero.

Provider-free consolidation and final decision boundary (2026-09-14):

- [x] Expanded local Item 14 stewardship from 27 to 34 exact sources: one
  frozen authority, 13 machine contracts, 17 accepted-evidence documents, and
  three mutable current-status documents.
- [x] Added the terminal trial definition/result, independently reviewed result
  document, role-stage contract/rehearsal, and final configuration decision
  packet without binding transient credential-bearing paths.
- [x] Complete scoped Go tests passed once and matching `go vet` was clean.
  The new deterministic stewardship/trial/stage/mutation tests passed 20
  repetitions.
- [x] Fresh provider-free verification preserved the sealed 12-session ledger,
  13/13 independent-review result, and 91-receipt role-stage trace exactly.
- [x] Local 34-source stewardship passes with all authority/effect counters
  zero. The separate shared knowledge-hub prerequisite fails closed on two
  concurrently changed, out-of-scope Canopy changelogs; they were not adopted
  or overwritten.
- [x] One final review found no new actionable defect. The completed bounded
  correction and terminal failures remain unchanged; no second correction or
  provider replay was introduced.

Provider-free qualification work is exhausted. No final model, topology, role,
or permanent seat is qualified or selected. The genuine next step is the owner
choice in `ITEM-14-FINAL-CONFIGURATION-DECISION.md`: hold with no permanent
selection (recommended), or authorize a new separately capped reliability
trial. No provider call, credential access, or production action is currently
authorized.

## Primary-subscription reliability trial v2 (2026-09-14)

- [x] Froze a new 12-session definition without altering the prior trial or its
  5 PASS / 2 INVALID / 1 FAILED result.
- [x] Ran the full fake precursor and prepare-only gate before subscription
  execution.
- [x] Completed one separate provider-free review and one consolidated harness
  correction. Windows-safe atomic replacement, selector/authority/runtime
  binding, receipt-derived call accounting, truthful pre-workdir cleanup, and
  immutable manifest sealing are now enforced.
- [x] Consumed all 12 fresh one-shot subscription sessions with zero retries,
  fallbacks, native tools, real repository/production/business/Phone Farm
  access, or permanent selection.
- [x] Sealed six PASS / zero INVALID / six FAILED observations. Both Claude and
  Codex passed R01 3/3 and failed R02 0/3.
- [x] Preserved one three-viewport in-app Browser evidence set for the sole R02
  plan that reached that boundary; its hidden verdict remained FAIL.
- [x] Passed a separate 12-check evidence review, including raw-response hashes,
  cleanup, ceilings, attribution, Browser evidence, manifest recomputation, and
  copied-evidence mutation denial.
- [x] Froze the derived machine result at SHA-256
  `9f99664568ff701cadddbac537a7548cebfd88fbd2bc7d3d1415176ddb1f1f1a`.

Provisional evidence decision: qualify only the local R01-like topology of the
shared Windows executor plus exact architecture-assurance skill inline.
Provider/model selection remains unset; Product/UI and R02 remain unqualified;
production remains forbidden. Next is a provider-free machine binding and
allow/deny rehearsal for this reversible scope, followed by continued
structural role/squad/instruction/skill qualification.

## Provider-free provisional configuration and R02 correction candidate

- [x] Froze the reversible R01-only shared-executor plus inline
  architecture-assurance binding. Provider, model, squad, role, and seat remain
  unselected.
- [x] Rehearsed 20 exact boundary scenarios: one allow and 19 fail-closed
  denials covering R02, Product/UI, separate-specialist, authority expansion,
  automatic dispatch, retry, fallback, and every external-effect surface.
- [x] Reconciled all 11 role packets, 11 instruction/skill pairs, ten
  controller-only outcome identities, one Mac-only skill packet, and three
  squads into the v2 readiness matrix. Zero squads are activated.
- [x] Derived one additive R02 v3 action contract from the six preserved
  terminal failures. It requires complete literal JS/CSS, responsive behavior,
  controller-owned evidence, post-repair checks, and one exact Browser action.
- [x] Passed the complete fake R02 workflow and hidden verifier. Focused
  mutations reject every observed failure pattern and authority expansion.
  Product/UI and R02 remain not runtime-qualified.
- [x] Added v2 successor stewardship without rewriting v1. It verifies all 31
  immutable v1 sources plus 16 new/current sources.
- [x] Ran the complete scoped Go package suite, matching `go vet`, 20 repeated
  focused tests, sealed-trial verification, and the independent 12-check
  evidence review.
- [x] Preserved the separate shared-hub failure on two out-of-scope Canopy
  changelog hashes. No hub, Canopy, production, or Phone Farm state changed.

Provider-free work is exhausted at an honest boundary. Safest current state:
hold the reversible R01-only local scope. Any R02 runtime qualification needs a
fresh separately capped owner grant. No provider call, credential access, or
permanent model/role/provider/squad/seat selection is authorized.

## Closeout integration/preservation milestone (2026-09-14 20:06 -04:00)

The owner superseded the old provider-free stopping gate for one bounded local
integration exercise, without changing any Item 14 qualification conclusion.
The temporary path is Claude Code subscription execution with Sonnet primary
and accepted possible background Haiku. It does not qualify or permanently
select any model, provider, role, squad, or seat.

The exact shared-Windows-executor plus architecture-assurance topology is being
exercised on a harmless Windows repository/worktree inventory. Its frozen packet
allows read-only Git/filesystem evidence across ten exact roots and writes only
three report files under a new isolated Company Engineering Foundation path.
Phone Farm is maintained-documentation-only and Windows Phone Farm access remains
forbidden.

The complete automatic squad lifecycle is capped at five task executions:
leader intake, executor, independent reviewer, and only after rejection one
correction plus re-review. Zero automatic retries/fallbacks, Anthropic API,
OpenRouter, production, deployment, outbound, destructive, credential, or
shared-access actions are allowed. No provider call had occurred when the packet
was frozen.

Pre-dispatch: one isolated Claude-only daemon, the exact architecture skill,
distinct temporary executor/reviewer, intake-only leader, and one-correction
squad template are configured. `CEF-1` holds the packet unassigned with zero
runs; no provider inference has yet occurred. These objects are exercise-local
and do not change the existing unselected qualification state.

The first leader claim revealed a provider-free wrapper framing defect and was
cancelled before Claude launched. One consolidated harness correction changed
the wrapper from read-to-EOF to first-frame-plus-stream forwarding and narrowed
the isolated task retry default from two attempts to one. Fake delegation,
local discovery, and version delegation passed. No provider call or role/model
selection occurred; the replacement leader run remains within the owner-granted
single workflow.

The provider-backed leader, initial executor, and independent reviewer have now
run through the real claim/daemon path. The executor created the bounded report
and recorded `implementation_ready`; the reviewer independently rejected it for
specific detached-count attribution and dirty-definition defects, which queued
the one allowed correction. The executor and reviewer each persisted their
handoff event and issue comment before Claude Code returned a max-turn terminal
error. Multica then generated redundant delegated-failure recovery tasks; both
were controller-cancelled while queued, with zero provider execution. The
intended correction is active and remains the fourth of at most five provider-
backed workflow tasks.

The correction task made partial report edits but failed before recording its
required workflow event. The fifth provider-backed slot was then consumed by a
delegated-failure reviewer follow-up rather than a structured re-review. It
completed without advancing the plan, after which Multica automatically claimed
a sixth leader task for the missing event. The controller stopped the isolated
daemon immediately; the sixth task is cancelled after eight messages and has no
usage receipt. It nevertheless counts as a provider-backed invocation, so the
five-task boundary was breached and this integration pilot is not qualified.

Independent controller validation found additional contract failures: no
machine-readable per-worktree rows, no complete path-existence pass, stale
post-correction hashes, and rejected pre-correction arithmetic still present in
`EVIDENCE.md`. The structured workflow is still `correction` with its single
correction consumed, the issue remains in progress, and no review-pass evidence
exists. Item 14's earlier provisional qualification remains unchanged; this
pilot adds a failed integration observation only.

The background built-in Browser required one isolated same-origin harness fix:
clear public cross-port API/WebSocket URLs and use the frontend's existing proxy
to `backend:8080`. Login then passed. Board, Issue, and Inbox all rendered, but
they exposed the missing projection rather than success: `CEF-1` is under
`Status unavailable`, issue Now has no verified progress/next step, and Inbox
has zero meaningful updates or completions. This materially confirms that the
workflow/event-recovery behavior is not ready for role/squad activation.

Final accounting preserves six provider-started tasks: five finalized exact-
Sonnet receipts totaling 6,314,257 tokens and one interrupted hard-stop task
with eight messages but no usage receipt. All tasks stayed at one attempt and
there are no retry rows. Subscription cost is not present in the receipt; the
UI's `$3.57` is an estimate only. The runtime is offline.

Fake transport checks, focused service/handler/workflow-action/projection tests,
focused `go vet`, compose rendering, and ten-root branch/HEAD/status-count
preservation checks pass. The passing unit tests omit the live cross-mechanism
recovery sequence that generated task six. Runtime-URL Vitest was unavailable
because the restored checkout intentionally has no frontend dependencies; no
installation was attempted. Phone Farm and every external-effect boundary
remained closed.

Role-build decision: do not activate the temporary squad and do not convert
this pilot into qualification evidence. The next separately authorized work is
provider-free Multica defect correction for a global workflow task ceiling,
event/status terminal reconciliation, artifact-contract validation, and
meaningful Inbox projection. Any real replay requires new bounded authority.

## Provider-free failed-pilot defect correction (2026-09-14 23:21 -04:00)

- [x] Preserved the failed real-provider pilot and its exact-model guardrail;
  no replay or provider contact occurred.
- [x] Completed one independent read-only review: six High and two Medium
  findings.
- [x] Applied one consolidated correction across artifact admission, accepted
  event finality, sweeper/live failure reconciliation, provider-start ceiling,
  full packet/runtime/model/skill binding, isolated tool-free child execution,
  and atomic technical-stop/owner-Inbox projection.
- [x] Passed focused database, claim mutation, child environment, exact skill,
  and complete guarded fake-provider controller/claim/daemon tests.
- [x] Complete broader affected-package tests, vet, final integrity checks,
  evidence hash verification, and disposable database shutdown.

This milestone selects no permanent role, squad, model, provider, or seat. A
new real workflow remains owner-gated after provider-free closeout. Focused
controller/claim/daemon/database tests and the one-launch fake-provider shadow
pass; matching vet and diff checks are clean. Broader package runs retain
documented unrelated repository/Windows/profile baseline failures. Frozen
failed-pilot evidence is unchanged and the disposable database is stopped and
absent.

## Authorized integration replay preflight (2026-09-14 23:39 -04:00)

- [x] Owner accepted the provider-free defect milestone and authorized one
  bounded replay through the existing Claude Code subscription.
- [x] Reconfirmed that the old failed report/evidence and original dirty C:
  checkout are immutable inputs, not replay targets.
- [x] Reconfirmed the global controller ceiling: five provider-started tasks
  across issue+squad lineage, serialized before provider launch and independent
  of later role mutation.
- [x] Prove the exact replay broker with fake credentials/provider and
  prepare-only execution. Claude must remain tool-free; the task-scoped Multica
  capability stays parent-only.
- [ ] Obtain one independent review and apply at most one consolidated
  correction if review finds an issue.
- [ ] Run the fresh three-stage replay (leader, executor, reviewer), using the
  fourth and fifth slots only for one correction and its re-review when needed.
- [ ] Verify accepted artifacts plus truthful Board, Issue, and Inbox evidence.

No provider call has started in this replay. The temporary Sonnet fixture is
execution evidence only and does not select a permanent role or model.

### Replay broker pre-review checkpoint (2026-09-15 00:15 -04:00)

- [x] Controller issuance and claim reconstruction derive broker authority only
  from one exact controller-owned acceptance marker and bind its SHA-256 in the
  immutable successor assignment.
- [x] Daemon verifies the broker before and after staging, provides only the
  task-scoped Multica capability and exact workflow task kind to that parent,
  and preserves `tool_free_v1` for the Claude stream.
- [x] The broker constructs a fresh Claude environment that excludes Multica,
  repository, MCP, tool, Anthropic API-key, and OpenRouter capability; the
  provider path is one Sonnet turn with zero retries and no fallback.
- [x] Fake-provider end-to-end coverage passed, including leader routing,
  exact three-file creation, typed artifact evidence, review pass, mutation
  denial, loopback-only API use, and the Windows correction replacement path.
- [x] Fresh PostgreSQL 17 claim tests passed for exact broker issuance, altered
  digest denial, full controller-snapshot mutation denial, and same worktree
  path under distinct authorized roots.
- [x] All four prepare-only role/task-kind cases completed with
  `provider_started=false`; the new report path remains absent.

The provisional pre-review broker SHA-256 is
`496458800a2ecae5e4bfe5e5f04325c3c873f364b962ce757eac4bc84c458d57`.
It is not yet the frozen replay authority. One independent review remains, with
at most one consolidated correction before the final binary is built and
bound. No provider or credential was contacted.

### Replay broker independent review (2026-09-15 00:31 -04:00)

- [x] Exactly one independent read-only review completed.
- [ ] Verdict: `REVISE`; provider launch remains prohibited.
- [ ] P0: place the initial leader and initial executor on the same
  controller-bound broker path as every successor, then prove the complete
  task lifecycle with a fake provider.
- [ ] P1: replace editable acceptance-derived promotion with a durable,
  expiring, revocable, task/issue/role/runtime/digest-bound controller grant
  consumed exactly once in the transaction that starts the task.
- [ ] P1: bind, stage, and re-hash both the broker and its one provider child;
  remove the ambient fake-provider selector.
- [ ] P1: require a response-bearing assistant frame from exact Sonnet and
  reject missing models, usage-only model claims, unknown event/content shapes,
  and every tool/server-tool result.
- [ ] P1: verify artifact totals and canonical bytes, recollect correction
  inventory, and perform a final hash read immediately before event emission.
- [ ] P2: make the composite worktree duplicate test use the same root and
  path, with a fresh request digest.

This is the single correction cycle authorized for the replay. The original
pre-review digest is obsolete. No provider, credential, production, outbound,
Phone Farm, Browser, or Orca surface was contacted.

### Replay consolidated correction implemented (2026-09-15 01:13 -04:00)

- [x] Bound the initial leader, initial executor, review, correction, and
  recovery paths to controller-owned `qualified-runtime-binding.v2` broker
  receipts; editable acceptance text grants no authority.
- [x] Made issuance expiring/revocable and bound it to the exact issue, agent,
  runtime, role artifact, task kind, broker path/digest, provider-child path/
  digest, model, effort, call limit, retry limit, and turn limit.
- [x] Convert the receipt to a task-specific capability in the enqueue
  transaction and consume that capability exactly once at task start. Altered,
  expired, revoked, duplicate, and replayed authority fails closed.
- [x] Stage and re-hash both executables and construct the allowlisted child
  environment without Multica, repository, MCP, tool, provider-key, or ambient
  task capability.
- [x] Require one response-bearing exact-model assistant result; reject
  missing or usage-only model claims, unknown frames/content, redirects,
  fallbacks, and tool/server-tool results.
- [x] Verify canonical artifact bytes and totals, recollect correction
  inventory, and perform the last hash read immediately before event emission.
- [x] Correct the duplicate-worktree test to use the same root and path with a
  fresh request digest.
- [x] Pass one complete fake-provider workflow through real service dispatch,
  authenticated HTTP claim/start/progress/messages/complete, daemon staging,
  broker, task-scoped parent API calls, and exact fake child. Leader,
  implementation, and review completed with three one-shot grants and no
  legacy packet or ambient task context.
- [x] Re-pass the original fake credential/controller shadow and focused broker
  and handler adversarial tests.
- [x] Complete broader regression, vet, integrity, and all four prepare-only
  gates; then build and freeze the final replay authority.

No real provider, credential, production, outbound, Phone Farm, Browser, or
Orca surface was contacted. Provider launch remains prohibited until the open
provider-free gates pass. The temporary fixture does not select a permanent
role, squad, model, provider, or seat.

### Replay provider-free freeze complete (2026-09-15 01:24 -04:00)

- [x] Exact authority, receipt, capability, workflow, handler, service, daemon,
  broker, credential, and complete-path fake tests pass.
- [x] Matching affected-package `go vet` and `git diff --check` pass.
- [x] All four prepare-only role/task-kind cases return
  `provider_started=false`.
- [x] The final broker rebuild is reproducible at SHA-256
  `366718905d210c833d10a1116e4ef4033613be65046fa647625be88e09b4d4da`.
  The fake child fixture is
  `ed4c842bd98f38b08ef444964edc7dc5977199aad90346d6f22ec4cb7cfd1f0f`.
- [x] Frozen failed-pilot evidence hashes and the owner checkout branch, HEAD,
  and 494/459 dirty counts are unchanged; the additive replay report is absent.
- [x] No disposable test schema remained, and its PostgreSQL container is
  stopped.

The full qualification package retains its known sealed-source drift failures;
the exact changed authority group passes. One stale test mutation was corrected
to actually create the now-absent legacy broker field, after which its intended
fail-closed claim passed. No product guard was weakened. Provider calls remain
zero. The provider-free freeze is complete; only the separately authorized
fresh replay may advance next.

### Fresh replay preparation (2026-09-15 01:47 -04:00)

- [x] Started a separate loopback-only replay stack and fresh database without
  altering or reusing the failed pilot stack or evidence.
- [x] Created only temporary local replay objects: task-local CLI profile,
  private runtime, three private agents, one skill, one bounded squad, issue,
  and frozen plan.
- [x] Verified zero task rows, zero issue runs, zero authority receipts, absent
  replay report, and zero provider calls.
- [x] Implemented and tested the controller-only authority helper. It rejects
  substituted identifiers, paths, executable digests, membership, skill,
  agent configuration, plan, pre-launch state, report state, and nonzero task
  state before issuance.
- [x] Passed the helper's non-writing `prepare` gate with a fake credential
  marker: three exact receipts prepared, none stored, provider not started.

The fake marker is not provider authority and will be removed before the
authorized opaque task-local Claude credential copy. No real credential has
been read or copied and no provider has run in this replay.

### Controller receipt issuance (2026-09-15 01:51 -04:00)

- [x] Removed the fake marker and performed the one authorized opaque copy into
  the exact task-local Claude profile; inspected presence only.
- [x] Re-ran prepare against the real task-local profile with
  `prepared=3 stored=0 provider_started=false`.
- [x] Issued the three exact receipts and repeated the identical issuance to
  prove idempotency; the durable total remained three.
- [x] Read back exact Sonnet/Medium/Claude bindings, broker and native child
  digests, allowlisted tool-free boundary, profile kind/path, agent names, and
  allowed task kinds.
- [x] Verified three active receipts, zero revocations, zero grants, zero
  consumptions, zero task rows, absent report, and zero provider starts.

The receipts expire at `2026-09-15T08:47:00Z`. Only one frozen workflow
dispatch is authorized next; no manual retry, fallback, replacement, sixth
task, or broadened model/role authority exists.

### Replay failed closed before provider; cross-OS correction complete (2026-09-15 02:05 -04:00)

- [x] Attempted the authorized workflow dispatch exactly once; controller
  admission returned `guarded_configuration_unbound`.
- [x] Proved the failed transaction created zero tasks, grants, consumptions,
  claims, provider starts, provider calls, report files, tokens, and cost.
- [x] Retried nothing. Revoked all three receipts and verified zero active /
  three revoked.
- [x] Deleted the opaque task-local credential and profile directories and
  verified both are absent.
- [x] Identified the defect: a Linux controller applied Linux-native path rules
  to exact paths attested for the Windows runtime.
- [x] Corrected path validation to use the attested host OS and added fail-closed
  coverage for slash drift, traversal, duplicate separators, invalid Windows
  characters, foreign syntax, and unknown hosts.
- [x] Passed the receipt store/prepare lifecycle with Windows paths inside the
  Linux builder and the complete target-Windows fake workflow in 59.731
  seconds; no disposable schema remains.

An optional Linux-runtime execution of the intentionally Windows inventory
broker admitted the leader and then failed at Windows-specific inventory, as
expected outside the target platform. It made no real provider call. Any new
real replay requires a fresh, separately bounded owner grant.

### Provider-free closure and stop (2026-09-15 02:13 -04:00)

- [x] Re-passed the exact authority/capability, claim, controller-helper, vet,
  and diff gates after the cross-OS fix.
- [x] Reproduced the unchanged frozen Windows broker digest
  `366718905d210c833d10a1116e4ef4033613be65046fa647625be88e09b4d4da`.
- [x] Built the corrected Linux controller image
  `sha256:18ff4edf2e3509c0e8a9f8e2f7a48c325383ea02b7d4e7d9f6e959ea2c19e3b3`
  under a new tag and updated only the inactive replay overlay.
- [x] Verified in the background built-in Browser that the isolated issue is
  visibly Blocked, the status transition is in activity, zero agents are
  working, and no release or verified progress is claimed.
- [x] Preserved the no-task truth: Now remains Status unavailable and no Inbox
  or workflow event was synthesized for the rolled-back enqueue.
- [x] Reverified zero task/grant/consumption/provider/usage/cost/report state,
  three revoked receipts, credential/profile absence, frozen failed-pilot
  hashes, and exact owner-checkout branch/HEAD/494/459 state.
- [x] Stopped the fresh daemon and fresh replay containers without deleting
  their retained database/volumes; the old failed-pilot stack remains running.

Item 14 is at a genuine owner gate. No provider request occurred. A new replay
requires fresh bounded credential and dispatch authority; it may not reuse the
revoked fixture and still cannot select a permanent role, model, provider, or
seat.

### Fresh v2 replay gate (2026-09-15 03:02 -04:00)

- [x] Retained the accepted leader -> implementation -> independent review ->
  optional correction -> re-review topology. The proposed three-task shortcut
  was revoked before any provider access and was not implemented.
- [x] Started a fresh isolated stack on `15435` / `18082` / `13002` using the
  corrected controller image and fresh workspace, issue, runtime, agents,
  skill, squad, plan, daemon, CLI profile, and database.
- [x] Set and read back the fresh database task default as exactly one attempt,
  preserving zero automatic provider retries and the global five-task ceiling.
- [x] Passed the complete fake-provider dispatch/claim/daemon/broker workflow
  for leader, implementation, and review, plus focused receipt lifecycle,
  cross-platform path, handler claim, and editable-authority denial tests.
- [x] Removed the disposable PostgreSQL 17 regression container after proving
  its schema lifecycle.
- [x] Completed one independent read-only review. It returned one P1 and no P0:
  the authority helper needed to fail closed if the database retry default
  drifted away from one.
- [x] Applied the one consolidated correction, added the exact-default denial
  test, passed focused command tests and vet, and rebuilt the v2 authority
  helper at SHA-256
  `098709fd94fd79907ebaaddc80dd0d5fdafec0faf38ada5634674f5fdb2982bd`.
- [x] Re-ran fake-marker prepare-only: three receipts prepared, zero stored,
  no provider start. Live counts remain zero tasks, receipts, revocations,
  grants, and consumptions; the report path is absent.
- [x] Perform the authorized opaque credential copy and real-profile prepare;
  three exact controller receipts are active with zero tasks, grants,
  consumptions, revocations, or provider starts.
- [ ] Dispatch exactly once, monitor the global five-task ceiling, then revoke
  and delete/verify cleanup unconditionally.

No real credential, provider, production, outbound, Phone Farm, Browser, or
Orca surface was contacted during this milestone. The temporary Sonnet/Medium
binding is a fixture only, not a permanent model, role, provider, or seat
selection.

Receipt checkpoint: issued once at `2026-09-15T07:01:00Z`, expires
`2026-09-15T10:01:00Z`; exact active count `3`. The credential operation was
opaque and presence-only. No provider call has occurred.

### Fresh v2 replay fail-closed closure (2026-09-15 03:18 -04:00)

- [x] Dispatched the frozen issue exactly once. The leader task was created at
  attempt 1/1 with one capability grant.
- [x] Preserved the terminal admission denial:
  `guarded_dispatch_invalid: incomplete guarded execution facts`.
- [x] Proved the denial preceded skill/environment/credential/provider
  exposure: zero starts, consumptions, provider processes, usage/token rows,
  cost, report files, retries, or successor tasks.
- [x] Retried nothing. Revoked all three receipts, deleted the credential
  opaquely, removed the empty profile, and verified all absences.
- [x] Isolated the concrete defect to the v2 manual daemon launch omitting the
  required parent-only `MULTICA_HOST_ATTESTATION=alienware_windows`; all other
  expected guarded facts were populated.
- [x] Added one fixture-specific no-credential launcher that pins the exact
  host attestation and broker while keeping every non-Claude provider on an
  absent sentinel. SHA-256:
  `e9f62e521a615b08e88ad2bc92146c8492d70110c4e573b2cf46f2af7ed8e896`.
- [x] Passed focused host-config, guarded-fact, substitution-denial, and child-
  environment tests. A credential-free launch/stop through the corrected
  harness registered only Claude, launched no child, and left no daemon.
- [x] Verified Board/Issue/Inbox in the built-in Browser: Status unavailable,
  Blocked, one failed run, no verified progress/next step/release, and one
  truthful Important incident with no update/completion claim.
- [x] Stopped the isolated v2 stack while retaining its containers and volumes.

The provider request count is zero. This exhausted receipt/profile/task fixture
cannot be retried. A new real replay requires separately bounded owner
authority; the provider-free correction itself does not select any permanent
role, model, provider, squad, or seat.

### Exact-launch rehearsal and owner-surface contract gate (2026-09-15 04:20 -04:00)

- [x] Froze every required runtime launch field in an exact JSON contract and
  automatic PowerShell preflight, including controller image/identity,
  workspace, daemon identity and auth, Windows attestation, broker/provider
  executable hashes, temporary Sonnet/Medium fixture, absent non-Claude
  providers, tool-free policy, one attempt, zero retries, and five-task ceiling.
- [x] Preserved the initial user-PAT daemon failure as a correct fail-closed
  result; guarded start requires the task's authenticated daemon identity.
- [x] Corrected the root harness with a unique opaque, expiring, workspace-
  scoped daemon token/profile. No controller or server admission guard was
  weakened.
- [x] Proved one altered fake-provider digest is rejected during prepare and
  then completed one exact leader -> implementation -> review workflow through
  the authenticated daemon and byte-identical broker using only the fake
  provider. Three tasks ran once, three grants were consumed once, no tools or
  repository reached the child, and no real credential/provider was touched.
- [x] Captured the exact 10-root/443-worktree report and verified its three
  hashes before moving it to the task evidence directory.
- [x] Failed the complete shadow acceptance honestly: `CEF-4` reached Done in
  legacy workflow state, but generated no v2 current picture or owner-relevant
  Inbox completion. Its three Inbox rows are routine `new_comment` items.
  Board/Now remained Status unavailable.
- [x] Reused the already completed independent review and single consolidated
  correction cycle; did not run a duplicate reviewer.
- [x] Revoked receipts, removed daemon and fake-provider credentials, verified
  zero daemon tokens and zero real tasks/receipts, and stopped the isolated
  stack with volumes retained.

The remaining decision is architectural, not model qualification. The frozen
broker's legacy event is not sufficient for the accepted v2 owner surface, and
there is no task-scoped v2 projection write route. The controller already has
an atomic legacy-failure adapter; the smallest safe change is a matching
controller-only success adapter that preserves the broker digest. Recommended
next slice: separately authorize that provider-free controller contract and
repeat the full fake rehearsal. Do not run a real provider, select a permanent
role/model, or weaken the owner-surface contract to make the rehearsal green.

### Legacy-success adapter and full fake acceptance (2026-09-15 05:22 -04:00)

- [x] Implemented the controller-only accepted-success adapter. It admits only
  legacy `review_passed -> done` for a non-deployment plan and atomically stores
  the event/status, revision-1 Current Picture, and owner completion Inbox row.
- [x] Protected existing truth with expected revision zero and exact identity,
  source-task, review-reason, and timestamp validation; replay remains one
  logical effect.
- [x] Preserved `CEF-5` as a valid partial acceptance observation: transport and
  atomic writes passed, but detail validation rejected a null top-level scope
  revision. Its report is retained before correction.
- [x] Corrected that field within the same consolidation, added real detail-
  handler readback to the database test, and rebuilt the isolated controller.
- [x] Completed fresh `CEF-6` through leader -> implementation -> independent
  review, all once at attempt 1/1 through real dispatch/claim/daemon/broker and
  fake provider. Three exact grants were consumed once; zero tools or task
  messages were exposed.
- [x] Proved Current Picture revision/scope revision one, satisfied outcome,
  completed execution, no required release, resolved attention, complete
  milestone, one completion Inbox row, and zero replacement of prior truth.
- [x] Verified Board Done, truthful issue Now, and one Verified completion Inbox
  item in the built-in Browser. The final evidence reports 10/10 roots and
  444/444 path checks.
- [x] Completed terminal revocation and cleanup: zero active / three revoked
  receipts, offline runtime, zero daemon tokens, absent task-local profiles and
  child processes, stopped retained-volume stack, and removed disposable test
  database.
- [x] Passed focused and full projection tests, three database-backed atomicity
  tests, workflow-v2, authority-helper tests, focused vet, launcher parse, diff
  integrity, frozen artifact hashes, and original owner-checkout 494/459 state.

The isolated issue page also exposed the retained baseline's independent
`meaningful-activity` query mismatch (`dr.revision` missing). It did not alter
the authoritative Current Picture or the required owner surfaces and is not
silently folded into this narrow adapter correction.

No second independent review was run; the existing review and one consolidated
correction boundary remain intact. No real provider/credential, production,
outbound action, Phone Farm technical access, Orca, or final selection occurred.
This historical stop is superseded by the completed `CEF-7` real integration
replay recorded at the top. The remaining local Item 14 gate is to supersede
the fixture-only v1 provisional binding with the smallest reversible R01-like
operating contract supported by the combined evidence, then prove its exact
allow/deny, drift, replay, and retirement behavior provider-free. It may not
select a permanent role/model/provider/squad/seat, activate production, or
enable automatic dispatch. An owner decision is required only for a later
boundary expansion.
