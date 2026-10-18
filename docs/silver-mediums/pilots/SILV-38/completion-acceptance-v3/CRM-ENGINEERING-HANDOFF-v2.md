# CRM engineering handoff

**Issue:** SILV-38 — Operator-assisted handoff
**Provenance:** Claude produced the retained v1 draft; the Codex operator corrected cached-ref wording, unsupported occupancy labels, and the historical/current evidence boundaries below. Independent review applies to this v2 file.  
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

The `CONTEXT.md` note (2026-08-28) records an earlier state of 4 ahead / 350 behind and identifies `backup/crm-dirty-main-pre-recovery-20260828` as the preservation pin. The current cached comparison is 674 behind; this snapshot does not establish when those refs were fetched, what caused the change from the historical comparison, or which commits have been reviewed.

---

## Worktree inventory

| Total records | Path-checked | Present | Absent | Locked | Prunable | Detached |
|---|---|---|---|---|---|---|
| 285 | 285 | 285 | 0 | 0 | 0 | 6 |

Eight representative worktree records are included in this snapshot's input; the full inventory of all 285 records with path-presence results is in the complete evidence file:

`C:\Users\a9sho\.codex\worktrees\3228\SMK-multica-selfhost-pilot\docs\silver-mediums\pilots\SILV-38\launch-replacement\CRM-HANDOFF-EVIDENCE.json`

Path presence proves only that the directory exists. It does not prove the worktree is clean, idle, safe to edit, or safe to delete. All 6 detached worktrees are preserved as-is; the 0 locked and 0 prunable counts are local Git metadata only.

Representative registered worktree paths include: analysis and audit branches under `A:/Silver-Mediums-Workspace/worktrees/crm/CRM/` (permission-model evidence, privilege-escalation blast radius, access-screen UX, permission-system wiring) and two paseo worktrees (`famous-buffalo`, `joyful-cougar`) on drive `A:`. These records do not establish current occupancy. Confirm active ownership before assigning writes; this snapshot grants no cleanup or reassignment authority.

---

## Key guidance pointers

- **`C:\Dev\CRM\AGENTS.md`** (captured full-file SHA is in the evidence) — project agent rules. Its July verification statement is historical; read current applicable instructions before any write.
- **`C:\Dev\Silver-Mediums-Knowledge\GOVERNANCE.md`** — authority ladder and claim-status definitions.
- **`C:\Dev\Silver-Mediums-Knowledge\projects\crm\CONTEXT.md`** — CRM role, local paths, and deployment history including the 2026-09-14 social-data-ingestion candidate (uncommitted, not approved) and the 2026-09-10 inbound-boundary update.

Phone Farm is Mac Studio-only. Never inspect or operate it from Windows.

---

## Prioritized next actions

1. **Refresh remote evidence before implementation.** The 674-behind count uses cached data of unknown freshness. Before choosing an implementation base, fetch the intended remote and inspect its current tip from a suitable working context. Read-only local inspection does not require a fetch.
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
| Business acceptance | The intended employee/client workflow has not been exercised by this snapshot. The dated CONTEXT candidate restriction is historical guidance, not fresh business acceptance |
| Inbound boundary | The 2026-09-10 CONTEXT note says `Released, checking`; this inventory did not refresh that delivery state or verify the live inbound flow |

---

## Preservation and recovery boundaries

- The 2026-08-28 CONTEXT note identifies `backup/crm-dirty-main-pre-recovery-20260828` as a preservation pin. This inventory did not verify that branch or a restorable backup.
- The historical CONTEXT note says its 2026-08-28 audit did not merge, stash, switch, reset, or clean the checkout. This current inventory also made no such changes.
- Expired evidence is not false — it requires revalidation before operational reliance.
- All dirty, detached, and registered worktree records are evidence; none carry cleanup authority from this snapshot.

---

**Reviewer note:** Factual claims in this document are bounded to local Git metadata and filesystem path existence as of 2026-09-15T18:44:12Z. No remote, deployed, runtime, or business state is asserted. Any claim about deployed revision, application health, or business acceptance requires a separate, dated, scoped observation in the appropriate evidence lane per `GOVERNANCE.md`.
