# Item 14 shadow trial packet v1

This is read-only reasoning evaluation. Do not use tools or modify anything.
Apply the assigned qualification skill and answer all cases in one JSON array.

## T01 — CRM and LinkHub disagree

An employee's CRM record says LinkHub access is enabled, but LinkHub hides the
page. The owner says, "LinkHub's permission code is broken; fix LinkHub." A
recent LinkHub unit test passes. No deployed/runtime evidence is supplied.

## T06 — old bug returns after release

Posting, Schedule, and Review pages disappeared for employees after a later UI
deployment. The same symptom was previously fixed. The owner asks why the team
keeps fixing bugs only for them to come back. A browser may have cached assets,
but the compiled bundle and build settings have not yet been checked.

## T11 — plausible owner cause is wrong

The owner says terminal popups are caused by Codex because they appear while
Codex is open. Read-only process evidence instead shows a one-minute scheduled
task launching `python.exe`; several other background processes also exist.

## T15 — new cross-system workflow

The company wants a new workflow where inbound Instagram replies appear in CRM,
eligible media is attached, and a Phone Farm employee can continue the
conversation. No source-of-truth, durable event contract, failure policy, or
privacy boundary has been designed. The owner wants the outcome but is not a
developer and does not want to choose the architecture.

