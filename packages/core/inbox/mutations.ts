import { useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "../api";
import { inboxKeys } from "./queries";
import { useWorkspaceId } from "../hooks";
import { issueKeys } from "../issues/queries";
import type {
  CreateOwnerDirectiveRequest,
  CreateOwnerReviewFeedbackRequest,
  InboxItem,
  RespondWorkflowDecisionRequest,
} from "../types";

export function useMarkInboxRead() {
  const qc = useQueryClient();
  const wsId = useWorkspaceId();
  return useMutation({
    mutationFn: (id: string) => api.markInboxRead(id),
    onMutate: async (id) => {
      await qc.cancelQueries({ queryKey: inboxKeys.all(wsId) });
      const prev = qc.getQueryData<InboxItem[]>(inboxKeys.list(wsId));
      const prevArchived = qc.getQueryData<InboxItem[]>(inboxKeys.archived(wsId));
      const markRead = (old: InboxItem[] | undefined) =>
        old?.map((item) => (item.id === id ? { ...item, read: true } : item));
      qc.setQueryData<InboxItem[]>(inboxKeys.list(wsId), markRead);
      // Opening a notification from the archived sub-view marks it read too —
      // patch that cache as well, or its unread dot would sit there until the
      // next refetch.
      qc.setQueryData<InboxItem[]>(inboxKeys.archived(wsId), markRead);
      return { prev, prevArchived };
    },
    onError: (_err, _id, ctx) => {
      if (ctx?.prev) qc.setQueryData(inboxKeys.list(wsId), ctx.prev);
      if (ctx?.prevArchived) qc.setQueryData(inboxKeys.archived(wsId), ctx.prevArchived);
    },
    onSettled: () => {
      qc.invalidateQueries({ queryKey: inboxKeys.all(wsId) });
    },
  });
}

export function useAnswerWorkflowOwnerDecision() {
  const qc = useQueryClient();
  const wsId = useWorkspaceId();
  return useMutation({
    mutationFn: async ({ item, answer }: { item: InboxItem; answer: string }) => {
      if (!item.issue_id || !item.details?.workflow_event_id) throw new Error("Decision request is missing its workflow reference");
      await api.answerWorkflowOwnerDecision(item.issue_id, item.details.workflow_event_id, answer);
      return api.archiveInbox(item.id);
    },
    onSettled: () => {
      qc.invalidateQueries({ queryKey: ["chat-work", wsId] });
      qc.invalidateQueries({ queryKey: ["chat-work-index", wsId] });
      qc.invalidateQueries({ queryKey: inboxKeys.all(wsId) });
    },
  });
}

export function useRespondWorkflowDecision() {
  const qc = useQueryClient();
  const wsId = useWorkspaceId();
  return useMutation({
    mutationFn: ({ issueId, decisionId, request }: {
      issueId: string;
      decisionId: string;
      request: RespondWorkflowDecisionRequest;
    }) => api.respondWorkflowDecision(issueId, decisionId, request, crypto.randomUUID()),
    onSettled: (_data, _error, vars) => {
      qc.invalidateQueries({ queryKey: ["chat-work", wsId] });
      qc.invalidateQueries({ queryKey: ["chat-work-index", wsId] });
      qc.invalidateQueries({ queryKey: inboxKeys.decision(vars.issueId, vars.decisionId) });
      qc.invalidateQueries({ queryKey: inboxKeys.all(wsId) });
      qc.invalidateQueries({ queryKey: inboxKeys.unreadSummary() });
    },
  });
}

export function useAcknowledgeWorkflowAttention() {
  const qc = useQueryClient();
  const wsId = useWorkspaceId();
  return useMutation({
    mutationFn: ({ issueId, attentionId }: { issueId: string; attentionId: string }) =>
      api.acknowledgeWorkflowAttention(issueId, attentionId, crypto.randomUUID()),
    onSettled: () => qc.invalidateQueries({ queryKey: inboxKeys.all(wsId) }),
  });
}

export function useCreateOwnerDirective() {
  const qc = useQueryClient();
  const wsId = useWorkspaceId();
  return useMutation({
    mutationFn: ({ issueId, request }: { issueId: string; request: CreateOwnerDirectiveRequest }) =>
      api.createOwnerDirective(issueId, request),
    onSettled: () => {
      qc.invalidateQueries({ queryKey: ["chat-work", wsId] });
      qc.invalidateQueries({ queryKey: ["chat-work-index", wsId] });
      qc.invalidateQueries({ queryKey: inboxKeys.all(wsId) });
      qc.invalidateQueries({ queryKey: ["issues", wsId] });
    },
  });
}

/**
 * Persist a rendered owner markup and its structured review receipt as one UI
 * mutation. The API is necessarily two-step (upload, then bind evidence), but
 * views never call the API client directly and never advance the decision
 * unless both steps succeed.
 */
export function usePersistOwnerReviewFeedback() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async ({
      issueId,
      file,
      request,
    }: {
      issueId: string;
      file: File;
      request: Omit<CreateOwnerReviewFeedbackRequest, "marked_screenshot_attachment_id">;
    }) => {
      const marked = await api.uploadFile(file, { issueId });
      const feedback = await api.createIssueReviewFeedback(issueId, {
        ...request,
        marked_screenshot_attachment_id: marked.id,
      });
      return { feedback, marked };
    },
    onSettled: (_data, _error, variables) => {
      qc.invalidateQueries({ queryKey: issueKeys.attachments(variables.issueId) });
      qc.invalidateQueries({ queryKey: ["owner-review-feedback", variables.issueId] });
    },
  });
}

export function useRetrySourceContextQuickCreate() {
  const qc = useQueryClient();
  const wsId = useWorkspaceId();
  return useMutation({
    mutationFn: (taskId: string) => api.retrySourceContextQuickCreate(taskId),
    onSettled: () => {
      qc.invalidateQueries({ queryKey: inboxKeys.all(wsId) });
    },
  });
}

/**
 * Flip a notification back to unread — the inverse of {@link useMarkInboxRead}.
 *
 * Same optimistic shape as marking read (predictable outcome, no navigation,
 * trivial rollback), and it patches BOTH caches for the same reason: an item
 * can be actioned from either list, and leaving the other one stale would show
 * two different read states for one notification after a view switch.
 *
 * The unread badge is derived from the main list's cache (`useInboxUnreadCount`
 * dedupes it client-side), so the optimistic patch raises the badge without
 * waiting for the round-trip; `onSettled` re-pulls the cross-workspace summary,
 * which is server-computed and cannot be patched here.
 */
export function useMarkInboxUnread() {
  const qc = useQueryClient();
  const wsId = useWorkspaceId();
  return useMutation({
    mutationFn: (id: string) => api.markInboxUnread(id),
    onMutate: async (id) => {
      await qc.cancelQueries({ queryKey: inboxKeys.all(wsId) });
      const prev = qc.getQueryData<InboxItem[]>(inboxKeys.list(wsId));
      const prevArchived = qc.getQueryData<InboxItem[]>(inboxKeys.archived(wsId));
      const markUnread = (old: InboxItem[] | undefined) =>
        old?.map((item) => (item.id === id ? { ...item, read: false } : item));
      qc.setQueryData<InboxItem[]>(inboxKeys.list(wsId), markUnread);
      qc.setQueryData<InboxItem[]>(inboxKeys.archived(wsId), markUnread);
      return { prev, prevArchived };
    },
    onError: (_err, _id, ctx) => {
      if (ctx?.prev) qc.setQueryData(inboxKeys.list(wsId), ctx.prev);
      if (ctx?.prevArchived) qc.setQueryData(inboxKeys.archived(wsId), ctx.prevArchived);
    },
    onSettled: () => {
      qc.invalidateQueries({ queryKey: inboxKeys.all(wsId) });
      // The switcher dot must light again when the workspace goes back to
      // having unread items — that count lives on the server.
      qc.invalidateQueries({ queryKey: inboxKeys.unreadSummary() });
    },
  });
}

export function useArchiveInbox() {
  const qc = useQueryClient();
  const wsId = useWorkspaceId();
  return useMutation({
    mutationFn: (id: string) => api.archiveInbox(id),
    onMutate: async (id) => {
      await qc.cancelQueries({ queryKey: inboxKeys.list(wsId) });
      const prev = qc.getQueryData<InboxItem[]>(inboxKeys.list(wsId));
      qc.setQueryData<InboxItem[]>(inboxKeys.list(wsId), (old) =>
        old?.map((item) => item.id === id ? { ...item, archived: true } : item),
      );
      return { prev };
    },
    onError: (_err, _id, ctx) => {
      if (ctx?.prev) qc.setQueryData(inboxKeys.list(wsId), ctx.prev);
    },
    onSettled: () => {
      // Both lists: the item just moved from the main inbox into the archive.
      qc.invalidateQueries({ queryKey: inboxKeys.all(wsId) });
    },
  });
}

/**
 * Restore an archived notification to the main inbox.
 *
 * Optimistic on the ARCHIVED cache only: flipping `archived` there makes the
 * row leave the archived list at once (the dedup helper filters on it), the
 * user stays put, and rollback is a single snapshot restore. The main list is
 * left to `onSettled` — its contents after a restore are the server's call
 * (which sibling rows come back, their read state, their order), so it is
 * invalidated rather than reconstructed client-side.
 */
export function useUnarchiveInbox() {
  const qc = useQueryClient();
  const wsId = useWorkspaceId();
  return useMutation({
    mutationFn: (id: string) => api.unarchiveInbox(id),
    onMutate: async (id) => {
      await qc.cancelQueries({ queryKey: inboxKeys.archived(wsId) });
      const prev = qc.getQueryData<InboxItem[]>(inboxKeys.archived(wsId));
      qc.setQueryData<InboxItem[]>(inboxKeys.archived(wsId), (old) =>
        old?.map((item) => item.id === id ? { ...item, archived: false } : item),
      );
      return { prev };
    },
    onError: (_err, _id, ctx) => {
      if (ctx?.prev) qc.setQueryData(inboxKeys.archived(wsId), ctx.prev);
    },
    onSettled: () => {
      // Both lists: the item moves from one to the other, and the unread badge
      // rises again when it was archived unread.
      qc.invalidateQueries({ queryKey: inboxKeys.all(wsId) });
      qc.invalidateQueries({ queryKey: inboxKeys.unreadSummary() });
    },
  });
}

export function useMarkAllInboxRead() {
  const qc = useQueryClient();
  const wsId = useWorkspaceId();
  return useMutation({
    mutationFn: () => api.markAllInboxRead(),
    onMutate: async () => {
      await qc.cancelQueries({ queryKey: inboxKeys.list(wsId) });
      const prev = qc.getQueryData<InboxItem[]>(inboxKeys.list(wsId));
      qc.setQueryData<InboxItem[]>(inboxKeys.list(wsId), (old) =>
        old?.map((item) =>
          !item.archived ? { ...item, read: true } : item,
        ),
      );
      return { prev };
    },
    onError: (_err, _vars, ctx) => {
      if (ctx?.prev) qc.setQueryData(inboxKeys.list(wsId), ctx.prev);
    },
    onSettled: () => {
      qc.invalidateQueries({ queryKey: inboxKeys.list(wsId) });
    },
  });
}

// The three batch-archive mutations below all move items into the archive, so
// each invalidates BOTH lists on settle.
export function useArchiveAllInbox() {
  const qc = useQueryClient();
  const wsId = useWorkspaceId();
  return useMutation({
    mutationFn: () => api.archiveAllInbox(),
    onSettled: () => {
      qc.invalidateQueries({ queryKey: inboxKeys.all(wsId) });
    },
  });
}

export function useArchiveAllReadInbox() {
  const qc = useQueryClient();
  const wsId = useWorkspaceId();
  return useMutation({
    mutationFn: () => api.archiveAllReadInbox(),
    onSettled: () => {
      qc.invalidateQueries({ queryKey: inboxKeys.all(wsId) });
    },
  });
}

export function useArchiveCompletedInbox() {
  const qc = useQueryClient();
  const wsId = useWorkspaceId();
  return useMutation({
    mutationFn: () => api.archiveCompletedInbox(),
    onSettled: () => {
      qc.invalidateQueries({ queryKey: inboxKeys.all(wsId) });
    },
  });
}
