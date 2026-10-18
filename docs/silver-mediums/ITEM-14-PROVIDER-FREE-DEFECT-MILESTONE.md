# Item 14 provider-free defect milestone

- Snapshot: 2026-09-14 23:21 America/New_York
- Evidence stage: local-only
- Scope: isolated worktree correction and deterministic/fake-provider proof
- Real provider calls: zero
- Credential access: zero
- Production, outbound, Phone Farm, Browser, and Orca access: zero
- Permanent role/model/provider selection: none

## Preserved observation

The failed Item 14 integration pilot remains valid and unchanged. Its real
provider tasks are not retried, its exact-model guardrail is not weakened, and
its three frozen hub evidence files retain their recorded SHA-256 values. This
milestone works only in the isolated A: worktree and disposable PostgreSQL 17
database.

## Defects corrected

1. `implementation_ready` now admits actual typed root and per-worktree records,
   derives all counts and dirty/path arithmetic from those records, and binds
   their canonical SHA-256. Scalar claims alone are insufficient.
2. An accepted advancing workflow event remains authoritative even if its
   successor row is later absent. A subsequent process failure cannot reverse
   the durable handoff.
3. Workflow-owned live failures and sweeper failures bypass generic retry,
   delegated-failure recovery, missing-event fan-out, and generic issue reset.
4. Every provider start for an issue+squad workflow serializes under the plan
   row. The ceiling remains active after role mutation and fails closed when
   transaction support is unavailable.
5. `workflow_assignment_v1` binds every prompt-bearing issue/event/task field,
   the exact runtime/provider/model/reasoning selector, and the exact skill
   bundle snapshot. Claim reconstructs authoritative rows and rejects drift.
6. The daemon independently rejects ambient repository/project/comment/session,
   MCP/plugin/app, custom environment/arguments, runtime configuration, and
   service-tier surfaces. It stages one resolved executable, launches in a
   fresh task-local HOME/config/temp/empty workspace, uses an exact environment
   allowlist and tool-free policy, skips provider-native instruction files and
   model discovery, exposes the exact skill bytes in the bound prompt, and
   allows one turn with no resume, fallback, MCP, or repository token.
7. Plan block, issue status, technical-stop Current Picture, and owner Inbox
   admission now share one transaction. An injected Inbox failure rolls all of
   them back rather than publishing a partial stop.

## Independent review and correction discipline

One independent read-only review returned REVISE with six High and two Medium
findings: self-asserted inventory counts, sweeper recovery bypass, successor-row
dependence, role-mutation ceiling bypass, partial packet validation, ambient
child execution, missing transaction serialization, and non-atomic owner
projection. One consolidated correction addressed all eight. No second review
or second correction phase was introduced.

## Focused proof completed

- Artifact mutations fail before event/task creation.
- Whole-packet, skill, model, current-agent, and capability mutations fail at
  claim with terminal invalid-identity evidence.
- Accepted-event finality, role-mutation ceiling, nil-transaction denial,
  sweeper containment, and atomic rollback tests pass on disposable PostgreSQL
  17.
- Exact child-environment and resolved-skill tests pass.
- The complete existing controller/store -> claim -> daemon -> staged
  executable -> fake provider shadow passes with exactly one provider fixture
  launch and zero tools; altered, expired, revoked, duplicate, replayed, wrong
  executable, wrong task, and wrong package authority remains denied.

## Final verification and evidence boundary

- The complete 11-test controller/claim regression group passes, including all
  nested artifact and assignment mutations. The claim matrix uses an isolated
  runtime per case and no longer depends on mutable package-global runtime
  order.
- The four exact daemon child-boundary tests pass. The final complete guarded
  fake-provider shadow passes in 28.35 seconds with exactly one staged fake
  provider launch and zero tools.
- Full `workflowguard` and `workflowprojection` package tests pass. Matching
  `go vet` over `workflowguard`, `workflowprojection`, `service`, `handler`, and
  `daemon` is clean. `git diff --check` reports no whitespace error; its output
  contains only existing LF-to-CRLF warnings.
- SQL query generation completed successfully with sqlc and the affected Go
  packages compile through the focused and vet gates.
- Broader repository package runs were executed but are not a green acceptance
  lane. `service` still fails existing built-in-skill frontmatter/strict-YAML,
  legacy-redirect, and platform-description tests. `daemon` still fails
  host/profile-dependent CLI discovery, identity, user-local skill/MCP,
  Windows symlink/path, and worktree tests. `handler` still contains unrelated
  Windows path, Inbox/recovery, briefing, archive parsing, and workspace
  deletion-manifest baseline failures. The one new claim-matrix order
  dependency exposed by that run was corrected and its complete focused group
  rerun passed.
- Neither root nor `apps/web` has `node_modules`. No dependency installation or
  frontend/network gate was attempted; this correction is confined to the Go
  controller, service, claim, daemon, query, and test surfaces.
- Before shutdown, the disposable database contained no schema whose name
  matched Item 14, guarded, or shadow test patterns. The exact task-local
  PostgreSQL container was stopped and is absent.
- Frozen evidence was rehashed after all tests and still equals:
  `EVIDENCE.md`
  `e72d2f7217efb39992ee0e03f2ee94469685d1833e2c7b56b54856de644e4349`,
  `inventory.json`
  `a6b486f6ccbb01f2df090b734a0fdeb8311a9ca4d2bd3af341f53a3c69944bd6`,
  and `WINDOWS-REPOSITORY-WORKTREE-INVENTORY.md`
  `750cf7d03a6b22cdc964d64f83cb4634d56190d946d044a17650adfb6f890406`.
- The original dirty checkout remains on
  `codex/selfhost-pilot-20260909` at
  `60fea34b9770bd8efbf6f6f0854d4b3ec26b3e00`; it was not edited, cleaned,
  reset, stashed, switched, or used for this implementation.

## Natural stop

This provider-free defect milestone is complete. It does not establish a real
provider pass, model qualification, permanent role/squad/provider/seat
selection, deployment, or business acceptance. A new real workflow replay
requires a separate bounded owner grant; without that grant, the safe choice is
to preserve this local proof and the failed real-pilot observation unchanged.
