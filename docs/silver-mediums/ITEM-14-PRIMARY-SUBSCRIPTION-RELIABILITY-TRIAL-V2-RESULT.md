# Item 14 primary-subscription reliability trial v2 result

Status: complete, independently reviewed, provisional local scope only  
Prepared: 2026-09-14 18:26 -04:00  
Completed: 2026-09-14 18:40 -04:00  
Evidence boundary: disposable synthetic fixtures, subscription CLIs, and one
controller-owned loopback Browser capture; no real repository or production
system

## Outcome

The fresh trial consumed exactly 12 authorized one-shot subscription sessions:
three repetitions of each Claude/Codex by R01/R02 cell. The sealed result is
six PASS, zero INVALID, and six FAILED observations.

| Lane | R01 | R02 | Sessions | Input tokens | Cached input | Output tokens | Time | Cost boundary |
|---|---:|---:|---:|---:|---:|---:|---:|---|
| Claude Code subscription | 3/3 PASS | 0/3 PASS | 6 | 43,484 | 17,080 | 15,550 | 239.440 s | $0.396763 CLI API-equivalent estimate |
| Codex subscription | 3/3 PASS | 0/3 PASS | 6 | 56,475 | 0 | 5,362 | 151.585 s | no provider cash-cost receipt |

Every provider session returned a structurally valid tool-free envelope and
removed its disposable work directory. Five R02 action plans failed during
deterministic controller execution. The sixth reached the in-app Browser,
captured the local report at 390, 820, and 1440 pixels, then failed the hidden
verifier. No failed observation was retried.

## Model attribution

Claude Code was evaluated as the explicitly accepted configuration of
`claude-sonnet-4-6` primary plus possible disclosed background Haiku. All six
receipts in this run happened to report Sonnet only; this does not remove the
Claude Code composite-transport limitation.

All six Codex commands requested `gpt-5.6-luna` at Medium with retries and
tool-bearing features disabled. Codex CLI did not expose trustworthy actual
model identity, so the result remains request-pinned and identity-unavailable.

Neither observation selects a permanent model, provider, role, or seat.

## Provisional decision

The evidence supports only the smallest topology that survived reliably:

- local R01-like work may provisionally use the shared Windows executor with
  the exact architecture-assurance skill attached inline;
- provider/model selection remains unset because Claude and Codex both passed
  R01 3/3 and the Codex actual-model identity is unavailable;
- Product/UI and all R02-like execution remain unqualified;
- production activation, repository use by the participant, credentials,
  outbound actions, Phone Farm access, retries, and fallbacks remain forbidden.

This is reversible local qualification evidence, not production activation or
a permanent staffing decision.

## Integrity evidence

- v2 definition SHA-256:
  `d8bed64fa40603089e532224b98085e8c3410edc1f2069127341cf5be44db693`
- immutable prior v1 result SHA-256:
  `03208af564632ff8a030029c963085f3ffce3c3b1e442714512cacadfabbc41a`
- terminal ledger SHA-256:
  `cea2000f9811d0ae5254ea8c86a94faf793389d9340cf29677ebb3f4478e730c`
- evidence-manifest SHA-256:
  `c81d4c185bca6a01c1fa51670af6c9987e5e9ff55e0f7e569b09204f115f8eb2`
- independent 12-check review SHA-256:
  `f215234e6bf352255e72bef543160aeb6580d82daf385ac793fc825123c0dd8f`
- machine result SHA-256:
  `9f99664568ff701cadddbac537a7548cebfd88fbd2bc7d3d1415176ddb1f1f1a`

The independent reviewer rehashed every raw response, verified all 12
one-call receipts and cleanup attestations, checked the model-attribution and
token ceilings, verified all terminal fixture absences, validated the sole
three-viewport Browser evidence set, recomputed the evidence manifest, and
rejected a copied root after raw-response mutation.

## Safety accounting

Provider calls: 12. Retries, fallbacks, native tool executions, real repository
accesses, production accesses, business actions, Phone Farm accesses, permanent
selections, and post-start corrections: all zero. No additional subscription
or provider call is authorized by this result.

Next: bind the provisional topology into a provider-free local role-stage
contract and rehearse its exact allow and deny surfaces. Continue structural
qualification of the other role, squad, instruction, and skill artifacts
without inferring model qualification or enabling production.
