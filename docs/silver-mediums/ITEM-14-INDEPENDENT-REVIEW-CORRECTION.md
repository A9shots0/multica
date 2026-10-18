# Item 14 consolidated independent-review correction

- Review: SILV-14, one independent Sol-high architecture pass
- Original verdict: revise, P0 0 / P1 6 / P2 1
- Correction policy: one consolidated author pass, then one final independent
  review; no rolling reviewer loop

| Finding | Correction | Executable/document proof |
| --- | --- | --- |
| F1 outcome label lacked actor | Added exactly one `accountable_actor_binding` for every graph journey, including capability-specific configuration, acceptance authority, precedence, fallback, and evidence-based rebinding; separated ordinary CRM product work from CRM identity/access | `roles.json`; `RoleCatalog.Validate`; `TestEveryJourneyHasExactlyOneConcreteAccountableActorBinding`; `j_crm_product` |
| F2 comparison lacked route/cost evidence | Locked a 19-case machine bank and added all four topology traces with actor, DAG, bounded run/handoff/packet/retry/owner estimates and termination | `cases.json`; `ITEM-14-TOPOLOGY-TRACE-LEDGER.md` |
| F3 graph was not executable authority | Added strict JSON decode, host-aware graph fields, typed expiring/revocable capability receipts, target/tool/host/action checks, coverage assurance, and execution-receipt contract | `roles.go`; `TestCapabilityReceiptFailsClosedWithoutCoverageAssurance` |
| F4 review independence was prose only | Added immutable agent/config/model provenance checks, mutable-channel denial, and different-model-family or deterministic-substitute rule for high risk | `ValidateReviewerIndependence`; provenance test |
| F5 product/provider families missing | Added CRM-native product, Canopy Accounts/OAuth, installed Canopy Desktop/updater, and Telnyx consent/delivery nodes, edges, journeys, regressions, and T17-T19 | `default_graph.json`; `cases.json`; scenario bank; graph coverage test |
| F6 LLM could waive unknown graph area | Controller now derives consequential risk from changed node kinds, authority, credential, and host properties; incomplete coverage denies admission until a separately attributable assurance receipt | `RouteIssue`; `TestChangedAuthorityNodeDeterministicallyRequiresAssurance` |
| F7 versions disagreed | Locked `silver-mediums.topology-cases.v2` to SHA-256 `d819bb8dd5d71bb4234ebdcf9707a45652fabb0a1a4a4301a59a421544fb25a3` and references it from scenario, comparison, topology, and trace documents | locked metadata and `TestCasesCoverAllNineteenArchitectureScenarios` |

## Enforcement boundary

The graph, queries, route plan, actor bindings, receipt validator, and
provenance validator are implemented and live in the local backend API. They do
not silently authorize an existing generic Multica run. Real consequential
dispatch remains denied until the controller persists and supplies the full
receipt to the execution gateway. This is deliberate fail-closed staging, not a
claim that prompts alone enforce tool permissions.
