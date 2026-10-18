# Skill OS acceptance record

Production source: protected `main` merge `17ebe8d89f2bcdc246f3669442f15e2d1bd9c3fb`,
source seal `17ebe8d89f2b-local-024f1f1e08a5`.

This record separates implementation evidence from protected integration and
real-runtime acceptance. A passing unit test does not make the candidate live.

| Requirement | Current evidence | State |
| --- | --- | --- |
| Audit existing skills and instructions | Accessible inventory found 232 `AGENTS.md` paths, 35 distinct contents and a small canonical set; decisions are recorded in `AGENTS-MD-AUDIT-20261014.md`. Multica's provider-native built-in skill delivery and Brain path were inspected in source. | Implemented for accessible environments; remote freshness must be rechecked before each canonical edit. |
| Compact task/skill contract | `SKILL-OS.md` defines task selection, method delivery, scoped Brain evidence, deterministic controls and evidence-based completion. | Implemented. |
| Outcome ownership | Built-in `multica-outcome-ownership`. | Implemented and template-tested. |
| Evidence-first debugging | Built-in `multica-evidence-first-debugging`. | Implemented and template-tested. |
| Long-work supervision | Built-in `multica-long-work-supervision`. | Implemented and template-tested. |
| Research and REA | Built-in `multica-research-rea`. | Implemented and template-tested. |
| Learn and harden | Built-in `multica-learn-and-harden`. | Implemented and template-tested. |
| Independent UI acceptance review | Built-in `multica-ui-acceptance-review`, adapted from the existing Codex method for cross-provider use. | Implemented and template-tested. |
| Automatic selection without slash commands | One shared routing contract filters built-in method bundles at the server claim boundary and lets current daemons render the selected names and reasons. Old daemons receive the same exact subset without a client update. | Focused service, daemon and old/new claim-path tests pass on current protected source. |
| Conservative usage | Plain explanations, trivial copy edits, quick-create, automatic evidence reports and controller-bound tasks avoid unrelated methods. Project descriptions and ambient chat-work snapshots do not influence routing. Skill bodies are materialized but only short descriptions are always listed. | Static and focused test evidence passes; production token/latency change is unmeasured. |
| Scoped Brain compatibility | Ordinary work can receive both selected methods and bounded untrusted Brain reference data; workflow-packet and guarded tasks exclude ambient methods and Brain tools. | Combined prompt regression passes; existing Brain transport tests remain the source for broker behavior. |
| Evidence-based completion and plain-English owner communication | Outcome, debugging and supervision skills distinguish code, tests, merge, deployment, workflow proof and acceptance; updates must be short, plain and emoji-free. | Instruction contract is live; worker adherence must be measured from ordinary work rather than inferred from deployment. |
| Rollback | Remove the automatic prompt block and stop shipping the six methods; no stored task or Brain data depends on them. | Exact prior images, CLI and host environment were retained. Activation was guarded and rollback-capable; rollback was not needed. |
| Representative historical failure replay | Tests cover modernization audit, long-running supervision, repeated regression, research chat, resumed work, AGENTS cleanup, substantial UI review, simple explanation and trivial copy edit. | Deterministic replay passes; full provider runs against recorded tasks remain outstanding. |
| Protected integration | PR #40 added the Skill OS; PR #44 reconciled it with the live reliability source; PR #46 repaired mixed-case GHCR publishing. All protected checks passed before merge. | Complete. |
| Production service rollout | Sealed backend and frontend archives were hash-verified, activated with zero active work and a fresh three-destination backup, and independently checked through container health, `/health`, `/healthz`, migrations and the private HTTPS route. PostgreSQL stayed on the same container. | Live at `17ebe8d89f2b-local-024f1f1e08a5` since 2026-10-15T07:39:04Z. |
| Worker rollout | The Mac Studio shared worker CLI was atomically replaced with a rollback copy retained; Codex, Claude and OpenCode runtimes reconnected and reported the exact reviewed source commit. | Mac Studio live at `v0.4.44-silver-skill-os.20261015.1`. Windows workers remain online on the earlier CLI and still receive server-delivered built-in skills, but do not yet have the new daemon-side deterministic routing hint. |
| Real Multica runtime acceptance | A real task should receive exactly its selected methods, use scoped Brain where bound, produce evidence and avoid unnecessary methods/usage. | Live service and worker transport are proven. A provider-consuming task was deliberately not launched during this rollout because the owner required conservative subscription usage; behavioral acceptance remains observational. |
| Canonical `AGENTS.md` cleanup | Safe rollout and canonical targets are recorded. Multica's canonical `AGENTS.md` is already a concise pointer, so changing it would add duplication rather than remove it. Other repositories keep their hard safety and release rules until ordinary tasks prove equivalent protection. | Audit decision complete; no mass edit or speculative deletion. |
| Durable reporting | Task Control task `skill-os` and Brain progress observations record the candidate and lack of acceptance. | Recorded; update commit references after each amendment. |

## Production rollout evidence

1. Protected merges: PRs #40, #44 and #46.
2. Candidate build workflow: run `38695421060`; backend and frontend archive
   hashes are bound in the release receipt.
3. Multi-architecture release workflow: run `38697216522`, successful.
4. Production backup: `20261015T071447Z-a5b7ab43`, with local, off-box and
   private R2 results all `ok` before activation.
5. Live images: backend
   `sha256:a8512ded2b39143f2f8ae8f29b0023a6eb13e01db932fee12f71b14584235890`
   and frontend
   `sha256:51621e798251a59c5a647aa00b661d50f25825c78c376f1e86f5a2641038b2a1`.
6. Post-cutover observations: both application containers and PostgreSQL were
   healthy; database and migrations checks returned `ok`; runnable task count
   was zero; the private `/health` route reported the sealed source.

## Behavioral observation still required

Do not spend provider quota merely to manufacture an acceptance percentage.
Measure the next ordinary explanation, implementation, diagnosis, long-work,
research, repeated-failure and substantial-UI tasks as they naturally occur.
Record selected methods, completion evidence, retries, owner intervention,
context size and usage. Expand canonical instruction cleanup only when those
observations prove the same protections at lower cost.
