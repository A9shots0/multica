export interface HubDocSection {
  id: string;
  label: string;
  content: string;
}

function slugify(label: string): string {
  const base = label
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/(^-|-$)/g, "");
  return base || "section";
}

function uniqueId(base: string, used: Set<string>): string {
  let id = base;
  let n = 2;
  while (used.has(id)) {
    id = `${base}-${n++}`;
  }
  used.add(id);
  return id;
}

/**
 * CONTEXT.md convention (see server/internal/handler/knowledge.go's
 * knowledgeHubRoleSummary, which parses the same structure): a flat list of
 * top-level `- Label:` bullets, each followed by indented body lines — no
 * markdown headings. Splits on those top-level bullets so the reader gets
 * "chapters" (Role, Repository, Deployment/runtime, ...) instead of one
 * undifferentiated wall of nested bullets.
 */
export function parseContextSections(markdown: string): HubDocSection[] {
  const lines = markdown.split(/\r?\n/);
  const sections: HubDocSection[] = [];
  const used = new Set<string>();
  let current: { label: string; lines: string[] } | null = null;
  const topLevelBullet = /^- (.+):\s*$/;

  const flush = () => {
    if (!current) return;
    const body = current.lines
      .map((l) => (l.startsWith("  ") ? l.slice(2) : l))
      .join("\n")
      .trim();
    if (body) {
      sections.push({ id: uniqueId(slugify(current.label), used), label: current.label, content: body });
    }
  };

  for (const line of lines) {
    const match = !line.startsWith(" ") && line.match(topLevelBullet);
    if (match) {
      flush();
      current = { label: match[1]!.trim(), lines: [] };
    } else if (current) {
      current.lines.push(line);
    }
  }
  flush();
  return sections;
}

/**
 * CHANGELOG.md convention across the hub: dated `## <date> - <title>`
 * entries, newest first. Splits on those H2 headings into one section per
 * entry.
 */
export function parseChangelogEntries(markdown: string): HubDocSection[] {
  const lines = markdown.split(/\r?\n/);
  const sections: HubDocSection[] = [];
  const used = new Set<string>();
  let current: { label: string; lines: string[] } | null = null;
  const heading = /^##\s+(.+?)\s*$/;

  const flush = () => {
    if (!current) return;
    const body = current.lines.join("\n").trim();
    if (body) {
      sections.push({ id: uniqueId(slugify(current.label), used), label: current.label, content: body });
    }
  };

  for (const line of lines) {
    const match = line.match(heading);
    if (match) {
      flush();
      current = { label: match[1]!.trim(), lines: [] };
    } else if (current) {
      current.lines.push(line);
    }
  }
  flush();
  return sections;
}
