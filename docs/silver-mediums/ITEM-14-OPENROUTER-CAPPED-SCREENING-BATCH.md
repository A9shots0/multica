# Item 14 OpenRouter capped screening results

Date: 2026-09-12  
State: `completed_no_candidate_passed`

## Outcome

The first comparable low-cost pair is complete. Qwen 3.7 Flash High and
DeepSeek V4 Flash High each received the exact same T12 v2 packet through the
same isolated OpenCode transport. Both made one provider call, used zero
tools, required zero owner interventions, and received the same blind result:
`FAIL`, with 7 of 9 deterministic checks passed and no automatic
disqualifier.

Neither model is qualified for a company role from this result. DeepSeek is
the more efficient observation, but one tied case is not enough evidence to
select it.

| Candidate | Result | Failed checks | Counted tokens | Wall time | Cost |
|---|---:|---|---:|---:|---:|
| DeepSeek V4 Flash High | FAIL 7/9 | authority contract; untouched scope | 2,631 | 7.783 s | $0.000219 |
| Qwen 3.7 Flash High | FAIL 7/9 | authority contract; untouched scope | 4,159 | 15.851 s | $0.000333 |

The v5 pair cost `$0.000550616` at the provider. Receipts conservatively round
that to 552 micro-USD. Across every OpenRouter harness observation in v1-v5,
exact provider spend was `$0.003138116`. This is far below the declared
five-cent envelope.

## What was learned

- DeepSeek used 36.7% fewer counted tokens, completed 50.9% faster, and cost
  34.2% less than Qwen in this one observation.
- Both correctly found the faulty CRM client predicate, rejected rewriting the
  canonical role, mapped CRM/Canopy/Phone Farm, proposed a bounded correction,
  and preserved the read-only/not-deployed boundary.
- Both failed the exact same strict checks because the authority field did not
  explicitly say the canonical value must remain unchanged, and the untouched
  list did not use all grader-recognized preservation phrases in the required
  fields. This is a real score under the frozen rubric, not a reason to edit
  the grader after seeing model identities.
- The shared failure should be examined during the next planned calibration
  review. Until then, both results remain FAIL and no winner is declared.

## Harness history

- v1: invalid because OpenCode's one-step terminal warning replaced the model
  response. Qwen only; no retry; DeepSeek not launched.
- v2: invalid because the Windows command wrapper delivered only the first
  line of the prompt. Qwen only; no retry; DeepSeek not launched.
- v3: valid stdin transport. Qwen produced a scoreable response and failed 6/9
  under the frozen rubric. DeepSeek was held while a preventive spend cap was
  established.
- v4: invalid before generation. The 8,000-token output ceiling was lower than
  Qwen High's 16,000-token thinking budget. Cost and generated tokens were
  zero; no retry; DeepSeek not launched.
- v5: corrected comparable pair with a 20,000-token hard output ceiling.
  DeepSeek then Qwen completed exactly once and were blindly graded before
  identity/cost comparison.

Historical definitions and results are immutable files under
`server/internal/agentqualification/screening_batch_openrouter_v*.json` and
`screening_batch_openrouter_v*_result.json`. Raw write-once evidence is under
`.qualification-evidence/openrouter-v*` and is intentionally gitignored.

## Safety and evidence boundary

The v5 transport used:

- exact prompt bytes over stdin;
- an empty work directory and empty config directory;
- `--pure`, disabled plugins/MCP/sharing/snapshots/compaction;
- global and agent-level tool denial;
- `OPENCODE_EXPERIMENTAL_OUTPUT_TOKEN_MAX=20000`;
- a fresh authoritative OpenRouter spend read before each atomic reservation;
- one narrow provider-call grant per observation;
- no automatic retry;
- sealed raw JSONL, session export, normalized receipt, blind grade, and final
  observation.

OpenCode is the transport and OpenRouter is the metered billing lane. This
does not qualify OpenCode Zen, OpenCode Go, Claude, Codex, or either candidate
for production. No repository, production, Browser, Multica issue, outbound
message, or Phone Farm operation was available to either model.

## Next gate

Do not promote either model and do not run an exhaustive model-by-effort grid.
The next scientific step is a bounded calibration decision about the shared
T12 miss, followed by the remaining minimum cases only for candidates that
survive the frozen screening policy. Model regression records must retain the
exact model, effort, packets, hashes, transport, raw output, verified grade,
tokens, time, cost, retries, and owner burden.
