// @vitest-environment node

import { describe, expect, it, vi } from "vitest";
import {
  refreshWindowsPath,
  setWindowsPath,
  windowsPathRegistryKeys,
} from "./windows-path";

describe("refreshWindowsPath", () => {
  it("uses current machine and user PATH values before a stale inherited PATH", () => {
    const read = vi.fn((key: string) => {
      if (key === windowsPathRegistryKeys.machine) return "C:\\Windows\\System32";
      if (key === windowsPathRegistryKeys.user) return "%APPDATA%\\npm";
      return undefined;
    });

    const result = refreshWindowsPath(
      {
        PATH: "C:\\Windows",
        APPDATA: "C:\\Users\\Drew\\AppData\\Roaming",
      },
      read,
    );

    expect(result.split(";")).toEqual([
      "C:\\Windows\\System32",
      "C:\\Users\\Drew\\AppData\\Roaming\\npm",
      "C:\\Windows",
    ]);
    expect(read).toHaveBeenCalledTimes(2);
  });

  it("expands registry variables case-insensitively", () => {
    const result = refreshWindowsPath(
      { USERPROFILE: "C:\\Users\\Drew" },
      (key) =>
        key === windowsPathRegistryKeys.user
          ? "%userprofile%\\.local\\bin"
          : undefined,
    );

    expect(result).toBe("C:\\Users\\Drew\\.local\\bin");
  });

  it("deduplicates entries case-insensitively", () => {
    const result = refreshWindowsPath(
      { PATH: "C:\\TOOLS;C:\\Windows" },
      (key) => (key === windowsPathRegistryKeys.user ? "c:\\tools" : undefined),
    );

    expect(result.split(";")).toEqual(["c:\\tools", "C:\\Windows"]);
  });

  it("preserves inherited PATH when registry queries fail", () => {
    const read = vi.fn(() => undefined);

    expect(refreshWindowsPath({ PATH: "C:\\Windows;C:\\Tools" }, read)).toBe(
      "C:\\Windows;C:\\Tools",
    );
    expect(read).toHaveBeenCalledTimes(2);
  });

  it("updates the existing Windows Path key without creating a duplicate", () => {
    const env: NodeJS.ProcessEnv = { Path: "stale" };

    setWindowsPath(env, "fresh");

    expect(env).toEqual({ Path: "fresh" });
  });
});
