# Local integration progress

Updated: 2026-09-15 15:49Z runtime snapshot; 15:50:56Z source recheck  
Stage: local candidate verified; owner release boundary  
Active worktree: `C:\Users\a9sho\.codex\worktrees\3228\SMK-multica-selfhost-pilot`

## Outcome

The accepted Items 1–14 implementation is locally integrated and browser-tested,
with its remaining go-live gate recorded. This file is the successor progress
record for integration after Item 14 qualification. Detailed acceptance:
`LOCAL-CANDIDATE-ACCEPTANCE-20260915.md`; preview/runbook:
`LOCAL-CANDIDATE-README.md` and http://127.0.0.1:13004/local-candidate-samples/issues.

## Verified boundary

- Both the inherited checkout and accepted A: source have HEAD
  `60fea34b9770bd8efbf6f6f0854d4b3ec26b3e00`.
- Inventory covers Git-tracked and nonignored untracked files: 6,852 inherited
  and 6,900 accepted. The material gap is 38 changed plus 48 source-only paths;
  151 additional paths differ only in line endings. There are no target-only
  paths. Counts are files, with untracked directories expanded (`-uall`).
- All inherited dirty file contents are backed up in the task evidence folder.
  No original checkout or accepted A: source has been edited.
- An isolated comparison clone fetched upstream `main` at
  `a843b44a48b4cb3d94c08e0c56ddef1fc1105257`. The merge base is
  `9fab6da917953c6f1a967a17beef045aff3d0ce5`: upstream has 103 exclusive commits
  and the pilot has 14. The independent audit is complete: preserve the pilot
  base, adopt the exact accepted delta, and defer the blanket merge (21 textual
  conflicts; 34 numeric migration-prefix collisions, not full ledger keys).
- All 86 material paths are adopted. Independent review verified all paths,
  1,380 inherited backups, 60 archived stewardship sources and three manifests.
  The consolidated correction fixes the archive check path and two stale test
  labels. Product wording and frozen manifests remain accepted bytes.
- Focused checks pass: 204 Core schema tests, 23 Views tests, Go guard/projection/
  state packages, bounded daemon tests, three controller adapter tests and the
  real 60-source archived stewardship check. Windows has no `make`; its exact
  Go command was executed directly.
- The activity HTTP 500 and malformed-cursor panic were reproduced, repaired,
  and verified by six focused tests on fresh PostgreSQL 17 through migration
  574. The sole regenerated semantic SQL output is workflow_v2; 57 unrelated
  generated newline changes were restored to exact preimages.
- Isolated Next.js and web OpenSSL upstream patches are applied and read back
  as 16.3.4 and 3.5.8-r0. Candidate builds pass, including web TypeScript. Another
  26 Inbox/Chat/navigation tests and focused handler/projection vet pass.
- The second slice's one independent review and consolidated correction are
  complete: `agent_runtime` occupancy preflight, transactional sample issue
  counter, and graceful gateway stream completion. A fixed-target gateway
  resolves Docker's internal-network port-publishing limitation while app,
  backend and database remain exclusively internal.
- Candidate `60fea34b9770-local-6f0af10d34dd` binds 5,865 build inputs. The three
  built image IDs match running containers and health reports that exact source.
  Four services are healthy; fresh PostgreSQL 17.11 applied all 603 embedded
  migrations. Empty inherited environment loading and loopback ports are verified.
- Built-in Browser passed sign-in, truthful Board/Now, refresh, incident Inbox,
  Chat's agent requirement and manual SAMPLE-4 creation. Runtime activity pages
  all 4/4 sample records exactly once and rejects malformed UUID cursors with
  HTTP 400. Agents, runtimes, tasks, chats and owner notifications remain zero.
- The separate test database on 15436 is stopped after exact identity checking.
  The preview remains available with retained sample volumes and stop instructions.

## Next

Safe work in this phase is complete. The owner must authorize the production
destination/scope, access policy and applicable credential/runtime authority
before release work. Production authentication, TLS/origins and recovery custody
must then be configured and verified. Item 13's dated off-machine readback and
separate key-custody gates remain unverified; no provider refresh is implied.
Completed qualification/provider experiments stay sealed. The full upstream
merge remains separate. Use `./scripts/local-candidate.ps1 Stop` when the local
review is finished; its volumes and evidence are retained.

## Evidence

- `.qualification-evidence/local-integration-20260915/boundary-inventory.json`
- `.qualification-evidence/local-integration-20260915/boundary-summary.json`
- `.qualification-evidence/local-integration-20260915/inherited-dirty-files/`
- Comparison clone:
  `A:\Silver-Mediums-Workspace\audits\multica\upstream-integration-20260915T1450Z`
- Accepted checkpoint (read-only):
  `A:\Silver-Mediums-Workspace\worktrees\multica\integration-preservation-pilot-20260914T232814Z\docs\silver-mediums\CURRENT-CONTINUATION.md`

## Authority

Local integration, focused regression tests, independent review and browser
verification are authorized. Completed provider trials and fake workflow
replays remain closed. The exact provisional R01-like configuration expires
2026-09-22 13:15Z and provides preparation authority only. No production push,
deployment, outbound action, existing credential access or change, permanent
role selection, or Phone Farm Windows work is authorized.
