import { describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import { I18nProvider } from "@multica/core/i18n/react";
import type { ChatMessage } from "@multica/core/types";
import enChat from "../../locales/en/chat.json";
import {
  ChatExactReplyContext,
  ChatReplyReference,
  replySelectionFromMessage,
} from "./chat-exact-reply";

const resources = { en: { chat: enChat } };

describe("exact chat replies", () => {
  it("captures the durable event and selected artifact instead of transcript position", () => {
    const message = {
      id: "event-7",
      chat_session_id: "session-1",
      role: "assistant",
      content: "Four files are ready.",
      message_kind: "work_update",
      task_id: "task-1",
      created_at: "2026-10-04T12:00:00Z",
      work_update: {
        kind: "result",
        issue_id: "issue-1",
        title: "Research handoff",
        source_id: "task-1",
        occurred_at: "2026-10-04T12:00:00Z",
        body: "Four files are ready.",
        review: {
          schema_version: 1,
          source_task_id: "task-1",
          artifacts: [{
            id: "artifact-pdf",
            label: "Research.pdf",
            kind: "file",
            url: "/research.pdf",
            original: true,
          }],
        },
      },
    } as ChatMessage;

    expect(replySelectionFromMessage(message, { artifact_id: "artifact-pdf" })).toEqual({
      target: { event_id: "event-7", artifact_id: "artifact-pdf" },
      actor: "assistant",
      message_kind: "work_update",
      preview: "Research.pdf",
    });
  });

  it("shows one compact composer target and lets the member cancel it", () => {
    const onClear = vi.fn();
    render(
      <I18nProvider locale="en" resources={resources}>
        <ChatExactReplyContext
          selected={{
            target: { event_id: "event-7" },
            actor: "assistant",
            message_kind: "message",
            preview: "Which direction should I take?",
          }}
          onClear={onClear}
        />
      </I18nProvider>,
    );

    expect(screen.getByText("Replying to Mika")).toBeVisible();
    expect(screen.getByText("Which direction should I take?")).toBeVisible();
    fireEvent.click(screen.getByRole("button", { name: "Cancel reply" }));
    expect(onClear).toHaveBeenCalledOnce();
  });

  it("renders the immutable server snapshot on the sent turn", () => {
    render(
      <I18nProvider locale="en" resources={resources}>
        <ChatReplyReference
          context={{
            schema_version: 1,
            event_id: "event-7",
            actor: "assistant",
            message_kind: "message",
            content: "Which direction should I take?",
            created_at: "2026-10-04T12:00:00Z",
            attachments: [],
            artifacts: [],
          }}
        />
      </I18nProvider>,
    );

    expect(screen.getByText("Replying to Mika")).toBeVisible();
    expect(screen.getByText("Which direction should I take?")).toBeVisible();
  });
});
