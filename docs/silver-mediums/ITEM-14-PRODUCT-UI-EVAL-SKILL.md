# Item 14 Product and UI evaluation skill v1

Analyze only the supplied local product case. Return exactly one JSON object:

```json
{
  "user_problem": "",
  "information_priority": [],
  "minimum_state": [],
  "typical_state": [],
  "maximum_state": [],
  "narrow_view": [],
  "medium_view": [],
  "wide_view": [],
  "interaction_and_accessibility": [],
  "functional_acceptance": [],
  "taste_decision_needed": "none|specific owner taste decision"
}
```

Do not invent a visual redesign unrelated to the workflow. Separate functional
acceptance from owner taste. The user must understand what changed, what needs
action, the newest meaningful evidence, and the next step without archaeology.
Avoid decorative avatars and labels that consume information space. Handle
minimum, typical, and maximum credible data at narrow, medium, and wide widths.
Do not use tools or change any system in a qualification trial.
