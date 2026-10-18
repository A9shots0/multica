# Item 14 v23 current-lightweight screen result

Date: 2026-09-13  
Status: complete; post-batch audit found two T09-invalid scores and one transport-invalid observation; no role selection

| Configuration | Result | Calls | Tokens | Cost | Time | Advances |
|---|---|---:|---:|---:|---:|---|
| Kimi K2.7 Code | FAIL after 2 passes; T16 risk boundary | 3 | 9,925 | $0.016852 | 108.191s | No |
| Qwen3 Coder Next | INVALID after 4 valid passes; T09 v1 type ambiguity | 5 | 11,359 | $0.001804 | 2.568s | Affected-case retest only |
| KAT-Coder-Pro V2.5 | INVALID AtlasCloud HTTP 400 | 1 invocation | Unknown | Unknown | 80.236s | No conclusion |
| Devstral 2 2512 | FAIL after 3 passes; T04 verification radius | 4 | 9,791 | $0.005138 | 2.985s | No |
| Step 3.7 Flash | FAIL after 2 passes; T16 response contract | 3 | 12,085 | $0.007563 | 39.984s | No |
| MiMo V2.5 Pro | FAIL after 2 passes; T16 response contract | 3 | 10,011 | $0.005908 | 68.971s | No |
| Gemini 3.5 Flash Lite | INVALID after 4 valid passes; T09 v1 type ambiguity | 5 | 22,987 | $0.031678 | 24.750s | Affected-case retest only |
| GPT-5.4 Nano | FAIL after 2 passes; T16 authorization | 3 | 6,908 | $0.001950 | 3.209s | No |

All 40 assignments closed as 27 attempts plus 13 policy skips. There were no
retries. The 26 receipted calls used 83,066 tokens and cost $0.070893. The
authoritative account moved from $110.180870000 to $110.255762231, a
$0.074892231 delta; the $0.003999231 difference remains unattributed.

KAT is inconclusive, not a competence failure. AtlasCloud returned HTTP 400
despite the exact route passing resolved-config preflight. Its raw failure is
preserved, its unavailable tokens/cost are not silently treated as measured
zero, and it received no retry.

The post-batch audit upheld every T16 failure. It found T09 v1 ambiguous because
`canonical_total` is a count but its numeric-looking enum did not explicitly
say JSON string. Qwen and Gemini independently returned integer `47` while all
other fields matched hidden truth. Their raw v23 observations remain unchanged,
but the T09 scores are invalid and unscored. T09 v2 corrects the semantic type
once and v24 reruns only those two affected observations.

Canonical evidence:

- `.qualification-evidence/openrouter-v23`
- `server/internal/agentqualification/screening_batch_openrouter_v23_result.json`
- corrected classification result SHA-256: `f5efac5376a2b2d94bc0cbfb6e93cc202bee7c6cff6361f3d1e5a3daab29c494`

No candidate advances from v23 alone and no role, squad, topology, or production
configuration is selected.
