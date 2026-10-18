# Local candidate acceptance

**Result: ready for local browser review.**

Snapshot: **2026-09-15 15:49:43 UTC** (runtime/API/database), with source
rechecked at 15:50:56 UTC. Browser walkthrough: approximately 15:41–15:45 UTC.

- Preview: <http://127.0.0.1:13004/local-candidate-samples/issues>
- Run/stop instructions and synthetic sign-in: `LOCAL-CANDIDATE-README.md`.
- Candidate source ID: `60fea34b9770-local-6f0af10d34dd`.
- Build-input SHA-256:
  `6f0af10d34dd77de23dc5ff4a67ef1a79270b33671a8f9a89fe694b5303de16e`.
- Committed base remains `60fea34b9770bd8efbf6f6f0854d4b3ec26b3e00`.
  The suffix identifies 5,865 hashed local build inputs, not a new Git commit.

## Scope and preservation

The task adopted the accepted Item 14 delta into the existing isolated task
worktree, fixed the named activity blocker, applied two isolated upstream web
hardening patches, and built a provider-free local preview. Items 1–13 and the
completed Item 14 experiments were not reopened.

The independent adoption review verified **86/86** material paths, **1,380/1,380**
inherited dirty-file backups, **60/60** exact archived stewardship sources and
**3/3** manifests. The accepted A: source and original owner checkout remain
untouched. The 151 newline-only adoption differences were left alone. Later
sqlc regeneration churn in 57 unrelated files was restored to exact preimages.

The independent upstream audit found 103 upstream-exclusive commits, 14
pilot-exclusive commits, 21 textual conflicts against the complete accepted
snapshot, and 34 numeric migration-prefix overlaps. The migration ledger uses
full filenames; those prefixes alone are not duplicate ledger keys. Retain the
pilot base and accepted migration names. The blanket merge is deferred.

## Corrections and independent reviews

| Slice | One independent review | Consolidated correction and result |
| --- | --- | --- |
| Accepted adoption | `ADOPTION-REVIEW-20260915.md` | Frozen stewardship target now selects the archive; two stale assertions match accepted stage/response copy. 23/23 focused Views tests and 60/60 archive sources pass. |
| Browser readiness | `BROWSER-READINESS-REVIEW-20260915.md` | Sample preflight uses `agent_runtime`, the sample transaction advances `issue_counter`, and the gateway no longer destroys a client with queued response bytes on normal upstream EOF. Final images and manual SAMPLE-4 creation verify the corrected candidate. |

The review reports preserve their original findings. This acceptance record
closes the corrections; no second review round was requested. The gateway
review's controlled pending-write check delivered 0/12,345 bytes before the
one-line correction and 12,345/12,345 afterward. Its ordinary 16,789,561-byte
slow-reader case passed both variants; no ordinary-network failure is claimed.

Meaningful activity now assigns revision one to immutable decision responses,
reviews and evidence, matching the canonical record reader. Invalid cursor UUIDs
return a validation error instead of panicking. The original database 500 and
cursor panic were reproduced before their focused regressions passed.

## Verification

| Check | Observed result |
| --- | --- |
| Core schemas | 204/204 tests pass |
| Operational Views | 23/23 tests pass |
| Inbox, Chat, issue navigation consumers | 26/26 tests pass |
| Meaningful-activity database/cursor regressions | 6/6 pass on a fresh isolated PostgreSQL database |
| Controller success adapters | 3/3 pass |
| Go guard, projection, state and bounded daemon checks | Selected packages/cases pass; no provider process launched |
| Go vet | Handler and workflow projection pass |
| Build | Go binaries and standalone web build, including web TypeScript, pass |
| Current preview database | PostgreSQL 17.11; 603/603 embedded migration files applied, through numbered migration 574 |
| Current runtime | Health source ID matches the manifest; all three built container image IDs match the successful build receipt; four Compose services healthy |
| Web hardening readback | Next.js 16.3.4; installed libssl3 and libcrypto3 both 3.5.8-r0 |
| Activity API | 4/4 distinct sample records page exactly once at limit one, each with revision one; malformed UUID cursor returns HTTP 400 |
| Operational counts | 0 agents, 0 runtimes, 0 tasks, 0 chat sessions and 0 owner notifications |

### Built-in Browser walkthrough

The signed-in in-app Browser used the default 1280×720 viewport and synthetic
local data. No separate browser, desktop control or provider conversation was
used.

- **Sign-in:** the sample owner completed the normal local code flow. The
  resend cooldown also rejected an immediate repeat request as expected.
- **Board:** the stopped sample appears in Blocked, the completed local sample
  in Done, and unprojected samples in Status unavailable. Nothing is shown as
  Verified Live. The agent-working count is zero.
- **Issue:** the stopped sample shows the technical stop, “Inspect the technical
  incident,” “Your response is required,” and no active release. The newest
  review record supplies meaningful progress. Reload retained those facts.
  The UUID route canonicalizes to the human issue identifier.
- **Inbox:** two labeled sample notifications appear in incident/completion
  categories. Selecting the incident loads the linked owner panel and changes
  the observed unread count from two to one. Read flags in later database
  snapshots reflect subsequent interactive navigation, not an exclusive test.
- **Chat:** the intake guidance and empty history render. Starting a new chat
  shows that an agent is required and keeps Send disabled.
- **Manual creation:** switching New Issue to Manual creates SAMPLE-4 without
  a counter collision. Its Board and Now views retain Status unavailable and
  no verified progress, as expected without a workflow projection.

The three initial fixtures are static display rows. The fourth was created
through the actual UI/API. These observations prove the local product surfaces
and their data handling, not workflow execution or business completion.

## Runtime boundary and evidence

Compose project: `smk-local-candidate-20260915`. App, API and PostgreSQL have
only the internal network. A separate fixed-target TCP gateway publishes
127.0.0.1 ports 13004, 18084 and 15437. It has no application configuration or
credentials and forwards only to the three named internal services.

Builds use an empty Compose environment file. Startup rejects source or image
drift. The active gateway source hash matches the local corrected file:
`39760c1b9f220a3706509e1b8956900fce5488b4c0a9dd67d395e7a6d64b94a0`.
The separate focused-test PostgreSQL container on 15436 was stopped after its
exact task identity was checked. Preview containers and sample volumes remain
available; the README includes their stop command.

Evidence root: `.qualification-evidence/local-integration-20260915/`.
Primary artifacts: `boundary-inventory.json`, `adoption-receipt.json`,
`candidate-source-manifest.json`, `candidate-build-receipt.json`,
`runtime-verification.json`, `preview-fixtures.json`,
`independent-gateway-eof-check.json`, and the named test/build/start logs.
The exact frozen archive is
`.qualification-evidence/item14-accepted-checkpoint-20260915/`.

## Confidence, limits and next decision

Confidence is high in the measured adoption, focused repair, source/image
binding and observed local flows. This is not a full-repository clean-suite
claim: inherited broad qualification/platform test failures remain outside the
accepted continuation scope. Windows lacks `make`; the target's actual Go
stewardship command ran directly. Browser evidence covers the web viewport
above; native mobile/desktop and real provider behavior were not retested.

The fresh preview has no backup configuration or custody proof. Its recovery
panel remains unproven. The older accepted Item 13 record still lists pending
off-machine readback and separate recovery-key custody; neither was refreshed
or silently inferred complete here. The two upstream hardening patches do not
constitute an exhaustive vulnerability assessment.

**Remaining owner-only release gate:** authorize the production destination and
scope, choose its access policy, and supply applicable production secret/runtime
authority. Configure production authentication without the development code,
TLS/origins and durable recovery custody before external exposure. Real agent
execution requires its own current runtime/capability and credential authority.
The temporary R01-like preparation default expires exclusively at
2026-09-22 13:15Z and is not that authority.

No push, deployment, production-data write, outbound message, completed
provider/fake-workflow replay, permanent model/seat selection, Orca or Phone Farm
operation occurred. No remote-Git, production-runtime or real-business outcome
is claimed.

### Final closure check - 2026-09-15 15:55:26Z

The source manifest still matches the running reviewed candidate, all four
preview services remain healthy with the exact loopback mappings, and the
separate focused-test database is stopped. Focused tracked-file diff checks
pass. The knowledge hub was updated with a dated local-only packet and current
continuation pointer; `check_hub.py` passes with inherited stale-claim and older
registry/global-snapshot warnings. Those unrelated warnings were not promoted
into current production evidence. The in-app Browser preview is retained.
