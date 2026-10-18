# Item 14 fifth screening review

Date: 2026-09-11  
Decision: `NO-GO` for paid execution; corrected plan-only harness accepted

## Independent findings

The single independent review found four paid-execution blockers:

1. the executable bypassed `PrepareNextScreeningLaunch`, so capacity, ledger,
   seeded order, repeat, and prior-failure gates were not enforced;
2. post-hoc tool detection did not prevent effects, inherited environment
   values remained available, and a hardlinked `auth.json` could mutate the
   owner's credential inode;
3. the receipt lacked durable provider/model/runtime and raw-trace provenance;
4. the T12 grader needed positive paraphrase and adversarial negative
   calibration, with scored field intent disclosed to candidates.

The review also required V4 documentation to say seven tools were observed in
that run, not that the transport necessarily required them.

## Consolidated correction

- Removed the paid execution path. `--execute` now always refuses before model,
  credentials, environment, runtime, or provider access.
- Kept V5 as a zero-model plan compiler that pins the seeded candidate, model,
  effort, prompt, and schema.
- Added raw JSONL trace hashing, event counts, and broader tool-capability event
  detection for the future receipt parser.
- Disclosed the semantic purpose of every scored T12 response section.
- Added a valid business-outcome paraphrase fixture and an adversarial response
  that gives only a technical plan; the first passes and the second fails.
- Qualified the V4 causal wording.

## Proof

- `go test ./internal/agentqualification ./cmd/screening-grade ./cmd/screening-run -count=1` — PASS
- `go run ./cmd/screening-run` — PASS, `paid_model_called: false`,
  `paid_execution_enabled: false`, seeded Terra-medium, 5,106-byte prompt,
  hashes matching the V5 batch document
- `go run ./cmd/screening-run --execute` — REFUSED with the admission,
  isolation, and provenance gates named

No V5 model ran. This review does not qualify any model or production role.
The next implementation slice is the missing safe executor, not a paid trial.

## Subsequent zero-cost hardening

The provider-boundary audit found additional ledger defects. They are now
corrected without enabling execution:

- every existing observation is fully validated before another launch;
- invalid/inconclusive observations and automatic disqualifiers stop the batch;
- observations and run receipts share the immutable seeded
  `evaluation_binding`, so a completed assignment cannot be selected again;
- an atomic filesystem reservation is the cross-process compare-and-swap;
  duplicate or crash-partial claims remain ambiguous and block retry;
- subscription percentage and API-dollar capacity use separate schemas;
- the normalized execution receipt requires provider/runtime/model/billing
  identity, admission/config/prompt/schema hashes, timestamps, response ID,
  event manifest, exact pointer-valued usage, immutable trace/output hashes,
  and tool policy; it seals exactly once.

The CLI dry-run no longer states that tools are actually disabled. It reports
the required policy (`none`) and runtime enforcement (`not_implemented`).
