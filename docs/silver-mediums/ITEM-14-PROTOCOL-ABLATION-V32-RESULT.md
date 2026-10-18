# Item 14 — protocol-ablation v32 result

Status: complete but invalid calibration batch; no model, agent, role, squad, topology, or production permission selected.

Freeze SHA-256: `8a8f19d766ec229d5d07d300d157a84305f7b8b5cf3f3e3cdeb099bb0bffd77a`  
Final ledger SHA-256: `986291baef291c0279d79e697c635ca7bafd806f30704c8d7a8df5ff04c4ab8f`

## Result

| Candidate | Protocol | Result | Exact reason | Calls | Tokens | Cost | Time |
|---|---|---|---|---:|---:|---:|---:|
| Claude Sonnet 5 Medium | one-action loop | Invalid / unscored | Observation-scoped parser invalidity compared split upstream identity with the route-qualified model string; preserved without retry | 1 | 5,287 | $0.017724 | 8.144s |
| Claude Sonnet 5 Medium | one-shot plan | Invalid / unscored | Shared harness defect: the frozen schema was hash-bound but never sent to the candidate; the Markdown response cannot be scored as schema noncompliance | 1 | 6,326 | $0.028047 | 17.905s |
| GPT-5.4 Medium | one-shot plan | Invalid / unscored | Shared harness defect: the frozen schema was hash-bound but never sent to the candidate; the differently shaped JSON cannot be scored as schema noncompliance | 1 | 5,844 | $0.053498 | 20.206s |
| GPT-5.4 Medium | one-action loop | Invalid / unscored | Shared harness defect: the frozen schema was hash-bound but never sent to the candidate; the batched edit cannot be scored as schema noncompliance | 1 | 5,165 | $0.043375 | 16.850s |

Batch total: 4 calls, 22,622 tokens, $0.142644, 63.105 seconds of summed provider-session time, zero retries, and no global halt.

## Evidence and recovery history

- `.qualification-evidence/protocol-ablation-v32` is the preserved zero-provider-call setup failure: the workspace parent was missing.
- `.qualification-evidence/protocol-ablation-v32-v2` preserves the consumed Claude loop call and its observation-scoped parser invalidity.
- `.qualification-evidence/protocol-ablation-v32-v3` preserves the recovered Claude loop cell and consumed Claude one-shot cell.
- `.qualification-evidence/protocol-ablation-v32-v4` is the final ledger. It atomically carries both Claude cells and closes the two GPT cells.
- OpenCode sessions are `ses_f65cc7190ffeCTv632lIB62CN5`, `ses_f65c026bdffew1QiN3HWdL8v6A`, `ses_f65ba537cffeU2QE2J0y80rWsn`, and `ses_f65b9ee7bffeG03Z2NNXF9QPWK` in matrix order.
- Replay reused every completed observation, made zero provider calls, and preserved the ledger hash.

## Interpretation boundary

The post-batch audit found a shared evidence defect. `protocol_ablation_v32.go` built and hashed each response schema, but the executable prompt contained only the common packet and protocol instruction. `execute.go` invoked `opencode run --format json`; in OpenCode 1.18.18 that option selects raw JSON event output and does not enforce or transmit a response schema. Strict decoding happened only after generation.

All four cells are therefore invalid/unscored calibration evidence. Their outputs may be inspected as qualitative evidence of semantic understanding, but they cannot establish schema compliance, protocol causality, reliability, or model competence. The $0.142644 spend remains fully accounted and no cell will be silently rewritten or retried under v32.

The next step is provider-free: build and test a corrected, separately versioned transport that proves the exact schema reaches the model. OpenCode's server API supports schema-validated structured output, while the current CLI `run` path does not expose a schema flag. No additional paid model calls should run until the corrected transport and accounting are independently validated and frozen.

## Verification

- `go test ./internal/agentqualification ./cmd/protocol-ablation-v32 ./cmd/screening-run -count=1` — PASS.
- `go vet ./internal/agentqualification ./cmd/protocol-ablation-v32 ./cmd/screening-run` — PASS.
- Replay — zero calls and unchanged final ledger SHA-256.
- No Browser, production, outbound, owner, or Phone Farm action occurred.
