// @vitest-environment node
import { describe, expect, it } from "vitest";
import { ChatWorkSchema, chatWorkDraftKey, chatWorkRequestResolved, seedChatWorkDraft, selectChatWorkOption, workCounts, sortWork, type ChatWorkIssue } from "./work";
import { parseWithFallback } from "../api/schema";

const issue = (state: string, number = 1): ChatWorkIssue => ({ id: String(number), number, title: "Audit", revision: 1, project_id: null, state, source: "origin", task_id: null, task_status: "", wait_reason: "", observed_at: "2026-09-23T00:00:00Z", summary: { stage: "active", current_picture_revision: 1 }, target: { issue_id: String(number), issue_revision: 1, owner_id: "owner" }, picture: {} });
describe("conversation work", () => {
  it("keeps a separate draft for General chat and every exact pending request", () => {
    expect(chatWorkDraftKey("chat-a", { issue_id: "issue-1", issue_revision: 2, owner_id: "owner", request_id: "question-1" })).toBe("chat-a::work::question-1");
    expect(chatWorkDraftKey("chat-a", { issue_id: "issue-2", issue_revision: 3, owner_id: "owner", request_id: "question-2" })).toBe("chat-a::work::question-2");
  });
  it("seeds a suggested reply without overwriting or duplicating an existing request draft", () => {
    expect(seedChatWorkDraft(undefined, "  Inspect more evidence  ")).toBe("Inspect more evidence");
    expect(seedChatWorkDraft("My edited answer", "Inspect more evidence")).toBe("My edited answer");
  });
  it("changes an authored choice without discarding the member's explanation", () => {
    const options = ["Lock this direction", "Do one more pass", "Pause this work"];
    expect(selectChatWorkOption(undefined, options, options[0]!)).toBe("Lock this direction\n\n");
    expect(selectChatWorkOption("Lock this direction\n\nOnly on desktop", options, options[1]!))
      .toBe("Do one more pass\n\nOnly on desktop");
    expect(selectChatWorkOption("A completely custom response", options, options[2]!))
      .toBe("A completely custom response");
  });
  it("accepts older snapshots and fails closed on malformed catch-up identities", () => {
    const packet = { observed_at: "now", issues: [], activity: [], has_older: false };
    expect(ChatWorkSchema.safeParse(packet).success).toBe(true);
    const update = {message_id:"a2b38f43-5273-cd2b-3475-883a4d38b903",issue_id:"issue",title:"Audit",kind:"result",body:"Report",occurred_at:"now"};
    expect(ChatWorkSchema.parse({...packet,catch_up:[update]}).catch_up).toHaveLength(1);
    expect(ChatWorkSchema.safeParse({...packet,catch_up:[{...update,message_id:12}]}).success).toBe(false);
  });
  it("counts mutually exclusive issue states, not Mika turns", () => {
    expect(workCounts({ observed_at: "now", issues: [issue("working"), issue("needs_you"), issue("waiting")], activity: [], has_older: false })).toEqual({ total: 3, working: 1, needsYou: 1, waiting: 1, done: 0, stopped: 0, unavailable: 0 });
  });
  it("makes unknown server states unavailable", () => {
    expect(workCounts({ observed_at: "now", issues: [issue("new-state")], activity: [], has_older: false }).unavailable).toBe(1);
  });
  it("rejects malformed packets without inventing an empty success", () => {
    expect(parseWithFallback({ issues: [{ state: "working" }] }, ChatWorkSchema, null, { endpoint: "chat-work", redactReceived: true })).toBeNull();
  });
  it("keeps stable order within attention tiers", () => {
    expect(sortWork([issue("done", 1), issue("working", 4), issue("needs_you", 3), issue("working", 2)]).map((i) => i.number)).toEqual([3,2,4,1]);
  });
  it("preserves explicit question targets and rejects malformed choices", () => {
    const target = { ...issue("needs_you").target, request_id: "question-2", intake_id: "question-2" };
    const packet = { observed_at: "now", issues: [{ ...issue("needs_you"), target: { ...target, needs_request_selection: true }, pending_requests: [{ id: "question-2", question: "Which deadline?", target }] }], activity: [], has_older: false };
    expect(ChatWorkSchema.parse(packet).issues[0]?.pending_requests?.[0]?.target.request_id).toBe("question-2");
    expect(ChatWorkSchema.safeParse({ ...packet, issues: [{ ...packet.issues[0], pending_requests: [{ id: "question-2" }] }] }).success).toBe(false);
  });
  it("preserves canonical request options for an inline Chat decision", () => {
    const packet = {
      observed_at: "now", issues: [], activity: [], has_older: false,
      requests: [{
        id: "question-2", workspace_id: "workspace", recipient_type: "member", recipient_id: "owner",
        type: "workflow_owner_decision", severity: "action_required", issue_id: "1", title: "Choose",
        body: "Which deadline?", issue_status: "open", issue_priority: null, read: false, archived: false,
        created_at: "2026-09-23T00:00:00Z", details: { options: "Today\nTomorrow", recommendation: "Today" },
      }],
    };
    const request = ChatWorkSchema.parse(packet).requests?.[0];
    expect(request?.details).toEqual({ options: "Today\nTomorrow", recommendation: "Today" });
  });
  it("clears an answered target without silently retargeting a different question", () => {
    const selected = { ...issue("needs_you"), target: { ...issue("needs_you").target, request_id: "question-1" } };
    const snapshot = { observed_at: "now", issues: [{ ...selected, revision: 2, pending_requests: [{ id: "question-2", question: "Another question", target: { ...selected.target, request_id: "question-2" } }] }], activity: [], has_older: false };
    expect(chatWorkRequestResolved(selected, snapshot)).toBe(true);
    expect(chatWorkRequestResolved(selected, null)).toBe(false);
    expect(chatWorkRequestResolved(selected, { ...snapshot, issues: [] })).toBe(false);
    expect(chatWorkRequestResolved(selected, { ...snapshot, issues: [{ ...selected, revision: 3 }] })).toBe(false);
    expect(chatWorkRequestResolved(selected, { ...snapshot, issues: [{ ...selected, revision: 3, pending_requests: [{ id: "question-1", question: "Original question", target: selected.target }] }] })).toBe(false);
  });
});
