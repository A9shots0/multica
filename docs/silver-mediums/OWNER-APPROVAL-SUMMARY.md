# Owner approval summary

This is the short approval view for the complete operating requirements. It is
not a UI mockup and does not activate code, agents, SMS, or production changes.

```text
Drew tells Mika the outcome once
              |
      controller runs the work
       /             |             \
routine progress  needs Drew     verified outcome
 Board + Issue    Inbox + SMS       SMS once
                      |
              secure phone screen
                      |
            exact paused work resumes
```

- **Board** answers: what is happening across the company?
- **Issue** answers: what is the complete current truth for this outcome?
- **Inbox** contains only unresolved decisions, incidents/consequences, and
  independently verified completions—not agent chatter or raw run failures.
- **Chat** is for new requests, changes, corrections, questions, and plain
  explanations. It is not the workflow engine or a status-polling loop.
- **SMS** interrupts for consequential events and sends exactly one notice for
  every independently verified completed business outcome.

Drew authorizes a bounded outcome once. Ordinary routing, correction, review,
release, verification, and safe recovery continue automatically inside that
envelope. A material change to behavior, scope, target, audience, risk,
permissions, external communication, destructive effect, or spending asks
Drew again before the consequential action.

There are no system quiet hours; Drew uses device Do Not Disturb. Drew is the
only recipient and approver. SMS links to the authenticated current decision;
`ACK` only acknowledges and free text remains discussion until a typed action
is selected or confirmed.

“Complete” means the promised business result is independently verified. It
does not mean that an agent, review, build, deployment, or intermediate recovery
step merely finished.

Approval of this summary authorizes implementation planning and local pilot
tests only. It does not authorize outbound SMS, hosted migration, production
deployment, Phone Farm technical access from Windows, or permanent agent/model
choices.
