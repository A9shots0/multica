# Role qualification execution v6 prevalidation

Status: frozen, provider locked pending a fresh v6 preflight. No v6 provider calls were made while building or validating this correction.

## Narrow correction

V5 is preserved byte-for-byte as historical evidence. V6 changes only the provider normalization boundary that mislabeled OpenCode's documented parser success value:

- `structured` is the only successful OpenCode outcome and its typed structured output is retained.
- `completed` is rejected as a success sentinel.
- `structured_output_error` remains a zero-retry candidate-local `response_or_tool_protocol_failure`.
- Every accepted OpenCode parser outcome first seals the exact request capsule and complete HTTP response bytes with SHA-256. The normalized receipt is derived from those bytes, and replay reparses them before accepting the grade or accounting.
- A crash after the raw HTTP seal is recovered provider-free; the paid turn is never repeated.
- The Claude Code subscription class is held incomplete. V5's multi-model `modelUsage` route evidence remains invalid and V6 performs no Claude preflight or call until a separate route-accounting correction exists.

The frozen v1 cases, hidden graders, 36-cell plan, role bindings, caps, stop policy, one-call limit, zero retries, and zero fallbacks are unchanged. V6 does not select a role, model, squad, topology, permission, or production configuration.

## Provider-free proof

Focused tests cover the exact `completed` versus `structured` regression, structured-output preservation, structured-error classification, raw HTTP/capsule tamper rejection, replay derivation, and the Claude pre-call hold. The atomic freeze binds all v6 command/source/test/doc bytes and the V33 parser source used for rederivation.

