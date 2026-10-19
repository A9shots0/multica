# Agent Continuity Contract

Status: candidate implemented; merge, rollout, and shadow validation pending  
Owner: Drew  
Recorded: 2026-10-18

## Outcome

Agents recover and continue by default. They involve the owner only when the
owner's judgment or authority is genuinely required. A preferred tool failing,
missing or stale evidence, an occupied lane, or a dirty shared checkout must
not stop unrelated safe work. UI work is the exception to evidence fallback:
it must regain browser access and complete browser-based verification rather
than substituting source, API, logs, tests, screenshots, or agent claims.

This contract is provider-neutral. Multica enforces the state decision for the
agents it runs; generated Codex, Claude Code, and Copilot instructions adapt the
same contract for sessions launched outside Multica.

## Decision lineage and coverage

| Source | Current state | Decision or fact | Accounted for here |
| --- | --- | --- | --- |
| Owner discussion, 2026-10-18 | accepted | Recoverable inconvenience must not become owner-required work. | The decision table keeps the goal active whenever a safe in-scope action remains. |
| Owner discussion, 2026-10-18 | accepted | Browser access is mandatory for UI work. Losing one or every current surface is an agent-owned recovery problem, never permission to finish through non-browser evidence. | Browser failure has its own non-terminal classification and a tested recovery-only path. |
| Read-only audit of `Audit AI token usage`, 2026-10-18 | current implementation evidence | Multica's continuous-work program already owns persistent Mika identity, goals, capsules, attempts, checkpoints, context manifests, receipts, cold worker rotation, budgets, loop detection, route preflight, and event-driven continuation. | This contract does not recreate those mechanisms. It supplies obstacle classification and stop/owner rules for that controller and standalone provider sessions. |
| Global agent guidance | current on inspected Mac and Windows installations | Direct owner authority is durable; routine in-scope work continues across phase boundaries. | Owner action is limited to explicit consequential boundaries. |
| `server/pkg/taskfailure` | current source | Multica already has a provider-neutral technical failure taxonomy and resume safeguards. | The continuity contract complements it; it does not replace provider/runtime failure classification. |
| `server/internal/daemon/execenv/runtime_config_sections.go` | current source | Every issue agent is told to mark an issue blocked when it "cannot proceed," without a recovery or whole-goal test. | Runtime guidance will use the decision table below. |
| Brain company context, read 2026-10-18 | incomplete/reference-only | Existing agent-team plan says role architecture and tests exist, but the retrieved plan is proposed and unverified. | Continuity is treated as the execution foundation for later role/model evaluation, not as proof that the older plan shipped. |
| PhoneFarm repository guidance | current source | Real release locks, owner UI decisions, task isolation, and production safety gates remain mandatory. | Provider-neutral continuity cannot override a concrete project safety gate. |

Nearby sources excluded from authority:

- Brain `working/unverified-imports` results are historical evidence, not live
  execution rules.
- The dirty, divergent Alienware knowledge checkout is not a safe deployment
  source until reconciled independently.
- The current non-main Multica checkout is preserved; implementation uses a
  fresh worktree from protected `origin/main`.

External loading behavior used for the adapters:

- OpenAI documents that Codex loads `AGENTS.md` from the user configuration and
  repository hierarchy, so the standalone Codex adapter is a bounded
  `AGENTS.md` block rather than another prompt pasted into each task:
  <https://developers.openai.com/cookbook/examples/gpt-5/codex_prompting_guide>.
- Anthropic documents user- and project-level `CLAUDE.md` loading and recommends
  keeping durable guidance compact, so Claude receives the same bounded block:
  <https://code.claude.com/docs/en/features-overview>.
- GitHub documents repository-wide Copilot instructions at
  `.github/copilot-instructions.md`; standalone Copilot rollout uses that
  supported file while Multica-managed Copilot receives the generated runtime
  brief directly:
  <https://docs.github.com/en/copilot/customizing-copilot/about-customizing-github-copilot-chat-responses>.

## States

- **active**: normal work can continue.
- **degraded**: a preferred tool or evidence path failed, but recovery,
  fallback, or other safe work exists.
- **waiting_lane**: one bounded lane is waiting; the parent goal stays active.
- **needs_owner**: the whole remaining goal requires a consequential decision
  or authority only the owner can supply.
- **external_blocked**: the whole remaining goal has no safe action, an
  external dependency is evidenced, recovery has been exhausted, and the
  exact resume condition is known.
- **complete** and **cancelled**: explicit terminal states.

## Candidate implementation

- `server/pkg/taskcontinuity` owns the typed decision table, external-block
  proof requirements, canonical runtime instruction, contract hash, and
  provider adapter rendering.
- The Multica runtime brief injects that same canonical instruction for Codex,
  Claude, and Copilot across issue, chat, quick-create, and run-only autopilot
  work.
- Issue status guidance permits `blocked` only for a whole-issue external block
  and explicitly keeps waiting lanes from blocking their parent.
- Standalone installations are identifiable by the SHA-256 contract hash. A
  matching file in source is not proof that a target host loaded it; installed
  files and a fresh agent session must still be checked during rollout.

## Boundary with Multica continuous work

The continuous-work program audited in `Audit AI token usage` owns **how work
keeps running**: Mika's persistent owner-facing identity, disposable workers,
durable goals and capsules, attempts and receipts, checkpoint/cold-resume
rotation, context and token budgets, loop detection, cheap route preflight,
and event-driven wakeups. Brain holds reviewed reusable knowledge;
Intelligence holds operational measurement; Multica holds live execution
state.

This contract owns **how an obstacle is classified**: recoverable degradation,
a waiting lane, a genuine owner decision, or a proven whole-goal external
block. It also supplies the provider-neutral instruction and standalone
Codex/Claude/Copilot adapters.

Integration should therefore pass obstacle observations from the continuous
controller through this decision table and record the resulting state in its
existing attempt/checkpoint receipts. It must not introduce a second goal
ledger, checkpoint schema, supervisor hierarchy, budget system, polling loop,
or worker-rotation mechanism.

Render a standalone block from the canonical source without copying prose:

```sh
cd server
go run ./cmd/agent-continuity --provider codex
go run ./cmd/agent-continuity --provider claude
go run ./cmd/agent-continuity --provider copilot
go run ./cmd/agent-continuity --hash
go run ./cmd/agent-continuity --provider codex --install ~/.codex/AGENTS.md
go run ./cmd/agent-continuity --provider claude --install ~/.claude/CLAUDE.md
go run ./cmd/agent-continuity --provider copilot --install /repo/.github/copilot-instructions.md
go run ./cmd/agent-continuity --check ~/.codex/AGENTS.md
```

## Decision rules

1. If any safe in-scope action remains, the whole goal stays active.
2. A tool failure, stale or missing evidence, dirty shared workspace, occupied
   resource, transient provider failure, or unavailable verification method is
   never owner-required by itself.
3. A blocked lane does not block its parent goal while another lane can move.
4. `needs_owner` is reserved for an ungranted product/UI decision,
   security/trust/access change, destructive or production-data action, budget,
   outbound communication, scope expansion, or interruption of other work.
5. `external_blocked` requires all of: whole-goal scope, zero safe actions,
   three unchanged observations, dated evidence, an accountable external
   system or role, and an exact resume condition.
6. A record that names a safe action cannot classify the whole goal as
   externally blocked.
7. Provider safety controls and project-specific production locks remain real
   boundaries. The contract classifies them accurately; it does not bypass
   them.

## Browser recovery

For UI work, browser interaction and browser-based verification are mandatory.
When a browser surface is unavailable, the agent owns recovery and tries in
order:

1. refresh the browser and tab inventory;
2. reacquire the intended tab;
3. open a fresh controlled tab;
4. use another supported browser or browser-automation surface;
5. repair or restart the browser session, local tooling, connection, or
   configuration when that is safe and in scope;
6. repeat the real UI workflow in a working browser.

Non-browser evidence may help diagnose the failure, but it cannot replace the
browser path, close the UI lane, or support a UI-complete claim. Browser
unavailability remains a non-terminal recovery state and does not become an
owner decision merely because every currently discovered surface failed.

## Host and product boundaries

- A32 hosts the Multica control plane and Brain integration.
- The PhoneFarm Mac Studio may run its existing Multica worker but does not
  gain another Multica control-plane stack from this work.
- Alienware receives the Windows adapter and browser recovery behavior.
- PhoneFarm keeps its repository-specific release controller, locks, approval
  rules, and rollback custody.
- Brain records evidence and installed contract versions. It does not grant or
  revoke execution authority.

## Validation cases

The contract must be replayed against at least:

- browser surface unavailable;
- stale or unavailable Brain evidence;
- dirty shared checkout with isolated-worktree alternative;
- one occupied work lane with other queue items available;
- transient provider or network failure;
- context compaction or resumable session failure;
- genuine product decision;
- destructive or security-sensitive action;
- production lock;
- provider safety lock.

The first six continue or recover without owner intervention; browser recovery
must end in a real working browser before UI completion. The final four may
wait or request owner action only when their exact boundary applies.

## Disproof and acceptance

This direction fails if agents still ask the owner to resolve a recoverable
tool/evidence/workspace problem, or if the stricter classification allows an
agent to bypass a real safety gate.

Acceptance requires provider-neutral decision tests, runtime-instruction tests,
provider adapter tests, matching installed contract hashes on target hosts, and
a shadow run showing no false owner-required stops for the recoverable cases.
