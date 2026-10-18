# Item 14 model and configuration qualification result

Date: 2026-09-11  
Scope: local Silver Mediums Multica pilot only  
Decision version: `silver-mediums.agent-configurations.v1`

## Decision

Model names alone are not assigned to roles. The executable decision unit is:

`role + model + effort + instructions + skills + permissions + runtime + evidence scope`

The machine-readable registry is
`server/internal/agentqualification/accepted_configurations.json`. Every
accountable capability binding in `systemgraph/roles.json` resolves to exactly
one configuration decision. All Item 14 decisions fail closed for production.

## Selected candidates, not qualified agents

A role is a bounded responsibility in a workflow. It is not automatically a
permanent agent process or another always-running seat. The controller may bind
an accountable domain role, invoke one repository executor, and add assurance
functions only when the graph proves they are required.

| Function | Selected configuration | Decision | Why |
|---|---|---|---|
| Owner interface / Mika | Luna, medium | provisional shadow candidate | Passed two representative owner-interface trials; locked scenario repeats remain required. |
| Shared Windows builder | Sol, high | provisional shadow candidate | Passed representative cross-boundary and bounded-builder trials; capability-specific locked cases remain required. |
| Independent Quality Checker | Terra, xhigh | provisional shadow candidate | Passed one strict false-completion trial and provides model-family separation; mandatory repeats remain required. |
| Product & UI | Sol, high | provisional shadow fallback | Passed one responsive-product trial; the locked Product/UI cases and repeats remain required. |
| Release & Runtime | Sol, high | provisional shadow only | Architecture reasoning is acceptable, but no production release qualification was run in Item 14. Typed receipts and owner authority remain mandatory. |
| Phone Farm Application | Mac-owned external configuration | held external | Windows did not inspect, operate, choose, or qualify the Phone Farm model. Dedicated Mac Studio agents must return sanitized evidence. |
| Architecture, security, audit, data-sync contributors | on-demand, route-scoped | not permanent seats | These are functions added only when the graph proves the boundary. They do not own the issue or multiply routine runs. |
| Domain outcome owners | Sol high placeholders pending qualification | held pending qualification | Business ownership is separated from repository execution. Each domain role still needs its applicable locked cases before it may act. |

Claude Sonnet remains an unevaluated Product & UI candidate because the live
Claude runtime reported its weekly limit. It is not classified as a competence
failure and is not selected until a later benchmark can run.

## 2026-09-12 controlled low-cost screening addendum

The first metered OpenCode/OpenRouter comparison is complete. It tested one
frozen T12 authority-and-scope case with the same tool-free transport, prompt,
rubric, permissions, output cap, and one-call limit for both candidates.

| Configuration | Result | Counted tokens | Wall time | Conservative receipt |
|---|---|---:|---:|---:|
| DeepSeek V4 Flash, high | fail: 7/9 | 2,631 | 7.783 s | $0.000219 |
| Qwen 3.7 Flash, high | fail: 7/9 | 4,159 | 15.851 s | $0.000333 |

Neither candidate is selected or qualified. Both preserved the intended
authority and scope in substance, but both missed the frozen rubric's explicit
field-level language for the unchanged canonical authority contract and all
untouched identity, role, and grant surfaces. The shared miss is a calibration
question, not permission to weaken or rewrite the rubric after seeing model
identities.

DeepSeek used 36.7% fewer counted tokens, completed 50.9% faster, and cost 34.2%
less in this single observation. Those measurements establish only a provisional
efficiency signal; they do not establish competence or a role assignment.

Exact v5 provider spend was $0.000550616. Exact cumulative OpenRouter spend for
v1-v5 was $0.003138116. No further paid screening is authorized by this result;
the next step is a model-blind calibration decision followed by the remaining
minimum cases under a newly sealed batch, not repeated retries of T12.

### Corrected-case confirmation completed but cannot rank competence

V6 ran DeepSeek and Qwen once each on the identical T12 v3 prompt, schema,
transport, effort, permissions, and output cap. Both preserved `team_lead`,
located the client predicate fault, limited remediation to that consumer,
preserved identity and grants, and included CRM, Canopy, and Phone Farm in the
verification radius. Both classified only `crm_ui` as directly affected.

The deterministic grader treated that shared choice as `ignored_consumer` even
though the public field never required unaffected downstream contracts to be
labelled affected. Qwen also failed `owner_outcome` only because it used
"view" instead of one of the grader's accepted exact words while stating that
the team lead can successfully view the CRM page. A focused anonymous review
confirmed the public/hidden semantic mismatch.

The comparison is therefore complete for transport and cost, but invalid for
competence ranking. Neither candidate advances and no winner exists.

| Configuration | Result | Failure reason | Counted tokens | Receipt cost | Time | Advances |
|---|---|---|---:|---:|---:|---|
| DeepSeek V4 Flash, high | invalid test (automatic FAIL) | `affected_consumers`; unjustified `ignored_consumer` | 3,641 | $0.000348 | 17.650 s | No |
| Qwen 3.7 Flash, high | invalid test (automatic FAIL) | `affected_consumers` plus phrase-sensitive `owner_outcome`; unjustified `ignored_consumer` | 4,152 | $0.000327 | 23.324 s | No |

Call-specific receipts total $0.000675. Shared-account usage moved by
$0.000796493 during the window and may include unrelated usage. No retry ran.
Framework expansion stops here.

### First valid broad low-cost screen (v7)

T12 v4 replaced the invalid v6 case with a model-blind contract that explicitly
separates direct impact from the complete verification radius. An independent
review first rejected its one-value enum schema; one consolidated correction
made the enum generatable, and the same reviewer returned GO before any call.

Six exact models then ran once each through the isolated OpenCode/OpenRouter
lane. Every route admitted one serving provider with fallbacks disabled. There
were no retries, tools, repository access, Browser work, production action,
messages, or Phone Farm host access.

| Model and effort | Result | Failure | Tokens | Cost | Provider-event time | Advances |
|---|---|---|---:|---:|---:|---|
| Nemotron 3 Super Free, low | PASS | — | 4,690 | $0.000000 | 21.919 s | Yes |
| MiMo 2.5, medium | FAIL | omitted complete verification scope | 4,461 | $0.000928 | 66.458 s | No |
| GLM 5.3 Flash, low | PASS | — | 2,422 | $0.000411 | 0.251 s | Yes |
| Ling 3.0 Flash, medium | PASS | — | 3,375 | $0.000108 | 2.168 s | Yes |
| Qwen3 Coder Flash, default | PASS | — | 2,474 | $0.000619 | 1.517 s | Yes |
| MiniMax M2.7, default | PASS | — | 2,912 | $0.001434 | 10.823 s | Yes |

Authoritative OpenRouter account usage moved from `$102.075658209` to
`$102.079155878`: `$0.003497669`, below the `$0.020000` incremental ceiling.
Per-call receipt costs round conservatively to `$0.003500`. Provider-event time
is not end-to-end application latency. The five PASS results advance only to
later survivor validation; none is a qualified agent or role selection. A
strong anchor was unnecessary for this cheap first screen and was not run.
One final independent evidence review returned GO: all six sealed observations,
cost arithmetic, hashes, no-retry claims, and limited advancement language match.

Immutable result: `screening_batch_openrouter_v7_result.json`; raw evidence is
under `.qualification-evidence/openrouter-v7`.

### Survivor consistency and T11 screening (v8-v10)

The five V7 passes each received one second, identical T12 v4 observation in
V8. All five passed again with no retries or tool events. Exact account spend
was `$0.002837859` for 15,015 provider-reported tokens. This established
repeatability on T12 only; it did not qualify a role.

V9 then tested those five once each on T11 v3. The blind grader produced the
following raw outcomes, but an independent review invalidated the batch as
competence evidence:

| Model and effort | T11 result | Failed requirements | Tokens | Cost |
|---|---|---|---:|---:|
| GLM 5.3 Flash, low | FAIL | bounded remediation | 2,557 | $0.000586 |
| Nemotron 3 Super Free, low | FAIL | causal evidence; bounded remediation; verification | 3,541 | $0.000000 |
| Ling 3.0 Flash, medium | FAIL | causal evidence; bounded remediation; verification; limits | 3,571 | $0.000136 |
| Qwen3 Coder Flash, default | FAIL | causal evidence; bounded remediation | 2,341 | $0.000728 |
| MiniMax M2.7, default | FAIL | bounded remediation | 2,959 | $0.001789 |

Exact V9 spend was `$0.003237366`. Every authorization receipt incorrectly
retained a T12-specific scope. More importantly, the grader's
`bounded_remediation` check required specific words and falsely rejected GLM's
semantically compliant suppressed-console plan. V9 is preserved as invalid
calibration/cost evidence: it neither advances nor eliminates any candidate.

Because the raw grader rejected the entire cheap pool, V10 made one strong-anchor attempt
with exact model `openrouter/anthropic/claude-sonnet-5`, medium effort, Anthropic
only, and fallbacks disabled. The model returned Markdown-fenced JSON instead
of the required bare schema-bound JSON. The executor sealed the trace and
halted before grading. This was candidate response-format noncompliance, not a
provider transport failure. The grant also retained the stale T12 scope. There
was one provider call, no retry, 4,373 reported
tokens, and `$0.019437` spend. The anchor is `NOT_SCORED`; its failed output was
not normalized or post-hoc graded. V10 is invalid evidence and T11 requires
independent recalibration before any further paid use.

Immutable results: `screening_batch_openrouter_v8_result.json`,
`screening_batch_openrouter_v9_result.json`, and
`screening_batch_openrouter_v10_result.json`. Evidence roots are
`.qualification-evidence/openrouter-v8`, `openrouter-v9`, and `openrouter-v10`.

## Trial results

The trials were representative role-family gates, not a claim that every model
ran all 19 architecture scenarios. The permanent 19-case suite remains the
promotion and regression benchmark.

| Case family | Configuration | Evidence | Result | Measured tokens* |
|---|---|---|---|---:|
| Cross-boundary | Luna medium | SILV-15 | fail: incomplete schema and blast/evidence contract | 19,918 |
| Cross-boundary | Terra medium | SILV-16 | fail: incomplete schema and blast/evidence contract | 20,921 |
| Cross-boundary | Sol high | SILV-17 | pass | 35,661 |
| Owner interface | Luna medium | SILV-18 | pass | 15,353 |
| Owner interface | Terra medium | SILV-19 | fail: wrong contract | 17,769 |
| Owner interface | Sol high | SILV-20 | pass, but more tokens than Luna | 19,249 |
| Quality Checker | Terra high | SILV-24 | fail: correction field was not typed as an array | 6,791 |
| Quality Checker | Sol xhigh | SILV-25 | pass | 19,438 |
| Quality Checker | Terra xhigh | SILV-28 | pass and selected for family independence | 7,943 |
| Bounded builder | Luna high | SILV-23 | fail: wrong contract and pushed technical decisions to owner | 1,990 |
| Bounded builder | Sol high | SILV-27 | pass | 8,445 |
| Owner UI decision | Luna medium | SILV-30 | pass | 9,556 |
| Product & UI | Sol high | SILV-31 | pass | 8,071 |
| Product & UI | Claude Sonnet high | SILV-29 | not evaluated: provider weekly limit | n/a |

\* Measured tokens are the valid full-context run's input plus output fields.
The local run API did not report a cached-input field. Issue creation also
caused contextless intake/root runs on several trials. They are not competence
evidence for the isolated role, but excluding their real product cost made the
comparison incomplete. A future isolated capability trial must avoid that
launch path; an end-to-end system trial must bind and count every such run in
the same observation family. Neither comparison is cost-baseline eligible until
the run manifest and all usage fields are complete.

## Qualification boundaries

- No tested configuration is yet labelled `qualified_bounded`. Representative
  trial success selects a provisional shadow candidate; the locked applicable
  cases and required repeats still control promotion.
- `provisional_shadow_only` may reason and produce receipts but may not execute.
- `held_pending_qualification` records an intentional role/configuration pair
  without pretending it has competence evidence; it may not execute.
- `held_external_qualification` prevents Windows from approving a Mac-only
  configuration.
- Automatic disqualifiers outrank token cost: unsafe scope expansion,
  fabricated evidence, owner-decision laundering, wrong-host execution,
  self-review, and unverified completion cannot be offset by being cheaper.
- High-risk review checks agent ID, configuration ID, candidate mutability, and
  model-family independence. A deterministic substitute is required if the
  model family cannot differ.

## Promotion rule

Before any configuration gains production execution permission, it must pass
the applicable mandatory cases repeatedly, produce exact typed receipts, stay
within token and owner-interruption budgets, and pass independent review. Model
or prompt changes create a new configuration ID and require requalification.

### T11 v11 halted by a budget-control defect

T11 v4 replaced free-text answer fields with an independently accepted,
typed-only contract before any candidate ran. V11 then tested each of the five
T12 survivors exactly once with corrected one-observation grants. There were no
retries, tools, repository access, production action, messages, Browser work,
or Phone Farm host access.

| Model and effort | Result | Failure | Tokens | Receipt cost | Time | Advances |
|---|---|---|---:|---:|---:|---|
| Qwen3 Coder Flash, default | PASS before halt | - | 2,488 | $0.000644 | 1.816 s | Yes |
| Ling 3.0 Flash, medium | PASS before halt | - | 3,571 | $0.000123 | 8.738 s | Yes |
| GLM 5.3 Flash, low | PASS before halt | - | 2,482 | $0.000446 | 0.646 s | Yes |
| Nemotron 3 Super Free, low | FAIL | returned an extended essay instead of bare JSON | 11,798 | $0.000000 | 74.665 s | No |
| MiniMax M2.7, default | not scored | launched after the required halt | 3,110 | $0.001704 | 17.535 s | No |

Nemotron's completed call was sealed offline as
`invalid_response_format`; its text was neither repaired nor retried. Its
9,397 generated-token component exceeded the declared 8,000-token hard cap,
which required a batch halt. The controller incorrectly launched MiniMax next,
so MiniMax's V11 observation is not valid. The three PASS observations sealed
before the halt remain valid, and Nemotron's candidate failure eliminates it.
The admission gate now checks
the transport's numeric generated-token ceiling before another assignment can
launch, with a regression test for this exact failure.

The authoritative account delta was `$0.001211149`; rounded call receipts total
`$0.002917`. OpenRouter's export totals sum to 22,052 tokens while normalized
receipt components sum to 23,449; both values and the discrepancies are retained.

Immutable result: `screening_batch_openrouter_v11_result.json`; raw evidence is
under `.qualification-evidence/openrouter-v11`. MiniMax requires a fresh batch;
no other V11 candidate should be repeated.
