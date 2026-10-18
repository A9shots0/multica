# Item 14 — Claude subscription realistic R03/R04 v36 prevalidation

Status: `frozen_after_one_consolidated_correction`

Evidence stage: `local-only`

Provider calls during preparation/correction: **zero**. The independent review returned HOLD. One consolidated correction implemented only its pending-claim, replay-integrity, and checked-accounting findings; no second reviewer cycle was run. The corrected source is bound by the immutable v36 freeze before execution is enabled.

## Purpose and exact experiment

V36 is the next realistic Claude Code subscription batch. It measures the already-frozen realistic cases without changing their fixtures, graders, controller, or expected evidence:

1. `R03:v2` — release regression, safe simulated rollback, permanent prevention, historical journey verification, and truthful open outcome.
2. `R04:v1` — forced interruption followed by exactly-once continuation from durable workspace truth.

The topology is only `candidate decision -> deterministic controller`. There is no reviewer arm, correction arm, Mika, production action, Browser action, external message, or Phone Farm action.

The candidate is `claude-sonnet-4-6`, Medium effort, Claude Code subscription, Anthropic serving boundary, using the proven non-bare restricted stdin JSON-schema lane. Claude receives no tools. It returns one structured action; only the frozen controller may execute an allowlisted synthetic workspace action.

## Frozen case and validation binding

The definition binds the unchanged realistic group, fixture manifest, foundation freeze, controller, runner, verifier, receipt/declaration sources, adversarial suites, Claude transport/receipt parser, and the accepted v35 transport freeze by SHA-256. The bound realistic sources remain unmodified.

Prepared definition SHA-256: `572a649ad9e65d9ab3106aeeaf941d8e373322aba7befed686b6861ebf66eb44`

The immutable freeze binds the exact corrected adapter, native sealer, lifecycle, provider adapter, tests, this document, unchanged definition, unchanged frozen antecedents, route, case order, and caps. The prepared definition remains immutable at the SHA above.

## Consolidated HOLD correction

- A durable `pending` claim is never skipped or resumed with a duplicate provider call. Checkpoint and available provider-turn receipts are validated, known usage is retained, the entire remaining case envelope is conservatively reserved, the observation is closed invalid, and the campaign halts before R04 can be admitted after an unresolved R03 claim.
- Explicit replay and every next-case admission reject pending claims; recompute aggregate usage from claims; compare the sealed result claim and grade to the ledger; regenerate and verify the execution receipt and semantic grade; bind provider-turn file order, hashes, request JSON, request hashes, request IDs, route, prompt/schema, and controller accounting; and enforce the exact R04 attempt history.
- All usage accumulation and cap admission use checked integer arithmetic. Negative or overflowing usage is evidence corruption, never a wrapped low value that could pass a budget gate.

## Exact gates and caps

- R03 PASS runs R04.
- R03 candidate competence, protocol, hard-disqualifier, or resource failure eliminates this candidate and skips R04.
- An R03 observation-local transport/harness invalidity preserves R03 as invalid and still runs unaffected R04.
- R04 attempt 1 is a harness-forced durable interruption with zero provider calls. Attempt 2 resumes the same run family exactly once. This is not a retry.
- Zero retries, zero fallbacks, zero owner interventions.

| Boundary | Provider turns | Total reported tokens | Generated tokens | Cost-equivalent micros |
|---|---:|---:|---:|---:|
| R03 | 15 | 75,000 | 20,000 | 200,000 |
| R04 | 15 | 60,000 | 12,000 | 250,000 |
| Campaign | 30 | 135,000 | 32,000 | 450,000 |

Unknown transport usage is marked observation-local invalid and conservatively reserves that case's complete token/generated/cost envelope before the unaffected case can be admitted. A completed response that crosses a case resource ceiling is a candidate-local resource failure. A campaign ceiling, credential failure, unsafe external action, or evidence corruption is global.

## Lifecycle and evidence controls

- atomic canonical ledger and write-once per-case result;
- atomic controller checkpoints after in-flight markers, native provider receipts, action decisions, and deterministic tool results;
- one immutable native Claude receipt per provider turn, binding request, prompt, schema, exact model/effort/provider/transport, raw Claude JSON, and reported usage;
- provider-free replay of ledger identity plus result, controller, execution receipt, grade, runtime, and every native provider receipt before another case is admitted;
- structured Claude credential classification: the real zero-turn signed-out envelope or typed `authentication_error` globally halts; auth-like plain text and model-shaped responses cannot spoof it;
- candidate-local R03 failure skips R04 without turning into a global batch halt;
- observation-local invalidity continues R04 without retrying R03;
- independently derived realistic evidence indexes and hidden semantic grade;
- exact R04 attempt history (`interrupted` attempt 1, `completed` attempt 2) and zero-call first transition.

The native v36 sealer replays the unmodified Claude Code JSON inside every provider receipt and independently recomputes its accounting. It does not translate Claude evidence into a fake OpenCode envelope. The shared semantic verifier is then applied to the same frozen R03/R04 evidence contract.

## Verification completed

From `server/`:

```text
go test ./internal/agentqualification ./cmd/claude-subscription-realistic-v36 -count=1
```

Result: both packages passed, including a complete provider-free scripted R03 PASS -> R04 forced interruption -> exact-once resume -> PASS lifecycle, pending in-flight crash closure with zero duplicate provider calls and no R04 admission, forged ledger/result/controller/receipt/grade rejection, forged R04 attempt-history rejection, checked overflow rejection, local-invalid continuation, candidate-failure gate, resource ceiling, structured credential classification, native receipt replay, and zero-call R04 attempt 1.

```text
go vet ./internal/agentqualification ./cmd/claude-subscription-realistic-v36
```

Result: passed.

```text
go run ./cmd/claude-subscription-realistic-v36
```

Result before freeze: emitted the exact review-locked plan with `provider_called=false`, `provider_calls_allowed=false`, exact R03/R04 order, direct-controller topology, model/effort, and caps.

```text
go run ./cmd/claude-subscription-realistic-v36 -execute -artifact-root C:\Dev\SMK-multica-selfhost-pilot\.qualification-evidence\SHOULD-NOT-EXIST-v36
```

Result before freeze: failed closed; the target directory was not created.

## Freeze decision

The required independent review returned one consolidated HOLD. Its complete bounded correction is now implemented and tested without a second review cycle. The immutable freeze is `claude_subscription_realistic_v36_freeze.json`; execution remains fail-closed unless that freeze and every embedded corrected component validate byte-for-byte.
