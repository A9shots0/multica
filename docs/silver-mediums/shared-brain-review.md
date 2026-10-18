# Shared brain implementation review — 2026-09-24

Scope: this branch's shared-brain client, claim context, task broker, outbox/migrations and tests against base3d6d3acb. Self-review plus executable tests; independent owner-task integration review remains required before release. The installed review orchestrator's specialized bug/maintainability/simplicity/testability/coverage/docs skill files were not installed; those categories were assessed directly, without claiming those missing skills ran.

Findings corrected during implementation:

- Reading context through a personal global MCP alone would omit daemon runtimes with explicit MCP configuration. Added a local per-task MCP broker using Multica's authenticated server, with project frozen by canonical task binding. Shared bearer never enters daemon/model configuration.
- A polling-only lifecycle reader could miss a state overwritten by a quick retry. Added a transaction-bound task trigger; the anti-join reconciler recovers late commits and link timing. Stable event payloads survive send/receipt crashes.
- Projectless Chat context could refer to a selected issue in another project. Revalidate the captured owner-selected target and derive its project from the scoped canonical issue. Changed selection withholds context.
- Reviewed all new migrations against the no-FK/concurrent-index rules: each index has its own migration; dependent cleanup is explicit. New DB tests use testutil fixtures and testutil.Call. No real agent executable in default tests.
- Reviewed immutable workflow boundaries: server strips/excludes brain context, daemon rejects ambient payload and does not start a brain broker for guarded/controller packets. Ordinary context is per-turn, outside cached instruction files.

Checks: full schema migration through586 on a dedicated PostgreSQL18 container; real database transaction/replay tests; API workspace/daemon isolation and handoff conflict tests; HTTP credential/redirect/oversize checks; MCP transport round trip; gofmt and go vet for affected packages; focused prompt/plugin/workflow regressions. Exact outputs are private task evidence under C:/Dev/_work/brain-completion-evidence. No owner preview or production database was used.

Remaining release acceptance: configure real workspace/project mappings, exercise a fresh Multica task with server+daemon versions containing this change, observe its handoff from another device, and integrate with the owner branch. Cloud runtimes with no daemon identity need a separate enrolled transport. Mapping-file administration and blocked-outbox triage are currently operator workflows; a Multica settings/status surface may follow. No claim of production Multica release or company-wide completeness.

Independent owner review reproduced an acknowledgement gap: bare2xx with only schemaVersion was treated as delivered. Client.Send now requires a durable nonzero UUID event receipt with task_observation status and not_established acceptance. Missing, malformed, queued, or purportedly accepted receipts remain retryable and never set delivered_at. Exact initial/replay receipts still pass. Focused client/outbox tests and go vet passed; owner will rerun its independent reproducer and real-service acceptance.
