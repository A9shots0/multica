# Item 14 topology-v5 prevalidation

Status: `frozen_reviewed`. Provider calls are enabled by the atomic reviewed
freeze. This group
does not select a topology, model, role, or squad.

Topology-v5 is a clean rerun of the GPT-5.4 topology-v4 R03 comparison after
the v4 reviewed arm exposed a pre-call reviewer-binding defect. It preserves
topology-v1 through topology-v4 sources, freezes, results, and evidence exactly.
It does not change the R03 v2 fixture, grader, prompts, topology
logic, tool-free review contract, one-correction ceiling, no-retry policy, or
no-fallback policy.

The only planned comparison change is the exact matched model configuration:

- executor: `realistic.gpt54.medium.anchor.r04.v8`,
  `openrouter/openai/gpt-5.4`, Medium, serving provider `openai`;
- reviewer: fresh identity
  `realistic.gpt54.medium.reviewer.topology-v5`, bound to the same exact model,
  effort, provider, and tool-free review contract.

The four fresh observations are counterbalanced and write only beneath
`.qualification-evidence/topology-v5`:

1. `v5-r03-p1-direct`
2. `v5-r03-p1-reviewed`
3. `v5-r03-p2-reviewed`
4. `v5-r03-p2-direct`

Budget geometry matches the validated v3 geometry: 75,000 tokens per Direct
arm, 130,000 per Reviewed arm, 16,000 per review stage, 410,000 campaign
tokens, and a $1.80 incremental campaign ceiling. Reviewed permits at most one
correction and two review stages. The transport makes no retries and allows no
fallbacks.

The required independent review returned one bounded NO-GO. Its single
consolidated correction was applied without a second reviewer loop, then the
experiment and freeze were activated atomically. `cmd/topology-run-v5 --plan`
remains provider-free; only an explicit exact-next `--execute` may call the
provider.

V5 fixes only two harness boundaries before the new independent review: the
shared review-plan builder now validates against the reviewer frozen by the
calling experiment generation, and an executor response/tool protocol failure
is recorded as a candidate-local failed observation before any reviewer call.
A correction-stage response/tool protocol failure likewise stops before the
second review. A provider budget breach remains a campaign-wide uncontrolled-
spending halt.
It does not become evidence corruption or halt unrelated observations. True
hash, fixture, receipt, or matched-pair evidence corruption remains a global
halt.

V5 admission also reads and hashes the actual frozen command sources for the
preserved v4, v3, v2, and v1 chain. A nested command-source change at any generation
blocks execution even when its manifest and embedded package components still
validate. Adversarial tests cover direct v2 and v1 source drift.
