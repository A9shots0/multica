# Item 14 — v23 current-lightweight prevalidation

Date: 2026-09-13  
Status: frozen after one independent PASS review; provider execution authorized

## Purpose

Screen eight current, inexpensive, previously untested model families on the
unchanged frozen static capability group. This is discovery evidence only. It
does not select a model, role, squad, topology, or production configuration.

## Candidate pool

| Model | Effort | Pinned provider |
|---|---|---|
| Kimi K2.7 Code | provider default | Moonshot AI |
| Qwen3 Coder Next | provider default | Alibaba |
| KAT-Coder-Pro V2.5 | provider default | AtlasCloud |
| Devstral 2 2512 | provider default | Mistral |
| Step 3.7 Flash | provider default | StepFun |
| MiMo V2.5 Pro | provider default | Xiaomi |
| Gemini 3.5 Flash Lite | provider default | Google AI Studio |
| GPT-5.4 Nano | provider default | OpenAI |

The provider-default effort is intentional for the first broad screen. Effort
comparisons occur only for a documented survivor or near-miss.

## Frozen execution

- Cases: unchanged T11 v4, T08 v3, T16 v3, T04 v1, T09 v1
- One observation per candidate/case; zero retries
- Candidate failure eliminates only that candidate
- A non-safety harness imperfection invalidates only the affected observation
- Shared failure is logged for post-batch case/grader audit without rewriting
  the frozen case during the batch
- Tools, repository access, Browser, production, messaging, and Phone Farm are
  excluded
- Exact model and provider route; fallbacks disabled
- 10,000-token reservation per launch; 400,000-token batch ceiling
- 8,000 generated-token hard cap per response
- Absolute OpenRouter ceiling: $111.180870 lifetime usage, providing at most
  $1.00 incremental spend from the v12 final authoritative snapshot
- Zero owner interventions

## Pre-call gate

The eight-model selection received a separate read-only audit and all are
genuinely untested configurations. One final independent review passed the
batch, accounting, stop behavior, provider routes, immutable case hashes, and
runtime preflight without corrections. The frozen batch SHA-256 is
`df98fb264074f428c40e3a0f6c195212c7b863e8aaeebf76b58edbed815738a1`.
No further reviewer cycle or mid-batch harness change is allowed.
