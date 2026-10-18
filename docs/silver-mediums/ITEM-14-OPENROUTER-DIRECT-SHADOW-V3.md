# Item 14 OpenRouter direct shadow v3

Status: terminal fail-closed observation sealed; do not rerun

Snapshot: 2026-09-14 13:58 America/New_York

V3 is the smallest correction justified by the terminal zero-inference V2
receipt. It keeps the same eligible temporary Qwen/Alibaba route and all exact
model/provider, attempt, tool, retry, redirect, budget, and evidence gates. It
removes only `response_format` and `require_parameters`, which Alibaba rejected
before inference, and moves the unchanged strict response validation entirely
into the controller after the provider responds.

Frozen bindings:

- Definition SHA-256:
  `b3241a97ac57418868c71059a6b443754e61069405025d0f3da56ad913372c5c`
- V2 definition antecedent:
  `fbe314bc6d779d0ad57bb6150f200e7a9db6af8e9a7c0b4ca6e814007c9441b4`
- V2 terminal response antecedent:
  `611dfac19c079fcd560ebf89f042049646a53312b0b9eac2bc14b4991aa04aa6`
- Frozen suite:
  `9c8803d41b12b99fa7286ecd379d82ca3f2497bae8f181f1d4c524e8912e32be`
- Requested model: `openrouter/qwen/qwen3-coder-next`
- API alias: `qwen/qwen3-coder-next`
- Required selected/attempted endpoint model:
  `qwen/qwen3-coder-next-2025-02-03`
- Required serving provider: `alibaba`

The request still contains exact provider `order` and `only`, disables
fallbacks, and contains no tools, plugins, fallback models, repositories, or
external actions. Router metadata must prove exactly one selected Alibaba
endpoint and exactly one successful Alibaba attempt of the frozen model
revision. Non-selected inventory may be present but cannot be selected or
attempted. A response-contract miss remains candidate-local so the other
workflows continue; identity/provider/evidence ambiguity remains a global
no-retry halt.

Plan, fake-provider execution, prepare-only, focused mutation tests, and the
post-review correction passed without credential access or real calls. The
correction explicitly distinguished non-selected router inventory from the
single selected/attempted route and rechecked price-reservation arithmetic.

The exact V3 command then ran once. One direct request completed through
OpenRouter and returned:

- requested and actual alias `qwen/qwen3-coder-next`;
- top-level provider `Alibaba`;
- one selected Alibaba endpoint with model
  `qwen/qwen3-coder-next-2025-02-03`;
- router attempt value `1`, one provider inference, and zero retry;
- 321 input tokens, 268 output tokens, no reasoning tokens, and receipt cost
  `$0.0004983` (`499` normalized micros);
- response SHA-256
  `52e3ebb36be48988d3eaef1db17c20e73551173e5618e2338479066a2226b98e`.

This positively proves the exact requested model alias and selected serving
provider/endpoint for that one request. It does not qualify the candidate. The
successful response omitted the documented `attempts` array, so the V3
harness failed closed before grading. Independent inspection also showed the
model response failed the frozen field contract: it added unrequested access
fields and omitted required planned-access booleans. The account endpoint
reported the same 111,582,154-micro snapshot before and immediately after;
the provider receipt's 499-micro charge is preserved separately rather than
being erased by that apparent accounting lag.

Provider-free reconciliation sealed the V3 terminal and accounting receipts.
V3 authority is exhausted, all three later workflows remain uncalled, and V3
must never be rerun. The separately versioned V4 continuation binds this
terminal response, begins with only those three unattempted workflows, and
cannot qualify the candidate because the antecedent workflow already failed.
