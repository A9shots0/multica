import { describe, expect, it } from "vitest";
import { ChatMessageSchema, ChatSessionSchema } from "./schemas";

const update = { kind: "question", issue_id: "issue", title: "Review", source_id: "request", request_id: "request", occurred_at: "2026-09-24T00:00:00Z", body: "Which wording?" };
const message = { id: "message", chat_session_id: "session", role: "assistant", content: "Work update: Which wording?", message_kind: "work_update", work_update: update };
describe("work update API boundary", () => {
  it("preserves a saved reply context while malformed labels do not hide messages", () => {
    const context = { issue_id: "issue", title: "Review mobile", request_id: "question" };
    expect(ChatMessageSchema.parse({ ...message, work_context: context }).work_context).toEqual(context);
    expect(ChatMessageSchema.parse({ ...message, work_context: { title: 5 } }).work_context).toBeNull();
  });
  it("preserves the kind and structured source through the actual message parser", () => {
    const parsed = ChatMessageSchema.parse(message);
    expect(parsed.message_kind).toBe("work_update");
    expect(parsed.work_update).toEqual(update);
    expect(parsed.task_id).toBeNull();
  });
  it("preserves a versioned review artifact without making it required for older updates", () => {
    const review = {
      schema_version: 1 as const,
      source_task_id: "task",
      artifacts: [{ id: "artifact", label: "preview", kind: "page" as const, url: "/preview", original: true }],
    };
    const parsed = ChatMessageSchema.parse({ ...message, work_update: { ...update, review } });
    expect(parsed.work_update?.review).toEqual(review);
    expect(ChatMessageSchema.parse(message).work_update?.review).toBeUndefined();
  });
  it("drops only malformed optional review metadata", () => {
    const parsed = ChatMessageSchema.parse({ ...message, work_update: { ...update, review: { schema_version: 2 } } });
    expect(parsed.work_update?.body).toBe(update.body);
    expect(parsed.work_update?.review).toBeNull();
  });
  it("preserves readable evidence when optional metadata is malformed", () => {
    const parsed = ChatMessageSchema.parse({ ...message, work_update: { body: 5 } });
    expect(parsed.work_update).toBeNull();
    expect(parsed.content).toBe(message.content);
  });
  it("keeps the update kind in the chat library preview", () => {
    expect(ChatSessionSchema.parse({ id: "session", last_message: message }).last_message?.message_kind).toBe("work_update");
  });
});
