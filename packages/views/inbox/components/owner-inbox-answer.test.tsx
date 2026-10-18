/** @vitest-environment jsdom */
import { act, renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { useAnswer, choicesFor } from "./owner-inbox-card";
import { groupOwnerInbox } from "@multica/core/inbox/owner-inbox-model";
import type { InboxItem } from "@multica/core/types";
import enInbox from "../../locales/en/inbox.json";

const state = vi.hoisted(() => ({ decision: { id: "decision", revision: 1, state: "unanswered", question: "Use preview one?" }, respond: vi.fn(), directive: vi.fn() }));
vi.mock("@multica/core/hooks", () => ({ useWorkspaceId: () => "workspace" }));
vi.mock("@tanstack/react-query", () => ({ useQuery: (options: { kind: string; enabled?: boolean }) => ({ data: options.kind === "decision" && options.enabled !== false ? { decision: state.decision } : options.kind === "issue" ? { revision: 2 } : undefined }) }));
vi.mock("@multica/core/issues/queries", () => ({ issueDetailOptions: () => ({ kind: "issue" }), issueAttachmentsOptions: vi.fn() }));
vi.mock("@multica/core/inbox/queries", () => ({ workflowDecisionOptions: (_issueId: string, decisionId: string) => ({ kind: "decision", enabled: !!decisionId }) }));
vi.mock("@multica/core/inbox/mutations", () => ({ useRespondWorkflowDecision: () => ({ mutateAsync: state.respond }), useCreateOwnerDirective: () => ({ mutateAsync: state.directive }), useAnswerWorkflowOwnerDecision: () => ({ mutateAsync: vi.fn() }) }));
vi.mock("../../i18n", () => ({ useT: () => ({ t: (selector: (r: typeof enInbox) => string) => selector(enInbox) }) }));

const item: InboxItem = { id: "request", workspace_id: "workspace", recipient_type: "member", recipient_id: "owner", actor_type: "system", actor_id: null, issue_status: "in_progress", type: "workflow_owner_decision", severity: "action_required", title: "Review preview", body: "Use preview one?", issue_id: "issue", read: false, archived: false, created_at: "2026-09-24T12:00:00Z", details: { decision_id: "decision", options: "Decline\nApprove" } };
const group = groupOwnerInbox([item])[0]!;
describe("captured decision review", () => {
  beforeEach(() => { state.decision = { id: "decision", revision: 1, state: "unanswered", question: "Use preview one?" }; state.respond.mockReset().mockResolvedValue({}); state.directive.mockReset(); });
  it("never maps a versioned request's first prose option to approval", () => {
    expect(choicesFor(group)[0]).toMatchObject({ label: "Approve", send: "Approved.", directive: "correct" });
    expect(choicesFor(group).find(choice => choice.label === "Decline")?.directive).toBe("cancel");
  });
  it("holds the reviewed version across refresh and requires reviewing its replacement", async () => {
    const hook = renderHook(() => useAnswer(group));
    expect(hook.result.current.ready).toBe(true);
    state.decision = { ...state.decision, revision: 2, question: "Use revised preview?" };
    hook.rerender();
    expect(hook.result.current.decision?.question).toBe("Use preview one?");
    expect(hook.result.current.ready).toBe(false);
    await act(async () => { expect(await hook.result.current.send("correct", "Approved.")).toBe(false); });
    expect(state.respond).not.toHaveBeenCalled();
    act(() => hook.result.current.refreshReview());
    expect(hook.result.current.decision?.question).toBe("Use revised preview?");
    await act(async () => { expect(await hook.result.current.send("request_changes", "Please change this.", "Use larger labels.")).toBe(true); });
    expect(state.respond).toHaveBeenCalledWith({ issueId: "issue", decisionId: "decision", request: { expected_revision: 2, outcome: "reject", comment: "Please change this.\n\nUse larger labels." } });
    expect(state.directive).not.toHaveBeenCalled();
  });
  it("keeps a failed submission unsent and supports retry", async () => {
    state.respond.mockRejectedValueOnce(new Error("Unavailable"));
    const hook = renderHook(() => useAnswer(group));
    await act(async () => { expect(await hook.result.current.send("correct", "Approved.")).toBe(false); });
    expect(hook.result.current.sent).toBeNull();
    await act(async () => { expect(await hook.result.current.send("correct", "Approved.")).toBe(true); });
    expect(hook.result.current.sent).toBe("Approved.");
  });

  it("records investigation answers as direction instead of recursively declaring another failure", async () => {
    const investigationItem: InboxItem = {
      ...item,
      id: "investigation-question",
      details: {
        investigation_id: "investigation-1",
        question: "Which prevention plan should we use?",
        options: "Use the reviewed plan\nChange the plan first",
      },
    };
    const investigationGroup = groupOwnerInbox([investigationItem])[0]!;
    const hook = renderHook(() => useAnswer(investigationGroup));

    await act(async () => {
      expect(await hook.result.current.send("request_changes", "Change the plan first", "Keep the stricter gate.", 7)).toBe(true);
    });

    expect(state.directive).toHaveBeenCalledWith({
      issueId: "issue",
      request: {
        expected_issue_revision: 7,
        directive: "correct",
        content: "Change the plan first\n\nKeep the stricter gate.",
        inbox_item_id: "investigation-question",
      },
    });
  });
});
