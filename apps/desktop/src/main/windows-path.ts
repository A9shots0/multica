import { execFileSync } from "node:child_process";
import { win32 } from "node:path";

const MACHINE_ENVIRONMENT_KEY =
  "HKLM\\SYSTEM\\CurrentControlSet\\Control\\Session Manager\\Environment";
const USER_ENVIRONMENT_KEY = "HKCU\\Environment";

export type WindowsRegistryPathReader = (key: string) => string | undefined;

function getEnvironmentValue(
  source: NodeJS.ProcessEnv,
  name: string,
): string | undefined {
  const key = Object.keys(source).find(
    (candidate) => candidate.toUpperCase() === name.toUpperCase(),
  );
  return key ? source[key] : undefined;
}

function expandEnvironmentVariables(
  value: string,
  source: NodeJS.ProcessEnv,
): string {
  return value.replace(/%([^%]+)%/g, (match, name: string) => {
    return getEnvironmentValue(source, name) ?? match;
  });
}

function uniquePathEntries(entries: string[]): string[] {
  const seen = new Set<string>();
  const result: string[] = [];
  for (const entry of entries) {
    const trimmed = entry.trim().replace(/^"|"$/g, "");
    if (!trimmed) continue;
    const key = trimmed.toLowerCase();
    if (seen.has(key)) continue;
    seen.add(key);
    result.push(trimmed);
  }
  return result;
}

function parseRegistryPath(output: string): string | undefined {
  for (const line of output.split(/\r?\n/)) {
    const match = line.match(/^\s*Path\s+REG_(?:EXPAND_)?SZ\s+(.*)$/i);
    if (match?.[1]?.trim()) return match[1].trim();
  }
  return undefined;
}

function createRegistryPathReader(
  source: NodeJS.ProcessEnv,
): WindowsRegistryPathReader {
  const systemRoot = getEnvironmentValue(source, "SystemRoot") ?? "C:\\Windows";
  const regExe = win32.join(systemRoot, "System32", "reg.exe");

  return (key) => {
    try {
      const output = execFileSync(regExe, ["query", key, "/v", "Path"], {
        encoding: "utf8",
        windowsHide: true,
        timeout: 5_000,
      });
      return parseRegistryPath(output);
    } catch {
      return undefined;
    }
  };
}

/**
 * Windows GUI applications inherit Explorer's environment snapshot, which may
 * predate a newly installed agent CLI. Re-read the current machine and user
 * PATH before Desktop starts a child process, then preserve inherited entries
 * as fallbacks. Registry failures are non-fatal.
 */
export function refreshWindowsPath(
  source: NodeJS.ProcessEnv,
  readRegistryPath: WindowsRegistryPathReader = createRegistryPathReader(source),
): string {
  const inherited = getEnvironmentValue(source, "PATH") ?? "";
  const registryValues = [
    readRegistryPath(MACHINE_ENVIRONMENT_KEY),
    readRegistryPath(USER_ENVIRONMENT_KEY),
  ].filter((value): value is string => Boolean(value));

  const entries = [...registryValues, inherited].flatMap((value) =>
    expandEnvironmentVariables(value, source).split(win32.delimiter),
  );
  return uniquePathEntries(entries).join(win32.delimiter);
}

export function setWindowsPath(
  target: NodeJS.ProcessEnv,
  value: string,
): void {
  const pathKeys = Object.keys(target).filter(
    (key) => key.toUpperCase() === "PATH",
  );
  const pathKey = pathKeys[0] ?? "PATH";
  for (const duplicate of pathKeys.slice(1)) delete target[duplicate];
  target[pathKey] = value;
}

export const windowsPathRegistryKeys = {
  machine: MACHINE_ENVIRONMENT_KEY,
  user: USER_ENVIRONMENT_KEY,
} as const;
