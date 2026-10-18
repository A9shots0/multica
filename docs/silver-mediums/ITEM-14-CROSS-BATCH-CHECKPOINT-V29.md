# Item 14 cross-batch checkpoint after v29

Snapshot: 2026-09-13. This checkpoint consolidates the completed static,
realistic, effort, topology, and transport evidence. It does not select an
agent, role, squad, topology, permission, or production configuration.

## What the evidence now supports

| Question | Strongest current evidence | Conclusion |
|---|---|---|
| Can the static suite distinguish candidates? | Strong anchors solve it while inexpensive candidates fail different disclosed gates. Reliability repeats eliminate several one-pass successes. | Yes. Static breadth is useful screening, but is not qualification. |
| Did any inexpensive candidate survive realistic work? | Luna Pro passed 15 consecutive static cases, then failed R01 protocol. Gemini 3.5 Flash Lite, Qwen3 Coder Next, Solar Pro 4, and free Nex N2.5 Pro passed static reliability; all were eliminated in v27 realistic testing. | No inexpensive configuration has yet survived the realistic capability gate. |
| What is the strongest synthetic configuration? | Claude Sonnet 5 Medium through the exact OpenRouter/Anthropic/Medium lane has two valid passes each on R01, R03, and R04. R02 has one initial pass; its repeat is sealed invalid because the trusted Browser evidence surface could not complete the frozen run. | Claude Medium is the strongest current synthetic reference, not a production selection. |
| Does higher effort reliably fix failures? | GLM Medium changed rather than removed its failure. Claude High failed unchanged R04 twice. | No. Blanket high effort is unsupported and wasteful. |
| Does a worker plus reviewer outperform one capable executor? | Topology v3, v5, v7, v8, and v9 found no repeatable rescue. Valid reviewed arms were equal or worse while adding calls, tokens, cost, and time. | The current evidence favors direct execution as the next baseline. It does not prove reviewers are never useful for high-risk gates. |
| Is the Claude subscription transport calibrated? | V29 passed provider-free review, then the Windows command wrapper rejected the 6,794-byte prompt before Claude or a provider request started. One process attempt, zero provider calls/tokens/cost, no retry. | No. V29 is invalid transport evidence. The prompt-to-stdin correction is tested but unexecuted. |

## Scientific interpretation

1. A cheap model passing once is not meaningful enough. Repeats and realistic
   work exposed failures that the broad screen could not.
2. Model, effort, transport, instructions, tools, and controller are one
   material configuration. Claude through OpenRouter and Claude through the
   signed-in Claude Code subscription cannot share qualification evidence.
3. More agents are not automatically safer. In the completed matched tests,
   review overhead did not produce a repeatable outcome improvement.
4. The benchmark is already large enough to reject premature role design and
   blanket use of expensive/high-effort models. It is not yet enough to choose
   production agents.
5. Provider calls should now pause while coverage, cost accounting, and the
   next realistic business-case group are selected from genuine remaining
   uncertainty rather than another long-tail model search.

## Remaining evidence gaps

- A valid second R02 Browser observation on a trusted compatible surface.
- One real Claude Code subscription receipt through the corrected stdin
  transport, only after a new independently frozen observation authorizes it.
- Realistic coverage for documentation continuity, authenticated UI work,
  deployment/live verification, cross-repository contract change, and a
  bounded owner-decision/resume path.
- Reconciliation of receipt totals with account-level OpenRouter deltas where
  the sources disagree; neither source should be rewritten to force equality.
- Real workflow trials only after the synthetic gaps are deliberately scoped.

## V29 immutable evidence

- Definition SHA-256:
  `b9c299879022a62fcca627464c628e45709ae88b4b6f07249e8560cc5d2c5976`
- Raw result SHA-256:
  `880cf27d8e72f69e112d935ea1ab2ad62dfe4ad12db847879d296396b751ae6b`
- Stderr SHA-256:
  `712f89f4976302078a5046faca78c00b32aaf1aa6c5f6455945d776ff60eafb5`
- Empty stdout SHA-256:
  `e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855`
- Machine result:
  `server/internal/agentqualification/claude_calibration_v29_result.json`
- Raw root:
  `.qualification-evidence/claude-subscription-v29/f0bcf5256786d96aa08c676b98ea86c61b593fc623feb02d8494e1716b1bfa1a`

No retry, candidate score, or role advancement is permitted from v29.
