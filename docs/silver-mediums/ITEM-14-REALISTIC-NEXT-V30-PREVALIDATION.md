# Item 14 realistic next capability batch v30 — prevalidation

Date: 2026-09-13

Status: frozen and provider-ready; no model call had been made when this receipt
was written.

## Purpose

Measure two existing strong reference configurations on two deliberately
uncovered capabilities without selecting production roles:

- R05: resume an authoritative partial owner decision, reject stale chat,
  avoid repeated work, repair a bounded declarative policy, and reconcile the
  canonical documentation.
- R06: separate directly affected consumers from verification radius, release
  a synthetic cross-repository contract in order, distinguish deployed from
  Live, detect failed authenticated readback, roll back, and keep the requested
  business outcome open.

## Frozen matrix

- `openrouter/anthropic/claude-sonnet-5`, requested provider `anthropic`,
  effort `medium`.
- `openrouter/openai/gpt-5.4`, requested provider `openai`, effort `medium`.
- R05 runs first for both. Only a valid R05 pass may continue to R06.
- Zero retries and no provider fallbacks.
- Maximum incremental spend: $3.00; maximum total tokens: 300,000.
- Per observation: 75,000 total-token reservation and 20,000 combined
  generated-token ceiling.
- Exact price-derived worst-case reservations are checked before each paid
  observation: $0.465 for Claude and $0.4375 for GPT-5.4.

## Independent review and consolidated correction

The one independent complete review found four launch defects. The one allowed
consolidated correction:

1. Replaced candidate-editable host Python in R05 with a Go-owned deterministic
   simulator over declarative JSON.
2. Bound R05 pass status to harness-owned successful apply/verify receipts and
   immutable input, policy, and final-state hashes; candidate-authored evidence
   cannot manufacture a pass.
3. Added deterministic run identities, durable pre-call claims, fail-closed
   ambiguous restart handling, completed-observation reuse, and reconstructed
   campaign accounting so a restart cannot duplicate a paid call.
4. Replaced the fixed $0.25 admission guess with exact frozen-model worst-case
   price reservations that cannot admit work beyond the $3 campaign ceiling.

No second reviewer loop ran.

## Provider-free proof

- `go test ./internal/agentqualification ./cmd/screening-run -count=1`: PASS.
- `go vet ./internal/agentqualification ./cmd/screening-run`: PASS.
- `go run ./cmd/screening-run -realistic-next-plan`: four exact bindings,
  provider ready, provider called false, no fallbacks, zero retries.
- Freeze SHA-256:
  `cca629d86d79666cc29dae7815e5226430f821adc0a9c5098c974c44598f0f77`.

The full repository test suite was not used as this batch gate because the
independent review observed existing unrelated migration cleanup failures.
No production, Browser, outbound-message, owner-intervention, or Phone Farm
action is permitted by this synthetic batch.
