# Item 14 measurement integrity result

Date: 2026-09-11  
Evidence stage: `local-only`; no paid model run, production action, browser
change, or Phone Farm operation

## Result

The evaluation harness now fails closed unless it can prove the complete paid
run family and its usage. This makes future single-agent versus controller-led
comparisons measurable without pretending omitted intake, retry, reviewer, or
failed runs were free.

Implemented proof:

- immutable, versioned observation and configuration identities;
- pre-execution and final run-manifest digests;
- a separately hashed, closed authoritative controller run ledger;
- exact reconciliation among ledger runs, manifests, and receipts;
- complete fresh-input, cached-input, output, and run-time measurements;
- separate end-to-end elapsed time and summed run compute time;
- duplicate observation and duplicate attempt rejection for reliability;
- cost accounting for failed, invalid, and inconclusive paid work;
- a batch token ceiling with a required reservation before the next launch;
- fail-closed behavior when any authoritative run lacks a usable receipt.

## Verification

- `go test ./internal/agentqualification -count=1` — pass
- `go test ./internal/systemgraph ./internal/agentqualification ./internal/handler ./cmd/server -count=1` — pass
- `git diff --check` on the Item 14 measurement and documentation files — pass

The single independent review returned `REVISE` with four initial P1 findings.
One consolidated correction addressed all four. Its blocker-only terminal check
found one exact receipt-reconciliation gap, which was corrected and locked by
`TestBatchUsageFailsClosedWhenAuthoritativeRunHasNoReceipt`. Per the bounded
review policy, no continuous reviewer loop was started.

## Boundary

Exactly zero configurations are production-qualified. No paid screening batch
has run. The next action is to define and project the smallest capped screening
batch; passing this gate does not authorize an uncapped tournament.
