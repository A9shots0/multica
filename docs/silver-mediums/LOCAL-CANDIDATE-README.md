# Local candidate

This preview integrates the accepted Items 1–14 source with the independently
audited adoption boundary and the focused meaningful-activity repair.
See `LOCAL-INTEGRATION-PROGRESS.md` for the latest verification state.

## Run this checkout

From the task worktree, with Docker, Python, Node and the repository's pnpm
dependencies available:

```powershell
./scripts/local-candidate.ps1 Build
./scripts/local-candidate.ps1 Start
node scripts/local-candidate-fixtures.mjs  # once, on the empty preview database
node scripts/owner-review-preview-fixtures.mjs # synthetic owner-review acceptance
./scripts/local-candidate.ps1 Status
```

Open `http://127.0.0.1:13004`. The synthetic sign-in is
`preview-owner@local.invalid` with local development code `654321`. The sample
initializer creates that account, its workspace and static display rows. It
marks onboarding complete to focus verification on the operational surfaces.
Sample records are explicitly labeled and prove no workflow execution.

The owner-review fixture adds one clearly labelled SAMPLE issue, one inert
Preview Mika identity with no runtime, one Chat review image, and one
owner-actionable Inbox decision. It creates no task and contacts no model or
provider. Re-running it while the question is active is idempotent. After the
question is answered, rerunning deliberately creates the next synthetic
acceptance question so the workflow can be tested again.

`./scripts/local-candidate.ps1 Stop` stops only this Compose project and retains
its database and upload volumes. `Start` reuses them without reseeding. Existing
pilot stacks and the accepted source are separate resources.

## Local boundary

- Unique Compose project: `smk-local-candidate-20260915`.
- Loopback web/API/database ports: 13004, 18084 and 15437.
- App, API and database use only an internal network. A fixed-target TCP
  gateway publishes their loopback ports; it has no application credentials.
  No provider runtime, agent or external
  integration is configured. The development sign-in code and sample database
  password belong only to this local preview.
- An explicitly empty Compose environment file prevents inherited `.env`
  loading. Preservation evidence is excluded from Docker build contexts.
- Build inputs are hashed. Successful builds record the source identifier and
  all three built image IDs. Startup rejects changed source or substituted images.
- Focused handler tests use a different temporary database on port 15436.

Evidence is in `.qualification-evidence/local-integration-20260915/`, including
source manifests, build receipts, test logs and the static fixture receipt.
The accepted Item 14 source archive is independently verified under
`.qualification-evidence/item14-accepted-checkpoint-20260915/`; its original
manifests are unchanged. The Makefile's local stewardship target verifies that
archive. Current candidate checks are recorded separately.

## Release boundary

The next release requires an authorized destination and production configuration:
production authentication and secrets, disabled development sign-in, the chosen
signup/access policy, TLS/origins, and durable backup custody. Item 13 still
records unverified off-machine readback and separate recovery-key custody.
No production release or exposure is authorized in this continuation.

Real agent work separately requires applicable runtime/capability and credential
authority. The frozen R01-like preparation default expires exclusively at
2026-09-22 13:15Z and grants no provider execution or permanent role selection.
The accepted CEF-7 evidence stays sealed; completed experiments are not rerun.

The independent upstream audit recommends preserving the pilot base and the
exact accepted delta. A blanket upstream merge remains a separate migration
and lifecycle integration project. Two isolated web hardening patches are
included in this candidate.
