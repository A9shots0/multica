import { describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import { I18nProvider } from "@multica/core/i18n/react";
import enChat from "../../locales/en/chat.json";
import { ChatReplyContext } from "./chat-reply-context";

const selected = {
  id: "issue-1",
  title: "Review the layout",
  target: { issue_id: "issue-1", issue_revision: 3, owner_id: "owner", request_id: "question-1" },
  pending_requests: [{
    id: "question-1",
    question: "Should I rebuild the review gate?",
    target: { issue_id: "issue-1", issue_revision: 3, owner_id: "owner", request_id: "question-1" },
  }],
};

describe("ChatReplyContext", () => {
  it("shows the exact temporary reply target and clears it without a destination menu", () => {
    const onClear = vi.fn();
    render(<I18nProvider locale="en" resources={{ en: { chat: enChat } }}><ChatReplyContext selected={selected} onClear={onClear} /></I18nProvider>);
    expect(screen.getByText("Should I rebuild the review gate?")).toBeVisible();
    expect(screen.queryByRole("button", { name: "Choose message topic" })).toBeNull();
    fireEvent.click(screen.getByRole("button", { name: "Return to the whole conversation" }));
    expect(onClear).toHaveBeenCalledOnce();
  });
});
