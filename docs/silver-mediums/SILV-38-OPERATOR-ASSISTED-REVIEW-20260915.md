# SILV-38 operator-assisted closure authority

The sixth native task produced a real Claude handoff but failed its first
submission at 2026-09-15 18:46:17Z. Backend HTTP evidence records POST comments
409. The matching handler (`server/internal/handler/comment.go:1769`) requires a
comment-triggered task to use its exact trigger or coalesced comment as parent.
The operational broker omitted `parent_id`. No implementation_ready event ran.
The application correctly retained a technical stop.

Source coordination task `01a06e85-9068-74f1-9c2d-64fc61dd7bd8` explicitly approved
retaining the original artifact/receipt, provider-free factual corrections with
operator provenance, and the single remaining independent reviewer call. After a
pass the operator may publish and complete through supported application controls.
No additional writer, replayed provider result, direct workflow-state mutation or
broader authority is allowed. The maximum remains seven native attempts and six
actual provider calls. Automatic native handoff is explicitly not accepted.

## Required correction before a future automatic pilot

The future broker must derive the current trigger/coalesced reply parent from
current task metadata and include `parent_id` on comment submissions. It must
retain bounded safe HTTP stage/status diagnostics. Add provider-free coverage for
assignment-triggered top-level comments, comment-triggered replies, stale/wrong
parents and resumed/coalesced comments. Verify artifact publication and the full
implementation/review transition against the actual application contract before
any separately authorized automatic provider replay. Do not bypass the guard or
reuse the failed task's identity. This correction is documented, not deployed as
an automatic workflow fix by this closure.

The final review harness only produces hash-bound independent review evidence and
returns the real provider response. It leaves publication and status completion
to the explicitly authorized operator. Its role guard rejects every non-review
call. The original model draft and captured evidence remain preserved alongside
v2 and operator-correction.json.
