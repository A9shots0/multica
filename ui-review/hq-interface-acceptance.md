# AI Workforce HQ interface acceptance

Candidate: real Multica Inbox preview at `http://localhost:33664/silver-mediums-pilot/inbox?mode=work`, backed by the live Silver Mediums workspace. Independent acceptance review passed on 2026-10-15.

| Requirement | User task | Expected result | Result | Evidence |
| --- | --- | --- | --- | --- |
| Needs-you queue remains focused | Open Inbox to see what requires a decision | The existing actionable queue remains the default and ordinary healthy work does not appear there | Passed | Independent reviewer saw `Needs you 0`; SILV-98 was absent |
| Company work is understandable | Switch to All work | Active, waiting and stopped work are shown in plain English without guessed percentages | Passed | Independent reviewer saw all five real open issues; missing evidence says `No recent update` |
| Urgency is visible | Scan mixed routine and urgent work | Urgent/stopped/owner-blocked work is visibly prior to routine work without oversized alert cards | Passed | High-priority labels remain compact; state order is deterministic and unit-tested |
| Outside work stays honest | Inspect a Codex/Claude/ChatGPT report | The provider and reported state are visible, but reported completion is not presented as accepted Done | Passed | SILV-98 shows `Working` and `Reported by Claude`; model test protects `Reported done` boundary |
| Next action is clear | Open a piece of work | The latest meaningful change and next action are visible; technical evidence is available on demand | Passed | SILV-98 detail shows four short sections and a full-history link |
| Chat is the direction surface | Discuss selected work | The owner can open Chat with the work target visible; no control over an outside provider conversation is implied | Passed without mutating production | Existing `Talk to Mika about this` workflow is reused; prompt generation is covered by tests |
| Dense data remains usable | Inspect realistic current data | Desktop and phone layouts stay compact, legible and free of horizontal overflow | Passed on current live data | Independently checked at 1440×900 and 390×844; prior self-check covered 1920×1080 and 1280×720 |
| State changes do not interrupt | Let data refresh | Counts and meaningful state changes update without stealing focus | Passed | 30-second query refresh; no focus-changing refresh behavior |

Independent evidence:

- `/private/tmp/multica-hq-recheck-needs-1440x900.png`
- `/private/tmp/multica-hq-recheck-work-1440x900.png`
- `/private/tmp/multica-hq-recheck-work-phone-list-390x844.png`
- `/private/tmp/multica-hq-recheck-work-phone-detail-390x844.png`

## Searchable outside-work history — pending candidate

Candidate: `codex/hq-history-inbox-20261016` in the real Multica Inbox. This section is the acceptance contract for the next candidate; no result is recorded until the working UI is reviewed.

| Requirement | User task | Expected result | Result | Evidence |
| --- | --- | --- | --- | --- |
| Existing Inbox job stays intact | Open Inbox without searching | `Needs you` remains the default; `All work` continues to show current company work exactly as before | Pending | Pending working preview |
| Past outside work is findable | In `All work`, search `LinkHub 403` | Results from the real imported LinkHub conversation appear, grouped by day and project | Pending | Pending working preview |
| History is clearly unverified | Open a conversation result | The UI says it is conversation memory, not verified company evidence or accepted completion | Pending | Pending working preview |
| Private machine details stay private | Inspect result rows and details | No local file path, opaque source reference, checkpoint, or raw session identifier is shown | Automated check passed; interactive review pending | `workforce-overview.test.tsx` |
| Search is economical | Type and edit a query, then submit it | No request is made while typing; one request runs on submit, and clearing restores current work | Automated check passed; network observation pending | `workforce-overview.test.tsx`; `queries.test.ts` |
| Empty and failed searches recover | Search for a missing phrase and simulate a request failure | Plain-language empty/error states appear, and the owner can retry or clear without losing the current-work view | Pending | Pending component tests |
| Compact layout remains usable | Search and open a result at narrow and phone widths | The list appears first, detail opens without horizontal overflow, and Back returns to results | Pending | Pending working preview |
| Real data, real shell | Review the candidate | The preview uses Multica's actual authenticated shell, actual Inbox layout, and actual Brain-backed data; no isolated mockup or test-data screenshot is used for approval | Preview server ready; interactive review pending because no in-app Browser surface is currently available | `http://localhost:33664/silver-mediums-pilot/inbox?mode=work`; production web build passed |

## HQ replacement for Issues — acceptance contract

Candidate: not implemented. This follows the owner's rejection of the
floating-card prototype. No visual result is claimed in this stopping slice.

| Requirement | User task | Expected result | Result | Evidence |
| --- | --- | --- | --- | --- |
| Company is understandable | Open HQ with many concurrent items | Outcomes, effort, urgency and stale areas are clear without reading every issue | Untested | `docs/silver-mediums/HQ-UI-DEFINITION.md` |
| Default organization is useful | Open HQ without arranging it | Deterministic regions and grouping are meaningful before any manual drag/pin action | Untested | Definition only |
| Work hierarchy is honest | Inspect Phone Farm modernization | One outcome reveals its work, workers, blockers and evidence without treating a chat as the task | Untested | Definition only |
| Cross-area work stays singular | Inspect work affecting CRM and Phone Farm | One work record connects to both areas; no duplicate status cards appear | Untested | Definition only |
| Operational signals fit the work | Inspect watcher or support impact | The incident connects to the affected system/outcome with privacy-safe evidence | Untested | Company operations API exists; UI absent |
| Relationships explain themselves | Select an object or connection | Only its relevant dependency/impact path appears and each connection has plain-language meaning | Untested | Definition only |
| Scale is controlled | Load dozens of work records | Company view summarizes within outcomes instead of rendering an unbounded field of cards | Untested | Definition only |
| Real detail remains authoritative | Open work then return | Existing issue detail/actions open and map position is preserved | Untested | Definition only |
| Dark Multica fit | Review desktop, narrow and phone | Existing shell, tokens, type, icon language and density are preserved; no light-mode or standalone mockup evidence | Untested | Definition only |
