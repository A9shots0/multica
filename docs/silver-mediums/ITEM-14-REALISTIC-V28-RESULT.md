# Item 14 Claude R03 affected reliability result

Date: 2026-09-13  
Evidence stage: sealed synthetic qualification evidence only; no production role,
squad, topology, or permission is selected

## Why this observation ran

Claude Sonnet 5 Medium already had two valid passes on R01 and R04. Its first
R03 reliability repeat was invalid only because the old declaration parser did
not recognize the truthful field form `outcome_status:open`. V28 therefore ran
one affected-only R03 v2 observation after that parser correction. It did not
repeat unaffected cases or substitute the newly available Claude Code
subscription, which would have changed the tested transport configuration.

## Result

| Configuration | Case | Result | Calls | Tokens | Receipt cost | Provider time |
|---|---|---:|---:|---:|---:|---:|
| Claude Sonnet 5 Medium, OpenRouter, requested Anthropic | R03 v2 | PASS | 12 | 49,202 | $0.141414 | 54.953s |

Token detail: 46,747 input, 2,035 output, and 420 reasoning tokens. The run
used zero retries and zero owner interventions. It produced the required local
repair, rollback receipt, restored-baseline proof, current-truth receipt, and
open-successor declaration. The frozen verifier passed it.

The isolated receipt sum is $0.141414. OpenRouter's account reading moved from
$110.383288 to $110.475429, a $0.092141 delta. The $0.049273 difference is
preserved for later accounting audit; neither source was rewritten. OpenCode
proved direct OpenRouter routing, requested Anthropic upstream, and fallbacks
disabled, but did not expose authoritative actual-upstream metadata.

## Interpretation

The missing R03 reliability datum is now validly closed. Across the unchanged
realistic group, this material Claude configuration has an initial valid pass
and a valid reliability pass on R01, R03, and R04. That is a small synthetic
reliability sample, not production qualification. R02 remains deferred at the
trusted Browser evidence boundary.

No Browser, production, outbound-message, owner-decision, or Phone Farm action
occurred. Raw immutable evidence is under
`.qualification-evidence/realistic-v28`.
