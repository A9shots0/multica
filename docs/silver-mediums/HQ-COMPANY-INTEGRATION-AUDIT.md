# HQ company integration audit

Updated 2026-10-18 America/New_York.

Status: architecture audit plus production-readiness checkpoint. No HQ UI
implementation is authorized by this document. The current Multica control
panel and the rejected HQ prototypes are replaceable consumers, not foundations
of the company model.

As of 2026-10-18, protected source and current runtimes contain the first
control-panel-independent implementation: a Multica service catalog, normalized signal intake,
replay/order/conflict handling, deterministic incident correlation, evidence
lanes, an operating-picture API, a durable 8 GB watcher outbox, and a
privacy-safe Phone Farm support outbox. Automated tests pass for these bounded
components. The Multica company-operations foundation is merged, the privacy-
safe Phone Farm exporter is present in the healthy live Sidecar release, and
the 8 GB watcher's clean running branch contains its durable exporter. None is
actively delivering yet: source bindings, credentials, catalog rows, safe
network reachability and live acceptance are still required.

This audit turns the product definition in `HQ-WORKSPACE-DEFINITION.md` into a
current-system map. It distinguishes verified source behavior, historical
evidence, design decisions, and unknowns so future control-panel changes do not
force the company operating model to be rebuilt.

## Executive conclusion

Silver Mediums already has many of the hard pieces, but they are not yet joined
by one stable company contract:

- Brain inventories repositories and several live sources with cited receipts.
- Multica stores projects, issues, dependencies, agent runtimes, workflow
  evidence, external incidents, and outside-agent work receipts.
- Phone Farm support stores messages, conversations, grouped issues, incidents,
  recurrence, affected devices, evidence, and read-only audit jobs.
- The 8 GB watcher filters Railway logs, groups recurring incidents, stores
  local incident history, and can notify or prepare a bounded fix handoff.
- Railway snapshots already reach Brain independently of the 8 GB watcher.
- Task Control tracks current AI tasks and owner decisions during the temporary
  Dot/ChatGPT period.

The systems do not yet agree on canonical company entities, stable identities,
relationships, lifecycle states, or correlation. There is also no verified
end-to-end path that turns the 8 GB watcher's incident or a Phone Farm support
incident into one correlated Multica operating object.

The current HQ branch now closes two earlier contract gaps without choosing a
UI: the web client has typed bounded journey/blast-radius queries, and a company
service stores explicit validated system-node and journey IDs. Unknown IDs are
rejected; names, descriptions and visual proximity cannot create topology.

**Architectural decision:** build a control-panel-independent company model and
read-only adapters before building another HQ canvas. Source systems keep their
own authority. Multica owns the operational projection and owner workflow.
Brain preserves and retrieves cited evidence. Any current or future control
panel reads those stable records instead of becoming their source of truth.

## Evidence and freshness

### Current evidence inspected

- Brain company source registry read at 2026-10-16 22:16 UTC, registry ref
  `release:048768634600dfcfdf2f3331b07f4f387e7a9dfb596359845d762fef830168c4:src/smi/projections/company_source_registry.py`.
- Brain project registry and bounded Phone Farm relationship review case read at
  2026-10-16 22:19 UTC, latest Phone Farm receipt `source_event:220945`.
- Current Task Control dashboard read at 2026-10-16 22:16 UTC.
- Current protected Phone Farm `origin/main` at
  `b914089a8fa3a2ade2dc2e1f0d657f9de8f42670`.
- Current Multica source in the isolated HQ worktree at
  `685f1de7a4f2a728b5a66c3555061354452332a9`, including migrations 591-602 and
  621-624, external incident handlers, external work observers, and existing
  project, issue, dependency, agent, Inbox, and deployment types.
- Private `A9shots0/railway-ollama-alerts` default branch source, inspected
  through the authenticated GitHub API on 2026-10-16.
- Railway account project names and IDs returned by the Railway integration.

### Important limits

- Brain says its registry is not a complete census of every company system or
  AI runtime.
- Brain currently classifies 17 account repositories for collection, leaves 20
  accessible repositories ownership-unclassified, and excludes 3. Other
  GitHub accounts and organizations have not been inventoried.
- Human stewardship is unverified for every source selected in this audit.
- A source receipt proves only its declared source, scope, and window. It does
  not prove the business outcome.
- Railway project listing succeeded, but live service listing and direct
  environment inspection were unavailable because the present Railway MCP
  credential is not authenticated. Brain's hourly Railway receipts therefore
  supply the freshest service-count evidence in this audit.
- The 8 GB watcher source is current, but its enrolled Brain project explicitly
  lacks current-runtime and active-owner-handoff proof.
- Phone Farm relationship history is not complete. Brain reports 894 projected
  relationships, 133 quarantines, 190 review cases, and no history-complete
  checkpoint. This is relevant evidence, not permission to replay or promote
  those records.
- The existing Intelligence-to-Multica incident receiver is present in current
  Multica source. The retained handoff says the producer delivery worker,
  production binding, credential installation, and live acceptance were still
  separate work. Current production activation was not established here.

## Stable company model

These concepts should survive a Multica redesign, a different control panel,
or a future replacement of the current UI.

| Stable object | What it means | Authoritative owner |
| --- | --- | --- |
| Company area | A durable business or product area such as Phone Farm, CRM, Brain, or Multica | Multica company catalog, initialized from owner-approved company structure |
| Project | A longer-running organized effort with resources and planning | Multica Projects |
| Outcome | One business result the company is trying to achieve | Multica operational model, usually linked to a project |
| Work item | A bounded problem, task, or stage contributing to an outcome | Multica issue/workflow records |
| Agent or runtime | A configured worker and the machine/provider capable of running it | Multica Agents/Runtimes plus provider receipts |
| Service | A technical or business service with a stable identity and environment | Source platform; projected into Multica catalog |
| Deployment | A release attempt and exact deployed or rollback identity | Deployment/release system |
| Incident | One operational episode affecting a service, workflow, device, account, or outcome | The admitted incident projection; source observations remain evidence |
| Support report | One employee/customer report or occurrence | Phone Farm support source; HQ receives a privacy-safe projection |
| Evidence | A cited observation, receipt, test, log window, support occurrence, or human confirmation | Original source plus immutable reference; Brain may index it |
| Decision | A real owner choice, policy boundary, or missing authority | Multica owner workflow/Inbox |
| Workspace layout | Positions, groups, pins, visibility, and camera state | HQ workspace document only; never business truth |

### Required identity fields

Every projected object needs:

- a stable Multica ID;
- source system and source-native ID;
- source-observed time and Multica-received time;
- source evidence reference;
- authority and confidence class;
- freshness or expiry;
- lifecycle state and revision;
- links to company area, project, outcome, service, environment, and affected
  entity when supported by evidence;
- privacy classification and retention rule.

The UI may change names, grouping, layout, and visual treatment. It must not
change these identities or fabricate relationships from proximity.

## Current system audit

### 1. Brain

What is useful now:

- a live source registry with per-source receipts, coverage, counts, collection
  routes, and explicit unknowns;
- current GitHub and commit-graph collection for the important repositories;
- hourly Railway production-environment snapshots for five known projects;
- Phone Farm relationship and CRM post-mirror evidence;
- bounded evidence retrieval and enrolled project identities.

Current Railway registry evidence:

| Project | Declared services | Registry state | Latest receipt |
| --- | ---: | --- | --- |
| PhoneFarm-Uptime | 4 | Unknown; latest receipt incomplete | `source_event:220712` |
| SilverMediums-CRM | 12 | Current | `source_event:220728` |
| Linkhub | 2 | Unknown; latest receipt incomplete | `source_event:220731` |
| VCP | 2 | Current | `source_event:220754` |
| AdminCRM | 3 | Unknown; latest receipt incomplete | `source_event:220758` |

Brain also knows Railway projects named `style-builder` and `williams-media`,
but no matching production source rows were established in this audit.

What Brain is not:

- the owner-operating UI;
- a complete inventory;
- a live execution controller;
- proof that a reported task or business outcome is complete;
- a semantic index of every Codex, Claude, or ChatGPT conversation today.

HQ use: Brain is the evidence and retrieval layer. Its source registry can seed
catalog discovery and warn about gaps. Multica must not silently turn a Brain
receipt into accepted operational truth.

### 2. Multica

What exists in current source:

- projects with status, priority, lead, dates, repository/local-directory
  resources, and issue counts;
- issues with project membership, parent/child structure, priority, status,
  assignee, metadata, custom properties, and Current Picture projections;
- persisted issue relationships for `blocks`, `blocked_by`, and `related`;
- agents and runtimes with status and capacity;
- workflow records for scope, outcomes, authority, executions, decisions,
  reviews, releases, evidence, incidents, budgets, attention, and current
  picture;
- provider-neutral structured outside-work receipts attached to exact issues;
- an authenticated external-incident receiver with durable replay, ordering,
  quarantine, episode, and human-override behavior;
- Inbox, Chat links, deployments, and Brain context seams.

Important gaps:

- `project` is a useful planning container, not yet a full company area or
  business-service model;
- each issue has one `project_id`; cross-project contribution needs an explicit
  relationship instead of invented multi-project membership;
- no general service/environment/dependency catalog exists;
- no first-class support-report, affected-person, affected-device, or customer-
  impact projection exists in Multica;
- issue dependencies exist in storage but are not yet a complete HQ graph/API;
- external work is structured receipt intake, not automatic discovery or full
  transcript ingestion;
- external incident intake does not by itself prove the 8 GB watcher or Phone
  Farm support is connected in production;
- no accepted HQ workspace document contract exists yet.

HQ use: Multica should own the normalized operational projection and owner
workflow. Existing issues, workflow evidence, receipts, and external incidents
should be extended, not copied into a second dashboard database.

### 3. Task Control and Dot

Task Control currently provides a useful temporary list of AI tasks, statuses,
events, and owner decisions. The current read contained active, completed,
waiting, and needs-attention tasks across Brain, Phone Farm, Multica, LinkHub,
CRM, X outreach, Railway cost, and other work. Its 2026-10-16 snapshot contained
31 task records: 14 `active`, 11 `complete`, 4 `waiting_for_owner`, and 2
`needs_attention`, plus 4 open decisions. These are dashboard records, not proof
that 14 agents or processes were actually running.

It also contained decisions that are no longer good company truth, including an
open HQ decision asking the owner to approve the abandoned Inbox history-search
preview. This proves Task Control needs lifecycle cleanup and cannot be the
canonical company model.

HQ use during transition:

- read task status and dated evidence through a bounded adapter;
- mark the source as temporary and provider-reported;
- correlate only to an exact Multica issue/outcome or explicit external work ID;
- do not import Dot's page layout, decision wording, guessed progress, or stale
  task state as permanent structure.

When Multica becomes the owner's normal control plane, this adapter can be
retired without changing HQ's stable objects.

### 4. 8 GB Railway watcher

Current source behavior:

- streams Railway JSON logs through the Railway CLI;
- uses deterministic patterns before any model call;
- groups suspicious lines by service and incident fingerprint;
- suppresses expected/transient noise and rate-limits repeats;
- uses local Ollama only for a short summary after deterministic filtering;
- persists incident, recurrence, fix-attempt, GitHub issue, and notification
  state in local files;
- retains service, repository, likely-file, raw-sample, severity, first/last
  seen, occurrence, and recurrence fields;
- can notify through Twilio or Telegram, optionally create GitHub issues, and
  prepare a bounded Windows-agent fix prompt;
- exposes local health and a local dashboard;
- defaults to dry-run and caps recurrence/fix retry behavior.

What is missing for HQ:

- stable service/environment IDs aligned with the company catalog;
- a production source binding to Multica;
- renewed provider discovery for expanding the target catalog; and
- current accountable-owner proof.

The running 8 GB branch includes the versioned bounded projection, exact
service mapping, privacy-safe evidence references, and durable acknowledged
outbox. Its current target database contains no PhoneFarm-Uptime services, so
it cannot truthfully supply a Phone Farm application-log lane today. It remains
healthy and receives current logs for its existing target set. Production
Multica configuration and live proof remain missing.

The watcher must remain a signal producer. It should not create company
outcomes, steer Multica, or become the system of record. Its local incident
fingerprint and recurrence memory are useful source identities, but Multica
decides whether a new observation updates an existing company incident.

### 5. Railway platform snapshots and PhoneFarm-Uptime

The company has two different operational evidence paths:

1. Brain's Mac Studio collector takes hourly Railway service snapshots.
2. `PhoneFarm-Uptime` is a Railway-hosted service that probes the public Hub,
   Sidecar, and a real R2 gallery object every five seconds.

`PhoneFarm-Uptime` persists a durable incident sequence and delivery state,
groups a broad host failure for SMS, keeps target-level Telegram behavior, and
requires sustained healthy probes before rearming or closing. It does not
authenticate to Phone Farm or operate devices.

HQ should not collapse these sources prematurely:

- Railway snapshots describe declared deployment/service state.
- The 8 GB watcher describes suspicious application logs.
- PhoneFarm-Uptime describes external availability and one real storage read.

They become separate evidence lanes on one incident when correlation supports
the same service, workflow, and time window.

### 6. Phone Farm support

Current protected source contains a mature support domain:

- durable inbound messages and conversations;
- CRM-authoritative employee identity where available;
- affected phone/device resolution with ambiguity preserved;
- one occurrence per actionable report;
- deterministic grouping by reporter, confirmed device, incident family,
  scope, compatibility, and time;
- recurrence, same-device/different-reporter correlation, and cross-device
  likely-systemic detection;
- support incidents with status, summary, next action, linked issues, and
  conversation workflow;
- attachments and evidence with protected media storage;
- a separate read-only Codex audit worker with durable jobs, events, steering,
  cancellation, budgets, and safe reduced results;
- admin-only APIs for messages, conversations, incidents, tracking summary,
  and agent state.

The source already defines a future-facing `SUPPORT_DOMAIN_CONTRACT_VERSION =
1` with record types for conversations, messages, reports, issues, incidents,
affected entities, evidence, triage and relationship suggestions, corrections,
execution links, and verification receipts. Its enumerations also define
authority, evidence, execution, and verification semantics. The contract is
currently an additive validation boundary; it is not an HQ delivery pipeline.

What is missing for HQ:

- explicit Multica workspace/project/company-area binding;
- deterministic mapping from Phone Farm device/account/workflow identities to
  the company catalog;
- a correlation receipt when a support incident attaches to an operational
  incident or existing outcome;
- a closure receipt tying deployment/recovery evidence back to the affected
  support occurrences;
- observed volume and retention measurements suitable for HQ scale testing.

The local implementation now includes a restricted projection that
pseudonymizes reporter/device references with an HMAC salt, refuses incidents
without a deterministic failure family, and durably retries delivery from
MongoDB. Production policy/configuration and live acceptance remain missing.

Raw support text must stay in Phone Farm. HQ receives a short safe summary,
counts, affected-entity references, severity, time window, source links, and
verification state. Authorized drill-in can open the protected source view.

### 7. External ChatGPT, Claude, Codex, Copilot, and EVO work

Multica already has the correct narrow foundation:

- `observe-codex` reads one explicitly linked Codex rollout file and one exact
  Task Control task;
- `observe-stream` accepts the same structured receipt contract for other
  approved providers;
- receipts attach to one exact issue/provider/work ID and remain a reported
  claim until Multica accepts the business result;
- Brain context requests are bounded by Multica workspace/project authority.

This does not discover every active chat, parse ordinary conversation prose, or
provide full semantic conversation history. Those are separate History/Brain
capabilities. HQ should show only correlated current activity and evidence, not
pretend every chat is a canonical worker or issue.

## Source-of-truth boundaries

| Fact | Source of truth | HQ behavior |
| --- | --- | --- |
| Repository/ref/CI | GitHub | Link current evidence; do not infer deployment |
| Railway service/deploy state | Railway, captured by a cited collector receipt | Project into service/deployment records with freshness |
| Watcher observation | 8 GB watcher | Attach as signal evidence until admitted into an incident |
| Public Phone Farm availability | PhoneFarm-Uptime | Attach availability evidence to the relevant services |
| Support message and reporter identity | Phone Farm support/CRM identity path | Keep raw source private; project safe impact facts |
| Phone/device/account relationship | Phone Farm or CRM domain source | Preserve confidence, revision, and unknown/quarantine state |
| Work and owner decision | Multica workflow | Display directly in HQ and Inbox |
| Outside agent progress | Structured provider receipt | Display as reported, never silently accepted |
| Accepted company knowledge | Reviewed Brain evidence | Cite it; do not treat stale records as live status |
| Visual placement | HQ workspace document | Never create business relations from geometry |

## Normalized signal contract

Every watcher, support, deployment, and provider adapter should emit the same
small envelope before source-specific data is considered:

```text
contract_version
event_id
source_system
source_instance
source_native_id
source_sequence or source_revision
observed_at
received_at
event_kind
lifecycle_state
severity
action_required
summary
affected_refs[]
service_ref / environment_ref
project_ref / outcome_ref when explicitly known
evidence_refs[]
predecessor_event_id when ordered
privacy_class
raw_payload_hash
```

The envelope is not a universal raw-event dump. Each adapter keeps a bounded
source payload, validates exact allowed fields, and stores only safe projections
in Multica.

## Deterministic grouping and correlation

### Same-source grouping

- Watcher: source instance + environment + service + fingerprint + episode.
- Railway snapshots: project + environment + service + deployment identity.
- PhoneFarm-Uptime: monitor target + durable incident sequence.
- Phone Farm support: authoritative reporter when available + affected entity +
  incident family/scope + compatibility + session window; preserve uncertainty.
- Outside AI work: provider + exact external work ID + run/receipt revision +
  linked Multica issue.

### Cross-source correlation

Deterministic identifiers come first:

1. exact service/environment or device/account/workflow identity;
2. exact deployment or release identity;
3. explicit existing issue/outcome/incident link;
4. overlapping observation window plus compatible failure family;
5. only then a reviewable AI suggestion.

AI may propose a relationship with evidence and confidence. It may not merge
incidents, rewrite source identity, or close work on its own.

### Card creation rule

Signals do not become cards. A new HQ-visible incident is created only when:

- a trusted deterministic source opens a new episode under its admitted
  contract; or
- the controller accepts a triage candidate after deterministic correlation;
  or
- an authorized human creates or confirms it.

Repeated logs, support messages, health probes, and deployment events update
the existing incident and its evidence lanes.

## Relationship vocabulary

The first shared relationship types should be small and typed:

- `belongs_to_area`
- `belongs_to_project`
- `contributes_to_outcome`
- `parent_of` / `supports`
- `depends_on` / `blocks`
- `runs_as_service`
- `deployed_by`
- `affects_service`
- `affects_device`
- `affects_account`
- `reported_by_support`
- `observed_by_monitor`
- `worked_by`
- `discussed_in_chat`
- `produced_artifact`
- `verified_by`

Every edge stores its source, evidence reference, revision, and authority. A
shared project or agent does not by itself create a dependency edge.

## Control-panel independence

The current Multica shell, Dot dashboard, rejected HQ prototypes, and future
control panel may all change. To keep the architecture replaceable:

1. Adapters write stable domain records through versioned backend contracts,
   never through page-specific UI state.
2. The workspace stores only presentation state and references stable IDs.
3. Chat, Inbox, HQ, History, Projects, Analytics, and Deployments are separate
   projections over the same records.
4. No page owns incident correlation, progress truth, source freshness, or
   acceptance state.
5. A new UI can replace the current control panel without migrating watcher,
   support, Brain, or agent source data again.
6. Transitional adapters, especially Task Control/Dot, declare an expiry and
   cannot become required infrastructure accidentally.

## Capability matrix

| Needed HQ capability | Current foundation | Readiness |
| --- | --- | --- |
| Company/project inventory | Brain registry + Multica projects | Partial; company ownership and purpose coverage incomplete |
| Outcome/work hierarchy | Multica issues, parent links, workflow outcomes | Explicit work-to-topology binding implemented; dedicated Outcome decision remains open |
| Dependency graph | `issue_dependency` + Current Picture evidence + work topology bindings | Typed bounded reads and explicit work-to-system mapping implemented; broader company vocabulary remains incomplete |
| Agent/runtime activity | Multica agents/tasks + outside receipts | Typed bounded current-effort projection implemented; live external discovery remains incomplete |
| Service/deployment catalog | Railway receipts + new Multica `company_service` contract | Local service model implemented; catalog bootstrap and deployment identity remain |
| Watcher incident ingestion | New normalized intake + watcher durable outbox | Local path and tests exist; source binding/live proof missing |
| Support impact | Phone Farm incidents + restricted Mongo outbox | Local path and privacy tests exist; source binding/live proof missing |
| Correlation across support/monitor/deploy | Deterministic service/environment/failure key + evidence lanes | Watcher/support proven locally; deployment and live multi-source proof remain |
| Owner attention | Multica Inbox/workflow attention | Exists; stale Task Control cleanup needed |
| History/search | Brain evidence + structured outside receipts | Partial; cross-agent semantic session history missing |
| Spatial workspace | Product definition only | Intentionally frozen until data contracts are proven |

## Recommended build order

### Phase 1 — freeze UI and finish contracts

1. Approve the stable entity and relationship vocabulary.
2. Define the versioned service identity and normalized signal envelope.
3. Define authority, freshness, privacy, retention, and source ownership for
   each adapter.
4. Decide whether `Outcome` is a dedicated Multica record or a strict role of
   the current issue/workflow model. Do not create a parallel copy.

### Phase 2 — read-only company catalog

1. Add service/environment records and typed relationships in Multica.
2. Import the current Brain source registry as cited discovery evidence.
3. Map Railway projects/environments/services and Phone Farm technical services
   to stable IDs.
4. Expose one API that returns company areas, projects, outcomes, services,
   agents, and typed edges without layout concerns.

### Phase 3 — two real signal pilots

1. Add a durable, versioned 8 GB watcher sender to the existing Multica
   external-incident receiver.
2. Add a privacy-safe Phone Farm support incident projection and outbox.
3. Admit both against their correctly bound services. Use a provider-free
   Phone Farm fixture to prove cross-source correlation without claiming that
   the current 8 GB target set observes the Sidecar.
4. Prove duplicate, stale, out-of-order, recurrence, recovery, and source-
   unavailable behavior without model calls.
5. Show that many source events produce one incident with separate evidence
   lanes and that support closure receives a verification receipt.

### Phase 4 — operating projections

1. Build compact APIs for owner attention, active outcomes, current incidents,
   current responders, support pressure, deployments, and freshness gaps.
2. Keep Inbox actionable and HQ explanatory.
3. Add History/search links without copying raw source content into HQ.

### Phase 5 — real UI directions

Only after the first four phases are proven on real data:

1. use the same dataset for three materially different dark-mode directions;
2. evaluate company scan, selected-item coordination, incident impact, scale,
   mobile/narrow access, and owner organization;
3. choose the interaction model before committing to a canvas engine;
4. run independent UI acceptance on the real preview.

## Decisions still required later

These are genuine product or trust decisions, not blockers for the present
audit:

- whether Outcome becomes a first-class record or a strict issue/workflow role;
- which support fields may appear outside the protected Phone Farm surface;
- retention windows for support projections, log evidence, and external session
  history;
- which trusted sources may automatically open an incident versus only attach
  evidence;
- whether any incident class may start predefined diagnosis or recovery without
  owner input;
- the eventual workspace interaction direction and canvas engine.

## Immediate acceptance test for the architecture

Before HQ UI work resumes, a provider-free fixture must prove this story:

1. PhoneFarm-Uptime reports the public Sidecar unavailable.
2. A bounded watcher fixture reports a matching Sidecar failure using the exact
   service, environment and deterministic failure-family contract. This tests
   correlation; it is not evidence that the current live watcher monitors the
   Sidecar.
3. Three Phone Farm employees report the same workflow failure from two phones.
4. One recent deployment is linked to the affected service.
5. Multica creates or updates one incident, not six cards.
6. The incident shows three evidence lanes, affected people/devices, the current
   responder, linked work, deployment, next action, and verification state.
7. Inbox contains only a real owner decision if one is required.
8. Recovery evidence closes the operational episode and returns verification to
   the support occurrences.
9. Brain receives the accepted result with immutable references after review.
10. Replacing the UI projection leaves all records and relationships intact.

That test can disprove the architecture before another polished interface hides
the gaps.

## External patterns informing the decision

- Backstage catalog entities and typed relations support a source-mirroring
  company catalog, but not the owner workflow or canvas:
  https://backstage.io/docs/features/software-catalog/
- Backstage's catalog graph reinforces explicit typed relations rather than
  visual proximity:
  https://backstage.io/docs/features/software-catalog/creating-the-catalog-graph/
- Prometheus Alertmanager grouping and inhibition support deduplicating many
  observations before owner interruption:
  https://prometheus.io/docs/alerting/latest/configuration/
- Grafana incident routing and escalation guidance distinguishes many alerts
  from one coordinated incident:
  https://grafana.com/docs/grafana-cloud/observe-and-act/respond-to-incidents/introduction/routing-and-escalation/
- PagerDuty business and technical service dependencies inform impact links,
  not the entire product model:
  https://support.pagerduty.com/main/docs/service-dependencies
- Linear Customer Requests demonstrates many support reports linked to one
  issue or project rather than one work object per message:
  https://linear.app/docs/customer-requests
- OpenTelemetry service resource conventions provide stable service,
  namespace, instance, and deployment identity semantics:
  https://opentelemetry.io/docs/specs/semconv/resource/service/
- tldraw's store, persistence, bindings, and migrations are relevant only to a
  future workspace document after the operating model is proven:
  https://tldraw.dev/docs/persistence
