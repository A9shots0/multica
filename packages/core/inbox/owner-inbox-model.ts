import type { InboxItem } from "../types";

/**
 * The owner's inbox, expressed as the owner reads it.
 *
 * The stored rows are events: a run failed, an execution changed, a workflow
 * asked something. One piece of work routinely produces four of them, so the
 * raw list shows the same issue title four times and reads as chaos. Worse,
 * the event text is written for whoever wrote the emitter — `execution_changed`,
 * `exit status 0xc0000017`, `workflow_incident` — none of which tells the
 * person who has to answer what they are being asked.
 *
 * This module does two things and nothing else: it collapses the events of one
 * piece of work into a single thing to look at, and it says what that thing is
 * in the words the owner would use. Everything visual is built on top of it, so
 * a category added later cannot quietly fall through to a raw dump.
 */

/** What the owner is being asked to do, in order of how much it wants them. */
export type OwnerAskKind =
  /** A question only the owner can answer. Nothing moves until they do. */
  | "decision"
  /** Work halted and did not restart itself. The owner decides whether to continue. */
  | "stopped"
  /** Something broke on its own. Usually not the owner's to fix, but they should see it. */
  | "broke"
  /** Finished or progressed. Nothing is required. */
  | "news";

const ASK_RANK: Record<OwnerAskKind, number> = {
  decision: 0,
  stopped: 1,
  broke: 2,
  news: 3,
};

export interface OwnerAsk {
  kind: OwnerAskKind;
  /** One line, plain English, describing what this wants from the owner. */
  headline: string;
  /** The question or statement itself, as the owner should read it. */
  detail: string;
  /** The raw machine text this was translated from, if any. Shown only on request. */
  technical?: string;
}

export interface OwnerInboxGroup {
  /** Stable identity — the issue when there is one, otherwise the single item. */
  key: string;
  issueId: string | null;
  title: string;
  /** The item that decides what this group is and what its buttons do. */
  lead: InboxItem;
  /** Every item folded into this group, newest first, `lead` included. */
  items: InboxItem[];
  ask: OwnerAsk;
  /** How many runs of this work stopped on their own. Drives "3 runs stopped". */
  failureCount: number;
  unread: boolean;
  /** Newest `created_at` across the group. */
  updatedAt: string;
}

/**
 * Known machine failures, and what actually happened, said plainly.
 *
 * Ordered: the first match wins, so the specific causes come before the
 * generic "a process died" catch-all. A cause that is not listed keeps its raw
 * text rather than being described wrongly — a confident wrong explanation is
 * worse to act on than an honest unknown.
 */
const FAILURE_TRANSLATIONS: ReadonlyArray<readonly [RegExp, string]> = [
  [
    /codex initialize failed[\s\S]*(?:unexpected argument ['"]?--sandbox|codex app-server --listen)/i,
    "The coding agent could not start because Multica used a launch setting this Codex version does not accept. The launch settings need updating, then this work can be started again.",
  ],
  [
    /0xc0000017|not enough memory|out of memory|\boom\b|cannot allocate/i,
    "The machine running this agent ran out of memory, so the run stopped partway. Nothing was lost — it needs starting again.",
  ],
  [
    /quota|rate.?limit|\b429\b|usage limit|credit/i,
    "The AI subscription this agent runs on hit its limit. It can carry on once the limit resets, or on a different subscription.",
  ],
  [
    /context (window|length)|maximum context|too many tokens|prompt is too long/i,
    "The agent was handed more to read at once than it can hold. The work needs splitting up.",
  ],
  [/timed? ?out|deadline exceeded/i, "The run took longer than it was allowed and was stopped."],
  [
    /permission denied|unauthori[sz]ed|forbidden|\b40[13]\b/i,
    "The agent was refused access to something it needed.",
  ],
  [
    /network|econnrefused|enotfound|dns|connection reset/i,
    "The agent could not reach something it needed over the network.",
  ],
  [/exit status|process_failure|signal|panic|crash/i, "The agent's process stopped unexpectedly."],
];

/** Turns a machine failure message into a sentence about what happened. */
export function explainFailure(raw: string | null | undefined): string | null {
  if (!raw) return null;
  for (const [pattern, plain] of FAILURE_TRANSLATIONS) {
    if (pattern.test(raw)) return plain;
  }
  return null;
}

/**
 * The generic text the driver writes when it escalates work that halted without
 * a question of its own. It is not a question, so it must not be presented as
 * one — the owner cannot answer "this work stopped and needs your decision".
 */
const GENERIC_STOP = /^this work stopped and needs your decision/i;

function detailOf(item: InboxItem, key: string): string | undefined {
  const value = item.details?.[key];
  return typeof value === "string" && value.trim().length > 0 ? value.trim() : undefined;
}

/** What one event is asking of the owner, said as the owner would say it. */
export function ownerAsk(item: InboxItem): OwnerAsk {
  const question = detailOf(item, "question");
  const error = detailOf(item, "error") ?? item.body?.trim();

  if (item.type === "external_work_update") {
    const provider = externalProviderName(detailOf(item, "provider"));
    const state = detailOf(item, "reported_state");
    const blocker = detailOf(item, "blocker");
    const change = item.body?.trim() || detailOf(item, "current_milestone") || item.title;
    if (state === "blocked" || state === "stopped" || state === "waiting_external") {
      return {
        kind: "broke",
        headline: `${provider} reported that this work stopped`,
        detail: blocker
          ? `${provider} is waiting on: ${blocker}`
          : `${provider} reported: ${change}`,
      };
    }
    if (state === "waiting_owner") {
      return {
        kind: "news",
        headline: `${provider} says it needs your decision`,
        detail: question || `${provider} says it needs a decision before continuing: ${change}`,
      };
    }
    if (state === "complete") {
      return {
        kind: "news",
        headline: `${provider} reported this work complete`,
        detail: `${provider} reported complete: ${change}`,
      };
    }
    return {
      kind: "news",
      headline: `${provider} sent an update`,
      detail: `${provider} reported: ${change}`,
    };
  }

  if (
    item.type === "task_failed" ||
    item.type === "workflow_incident" ||
    item.type === "agent_blocked"
  ) {
    const plain = explainFailure(error);
    return {
      kind: "broke",
      headline: "A run stopped on its own",
      detail:
        plain ??
        // `execution_changed` is the event's name, not a description of anything.
        (error && !/^execution_changed$/i.test(error)
          ? error
          : "A run on this work ended early. The reason was not recorded in words."),
      technical: error,
    };
  }

  if (item.type === "workflow_owner_decision") {
    // Halted work, not a question. The driver labels it, and the old generic
    // wording is matched too so rows raised before that label existed are read
    // the same way. Either way the owner is offered the only thing that can
    // actually be answered: carry on, or not.
    if (detailOf(item, "required_action") === "decide_stopped_work" || !question || GENERIC_STOP.test(question)) {
      return {
        kind: "stopped",
        headline: "This stopped and hasn't picked itself back up",
        detail:
          question && !GENERIC_STOP.test(question)
            ? question
            : "Nobody is working on this right now. Tell the team to carry on, change what you asked for, or drop it.",
        technical: question,
      };
    }
    return { kind: "decision", headline: "Waiting on your answer", detail: question };
  }

  if (item.type === "chat_intake") {
    return {
      kind: "decision",
      headline: "Should this become a piece of work?",
      detail: detailOf(item, "request") ?? item.body?.trim() ?? item.title,
    };
  }

  if (item.type === "task_completed" || item.type === "quick_create_done") {
    return { kind: "news", headline: "Finished", detail: item.body?.trim() || item.title };
  }

  if (item.type === "autopilot_quota_exceeded") {
    return {
      kind: "broke",
      headline: "An automation ran out of its allowance",
      detail: item.body?.trim() || "An autopilot rule hit its usage limit and stopped running.",
    };
  }

  return {
    kind: "news",
    headline: "An update on this work",
    detail: /^(execution_changed|decision_answered|action_changed|evidence_recorded)$/.test(item.body?.trim() || item.title)
      ? "The work status was updated."
      : item.body?.trim() || item.title,
  };
}

/** True when the buttons should offer to answer, rather than to file away. */
export function wantsAnswer(kind: OwnerAskKind): boolean {
  return kind === "decision" || kind === "stopped";
}

/**
 * The owner Inbox is a control surface, not a system event log. A group belongs
 * there only when the owner can move it forward from the Inbox itself.
 *
 * Most entries qualify because they carry a real question. The small set of
 * issue-less recovery notices qualify because the page renders their bounded
 * recovery action directly. Crashes, completed work, and ordinary updates stay
 * in Chat/issue history and in the investigation system; showing them here
 * would make the owner diagnose or clear work the system owns.
 */
export function needsOwnerAction(group: OwnerInboxGroup): boolean {
  if (group.lead.type === "external_work_update") {
    return detailOf(group.lead, "reported_state") === "waiting_owner";
  }
  if (wantsAnswer(group.ask.kind)) return true;
  return [
    "quick_create_failed",
    "quick_create_unconfirmed",
    "autopilot_quota_exceeded",
  ].includes(group.lead.type);
}

/**
 * Collapses a list of events into one entry per piece of work.
 *
 * Events without an issue cannot be grouped with anything and stand alone.
 * Within a group the lead is the most demanding event, breaking ties by
 * recency, because that is what decides the buttons the owner is offered: an
 * issue that is both waiting on an answer and carrying three crashed runs is
 * first of all waiting on an answer.
 */
export function groupOwnerInbox(items: readonly InboxItem[]): OwnerInboxGroup[] {
  const groups = new Map<string, InboxItem[]>();
  for (const item of items) {
    const key = item.issue_id ?? `item:${item.id}`;
    const bucket = groups.get(key);
    if (bucket) bucket.push(item);
    else groups.set(key, [item]);
  }

  const built: OwnerInboxGroup[] = [];
  for (const [key, bucket] of groups) {
    const sorted = [...bucket].sort(
      (a, b) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime(),
    );
    const lead = [...sorted].sort((a, b) => {
      const rank = ASK_RANK[ownerAsk(a).kind] - ASK_RANK[ownerAsk(b).kind];
      if (rank !== 0) return rank;
      return new Date(b.created_at).getTime() - new Date(a.created_at).getTime();
    })[0]!;
    built.push({
      key,
      issueId: lead.issue_id,
      title: lead.title,
      lead,
      items: sorted,
      ask: ownerAsk(lead),
      failureCount: sorted.filter((i) => i.type === "task_failed").length,
      unread: sorted.some((i) => !i.read),
      updatedAt: sorted[0]!.created_at,
    });
  }

  return built.sort((a, b) => {
    const rank = ASK_RANK[a.ask.kind] - ASK_RANK[b.ask.kind];
    if (rank !== 0) return rank;
    return new Date(b.updatedAt).getTime() - new Date(a.updatedAt).getTime();
  });
}

/**
 * Sentences that are printed on every item of their kind and therefore carry
 * no information. "This issue stays paused until you answer" is true of every
 * unanswered question there has ever been; showing it turns the one line that
 * should be specific into furniture the reader learns to skip.
 */
const EMPTY_CONSEQUENCES = [
  /^this issue stays paused until you answer\.?$/i,
  /^nothing further happens on this issue until you decide\.?$/i,
  /^nothing moves on this until you answer\.?$/i,
];

/** The consequence, if it actually says something this one item will cause. */
export function meaningfulConsequence(text: string | null | undefined): string | null {
  const trimmed = text?.trim();
  if (!trimmed) return null;
  return EMPTY_CONSEQUENCES.some((p) => p.test(trimmed)) ? null : trimmed;
}

/** Several pieces of work in the identical situation, answerable together. */
export interface OwnerInboxBatch {
  key: string;
  kind: OwnerAskKind;
  groups: OwnerInboxGroup[];
  /** The one sentence that is true of all of them. Only set when there is more
   *  than one — a batch of one is just that piece of work. */
  shared: string | null;
}

/**
 * Merges pieces of work that are stuck for the identical reason.
 *
 * Six issues halted by the same machine crash produced six entries reading
 * word for word the same, and the owner correctly read that as the page
 * repeating itself. They are one situation with six subjects, so they are
 * offered as one thing to answer, with the subjects listed underneath.
 *
 * Only ever merges within a kind, and only on an exact match of what the owner
 * would read — two questions that merely resemble each other are two questions.
 */
export function batchOwnerInbox(groups: readonly OwnerInboxGroup[]): OwnerInboxBatch[] {
  const order: string[] = [];
  const buckets = new Map<string, OwnerInboxGroup[]>();
  for (const group of groups) {
    // A decision is never merged: two issues can ask the same words and still
    // want separate answers. Only the automatic stoppages repeat verbatim.
    const key =
      group.ask.kind === "decision" || group.lead.type === "external_work_update"
        ? `single:${group.key}`
        : `${group.ask.kind}:${group.ask.detail}`;
    const bucket = buckets.get(key);
    if (bucket) bucket.push(group);
    else {
      buckets.set(key, [group]);
      order.push(key);
    }
  }
  return order.map((key) => {
    const members = buckets.get(key)!;
    return {
      key,
      kind: members[0]!.ask.kind,
      groups: members,
      shared: members.length > 1 ? members[0]!.ask.detail : null,
    };
  });
}

function externalProviderName(provider: string | undefined): string {
  if (!provider) return "An outside agent";
  const known: Record<string, string> = {
    codex: "Codex",
    claude: "Claude",
    chatgpt: "ChatGPT",
  };
  return known[provider.toLowerCase()] ?? provider;
}

/**
 * A question with lettered parts, split into its parts.
 *
 * Agents write four decisions into one paragraph — "(a) … (b) … (c) … (d) …" —
 * and as prose it is unreadable: the reader has to hold four questions in their
 * head while hunting for where each one ends. As a list each part is one line
 * they can answer.
 *
 * The markers have to run in sequence from (a) or (1) for this to fire. A
 * passing mention of "(c)" further down a sentence is a reference to a part,
 * not the start of a new one, and splitting on it would cut the text in half.
 */
export function splitQuestion(text: string): {
  lead: string;
  parts: { label: string; text: string }[];
} {
  const markers = [...text.matchAll(/\(([a-z0-9])\)/gi)];
  if (markers.length < 2) return { lead: text, parts: [] };

  const labels = markers.map((m) => m[1]!.toLowerCase());
  const first = labels[0]!;
  const start = first === "a" ? "a".charCodeAt(0) : first === "1" ? "1".charCodeAt(0) : 0;
  if (start === 0) return { lead: text, parts: [] };
  const inSequence = labels.every((label, i) => label.charCodeAt(0) === start + i);
  if (!inSequence) return { lead: text, parts: [] };

  const lead = text.slice(0, markers[0]!.index).trim();
  const parts = markers.map((marker, index) => {
    const from = marker.index! + marker[0].length;
    const to = index + 1 < markers.length ? markers[index + 1]!.index! : text.length;
    return {
      label: marker[1]!.toLowerCase(),
      text: text.slice(from, to).trim().replace(/[;,]+$/, ""),
    };
  });
  return { lead, parts };
}

/**
 * The options an agent offered, one per answer.
 *
 * A wrapped sentence arrives as two lines, and split naively it became two
 * buttons — "Decide (a) and (d) first" and "then send me the preview" — neither
 * of which is an answer. A line starting lower-case is the rest of the line
 * above, not a new choice.
 */
export function parseOptions(raw: string | null | undefined): string[] {
  if (!raw) return [];
  const lines = raw
    .split(/\r?\n/)
    .map((line) => line.replace(/^\s*(?:\d+[.)]|[-*•])\s*/, "").trim())
    .filter(Boolean);
  const merged: string[] = [];
  for (const line of lines) {
    const previous = merged[merged.length - 1];
    if (previous && /^[a-z]/.test(line) && !/[.!?]$/.test(previous)) {
      merged[merged.length - 1] = `${previous} ${line}`;
    } else {
      merged.push(line);
    }
  }
  return merged;
}

/**
 * What each option actually does, taken off the end of the consequence field.
 *
 * Agents write the outcomes as one paragraph — "Option 1 gets the board in
 * front of you now… Option 2 delays the preview by one pass… Option 3 holds
 * verified work behind pre-existing defects" — and printed whole it refers to
 * numbers that appear nowhere on the screen. Sentence by sentence it belongs
 * under the button it describes, and the buttons get the numbers.
 *
 * Anything that does not name an option is returned as `rest` rather than
 * dropped: it is usually the sentence that applies to all of them.
 */
export function splitOptionConsequences(text: string | null | undefined): {
  byOption: Map<number, string>;
  rest: string;
} {
  const byOption = new Map<number, string>();
  const trimmed = text?.trim();
  if (!trimmed) return { byOption, rest: "" };

  const markers = [...trimmed.matchAll(/\bOption\s+(\d+)\b[:.]?\s*/gi)];
  if (markers.length === 0) return { byOption, rest: trimmed };

  for (const [index, marker] of markers.entries()) {
    const from = marker.index! + marker[0].length;
    const to = index + 1 < markers.length ? markers[index + 1]!.index! : trimmed.length;
    const body = trimmed.slice(from, to).trim();
    if (body) byOption.set(Number(marker[1]), body);
  }
  return { byOption, rest: trimmed.slice(0, markers[0]!.index).trim() };
}

/** The first sentence, for a line that has to stay one line. */
export function firstSentence(text: string | null | undefined): {
  head: string;
  tail: string;
} {
  const trimmed = text?.trim();
  if (!trimmed) return { head: "", tail: "" };
  const end = trimmed.search(/[.!?](\s|$)/);
  if (end === -1) return { head: trimmed, tail: "" };
  return { head: trimmed.slice(0, end + 1), tail: trimmed.slice(end + 1).trim() };
}

/** How long this has been sitting there, said the way a person would say it. */
export function waitingFor(iso: string, now: number = Date.now()): string {
  const minutes = Math.floor((now - new Date(iso).getTime()) / 60000);
  if (minutes < 1) return "just now";
  if (minutes < 60) return `${minutes}m`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}h`;
  const days = Math.floor(hours / 24);
  return `${days}d`;
}

/* ------------------------------------------------------------------------- *
 * Naming the things the owner is asked to look at
 * ------------------------------------------------------------------------- */

/** One thing that can be opened full screen: a picture, or a page that is
 *  running somewhere and can be loaded live. */
export interface NameableSource {
  id: string;
  /** Whatever it was called where it came from: a filename, or a URL. */
  label: string;
  kind: "image" | "page";
  url: string;
}

/**
 * The width a capture was taken at, if its name says so.
 *
 * Agents name screenshots after the width they were taken at — `R-390.png`,
 * `R-1440.png` — which is precise and useless to the person being asked
 * whether the layout is right. The number is the only real information in
 * those names, so it is pulled out and spent on a sentence instead.
 */
function widthIn(label: string): number | null {
  // An issue key is `SILV-41`, and its number is not a measurement of
  // anything. Stripping it first is what keeps `SILV-1440-notes` from being
  // announced as a monitor while `R-1440` still is one: an issue key is two or
  // more letters, which a capture prefix never is.
  const withoutKey = label.replace(/\b[A-Z]{2,6}-\d+\b/gi, " ");
  const matches = [...withoutKey.matchAll(/(?:^|[^0-9])(\d{3,4})(?:px)?(?![0-9])/g)];
  for (const match of matches) {
    const value = Number(match[1]);
    // Below 280 is not a screen and above 3840 is not a browser window.
    if (value >= 280 && value <= 3840) return value;
  }
  return null;
}

/** What a width means to somebody who owns the business rather than the CSS. */
function screenFor(width: number): string {
  if (width < 520) return "On a phone";
  if (width < 900) return "On a tablet";
  // A 13" laptop is 1280-1366 across; a desk monitor starts at 1440. The line
  // sits between them rather than on a round number.
  if (width < 1400) return "On a laptop";
  return "On your monitor";
}

/**
 * What to call a picture or a page on screen.
 *
 * The rule is that nothing the owner reads may be a filename, a hostname or an
 * address. `R-390` and `100.64.218.108:3135/viral_queue_accounts.html` were
 * both on screen as the only labels distinguishing six things he was asked to
 * compare, and neither says what it is.
 *
 * Words in the name win over numbers in it, because an agent that wrote
 * "phone-detail" meant something a measurement cannot recover.
 */
export function describeSource(source: NameableSource): string {
  const label = source.label.replace(/\.(png|jpe?g|webp|gif|html?)$/i, "");
  const words = label.toLowerCase();

  if (/\bphone\b|\bmobile\b|\bhandset\b/.test(words)) {
    if (/\bdetail\b/.test(words)) return "On a phone — detail";
    if (/\blist\b/.test(words)) return "On a phone — list";
    return "On a phone";
  }
  if (/\btablet\b|\bipad\b/.test(words)) return "On a tablet";
  if (/\bwide\b|\bdesktop\b|\bmonitor\b|\bfull\b/.test(words)) return "On your monitor";
  if (/\blaptop\b|\bmedium\b/.test(words)) return "On a laptop";
  if (/\bnarrow\b|\bsmall\b/.test(words)) return "In a narrow window";

  // Before any measuring: a page's label is its address, and an address is
  // full of numbers that are not widths. `100.64.218.108:3135` was read as a
  // 3135-pixel screen.
  if (source.kind === "page") return "The live page";

  const width = widthIn(label);
  if (width) return screenFor(width);

  // Nothing recognisable: make the name itself readable rather than inventing
  // a meaning. `SILV-55-source-account-layout-approval` becomes
  // `Source account layout approval`.
  const cleaned = label
    .replace(/^[A-Z]{2,6}-\d+[-_\s]*/i, "")
    .replace(/[-_]+/g, " ")
    .replace(/\s+/g, " ")
    .trim();
  if (!cleaned) return source.kind === "image" ? "The picture" : "The live page";
  return cleaned.charAt(0).toUpperCase() + cleaned.slice(1);
}

/**
 * Names for a whole set, with no two the same.
 *
 * Four captures of one screen at four widths can genuinely all be "On your
 * monitor"; when that happens the widths come back as a suffix, because a
 * label that does not tell two things apart is worse than a technical one.
 */
export function describeSources(
  sources: readonly NameableSource[],
): Map<string, string> {
  const named = sources.map((source) => ({ source, name: describeSource(source) }));
  const seen = new Map<string, number>();
  for (const entry of named) seen.set(entry.name, (seen.get(entry.name) ?? 0) + 1);

  const used = new Map<string, number>();
  const out = new Map<string, string>();
  for (const { source, name } of named) {
    if ((seen.get(name) ?? 0) < 2) {
      out.set(source.id, name);
      continue;
    }
    // Only a picture has a capture width. A page's label is its address, and
    // measuring that produced "The live page — 3135px wide" out of a port
    // number.
    const width = source.kind === "image" ? widthIn(source.label) : null;
    if (width) {
      out.set(source.id, `${name} — ${width}px wide`);
      continue;
    }
    const nth = (used.get(name) ?? 0) + 1;
    used.set(name, nth);
    out.set(source.id, `${name} ${nth}`);
  }
  return out;
}

/**
 * Whether a browser will refuse to show this page inside the inbox.
 *
 * A live preview served over plain http cannot be framed by a page served over
 * https — every browser blocks it silently, and the owner gets a blank rectangle
 * with no reason given. The inbox is reached over https from his phone and over
 * http at the desk, so the same preview works in one place and not the other.
 * Saying so, and offering to open it in a tab where it does work, beats a blank
 * frame.
 */
/**
 * What a framed preview is allowed to do.
 *
 * The address comes out of whatever an agent wrote, or out of an HTML file an
 * agent attached, so the frame is never trusted with the tab it sits in:
 * `allow-top-navigation` is absent everywhere, which is what stops a preview
 * page sending the owner's whole inbox somewhere else.
 *
 * `allow-same-origin` is the subtle one. A sandboxed frame that is BOTH
 * same-origin and allowed to run scripts can reach out of the sandbox and into
 * this page — so for an attached preview, which the app serves from its own
 * address, it is withheld and the page runs in an origin of its own. A live
 * preview on somebody else's address cannot reach us whatever we grant, and
 * genuinely needs its own origin to use its own storage, so there it stays.
 */
export function sandboxFor(url: string, pageOrigin: string | undefined): string {
  const base = "allow-scripts allow-forms allow-popups";
  if (!pageOrigin) return base;
  try {
    return new URL(url, pageOrigin).origin === new URL(pageOrigin).origin
      ? base
      : `${base} allow-same-origin`;
  } catch {
    return base;
  }
}

/**
 * Returns the attachment id when a page source is Multica's own HTML-preview
 * route. These pages must be fetched and rendered as srcDoc instead of framing
 * the Next.js route: the app shell reads browser storage, while the secure
 * preview sandbox deliberately has an opaque origin.
 *
 * A look-alike path on another origin is never treated as one of our
 * attachments.
 */
export function attachmentPreviewId(
  url: string,
  pageOrigin: string | undefined,
): string | null {
  const fallbackOrigin = "https://multica.invalid";
  try {
    const parsed = new URL(url, pageOrigin ?? fallbackOrigin);
    const expectedOrigin = new URL(pageOrigin ?? fallbackOrigin).origin;
    if (parsed.origin !== expectedOrigin) return null;
    const match = parsed.pathname.match(
      /\/attachments\/([0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12})\/preview\/?$/i,
    );
    return match?.[1] ?? null;
  } catch {
    return null;
  }
}

export function framingBlocked(url: string, pageOrigin: string | undefined): boolean {
  if (!pageOrigin) return false;
  try {
    return new URL(pageOrigin).protocol === "https:" && new URL(url).protocol === "http:";
  } catch {
    return false;
  }
}
