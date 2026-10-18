// @vitest-environment node
import { describe, it, expect } from "vitest";
import { createChatStore } from "./store";
import { readChatWorkSelections } from "./work";

describe("composer work topic recovery", () => {
  it("does not restore a destination created by the retired auto-targeting model", () => {
    const target = { issue_id: "issue-a", issue_revision: 7, owner_id: "owner", request_id: "question-a" };
    const data = new Map<string, string>([["multica:chat:work-topics", JSON.stringify({
      "chat-a": { id: "issue-a", title: "Old automatic target", target },
    })]]);
    const storage = { getItem: (key: string) => data.get(key) ?? null, setItem: (key: string, value: string) => { data.set(key, value); }, removeItem: (key: string) => { data.delete(key); } };
    expect(createChatStore({ storage }).getState().workTopics).toEqual({});
  });

  it("restores the exact question and revision alongside the draft without caching live work state", () => {
    const data = new Map<string, string>();
    const storage = { getItem: (key: string) => data.get(key) ?? null, setItem: (key: string, value: string) => { data.set(key, value); }, removeItem: (key: string) => { data.delete(key); } };
    const store = createChatStore({ storage });
    const target = { issue_id: "issue-a", issue_revision: 7, owner_id: "owner", request_id: "question-a", result_digest: "captured" };
    store.getState().setInputDraft("chat-a", "Choose Warm");
    store.getState().setWorkTopic("chat-a", { id: "issue-a", title: "Welcome", target, pending_requests: [{ id: "question-a", question: "Which tone?", target }, { id: "other", question: "Unselected question", target }] });
    const restored = createChatStore({ storage });
    expect(restored.getState().inputDrafts["chat-a"]).toBe("Choose Warm");
    expect(restored.getState().workTopics["chat-a"]?.target).toEqual(target);
    expect(restored.getState().workTopics["chat-a"]?.pending_requests).toHaveLength(1);
    expect(restored.getState().workTopics["chat-b"]).toBeUndefined();
    restored.getState().setWorkTopic("chat-a", undefined);
    expect(createChatStore({ storage }).getState().workTopics).toEqual({});
    expect(restored.getState().inputDrafts["chat-a"]).toBe("Choose Warm");
  });
  it("rejects corrupt or mismatched saved targets", () => {
    for (const raw of ["null", "[]", "broken", '{"chat":{"id":"a","title":"A","target":{"issue_id":"b","issue_revision":1,"owner_id":"owner"}}}']) {
      expect(readChatWorkSelections(raw)).toEqual({});
    }
  });

  it("keeps several independently targeted answers in one session", () => {
    const data = new Map<string, string>();
    const storage = { getItem: (key: string) => data.get(key) ?? null, setItem: (key: string, value: string) => { data.set(key, value); }, removeItem: (key: string) => { data.delete(key); } };
    const store = createChatStore({ storage });
    const first = { issue_id: "issue-a", issue_revision: 7, owner_id: "owner", request_id: "question-a" };
    const second = { issue_id: "issue-b", issue_revision: 3, owner_id: "owner", request_id: "question-b" };
    store.getState().setWorkResponse("chat-a", { id: "issue-a", title: "Checkout", target: first }, "One more spacing pass");
    store.getState().setWorkResponse("chat-a", { id: "issue-b", title: "Profile", target: second }, "Move the action below");
    expect(Object.values(store.getState().workResponses["chat-a"] ?? {}).map(item => item.answer)).toEqual([
      "One more spacing pass",
      "Move the action below",
    ]);
    const restored = createChatStore({ storage });
    expect(Object.keys(restored.getState().workResponses["chat-a"] ?? {})).toEqual(["question-a", "question-b"]);
    restored.getState().setWorkResponse("chat-a", { id: "issue-a", title: "Checkout", target: first }, undefined);
    expect(Object.keys(restored.getState().workResponses["chat-a"] ?? {})).toEqual(["question-b"]);
    restored.getState().clearWorkResponses("chat-a");
    expect(restored.getState().workResponses["chat-a"]).toBeUndefined();
  });
});
