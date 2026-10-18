/** @vitest-environment jsdom */
import React from "react";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { WorkforceOverview } from "./workforce-overview";

const state = vi.hoisted(() => ({ search: vi.fn() }));

vi.mock("@multica/core/api", () => ({ api: { searchWorkHistory: state.search } }));
vi.mock("@multica/core/hooks", () => ({ useWorkspaceId: () => "workspace-1" }));
vi.mock("@multica/core/paths", () => ({ useWorkspacePaths: () => ({ issueDetail: (id: string) => `/issues/${id}` }) }));
vi.mock("@multica/ui/hooks/use-mobile", () => ({ useIsCompact: () => false }));
vi.mock("react-resizable-panels", () => ({
  useDefaultLayout: () => ({ defaultLayout: undefined, onLayoutChanged: vi.fn() }),
}));
vi.mock("@multica/ui/components/ui/resizable", () => ({
  ResizablePanelGroup: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
  ResizablePanel: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
  ResizableHandle: () => null,
}));
vi.mock("../../i18n", async () => {
  const resource = (await import("../../locales/en/inbox.json")).default;
  return {
    useT: () => ({
      t: (selector: (value: typeof resource) => string, vars?: Record<string, unknown>) => {
        let value = selector(resource);
        for (const [key, replacement] of Object.entries(vars ?? {})) {
          value = value.replace(`{{${key}}}`, String(replacement));
        }
        return value;
      },
    }),
  };
});

function renderOverview() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <WorkforceOverview
        issues={[]}
        loading={false}
        selectedId=""
        needsCount={0}
        onNeedsYou={vi.fn()}
        onSelect={vi.fn()}
        onClearSelection={vi.fn()}
      />
    </QueryClientProvider>,
  );
}

describe("WorkforceOverview past-work search", () => {
  beforeEach(() => {
    state.search.mockReset().mockResolvedValue({
      schemaVersion: "silvermediums.brain.v2",
      servedAt: "2026-10-16T13:00:00Z",
      history: {
        matchCount: 1,
        recordClasses: ["conversation_recollection"],
        groups: [{
          day: "2026-10-15",
          project: "linkhub",
          matches: [{
            id: "memory-1",
            provider: "codex",
            sessionId: "private-session",
            project: "linkhub",
            occurredAt: "2026-10-15T13:00:00Z",
            role: "assistant",
            excerpt: "The LinkHub 403 came from the account-status request.",
            excerptTruncated: false,
            textSha256: "private-hash",
            sourceReference: "work-history://private/source",
            sourceOrdinal: 9,
            sourceOffset: 120,
            recordClass: "conversation_recollection",
            assured: false,
            accepted: false,
            authority: "none",
            matchTypes: ["exact", "semantic"],
          }],
        }],
      },
    });
  });

  afterEach(() => {
    cleanup();
    vi.clearAllMocks();
  });

  it("searches only on submit, shows honest memory, and hides private source data", async () => {
    renderOverview();
    const input = screen.getByRole("searchbox", { name: "Search past AI work" });

    fireEvent.change(input, { target: { value: "LinkHub 403" } });
    expect(state.search).not.toHaveBeenCalled();

    fireEvent.click(screen.getByRole("button", { name: "Search" }));
    await waitFor(() => expect(state.search).toHaveBeenCalledTimes(1));
    expect(state.search).toHaveBeenCalledWith({ query: "LinkHub 403", limit: 30 });
    expect(await screen.findByText("The LinkHub 403 came from the account-status request.")).toBeTruthy();
    expect(screen.getByText("Conversation memory — not verified work")).toBeTruthy();
    expect(screen.getByText("This is memory, not proof")).toBeTruthy();
    expect(screen.queryByText(/private-session|private-hash|work-history:\/\//)).toBeNull();
  });

  it("clears the search and restores the ordinary current-work view", async () => {
    renderOverview();
    fireEvent.change(screen.getByRole("searchbox"), { target: { value: "LinkHub 403" } });
    fireEvent.click(screen.getByRole("button", { name: "Search" }));
    expect(await screen.findByText("Conversation memory — not verified work")).toBeTruthy();

    fireEvent.click(screen.getByRole("button", { name: "Clear past-work search" }));
    expect(await screen.findAllByText("No open work.")).not.toHaveLength(0);
  });
});
