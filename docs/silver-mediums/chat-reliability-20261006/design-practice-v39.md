# Shared product-design practice

Owner decision, 2026-10-06 EDT: every UI designer must operate as an accountable
specialist commissioned by the client, using research, staged artifacts,
feedback and evidence. The client approved research/page proposal -> composition
wireframes -> visual design -> clickable prototype -> implementation/validation.
The Phone Farm constraints include VAs managing several phones and filling close
to the available 24-hour schedule; visual posts need recognizable media. These
are research/job requirements, not an approved composition or posting quota.

## Scope and implementation

- `multica-design` is a universal embedded skill. Existing and future ordinary
  builders, specialists, Mika and reviewers receive it through inline skills
  and skill-bundle claims. No agent-name classifier, model preference, agent-row
  rewrite or new schema is needed.
- Every ordinary claim gets a short mandatory conditional design-practice
  instruction alongside its saved instructions. Non-UI work has no design
  checkpoints; isolated corrections within an accepted design use a focused
  path. Exact sealed legacy/guarded role artifacts remain unchanged.
- Mika routes new/substantial UI work through the staged client-review process
  on the original issue. Checkpoints include artifacts, a recommendation and a
  focused question. Explicit client direction is required before advancement;
  general authorization, silence and reviewer agreement do not select a design.
- A bound Product & UI task can return a validated `<design-checkpoint>` for
  research, wireframes, visual design or prototype while awaiting client
  feedback. It cannot claim implementation readiness, mix a final ship receipt,
  reference another bound pack or repeat one artifact as two wireframes.
  The original full delivery and distinct reviewer-provenance gates remain for
  final implementation delivery. Machine receipts are removed from concise
  Chat summaries while retained in the task result.
- The shared skill includes the intermediate receipt contract so installed
  daemons receive it without a worker binary replacement. The daemon source
  prompt also clarifies intermediate versus final delivery for future builds.

## Evidence boundaries

This is a behavior contract and intermediate-result validation, not a new
server-enforced phase/approval state machine. The server validates declared
checkpoint data; it does not inspect whether code was built prematurely, prove
two artifacts are good alternatives, or infer that an earlier human message
approved a phase. Same-issue continuation and explicit client feedback remain
essential. Do not claim all agents always comply from tests or prompt delivery.

No new product composition is approved. Existing CRM and Phone candidates
remain rejected, acceptance 0/2. Employee usability testing has not occurred.
Methods and model agreement cannot guarantee design quality.

## Verification

Local checks passed: complete `internal/designgate` suite; selected universal
skill/template/YAML/source-boundary, practice layering and Chat-summary tests;
bound/ordinary daemon prompt checks; `go vet` for designgate/service/handler.

Owned isolated PostgreSQL fixture, handler tests PASS (15.824 seconds): ordinary
specialist claim preserves workspace notes and receives shared practice;
invalid checkpoint refuses without changing a running task; valid research
checkpoint finishes only that run, leaves issue in progress and creates zero
successors. Existing bound-pack/final-review and unconfigured-pack refusal
tests also pass, plus Mika composition/name tests. Log:
`C:/Dev/_artifacts/multica-chat-reliability-20261006/design-practice-handler-tests.log`.

Deployment and actual designer behavior must be recorded separately below.

## Active runtime and first ordinary Chat observation - 2026-10-07 02:14 UTC

Source `7512b6b18bf109c892fca616afb07cda2739ce58` is pushed and active.
Backend image `sha256:209cc7c673ba98387bdec248af727e1829da1ee64514a4838e96056f3d0fcf1f`,
binary SHA-256 `9050e25a1ca71bc85fb6f219f748447d18e8c00edca30e90be68d22043404619`.
Health readback reports that exact source, started `2026-10-07T02:11:54Z`.
Fresh backup `20261007T020937Z-537fb463` verified local/offbox/private R2.
Preflight and normal application live checks passed; frontend/database container
IDs and installed worker binaries remained unchanged. Host/backup LaunchAgents
point at v39. Reboot recovery remains untested. Rollback retains v38 and its
protected pre-v39 environment snapshot; no schema change in this release.
Activation log: `C:/Dev/_artifacts/multica-chat-reliability-20261006/v39-design-practice-activation.log`.

An ordinary request synthesized from the owner's latest Phone Farm needs was
sent through the existing SILV-94 Chat, without technical test instructions or
an explicit list of process stages. Mika task
`01a11422-409e-7064-9192-9504a2c43c74` read the shipped `multica-design` skill
(task_message seq2), chose research/page proposal before implementation, and
canonically continued the same issue. Specialist task
`01a11422-de40-7a23-94ec-b566278abef3` started at `02:13:11.874238Z`.
The visible Chat reply says the next handoff is a proposed product direction
for client feedback before wireframes. Screenshot: `design-practice-first-chat.jpg`.
At 02:16 UTC the Mac Studio specialist also read the shipped skill (seq4) and
stated it would inspect real app support and return a page proposal before
another implementation. The run remained active; this is not a finished proposal.

This proves initial policy delivery and route selection in one ordinary
continuation. It does not prove the proposal's quality, completed stage handoff,
subsequent client-feedback loop, employee usability or all-agent compliance.
No new composition or implementation has been accepted. The prior technical
negative admission probe remains inconclusive; v39 does not rewrite that result.

## Method references checked 2026-10-06 EDT

- [Design Council Double Diamond](https://www.designcouncil.org.uk/resources/the-double-diamond/):
  discover/define the problem, explore alternatives, test/refine solutions.
- [GOV.UK user needs](https://www.gov.uk/service-manual/user-research/start-by-learning-user-needs):
  current tasks, existing evidence, actual users and continued research.
- [GOV.UK usability testing](https://www.gov.uk/service-manual/user-research/using-moderated-usability-testing):
  representative participants and realistic tasks; agent walkthrough is not
  employee research.
- [W3C WCAG](https://www.w3.org/WAI/standards-guidelines/wcag/): applicable
  accessibility requirements, with explicit verification limits.
