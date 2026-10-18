# Item 14 — Claude Code subscription stdin calibration result

Date: 2026-09-13

Status: valid PASS for the affected-only transport calibration. This does not
select a model, role, squad, topology, permission, or production configuration.

## Result

- Runtime: Claude Code `2.1.270`.
- Model and effort: `claude-sonnet-4-6`, Medium.
- Frozen case: T11 v4.
- Transport: restricted, tool-free Claude Code print mode with the prompt
  delivered through stdin.
- Verdict: PASS under deterministic grader `deterministic-t11.v4`.
- Provider invocations: 1; retries: 0.
- Usage: 6,100 input tokens and 980 output tokens. Reasoning-token accounting
  was unavailable from the provider receipt.
- Provider-reported cost-equivalent: 51,295 micros. This is subscription
  accounting metadata, not cash spend for this observation.
- Wall time: 14.719s.

The exact evidence root is
`.qualification-evidence/claude-subscription-v29-stdin-01/f0bcf5256786d96aa08c676b98ea86c61b593fc623feb02d8494e1716b1bfa1a`.
The receipt binds raw result SHA-256
`6b9400c70c380f14996183f973f2368e5b52a0ac800c6acb4929489c7e04b8c5`
and parsed output SHA-256
`e7777d36df33c390f68daa8a8b1a6281d559789ce0ff3b1651681be960d13d0c`.

The original v29 Windows argument-length failure remains immutable invalid
transport evidence. This affected-only stdin observation proves the corrected
Claude Code subscription transport can produce one valid, schema-graded T11
result; it does not inherit or replace OpenRouter evidence and does not by
itself establish reliability.
