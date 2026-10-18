import { describe, expect, it } from "vitest";
import { readDiagramReviewDrafts } from "./diagram-review";
import { createChatStore } from "./store";
import type { StorageAdapter } from "../types";

describe("diagram review drafts", () => {
  it("rejects corrupt marks without losing valid drafts", () => {
    expect(readDiagramReviewDrafts(JSON.stringify({ good: { marks: [], note: "Keep me" }, bad: { marks: [{ kind: "pin", point: null }], note: "" } })))
      .toEqual({ good: { marks: [], note: "Keep me" } });
    expect(readDiagramReviewDrafts("not JSON")).toEqual({});
  });
  it("restores marks after a new store instance and isolates each source/session key", () => {
    const entries = new Map<string, string>();
    const storage: StorageAdapter = { getItem: k => entries.get(k) ?? null, setItem: (k, v) => { entries.set(k, v); }, removeItem: k => { entries.delete(k); } };
    const store = createChatStore({ storage });
    const draft = { note: "Why?", marks: [{ id: "1", kind: "pin" as const, point: { x: 20, y: 30 }, text: "This arrow" }] };
    store.getState().setDiagramReview("session-a:source-a", draft);
    const restored = createChatStore({ storage });
    expect(restored.getState().diagramReviews["session-a:source-a"]).toEqual(draft);
    expect(restored.getState().diagramReviews["session-b:source-a"]).toBeUndefined();
    restored.getState().setDiagramReview("session-a:source-a", { marks: [], note: "" });
    expect(createChatStore({ storage }).getState().diagramReviews).toEqual({});
  });
});
