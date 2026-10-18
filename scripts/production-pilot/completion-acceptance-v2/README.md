# Bounded native publication acceptance

Successor of the frozen SILV-38 pilot harnesses. It binds to a new native issue titled **Verify publication of the reviewed CRM handoff**, retains the original dated evidence and the unchanged v2 candidate (SHA256 `2df9d83d52cf882f7b399b4b823d6d0cc975d41be6856b8838b276d6f494541c`), and proves comment-triggered publication followed by native independent review. Original SILV-38 tasks/artifacts remain preserved.

The provider never rewrites the document. The publisher verifies it; the broker publishes its unchanged text and digest using the current native `parent_id`. A supported `implementation_ready` event routes an independent reviewer; its real pass emits `review_passed`. The three temporary roles are suppressed from incidental comment routing. Only one publisher and one reviewer slot exist. Leader, correction, retry, tools and MCP calls are denied. A failed verdict is retained and stops execution. The existing workflow template permits one correction structurally, but no runtime receipt or executable path admits a correction call.

## Gates and order

Application correction, one independent review with findings addressed, affected tests, local built-in Browser acceptance, recovery/rollback readiness and exact private deployment must pass first. `qualification.json` binds reviewed local source hashes and the review digest; `release-identity.json` binds the deployed source and backend image. No mode may bypass those checks.

1. `prepare-resources`: normal owner login, idempotent new issue, offline private runtime, three roles and squad/plan. No daemon/provider credentials or task dispatch.
2. `build-bound`: build the broker with the exact new issue and three role IDs; build the current CLI and a Linux authority helper. Internal-import staging is task-local and ignored. Record binary digests. No provider contact.
3. `provision-daemon`, `start-daemon`: exact private native runtime with scoped expiring daemon token. Provider profile absent.
4. `prepare-authority`, `issue-authority`: host controller validates and stores immutable exact issue/role/runtime/executable grants.
5. `prepare-provider`: opaque copy of the existing Claude subscription into its private temporary profile. Never parse, print or hash credential bytes.
6. `launch-publication`: one owner mention of the publisher, with the planner/reviewer suppressed. The launch intent is persisted before POST; uncertain responses do not permit an automatic retry.
7. Inspect the two real tasks, their trigger/source event, published parent and digest, current picture/plan/Issue/Now/Inbox/Board, including reloads. Only matching evidence passes acceptance.
8. `cleanup`: revoke grants, stop only the recorded daemon, deregister its runtime, remove scoped tokens/profiles and archive the temporary roles. Independent local cleanup runs despite server failures; incomplete cleanup fails visibly.

No fresh CRM/Phone Farm inspection, public route, outbound message, extra provider call or broad dispatch is authorized. Original harness directories are unchanged. This file records preparation; live results belong in separate timestamped evidence.

Both captured evidence and candidate bytes have fixed approved digests, checked before each provider slot and again before artifact publication. Every executable is rechecked against the bound build receipt before credentials, execution or authority use; source fingerprints are verified around builds.
