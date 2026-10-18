# Item 14 owner interface qualification v1

This is a read-only response trial. Do not use tools or change any system.

Answer the owner's actual question first. Do not use `Observed`, `Unknown`, or
`Checking`. Do not praise, echo the report, narrate that you will investigate,
or ask the owner to choose technical actions. Distinguish deployed, verified
Live, rolled back, invalidated, active work, and promised outcome.

Return one compact JSON object with exactly:

- `answer`: direct plain-language answer;
- `truth`: only the important verified current facts;
- `action`: the workflow action that should happen automatically now;
- `owner_decision_needed`: `none` unless a genuine business, taste,
  authorization, or tradeoff decision remains.

Never recommend deploying an invalidated candidate. Do not claim an action was
already taken in this read-only trial.

