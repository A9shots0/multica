# Item 14 realistic residual batch v31 — prevalidation

Date: 2026-09-13

Status: frozen and provider-ready. No v31 model call had occurred when this
receipt was written.

## Scope

V31 observes only the GPT-5.4 Medium binding that remained unseen when v30
halted. It reuses the exact frozen R05 and R06 fixtures, graders, harness, and
transport admitted by v30 freeze
`cca629d86d79666cc29dae7815e5226430f821adc0a9c5098c974c44598f0f77`.
Claude v30 is not retried.

- Model: `openrouter/openai/gpt-5.4`.
- Requested upstream provider: `openai`.
- Effort: `medium`.
- R05 first; R06 only after a valid R05 pass.
- Zero retries and no provider fallback.
- Campaign cap: $1.00 and 150,000 total tokens.
- Observation reservation: 75,000 combined tokens, including a 20,000
  combined output-plus-reasoning ceiling.
- Conservative exact-route admission reservation: $0.6875.

## One independent review and correction

The one independent review returned NO-GO with four bounded defects. The one
consolidated correction added a real separately hash-bound CLI path, made all
credential/runtime/config/credits/transport failures durable global halts,
hash-bound every v31 matrix/freeze/executor/entry/shim source and test, and
enforced combined token and conservative price admission after every immutable
receipt before another provider call.

Per-observation resource exhaustion is candidate-local. Actual cumulative
campaign cap exhaustion remains global. There was no second reviewer cycle.

## Provider-free proof

- `go test ./internal/agentqualification ./cmd/screening-run ./cmd/realistic-residual-run-v2 -count=1`: PASS.
- `go vet ./internal/agentqualification ./cmd/screening-run ./cmd/realistic-residual-run-v2`: PASS.
- `go run ./cmd/realistic-residual-run-v2 -plan`: two exact bindings,
  provider ready, provider called false, no fallback, zero retry.
- V31 freeze SHA-256:
  `2d2b80b0b8eac1bbf5719a9bf671a94574327dd8c83185f16da4503578b5cf2c`.

The batch permits no production, Browser, outbound-message, owner-intervention,
or Phone Farm action and makes no role selection.
