import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { execFileSync } from "node:child_process";
import { readFileSync, readdirSync, writeFileSync } from "node:fs";
import { resolve } from "node:path";
import { pathToFileURL } from "node:url";

// Vitest owns selection. These receipts independently require the union of its
// actual JSON results to equal its unfiltered, freshly collected file inventory.
// Turbo may restore a result only with its complete source/dependency task hash.
function fileName(value) {
  assert.equal(typeof value, "string", "file name must be a string");
  const normalized = value.replaceAll("\\", "/");
  const marker = "/packages/views/";
  const index = normalized.lastIndexOf(marker);
  assert.ok(index >= 0, `foreign file: ${value}`);
  const name = normalized.slice(index + 1);
  assert.ok(!name.split("/").includes(".."), "parent traversal");
  assert.match(name, /\.test\.tsx?$/, "unexpected test extension");
  return name;
}

function unique(values, label) {
  assert.equal(new Set(values).size, values.length, `duplicate ${label}`);
  return [...values].sort();
}

export function makeReceipt(inventory, report, identity) {
  assert.match(identity.sha, /^[0-9a-f]{40}$/, "invalid source revision");
  assert.ok(Number.isInteger(identity.index) && identity.index > 0);
  assert.ok(Number.isInteger(identity.total) && identity.total >= identity.index);
  assert.ok(identity.run && identity.attempt, "missing run identity");
  assert.match(identity.node, /^v\d+\.\d+\.\d+$/, "missing resolved Node version");
  const files = unique(inventory.map((entry) => fileName(entry.file)), "inventory file");
  assert.ok(files.length > 0, "empty complete inventory");
  assert.equal(report.success, true, "Vitest did not succeed");
  assert.equal(report.numFailedTests, 0, "failed tests");
  assert.equal(report.numFailedTestSuites, 0, "failed suites");
  assert.equal(report.numPendingTestSuites, 0, "unfinished suites");
  const selected = unique(report.testResults.map((entry) => {
    assert.equal(entry.status, "passed", "failed file");
    assert.ok(entry.assertionResults.length > 0, "empty test file");
    assert.ok(entry.assertionResults.every((test) =>
      ["passed", "pending", "todo", "skipped"].includes(test.status)), "failed assertion");
    return fileName(entry.name);
  }), "selected file");
  assert.ok(selected.length > 0, "empty shard");
  assert.ok(selected.every((name) => files.includes(name)), "foreign selected file");
  const assertions = report.testResults.flatMap((entry) => entry.assertionResults);
  assert.equal(assertions.length, report.numTotalTests, "assertion count mismatch");
  assert.equal(assertions.filter((test) => test.status === "passed").length,
    report.numPassedTests, "passed assertion count mismatch");
  return { schema: 1, ...identity, files, selected,
    inventoryHash: createHash("sha256").update(JSON.stringify(files)).digest("hex"),
    counts: { tests: report.numTotalTests, passed: report.numPassedTests,
      pending: report.numPendingTests, todo: report.numTodoTests } };
}

export function verifyReceipts(receipts, identity) {
  assert.match(identity.sha, /^[0-9a-f]{40}$/, "invalid source revision");
  assert.ok(identity.run && identity.attempt, "missing run identity");
  assert.match(identity.node, /^v\d+\.\d+\.\d+$/, "missing resolved Node version");
  assert.match(identity.attempt, /^[1-9][0-9]*$/, "invalid current attempt");
  assert.ok(Number.isInteger(identity.total) && identity.total > 0);
  // A failed-only rerun leaves successful shards in an earlier attempt of the
  // same immutable workflow run/source. Accept those, never another run/SHA.
  // The native matrix result is also independently required by `frontend`.
  const latest = new Map();
  const seen = new Set();
  for (const receipt of receipts) {
    for (const key of ["sha", "run", "total"])
      assert.equal(receipt[key], identity[key], `stale ${key}`);
    assert.match(receipt.attempt, /^[1-9][0-9]*$/, "stale attempt");
    assert.ok(Number(receipt.attempt) <= Number(identity.attempt), "future attempt");
    assert.ok(Number.isInteger(receipt.index) && receipt.index > 0 && receipt.index <= identity.total,
      "invalid shard index");
    const key = `${receipt.index}:${receipt.attempt}`;
    assert.ok(!seen.has(key), "duplicate shard attempt");
    seen.add(key);
    if (!latest.has(receipt.index) || Number(latest.get(receipt.index).attempt) < Number(receipt.attempt))
      latest.set(receipt.index, receipt);
  }
  receipts = [...latest.values()];
  assert.equal(receipts.length, identity.total, "missing shard receipt");
  const indexes = unique(receipts.map((r) => r.index), "shard index");
  assert.deepEqual(indexes, Array.from({ length: identity.total }, (_, i) => i + 1));
  const first = receipts[0];
  const all = [];
  for (const receipt of receipts) {
    assert.equal(receipt.schema, 1, "unsupported receipt schema");
    for (const key of ["sha", "run", "total", "node"])
      assert.equal(receipt[key], identity[key], `stale ${key}`);
    const inventory = unique(receipt.files, "inventory file");
    assert.deepEqual(inventory, first.files, "inventory drift");
    assert.equal(receipt.inventoryHash,
      createHash("sha256").update(JSON.stringify(inventory)).digest("hex"), "inventory hash drift");
    assert.ok(receipt.selected.length > 0, "empty shard");
    assert.ok(receipt.selected.every((file) => inventory.includes(file)), "foreign selected file");
    assert.ok(Number.isInteger(receipt.counts.tests) && receipt.counts.tests > 0);
    assert.ok(Number.isInteger(receipt.counts.passed) && receipt.counts.passed >= 0);
    assert.ok(Number.isInteger(receipt.counts.pending) && receipt.counts.pending >= 0);
    assert.ok(Number.isInteger(receipt.counts.todo) && receipt.counts.todo >= 0);
    assert.equal(receipt.counts.passed + receipt.counts.pending + receipt.counts.todo,
      receipt.counts.tests, "incomplete assertion accounting");
    all.push(...receipt.selected);
  }
  assert.deepEqual(unique(all, "covered file"), first.files, "missing test file");
  return { files: all.length,
    tests: receipts.reduce((sum, r) => sum + r.counts.tests, 0),
    passed: receipts.reduce((sum, r) => sum + r.counts.passed, 0),
    pending: receipts.reduce((sum, r) => sum + r.counts.pending, 0),
    todo: receipts.reduce((sum, r) => sum + r.counts.todo, 0) };
}

if (process.argv[1] && import.meta.url === pathToFileURL(resolve(process.argv[1])).href) {
  const [mode, indexArg, totalArg] = process.argv.slice(2);
  const sha = execFileSync("git", ["rev-parse", "HEAD"], { encoding: "utf8" }).trim();
  assert.equal(sha, process.env.GITHUB_SHA, "checkout differs from workflow revision");
  const identity = { sha, node: process.version, run: process.env.GITHUB_RUN_ID, attempt: process.env.GITHUB_RUN_ATTEMPT,
    total: Number(totalArg), index: Number(indexArg) };
  const read = (path) => JSON.parse(readFileSync(path, "utf8"));
  if (mode === "create") {
    const receipt = makeReceipt(read("packages/views/.ci-vitest/inventory.json"),
      read("packages/views/.ci-vitest/results.json"), identity);
    writeFileSync(`ci-views-receipt-${identity.index}.json`, JSON.stringify(receipt, null, 2));
    console.log(JSON.stringify({ index: identity.index, files: receipt.selected.length, ...receipt.counts }));
  } else {
    assert.equal(mode, "verify", "unknown receipt operation");
    const receipts = readdirSync("ci-views-receipts", { recursive: true })
      .filter((name) => /^ci-views-receipt-\d+\.json$/.test(name.split(/[\\/]/).at(-1)))
      .map((name) => read(resolve("ci-views-receipts", name)));
    console.log(JSON.stringify(verifyReceipts(receipts, identity)));
  }
}
