# Controlled operational adoption — result, 2026-09-16

**Outcome: implemented, independently reviewed, corrected, privately deployed and
verified.** No provider was called, no credential was read, and no issue was
dispatched. Source `60fea34b9770-local-24b0121ef7a0` is running on the private
Alienware/Tailscale instance.

Snapshot: 2026-09-16T02:10Z to 2026-09-16T03:12Z (UTC). Evidence:
`.qualification-evidence/adoption-20260916/` (pre-release) and
`.qualification-evidence/production-adoption-20260916/` (release and deployment).

## What shipped

| Part | Result |
|---|---|
| A. Initial operating configuration | `silver-mediums-initial-operating-configuration-20260916`, active to 2026-09-30, revocable, with a controller lifecycle registry and a provider-free checker |
| B. Cleanup PowerShell defect | Fixed in both harnesses and in three recovery scripts; PowerShell 5 and Store stubs can never satisfy it |
| B. OneDrive readback defect | Now waits for the artifact to settle before hashing; passed first time twice on a 735 MB artifact |
| C. Secondary product identity | "Silver Mediums Agent Control Plane / Built on Multica" with an About surface, in the app shell and on all three sign-in steps, in four languages |
| D. Release | Fresh backup, isolated restore, rollback rehearsal, deploy, post-deployment verification, closing recovery point |

Exactly 17 files changed the build: 10 added, 7 modified, 0 removed. The
build-input delta against the previously deployed release is recorded in
`adoption-qualification.json` and contains nothing unrelated to this milestone.

## A. The initial operating configuration

Full record: `INITIAL-OPERATING-CONFIGURATION-20260916.md`. Configuration
SHA-256 `dadf6a215f9ed8fb6d4ed5b957a6be143e3226dab080452c4c9d63454aaffa6d`,
pinned by a golden-hash test and by the lifecycle registry.

It makes the accepted publisher → independent reviewer workflow usable for one
ordinary internal issue at a time, started by the owner, on this repository and
host, with a named distinct reviewer and no declared release gate. It routes
through the controller and the existing truthful workflow states, uses separate
execution and review sessions, and creates **zero permanent agent seats** — all
three roles stay task-scoped, as the accepted run did.

Twelve conditions stop the work and wake the owner, and the list is closed: a
condition the configuration does not recognise also stops the work. Automatic
dispatch, public access, production access, other production systems, other
repositories, outbound actions and Phone Farm access are all disabled and
asserted false by the validator.

**It does not make Claude the permanent answer.** The record states in
machine-checked form that both lanes run on the same provider so review
independence is session-level only; that the exact served model is not proven;
that the requested effort was never observed in the accepted run; and that
recovery-sensitive authority is **withheld** because this exact configuration
validly failed the R04 exact-once recovery case. Release, rollback, recovery and
production verification remain owner-operated.

Eight provisional items and five re-evaluation triggers are recorded, with an
explicit change path (successor record plus tombstone) and rollback action.

## B. The two operational defects

### PowerShell 7 resolution

The accepted run's cleanup failed one step because it invoked `pwsh`, which is
not on this host's PATH. Resolution now tries an explicit `MULTICA_POWERSHELL7`
override, PATH, `$PSHOME`, the well-known install roots, the Store path, the
Chocolatey path, the local Codex runtime dependency and the dotnet tool path —
and confirms each candidate by asking the executable for its own major version.

Guarantees, not relaxations:

- Windows PowerShell 5.x can never satisfy it. A stub reporting major 5 is
  rejected in both the PowerShell and the Python implementation.
- A zero-length Microsoft Store app-execution alias is skipped rather than
  launched, and the probe runs as a bounded child process, so an unattended
  scheduled run cannot hang on the Store.
- An override that is set but does not answer fails loudly by name instead of
  quietly resolving something else.
- No identity guard changed. The daemon-stop script body is byte-identical:
  case-sensitive executable-path comparison, the exact daemon-id pattern (now
  asserted by test, not only its error message) and the `taskkill` exit check.

Applied identically to both acceptance harnesses and to three recovery scripts.
The pre-correction bytes of both harnesses are preserved and hash-verified in
`.qualification-evidence/adoption-20260916/preserved-harness/`; the accepted
run's own evidence directory is untouched.

### OneDrive recovery readback

The verifier hashed the cloud copy the instant it appeared, which is a
predictable failure while a large encrypted artifact is still arriving. It now
waits for the copy to reach its recorded size and stop changing, then compares
the hash; it re-issues the "free up space" verb while waiting instead of once;
and off-machine custody has its own longer budget instead of borrowing the
local-copy one. Every wait is recorded in the receipt.

The hash requirement is unchanged and was not weakened:

- a settled, complete copy whose hash differs still **fails**, and fails
  promptly rather than after the timeout;
- an incomplete, absent, still-changing or unreadable copy is `sync_pending`,
  never a false mismatch;
- the post-rehydration provider readback hash check is untouched.

Proved twice in production conditions during this release: both backups passed
the cloud readback on the **first** attempt on a 735,238,104-byte artifact —
roughly twice the size of the 361 MB artifact that used to fail — with
`local_copy_attempts: 2` and 5 and 13 dehydrate attempts respectively.

## C. The secondary product identity

The app shell footer and all three sign-in steps now show
**Silver Mediums Agent Control Plane** with **Built on Multica** beneath it, and
an information button opening a short About surface.

Multica branding is preserved and is more visible than before, not less:

- the Multica mark renders unmodified, and sits on the "Built on Multica" line
  rather than beside the layer name, so it is never read as branding this layer;
- "Sign in to Multica" stays exactly where it was, above the badge;
- the About surface carries the Multica attribution, the copyright line
  `Multica © 2025-2026 Multica, Inc. Licensed under the Multica License.`
  matching `NOTICE`, and a link to the canonical project. Before this change the
  signed-in app displayed no Multica copyright or attribution at all;
- all four locales keep "Multica" in the byline, the attribution and the
  copyright, enforced by test.

The About surface explains nine things in plain language: honest status,
controlled routing, shared instructions, bounded permissions, independent
review, release and runtime checks, recovery and rollback, your Inbox, and usage
awareness. Every point was checked against implemented, verified behaviour. One
draft point claimed "one shared record of how this organization does things";
the review could not substantiate it, so it now describes the skills feature
that exists. The independent-review line is a one-line summary; the
configuration record behind it is more precise about session-level independence.

No broad visual redesign: the change is one footer row, one row on each sign-in
step, and one popover.

## D. Review, correction and verification

One independent reviewer — a separate agent instance from the implementer —
returned **REVISE** with 15 findings. All 15 were addressed in one consolidated
correction. The four that mattered most:

1. The record claimed revoking it "denies admission immediately". Nothing in the
   running product consults it. It now states plainly that it is a declarative
   contract checked out of band, and the validator rejects any other claim.
2. Revocation was not durable: clearing the revocation entry and flipping the
   status back was a two-line edit. The status word and the record must now
   agree, or the registry fails closed.
3. The configuration shipped active with no record of who authorized it. The
   registry now carries an activation block naming the owner, the authorization
   and the scope, and refuses to load without it.
4. With zero automatic corrections, a failed review had no declared destination.
   It now declares `blocked` plus `owner_decision_required`.

Also corrected: three unit assertions that only exercised their own mock; a
truncation guard covering one of two labels; an About panel that opened
mid-scroll; the sign-in gutter applied to one of three steps; a focus token used
outside its scope; and the same PowerShell defect left unfixed in a third script.

**Provider routing:** `provider-unavailable: orca runtime_unavailable
(orca-runtime.json absent) | fallback: in-session independent agent instance |
lane: independent review`. Orca worker dispatch was unavailable for this run, so
the review was a separate agent instance on a single provider. Recorded as
`review: single-provider`, not as a second-provider review.

### Checks run

| Check | Result |
|---|---|
| Initial operating configuration (Go) | 10 pass, including 38 mutation denials, 9 out-of-scope denials, all 12 owner gates and lifecycle revocation/tombstone/activation |
| `go vet` | clean |
| OneDrive waiting and PowerShell resolution | 12 pass |
| Existing recovery suite | 5 pass |
| Acceptance harness v2 and v3 | 18 pass each |
| Views branding and app shell | 70 pass |
| `pnpm typecheck` | 9 workspaces pass |
| Rendered branding (candidate) | 5 pass at 200 / 256 / 360 px, phone 390 px and 1920 px |
| Rendered branding (deployed) | 3 pass at 390 / 1280 / 1920 px |

The whole `agentqualification` package was run with and without the new files;
the failing set is byte-identical, so no new failure was introduced. Nine tests
in that package and the `inbox`/`chat` locale-parity tests were already failing
before this change because historical frozen manifests pin superseded document
and source hashes. That preservation is deliberate and is not repaired here.

## Release and deployment

| Stage | Result |
|---|---|
| Preflight | verified — exact reviewed images, matching compose hashes, loopback-only listeners, private Tailscale route, no public Funnel, zero runnable tasks / autopilots / triggers / webhooks / channels / online runtimes |
| Image archive | verified, 85 blobs, 5 expected images |
| Backup `20260916T025449Z-ea0d859d` | verified, all 10 components, replication verified |
| Isolated restore `ae3e246f4e` | verified, 145 user tables, uploads manifest matched, recovery point age 19.8 s |
| Application and rollback rehearsal `app8701907fc9` | verified against **both** the new and the rollback image on the restored database: health and readiness ok, 603 migrations before and after with none added or removed, identical row counts, cleanup verified |
| OneDrive off-machine readback | verified 02:59Z by dehydrate/rehydrate hash match, key custody verified |
| Deploy | reviewed images started, health and readiness ok, container identity and health matched |
| Closing recovery point `20260916T030645Z-89971836` | backup, isolated restore `3fe81ee797`, OneDrive readback and watchdog all verified |

Rollback was proved before deploying, not after: the rollback image
(`60fea34b9770-local-9eaa711d97b1`, the release accepted on 2026-09-16) was
booted against the restored database and reported healthy with identical data.

### Post-deployment verification

- `/health` reports `60fea34b9770-local-24b0121ef7a0`; `/healthz` reports
  `db: ok`, `migrations: ok`.
- Data preserved exactly: 1 user, 1 workspace, 51 issues, 149 comments, 120
  tasks, 0 runnable, 603 migrations, 10 active agents, 0 online runtimes.
- Runtime policy unchanged: `APP_ENV=production`, `ALLOW_SIGNUP=false`,
  `DISABLE_WORKSPACE_CREATION=true`, `MULTICA_OWNER_SMS_MODE=off`,
  `MULTICA_VCS_INTEGRATION_ENABLED=false`, controller
  `c43dc900-4554-4cbb-a664-759067b81118`.
- Listeners are loopback-only; the Tailscale route is private and `AllowFunnel`
  is unset.
- Operating configuration re-checked on the deployed source: lifecycle active,
  0 provider calls, 0 credentials read, 0 permanent seats, automatic dispatch /
  public / production / other-repository / Phone Farm access all false, recovery
  authority withheld, review independence `single_provider_distinct_session`.

**Evidence boundary for the branding check.** The signed-out surface was
rendered and asserted against the deployed instance itself. The signed-in
sidebar surface was rendered and asserted against the candidate preview, whose
frontend image is
`sha256:8fe461d27bdfac9ae5c71a2a410a8d044622969a5c478986d8515fb10e36a802` —
byte-identical to the image now serving production. No account, session or data
was created on the private instance to verify branding.

Neither the Orca Browser (runtime unavailable) nor the Chrome extension (not
connected) was available, so rendered verification used Playwright against real
Chromium. Screenshots are retained in both evidence directories.

## One unintended action, recorded

While checking that the corrected PowerShell resolver loaded, a probe imported
`scripts/completion-recovery-rehearsal.py`, which executes on import. It ran a
complete application and rollback rehearsal (`drill_id appc0306799e5`) and
overwrote `.qualification-evidence/production-completion-20260915/application-recovery-rehearsal.json`.

It was harmless and it passed: `status: verified`, `cleanup_verified: true`, 603
migrations unchanged, identical row counts, production never restarted
(`started_at 2026-09-15T22:12:43Z` unchanged at the time), and no drill
container, volume or network survived. The file it replaced described the same
release and the same check. It is recorded here because it was not intended, not
because it caused harm. The rehearsal scripts are executable modules, so they
must be run, never imported.

## Hub write-back and two pre-existing gate conditions

`projects/multica/CONTEXT.md` and `projects/multica/CHANGELOG.md` in
`C:\Dev\Silver-Mediums-Knowledge` were updated with the deployed source label,
the active configuration and its limits, the two defect fixes, the branding
change, and the review outcome.

Two stewardship gates do not pass, and neither was caused by this milestone:

- `silver-mediums-stewardship-check` reports eight hash mismatches against the
  hub manifest: the root `CHANGELOG.md` and `CURRENT_STATE.md`,
  `projects/registry.json`, and the canopy-backend, canopy-desktop, crm and
  viral-queue records. **None of them is a `projects/multica/*` path** — Multica
  has no entry in that manifest at all (20 sources, zero of them Multica), so
  the hub write-back performed here cannot have moved any pinned hash. The hub
  checkout was already dirty across those projects before this run.
- `silver-mediums-item14-local-stewardship-check` cannot run in this worktree:
  `.qualification-evidence/item14-accepted-checkpoint-20260915/` is absent here,
  so the checker fails resolving its v2 predecessor manifest. That is a missing
  preserved checkpoint, not drift, and it is not repaired by fabricating one.

Both are recorded rather than worked around.

## What this does not do

No provider run, no credential read, no permanent role or model selection, no
wider automatic execution, no public exposure, no outbound communication, no
Phone Farm access, no other repository or host touched, and no production data
mutated. Starting a real issue under this configuration is a separate action the
owner takes.

## Next decision

None outstanding. The configuration expires on its own at 2026-09-30T00:00:00Z.
Widening the eligible work class, enabling the automatic correction cycle,
adding a second provider for the review lane, or granting recovery-sensitive
authority each need a successor record and its own evidence.
