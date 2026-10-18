import { queryOptions, useQuery } from "@tanstack/react-query";
import { api } from "../api";
import type { InboxItem, InboxWorkspaceUnread } from "../types";
import { isOwnerRelevantInboxItem } from "./owner-control";
import { groupOwnerInbox, needsOwnerAction } from "./owner-inbox-model";

export const inboxKeys = {
  all: (wsId: string) => ["inbox", wsId] as const,
  list: (wsId: string) => [...inboxKeys.all(wsId), "list"] as const,
  archived: (wsId: string) => [...inboxKeys.all(wsId), "archived"] as const,
  // Account-level (not workspace-scoped): a single shared cache entry that
  // holds unread counts for every workspace the user belongs to.
  unreadSummary: () => ["inbox", "unread-summary"] as const,
  decision: (issueId: string, decisionId: string) =>
    ["inbox", "workflow-decision", issueId, decisionId] as const,
  ownerNotifications: (issueId: string) =>
    ["inbox", "owner-notifications", issueId] as const,
  reviewFeedback: (issueId: string) =>
    ["owner-review-feedback", issueId] as const,
};

/**
 * The marked screenshot is immutable review evidence, not another candidate
 * result. Consumers use this list to keep generated evidence out of the media
 * carousel while retaining it in the issue's audit trail.
 */
export function ownerReviewFeedbackOptions(issueId: string) {
  return queryOptions({
    queryKey: inboxKeys.reviewFeedback(issueId),
    queryFn: () => api.listIssueReviewFeedback(issueId),
    enabled: !!issueId,
    staleTime: 0,
  });
}

export function ownerNotificationOptions(issueId: string) {
  return queryOptions({
    queryKey: inboxKeys.ownerNotifications(issueId),
    queryFn: () => api.listIssueOwnerNotifications(issueId),
    enabled: !!issueId,
    staleTime: 0,
  });
}

export function workflowDecisionOptions(issueId: string, decisionId: string) {
  return queryOptions({
    queryKey: inboxKeys.decision(issueId, decisionId),
    queryFn: () => api.getWorkflowDecision(issueId, decisionId),
    enabled: !!issueId && !!decisionId,
    staleTime: 0,
  });
}

export function inboxListOptions(wsId: string) {
  return queryOptions({
    queryKey: inboxKeys.list(wsId),
    queryFn: () => api.listInbox(),
  });
}

/**
 * Archived notifications, backing the inbox's "Archived" sub-view. A separate
 * cache entry from the main list rather than one flat cache split locally
 * (which is what chat does): the archive grows without end, so it is fetched
 * from its own capped endpoint, and the server — not the client — decides
 * which issues belong in which list.
 */
export function archivedInboxListOptions(wsId: string) {
  return queryOptions({
    queryKey: inboxKeys.archived(wsId),
    queryFn: () => api.listArchivedInbox(),
  });
}

/**
 * Cross-workspace unread inbox summary. One cache entry shared across all
 * workspaces — the data is account-level, so switching workspaces does not
 * refetch it; only the derived "is this for another workspace" view changes.
 */
export function inboxUnreadSummaryOptions() {
  return queryOptions({
    queryKey: inboxKeys.unreadSummary(),
    queryFn: () => api.getInboxUnreadSummary(),
  });
}

/**
 * Whether any workspace OTHER than `currentWsId` has unread inbox items.
 * Drives the workspace-switcher dot: the active workspace's own unread is
 * already surfaced by the Inbox nav count, so it is excluded here to avoid a
 * duplicate signal.
 */
export function hasOtherWorkspaceUnread(
  summary: InboxWorkspaceUnread[],
  currentWsId: string | null | undefined,
): boolean {
  return summary.some((s) => s.workspace_id !== currentWsId && s.count > 0);
}

/**
 * Set of workspace ids that have unread inbox items. Lets the workspace
 * switcher dropdown mark WHICH workspace a pending message lives in (the
 * aggregate switcher dot only says "somewhere else"). Workspaces with a zero
 * count are excluded.
 */
export function unreadWorkspaceIds(summary: InboxWorkspaceUnread[]): Set<string> {
  return new Set(summary.filter((s) => s.count > 0).map((s) => s.workspace_id));
}

/**
 * Unread inbox count for the given workspace, aligned with what the inbox
 * list UI renders: archived items excluded, then deduplicated by issue so a
 * single issue with three unread notifications counts once.
 */
export function useInboxUnreadCount(wsId: string | null | undefined): number {
  const { data } = useQuery({
    queryKey: inboxKeys.list(wsId ?? ""),
    queryFn: () => api.listInbox(),
    enabled: !!wsId,
    select: (items: InboxItem[]) =>
      groupOwnerInbox(deduplicateInboxItems(items.filter(isOwnerRelevantInboxItem)))
        .filter(needsOwnerAction)
        .filter((group) => group.unread).length,
  });
  return data ?? 0;
}

/**
 * How many pieces of work are actually waiting on the owner — the same number
 * the Inbox page itself shows in its title.
 *
 * `useInboxUnreadCount` above counts unread EVENTS: a stalled run that failed
 * twice before asking a question is three unread rows. This counts pieces of
 * work, grouped the same way the page groups them, and only the ones with a
 * real decision or stall sitting on them. Nothing that reads "Inbox" anywhere
 * in the app should show a number the Inbox page itself would not stand
 * behind — a sidebar badge reading 11 beside a page heading reading 7 is the
 * same failure this whole rebuild exists to fix, just moved one component
 * over. Shares the list query's cache key, so this costs no extra fetch.
 */
export function useOwnerInboxWaitingCount(wsId: string | null | undefined): number {
  const { data } = useQuery({
    queryKey: inboxKeys.list(wsId ?? ""),
    queryFn: () => api.listInbox(),
    enabled: !!wsId,
    select: (items: InboxItem[]) =>
      groupOwnerInbox(deduplicateInboxItems(items.filter(isOwnerRelevantInboxItem))).filter((g) =>
        needsOwnerAction(g),
      ).length,
  });
  return data ?? 0;
}

/**
 * Keep each owner-relevant attention independently actionable. Legacy noise
 * may still be grouped by issue, but decisions and material incidents must
 * never hide one another merely because they belong to the same issue.
 * Exported for consumers to use in useMemo — not in queryOptions select
 * (to avoid new array references on every cache update).
 */
export function deduplicateInboxItems(items: InboxItem[]): InboxItem[] {
  return groupInboxItemsByIssue(items.filter((i) => !i.archived));
}

/**
 * Same grouping for the archived sub-view. The `archived` filter is what makes
 * an optimistic unarchive drop the row out of the archived list immediately —
 * exactly mirroring how `deduplicateInboxItems`' filter drops an optimistically
 * archived row out of the main list.
 */
export function deduplicateArchivedInboxItems(items: InboxItem[]): InboxItem[] {
  return groupInboxItemsByIssue(items.filter((i) => i.archived));
}

function groupInboxItemsByIssue(items: InboxItem[]): InboxItem[] {
  const groups = new Map<string, InboxItem[]>();
  for (const item of items) {
    // Failed runs are issue incidents and must share one group across retries;
    // issue-less owner actions still remain independently addressable.
    const key = item.type === "task_failed" && item.issue_id
      ? item.issue_id
      : isOwnerRelevantInboxItem(item)
        ? item.id
        : (item.issue_id ?? item.id);
    const group = groups.get(key) ?? [];
    group.push(item);
    groups.set(key, group);
  }
  const merged: InboxItem[] = [];
  for (const group of groups.values()) {
    group.sort(
      (a, b) =>
        new Date(b.created_at).getTime() - new Date(a.created_at).getTime(),
    );
    const actionable = group.filter((item) =>
      item.type === "workflow_owner_decision" ||
      item.severity === "action_required" ||
      item.type === "task_completed" ||
      item.type === "task_failed");
    // Repeated failed attempts are one incident for the owner, not one Inbox
    // card per retry/manual rerun. Keep the newest row, but carry the count so
    // the detail view remains honest about repeated failures.
    const failures = actionable.filter((item) => item.type === "task_failed");
    const nonFailures = actionable.filter((item) => item.type !== "task_failed");
    const newestFailure = failures[0];
    const failureRepresentative: InboxItem | null = newestFailure
      ? {
          ...newestFailure,
          details: {
            ...(newestFailure.details ?? {}),
            failure_count: String(failures.length),
          },
        }
      : null;
    const representatives = actionable.length > 0
      ? [...nonFailures, ...(failureRepresentative ? [failureRepresentative] : [])]
      : group.slice(0, 1);
    const commentId = group.find((item) => item.details?.comment_id)?.details?.comment_id;
    for (const representative of representatives) {
      merged.push(commentId && !representative.details?.comment_id
        ? { ...representative, details: { ...(representative.details ?? {}), comment_id: commentId } }
        : representative);
    }
  }
  return merged.sort(
    (a, b) =>
      new Date(b.created_at).getTime() - new Date(a.created_at).getTime(),
  );
}
