// @vitest-environment node
import { describe, it, expect } from "vitest";
import type { ChatMessage } from "@multica/core/types";
import { groupChatHistory, anchoredFirstIndex, latestQuestionMessageIds, latestVisualReviewMessageIds } from "./work-update-groups";

const message = (id: string, update = false): ChatMessage => ({
  id, chat_session_id: "chat", role: "assistant", content: id, created_at: "now", task_id: null,
  ...(update ? { message_kind: "work_update", work_update: { kind: "started", issue_id: id, title: id, source_id: id, occurred_at: "now", body: id } } : {}),
});
describe("work history grouping", () => {
  it("folds consecutive events without losing any evidence or crossing messages", () => {
    const rows = groupChatHistory([message("a",true),message("b",true),message("reply"),message("c",true)]);
    expect(rows.map(row => row.kind)).toEqual(["work_updates","message","work_updates"]);
    expect(rows[0]?.kind === "work_updates" && rows[0].messages.map(m => m.id)).toEqual(["a","b"]);
  });
  it("keeps a live task's row identity and leaves unknown updates readable", () => {
    expect(groupChatHistory([{ ...message("reply"),task_id:"task" },{ ...message("invalid"), message_kind:"work_update" }]).map(row => row.key)).toEqual(["task:task","invalid"]);
  });
  it("anchors when a prepended page merges its last event into the first group", () => {
    expect(anchoredFirstIndex({keys:["b","reply"],first:100},["a","reply"],90)).toBe(100);
    expect(anchoredFirstIndex({keys:["b","reply"],first:100},["older","a","reply"],90)).toBe(99);
  });
  it("does not move virtual indexes when a burst grows at the end", () => {
    expect(anchoredFirstIndex({keys:["reply","a"],first:100},["reply","a"],100)).toBe(100);
  });
  it("promotes only the newest visual result for an issue across interrupted update groups", () => {
    const review = (id: string, issueId = "issue") => ({
      ...message(id, true),
      work_update: {
        kind: "result" as const,
        issue_id: issueId,
        title: "Review",
        source_id: id,
        occurred_at: "now",
        body: id,
        review: {
          schema_version: 1 as const,
          source_task_id: id,
          artifacts: [{ id, label: id, kind: "image" as const, url: `/${id}.png`, original: true }],
        },
      },
    });
    expect([...latestVisualReviewMessageIds([
      review("old"),
      message("ordinary reply"),
      review("new"),
      review("other", "another-issue"),
    ])]).toEqual(["new", "other"]);
    const stopped = review("blocked");
    expect([...latestVisualReviewMessageIds([
      review("old"),
      { ...stopped, work_update: { ...stopped.work_update, kind: "stopped" as const } },
    ])]).toEqual(["blocked"]);
  });
  it("promotes only the latest copy of one owner question", () => {
    const question = (id: string) => ({
      ...message(id, true),
      work_update: {
        kind: "question" as const,
        issue_id: "issue",
        title: "Review",
        source_id: id,
        request_id: "request",
        occurred_at: "now",
        body: "What next?",
      },
    });
    expect([...latestQuestionMessageIds([question("old"), message("reply"), question("new")])]).toEqual(["new"]);
  });
  it("keeps separate unresolved requests individually actionable", () => {
    const question = (id: string, requestId: string) => ({
      ...message(id, true),
      work_update: {
        kind: "question" as const, issue_id: "issue", title: "Review", source_id: id,
        request_id: requestId, occurred_at: "now", body: `Question ${requestId}`,
      },
    });
    expect([...latestQuestionMessageIds([
      question("old-a", "request-a"),
      question("question-b", "request-b"),
      question("new-a", "request-a"),
    ])]).toEqual(["new-a", "question-b"]);
  });
});
