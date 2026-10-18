// @vitest-environment jsdom

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, fireEvent, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { DevelopmentMemoryProposal } from "@multica/core/types";
import { issueKeys } from "@multica/core/issues/queries";
import { renderWithI18n } from "../../test/i18n";

const mutationState = vi.hoisted(() => ({ approve: vi.fn(), reject: vi.fn(), retry: vi.fn() }));

vi.mock("@multica/core/issues/mutations", () => ({
  useApproveIssueDevelopmentMemory: () => ({ mutate: mutationState.approve, isPending: false }),
  useRejectIssueDevelopmentMemory: () => ({ mutate: mutationState.reject, isPending: false }),
  useRetryIssueDevelopmentMemory: () => ({ mutate: mutationState.retry, isPending: false }),
}));

import { DevelopmentMemorySection } from "./development-memory-section";

const proposal: DevelopmentMemoryProposal = {
  id: "proposal-1", workspaceId: "workspace-1", issueId: "issue-1", taskId: "task-1",
  workstreamId: "multica-issue-issue-1", agentName: "Codex",
  taskStartedAt: "2026-10-01T12:00:00Z", taskCompletedAt: "2026-10-01T12:05:00Z",
  taskStatus: "complete",
  candidate: {
    summary: "Kept CRM and backend contracts aligned.",
    nextAction: "Run owner acceptance.",
    remainingUncertainty: ["Final preview is not accepted."],
    memoryItems: [{ repositoryIdentity: "a9shots0/smcrm", scope: "project", kind: "decision", statement: "CRM owns the UI.", evidenceRefs: ["commit:abc"] }],
    repositoryEvidence: [{ canonicalIdentity: "a9shots0/smcrm", revision: null, branch: "codex/test", dirty: null, evidenceRefs: [] }],
  },
  approvedPayload: null, reviewStatus: "pending", reviewedBy: null, reviewedAt: null,
  reviewNote: null, deliveredAt: null, deadLetteredAt: null, lastError: null,
  attemptCount: 0, createdAt: "2026-10-01T12:05:00Z", updatedAt: "2026-10-01T12:05:00Z",
  issueTitle: null,
};

afterEach(() => { cleanup(); vi.clearAllMocks(); });

describe("DevelopmentMemorySection", () => {
  it("shows only the durable statement and plain owner actions", () => {
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    client.setQueryData(issueKeys.developmentMemory("workspace-1", "issue-1"), [proposal]);
    renderWithI18n(<QueryClientProvider client={client}><DevelopmentMemorySection wsId="workspace-1" issueId="issue-1" /></QueryClientProvider>);

    expect(screen.getByText("Save this as a rule for future agents?")).toBeInTheDocument();
    expect(screen.getByText(/future agents working on this project will receive it automatically/i)).toBeInTheDocument();
    expect(screen.getByText("CRM owns the UI.")).toBeInTheDocument();
    expect(screen.queryByText("Kept CRM and backend contracts aligned.")).not.toBeInTheDocument();
    expect(screen.queryByText(/repository evidence/i)).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Edit first" }));
    const editor = screen.getByRole("textbox", { name: /what to remember/i });
    fireEvent.change(editor, { target: { value: "CRM owns every operator-facing UI." } });
    fireEvent.click(screen.getByRole("button", { name: "Save edited rule" }));
    expect(mutationState.approve).toHaveBeenCalledWith(
      expect.objectContaining({
        proposalId: "proposal-1",
        payload: expect.objectContaining({
          memoryItems: [expect.objectContaining({ statement: "CRM owns every operator-facing UI." })],
        }),
      }),
      expect.any(Object),
    );
  });

  it("ignores a proposal without asking the owner for a technical note", () => {
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    client.setQueryData(issueKeys.developmentMemory("workspace-1", "issue-1"), [proposal]);
    renderWithI18n(<QueryClientProvider client={client}><DevelopmentMemorySection wsId="workspace-1" issueId="issue-1" /></QueryClientProvider>);

    fireEvent.click(screen.getByRole("button", { name: "Don’t save" }));
    expect(mutationState.reject).toHaveBeenCalledWith(
      { proposalId: "proposal-1", note: "Not useful for future work." },
      expect.any(Object),
    );
  });

  it("keeps a dead-lettered approved update visible and retryable", () => {
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const paused = { ...proposal, reviewStatus: "approved" as const, approvedPayload: proposal.candidate, attemptCount: 24, deadLetteredAt: "2026-10-01T13:00:00Z" };
    client.setQueryData(issueKeys.developmentMemory("workspace-1", "issue-1"), [paused]);
    renderWithI18n(<QueryClientProvider client={client}><DevelopmentMemorySection wsId="workspace-1" issueId="issue-1" /></QueryClientProvider>);

    expect(screen.getByText(/could not be saved yet/i)).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "Retry" }));
    expect(mutationState.retry).toHaveBeenCalledWith("proposal-1", expect.any(Object));
  });
});
