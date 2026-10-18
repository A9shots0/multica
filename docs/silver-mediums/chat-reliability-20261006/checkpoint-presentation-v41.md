# Chat worker receipt presentation repaired (v41)

The CRM wireframe delivery exposed raw design-checkpoint JSON in an ordinary
worker update. Its 3895-character body fit the display budget, bypassing the
receipt filter used only for oversized final-output fallback. Historical
transcript and shared work context also returned the saved body unchanged.

One presentation filter now strips the four existing machine-receipt types
before display budgeting in new delivery, historical transcript, work context
and catch-up. Incomplete or machine-only bodies use the existing longer-update
notice. Human messages, raw comments/task results/saved messages, structured
review identities and additive payload fields remain intact. No schema change.

## Verification

- Reproduction test failed before fix for all four receipt types.
- Canonical service tests PASS (0.430s); go vet handler/service PASS.
- Nine isolated PostgreSQL handler/delivery/catch-up checks PASS, including
  normal delivery, historical paged readback, immutable evidence, review image
  identity, malformed payload and untouched human text. Earlier TCP fixture
  attempts skipped for authentication and are not counted as passes. Final run
  used the task-only container's local socket; container and tunnel stopped.
- Source07ebf09bf595bb74a95be55ea997d308f357fb4f pushed on owned branch.
- Linux binary SHA2568dc390579fd374dbf416e0fb5e088c3d44ea2bbdde91f7dc69c901d540d724fc.
- Backup20261007T075101Z-a1c2263a verified local/offbox/privateR2.
- Active backend image sha256:cca77bb0019bed85b0752683c529a59cf3f60925a1a16ad28a55ed989a6ee482.
- Normal preflight/live host verification PASS; exact health source07ebf09bf,
  start07:56:09Z. Frontend/database containers and worker binaries preserved.
- Root reloaded original CRM Chat in built-in background Browser: raw JSON gone,
  both image attachments and final Mika summary retained. Before/after DB hashes
  for messagee5757827-e2fd-a9bb-0c99-d6d724e86410 identical, including two artifacts.
- v40 rollback image/env preserved; LaunchAgents now reference v41. Reboot untested.

## Wireframe handoff

Phone task01a11545-3c80 completed07:42:40Z. Original Mika Chat retained first-view
A/B PNGs, full-composition PNGs and reopenable ZIP01a11550-0a92. Root downloaded
final A/B through IAB; final A SHA08fb553c matches earlier shown image. Final B
SHA804974405dfad3d85fbf95fdb56e004f779965f7bfa901187d891aa6f6c76e3e differs from
earlier shown copy258aa79c: final PNG adds the explicit Last check12:05 label
on the unavailable-metrics phone. Both inspected in IAB; composition unchanged.
Preserved final file: phone-wireframes-v1/B-final-attachment.png. CRM task01a11544-c3bd
completed07:35:45Z. All four layouts already shown inline to Drew. Neither
composition selected or accepted; styling/implementation must not be inferred.

## Remaining boundaries

Whole Multica operational trial remains incomplete: design usefulness/acceptance,
hard phase enforcement, generic summary authority, normal-Chat runtime negative
probe and reboot recovery are not established. No operational CRM/Phone data or
devices changed. Questions/remarks are not automatically new implementation orders.
Research remains preparation, not mandatory written homework. Monitor stays active
for meaningful new evidence and independent authorized engineering work.

Evidence: C:/Dev/_artifacts/multica-chat-reliability-20261006/ including
checkpoint-presentation-handler-tests.log, v41-checkpoint-presentation-activation.log,
checkpoint-presentation-v41/crm-before.jpg and crm-after.jpg, historical-message
before/after receipts and bugfix-20261007-checkpoint-presentation.md.
