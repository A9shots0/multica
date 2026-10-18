import type { IssuePriority, IssueStatus } from "./issue";

export type InboxSeverity = "action_required" | "attention" | "info";

export type InboxItemType =
  | "chat_intake"
  | "issue_assigned"
  | "issue_subscribed"
  | "unassigned"
  | "assignee_changed"
  | "status_changed"
  | "priority_changed"
  | "start_date_changed"
  | "due_date_changed"
  | "new_comment"
  | "mentioned"
  | "review_requested"
  | "task_completed"
  | "task_failed"
  | "agent_blocked"
  | "agent_completed"
  | "reaction_added"
  | "quick_create_done"
  | "quick_create_failed"
  // Quick create whose outcome could not be verified. Distinct from
  // quick_create_failed because it must NOT be rendered with failure framing:
  // the issue may actually have been created.
  | "quick_create_unconfirmed"
  | "workflow_owner_decision"
  // Emitted by the workflow driver when a run changes state on its own (a
  // stopped execution, a failed release). It has been written to the Inbox for
  // as long as the driver has existed; leaving it out of this union meant the
  // client's own type said these rows could not arrive, while the owner was
  // looking at seven of them.
  | "workflow_incident"
  // Important state reported by work running outside Multica. This remains an
  // observation: it does not grant Multica control of the outside provider.
  | "external_work_update"
  // System notifications are intentionally issue-less. Keep them in the
  // same Inbox model so read/archive/realtime behavior remains consistent.
  | "autopilot_paused"
  | "autopilot_quota_exceeded";

/**
 * One workspace's unread inbox count in the cross-workspace summary
 * (`GET /api/inbox/unread-summary`). The sidebar uses this to light a dot on
 * the workspace switcher when a workspace OTHER than the active one has
 * unread items.
 */
export interface InboxWorkspaceUnread {
  workspace_id: string;
  count: number;
}

export interface RecoveryStatus {
  status:
    | "protected"
    | "backup_due"
    | "recovery_unproven"
    | "secondary_copy_needed"
    | "backup_failed"
    | "platform_unavailable";
  last_backup_at?: string;
  backup_age_hours?: number;
  last_restore_at?: string;
  replication: string;
  last_checked_at?: string;
  next_action: string;
  evidence_current: boolean;
}

export interface InboxItem {
  id: string;
  workspace_id: string;
  recipient_type: "member" | "agent";
  recipient_id: string;
  actor_type: "member" | "agent" | "system" | null;
  actor_id: string | null;
  type: InboxItemType;
  severity: InboxSeverity;
  issue_id: string | null;
  title: string;
  body: string | null;
  issue_status: IssueStatus | null;
  /**
   * Current priority of the linked issue. Optional so an installed Desktop
   * client remains compatible with an older backend that predates this Inbox
   * projection; null also covers notifications without a linked issue.
   */
  issue_priority?: IssuePriority | null;
  read: boolean;
  archived: boolean;
  created_at: string;
  details: Record<string, string> | null;
}

export type OwnerInboxCategory =
  | "needs_you"
  | "incident"
  | "update"
  | "completion"
  | "history";
