# Chat legacy projection admission

2026-09-24. Continues checkpoint e8deb760f. Local-only work; no release authorized.

- [x] Trace three direct projection writes and their transaction owners.
- [x] Add scoped admission inside the existing transaction, with nonblocking owner locks to avoid legacy/archive lock inversion.
- [x] Pass real transaction handles through route, event, task-start and terminal-stop callers; reject untransactional projection writes.
- [x] Verify scope, rollback, lock contention and caller-owned commit behavior.
- [x] Run unchanged architecture guard and broader workflow regressions.
- [x] Fix concurrent operator-completion replays revealed by the wider run; preserve nonblocking legacy contention.
- [x] Pin embedded skill bundle line endings after full service checks exposed Windows CRLF checkout failures; rerun unchanged tests.
- [x] Split the over-budget issue reference into an explicitly routed Chat/workflow reference, preserving its instructions and enforcing discovery in the skill tests.
- [x] Review changes, rebuild isolated preview, verify browser.
- [x] Commit checkpoint and record handoff.

Files: workflowstore/writer.go and operator_completion.go; workflow_v2.sql, issue.sql and generated queries; service owner_runtime_projection.go, workflow_legacy_guard.go, task.go; handler workflow_control.go, owner_runtime_route.go; admission regression tests; .gitattributes; this record.

Initial shared-brain lookups returned HTTP 404 because the locator format was wrong, not because the service was unavailable. Resolution with host `windows` and the enrolled worktree path succeeded. The full immutable Multica context revision was read. Source-refresh generation 17 still has 11 pending sources and no submitted revision; it is not runtime authority. The final handoff corrects the earlier progress observation's unavailable-service diagnosis.

## Source verification, 2026-09-24 21:49 UTC

The three direct projection writes are removed. Their existing transactions now
pass the actual pgx transaction into a narrow workflowstore admission operation.
It validates workspace/issue ownership and retains owner key-share locks until
the outer transaction ends. Nonblocking acquisition prevents legacy plan/task
locks from waiting behind archival in reverse order. No architecture-guard
exemption was added. Missing transaction support fails before projection or
terminal-stop mutation.

The wider run exposed a separate regression from the earlier NOWAIT completion
change: concurrent exact operator requests returned 409. A transaction-scoped,
issue-specific completion key now serializes those requests before owner locks.
Legacy issue/plan contention still returns promptly. Four concurrent identical
requests return the one receipt with one event, one completion notification and
one picture revision. Existing assertions were retained.

The full service suite exposed Windows CRLF frontmatter failures, then an
over-budget 530-line issue guide. Embedded bundle paths are now LF-pinned.
The unchanged owner/Chat instructions moved into an explicitly advertised
workflows.md reference; discovery and critical Chat contracts are checked.
Byte comparison confirmed the moved instructions were preserved.

Database-backed tests used only isolated PostgreSQL port 55440:
- Handler: 132 passing test/subtest events, zero failures/skips. Includes new
  caller commit/rollback, invalid tenant, owner lock lifetime and archival
  contention cases, plus owner route stages, task start/stop, question resume,
  operator completion/replay, Chat work and workflow control regressions.
- Workflow store, command and projection: 253 passing test/subtest events.
- Full service: 586 passing test/subtest events; 18 Redis-specific tests skipped
  because REDIS_TEST_URL is not configured. No service test failures remain.
- All backend packages build with -buildvcs=false -p 1.
- Go vet passes for workflowstore, handler, service, workflowcommand and
  workflowprojection. gofmt, regenerated sqlc and git diff --check pass.

Review covered scope ownership, transaction atomicity, lock ordering, replay,
caller error propagation, nil transaction paths and skill discovery. No further
blocking finding was identified in this change. Specialized review subskills
were not installed; those checks were performed directly. No UI code changed.
The existing owner edits in apps/web were excluded.

Logs are in C:/Dev/_work/mika-chat-prototype-20260924:
chat-admission-handler-final.jsonl, chat-admission-backend.jsonl,
chat-admission-service-final.jsonl, chat-admission-skills.log,
chat-admission-build.log and chat-admission-vet.log. The first backend log
retains the earlier skill failures; service-final records the corrected run.

## Local runtime verification, 2026-09-24 21:56 UTC

Code checkpoint: `9f47cb811ef336d362a754711763abf513318891`.
The isolated API was rebuilt from that commit and started hidden at
2026-09-24T21:51:33Z. PID 34476 owns port 3220. `/health` reports that exact
commit; `/readyz` reports database and migrations OK. The executable is
`C:/Dev/_work/mika-chat-prototype-20260924/mika-chat-admission-server.exe`,
SHA256 `F983766D4781A742A9BCC9006B32FDC8FCFFC05E9CE7685954FFD0D4184AD49C`.
Only the verified previous review API process was replaced. Web PID 50948,
review worker PID 45472, other daemons and port 3000 were preserved. The review
worker heartbeated after the API restart. Hidden launch is best-effort survival,
not a supervised startup service or proof the Windows shell defect is fixed.

Built-in Browser reloaded the working local Chat tab and showed the existing
accepted issue "Improve the welcome message" as Done, including its recorded
issue reply context and acceptance history. No fresh worker result was generated
in this pass. URL:
http://127.0.0.1:3221/mika-local-review/chat?session=01a0d454-5dfb-743b-918d-055136d981a8

Phone DOM geometry was checked at 393x852 with document width 393 and no
horizontal overflow. Browser capture scaling was inconsistent, so these new
captures do not establish phone visual acceptance. No UI source changed this
pass. All emulation overrides were reset. Existing stale error tabs were left
alone after Browser policy blocked navigating their error documents; the working
tab was marked as the deliverable.

All known blockers from the preceding admission checkpoint are closed locally.
The optional Redis lane remains untested. No production release, Windows crash
root-cause fix, new end-to-end worker acceptance or production-scale qualification
is claimed. Evidence and the local-only boundary are recorded in the Multica hub
changelog and shared-brain task `mika-chat-continuation-20260924`.
