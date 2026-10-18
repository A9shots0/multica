# Item 14 role architecture evaluation

- Status: active evaluation; no permanent roster or topology selected
- Evidence stage: `local-only`
- Owner outcome: a competent, economical agent organization that can carry a
  business problem across Silver Mediums without losing scope, missing blast
  radius, duplicating work, or asking Drew to coordinate engineering
- Frozen foundation: accepted Items 1-13

## Why this evaluation exists

The current role material is a coverage scaffold, not a proven final
organization. It mixes product owners, repository expertise, cross-system
capabilities, host-specific execution, independent assurance, and owner-facing
interfaces. Those are all necessary responsibilities, but they do not
automatically belong as one permanent agent each.

A question, concern, screenshot, or bug report from Drew is evidence about the
problem. It is not automatically a diagnosis, design decision, or instruction
to replace an accepted architecture. The system must preserve the report,
investigate it, and decide from current evidence.

## Non-negotiable operating properties

1. One business outcome remains one issue even when multiple repositories,
   providers, hosts, or specialists participate.
2. The deterministic controller owns dispatch, transitions, leases, timers,
   retries, correction ceilings, and recovery. Mika is not a recurring relay.
3. Every consequential decision has exactly one accountable authority.
4. Contributors receive bounded work and return structured evidence to the same
   issue. They do not create competing narratives or declare their local piece
   to be the complete outcome.
5. Independent verification tests the requested business result and relevant
   unaffected controls, not merely the changed repository.
6. Phone Farm technical work remains Mac Studio-only.
7. A role is not seated because a model is impressive or cheap. The complete
   configuration must qualify for the exact job.
8. Total cost is measured per accepted business outcome, including retries,
   corrections, owner interruptions, cache reads, and failed runs.

## Required intake intelligence

Every owner report enters the same reasoning contract before implementation.
This is required whether Drew sends a broad goal, a small concern, or an
apparently obvious bug.

```text
Preserve the owner's actual report
              |
              v
Classify it: observation / desired outcome / hypothesis / decision / constraint
              |
              v
Identify the affected business journey and current issue
              |
              v
Load the smallest current system map needed for that journey
              |
              v
Trace upstream authority, downstream consumers, shared contracts, providers,
hosts, stored copies, scheduled work, release history, and prior regressions
              |
              v
Separate known facts, conflicts, hypotheses, and missing evidence
              |
              v
Choose the accountable outcome owner and bounded contributors
              |
              v
Diagnose the first failing boundary before selecting a repair
              |
              v
Implement the root correction plus a permanent regression check
              |
              v
Release and independently verify the complete journey and unaffected controls
```

### Prohibited shortcuts

- Treating Drew's proposed cause as verified.
- Agreeing with a concern and redesigning the workflow before comparison.
- Fixing the visible symptom without checking earlier and later boundaries.
- Assigning ownership from the repository containing the first visible error.
- Closing after diagnosis, implementation, tests, deployment, or one component
  check when the requested business outcome remains unproven.
- Allowing each repository agent to say its own side works without an
  end-to-end verdict.

## The system map must become executable infrastructure

The maintained text map and boundary contract are valuable but insufficient as
the only runtime input for a large connected company. Item 14 will define a
machine-readable company graph that can later drive an owner-facing Multica
visualizer.

Each graph node must be able to represent:

- business capability and employee/client journey;
- application and repository;
- service, job, datastore, provider, device fleet, and host;
- responsible truth owner and allowed executor;
- required skill and evidence source;
- current evidence freshness and known uncertainty.

Each graph edge must be able to represent:

- source and destination;
- direction and allowed/forbidden status;
- business data or command carried;
- contract/schema/version and identity key;
- synchronous, asynchronous, scheduled, or provider behavior;
- credential owner without storing credential values;
- durable fallback, retry, monitoring, and failure consequence;
- upstream and downstream blast radius;
- regression checks required after a change.

The visualizer must be a view of this canonical graph, not a second manually
maintained architecture diagram. Agents will query a bounded subgraph for the
current issue instead of loading the whole company map into every run.

## Organizational structures under evaluation

No option is accepted yet.

| Candidate | Core idea | Main strength to test | Main failure to test |
| --- | --- | --- | --- |
| Repository-aligned | One permanent owner for each product/repository | Deep local continuity | Partial ownership and cross-repo ping-pong |
| Capability-aligned | Identity, sync, media, UI, release, and similar specialists span repositories | Follows the failure across boundaries | Excessive context and conflicting code authority |
| Shared execution pool | A small number of strong builders load bounded repository skills per issue | Fewer seats and less duplicated idle context | Shallow system knowledge and weak continuity |
| Layered outcome ownership | A stable business/domain owner remains accountable while bounded executors and assurance stages contribute | One outcome with specialized execution | Matrix ambiguity and handoff amplification |

Generic industry patterns are research inputs only. Silver Mediums selects a
structure from its own scenario evidence.

## Responsibility taxonomy to audit

Before deciding how many agents exist, every responsibility will be classified
as exactly one of:

1. **Deterministic controller function** — transitions, timers, retries,
   decision binding, dedupe, notification admission, and recovery.
2. **Owner interface** — understanding Drew, explaining current truth, and
   presenting genuine decisions.
3. **Persistent domain authority** — judgment requiring durable expertise in a
   coherent business capability and its failure modes.
4. **Dynamically loaded skill** — repository or tool knowledge that does not
   justify a permanent identity.
5. **Host-bound executor** — work that must occur on a particular controlled
   runtime.
6. **Independent assurance** — review, release-truth verification, audit, or
   business-outcome proof that must remain separate from implementation.

A named responsibility does not automatically become an agent. A repository
does not automatically receive an agent. A cross-repository capability does not
automatically receive an agent. The scenario results determine which form is
necessary.

## Locked scenario families

The detailed cases and hidden expected outcomes must be written before any
topology is scored.

1. CRM identity/distribution authority disagrees with LinkHub public behavior.
2. A LinkHub change passes local tests but fails the real customer journey.
3. CRM-created media state fails between Drive, Canopy ingestion, R2, catalog,
   and employee display.
4. Canopy outbound outreach succeeds but inbound reply ingestion or display
   fails.
5. A Phone Farm symptom may originate in application code, Mac runtime, bridge,
   session, or physical device state.
6. A later release reintroduces a previously fixed employee-critical bug.
7. Viral Queue data is correct locally but absent or misleading in CRM Content
   Inspo.
8. A reviewed release deploys, degrades, rolls back, and leaves the original
   outcome open.
9. A reporting request spans metric truth, UX, implementation, deployment, and
   business acceptance.
10. A deterministic monitor detects an incident while Multica or Mainstation is
    unavailable.
11. Drew reports a plausible but incorrect cause; the system must investigate
    without blindly adopting or dismissing it.
12. A small bug has a shared contract or downstream blast radius that makes the
    obvious local patch unsafe.
13. A CRM-native employee/client workflow is broken but does not belong to the
    narrower identity/access specialty.
14. Mainstation is lost and the company workflow must restore from independently
    held evidence without duplication or secret exposure.
15. A new business journey spans CRM and multiple satellites while preserving
    the accepted hub/satellite boundary.
16. A credential, privacy, or authorization concern appears during otherwise
    routine work.

## Comparison evidence

Every candidate topology will receive the same case packet. Record:

- first accountable owner selected;
- necessary and unnecessary contributors;
- system edges inspected and missed;
- unsupported assumptions;
- permissions and host decisions;
- number and direction of handoffs;
- whether Mika or Drew had to relay routine work;
- whether the issue could close partially or incorrectly;
- fresh and cached context volume;
- expected recovery after interruption;
- clarity of the final owner result.

### Automatic topology rejection

Reject a topology if any tested path:

- gives two roles final authority over the same consequential decision;
- permits completion after only one participating system is proven;
- requires Mika or Drew to move evidence between specialists;
- bypasses host, identity, deployment, or independent-review boundaries;
- requires company-wide context for ordinary bounded work;
- creates unbounded handoffs or reviewer loops;
- treats deployment, runtime health, and business success as equivalent; or
- cannot identify the previous fixes and downstream controls a change could
  regress.

## Evaluation and decision sequence

1. Inventory the current role packets and classify each responsibility using
   the taxonomy above.
2. Produce an overlap/gap/conflict matrix. Do not rewrite roles during this
   audit.
3. Write and lock the concrete scenario packets and expected authority/evidence
   outcomes.
4. Walk every candidate topology through every scenario and preserve the full
   routing/evidence trace.
5. Obtain an independent adversarial review focused on counterexamples, hidden
   coupling, and false confidence.
6. Perform one consolidated correction pass and one final independent review.
7. Select a provisional topology and only then rewrite role, skill, permission,
   and squad packets.
8. Benchmark complete model configurations in the intended Multica runtime.
9. Shadow real issues without independent consequential authority.
10. Activate qualified role clusters incrementally with rollback and continued
    measurement.

## Current conclusion boundary

The permanent organization is not selected. The evidence currently supports
only these conclusions:

- pure repository ownership has not been proven;
- pure capability ownership has not been proven;
- the previously suggested hybrid has not been proven;
- the existing roster is broad coverage evidence, not a certified final team;
- the company graph and concern/bug reasoning contract are prerequisites for
  evaluating competent agents across Silver Mediums' actual blast radius.

## External research boundary

External architecture guidance is used to challenge assumptions, not to select
the Silver Mediums roster:

- OpenAI's practical agent guide distinguishes a central manager pattern from
  decentralized handoffs and recommends avoiding unnecessary multi-agent
  complexity. Silver Mediums already has a deterministic controller, so neither
  LLM-led pattern can be copied without adaptation:
  <https://openai.com/business/guides-and-resources/a-practical-guide-to-building-ai-agents/>.
- Anthropic reports that successful systems commonly use simple, composable
  workflows and that predictable tasks often benefit from code-defined
  workflows rather than unconstrained agent autonomy:
  <https://www.anthropic.com/engineering/building-effective-agents>.
- Team Topologies argues for business-capability alignment while using platform,
  enabling, and complicated-subsystem structures to contain cognitive load. It
  also makes clear that one pattern does not fit every responsibility:
  <https://martinfowler.com/bliki/TeamTopologies.html>.

These sources support testing simplicity, accountability, specialization, and
cognitive load. They do not prove whether CRM, LinkHub, Data Sync, Product/UI,
or another Silver Mediums responsibility deserves a permanent agent.
