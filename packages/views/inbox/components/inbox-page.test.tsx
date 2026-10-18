import { act, fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { toast } from "sonner";
import { ApiError } from "@multica/core/api";
import type { InboxItem } from "@multica/core/types";
import { useInboxFilterStore } from "@multica/core/inbox/filter-store";
import { InboxPage } from "./inbox-page";

vi.mock("sonner", () => ({
  toast: { success: vi.fn(), error: vi.fn() },
}));

vi.mock("react-resizable-panels", () => ({
  useDefaultLayout: () => ({ defaultLayout: undefined, onLayoutChanged: vi.fn() }),
}));

// The page runs two queries — the active list and the archived one. They are
// told apart by the queryKey their options carry, so each test can stock the
// two lists independently.
const listData: { active: InboxItem[]; archived: InboxItem[]; memory: unknown[] } = {
  active: [],
  archived: [],
  memory: [],
};

vi.mock("@tanstack/react-query", () => ({
  queryOptions: <T,>(options: T) => options,
  useQuery: (options: { queryKey: readonly unknown[] }) => ({
    data: options.queryKey.includes("development-memory")
      ? listData.memory
      : options.queryKey.includes("archived")
        ? listData.archived
        : listData.active,
    isLoading: false,
    isError: false,
  }),
}));

vi.mock("@multica/core/hooks", () => ({
  useWorkspaceId: () => "workspace-1",
}));

vi.mock("@multica/core/paths", () => ({
  useWorkspacePaths: () => ({
    inbox: () => "/acme/inbox",
    issueDetail: (id: string) => `/acme/issues/${id}`,
  }),
}));

const modalState: { modal: string | null; open: ReturnType<typeof vi.fn> } = {
  modal: null,
  open: vi.fn(),
};
vi.mock("@multica/core/modals", () => ({
  useModalStore: { getState: () => modalState },
}));

vi.mock("@multica/core/issues/stores/draft-store", () => ({
  useIssueDraftStore: { getState: () => ({ setDraft: vi.fn() }) },
}));

vi.mock("@multica/core/inbox/queries", () => ({
  inboxListOptions: () => ({ queryKey: ["inbox", "workspace-1", "list"] }),
  archivedInboxListOptions: () => ({ queryKey: ["inbox", "workspace-1", "archived"] }),
  deduplicateInboxItems: (items: InboxItem[]) => items.filter((i) => !i.archived),
  deduplicateArchivedInboxItems: (items: InboxItem[]) => items.filter((i) => i.archived),
  useInboxUnreadCount: () => 2,
}));

// Stable spies: the auto-mark-read effect keys on the mutate identity, so a
// fresh `vi.fn()` per render would make the effect's deps churn.
const markReadMutate = vi.fn();
const markUnreadMutate = vi.fn();
const archiveMutate = vi.fn();
const unarchiveMutate = vi.fn();
const retrySourceContextMutateAsync = vi.fn();
const showIssueLimitUpgradePrompt = vi.hoisted(() => vi.fn());
const showAutopilotQuotaRecoveryPrompt = vi.hoisted(() => vi.fn());

vi.mock("../../modals/use-issue-limit-upgrade-prompt", () => ({
  useIssueLimitUpgradePrompt: (reason?: string) =>
    reason === "autopilot_quota"
      ? showAutopilotQuotaRecoveryPrompt
      : showIssueLimitUpgradePrompt,
}));

vi.mock("@multica/core/inbox/mutations", () => {
  const mutation = () => ({ mutate: vi.fn() });
  return {
    useMarkInboxRead: () => ({ mutate: markReadMutate }),
    useMarkInboxUnread: () => ({ mutate: markUnreadMutate }),
    useArchiveInbox: () => ({ mutate: archiveMutate }),
    useUnarchiveInbox: () => ({ mutate: unarchiveMutate }),
    useMarkAllInboxRead: mutation,
    useArchiveAllInbox: mutation,
    useArchiveAllReadInbox: mutation,
    useArchiveCompletedInbox: mutation,
    useRetrySourceContextQuickCreate: () => ({
      mutateAsync: retrySourceContextMutateAsync,
      isPending: false,
    }),
  };
});

// Render-time capture of the props IssueDetail receives, so tests can assert
// what the page threads into it (highlight replay token) without standing up
// the real detail.
const issueDetailProps = vi.hoisted(
  () => [] as Array<Record<string, unknown>>,
);
vi.mock("../../issues/components", () => ({
  IssueDetail: (props: Record<string, unknown>) => {
    issueDetailProps.push(props);
    return null;
  },
  StatusIcon: () => null,
  issueHighlightMementoKey: (issueId: string) => `highlight:${issueId}`,
}));

vi.mock("../../issues/components/development-memory-section", () => ({
  DevelopmentMemoryReviewList: ({ proposals }: { proposals: unknown[] }) => (
    <div data-testid="memory-review-list">{proposals.length}</div>
  ),
}));

vi.mock("./owner-inbox-panel", () => ({
  OwnerInboxPanel: ({ group }: { group: { lead: InboxItem } }) => (
    <div data-testid="owner-inbox-panel">{group.lead.id}</div>
  ),
}));

// The main inbox is list-beside-detail now, the same shape the archive always
// used: a row per piece of work on the left, selecting one shows it on the
// right. These mocks stand in for the two real components so the tests below
// exercise the page's own wiring (selection, filters, archive, mark-read)
// without standing up the real preview/answer machinery.
type MockGroup = { key: string; lead: InboxItem; items: InboxItem[]; unread: boolean };
type MockBatch = { key: string; shared: string | null; groups: MockGroup[] };

vi.mock("./owner-inbox-list", () => ({
  OwnerInboxList: ({
    batches,
    selectedKey,
    onSelect,
    emptyLabel,
    emptyAction,
  }: {
    batches: MockBatch[];
    selectedKey: string;
    onSelect: (group: MockGroup) => void;
    emptyLabel?: string;
    emptyAction?: React.ReactNode;
  }) => (
    <div data-testid="list" data-view="inbox">
      {batches.map((batch) => (
        <button
          key={batch.key}
          type="button"
          data-testid="row"
          data-selected={batch.groups.some((g) => g.items.some((i) => i.id === selectedKey))}
          onClick={() => onSelect(batch.groups[0]!)}
        >
          {batch.groups[0]!.lead.id}
        </button>
      ))}
      {batches.length === 0 && emptyLabel && <p>{emptyLabel}</p>}
      {batches.length === 0 && emptyAction}
    </div>
  ),
}));

vi.mock("./owner-inbox-detail", () => ({
  OwnerInboxDetail: ({
    group,
    onClear,
    onLater,
    extras,
  }: {
    group: MockGroup;
    onClear: () => void;
    onLater?: () => void;
    extras?: React.ReactNode;
  }) => (
    <div data-testid="owner-inbox-detail">
      <span data-testid="detail-row">{group.lead.id}</span>
      <button type="button" data-testid="row-clear" onClick={onClear}>
        clear
      </button>
      {group.unread && onLater && (
        <button type="button" data-testid="row-later" onClick={onLater}>
          later
        </button>
      )}
      {!group.lead.issue_id && extras}
    </div>
  ),
  OwnerInboxBatchDetail: ({ batch }: { batch: MockBatch }) => (
    <div data-testid="owner-inbox-batch-detail">
      {batch.groups.map((g) => (
        <span key={g.key} data-testid="detail-row">
          {g.lead.id}
        </span>
      ))}
    </div>
  ),
}));

const replace = vi.fn();
let searchParams = new URLSearchParams();

vi.mock("../../navigation", () => ({
  useNavigation: () => ({ searchParams, replace }),
  // Real hook: reports the detail-pane swap to the shell's progress bar.
  // Nothing here renders that bar, and the page reads nothing back from it.
  useReportNavigating: () => {},
}));

// Drive the layout from a viewport width so a test can name the device it
// cares about instead of a boolean, and the two breakpoints stay in one place.
// Phone-width by default — one column keeps most assertions simple. Tests
// about actioning a row WHILE it is open need the two-panel desktop layout,
// since a single column swaps the list out for the detail on selection.
const PHONE = 390;
const FOLD_INNER = 851;
const TABLET = 1024;
const DESKTOP = 1440;
const layout = { width: PHONE };
vi.mock("@multica/ui/hooks/use-mobile", () => ({
  useIsMobile: () => layout.width < 768,
  useIsCompact: () => layout.width < 1024,
}));
vi.mock("@multica/ui/components/ui/resizable", () => ({
  ResizablePanelGroup: ({ children }: { children: React.ReactNode }) => (
    <div>{children}</div>
  ),
  ResizablePanel: ({ children }: { children: React.ReactNode }) => (
    <div>{children}</div>
  ),
  ResizableHandle: () => null,
}));
vi.mock("./inbox-list", () => ({
  InboxList: ({
    items,
    view,
    onSelect,
    emptyLabel,
    emptyAction,
  }: {
    items: InboxItem[];
    view: string;
    onSelect: (item: InboxItem) => void;
    emptyLabel?: string;
    emptyAction?: React.ReactNode;
  }) => (
    <div data-testid="list" data-view={view}>
      {items.map((i) => (
        <button key={i.id} data-testid="row" onClick={() => onSelect(i)}>
          {i.id}
        </button>
      ))}
      {items.length === 0 && emptyLabel && <p>{emptyLabel}</p>}
      {items.length === 0 && emptyAction}
    </div>
  ),
}));
vi.mock("./inbox-filter-menu", () => ({
  InboxFilterMenu: () => <button type="button">Filter inbox</button>,
}));
vi.mock("./inbox-list-item", () => ({ useTimeAgo: () => vi.fn() }));

// Capture the row actions the page hands the context menu, so the read/unread
// handlers can be driven without standing up Base UI's menu.
let rowActions: {
  onMarkRead: (id: string) => void;
  onMarkUnread: (id: string) => void;
  onAction: (id: string) => void;
} | null = null;
vi.mock("./inbox-context-menu", () => ({
  InboxContextMenuProvider: ({
    actions,
    children,
  }: {
    actions: NonNullable<typeof rowActions>;
    children: React.ReactNode;
  }) => {
    rowActions = actions;
    return children;
  },
}));
vi.mock("./inbox-detail-label", () => ({ useTypeLabels: () => ({}) }));
vi.mock("./autopilot-quota-notice", () => ({
  AutopilotQuotaNotice: ({
    onOpenRecovery,
  }: {
    onOpenRecovery: () => void;
  }) => (
    <button
      type="button"
      data-testid="autopilot-quota-recovery"
      onClick={onOpenRecovery}
    >
      Recover
    </button>
  ),
}));
vi.mock("../../i18n", () => ({ useT: () => ({ t: () => "Inbox" }) }));

function item(overrides: Partial<InboxItem> = {}): InboxItem {
  return {
    id: "inbox-1",
    workspace_id: "workspace-1",
    recipient_type: "member",
    recipient_id: "member-1",
    actor_type: "agent",
    actor_id: "agent-1",
    type: "workflow_owner_decision",
    severity: "action_required",
    issue_id: "issue-1",
    title: "Issue title",
    body: null,
    issue_status: null,
    issue_priority: null,
    read: true,
    archived: false,
    created_at: "2026-06-15T08:00:00Z",
    details: { owner_relevant: "true", question: "What should the team do next?" },
    ...overrides,
  };
}

function reset() {
  listData.active = [];
  listData.archived = [];
  listData.memory = [];
  searchParams = new URLSearchParams();
  replace.mockClear();
  markReadMutate.mockClear();
  markUnreadMutate.mockClear();
  archiveMutate.mockClear();
  unarchiveMutate.mockClear();
  retrySourceContextMutateAsync.mockReset();
  retrySourceContextMutateAsync.mockResolvedValue({});
  showIssueLimitUpgradePrompt.mockClear();
  showAutopilotQuotaRecoveryPrompt.mockClear();
  modalState.modal = null;
  vi.mocked(toast.success).mockClear();
  vi.mocked(toast.error).mockClear();
  rowActions = null;
  issueDetailProps.length = 0;
  layout.width = PHONE;
  useInboxFilterStore.setState({ filtersByWorkspace: {} });
}

describe("InboxPage", () => {
  it("puts pending memory decisions in the owner attention queue", () => {
    reset();
    listData.memory = [{ id: "memory-1" }];

    render(<InboxPage />);

    expect(screen.getByTestId("memory-review-list")).toHaveTextContent("1");
  });

  // The number beside the title used to be the server's unread EVENT count. It
  // read 7 on a page showing five questions and one group of six, so nothing on
  // screen added up to it. It counts what the page is actually showing: the
  // pieces of work waiting on an answer.
  it("counts the pieces of work waiting on the owner, not the events", () => {
    reset();
    listData.active = [
      item({
        id: "ask-1",
        issue_id: "issue-a",
        type: "workflow_owner_decision",
        details: { owner_relevant: "true", question: "Approve the layout?" },
      }),
      // Same piece of work, two more events. One thing to answer, not three.
      item({
        id: "crash-1",
        issue_id: "issue-a",
        type: "task_failed",
        details: { owner_relevant: "true", error: "exit status 0xc0000017" },
      }),
      item({
        id: "crash-2",
        issue_id: "issue-a",
        type: "task_failed",
        details: { owner_relevant: "true", error: "exit status 0xc0000017" },
      }),
      item({
        id: "ask-2",
        issue_id: "issue-b",
        type: "workflow_owner_decision",
        details: { owner_relevant: "true", question: "Approve the copy?" },
      }),
      // Nothing to answer here, so it is not part of the count.
      item({ id: "done", issue_id: "issue-c", type: "task_completed" }),
    ];

    const { container } = render(<InboxPage />);
    const titleCount = container.querySelector("h1")?.parentElement?.querySelector(
      "number-flow-react",
    ) as (HTMLElement & { animated?: boolean }) | null;

    expect(titleCount?.getAttribute("aria-label")).toBe("2");
    expect(titleCount?.animated).toBe(false);
  });

  it("shows the active list by default", () => {
    reset();
    listData.active = [item({ id: "active-1" })];
    listData.archived = [item({ id: "archived-1", archived: true })];

    render(<InboxPage />);

    expect(screen.getByTestId("list").dataset.view).toBe("inbox");
    expect(screen.getByTestId("row").textContent).toBe("active-1");
  });

  // The owner used to get one row per event, so a single piece of work that
  // had asked a question and then crashed twice filled three rows with the
  // same title — and four coloured counters above them asked him to sort his
  // own inbox before he could start. One row per piece of work, led by the
  // event that actually wants him.
  it("shows one row per piece of work, led by what waits on the owner", () => {
    reset();
    listData.active = [
      item({
        id: "crash-1",
        issue_id: "issue-a",
        type: "task_failed",
        created_at: "2026-09-21T12:05:00Z",
        details: { owner_relevant: "true", error: "exit status 0xc0000017" },
      }),
      item({
        id: "question",
        issue_id: "issue-a",
        type: "workflow_owner_decision",
        created_at: "2026-09-21T12:00:00Z",
        details: { owner_relevant: "true", owner_category: "needs_you", question: "Approve?" },
      }),
      item({
        id: "crash-2",
        issue_id: "issue-a",
        type: "task_failed",
        created_at: "2026-09-21T12:02:00Z",
        details: { owner_relevant: "true", error: "exit status 0xc0000017" },
      }),
      item({
        id: "other",
        issue_id: "issue-b",
        type: "workflow_owner_decision",
        created_at: "2026-09-21T11:00:00Z",
        details: { owner_relevant: "true", owner_category: "needs_you", question: "Ship it?" },
      }),
    ];

    render(<InboxPage />);

    const rows = screen.getAllByTestId("row");
    expect(rows).toHaveLength(2);
    // The newer crashes must not bury the question — the question is what
    // stopped the work.
    expect(rows[0]).toHaveTextContent("question");
    expect(screen.queryByTestId("owner-category-incident")).toBeNull();
  });

  // A crash is owned by the system and its investigation path. It remains in
  // issue/Chat history, but never becomes a chore the owner has to restart or
  // clear from the action Inbox.
  it("does not put failed runs in the owner action queue", () => {
    reset();
    listData.active = [
      item({
        id: "crash-a",
        issue_id: "issue-a",
        type: "task_failed",
        details: { owner_relevant: "true", error: "exit status 0xc0000017" },
      }),
      item({
        id: "crash-b",
        issue_id: "issue-b",
        type: "task_failed",
        details: { owner_relevant: "true", error: "exit status 0xc0000017" },
      }),
      item({
        id: "crash-c",
        issue_id: "issue-c",
        type: "task_failed",
        details: { owner_relevant: "true", error: "exit status 0xc0000017" },
      }),
    ];

    render(<InboxPage />);

    expect(screen.queryByTestId("row")).toBeNull();
  });

  it("keeps a routine outside-agent completion report out of Needs you", () => {
    reset();
    listData.active = [
      item({
        id: "outside-complete",
        issue_id: "issue-outside",
        type: "external_work_update",
        severity: "info",
        body: "The importer is merged and its checks passed.",
        details: {
          owner_relevant: "true",
          provider: "codex",
          reported_state: "complete",
          provider_report_only: "true",
        },
      }),
    ];

    render(<InboxPage />);

    expect(screen.queryByTestId("row")).toBeNull();
  });

  it("shows an outside-agent question that genuinely needs the owner", () => {
    reset();
    listData.active = [
      item({
        id: "outside-question",
        issue_id: "issue-outside",
        type: "external_work_update",
        severity: "action_required",
        body: "The worker needs a product choice.",
        details: {
          owner_relevant: "true",
          provider: "codex",
          reported_state: "waiting_owner",
          question: "Should this replace the existing workflow?",
          provider_report_only: "true",
        },
      }),
    ];

    render(<InboxPage />);

    expect(screen.getByTestId("row")).toHaveTextContent("outside-question");
  });

  it("never merges two decisions, even asking the exact same words", () => {
    reset();
    listData.active = [
      item({
        id: "ask-a",
        issue_id: "issue-a",
        type: "workflow_owner_decision",
        details: { owner_relevant: "true", question: "Approve the layout?" },
      }),
      item({
        id: "ask-b",
        issue_id: "issue-b",
        type: "workflow_owner_decision",
        details: { owner_relevant: "true", question: "Approve the layout?" },
      }),
    ];

    render(<InboxPage />);

    expect(screen.getAllByTestId("row")).toHaveLength(2);
  });

  it("filters the list by status and priority together", () => {
    reset();
    listData.active = [
      item({
        id: "todo-high",
        issue_id: "issue-1",
        issue_status: "todo",
        issue_priority: "high",
      }),
      item({
        id: "done-low",
        issue_id: "issue-2",
        issue_status: "done",
        issue_priority: "low",
      }),
      item({ id: "system", issue_id: null }),
    ];
    const filters = useInboxFilterStore.getState();
    filters.toggleStatusFilter("workspace-1", "done");
    filters.togglePriorityFilter("workspace-1", "low");

    render(<InboxPage />);

    expect(screen.getAllByTestId("row")).toHaveLength(1);
    expect(screen.getByTestId("row")).toHaveTextContent("done-low");
  });

  it("hides read notifications while the unread filter is on", () => {
    reset();
    listData.active = [
      item({ id: "unread-row", issue_id: "issue-1", read: false }),
      item({ id: "read-row", issue_id: "issue-2", read: true }),
    ];
    useInboxFilterStore.getState().toggleUnreadOnly("workspace-1");

    render(<InboxPage />);

    expect(screen.getAllByTestId("row")).toHaveLength(1);
    expect(screen.getByTestId("row")).toHaveTextContent("unread-row");
  });

  it("filters by the actor the row carries", () => {
    reset();
    listData.active = [
      item({ id: "from-alice", issue_id: "issue-1", actor_type: "member", actor_id: "alice" }),
      item({ id: "from-bob", issue_id: "issue-2", actor_type: "agent", actor_id: "bob" }),
    ];
    useInboxFilterStore.getState().toggleActorFilter("workspace-1", "member:alice");

    render(<InboxPage />);

    expect(screen.getAllByTestId("row")).toHaveLength(1);
    expect(screen.getByTestId("row")).toHaveTextContent("from-alice");
  });

  it("offers to clear filters when they hide every notification", () => {
    reset();
    listData.active = [
      item({
        id: "todo-high",
        issue_status: "todo",
        issue_priority: "high",
      }),
    ];
    useInboxFilterStore
      .getState()
      .togglePriorityFilter("workspace-1", "urgent");

    render(<InboxPage />);

    expect(screen.queryByTestId("row")).toBeNull();
    fireEvent.click(screen.getByRole("button", { name: "Inbox" }));
    expect(screen.getByTestId("row")).toHaveTextContent("todo-high");
  });

  it("ignores a priority filter when a legacy response omits the projection", () => {
    reset();
    const legacyItem = item({
      id: "legacy-todo",
      issue_status: "todo",
    });
    delete legacyItem.issue_priority;
    listData.active = [legacyItem];
    useInboxFilterStore
      .getState()
      .togglePriorityFilter("workspace-1", "urgent");

    render(<InboxPage />);

    expect(screen.getByTestId("row")).toHaveTextContent("legacy-todo");
  });

  it("renders the archived list when the URL asks for it", () => {
    // ?view=archived is what makes a refresh, a back/forward step, or a mobile
    // detail-back land in the archive instead of the main inbox.
    reset();
    searchParams = new URLSearchParams("view=archived");
    listData.active = [item({ id: "active-1" })];
    listData.archived = [item({ id: "archived-1", archived: true })];

    render(<InboxPage />);

    expect(screen.getByTestId("list").dataset.view).toBe("archived");
    expect(screen.getByTestId("row").textContent).toBe("archived-1");
  });

  it("hides the batch-actions menu in the archived view", () => {
    // Every batch action archives from the MAIN inbox; offering them over the
    // archived list would read as "archive all of these" and do the opposite.
    reset();
    listData.archived = [item({ id: "archived-1", archived: true })];
    const { container: mainView } = render(<InboxPage />);
    expect(mainView.querySelector('[aria-haspopup="menu"]')).not.toBeNull();

    searchParams = new URLSearchParams("view=archived");
    const { container: archivedView } = render(<InboxPage />);
    expect(archivedView.querySelector('[aria-haspopup="menu"]')).toBeNull();
  });

  it("falls back to the main inbox when the archive drains", () => {
    // Restoring the last archived item must not strand the user on an empty
    // archive — same fallback chat's archived view has.
    reset();
    searchParams = new URLSearchParams("view=archived");
    listData.archived = [];

    render(<InboxPage />);

    expect(replace).toHaveBeenCalledWith("/acme/inbox");
  });

  // Answering "Not now" from the detail pane reads the group it is already
  // showing — it must not itself write a new selection to the URL.
  it("does not navigate when a card is acted on", () => {
    reset();
    layout.width = DESKTOP;
    listData.active = [item({ id: "active-1", issue_id: "issue-3", read: false })];

    render(<InboxPage />);
    fireEvent.click(screen.getByTestId("row"));
    replace.mockClear();
    fireEvent.click(screen.getByTestId("row-later"));

    expect(replace).not.toHaveBeenCalled();
  });

  it("keeps the archived view in the URL when selecting an item there", () => {
    // A bare `?issue=` write would silently drop the user back to the main
    // inbox on the next refresh — both pieces of state travel together.
    reset();
    searchParams = new URLSearchParams("view=archived");
    listData.archived = [
      item({ id: "archived-1", issue_id: "issue-9", archived: true }),
    ];

    render(<InboxPage />);
    fireEvent.click(screen.getByTestId("row"));

    expect(replace).toHaveBeenCalledWith("/acme/inbox?view=archived&item=archived-1");
  });

  it("clears every event of a piece of work, not just the one on show", () => {
    // Archiving only the lead would leave its crashed runs behind and the work
    // would reappear at the top of the list looking untouched.
    reset();
    listData.active = [
      item({ id: "ask", issue_id: "issue-3", type: "workflow_owner_decision",
             details: { owner_relevant: "true", question: "Approve?" } }),
      item({ id: "crash", issue_id: "issue-3", type: "task_failed",
             details: { owner_relevant: "true", error: "exit status 0xc0000017" } }),
    ];

    render(<InboxPage />);
    fireEvent.click(screen.getByTestId("row"));
    fireEvent.click(screen.getByTestId("row-clear"));

    expect(archiveMutate).toHaveBeenCalledWith("ask", expect.anything());
    expect(archiveMutate).toHaveBeenCalledWith("crash", expect.anything());
  });

  // `InboxItem.issue_id` is nullable: a quick-create outcome is a notification,
  // not an issue, so `IssueDetail` never renders for it — and `IssueDetail` is
  // what carries the way back in its own header on a phone. This branch has to
  // supply its own bar or opening one of these is a dead end.
  it("keeps a notice with no issue on its card, with its own repair control", () => {
    // A notice's repair control now lives in the detail pane it is opened
    // into, the same place every other piece of work answers.
    reset();
    listData.active = [
      item({ id: "inbox-note", issue_id: null, type: "quick_create_failed",
             details: { owner_relevant: "true", source_context_id: "src-1", task_id: "task-1" } }),
    ];

    render(<InboxPage />);

    expect(screen.getByTestId("row")).toBeInTheDocument();
    fireEvent.click(screen.getByTestId("row"));
    expect(screen.getByTestId("retry-source-context")).toBeInTheDocument();
  });

  it("retries a failed quick-create with its original source context", async () => {
    reset();
    listData.active = [
      item({
        id: "source-context-failure",
        issue_id: null,
        type: "quick_create_failed",
        details: {
          task_id: "task-1",
          source_context_id: "context-1",
          original_prompt: "make a child",
        },
      }),
    ];

    render(<InboxPage />);
    fireEvent.click(screen.getByTestId("row"));
    fireEvent.click(screen.getByTestId("retry-source-context"));

    await act(async () => undefined);
    expect(retrySourceContextMutateAsync).toHaveBeenCalledWith("task-1");
    expect(toast.success).toHaveBeenCalledTimes(1);
  });

  it("opens quota-specific recovery from an autopilot quota notice", () => {
    reset();
    listData.active = [
      item({
        id: "autopilot-quota",
        issue_id: null,
        type: "autopilot_quota_exceeded",
      }),
    ];

    render(<InboxPage />);
    fireEvent.click(screen.getByTestId("row"));
    fireEvent.click(screen.getByTestId("autopilot-quota-recovery"));

    expect(showAutopilotQuotaRecoveryPrompt).toHaveBeenCalledTimes(1);
    expect(showIssueLimitUpgradePrompt).not.toHaveBeenCalled();
  });

  it("shows issue-limit recovery when a source-context retry is rejected", async () => {
    reset();
    retrySourceContextMutateAsync.mockRejectedValue(
      new ApiError(
        "workspace has reached its issue limit",
        402,
        "Payment Required",
        { code: "issue_limit_reached" },
      ),
    );
    listData.active = [
      item({
        id: "source-context-limit",
        issue_id: null,
        type: "quick_create_failed",
        details: {
          task_id: "task-1",
          source_context_id: "context-1",
          original_prompt: "make a child",
        },
      }),
    ];

    render(<InboxPage />);
    fireEvent.click(screen.getByTestId("row"));
    fireEvent.click(screen.getByTestId("retry-source-context"));

    await act(async () => undefined);
    expect(showIssueLimitUpgradePrompt).toHaveBeenCalledTimes(1);
    expect(showAutopilotQuotaRecoveryPrompt).not.toHaveBeenCalled();
    expect(toast.error).not.toHaveBeenCalled();
  });

  it("marks a piece of work read only when the owner puts it off", () => {
    // Nothing marks itself read on sight. Unread means "not yet dealt with",
    // and a card that flipped on scroll would make both the sidebar count and
    // the Not-now button meaningless.
    reset();
    listData.active = [item({ id: "inbox-a", issue_id: "issue-a", read: false })];

    render(<InboxPage />);
    expect(markReadMutate).not.toHaveBeenCalled();

    fireEvent.click(screen.getByTestId("row"));
    fireEvent.click(screen.getByTestId("row-later"));
    expect(markReadMutate).toHaveBeenCalledWith("inbox-a", expect.anything());
  });

  it("puts off every event of the piece of work together", () => {
    reset();
    listData.active = [
      item({ id: "ask", issue_id: "issue-a", read: false, type: "workflow_owner_decision",
             details: { owner_relevant: "true", question: "Approve?" } }),
      item({ id: "crash", issue_id: "issue-a", read: false, type: "task_failed",
             details: { owner_relevant: "true", error: "exit status 0xc0000017" } }),
    ];

    render(<InboxPage />);
    fireEvent.click(screen.getByTestId("row"));
    fireEvent.click(screen.getByTestId("row-later"));

    expect(markReadMutate).toHaveBeenCalledWith("ask", expect.anything());
    expect(markReadMutate).toHaveBeenCalledWith("crash", expect.anything());
  });

  it("leaves an already-read piece of work alone", () => {
    // The detail pane only offers "Not now" while something is genuinely
    // unread — for an already-read group there is nothing to put off, so the
    // control is absent rather than a no-op button.
    reset();
    listData.active = [item({ id: "inbox-a", issue_id: "issue-a", read: true })];

    render(<InboxPage />);
    fireEvent.click(screen.getByTestId("row"));

    expect(screen.queryByTestId("row-later")).toBeNull();
    expect(markReadMutate).not.toHaveBeenCalled();
  });

  it("stays one column on a folded inner screen", () => {
    // 851px — the reported Pixel Fold inner screen. The owner's inbox is one
    // column at every width now, so nothing swaps out from under the reader.
    reset();
    layout.width = FOLD_INNER;
    listData.active = [item({ id: "inbox-a", issue_id: "issue-a" })];

    render(<InboxPage />);

    expect(screen.getByTestId("list")).toBeInTheDocument();
    expect(screen.getAllByTestId("row")).toHaveLength(1);
  });

  it("keeps both panes at the compact breakpoint", () => {
    // 1024px is the first width that keeps the two-pane layout. The nav
    // auto-collapses there instead (see the sidebar), so the list has to stay
    // on screen next to an open item.
    reset();
    layout.width = TABLET;
    listData.active = [item({ id: "inbox-a", issue_id: "issue-a" })];

    render(<InboxPage />);
    fireEvent.click(screen.getByTestId("row"));

    expect(screen.queryByTestId("list")).not.toBeNull();
  });

  // Selection, the row menu and the archive shortcut belong to the archived
  // view now: the owner's inbox answers on the card and never opens anything.
  function renderWithActiveItem() {
    reset();
    layout.width = DESKTOP;
    searchParams = new URLSearchParams("view=archived");
    listData.archived = [item({ id: "inbox-a", issue_id: "issue-a", archived: true })];
    return render(<InboxPage />);
  }

  function renderWithOpenItem() {
    const view = renderWithActiveItem();
    fireEvent.click(screen.getByTestId("row"));
    return view;
  }

  describe("archive shortcut", () => {
    function pressArchiveKey(target: Element | Document = document) {
      fireEvent.keyDown(target, { key: "e" });
    }

    function typeArchiveKeyInto(element: Element) {
      document.body.appendChild(element);
      pressArchiveKey(element);
      element.remove();
    }

    it("restores the open record from the archive", () => {
      renderWithOpenItem();
      pressArchiveKey();

      expect(unarchiveMutate).toHaveBeenCalledWith("inbox-a", expect.anything());
    });

    it("restores the open notification while reading the archive", () => {
      reset();
      layout.width = DESKTOP;
      searchParams = new URLSearchParams("view=archived");
      listData.archived = [
        item({ id: "archived-1", issue_id: "issue-9", archived: true }),
      ];

      render(<InboxPage />);
      fireEvent.click(screen.getByTestId("row"));
      pressArchiveKey();

      expect(unarchiveMutate).toHaveBeenCalledWith("archived-1", expect.anything());
      expect(archiveMutate).not.toHaveBeenCalled();
    });

    it("leaves the selection on the next record", () => {
      reset();
      layout.width = DESKTOP;
      searchParams = new URLSearchParams("view=archived");
      listData.archived = [
        item({ id: "inbox-a", issue_id: "issue-a", archived: true }),
        item({ id: "inbox-b", issue_id: "issue-b", archived: true }),
      ];

      render(<InboxPage />);
      fireEvent.click(screen.getAllByTestId("row")[0]!);
      replace.mockClear();
      pressArchiveKey();

      expect(replace).toHaveBeenCalledWith("/acme/inbox?view=archived&item=inbox-b");
    });

    it("does not fire while typing in an editable control", () => {
      renderWithOpenItem();

      typeArchiveKeyInto(document.createElement("input"));
      typeArchiveKeyInto(document.createElement("textarea"));
      const richText = document.createElement("div");
      richText.setAttribute("contenteditable", "true");
      typeArchiveKeyInto(richText);

      expect(archiveMutate).not.toHaveBeenCalled();
    });

    it("stands down while a dialog is open", () => {
      renderWithOpenItem();
      modalState.modal = "create-issue";
      pressArchiveKey();

      expect(archiveMutate).not.toHaveBeenCalled();
    });

    // Keypresses in portaled popups still reach the page listener, where `e`
    // is typeahead, not archive.
    it.each([
      ["menu", "menuitem"],
      ["dialog", "button"],
      ["listbox", "option"],
    ])("stands down while a portaled %s owns the keyboard", (layerRole, itemRole) => {
      renderWithOpenItem();

      const layer = document.createElement("div");
      layer.setAttribute("role", layerRole);
      const focused = document.createElement("div");
      focused.setAttribute("role", itemRole);
      layer.appendChild(focused);
      document.body.appendChild(layer);
      pressArchiveKey(focused);
      layer.remove();

      expect(archiveMutate).not.toHaveBeenCalled();
    });

    it("stands down while a modal layer holds the page inert", () => {
      // Base UI marks everything outside a modal popup `data-base-ui-inert`,
      // even when focus never left the page.
      const { container } = renderWithOpenItem();
      container.firstElementChild?.setAttribute("data-base-ui-inert", "");
      pressArchiveKey();

      expect(archiveMutate).not.toHaveBeenCalled();
    });

    it("ignores an auto-repeated key", () => {
      renderWithOpenItem();
      fireEvent.keyDown(document, { key: "e", repeat: true });

      expect(archiveMutate).not.toHaveBeenCalled();
    });

    it("ignores a press a nearer handler already consumed", () => {
      renderWithOpenItem();

      const consumer = document.createElement("button");
      consumer.addEventListener("keydown", (event) => event.preventDefault());
      document.body.appendChild(consumer);
      pressArchiveKey(consumer);
      consumer.remove();

      expect(archiveMutate).not.toHaveBeenCalled();
    });

    it("does nothing when no notification is open", () => {
      renderWithActiveItem();
      pressArchiveKey();

      expect(archiveMutate).not.toHaveBeenCalled();
    });
  });

  // Toasts live in the shared handlers, so every archive surface reports alike.
  it("confirms clearing a card from the owner's inbox", () => {
    reset();
    layout.width = DESKTOP;
    listData.active = [item({ id: "inbox-a", issue_id: "issue-a" })];

    render(<InboxPage />);
    fireEvent.click(screen.getByTestId("row"));
    fireEvent.click(screen.getByTestId("row-clear"));

    expect(archiveMutate).toHaveBeenCalledWith("inbox-a", expect.anything());
  });

  describe("archive feedback", () => {
    type MutateOptions = {
      onSuccess?: () => void;
      onError?: (err: unknown) => void;
    };

    function settle(
      spy: typeof archiveMutate,
      outcome: "success" | "error",
      err: unknown = new Error("boom"),
    ) {
      const options = spy.mock.calls.at(-1)?.[1] as MutateOptions | undefined;
      act(() => {
        if (outcome === "success") options?.onSuccess?.();
        else options?.onError?.(err);
      });
    }

    it("confirms a restore from the row action", () => {
      renderWithActiveItem();
      act(() => rowActions?.onAction("inbox-a"));
      settle(unarchiveMutate, "success");

      expect(toast.success).toHaveBeenCalledTimes(1);
    });

    it("confirms a restore driven by the shortcut", () => {
      renderWithOpenItem();
      fireEvent.keyDown(document, { key: "e" });
      settle(unarchiveMutate, "success");

      expect(toast.success).toHaveBeenCalledTimes(1);
    });

    it("confirms a restore", () => {
      reset();
      layout.width = DESKTOP;
      searchParams = new URLSearchParams("view=archived");
      listData.archived = [
        item({ id: "archived-1", issue_id: "issue-9", archived: true }),
      ];

      render(<InboxPage />);
      act(() => rowActions?.onAction("archived-1"));
      settle(unarchiveMutate, "success");

      expect(toast.success).toHaveBeenCalledTimes(1);
    });

    it("reports a failed restore as a failure only", () => {
      renderWithActiveItem();
      act(() => rowActions?.onAction("inbox-a"));
      settle(unarchiveMutate, "error");

      expect(toast.error).toHaveBeenCalledTimes(1);
      expect(toast.success).not.toHaveBeenCalled();
    });
  });

  it("does not swallow a deep link to an issue that is not in the archive", () => {
    // ?view=archived&issue=X with an empty archive: the drain effect and the
    // unresolved-selection fallback both want to navigate. The fallback must
    // win, or the deep link silently lands on an empty inbox instead of X.
    reset();
    searchParams = new URLSearchParams("view=archived&issue=issue-404");
    listData.archived = [];

    render(<InboxPage />);

    expect(replace).toHaveBeenCalledWith("/acme/issues/issue-404");
  });
});
