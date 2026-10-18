# Item 14 — v32 protocol-ablation prevalidation draft

Status: historical prelaunch record only. The completed v32 batch was later invalidated by a shared schema-delivery defect; see `ITEM-14-PROTOCOL-ABLATION-V32-RESULT.md`.

Freeze SHA-256: `8a8f19d766ec229d5d07d300d157a84305f7b8b5cf3f3e3cdeb099bb0bffd77a`.

The independent review returned NO-GO on the first draft. This document records the single consolidated correction pass; no second reviewer loop is planned.

## Question

Does the controller’s response protocol materially change model performance on the same bounded owner-decision continuity work?

This is calibration evidence only. One observation per arm cannot establish causality or reliability. Counterbalanced repeats remain deferred until the calibration result shows they are worth buying.

## Controlled comparison

- Case identity: `P32/v1`, newly identified for this experiment.
- Business facts, start fixture, editable operations, allowlisted checks, grader, evidence semantics, and failure classes: unchanged from corrected R05 v1.
- Candidates: Claude Sonnet 5 Medium and GPT-5.4 Medium through the same isolated OpenCode → OpenRouter transport, each pinned to its exact provider with fallbacks disabled.
- Arm A: current single bounded action per response, maximum 10 provider turns.
- Arm B: one provider response containing a maximum-10-step conditional action program. Each step declares success and failure branches; the deterministic controller still executes and observes every action.
- Both arms retain the same 20,000 generated-token ceiling and 75,000 total-token reservation.
- Every first call receives the same canonical JSON packet byte-for-byte: task, required fixture contents, editable and immutable paths, allowed actions and checks, acceptance boundary, fixture-tree hash, and grader identity. The exact bytes and SHA-256 are emitted in every plan. Only the response schema and protocol instruction differ.
- Order is explicitly counterbalanced: Claude A then B; GPT B then A.

The initial six-turn proposal was invalid: unchanged R05 requires at least eight turns (policy repair, two harness checks, three canonical documents, evidence, and finish). Ten turns preserves the exact acceptance contract and adds two bounded inspection/recovery turns. No grader or case requirement was weakened.

## Hard caps

- Four total observations: two models × two protocols × one observation.
- No retries, fallbacks, Browser, production, outbound messages, owner interruption, or Phone Farm access.
- Combined maximum: 300,000 tokens, 1,805,000 micros ($1.805), and 22 provider turns.
- Cost cap is derived from pinned input/generated prices and each observation’s full token reservation.
- Pending recovery claims halt as ambiguous; sealed completed observations are reused without another call.
- Existing global halts are honored before admission. Exact frozen binding membership is required, and worst-case cost is admitted before a durable pending claim is written.
- A preserved zero-call attempt exposed that the per-binding workspace parent was not created. The corrected executor now creates and fully prepares the disposable workspace before writing the exclusive claim or pending ledger. A local setup failure therefore makes no provider call and strands no claim.
- Negative usage is rejected. An actual cap breach is preserved as a completed invalid observation, fully accounted, then globally halted.
- `test_invalidity` is the canonical failure class; the legacy `invalid_test` spelling is normalized on ingestion.
- An observation-scoped parser invalidity is preserved and unscored without halting unrelated bindings. Global test invalidity, unsafe execution, credential failure, budget breach, ambiguous transport, or evidence corruption still halt.

## Executable path and evidence

- `cmd/protocol-ablation-v32 execute <artifact-root>` is wired and unlocks only while the exact reviewed definition, freeze manifest, local sources, command sources, R05 fixture, and grader hashes match.
- Execution uses a fresh synthetic R05 workspace per observation and the accepted isolated, deny-all OpenCode/OpenRouter route. Provider fallbacks are disabled and one upstream provider is pinned.
- Before calls, the command verifies authoritative OpenRouter spend, the resolved OpenCode configuration, exact model, effort, provider route, and the durable ledger.
- Both response interfaces have strict parsers. The loop executes one bounded action at a time; the plan arm executes its conditional branches through the same deterministic action executor and unchanged grader.
- Raw JSONL/export receipts are sealed write-once. Evidence hashes are recomputed from provider identity, raw receipts, executed actions, and grader result rather than trusted from model output.
- Direct transport provider (`openrouter`) is validated separately from the route-stripped upstream model (`anthropic/...` or `openai/...`); serving-provider evidence remains separately required and fallbacks remain disabled.
- The reviewed R05 fixture and grader sources are hash-bound, and the provider-free plan emits hashes for every v32 Go/JSON/test source. v30/v31 command sources remain unchanged.

## Preserved harness-invalid attempts

- `protocol-ablation-v32`: zero provider calls; local workspace-parent setup defect. Preserved unchanged.
- `protocol-ablation-v32-v2`: exactly one Claude loop call. OpenCode session `ses_f65cc7190ffeCTv632lIB62CN5` reported direct provider `openrouter`, upstream model `anthropic/claude-sonnet-5`, effort `medium`, 4,686 input/cache tokens, 383 output tokens, 218 reasoning tokens, and 17,724 micros. The observation is invalid/unscored because the old parser compared the split identity against the route-qualified model string.
- The v2 ledger was not rewritten. Provider-free recovery sealed the exact local session export and a separate observation record under `recovered-observations/`; `provider_calls` for recovery is zero and the original ledger SHA-256 is retained.
- Execution root `.qualification-evidence/protocol-ablation-v32-v3` was atomically initialized from three exact v2 hashes: original ledger, recovered session export, and recovered observation record. Its sole completed claim is the invalid Claude one-action cell and its ledger accounts the real 1 turn, 4,686 input/cache, 383 output, 218 reasoning, and 17,724 micros. Ordinary execution reuses that claim, so the only remaining launch order is Claude one-shot, GPT one-shot, then GPT one-action.
- The v3 Claude one-shot made one call and returned Markdown instead of the required conditional-plan JSON. Its presealed receipt proves the exact OpenRouter route, upstream model, medium effort, frozen prompt/schema, and a resolved configuration enforcing one Anthropic route with fallbacks disabled. Because OpenCode emitted no independent selected-provider field, the truthful evidence mode is `enforced_single_route_no_fallback`; contradictory nonempty provider metadata remains rejected. The cell is a valid local `response_or_tool_protocol_failure`, not a transport failure, and was not retried.
- `.qualification-evidence/protocol-ablation-v32-v4` was atomically seeded with both consumed Claude cells. Total carried usage is 2 turns, 9,381 input/cache, 1,745 output, 487 reasoning, and 45,771 micros. Ordinary execution now launches only GPT one-shot followed by GPT one-action.

## Interpretation

Two views must be reported separately after execution:

1. Equal-resource: compare each arm under the identical per-observation token and spend reservation.
2. Capability-normalized: account for the fact that the loop may spend up to ten provider turns while the conditional plan spends one.

No result from this calibration selects agents or production roles. The atomic freeze records the completed independent GO review. A post-batch audit subsequently proved the schema was hash-bound but absent from the executable prompt, while CLI `--format json` only selected raw event output. The four resulting observations are invalid/unscored and are preserved without revision.
