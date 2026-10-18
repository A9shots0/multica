# Shared Windows executor R02 action overlay v3

Status: **provider-free correction candidate; not runtime qualified**.

This overlay applies only when the controller supplies the frozen synthetic R02
fixture. It narrows the immutable shared Windows executor v1 packet; it never
widens its authority.

Return one self-contained controller action array. Every `edit.contents` value
must be the complete literal final file, not prose, a diff, a placeholder, a
template, or an instruction for the controller. For this fixture, edit exactly
`report/report.js` and `report/styles.css` once each. Do not edit
`EVIDENCE.json`; the controller alone materializes evidence and Browser proof
hashes.

The exact order is:

1. replace `report/report.js` with complete valid JavaScript that derives the
   total from `published_posts` and renders every published post;
2. replace `report/styles.css` with complete responsive CSS that has no fixed
   desktop-width overflow and adapts at narrow, medium, and wide widths;
3. run `node tests/report.test.mjs`;
4. run `local-static-preview report`;
5. request `browser` exactly once;
6. finish with one truthful local-only summary.

Do not run the known-failing data check before the repair. R02 treats an action
failure as terminal, so the two required allowlisted checks both run after the
literal edits.

Do not claim that Browser evidence already exists when returning the plan. Do
not request another model turn, provider tool, shell, repository, credential,
network, deployment, outbound action, production system, or Phone Farm.
