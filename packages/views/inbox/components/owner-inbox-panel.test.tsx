/** @vitest-environment jsdom */
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { InboxItem } from "@multica/core/types";
import { OwnerInboxPanel } from "./owner-inbox-panel";
vi.mock("./owner-runtime-route-fields", () => ({ OwnerRuntimeRouteFields: () => null }));

const respond = vi.fn(async () => undefined);
const comment = vi.fn(async () => undefined);
const acknowledge = vi.fn(async () => undefined);
const updateIssue = vi.fn(async () => undefined);
const directive = vi.fn(async () => undefined);
const answerWorkflowQuestion = vi.fn(async () => undefined);

vi.mock("@tanstack/react-query", () => ({
  useQuery: (options: { kind?: string }) => options.kind === "decision"
    ? { data: { decision: { id: "decision-1", revision: 3, type: "authorization", state: "unanswered", question: "Ship this candidate?", recommendation: "Approve the reviewed candidate.", consequence: "Rejecting keeps it paused." }, attention: { id: "attention-1", acknowledged: false } } }
    : options.kind === "no-decision" ? { data: undefined }
    : options.kind === "owner-notifications"
      ? { data: [
        { id: "sms-2", event_type: "owner_decision", delivery_mode: "live_test", state: "delivered", provider_state: "delivered", admitted_at: "2026-09-10T00:00:02Z", delivered_at: "2026-09-10T00:00:03Z" },
        { id: "sms-1", event_type: "owner_decision", delivery_mode: "dry_run", state: "delivered", provider_state: "dry_run_delivered", admitted_at: "2026-09-10T00:00:00Z", delivered_at: "2026-09-10T00:00:01Z" },
      ] }
    : { data: { identifier: "SILV-10", title: "Release the candidate", revision: 7, current_picture: {
      owner_summary: { stage: "needs_you", latest_meaningful_change: { code: "review_passed", result_code: "pass" } },
      evidence_summary: { candidate: "v197", preview_url: "https://example.test/candidate/v197" },
    } } },
}));
vi.mock("@multica/core/hooks", () => ({ useWorkspaceId: () => "workspace-1" }));
vi.mock("@multica/core/paths", () => ({ useWorkspacePaths: () => ({ issueDetail: (id: string) => `/acme/issues/${id}` }) }));
vi.mock("@multica/core/issues/queries", () => ({ issueDetailOptions: () => ({ kind: "issue" }) }));
vi.mock("@multica/core/inbox/queries", () => ({
  workflowDecisionOptions: (_issue: string, decision: string) => ({ kind: decision ? "decision" : "no-decision" }),
  ownerNotificationOptions: () => ({ kind: "owner-notifications" }),
}));
vi.mock("@multica/core/issues/mutations", () => ({
  useCreateComment: () => ({ mutateAsync: comment, isPending: false }),
  useUpdateIssue: () => ({ mutateAsync: updateIssue, isPending: false }),
}));
vi.mock("@multica/core/inbox/mutations", () => ({
  useRespondWorkflowDecision: () => ({ mutateAsync: respond, isPending: false }),
  useAcknowledgeWorkflowAttention: () => ({ mutateAsync: acknowledge, isPending: false }),
  useCreateOwnerDirective: () => ({ mutateAsync: directive, isPending: false }),
  useAnswerWorkflowOwnerDecision: () => ({ mutateAsync: answerWorkflowQuestion, isPending: false }),
}));
vi.mock("sonner", () => ({ toast: { success: vi.fn(), error: vi.fn() } }));
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

const item: InboxItem = {
  id: "inbox-1", workspace_id: "workspace-1", recipient_type: "member", recipient_id: "member-1",
  actor_type: "agent", actor_id: "agent-1", type: "workflow_owner_decision", severity: "action_required",
  issue_id: "issue-1", title: "Release the candidate", body: null, issue_status: null, read: false,
  archived: false, created_at: "2026-09-10T00:00:00Z", details: { decision_id: "decision-1" },
};

describe("OwnerInboxPanel", () => {
  it("answers a route question through its exact workflow event instead of a comment or directive", async () => {
    const question = { ...item, id: "route-question", details: { workflow_event_id: "event-1", question: "Which deployment target?", recommendation: "Use the approved target.", consequence: "The other target expands scope.", options: "Approved target\nOther target" } };
    render(<OwnerInboxPanel item={question} onAcknowledge={vi.fn()} />);
    expect(screen.getByText("Which deployment target?")).toBeVisible();
    expect(screen.getByText("Use the approved target.")).toBeVisible();
    expect(screen.getByRole("button", { name: "Approved target" })).toBeVisible();
    expect(screen.getByRole("button", { name: "Other target" })).toBeVisible();
    const consequence = screen.getByText(/The other target expands scope/);
    expect(consequence).not.toBeVisible();
    fireEvent.click(screen.getByText("Impact & consequence details"));
    expect(consequence).toBeVisible();
    expect(screen.queryByRole("button", { name: "Add comment" })).toBeNull();
    expect(screen.queryByRole("button", { name: "Correct" })).toBeNull();
    const send = screen.getByRole("button", { name: "Send answer and continue" });
    expect(send).toBeDisabled();
    fireEvent.change(screen.getByPlaceholderText(/add context/i), { target: { value: "  Use only the approved target.  " } });
    fireEvent.click(send);
    await waitFor(() => expect(answerWorkflowQuestion).toHaveBeenCalledWith({ item: question, answer: "Use only the approved target." }));
    expect(directive).not.toHaveBeenCalled();
    expect(comment).not.toHaveBeenCalled();
    expect(respond).not.toHaveBeenCalled();
  });
  it("requires an explicit answer to a durable ordinary-worker question and submits its exact receipt", async () => {
    render(<OwnerInboxPanel item={{ ...item, id: "question-1", body: "Which contract should the worker use?", details: { question_scope: "ordinary_task", owner_category: "needs_you", owner_relevant: "true", intake_state: "pending", recommendation: "Use contract A.", consequence: "A preserves the approved scope.", options: "Contract A\nContract B" } }} onAcknowledge={vi.fn()} />);
    expect(screen.getByText("Use contract A.")).toBeVisible();
    expect(screen.getByRole("button", { name: "Contract A" })).toBeVisible();
    expect(screen.getByRole("button", { name: "Contract B" })).toBeVisible();
    const send = screen.getByRole("button", { name: "Send answer and continue" });
    expect(send).toBeDisabled();
    expect(screen.queryByRole("button", { name: "Add comment" })).toBeNull();
    fireEvent.change(screen.getByPlaceholderText(/add context/i), { target: { value: "Use A, without expanding scope." } });
    fireEvent.click(send);
    await waitFor(() => expect(directive).toHaveBeenCalledWith({ issueId: "issue-1", request: { expected_issue_revision: 7, directive: "correct", content: "Use A, without expanding scope.", intake_id: "question-1" } }));
    expect(comment).not.toHaveBeenCalled();
    expect(respond).not.toHaveBeenCalled();
  });

  beforeEach(() => { respond.mockClear(); comment.mockClear(); acknowledge.mockClear(); updateIssue.mockClear(); directive.mockClear(); answerWorkflowQuestion.mockClear(); });

  it("continues a persisted Chat intake through a revision-checked owner direction, never approval or a raw comment", async () => {
    render(<OwnerInboxPanel item={{ ...item, id: "intake-1", type: "chat_intake", details: { owner_relevant: "true", owner_category: "needs_you", intake_state: "pending", request: "Verify the existing issue." } }} onAcknowledge={vi.fn()} />);
    expect(screen.queryByRole("button", { name: "Approve" })).toBeNull();
    expect(screen.queryByRole("button", { name: "Add comment" })).toBeNull();
    expect(directive).not.toHaveBeenCalled();
    fireEvent.click(screen.getByRole("button", { name: "Continue work" }));
    await waitFor(() => expect(directive).toHaveBeenCalledWith({ issueId: "issue-1", request: {
      expected_issue_revision: 7, directive: "correct", content: "Verify the existing issue.", intake_id: "intake-1",
    } }));
    expect(respond).not.toHaveBeenCalled();
    expect(comment).not.toHaveBeenCalled();
  });

  it("separates atomic authorization from bounded work-direction controls", () => {
    render(<OwnerInboxPanel item={item} onAcknowledge={vi.fn()} />);
    for (const label of ["Approve", "Reject", "Correct", "Request revisions", "Partially accept", "Change scope", "Cancel work"]) {
      expect(screen.getByRole("button", { name: label })).toBeTruthy();
    }
    expect(screen.getByText("candidate")).toBeTruthy();
    expect(screen.getByText("v197")).toBeTruthy();
    expect(screen.getByRole("link", { name: /open evidence or ui/i })).toHaveAttribute("href", "https://example.test/candidate/v197");
    expect(screen.getByRole("link", { name: /inspect evidence and full issue/i })).toHaveAttribute("href", "/acme/issues/issue-1");
    expect(screen.getByText("Dry-run recorded")).toBeTruthy();
    expect(screen.getByText("Live test delivered")).toBeTruthy();
  });

  it("records a revision directive with the owner's exact correction on the issue", async () => {
    render(<OwnerInboxPanel item={item} onAcknowledge={vi.fn()} />);
    fireEvent.click(screen.getByRole("button", { name: "Request revisions" }));
    fireEvent.change(screen.getByPlaceholderText(/add context/i), { target: { value: "Keep the approved layout, but fix the mobile overflow." } });
    fireEvent.click(screen.getByRole("button", { name: "Request revisions" }));

    await waitFor(() => expect(directive).toHaveBeenCalledWith({
      issueId: "issue-1",
      request: { expected_issue_revision: 7, directive: "request_changes", content: "Keep the approved layout, but fix the mobile overflow." },
    }));
    expect(respond).not.toHaveBeenCalled();
  });

  it("records cancellation context and then stops the bound issue", async () => {
    render(<OwnerInboxPanel item={item} onAcknowledge={vi.fn()} />);
    fireEvent.click(screen.getByRole("button", { name: "Cancel work" }));
    fireEvent.change(screen.getByPlaceholderText(/add context/i), { target: { value: "The target account is no longer in scope." } });
    fireEvent.click(screen.getByRole("button", { name: "Cancel work" }));

    await waitFor(() => expect(directive).toHaveBeenCalledWith({
      issueId: "issue-1",
      request: { expected_issue_revision: 7, directive: "cancel", content: "The target account is no longer in scope." },
    }));
    expect(updateIssue).not.toHaveBeenCalled();
  });
});
