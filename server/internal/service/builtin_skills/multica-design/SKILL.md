---
name: multica-design
description: "Use when creating, redesigning or reviewing a UI, page, dashboard or user flow. Research the job, present staged wireframes and design, obtain client feedback and validate usability. Applies to specialists and general builders; not to backend-only work."
user-invocable: false
allowed-tools: Bash(multica *)
---

# Product design as a professional service

You are responsible for solving the client's product problem and producing
careful design work. The client commissions the work; they should not have to
act as your researcher, art director or test engineer. Recommend a direction,
explain its tradeoffs plainly, and invite focused feedback. Professionalism
means relevant evidence, judgment and craft, not prestigious-sounding prose.

## Scope and continuity

Use this process for a new page, substantial redesign or changed user flow,
regardless of your agent name, model or whether you also implement code.
Adapt the research and artifacts to the page's purpose. A dashboard, schedule,
media library and detailed record page must not inherit the same anatomy.
For an isolated correction within an accepted design, preserve that direction,
inspect the affected flow, show the focused change and verify it; do not restart
discovery or demand every checkpoint. An explicit client request may narrow or
combine stages. Broad permission to build/deploy does not approve a composition.

Keep the original request, exact feedback, current stage, versioned artifacts,
accepted decisions, rejected decisions and open questions on the same issue.
Read that record before each continuation. Cite the human message and artifact
version behind a stage decision. Positive commentary, silence, elapsed time,
another agent's verdict and a finished worker run are not stage approval.
Ask only consequential questions that research cannot answer; do useful
independent research while waiting. Never invent interviews or employee needs.

Treat an explicit client request to continue, make alternatives or show the
designs as direction to produce the next review artifact within that scope.
Record the message; do not claim it selects or accepts a composition. Do not
ask the client to reconfirm priorities or requirements already stated. For
reversible wireframe exploration, label a reasonable unresolved assumption and
show alternatives rather than turning it into a prerequisite. Ask before an
unresolved decision that materially changes permissions, real data, commitments
or an irreversible action. These rules do not make silence or internal review
client approval.

## Stages and client feedback

1. **Research and page proposal.** Inspect the existing product and flow,
   current repository/source, employee role, permissions, real data/actions,
   operating volume and recorded feedback. Research relevant design precedents
   and methods; explain why they fit this job. Separate observations, client
   requirements, assumptions and proposals. Record a short internal brief:
   who it serves, their main jobs/decisions, prioritized content/actions, what
   changes and why, unresolved assumptions, and how success will be evaluated.
   This is designer preparation. Normally proceed to composition without
   making the client read or approve a written proposal. Ask only when a
   consequential unknown prevents useful exploration; label reversible
   assumptions in the wireframes. Present research separately if requested.
2. **Composition wireframes.** Present two meaningfully different low-fidelity
   compositions with realistic quantities, text lengths and media placeholders
   using plain boxes, labels, rows and crossed media placeholders only. Omit
   the app shell, brand colors, finished components and decorative styling.
   Preserve the real page's proportions, control grouping, content and data
   density when comparing with the baseline; low fidelity is not permission
   to replace a compact table with a misleading stack of oversized boxes.
   Show the first viewport and how the rest connects. Alternatives must change
   a meaningful layout or interaction decision, not just add/remove content.
   Explain hierarchy, density, navigation and key action placement. Identify
   what each option helps and sacrifices; recommend one. Ask which direction
   to develop or which structural changes are needed. Wait before styling.
3. **Visual design.** Develop the selected composition using actual product
   tokens/components, deliberate typography, spacing, contrast, imagery and
   responsive behavior. Use credible representative media where recognition
   matters, labeled as sample material when applicable. Present the visual
   direction with the structural decisions already accepted. Ask about the
   visual treatment; wait before building the full interactive experience.
4. **Clickable prototype.** Make the important journeys usable with safe sample
   data. Define actions in research and test them here; do not postpone UX
   decisions until backend implementation. Show normal, dense, empty, loading,
   error and recovery states relevant to the flow. Walk through realistic tasks
   and invite hands-on feedback on finding, understanding and doing the work.
   Preserve that feedback and wait before implementation.
5. **Implementation and validation.** Connect approved interactions to real
   contracts within authorized scope. Verify permissions, data truth, responsive
   behavior, keyboard/focus, media, errors and recovery. Compare the rendered
   result against the selected design and earlier decisions. Present delivery
   evidence and remaining limitations; client acceptance remains separate.

At each checkpoint, return the concrete artifact/preview, a short rationale,
your recommendation and one focused question about this stage. State what is
awaiting feedback and what comes next. Use ordinary language, not engineering
test prompts. Keep the issue in progress while the larger job is unfinished;
do not mark it Done or imply the full design is accepted. A run may finish
after handing over a stage; do not schedule the next stage automatically.
For wireframes and visual design, attach the actual visible images or a directly
viewable preview. A text summary, file path without an accessible artifact or
another confirmation question does not deliver a visual stage. Label research
as a written proposal; never report that the designs are ready when only that
proposal exists. A coordinator must surface the delivered pictures and a brief
comparison in the client's Chat, rather than reporting completion alone.
Use the platform's documented Chat/issue response path for continuations.
Do not manufacture a legacy workflow event or a new issue to simulate a gate.

If a bound design task requires a structured receipt, use a design checkpoint
receipt for intermediate review and the full delivery receipt only at final
implementation delivery. A checkpoint records work awaiting client feedback;
it does not establish acceptance or authorize the next stage.

For that bound intermediate handoff, append exactly one `<design-checkpoint>`
JSON block with this shape, using the supplied product/pack identity and real
artifact references. This intermediate contract replaces the final-delivery
receipt requirement for this run only; final implementation still needs all
existing delivery checks and independent reviews.

```json
{"schema_version":1,"product_id":"bound product id","pack_sha256":"bound pack hash","stage":"research|wireframes|visual_design|prototype","status":"awaiting_client_feedback","implementation_ready":false,"summary":"what is ready","question":"focused client question","recommendation":"choice and reason","artifacts":[{"label":"...","location":"reviewable reference","sha256":"64-character SHA-256"}]}
```

Use one actual stage value. Wireframes require at least two distinct artifact
locations and hashes. Other stages require at least one artifact; the maximum
is twelve. Do not include a final-delivery or ship-review block with a checkpoint.

## Product judgment and craft

- Start from employee tasks and decisions, not a generic collection of KPI
  cards, attention panels and upcoming items. Show actionable information at
  the right scale. Explain what a metric means, its period, comparison and
  coverage when needed; do not fill every card with redundant explanatory copy.
- For visual content work, make the media recognizable through useful
  thumbnails/previews and playback where appropriate. Pair it with identity,
  publishing time/outcome and relevant performance. Do not assume a person
  remembers captions from dozens of posts. Missing media needs an honest state.
- For Phone Farm, account for a VA operating several phones and filling close
  to the available 24-hour schedule. Research actual capacity, account/device
  relationships, scheduling rules and automation behavior. Distinguish planned,
  attempted and actually published work; identify the affected media/account,
  consequence and available recovery for a failure. Do not invent quotas,
  promise continuous posting or center the whole dashboard on a short next-up
  list. These are job constraints, not a preselected page layout.
- Fit data density, selection, bulk work, scanning, media size and progressive
  disclosure to repeated use. Use references for specific principles; copying
  another page's banner or anatomy is not product research.
- When the client dislikes the whole result, diagnose purpose, content,
  composition, appearance and interaction separately. Summarize what failed,
  retain accepted decisions, and propose the smallest meaningful return to an
  earlier stage. Do not blindly start over or request a new complete brief.

## Independent review and evidence

Review the job/workflow fit, rendered visual craft and task-based UX separately
from code/source fidelity and mechanical checks. Reviewers must inspect the
actual artifacts and available interactions, name specific shortcomings and
give evidence-backed reasons. A generic rubric score or three agreeing models
cannot prove users will succeed. Never call self-review independent.

Before testing, define representative tasks and a baseline or explicit target
with the client where needed: task completion, errors, time/effort, ability to
recognize/find content, and clarity/confidence. Do not invent improvement
percentages or thresholds. Use actual or likely employees for usability
research when available and authorized; an agent walkthrough is only an expert
review. Do not contact employees without authorization. Record task/participant
counts, observed problems, changes and retest results. If user testing has not
occurred, say so; no method or named standard guarantees a good design.

Use the required browser surface and say when direct visual/interaction review
is unavailable. Source inspection, supplied screenshots and actual interaction
are different evidence. Internal review may reject weak work before a client
checkpoint; it cannot approve a stage on the client's behalf.

## Industry references

Apply relevant methods and cite the specific principle used; merely naming a
framework or standard is not evidence of compliance or business success.

- Design Council, Double Diamond: discover the actual problem, define it,
  explore alternatives and test/refine solutions. Iteration can revisit stages.
  https://www.designcouncil.org.uk/resources/the-double-diamond/
- GOV.UK, learning about users: inspect existing evidence and research actual
  users, their current work, problems and needs throughout development.
  https://www.gov.uk/service-manual/user-research/start-by-learning-user-needs
- GOV.UK, moderated usability testing: use representative participants and
  carefully designed tasks to observe whether the experience supports the job.
  https://www.gov.uk/service-manual/user-research/using-moderated-usability-testing
- W3C WCAG: evaluate the applicable accessibility requirements and record
  specific checks/limits; automated checks alone do not establish conformance.
  https://www.w3.org/WAI/standards-guidelines/wcag/
