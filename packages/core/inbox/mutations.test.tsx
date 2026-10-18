/**
 * @vitest-environment jsdom
 */
import { beforeEach, describe, expect, it, vi } from "vitest";
import { renderHook, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import type { ReactNode } from "react";

import { setApiInstance } from "../api";
import type { ApiClient } from "../api/client";
import type { InboxItem } from "../types";
import {
  useArchiveInbox,
  useAnswerWorkflowOwnerDecision,
  useMarkInboxUnread,
  useUnarchiveInbox,
} from "./mutations";
import { inboxKeys } from "./queries";

vi.mock("../hooks", () => ({
  useWorkspaceId: () => "workspace-1",
}));

const WORKSPACE_ID = "workspace-1";

function item(overrides: Partial<InboxItem> = {}): InboxItem {
  return {
    id: "inbox-1",
    workspace_id: WORKSPACE_ID,
    recipient_type: "member",
    recipient_id: "member-1",
    actor_type: "agent",
    actor_id: "agent-1",
    type: "new_comment",
    severity: "info",
    issue_id: "issue-1",
    title: "Issue title",
    body: null,
    issue_status: null,
    read: false,
    archived: true,
    created_at: "2026-06-15T08:00:00Z",
    details: null,
    ...overrides,
  };
}

function createWrapper(queryClient: QueryClient) {
  return function Wrapper({ children }: { children: ReactNode }) {
    return (
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    );
  };
}

function archivedCache(qc: QueryClient) {
  return qc.getQueryData<InboxItem[]>(inboxKeys.archived(WORKSPACE_ID)) ?? [];
}

function listCache(qc: QueryClient) {
  return qc.getQueryData<InboxItem[]>(inboxKeys.list(WORKSPACE_ID)) ?? [];
}

describe("useMarkInboxUnread", () => {
  let queryClient: QueryClient;
  let markInboxUnread: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
    });
    markInboxUnread = vi.fn(async (id: string) => item({ id, read: false }));
    setApiInstance({ markInboxUnread } as unknown as ApiClient);
  });

  it("flips only the targeted item unread, in both lists", async () => {
    // Item-level, mirroring mark-read: the list shows one row per issue
    // carrying that group's newest item, so flipping siblings would resurrect
    // notifications the user already dealt with without changing the row.
    queryClient.setQueryData<InboxItem[]>(inboxKeys.list(WORKSPACE_ID), [
      item({ id: "inbox-1", read: true, archived: false }),
      item({ id: "sibling", issue_id: "issue-1", read: true, archived: false }),
    ]);
    queryClient.setQueryData<InboxItem[]>(inboxKeys.archived(WORKSPACE_ID), [
      item({ id: "inbox-1", read: true }),
    ]);

    const { result } = renderHook(() => useMarkInboxUnread(), {
      wrapper: createWrapper(queryClient),
    });
    result.current.mutate("inbox-1");

    await waitFor(() => {
      expect(
        listCache(queryClient).find((i) => i.id === "inbox-1")?.read,
      ).toBe(false);
    });
    expect(listCache(queryClient).find((i) => i.id === "sibling")?.read).toBe(true);
    // Actioned from either list, patched in both — otherwise a view switch
    // shows two different read states for one notification.
    expect(archivedCache(queryClient)[0]?.read).toBe(false);
  });

  it("refreshes the cross-workspace summary so the switcher dot lights again", async () => {
    queryClient.setQueryData<InboxItem[]>(inboxKeys.list(WORKSPACE_ID), [
      item({ id: "inbox-1", read: true, archived: false }),
    ]);
    const invalidate = vi.spyOn(queryClient, "invalidateQueries");

    const { result } = renderHook(() => useMarkInboxUnread(), {
      wrapper: createWrapper(queryClient),
    });
    result.current.mutate("inbox-1");

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(invalidate).toHaveBeenCalledWith({
      queryKey: inboxKeys.unreadSummary(),
    });
  });

  it("rolls both lists back when the request fails", async () => {
    markInboxUnread.mockRejectedValue(new Error("boom"));
    const active = [item({ id: "inbox-1", read: true, archived: false })];
    const archived = [item({ id: "inbox-1", read: true })];
    queryClient.setQueryData<InboxItem[]>(inboxKeys.list(WORKSPACE_ID), active);
    queryClient.setQueryData<InboxItem[]>(inboxKeys.archived(WORKSPACE_ID), archived);

    const { result } = renderHook(() => useMarkInboxUnread(), {
      wrapper: createWrapper(queryClient),
    });
    result.current.mutate("inbox-1");

    await waitFor(() => expect(result.current.isError).toBe(true));
    expect(listCache(queryClient)).toEqual(active);
    expect(archivedCache(queryClient)).toEqual(archived);
  });
});

describe("useAnswerWorkflowOwnerDecision current split mutation", () => {
  it("rejects and leaves the item unarchived after the decision succeeds but archiving fails", async () => {
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
    });
    let authorityCommitted = false;
    const answerWorkflowOwnerDecision = vi.fn(async () => {
      authorityCommitted = true;
      return { accepted: true };
    });
    const archiveError = new Error("archive failed after decision commit");
    const archiveInbox = vi.fn(async () => {
      throw archiveError;
    });
    setApiInstance({
      answerWorkflowOwnerDecision,
      archiveInbox,
    } as unknown as ApiClient);
    const decisionItem = item({
      archived: false,
      details: { workflow_event_id: "workflow-event-1" },
    });
    queryClient.setQueryData<InboxItem[]>(inboxKeys.list(WORKSPACE_ID), [
      decisionItem,
    ]);

    const { result } = renderHook(() => useAnswerWorkflowOwnerDecision(), {
      wrapper: createWrapper(queryClient),
    });
    result.current.mutate({ item: decisionItem, answer: "approved" });

    await waitFor(() => expect(result.current.isError).toBe(true));

    // Characterization only: the first request may have committed authority,
    // while the composed UI mutation reports failure and never archives the
    // decision notification. A later contract change must make this atomic or
    // reconcile the partial success explicitly.
    expect(authorityCommitted).toBe(true);
    expect(answerWorkflowOwnerDecision).toHaveBeenCalledWith(
      "issue-1",
      "workflow-event-1",
      "approved",
    );
    expect(archiveInbox).toHaveBeenCalledWith("inbox-1");
    expect(result.current.error).toBe(archiveError);
    expect(listCache(queryClient)[0]?.archived).toBe(false);
  });
});

describe("useUnarchiveInbox", () => {
  let queryClient: QueryClient;
  let unarchiveInbox: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
    });
    unarchiveInbox = vi.fn(async (id: string) => item({ id, archived: false }));
    setApiInstance({ unarchiveInbox } as unknown as ApiClient);
  });

  it("restores only the selected archived owner-control record", async () => {
    queryClient.setQueryData<InboxItem[]>(inboxKeys.archived(WORKSPACE_ID), [
      item({ id: "sibling-a", issue_id: "issue-1" }),
      item({ id: "sibling-b", issue_id: "issue-1" }),
      item({ id: "other-issue", issue_id: "issue-2" }),
    ]);

    const { result } = renderHook(() => useUnarchiveInbox(), {
      wrapper: createWrapper(queryClient),
    });
    result.current.mutate("sibling-a");

    await waitFor(() => {
      const stillArchived = archivedCache(queryClient).filter((i) => i.archived);
      expect(stillArchived.map((i) => i.id)).toEqual(["sibling-b", "other-issue"]);
    });
  });

  it("restores only one independently actionable decision", async () => {
    queryClient.setQueryData<InboxItem[]>(inboxKeys.archived(WORKSPACE_ID), [
      item({ id: "decision-a", issue_id: "issue-1", type: "workflow_owner_decision" }),
      item({ id: "decision-b", issue_id: "issue-1", type: "workflow_owner_decision" }),
    ]);

    const { result } = renderHook(() => useUnarchiveInbox(), {
      wrapper: createWrapper(queryClient),
    });
    result.current.mutate("decision-a");

    await waitFor(() => {
      expect(archivedCache(queryClient).filter((i) => i.archived).map((i) => i.id)).toEqual(["decision-b"]);
    });
  });

  it("preserves unread state and refreshes the badge sources, so the badge rises again", async () => {
    // Restoring an item that was archived while unread legitimately RAISES the
    // unread badge: the count only ever included non-archived items. Two halves
    // make that work, and this covers the client's half — never touch `read`,
    // and re-pull both the workspace list (the Inbox nav count) and the
    // cross-workspace summary (the switcher dot). The server's half — that
    // UnarchiveInboxItem leaves `read` alone — is pinned by
    // TestUnarchiveInboxPreservesUnread in the Go suite.
    queryClient.setQueryData<InboxItem[]>(inboxKeys.archived(WORKSPACE_ID), [
      item({ id: "inbox-1", read: false }),
    ]);
    const invalidate = vi.spyOn(queryClient, "invalidateQueries");

    const { result } = renderHook(() => useUnarchiveInbox(), {
      wrapper: createWrapper(queryClient),
    });
    result.current.mutate("inbox-1");

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    // No cache write may flip `read` — an unread item must come back unread.
    expect(archivedCache(queryClient).every((i) => i.read === false)).toBe(true);
    expect(invalidate).toHaveBeenCalledWith({
      queryKey: inboxKeys.all(WORKSPACE_ID),
    });
    expect(invalidate).toHaveBeenCalledWith({
      queryKey: inboxKeys.unreadSummary(),
    });
  });

  it("rolls the archived list back when the request fails", async () => {
    unarchiveInbox.mockRejectedValue(new Error("boom"));
    const original = [item({ id: "inbox-1" })];
    queryClient.setQueryData<InboxItem[]>(
      inboxKeys.archived(WORKSPACE_ID),
      original,
    );

    const { result } = renderHook(() => useUnarchiveInbox(), {
      wrapper: createWrapper(queryClient),
    });
    result.current.mutate("inbox-1");

    await waitFor(() => expect(result.current.isError).toBe(true));
    expect(archivedCache(queryClient)).toEqual(original);
  });
});

describe("useArchiveInbox", () => {
  it("archives only one independently actionable decision", async () => {
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false }, mutations: { retry: false } } });
    const archiveInbox = vi.fn(async (id: string) => item({ id, archived: true }));
    setApiInstance({ archiveInbox } as unknown as ApiClient);
    queryClient.setQueryData<InboxItem[]>(inboxKeys.list(WORKSPACE_ID), [
      item({ id: "decision-a", archived: false, type: "workflow_owner_decision" }),
      item({ id: "decision-b", archived: false, type: "workflow_owner_decision" }),
    ]);

    const { result } = renderHook(() => useArchiveInbox(), { wrapper: createWrapper(queryClient) });
    result.current.mutate("decision-a");

    await waitFor(() => {
      const cached = listCache(queryClient);
      expect(cached.find((i) => i.id === "decision-a")?.archived).toBe(true);
      expect(cached.find((i) => i.id === "decision-b")?.archived).toBe(false);
    });
  });
});
