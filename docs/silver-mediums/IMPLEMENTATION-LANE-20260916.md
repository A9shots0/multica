# Implementation lane and repeatability trial — 2026-09-16

**Status: lane built, proved once on a real defect, and frozen. Repeatability
trial in progress.** No production access, deployment, outbound communication,
Phone Farm access, cross-repository authority or `git push` at any point. No
permanent role, model or seat selection.

## Why this exists

The accepted publication workflow could not do engineering work. Traced in
source, not inferred: its provider child runs with no tools and no repository
access (`provider_child_tools_allowed: false`,
`provider_child_repository_access_allowed: false`), and the broker's only two
permitted actions are publishing a frozen pre-approved document and recording an
independent review of that publication. `roleSlot()` rejects anything else.
Every accepted end-to-end "implementation → review" proof in the Item 14 record
used a **fake** provider; the only real-provider runs were document verification.

A five-run repeatability trial on that workflow would therefore have measured
the repeatability of document verification, not of getting work done. The owner
chose to pause it and build the smallest genuine code-changing lane first.

## What one run does

```text
isolated worktree -> offline dependencies -> executor session
  -> harness runs the task's own verification -> independent reviewer session
  -> at most one bounded correction -> cleanup
```

- **Isolated worktree.** A detached linked worktree created per run and removed
  at the end. The executor never sees the main checkout.
- **Offline dependencies.** `pnpm install --offline --frozen-lockfile
  --ignore-scripts`, so the worktree can run the repository's real tests without
  reaching the network. This is harness setup, not an agent action.
- **Executor session.** `Read`, `Grep`, `Glob`, `Edit`, `Write`, and `Bash`
  restricted to an allowlist of this repository's own check commands. The
  provider CLI's `--restricted` mode confines tools to the working directory.
  No MCP, no session persistence, no browser, no slash commands.
- **The harness runs the verification, not the agent.** An agent claiming
  success does not make a run successful. The task's own command is executed by
  the harness before and after the change.
- **Independent reviewer session.** A separate session with `Read`, `Grep`,
  `Glob` only — no `Edit`, no `Write`, no `Bash`. It sees the diff, the scope
  and the harness's own verification result.
- **At most one correction**, and only when the reviewer returns findings.
- **Containment check.** After the fact, every touched path is compared against
  the task's allowed paths; anything outside fails the run.
- **Cleanup.** Authority revoked, worktree removed, both verified.

## Authority is eligibility, never standing permission

Authority is minted fresh per run with a nonce, an expiry and a revocation flag,
and is checked immediately before every provider session. It records
`production_access`, `deployment`, `outbound_communication`, `phone_farm`,
`other_repositories` and `permanent_selection` as false. Nothing is reused
between runs. Revoking or expiring it denies every further session in that run.

The forbidden-capability set is the vocabulary the accepted benchmark records
already used: `browser`, `git_push`, `deployment`, `external_messaging`,
`credential_access`, `phone_farm`, `host_files_outside_workspace`.

## First proof — chat locale parity

| | |
|---|---|
| Defect | `en/chat.json` defined `page.purpose_primary` and `page.purpose_handoff` with no counterpart in ja, ko or zh-Hans, so those locales silently fell back to English |
| Recorded by | the failing `packages/views/locales/parity.test.ts` |
| Run | `20260916T054524Z-3f9f7e6f` |
| First attempt passed | **yes** |
| Correction used | no |
| Executor | 49.2 s, `claude-sonnet-4-6`, 11 turns |
| Reviewer | 4.4 s, `claude-sonnet-4-6`, 1 turn |
| Cost-equivalent | $0.3521 |
| Containment | 3 files touched, 0 outside the allowed paths |
| Cleanup | authority revoked, worktree removed, both verified |

The six translations are accurate to the English source, correctly placed inside
`page`, and match the surrounding tone. Applied and committed as
`243c5a6df`.

### Two harness failures recorded before that proof

Neither was an agent failure and neither was retried into a success. They are
kept because the safety machinery is what they actually tested.

| Run | Failure | Cleanup |
|---|---|---|
| `20260916T054016Z-e082281b` | `pnpm` is a `.CMD` shim that `subprocess` cannot resolve without a shell | complete |
| `20260916T054054Z-70cddce9` | subprocess text decoding defaulted to cp1252 and could not read the CJK bundles | complete |

Both times authority was revoked and the worktree removed correctly. The lane
was corrected — executables resolved through `shutil.which`, all subprocess I/O
pinned to UTF-8 — and the run re-attempted as a **new** run, not a retry of a
recorded failure.

## The frozen lane

`scripts/implementation-lane/FROZEN.json` pins the SHA-256 of `lane.py`,
`run_task.py` and `test_lane.py` as of the first proof. The repeatability trial
runs against that unchanged lane. 16 provider-free tests cover the verdict
contract, authority expiry and revocation, the tool policy and containment.

## Repeatability trial plan

Five real code-changing tasks, sequential, each selected from recorded evidence.

| # | Task | Shape | Graded by |
|---|---|---|---|
| 1 | chat locale parity, 6 keys | mechanical | an existing failing test |
| 2 | inbox locale parity, 180 keys | volume and long context | an existing failing test |
| 3 | workspace-leave known debt | **judgment** | nothing — the executor must decide |
| 4 | to be selected from recorded evidence | | |
| 5 | to be selected from recorded evidence | | |

Task 3 was added on a proposal relayed from the owner's planning session by
another Claude session, which this session independently agreed with. The
reasoning is sound and worth recording: tasks 1 and 2 are the same shape. 180
keys is *bigger* than 6, not *harder*, because in both cases a test already
defines what correct means. If all five tasks had a pre-written answer, the
trial would prove the lane repeats on problems whose correctness was already
determined and leave the day-to-day question unanswered.

### Task 3 — the judgment task

`CLAUDE.md:179` records: "Workspace leave currently clears/navigates before
mutation only to avoid the `member:removed` realtime race; treat that as known
debt, not a reusable pattern." No test anywhere references `leaveWorkspace`, so
nothing defines correct behaviour. The executor must decide what correct is,
write the test, then fix it. The sibling delete flow solves its equivalent race
with a `markWorkspaceDeletePending` guard, but whether that is the right answer
here is exactly the judgment being tested.

Three things are recorded for the first time on that task, because each is
genuinely exercised for the first time:

1. **Reviewer independence stops being cosmetic.** With a failing test, the test
   arbitrates and the reviewer is a second opinion. On a judgment task the
   reviewer's opinion is the only check. Both lanes run on one subscription, so
   independence is session-level, not provider-level — the effect of that on the
   outcome is stated explicitly rather than assumed away.
2. **"At most one correction" may prove too tight.** It will not be loosened
   during the trial. If one correction is insufficient, that is recorded as a
   finding for the next configuration revision.
3. **How the executor decided what correct behaviour was**, not merely whether
   the result passed.

## Trial results so far

Seven runs, six completed. Four of the five non-successes were **not** agent
failures — three were defects in my own harness and one was the host killing the
process for memory.

| Run | Outcome | Cost | What it actually shows |
|---|---|---|---|
| chat `…e082281b` | failed | $0 | harness: `pnpm` is a `.CMD` shim `subprocess` cannot resolve |
| chat `…70cddce9` | failed | $0 | harness: subprocess decoding defaulted to cp1252 |
| chat `…3f9f7e6f` | **success** | $0.3521 | first attempt, no correction, 3 files, 0 outside scope |
| inbox `…20334296` | failed | $3.5180 | reviewer caught a real translation error; correction was silently denied by a harness permission defect |
| inbox `…4520c188` | **success** | $0.3280 | first attempt, no correction, 160/160 parity suite passes |
| leave `…fd89dafb` | killed | $0 | host memory, during dependency install, before any provider session |
| leave (current) | running | | the judgment task |

Every failed run cleaned up correctly except the killed one, which could not:
a hard kill bypasses the cleanup block.

### The single most useful result

On inbox `…20334296` the parity test **passed**, the executor certified itself
as done, and the independent reviewer still failed it — because the Chinese for
"Hold" was written `搞置`, a wrong character that is not a word, instead of
`搁置`. One character in 180 keys. The test structurally could not catch it: it
checks that keys *exist*, never that translations are *correct*.

That is the clearest evidence in the whole programme that independent review is
doing real work rather than ceremony, and it survives the harness defect that
followed it — the reviewer was reading the diff, not depending on permissions.

### A cost conclusion that was wrong, and is corrected here

The first inbox run took 25 minutes, hit its 30-turn ceiling and cost $3.52,
and this session initially reported that as evidence that bulk work is the
lane's weakest case. **That was wrong.** The entire cost was the executor
fighting a permission surface that had been misconfigured. On the corrected
lane the same 180-key task took 107 seconds and cost $0.33 — about the same as
the six-key task.

| | 6 keys | 180 keys (broken) | 180 keys (fixed) |
|---|---|---|---|
| Executor | 49 s | 25 min, turn ceiling | 107 s |
| Cost | $0.3521 | $3.5180 | $0.3280 |

Volume is close to free in this lane. The earlier conclusion is retracted rather
than quietly dropped.

### Findings recorded, not acted on during the trial

1. **A hard kill bypasses cleanup.** The killed run left an orphaned worktree
   and an unrevoked authority record. The authority grants no capability by
   itself and carries an expiry, so the exposure is a stale file and disk usage
   rather than live permission — but the lane needs an orphan sweep. Recorded
   for the next configuration revision; **not** changed mid-trial.
2. **The provider receipt stores frame types, not rate-limit detail.** When the
   correction session was denied, the receipt could not say why. Worth widening.
3. **"At most one correction" is still untested.** The only run that reached a
   correction had that correction silently denied, so it never really ran.

## The go/no-go will give three answers, not one

At the end of the trial, three separate judgements:

- **(a) Does the lane repeat reliably?** — first-attempt rate, correction rate,
  cleanup integrity, containment.
- **(b) Is the work actually good?** — quality of the diffs, whether the
  reviewer caught anything real, whether the judgment task produced a defensible
  design decision.
- **(c) Is it worth the cost?** — time and subscription usage per task.

A yes on (a) and (b) with an unclear (c) is a different decision from a yes on
all three, and will be reported as such.

## Boundaries

This lane produces a reviewed patch. It never commits, never pushes, and never
applies its own work; applying a patch is a separate owner-visible step. It does
not integrate with the Multica server, task queue or workflow states — that is a
later step, not part of proving the lane can change code safely.

## Final trial result — 2026-09-16T09:05Z

All five tasks are complete, applied and committed. **Each passed on the first
attempt of its final run, and none needed a correction.**

| # | Task | Shape | Final run | Cost | Agent time | Human verification needed |
|---|---|---|---|---|---|---|
| 1 | chat locale parity, 6 keys | mechanical | `…3f9f7e6f` | $0.352 | 54 s | translation spot-check |
| 2 | inbox locale parity, 180 keys | volume | `…4520c188` | $0.328 | 115 s | translation check; 160/160 parity |
| 3 | workspace-leave known debt | **judgment** | `…83c76d6a` | $0.739 | 287 s | test-fails-before check; flaky-suite triage |
| 4 | preview allowlist single source | cross-language | `…6113412a` | $0.967 | 366 s | Go parity 3,026 inputs; false CI claim corrected |
| 5 | slug lookup cache | **judgment**, false premise | `…5caf081b` | $0.905 | 324 s | Go build/vet/test; unrequested change reviewed |

Across all 13 recorded runs, including every failure: **$7.77** and 53 minutes
of agent time. The five final runs cost **$3.29**, about $0.66 per task.

### Every non-success, attributed

| Run | Outcome | Cause | Agent's fault? |
|---|---|---|---|
| chat `…e082281b` | failed | harness: `pnpm` `.CMD` shim unresolvable | no |
| chat `…70cddce9` | failed | harness: cp1252 subprocess decoding | no |
| inbox `…20334296` | failed | reviewer caught a real wrong character; correction then silently denied by a harness permission defect | partly — the translation error was real |
| leave `…fd89dafb` | killed | host memory, before any provider session | no |
| leave `…9f456ea1` | killed | host memory, before any provider session | no |
| leave `…0ed50524` | failed | harness: invalid pnpm flag | no |
| leave `…9cb5ba6b` | failed | task spec error: the baseline guard correctly refused a task with nothing failing | no |
| leave `…c502cdc6` | **"success", work lost** | harness: new files were never captured, then deleted with the worktree | no |

Seven of eight problems were the harness, the task specification or the host.
One was the agent, and the reviewer caught it.

### What the judgment tasks showed

**Task 3.** No test defined correct behaviour. The executor built a
pending-leave guard modelled on delete, but reasoned that leave is not
symmetric: a failed leave leaves the user still a member, so unlike delete the
marker must be cleared on failure or a later admin removal would be ignored. That
distinction was not hinted at. Two independent runs reached the same design.
Verified by hand that 2 of its 5 tests fail against the previous implementation,
so the test encodes a real behaviour change rather than restating old code.

**Task 5.** The recorded TODO asserted that slugs are immutable and therefore
safe to cache. The executor checked the premise before acting and found it
partly false: slugs never change on a live row, but workspaces are hard-deleted
and a slug can be reused, so a cache would route requests to a deleted
workspace. It declined to cache and replaced the misleading TODO with that
finding. This session had independently reached the same conclusion before the
run.

It also made **one change it was not asked to make**: it swapped which slug
input wins in workspace identity resolution. The claim behind it was verified —
the sibling resolver checks the header first and this one checked the query
first, and no existing test covered a request carrying both — so the change was
kept. The reviewer accepted it without flagging it as out of scope.

### Reviewer independence, measured rather than assumed

Both lanes ran on one subscription, so independence was session-level only.
What that produced in practice:

- **Caught one real defect** a test could not: `搞置` for "Hold" in 180 keys.
- **Approved a change on a false factual premise.** On task 4 it justified a
  behaviour edit by claiming Go's `path.Ext` returns empty for dotfiles. It
  returns `".gitignore"`. The change happened to be harmless — verified equal on
  3,026 inputs — but the reasoning was wrong.
- **Missed a false guarantee.** Task 4's JSON source claimed "CI re-runs the
  generator and fails on any diff". CI regenerates reserved slugs only. The real
  guard is the drift test in the sharded views job. Corrected by hand in a
  separate commit.
- **Did not flag scope creep** on task 5.

A same-provider reviewer is useful and not sufficient. Claims about the rest of
the system still needed a human or a deterministic check.

### "At most one correction"

Still untested. The only run that reached a correction had it silently denied by
a harness defect. No finding either way.

### Findings for the next lane revision (not changed during the trial)

1. A hard kill bypasses cleanup — add an orphan sweep at lane start.
2. The executor cannot run `go`, so every Go change needed human verification —
   allow `go build`, `go vet` and `go test` for Go-scoped tasks.
3. Mechanically confirm a judgment task's new test fails against the original
   code, instead of relying on the reviewer.
4. Skip the pnpm install for Go-only tasks; task 5 spent 243 s installing
   JavaScript dependencies it never used.
5. Store denial and rate-limit detail in provider receipts.
6. Require the reviewer to list any change outside the stated objective.
7. Several `packages/views` settings tests fail intermittently under host memory
   pressure (21–30 failures, varying by run); pre-existing and unrelated to this
   trial, but it makes whole-suite verification unreliable on this machine.

## Go / no-go: three answers

**(a) Does the lane repeat reliably? — Not yet proven.** The *agent* repeated:
five tasks, five first-attempt passes, zero corrections, and the judgment task
reached the same design twice. The *lane* did not: it changed four times during
the trial to fix defects, and only three runs executed on its final version.
Three is too few to call it reliable.

**(b) Is the work actually good? — Yes, with human verification of claims.**
The code was genuinely good, including two non-obvious correct judgments. But
the same-provider reviewer approved one false premise, missed one false
guarantee and let one unrequested change through. The work should be verified
before it is applied, not merely reviewed by a second session of the same model.

**(c) Is it worth the cost? — Yes on money; unclear on operator time.** About
$0.66 per completed task and a few minutes of agent time is cheap. The real costs
were a 1–5 minute dependency setup per run that the host killed twice at 94%
memory, and the human time spent verifying Go changes the agent could not run.

**Recommendation: GO for supervised use; NO-GO as the unsupervised standard
workflow yet.** Use the lane now for small, reversible, single-repository tasks,
with a person verifying the patch before it is applied. Make findings 1–4 and 6
the next revision, then run five more tasks on that unchanged version. Adopt it
as the standard workflow only if that run is clean.

## Lane revision 2 (freeze v5) and the clean-pass definition — 2026-09-16T18:30Z

The owner approved the proposed revision and a second five-task trial. The
revision is built, tested and frozen. **No trial run has started.**

### What changed

Approved by the owner:

- Go tasks may run `go build`, `go vet` and `go test`. Other tasks may not.
- Before every run, leftover worktrees from an interrupted run are swept. A clean
  one is removed. One holding changes, or an unknown directory, blocks the run so
  a person looks first. Nothing holding work is ever deleted.
- The harness checks that the test catches the defect: it takes the product
  change out, keeps the test, re-runs verification (which must fail), then puts
  the change back and checks it is byte-identical.
- Go-only tasks skip the JavaScript dependency install.
- The reviewer must list every change the task did not ask for, even a harmless
  one, in `out_of_scope_changes`.
- Cleanup guard: before deleting a run's worktree, `git status` (a different view
  from the `git diff` capture) must find nothing the patch lacks and no new
  ignored file. Otherwise the worktree is kept and the run fails.
- Every run checks host memory first: at least 4 GB available physical memory
  and 4 GB available commit.
- Provider receipts keep permission denials, rate-limit frames, API error status
  and the stderr tail.

Found while building, and added because the approved changes needed them:

- **Database isolation.** A native PostgreSQL listens on `localhost:5432`, and
  the server's DB-backed tests default to that address. Giving agents `go test`
  without isolation could have written test fixtures into a real database. Every
  command the lane starts now gets `DATABASE_URL` pointing at `127.0.0.1:9`, where
  nothing listens.
- **Credential withholding.** The operator's shell held connection strings for
  four other applications' databases (LinkHub, VQ, Canopy, CRM) and this
  session's messaging token. The first trial's executors could run `node`, so
  they could have read them. There is no evidence any did. Variables whose names
  look like credentials are now withheld from everything the lane starts.
- **Offline Go.** `GOPROXY=off`, `GOFLAGS=-mod=readonly -p=2`. `GOTOOLCHAIN=local`
  was tried and removed: this host's go 1.26.1 must switch to the 1.26.6 the
  module requires, which works offline from the local cache.
- **Freeze enforcement.** Every run compares the lane's file hashes with
  `FROZEN.json` and refuses to start on any difference.
- Build caches written by the repository's own check commands (`node_modules`,
  `.turbo`, `.next`, `*.tsbuildinfo`) do not block cleanup but are recorded.

Not changed: the tool list and existing Bash allowlist, forbidden capabilities,
path containment, the one-correction limit, authority minting and revocation,
the read-only reviewer, harness-run verification, and the model and effort.

**Residual risk, unchanged and not closed by this revision:** running tests means
running code the agent wrote, with the operator's user rights. The boundary is
a process and environment boundary, not an operating-system sandbox.

### Verification

- 36 provider-free tests pass. They include real git in throwaway repositories
  for the guard, the fails-before check and the orphan sweep, plus end-to-end
  runs of the whole runner with a stand-in provider: a clean run, a run that
  leaves uncaptured output (worktree kept, run failed), and a starved host
  (refused before any authority or worktree).
- `TestWorkspaceSlugResolution` passes offline under the sandbox environment
  (11 subtests, 2 min 15 s on this host).
- Live admission check at 18:26Z: freeze intact, no orphans, six credential
  variables withheld, **memory refused** (91% load, 2.59 GB physical and 2.0 GB
  commit available).
- No independent review of the revision was run.

### Clean pass, defined before any run

A run is a **clean pass** only if all of these hold:

1. It was admitted: freeze intact, memory floor met, no blocking orphan.
2. Baseline verification failed.
3. Executor and reviewer each returned a valid verdict, and the reviewer's
   includes `out_of_scope_changes`.
4. No Edit or Write permission denial in any receipt.
5. Harness verification passed after the last agent session.
6. The test fails without the product change.
7. No path outside the task's allowed scope.
8. The reviewer passed on the first review, with no correction.
9. Authority revoked, worktree removed, cleanup guard safe.
10. A human check of the patch finds no real defect the reviewer missed and no
    unrequested change the reviewer failed to list.

A pass after the one correction counts as a **pass, not a clean pass**. A
host-blocked attempt is not a run and does not count either way. Three or more
in one trial is a reliability finding in itself.

The **trial is clean** only if: five different real tasks run once each, with no
reruns; at least four are clean passes and the fifth is at least a pass with
correction; the lane is not changed during the trial; there are no harness
failures and no lost work; and the human check finds no real defect the reviewer
missed. **Adopt the lane as the standard supervised workflow only if the trial
is clean.** A trial that is not clean is recorded as observed, not retried into a
pass.

### Task selection — started, not finished

- Recorded-debt TODOs are almost exhausted in the code.
- Constraint found: `server/internal/handler` tests exit green without running
  when no database is reachable (`handler_test.go` TestMain). With the lane's
  database isolation, no handler-package task can have a passing verification.
  Go tasks must come from packages with database-free tests.
- Upstream open bugs checked against this copy: #7386 (skill bundle timeout) is
  already addressed here, and #7653 (CLI invalid JSON) does not reproduce from
  the code, since the printer uses `json.Encoder`. #7158 (`Europe/Kiev`) is not in
  tracked source; the list likely comes from the runtime and is a possible
  judgment task, but it changes a visible label.

### Before the trial can start

1. Host memory must be above the floor. The lane refuses on its own until then.
2. Five tasks must be chosen and written into `tasks/`.
3. The owner's online-copy request came first. It is waiting for the owner's own
   confirmation in the session, because a relayed approval is not used for
   creating a repository or pushing code.

## Trial 2 result on lane v5 — 2026-09-16T19:50Z

**Result: NOT a clean trial under the definition written before it started.
All five runs passed every mechanical gate on the first attempt with no
correction, but the human check found a real defect in run 5 that both the
tests and the independent reviewer missed.** The lane stays GO for supervised
use and is not adopted as the unsupervised standard.

### Runs (one each, no reruns, lane unchanged: freeze intact on every run)

| # | Task | Mechanical gates | Human check | Cost | Agent time |
|---|---|---|---|---|---|
| 1 | inbox count responses | all pass, first attempt | no defect — **clean** | $0.72 | 86 s |
| 2 | issue batch + assignee frequency | all pass, first attempt | no defect — **clean** | $0.73 | 116 s |
| 3 | agent task snapshot / run counts / active task | all pass, first attempt | no defect; reused the existing schema; shapes match the server — **clean** | $0.82 | 122 s |
| 4 | CLI token response (**judgment**) | all pass, first attempt | correct decision: fail closed; login page already catches it — **clean** | $0.48 | 104 s |
| 5 | issue usage + task messages | all pass, first attempt | **real defect missed** — not clean | $1.16 | 182 s |

Totals: $3.92, 609 s of agent time, about 4 minutes per run end to end
including a 43-51 s offline install. No host refusal, no orphan, no permission
denial, no rate limit, no correction, no lost work. Every mechanical
fails-before check detected its defect. The reviewer listed out-of-scope
changes on three runs, all harmless added comments.

### The run 5 defect

`TaskMessagePayloadSchema` requires `issue_id: z.string()`. The server's
`protocol.TaskMessagePayload` marks `issue_id` as `omitempty`, and
`ListTaskMessagesByUser` fills it from `task.IssueID`, which is empty for chat
tasks. So every chat task's message list would fail validation and fall back to
an empty list. `taskMessagesOptions` in `packages/core/chat/queries.ts` feeds the
live chat view and completed assistant messages with `staleTime: Infinity`, so
completed chat timelines would render empty and not refetch. The tests only used
issue-bound messages. The reviewer passed it and noted only that a comment about
`.catch` was inaccurate. The comment claimed malformed items are dropped one by
one; they are not. Found by reading the server struct; not reproduced at runtime.
**The patch was not applied.**

This is the same class of miss as trial 1: the reviewer checks the diff against
the task and the client's own types, not against what the server actually sends.

### Applied

Runs 1-4 were applied to the accepted branch after the human check, one commit
each (`c5c2aec0f`, `f4369f279`, `b6990ef29`, `3d97bd9a0`). The core API suite
(10 files, 467 tests) passes and `tsc --noEmit` on `packages/core` is clean. Not
deployed.

### Findings about the lane itself (recorded, not fixed during the trial)

- **Patch evidence is not byte-exact.** `final.patch` and the other `.diff` files
  are written in Python text mode, so on Windows every line ends CRLF. They do
  not apply with `git apply` as saved and had to be normalised first. The
  in-memory comparisons the gates use were unaffected.
- **Go access was not exercised.** No database-free Go task was found in the
  recorded backlog. `server/internal/handler` tests exit green without running
  when no database is reachable, so handler tasks cannot be verified inside the
  lane.
- Side finding while checking run 3: `CancelTask` (`server/internal/handler/daemon.go:5434`)
  parses the `taskId` URL parameter with the panicking `parseUUID`, against the
  repository's UUID rule. A malformed id would panic the request instead of
  returning 400. Not a lane task because of the database constraint.

### What would make the next trial likely to be clean

The one miss in each trial was a claim about the real system (server shape, CI
behaviour, language semantics) that neither session checked. A deterministic
check would close this class better than a stronger prompt. For API schema work,
that means comparing each required schema field with the Go struct's JSON tag,
where `omitempty` must be optional. Alternatively, the reviewer could be given
the relevant server file. That is a lane change and needs owner approval before
another trial.

### Online copy — still not done

This session's safety check refused `gh repo create` for the private
repository, even after the owner's direct "do it now". Nothing was created or
pushed. Before the attempt, a secret scan of all 37 commits not in upstream
found no local secret value (database password, JWT secret, VCS secret key,
Telnyx key) in the pushed content; the only match was a default setting name
copied from `.env.example`.

## Lane v6, trial 3, and lane v7 — 2026-09-16T20:00Z-21:00Z

The owner's answers arrived through the planning session: add the server check,
fix the patch files, run trial 3, fix the cancel-task crash, and compare the old
checkout read-only. A standing rule came with them: make engineering decisions
without asking.

### Lane v6 (`0ddef253a`)

- **Server-contract check.** After each agent session, every field a patch adds
  as required to a zod schema is looked up in the Go server's JSON tags. A field
  that is `omitempty` anywhere becomes a warning. The reviewer must return one
  resolution per warning, naming the Go file and struct it checked, or the run
  fails.
- Reviewer turns raised from 6 to 25. The executor is told to read the Go
  handler before writing client parsing.
- Patches and diffs are written as bytes. Verified in practice: trial 3's patches
  applied exactly as saved.

### Trial 3 on v6 — not completed

| # | Task | Result |
|---|---|---|
| 1 | inbox item mutations | all gates passed; 6 contract warnings, each resolved by the reviewer with correct file and line citations |
| 2 | comment mutations | all gates passed; 0 warnings; server builds every comment with `commentToResponse`, which never emits null lists |
| 3 | reaction responses | **killed by the session for low system memory** during review; agent work verified captured, authority revoked, worktree removed by hand |
| 4 | task action responses | **killed** during dependency install; no agent session |
| 5 | agent env response | not started |

The memory was used mostly by 22 other Claude processes (about 4.4 GB), not by
the lane. Both kills happened after the lane had admitted the run above its 4 GB
floor.

**Runs 1 and 2 were not clean after all.** Applying them exposed two defects
that no gate and no human check caught:

- Run 2's stricter `updateComment` broke an existing test in
  `api/client.test.ts`, which mocked the comment write with `{}`. The lane only
  ran the task's new test file.
- Run 1's new test cast `InboxItem` to `Record<string, unknown>` without going
  through `unknown`, which `tsc` rejects. The lane never type-checks.

Both were fixed by hand while applying (`480f3f4e4`). Afterwards: core API
suite 487/487, `tsc --noEmit` on `packages/core` clean.

### Lane v7 (`fb2ce605f`)

- **Regression checks.** Tasks declare `regression_commands`, such as the whole
  API suite and the core type check. The harness runs them after each agent
  session and shows the results to the reviewer; a failure fails the run.
- **Narrower contract check.** JSON tags are read only from
  `server/internal/handler` and `server/pkg/protocol`, and moved field lines are
  ignored. Replayed on all eight real patches from trials 2 and 3, warnings fell
  from 21 to 6, and trial 2's real `issue_id` defect is still flagged.
- Worktree removal uses the extended-length path prefix.
- 42 provider-free tests.
- Still open: memory is checked only at admission.

Trial 4 is pre-registered (`1c93a0d64`): the three unfinished trial 3 tasks plus
agent lifecycle responses and inbox archive and owner-decision responses (the
judgment task), all with API-suite and typecheck regression commands. It has not
run.

### Cancel-task crash (`f7ef89c17`)

- `CancelTask` passed the `taskId` URL parameter to `MustParseUUID`. A malformed
  id panicked and came back as a 500 through the recoverer. It now returns 400.
- The same class existed in `isCurrentWorkflowLeaderTask`: an absent or
  malformed `X-Task-ID` header panicked before its own validity check. It now
  means "not the leader's task".
- Every other `MustParseUUID` call on request input in the handler package was
  checked and is validated first.
- Verification ran against a disposable migrated `pgvector/pgvector:pg17`
  container, removed afterwards: both new tests panic on the old code and pass
  now, 59 related tests pass, and `go vet` is clean.
- **Pre-existing failure found:**
  `TestWorkflowControlCommentBeforeMissingDispatchDoesNotRaceLeaderFallback` fails
  4 of 4 times with and without the change ("terminal recovery tasks = 0, want
  exactly 1"). Not investigated yet.

### Old checkout comparison

See `OLD-CHECKOUT-COMPARISON-20260916.md`. Of 1,749 uncommitted files, 1,696 are
identical to the accepted branch, 53 differ, and none exist only there. The 51
whose content is nowhere in accepted history are archived and verified at
`A:/Multica-Recovery/old-checkout-unique-files-20260916T201457Z.tar`. Nothing in
the old checkout was changed.

## Lane v8 and trial 4 — 2026-09-16T22:00Z-22:45Z

### Why v8

The host now runs permanently near 90% memory. A kernel nonpaged-pool leak
(7.3 GB, where under 1 GB is normal) only clears on a reboot, and the owner will
not reboot. The Docker and WSL VMs must stay up because they run the live
private Multica instance. Trial 3 lost two runs to session memory kills. So the
lane was changed to fit the machine (`508b86d89`):

- **Warm dependency install.** One offline pnpm install per lockfile and scope,
  kept in `implementation-lane-warm/<key>`. A task worktree gets real
  `node_modules` directories whose entries are junctions into the warm install.
  Entries pnpm links to a workspace package's own source (five for
  `@multica/core`) point at the task worktree instead, so tests never run stale
  code. Junctions are removed before any recursive delete, in normal cleanup and
  in the orphan sweep.
- **Dependency fingerprint.** The size and mtime of all 27,501 files in the warm
  install are recorded before and after each run. A change fails the run and
  discards the install, because an agent could otherwise make its tests pass by
  editing a dependency.
- **Wait instead of refuse.** Admission and every heavy step wait with backoff
  for 2.5 GB physical and 3.0 GB commit, up to 30 minutes.
- **Memory sampler.** The lane's process tree and the host are sampled every
  3 s; each outcome records per-phase peaks.
- **Single-worker vitest** in trial 4's commands.
- A provider-free smoke test on the real repo passed first: the install took
  38 s once, linking 3 s, the API suite ran 487/487 and `tsc` was clean through
  the links, and the warm install was unchanged afterwards.
- 50 provider-free tests.

### Trial 4 result

| # | Task | Result | Lane peak | Host min | Cost |
|---|---|---|---|---|---|
| 1 | reaction responses | **clean pass**; 3 contract warnings resolved correctly | 776 MB | 2.75 GB | $1.16 |
| 2 | task action responses | **clean pass** | 792 MB | 2.57 GB | $1.59 |
| 3 | agent lifecycle responses | **clean pass**; 3 warnings resolved; `AgentResponse` list fields verified never null | 779 MB | 2.53 GB | $1.43 |
| 4 | inbox archive + owner decision (**judgment**) | **clean pass**; `event_id` is present on both success paths, so the new throw cannot fire on a real success | 770 MB | 1.58 GB | $1.22 |
| 5 | agent env responses (**judgment**) | **pass with correction** | 1,187 MB | 2.90 GB | $1.14 |

Totals: $6.54, no memory wait was needed, no kill, no harness failure, no lost
work, and the warm install was unchanged after every run.

**Memory requirement, measured.** A normal run peaks at 770-800 MB, always in
the post-change checks (`tsc`). Agent sessions take 390-490 MB and reviews
280-470 MB. One run reached 1,187 MB while `tsc` was failing. Budget about
1.2 GB of headroom for the lane.

**Run 5.** The first attempt left an unused import; the `tsc` regression check
failed, the reviewer turned that into a finding, and the one correction fixed
it. This is exactly the gap v7 added regression checks for. The final change
validates the env responses, never logs the received payload (new
`ParseOptions.redactReceived`), and throws on malformed data. zod issues were
checked to carry key names but not values. One comment was false: it said the
strict schema "catches" extra fields, but zod strips them silently. Both reviews
missed that; it was corrected when applying.

**Against the pre-written definition:** four clean passes and a fifth passing
after its correction, no lane change, no harness failure, and no lost work. The
human checks found no functional defect the reviewer missed; the only miss was
the false comment in run 5. **The trial meets the clean-trial rule**, with that
comment as the one caveat.

### Applied

`91ef80938`. The five patches were generated from one base and overlapped in
imports and schema additions, so they were merged by hand; one merge dropped an
object's closing line, which `tsc` caught before commit. Core API 535/535, core
inbox/issues/agents 556/556, `tsc` clean. Not deployed.

### Remaining known limits

- The contract check is still a name-level heuristic for zod schemas.
- Patches from one trial overlap when applied together; applying them is a
  separate manual step.
- Running tests still executes agent-written code with the operator's rights.

## Issue-started lane and Codex reviewer — 2026-09-16T22:45Z-23:25Z

Goal: start code work from a Multica issue instead of this chat, use Codex as
the reviewer, and then run a first CRM job.

### Codex reviewer — working (lane v9, `1e3e0b5ff`)

- Review sessions run `codex exec` with **model `gpt-5.6-luna`, effort `high`**.
  Both are named explicitly and recorded in every receipt.
- Each review gets a read-only sandbox, no approvals, and a fresh task-scoped
  `CODEX_HOME` containing an opaque copy of the Codex sign-in and a minimal
  config. The home is deleted after the session.
- Why the isolated home: probing showed the operator's own Codex setup connects
  a Vercel MCP server, and a fresh default home blocked every read command, so
  Codex answered from web searches and got a fact wrong. The minimal config was
  verified to read the real file.
- Any web search, MCP call or file change fails the review. The Claude reviewer
  runs only if Codex cannot review at all, and the outcome records why.
- 54 provider-free tests.

**Proof on a real task** (`codex-review-...-20260916T230645Z-34364113`): the task
whose defect the Claude reviewer missed in trial 2, where task messages for chat
tasks omit `issue_id`.

- The executor, now told to read the server struct, made `issue_id` optional
  itself.
- Codex ran 25 read-only commands in `daemon.go`, `protocol/messages.go` and
  `taskMessageToPayload`, and resolved the one contract warning correctly.
- Its first review returned two real findings: a `tsc` failure in the new test
  and a missing `// @vitest-environment node` directive the repository rules
  require. The one correction fixed both, and Codex passed the second review.
- No fallback to Claude and no forbidden tool.
- The run took 13.3 minutes (reviews 2.9 and 3.5 min) with a lane memory peak
  of 818 MB.
- Human check: correct. Applied as the API validation commit after `1e3e0b5ff`:
  core API 547/547, chat 90/90, `tsc` clean.

### Issue-started lane — built, end-to-end proof blocked

Built and committed:

- `server/cmd/implementation-lane-bridge`: the local daemon runs it in place of
  the Claude CLI. It finds its issue in the daemon prompt and its role from the
  issue's workflow plan.
  - **Implementer turn:** reads one fenced lane-task JSON spec from the issue
    and checks it against an eligibility policy (repository allowlist, required
    fields). It runs the lane, posts the outcome as a comment and emits
    `implementation_ready` with `repository_worktree_inventory.v1` evidence
    built from git. A failed lane run fails the task so it shows as failed.
  - **Reviewer turn:** re-verifies the patch and outcome digests and every lane
    gate from disk, then emits `review_passed` or `review_failed`.
  - A second implementer turn for the same issue is refused, because the lane
    already used its one correction.
- `scripts/implementation-lane/issue-local/local_instance.py`: a disposable local
  Multica (its own pgvector container and locally built server). It never
  touches the live instance.

Proven so far: the local server came up healthy and the owner signed in and
created a workspace.

**Blocked:** starting the local daemon with the bridge was refused by this
session's safety check ("Create Unsafe Agents"). A daemon that launches agents
on its own from issue activity is exactly what that check stops, and it was not
worked around. So no issue-driven run has happened yet, and the five surfaces
were not checked for a lane run. The local server and database were stopped.

### First CRM job — skipped

It was conditional on the issue-started lane working end to end, which is
blocked. No CRM repository was read or changed.

## While the daemon start is parked — 2026-09-16T23:30Z-00:15Z

### One-step issue proof (`bcd93afed`)

`python scripts/implementation-lane/issue-local/local_instance.py prove`, run
from `C:/Dev/multica`, does everything in one command:

1. Builds any missing local binaries.
2. Starts the disposable database and server.
3. Bootstraps the owner, daemon, roles, squad and workflow template.
4. Creates one issue carrying a lane-task spec: validate the `listAgents`
   response, a real unvalidated endpoint.
5. Launches it with an owner mention of the implementer and waits for both
   tasks.
6. Reads Issue, Current Picture, Now (the product's `operationalLaneForIssue`
   rule, ported), Inbox and Board.
7. Writes `proof-result.json` and `proof-surfaces.json` under
   `.qualification-evidence/issue-lane-local/`.
8. Always shuts everything down.

Smoke-tested without the daemon: up, owner login, issue, surface read, down.
Two defects were found and fixed along the way: sessions saved against an
earlier database are now cleared, and each surface read records its own error.
The Current Picture read returns 404 for an issue without a workflow; that is
expected, and the proof issue has one.

### Always-failing server test — diagnosed, not changed

`TestWorkflowControlCommentBeforeMissingDispatchDoesNotRaceLeaderFallback`
expects that when an accepted `implementation_ready` event exists but its
reviewer task was never created, completing the implementer task wakes the
squad leader with one `workflow_missing_event` recovery task.

- When the test was written (`ecd79eb56`, 2026-09-09), the query
  `HasAdvancingIssueWorkflowEventForTask` counted an event as advancing only if
  its dispatched task existed.
- The accepted work preserved in `ffec7d703` (2026-09-16) changed that query:
  any accepted event with a `next` state now counts as advancing, "even if
  successor enqueue fails after commit; replaying the same event reconciles
  that dispatch idempotently". So `recoverWorkflowTaskWithoutEvent` returns
  early and the test sees 0.

**Cause:** the test encodes the superseded design. It was not edited, because
the two designs trade a spurious recovery race against a possible stuck
workflow when a dispatch fails and nothing replays the event. That is a
workflow recovery decision the accepted #40 release depends on. Options:
- update the test to the accepted design and add a reconciler test for a failed
  dispatch; or
- restore a bounded recovery for accepted events whose dispatch never appeared.

### First CRM job — nothing fit the limits

Worked in a throwaway clone of `C:/Dev/CRM` at its last-fetched `origin/main`
`1291141e` (no remote configured, removed afterwards). `C:/Dev/CRM` was only
read; its checkout stayed at 14 status entries on `ba1447a`.

Candidates checked, with results:

- `test_content_request_copy_never_promises_delivery` fails on main. The copy
  was reworded on purpose in `fb89f3d2` and still never promises a text; only
  the test's exact-sentence assertion is stale. A test-only change does not fit
  the lane (the harness requires a product change the test catches).
- `services/account_detail_view.py` returns a `TODO(api)` landing-page note in
  an API payload no page or other app renders; not a user-facing defect.
- The failures recorded in the hub (authority role, DMCA reader parity,
  featured-monitor 503, stale Active Accounts assertion) now pass or no longer
  exist: `test_of_featured_monitor.py` + `test_ui_route_registry.py` 53/53.
- The recorded archived-client unreachability needs a route behaviour change,
  which is outside the limits.
- A full-suite run to find other failures was stopped: on this Windows host it
  reached 8% in about 15 minutes.

Recommendation: pick the first CRM job from a current owner-visible defect, or
allow test-only corrections, rather than mining stale CI history.

### Workflow missing-dispatch test resolved, with one finding — 2026-09-17T00:40Z

Decision from the planning session: align the test with the accepted replay
rule and add a test for a hand-off that never happens. Do not restore the leader
fallback.

- `TestWorkflowControlCommentBeforeMissingDispatchDoesNotRaceLeaderFallback` now
  proves the accepted rule. With an accepted `implementation_ready` whose
  reviewer task was never enqueued, resending the same event from the still
  running task replays it and queues exactly one reviewer task. Completing the
  task afterwards wakes no leader.
- **FINDING for the owner, not fixed:**
  `TestWorkflowControlMissingDispatchAfterCompletionIsNotRecovered` pins it. If
  the successor enqueue fails and the task completes before anyone resends the
  event, nothing recovers the hand-off:
  - completion treats the accepted event as advancing, so no leader recovery;
  - a resend returns 409, because only the currently running task may post
    workflow events;
  - the only other code that creates the next task is the owner-decision
    answer, which does not apply.

  The issue is left with an accepted `implementation_ready`, no reviewer task,
  and no automatic way forward. The window is narrow (an enqueue error after
  commit, then task completion without a resend), but when it happens the
  workflow stalls silently.
- Verified on a disposable migrated Postgres: handler workflow and cancel-task
  tests 61/61, including both tests. Commit on `codex/workflow-completion-20260915`.

### Stalled hand-off fixed: recovered, or raised to the owner — 2026-09-17T00:10Z (system clock)

Decision from the planning session: make the stall visible, and recover it only
if that can be proven idempotent. Both parts shipped in `539187f5e`.

- **Recovery.** When a task completes, any accepted event from that task that
  names a successor agent but has no task carrying it as trigger evidence is
  dispatched once, through the normal workflow enqueue. New query:
  `ListUndispatchedIssueWorkflowEventsForTask`.
- **Why it cannot duplicate.** The lookup excludes any event that already has a
  task, in any status, carrying it as trigger evidence. In the normal #40 path
  the reviewer task exists before the task completes (the agent's event call
  returns before its process ends), so completion finds nothing to do. After a
  recovery the lookup is empty, so a repeat pass does nothing. If the successor
  already has pending work on the issue, the duplicate-pending guard counts as
  covered.
- **Visibility.** If the successor still cannot be created (for example, the
  reviewer was archived), the workflow takes the existing technical stop: the
  issue is blocked, the Current Picture shows a technical stop with a manual
  `inspect_incident` next action, and the owner gets one `workflow_incident` in
  the Inbox. `stopLegacyWorkflowAfterFailure` now shares its body with
  `stopLegacyWorkflowAfterMissingDispatch`.
- **Tests.**
  - `TestWorkflowControlMissingDispatchAfterCompletionIsNotRecovered` was
    replaced by `TestWorkflowControlMissingDispatchIsRecoveredAtCompletion`:
    exactly 1 reviewer task and no leader wake; nothing is left to dispatch,
    the issue is not blocked, and a late resend is still refused.
  - New `TestWorkflowControlUnrecoverableMissingDispatchRaisesIncident`:
    blocked issue, technical stop, one Inbox incident, no reviewer task.
  - `TestWorkflowControlSuccessfulDispatchDoesNotWakeRecoveryLeader` now also
    checks the normal path: exactly 1 reviewer task and 0 incidents.
  - Fails-before: with the recovery call removed, both new tests fail.
  - On a disposable migrated Postgres, removed afterwards: handler workflow and
    cancel-task tests 62/62, and service workflow, recovery and completion
    tests 22/22 passed. `go vet` is clean.
  - Unrelated and not touched: `TestLegacyRedirectsFollowTheDaemonsBrief` fails
    in this Windows checkout ("redirect stub has no frontmatter").
- **Residual risk, recorded.** Two completions of the same task running at the
  same moment could both see the event as undispatched. Enqueue has no unique
  key on the trigger evidence, and the duplicate-pending guard is the only
  backstop. A task completes once, so this is theoretical.
- **Push blocked** by the local permission classifier. The commit is on
  `codex/workflow-completion-20260915` locally; push with
  `git -C C:/Dev/multica push origin codex/workflow-completion-20260915`.

### Owner's issue proof collided with a lane run; lane v10 and v11 — 2026-09-17T00:50Z

- **The owner ran `local_instance.py prove`** at 00:23Z.
  - The server, bootstrap and daemon all worked, and the issue launched.
  - Both tasks failed within 30 seconds. The bridge's lane run was refused:
    "directory is not a registered worktree". The worktree belonged to the CRM
    Pulse lane run that was active at the same time.
  - The proof then crashed reading the surfaces: owner-summary `signals` are
    `{code, references}` objects, not strings.
  - Fixed in `1ced56c17`. `lane_for` matches signal codes, a surface error is
    recorded instead of aborting, and `prove` refuses to start while a lane
    worktree exists. **The proof needs to be run again.**
- **v10** (`0928f86cf`): the fails-before check recognises `test_*.py`,
  `*_test.py` and `*.test.mjs`, so Python repositories such as the CRM can pass
  it. Python caches no longer block worktree deletion. 54/54 lane tests.
- **v11** (`c80660ee8`): one lane run on the host at a time. The lock is taken
  before the orphan sweep, which could otherwise remove a concurrent
  same-repository run's clean worktree. A stale lock whose process has exited
  is taken over. 56/56 lane tests.
- **First CRM job** (owner's Pulse report): audit and fix recorded in
  `CRM-PULSE-AUDIT-20260917.md`.
  - Lane run `crm-pulse-batch-integrity-20260917T002114Z-86c04dc3` on v10
    succeeded.
  - The Codex reviewer failed the first attempt on a real defect
    (NaN/Infinity gave a 500). One correction fixed it, and the second review
    passed.
  - Fails-before is shown.
