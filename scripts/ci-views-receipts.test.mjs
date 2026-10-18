import assert from "node:assert/strict";
import test from "node:test";
import { makeReceipt, verifyReceipts } from "./ci-views-receipts.mjs";

const identity = { sha: "a".repeat(40), node: "v22.13.1", run: "42", attempt: "1", total: 4 };
const inventory = Array.from({ length: 4 }, (_, i) => ({ file: `/repo/packages/views/${i}.test.ts` }));
function report(index) {
  return { success: true, numFailedTests: 0, numFailedTestSuites: 0, numPendingTestSuites: 0,
    numTotalTests: 1, numPassedTests: 1, numPendingTests: 0, numTodoTests: 0,
    testResults: [{ name: inventory[index].file, status: "passed",
      assertionResults: [{ status: "passed", fullName: "same title in different files" }] }] };
}
const fresh = () => inventory.map((_, i) => makeReceipt(inventory, report(i), { ...identity, index: i + 1 }));

test("accepts complete coverage and portable cached report paths", () => {
  const receipts = fresh();
  assert.deepEqual(verifyReceipts(receipts, identity), { files: 4, tests: 4, passed: 4, pending: 0, todo: 0 });
  const windows = report(0);
  windows.testResults[0].name = "C:\\old-cache-root\\packages\\views\\0.test.ts";
  assert.deepEqual(makeReceipt(inventory, windows, { ...identity, index: 1 }).selected, receipts[0].selected);
});

test("retains explicit existing skip/todo accounting", () => {
  const input = report(0);
  input.testResults[0].assertionResults.push({ status: "skipped" }, { status: "todo" });
  Object.assign(input, { numTotalTests: 3, numPendingTests: 1, numTodoTests: 1 });
  const receipts = fresh();
  receipts[0] = makeReceipt(inventory, input, { ...identity, index: 1 });
  assert.equal(verifyReceipts(receipts, identity).tests, 6);
});

test("supports failed-only reruns within one immutable run/source", () => {
  const receipts = fresh();
  receipts.push({ ...receipts[1], attempt: "2" });
  assert.equal(verifyReceipts(receipts, { ...identity, attempt: "2" }).files, 4);
  const future = fresh();
  future[0].attempt = "3";
  assert.throws(() => verifyReceipts(future, { ...identity, attempt: "2" }), /future attempt/);
  const staleSource = fresh();
  staleSource.push({ ...staleSource[1], attempt: "2", sha: "b".repeat(40) });
  assert.throws(() => verifyReceipts(staleSource, { ...identity, attempt: "2" }), /stale sha/);
  assert.throws(() => verifyReceipts(fresh(), { ...identity, node: "v22.14.0", attempt: "2" }), /stale node/);
  const upgraded = fresh();
  upgraded.push(...fresh().map((r) => ({ ...r, node: "v22.14.0", attempt: "2" })));
  assert.equal(verifyReceipts(upgraded, { ...identity, node: "v22.14.0", attempt: "2" }).files, 4);
});

test("rejects missing, duplicate and foreign file coverage", () => {
  assert.throws(() => verifyReceipts(fresh().slice(1), identity), /missing shard/);
  const duplicate = fresh();
  duplicate[1].selected = duplicate[0].selected;
  assert.throws(() => verifyReceipts(duplicate, identity), /duplicate covered/);
  const missing = fresh();
  missing[0].selected = [];
  assert.throws(() => verifyReceipts(missing, identity), /empty shard/);
  const foreign = fresh();
  foreign[0].selected = ["packages/views/foreign.test.ts"];
  assert.throws(() => verifyReceipts(foreign, identity), /foreign selected/);
});

test("rejects stale revision, run, attempt and duplicate indexes", () => {
  for (const key of ["sha", "run", "attempt", "total", "node"]) {
    const receipts = fresh();
    receipts[0][key] = "stale";
    assert.throws(() => verifyReceipts(receipts, identity), /stale/);
  }
  const receipts = fresh();
  receipts[0].index = 2;
  assert.throws(() => verifyReceipts(receipts, identity), /duplicate shard/);
});

test("rejects inventory, hash and assertion accounting drift", () => {
  const drift = fresh();
  drift[1].files = drift[1].files.slice(1);
  assert.throws(() => verifyReceipts(drift, identity), /inventory drift/);
  const hash = fresh();
  hash[0].inventoryHash = "bad";
  assert.throws(() => verifyReceipts(hash, identity), /hash drift/);
  const counts = fresh();
  counts[0].counts.tests = 2;
  assert.throws(() => verifyReceipts(counts, identity), /accounting/);
});

test("rejects failed, unfinished, empty and malformed results", () => {
  for (const property of ["numFailedTests", "numFailedTestSuites", "numPendingTestSuites"]) {
    const input = report(0);
    input[property] = 1;
    assert.throws(() => makeReceipt(inventory, input, { ...identity, index: 1 }));
  }
  const input = report(0);
  input.testResults[0].status = "failed";
  assert.throws(() => makeReceipt(inventory, input, { ...identity, index: 1 }), /failed file/);
  input.testResults[0].status = "passed";
  input.testResults[0].assertionResults = [];
  assert.throws(() => makeReceipt(inventory, input, { ...identity, index: 1 }), /empty test/);
  input.testResults = [];
  assert.throws(() => makeReceipt(inventory, input, { ...identity, index: 1 }), /empty shard/);
  assert.throws(() => makeReceipt([...inventory, inventory[0]], report(0), { ...identity, index: 1 }), /duplicate inventory/);
  assert.throws(() => makeReceipt([{ file: "/outside/0.test.ts" }], report(0), { ...identity, index: 1 }), /foreign file/);
});
