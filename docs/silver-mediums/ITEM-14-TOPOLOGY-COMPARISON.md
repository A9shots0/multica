# Item 14 topology comparison — analyst pass

- Status: consolidated reviewer correction complete; final review pending
- Evidence stage: `local-only`
- Inputs: `ITEM-14-ROLE-ARCHITECTURE-EVALUATION.md`,
  `ITEM-14-RESPONSIBILITY-AUDIT.md`, and
  `silver-mediums.topology-cases.v2`
- Lock hash: `sha256:d819bb8dd5d71bb4234ebdcf9707a45652fabb0a1a4a4301a59a421544fb25a3`
- Complete route/cost traces: `ITEM-14-TOPOLOGY-TRACE-LEDGER.md`
- Important: this is not permission to change agents, models, squads, or skills

## Comparison method

The four structures were walked through the same nineteen scenario families. The
comparison considers structural behavior before model quality: accountability,
required context, system-boundary coverage, handoffs, deterministic
continuation, authority containment, and truthful completion.

Ratings:

- **Strong** — naturally represents the case with one accountable outcome and
  bounded collaboration.
- **Conditional** — can represent it, but requires explicit controller or skill
  rules to avoid a known failure.
- **Weak** — naturally creates fragmented ownership, excessive context, or a
  missing specialist boundary.
- **Rejecting failure** — permits partial completion, owner relay, unsafe
  authority, or another automatic rejection condition.

## Per-scenario result

| Case | Repository-aligned | Capability-aligned | Shared execution pool | Layered outcome ownership |
| --- | --- | --- | --- | --- |
| T01 CRM -> LinkHub disagreement | **Rejecting failure:** two repo owners can each prove only their side; needs a third relay | **Strong** for a bounded sync/authority capability, but local repair authority must transfer cleanly | **Conditional:** one builder can trace both repos if the contract graph and skills are complete | **Strong:** one boundary outcome stays accountable while CRM and LinkHub executors contribute |
| T02 LinkHub Save/customer journey | **Strong** locally, conditional on shared release/QC | **Conditional:** generic UI/release capabilities can miss LinkHub-specific domain behavior | **Conditional:** efficient if a LinkHub skill carries admin/public/provider knowledge | **Strong:** LinkHub journey owner plus bounded release and QC stages |
| T03 Drive -> Canopy -> employee media | **Rejecting failure:** spans CRM rules, Canopy, provider/storage, and UI | **Strong** for media-flow ownership, conditional on app-specific implementation | **Weak/conditional:** very large temporary context and many specialized provider boundaries | **Strong:** persistent media outcome with scoped CRM/Canopy/UI execution |
| T04 outbound works/inbound reply missing | **Conditional:** may be one Canopy repo but crosses provider, worker, storage, API, UI, and notification | **Strong** if publishing/automation is a coherent capability | **Conditional:** skill packet must preserve exactly-once and authorization expertise | **Strong:** outreach outcome owner with bounded provider/data/UI checks |
| T05 Phone Farm app versus fleet | **Rejecting failure:** one repo owner obscures the application/fleet diagnostic boundary | **Strong:** app and complicated fleet subsystem remain distinct | **Weak:** general pool cannot safely substitute for persistent Mac/fleet expertise | **Strong:** one Phone Operations outcome with Mac-only app/fleet contributors selected by evidence |
| T06 release revives old bug | **Conditional:** repo owner can repair, but cross-release regression registry is shared | **Conditional:** release/UI capabilities can own gates but may lack domain history | **Conditional:** works only with deterministic regression registry and repo skill | **Strong:** domain outcome owns historical behavior; release/QC enforce it every time |
| T07 Viral Queue -> CRM Content Inspo | **Rejecting failure:** two repos can each report local success | **Strong** if one content-supply capability owns the full flow | **Conditional:** bounded builder needs both product and data-contract skills | **Strong:** content-supply outcome remains accountable across backend and CRM presentation |
| T08 deploy/degrade/rollback/outcome open | **Conditional:** needs shared lifecycle authority outside repository owner | **Strong** for release/runtime, conditional on domain outcome remaining open | **Conditional:** controller can preserve state but executor competence varies | **Strong:** controller preserves outcome; release handles runtime truth; domain/QC handle recovery proof |
| T09 report data + design + workflow | **Weak:** a CRM repo owner is not sufficient for metric, product, rendering, and business judgment | **Conditional:** several capabilities can cause collaboration overhead | **Conditional:** strong builder may integrate efficiently, but design/data qualification differ | **Strong/conditional:** one report outcome with data, product, execution, and QC stages; must cap handoffs |
| T10 monitor survives Multica outage | **Weak:** monitoring is not naturally repository-owned | **Conditional:** monitoring capability works, but critical path must stay deterministic | **Strong:** no permanent LLM seat is needed for deterministic detection; bounded analyst only when useful | **Strong:** deterministic platform function, later routed to the affected domain outcome |
| T11 owner proposes wrong cause | **Conditional:** repo owner may anchor on its own system | **Conditional:** capability owner may anchor on its own failure class | **Strong/conditional:** neutral triage plus bounded system graph can test the hypothesis cheaply | **Strong:** intake preserves hypothesis; accountable lead chosen after boundary evidence, not wording |
| T12 small bug/shared contract blast radius | **Rejecting failure:** local ownership encourages diff-sized reasoning | **Strong** when contract/identity capability is correctly recognized | **Conditional:** safe only if graph expansion and contract gates are deterministic | **Strong:** graph determines required contributors and controls before implementation |
| T13 CRM-native operational feature | **Strong/conditional:** deep repo knowledge helps, but one repo owner may still be too broad | **Weak/conditional:** no current capability cleanly owns all CRM product behavior | **Strong:** qualified executor plus CRM product/repo skill avoids inventing another broad permanent owner | **Strong/conditional:** one CRM business outcome with bounded execution; taxonomy must not force it into identity/access |
| T14 workstation loss/recovery | **Rejecting failure:** recovery spans host, database, files, controller state, private access, and notification reconciliation | **Strong/conditional:** recovery capability can coordinate proof but should be mostly deterministic | **Conditional:** executor can run the documented restore, but independent assurance and key custody remain separate | **Strong:** deterministic recovery plan plus host executor and independent restore/outcome assurance |
| T15 new cross-system workflow | **Rejecting failure:** encourages separate repo designs and prohibited direct edges | **Strong:** architecture/contract capabilities follow truth ownership, but can become broad | **Conditional:** shared builders can implement after one architecture authority locks the journey and contracts | **Strong:** one accepted outcome, graph-bound allowed edges, scoped executors, release, and end-to-end QC |
| T16 credential/privacy/authorization concern | **Weak:** repo owner may lack independence or expose the value while investigating | **Conditional:** security/access capability helps, but a permanent unrestricted security seat is unsafe | **Conditional:** bounded read-only security skill and deterministic redaction can work with separate consequential approval | **Strong/conditional:** containment and redaction are deterministic; app authority, security review, owner gate, and QC remain separated |
| T17 Canopy accounts/OAuth/identity | **Rejecting failure:** CRM and Canopy owners can each prove a different identity layer | **Strong/conditional:** account capability still needs CRM stable-ID and provider evidence | **Conditional:** requires exact account, identity, restriction, and credential-purpose skills | **Strong:** Canopy Account outcome with Data Sync, security, bounded implementation, and QC |
| T18 installed Canopy Desktop/updater | **Rejecting failure:** backend and desktop repositories can each close on component evidence | **Strong/conditional:** desktop capability still needs installed artifact and employee proof | **Conditional:** executor needs both desktop and backend transport context | **Strong:** Desktop outcome with Product/UI, release, installed-version evidence, and QC |
| T19 Telnyx consent/attempt/delivery | **Rejecting failure:** CRM success can stop at provider acceptance | **Strong/conditional:** messaging capability must preserve CRM consent authority | **Conditional:** safe only with no-send default and typed provider receipts | **Strong:** Messaging outcome with Data Sync, security, one-attempt receipt, final delivery, and QC |

## Structural findings

### Repository-aligned permanent agents

Repository expertise is valuable, but repository ownership fails as the primary
company organization. It naturally fragments CRM-to-satellite outcomes and can
reward partial "my repository works" evidence. Repository knowledge should
remain available through domain roles, skills, and execution lanes; it should
not define issue completion.

### Capability-aligned permanent specialists

Capability ownership handles data, media, fleet, release, and user-experience
boundaries better than repo ownership. Its weakness is permanent breadth: a
cross-company capability can accumulate too many repositories, tools, and
permissions. It needs strict evidence that persistent context materially
improves performance over bounded skills.

### Shared execution pool

A smaller pool is promising for routine implementation and can materially
reduce permanent seats and duplicated context. It is not sufficient alone for
Phone Farm fleet work, high-risk identity authority, complex media/provider
flows, or independent assurance. Its viability depends on high-quality bounded
skill packets and the machine-readable company graph.

### Layered outcome ownership

This structure fits the most scenarios because it separates four different
questions:

1. Who remains accountable for the requested business result?
2. Who has the bounded expertise and host access to implement the current step?
3. Who proves release/runtime truth?
4. Who independently proves the accepted outcome?

However, an unrestricted matrix would recreate the handoff explosion. The
structure is viable only if the controller selects a small fixed route from the
system graph, sends compact packets, prevents duplicate authority, and keeps
Mika out of routine movement.

## Provisional recommendation for independent challenge

The analyst pass recommends a **bounded layered organization**, not the vague
"hybrid" previously suggested in conversation:

```text
Owner report
    |
    v
Deterministic intake record + bounded system subgraph
    |
    v
One accountable business/domain outcome owner
    |
    +--> one primary executor with repo/host skills
    |        (additional contributor only when a named boundary requires it)
    |
    +--> Release/Runtime stage only for a consequential release
    |
    `--> Independent outcome verification
```

This recommendation differs from simply combining repo and feature agents:

- the issue owner is accountable for the business journey, not a repository;
- repository knowledge is usually a scoped skill/execution capability, not a
  second competing owner;
- persistent specialists are retained only when repeated scenario evidence
  proves that durable domain context is necessary;
- deterministic controller stages replace conversational coordination;
- release and independent verification remain distinct authorities;
- the route is capped rather than allowing arbitrary agent collaboration.

## Candidate permanent role families to test

This list is an evaluation shortlist, not a final roster.

### Likely persistent domain candidates

- CRM identity, access, and authority contracts
- LinkHub public customer journey and domains
- Media supply chain
- Phone Farm application on Mac Studio
- Phone fleet complicated subsystem on Mac Studio
- Canopy account/connection domain
- Canopy publishing/automation domain

### Roles requiring split/merge evidence

- Company Guide versus Mika/Chat
- Product strategy/design versus UI implementation
- Release execution versus runtime assurance
- Content Inspiration versus Telnyx messaging
- Canopy intelligence, analytics, AI behavior, and reporting
- Company Auditor versus issue-level Quality Checker

### Responsibilities likely implemented without permanent seats

- deterministic transitions, retries, timers, dedupe, decision binding, and
  notification admission
- critical Mac Mini monitoring and alert wording
- repository navigation and common build commands as selectively loaded skills
- permanent regression registry and blast-radius expansion
- routine bounded implementation through a qualified shared executor pool

## Required challenge questions

The independent review must try to falsify the provisional recommendation:

1. Which case still has unclear single accountability?
2. Where does an outcome owner become a coordinator with no unique expertise?
3. Which contributor could be removed without weakening the result?
4. Which persistent specialist can be replaced by a skill pack?
5. Which shared executor lacks enough continuity to diagnose safely?
6. Can a route exceed one owner, two contributors, release, and QC? If so, why
   is that not a new handoff loop?
7. Does any stage require Mika or Drew to relay evidence?
8. Can the same topology enforce the CRM hub/satellite boundary and Mac-only
   Phone Farm operation mechanically?
9. Can the system graph drift silently from code or runtime reality?
10. What scenario would make repository-aligned or capability-aligned ownership
    clearly outperform the recommendation?
11. Does bounded read-only triage resolve ambiguous ownership without becoming
    another coordinator?
12. Do route-DAG, coverage-state, architecture-assurance, and cross-issue
    dependency rules close the preflight findings without adding hidden
    complexity?

## Decision gate

Do not rewrite roles or build squads until one independent reviewer evaluates
the locked cases, this complete comparison, and the system/boundary contracts.
After one consolidated correction, a different final reviewer must return
approve/revise/reject with no unresolved high-severity authority conflict.
