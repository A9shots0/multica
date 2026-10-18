import { describe, expect, it } from "vitest";
import { summarizeActivity } from "./activity-summary";

describe("activity summary", () => {
  it("counts requests once without treating failed results as completed work", () => {
    expect(summarizeActivity([
      { seq: 1, type: "tool_use", tool: "functions.exec_command", input: { cmd: "multica issue create" } },
      { seq: 2, type: "tool_result", output: "server error" },
      { seq: 3, type: "error", content: "failed" },
      { seq: 4, type: "text", content: "Trying again" },
      { seq: 5, type: "tool_use", tool: "exec_command" },
    ])).toEqual([{ kind: "commands", count: 2 }]);
  });
  it("does not infer actions from arbitrary command text or unknown tools", () => {
    expect(summarizeActivity([
      { seq: 1, type: "tool_use", tool: "new_tool", input: { command: "write file; completed" } },
      { seq: 2, type: "tool_use", tool: "Read" },
      { seq: 3, type: "tool_use", tool: "apply_patch" },
    ])).toEqual([{ kind: "other", count: 1 }, { kind: "reading", count: 1 }, { kind: "editing", count: 1 }]);
  });
});
