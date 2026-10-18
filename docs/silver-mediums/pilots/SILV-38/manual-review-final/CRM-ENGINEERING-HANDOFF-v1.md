# CRM engineering handoff

**Issue:** SILV-38 — Launch replacement  
**Snapshot:** 2026-09-15T18:44:12Z (UTC) · host `a9drew` · platform `windows/amd64`  
**Boundary:** Local Git metadata and worktree path existence only. No remote contact. No CRM application access.

---

## Owner branch and HEAD

| Field | Value |
|---|---|
| Branch | `main` |
| HEAD SHA | `ba1447ac85cbce4c1b04273c14f3d2c4a524582e` |
| Remote origin | `https://github.com/A9shots0/SMCrm.git` |
| Repo root | `C:\Dev\CRM` |

---

## Dirty state (definitions apply)

- **Tracked changes:** 6 porcelain records (staged or unstaged modifications to tracked files)
- **Untracked files:** 110 files enumerated via `--untracked-files=all`

Per `AGENTS.md`: all existing modifications and untracked files are owner work. Do not revert, delete, or overwrite any of them. Keep changes scoped; no large rewrites in a dirty mixed worktree.

---

## Cached upstream divergence

| Ref | SHA |
|---|---|
| `origin/main` (cached) | `b58807067d7f667b486ad289f36671204c07212f` |
| Local `main` | `ba1447ac85cbce4c1b04273c14f3d2c4a524582e` |

**Local is 4 ahead, 674 behind cached `origin/main`.** These values come from locally cached refs — no fetch was performed. Actual remote divergence is unknown. Do not branch, integrate, or clean up based on these counts without fetching first.

The `CONTEXT.md` note (2026-08-28) records an earlier state of 4 ahead / 350 behind and identifies `backup/crm-dirty-main-pre-recovery-20260828` as the preservation pin. The 674-behind count now reflects additional upstream commits that have not been fetched or reviewed locally.

---

## Worktree inventory

| Total records | Path-checked | Present | Absent | Locked | Prunable | Detached |
|---|---|---|---|---|---|---|
| 285 | 285 | 285 | 0 | 0 | 0 | 6 |

Eight representative worktree records are included in this snapshot's input; the full inventory of all 285 records with path-presence results is in the complete evidence file:

`C:\Users\a9sho\.codex\worktrees\3228\SMK-multica-selfhost-pilot\docs\silver-mediums\pilots\SILV-38\launch-replacement\CRM-HANDOFF-EVIDENCE.json`

Path presence proves only that the directory exists. It does not prove the worktree is clean, idle, safe to edit, or safe to delete. All 6 detached worktrees are preserved as-is; the 0 locked and 0 prunable counts are local Git metadata only.

Notable occupied worktrees include: analysis and audit branches under `A:/Silver-Mediums-Workspace/worktrees/crm/CRM/` (permission-model evidence, privilege-escalation blast radius, access-screen UX, permission-system wiring) and two paseo worktrees (`famous-buffalo`, `joyful-cougar`) on drive `A:`. Do not reassign or reuse any of these without direct owner authorization.

---

## Key guidance pointers

- **`C:\Dev\CRM\AGENTS.md`** (SHA `3dc84e5c...`) — canonical agent rules; wins over all other docs. Verified 2026-07-26 against code. Review before any write.
- **`C:\Dev\Silver-Mediums-Knowledge\GOVERNANCE.md`** — authority ladder and claim-status definitions.
- **`C:\Dev\Silver-Mediums-Knowledge\projects\crm\CONTEXT.md`** — CRM role, local paths, and deployment history including the 2026-09-14 social-data-ingestion candidate (uncommitted, not approved) and the 2026-09-10 inbound-boundary update.

Phone Farm is Mac Studio-only. Never inspect or operate it from Windows.

---

## Prioritized next actions

1. **Fetch before any Git operation.** The 674-behind count is stale cached data. Run `git fetch origin` from a clean context before branching, rebasing, or assigning any worktree write.
2. **Confirm worktree occupancy before assigning writes.** All 285 paths exist; check active-agent occupancy in real time before writing to any worktree.
3. **Review the 6 tracked modifications** before any commit or push. These are owner work; understand their intent before staging anything on top.
4. **Do not run `python main.py` or any script under `scripts/`** without explicit owner instruction. Boot-time DB writes, DDL, and destructive scripts activate on import.
5. **Scope any change to a clean isolated worktree** based on a current `origin/main` tip, not the local `main` head, to avoid replaying the 4 local-only commits blindly.

---

## Verification still required (not covered by this snapshot)

| Area | What is unknown |
|---|---|
| Remote Git (`github.com/A9shots0/SMCrm`) | Current default branch tip, open PRs, CI status, and whether local commits are already upstream-equivalent |
| SMCrm-Jobs deployment | Deployed revision, health, scheduler state, and whether paired jobs are current with SMCrm |
| SMCrm deployed revision | Railway deployment SHA vs. local HEAD; `scripts/railway_predeploy.py` side-effects on last deploy |
| Runtime health | Live application reachability, authenticated endpoint responses, and DB migration state |
| Business acceptance | The 2026-09-14 social-data-ingestion candidate remains uncommitted and explicitly owner-prohibited from live CRM until separate approval |
| Inbound boundary | 2026-09-10 delivery state (`Released, checking`) — independent live verification still open |

---

## Preservation and recovery boundaries

- `backup/crm-dirty-main-pre-recovery-20260828` pins the pre-recovery state of local `main`.
- The checkout was not merged, stashed, switched, reset, or cleaned during the 2026-08-28 audit.
- Expired evidence is not false — it requires revalidation before operational reliance.
- All dirty, detached, and occupied worktree records are evidence; none carry cleanup authority from this snapshot.

---

**Reviewer note:** Factual claims in this document are bounded to local Git metadata and filesystem path existence as of 2026-09-15T18:44:12Z. No remote, deployed, runtime, or business state is asserted. Any claim about deployed revision, application health, or business acceptance requires a separate, dated, scoped observation in the appropriate evidence lane per `GOVERNANCE.md`.
