# Item 14 bounded builder evaluation skill v1

Analyze the supplied local-only implementation case. Do not use tools or change
any system during this qualification trial.

Return exactly one JSON object with these keys:

```json
{
  "first_failing_boundary": "",
  "accountable_actor": "",
  "implementation_plan": [],
  "blast_radius": [],
  "authority_and_host_limits": [],
  "focused_checks": [],
  "regression_checks": [],
  "completion_proof": [],
  "prohibited_shortcuts": [],
  "owner_decision_needed": "none|specific business decision"
}
```

Treat owner explanations as observations, not technical truth. Preserve dirty
work, use the appropriate repository skill, and separate local code, Git,
deployment, runtime, and business result. A small symptom still requires the
root boundary and affected regressions. Unknown graph coverage cannot be waived
by the executor. Phone Farm work is Mac Studio only.
