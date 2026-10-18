# Independent review: accepted Item 14 adoption

## Verdict

**Changes requested: two bounded P2 verification corrections.** The adoption
itself is complete against the recorded 86-path boundary. No P0/P1 defect,
missing material adoption path, changed accepted authority contract, or lost
archived/preimage evidence was found in this review.

Apply the two corrections together, run their focused checks, and continue the
local candidate work. This review does not request a new review round, changes
to accepted product copy, historical manifest rebinding, model experiments, or
blanket historical test reruns.

## Scope and snapshots

- Reviewed source worktree:
  `A:\Silver-Mediums-Workspace\audits\multica\accepted-integration-review-20260915`.
- Immutable accepted source snapshot:
  `04b762a0eb51edaea7f506aee9fb890bf89a3e4e`; 6,900 files. It is the accepted
  source captured in the upstream audit, not a new upstream integration.
- Inherited and accepted committed HEAD:
  `60fea34b9770bd8efbf6f6f0854d4b3ec26b3e00`.
- Integrator inventory snapshot: `2026-09-15T14:54:12.414377+00:00`.
- Adoption receipt: `2026-09-15T15:03:12.310957+00:00`.
- Independent preservation measurement completed:
  `2026-09-15T15:10:45.510063+00:00`.
- Root evidence directory:
  `C:\Users\a9sho\.codex\worktrees\3228\SMK-multica-selfhost-pilot\.qualification-evidence\local-integration-20260915`.
- Frozen archive:
  `C:\Users\a9sho\.codex\worktrees\3228\SMK-multica-selfhost-pilot\.qualification-evidence\item14-accepted-checkpoint-20260915`.

This is the single independent review of the adoption slice. It checks transfer
completeness, inherited-work preservation, accepted evidence preservation,
integration coherence, and unintended authority expansion. It does not
re-review frozen model/provider experiments, Items 1-13, the complete repository,
or the separate meaningful-activity SQL correction. No provider/fake workflow
trial, runtime, browser, database mutation, credential action, or outbound action
was performed by this reviewer. Only review artifacts were written here.

## Findings

### [P2] Route frozen stewardship verification to its preserved source tree

**Location:** `Makefile:243` in the reviewed snapshot.

The adopted `silver-mediums-item14-local-stewardship-check` target selects the
v3 manifest but still passes `-source-root ..`. Four v3-bound evidence files are
intentionally stored only under the new accepted-checkpoint archive, so the
documented target reads a source tree that cannot satisfy its manifest:

- `.qualification-evidence/item14-integration-replay-20260915-v3/EXACT-LAUNCH-CONTRACT.json`
- `.qualification-evidence/item14-integration-replay-20260915-v3/real-claude-replay-report/EVIDENCE.md`
- `.qualification-evidence/item14-integration-replay-20260915-v3/real-claude-replay-report/inventory.json`
- `.qualification-evidence/item14-integration-replay-20260915-v3/real-claude-replay-report/WINDOWS-REPOSITORY-WORKTREE-INVENTORY.md`

The current continuation file was also being updated during this review,
creating the expected live-versus-frozen hash difference. The archive contains
all 60 exact source bytes and all three exact manifests; the defect is the
verification entry point, not missing historical evidence.

**Correction:** route the frozen-checkpoint verification target to the immutable
archive and matching manifest, optionally with an explicit source-root override.
Point current continuation/progress instructions to separate current candidate
checks. Preserve the manifests and archived originals. The existing command
already supports `-source-root`; no new framework is necessary.

**Evidence:** `adoption-verification.json` lists four absent live-root paths and
the changed live continuation; it reports zero archive mismatches. The
integrator's `accepted-stewardship.log` reports `valid:true`, 60 sources, 44
immutable predecessors, and 16 extension sources for the explicit archived
check. `server/cmd/item14-stewardship-check/main.go:15-16` exposes the manifest
and source-root arguments.

### [P2] Align the technical-stop assertion with accepted product copy

**Location:** `packages/views/issues/components/operational-workflow.test.tsx:180`.

The test expects two occurrences of `Technical stop`, while the accepted English
stage copy is `Stopped by a technical problem`
(`packages/views/locales/en/issues.json:748`). The captured DOM renders that
accepted copy in both surfaces. The stale assertion causes the focused suite to
fail at 22/23 tests even though the next step and owner-action text render.

**Correction:** update the assertion to the existing accepted stage text. Keep
the product copy and its intended behavior. Rerun this focused Views suite.

**Evidence:** the assertion and locale mismatch are present in the immutable
review snapshot; the adoption receipt and independent hash check show the
candidate received those exact accepted files. The integrator's
`adoption-views.log` contains the failing line, both rendered surfaces, and the
22-pass/1-fail result. This is accepted assertion drift, not a failed file copy.

## Preservation and completeness results

| Check | Result |
| --- | --- |
| Material adoption set equals inventory and receipt | 86 / 86 paths; 38 existing-file changes + 48 additions |
| Adopted candidate and accepted-source bytes match recorded accepted hashes | 86 / 86; zero mismatches |
| Independent review snapshot matches accepted inventory | 6,900 / 6,900 after the inventory's text newline normalization |
| Preserved inherited dirty/untracked preimage files | 1,380 / 1,380 exact hashes |
| Overwritten existing-file preimages available | 38 / 38 |
| Archived accepted stewardship sources | 60 / 60 exact recorded hashes |
| Archived predecessor/current manifests | 3 / 3 exact accepted-source hashes |
| Accepted source chain still matches recorded hashes | 60 / 60 |
| Target-only paths requiring deletion or reconciliation | 0 |

`adopt.py` checks both source and destination preimages before copying, requires
the 86-path set, verifies every copied hash, preserves the accepted chain before
mutating active files, and refuses to overwrite an existing checkpoint archive.
The review independently checked the receipt against the full boundary
inventory rather than trusting its counts alone.

The existing accepted role/lifecycle contracts retain the same scoped,
temporary, reversible preparation default, exclusive expiry, separate per-run
authority requirement, no automatic dispatch, and no provider/credential,
production, outbound, or permanent-seat grant. The relevant reviewed validators
remain in `server/internal/agentqualification/provisional_role_configuration_v2.go:94`
and its strict authority checks at `:130-134`. Adoption adds no different
configuration bytes or startup activation.

### Concurrent SQL-slice changes were kept outside this verdict

The integrator continued the explicitly separate meaningful-activity correction
while the full live-tree preservation scan ran. Consequently, the final live
scan is not an atomic adoption snapshot. It observed 61 later differences from
the expected post-adoption bytes:

- Four content changes: `workflow_meaningful_activity.go`, its test,
  `server/pkg/db/queries/workflow_v2.sql`, and its generated Go output.
- Fifty-seven generated SQL access files with only CRLF/LF differences. Their
  normalized content still matches the recorded inherited/accepted preimages.

The exact paths and classification are recorded in
`concurrent-slice-classification.json`. These are not attributed to adoption and
are not reviewed here. The adoption receipt's 151 untouched newline-only paths
is a point-in-time adoption claim; later sqlc regeneration changes that live
byte comparison. Keep any unrelated generated newline churn out of the eventual
SQL slice when practical, using the preserved preimages.

## Verification evidence and limits

Reviewer-run read-only checks are reproducible with `python review_adoption.py`.
Results are retained in `adoption-verification.json`; concurrent drift was
classified separately. The reviewer also inspected the adoption script,
manifest/checker wiring, accepted authority validators, relevant UI assertion,
locale, and the integrator's focused output logs.

Observed integrator evidence:

- Core schema suite: 204/204 pass (`adoption-core.log`).
- Go `workflowguard`, `workflowprojection`, and `workflowstate/v2`: pass
  (`adoption-go-contracts.log`).
- Archived stewardship: valid, 60/60 sources (`accepted-stewardship.log`).
- Views: 22/23 pass, with the single P2 assertion above (`adoption-views.log`).

These logs are observed integrator runs, not tests independently rerun by the
reviewer. Pending controller/daemon/build/Browser checks are not represented as
passed. No deployment, remote Git change, runtime qualification, or business
acceptance follows from this review.

Historical evidence remains at the accepted A: source and in its new exact
archive. Compacting active continuation/progress records is appropriate after
preservation; do not rebind frozen manifests to new live prose merely to make a
historical check pass.

The upstream audit timestamp was corrected before its first handoff to the
exact measurement `2026-09-15T14:59:39.519360+00:00`; no future approximate
completion time remains in that audit.
