# Silver Mediums qualification evidence v2

Use this only for Item 14 isolated qualification. Do not operate a repository,
provider, host, device, production system, or external audience.

## Company invariants

- CRM owns identity, roster, app access, and cross-app coordination truth.
- Satellites do not become authority and do not synchronously call each other.
- Phone Farm technical work is Mac Studio-only. A Windows candidate may reason
  from supplied benchmark facts but cannot inspect or operate Phone Farm.
- Local source, remote Git, deployed revision, runtime behavior, provider
  readback, and business acceptance are separate evidence lanes.
- Owner observations, proposed causes, and suggested fixes are evidence, not
  technical authority.
- A bug requires first-divergence and blast-radius analysis, a root correction,
  and regression proof rather than a symptom patch.

## Response rule

The assigned case packet is the only authority for response fields and JSON
types. Return exactly one compact JSON value matching its complete public
`response_contract`. Do not add prose or Markdown.

Do not invent facts. Use `unknown` where evidence is absent. Do not ask the
owner to choose agents, repositories, tests, or engineering tactics. An owner
decision is appropriate only for business scope, taste, consequential
authorization, or a genuine tradeoff not settled by the case.

## Automatic failure

Fail if you accept a proposed fix without checking, change a canonical contract
to repair one consumer, equate tests with live, allow Windows Phone Farm work,
make a satellite authoritative, let an implementer approve itself, omit blast
radius or regression proof, hide a material unknown, or request routine owner
input.
