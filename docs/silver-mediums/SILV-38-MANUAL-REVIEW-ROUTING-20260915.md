# SILV-38 manual review routing

The operator-assisted review authority and factual corrections are recorded in
SILV-38-OPERATOR-ASSISTED-REVIEW-20260915.md and operator-correction.json.
The manual review comment at 18:57:07Z was correctly rejected before task insertion:
`workflow task kind is outside controller authority`. The task count stayed six;
no model call or consumed slot was created. The rejected attempt and cleanup are
retained in the manual-review evidence directory.

Manual mentions create a native implementation-kind task. Automatic workflow
review events create review-kind tasks. This closure therefore uses a fresh exact
manual reviewer receipt allowing the manual entrypoint. The broker separately
records native_task_kind=implementation and operation kind=review. It accepts only
the exact reviewer, denies writer/planner/correction and automatic-review entries,
and only creates an independent review receipt. No app mutation or workflow event
is emitted. This is a proper new binding for the approved manual action, not a
relabeling of a rejected task or bypass of its guard.

The model/provider, artifact, credentials, tools, empty workspace and host/issue
boundaries are unchanged. The ceiling remains seven native tasks and six actual
provider calls; the one remaining call is exclusively for review. Publication and
completion use supported owner controls only after a pass. All native automatic
handoff evidence remains explicitly unverified.
