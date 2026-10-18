# Chat diagram annotations - local verification 2026-09-25

## Latest: owner rejected annotation UX; simplified local revision

Follow-up: owner also rejected the first revision's narrow editor and clipped
footer. Latest layout gives the editor its full container width, with Comment N
and Done above it. Other footer actions are hidden until Done. The collapsed
footer has two constrained columns; questions are available in the comments
menu. Below 340px container width, tool selection becomes a labeled menu and
Fit is available in options. This is container-based, not a device-name heuristic.
Screenshot: `ui-reference/owner-rejects-narrow-comment-editor.png`.

Final narrow-state verification: use the Browser's `viewport.set/reset` API,
not ad hoc CDP device-metric overrides (the latter produced a scaled/clipped
capture on this host). At measured 280x680, dialog width248 and textarea
width224; Done and all visible toolbar controls were within the viewport.
After Done, Add to chat occupied x163..256 within the 280px viewport. At
393x852, the full-width editor was visually inspected with the saved question.
No draft text or marks were changed; temporary overrides were reset. The final
29-test rerun, focused lint and Views typecheck passed. Owner approval pending.

This supersedes the earlier positive annotation feedback. Owner called the
entire viewer bad UX and ugly on mobile; screenshot preserved as
`ui-reference/owner-rejects-annotation-ux.png`. Earlier geometry checks proved
fit, not usability or visual approval.

Chat now opens directly into a Move-first review with one icon toolbar. The
technical source/export/zoom controls are in Diagram options. Fit and Close
stay visible. The footer exposes saved comments and Add to chat; only the
selected pin's comment or optional general question opens for editing. Pin
selection also supports keyboard Enter/Space. Blank pins open for editing on
Add to chat instead of silently blocking a disabled button. Draw/Erase/Undo
and the PNG/comment upload path remain. Stored owner drafts were preserved.

The previous hook zoomed on every ordinary wheel event, explaining the owner's
283% screenshot. Chat review opts into wheel panning; deliberate Ctrl/pinch and
menu/keyboard zoom remain. Other image/non-Chat viewers retain their behavior.

Verification: 29 diagram/viewer tests pass, including ordinary-wheel pan without
scale change and explicit Ctrl-wheel zoom, default collapsed fields, saved-note
reopening, erasing/restoring marks and comment export. Views typecheck and
focused lint pass. Built-in Browser at actual 393x852 CSS pixels showed one-row
toolbar, zero default textareas, no horizontal document overflow and ~660px of
canvas. Screenshot inspected; opening pin 1 showed only its saved question.
No send or model run performed; transport is unchanged from the earlier proof.
Physical phone touch/keyboard and owner visual approval remain unverified.
Root handoff now leads with rejection rather than suggesting copying this UI
as an accepted pattern. No push, deployment or backend restart.

## Owner follow-up and eraser verification - 2026-09-25 04:36 UTC

Owner likes the feature and requested an eraser before wrapping up. Erase now
removes one selected whole stroke or comment pin (including its comment). Undo
restores removed marks and their comments during the current viewer session.
Widened invisible stroke hit targets make selection easier when zoomed out.
No partial-stroke erasing or redo is claimed. Existing marks/notes still persist;
the Undo history itself is transient.

Three interaction tests pass, including deleting a stroke/pin, preserving the
overall question and restoring the last removed mark. Focused lint and Views
typecheck pass. Built-in Browser verified 4 -> 3 -> 4 pins and 4 -> 3 -> 4
strokes using Erase then Undo; the owner's marks were restored. At measured
392x852 CSS pixels, controls wrap inside the dialog with Add to chat visible
and no document overflow (width 392). Normal viewport restored. No new model
test was necessary: the attachment/send path is unchanged from the proof below.

Root `MULTICA-UI-HANDOFF.md` now records owner acceptance, UX standards,
rejected patterns, source/runtime recovery pointers, the evidence index,
remaining limits and proposed next-page order. Root AGENTS.md points to it.
Local-only; no agent spawned, new page started, push or deployment.

Owner requirement: draw on the diagram and annotate it to ask questions and
leave comments. Preserve the accepted Chat design and compact inline preview.

## Implemented locally

Expand a Chat Mermaid diagram and choose the pencil. Move, Draw, Comment and
Undo operate inside the existing viewer. Numbered pins have editable comments;
an optional overall question accompanies them. Marks use natural diagram
coordinates so zoom/pan does not move them relative to diagram content.

Add to chat rasterizes the marked diagram as a PNG and stages it with the
numbered comments in the existing composer. Existing draft text is preserved;
the action does not send. The existing upload/send path supplies the image to
Mika. Workspace/session gates reject a stale viewer targeting another draft.
Annotation drafts are stored through Core's workspace-scoped storage adapter,
keyed by conversation and diagram source. Reopening restores marks and notes.
This applies to Chat diagrams, not arbitrary photos or PDFs.

## Evidence

- Isolated worktree: `C:/Dev/_work/multica-brain-owner-review-20260924`.
- Local preview: `http://127.0.0.1:3221/mika-local-review/chat?session=01a0d661-e5c8-74a8-809c-b461300d57f3`.
- Built-in Browser drew an orange circle around **Reviewers give specific
  feedback** and placed pin 1 on **Summarize feedback and decide what to change**.
  The question asked Mika to identify those marks without naming either step.
- Add to chat staged `diagram-review.png` (654 x 1882 pixels), comments and
  question. After sending the bounded local test, Mika correctly identified
  both steps and explained that the loop sorts requested changes before revision.
  The image, question and answer remain in the rehearsal conversation.
- Closing/reopening the viewer retained drawing, pin, comment and question.
- Phone geometry measured 392 x 852 CSS pixels; document width 392, dialog
  width 360, all toolbar controls and Add to chat inside the viewport. Drawing
  and pin remained aligned. Normal viewport was restored after verification.
- Core focused tests: 27 passed. Views initial focused run: 107 passed;
  annotation interaction tests: 2 passed. Final annotation/viewer run: 27 passed.
  Views typecheck passed. Focused lint and whitespace checks passed.
- Tests cover persisted draft validation/isolation, zoom-coordinate mapping,
  pin numbering, SVG export with nonzero origin, unavailable-composer recovery,
  preserving existing text, rejecting another draft and staging without sending.

## Boundaries

Local code and local runtime evidence only. No push, deployment, production
data change or backend restart. Owner web configuration changes remain excluded.
Phone layout was verified; physical finger/stylus input was not (the Browser
backend does not expose touch dispatch). Drawing was exercised using pointer
input. Full Core typecheck still has the previously recorded unrelated Inbox
fixture mismatch; this change does not claim a clean full-repository gate.
Brain source refresh remains pending; current source and local Browser evidence
are the basis for this handoff. Owner subsequently liked the annotation feature
and requested the eraser documented above; the eraser itself awaits owner review.
