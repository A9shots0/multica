Pilot-only workflow verification. Do not access repositories, external systems,
or real Silver Mediums work.

Required workflow:

1. Keep this issue assigned to Pilot Operations for its entire lifecycle.
2. Mika moves it to Working and delegates execution inside this issue to Pilot
   Worker.
3. Pilot Worker comments exactly: `WORKER RESULT: 2 + 2 = 4`
4. Mika moves it to In Review and delegates independent review inside this
   issue to Pilot Reviewer.
5. Pilot Reviewer checks the worker's result and comments exactly:
   `REVIEW PASS: 2 + 2 = 4`
6. Mika marks the issue Done only after both exact comments exist.

Do not create another issue. Do not perform any other work. If the platform
cannot complete a step, leave the issue in the truthful current state and
comment the exact blocker once.
