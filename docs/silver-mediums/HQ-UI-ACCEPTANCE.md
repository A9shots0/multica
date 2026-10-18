# HQ UI acceptance record

Updated 2026-10-18 America/New_York.

## Candidate under review

- Base revision: `8d9408997acce214d907d4be0205d910e1adeeb3`
- A32 worktree: `/Users/a32/Worktrees/multica-hq-ui-continuation-20261016`
- Preview used by browser review: `http://localhost:33666` from A32 itself
- Role/data: the owner's existing private Silver Mediums Pilot account and real
  workspace records; dark mode only
- Candidate delta: the modified HQ model, model tests and HQ component visible
  in the A32 worktree, plus the decision-ledger update. `output/` is review
  evidence only and is not product source.

## Observable scenarios

| Requirement/source | User task | Expected result | Result | Evidence |
| --- | --- | --- | --- | --- |
| HQ UI definition, review 1 | Identify the most urgent outcome and what happens next in under ten seconds | The urgent outcome is visually prominent and its next step is available with one selection | Pass | Ranked Shared work appears first at desktop and phone widths. |
| HQ UI definition, review 2 | Distinguish work that is active now from work that is stale or unclear | `Working now` is earned by current work evidence; stale records say when their last update occurred | Pass | The two-day-old outside report says `Waiting / unclear`; the summary says `0 moving`. |
| HQ UI definition, review 3 | Inspect Phone Farm | The outcome, contributing work, worker, blocker/next step and latest evidence are distinguishable without reading an issue table | Pass | The selected view shows the failed queued run, assigned specialist and recovery next step. |
| HQ UI definition, review 4 | Follow cross-area CRM/Phone Farm work | One real record appears once and exposes both typed connections | Untested | SILV-79 mentions CRM in prose, but the live data has no typed CRM relationship; the UI correctly does not invent one. |
| HQ UI definition, review 5 | Inspect an operational incident | Watcher/support impact is visible without raw private support text | Untested | The live operating picture reports zero incidents. |
| HQ UI definition, review 6 | Open a real work record and return | The existing issue detail opens and returning preserves the selected HQ context | Pass | SILV-79 opened and browser Back restored the Phone Farm selection. |
| HQ UI definition, review 7 | Repeat core inspection at desktop and phone widths | No horizontal overflow, unreadable miniature cards or hidden essential meaning | Pass | Independent dark-mode review at 1440, 1024 and 390 pixels found no root/body horizontal overflow. |
| Owner correction after spatial prototype | Understand the company without manual organization | Regions use the available page, grouping is meaningful, and the default view does not collapse into loose cards or a narrow endless list | Pass for the current real-data scale | Ranked regions and responsive auto-fit cards use the available space; the global view stays capped at eight outcomes. |
| Multica visual constraint | Use the product in dark mode | HQ remains recognizably inside the existing Multica shell and design system | Pass | Reviewed inside the signed-in dark Multica shell, not a mockup. |

## Builder observations

- The authenticated pre-correction capture showed one narrow company column and
  incorrectly called stale Phone Farm work `Moving`.
- The corrected candidate separates project outcomes from unprojected shared
  work, uses responsive regions, labels state in plain words and shows task and
  assignment evidence when an outcome is selected.
- Focused model tests, Views typecheck, focused lint and the low-memory web
  production build pass on A32.

## Independent result

Independent current-task reviewer `/root/hq_a32_real_acceptance_v2` accepted the
corrected UI scenarios on 2026-10-18. Evidence is stored on A32 at
`output/playwright/hq-acceptance-v3-20261018` and is not product source.

The A32 development preview's WebSocket upgrade still returns HTTP 403, so
live push refresh is not proven by this preview. Ordinary authenticated reads
and the tested HQ interactions work; production realtime remains a separate
verification before release. Cross-area and incident behavior remain untested
because the real workspace did not provide typed examples. No fake records were
introduced to manufacture those passes.
