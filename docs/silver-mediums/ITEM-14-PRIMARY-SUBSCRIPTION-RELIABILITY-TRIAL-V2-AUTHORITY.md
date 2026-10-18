# Item 14 primary-subscription reliability trial v2 authority

- Status: owner-authorized, provider execution not yet started
- Authority date: 2026-09-14
- Purpose: measure the repeat reliability of the best surviving inline shared
  executor plus exact-skill configuration
- Production activation authority: none

## Bounded interpretation

The owner authorized a fresh, capped reliability trial through the existing
Claude Code and Codex subscriptions. This record makes the cap numeric before
any new provider call:

- exactly 12 fresh one-shot CLI sessions at most;
- exactly three new repetitions of each Claude/Codex by R01/R02 inline
  exact-skill cell;
- one provider call per session, zero retries, fallbacks, redirects,
  relaunches, or automatic correction replays;
- Claude Code requests `claude-sonnet-4-6` at Medium and evaluates the disclosed
  Sonnet-primary plus possible background-Haiku Claude Code configuration;
- Codex requests `gpt-5.6-luna` at Medium through a fresh task-local profile;
  missing served-model identity remains missing and may not be upgraded to an
  exact-provider claim;
- no direct metered API, OpenRouter, alternate credential source, or additional
  provider lane;
- 75,000 maximum reported tokens and 20,000 maximum generated tokens per
  session;
- fresh disposable synthetic fixture per observation and fresh disposable
  provider work directory per session, with verified removal;
- provider-native tools forbidden; only the deterministic controller may
  perform the exact fixture-local actions returned in the response;
- R02 may use only one controller-owned ChatGPT in-app Browser capture against
  its one-time `127.0.0.1` preview after a valid action envelope;
- no real repository, production, business-system, outbound, deployment, or
  Phone Farm access.

The four matching inline observations from the completed v1 trial remain
immutable prior evidence. V2 uses a new definition, observation IDs, fixture
roots, session roots, and ledger; it cannot retry, repair, or rewrite v1.

## Selection boundary

After all v2 observations are terminal, independently verified, and reviewed,
the controller may choose the safest reversible **local-only provisional**
configuration if the reliability evidence supports one. This does not select a
permanent model, permanent seat, or production configuration and grants no
production activation.
