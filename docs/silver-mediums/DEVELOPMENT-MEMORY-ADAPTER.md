# Silver Mediums durable development memory adapter

**State:** local end-to-end candidate in the 32GB-host worktree. It is not deployed, configured, migrated, or accepted in production.

## What this slice does

Before Multica dispatches an ordinary issue task to Codex or Claude, the Multica API:

1. resolves the issue, project/workspace, and repository set it already owns;
2. asks the private Intelligence API for the workstream identified by the stable Multica issue UUID;
3. creates that workstream on first use, without guessing an unknown default branch;
4. verifies every repository in the current task is bound to that workstream;
5. attaches the bounded Intelligence context packet to the claim; and
6. requeues the task instead of silently launching without memory when a configured Intelligence service fails.

The daemon receives only the context packet. The Intelligence reader and writer credentials remain in the Multica API container. The packet is appended to the run-scoped prompt after the stable prompt prefix, and it explicitly distinguishes durable memory from current Git, deployment, runtime, and business proof.

When an ordinary repository task reaches a successful or failed terminal state, Multica now also:

1. retains the exact issue/workstream/repository binding established before launch;
2. parses an optional strict agent proposal as untrusted draft content, or creates a visible fallback draft;
3. creates exactly one pending review record for the terminal task, including a failure-oriented fallback when a failed run did not produce structured memory;
4. shows that record on the issue in Multica, where a human can edit, approve, or reject it;
5. queues only the human-approved payload for delivery; and
6. uses a leased, retryable worker to register the task session, write approved memory items and checkpoint, and close the task session in Intelligence with its real terminal status.

The agent cannot approve its own proposal. Rejected proposals never leave Multica. A timeout can replay the full ordered Intelligence sequence safely because every destination identity is deterministic and idempotent. Approved deliveries remain visible while pending or retrying. After 24 failed attempts, delivery pauses visibly instead of disappearing; a human can requeue it after the underlying problem is fixed.

The adapter is disabled when all three `MULTICA_INTELLIGENCE_*` variables are empty. Partial or invalid configuration logs a startup error and fails closed: repository-backed tasks requeue instead of silently running without memory. Plain HTTP is limited to loopback or `host.docker.internal`; other connections require HTTPS.

## Deliberate boundaries

- Controller-bound workflow tasks receive no ambient development memory.
- Tasks without a Git repository continue normally; there is no repository identity to query.
- A later task cannot silently add a repository to an existing workstream. The claim is requeued and the mismatch is reported so repository scope must be reviewed explicitly.
- The issue title and goal are a first-observed snapshot in this slice. Editing the Multica issue later does not silently rewrite the retained workstream; a reviewed revision endpoint is still required.
- Full checkpoints and ordinary memory remain scoped to the same stable Multica issue, so unrelated workstreams are not dumped into every prompt.
- A human reviewer can promote an individual memory item from this issue to the Multica project baseline. New issues in that project receive those approved items with their source workstream, task, repository, and evidence references. Workstream scope remains the default; agents cannot promote their own output.
- Task sessions are registered only after human approval, not at claim time, so a failed dispatch cannot create a false active task in Intelligence. Failed tasks that actually started can still produce a reviewed failure checkpoint.
- No historical Codex or Claude chats are imported automatically.

## Required acceptance before release

1. Add reviewed workstream revisions for changed goals or repository scope.
2. Configure distinct Multica writer and reader credentials in Intelligence.
3. Repeat the accepted disposable CRM + Viral Queue HTTP canary on the real 32GB stack.
4. Prove the first task creates one editable pending proposal, rejection sends nothing, and approval reaches Intelligence once.
5. Prove the second task receives the first task's approved decision and checkpoint, and a separate issue receives only approved project-scoped memory.
6. Prove Intelligence outage requeues a new task and retries an approved delivery without exposing a credential or losing the issue.
7. Review, qualify, and release Intelligence and Multica independently; then verify the deployed revisions and owner-visible behavior.
