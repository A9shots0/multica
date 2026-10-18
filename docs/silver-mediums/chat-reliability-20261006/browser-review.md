# In-app Browser review record

## Current checkpoint - 2026-10-06 23:43 UTC

Drew still rejects both current designs. Neither is owner-ready. Technical checks, actual-shell reuse, independent package reopening and reviewer observations did not establish useful product outcomes. No new product brief or approval is required from Drew. The current priority is backend reliability and understanding the product/review failure, not another cosmetic iteration.

Backend v38 `5351e442d5ee14d67e45f01475b2603fafc37965` is pushed and active, image `sha256:8e891f95b2b2c66d5dfc227e64b058721e363a611d29592bfd334963adfd4ded`. It refuses ordinary tasks attempting a legacy squad handoff, and refuses missing explicit successor model/reasoning before advancing state or writing an event. Actual dispatch failures now log the event/source/target and reason. Existing completion recovery is preserved, not newly invented. The failed historical CRM event remains visible. Normal application live verification and health revision readback passed after verified backup `20261006T233800Z-8ac0c41b`; frontend/database container IDs and worker binaries preserved. No schema change in v38. Reboot untested.

Focused regression suite passed (35.826s), affected vet/build passed. The broad run was interrupted without verdict; the bounded combined run timed out at 180 seconds while moving through fixtures, with no assertion failure before timeout. Its completed cases and a separate green tail suite (25.340s) establish the individual workflow-control checks and two selected owner-route test groups, not an all-green combined suite. Exact logs/receipt are in the task artifact directory.

CRM final ZIP and Phone v3.2 final ZIP were independently downloaded, hash checked, extracted and reopened. The repaired Phone mobile menu round-trip was verified in both served candidate and freshly extracted archive. Candidate links/receipts remain in the structured record as rejected evidence. CRM ordinary independent review genuinely began but was cancelled by the newer rejection; no final visual verdict is claimed. Phone independent review was bounded to screenshots/source, with supervisor Browser interactions distinguished. The Mika follow-up's invented request for a new owner direction was corrected through Chat; a general summary-authority fix remains outstanding.


## Owner rejection supersedes technical review — 2026-10-06 22:28 UTC

Owner reviewed and rejected both CRM Clients v2r2 and Phone Farm Dashboard v2.1 for product concept, UI and UX. Correct source-shell reuse and technical delivery do not establish a business improvement. Technical packages delivered/reopened: 2/2; owner-accepted designs: 0/2; rejected concepts: 2/2. The earlier delivery checkpoint is historical, not current design readiness.

Exact feedback and original screenshots are preserved in `C:/Dev/_work/multica-design-owner-feedback-20261006/` and submitted through the existing SILV-93/94 Mika Chats. Both same-issue corrections require research of the actual existing pages, employee jobs, real available data/actions and a comparative revised concept before another implementation. CRM must prioritize clients, remove detailed-page banner anatomy, redundant scope, avatars and unactionable flagged-account emphasis. Phone Farm must communicate performance, published outcomes, tasks/issues and automation failures; a small upcoming schedule cannot dominate a future full schedule. No live CRM/Phone changes. Supervisor functional/asset checks remain technical evidence only.

Supervisor review on 2026-10-06. These are fictional-data previews produced by actual Multica workers after the recovered original inputs were sent through fresh Mika Chats. The supervisor did not author either product design. Review does not constitute the owner's visual acceptance.

## Evidence correction

The original Phone v1 `1920` and `1440` filenames were incorrect: they contained identical narrower images. They remain historical evidence, not responsive passes. New PNG captures use tab-scoped in-app CDP emulation, DOM `innerWidth`/`innerHeight` readback, and a full viewport clip. `verified-capture-inventory.json` records actual bitmap dimensions and hashes. The accidental `.jpg` capture is excluded. No standalone browser was used by this supervisor.

## Phone Farm v2.1

Actual React app shell, sidebar and existing theme render at 1920x1080 and the owner's 1279x777 target. Explicit grid positions resolve the observed schedule-time/badge overlap. Captures: `phone-v2.1-1920.png`, `phone-v2.1-1279.png`.

The complete weekly fixture has 12 expected posts, 11 sent, 11 platform-confirmed, one explained non-publication and six qualifying posts. The visible exact boundary examples are 9,999 -> No, 10,000 -> No, 10,001 -> Yes. Preparation uses those six confirmed qualifying posts.

The revised normal HTTP CSV link downloaded successfully through the in-app Browser to `C:/Users/a9sho/Downloads/SAMPLE-Trial-Reels-2026-09-28.csv`. Independent file inspection confirmed 12 post rows, 11 confirmed, six qualifying and the exact three boundary outcomes. A copy is retained under `C:/Dev/_artifacts/multica-chat-reliability-20261006/`. An earlier blob download-event check timed out; its cause is unknown and it is not represented as a pass.

Needs-checks mode shows nine confirmed, two unconfirmed, one not sent and four qualifying. Report preparation is disabled. Inspecting a gap exposes two sample matches and requires a reason; saving a chosen fictional match changes totals to ten confirmed and five qualifying, leaves one unconfirmed, records one history entry and keeps the report disabled. These are origin-local fixture changes only.

VA role removes manager/admin navigation and Kaya's weekly reporting module, showing assigned sample work. Activity load failure shows unavailable counts. The weekly module has its own explicit fixture mode; its Unavailable data state hides aggregate values and disables reporting/export instead of inventing zeroes. No real API, device action or social publication was exercised.

## CRM v2

Real Flask/base-template shell, navigation, fonts and local assets render at 1920x1080, 1440x900, 1024x900 and 390x844. Typical and 48-client dense states were inspected. At 1440 the dense page has no horizontal overflow. Empty-results state correctly retains the underlying 48-client aggregate/coverage and offers reset.

The drawer is fixed across the whole app rather than constrained to the roster. Escape closes it and returns focus to Nora's trigger. The first v2 drawer retained a purple primary button because it is outside `.clients-fresh`; this was sent through Chat for an overlay-scoped dark-button correction.

Nora's individual unavailable account value and partial-coverage aggregates are correctly separated. A later Minimum-state review found all account/flag coverage absent (0/1 clients reporting), but aggregate totals and attention still claimed zero/no alerts. This was sent as ordinary same-Chat feedback at 22:11 UTC. The cancelled predecessor and exactly one replacement remained on SILV-93. `crm-v2-1024-minimum.png` records the defect, not a passing state. Final corrected-state recheck remains required at this checkpoint.

## Boundaries

Worker HTTP/asset checks, supervisor rendered/interaction checks, portable delivery and owner acceptance are separate lanes. Both final ZIPs have now been downloaded through Chat, hash-checked, safely extracted into new folders and independently reopened in the in-app Browser. Phone's extracted copy also passed all 14 portability checks and 65 asset hashes; CRM's extracted copy passed minimum/typical/dense state assertions. Final CRM checks confirmed unavailable aggregate/attention wording, dark drawer action, selection/clear and Escape focus return. Captures `crm-delivered-1920.png`, `crm-v2r2-1024-minimum.png`, `crm-v2r2-drawer.png` and `phone-delivered-1920.png` are the final evidence. Earlier defect captures remain evidence of corrected findings.

Local preview services are best-effort processes with no guaranteed restart or hosting lifetime. Both real applications' live data and deployments remain unchanged. Sequential worker review lenses are not an independent design approval. Chat still emits verbose and sometimes stale predecessor summaries during replacement work; that remains a communication limitation. The CRM package's "owner verified" wording was explicitly clarified through Chat: these were supervisor checks sent through the owner channel, not Drew's review or acceptance.
