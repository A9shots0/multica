# Shared brain integration

Multica supplies authorized project context and records durable handoffs for ordinary agent runs. Brain context is reference evidence, never permission or task acceptance. Guarded/controller-owned runs retain their immutable input and capability boundary and receive no ambient brain context or tools.

## Configuration

Set `MULTICA_BRAIN_CONFIG` to a protected, server-local JSON file. Keep that file and the bearer credential out of Git, prompts, runtime environment overlays and logs. Example structure (placeholders only):

```json
{
  "url": "http://enrolled-private-host:8899",
  "tokenFile": "/protected/multica-brain-credential",
  "workspaces": {
    "canonical-multica-workspace-uuid": {
      "projects": {"canonical-multica-project-uuid": "enrolled-brain-project-id"},
      "defaultProject": "company"
    }
  }
}
```

Only an explicitly projectless task may use `defaultProject`; an unknown project never falls back. The server verifies workspace access and revalidates an owner-selected Chat target, then freezes brain project and target revision per task attempt. It reports missing configuration/binding/network evidence explicitly. A brain outage does not fail the run. Local daemons receive five scoped MCP tools: context, full immutable document read, paginated events, immutable evidence read, and queued handoff. The server holds the shared credential; the daemon retains only its existing workspace/daemon token. Full reads reject a document outside the bound project. Native/global MCP configuration is not required for this path.

## Durable delivery

The scoped tool catalog also includes `brain_events` and `brain_evidence`.
`brain_events` pages older full handoffs/lifecycle observations in the frozen
project; `taskId` is an optional exact filter, `cursor` is the last sequence
(initially0), and `limit` is1-100 (default20). Every event and any redundant
payload identity must agree with that scope. `brain_evidence` requires both a
UUID and exact64-character SHA256 revision, and rejects another project,
revision, truncated record or missing observation. A successful empty history
page is distinct from a service error. Follow `hasMore`/`nextCursor`.

The four read tools advertise read-only, non-destructive, closed-world,
idempotent MCP hints. Saving a handoff is an idempotent append and advertises
read-only false. These descriptions do not grant approval or override managed
provider policy. Any provider gate remains a separate runtime acceptance check.

Migrations **583–586**, in order, add the binding, outbox, two concurrent unique indexes, and a task-state trigger. The trigger saves lifecycle observations inside the same PostgreSQL transaction as the source transition. A reconciler also finds late records without a high-water mark. HTTP runs outside task transactions, every ten seconds with bounded batches/timeouts and persisted retry backoff. Crash-after-send retries retain the original key and body. Non-retryable client errors/conflicts are blocked for operator review. A removed workspace/project mapping blocks delivery. No raw worker output or prompts are copied; run references link to Multica's canonical evidence. Agents can additionally queue a concise redacted handoff with stable event UUID and source references.

`completed` always means a run observation. Neither it nor a queued handoff approves an issue, accepts an outcome, or certifies deployment. Handoff responses distinguish queued from delivered. Lifecycle rows and immutable source details remain available during brain outages.

## Operations and acceptance

Query `brain_event_outbox` for pending/blocked rows, age, attempts and generic error. Never fix a conflict by changing the event identity or overwriting its payload; inspect source and create an explicit correction. Keep the mapping file under deployment change control. Credentials travel directly to the enrolled endpoint without ambient proxies or redirects.

Verified locally with a separate PostgreSQL 18 fixture on port55439: all migrations, transactional rollback, one event per source/retry, immutable binding, workspace/daemon isolation, rejection of out-of-project reads, queued handoff replay/conflict. Unit tests also cover endpoint errors, redirects, response bounds, removed mappings, and immutable workflow prompt isolation. No installed agent CLI is invoked by these tests. Fresh real shared-brain delivery through this integration and owner-task merge/release remain separate acceptance gates.

This branch is based on Multica owner commit `3d6d3acb`; the owner's later UI-only `8ee884137` and preview ports3220/3221/55438 remain untouched. No production Multica deployment is claimed. Cloud runtimes without a daemon identity receive context but cannot use the local daemon MCP transport; that lane must be enrolled explicitly before claiming coverage.

Rollback: disable `MULTICA_BRAIN_CONFIG` to stop new retrieval/delivery while preserving durable rows. Keep migrations/data when rolling back application binaries. Dropping migration583 destroys queued handoffs and is a deliberate data deletion, not the routine rollback path.
