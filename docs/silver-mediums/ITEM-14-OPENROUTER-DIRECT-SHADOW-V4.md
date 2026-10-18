# Item 14 OpenRouter direct shadow v4

Status: terminal real continuation complete; exact route proved; candidate
failed all frozen response contracts; authority exhausted

Snapshot: 2026-09-14 14:11 America/New_York

V4 is a separately versioned continuation of the terminal V3 observation. It
does not replay `local-intake-decision-coordination`. It binds the V3
definition and terminal response hashes and permits exactly the three frozen
workflows which V3 did not call:

- `local-implementation-review-mobile`
- `local-release-regression-rollback`
- `local-interruption-recovery`

Frozen boundaries:

- Definition SHA-256:
  `c9368b0dbb9f9bf3a01c6dc5d5a554fa840ab0ce4d144f5d1f62ea2522190af6`
- V3 definition antecedent:
  `b3241a97ac57418868c71059a6b443754e61069405025d0f3da56ad913372c5c`
- V3 response antecedent:
  `52e3ebb36be48988d3eaef1db17c20e73551173e5618e2338479066a2226b98e`
- V3 terminal receipt antecedent:
  `116630044075634e7d6bf479fd5efd2c0cc6c33542e44f26eaa3bf4369780ff1`
- Frozen workflow suite:
  `9c8803d41b12b99fa7286ecd379d82ca3f2497bae8f181f1d4c524e8912e32be`
- Requested alias: `qwen/qwen3-coder-next`
- Required top-level provider: `Alibaba`
- Required selected endpoint provider/model:
  `Alibaba` / `qwen/qwen3-coder-next-2025-02-03`

The direct request pins provider `order` and `only` to Alibaba, disables
fallbacks, and contains no tools, plugins, fallback model list, repository, or
external action. The controller requires the exact top-level model/provider,
router attempt value `1`, and exactly one selected Alibaba dated endpoint. The
optional detailed attempt array may be absent, as in the terminal V3 response,
or may contain exactly one matching Alibaba/model entry. A second or mismatched
attempt fails closed. This accepts the actual API shape without weakening the
exact-route proof.

Each workflow has one call, zero retries and redirects, a 1,200-token output
ceiling, and a conservative 80,444-micro reservation. The whole continuation
is capped at `$1` inside the unchanged cumulative `$40` testing ceiling. One
exclusive global claim prevents alternate-root replay. Identity, provider,
tool, evidence, credential, or accounting ambiguity halts globally without
retry. A response-contract failure is recorded as candidate-local and permits
the next unattempted workflow to run.

Provider-free evidence:

- focused V4 boundary and lifecycle tests passed 20 repetitions before review;
- the complete affected qualification and runner packages passed;
- focused Go vet passed;
- plan and prepare-only produced zero credential reads and zero provider calls;
- the deterministic fake lifecycle produced three passing observations, zero
  real requests, no tools/access/actions, and kept `candidate_qualified:false`
  because the bound V3 antecedent failed;
- both the V4 evidence root and global claim were verified absent.

The independent provider-free review returned REVISE. Two observations referred
to an earlier generation rather than V4: the V3 terminal receipt itself proves
Alibaba, and the V4 executor already owns the direct transport, one credential
read, reservations, cleanup, and an exclusive global replay claim. The review
still identified two valid hardening gaps. One consolidated correction now:

- embeds and strictly reparses the hash-bound V3 terminal receipt instead of
  trusting the definition's antecedent boolean alone;
- freezes each workflow's canonical request hash and the response schema hash,
  and binds the exact system prompt;
- directly tests absent, one exact, wrong, and multiple attempt arrays;
- seals any post-exposure global halt as a terminal receipt with safe identity,
  response hash, token/cost fields when present, spend readback when available,
  credential cleanup, remaining-work count, exhausted authority, and no retry;
- proves a paid provider-identity drift stops after one request and rejects an
  alternate-root replay while preserving accounting.

The corrected focused tests passed 20 repetitions; full affected packages,
vet, plan, fake, and prepare-only also pass. No second review loop ran.

Terminal real result:

- the frozen command ran exactly once and issued three provider requests, one
  for each previously unattempted workflow, with zero retries;
- every request returned exact alias `qwen/qwen3-coder-next`, top-level
  provider `Alibaba`, selected endpoint provider `Alibaba`, selected endpoint
  model `qwen/qwen3-coder-next-2025-02-03`, and router attempt `1`;
- all three responses failed the unchanged frozen field contract. Each added
  substitute `has_*` or `any_*` fields while omitting the exact required
  repository-, production-, outbound-, and role-selection fields. The
  controller did not normalize or repair them;
- current V4 usage was 988 input and 758 output tokens, zero reasoning tokens,
  and 1,434 microdollars of provider-receipt cost. Durations were 1,838 ms,
  1,977 ms, and 1,813 ms;
- combined with the bound V3 antecedent, the candidate evaluated four frozen
  workflows, passed zero, used 1,309 input and 1,026 output tokens, and cost
  1,933 microdollars by provider receipt;
- immediate authoritative account readback remained 111,582,652 microdollars
  before and after V4. That snapshot had incorporated V3's earlier lagged
  spend, but not the new 1,434-micro V4 receipt cost. Both evidence layers are
  preserved without treating the lagging account view as zero cost;
- response hashes are
  `7f147252c72bb81552d89a2d6fca9eeea153058fc750d87a54134b395ba6714c`,
  `1f71497d6bbd8959688aed4e09ae28b2212e1dcda573dd595684a20359680cac`,
  and
  `b72de826baa98430eaec9b9f39583acb3b28d64516ccb47568e65281a1d7380a`;
- accounting records one credential read, no persisted credential copy, and
  memory zeroing. Tool, repository, production, outbound-action, and
  role/model-selection counts are all zero.

Evidence is sealed under
`.qualification-evidence/item14-openrouter-direct-shadow-v4/`, with the fixed
global replay claim outside that root. V4 is terminal and must never be rerun.
It proves the direct OpenRouter exact-route transport for this eligible small
non-Claude fixture, but eliminates this candidate for the frozen local-shadow
contract. It does not select a permanent role or model.
