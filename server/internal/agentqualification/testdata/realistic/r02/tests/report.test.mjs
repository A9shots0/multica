import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { canonicalTotal } from "../report/report.js";

const data = JSON.parse(await readFile(new URL("../report/data.json", import.meta.url)));
assert.equal(canonicalTotal(data), 47, "total counts each published post once");
assert.equal(data.published_posts.length, 4, "all four real posts remain available");
