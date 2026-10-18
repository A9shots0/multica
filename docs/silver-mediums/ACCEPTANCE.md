# Owner acceptance walkthrough

These are business tests, not implementation claims. Each row must carry fresh
evidence before it can pass.

The owner accepted the intended combined Board, Issue, Inbox, Chat, and phone
experience on 2026-09-10. Individual implementation rows remain unproven until
their named local or real-work evidence exists.

| Scenario | What Drew should experience | State |
| --- | --- | --- |
| Report a problem | Tell Mika once in plain language; no routing choice required | Not tested |
| Evidence | Mika checks relevant sources before stating cause or conclusion | Not tested |
| Issue ownership | One issue is owned by a squad with visible internal roles | Pilot passed on SILV-3 and SILV-5; real issue not tested |
| Automatic execution route | A configured squad installs worker/reviewer/retry rules before Mika's first issue run; Drew or Mika never has to create the route per issue | Pilot passed on SILV-11; real issue not tested |
| Progress | Board status changes only when real work state changes | Pilot passed on SILV-3 and SILV-5; real issue not tested |
| Review | A different agent reviews implementation; Drew is not code reviewer | Pilot passed on SILV-3, SILV-5, SILV-6, and SILV-9; real issue not tested |
| Review correction | A failed review sends one bounded correction directly to the worker, then re-reviews without Mika relaying | Pilot passed on SILV-9; real issue not tested |
| Retry ceiling | Repeated review failure stops once, explains why, and cannot loop forever | Pilot passed on SILV-10; real issue not tested |
| Urgent repair | Approved employee-blocking repair releases and verifies without a second deploy command | Not tested |
| Durable bounded authority | Drew authorizes the intended outcome, target, audience, ordinary actions, safety invariants, exclusions, recovery conditions, and risk/budget bounds once. Authority survives handoffs, corrections, rebuilds, retries, and restarts while those boundaries remain unchanged; fresh technical review still applies to every changed candidate. Material boundary changes require Drew again before the consequential action | Policy ratified 2026-09-10; execution not tested. Main Multica repeatedly asked again after prior authorization on 2026-09-09 |
| Investigation versus repair | An investigation cannot close Done while accepted repair work remains unbuilt | Not tested; main Multica closed a Phone Farm investigation without implementation on 2026-09-09 |
| Regression | Rollback/failure immediately produces a clear owner alert | Not tested |
| Release regression protection | Every Phone Farm release rechecks the permanent employee-critical behaviors fixed by earlier releases, and cannot pass when its code, bundled assets, or build configuration silently revives a closed bug | Not tested; owner reported a later Phone Farm build reintroduced an earlier access/UI failure on 2026-09-09 |
| Recoverable blocker | A repairable runtime, skill, or routing failure automatically repairs/retries or escalates; diagnosis alone cannot remain Blocked | Not tested |
| Needs Drew | Only a concrete business decision enters the owner-decision state; answering resumes the exact suspended worker without Mika relaying | Pilot passed on SILV-13; real issue not tested |
| Inbox | One decision card shows the question, recommendation, consequence, and bounded choices; it remains visible above newer issue noise and archives after the answer | Item 10 local browser and SILV-13 pilot passed; real issue not tested |
| Useful update | Each owner update says what changed since the previous update, the verified current state, and the automatic next action | Item 10 local projection/presentation passed; real issue not tested |
| Phone decision | A time-critical alert opens the current secure mobile decision; `ACK` only acknowledges, and a consequential choice resumes exactly the bound work once | Item 11 secure compact control screen and command wiring passed locally; real phone, delivery, and resume not tested |
| Phone completion | Every independently verified completed business outcome sends Drew exactly one SMS; an agent, build, review, deployment, or intermediate recovery finishing does not qualify | Item 12 deterministic policy, atomic admission, durable dedupe, dry-run, and one real provider-delivered live-test passed; real business completion not yet tested |
| Phone recipient and timing | Eligible notices have no system quiet hours and go only to Drew; the system never invents a backup recipient or substitute approver | Item 12 explicit owner binding and one real live-test passed; pilot returned to dry-run |
| Typed owner response | Approve, reject, revise, comment, partial acceptance, cancel, scope change, and acknowledgment are distinct actions; ambiguous prose cannot silently authorize work | Items 10-11 local typed controls passed; `correct` receipt passed on SILV-911; other live paths not tested |
| UI feedback | Drew can inspect the relevant candidate at useful device/data states, annotate or comment, request a revision, and see the replacement candidate without losing prior context | Items 10-11 bounded desktop/compact evidence and comment controls passed locally; annotation/replacement-candidate history remains later proof |
| Stale decision | A stale or superseded link refuses mutation, opens the current decision, and cannot resume the wrong candidate or generation | Not tested |
| Concurrent work | One issue's retry, cancellation, correction, notification, or release cannot mutate or suppress another issue | Not tested |
| Cancellation and scope change | Stop/cancel prevents further consequential action; changed scope creates a new authority revision rather than inheriting old approval | Not tested |
| Capacity and quota | The controller uses only compatible bounded fallback, preserves the checkpoint, and escalates a technical stop without asking Drew to route agents | Not tested |
| Platform outage | A detector outside the failed Multica path alerts Drew once, preserves receipts, and reconciles without duplicate issues or messages after recovery | Not tested |
| Notification truth | Detection, admission, provider acceptance, delivery, acknowledgment, and resolution remain separate; ambiguous provider results reconcile before retry | Item 12 migration/worker/provider tests, local database lifecycle, and one real provider-delivered live-test passed; webhook and outage-monitor proof remain later work |
| Release stall | A long-running release has a controller-owned progress signal, stall threshold, and automatic recovery/escalation | Not tested |
| Messaging outcome | Owner sees sent/not sent and reply-path proven/not proven without polling a chat | Not tested |
| Cost | Model, retry count, and token use are bounded and understandable | Failed: SILV-13 used the intended four runs but still consumed 123,767 fresh input plus 529,408 cached input tokens |
| Recovery | Interrupted work resumes once without duplication or lost direction | Not tested |

Final acceptance requires one real but noncritical issue to pass the entire
workflow and Drew to confirm that the experience is understandable and useful.

## Item 10 acceptance boundary

- Inbox contains only genuine owner decisions, important incidents or
  consequences, meaningful state changes, and independently verified
  completions. Raw agent starts, runs, retries, heartbeats, and ordinary
  comments are not owner attention.
- Each independently actionable record remains selectable and archivable even
  when several records belong to one issue. Selection uses the attention item,
  not the issue id.
- Category totals are usable filters rather than decorative counters. The
  desktop list preserves enough width for two-line titles and meaningful
  details, and rows do not spend their primary space on decorative avatars or
  duplicate status glyphs.
- The bounded detail shows what happened, current state, recommendation and
  consequence when applicable, latest meaningful progress, a comment or exact
  correction field, an evidence/full-issue link, and acknowledgment without
  embedding the full issue timeline.
- Authorization decisions expose only the two outcomes the accepted v2 command
  path can execute atomically: approve and reject. Comment, correction, request
  revisions, partial acceptance, scope change, and cancellation are separate
  issue-bound controls, so none can be mistaken for approval. Cancellation also
  moves the issue to Cancelled. Legacy requests without a v2 receipt fail
  closed instead of presenting false approval controls.
- Chat explicitly directs existing-work decisions, corrections, evidence, and
  progress to Inbox. Its runtime contract permits brainstorming, new goals,
  serious or unclear problems, cross-issue questions, and explanations; a
  concrete request binds exactly one issue and Chat does not poll or relay the
  workflow.
- Required proof before acceptance: focused Core/Views/Go tests, TypeScript and
  production build, complete built-in-browser walkthrough of Inbox and Chat,
  one consolidated correction pass, one final independent review, and a dated
  run-journal receipt. Local fixtures prove presentation only.

## Item 11 acceptance boundary

- Phone-sized Inbox uses the same owner-relevant categories, counts, item ids,
  archive semantics, authority meanings, and evidence boundaries as desktop.
- A tap opens one bounded full-screen control surface. It does not dump the
  issue timeline or collapse multiple material records from the same issue.
- Exact authorization remains Approve/Reject only. Comments, corrections,
  revisions, partial acceptance, scope changes, cancellation, acknowledgment,
  evidence, and full-issue inspection remain distinct actions.
- Missing or stale decision/issue evidence fails closed and visibly disables
  consequential controls. Mutation failures state that nothing changed.
- Issue opens with Current Picture before activity; Chat visibly directs
  existing-work control to Inbox.
- Local compact Browser, mobile tests/typecheck/lint, Expo web export, private
  HTTPS host readback, one correction pass, and one independent review are the
  acceptance gates. A signed native install and real-phone HTTPS login remain
  separate proof.

## Item 12 acceptance boundary

- Deterministic admission uses only exact authoritative Current Picture
  references and occurs in the same transaction as the projection.
- Durable mode-scoped dedupe, leased claims, attempt receipts, bounded retries,
  ambiguous-send fail-closed behavior, and bounded provider reconciliation are
  required before any real send.
- Only the explicitly bound owner receives eligible events. The destination is
  environment-only; live modes require an explicit verified-destination gate;
  administrators cannot substitute for the owner; Inbox and API receipts
  expose no number or provider id.
- `dry_run`, `live_test`, and `live` are separate namespaces and gates. Passing
  a live-test does not activate live notifications.
- The accepted proof is one dry-run lifecycle, dedupe/namespace/recovery checks,
  one provider-confirmed labeled SMS, and the built-in-browser receipt. Real
  decision/resume, real business completion, and external outage monitoring
  remain later end-to-end scenario gates.
- The one final independent review returned NO-GO with P0=0/P1=3 plus one
  important P2. One consolidated pass corrected migration compliance,
  revision-stable semantic dedupe, duplicate incident suppression, owner-only
  verified binding, and worker transition tests; focused verification passed
  afterward without a second reviewer loop.

## Item 14 acceptance boundary

- The accepted local foundation is the bounded layered architecture: one
  accountable business-outcome role, a separately selected repository
  executor, only graph-required contributors, independent release/review, and
  controller-owned transitions. A role is not automatically a permanent agent.
- The canonical graph and its queries must preserve maintained system direction,
  authority, freshness, technical/host boundaries, related outcomes, permanent
  regressions, and bounded blast radius across all 19 locked cases.
- Consequential execution fails closed without exact capability, independent
  architecture-assurance, and durable owner-authorization receipts. Exact
  existing owner authority is reusable without another permission request;
  another candidate or release path requires different authority.
- Configuration qualification applies to the complete versioned combination of
  role, model, effort, instructions, skills, permissions, runtime, and evidence
  scope. Applicable cases come from the locked bank, never from the candidate.
- Final proof: four focused Go package suites passed, the rebuilt authenticated
  local API returned the accepted graph/role/configuration catalogs, Phone Farm
  remained Mac-only, and the terminal independent review returned
  `APPROVE ITEM 14 LOCAL FOUNDATION`.
- This acceptance does not label any configuration `qualified_bounded`, grant
  production authority, accept the duplicate-root-run cost baseline, alter the
  hosted roster, or operate Phone Farm from Windows.
