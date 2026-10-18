import type { InboxItem, OwnerInboxCategory } from "../types";

export function isOwnerRelevantInboxItem(item: InboxItem): boolean {
  // A failed run is an owner-visible incident even when older producers wrote
  // it with `info`/`attention` instead of the legacy action-required severity.
  if (item.type === "task_failed") return true;
  if (item.type === "workflow_owner_decision") return true;
  if (item.details?.owner_relevant === "true") return true;
  if (item.type === "autopilot_quota_exceeded") return true;
  if (item.type === "quick_create_done" || item.type === "quick_create_failed" || item.type === "quick_create_unconfirmed") return true;
  if (item.type === "task_completed" && item.details?.independently_verified === "true") return true;
  return item.severity === "action_required" && !!item.details?.required_action;
}

export function ownerInboxCategory(item: InboxItem): OwnerInboxCategory {
  // A failed run is a fact about system health, not a question that can be
  // answered with a comment. Keep it in Incidents even for legacy rows that
  // were persisted as action_required.
  if (item.type === "task_failed") return "incident";
  const projected = item.details?.owner_category;
  if (projected === "needs_you" || projected === "incident" || projected === "update" || projected === "completion" || projected === "history") return projected;
  if (item.type === "workflow_owner_decision" || item.severity === "action_required") return "needs_you";
  if (item.type === "task_completed") return "completion";
  if (item.type === "agent_blocked" || item.severity === "attention") return "incident";
  if (["status_changed", "priority_changed", "review_requested"].includes(item.type)) return "update";
  return "history";
}
