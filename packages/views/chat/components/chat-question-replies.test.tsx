import { describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import { I18nProvider } from "@multica/core/i18n/react";
import type { InboxItem } from "@multica/core/types";
import type { ChatWorkIssue } from "@multica/core/chat/work";
import enChat from "../../locales/en/chat.json";
import { ChatQuestionReplies } from "./chat-question-replies";

const item: InboxItem = {
  id: "request-1", workspace_id: "workspace", recipient_type: "member", recipient_id: "owner",
  actor_type: "system", actor_id: null, type: "workflow_owner_decision", severity: "action_required",
  issue_id: "issue-1", title: "Choose", body: "What next?", issue_status: "open", read: false,
  archived: false, created_at: "2026-10-04T00:00:00Z",
  details: { options: "Rebuild the gate.\nInspect more evidence.\nStop the request." },
};
const target = { issue_id: "issue-1", issue_revision: 4, owner_id: "owner", request_id: "request-1" };
const issue: ChatWorkIssue = {
  id: "issue-1", number: 5, title: "Design review", revision: 4, project_id: null, state: "needs_you",
  source: "test", task_id: null, task_status: "waiting", wait_reason: "", observed_at: item.created_at,
  summary: { stage: "review", current_picture_revision: 1 }, target, picture: {},
  pending_requests: [{ id: "request-1", question: "What next?", target }],
};

describe("ChatQuestionReplies", () => {
  it("turns an authored option into an editable reply to the exact request", () => {
    const onReply = vi.fn();
    render(<I18nProvider locale="en" resources={{ en: { chat: enChat } }}><ChatQuestionReplies item={item} issue={issue} onSelect={onReply} /></I18nProvider>);
    fireEvent.click(screen.getByRole("button", { name: "Rebuild the gate." }));
    expect(onReply).toHaveBeenCalledExactlyOnceWith(issue, "Rebuild the gate.\n\n");
    expect(screen.queryByRole("radio")).toBeNull();
    expect(screen.queryByRole("button", { name: "Continue" })).toBeNull();
  });

  it("shows the selected choice inline without a reply banner or instruction", () => {
    const onReply = vi.fn();
    render(<I18nProvider locale="en" resources={{ en: { chat: enChat } }}><ChatQuestionReplies item={item} issue={issue} selectedDraft={"Inspect more evidence.\n\nOnly on mobile"} onSelect={onReply} /></I18nProvider>);
    expect(screen.getByRole("button", { name: "Inspect more evidence." })).toHaveAttribute("aria-pressed", "true");
    expect(screen.queryByText("Selected — edit or add context below")).toBeNull();
    expect(screen.queryByRole("button", { name: "Write a reply" })).toBeNull();
  });
});
