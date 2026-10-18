# Item 14 topology trace ledger

- Scenario bank: `silver-mediums.topology-cases.v2`
- Lock hash: `sha256:d819bb8dd5d71bb4234ebdcf9707a45652fabb0a1a4a4301a59a421544fb25a3`
- Evidence stage: analytical architecture comparison, not runtime token proof

Each cell records `accountable actor | route DAG | estimated runs/handoffs/
largest packet/retries/owner interruptions | termination`. Packet estimates
are architecture ceilings for normalized evidence, not measured model tokens.
`R` is repository owner, `C` capability specialist, `B` bounded executor, `D`
boundary contributor, `A` architecture/security assurance, `L` release, and
`Q` independent review. Correction is capped at two. Owner interruption is zero
unless a genuine business decision remains.

| Case | Repository permanent | Capability permanent | Shared pool | Bounded layered |
| --- | --- | --- | --- | --- |
| T01 | CRM-R + LinkHub-R; 4/3/18k/1/0; reject partial owners | Identity-C -> LinkHub-C -> Q; 4/3/14k/1/0; public proof | triage-B -> two skills -> Q; 3/2/18k/1/0; conditional | LinkHub actor -> D -> B -> L -> Q; 5/4/9k/1/0; public proof |
| T02 | LinkHub-R -> L -> Q; 3/2/12k/1/0; customer proof | LinkHub-C -> L -> Q; 3/2/10k/1/0; customer proof | LinkHub-B -> L -> Q; 3/2/12k/1/0; conditional | LinkHub actor -> B -> L -> Q; 4/3/8k/1/0; customer proof |
| T03 | CRM-R + Canopy-R; 5/4/22k/1/0; reject component closure | Media-C -> app-C -> Q; 4/3/15k/1/0; lineage proof | media-B -> Q; 3/2/20k/1/0; conditional breadth | Media actor -> D -> B -> Q; 4/3/10k/1/0; same-item proof |
| T04 | Canopy-R + CRM-R; 5/4/20k/1/0; reject outbound-only | Outreach-C -> D -> Q; 4/3/14k/1/0; inbound proof | outreach-B -> D -> Q; 3/2/18k/1/0; conditional | Outreach actor -> D -> B -> Q; 4/3/9k/1/0; exact reply proof |
| T05 | PhoneFarm-R hides fleet; 4/3/18k/1/0; reject | App-C -> Fleet-C -> Q; 4/3/12k/1/0; Mac proof | generic pool prohibited; stop before Windows action | PF Mac actor -> Mac app/fleet -> Q; 4/3/8k/1/0; device+app proof |
| T06 | Repo-R -> L -> Q; 3/2/14k/1/0; conditional registry | Release-C -> domain-C -> Q; 4/3/13k/1/0; regression proof | B -> L -> Q; 3/2/15k/1/0; conditional | Domain actor -> B -> L -> Q; 4/3/8k/1/0; new+historic proof |
| T07 | VQ-R + CRM-R; 4/3/17k/1/0; reject local success | Content-C -> D -> Q; 4/3/12k/1/0; contract proof | content-B -> D -> Q; 3/2/16k/1/0; conditional | Content actor -> D -> B -> Q; 4/3/8k/1/0; visible proof |
| T08 | Repo-R -> L -> Q; 3/2/13k/1/0; conditional continuity | Release-C -> domain-C -> Q; 4/3/12k/1/0; scope open | B -> L -> Q; 3/2/14k/1/0; conditional | Domain actor -> L rollback -> B -> Q; 4/3/8k/1/0; recovered outcome |
| T09 | CRM-R overloaded; 4/3/19k/1/1; weak product decision | Reporting-C -> Product-C -> Q; 4/3/14k/1/0; capped | reporting-B -> Product-C -> Q; 3/2/17k/1/0; conditional | Reporting actor -> D + Product -> B -> Q; 5/4/9k/1/0; data+layout |
| T10 | no natural R owner; 4/3/18k/1/0; reject | Monitoring-C -> Messaging-C -> Q; 4/3/13k/1/0; conditional | deterministic monitor -> Telnyx/dedupe -> durable receipt; 0 LLM runs until later reconciliation | Platform actor -> external monitor -> Telnyx/dedupe -> durable receipt -> later Multica reconciliation; 0 immediate LLM runs/0 handoffs/0 packet/0 retries/0 owner queries; delivery and later reconciliation proof |
| T11 | named R anchors; 3/2/16k/1/0; conditional | neutral C -> domain-C -> Q; 4/3/13k/1/0; divergence | triage-B -> skill -> Q; 3/2/14k/1/0; conditional | triage binding -> D -> rebind -> B -> Q; 4/3/8k/1/0; root proof |
| T12 | local R patch; 3/2/16k/1/0; reject consumers | Contract-C -> domain-C -> Q; 4/3/13k/1/0; conditional | graph-B -> skill -> Q; 3/2/15k/1/0; deny unknown | actor -> A -> B -> L? -> Q; 4-5/3-4/8k/1/0; consumer proof |
| T13 | CRM-R -> Q; 2/1/13k/1/0; broad | no coherent generic CRM-C; 3/2/14k/1/0; weak | CRM-product-B -> Q; 2/1/10k/1/0; focused | CRM-product actor -> B -> Q; 3/2/7k/1/0; excludes identity |
| T14 | host/file/db R split; 5/4/22k/1/0; reject | Recovery-C -> Host-C -> Q; 4/3/14k/1/0; conditional | recovery-B -> Q; 2/1/15k/1/0; conditional | Recovery actor -> deterministic restore -> B -> Q; 3/2/8k/1/0; isolated proof |
| T15 | one R per repo; 6/5/25k/1/1; reject | Architecture-C -> contributors -> Q; 5/4/17k/1/0; broad | A -> B -> L -> Q; 4/3/18k/1/0; conditional | actor -> A -> B + D -> L -> Q; 5/4/9k/1/0; E2E proof |
| T16 | repo self-review; 3/2/16k/1/0; reject | Security-C -> domain-C -> Q; 4/3/12k/1/0; broad privilege | redacted security-B -> Q; 2/1/13k/1/0; no mutation | redaction -> A -> actor -> B -> Q; 4/3/7k/1/0; authorized proof |
| T17 | CRM-R + Canopy-R dispute; 4/3/18k/1/0; reject | Accounts-C -> Identity-C -> Q; 4/3/13k/1/0; provider proof | accounts-B -> D -> Q; 3/2/15k/1/0; conditional | Account actor -> D -> B -> Q; 4/3/8k/1/0; identity+restriction |
| T18 | desktop-R + backend-R; 4/3/19k/1/0; reject artifact closure | Desktop-C -> L -> Product-C -> Q; 5/4/14k/1/0; installed proof | desktop-B -> L -> Q; 3/2/16k/1/0; conditional | Desktop actor -> B -> L -> product evidence -> Q; 5/4/8k/1/0; employee proof |
| T19 | CRM-R treats acceptance as delivery; 3/2/17k/1/0; reject | Messaging-C -> Data-C -> Q; 4/3/12k/1/0; delivery proof | telnyx-B -> D -> Q; 3/2/14k/1/0; conditional | Messaging actor -> D -> B -> Q; 4/3/8k/1/0; one-attempt final proof |

Repository ownership automatically fails 12 cases through fragmented or false
component completion. Capability permanence is viable but overstaffed or
permission-broad in 13. The shared pool cannot replace the Mac-only fleet lane
and remains skill-depth conditional in 15. Bounded layering is the only
candidate without an automatic accountability or safety rejection across all
19 cases; its largest estimated route is five runs and four handoffs.

These are architecture estimates. Runtime qualification records measured model
tokens separately and cannot be substituted by this table.
