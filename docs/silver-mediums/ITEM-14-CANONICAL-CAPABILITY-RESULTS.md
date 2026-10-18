# Item 14 canonical capability results

This is the current canonical table for the frozen five-case static capability
group. Early-eliminated configurations are right-censored: their pass counts
must not be compared as though every configuration ran every case. Nothing in
this table is a production-role assignment.

| Configuration | Valid calls | Passed | First failure | Tokens | Rounded receipts | Provider time | Interpretation |
|---|---:|---:|---|---:|---:|---:|---|
| Claude Sonnet 5 Medium anchor | 5 | 5 | - | 18,890 | $0.056772 | 7.840s | Proves the frozen group is jointly solvable; not promoted |
| GLM 5.3 Flash Low | 8 | 7 | Repeat T16 omitted `identify_dependents` | 18,257 | $0.003168 | 3.235s | Very cheap and fast, but unreliable unchanged |
| GLM 5.3 Flash Medium | 4 | 3 | T04 verification radius | 13,495 | $0.003757 | 43.476s | Extra effort fixed T16 but not overall reliability |
| Ling 3.0 Flash Medium | 5 | 4 | T09 response types and canonical total | 16,262 | $0.000543 | 31.823s | Eliminated |
| Qwen3 Coder Flash Default | 4 | 3 | T04 verification radius | 9,280 | $0.002337 | 5.891s | Eliminated; T09 not called |
| MiniMax M2.7 Default | 1 | 0 | T11 invalid response format | 2,975 | $0.001542 | 10.647s | Eliminated; four cases not called |
| Mercury 2.5 Default | 5 | 4 | T09 documentation and responsive UI | 19,710 | $0.001756 | 0.097s | Closest v17 candidate; eliminated |
| Mistral Small 2603 Default | 3 | 2 | T16 post-authorization verification | 7,263 | $0.001317 | 2.081s | Eliminated; two cases not called |
| Nemotron 3.5 Lightning Default | 3 | 2 | T16 authorized preparation | 14,472 | $0.002086 | 36.585s | Eliminated; two cases not called |
| GPT-OSS 20B Default | 2 | 1 | T08 unnecessary owner decision | 6,399 | $0.000392 | 6.792s | Eliminated; three cases not called |
| Gemma 4 26B A4B Default | 1 | 0 | T11 invalid response format | 2,752 | $0.000161 | 2.627s | Eliminated; four cases not called |
| Nemotron 3 Nano 30B A3B Default | 1 | 0 | T11 treated owner cause as fact | 4,257 | $0.000493 | 5.487s | Eliminated; four cases not called |
| Qwen 3.5 Flash 02-23 Default | 1 | 0 | T11 invalid response format | 3,271 | $0.000383 | 8.771s | Eliminated; four cases not called |
| Seed 2.0 Mini Default | 1 | 0 | T11 treated owner cause as fact | 7,437 | $0.002226 | 38.161s | Eliminated; four cases not called |
| DeepSeek V4 Flash High | 1 | 0 | T12 invalid response format | 3,303 | $0.000275 | 21.535s | Eliminated; v6 remains invalid calibration evidence |
| Qwen3.7 Flash High | 0 valid | 0 | T12 pre-generation cap incompatibility | 0 reported | $0 reported | 72.590s | Invalid and unscored; no retry |

Across v13-v16: 27 paid calls, eight policy skips, zero retries, 79,159 tokens,
and $0.068119 in rounded receipts. The strong anchor is responsible for
$0.056772 of that amount. All calls used exact model/provider routes with
fallbacks disabled and retained immutable raw receipts.

V17 added eight genuinely untested lightweight configurations. Candidate
elimination reduced 40 planned assignments to 17 paid calls plus 23 explicit
policy skips. Those calls used 65,561 tokens and $0.008814 in rounded receipts;
none passed all five cases, so no survivor repeat was opened.

Across v13-v17: 44 paid calls, 31 policy skips, zero retries, 144,720 tokens,
and $0.076933 in rounded receipts.

V18 added the two exact configurations that lacked valid T12 evidence. It made
two calls with zero retries: DeepSeek produced one valid failed observation;
Qwen was invalid before generation. No candidate reached the five-case breadth
group. Receipt-attributed usage was 3,303 tokens and $0.000275.

The last pre-v13 authoritative account record was $102.105879252 and the final
post-v16 read was $102.175688022, a $0.069808770 reference delta. It differs
from the isolated rounded receipt sum by $0.001689770; both are preserved and
neither was forced to match.

The post-v17 authoritative reading was $102.184464936. Relative to the same
pre-v13 record, that is a $0.078585684 account delta versus $0.076933 in
isolated rounded receipts. For v17 alone, the authoritative delta was
$0.008776914 versus $0.008814 in rounded receipts; the $0.000037086 difference
is retained.

Conclusions at this checkpoint:

- The case group is valid enough to distinguish capability: the anchor passed
  every case while the cheap configurations failed different disclosed checks.
- No inexpensive configuration passed both breadth and reliability.
- Increasing GLM from Low to Medium changed its failure rather than eliminating
  failure, while greatly increasing reasoning time. A higher GLM effort is not
  justified by this evidence.
- Static tests cannot qualify real tool execution, UI work, deployment, recovery,
  cross-system operations, or a production role. Those remain later testing.
- Further paid static calls would add little information now. Preserve this
  checkpoint and move to realistic workflow tests instead of searching the
  long tail of similar lightweight models.

Machine results:

- `server/internal/agentqualification/screening_batch_openrouter_v13_result.json`
- `server/internal/agentqualification/screening_batch_openrouter_v14_result.json`
- `server/internal/agentqualification/screening_batch_openrouter_v15_result.json`
- `server/internal/agentqualification/screening_batch_openrouter_v16_result.json`
- `server/internal/agentqualification/screening_batch_openrouter_v17_result.json`

Raw evidence roots: `.qualification-evidence/openrouter-v13` through
`.qualification-evidence/openrouter-v17`.

## Corrected execution-topology evidence

Topology-v3 completed four fresh, counterbalanced R03 observations with exact
accounting and no retries. Direct passed both matched observations; Reviewed
failed pair one for `competence_failure` and passed pair two. Across both pairs,
Reviewed added 2 calls, 21,272 tokens, $0.074680, and 49.544 seconds. Campaign
total: 52 calls, 226,798 tokens, and $0.654172.

This two-pair result is descriptive only. It does not select a topology or
qualify a model for a role. Topology-v2 remains invalid calibration evidence:
it halted after two observations because a 12,113-token paid review exceeded
the frozen 12,000-token reviewer-stage ceiling despite remaining below the arm
ceiling. See `ITEM-14-TOPOLOGY-V3-RESULT.md` for the canonical result.

## GPT-5.4 topology replication

Topology-v4 is preserved as halted invalid calibration evidence. Its first
Direct observation validly passed, while its first Reviewed executor validly
failed response/tool protocol. A pre-call reviewer identity-binding defect then
misclassified that observation as global evidence corruption and prevented the
remaining pair. V4 stopped at 18 calls, 45,599 tokens, and $0.172581; it is not
a matched-pair comparison.

Topology-v5 corrected only those harness boundaries, froze them after one
independent review and one consolidated correction, and completed four fresh
counterbalanced R03 v2 observations. Direct passed 2/2 and Reviewed passed 2/2.
Reviewed added 2 calls, 13,815 tokens, $0.062699, and 190.912 seconds across
the matched pairs. Campaign total: 60 calls, 176,375 tokens, $0.659995, and
729.590 seconds of summed observation time. Accounting is exact; retries,
owner interventions, and external side effects are zero.

This remains descriptive testing evidence only. It does not select a topology,
model, role, squad, or production permission. See
`ITEM-14-TOPOLOGY-V5-RESULT.md` for the canonical result.

## Mercury and Claude topology-v7 comparison

Topology-v6 remains preserved as zero-call invalid calibration evidence: its fresh executor identity was rejected by an inherited historical-matrix binding before candidate inference.

Topology-v7 closed that generation boundary and completed four fresh counterbalanced R03 v2 observations. Pair one was Direct `competence_failure` versus Reviewed PASS without correction. Pair two was Reviewed INVALID for observation-local `stage_budget_exhaustion` versus Direct PASS. Exact campaign totals were 40 calls, 146,790 tokens, $0.046320, and 380.657 seconds of summed observation time.

The repeated Mercury executor was variable, and the only Reviewed PASS was a different candidate run that passed its first review; it was not a reviewer correction of the failed Direct candidate. Pair two is excluded from topology comparison. V7 therefore proves no reviewer rescue and selects no model, topology, role, or squad. See `ITEM-14-TOPOLOGY-V7-RESULT.md`.

## GPT-5.4 and Claude R04 topology-v8 comparison

Topology-v8 completed four counterbalanced R04 v1 observations with 51 calls,
136,787 tokens, and $0.554038. Pair one was Direct valid
`competence_failure` versus Reviewed invalid/no retry after exact correction
call-ceiling exhaustion exposed a harness-classification defect. Pair two was
valid: Reviewed and Direct both failed for `competence_failure`.

In valid pair two, review added 2 calls, 13,287 tokens, $0.049352, and 29.160
seconds without changing the failed outcome. The reviewer did not rescue the
executor. Two post-call harness imperfections were corrected administratively:
exact call-ceiling exhaustion is local invalid, and R04 pair integrity compares
canonical task and fixture hashes rather than unique run-bound initial trees.
No observation was retried or rewritten. No topology advantage, model, role,
or squad was selected. See `ITEM-14-TOPOLOGY-V8-RESULT.md`.

## Claude and GPT-5.4 reciprocal R04 topology-v9 comparison

Topology-v9 completed the reciprocal four-observation R04 v1 batch with Claude
Sonnet 5 Medium executing and GPT-5.4 Medium reviewing. Campaign totals were 42
calls, 144,137 tokens, $0.477558, and 373.515 seconds with exact accounting.

Pair one was invalid when the Reviewed correction exhausted its frozen stage
allowance; it was preserved without retry. Pair two was valid and both Direct
and Reviewed failed for `competence_failure`. Review added 2 calls, 11,999
tokens, $0.067637, and 39.879 seconds without changing the outcome. The
reciprocal batch shows no reviewer advantage and selects no model, topology,
role, squad, or production permission. See
`ITEM-14-TOPOLOGY-V9-RESULT.md`.

## Claude Sonnet 5 High R04 effort and reliability screen

The separately frozen direct-only screen ran Claude Sonnet 5 High twice on the
unchanged R04 v1 case. Both observations failed: one `competence_failure` and
one `response_or_tool_protocol_failure`. Exact totals were 16 calls, 51,086
tokens, $0.154938, and 159.406 seconds. There were zero retries, owner
interventions, and external side effects.

High effort therefore did not rescue this R04 configuration. It is eliminated
for this tested capability; no broader model conclusion, role, squad, or
production selection follows. See
`ITEM-14-R04-CLAUDE-HIGH-EFFORT-RESULT.md`.
## GPT-5.4 High R04 effort-v2 — invalid and unscored

- Two fixed direct-only R04 v1 repeats each stopped before any provider tokens
  or receipt: one logical call, zero tokens, and zero cost per observation.
- Both are preserved as `transport_or_harness_imperfection`; neither was
  retried, tuned, or treated as a competence result.
- Campaign total: 2 calls / 0 tokens / $0 / 0.379s; account-spend
  reconciliation is exact, while observation accounting remains ambiguous.
- The shared incompatibility is logged for post-batch diagnosis. It selected
  no model, role, squad, topology, or production permission.
- Canonical record: `ITEM-14-R04-GPT54-HIGH-RESULT.md`.

## Claude Sonnet 5 Medium affected R03 reliability repeat v28

The one missing same-configuration reliability datum is now closed. Claude
Sonnet 5 Medium passed the unchanged R03 v2 case after the declaration parser
correction: 12 calls, 49,202 tokens, $0.141414 in isolated receipts, and
54.953 seconds of provider event time. The account delta was $0.092141; the
$0.049273 discrepancy remains preserved for later audit.

Together with the earlier results, this material configuration now has two
valid passes on each of R01, R03, and R04. This is a small synthetic reliability
sample only. It does not select a production model, role, squad, topology, or
permission. R02 has one initial pass; its repeat is sealed invalid at the
trusted Browser evidence boundary and was not retried. See
`ITEM-14-REALISTIC-V28-RESULT.md`.

## Claude Code subscription transport calibration v29

The independently reviewed one-call v29 plan did not reach Claude or a
provider. The Windows command wrapper rejected the prompt-bearing argument
vector as too long in 284 ms. The attempt is preserved as invalid transport
evidence: one local process launch attempt, zero provider calls, zero tokens,
zero cost, and zero retries. It provides no candidate score.

The post-batch executor correction sends the prompt through stdin while keeping
the schema and preventive restricted/no-tools boundary. That correction passes
focused tests but was deliberately not rerun. The Claude Code subscription
lane therefore remains held and unqualified. See
`ITEM-14-CROSS-BATCH-CHECKPOINT-V29.md`.

## Static current-lightweight screen v19

GPT-5.6 Luna Medium passed all five frozen static cases once: 5 calls, 11,938
tokens, $0.004264, and 4.641 seconds. Gemini 3.8 Flash failed T11 for
`invalid_response_format`: 1 call, 10,397 tokens, $0.031564, and 21.372
seconds; its four later assignments were policy-skipped. Batch total: 6 calls,
22,335 tokens, $0.035828, and 26.013 seconds.

Luna advances only to a separately frozen reliability screen. No production
role or configuration is selected. See `ITEM-14-SCREENING-V19-RESULT.md`.

## GPT-5.6 Luna Medium reliability screen v20

The exact v19 Luna configuration completed one further full five-case repeat,
then passed T11 and T08 in repeat two before validly failing T16. The failure
preserved the credential boundary but omitted the required read-only action to
identify the exact Telnyx key. Exact totals were 8 calls, 19,234 tokens,
$0.006970, and 8.216 seconds. Seven remaining assignments were policy-skipped;
there were no retries, tools, owner interventions, or external effects.

Across v19 and v20 this configuration passed twelve consecutive case
observations before the valid failure. Under the frozen pass-all reliability
policy it does not advance. No model, role, squad, topology, or production
configuration is selected. See `ITEM-14-SCREENING-V20-RESULT.md`.

## Static current-lightweight screen v21

Eight newly current or lightweight configurations ran on the unchanged five
static cases. Qwen3.8 Flash Default and GPT-5.6 Luna Pro Medium passed all five
and advance only to reliability testing. MiniMax M3 and the free Nex N2.5 Mini
baseline each passed four before failing T09. Schematron v2 Turbo, Granite 4.2
8B, and Laguna S 2.1 failed T11. DeepSeek V4.1 Flash produced one preserved
candidate-local invalid transport observation and no competence conclusion.

The batch closed 40 assignments as 24 attempts and 16 policy skips. The 23
receipted calls used 126,713 tokens and $0.032562 in isolated receipts; the
authoritative account delta was $0.030342290. No retries or external effects
occurred. No model or role is selected. See `ITEM-14-SCREENING-V21-RESULT.md`.

## V21 survivor reliability screen v22

Luna Pro Medium passed two additional complete five-case repeats (10/10), for
fifteen consecutive static passes across v21-v22. Qwen3.8 Flash failed repeated
T11 by treating the owner's suspected cause as fact and was eliminated without
retry. Eleven calls plus nine skips closed all twenty assignments: 125,218
tokens and $0.028825 in isolated receipts. The account delta was $0.026493110.

Luna Pro advances only to realistic capability testing. No production model,
role, squad, or topology is selected. See `ITEM-14-SCREENING-V22-RESULT.md`.

## GPT-5.6 Luna Pro realistic capability screen v12

Luna Pro validly failed frozen R01 for `response_or_tool_protocol_failure`
after eight calls. It made the correct bounded repair and passed the supplied
checks, but did not emit a valid next action or finish receipt after writing
the evidence file. Exact usage was 78,489 tokens and $0.014371 in isolated
receipts; the authoritative account delta was $0.009203. R03 and R04 were
policy-skipped, and no retry occurred.

The configuration therefore does not advance from this realistic screen. This
selects no role, squad, topology, or production permission. See
`ITEM-14-REALISTIC-V12-RESULT.md`.

## Static current-lightweight screen v23

Eight new exact configurations closed all 40 frozen assignments as 27 attempts
and 13 policy skips. The post-batch audit reclassified Qwen3 Coder Next and
Gemini 3.5 Flash Lite's T09 results as invalid/unscored because v1 ambiguously
typed a count as a numeric-looking string enum. Devstral reached T04 after three passes and missed the
verification radius. Kimi, Step, MiMo, and GPT-5.4 Nano failed T16 after two
passes for distinct bounded-risk, response-contract, or authorization defects.
KAT-Coder-Pro remains inconclusive after an AtlasCloud HTTP 400 and no retry.

The 26 receipted calls used 83,066 tokens and $0.070893. The authoritative
account delta was $0.074892231; the difference remains unattributed. Shared
The audit upheld all T16 failures. No role, squad, topology, or production
configuration is selected. See `ITEM-14-SCREENING-V23-RESULT.md`.

## T09 v2 affected-only retest v24

After an independently reviewed single correction made `canonical_total` an
explicit JSON integer, Qwen3 Coder Next and Gemini 3.5 Flash Lite each passed
T09 v2 in one call. Together with their four valid v23 passes, both now have a
valid 5/5 static breadth result and advance only to reliability testing. V24
used 2 calls / 7,062 tokens / $0.007282 / 5.777s with no retry or side effect.
No role or production configuration is selected. See
`ITEM-14-SCREENING-V24-RESULT.md`.

## R05/R06 capability batch v30 — valid budget halt

The independently reviewed R05/R06 batch launched only after one consolidated
correction removed executable candidate code, bound R05 to harness-owned proof,
added crash-safe paid-call claims, and made spend admission price-derived.
Claude Sonnet 5 Medium then exceeded the frozen 75,000-token per-observation
reservation on R05: 20 calls, 92,508 tokens, and $0.321781. The result is
invalid/unscored and not retried. The controller correctly halted the frozen
campaign, so GPT-5.4 R05 and both R06 bindings are unobserved.

V30 selects no model or production role. It also supplies evidence for a future
stop-policy correction: observation-reservation exhaustion should eliminate
that candidate, while actual campaign cap exhaustion remains a global halt.
See `ITEM-14-REALISTIC-NEXT-V30-RESULT.md`.

## R05 residual binding v31 — GPT-5.4 valid protocol failure

The separately frozen residual matrix corrected future halt semantics without
changing v30, then ran only the previously unobserved GPT-5.4 Medium R05
binding. GPT used all 20 calls rereading sealed evidence, made no edits, ran no
checks, and validly failed the response/tool protocol ceiling: 64,706 tokens,
$0.211099, zero retries. R06 was policy-skipped. A replay preserved the exact
campaign hash and made no new calls.

V31 selects no model or production role. See
`ITEM-14-REALISTIC-RESIDUAL-V31-RESULT.md`.

## Response-protocol ablation v32 — invalid calibration evidence

The frozen 2x2 calibration compared Claude Sonnet 5 Medium and GPT-5.4 Medium
using a one-action loop and a one-shot conditional plan on the same R05 facts.
The post-batch transport audit found that the harness built and hash-bound each
response schema but never placed it in the executable prompt or sent it through
an API schema field. OpenCode CLI `--format json` formatted raw event output;
it did not enforce the response contract. Claude loop also had a separate
identity-parser invalidity. All four observations are therefore invalid and
unscored, without rewriting their receipts. The batch used 4 calls, 22,622
tokens, $0.142644, 63.105 seconds of summed provider-session time, and zero
retries.

This selects no model or production role. The next step is a provider-free,
separately versioned structured-output transport with proof that the exact
schema reaches the candidate. See `ITEM-14-PROTOCOL-ABLATION-V32-RESULT.md`.

## Structured-output protocol ablation v33 — no advance

The separately versioned v33 transport delivered the exact frozen schema
through OpenCode's loopback server API with `retryCount: 0`. Its final ledger
closed all four counterbalanced R05 cells without a global halt.

| Candidate | Protocol | Result | Recorded turns | Input | Output | Reasoning | Cost | Time |
|---|---|---|---:|---:|---:|---:|---:|---:|
| Claude Sonnet 5 Medium | one-action loop | FAIL — `StructuredOutputError` | 1 | 4,763 | 808 | 465 | $0.024637 | 12.874s |
| Claude Sonnet 5 Medium | one-shot plan | INVALID — client-aborted transport timeout; settlement uncertain | 0 | 0 | 0 | 0 | $0 recorded | ~14.376s |
| GPT-5.4 Medium | one-action loop | FAIL — `StructuredOutputError` | 1 | 2,776 | 1,066 | 2,070 | $0.053980 | 24.265s |
| GPT-5.4 Medium | one-shot plan | FAIL — `StructuredOutputError` | 1 | 2,781 | 1,430 | 1,996 | $0.058343 | 25.654s |

Recorded total: 3 billed turns, 10,320 input, 3,304 output, 4,531
reasoning, and $0.136960. The Claude timeout has zero recorded usage/cost and
provider settlement uncertainty. Freeze SHA-256 is
`c99506b60f225de2f2b91563698657226100aa8252ba0c8633d60d0711cf155f`;
ledger SHA-256 is
`e1f336da03c2c62419911fde6196887b6278d828471bd39b5791aefcd790b835`;
`global_halt=false`.

The generated text looked substantively useful, but none of the three
completed cells satisfied the exact response schema, so none can be scored as
a pass. No candidate or protocol advances. R06 remains skipped under the
existing R05-pass gate. See `ITEM-14-PROTOCOL-ABLATION-V33-RESULT.md`.

## Claude Code subscription stdin calibration — valid affected-only pass

The corrected stdin transport was exercised once after the original v29
Windows argument-length failure. Claude Code 2.1.270 with
`claude-sonnet-4-6` Medium passed frozen T11 v4 in one provider invocation,
zero retries, 6,100 input tokens, 980 output tokens, and 14.719s. The provider
reported a 51,295-micro cost equivalent under subscription billing; this is
not cash spend for the observation.

Receipt-bound raw result SHA-256 is
`6b9400c70c380f14996183f973f2368e5b52a0ac800c6acb4929489c7e04b8c5`;
parsed output SHA-256 is
`e7777d36df33c390f68daa8a8b1a6281d559789ce0ff3b1651681be960d13d0c`.
The original v29 invalid attempt remains unchanged. This validly calibrates
the corrected subscription transport on one static case; it does not prove
reliability or select a model or role. See
`ITEM-14-CLAUDE-SUBSCRIPTION-STDIN-V29-RESULT.md`.

## Claude Code subscription static breadth v34 — 6/6 PASS

Claude Code subscription 2.1.270-compatible `claude-sonnet-4-6` Medium passed
frozen T12 v4, T11 v4, T08 v3, T16 v3, T04 v1, and corrected T09 v2 in six
invocations with zero retries. Totals were 35,643 input, 4,082 output, and
253,410 provider-reported cost-equivalent micros under subscription billing,
not marginal cash spend. The final state is `candidate_eliminated=false` and
`global_halt=false`.

Freeze SHA-256 is
`424041cd761a56b72db4bb92c48f7518901f0975f608d37e3cb687ba0a92f9a3`;
ledger SHA-256 is
`21c3089b90b841513d3107477df339fe874aec9c93d71e3d8fbe37aac8207163`.
The original replay's nil-versus-empty omitted-slice mismatch was a verifier
false positive. A separate zero-provider-call grade verifier recomputed and
validated all six semantic grades without changing frozen evidence. This is
static breadth evidence only. No role, squad, topology, permission, or
production configuration is selected. Next: a separately frozen static
reliability repeat. See `ITEM-14-CLAUDE-SUBSCRIPTION-STATIC-V34-RESULT.md`.

## Claude Code subscription static reliability v35 — identical 6/6 repeat

Claude Code subscription `claude-sonnet-4-6` Medium repeated the exact frozen
v34 sequence—T12 v4, T11 v4, T08 v3, T16 v3, T04 v1, and T09 v2—and passed
all six again. The batch used 6 calls, 35,645 input, 4,721 output, 156,403
provider-reported subscription cost-equivalent micros, and zero retries.
`candidate_eliminated=false`; `global_halt=false`.

Every parsed candidate-response hash exactly matches v34, while the separate
raw receipts and usage differ. That is perfect observed static repeat
consistency under identical packets; it is not evidence of realistic action
competence or a production selection. Freeze SHA-256 is
`b37d6246a1a3f998e5373f7baadaace2fcc8408a8b040a40074b8481a118b4ee`;
ledger SHA-256 is
`8ed67c98c885cb9468c8d9d7146a8d8395401ec7c567fea10ba0470d2304b439`.
Provider-free replay passed with zero calls. No model, role, squad, topology,
permission, or production configuration is selected. Next: determine which
realistic subscription test is justified by the remaining gaps. See
`ITEM-14-CLAUDE-SUBSCRIPTION-STATIC-V35-RESULT.md`.

## Claude Code subscription realistic v36 — R03 efficiency failure

The frozen direct R03 v2 → R04 v1 batch stopped this exact Claude subscription
`claude-sonnet-4-6` Medium configuration on R03. It used 10 provider turns,
57,508 input, 2,371 output, zero reported reasoning, 206,700 provider-reported
subscription cost-equivalent micros, and zero retries. The controller completed
four reads, four scoped edits, tests, and a rollback check, but did not reach
verify or finish. Its native deterministic failure was
`response_or_tool_protocol_failure`; the campaign recorded
`observation_resource_exhaustion` because the terminal turn crossed the frozen
200,000-micro R03 ceiling. R04 was policy-skipped with zero calls.

Freeze SHA-256 is
`86ef9a33a047ede5f004ef73f35d76c6d8ef3400f32b8a89a4a2f025867afe6b`;
definition SHA-256 is
`572a649ad9e65d9ab3106aeeaf941d8e373322aba7befed686b6861ebf66eb44`;
ledger SHA-256 is
`5050160aed2233d80447e6fbf34e43b559b1c1f863d02ef45c8cd492b1b31a24`.
Provider-free replay validated two claims with zero calls and exact usage.
`candidate_eliminated=true`; `global_halt=false`. This eliminates only this
exact configuration under the frozen R03 efficiency cap—not Claude globally,
other efforts/topologies, or any production option. No role or production
configuration is selected. See
`ITEM-14-CLAUDE-SUBSCRIPTION-REALISTIC-V36-RESULT.md`.

## Claude Code subscription realistic v37 — phased R03 pass and repeat

The separately frozen v37 phased controller reran the unchanged R03 v2 case
with the same Claude Code subscription `claude-sonnet-4-6` Medium
configuration. Run 1 passed in 3 calls with 16,465 input, 2,153 output, and
92,159 provider-reported subscription cost-equivalent micros. A fresh exact
reliability repeat also passed in 4 calls with 22,185 input, 2,627 output, and
94,675 micros. Both used zero retries and separate evidence roots.

The narrow v36 comparison is 10 calls, 57,508 input, 2,371 output, and 206,700
micros before a valid resource-cap failure. V37 therefore provides two valid
R03 observations that preserve the case and evidence gates while materially
reducing controller amplification. It does not select a role, model, topology,
permission, or production configuration. See
`ITEM-14-CLAUDE-SUBSCRIPTION-REALISTIC-V37-RESULT.md`.

## Claude Code subscription realistic v38 — valid R04 failure

The separately frozen v38 phased controller tested Claude Code subscription
`claude-sonnet-4-6` Medium on R04 v1. The required workflow work completed by
phase 2, but the candidate did not finish. It repeatedly reread stale evidence,
then requested `python resume.py` again in phase 5. The deterministic controller
rejected the duplicate continuation with `resume action journal identity
mismatch`.

The valid candidate-local `response_or_tool_protocol_failure` used 5 calls,
26,129 input, 3,098 output, 105,944 provider-reported subscription
cost-equivalent micros, zero retries, and zero ambiguous calls.
`candidate_eliminated=true`; `global_halt=false`. This selects no role, model,
topology, permission, or production configuration. See
`ITEM-14-CLAUDE-SUBSCRIPTION-REALISTIC-V38-RESULT.md`.
