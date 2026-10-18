# Item 14 topology v9 R04 prevalidation

Date: 2026-09-12

Status: `completed`. One independent review and one consolidated correction
preceded the atomic freeze. The result is recorded in
`ITEM-14-TOPOLOGY-V9-RESULT.md`. This group does not select a production model,
role, agent, or topology.

## Purpose and unchanged evidence

Topology v9 is the reciprocal of topology v8. It compares Claude Sonnet 5
Medium executing the unchanged frozen R04 v1
`synthetic-exact-once-resume-v1` case directly against the same executor with
one independent GPT-5.4 Medium review. The R04 task, fixture, declarations,
hidden grader, controller, forced interruption, transport, receipt parser, and
accounting sources are unchanged and hash-bound. Topology v8 through v1 are
recursively preserved.

Exact routes:

- executor: `realistic.claudesonnet5.medium.executor.topology-v9` /
  `openrouter/anthropic/claude-sonnet-5` / `medium` / upstream `anthropic`;
- reviewer: `realistic.gpt54.medium.reviewer.topology-v9` /
  `openrouter/openai/gpt-5.4` / `medium` / upstream `openai`.

Provider fallback is disabled. Transport retries, owner interventions, Browser
actions, and external actions are zero.

## Frozen candidate matrix

| Order | Observation | Pair | Arm |
|---:|---|---:|---|
| 1 | `v9-r04-p1-direct` | 1 | Direct |
| 2 | `v9-r04-p1-reviewed` | 1 | Reviewed |
| 3 | `v9-r04-p2-reviewed` | 2 | Reviewed |
| 4 | `v9-r04-p2-direct` | 2 | Direct |

R04 attempt 1 is the deterministic zero-provider-call interruption checkpoint.
Initial execution resumes attempt 2 under the same run-family ID; this is
continuation, not retry. A reviewer-requested correction has its own attempt-2
run and cannot replay the interruption. At most one correction and two reviews
are possible.

## Enforced limits

| Boundary | Calls | Total tokens | Generated tokens | Cost |
|---|---:|---:|---:|---:|
| Initial execute or Direct arm | 15 | 60,000 | 12,000 | $0.25 |
| Review 1 | exactly 1 | 20,000 | 5,000 | $0.10 |
| Correction | 8 | 40,000 | 10,000 | $0.20 |
| Review 2 | exactly 1 | 20,000 | 5,000 | $0.10 |
| Reviewed arm | 25 | 140,000 | 32,000 | $0.60 |
| Campaign | 80 | 400,000 | - | $1.70 |

The corrected topology-v8 sealing and accounting behavior is generation-owned
by v9: stage cost and token ceilings are checked before every subsequent call;
pending reviewer calls are durable for crash accounting; exact call-ceiling
exhaustion and isolated non-safety harness or transport defects invalidate only
the affected observation. Global halt is limited to credential failure,
uncontrolled spending or campaign budget breach, unsafe external action, and
campaign-wide evidence corruption. No retries or fallbacks are permitted.

The consolidated pre-execution correction makes cost equality a hard preflight
stop before another execute or correction call. Ambiguous executor or
correction transport outcomes remain observation-local invalids unless they
meet a frozen global-halt condition, retain ambiguous token/accounting status,
and reconcile authoritative provider spend. Immediate reviewer transport
ambiguity performs the same spend reconciliation before continuation; failure
to reconcile stops the campaign as uncontrolled spending.

## Provider-free proof required before review

```powershell
go test ./internal/agentqualification ./cmd/topology-run-v9 -count=1 -timeout 240s
go vet ./internal/agentqualification ./cmd/topology-run-v9
go run ./cmd/topology-run-v9 --plan --artifact-root <temporary-empty-root>
```

The pre-activation plan returned the first exact observation with
`provider_called:false` and `provider_ready:false`. After the one review and
one consolidated correction, the atomic freeze changed only admission state
and bound hashes; the provider-free plan then returned `provider_ready:true`.
No Browser, production, Phone Farm, message, or other external action was part
of prevalidation or execution.
