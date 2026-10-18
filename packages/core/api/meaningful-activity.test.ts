import { describe, expect, it } from "vitest";
import { IssueMeaningfulActivityPageSchema } from "./schemas";

const item = {
  code: "review_recorded" as const,
  result_code: "succeeded" as const,
  occurred_at: "2026-09-10T12:00:00Z",
  reference: { type: "review", id: "review-1", revision: 2 },
};

describe("IssueMeaningfulActivityPageSchema", () => {
  it("accepts empty, one-item, and bounded maximum pages", () => {
    for (const items of [[], [item], Array.from({ length: 100 }, (_, index) => ({
      ...item, reference: { ...item.reference, id: `review-${index}` },
    }))]) {
      expect(IssueMeaningfulActivityPageSchema.safeParse({
        schema_version: 1, issue_id: "issue-1", current_picture_revision: 7,
        items, next_cursor: items.length === 100 ? "next" : null,
      }).success).toBe(true);
    }
  });

  it("rejects malformed, duplicate, oversized, and non-newest-first pages", () => {
    const base = { schema_version: 1, issue_id: "issue-1", current_picture_revision: 7, next_cursor: null };
    expect(IssueMeaningfulActivityPageSchema.safeParse({ ...base, items: [item, item] }).success).toBe(false);
    expect(IssueMeaningfulActivityPageSchema.safeParse({ ...base, items: Array.from({ length: 101 }, (_, i) => ({ ...item, reference: { ...item.reference, id: `${i}` } })) }).success).toBe(false);
    expect(IssueMeaningfulActivityPageSchema.safeParse({ ...base, items: [
      { ...item, occurred_at: "2026-09-09T12:00:00Z" }, item,
    ] }).success).toBe(false);
    expect(IssueMeaningfulActivityPageSchema.safeParse({ ...base, items: [{ ...item, code: "chat_changed" }] }).success).toBe(false);
  });
});
