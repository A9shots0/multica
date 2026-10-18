# Continuation handoff

> **Current continuation router — 2026-09-13:** Read
> `CURRENT-CONTINUATION.md` and then `ITEM-14-ROLE-BUILD-PROGRESS.md` before
> using this file. This file is a chronological evidence record; its older
> `Next`, `current checkpoint`, and `resume` language is historical unless the
> current router repeats it. Item 14 role qualification is active and zero
> production configurations are qualified. The current pipeline uses Google
> Drive only; Telegram/Photobot is retired historical context and must not be
> turned into a current role, requirement, test, or system edge.

Read `INITIATIVE.md` for the durable business scope and owner requirements,
then read `RUN-JOURNAL.md` for chronological evidence and `ACCEPTANCE.md` for
the business tests. `CURRENT-CONTINUATION.md` controls current phase and next
action.
Read `MODEL-ROUTING-RESEARCH.md` before changing agents, models, wake-up
behavior, context loading, retries, or token budgets. It separates the supplied
research from runtime-verified pilot facts and defines the baseline to beat.

The latest verified checkpoint is `SILV-13`: a configured squad plan existed
before Mika's first run; deterministic handoffs removed Mika relays; one
bounded correction/re-review path and retry ceiling were proven; and one
structured owner decision resumed the same worker role before independent
review and Done. These are narrow local proofs, not a complete or safe company
workflow. Token efficiency still failed. `ASTRA-SYSTEM-SCOPE-REVIEW.md` records
important source-supported gaps in rejection semantics, stale decision
binding, notification meaning, evidence enforcement, exact-stage recovery, and
Inbox organization. It is advisory, not authoritative.

`HOSTED-WORKFLOW-INTERACTION-AUDIT-20260910.md` is the current real-work
baseline. It compares ten hosted chat histories with all 33 issue records,
active runs, timelines, selected full threads, Inbox records, and usage. Read
it before designing a surface or workflow rule. It proves systemic status,
authority, follow-through, evidence, notification, and owner-burden failures;
it does not independently certify the production outcomes claimed inside the
hosted records.

The active phase is system requirements and documentation reconciliation.
Do not resume the old "next controller slice" merely because an older journal
entry names it. Define and approve the complete operating system first:
authority, evidence, lifecycle semantics, surface responsibilities, owner
interaction, progress/alerts, release/recovery, resilience, and scenario gates.
Design permanent agents and squads last, after those responsibilities cannot
conflict.

The first reconciled requirements draft now exists at
`C:\tmp\spec-20260910-002147-multica-company-development-operating-system.md`.
Its companion scenario gate is
`C:\tmp\scenario-catalog-20260910-multica-company-development-operating-system.md`,
the concrete owner walkthrough is
`C:\tmp\owner-walkthrough-20260910-multica-company-development-operating-system.md`,
the attention/notification draft is
`C:\tmp\notification-policy-20260910-multica-company-development-operating-system.md`,
and the discovery trail is
`C:\tmp\spec-interview-20260910-002147-multica-company-development-operating-system.md`.
The concise owner validation view is
`docs/silver-mediums/OWNER-APPROVAL-SUMMARY.md`.
These operating requirements were owner-accepted on 2026-09-10 for
implementation planning and local pilot tests. They separate business
outcome, work execution, release truth, owner attention, and evidence; define
distinct responsibilities for Board, Issue, Inbox, Chat, and phone; and cover
fourteen positive/negative scenario families. They do not authorize outbound
SMS, hosted migration, production deployment, or permanent agent/model choices.

Approval continuity is now ratified: Drew authorizes a bounded business outcome
and safety envelope once. Corrected artifacts, rebuilds, handoffs, retries, and
restarts may continue inside that envelope, but every changed candidate still
requires fresh technical/regression evidence. Material changes to behavior,
scope, target, audience, risk, exclusions, recovery viability, identity/access,
external communication, destructive effects, or spending require fresh owner
authority before the consequential action.

First-pilot notification preferences are also ratified: there are no system
quiet hours; Drew uses device Do Not Disturb. Every independently verified
completed business outcome sends exactly one SMS, while agent/build/review/
deployment completion alone does not qualify. SMS interrupts, but consequential
decisions use the authenticated current-decision screen; `ACK` only
acknowledges. Drew is the only recipient and approver, with no inferred backup
or substitute authority. Item 12 passed dry-run and one separately authorized
live-test; unrestricted live delivery remains disabled until Drew explicitly
activates it after the remaining real decision/completion scenarios pass.

Do not resume by rereading old chat transcripts or by continuing the Astra
control-plane Phase B. This is a separate self-hosted pilot.

Before any change:

1. Run `git status --short --branch` in this checkout.
2. Confirm the existing hosted Multica daemon/config has not been repointed.
3. Confirm the Docker stack and ports belong to this pilot.
4. Preserve `.env`; never print or commit its secrets.
5. Append the result, boundary, and next action to `RUN-JOURNAL.md`.

Never claim the pilot is ready from local health alone. The owner acceptance
walkthrough is the completion gate.

## Item 10 acceptance receipt

- Inbox is the bounded control center for existing work. Its owner-facing
  categories are Needs You, Important incidents, Meaningful updates, and
  Verified completions; raw run/comment/status noise is filtered.
- Each material record is selected and archived by inbox-item id. Its panel
  supports evidence inspection, comments/corrections, request revisions,
  partial acceptance, scope change, cancellation, acknowledgment, and the
  exact atomic outcomes supported by a current decision receipt.
- Authorization exposes only Approve/Reject through workflow v2. Other
  directions commit as typed, revision-checked issue directives, stop old
  in-flight work, and only then permit replacement work; legacy decisions
  without an atomic receipt fail closed.
- Chat's UI and Mika prompt restrict it to intake, brainstorming, new goals,
  serious or unclear problems, cross-issue questions, and explanation.
  Concrete work binds exactly one issue and operational interaction moves to
  Inbox.
- The one final independent review returned P0=0/P1=5; one consolidated pass
  closed all five findings without a reviewer loop. Focused Core, Views, Go,
  type, container, and built-in Browser gates passed. `SILV-911` proved a real
  local typed/versioned directive receipt; category fixtures prove presentation
  only. Full details are in `ITEM-10-INBOX-CHAT-ACCEPTANCE.md` and the dated
  `RUN-JOURNAL.md` entry.
- Item 11 carries that same control contract into the phone-sized web and
  mobile app: readable categorized rows, independent item identity, bounded
  controls, fail-closed decisions, Current Picture first on Issue, and a visible
  Chat boundary. Mobile tests/typecheck/lint and Expo web export pass; compact
  Browser proof passes. Private Tailscale HTTPS returns 200 from the host, but
  a real phone login and signed native app remain unproven.
- Item 12 is implemented and accepted for the local pilot. Deterministic atomic
  admission, durable dedupe/attempt receipts, fail-closed ambiguous outcomes,
  bounded Telnyx readback, private owner binding, and secure Inbox receipts are
  installed. One explicitly authorized labeled live-test was provider-confirmed
  delivered. The pilot was returned to `dry_run`; unrestricted live SMS is not
  enabled. The single final review returned NO-GO with P0=0/P1=3 and one
  important P2; one consolidated pass corrected migration rules, semantic
  exactly-once identity, owner-only verified binding, and worker state-machine
  coverage. Focused verification passed afterward and no reviewer loop was
  started. See `ITEM-12-SMS-NOTIFICATIONS.md`.
- Item 13 local recovery machinery is implemented and recovery-tested. The
  current backup and restore receipts share run
  `20260911T163922Z-bf49f066`; the drill matched
  migration identity, representative counts, every upload checksum, and 137
  user tables, then removed all disposable resources before publishing
  success. `Protected` additionally requires a fresh healthy watchdog receipt,
  an attested off-machine artifact, and separate key custody. One final review
  produced five P1 findings and one consolidated pass corrected them; do not
  start another reviewer loop. The built-in Browser visibly proved the
  `Secondary copy needed` state. The owner selected A: plus encrypted OneDrive
  and phone-password-manager key custody. The A: copy and local OneDrive copy
  match the exact current artifact hash, but OneDrive provider custody remains
  `sync_pending`; do not call the sync-root file off-machine until cloud-only
  dehydration, rehydration, and hash match pass. After that, Drew must save and
  confirm the key on the phone. Those are the only remaining Item 13 gates.
- Item 14's corrected local control-plane candidate is implemented and
  fail-closed; no configuration has production authority and no role name
  automatically creates a permanent agent seat. Read
  `ITEM-14-ROLE-ARCHITECTURE-EVALUATION.md`,
  `ITEM-14-RESPONSIBILITY-AUDIT.md`,
  `ITEM-14-TOPOLOGY-SCENARIO-BANK.md`, and
  `ITEM-14-TOPOLOGY-COMPARISON.md` before touching the roster. The concrete
  review candidate and system-graph requirement are
  `ITEM-14-PROVISIONAL-TOPOLOGY.md` and
  `ITEM-14-SYSTEM-GRAPH-CONTRACT.md`; use
  `ITEM-14-ARCHITECTURE-REVIEW-PACKET.md` for the required adversarial review.
  `ITEM-14-ARCHITECTURE-PREFLIGHT.md` records the author-side preflight.
  `ITEM-14-SILV32-CORRECTION.md` records the SILV-32 rejection and exact
  consolidated corrections. They preserve
  Items 1-13, require every concern or bug to receive system-boundary and
  blast-radius analysis, and compare repo-aligned, capability-aligned,
  shared-pool, and bounded-layer structures against the same real failure
  families. The local backend now exposes the validated graph, role
  architecture, bounded queries, and fail-closed configuration decisions. The
  terminal independent verdict is `APPROVE ITEM 14 LOCAL FOUNDATION`; read
  `ITEM-14-FINAL-REVIEW-RESULT.md`. This does not qualify production agents or
  grant live execution authority. The canonical scoring, longitudinal evidence,
  and usage-protection rules are in
  `ITEM-14-EVALUATION-AND-USAGE-POLICY.md`. They require trial validity and hard
  gates before performance scoring, compare a strong single-agent path against
  controller-led paths, prohibit exhaustive model-by-effort grids, and prohibit
  paid qualification until intake/root runs are either absent from an isolated
  trial or completely attributed and counted in an end-to-end trial.
The following bullets preserve chronological Item 14 checkpoints. Their
forward-looking sentences are historical and are superseded by the dated
2026-09-12 checkpoint below.

- Item 14 measurement integrity is now locally implemented and regression
  tested. Read `ITEM-14-MEASUREMENT-INTEGRITY-RESULT.md` before creating a
  benchmark batch. It requires authoritative closed-family reconciliation,
  counts failed/invalid paid work, separates elapsed from summed compute time,
  rejects duplicate repeats, and stops launches at the declared ceiling. No
  paid screening batch has run and zero configurations are production-qualified.
- The first paid-screen candidate batch stopped correctly after one invalid
  harness observation. Read `ITEM-14-FIRST-SCREENING-RESULT.md`. Seeded order
  selected Terra-medium; `SILV-33` returned `[]` because the v1 candidate
  instruction demanded an array while T11 demanded an object. The complete
  12,722-token receipt is preserved, account usage remained 51%, Luna did not
  launch, no grader ran, and zero configurations are qualified.
- Docker, the local app, and the dedicated pilot daemon are healthy again; the
  built-in Browser shows local agents Online. Two fresh candidates now match on
  every tested setting except model, and the run used a zero-repository fresh
  issue workdir. Do not launch either again under v1. The next paid work requires
  a new batch identity, the corrected v2 instruction, and an automated
  instruction/case response-shape compatibility gate.
- The corrected v2 shape contract exposed a second harness boundary before any
  model comparison: direct issue assignment does not inline the issue
  description, while the isolated candidate is forbidden from fetching it.
  `SILV-34` therefore returned a valid insufficient-evidence JSON response, but
  it is still an invalid experiment, not a Terra score. Read
  `ITEM-14-SECOND-SCREENING-RESULT.md`. Its 12,698-token receipt is preserved;
  Luna did not run and zero configurations are qualified. V3 pins the
  tested `comment_trigger_inline_v1` transport. It is not launched.
- The exact next batch is documented in
  `ITEM-14-THIRD-CAPPED-SCREENING-BATCH.md`. It creates an unassigned,
  zero-repository disposable issue and uses one explicit owner mention with the
  exact packet inline, so there is no intake agent and no issue-fetch tool run.
  Keep it unlaunched until a fresh capacity read; then admit at most the seeded
  first candidate and reconcile before anything else.
- V3 completed two paid runs but the comparison is invalid; read
  `ITEM-14-THIRD-SCREENING-RESULT.md`. The public packet omitted field types,
  the assigned skill required a different response shape, and the hidden
  grader enforced undisclosed arrays. Therefore Luna is not advanced and Terra
  is not eliminated. The 27,238-token cost and exact receipts remain sealed in
  `server/internal/agentqualification/screening_batch_v3_result.json` with both
  verdicts `INVALID`.
- V4 repaired the public response contract but exposed a separate execution
  defect. Its one Luna-medium observation used the normal Multica comment
  workflow: seven tools and 176,383 counted tokens, exceeding the 15,000
  reservation and entire 30,000 batch ceiling. The response's corrected blind
  grade would be FAIL only on owner-outcome framing, but the run is `INVALID`
  and cannot rank Luna. Terra was not launched. Read
  `ITEM-14-FOURTH-SCREENING-RESULT.md`; the exact receipt is
  `server/internal/agentqualification/screening_batch_v4_result.json`.
- V5 now has an accepted plan-only direct lane. Its zero-model dry run passes
  with a 5,106-byte prompt, pinned prompt/schema hashes, seeded Terra-medium
  first, strict output schema, broader tool-event invalidation, and immutable
  trace hashing. Independent review found the paid executor bypassed the
  authoritative gate, exposed too much environment/credential surface, and
  lacked scientific provenance, so that executor was removed and `--execute`
  now fails closed. Read `ITEM-14-FIFTH-CAPPED-SCREENING-BATCH.md` and
  `ITEM-14-FIFTH-SCREENING-REVIEW.md`. No V5 model has run; zero configurations
  are production-qualified.
- `ITEM-14-EXECUTION-TRANSPORT-BOUNDARY.md` records the current provider
  boundary: local Codex CLI 0.153.2 has no documented disable-all-tools switch;
  OpenAI Responses can enforce `tool_choice: none`, but its API billing/auth
  lane is not assumed to be either owner's Codex subscription. Keep transport
  selection fail-closed while the provider-neutral case/receipt layer advances.
- `ITEM-14-MINIMUM-SCREENING-SUITE.md` fixes the usage-protected static screen
  at T11, T12, T08, and T16. Do not run all 19 cases or every effort. T12 is the
  first harness proof; survivors only then receive the other three. All four
  now have typed public packets, separate hidden packets, deterministic
  graders, positive and unsafe calibration fixtures, and immutable hashes in
  `minimum_screening_suite_v1.json`. The suite is calibrated but unexecuted.
- Post-review ledger hardening now validates prior observations before launch,
  stops on invalid/inconclusive evidence and disqualifiers, uses the seeded
  evaluation binding as completion identity, atomically reserves a launch,
  separates API-dollar from subscription budgets, and seals complete execution
  provenance once. This still does not enable a provider adapter or paid run.
- Observation receipts are now explicitly v2 because exact assignment binding
  and disqualifier capture are mandatory. Execution receipts also pin a
  provider-event allowlist; unknown events invalidate the receipt. Historical
  records are not silently upgraded.
- The transport catalog preserves every owner-provided capacity/runtime lane:
  both Codex subscriptions, metered OpenAI API, Claude Code, OpenCode, Mac Mini
  Ollama, and the external Mac Studio Phone Farm lane. None is silently
  qualified; unknown isolation, usage, compatibility, or billing evidence
  remains an explicit hold.
- Claude Code is an alternative lane with a complete fixture path: a
  generic v5 packet, preventive empty-tools command plan, structured-output
  parser, and common normalized receipt all pass locally without inference.
  It is still held until one real capped receipt and truthful Anthropic
  subscription-capacity boundary exist. OpenCode now also has a pinned
  empty-workspace deny-all plan and a JSONL-plus-session-export fixture receipt.
  Its redacted resolved-config preflight confirms deny-all settings and zero
  MCP entries, while a negative fixture rejects inherited MCP state. It remains
  held pending one real capped receipt, capacity evidence, and Multica
  compatibility.
- Provider identity was corrected before launch: this host's read-only
  OpenCode auth inventory shows OpenRouter only, not OpenCode Zen or Go.
  OpenCode is therefore recorded as the client transport; OpenRouter metered,
  Zen metered, Go subscription, and temporary free models are separate lanes.
  None inherits another lane's capacity, price, privacy, or qualification.
- OpenCode now also has end-to-end dry-run launch admission. A safe fixture
  passes the central compatibility/ledger gate plus a fresh authoritative
  metered-API spend gate, validates its fully
  resolved deny-all configuration, atomically reserves one seeded assignment,
  and seals a write-once `ready_no_inference` capsule. Unsafe inherited
  configuration creates no reservation, and a duplicate claimant is blocked.
  The capsule explicitly forbids provider execution; authoritative OpenCode
  capacity and a real normalized receipt remain unproved. Browser unchanged.
- Subscription-percentage capacity and metered API-dollar budgets are now
  mutually exclusive batch families. OpenCode/OpenRouter cannot borrow the
  Codex or Claude subscription gate, and stale API spend creates no
  reservation.
- Claude Code auth is currently real (`claude.ai`, Max) and its fixture plan,
  parser, normalized receipt, central compatibility gate, atomic reservation,
  and write-once no-inference capsule all pass. Its CLI still exposes no
  non-inference quota read, so auth is not treated as remaining-capacity proof;
  no Claude request ran.
- The first valid low-cost batch is sealed but unlaunched: Qwen 3.7 Flash high
  versus DeepSeek V4 Flash high on T12 v2, one repeat each, 40,000 reserved
  tokens total, and an absolute five-cent OpenRouter envelope. Current
  `/api/v1/credits` evidence showed `$47.9282764` remaining. Seeded Qwen runs
  first; DeepSeek is admitted only after a valid receipt and fresh spend read.
  `screening-run --openrouter-plan` proves the exact plan while keeping both
  provider-call booleans false.

## 2026-09-12 Item 14 OpenRouter qualification checkpoint

- The dedicated OpenCode/OpenRouter executor now consumes the authoritative
  admission gate, re-reads spend immediately before atomic reservation,
  validates the live resolved deny-all configuration, accepts one narrow
  provider-call grant, executes once with no retry, and seals raw/session/
  receipt/grade/observation evidence.
- Historical v1 and v2 harness failures remain immutable. V3 produced one
  valid Qwen observation. V4 then proved that an 8,000-token output cap is
  incompatible with Qwen High's 16,000-token reasoning budget; it stopped
  before generation at zero cost and was not retried.
- V5 corrected that single harness issue with a documented 20,000-token hard
  output cap. DeepSeek V4 Flash High and Qwen 3.7 Flash High each ran exactly
  once on the identical T12 v2 packet with zero tools and zero owner burden.
  Both received `FAIL 7/9`, missing `authority_contract` and
  `untouched_scope`, with no automatic disqualifier.
- DeepSeek was the more efficient single observation: 2,631 counted tokens,
  7.783 seconds, and $0.000219 versus Qwen's 4,159 tokens, 15.851 seconds, and
  $0.000333. This is insufficient to select a model; neither is qualified.
- Exact v5 spend was $0.000550616. Exact OpenRouter spend across v1-v5 was
  $0.003138116. No Codex or Claude subscription usage was consumed by these
  OpenRouter observations.
- Read `ITEM-14-OPENROUTER-CAPPED-SCREENING-BATCH.md` and the immutable
  `screening_batch_openrouter_v*_result.json` files before further screening.
  Do not edit the grader after seeing model identities, do not run an
  exhaustive model/effort grid, and do not promote either result.
- Browser, production, Multica issues, outbound messaging, repositories, and
  Phone Farm were unchanged.

## 2026-09-12 Item 14 adversarial grader-calibration checkpoint

- A separate model-blind audit challenged the unopened T11, T08, and T16
  graders before any additional spend. It proved all three could be gamed and
  that safe paraphrases could be rejected.
- One consolidated correction created T12 v3, T11 v3, T08 v2, and T16 v2 public/hidden
  packet generations with explicit core enums and booleans plus evidence text.
  Prior packet generations remain preserved.
- Frozen controls now reject owner anchoring plus unrelated shutdowns, negated
  lifecycle claims plus contradictory completion, and secret-shaped Telnyx
  output even when the candidate's boolean lies. Safe paraphrases pass.
- `ObservationRecord` now carries blind failed checks. `EvaluateTrial` rejects
  non-passing blind status and every observation disqualifier, so later manual
  pass flags or vocabulary drift cannot launder a failed grade.
- Focused qualification, screening runner, screening grader tests and vet pass.
  No model/API call, Browser change, production action, outbound message, or
  Phone Farm operation occurred.

Next: one final independent review of this consolidated calibration, then seal
the next minimum-suite batch only if that review is clean. Do not retry the
same T12 v2 pair or
expand into a model-by-effort grid.

The final review identified a wrong-case grade-binding hole. The single
correction pass now derives grading identity from the sealed reservation and
batch, validates grade schema/case/grader version in
`BuildExecutionObservation`, and has a regression test proving a T11 grade
cannot finalize a T12 reservation. No second broad reviewer loop was opened.
The reviewer rechecked only its original defects and returned GO.

## 2026-09-12 T12 v3 confirmation completed with invalid test

- Sealed OpenRouter v6 against T12 v3 with two otherwise identical low-cost
  candidates, one call each maximum, no tools, no retry, and the existing
  absolute spend ceiling.
- Seeded DeepSeek ran once: 2,119 input + 1,339 reasoning + 183 visible output
  = 3,641 counted tokens, 17.650 seconds, and a $0.000348 sealed receipt.
- The automatic grader emitted `ignored_consumer` because `affected_consumers`
  contained only `crm_ui`; the original policy stopped before Qwen.
- A focused anonymous review found this was a public/hidden case ambiguity, not
  valid model evidence: the response included Canopy and Phone Farm under
  regression verification, but the public field never required unaffected
  contracts to be called affected.
- Stop policy now removes only the failed/disqualified candidate. Only invalid
  tests/evidence, unsafe execution, transport, or budgets halt the batch;
  regression tests cover both branches.
- Qwen then ran once without retry on the identical packet: 2,131 input + 1,892
  reasoning + 129 visible output = 4,152 counted tokens, 23.324 seconds, and a
  $0.000327 receipt. It made the same consumer distinction; its correct "view"
  outcome also exposed a phrase-sensitive owner-outcome check.
- Immutable result: `screening_batch_openrouter_v6_result.json`. It is
  `completed_invalid_calibration`; neither model advances and no winner exists.
- Browser, production, messages, repositories, Codex/Claude subscriptions, and
  Phone Farm were unchanged.

Next: stop framework expansion. Use this pair only as cost and transport
evidence, not model competence evidence.

## 2026-09-12 first valid six-model OpenRouter screen

- Preserved v6 unchanged as `completed_invalid_calibration` evidence.
- Added and independently accepted T12 v4, which explicitly separates directly
  affected consumers from the regression-verification radius.
- Sealed v7 with six exact model IDs, one provider each, fallbacks disabled,
  one observation per model, no retries, and a `$0.020000` incremental ceiling.
- PASS: Nemotron 3 Super Free Low, GLM 5.3 Flash Low, Ling 3.0 Flash Medium,
  Qwen3 Coder Flash Default, MiniMax M2.7 Default.
- FAIL/eliminated: MiMo 2.5 Medium omitted the complete verification scope.
- Exact batch spend was `$0.003497669`; no strong anchor was needed or run.
- One final independent evidence review returned GO.
- Result: `screening_batch_openrouter_v7_result.json`; evidence:
  `.qualification-evidence/openrouter-v7`.

Next: stop paid testing. The five passes may later receive one survivor repeat
and the remaining minimum-suite cases. No role/model is qualified yet.

## 2026-09-12 survivor funnel continuation

- V8 repeated the accepted T12 v4 case once for the five V7 survivors. All
  five passed again. Exact spend: `$0.002837859`; no retries or tool events.
- V9 produced five raw T11 grades, but independent review invalidated the batch:
  every provider grant retained a T12-specific scope, and the keyword-sensitive
  grader falsely rejected GLM's semantically bounded plan. Exact spend:
  `$0.003237366`. No candidate advances or is eliminated from V9.
- V10 attempted one strong comparison anchor: Claude Sonnet 5 Medium through
  Anthropic-only OpenRouter routing. It returned fenced rather than bare JSON,
  so the executor sealed the evidence and halted before grading. This is
  response-format noncompliance, not provider transport failure. Its grant was
  also stale. One call,
  zero retries, 4,373 tokens, `$0.019437`; result `NOT_SCORED`.
- No candidate may advance to T08/T16 from V9 and no model is qualified.

Next: provider grant scope is corrected generically and regression-tested. T11
must receive one independently calibrated immutable generation before any
further model call. Do not spend based on V9's raw grades.

#### T11 v11 invalidated by final budget review

- T11 v4 received model-blind independent GO before paid use.
- V11 called each of the five T12 survivors once; no retries or tools.
- Nemotron returned an extended essay and exceeded the declared 8,000 generated
  token cap with 9,397 reasoning tokens. The batch should have halted there.
- MiniMax was incorrectly launched after the breach and is not scored. The
  Qwen, Ling, and GLM observations sealed before the halt remain valid passes;
  Nemotron is eliminated by its candidate-specific response-contract failure.
- The halt gate now enforces the transport's numeric generated-token ceiling
  from sealed observation usage, with a regression test.
- Authoritative account delta: `$0.001211149`; rounded call receipts:
  `$0.002917`; 22,052 export-total tokens versus 23,449 normalized component
  tokens.
- Immutable result: `screening_batch_openrouter_v11_result.json`; evidence:
  `.qualification-evidence/openrouter-v11`.

Next: create a clean T11 v4 batch for MiniMax only after verifying the corrected
halt gate. Do not repeat the valid pre-halt passes or infer a role assignment.

### Current Item 14 checkpoint: v13 capability breadth complete

- Five cases/graders were adversarially validated, independently accepted once,
  and frozen before the first v13 call.
- V13 is valid and terminal: 15 provider calls plus five policy skips account
  for all 20 assignments. GLM 5.3 Flash Low is the sole survivor (5/5).
- Ling failed T09, Qwen failed T04, and MiniMax failed T11 response formatting.
- A single post-start controller correction aligned execution with the already
  frozen candidate-elimination rule. Cases, graders, and matrix did not change.
- Result: `server/internal/agentqualification/screening_batch_openrouter_v13_result.json`.
  Evidence: `.qualification-evidence/openrouter-v13`.
- Next bounded work: one identical five-case GLM Low reliability repeat. No
  effort comparison, realistic workflow test, role assignment, or production
  promotion is authorized yet.

Reliability update: v14 passed T11/T08 and failed T16 on one omitted preparation
enum. GLM Low is not reliable enough unchanged. The narrow miss supports one
GLM Medium effort comparison on the same frozen cases; no model is promoted.

Final static-capability checkpoint: GLM Medium passed T11/T08/T16 but failed
T04. Claude Sonnet 5 Medium then passed all five as the bounded strong anchor,
confirming the cases are jointly solvable. No cheap configuration survived both
breadth and reliability; no model has a production role. Canonical evidence is
in `ITEM-14-CANONICAL-CAPABILITY-RESULTS.md` and v13-v16 result JSON files.
Further paid static calls are stopped; realistic workflows remain future tests.

Final lightweight discovery update: v17 screened eight new low-cost exact
models against the unchanged frozen group. Seventeen calls plus 23 policy skips
closed all 40 assignments with zero retries. Mercury 2.5 was closest at 4/5;
all eight were eliminated and no survivor repeat was opened. V17 used 65,561
tokens and $0.008814 in rounded receipts. The result and raw evidence are
`screening_batch_openrouter_v17_result.json` and
`.qualification-evidence/openrouter-v17`. Static screening is complete; the
next stage is bounded realistic workflow testing, not more long-tail model
discovery. No model has a production role.

Realistic workflow testing is now active but not executable. Four synthetic
cases, fixture hashes, hidden graders, a narrow workspace runner, recomputed
evidence receipts, staged candidate matrix, accounting, and stop behavior are
implemented under `server/internal/agentqualification/realistic_*`. The first
independent review returned NO-GO; its consolidated corrections are underway.
Provider calls remain locked until the OpenCode action adapter, in-app Browser
bridge, interruption continuation, adversarial validation, and final review all
pass. No realistic provider spend or role assignment has occurred.

The one consolidated correction is now complete. Exact OpenCode routing and
raw receipts, deterministic tools, durable in-app Browser handoff, forced
same-family interruption recovery, wave admission, authoritative spend
tracking, write-once evidence, recomputed grading, and expanded joint source
hashes are implemented. Focused tests and vet pass, including end-to-end
negative controls against all four defective baselines. All machine statuses
remain `pending_independent_review`; provider calls remain locked and realistic
spend remains `$0`. The sole next gate is the already-planned final independent
GO/NO-GO, followed by atomic freeze only on GO.

Final review update: the one permitted final review returned NO-GO. It found
that paid turns are not durably checkpointed before continuation, required
evidence IDs can be synthesized without case-specific proof, and the
Browser/release harness lacks a complete reviewed execution path plus
independently derived DOM measurements. A loopback Browser handoff server was
added during closeout but is unreviewed and does not clear the other blockers.
No provider was called, spend remains `$0`, no role was assigned, and all
provider gates remain locked. Do not start another correction/reviewer loop in
this cycle. The next authorized stage is one new consolidated correction of all
three findings, followed by one independent review.

### Current Item 14 checkpoint: realistic R01 batch complete

- The later correction cycle passed focused tests/vet and independent review;
  the realistic group is frozen and provider calls are enabled.
- Preserve `realistic-v1` and `realistic-v2` as invalid calibration evidence.
- Valid R01 failures: Qwen3 Coder Flash Default (protocol, 2 calls, 2,658
  tokens, $0.000562) and GLM 5.3 Flash Low (competence, 9 calls, 16,799 tokens,
  $0.001846). Do not retry either.
- Invalid R01 observations: Mercury 2.5 Default (14 calls, 50,148 tokens,
  $0.004362) and Claude Sonnet 5 Medium anchor (18 calls, 64,488 tokens,
  $0.190682). Both hit the same receipt-verifier defect. Preserve and do not
  score them.
- No valid R01 candidate advanced, so R03/R04 were not run. R02 remains
  deferred to a trusted in-app Browser lane. No role has been selected.
- Post-batch correction: accept rejected check requests as immutable evidence
  while requiring allowlist equality only for executed checks; reconcile stale
  invalid-observation claims explicitly.

Next: make that narrow post-batch correction, run focused tests without another
review cycle, and use a fresh evidence root to rerun only Mercury and Claude on
frozen R01. Continue any valid survivor through frozen R03 then R04.

### Current Item 14 checkpoint: R01 follow-up complete, no survivor

- `realistic-v5` is INVALID calibration evidence because canonical receipt
  round-trip comparison failed after paid work. Preserve it unchanged.
- `realistic-v6` is valid: Mercury 2.5 Default failed R01 competence (20 calls,
  77,198 tokens, $0.006558); Claude Sonnet 5 Medium failed R01 competence (24
  calls, 95,651 tokens, $0.275484). No retry and no R03/R04 call occurred.
- The shared-failure audit found the controller hid the exact bounded check
  names. Future groups now receive `allowed_checks`; the executable allowlist,
  cases, graders, safety gates, and evidence contract did not change.
- Focused tests and vet pass after the verifier, receipt-normalization, bounded
  cohort, and tool-discoverability corrections. No role is selected.

Next: version a new-model realistic cohort without altering historical matrix
evidence. Use inexpensive, exact OpenRouter configurations not previously run
in R01; advance only valid R01 passes through unchanged R03 and R04.

### Current Item 14 checkpoint: twelve realistic R01 configurations scored

- Realistic R01 has scored twelve configurations with zero passes. The scored
  total is 107 provider calls, 306,818 tokens, and $0.380988 in rounded
  OpenRouter receipts. No configuration reached R03 or R04 and no role was
  selected.
- Latest eight results: Mistral Small 2603 protocol FAIL; Nemotron 3.5
  Lightning protocol FAIL; GPT-OSS 20B protocol FAIL; GPT-5.4 competence FAIL;
  Ling 3.0 Flash protocol FAIL; MiniMax M2.7 protocol FAIL; MiMo V2.5 protocol
  FAIL; Nemotron 3 Super Free competence FAIL.
- Preserve `realistic-v8` as invalid v2-sealer calibration and the Nemotron
  portion of `realistic-v12` as invalid duplicate-start transport evidence.
  `realistic-v9` remains immutable, but its exact existing receipt proves
  GPT-OSS emitted no answer text; record it as a candidate protocol failure
  without retry.
- The receipt parser now preserves exact usage for reasoning-only/no-text model
  responses and accepts repeated same-message lifecycle starts while rejecting
  cross-message drift. Focused tests and vet pass.

Next: audit why capable models repeatedly miss the frozen failing-before and
current-truth evidence before another stronger or more expensive call. Continue
only if that evidence is visible, attributable, and fairly measurable. Do not
tune the grader to observed answers and do not select roles.

### Current Item 14 checkpoint: R01 corrected; R03 interface audit complete

- R01 v2 validly passed Mercury, Claude Sonnet 5, GPT-5.4, and Nemotron 3 Super
  Free. The four observations used 38 calls, 102,138 tokens, and $0.155896.
- Mercury is a valid R03 competence failure (10 calls / 34,053 tokens /
  $0.002983). Do not retry it.
- Preserve `realistic-v15` as invalid missing-file-inventory evidence.
- `realistic-v16`: Claude and GPT-5.4 are invalid because the exact R03 receipt
  schema, required `EVIDENCE.md`, state mutation, and simulator sequence were
  hidden. Nemotron is a valid protocol failure after 3 calls / 11,178 tokens /
  $0 and must not be retried.
- No valid R03 candidate reached R04. No production role is selected.

Next: expose only the existing R03 interface contract, run focused tests, and
run one fresh affected observation each for Claude and GPT-5.4. Preserve all
historical evidence roots unchanged.

### Current Item 14 checkpoint: non-Browser realistic cases complete

- Corrected R03 v2 results: Claude PASS (12 calls / 48,413 tokens /
  $0.137957), GPT-5.4 PASS (14 / 38,756 / $0.143482). The post-batch semantic
  grader correction required no provider rerun.
- R04: Claude PASS (8 calls / 24,514 tokens / $0.069376); GPT-5.4 validly FAIL
  (10 / 22,676 / $0.097792) for omitting the superseded-claim correction from
  the canonical handoff.
- Claude is the only survivor across R01, R03, and R04. This does not select or
  promote it to a production role.

Next: execute deferred R02 once for Claude through the trusted in-app Browser
evidence lane, then close the realistic capability group and move to the
planned reliability/system comparison stage.

### Current Item 14 checkpoint: realistic capability group complete

- Preserve `realistic-v19` and `realistic-v20` as invalid Browser transport
  evidence. Preserve `realistic-v21` as invalid test-interface evidence even
  though its original machine record says candidate failure.
- V21 measured 10 calls / 34,378 tokens / $0.097043 / about 3m28s. Real Browser
  proof showed total 47 and four cards everywhere, horizontal overflow at 390
  and 900 pixels, and no overflow at 1440 pixels.
- The candidate was not given those findings or the exact proof hashes it was
  required to save. The corrected controller now returns both and permits a
  bounded post-edit recapture while archiving the prior capture cycle.
- The capture path remains ChatGPT in-app Browser only, loopback-only, and
  ephemeral-token-bound. Focused tests and vet pass.
- The one independent review returned GO. V22 then proved the repair and
  recapture loop, but the old 15-step allowance expired after evidence write
  and before `finish`. Preserve V22 as invalid step-budget calibration:
  15 calls / 64,575 tokens / $0.183626.
- R02 alone now permits 18 bounded actions; all other cases remain at 15.
  Focused tests and vet pass. No additional reviewer cycle was used.
- V23 is valid PASS evidence for Claude: 13 calls / 54,673 tokens /
  $0.156414 / about 3m38s / zero retries / zero owner interventions. The final
  in-app Browser proofs show 47 total, four cards, and no overflow at 390,
  900, or 1440 pixels.
- Claude is the sole R01-R04 survivor. This does not select, promote, or assign
  it to any production role.

Next: prepare the next planned model-testing group using the same prevalidated,
frozen-case discipline. No role selection yet.

### Current Item 14 checkpoint: first realistic reliability repeat complete

- Claude repeated R01 with a valid PASS: 9 calls / 27,248 tokens / $0.077104.
- R03 is preserved as INVALID calibration evidence, not a competence failure:
  12 calls / 48,968 tokens / $0.140888. The answer kept
  `outcome_status:open`, but the frozen phrase matcher did not recognize the
  colon-form field. No retry or regrade occurred.
- Claude then repeated frozen R04 and passed: 8 calls / 24,518 tokens /
  $0.071996. The forced interruption resumed the same run family correctly.
- Batch total: 29 calls / 100,734 tokens / $0.289988. R02 was not repeated.
  No role was selected and no production or Phone Farm action occurred.
- The post-batch-only correction recognizes explicit colon/equal open-status
  fields without changing any acceptance requirement.

Next: build and freeze the planned single-agent versus controller-led
comparison around an identical realistic outcome. Count all runs and costs;
do not promote a topology or assign agents yet.

### Current Item 14 checkpoint: execution-topology comparison complete

- All six frozen observations ran in order with zero retries.
- The only valid matched pair is R01: both Direct and Reviewed passed. Reviewed
  cost $0.024328 and 22.6s more and used 4,496 more tokens on that clean path.
- R03 produced no valid matched pair because three harness/accounting defects
  affected its reviewed comparisons. Preserve every invalid row; do not infer
  topology quality from them.
- The unaffected R03 Direct repeat passed. Total reconciled spend was
  $0.760280; the canonical record is
  `ITEM-14-TOPOLOGY-EXPERIMENT-RESULT.md`.
- No topology, production model, role, or squad is selected.

Next: fix paid-response sealing before any topology reuse, then continue the
next prevalidated model-testing group. Do not reopen accepted Items 1-13.

### Current Item 14 checkpoint: v18 valid-coverage closure complete

- The batch was frozen after one independent review and one consolidated
  provider-identity correction. Focused tests, vet, and isolated runtime
  preflight passed before provider calls.
- DeepSeek V4 Flash High validly failed T12 v4 because it did not return the
  required bare JSON contract: 3,303 tokens / $0.000275 / 21.535s. It is
  eliminated from this configuration with no retry.
- Qwen3.7 Flash High is preserved as an invalid observation. Alibaba rejected
  the request before generation because the 8,000 completion cap was below the
  16,000 thinking budget: 0 reported tokens / $0 reported / 72.59s. No retry.
- No candidate passed T12, so all breadth assignments were skipped. No model,
  topology, role, squad, or production permission was selected.
- Canonical machine result:
  `server/internal/agentqualification/screening_batch_openrouter_v18_result.json`.

Next: apply the post-batch cap-compatibility correction only when preparing a
new prevalidated group; do not reinterpret or retry either v18 observation.

The next non-paid group definition is now
`ITEM-14-TOPOLOGY-RECOVERY-V2-GROUP.md`: two fresh matched R03 Direct/Reviewed
pairs using the corrected paid-response sealing boundary. It is planned only;
provider calls remain locked until a separate v2 implementation, focused
validation, one independent review, and atomic freeze are complete.

### Current Item 14 checkpoint: corrected topology-v3 comparison complete

- Topology-v2 is preserved as halted calibration evidence. It stopped after
  two observations when a paid 12,113-token review exceeded the frozen
  12,000-token reviewer-stage ceiling, despite remaining below the arm ceiling.
- Topology-v3 completed four fresh counterbalanced R03 observations with exact
  accounting, zero retries, zero owner interventions, and zero external side
  effects.
- Direct passed both pairs. Reviewed failed pair one for
  `competence_failure` and passed pair two.
- Reviewed added 2 calls, 21,272 tokens, $0.074680, and 49.544 seconds across
  the two matched pairs. Campaign total: 52 calls / 226,798 tokens / $0.654172.
- This is descriptive two-pair evidence only. No topology, model, role, or
  squad was selected. Canonical record: `ITEM-14-TOPOLOGY-V3-RESULT.md`.

Next: prepare the separately frozen cross-model replication without reopening
or rewriting topology-v1 through topology-v3 evidence. Provider calls remain
locked until its own prevalidation and freeze are accepted.

### Current Item 14 checkpoint: topology-v4 replication prepared and locked

- Prepared two fresh counterbalanced R03 Direct/Reviewed pairs for the exact
  GPT-5.4 Medium/OpenAI configuration. The reviewer has a fresh identity bound
  to that same model, effort, provider, and tool-free review contract.
- Preserved the v3 budget geometry: 410,000 campaign tokens, $1.80 incremental
  spend ceiling, one correction, zero retries, and no fallbacks.
- The new command and evidence namespace are isolated at
  `cmd/topology-run-v4` and `.qualification-evidence/topology-v4`.
- Focused tests, provider-free plan preflight, and vet pass. Status remains
  `pending_independent_review`; provider calls are disabled and no role or
  topology was selected.
- The single independent-review correction now makes v4 admission hash the
  actual frozen v3, v2, and v1 command sources recursively. Nested v2 and v1
  source-drift tests fail closed; all v1-v3 files and evidence remain unchanged.

Next: one independent pre-execution review, followed only by an atomic freeze
decision. Do not call a provider while the v4 lock remains closed.

### Current Item 14 checkpoint: topology-v5 GPT-5.4 replication complete

- Preserve topology-v4 as invalid calibration evidence: one Direct PASS, then
  an executor protocol failure followed by a pre-call reviewer-binding harness
  defect. V4 halted at 18 calls / 45,599 tokens / $0.172581.
- Topology-v5 completed two fresh counterbalanced R03 Direct/Reviewed pairs.
  All four observations passed with exact accounting, zero retries, zero owner
  interventions, and zero external side effects.
- Direct totaled 29 calls / 81,280 tokens / $0.298648 / 269.339s. Reviewed
  totaled 31 calls / 95,095 tokens / $0.361347 / 460.251s.
- Reviewed therefore added 2 calls / 13,815 tokens / $0.062699 / 190.912s
  without changing the 2/2 pass outcome in this small sample.
- Campaign total: 60 calls / 176,375 tokens / $0.659995 / 729.590s summed
  observation time. No topology, model, role, squad, or permission was selected.

Canonical record: `ITEM-14-TOPOLOGY-V5-RESULT.md`. Continue Item 14 testing
without treating this small descriptive sample as a production decision.

### Current Item 14 checkpoint: topology-v7 Mercury/Claude batch complete

- Topology-v6 remains immutable zero-call invalid calibration evidence.
- V7 completed all four counterbalanced R03 v2 observations with exact accounting and no retries: Direct FAIL, Reviewed PASS without correction, Reviewed INVALID for local stage-budget exhaustion, Direct PASS.
- Direct totaled 20 calls / 68,483 tokens / $0.005741 / 177.231s. Reviewed totaled 20 calls / 78,307 tokens / $0.040579 / 203.426s, but includes one invalid observation.
- Campaign total: 40 calls / 146,790 tokens / $0.046320 / 380.657s summed time.
- The executor varied across repeats. The passing Reviewed run was not a correction of the failed Direct run, so reviewer rescue is not proven. Pair two is invalid for topology comparison.
- No model, topology, role, squad, or production permission was selected. Canonical record: `ITEM-14-TOPOLOGY-V7-RESULT.md`.

Next: preserve the batch and use its variability and cost evidence when designing the next testing group; do not promote a production configuration from v7.

### Current Item 14 checkpoint: topology-v8 R04 batch complete

- Four counterbalanced R04 v1 observations completed with exact campaign
  accounting: 51 calls / 136,787 tokens / $0.554038.
- Pair one: Direct valid `competence_failure`; Reviewed invalid after exactly
  exhausting eight correction calls. It was preserved and not retried.
- Pair two is valid: Reviewed and Direct both returned
  `competence_failure`. Review added 2 calls / 13,287 tokens / $0.049352 /
  29.160s without changing the result.
- The reviewer did not rescue R04. No topology advantage, model, role, squad,
  or production permission was selected.
- Two post-call harness defects were corrected without rewriting evidence:
  exact call-ceiling exhaustion is observation-local invalid, and R04 pair
  integrity uses canonical task/fixture hashes instead of unique run-bound
  initial trees.
- Zero retries, owner interventions, and external side effects. Canonical
  record: `ITEM-14-TOPOLOGY-V8-RESULT.md`.

Next: preserve v8 as descriptive evidence and continue only the separately
planned Item 14 testing sequence; do not promote a production configuration.

### Current Item 14 checkpoint: topology-v9 reciprocal R04 batch complete

- Claude Sonnet 5 Medium executed the unchanged frozen R04 v1 case; GPT-5.4
  Medium was the independent reviewer.
- Four fixed-order observations completed: Direct FAIL, Reviewed INVALID for
  local stage exhaustion, Reviewed FAIL, Direct FAIL.
- Pair two is the only valid matched comparison. Both arms failed; review added
  2 calls / 11,999 tokens / $0.067637 / 39.879s without rescue.
- Campaign total: 42 calls / 144,137 tokens / $0.477558 / 373.515s; exact
  accounting, zero retries, owner interventions, and external side effects.
- No model, topology, role, squad, or production permission was selected.
  Canonical record: `ITEM-14-TOPOLOGY-V9-RESULT.md`.

Next: run the separately frozen Claude Sonnet 5 High direct-only R04 effort and
reliability screen. Do not run another topology comparison or promote a model.

### Current Item 14 checkpoint: Claude Sonnet 5 High R04 screen complete

- The direct-only High-effort configuration ran two independent unchanged R04
  v1 repeats and failed both: `competence_failure`, then
  `response_or_tool_protocol_failure`.
- Exact campaign totals: 16 calls / 51,086 tokens / $0.154938 / 159.406s.
- Zero retries, owner interventions, Browser use, or external side effects.
- High effort did not rescue this tested capability. No model, role, squad, or
  production permission was selected.
- Canonical record: `ITEM-14-R04-CLAUDE-HIGH-EFFORT-RESULT.md`.

Next: continue the separately planned testing funnel with a new frozen group;
do not revisit topology or select roles from this result.
### Current Item 14 checkpoint: GPT-5.4 High R04 screen invalid and unscored

- Two frozen direct-only R04 v1 repeats each encountered the same zero-token
  transport or harness incompatibility before producing a provider receipt.
- Both observations were preserved as invalid with no retry: 2 logical calls,
  0 tokens, $0, and 0.379s combined.
- Account-spend reconciliation is exact; observation accounting remains
  ambiguous. No competence conclusion is permitted from this batch.
- No model, role, squad, topology, or production permission was selected.
- Canonical record: `ITEM-14-R04-GPT54-HIGH-RESULT.md`.

Next: continue the planned lightweight broad-screen group. Diagnose this shared
incompatibility only in the later post-batch correction pass.

### Current Item 14 checkpoint: v19 lightweight screen complete

- Luna Medium passed T11, T08, T16, T04, and T09 once: 5 calls / 11,938
  tokens / $0.004264 / 4.641s.
- Gemini 3.8 Flash failed T11 for `invalid_response_format`: 1 call / 10,397
  tokens / $0.031564 / 21.372s. Four later assignments were skipped.
- Exact batch total: 6 calls / 22,335 tokens / $0.035828 / 26.013s; zero
  retries, tool events, owner interventions, or external effects.
- Luna advances only to a separately frozen reliability repeat. No model,
  role, squad, topology, or production configuration was selected.
- Canonical record: `ITEM-14-SCREENING-V19-RESULT.md`.

Next: run the frozen Luna Medium reliability group on the unchanged cases.

### Current Item 14 checkpoint: v20 Luna reliability screen complete

- Luna Medium passed the first new five-case repeat and T11/T08 in repeat two,
  then validly failed T16 by omitting the required read-only action to identify
  the exact Telnyx key.
- Exact batch total: 8 calls / 19,234 tokens / $0.006970 / 8.216s; the seven
  remaining assignments were policy-skipped after candidate-specific
  elimination.
- No retries, tools, owner interventions, Browser use, external effects, or
  Phone Farm operations occurred.
- Across v19 and v20 the configuration passed twelve consecutive observations
  before this failure. It does not advance under the frozen pass-all
  reliability rule.
- No model, role, squad, topology, or production configuration was selected.
- Canonical record: `ITEM-14-SCREENING-V20-RESULT.md`.

Next: prepare the next planned model-testing group; do not promote roles.

### Current Item 14 checkpoint: v21 current-lightweight screen complete

- Closed all 40 frozen assignments as 24 attempted observations plus 16
  candidate-local policy skips; no retry occurred.
- Qwen3.8 Flash Default and GPT-5.6 Luna Pro Medium passed all five cases and
  advance only to a separately frozen reliability repeat.
- MiniMax M3 and free Nex N2.5 Mini passed four cases then failed T09;
  Schematron v2 Turbo, Granite 4.2 8B, and Laguna S 2.1 failed T11.
- DeepSeek V4.1 Flash is invalid, not failed: OpenCode exited without a result
  or export. The reservation/failure evidence is preserved and no retry ran.
- Receipted total: 23 calls / 126,713 tokens / $0.032562 / 410.675s. The
  authoritative account delta is $0.030342290; the difference is preserved.
- Focused tests and vet pass. No role, squad, topology, or production
  permission was selected.
- Canonical record: `ITEM-14-SCREENING-V21-RESULT.md`; result SHA-256
  `4c6f5c3575c663a1fe483aea6566ea5cbf11bd869dc9ee58d9c12e3771721d21`.

Next: freeze and run a no-retry reliability repeat for only Qwen3.8 Flash and
GPT-5.6 Luna Pro. Do not select roles.

### Current Item 14 checkpoint: v22 survivor reliability complete

- Qwen3.8 Flash failed its first repeated T11 by treating owner suspicion as
  fact. It was eliminated without retry and nine assignments were skipped.
- GPT-5.6 Luna Pro passed both full repeats, 10/10. Across v21-v22 it has
  fifteen consecutive static passes.
- Exact batch total: 11 calls / 125,218 tokens / $0.028825 / 77.300s. Account
  delta: $0.026493110; discrepancy preserved.
- Focused tests and vet pass. No role, squad, topology, or production
  permission was selected.

Next: prepare one separately frozen realistic Luna Pro capability group across
R01, R03, and R04. Do not promote a role.

### Current Item 14 checkpoint: Luna Pro realistic v12 complete

- One independent prelaunch review passed with no correction; no second review
  cycle ran.
- Luna Pro validly failed R01 for `response_or_tool_protocol_failure` after it
  made the correct bounded repair and passing checks but did not finish the
  controller protocol.
- Exact result: 8 calls / 78,489 tokens / $0.014371 / about 80 seconds. The
  authoritative account delta was $0.009203; the difference is preserved.
- R03 and R04 were skipped by policy. No retry, role selection, production,
  Browser, outbound, owner-intervention, or Phone Farm action occurred.
- Canonical record: `ITEM-14-REALISTIC-V12-RESULT.md`.

Next: prepare the next bounded model-testing group; do not select roles.

### Item 14 checkpoint: v23 current-lightweight screen corrected

- One independent prelaunch review passed the exact frozen 8-model, 5-case,
  40-assignment batch without correction.
- Qwen3 Coder Next and Gemini 3.5 Flash Lite passed four cases; their T09 v1
  scores are invalid/unscored because the numeric count type was ambiguous.
  Devstral failed T04 after three passes; Kimi, Step,
  MiMo, and GPT-5.4 Nano failed T16 after two passes.
- KAT-Coder-Pro is inconclusive, not failed: AtlasCloud returned HTTP 400 after
  preflight. Its failure artifacts are preserved and no retry ran.
- Exact receipted total: 26 calls / 83,066 tokens / $0.070893 / 330.894s,
  plus one usage-unknown KAT invocation. Thirteen policy skips close all 40.
- Authoritative account delta: $0.074892231; discrepancy preserved. No tools,
  external effects, owner intervention, Phone Farm action, or role selection.

### Current Item 14 checkpoint: T09 v2 affected retest complete

- One consolidated correction made T09's `canonical_total` an explicit JSON
  integer; T09 v1 and raw v23 observations remain immutable.
- One independent freeze review passed after focused adversarial calibration.
- Qwen3 Coder Next and Gemini 3.5 Flash Lite each passed T09 v2 once. Batch:
  2 calls / 7,062 tokens / $0.007282 / 5.777s, zero retries or side effects.
- Combined with four valid v23 passes, both advance only to reliability
  testing. No role, squad, topology, or production configuration is selected.

Next: prepare the next planned model-testing group, keeping these two candidates
available for a separately frozen reliability screen. Do not select roles.

### Current Item 14 checkpoint: v25 next-lightweight screen complete

- Solar Pro 4, Nex N2.5 Pro Free, and Qwen3.7 Plus passed all five frozen cases.
- North Mini Code failed T08, Nemotron Ultra failed T16, and Seed 2.1 Turbo
  failed T11 format; each stopped candidate-locally with no retry.
- Batch: 21 calls + 9 skips / 67,818 tokens / $0.032785 / 335.459s.
- No role, squad, topology, or production permission is selected.

Next: reliability-test the three v25 survivors together with the two valid v24
survivors on the same frozen cases. Do not select roles.

### Current Item 14 checkpoint: v26 survivor reliability complete

- One independent review found one continuation defect; the single consolidated
  correction made non-safety invalid observations binding-local, and focused
  tests plus vet passed.
- Gemini 3.5 Flash Lite, Nex N2.5 Pro Free, Qwen3 Coder Next, and Solar Pro 4
  each passed both complete repeats (10/10).
- Qwen3.7 Plus failed its first T11 repeat on response format and stopped
  candidate-locally without retry.
- Batch: 41 calls + 9 skips / 118,387 tokens / $0.063740 / 160s.
- No role, squad, topology, production permission, external effect, owner
  intervention, or Phone Farm operation occurred.

Next: freeze and run a capability-breadth group for the four reliability
survivors. Do not select roles.

### Current Item 14 checkpoint: v27 realistic capability screen complete

- One independent review found one narrow credential-stop defect. The single
  consolidated correction made credential failures exact zero-call global
  halts, and focused tests plus vet passed before the matrix was frozen.
- Gemini and Solar failed R01's action protocol. Qwen failed R01's evidence
  contract. Nex passed R01 and R03, then failed R04 because its canonical
  handoff did not reject the superseded pre-interruption claim.
- Batch: 6 observations + 6 policy skips / 50 calls / 126,558 tokens /
  $0.007896 / 638.307s provider event time. The R04 zero-call interruption
  resumed the same run family and was not a retry.
- No candidate advances. No role, squad, topology, production permission,
  external effect, owner intervention, Browser action, or Phone Farm operation
  occurred.

Next: preserve v27 as valid capability evidence and design the next planned
testing group without selecting production roles.

### Current Item 14 checkpoint: Claude R03 affected reliability v28 complete

- One provider-free planning audit found the only missing repeat datum: Claude
  Sonnet 5 Medium's prior R03 repeat was invalid solely because of the now-fixed
  declaration parser. R01 and R04 already had two valid passes each.
- One independent prelaunch review passed the minimal R03-only matrix. The
  unchanged OpenRouter/Anthropic/Medium material configuration then passed R03
  v2 in 12 calls / 49,202 tokens / $0.141414 / 54.953s provider event time.
- Zero retries, owner interventions, Browser actions, external effects, role
  selections, or Phone Farm operations occurred.
- The isolated receipt/account delta discrepancy of $0.049273 is preserved.
  The Claude Code subscription was not substituted because that would be a
  different, uncalibrated transport configuration.
- Canonical record: `ITEM-14-REALISTIC-V28-RESULT.md`.

Next: use the completed static and realistic evidence to prepare the next
bounded testing group. Do not select production roles yet.

### Current Item 14 checkpoint: Claude subscription v29 invalid; cross-batch analysis complete

- One independently reviewed subscription-safe plan removed `--bare`, which
  current Claude CLI help proves disables signed-in OAuth/keychain auth.
- The only permitted launch attempt stopped locally in 284 ms because the
  Windows command wrapper rejected the prompt-bearing argument vector as too
  long. Exact adjudication: one process attempt, zero provider calls, zero
  tokens, zero cost, zero retries, and no candidate score.
- Raw evidence is immutable. The future executor now sends the prompt through
  stdin; focused tests pass, but v29 was not retried and the subscription lane
  remains held.
- Cross-batch analysis confirms that cheap static success has not survived the
  realistic gate, high effort has not rescued tested failures, and reviewed
  topologies have added overhead without repeatable rescue. Claude Sonnet 5
  Medium through OpenRouter remains only the strongest synthetic reference,
  not a selected role or production configuration.

Canonical record: `ITEM-14-CROSS-BATCH-CHECKPOINT-V29.md`.

Next: pause provider calls. Define the smallest new realistic case group from
the explicit remaining coverage gaps; do not search more long-tail models or
select roles yet.

### Current Item 14 checkpoint: R05/R06 capability v30 halted validly

- One independent review found four launch defects; one consolidated
  correction removed executable candidate code, required harness-owned R05
  receipts, added crash-safe no-duplicate claims, and enforced price-derived
  spend admission. Focused tests and vet passed before atomic freeze.
- Claude Sonnet 5 Medium exceeded the frozen R05 observation reservation:
  20 calls / 92,508 tokens / $0.321781. It is invalid and unscored, with no
  retry.
- The frozen global budget-stop rule halted the campaign. GPT-5.4 R05 and both
  R06 bindings remain unobserved; no conclusion is assigned to them.
- No role, squad, topology, permission, production, Browser, outbound, owner,
  or Phone Farm action occurred.

Next: preserve v30 exactly. In the next separately frozen matrix, distinguish
candidate-local observation-reservation exhaustion from true campaign-cap
breach, then run only still-unobserved bindings; do not retry Claude v30 or
select roles.

### Current Item 14 checkpoint: R05 residual v31 complete

- V31 preserved v30 and corrected future halt semantics: candidate-local
  competence/protocol/resource failures do not halt unrelated candidates;
  infrastructure, evidence, credential, and true campaign-cap failures do.
- GPT-5.4 Medium validly failed R05 after 20 read-only actions, 64,706 tokens,
  $0.211099, and zero retries. It made no edits or check calls and reached the
  response/tool protocol ceiling.
- R06 was skipped by the frozen advancement rule. Replay validation made no
  additional calls and preserved result SHA
  `e0899c9a7ddc360b8eed3071b7858a663d083b0f08e2035ea5f40dcc8fcdc2e5`.
- No role, squad, topology, permission, production, Browser, outbound, owner,
  or Phone Farm action occurred.

Next: use the preserved v30/v31 evidence to isolate model competence from
action-protocol effects before another paid batch. Do not retry either result
or select roles.

### Current Item 14 checkpoint: response-protocol ablation v32 invalid

- The frozen 2x2 matrix compared Claude Sonnet 5 Medium and GPT-5.4 Medium on
  identical R05 facts using a one-action loop and a one-shot conditional plan.
- A post-batch audit proved the schema was hash-bound metadata but was absent
  from the executable prompt and was never sent through an API schema field.
  OpenCode CLI `--format json` controlled event formatting only. Claude loop
  also has an observation-scoped route-identity parser defect. All four cells
  are invalid/unscored and their receipts remain unchanged.
- Exact batch total: 4 calls / 22,622 tokens / $0.142644 / 63.105s summed
  provider-session time / zero retries. Replay made zero calls and preserved
  ledger SHA `986291baef291c0279d79e697c635ca7bafd806f30704c8d7a8df5ff04c4ab8f`.
- The outputs show relevant semantic understanding but establish no model or
  protocol result because the declared response contract was not delivered.
- No role, squad, topology, permission, production, Browser, outbound, owner,
  or Phone Farm action occurred.

Next: build and test a separately versioned OpenCode server/SDK structured-
output transport with retry count zero and exact schema-delivery proof. Do not
spend again or select roles until it is independently validated and frozen.

### Current Item 14 checkpoint: structured-output protocol ablation v33 complete

- The corrected OpenCode loopback transport delivered the exact frozen schema
  with zero retries and closed all four counterbalanced R05 cells.
- Claude loop failed `StructuredOutputError` in one billed turn: 4,763 input,
  808 output, 465 reasoning, $0.024637, 12.874s. Claude one-shot is
  invalid/unscored after a ~14.376s client-aborted transport timeout with zero
  recorded usage/cost and provider settlement uncertainty.
- GPT loop failed `StructuredOutputError`: one turn, 2,776 input, 1,066 output,
  2,070 reasoning, $0.053980, 24.265s. GPT one-shot failed the same contract:
  one turn, 2,781 input, 1,430 output, 1,996 reasoning, $0.058343, 25.654s.
- Recorded total: 3 billed turns, 10,320 input, 3,304 output, 4,531 reasoning,
  and $0.136960. Freeze is
  `c99506b60f225de2f2b91563698657226100aa8252ba0c8633d60d0711cf155f`;
  final ledger is
  `e1f336da03c2c62419911fde6196887b6278d828471bd39b5791aefcd790b835`;
  `global_halt=false`.
- The generated text looked substantively useful but violated the exact
  schema, so it cannot be scored as a pass. No candidate or protocol advances,
  and no role or production configuration is selected.

Next: R06 remains skipped because the existing frozen rule requires a valid
R05 pass. Scope the next smallest bounded realistic testing group from the
remaining documented capability gaps; do not search more long-tail models or
select roles.

### Current Item 14 checkpoint: Claude Code subscription stdin calibration passed

- The original v29 oversized-command failure remains preserved as invalid
  transport evidence. Its tested stdin correction was exercised once in a new
  affected-only evidence root.
- Claude Code 2.1.270, `claude-sonnet-4-6` Medium, passed frozen T11 v4 in one
  provider invocation, zero retries, 6,100 input tokens, 980 output tokens,
  and 14.719s.
- The provider-reported 51,295-micro cost equivalent is subscription billing
  metadata, not cash spend. Raw result SHA-256 is
  `6b9400c70c380f14996183f973f2368e5b52a0ac800c6acb4929489c7e04b8c5`;
  parsed output SHA-256 is
  `e7777d36df33c390f68daa8a8b1a6281d559789ce0ff3b1651681be960d13d0c`.
- This proves one valid static transport observation only. It selects no model,
  role, squad, topology, permission, or production configuration.

Next: continue the existing testing plan by scoping the next smallest bounded
realistic group from the documented capability gaps; do not infer reliability
from this single static pass or select roles.

### Current Item 14 checkpoint: Claude subscription static breadth v34 passed

- `claude-sonnet-4-6` Medium passed all six frozen static cases: T12 v4, T11
  v4, T08 v3, T16 v3, T04 v1, and corrected T09 v2.
- Exact totals: 6 invocations, 35,643 input, 4,082 output, 253,410
  provider-reported subscription cost-equivalent micros, and zero retries.
- Freeze SHA is
  `424041cd761a56b72db4bb92c48f7518901f0975f608d37e3cb687ba0a92f9a3`;
  ledger SHA is
  `21c3089b90b841513d3107477df339fe874aec9c93d71e3d8fbe37aac8207163`;
  `candidate_eliminated=false`; `global_halt=false`.
- The original replay reported a nil-versus-empty omitted-slice false
  positive. The separate grade replay verifier validated all six grades with
  zero provider calls; focused tests pass. Frozen runner/evidence are intact.
- No model, role, squad, topology, permission, or production configuration is
  selected.

Next: separately freeze and run the planned static reliability repeat. Do not
treat this breadth result as production promotion.

### Current Item 14 checkpoint: Claude subscription static reliability v35 passed

- Repeated the exact frozen v34 six-case order—T12 v4, T11 v4, T08 v3, T16 v3,
  T04 v1, and T09 v2—and passed all six again.
- Exact totals: 6 calls, 35,645 input, 4,721 output, 156,403
  provider-reported subscription cost-equivalent micros, zero retries,
  `candidate_eliminated=false`, and `global_halt=false`.
- Freeze SHA is
  `b37d6246a1a3f998e5373f7baadaace2fcc8408a8b040a40074b8481a118b4ee`;
  ledger SHA is
  `8ed67c98c885cb9468c8d9d7146a8d8395401ec7c567fea10ba0470d2304b439`;
  evidence is `.qualification-evidence/claude-subscription-static-v35`.
- Provider-free replay passed with zero calls. All six parsed response hashes
  exactly match v34; raw receipts and usage differ between the real runs.
- This is perfect static repeat consistency, not proof of realistic action
  competence. No model, role, squad, topology, permission, or production
  configuration is selected.

Next: evaluate the remaining evidence gaps and justify a separately frozen realistic
Claude subscription test before spending or promoting anything.

### Current Item 14 checkpoint: Claude subscription realistic v36 failed its R03 cap

- Frozen order was direct R03 v2 then R04 v1. R03 ended
  `observation_resource_exhaustion` after 10 provider turns, 57,508 input,
  2,371 output, zero reported reasoning, 206,700 subscription cost-equivalent
  micros, and zero retries.
- The controller made four reads, four scoped edits, ran tests, and checked
  rollback. It did not reach verify or finish. Its native deterministic failure
  was `response_or_tool_protocol_failure`; the terminal turn also crossed the
  frozen 200,000-micro efficiency ceiling.
- R04 was policy-skipped with zero calls. `candidate_eliminated=true` and
  `global_halt=false`.
- Freeze SHA:
  `86ef9a33a047ede5f004ef73f35d76c6d8ef3400f32b8a89a4a2f025867afe6b`;
  definition SHA:
  `572a649ad9e65d9ab3106aeeaf941d8e373322aba7befed686b6861ebf66eb44`;
  ledger SHA:
  `5050160aed2233d80447e6fbf34e43b559b1c1f863d02ef45c8cd492b1b31a24`.
- Provider-free replay validates both claims with zero calls and exact usage.
  Subscription cost metadata is not marginal cash spend.
- The result eliminates only this exact configuration under this frozen cap;
  it does not eliminate Claude globally or select a role, topology, or
  production configuration.

### Current Item 14 checkpoint: Claude subscription phased R03 v37 passed twice

- The separately frozen phased controller preserved the exact R03 v2 case and
  `claude-sonnet-4-6` Medium subscription configuration.
- Run 1 passed with 3 calls, 16,465 input, 2,153 output, 92,159
  cost-equivalent micros, and zero retries.
- A fresh exact reliability repeat passed with 4 calls, 22,185 input, 2,627
  output, 94,675 micros, and zero retries.
- Evidence roots are
  `.qualification-evidence/claude-subscription-realistic-v37` and
  `.qualification-evidence/claude-subscription-realistic-v37-repeat-01`.
- The narrow v36 baseline failed its resource cap after 10 calls, 57,508 input,
  2,371 output, and 206,700 micros. V37 closes only the R03 phased-controller
  comparison; it makes no role, model, or production selection.

Canonical result:
`ITEM-14-CLAUDE-SUBSCRIPTION-REALISTIC-V37-RESULT.md`.

### Current Item 14 checkpoint: Claude subscription R04 v38 failed validly

- The phased `claude-sonnet-4-6` Medium subscription candidate completed the
  required R04 workflow work by phase 2 but failed to finish.
- Later phases repeatedly reread stale evidence. In phase 5 it reran
  `python resume.py`; the controller rejected the duplicate continuation with
  `resume action journal identity mismatch`.
- Valid candidate-local result: `response_or_tool_protocol_failure`, 5 calls,
  26,129 input, 3,098 output, 105,944 cost-equivalent micros, zero retries, and
  zero ambiguity.
- `candidate_eliminated=true`; `global_halt=false`. No role, model, or
  production configuration was selected.

Canonical result:
`ITEM-14-CLAUDE-SUBSCRIPTION-REALISTIC-V38-RESULT.md`.
