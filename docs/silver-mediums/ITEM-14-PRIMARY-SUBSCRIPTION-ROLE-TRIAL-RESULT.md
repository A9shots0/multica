# Item 14 primary-subscription role trial result

- Status: complete, independently evidence-verified, selection held
- Completed: 2026-09-14
- Execution definition SHA-256:
  `bd412ada00e9dbccd26c42423fed193957afcf516e4e6bf0e347ba1338b9233a`
- Terminal ledger SHA-256:
  `5098d32f8781628547dde360f2c8f7b48bfdd733247d16c8b40990d0c3a53713`
- Evidence-manifest SHA-256:
  `7190de705aa51b213df2d92986fcfb725e9a00fdf0915fc1802f6568e3a12167`
- Independent review SHA-256:
  `f217c433a147b2aaa745d4c87b04182da4095cd1d5c7742e32f2b7ac43b46bec`
- Canonical result SHA-256:
  `03208af564632ff8a030029c963085f3ffce3c3b1e442714512cacadfabbc41a`

## Result

The owner-authorized trial consumed exactly 12 fresh subscription CLI sessions
and made exactly 12 provider calls. It produced eight terminal structural
observations: five provider-free hidden-verifier passes, two invalid outcomes,
and one controller-action failure. Failed cells remain failed; none was
replayed, redirected, corrected by another model turn, or removed from the
comparison.

| Lane | Sessions | Valid provider envelopes | Input | Output | Time | Cost evidence |
|---|---:|---:|---:|---:|---:|---:|
| Claude Code | 6 | 5 | 43,004 | 12,801 | 219.484 s | $0.372820 API-equivalent estimate |
| Codex | 6 | 5 | 45,755 | 3,702 | 95.121 s | subscription receipt; no provider cash receipt |

Every Claude receipt reported requested `claude-sonnet-4-6` only. This does not
remove Claude Code's accepted transport limitation: background Haiku may occur
and would be disclosed. Every Codex command pinned requested `gpt-5.6-luna` at
Medium, but the CLI did not emit trustworthy actual model identity. The trial
therefore proves the request configuration, not an independently attested
served-model identity.

## Observation outcomes

| Lane | Case | Inline exact skill | Separate specialist then executor |
|---|---|---|---|
| Claude | R01 authority/contract | PASS | PASS |
| Claude | R02 product/UI | PASS | INVALID: executor omitted one mandatory check |
| Codex | R01 authority/contract | INVALID: terminal transport/schema failure | PASS |
| Codex | R02 product/UI | PASS | FAILED: executor returned prose instead of literal file contents |

Inline shared-executor plus exact-skill passed three of four cells; the
separate-specialist route passed two of four. This is a single non-repeated
sample with one known early harness failure, so the difference is descriptive,
not a configuration or permanent-seat selection.

Both separate Product/UI specialist sessions themselves completed. Both full
observations then failed at the downstream executor contract for different
reasons. That is evidence against promoting a separate Product/UI seat now; it
also identifies an exact provider-free contract requirement: advisory
specialist evidence cannot replace the complete controller packet, and a
controller-mediated edit must contain literal file bytes rather than advice,
a diff, or a placeholder.

## Safety and cleanup

All provider work directories and all fixture/initial-copy pairs were removed
with verified receipts; disclosed fixture paths also re-stat absent. The two
successful R02 cells retain hash-bound 390/820/1440-pixel Browser and DOM
evidence, and their one-time capture tokens are absent. The final counters are
zero retries, fallbacks, actual native tools, repository accesses, production
accesses, business actions, Phone Farm accesses, or permanent selections.

The one bounded Codex correction preserved failed session 7 and applied only
to ordinals 8-12. Its record contains zero calls/retries/replays and no
credential content. The independent provider-free reviewer passed 13/13
integrity checks and rejected a copied evidence root after one raw response was
altered.

## Decision boundary

No complete configuration or permanent seat is qualified. The lower-complexity
shared executor plus selected exact skill remains a candidate for further
provider-free contract validation. No further provider call is authorized by
this result.
