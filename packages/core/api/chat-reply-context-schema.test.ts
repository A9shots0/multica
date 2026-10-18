import { describe, expect, it } from "vitest";
import { ChatMessageSchema, SendChatMessageResponseSchema } from "./schemas";

const replyContext = {
  schema_version: 1 as const,
  event_id: "event-1",
  artifact_id: "artifact-1",
  actor: "assistant" as const,
  message_kind: "work_update",
  content: "The latest review is ready.",
  created_at: "2026-10-04T20:00:00Z",
  work_issue_id: "issue-1",
  request_id: "request-1",
  attachments: [{ id: "attachment-1", filename: "review.png", content_type: "image/png" }],
  artifacts: [{ id: "artifact-1", label: "Review", kind: "image" as const }],
};

describe("chat exact-reply API boundary", () => {
  it("preserves a valid immutable reply snapshot on message readback", () => {
    const parsed = ChatMessageSchema.parse({
      id: "message-2",
      chat_session_id: "session-1",
      role: "user",
      content: "Make this quieter.",
      task_id: "task-1",
      created_at: "2026-10-04T20:01:00Z",
      reply_to_message_id: "event-1",
      reply_context: replyContext,
    });

    expect(parsed.reply_to_message_id).toBe("event-1");
    expect(parsed.reply_context).toEqual(replyContext);
  });

  it("drops malformed optional reply context without hiding the message", () => {
    const parsed = ChatMessageSchema.parse({
      id: "message-2",
      chat_session_id: "session-1",
      role: "user",
      content: "Still visible",
      task_id: null,
      created_at: "",
      reply_context: { ...replyContext, schema_version: 2 },
    });

    expect(parsed.content).toBe("Still visible");
    expect(parsed.reply_context).toBeNull();
  });

  it("parses the server-authoritative snapshot returned by send", () => {
    const parsed = SendChatMessageResponseSchema.parse({
      message_id: "message-2",
      task_id: "task-1",
      created_at: "2026-10-04T20:01:00Z",
      reply_context: replyContext,
    });

    expect(parsed.reply_context?.artifact_id).toBe("artifact-1");
  });
});
