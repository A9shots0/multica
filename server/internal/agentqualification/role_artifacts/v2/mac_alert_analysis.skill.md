# Mac alert analysis candidate skill v1

Status: **skill-only candidate; no seat, host access, or authority granted**.

Load this only when a deterministic Mac watcher produced a sanitized,
non-critical alert whose grouping or likely boundary cannot be explained from
its structured receipt. It is not a monitoring loop and may not suppress,
delay, rewrite, or acknowledge the source alert.

1. Confirm the packet contains no secrets and identifies its watcher version,
   host class, event window, affected service boundary, and evidence freshness.
2. Group only events that share a receipt-supported cause signature. Preserve
   every source event reference and conflicting signal.
3. Distinguish application, release/runtime, fleet/device, and unknown causes.
   Never infer Phone Farm technical facts from Windows evidence.
4. Return one bounded explanation, the evidence supporting it, the unknowns,
   and the exact dedicated Mac role or Windows release role that may investigate.
5. Stop. Do not access the Mac, poll logs, operate a provider, change alert
   severity, edit source, restart anything, or claim resolution.

Critical alerts bypass this skill and follow deterministic incident/SMS rules.
Phone Farm alerts remain inside its dedicated Mac roles. Retain this skill only
if later blind tests prove it reduces noise or diagnosis time without missed
signals, extra owner burden, or material token cost.
