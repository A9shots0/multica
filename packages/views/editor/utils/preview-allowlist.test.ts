// @vitest-environment node
import { readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";
import {
  TEXT_BASENAMES,
  TEXT_CONTENT_TYPES,
  TEXT_EXTENSIONS,
} from "./preview-allowlist";

const repoRoot = resolve(dirname(fileURLToPath(import.meta.url)), "../../../../");
const jsonPath = resolve(repoRoot, "server/internal/handler/preview_allowlist.json");

interface PreviewAllowlistJSON {
  extensions: string[];
  contentTypes: string[];
  basenames: string[];
}

const json: PreviewAllowlistJSON = JSON.parse(readFileSync(jsonPath, "utf8"));

describe("preview-allowlist: generated TS matches JSON source", () => {
  it("TEXT_EXTENSIONS matches extensions in JSON", () => {
    expect(TEXT_EXTENSIONS.size).toBe(json.extensions.length);
    for (const ext of json.extensions) {
      expect(TEXT_EXTENSIONS.has(ext)).toBe(true);
    }
  });

  it("TEXT_CONTENT_TYPES matches contentTypes in JSON", () => {
    expect(TEXT_CONTENT_TYPES.size).toBe(json.contentTypes.length);
    for (const ct of json.contentTypes) {
      expect(TEXT_CONTENT_TYPES.has(ct)).toBe(true);
    }
  });

  it("TEXT_BASENAMES matches basenames in JSON", () => {
    expect(TEXT_BASENAMES.size).toBe(json.basenames.length);
    for (const b of json.basenames) {
      expect(TEXT_BASENAMES.has(b)).toBe(true);
    }
  });
});
