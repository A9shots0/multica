// @vitest-environment node
import { describe, it, expect } from "vitest";
import { chatWorkDetails } from "./work-details";
import type { ChatWorkIssue } from "./work";

const issue: ChatWorkIssue = { id: "one", number: 1, title: "Preview", revision: 3, project_id: null, state: "waiting", source: "origin", task_id: null, task_status: "", wait_reason: "", observed_at: "now", summary: { stage: "blocked", current_picture_revision: 2 }, target: { issue_id: "one", issue_revision: 3, owner_id: "owner" }, picture: {} };
describe("recorded work detail", () => {
  it("keeps external dependencies and responsible actors without creating a percentage", () => {
    const details = chatWorkDetails({ ...issue, picture: { contract_version: 2, owner_summary_schema_version: 1, current_milestone: "blocked_external", active_actor: { actor_type: "agent", actor_id: "designer" }, unresolved_dependencies: [{ id: "dep", actor_type: "agent", actor_id: "designer", wait_reason: { description: "Waiting for the reviewed preview" } }], next_automatic_action: { expected_event: { type: "task.completed" } } } });
    expect(details?.dependencies).toEqual([{ id: "dep", actorId: "designer", actorType: "agent", reason: "Waiting for the reviewed preview" }]);
    expect(details?.nextEvent).toBe("task.completed");
  });
  it("does not interpret missing, malformed or future projections", () => {
    expect(chatWorkDetails(issue)).toBeNull();
    expect(chatWorkDetails({ ...issue, picture: { contract_version: 4, owner_summary_schema_version: 1 } })).toBeNull();
    expect(chatWorkDetails({ ...issue, picture: { contract_version: 2, owner_summary_schema_version: 1, unresolved_dependencies: "bad" } })).toBeNull();
    expect(chatWorkDetails({ ...issue, summary: { stage: "active", current_picture_revision: 0 }, picture: { contract_version: 2, owner_summary_schema_version: 1 } })).toBeNull();
  });
  it("preserves missing reasons as unknown instead of printing backend objects", () => {
    expect(chatWorkDetails({ ...issue, picture: { contract_version: 3, owner_summary_schema_version: 1, unresolved_dependencies: [{ id: "dep", wait_reason: { code: "internal_provider_code" } }] } })?.dependencies[0]?.reason).toBeNull();
  });
});
