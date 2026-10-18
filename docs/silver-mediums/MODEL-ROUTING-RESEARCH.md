# Model routing and token efficiency research

Status: research distilled; first mixed-model pilot verified; orchestration efficiency incomplete
Last updated: 2026-09-09 America/New_York

## Why this exists

Silver Mediums' hosted workflow consumed billions of tokens while still making
Drew chase updates and repeat decisions. The local pilot reproduced the central
mechanical problem on a tiny test: `SILV-3` reached the right result, but used
five complete model runs and reloaded very large context at every handoff.

The target is **minimum cost per successfully completed task while preserving
reliability**, not the cheapest model on every run and not the most capable
model everywhere.

## Main conclusions from the supplied research

1. A model and an agent are different things. An agent is a job description,
   tools, instructions, state, and permissions. The model is replaceable
   reasoning capacity used for a particular phase.
2. Model choice should follow the subtask, not remain permanently attached to
   a job title when the platform can safely support per-run selection.
3. Cheap models should reduce and structure a problem before escalation.
   Expensive models should receive a compact evidence packet, not reread the
   entire history from the beginning.
4. Deterministic operations should not require an LLM: status transitions,
   retry limits, duplicate prevention, test execution, health checks, and
   notification rules should be code or tools wherever possible.
5. The coordinator should not wake for every predictable handoff. Mika needs a
   model for ambiguous intake, decisions, and exceptions—not for mechanically
   moving a passed item to the next known stage.
6. More agents are not automatically better. Every extra handoff can duplicate
   context and introduce another failure point. The existing hosted agent and
   squad design must be evaluated by outcome and cost, not preserved by default.
7. Do not place this research report in every agent's prompt. Agents should
   receive only the short policy and evidence needed for their current role.

## Proposed model ladder

This is a hypothesis to test in the isolated pilot, not a final static mapping.

| Tier | Default work | Escalate when |
| --- | --- | --- |
| Luna | Intake classification, retrieval, summaries, routine coordination, log collection, simple bounded edits, test execution, status/result writing | Evidence conflicts, repeated bounded failure, or reasoning exceeds a clear routine procedure |
| Terra | Normal implementation, moderate debugging, synthesis, and ordinary independent review | Root cause stays ambiguous, cross-repository causality is difficult, or errors persist after a bounded attempt |
| Sol or a proven peer | Difficult debugging, architecture, risky cross-system reasoning, and ambiguous production failures | Multiple capable attempts fail or the remaining decision is unusually consequential and difficult |
| Astra | Exceptional escalation only: repeated lower-tier failure, hardest long-horizon diagnosis, or high-impact architectural judgment | No automatic upward tier; stop with an evidence-backed blocker if it cannot settle the problem |

Task length alone is not an escalation reason. Risk, ambiguity, evidence
conflict, and demonstrated failure are.

## Required escalation packet

An escalation must preserve and pass only the useful state:

- business outcome and current scope;
- verified facts and remaining uncertainty;
- relevant files, services, and evidence locations;
- commands and tests already run with their results;
- failed approaches and why they failed;
- exact decision or diagnosis the stronger model must settle.

The stronger model must continue from that packet. It must not restart discovery
or reread full conversation history without a specific evidence gap.

## Token controls to build

- One bounded intake run per owner request. Repeated owner messages should amend
  the same issue state rather than rebuild the full task context.
- Prefer deterministic transitions between known stages. Wake Mika only for an
  ambiguity, exception, failed gate, or owner-facing decision/result.
- Cap retries by stage and record the blocker after the cap. Never use endless
  heartbeat or status-check runs as a substitute for continuation logic.
- Retrieve the smallest relevant evidence slice. Do not inject every skill,
  report, issue comment, or repository document into every run.
- Run deterministic tests and health checks before asking a reviewer model to
  interpret only the failures or the final evidence.
- Keep issue ownership with the squad while assigning bounded internal work;
  do not create extra issues or conversations merely to change models.
- Use a concise final owner update generated from structured workflow state,
  not another full-history reasoning pass.

## Telemetry required for every run

Record:

- issue and workflow stage;
- agent role and task type;
- model and reasoning effort;
- fresh input, cached input, and output tokens;
- tool calls, wall time, attempts, and retries;
- escalation path and reason;
- whether a person had to intervene;
- gate result and final task success;
- estimated cost when reliable provider pricing is available.

Routing decisions should later be based on observed success rate and total cost
for Silver Mediums task types—not generic benchmark rankings alone.

## Local baseline to beat

`SILV-3` was a harmless arithmetic workflow test. It correctly completed:

`Squad owned -> Working -> Worker -> In Review -> Reviewer -> Done`

It nevertheless required three Mika runs, one worker run, and one reviewer run.
Aggregate usage was 65,611 fresh input tokens, 5,544 output tokens, and 864,000
cached input tokens. It reached Done in 2 minutes 59 seconds.

The next version must preserve the same visible behavior with fewer model
wake-ups and substantially less context. Merely changing those five runs to a
cheaper model does not pass.

## First mixed-model result

`SILV-5` repeated the same workflow with Mika and Pilot Worker on Luna/low and
Pilot Reviewer on Terra/medium. A self-hosted instruction conflict also had to
be fixed: stock Multica reserved `Done` for a human even after independent
technical review, while the pilot squad required Mika to finish routine work
after that review passed.

The corrected workflow reached Done automatically in 2 minutes 3 seconds. It
still used five complete model runs. Aggregate usage was 79,277 fresh input,
4,131 output, and 362,496 cached input tokens. Compared with `SILV-3`, total
reported tokens fell about 52%, cached reads fell about 58%, and output fell
about 25%, but fresh input rose about 21%.

This is one controlled sample, not a stable savings claim. The backend restart,
new sessions, and provider cache state may have affected cached reads. Repeat
tests are required. The result proves the cheaper mix can preserve this simple
workflow; it does not solve the structural waste of three Mika wake-ups.

## Assessment of later diagram feedback

This section records useful outside feedback on the proposed organization. It
is research input, not an approved rule or proof that the suggested org chart
fits Silver Mediums.

### Compatible with current evidence

- Design around static model-per-agent assignments first. The inspected stock
  execution path does not offer a task-stage model override, and the local fork
  does not need dynamic routing before its simpler architecture is proven.
- Keep Mika's normal job bounded to owner communication, evidence-aware routing,
  and exception handling. `SILV-5` proves Luna can coordinate one simple fixed
  workflow, but broader intake and synthesis still need tests.
- Split deterministic release machinery from intelligent incident diagnosis.
  Authorization checks, candidate/revision matching, deploy tracking, health
  checks, retries, and state transitions should normally be controller logic.
  Conflicting runtime evidence and ambiguous production failures may justify a
  stronger investigation agent.
- Give an independent reviewer a compact review packet: acceptance criteria,
  candidate identity, changed scope, required checks, and evidence locations.
  Do not automatically reload the entire issue and conversation history.
- Separate three layers: short role-specific instructions, selectively loaded
  shared knowledge/skills, and code-enforced permissions and state rules.
- Audit permanent responsibilities, permissions, required context, and normal
  reasoning difficulty before deciding which existing agents survive or which
  model each receives.

### Useful hypotheses that still require proof

- Mika=Luna, normal reviewer=Terra, difficult engineering=Sol, and rare
  adversarial audit=Astra are candidate defaults, not organization-wide
  assignments. Test each on representative Silver Mediums work and measure
  successful completion, corrections, latency, and total tokens.
- Release Controller as no-LLM infrastructure and Runtime Reliability as a
  separate reasoning role is promising, but current responsibilities and
  permissions must be inventoried before splitting or removing an agent.
- Shortening duplicated Mika and team-policy instructions should reduce fresh
  context, but the exact overlap and resulting behavior must be measured.

### Not adopted from the feedback

- Do not wake Mika for every "meaningful specialist result" by default. Current
  runtime evidence shows those automatic worker and reviewer wake-ups are the
  reason a trivial success path launches Mika three times. Deterministic gates
  should advance normal results; Mika should wake for ambiguity, failure,
  owner-facing decisions, or the one final owner update.
- Do not treat the sample agent chart as the final Silver Mediums team. It was
  produced without a complete audit of the existing agents, projects,
  permissions, host boundaries, or task mix.
- Do not collapse deployment and production verification into a vague board
  label merely to simplify the diagram. Owner-facing wording can be simple,
  but the stored state must still distinguish reviewing, deploying, verifying,
  rollback, Done, and Live truthfully.
- Do not add per-run dynamic model routing merely because the fork permits it.
  Reconsider it only after static roles, compact context, deterministic gates,
  and event-driven continuation are measured.

## Important unknowns

- The inspected stock execution path selects the model from the assigned agent,
  then falls back to the daemon default. No task-stage override was found. A
  backend change or a smaller role pool is required for true per-stage routing.
- Multica's generated instructions contribute to context, but their measured
  size does not explain the six-figure cached reads. Codex runtime/tool context
  and resumed-session growth are the leading causes; provider cache accounting
  has not been fully isolated.
- Luna, Terra, Sol, Astra, and non-OpenAI peer quality must be measured on real
  Silver Mediums task classes. Supplied benchmark and price claims are research
  leads, not verified pilot evidence.
- The correct number of permanent agents and squads has not been decided. Audit
  current responsibilities and duplicated context before consolidation.

## Decision rule

Adopt a routing change only if a repeatable test shows that it lowers total
tokens or cost without weakening completion, independent review, safety gates,
or owner clarity.
