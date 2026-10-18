# Item 11 — Mobile parity

## Accepted boundary

Item 11 carries the accepted Items 1–10 owner experience onto a phone. It does
not redesign workflow truth, add SMS, change agents or models, or touch hosted
Multica or production systems.

## Phone interaction

1. Inbox is the primary control center for existing work.
2. Its four categories are usable filters, and each material record remains
   independently addressable by inbox-item id.
3. A tap opens a bounded, full-screen mobile control view with the current
   outcome, meaningful progress, evidence, comments, exact decisions,
   corrections, revisions, partial acceptance, scope change, cancellation,
   and acknowledgment. The full issue is secondary.
4. Issue opens with the server-owned Current Picture before its timeline.
5. Chat visibly remains for new or unclear work, brainstorming, cross-issue
   questions, and explanations—not status polling or existing-work control.

Mobile uses stacked screens and native confirmations where desktop uses split
panes. Counts, visibility, item identity, authority, and state meanings must be
identical.

## Progress

- [x] Compare accepted web behavior with the existing mobile implementation.
- [x] Align owner-relevant projection, independent item identity, counts, and filters.
- [x] Replace decorative rows with readable two-line owner summaries.
- [x] Build the bounded mobile control screen and typed actions.
- [x] Put Current Picture first on mobile Issue and clarify Chat's boundary.
- [x] Run focused tests, typecheck, lint, and browser-visible phone verification.
- [x] Complete one consolidated correction pass and final review.
- [x] Record the acceptance receipt in the run journal, acceptance matrix, and handoff.

## Evidence boundary

Browser fixtures prove presentation and local command wiring only. The Expo
production web export proves that the mobile bundle builds, not that a signed
native app was installed or authenticated on a phone. Private Tailscale HTTPS
was verified from the host; the isolated in-app Browser cannot resolve tailnet
DNS, so remote phone access still needs a real-device check. Nothing here proves
SMS delivery, production execution, or a real business outcome.

## Local acceptance receipt

- Compact browser proof covers the categorized Inbox, bounded item controls,
  legacy decision fail-closed behavior, Issue Current Picture first, and the
  visible Chat boundary.
- Mobile data behavior preserves independent inbox-item identity, filters raw
  noise, archives only the addressed item, forwards query cancellation, and
  uses revision/idempotency-bound command paths.
- `@multica/mobile` typecheck passed; lint passed with zero errors and six
  pre-existing warnings; Vitest passed 22 files and 130 tests; Expo web export
  completed successfully.
- Private tailnet HTTPS serves the local Multica web app at
  `https://a9drew.tailda9924.ts.net/` without exposing it publicly.
- Item 12 SMS remains disabled and unstarted.
- The single independent review returned P0=0/P1=3. One consolidated
  correction pass added strict receipt schemas, native confirmation before
  Approve/Reject, a true full-screen route, explicit stale-decision handling,
  corrected empty-state copy, and focused receipt-schema coverage. No reviewer
  loop was started.
