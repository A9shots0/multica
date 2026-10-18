# Item 14 — Claude subscription token-efficient R03 v37 prevalidation

Status: `frozen_reviewed`

Evidence stage: `local-only`; provider calls during preparation: **zero**.

## Purpose

V37 tests whether Claude can solve the unchanged frozen `R03:v2` realistic release-regression case with materially fewer paid turns than v36. It does not select a role or production configuration.

The candidate remains `claude-sonnet-4-6`, Medium effort, through the proven Claude Code subscription restricted tool-free stdin JSON-schema lane. Claude has no native tools. It returns only bounded ordered plans; the deterministic frozen controller performs allowlisted synthetic workspace actions.

## Bounded phased design

- At most four ordered actions per paid phase.
- At most five paid phases and twenty total actions.
- Older completed phases are represented by a compact ledger containing only phase number, action count, result hash, and workspace-tree hash.
- Only the immediately prior phase's complete tool outputs are supplied to the next phase.
- A paid phase plan and its native provider receipt are checkpointed before action zero.
- A process crash resumes the remaining actions from that checkpoint without a new provider invocation. Claude `--resume` is not used.
- Zero provider retries, fallbacks, owner interventions, native tools, Browser actions, deployment, messages, Phone Farm access, or other external effects.

The unchanged v36 R03 limits remain the outer resource ceiling: 75,000 total reported tokens, 20,000 generated tokens, and 200,000 cost-equivalent micros. V37 adds tighter structural ceilings of five provider invocations, four actions per phase, and twenty actions total.

## Validation prepared

Focused adversarial coverage proves:

- the exact R03-only route, caps, topology, and pending-review lock;
- restricted tool-free, MCP-free, session-free requests with no Claude resume lane;
- hash-only cumulative history plus full immediately-prior phase results;
- rejection of excess actions, undisclosed paths, non-allowlisted checks, external action kinds, misplaced finish, and a fifth phase without finish;
- exact native provider-receipt replay and accounting-tamper rejection;
- action-level checkpoint recovery without duplicating the paid provider call; and
- checked action/provider/token/cost accounting.

The existing frozen realistic fixture, semantic grader, independent evidence derivation, and v36 evidence remain unchanged. The v37 native sealer replays every provider receipt and applies the existing R03 independent grading contract.

## Consolidated independent NO-GO correction

One consolidated correction pass addressed the six review blockers without changing R03, its grader, or any older harness:

- `release/receipt.json` and `EVIDENCE.md` are the only absent frozen R03 editable paths that may be created; reads remain existing-only.
- The executed plan must equal the structured plan reparsed from the raw Claude envelope inside the immutable provider receipt.
- Every next-phase request is reconstructed from persisted controller state. Completed result slices, prior-phase full output, cumulative hashes/counts, and the current workspace tree must reconcile before admission.
- The command owns a write-once pre-call claim, durable request/in-flight marker, write-once paid phase artifact, receipt-and-plan checkpoint before action zero, atomic action checkpoints, and provider-free restart of remaining paid actions.
- An in-flight call without a durable receipt is closed invalid with the full R03 resource envelope reserved; it is never called again.
- Candidate competence, protocol, and resource failures eliminate only the candidate. Credential, unsafe action, evidence corruption, and campaign-cap failures halt globally. Invalid, failed, and passing observations all receive write-once result records.
- The pending-review lock compares exact fixed SHA-256 bindings for every command source and this prevalidation document, rather than accepting any syntactically valid hash.

## Review and execution boundary

The definition and immutable execution freeze bind the exact new v37 source, focused tests, command lifecycle/provider, document, route, order, caps, and unchanged antecedents by SHA-256. The completed independent review returned NO-GO, one consolidated correction addressed all six findings, and the root accepted that corrected packet based on the review plus passing focused tests and vet. Per the accepted process, no second review cycle was started.

The freeze is now `review_outcome: accepted_after_one_consolidated_correction_no_second_review` and `provider_calls_allowed: true`. This promotion performed no provider call; it only enables the separately capped command for the later execution step.
