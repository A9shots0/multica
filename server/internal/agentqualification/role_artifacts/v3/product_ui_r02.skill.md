# Product/UI R02 exact-action skill v3

Status: **provider-free correction candidate; not runtime qualified**.

For the frozen R02 fixture only:

1. Preserve domain truth: the confirmed total is the sum of the four
   `published_posts`, not the duplicated metric rows.
2. Require all four published posts to render as cards.
3. Require responsive CSS, including a fluid outer width and an adaptive grid;
   a narrow viewport must not horizontally overflow.
4. Require complete literal contents for both `report/report.js` and
   `report/styles.css`. Reject prose, ellipses, TODOs, placeholders, token
   substitution, or instructions such as "controller will fill this".
5. Keep `EVIDENCE.json` controller-owned. The participant must not edit it or
   predict its hashes.
6. Require both allowlisted checks after the edits, one Browser action, and one
   final finish action in the frozen order. Do not request the known-failing
   data check before repair because R02 action failure is terminal.
7. Emit only the action payload. It has no tools and performs no action itself.

This packet does not qualify Product/UI, select a model, authorize a provider
call, or apply to a real repository or production route.
