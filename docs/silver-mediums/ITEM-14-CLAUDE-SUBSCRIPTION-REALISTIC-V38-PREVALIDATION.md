# Item 14 — Claude subscription phased R04 v38 prevalidation

Status: `reviewed_frozen`

Evidence stage: local-only. Provider calls during preparation: **zero**.

V38 is the smallest separately versioned extension of the proven v37 phased controller to the unchanged frozen `R04:v1` recovery case. It preserves all v37 sources and both earlier evidence roots.

The harness owns attempt 1: it records the frozen interruption with zero provider calls. Claude begins only in attempt 2, using the exact same run-family identity. Claude Sonnet 4.6 Medium remains on the restricted, tool-free, MCP-free, no-session subscription route. It returns at most four ordered actions per phase for at most five paid phases. The deterministic controller alone performs disclosed synthetic reads, searches, edits, and allowlisted checks.

The next phase receives a hash-only cumulative ledger and only the immediately prior phase's full outputs. State admission binds the current workspace tree, completed result hashes/counts, exact attempt history, model, effort, provider, request, prompt, schema, raw response, and reported accounting. No Claude resume session is used.

V38 deliberately changes the interaction protocol budget: v36 allowed up to 15 one-action provider turns, while v38 allows at most five paid phases with no more than four ordered actions per phase (20 deterministic actions total). It preserves the frozen `R04:v1` case and grader, Claude Sonnet 4.6 Medium, Anthropic subscription/tool-free route, 60,000 total reported-token ceiling, 12,000 generated-token ceiling, and 250,000 cost-equivalent-micro ceiling. Results are therefore comparable on case, model, route, token, generated-token, and cost dimensions, but invocation count is an intentional protocol variable rather than an unchanged control. Retries, fallbacks, owner interventions, Browser use, production changes, messages, Phone Farm access, and external effects remain zero.

The durable command writes a conservatively reserved claim before every paid call, checkpoints the raw receipt and exact plan before action zero, and resumes remaining deterministic actions without another provider call. A crash after claim but before a trustworthy receipt is conservatively sealed as an invalid observation with the full remaining R04 reservation; it is never retried. CLI, schema, and protocol failures preserve exact raw stdout/stderr and use reported usage when it can be parsed safely, otherwise they keep the conservative reservation. Completed, failed, invalid, and globally halted outcomes are sealed, hash-bound, and fully replayed on reload. Candidate competence, protocol, and resource failures remain candidate-local. Credential, unsafe-action, evidence-integrity, and campaign-cap failures halt globally through typed failures and a durable halt write.

Focused adversarial tests cover:

- the zero-call forced interruption and exact-family attempt-two completion;
- restricted tool-free transport with no Claude session resume;
- raw structured-plan equality and receipt/accounting replay;
- hash-bound cumulative state and current workspace tree admission;
- crash recovery after a paid plan checkpoint without a duplicate call;
- conservative ambiguous in-flight closure with full reservation;
- unsafe action, path, and check rejection;
- forged attempt-one history and arithmetic-overflow rejection;
- candidate-local versus global typed stop behavior; and
- review gating before creation of any artifact or provider invocation;
- exact and conservative failed-call receipt/accounting replay; and
- closed-result reload plus paid-phase drift rejection.

The independent review returned HOLD, the seven blockers were resolved in one consolidated correction pass, and root accepted that correction without starting a second reviewer cycle. The final executable immutable v38 freeze binds the accepted definition, source, tests, command, and this document. Provider calls are authorized only through that exact freeze; none were invoked while preparing, correcting, or freezing v38, and no v37 source or evidence was modified.
