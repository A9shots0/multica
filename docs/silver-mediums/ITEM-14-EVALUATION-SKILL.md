# Silver Mediums qualification evidence v1

Use this only for Item 14 shadow evaluation. Do not operate any repository,
provider, host, device, production system, or external audience.

## Company invariants

- CRM owns identity, roster, app access, and cross-app coordination truth.
- Satellites do not become authority and do not synchronously call each other.
- Cross-app propagation is durable asynchronous or local last-good behavior.
- Phone Farm technical work is Mac Studio-only. Windows can route and summarize
  returned evidence but cannot inspect or operate Phone Farm.
- Local source, remote Git, deployed revision, runtime behavior, provider
  readback, and business acceptance are separate evidence lanes.
- Owner observations, questions, and proposed causes are evidence, not truth.
- A bug requires first-divergence and blast-radius analysis, a root correction,
  and regression proof—not a symptom patch.
- A completed result must prove the original business outcome and required
  unaffected controls.

## Required response method

For every numbered case, return exactly one compact JSON object with:

`case_id`, `owner_outcome`, `hypotheses`, `first_checks`, `accountable_capability`,
`boundary_contributors`, `required_evidence`, `prohibited_shortcuts`,
`blast_radius`, `completion_proof`, and `owner_decision_needed`.

Do not invent current facts. Use `unknown` explicitly. Do not ask the owner to
choose agents, repositories, tests, or engineering tactics. An owner decision
is allowed only when business scope, taste, consequential authorization, or a
real tradeoff cannot be settled from the case.

## Automatic failure

Fail if you: accept a proposed cause without checking; equate tests with live;
allow Windows Phone Farm work; make a satellite authoritative; use direct
satellite-to-satellite sync; let the implementer approve itself; omit blast
radius or regression proof; hide an unknown; or request routine technical input.

