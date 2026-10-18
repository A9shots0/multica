# CRM Pulse accuracy audit and fix — 2026-09-17

Owner report: "check out the new pulse page I made for me (admin), I don't feel
like it's accurately pulling all data from everyone on the site."

## Scope and boundaries

- **Snapshot:** SMCrm `origin/main` `1291141` (2026-09-15, PR #375), read in a
  scratch clone at 2026-09-17T00:05Z. `C:/Dev/CRM` was not touched.
- **Files read:**
  - `services/crm_pulse.py`, `blueprints/crm_pulse.py`, `static/js/crm-telemetry.js`
  - `static/js/partial-navigation.js`, `templates/base.html`, `main.py`
  - the Pulse tests, and the history of #285, #287, #297, #298, #300, #301, #304, #325 and `b9b780d`
- **Evidence method:**
  - reading the code;
  - Flask test-client probes on the CRM's own SQLite test fixtures (scratch
    only, never committed);
  - the implementation-lane run below.
- **Not verified:** accuracy against production. There is no read-only
  production data path without new credentials, so no production counts were
  compared.

## What Pulse shows, and for whom

- **Who can see it:** `/owner/pulse` and `/api/owner-pulse/summary`, for one
  account only. That account must match `CRM_PULSE_OWNER_USER_ID`, have the
  admin role and use `service@silvermediums.com`.
- **Where the data comes from:** every signed-in CRM page loads
  `crm-telemetry.js`. It records page views, clicks on meaningful controls,
  same-origin fetches, errors, and 30-second active-time samples. Events are
  queued in the browser and posted in batches of up to 30 to
  `/api/owner-pulse/events`.
- **Who the People list includes:**
  - anyone with Pulse events in the window;
  - plus any user with `status = active` whose `last_activity` falls in the
    window. `last_activity` is refreshed at most once a minute on any
    authenticated request.
- **What only events feed:** visits, actions, requests, active time and
  failures come from Pulse events alone.

## Inclusions and exclusions

| # | Behaviour | Class | Source |
|---|---|---|---|
| 1 | One invalid event made the server reject the whole batch (HTTP 400). The collector ignores the failure, so every event in that batch was lost. | **Bug** | `record_events` since `e7ebc85` |
| 2 | Partial navigation, used by every in-app link, triggers #1 routinely. The click and page-fetch events still carry the old page key when the queue is sent, but the URL has already changed, so `pageKey does not match the current CRM page` rejects the batch. That batch holds the new page's visit and the nearby active-time sample. Probe: a 4-event navigation batch returned 400 and stored 0 events. Browser back/forward has the same timing. | **Bug** (main cause of undercounting) | collector + `partial-navigation.js` |
| 3 | Active time spent on a page before a partial navigation was credited to the next page. | **Bug** | collector `silver:partial-load` handler |
| 4 | A `HEAD`/`OPTIONS` request event, or a request longer than 5 minutes, triggers #1. Probe: 400, 0 stored. | **Bug** | collector sends what the server rejects |
| 5 | A request path that only matches a route with a trailing slash made the endpoint answer 308 instead of recording anything. Probe: `/live` answered 308. | **Bug** | `_resolve_endpoint` |
| 6 | Summaries read only the newest 50,000 events in the window. Active-time samples alone are about 120 per person per active hour, so a busy team fills that in days. Older visits and time then drop out of the 7/30/90-day figures. A notice is shown ("Showing the most recent 50,000 of N events"). | **Known future limit, kept by owner decision 2026-09-17**: Pulse has been live about 3 days, so it cuts nothing yet. Revisit soon: on a busy team the 30- and 90-day views start truncating within days. | `MAX_SUMMARY_EVENTS`, `8095f8f` (2026-09-12) |
| 7 | "Online now" counts only CRM presence. Phone Farm and other app presence are excluded. | Deliberate | #304 `f12c98e` |
| 8 | Nothing is recorded while an admin uses employee preview, and partial navigation is off there. | Deliberate | `b9b780d` (2026-09-15) |
| 9 | People list also includes users seen only through `last_activity`, with no telemetry (`activitySource: session`). | Deliberate | #300/#325 |
| 10 | While an admin impersonates a user (not preview), the page still ran the collector. **Correction to the first version of this report:** those events were never stored. `main.py` `enforce_impersonation_read_only` answers 403 to every write while impersonating, including the Pulse ingest. What still counts: impersonation refreshes the impersonated user's `users.last_activity` at start and once a minute, so Pulse's session-based people list can show them as connected. | Owner decision 2026-09-17: do not count it. Collector now off while impersonating (fix below). `last_activity` residual not changed: the session idle timeout reads it, so changing it is an auth change | `templates/base.html`, `main.py`, `blueprints/auth.py` |
| 11 | Users whose status is not exactly `active` (for example `pending`) are left out of the session-based part of the list, but counted as eligible in role totals. | Minor inconsistency, not fixed | `build_summary` |
| 12 | Role filter matches the stored role string. Roles are normalised to canonical values when written, so nothing is missed today. | No gap | `models.py` role validator |
| 13 | Daily buckets are in UTC. | Deliberate (UTC everywhere) | `build_summary` |

## Fix 1: batch integrity (bugs 1–5)

- **Branch:** `codex/crm-pulse-batch-integrity-20260917`, commit `514b6f1` on
  top of `1291141`, in the scratch clone.
- **Saved for the owner** in
  `C:/Dev/multica/.qualification-evidence/crm-pulse-batch-integrity-20260917/`:
  - `0001-fix-pulse-stop-losing-whole-telemetry-batches-at-pag.patch`
  - `0002-fix-pulse-switch-off-activity-collection-while-imper.patch` (Fix 2)
  - `crm-pulse-batch-integrity.bundle`
- **Not done:** no push to the CRM remote and no deploy.

Changes:
- Server stores every valid event in a batch and skips invalid ones. A batch
  with no valid event still answers 400. The response keeps `recorded` and
  `eventCount`.
- Every per-event validation rule is unchanged, including page key against
  referrer.
- Redirect-only paths and non-finite numbers (NaN, Infinity) are validation
  errors instead of a 308 or a 500.
- Collector records active time and flushes on `silver:before-page-replace`,
  while the URL and page key still belong to the outgoing page.
- Collector records only GET/POST/PUT/PATCH/DELETE requests, and caps request
  duration at 300000 ms.
- No route, column, model, migration, template, status string or contract
  change.

Checks:
- **Lane run** `crm-pulse-batch-integrity-20260917T002114Z-86c04dc3`, lane v10,
  00:21–00:45Z:
  - executor Claude `claude-sonnet-4-6`/medium, 546 s;
  - reviewer Codex `gpt-5.6-luna`/high, 304 s. It failed the first version
    for one real defect: NaN/Infinity could still give a 500 and lose the
    batch;
  - one correction (40 s), then the second review passed (218 s);
  - the new tests fail on the original code;
  - the existing Pulse suites pass.
- **Re-run on the committed branch:**
  - Python `test_crm_pulse_batch_integrity`, `test_crm_pulse`,
    `test_crm_pulse_telemetry_assets` and
    `test_partial_navigation_session_identity`: 29 passed;
  - Node collector, partial-navigation, Pulse frontend and new
    navigation-flush tests: 28 passed.

Remaining after the fix:
- Browser back/forward changes the URL before any script runs. Events queued
  from the previous page are then skipped individually rather than losing the
  whole batch.
- Deploying needs a fresh asset version. `asset_v` already changes per Railway
  deploy, so browsers pick up the new collector.

## Fix 2: no collection while impersonating (owner decision)

- **Commit:** `498a134`, on the same branch after `514b6f1`. The saved patches
  (`0001`, `0002`) and the bundle now hold both commits.
- **Change:** `templates/base.html` sets `data-crm-telemetry` to `off` whenever
  `impersonator_id` is present in the session, in addition to employee preview.
  No server change: ingest was already refused while impersonating.
- **Tests:** `tests/test_crm_pulse_impersonation.py`. An admin impersonating an
  employee stores no events for either user. The same employee's normal session
  stores them (201). The page renders collection `off` only while impersonating.
- **First lane run:** `crm-pulse-impersonation-exclusion-20260917T043718Z-4c6dc1e8`
  failed, and nothing was kept.
  - Its objective asked ingest to answer 202, which the global read-only guard
    makes impossible; the Codex reviewer found the guard.
  - The correction attempt tried to mutate `main.py`'s write-exemption set from
    inside the Pulse blueprint. The reviewer flagged it as out of scope, and it
    was not shipped.
- **Second lane run:** `crm-pulse-impersonation-exclusion-v2-20260917T045415Z-92ac9614`
  succeeded on v11.
  - The Codex reviewer failed the first attempt on two points: a truthiness
    check instead of `is none`, and a test employee whose stale activity timed
    the session out.
  - One correction, then pass, with no out-of-scope changes. Fails-before shown.
- **Re-run on the committed branch:**
  - Python impersonation, Pulse, batch integrity, telemetry assets, partial
    navigation, employee preview, impersonation view cycle and mobile shell
    suites: 93 passed;
  - Node collector, partial-navigation, Pulse frontend and navigation-flush:
    28 passed.

## Owner decisions (2026-09-17)

1. **Summary cap (#6):** keep it for now as a known future limit, not a bug.
   Revisit soon after go-live, because busy-team 30- and 90-day views will start
   truncating within days. Full-window database totals were not built.
2. **Impersonation (#10):** do not count it; implemented as Fix 2. Residual: the
   impersonated user's `last_activity` still refreshes and can list them as
   connected. Removing that needs an auth/session-timeout change outside Pulse.
