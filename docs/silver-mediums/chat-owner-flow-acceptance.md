# Chat owner workflow acceptance

2026-09-25 UTC. Authorized local disposable testing while the owner is busy.
Existing isolated review workspace only; no production actions or deployment.

Starting code: 77431292149dc57e3f50220770da81f7324d7f19. API 3220 reports
that revision and PID 39368. Web 3221, review database 55438; automated tests
must use separate database 55440. Owner web edits remain untouched.

## Scenarios

- [x] Create and link two disposable issues through one Mika conversation.
- [x] Independent progress, one question and one finished result arrive unaided.
- [x] Discuss a pending choice without answering it.
- [x] Targeted reply retains visible issue context and changes only that issue.
- [x] Request a revision, then accept exactly the finished result in Chat.
- [x] Inbox reflects the same pending question and retires the resolved item.
- [x] Draft, context and work state survive reload and a connection interruption.
- [x] Phone layout has bounded work details and a usable composer.

## Findings and evidence

Shared brain source guidance refresh remains pending; live source and this local
trial, not the guide, establish current behavior.

Initial conversation `01a0d661-e5c8-74a8-809c-b461300d57f3` failed at routing.
Mika read agent definitions but did not inspect runtimes, then incorrectly
reported that no online worker/model existed. Fresh runtime query showed its
own runtime online. Empty model means provider default; idle/working are activity
states. No issues were created, so this failure is retained without cleanup.
Transcript: `C:/Dev/_work/mika-chat-prototype-20260924/owner-flow-initial-transcript.jsonl`.

Root cause: chat guidance failed to require runtime evidence before an
availability refusal. Added the distinction and explicit runtime discovery to
the per-turn chat prompt and runtime reference. Existing canonical chat-prompt
regression now locks these semantics; focused daemon tests pass. Commit
`9843f4a71` was verified with the refreshed dedicated worker and replay below.

Mika also correctly reported that its CLI cannot rename a conversation. This is
a capability limitation, not a failed rename receipt. The UI can rename it;
do not claim Mika has a rename tool it lacks.

Current suites: all 322 Views Chat tests pass. Focused handler/service Chat
regressions pass; exact counts retained in the external owner-flow logs.

After worker `9843f4a71` restart, the unchanged-scope retry inspected runtimes,
created and dispatched exactly two linked issues: welcome note
`01a0d668-64a1-7df1-8990-0879db0fa3e6`, checklist
`01a0d668-bce0-7ecf-ad87-3c9b55d3e556`. Initial runs
`01a0d668-a2d2-73fe-9592-612d6f098cce` and
`01a0d668-cec4-7ded-be11-f9fc5670a22f` completed independently (single-slot
runtime executes sequentially). Welcome created pending Inbox question
`1b53fd33-926a-53fc-ad77-fc6f2342d88c`; checklist returned without accepting work.

Test tab alone went offline for roughly 30 seconds while both workers finished.
After reconnect/reload the exact unsent draft survived and the canonical question
and automatic reports appeared. A topic-targeted discussion produced a visible
Re: label, explained choices, and left question pending, both issues backlog,
and one worker run per issue. Inbox visibly showed the same welcome question.

Second finding: checklist worker's final result said only it had posted the
checklist in the issue thread. Tool-free automatic Mika report could not present
the actual checklist, although an interactive work read later obtained it.
Worker `68fcb1b61` now instructs ordinary issue runs to include the actual concise
deliverable in their saved final result without accepting work. Guarded/workflow
and chat tasks are excluded; regression tests pass. Revised-result acceptance
is still in progress. This is a guidance correction; no source results rewritten.

Third finding: the draft text survived reload but its selected question did not.
Commit `41707249b` persists the captured issue/question/revision beside the draft,
scoped to workspace and conversation, without caching live work status. Both
full Chat and floating Chat use the same selection and send target. Fresh browser
reload retained the exact Warm answer and question. Submission produced one
continuation (`01a0d674-1e11-7a37-901d-edbcff02b6d4`), retired the exact Inbox
question, and did not rerun or accept the checklist. Floating Chat in Inbox
retained the selected checklist topic; returning to full Chat retained it too.

Checklist revision `01a0d675-aae3-73c9-b045-9a3578f875e5` returned three concrete
pass/fail checks including keyboard accessibility. The saved worker result now
contained the actual deliverable, proving the worker handoff correction. The
automatic report still paraphrased it as posted elsewhere. Follow-up commit
`dfde0b98f` requires short deliverables in the report and avoids repeating unchanged
results from other issues; report remains tool-free and never accepts work.
Dedicated idle worker refreshed to that exact revision before the next trial.

The structured Compare options selection created a removable chip, inserted no
prompt text, was consumed on send, and remained visible on the saved user message.
Mika returned a three-column comparison table and qualified recommendation for
async review versus a 15-minute meeting. Table width 362 CSS px in the measured
392x852 CSS phone viewport; document width 392, no horizontal page overflow.

The first warm revision violated the two-sentence requirement (three sentences),
which was visible because the full text now arrived in Chat. A precise correction
through the same targeted request_changes flow produced the exact requested two
sentences in worker `01a0d67e-c379-7b25-bdf8-2596afedf496` and automatic report.
This is evidence that review/correction works, not that model output is infallible.

Welcome acceptance then marked only that issue Done, preserving checklist backlog
and its two worker runs. Welcome had four runs (initial question, answer, two
explicit revisions); acceptance started no fifth run. Reply topic was refreshed
through the picker before acceptance to capture the latest result identity.

Fourth finding: independent checklist acceptance was refused because the worker
left the issue in backlog, so the server intentionally withheld result identity.
No acceptance receipt or status change occurred. `4bfc2a26d` makes ordinary
finished-work handoff explicitly set in_review without starting a run, unless
owner instructions prohibit that change. Mika is also told not to attempt
accept_result without an eligible captured result. The existing strict server
acceptance gate is unchanged. Dedicated worker replay follows below.

Replay worker `01a0d684-b75b-777a-b509-ce102db87d30` returned a concise checklist
and moved it to in_review with no acceptance. Fresh topic selection followed by
explicit acceptance succeeded in Chat task `01a0d68a-0664-7aaf-b208-90fad00eb230`.
Final readback: welcome Done / four worker runs / one applied acceptance;
checklist Done / three worker runs / one applied acceptance. Fourteen stored
work updates have fourteen distinct source/kind pairs; reconnect did not duplicate
these updates. The two unrelated older Inbox requests were left untouched.

Plan mode returned a useful 30-minute preparation plan without creating issues.
It revealed a fifth defect: adjacent commentary and final answer were coalesced
without a paragraph break, so the final heading displayed as inline `##` text.
The persisted answer was correct. `5c08c3b18` uses the authoritative settled answer
to restore that known boundary without guessing boundaries within streamed token
fragments. Fifty transcript/copy/message tests, Views typecheck and focused lint
pass. This repairs rendering of existing stored replies as well as new settled
ones; source messages are not rewritten. During the UI hot refresh a send was
slow; readback established its eventual single successful task before any retry,
and no duplicate acceptance was submitted.

Checklist format returned four relevant mobile-review checks and distinguished
proposed checks from work actually completed. Selection chips clear on successful
send and remain as labels on their respective user messages.

Final Browser readback confirmed the older plan now has a distinct level-2
heading (the intro remains a separate paragraph). The diagram selection produced
a rendered Mermaid flow with Yes/No revision loop, copy-source and viewer controls,
plus a plain-language explanation. Phone document width remained 392 CSS px.
All four selections were exercised through real worker replies in this rehearsal.
No additional issues were created by discussion-only format tests.

## Verification and limits

- All 322 Views Chat tests passed before the new corrections; 34 focused Core
  tests, 13 focused topic/view tests, and 50 final transcript/copy/message tests
  passed for the affected changes. Focused daemon tests and stamped CLI builds
  passed after each prompt correction. Handler/service baseline had 86 passing
  test/subtest events in the isolated test database, not the review database.
- Views typecheck and focused Core/Views lint passed. Existing Core-wide Inbox
  fixture type mismatch remains as documented in chat-response-format-progress;
  this run does not claim a clean whole-repository typecheck.
- Logs and final receipts: `C:/Dev/_work/mika-chat-prototype-20260924/owner-flow-*`.
- Final local API remains `77431292149dc57e3f50220770da81f7324d7f19`; dedicated
  worker is `4bfc2a26d501b334bd9e8321d5f488158f40420e`; web renders this worktree
  including `5c08c3b18`. Different revisions are intentional: this turn changed
  worker guidance and web/Core state, not API behavior. No migrations this turn.
- Local-only, no push/deploy or production data. One worker slot; independent
  multi-issue state was verified, not simultaneous high-load execution. No new
  file-upload/provider matrix or physical phone/keyboard certification.
- Mika can still omit details or overstate text quality; one sentence-count error
  was corrected through the tested revision flow. Prompt guidance is not a
  deterministic guarantee. Mika chat-renaming CLI capability remains absent;
  UI renaming works. Automatic reports sometimes repeat unchanged sibling status.
- Development hot refresh caused temporary waits; API stayed healthy and a delayed
  send completed once. This does not certify all historical crash causes solved.
- Temporary phone/network overrides reset; original user tabs preserved; rehearsal
  retained at the URL below with empty composer and General chat selected.

Preview: http://127.0.0.1:3221/mika-local-review/chat?session=01a0d661-e5c8-74a8-809c-b461300d57f3

Knowledge hub validation passed at close-out (existing stale-claim/snapshot
warnings remain). Shared-brain handoff delivered as task_observation
`abebc5a2-1fc0-42c8-824c-9a2cf298c904`, acceptance not established.

## Owner visual approval and inline diagram correction - 2026-09-25 04:00 UTC

The owner explicitly approved the current Chat visuals except the oversized
diagram. This is visual acceptance of the reviewed local Chat, not deployment
authorization or a claim that every operational limitation above is resolved.

The existing diagram measured 327 x 941 CSS px. Chat now caps the preview at
240px high in a bounded inline block with visible copy/expand controls. The lazy
shell applies the same cap to cached natural heights. Short diagrams can remain
short. The full viewer retains the natural graph, pan/zoom, source and export.
Sandboxed diagram documents now receive the host font and an explicit background;
the white canvas and inherited serif font are removed. Issue-page sizing is unchanged.

Verification: 70 diagram/viewer/export/lazy tests, Views typecheck, focused ESLint
and diff checks passed; the final caption/control adjustment also passed all 17
diagram tests. Built-in Browser readback confirmed a 240px preview, the app's
Inter/system font stack, rgb(17,17,20) background and empty iframe sandbox. At
392x852 CSS phone dimensions the document stayed 392px wide, preview width was
305px, and its containing inline block was 332px wide. Expand opened the Diagram
viewer; Close returned to the same conversation and composer with the preview
still 240px high. Regular viewport restored afterwards.

Local-only web change. Existing owner web config/instruction files preserved.
Evidence log: C:/Dev/_work/mika-chat-prototype-20260924/diagram-inline-investigation.md.
The owner has not yet reviewed this final diagram adjustment.
