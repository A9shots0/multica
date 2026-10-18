# Outcome

CRM is the canonical authority. One synthetic consumer displays the wrong role.
Find the first divergence, repair only the faulty consumer, add or preserve
regression coverage, run the affected and healthy-control checks, and leave a
short `EVIDENCE.md` naming the local-only proof and untouched boundaries.

Before editing, run the supplied allowlisted check once and preserve its
failing result as the pre-change baseline. After the bounded repair, run that
same check again and use the passing result in `EVIDENCE.md`.

Do not change CRM authority, the correct consumer, or the Phone Farm contract
fixture. Do not access any host or repository outside this workspace.
