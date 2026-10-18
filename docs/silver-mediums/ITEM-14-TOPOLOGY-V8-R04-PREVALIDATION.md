# Item 14 topology v8 R04 prevalidation

Date: 2026-09-12

Status: `frozen_reviewed`. The single independent review returned two concrete
NO-GO findings; one consolidated correction pass closed both findings before
the experiment and freeze were activated together. This package does not
select a production model, role, agent, or topology.

## Purpose and unchanged case

Topology v8 compares GPT-5.4 Medium executing the already frozen R04 v1
`synthetic-exact-once-resume-v1` case directly versus the same executor output
receiving one independent Claude Sonnet 5 Medium review. The R04 task, fixture,
declaration, hidden grader, controller, forced interruption, transport, receipt
parser, and accounting source are hash-bound; none was modified for this group.

Exact routes:

- executor: `realistic.gpt54.medium.executor.topology-v8` /
  `openrouter/openai/gpt-5.4` / `medium` / upstream `openai`;
- reviewer: `realistic.claude-sonnet5.medium.reviewer.topology-v8` /
  `openrouter/anthropic/claude-sonnet-5` / `medium` / upstream `anthropic`.

Both routes disable provider fallback. Transport retries and owner
interventions are zero.

## Frozen run matrix

| Order | Observation | Pair | Arm |
|---:|---|---:|---|
| 1 | `v8-r04-p1-direct` | 1 | Direct |
| 2 | `v8-r04-p1-reviewed` | 1 | Reviewed |
| 3 | `v8-r04-p2-reviewed` | 2 | Reviewed |
| 4 | `v8-r04-p2-direct` | 2 | Direct |

R04 attempt 1 is the deterministic zero-provider-call interruption checkpoint.
The initial execute then resumes attempt 2 under the same run-family ID. This is
continuation, not a retry. A reviewer-requested correction uses a separate
attempt-2 correction run and cannot replay the interruption. At most one
correction and therefore at most two reviews are possible.

## Enforced limits

| Boundary | Calls | Total tokens | Generated tokens | Cost |
|---|---:|---:|---:|---:|
| Initial execute | 15 | 60,000 | 12,000 | $0.25 |
| Review 1 | exactly 1 | 20,000 | 5,000 | $0.10 |
| Correction | 8 | 40,000 | 10,000 | $0.20 |
| Review 2 | exactly 1 | 20,000 | 5,000 | $0.10 |
| Direct arm | 15 | 60,000 | 12,000 | $0.25 |
| Reviewed arm | 25 | 140,000 | 32,000 | $0.60 |
| Four-observation campaign | 80 | 400,000 | - | $1.70 |

The reviewed-arm ceiling is intentionally stricter than the sum of every
stage's independent cost ceiling. Crossing a stage ceiling seals and counts the
paid response, invalidates only that observation, and prevents a next provider
call. Global halt is limited to credential failure, uncontrolled spend or
campaign budget breach, unsafe external action, and evidence corruption.

The correction pass added two fail-closed boundaries. Execute and correction
now check their accumulated paid cost before every subsequent provider call,
so the first response that crosses `$0.25` or `$0.20` is preserved but no later
call can escape. Each reviewer call now writes a durable pending-review marker
before transport begins. Crash recovery cannot interpret a missing completed
review receipt as an exact zero-call result; it records a conservative
ambiguous call and retains provider-spend reconciliation in the stage ledger so
aggregation cannot erase it.

The frozen admission also distinguishes historical topology-v1 evidence from
the current shared screening entrypoint: v1's unchanged `topology_cli.go` and
immutable freeze manifest remain recursively verified, while v8 separately
hash-binds the current `screening-run/main.go`. This preserves the R02
administrative-seal CLI additions without rewriting any prior freeze.

## Provider-free proof

Commands completed:

```powershell
go test ./internal/agentqualification ./cmd/topology-run-v8 -count=1 -timeout 240s
go vet ./internal/agentqualification ./cmd/topology-run-v8
go run ./cmd/topology-run-v8 --plan --artifact-root <temporary-empty-root>
```

Focused tests and vet passed. The plan-only preflight returned the first exact
observation, zero calls, `provider_ready:true`, and route hashes for both exact
resolved configurations. The freeze recursively binds topology v7 through v1
and preserves the prior command-source verification chain. No provider, Browser,
production, Phone Farm, message, or external side effect was invoked.
