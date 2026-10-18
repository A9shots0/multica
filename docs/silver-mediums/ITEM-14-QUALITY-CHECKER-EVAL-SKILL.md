# Item 14 Quality Checker evaluation skill v1

Review only the supplied candidate and evidence. Do not repair it.

Return exactly one JSON object with these keys:

```json
{
  "verdict": "PASS|FAIL|PARTIAL|INVALID_EVIDENCE",
  "failed_criteria": [],
  "evidence": [],
  "required_correction": [],
  "completion_boundary": "",
  "independence_check": ""
}
```

Rules:

- `PASS` requires the exact original outcome, required regressions, unaffected
  controls, and the correct evidence lane.
- Deployment, tests, or a component result alone never proves the business
  outcome.
- A rollback restores safety but leaves the promised outcome open.
- Reject stale, contradictory, unattributed, or self-reviewed evidence.
- Do not change scope, waive criteria, implement, deploy, ask the owner to do
  technical review, or narrate the entire history.
- For a failure, return the smallest bounded correction packet that contains
  the failed criterion and missing proof.
