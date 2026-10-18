# Role qualification execution v2 prevalidation

Status: provider-free implementation complete; provider gate locked.

- Frozen v1 is inherited by exact SHA-256, without edits.
- The execution plan contains 36 deterministic, counterbalanced cells: three
  exact configuration classes by four model-neutral role bindings by three
  cases.
- Candidate packets contain the role instructions, role skill, scenario,
  referenced artifact IDs, and strict response schema. Hidden graders and
  calibration expectations are not exposed.
- Each cell allows one call, zero retries, and zero fallbacks. Observation and
  campaign token, time, call, and OpenRouter cash caps fail closed. Claude
  subscription cost-equivalent remains separate from marginal cash.
- A claim is sealed before any receipt. Raw receipts and requested/actual
  route identities are immutable. Reconciliation never retries a stranded
  claim, repairs an interrupted ledger seal from the existing receipt and
  observation, and replays all accounting before another grant.
- Candidate failures eliminate only the configuration-class and role-binding
  pair; isolated harness imperfections affect only their cell; the accepted
  global-halt allowlist stops unrelated grants.
- This version exposes only `plan`, `validate`, `status`, and `replay`.
  There is no provider execution command.

The OpenRouter receipt boundary follows the structured v33 loopback,
resolved-route, no-fallback, raw-receipt, and spend-separation contract. The
Claude boundary follows the tool-free subscription, strict structured-output,
single-invocation, and cost-equivalent accounting contract. Fresh runtime
preflight remains required by a later separately authorized execution step.
