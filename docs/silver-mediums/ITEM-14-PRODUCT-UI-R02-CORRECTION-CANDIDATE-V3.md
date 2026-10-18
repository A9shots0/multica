# Item 14 Product/UI R02 correction candidate v3

- Status: prepared and fake-tested; not runtime qualified
- Scope: frozen synthetic R02 fixture only
- Provider, credential, repository, production, and Phone Farm authority: none

## Source failure pattern

The reliability v2 trial preserved six terminal R02 failures. Five stopped in
deterministic controller execution. The final observation reached the local
Browser but failed its hidden verifier. The sealed action traces show three
repeatable causes: running a known-failing check before repair even though R02
execution is terminal on action failure, returning incomplete or placeholder
edit contents, and omitting the responsive CSS needed by the hidden verifier.

This interpretation does not relabel, reopen, or retry any trial observation.

## Additive correction

The v3 overlay narrows the accepted shared-executor packet for R02. It requires
exactly six controller actions in order:

1. replace `report/report.js` with complete literal JavaScript;
2. replace `report/styles.css` with complete literal responsive CSS;
3. run `node tests/report.test.mjs`;
4. run `local-static-preview report`;
5. request the controller-owned Browser once;
6. finish with a truthful local-only summary.

`EVIDENCE.json` stays controller-owned. Ellipses, TODOs, placeholders,
controller-fill instructions, pre-repair checks, extra paths, additional
model turns, tools, repositories, providers, retries, and fallbacks fail
closed.

Bound artifacts:

- action contract:
  `server/internal/agentqualification/product_ui_r02_action_contract_v3.json`
  (SHA-256
  `9816381e4916d76cd6f6faf295ee41cd921ffd2f29926ed1097cbb8d2bb9e309`);
- executor overlay:
  `server/internal/agentqualification/role_artifacts/v3/shared_windows_executor_r02.instructions.md`
  (SHA-256
  `5da1e406d7ec0a7711756b53e2e3e224828449e8f44e9dcbf53bfcec80266ca3`);
- Product/UI skill:
  `server/internal/agentqualification/role_artifacts/v3/product_ui_r02.skill.md`
  (SHA-256
  `028c928114ba31d84429c28520b0be5daf83944af8c0e601a0575573cd7ff87c`).

## Provider-free proof

The frozen fake R02 workflow passes the exact action contract and hidden
verifier. Focused mutations preserve denial of the observed failure patterns
and every authority expansion. This proves the correction is internally
executable in the deterministic harness; it does not prove subscription-runtime
reliability.

Product/UI and R02 remain unqualified. A real runtime measurement would require
a fresh, separately bounded owner authorization. No existing provider evidence
root can be reopened, and this packet authorizes no provider call.
