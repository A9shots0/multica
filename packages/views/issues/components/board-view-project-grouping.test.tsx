/**
 * @vitest-environment jsdom
 *
 * Board grouped by project. The server's group descriptors carry a project id
 * and nothing else, so both the column header text and the column's leading
 * glyph are resolved on the client — and `projectId` sits next to `assigneeId`
 * on BoardColumnGroup, where an unguarded fallthrough renders a project column
 * as the "no assignee" column.
 */
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, screen } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ViewStoreProvider } from "@multica/core/issues/stores/view-store-context";
import { getIssueSurfaceViewStore } from "@multica/core/issues/stores/surface-view-store";
import type { Issue, Project } from "@multica/core/types";
import type { IssueGrouping } from "@multica/core/issues/stores/view-store";
import { renderWithI18n } from "../../test/i18n";
import { IssueContextMenuProvider } from "../actions/issue-actions-context-menu";
import type { IssueGroupBranches } from "../surface/use-issue-group-branches";
import { BoardView } from "./board-view";

vi.mock("@multica/core/hooks", () => ({ useWorkspaceId: () => "ws-1" }));

vi.mock("@multica/core/properties", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@multica/core/properties")>()),
  propertyListOptions: () => ({
    queryKey: ["properties"],
    queryFn: async () => [],
  }),
  useSetIssueProperty: () => ({ mutate: () => {} }),
  useUnsetIssueProperty: () => ({ mutate: () => {} }),
}));

vi.mock("@multica/core/workspace/hooks", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@multica/core/workspace/hooks")>()),
  useActorName: () => ({ getActorName: () => "Someone" }),
}));

vi.mock("@multica/core/auth", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@multica/core/auth")>()),
  useAuthStore: (selector: (state: { user: { id: string } }) => unknown) =>
    selector({ user: { id: "viewer-1" } }),
}));

vi.mock("@multica/core/agents", async (importOriginal) => ({
  ...(await importOriginal<typeof import("@multica/core/agents")>()),
  isAgentRuntimeBound: () => true,
  useAgentPresenceDetail: () => ({ availability: "offline", workload: null }),
}));

vi.mock("@multica/core/paths", async (importOriginal) => {
  const actual =
    await importOriginal<typeof import("@multica/core/paths")>();
  return {
    ...actual,
    useCurrentWorkspace: () => ({ id: "ws-1", slug: "acme" }),
    useWorkspacePaths: () => actual.paths.workspace("acme"),
  };
});

vi.mock("../../navigation", () => ({
  AppLink: ({ children, newTabTitle: _newTabTitle, ...props }: React.ComponentProps<"a"> & { newTabTitle?: string }) => (
    <a {...props}>{children}</a>
  ),
  useNavigation: () => ({
    push: () => {},
    openInNewTab: () => {},
    getShareableUrl: (path: string) => `https://app.example${path}`,
    pathname: "/",
  }),
  resolveClickIntent: () => "push",
  useIntentNavigate: () => () => {},
}));

const ACME_ID = "11111111-1111-4111-8111-111111111111";
const GHOST_ID = "22222222-2222-4222-8222-222222222222";

function makeProject(id: string, title: string, icon: string | null): Project {
  return {
    id,
    workspace_id: "ws-1",
    title,
    description: null,
    icon,
    status: "in_progress",
    priority: "none",
    lead_type: null,
    lead_id: null,
    start_date: null,
    due_date: null,
    created_at: "2026-01-01T00:00:00Z",
    updated_at: "2026-01-01T00:00:00Z",
    issue_count: 0,
    done_count: 0,
    resource_count: 0,
  };
}

function makeIssue(id: string, projectId: string | null): Issue {
  return {
    id,
    workspace_id: "ws-1",
    number: 1,
    identifier: `MUL-${id}`,
    title: `Task ${id}`,
    description: null,
    status: "todo",
    priority: "none",
    assignee_type: null,
    assignee_id: null,
    creator_type: "member",
    creator_id: "member-1",
    parent_issue_id: null,
    project_id: projectId,
    position: 1,
    stage: null,
    start_date: null,
    due_date: null,
    labels: [],
    metadata: {},
    properties: {},
    created_at: "2026-01-01T00:00:00Z",
    updated_at: "2026-01-01T00:00:00Z",
  };
}

const ISSUES = [
  makeIssue("acme", ACME_ID),
  makeIssue("ghost", GHOST_ID),
  makeIssue("loose", null),
];

const NO_PROJECT_DESCRIPTOR = {
  key: "project:none",
  value: { kind: "project" as const, project_id: null },
  count: 1,
};
const PROJECT_DESCRIPTORS = [
  {
    key: `project:${ACME_ID}`,
    value: { kind: "project" as const, project_id: ACME_ID },
    count: 4,
  },
  {
    key: `project:${GHOST_ID}`,
    value: { kind: "project" as const, project_id: GHOST_ID },
    count: 1,
  },
];

/** What `useIssueGroupBranches` hands the board for `{ kind: "project" }`. */
function makeGroupBranches(
  descriptors: IssueGroupBranches["descriptors"],
  issues: Issue[],
): IssueGroupBranches {
  return {
    enabled: true,
    descriptors,
    issues,
    pagination: {},
    total: issues.length,
    isLoading: false,
    isRefreshing: false,
    isError: false,
    hasMoreGroups: false,
    isLoadingMoreGroups: false,
    loadMoreGroups: () => {},
    retryGroups: () => {},
  };
}

class ObserverStub {
  observe() {}
  unobserve() {}
  disconnect() {}
  takeRecords() {
    return [];
  }
}

describe("Board grouped by project", () => {
  let queryClient: QueryClient;

  beforeEach(() => {
    vi.stubGlobal("IntersectionObserver", ObserverStub);
    vi.stubGlobal("ResizeObserver", ObserverStub);
    queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false, staleTime: Infinity } },
    });
  });

  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
  });

  function render({
    descriptors = [NO_PROJECT_DESCRIPTOR, ...PROJECT_DESCRIPTORS],
    issues = ISSUES,
    grouping = "project",
  }: {
    descriptors?: IssueGroupBranches["descriptors"];
    issues?: Issue[];
    grouping?: IssueGrouping;
  } = {}) {
    const store = getIssueSurfaceViewStore(
      `board-project-${Math.floor(Math.random() * 1e9)}`,
    );
    store.getState().setGrouping(grouping);
    // Cards carry their own project chip by default, which would put the same
    // title in two places and make "the column header says X" unprovable.
    if (store.getState().cardProperties.project) {
      store.getState().toggleCardProperty("project");
    }
    renderWithI18n(
      <QueryClientProvider client={queryClient}>
        <ViewStoreProvider store={store}>
          <IssueContextMenuProvider>
          <BoardView
            issues={issues}
            visibleStatuses={["todo"]}
            hiddenStatuses={[]}
            onMoveIssue={() => {}}
            projectMap={
              new Map([[ACME_ID, makeProject(ACME_ID, "Acme Corp", "🚀")]])
            }
            groupBranches={makeGroupBranches(descriptors, issues)}
          />
          </IssueContextMenuProvider>
        </ViewStoreProvider>
      </QueryClientProvider>,
    );
  }

  it.each<IssueGrouping>(["status", "assignee", "project", "property:customer"])(
    "keeps the owner-summary Board authoritative when legacy grouping is %s",
    (grouping) => {
      render({ grouping });

      const board = screen.getByRole("region", { name: "Operational workflow board" });
      expect(screen.getByText("Active work")).toBeTruthy();
      // Todo fixtures with no run history have an explicit not-started state.
      expect(screen.getAllByText("Not started yet")).toHaveLength(3);
      expect(board.querySelector("[aria-roledescription=sortable]")).toBeNull();
      expect(board.querySelector("[data-board-card]")).toBeNull();
      expect(board.querySelector("img")).toBeNull();
      expect(screen.queryByText("No value")).toBeNull();
      expect(screen.queryByText("No project")).toBeNull();
      expect(screen.queryByText("Acme Corp")).toBeNull();
    },
  );
});
