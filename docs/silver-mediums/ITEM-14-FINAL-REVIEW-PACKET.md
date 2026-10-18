# Item 14 final independent review packet

## Objective

Perform exactly one final read-only review after the consolidated SILV-14
correction. Determine whether Item 14's local architecture and qualification
result may be accepted. Do not implement, repair, create more reviewers, or
request stylistic changes.

## Required reading

Read completely:

1. `ITEM-14-INDEPENDENT-REVIEW-CORRECTION.md`
2. `ITEM-14-TOPOLOGY-SCENARIO-BANK.md`
3. `ITEM-14-TOPOLOGY-TRACE-LEDGER.md`
4. `ITEM-14-TOPOLOGY-COMPARISON.md`
5. `ITEM-14-PROVISIONAL-TOPOLOGY.md`
6. `ITEM-14-SYSTEM-GRAPH-CONTRACT.md`
7. `ITEM-14-RESPONSIBILITY-AUDIT.md`
8. `ITEM-14-ROLE-INSTRUCTION-CONTRACT.md`
9. `ITEM-14-MODEL-QUALIFICATION-RESULT.md` when present
10. `server/internal/systemgraph/types.go`, `catalog.go`, `query.go`,
    `roles.go`, `default_graph.json`, `roles.json`, and their tests
11. `server/internal/agentqualification/qualification.go`, `decisions.go`,
    `cases.json`, `accepted_configurations.json`, and their tests

Compare structural claims with the maintained Silver Mediums system map,
boundary contract, and role-coverage authorities. Do not treat old runtime
health as current.

## Required challenges

- Verify every journey has one concrete accountable actor/configuration and
  that ordinary CRM product work is not silently owned by identity/access.
- Verify every locked case has route and cost evidence for all four candidate
  topologies.
- Try to bypass action, target, host, credential-purpose, expiry/revocation,
  unknown-coverage, and Phone Farm boundaries.
- Try to make a builder review itself or reuse the same model family on a
  high-risk review without a deterministic substitute.
- Check Canopy Accounts/OAuth, installed Canopy Desktop/updater, Telnyx
  consent/delivery, and CRM-native work coverage.
- Verify graph traversal exposes upstream authorities and downstream consumers
  without expanding every shared CRM node into the entire company.
- Verify model qualification rejects cheap incomplete candidates and does not
  claim unrun or quota-blocked configurations passed.
- Distinguish implemented local foundation from not-yet-connected real
  consequential dispatch. A fail-closed uninstalled gateway is acceptable;
  a claim that prompt prose enforces runtime authority is not.

## Verdict format

Return one comment containing:

- `APPROVE ITEM 14 LOCAL FOUNDATION`, `REVISE ITEM 14`, or `REJECT ITEM 14`;
- P0/P1/P2 counts;
- for each P0/P1, exact file, counterexample, violated requirement, and smallest
  correction;
- explicit statement on whether the seven SILV-14 findings are closed.

Approval requires P0=0 and P1=0. P2 may remain only if it cannot create wrong
ownership, unsafe action, false completion, hidden system coverage, or material
run/token amplification.
