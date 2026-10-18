# Item 14 topology-v4 prevalidation

Status: `pending_independent_review`. Provider calls are disabled. This group
does not select a topology, model, role, or squad.

Topology-v4 is a cross-model replication of the corrected topology-v3 R03
comparison. It preserves topology-v1 through topology-v3 sources, freezes, and
evidence. It does not change the R03 v2 fixture, grader, prompts, topology
logic, tool-free review contract, one-correction ceiling, no-retry policy, or
no-fallback policy.

The only planned comparison change is the exact matched model configuration:

- executor: `realistic.gpt54.medium.anchor.r04.v8`,
  `openrouter/openai/gpt-5.4`, Medium, serving provider `openai`;
- reviewer: fresh identity
  `realistic.gpt54.medium.reviewer.topology-v4`, bound to the same exact model,
  effort, provider, and tool-free review contract.

The four fresh observations are counterbalanced and write only beneath
`.qualification-evidence/topology-v4`:

1. `v4-r03-p1-direct`
2. `v4-r03-p1-reviewed`
3. `v4-r03-p2-reviewed`
4. `v4-r03-p2-direct`

Budget geometry matches the validated v3 geometry: 75,000 tokens per Direct
arm, 130,000 per Reviewed arm, 16,000 per review stage, 410,000 campaign
tokens, and a $1.80 incremental campaign ceiling. Reviewed permits at most one
correction and two review stages. The transport makes no retries and allows no
fallbacks.

Activation requires one independent review and an atomic experiment/freeze
status change. Until then, `cmd/topology-run-v4 --plan` is provider-free and
`--execute` fails closed.

V4 admission also reads and hashes the actual frozen command sources for the
preserved v3, v2, and v1 chain. A nested command-source change at any generation
blocks execution even when its manifest and embedded package components still
validate. Adversarial tests cover direct v2 and v1 source drift.
