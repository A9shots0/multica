import { describe, it, expect } from "vitest";
import { createChatStore } from "./store";
import { ChatMessageSchema, SendChatMessageResponseSchema } from "../api/schemas";

describe("chat response format", () => {
  it("persists per draft, replaces rather than accumulates, and clears with the sent draft", () => {
    const data = new Map<string, string>();
    const storage = { getItem: (k: string) => data.get(k) ?? null, setItem: (k: string, v: string) => { data.set(k, v); }, removeItem: (k: string) => { data.delete(k); } };
    const store = createChatStore({ storage });
    store.getState().setInputDraft("a", "My words");
    store.getState().setResponseFormat("a", "compare");
    store.getState().setResponseFormat("a", "diagram");
    store.getState().setResponseFormat("b", "plan");
    const reloaded = createChatStore({ storage });
    expect(reloaded.getState().responseFormats).toEqual({ a: "diagram", b: "plan" });
    expect(reloaded.getState().inputDrafts.a).toBe("My words");
    reloaded.getState().clearInputDraft("a");
    expect(reloaded.getState().responseFormats).toEqual({ b: "plan" });
    reloaded.getState().setResponseFormat("b", undefined);
    expect(createChatStore({ storage }).getState().responseFormats).toEqual({});
  });

  it("ignores unknown format metadata without discarding the message", () => {
    const message = { id: "m", chat_session_id: "s", role: "user", content: "My words", task_id: "t", created_at: "now" };
    expect(ChatMessageSchema.parse({ ...message, response_format: "compare" }).response_format).toBe("compare");
    for (const response_format of ["unknown", 42, null, {}]) {
      const parsed = ChatMessageSchema.parse({ ...message, response_format });
      expect(parsed.content).toBe("My words");
      expect(parsed.response_format).toBeUndefined();
    }
    expect(SendChatMessageResponseSchema.parse({ message_id: "m", task_id: "t", created_at: "now", response_format: "diagram" }).response_format).toBe("diagram");
  });
});
