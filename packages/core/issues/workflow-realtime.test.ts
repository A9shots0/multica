import { QueryClient } from "@tanstack/react-query";
import { describe, expect, it, vi } from "vitest";
import { onIssueWorkflowUpdated } from "./ws-updaters";

describe("workflow projection realtime admission", () => {
  it("invalidates once for a newer revision and ignores stale or equal updates", () => {
    const qc = new QueryClient();
    const invalidate = vi.spyOn(qc, "invalidateQueries").mockResolvedValue();
    onIssueWorkflowUpdated(qc, "ws-1", "issue-1", 4);
    onIssueWorkflowUpdated(qc, "ws-1", "issue-1", 4);
    onIssueWorkflowUpdated(qc, "ws-1", "issue-1", 3);
    expect(invalidate).toHaveBeenCalledTimes(1);
    expect(invalidate).toHaveBeenCalledWith({ queryKey: ["issues", "ws-1"] });
    onIssueWorkflowUpdated(qc, "ws-1", "issue-1", 5);
    expect(invalidate).toHaveBeenCalledTimes(2);
  });

  it("isolates workspaces and issues and refuses malformed revisions", () => {
    const qc = new QueryClient();
    const invalidate = vi.spyOn(qc, "invalidateQueries").mockResolvedValue();
    onIssueWorkflowUpdated(qc, "ws-1", "issue-1", 1);
    onIssueWorkflowUpdated(qc, "ws-1", "issue-2", 1);
    onIssueWorkflowUpdated(qc, "ws-2", "issue-1", 1);
    onIssueWorkflowUpdated(qc, "ws-1", "issue-1", Number.NaN);
    onIssueWorkflowUpdated(qc, "", "issue-1", 2);
    expect(invalidate).toHaveBeenCalledTimes(3);
  });
});
