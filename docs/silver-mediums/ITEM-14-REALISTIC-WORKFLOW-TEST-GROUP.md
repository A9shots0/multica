# Item 14 realistic workflow test group

Date: 2026-09-12  
Status: `R01_batch_complete`; no configuration advanced  
Machine definition: `server/internal/agentqualification/realistic_workflow_group_v1.json`

## Why this stage exists

The frozen static screen measured whether a model could reason about supplied
evidence and obey a typed answer contract. It did not show that a configuration
can inspect files, make a bounded change, run the right checks, use the Browser,
recover from interruption, or leave truthful documentation for the next clean
session. More lightweight static calls would not answer those questions.

This stage therefore uses four small synthetic workflows. They preserve the
shape of real Silver Mediums failures without touching a live repository,
production service, credential, customer, employee, message provider, or Phone
Farm. Every observation starts from a fresh disposable copy.

## The four cases

| Case | What the configuration must actually do | Why it matters |
|---|---|---|
| R01 cross-system authority repair | Trace canonical CRM authority through two synthetic consumers, fix only the faulty consumer, and prove the correct consumer and read-only Phone Farm contract fixture are unchanged | Separates confident diagnosis from competent tool use and bounded cross-system repair |
| R02 responsive weekly report | Establish the metric truth, repair the report and four-card layout, and prove narrow, medium, and wide local Browser states | Measures data correctness and real UI verification together |
| R03 release regression and rollback truth | Identify how a later build revived a bug, restore the simulated baseline, make the contract structurally safe, and prove both new and historical journeys | Measures release discipline without allowing a real deployment |
| R04 forced interruption and exact-once continuation | Seal an interrupted first run, resume the same run family from durable evidence, reject a superseded claim, avoid repeating completed work, and produce exactly one final output | Measures whether recovery and documentation genuinely replace chat archaeology |

## Non-negotiable execution boundary

- A new synthetic workspace is copied for every observation.
- The only network request allowed is the pinned model-provider call. R02's
  Browser can open only the fixture's local preview.
- Reads, searches, workspace-local edits, allowlisted checks, and the R02 local
  Browser are the entire tool surface.
- Git push, deployment, outbound messages, credentials, Phone Farm, and host
  files outside the disposable workspace are unavailable.
- There are no retries and no owner interventions.
- Raw commands, diffs, test results, browser evidence, and final receipts are
  preserved even when the candidate fails.
- A claimed result without the matching machine receipt is a hard failure.

## Scientific comparison

The unit being compared remains the complete configuration, not a model name.
Every viable configuration receives the same frozen fixture and clean starting
state. The grader checks the resulting workspace and receipts, not the wording
of the final response.

The first realistic pass will be narrow. It will compare the inexpensive
configurations with the strongest relevant static evidence against one capable
anchor. Repetitions happen only after a configuration produces one valid pass.
No observation selects a production role. The later system comparison will run
the same outcome once as a capable single-agent path and once through the
controller-led path, counting every run, handoff, token, retry, and owner
interruption.

## Required validation before spending

For each case, the fixture and grader must accept a strong solution and a
semantically equivalent solution, while rejecting subtle omissions,
adversarial instructions, fabricated evidence, scope expansion, missing
receipts, and invalid starting state. One independent reviewer then audits the
complete case group, runner, matrix, accounting, and halt behavior. We apply at
most one consolidated correction and request one final GO/NO-GO. Only a GO may
change the machine status to `frozen_reviewed` and unlock provider calls.

## Current boundary

The case identities, capabilities, required evidence, disqualifiers, execution
boundary, and review policy are implemented. All four defective starting
fixtures fail their hidden checks and all four offline reference solutions pass.
A separate realistic runner exposes only read, search, allowlisted edits,
allowlisted checks, and the R02 Browser boundary; arbitrary shell commands and
workspace escapes are rejected. The accepted static tool-free executor was not
weakened or overloaded.

The receipt layer recomputes initial/final trees, changed paths, evidence
artifacts, provider-receipt hashes, command allowlists, and required case
evidence. The campaign controller counts provider turns, tokens, and cost; a
candidate competence failure eliminates only that candidate, two unrelated
failures pause only the shared case for audit, and only test invalidity,
evidence failure, unsafe execution, transport failure, or budget breach halts
the campaign. The planned first stage is capped at $5 and 200,000 tokens with
zero retries and zero owner interventions.

One independent review correctly returned NO-GO before the consolidated
correction. That single correction is now implemented:

- the OpenCode adapter admits only the four exact matrix configurations, denies
  built-in tools, verifies resolved configuration, binds actual model and
  selected serving provider, and retains the full JSONL plus session export;
- the deterministic controller alone executes reads, searches, allowlisted
  edits/checks, and Browser handoffs, while counting every provider turn and
  preserving malformed model output as a candidate failure rather than a
  transport failure;
- the Browser bridge pauses durably, pins the edited workspace tree and local
  entry path, and accepts only narrow/medium/wide screenshot and DOM artifacts
  produced through the in-app Browser and bound to that request;
- R04's first interruption is controller-forced, not model-selected. Its second
  attempt retains one run family, cumulative usage and provider receipts, and
  exactly one interrupted plus one completed attempt;
- the campaign CLI enforces case waves, candidate-local elimination, shared-case
  audit pauses, zero retries, authoritative OpenRouter spend snapshots, and the
  `$5` / `200,000`-token ceiling;
- write-once evidence sealing and the campaign grader recompute workspace,
  diff, tool, provider, Browser, and evidence hashes before semantic checks;
- negative controls prove that unsupported completion claims fail every
  defective baseline, forbidden actions are candidate failures, post-seal
  tampering is rejected, and the public provider entrypoint remains locked.

The one permitted final independent review returned **NO-GO**. Every provider
entrypoint remains locked and no paid realistic call occurred. The review found
three release-blocking measurement defects:

1. Paid execution is not durably checkpointed after each provider turn, so a
   process crash could lose receipts, usage, and progress and permit an
   accidental rerun.
2. Required evidence identifiers can be sealed from the case definition rather
   than proven by case-specific runtime artifacts, so some completion claims can
   look satisfied without the underlying proof.
3. The Browser/release harness is incomplete: advertised preview/release checks
   do not have a reviewed executable path, and reported layout totals are not
   independently derived from hashed DOM evidence.

A loopback Browser handoff server was added after the review while closing the
local source set, but it is unreviewed and does not resolve the other two
blockers. Per the accepted one-correction/one-final-review policy, this cycle is
closed as NO-GO. A future explicitly authorized correction cycle must address
the three findings together before another independent review or provider call.

## R01 first paid batch — completed

The later correction cycle closed the three NO-GO findings, focused tests and
vet passed, and the frozen bundle received independent GO. Provider execution
was then unlocked without changing the frozen R01/R03/R04 cases or graders.
R02 remains deferred because its trusted in-app Browser attestation is not
available in this execution lane.

Two invalid calibration roots are preserved and excluded from model scoring:

- `realistic-v1`: OpenCode does not expose actual upstream provider metadata;
  the receipt contract was corrected to retain that fact while proving the
  direct OpenRouter route, requested upstream, fallback policy, and resolved
  configuration.
- `realistic-v2`: the transport referenced a supplied response schema without
  placing the frozen schema in the request. The exact frozen schema was added
  to the transport once; cases and graders were not tuned to observed answers.

| Configuration | Observation | Calls | Tokens | Cost | Elapsed | Advances |
|---|---|---:|---:|---:|---:|---|
| Qwen3 Coder Flash Default | Valid FAIL — response/tool protocol failure | 2 | 2,658 | $0.000562 | 15s | No |
| GLM 5.3 Flash Low | Valid FAIL — competence failure; runtime proof and required evidence were missing | 9 | 16,799 | $0.001846 | 12m56s | No |
| Mercury 2.5 Default | INVALID — receipt verifier treated rejected, non-allowlisted check requests as evidence corruption | 14 | 50,148 | $0.004362 | 2m47s | Not scored |
| Claude Sonnet 5 Medium anchor | INVALID — same receipt-verifier defect | 18 | 64,488 | $0.190682 | 3m02s | Not scored |

The Mercury and Claude observations are preserved exactly as observed and were
not retried in this batch. The shared non-safety verifier defect is logged for
one post-batch correction: rejected check requests must remain immutable tool
events, but only a successfully executed check can be required to match the
fixture allowlist. The invalid observations also left stale active claims that
must be reconciled before their affected candidates can be rerun under a fresh
evidence root.

There are zero valid R01 survivors, so R03 and R04 correctly received no calls.
No model was promoted and no production role was selected. The next testing
group begins only after the narrow post-batch verifier/claim correction passes
focused tests; it reruns only the observations affected by that harness defect,
then advances any valid R01 survivors through the already frozen R03 and R04.

## R01 affected-observation follow-up — completed

The post-batch receipt correction passed focused tests and vet. `realistic-v5`
then reran only Mercury and Claude, but exposed a JSON round-trip mismatch
between a newly sealed receipt and the in-memory receipt argument. Both paid
observations remain INVALID and unscored: Mercury used 13 calls, 45,871 tokens,
$0.003986, and 2m14s; Claude used 18 calls, 63,984 tokens, $0.182081, and 2m49s.

The round-trip comparison was corrected without changing a case, grader, or
model answer. Focused tests and vet passed again. Under fresh `realistic-v6`,
the same affected configurations produced valid, terminal observations:

| Configuration | Result | Calls | Tokens | Cost | Elapsed | Advances |
|---|---|---:|---:|---:|---:|---|
| Mercury 2.5 Default | FAIL — competence; no failing-before, passing-after, or current-truth runtime receipt | 20 | 77,198 | $0.006558 | 3m42s | No |
| Claude Sonnet 5 Medium anchor | FAIL — same missing runtime evidence | 24 | 95,651 | $0.275484 | 3m42s | No |

The v6 cohort used 44 calls, 172,849 tokens, and $0.282042. Both configurations
are eliminated from this realistic group with no retry. With no survivor, R03
and R04 received no call.

The required shared-failure audit found a test-interface defect before opening
another model cohort: the controller exposed `run_check` but hid the exact
bounded checks, causing models to guess command spellings that the runner then
rejected. This measures discovery of an undisclosed string, not realistic tool
competence. The next group therefore receives the exact fixture-owned bounded
check list in every request. The runner allowlist, cases, graders, evidence
requirements, safety boundary, and zero-retry policy remain unchanged. GLM's
owner-directed v3 result remains a valid failure; this correction applies only
to future groups.

No role is selected. The next planned group is a new-model R01 cohort drawn
from inexpensive configurations not previously observed in the realistic
harness; only its valid R01 survivors may enter R03 and R04.

## R01 remaining-candidate groups — completed

The v2 and v3 matrices reused frozen R01 without changing its fixture, task,
grader, runner allowlist, or acceptance rules. Exact OpenRouter model routes
were checked immediately before the v3 matrix was frozen. Fallbacks and retries
remained disabled. Invalid roots remain immutable and are not silently rewritten.

| Configuration | Scored R01 result | Calls | Tokens | Receipt cost | Observed wall span | Advances |
|---|---|---:|---:|---:|---:|---|
| Mistral Small 2603 Default | FAIL — malformed JSON action | 17 | 35,853 | $0.005518 | 1m43s | No |
| Nemotron 3.5 Lightning Default | FAIL — malformed JSON action | 1 | 1,549 | $0.000149 | 1s | No |
| GPT-OSS 20B Default | FAIL — emitted no answer text; proposed action appeared only in hidden reasoning | 4 | 6,113 | $0.000234 | 23s | No |
| GPT-5.4 Medium anchor | FAIL — missing failing-before and current-truth evidence | 14 | 27,667 | $0.089622 | 2m13s | No |
| Ling 3.0 Flash Medium | FAIL — JSON followed by an extraneous tool tag | 1 | 1,462 | $0.000036 | 0.53s | No |
| MiniMax M2.7 Default | FAIL — returned four JSON actions instead of one | 1 | 1,445 | $0.000612 | 3.87s | No |
| MiMo V2.5 Medium | FAIL — wrapped the JSON action in a Markdown code fence | 1 | 1,946 | $0.000367 | 10.31s | No |
| Nemotron 3 Super Free Low | FAIL — missing failing-before and current-truth evidence | 13 | 38,477 | $0.000000 | 5m19s | No |

Calibration and transport history is preserved separately:

- `realistic-v8` is invalid because the final sealer did not yet admit v2
  candidates. GPT-OSS used seven calls, 14,940 tokens, and $0.000748; it is not
  scored.
- `realistic-v9` remains sealed with its original transport-halt label. Offline
  receipt inspection proved its fourth call was complete and exactly accounted,
  but GPT-OSS emitted zero answer text. That existing four-call evidence is
  therefore recorded as the candidate protocol failure above without a retry.
- `realistic-v12` preserves Nemotron's invalid ten-call, 26,929-token duplicate
  lifecycle-start observation. The export proved both starts belonged to the
  same assistant message. The parser was narrowed to accept same-message
  lifecycle starts while rejecting cross-message drift; focused tests and vet
  passed. Only that affected observation was rerun in `realistic-v13`.

The eight scored observations above used 52 calls, 114,512 tokens, and
$0.096538 in rounded receipts. Including the two invalid calibration/transport
roots, v7–v13 used 69 calls, 156,381 tokens, and $0.097286. Across all twelve
validly scored realistic R01 configurations to date, no candidate passed; the
scored total is 107 calls, 306,818 tokens, and $0.380988. No configuration was
promoted, no role was selected, and R03/R04 received no calls.

The repeated absence of failing-before and current-truth receipts across
otherwise capable configurations is now a shared measurement concern. Before
calling another stronger or more expensive model, the next testing step is an
offline audit of whether those requirements are both visible and attributable
to the candidate in the frozen interface. The grader must not be tuned merely
to accept observed answers.

## Corrected R01 and first R03 capability batch

The offline audit proved R01 v1 hid its mandatory failing-before check from the
candidate. That observation set is retained as invalid calibration evidence.
R01 v2 exposes the same bounded check before editing and requires the same check
afterward; its grader was not weakened. Four fresh observations all passed:

| Configuration | R01 result | Calls | Tokens | Cost | Time |
|---|---|---:|---:|---:|---:|
| Mercury 2.5 Default | PASS | 8 | 22,830 | $0.001879 | 1m02s |
| Claude Sonnet 5 Medium | PASS | 9 | 26,685 | $0.074954 | 1m04s |
| GPT-5.4 Medium | PASS | 11 | 22,474 | $0.079063 | 1m26s |
| Nemotron 3 Super Free Low | PASS | 10 | 30,149 | $0 | 3m29s |

R01 v2 used 38 calls, 102,138 tokens, and $0.155896. This confirms the former
zero-pass result was a measurement defect, not evidence that every candidate
lacked the capability.

Mercury then validly failed R03 competence (10 calls, 34,053 tokens, $0.002983,
1m43s) because it did not produce the rollback/current-truth artifacts. Claude's
first R03 continuation exceeded the old 200,000-token campaign boundary and is
preserved as invalid budget evidence. The controller was then bounded to 15
turns and 75,000 tokens per observation, with candidate-local exhaustion.

The bounded v15 R03 group is also invalid interface evidence: the controller
did not expose the workspace file inventory, so candidates had to guess paths.
V15 used 39 calls, 114,160 tokens, and $0.243911. The inventory was exposed
without changing the task or grader, and v16 then produced:

| Configuration | R03 classification | Calls | Tokens | Cost | Time |
|---|---|---:|---:|---:|---:|
| Claude Sonnet 5 Medium | INVALID — hidden artifact schema and simulator sequence | 9 | 29,117 | $0.085203 | 1m39s |
| GPT-5.4 Medium | INVALID — affected by the same hidden interface contract | 15 | 37,219 | $0.156602 | 3m37s |
| Nemotron 3 Super Free Low | VALID FAIL — output-length/protocol failure before the hidden contract mattered | 3 | 11,178 | $0 | 1m12s |

Nemotron is eliminated from this R03 group with no retry. Claude produced the
right business interpretation in a semantically rich receipt, but the hidden
grader required an undisclosed scalar JSON schema, an undisclosed `EVIDENCE.md`,
an update to `release/state.json`, and a particular simulator sequence. GPT-5.4
was likewise asked to discover that hidden contract. Those two observations are
not competence scores. With no valid R03 pass, R04 received no call.

Post-batch correction is limited to making the existing R03 artifact contract
and simulator sequence candidate-visible. It must not change the desired
answer, weaken the grader, or retry Mercury or Nemotron. Only Claude and GPT-5.4
may receive one fresh affected-observation run, after focused tests pass. No
role is selected.

## R03 v2 and R04 completed batch

R03 v2 exposed the pre-existing artifact schema and simulator sequence. Claude
and GPT-5.4 each produced every required file and passed all three checks. Their
first deterministic grades were invalidated by one post-batch phrase-matcher
defect: the matcher treated “fixed” or “completed locally” as feature-complete
even when the same summary explicitly said the successor or `outcome_status`
remained open. The correction added semantic open-outcome paraphrases and did
not change workspace, receipt, or check requirements. Focused adversarial tests
and vet passed; no R03 provider call was repeated.

| Configuration | R03 corrected result | Calls | Tokens | Cost | Time |
|---|---|---:|---:|---:|---:|
| Claude Sonnet 5 Medium | PASS | 12 | 48,413 | $0.137957 | 2m25s |
| GPT-5.4 Medium | PASS | 14 | 38,756 | $0.143482 | 2m57s |

Both advanced once to R04's forced interruption and durable resume:

| Configuration | R04 result | Calls | Tokens | Cost | Time | Failure |
|---|---|---:|---:|---:|---:|---|
| Claude Sonnet 5 Medium | PASS | 8 | 24,514 | $0.069376 | 1m43s | - |
| GPT-5.4 Medium | FAIL | 10 | 22,676 | $0.097792 | 1m48s | Handoff omitted the superseded-claim correction |

R03 v2 used 26 calls, 87,169 tokens, and $0.281439. R04 used 18 calls,
47,190 tokens, and $0.167168. Claude is the only valid survivor of the three
non-Browser realistic cases. This is testing evidence only: no role is selected.
The next planned capability group is the deferred R02 responsive-UI case using
the trusted in-app Browser evidence lane.

## R02 Browser calibration and interface correction

R02 reached the real ChatGPT in-app Browser at three CDP-controlled viewport
widths. The first three roots are retained as invalid calibration evidence:

- `realistic-v19`: the candidate used a query-only Browser action, but the
  controller incorrectly read its empty model-supplied path instead of the
  fixture-owned `report/index.html` entrypoint.
- `realistic-v20`: three real viewport captures were sealed, but resume again
  consulted the empty model-supplied path and finalized an evidence-integrity
  halt.
- `realistic-v21`: the path defect was corrected and all evidence was graded,
  but the Browser tool returned only a generic capture count. It hid the actual
  overflow findings and the exact proof hashes that the task required the
  candidate to save. Its raw candidate-failure record is therefore not valid
  competence evidence.

V21 used 10 calls, 34,378 tokens, $0.097043 in rounded receipts, and about
3m28s wall time. Its Browser evidence itself is valid and informative: total
47 and all four cards rendered at 390, 900, and 1440 pixels; 390 and 900 pixels
had horizontal overflow, while 1440 did not.

The interface correction now returns every derived Browser fact and exact
screenshot/DOM hash to the candidate. A failed capture explicitly requires a
correction and another capture before finishing. If the candidate edits the
workspace, a fresh tree-bound capture cycle opens and the earlier cycle is
archived by request hash rather than overwritten. The capture sink is
loopback-only, protected by an out-of-page ephemeral token, and accepts only
the in-app Browser evidence payload. Focused package tests and `go vet` pass.

The single independent review returned GO. `realistic-v22` then proved the
corrected Browser contract worked: after the first 390/900 overflow result,
Claude repaired the CSS and the second capture passed at 390, 900, and 1440.
However, the run reached the old 15-step ceiling immediately after writing its
exact evidence receipt and before the final finish action. V22 is retained as
invalid step-budget calibration evidence: 15 calls, 64,575 tokens, and
$0.183626. It is not a candidate failure.

The only further harness correction raised R02's fixed action allowance from
15 to 18; every other case remains at 15. This was required by R02's already
frozen failed-capture -> correction -> recapture contract and was covered by
focused tests and `go vet`. No new reviewer cycle was opened.

`realistic-v23` is the one fresh affected observation. Claude first produced a
correct 47-view/four-card report with overflow at 390 and 900, received the
structured Browser facts, repaired only the report CSS, and then produced
independent Browser proofs with no overflow at 390, 900, or 1440. The hidden
grade passed. Exact result: 13 provider calls, 54,673 tokens, $0.156414, about
3m38s, zero retries, zero owner interventions, and no external side effects.

Claude therefore completes R01-R04 as the sole survivor of this realistic
capability group. This is qualification evidence only: no production role is
selected or assigned.
