# Old checkout comparison — 2026-09-16

Read-only comparison of `C:/Dev/SMK-multica-selfhost-pilot` against the accepted
branch. Nothing in that checkout was moved, deleted or overwritten. Git ran with
`GIT_OPTIONAL_LOCKS=0`, so there was no index refresh, and files were hashed with
`git hash-object` without `-w`, so no objects were written.

## Scope and definitions

- **Snapshot:** 2026-09-16T20:10Z.
- **Old checkout:** branch `codex/selfhost-pilot-20260909` at `60fea34b9`.
- **Compared against:** accepted commit `80a375542` on
  `codex/workflow-completion-20260915`, the branch the private remote and
  `C:/Dev/multica` now carry.
- **Population:** every modified or untracked file `git status
  --untracked-files=all` reports: **1,749 files**. The earlier "125 modified /
  1,255 untracked" counted untracked directories as one entry each; expanded,
  the checkout holds 494 modified and 1,255 untracked files.
- **Identical:** the file's content after the checkout's own line-ending
  filters has the same blob id as that path in the accepted commit.

## Result

| Classification | Files | Share |
|---|---:|---:|
| Identical to the accepted branch | 1,696 | 97.0% |
| Differs from it | 53 | 3.0% |
| Exists only in the old checkout | 0 | 0% |

Identical files by area: `server` 1,496, `docs` 136, `packages` 42, `apps` 14,
`scripts` 5, and 3 root files.

Of the 53 that differ:

- **2** match an older version of the same file in the accepted branch's
  history. They are superseded drafts.
- **51** contain content that appears nowhere in the accepted history. They are
  the only copy of that state.

For those 51, the old copy has few lines the branch lacks, and the branch has
many the old copy lacks, which is the pattern of an earlier draft later
reworked. The largest are:

- `docs/silver-mediums/CURRENT-CONTINUATION.md`: 1,128 lines only in the old
  copy. The accepted version was rewritten and pruned.
- `server/internal/handler/daemon.go`: 243 lines only in the old copy. No
  function exists only there; the differing blocks are inline claim-handler code
  that the accepted branch reorganised.
- `server/internal/daemon/daemon.go`: 31 lines only in the old copy.

Every other file has 20 or fewer such lines, and 14 have none (a strict subset
of the accepted version).

## Preservation

- The 51 files: `A:/Multica-Recovery/old-checkout-unique-files-20260916T201457Z.tar`,
  with an embedded `MANIFEST.json` of per-file sha256. Verified by reading every
  member back. The archive's sha256 is `c7f3707f…c979bf`.
- Full per-file data: `A:/Multica-Recovery/old-checkout-comparison-20260916T201457Z.json`.

## Limits

- This is a content comparison, not a semantic review. The 51 files were not
  judged line by line, and some old-only lines may be intentional work the
  branch dropped. They are preserved, so that judgement can be made later
  without risk.
- Ignored files, such as local `.env` files, were not compared and were not
  touched.

## Recommendation

The old checkout holds no file that is not also preserved elsewhere. 1,696
files are in the accepted branch, and the 51 unique files plus the 2 superseded
drafts are in the archive and the branch history. Retiring the checkout is still
a deletion of the owner's working copy, so it is not done here.
