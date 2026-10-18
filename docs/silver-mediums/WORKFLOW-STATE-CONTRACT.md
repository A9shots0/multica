# Workflow State and Event Contract

Status labels are projections of workflow truth. They do not create truth, and the current board layout is not the final product design.

## Canonical states

| State | Plain meaning | Required evidence |
|---|---|---|
| `received` | The issue exists but has not entered an executable lane. | Issue record. |
| `queued` | A runnable task has a real target and can start automatically. | Pending task, usable runtime, and no missing manual routing step. |
| `working` | An agent is actively advancing the issue. | Claimed/running task with a live lease or heartbeat. |
| `technical_review` | Implementation is complete and an independent reviewer is checking it. | Structured implementation-ready handoff plus a reviewer different from the implementer. |
| `correction` | Review failed and the implementation must be corrected. | Structured review-failed result with actionable findings. |
| `deploying` | An approved technical candidate is being installed. | Release task is active and identifies the candidate. |
| `production_verifying` | The candidate is deployed and post-release checks are running. | Deployment receipt plus active production verification. |
| `needs_owner` | Work cannot continue without one real business, authority, or safety decision. | Exact question, recommendation, consequence, and evidence showing automation cannot decide it. |
| `blocked` | Work cannot continue because of a concrete external dependency. | Named dependency, responsible party/system, and automatic resume signal or explicit next check. |
| `done` | The requested non-live work is independently verified complete. | Independent review pass and all required acceptance gates satisfied. |
| `live` | The requested live behavior is deployed and independently verified in production. | Deployment receipt and production/business verification pass. |
| `failed` | The current attempt ended unsuccessfully and cannot automatically continue. | Failure classification, preserved work, and recovery/escalation record. |
| `rolling_back` | A failed live candidate is being removed and the last verified version is being restored. | Active rollback task naming both candidate and recovery version. |
| `rolled_back` | A release failed verification and the prior verified version was restored. | Failed verification evidence, rollback receipt, and restored-health check. |

`queued` is forbidden when no runnable task exists, the runtime is unusable, or a human must manually restart the workflow. `needs_owner` is forbidden for technical uncertainty that a specialist can investigate.

## Structured events

The controller acts on structured events, never on optimistic wording in comments.

| Event | Producer | Controller result |
|---|---|---|
| `issue_received` | intake | Route to a squad and create a real runnable task. |
| `task_started` | runtime | Move to `working`. |
| `implementation_ready` | implementer | Select a different reviewer and move to `technical_review`. |
| `review_passed` | reviewer | Move to deployment, owner acceptance, `done`, or `live` according to declared gates. |
| `review_failed` | reviewer | Return one structured correction packet to the implementer; do not wake Mika merely to relay findings. After the declared correction limit, stop in `blocked` and wake the squad leader once. |
| `owner_decision_required` | leader/controller | Move to `needs_owner` and notify once with the complete decision packet. |
| `owner_decision_recorded` | owner | Resume the exact suspended step automatically. |
| `dependency_blocked` | any lane | Move to `blocked`; record a concrete resume condition. |
| `dependency_resolved` | watcher/member | Resume the exact suspended step automatically. |
| `deployment_started` | release lane | Move to `deploying`. |
| `deployment_succeeded` | release lane | Move to `production_verifying`; deployment alone is not `live`. |
| `production_verified` | independent verifier | Move to `live`. |
| `production_failed` | independent verifier | Move to `rolling_back`, start rollback/recovery, and create one urgent owner alert. |
| `rollback_succeeded` | release lane | Move to `rolled_back` only after restored-health evidence is recorded. |
| `task_interrupted` | runtime | Retry or recover from the saved checkpoint without duplicating the issue. |

## Deterministic routing rules

1. Mika handles intake, genuine ambiguity, and real owner decisions. She is not a message relay between known workflow stages.
2. A worker success routes directly to a different reviewer when the issue's execution plan names one.
3. A review failure routes directly back to the implementer with the findings.
4. A review pass advances only through gates declared before work began: deployment required, production verification required, or owner acceptance required.
5. Duplicate events are idempotent: the same event ID produces no second task, status transition, message, or notification.
6. A failure or ambiguous structured event wakes the squad leader; a normal structured transition does not.
7. Parallel issues never share transition state, retry counters, or cancellation scope.
8. Comments explain evidence to people. Structured events drive the workflow.
9. The local pilot allows one automatic correction cycle. A failed review must
   include 1-5 failed criteria, observed evidence, and one bounded required
   correction. A second failure stops the loop and escalates to the squad
   leader; it never starts a third worker/reviewer cycle automatically.
10. A release candidate is the complete reproducible artifact: source revision,
    bundled assets, build configuration, migrations, and runtime configuration
    contract. Matching source alone is not enough.
11. Production verification checks both the requested change and a permanent
    regression contract for previously fixed employee-critical behavior. A
    later release that revives a closed critical bug fails verification and
    enters rollback/correction; it cannot be called `live`.

## Board projection

The board may later combine or rename columns, but it must preserve these meanings:

```text
Received -> Queued -> Working -> Technical review -> Deploying -> Verifying -> Live
                         ^              |
                         |-- Correction-|

Any executable state -> Needs owner -> resume exact prior state
Any executable state -> Blocked -> resume exact prior state
Failed production verification -> Rollback -> restored health + correction
```

The owner-facing label for automated technical review is **In review**, never **Ready for review**. A separate owner decision must say **Needs you** and must identify the actual decision.
