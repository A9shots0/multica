# Browser-readiness slice: independent review

## Verdict

**One P2 correction remains before accepting the gateway.** Remove the upstream
`close` handler that destroys the downstream client. It can discard queued
response bytes after an otherwise normal upstream EOF. Two earlier P2 fixture
findings are corrected in the reviewed overlay. No P0/P1 finding, additional
authority expansion, or other concrete P2 defect was identified in this bounded
slice.

This is the one requested independent review, including the gateway added while
the review was open. Build completion, actual Docker ingress and Browser
acceptance remain separate checks owned by the parent task.

## Snapshot and scope

- Date: 2026-09-15. Gateway/fixture overlay read at 15:36–15:37 UTC; final focused
  transport check completed at **15:40:09.627 UTC**.
- Review worktree: `A:\Silver-Mediums-Workspace\audits\multica\browser-readiness-review-20260915`.
- Base: accepted synthetic snapshot
  `04b762a0eb51edaea7f506aee9fb890bf89a3e4e`, derived from accepted pilot work on
  committed HEAD `60fea34b9770bd8efbf6f6f0854d4b3ec26b3e00`.
- The local `BROWSER-READINESS-SLICE.json` lists the initial **16 paths**. This
  review additionally includes **3** late additions:
  `Dockerfile.local-candidate-gateway`, `docker/local-candidate-gateway.mjs`, and
  `docs/silver-mediums/LOCAL-CANDIDATE-README.md`: **19 total source/doc paths**.
- Scope: four-file meaningful-activity repair; Next.js/lockfile and web runtime
  OpenSSL patches; build telemetry setting; Docker evidence exclusion; isolated
  compose preview; source fingerprint and image receipt lifecycle; one-time
  static sample initializer; gateway ingress and runbook.
- Makefile and operational-workflow assertion changes were included for context
  as the already closed adoption correction. Accepted Item 14 source, manifests
  and frozen experiments were not reopened.
- Only report/evidence files were written in this review worktree. Root candidate
  and accepted source were read-only. No provider, model, production, credential,
  Phone Farm, Orca, seeder, application API or Browser operation was performed.

Definitions: **P2** means an actionable correctness defect in this local slice;
“corrected” below means the source correction is present, not that Browser
acceptance has occurred. Test counts are individual named cases or transport
variants, not repositories or complete-suite claims.

## Consolidated findings

### P2 — Preserve queued downstream writes on normal upstream EOF

**Location:** `docker/local-candidate-gateway.mjs:11`.

The gateway pipes upstream data into the client, then unconditionally calls
`client.destroy()` on upstream `close`. Normal EOF can close the upstream socket
while response data is still queued in the client's writable stream. Destroying
that client interrupts delivery. This affects any of the three transports,
including web responses and direct API/database access; a healthcheck with a
small fast response does not establish streaming correctness.

**Smallest correction:** remove
`upstream.on('close', () => client.destroy());`. Keep `client.pipe(upstream).pipe(client)`,
which ends the destination after source EOF, both socket error handlers, and the
client-close handler that cleans up its upstream connection. This does not add a
destination, credential, network or application behavior.

**Evidence:** `.review-evidence/gateway-eof-check.mjs` evaluates the reviewed
gateway's connection code, with its fixed destinations/listeners mapped to
ephemeral loopback sockets. The second pair introduces a controlled 50 ms delay
in the outgoing socket write to exercise a pending-write state. It runs no
workflow and accesses no application/database/provider.

| Transport variant | Expected bytes | Received bytes | Result |
| --- | ---: | ---: | --- |
| Reviewed code, ordinary slow reader | 16,789,561 | 16,789,561 | Complete |
| Close-destroy removed, ordinary slow reader | 16,789,561 | 16,789,561 | Complete |
| Reviewed code, controlled pending write | 12,345 | 0 | Truncated |
| Close-destroy removed, controlled pending write | 12,345 | 12,345 | Complete |

Thus **1/2 reviewed-code variants** fail; **2/2 proposed-correction variants**
deliver all bytes. This is a controlled transport reproduction, not a claim that
the actual Docker/browser session already exhibited truncation. Full output:
`.review-evidence/gateway-eof-check.json`. Node v22.13.1; reviewed gateway SHA256:
`9df9eda0d17200f80586de204dd5697d5869bb8e49a441c1d94ee24759a6c05e`.

### P2 — Fixture occupancy check queried a nonexistent table — corrected

The initial initializer queried `runtime`, which is absent in the accepted
schema; the actual table is `agent_runtime`
(`server/migrations/004_agent_runtime_loop.up.sql:1`,
`server/pkg/db/queries/runtime.sql:2`). This would stop seeding before local auth
or sample insertion on the fresh preview database.

The reviewed overlay now uses `agent_runtime` at
`scripts/local-candidate-fixtures.mjs:39`. Its zero-agents/runtimes/tasks and
other-workspace guards remain intact. Source correction confirmed; actual
initializer execution belongs to the parent.

### P2 — Fixture issue numbers did not advance the workspace counter — corrected

The initial initializer inserted issue numbers 1–3 directly without updating the
workspace counter. Normal issue creation increments that counter
(`server/pkg/db/queries/workspace.sql:60–63`), so the next ordinary creation would
retry number 1 and hit the issue workspace/number uniqueness constraint.

The reviewed overlay now updates the counter to `samples.length` inside the same
transaction, before COMMIT, at `scripts/local-candidate-fixtures.mjs:130–131`.
Existing sample issues already cause the one-time initializer to refuse a rerun
at lines 42–44, so this update does not renumber existing issues.

## Reviewed properties with no additional finding

### Meaningful activity

- Query uses immutable revision `1::bigint` for decision responses, reviews and
  evidence at `server/pkg/db/queries/workflow_v2.sql:1565–1569`. The canonical
  projection reader already represents those record types with revision 1.
  Generated SQL mirrors the query. There is no migration change.
- Cursor UUID parsing now returns an error at
  `server/internal/handler/workflow_meaningful_activity.go:96–99`; malformed
  input no longer reaches the panic helper before validation.
- Read-only inspection of root `activity-after.log` shows **6/6** focused cases
  pass, including all three immutable record kinds paged at a tied timestamp,
  malformed cursor UUID, identity binding and poisoned-row isolation. These were
  executed by the parent on its separate fresh PostgreSQL 17 test database;
  this reviewer did not rerun them or historical suites.

### Source/build evidence

- Next.js manifest/lockfile and web OpenSSL floor match the two isolated upstream
  patches already inspected in the upstream audit. No whole-upstream merge is
  part of this slice. This review does not independently attest the CVEs or all
  transitive dependencies.
- Builder `NEXT_TELEMETRY_DISABLED=1` is set before the Next build. Runtime has the
  same setting. `/.qualification-evidence/` is excluded from Docker context;
  preserved backups and sealed reports are outside the built application.
- Fingerprint covers the gateway Dockerfile explicitly and gateway source via
  the `docker/` prefix (`scripts/local-candidate-source.py:19–23`). The lifecycle
  script records and checks **3/3** built image IDs: backend, web, gateway
  (`scripts/local-candidate.ps1:33–41,51–63`). It rechecks source after building
  and before Start. The root's earlier in-build edit rejection is useful evidence
  that this gate is active; it is not a substitute for the final receipt.
- Fingerprint describes Git-visible build inputs, not an immutable Git commit or
  reproducible dependency/base-image rebuild. The image IDs bind the local build
  result. Provider profiles and evidence contents were not read by this review.

### Isolation and sample scope

- Compose app/backend/PostgreSQL have no published ports and attach only to the
  internal default network. Only the gateway joins both networks and publishes
  **3/3 ports to 127.0.0.1** (`docker-compose.local-candidate.yml:70–95`).
- Gateway forwards only frontend:3000, backend:8080 and postgres:5432; request
  content cannot choose another destination. Its non-root image, read-only
  filesystem, dropped capabilities and no-new-privileges setting are coherent
  with this task. The edge network does not give the three application services
  their own external interface.
- Startup explicitly uses the empty Compose environment file and fixed local
  configuration. The seeder now verifies the exact candidate gateway label and
  all three actual loopback bindings before its fixed local API/DB operations
  (`scripts/local-candidate-fixtures.mjs:12–19`).
- Samples explicitly label synthetic display rows and create zero agents,
  runtimes or queued tasks. They neither replay a workflow nor qualify a model,
  release, deployment or business outcome. Their issue/current-picture/inbox
  records are scoped to one sample workspace, with a transaction for row writes.

## Prioritized completion and limits

1. Apply the one-line gateway close-handler correction together with the two
   fixture fixes already staged by the parent. Keep this as the consolidated
   correction from this single review; no second review round is requested.
2. Finish the root-owned build receipt and Start checks for the final source.
   Verify actual loopback reachability through this engine's gateway topology,
   then run the one-time static initializer and built-in Browser acceptance.
3. Record those results in current continuation/progress evidence. Keep sealed
   Item 14 sources exact, and retain local-code, build, runtime and business
   acceptance as distinct claims.

Confidence is high for the three narrow findings and their source fixes,
moderate for the bounded integration assessment. No full-repository review,
exhaustive concurrency analysis, dependency/CVE audit, real provider activity,
Docker topology execution or Browser verification was performed by this reviewer.
The earlier healthy-but-unpublished internal-only containers and the final image
build are parent-owned runtime evidence. This review makes no deployment claim
and requires no additional user authority for the existing local task.
