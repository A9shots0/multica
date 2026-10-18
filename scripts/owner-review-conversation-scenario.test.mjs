import assert from "node:assert/strict";
import test from "node:test";
import { ownerReviewConversationScenario as scenario } from "./owner-review-conversation-scenario.mjs";

test("the fixture represents a conversation, iterations, and simultaneous issue decisions", () => {
  assert.equal(new Set(scenario.issues.map((issue) => issue.key)).size, 2);
  assert.equal(scenario.timeline.filter((entry) => entry.kind === "result" && entry.issueKey === "checkout").length, 2);
  assert.equal(scenario.timeline.filter((entry) => entry.kind === "result" && entry.issueKey === "profile").length, 2);
  assert.equal(new Set(scenario.timeline.map((entry) => entry.key)).size, scenario.timeline.length);

  const messages = scenario.timeline.filter((entry) => entry.kind === "message");
  assert.ok(messages.filter((entry) => entry.role === "user").length >= 3);
  assert.ok(messages.filter((entry) => entry.role === "assistant").length >= 3);
  assert.deepEqual(scenario.timeline.map((entry) => entry.at), [...scenario.timeline.map((entry) => entry.at)].sort((a, b) => a - b));

  const questions = scenario.timeline.filter((entry) => entry.kind === "question");
  assert.deepEqual(new Set(questions.map((entry) => entry.issueKey)), new Set(["checkout", "profile"]));
  for (const issue of scenario.issues) {
    assert.equal(issue.question.options.length, 3);
    assert.ok(issue.question.body.length > 20);
  }

  const mixedKinds = new Set(scenario.artifacts.map((artifact) => artifact.kind));
  assert.deepEqual(mixedKinds, new Set(["image", "page", "file"]));
  assert.ok(scenario.artifacts.some((artifact) => artifact.kind === "image" && artifact.height > artifact.width));
  assert.ok(scenario.timeline.some((entry) => entry.kind === "result" && entry.artifactKeys.length >= 4));

  const resultIndexes = scenario.timeline
    .map((entry, index) => entry.kind === "result" ? index : -1)
    .filter((index) => index >= 0);
  assert.ok(resultIndexes.some((index) => {
    const first = scenario.timeline[index];
    const second = scenario.timeline[index + 1];
    return second?.kind === "result" && first.issueKey !== second.issueKey;
  }));
});

