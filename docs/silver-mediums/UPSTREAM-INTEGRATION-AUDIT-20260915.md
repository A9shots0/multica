# Multica upstream integration audit

## Decision

Keep the accepted pilot base for the initial browser candidate. Adopt the exact
86 material source deltas identified by the integration boundary inventory,
repair the already-named meaningful-activity SQL defect, and verify the local
owner surfaces. A full upstream merge is neither necessary nor a smaller change.

The complete accepted source produces **21 textual conflict paths**, **68
same-path overlaps**, and **34 new migration-number collisions** against the
current upstream snapshot. A committed-HEAD-only comparison misleadingly merges
cleanly because most accepted implementation remains uncommitted.

This is a read-only source/Git audit, not the independent implementation-slice
review, a security certification, a provider qualification, or deployment
approval. No production, database, browser, provider, credential, outbound,
Phone Farm, or Orca operation was performed by this audit.

## Snapshot, scope, and evidence

- Measurement snapshot: `2026-09-15T14:59:39.519360+00:00`, also recorded as
  `observed_at_utc` in `divergence-quantities.json`.
- Comparison clone: `A:\Silver-Mediums-Workspace\audits\multica\upstream-integration-20260915T1450Z`.
- Upstream: `a843b44a48b4cb3d94c08e0c56ddef1fc1105257`, fetched by the integrator
  on 2026-09-15 from `https://github.com/multica-ai/multica.git`.
- Pilot committed HEAD: `60fea34b9770bd8efbf6f6f0854d4b3ec26b3e00`.
- Common ancestor: `9fab6da917953c6f1a967a17beef045aff3d0ce5`.
- Accepted source, read only:
  `A:\Silver-Mediums-Workspace\worktrees\multica\integration-preservation-pilot-20260914T232814Z`.
- Integrator boundary snapshot: `2026-09-15T14:54:12.414377+00:00`, at
  `C:\Users\a9sho\.codex\worktrees\3228\SMK-multica-selfhost-pilot\.qualification-evidence\local-integration-20260915\boundary-summary.json`
  and its full `boundary-inventory.json` sibling.
- Authoritative continuation: accepted `docs/silver-mediums/CURRENT-CONTINUATION.md:3`
  and `ITEM-14-INTEGRATION-PILOT-PROGRESS.md:679`; supervisory contract:
  `C:\Dev\Silver-Mediums-Knowledge\initiatives\company-engineering-foundation\MULTICA-TEAM-LEAD-CONTROL.md`.
- Reproducible local evidence: `audit_divergence.py`,
  `divergence-quantities.json`, and `accepted-merge-tree.txt` in this clone.

The synthetic accepted snapshot contains Git-tracked and nonignored untracked
source only. Ignored profiles, credentials, runtime state, dependency trees, and
retained external evidence volumes were excluded. A clone-owned alternate index
and unreferenced Git objects were used; neither source worktree nor its index
was modified. Both committed refs remain unchanged.

## Definitions and exact quantities

“Changed path” means a path changed from the common ancestor. “Overlap” means
both sides change that path; it is not automatically a textual or behavioral
conflict. “Textual conflict” means Git's default merge-tree reports an unmerged
path. “Migration-number collision” means different complete migration filenames
share the same numeric prefix. Existing historical duplicates at or below 128
are excluded from newly introduced collision counts.

| Measure | Numerator / denominator or count |
| --- | --- |
| Divergent commits | 14 pilot-only; 103 upstream-only |
| Committed pilot changed paths | 54 |
| Upstream changed paths | 1,002 |
| Committed pilot/upstream overlap | 7 / 54 pilot paths; 7 / 1,002 upstream paths |
| Committed pilot textual conflict paths | 0 / 7 overlaps; merge-tree exit 0 |
| Complete accepted changed/nonignored-new paths | 1,465 |
| Complete accepted/upstream overlap | 68 / 1,465 accepted paths; 68 / 1,002 upstream paths |
| Complete accepted textual conflict paths | 21 / 68 overlaps; merge-tree exit 1 |
| Accepted adoption gap from inherited checkout | 86 material paths: 38 content changes + 48 source-only paths |
| Adoption paths also changed upstream | 13 / 86 |
| Additional boundary differences | 151 line-ending-only paths; zero target-only paths |
| Common-ancestor up migrations | 485 files, including historical duplicate prefixes |
| Upstream up migrations | 519 files; 34 new, numbered 457-490 |
| Accepted up migrations | 603 files; 118 new, numbered 457-574 |
| New upstream migration-number collisions | 34 / 34 new upstream migrations; each collides with one accepted migration |
| New full migration-filename collisions | 0 / 34 new upstream migrations |

The integrator's source inventory contains 6,900 files, compared with 6,852 in
the inherited checkout. Its `-uall` status counts are 174 tracked dirty + 1,303
untracked for the accepted source and 125 tracked dirty + 1,255 untracked for the
inherited checkout. These status/file totals and the 1,465 paths changed from
the ancestor have different definitions and must not be interchanged.

### Reproduction queries

```text
git merge-base 60fea34b9770bd8efbf6f6f0854d4b3ec26b3e00 a843b44a48b4cb3d94c08e0c56ddef1fc1105257
git rev-list --left-right --count 60fea34b9770bd8efbf6f6f0854d4b3ec26b3e00...a843b44a48b4cb3d94c08e0c56ddef1fc1105257
git diff --name-only 9fab6da917953c6f1a967a17beef045aff3d0ce5 <side>
git merge-tree --write-tree --name-only <accepted-synthetic-commit> a843b44a48b4cb3d94c08e0c56ddef1fc1105257
python audit_divergence.py
```

The exact synthetic tree/commit and full path lists are in the JSON evidence.
No real merge was applied to either working checkout.

## Material integration hazards

### 1. Accepted source conflict surface is substantially larger than HEAD

The 21 textual conflicts span mobile Inbox API/state, core Inbox mutations and
queries, core type exports, web/desktop Inbox rendering, issue detail,
issue-page tests, the migration runner, Go dependency manifest, daemon handler,
Claude adapter, and generated database access code. Full paths are in
`accepted-merge-tree.txt` and the JSON evidence. These are owner-facing and
execution-authority boundaries, not merely documentation conflicts.

The 13 adoption/upstream overlaps are API schemas/tests, issue types, four issue
locale files, the daemon, agent and daemon handlers, task service, and agent SQL
plus its generated output. They should remain one accepted adoption slice, with
its own focused verification and independent review. No accepted source delta
needs replacement with upstream simply because both touch its file.

### 2. Migration collisions require a designed migration lineage

All new upstream numbers 457-490 are already used by different accepted
workflow migrations. For example:

| Number | Upstream | Accepted |
| --- | --- | --- |
| 457 | `task_message_output_truncated` | `issue_workflow_control` |
| 466 | `activity_log_member_assignee_frequency_index` | `issue_workflow_v2_records` |
| 469 | `issue_status_lifecycle_categories` | `issue_workflow_scope_key_idx` |
| 479 | `instance_telemetry_state` | `issue_workflow_command_key_idx` |
| 490 | `drop_triage_status_key_reservation` | `issue_workflow_decision_response_idempotency_idx` |

The complete 34-row mapping is retained in `divergence-quantities.json`.

The runner records the full basename, not just the number:
`server/internal/migrations/migrations.go:95` (`ExtractVersion`) and
`server/cmd/migrate/main.go:930`, `:942`. Thus a merge does not silently overwrite
an existing ledger key solely because the numbers match. It does create
ambiguous ordering and violates upstream's unique-number lint from 129 onward
(`server/internal/migrations/migrations_lint_test.go:13`). It also combines
independently evolved SQL objects and generated models.

Do not renumber already accepted/applied migrations as incidental cleanup.
A later upstream adoption needs an explicit forward migration mapping, the
matching runner, fresh disposable-database verification, and retained applied
history. No renumbering is needed to continue this local pilot lineage through
its existing migration 574.

### 3. Upstream lifecycle changes are an inseparable behavior family

`7dafc0cd6` changes 190 files to four stored lifecycle categories. It changes
status interpretation, board/list grouping, mobile, API schemas, daemon briefs,
handlers, service behavior, and migrations 469/470. Subsequent commits add
compatibility, GC/recovery fixes, ordering corrections, triage changes, and
maintenance/backfill control. Relevant follow-ups include `aac85ab26`,
`1b3137a00`, `9d18186e6`, `dddbb0336`, `ff60b2f25`, `7ebe0bf58`, and `02a8ff3e9`.

Upstream's own `docs/issue-status-lifecycle-rollout.md` requires its matching
migration runner, mixed-storage compatibility, and staged backfill. The current
runner explicitly skips historical migration 469 in favor of the additive 478
compatibility expansion (`server/cmd/migrate/main.go:418`). Applying SQL files
or selected status hunks without that family would produce a different system
from either accepted lineage.

The existing Item 14 success evidence binds the accepted workflow, projection,
runtime, and UI behavior. A broad lifecycle migration would require new focused
integration acceptance; it cannot inherit those results merely because the
role names and issue statuses still look familiar.

### 4. A blanket upstream startup adds default-on outbound telemetry

`dee1ed6bf` adds self-host telemetry, including server startup and four migrations.
`server/internal/selfhosttelemetry/config.go:15` documents and implements the
default-on behavior; only `DO_NOT_TRACK=1` or `true` disables it. That is a new
external-action surface relative to this task's authorization. Retaining the
pilot base avoids introducing it. Any later full upstream integration should
explicitly disable outbound telemetry for the isolated candidate before startup.

## Extension seams versus maintained fork integration

These are existing usable source contracts, not a proposal to rewrite the
accepted implementation during this phase. The plugin contract/capability files
have no changes between the common ancestor and this upstream snapshot; pulling
103 commits is not required to obtain these seams.

| Capability | Existing seam and evidence | Boundary |
| --- | --- | --- |
| Supplementary issue evidence display | Plugin `issue_panel` iframe and modal; `server/pkg/plugincontract/capabilities.go:27`, `packages/views/plugins/plugin-panel-section.tsx:21` | Suitable for optional evidence panels. Sidebar panels are declared in the schema but disabled by host capabilities. |
| Optional tools and role instruction resources | HTTP/MCP hooks, agent-trigger tools, skill resources; `server/pkg/plugincontract/manifest.go:4`, `capabilities.go:38` | Suitable for supplementary tools and static skill packets. Installation/tool grants remain explicit. |
| Informational event consumers | Seven versioned issue/comment/task events; `server/pkg/plugincontract/manifest.go:111`, `server/internal/service/plugin_event_bridge.go:54` | Asynchronous consumers can supplement reports; they cannot veto the originating transaction. |
| Scheduled refresh or reconciliation | Schedule trigger with a five-minute minimum; `server/pkg/plugincontract/manifest.go:57` | Suitable for coarse refresh. It is not a task claim/dispatch synchronization mechanism. |
| Scoped public issue/comment operations | Plugin action authorizer and request endpoints; `server/internal/service/plugin_action.go:46`, `server/internal/handler/plugin_action.go:350`, `:534` | A plugin can use authorized public operations; these endpoints do not expose the accepted normalized workflow ledger or replace its authority model. |

Three maintained integration areas remain necessary for the current accepted
requirements:

1. **Transactional controller state and admission.** Controller receipts,
   issue/workflow revisions, idempotency, task ownership, and review/release
   transitions must be checked before the relevant mutation or claim commits.
   Upstream explicitly keeps hooks out of the agent execution critical path;
   event dispatch can drop overflow and retains invocation records for seven
   days (`server/internal/service/plugin_event_dispatch.go:15-41`). It cannot
   substitute for the durable accepted ledger.
2. **Daemon capability and exact assignment enforcement.** The accepted
   `server/internal/daemon/plugin_hook_mcp.go:54`, `:276` validates each tool
   call against the controller-issued task capability scope. Accepted
   `workflow_assignment_runtime.go:28`, `:70` binds assignment identity and
   exact skill payloads. Optional plugin tools do not enforce these checks by
   themselves.
3. **Authoritative owner surfaces.** The accepted Board, Issue Now, Current
   Picture, Inbox, and read/write schemas are integrated into core/shared views
   and handlers. An optional iframe cannot make ordinary native Inbox or board
   status/revision behavior authoritative. Accepted issue detail imports
   `IssueNow` at `packages/views/issues/components/issue-detail.tsx:141` and
   passes the activity revision at `:3039`.

Keep these hooks small and documented, reuse public seams for new optional
capabilities, and assess extraction only when it removes real maintenance cost
without weakening accepted guarantees. Plugin migration is not a go-live gate
for this browser candidate.

## Exact upstream changes worth considering

No upstream commit is a prerequisite for the initial local browser acceptance.
Two isolated hardening changes are worth a separate bounded slice before
external exposure. The audit checked patch applicability against the complete
accepted synthetic source, without applying changes or running code:

| Priority | Exact upstream change | Concrete reason and disposition |
| --- | --- | --- |
| Before external exposure | `d30a96154e57b306b1863a1af0dc0d3648847168` | Raises web Next.js from `^16.2.5` to `^16.3.4`, with its matching lockfile. Upstream labels it security remediation. Patch applies cleanly to accepted source. Treat as an isolated dependency/build/browser verification slice; do not copy the whole upstream lockfile tip. |
| Before external exposure | `f0317cd0074b766797871be7f5e8b15c8d1f5ee2` | Adds a runner-image floor of `libcrypto3`/`libssl3 >=3.5.8-r0` in `Dockerfile.web`. One file, five lines; patch applies cleanly. Verify with the intended image build. |
| Before a future authorized Codex execution lane | `a00ebbb952f51e729d1d9ab61095c39167dc91ca` | Synchronizes `codexClient.threadID` across task/stdout goroutines. Two files; patch applies cleanly. It is irrelevant to a provider-free browser session and should not expand this phase into model testing. |
| Defer pending a separate usage-adapter slice | `cf52ba33ccf97f756de9e6b9d364fc0a57dc5260` | Stops double-counting Codex reasoning as additional output. Whole patch does not apply: accepted source lacks its expected `claude_usage_test.go` predecessor. Preserve recorded Item 14 usage; adopt later with its exact test dependencies and fixture-only checks. |
| Defer pending a separate usage-adapter slice | `e0b1f1cf1009aa1742dc9b3934eaf9dbad5a8d2f` | Deduplicates fallback Claude usage by response ID and excludes nested fallback accounting. Conflicts with accepted `claude.go` and its deadlock test. Do not substitute upstream over the accepted guarded adapter or reinterpret frozen CEF-7 receipts. |

These security labels are upstream Git claims, not an independently verified CVE
assessment. No broad dependency/CVE investigation was performed. Apply checks
prove patch context compatibility only, not build correctness or safety.

Other relevant upstream work, such as checkout preservation (`2441638c0`, 11
files) and delegated failure recovery (`dddbb0336`, 6 files), has a concrete
future value but touches execution and lifecycle behavior. It is not required
to adopt the accepted local checkpoint and should follow a named reproduced
defect or a separately bounded runtime-maintenance slice.

## Prioritized next actions and go-live boundary

1. **Now:** adopt the exact 86 material paths from the accepted-source boundary
   inventory. Keep all inherited work and accepted evidence. Avoid the 151
   line-ending-only copies. Preserve the accepted snapshot before updating
   mutable continuation/progress records.
2. **Now:** reproduce and correct the named missing-`dr.revision`
   meaningful-activity query defect, then run focused SQL/handler/projection
   checks and the single independent integration review with one consolidated
   correction. The historic 500 is documented in the accepted integration
   progress at lines 679-681; this audit did not reproduce it.
3. **Now:** verify the local Board, Issue Now/detail, Current Picture, and owner
   Inbox in the built-in Browser. Keep ordinary runtime dispatch denied under
   current authority. Do not replay completed Item 14 workflows.
4. **Before external exposure:** evaluate the two isolated web hardening patches
   above with their normal local build/browser checks. Record any unresolved
   image/dependency verification as a specific release gate.
5. **Later:** plan full upstream lifecycle and migration-lineage integration as
   its own scope. Keep the upstream telemetry switch and migration runner in
   that plan. Do not renumber retained applied evidence to make a merge appear
   clean.
6. **Owner authority only at the actual boundary:** production/push/deployment,
   credentials/provider execution, outbound operations, and broader/permanent
   business-policy choices remain separate. This audit creates no new grant.

## Confidence and limits

- High confidence in ref divergence, path intersections, migration filename
  counts, synthetic textual conflicts, and patch applicability: measured from
  exact local source/Git snapshots with retained queries/results.
- High confidence that existing asynchronous plugin contracts cannot replace
  the accepted transaction/claim/owner-projection controls without additional
  host work: the limitations are explicit in current source.
- Moderate confidence in the proposed minimal integration strategy: it follows
  the accepted evidence and minimizes semantic changes, but the integrator's
  focused checks, independent review, and Browser acceptance still determine
  candidate readiness.
- No full code review, full test suite, migration execution, build, browser
  session, vulnerability verification, runtime, provider, or business acceptance
  was performed by this audit. A clean textual apply is not any of those claims.
- The comparison is current only for the recorded upstream/accepted snapshots.
  A later upstream tip or changes to accepted source require a new bounded
  comparison, not reuse of these counts as permanent facts.
