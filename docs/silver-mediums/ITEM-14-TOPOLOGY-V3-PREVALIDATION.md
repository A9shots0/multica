# Item 14 topology-v3 prevalidation

Status: `pending_independent_review`. Provider calls are disabled. This batch does not select roles.

Topology-v3 is a fresh rerun of only the invalid R03 topology comparison. It preserves topology-v2 code, freeze bytes, and evidence. It also preserves the R03 v2 fixture and grader, Claude Sonnet 5 Medium through Anthropic, prompts, stop policy, one-correction ceiling, no fallback, and no retry.

The only experimental correction is budget geometry:

- Each reviewer stage reserves 16,000 total tokens instead of 12,000. The observed 12,113-token reviewer response now fits without changing the model or prompt.
- The reviewed arm reserves 130,000 tokens. Its maximum substages require 128,000: 60,000 execute + 16,000 first review + 36,000 correction + 16,000 final review.
- The campaign reserves 410,000 tokens exactly: two 75,000 direct arms + two 130,000 reviewed arms.
- Generated-token, cost, call, correction, and stop ceilings are unchanged.

The four new observation identities are counterbalanced and write only beneath `.qualification-evidence/topology-v3`:

1. `v3-r03-p1-direct`
2. `v3-r03-p1-reviewed`
3. `v3-r03-p2-reviewed`
4. `v3-r03-p2-direct`

Activation requires one independent review and an atomic experiment/freeze status change. Until then, `cmd/topology-run-v3 --plan` is provider-free and `--execute` fails closed.
