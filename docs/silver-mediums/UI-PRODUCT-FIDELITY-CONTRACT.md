# Silver Mediums UI product-fidelity contract

This contract applies to every Silver Mediums task that designs, mocks up,
implements, or reviews a visible product surface. It applies regardless of
which coding model or agent receives the task.

## The product is the starting point

Before proposing UI, inspect the exact project resource attached to the task
and record the source revision. Inspect the current route, its real application
shell, the closest accepted page, the project design guide, shared tokens,
shared components, and responsive rules. When access is available, inspect the
real signed-in product route as a visual baseline too.

The existing application chrome is locked unless the owner's request explicitly
includes it. Reuse the real sidebar, top bar, theme, typography, spacing scale,
controls, icons, and responsive behavior. A page mockup changes the requested
page inside that shell; it does not invent a second product around it. A
standalone approximation of global navigation is not acceptable evidence of
how the page will look in the live product.

## Every choice needs a reason

Every added, removed, moved, or visually emphasized element must map to all of:

1. the owner task or user job it serves;
2. a current workflow or data fact found in the project source;
3. the existing component or pattern it reuses, or a concrete reason that the
   existing pattern cannot serve the need; and
4. the state and viewport behavior that make its placement necessary.

Remove unexplained UI. Do not add generic dashboard cards, copy, filters,
charts, status language, navigation, or workflows merely because they look
plausible.

## Visual evidence is a delivery gate

A visible mockup or UI candidate is not owner-ready from source inspection,
tests, or a browser-openable file alone. Before delivery:

- render the candidate inside the real shell;
- inspect minimum, typical, and dense/maximum credible data;
- inspect wide desktop at 1920x1080 and 1440x900, plus the relevant medium and
  phone widths;
- compare baseline and candidate captures for shell fidelity, hierarchy,
  spacing, alignment, density, color meaning, overflow, and interaction states;
- complete a separate visual review pass that can reject the candidate; and
- correct every must-fix finding before the owner sees it.

If the assigned runtime cannot perform the required browser comparison, it must
not claim a visual pass or present the artifact as ready. Keep the result in the
work issue and route the same issue to a browser-capable visual reviewer. The
absence of a visual lane is a blocker, not permission to substitute source-only
confidence.

## Required handoff

The owner-facing result stays concise, while the issue evidence records:

- exact repository revision and inspected route/shell/token/component paths;
- baseline route and observation time;
- candidate preview and viewport captures;
- the element-to-reason mapping;
- the independent visual verdict and corrected must-fix items; and
- explicit boundaries between mockup, implemented candidate, deployed code,
  and observed live behavior.

## Machine enforcement

Product & UI work is identified from the configured agent or skill identity,
not guessed from words in the owner's prompt. For an ordinary repository-backed
issue, Multica fetches the bounded Intelligence context before launch and
requires exactly one owner-approved design pack matching a task repository.
The exact product, repository, source revision, canonical pack hash, and full
pack payload are persisted with the task and included in the run prompt.

If the matching pack is missing, ambiguous, malformed, or not approved, the
Product & UI task fails closed before an agent is launched. Other agents remain
ordinary development tasks even when their prompt happens to mention UI.

Successful completion requires exactly one `design-task-delivery.v1` receipt.
The server validates the receipt before it writes a successful task result,
issue comment, or owner Chat update. The receipt must bind to the persisted
pack; use pack-owned shell, component, and page-pattern references; prove all
required data states and viewports; pass every quality check; and cover the
three required review lenses. Each review entry must reference a real completed
child reviewer task and its agent. The server verifies that the task belongs to
the same issue, is a child of this design run, completed with a result, and was
performed by a different agent. A missing or invalid receipt returns a permanent
rejection so the daemon records a visible failed run instead of showing
unreviewed work as ready.

This gate proves that a separate reviewer run exists; it does not prove that the
reviewer's judgment was correct. Real captures and an evidence-backed reviewer
result remain required, and the first live CRM task is still a business
acceptance gate.
