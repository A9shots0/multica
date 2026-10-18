import { describe, expect, it } from "vitest";
import type { InboxItem } from "../types";
import {
  attachmentPreviewId,
  batchOwnerInbox,
  describeSource,
  describeSources,
  firstSentence,
  framingBlocked,
  sandboxFor,
  splitOptionConsequences,
  explainFailure,
  groupOwnerInbox,
  needsOwnerAction,
  ownerAsk,
  parseOptions,
  splitQuestion,
  waitingFor,
} from "./owner-inbox-model";

function item(over: Partial<InboxItem> & Pick<InboxItem, "id" | "type">): InboxItem {
  return {
    workspace_id: "w",
    recipient_type: "member",
    recipient_id: "m",
    actor_type: "agent",
    actor_id: "a",
    severity: "info",
    issue_id: null,
    title: "A piece of work",
    body: null,
    issue_status: null,
    read: true,
    archived: false,
    created_at: "2026-09-21T12:00:00Z",
    details: null,
    ...over,
  } as InboxItem;
}

describe("ownerAsk", () => {
  it("does not present a workflow event code as an update description", () => {
    expect(ownerAsk(item({id: "update", type: "status_changed", body: "execution_changed"})).detail).toBe("The work status was updated.");
  });
  // The whole point of the rewrite: no branch may fall through to raw machine
  // text. Every type the owner can actually receive is named here, so adding a
  // type without deciding how the owner reads it fails this test rather than
  // shipping a dump.
  const every: Array<[string, InboxItem, string]> = [
    [
      "a real question",
      item({
        id: "1",
        type: "workflow_owner_decision",
        details: { question: "Approve the wide-desktop layout?" },
      }),
      "decision",
    ],
    [
      "a stop with no question",
      item({
        id: "2",
        type: "workflow_owner_decision",
        details: { question: "This work stopped and needs your decision." },
      }),
      "stopped",
    ],
    [
      "a crashed run",
      item({
        id: "3",
        type: "task_failed",
        details: { error: "claude exited with error: exit status 0xc0000017" },
      }),
      "broke",
    ],
    ["an execution change", item({ id: "4", type: "workflow_incident", body: "execution_changed" }), "broke"],
    ["a completion", item({ id: "5", type: "task_completed", body: "Shipped." }), "news"],
    ["an unknown type", item({ id: "6", type: "mentioned" as InboxItem["type"] }), "news"],
  ];

  for (const [name, subject, kind] of every) {
    it(`reads ${name} as ${kind}, in words`, () => {
      const ask = ownerAsk(subject);
      expect(ask.kind).toBe(kind);
      expect(ask.detail.length).toBeGreaterThan(0);
      // Nothing the owner reads may be an event name or a machine code.
      expect(ask.detail).not.toMatch(/execution_changed|0xc0{5}|exit status/);
    });
  }

  it("does not present the driver's generic escalation as a question", () => {
    const ask = ownerAsk(
      item({
        id: "s",
        type: "workflow_owner_decision",
        details: { question: "This work stopped and needs your decision." },
      }),
    );
    expect(ask.kind).toBe("stopped");
    expect(ask.detail).not.toMatch(/needs your decision/i);
  });

  it("keeps an unrecognised failure verbatim rather than describing it wrongly", () => {
    expect(explainFailure("something nobody has seen before")).toBeNull();
    const ask = ownerAsk(
      item({ id: "u", type: "task_failed", details: { error: "something nobody has seen before" } }),
    );
    expect(ask.detail).toBe("something nobody has seen before");
  });

  it("names the out-of-memory crash as what it is", () => {
    expect(explainFailure("claude exited with error: exit status 0xc0000017")).toMatch(
      /ran out of memory/i,
    );
  });

  it("explains a Codex app-server launch mismatch without losing the diagnostic", () => {
    const raw =
      "codex initialize failed: codex process exited; codex stderr: error: unexpected argument '--sandbox' found Usage: codex app-server --listen <URL>";
    const ask = ownerAsk(
      item({ id: "codex-start", type: "task_failed", details: { error: raw } }),
    );

    expect(ask.detail).toBe(
      "The coding agent could not start because Multica used a launch setting this Codex version does not accept. The launch settings need updating, then this work can be started again.",
    );
    expect(ask.technical).toBe(raw);
    expect(ask.detail).not.toMatch(/stderr|--sandbox|app-server|Usage:/i);
  });

  it("labels outside completion as a provider report, not verified company completion", () => {
    const ask = ownerAsk(
      item({
        id: "external-complete",
        type: "external_work_update",
        body: "The importer is merged and its checks passed.",
        details: { provider: "codex", reported_state: "complete" },
      }),
    );

    expect(ask.kind).toBe("news");
    expect(ask.detail).toBe("Codex reported complete: The importer is merged and its checks passed.");
    expect(ask.detail).not.toMatch(/verified/i);
  });

  it("makes an outside blocker understandable without pretending Multica can restart it", () => {
    const ask = ownerAsk(
      item({
        id: "external-blocked",
        type: "external_work_update",
        body: "The release could not continue.",
        details: {
          provider: "claude",
          reported_state: "blocked",
          blocker: "The deployment account is unavailable.",
        },
      }),
    );

    expect(ask).toMatchObject({
      kind: "broke",
      detail: "Claude is waiting on: The deployment account is unavailable.",
    });
  });
});

describe("groupOwnerInbox", () => {
  it("shows one entry per piece of work, led by the most demanding event", () => {
    const groups = groupOwnerInbox([
      item({
        id: "fail-1",
        type: "task_failed",
        issue_id: "issue-a",
        created_at: "2026-09-21T12:05:00Z",
        details: { error: "exit status 0xc0000017" },
      }),
      item({
        id: "ask-1",
        type: "workflow_owner_decision",
        issue_id: "issue-a",
        created_at: "2026-09-21T12:00:00Z",
        details: { question: "Approve the layout?" },
      }),
      item({
        id: "fail-2",
        type: "task_failed",
        issue_id: "issue-a",
        created_at: "2026-09-21T12:02:00Z",
        details: { error: "exit status 0xc0000017" },
      }),
    ]);

    expect(groups).toHaveLength(1);
    // A newer crash must not bury the question: the question is what stops the work.
    expect(groups[0]!.lead.id).toBe("ask-1");
    expect(groups[0]!.ask.kind).toBe("decision");
    expect(groups[0]!.failureCount).toBe(2);
    expect(groups[0]!.items).toHaveLength(3);
  });

  it("puts what waits on the owner above what does not", () => {
    const groups = groupOwnerInbox([
      item({ id: "done", type: "task_completed", issue_id: "b", created_at: "2026-09-21T13:00:00Z" }),
      item({
        id: "ask",
        type: "workflow_owner_decision",
        issue_id: "c",
        created_at: "2026-09-21T09:00:00Z",
        details: { question: "Approve?" },
      }),
    ]);
    expect(groups.map((g) => g.ask.kind)).toEqual(["decision", "news"]);
  });

  it("keeps events without an issue as their own entries", () => {
    const groups = groupOwnerInbox([
      item({ id: "x", type: "autopilot_quota_exceeded" }),
      item({ id: "y", type: "autopilot_quota_exceeded" }),
    ]);
    expect(groups).toHaveLength(2);
  });

  it("marks a group unread when any of its events is unread", () => {
    const groups = groupOwnerInbox([
      item({ id: "r", type: "task_failed", issue_id: "d", read: true }),
      item({ id: "u", type: "task_failed", issue_id: "d", read: false }),
    ]);
    expect(groups[0]!.unread).toBe(true);
  });
});

describe("needsOwnerAction", () => {
  it("keeps decisions and bounded recovery controls, but not failures or updates", () => {
    const groups = groupOwnerInbox([
      item({ id: "ask", type: "workflow_owner_decision", issue_id: "ask", details: { question: "Approve?" } }),
      item({ id: "failed", type: "task_failed", issue_id: "failed", details: { error: "process_failure" } }),
      item({ id: "done", type: "task_completed", issue_id: "done" }),
      item({ id: "retry", type: "quick_create_failed" }),
      item({ id: "quota", type: "autopilot_quota_exceeded" }),
    ]);

    expect(groups.filter(needsOwnerAction).map((group) => group.lead.id)).toEqual([
      "ask",
      "quota",
      "retry",
    ]);
  });

  it("keeps routine outside reports out of the owner action queue", () => {
    const groups = groupOwnerInbox([
      item({
        id: "outside",
        type: "external_work_update",
        issue_id: "outside-work",
        details: { provider: "codex", reported_state: "complete" },
      }),
    ]);

    expect(groups.filter(needsOwnerAction)).toEqual([]);
  });

  it("keeps an outside provider's real owner question in the action queue", () => {
    const groups = groupOwnerInbox([
      item({
        id: "outside-question",
        type: "external_work_update",
        issue_id: "outside-work",
        details: {
          provider: "codex",
          reported_state: "waiting_owner",
          question: "Should this replace the existing workflow?",
        },
      }),
    ]);

    expect(groups.filter(needsOwnerAction).map((group) => group.lead.id)).toEqual(["outside-question"]);
  });
});

describe("batchOwnerInbox", () => {
  it("never merges reports from separate outside work streams", () => {
    const groups = groupOwnerInbox([
      item({
        id: "a",
        type: "external_work_update",
        issue_id: "a",
        body: "Same update",
        details: { provider: "codex", reported_state: "complete" },
      }),
      item({
        id: "b",
        type: "external_work_update",
        issue_id: "b",
        body: "Same update",
        details: { provider: "codex", reported_state: "complete" },
      }),
    ]);

    expect(batchOwnerInbox(groups)).toHaveLength(2);
  });
});

describe("splitQuestion", () => {
  // Taken verbatim from SILV's Schedule Cards question, which arrived as one
  // paragraph holding four separate decisions.
  const four =
    "Schedule Cards is built and verified. Four calls are yours: (a) the card is 280px on the board but 374px on a phone - does your 280px rule apply on phone? (b) Declined and Posted paint the same mark, as the design spec asks - keep or separate? (c) defects on /review, /devices, /watch, /publishing are pre-existing on main and not from this work - block this, or their own issue? (d) sign-off on the new card copy, including whether a VA should read a raw device id.";

  it("pulls four decisions out of one paragraph", () => {
    const { lead, parts } = splitQuestion(four);
    expect(lead).toBe("Schedule Cards is built and verified. Four calls are yours:");
    expect(parts.map((p) => p.label)).toEqual(["a", "b", "c", "d"]);
    expect(parts[0]!.text).toMatch(/^the card is 280px/);
    expect(parts[3]!.text).toMatch(/raw device id\.$/);
  });

  it("leaves prose alone when the markers are not a list", () => {
    // A later reference to an earlier part is not the start of a new one, and
    // cutting the sentence there would lose half of it.
    const prose = "Send the preview now and track (c) separately, then revisit (a).";
    expect(splitQuestion(prose).parts).toHaveLength(0);
    expect(splitQuestion(prose).lead).toBe(prose);
  });

  it("leaves a single marker alone", () => {
    expect(splitQuestion("Approve the layout (a) as drawn.").parts).toHaveLength(0);
  });
});

describe("parseOptions", () => {
  it("keeps a wrapped option as one option", () => {
    // Split naively this produced a button reading "then send me the preview",
    // which is not an answer to anything.
    expect(parseOptions("Decide (a) and (d) first\nthen send me the preview")).toEqual([
      "Decide (a) and (d) first then send me the preview",
    ]);
  });

  it("keeps genuinely separate options apart", () => {
    expect(parseOptions("1. Approve it\n2. Hold it back")).toEqual(["Approve it", "Hold it back"]);
  });

  it("starts a new option after a finished sentence", () => {
    expect(parseOptions("Ship it now.\nhold it until Monday")).toEqual([
      "Ship it now.",
      "hold it until Monday",
    ]);
  });
});

describe("splitOptionConsequences", () => {
  // Verbatim from SILV-41. Printed whole on the card it referred to "Option 1",
  // "Option 2" and "Option 3" while nothing on screen carried a number.
  const text =
    "Option 1 gets the board in front of you now and splits the unrelated defects into their own issue. Option 2 delays the preview by one pass. Option 3 holds verified work behind pre-existing defects on four other surfaces.";

  it("gives each option its own sentence", () => {
    const { byOption, rest } = splitOptionConsequences(text);
    expect(byOption.get(1)).toMatch(/^gets the board in front of you now/);
    expect(byOption.get(2)).toBe("delays the preview by one pass.");
    expect(byOption.get(3)).toMatch(/^holds verified work/);
    expect(rest).toBe("");
  });

  it("keeps text that names no option", () => {
    const { byOption, rest } = splitOptionConsequences("Nothing ships until you answer.");
    expect(byOption.size).toBe(0);
    expect(rest).toBe("Nothing ships until you answer.");
  });

  it("keeps a shared opening alongside the per-option parts", () => {
    const { byOption, rest } = splitOptionConsequences(
      "Either way the branch is kept. Option 1 ships today. Option 2 waits.",
    );
    expect(rest).toBe("Either way the branch is kept.");
    expect(byOption.get(1)).toBe("ships today.");
  });
});

describe("firstSentence", () => {
  it("separates the advice from the reasoning behind it", () => {
    const { head, tail } = firstSentence(
      "Send the preview now and track (c) separately. The Schedule work is verified and (a) (b) (d) are easier to answer looking at the real board.",
    );
    expect(head).toBe("Send the preview now and track (c) separately.");
    expect(tail).toMatch(/^The Schedule work is verified/);
  });

  it("leaves a one-sentence recommendation whole", () => {
    expect(firstSentence("Approve it")).toEqual({ head: "Approve it", tail: "" });
  });
});

describe("waitingFor", () => {
  const now = new Date("2026-09-21T12:00:00Z").getTime();
  it("counts in the units a person would use", () => {
    expect(waitingFor("2026-09-21T11:59:40Z", now)).toBe("just now");
    expect(waitingFor("2026-09-21T11:30:00Z", now)).toBe("30m");
    expect(waitingFor("2026-09-21T06:00:00Z", now)).toBe("6h");
    expect(waitingFor("2026-09-18T12:00:00Z", now)).toBe("3d");
  });
});

describe("describeSource", () => {
  const image = (label: string) => ({ id: label, label, kind: "image" as const, url: "u" });
  const page = (url: string) => ({ id: url, label: url, kind: "page" as const, url });

  it("turns a capture width into the screen it stands for", () => {
    // Verbatim from SILV-55: these four filenames were the only thing on screen
    // telling six previews apart.
    expect(describeSource(image("R-390.png"))).toBe("On a phone");
    expect(describeSource(image("R-1340.png"))).toBe("On a laptop");
    expect(describeSource(image("R-1440.png"))).toBe("On your monitor");
    expect(describeSource(image("R-768.png"))).toBe("On a tablet");
  });

  it("prefers the words an agent chose over a number it also wrote", () => {
    expect(describeSource(image("deployment-review-phone-list.png"))).toBe("On a phone — list");
    expect(describeSource(image("deployment-review-phone-detail.png"))).toBe("On a phone — detail");
    expect(describeSource(image("deployment-review-wide.png"))).toBe("On your monitor");
  });

  it("never shows an address for a live page", () => {
    const name = describeSource(page("http://100.64.218.108:3135/viral_queue_accounts.html"));
    expect(name).toBe("The live page");
    expect(name).not.toMatch(/\d+\.\d+|http|:\d{2,}/);
  });

  it("makes an unrecognised filename readable instead of guessing", () => {
    expect(describeSource(image("SILV-55-source-account-layout-approval.png"))).toBe(
      "Source account layout approval",
    );
  });

  it("does not read an issue number as a screen width", () => {
    expect(describeSource(image("SILV-1440-notes.png"))).toBe("Notes");
  });
});

describe("describeSources", () => {
  const image = (label: string) => ({ id: label, label, kind: "image" as const, url: "u" });

  it("keeps two things that would share a name apart", () => {
    const names = describeSources([image("R-1440.png"), image("R-1600.png")]);
    expect(names.get("R-1440.png")).toBe("On your monitor — 1440px wide");
    expect(names.get("R-1600.png")).toBe("On your monitor — 1600px wide");
  });

  it("leaves a name alone when nothing else claims it", () => {
    const names = describeSources([image("R-390.png"), image("R-1440.png")]);
    expect(names.get("R-390.png")).toBe("On a phone");
    expect(names.get("R-1440.png")).toBe("On your monitor");
  });

  it("numbers duplicates that carry no width", () => {
    const pageAt = (url: string) => ({ id: url, label: url, kind: "page" as const, url });
    const names = describeSources([pageAt("http://a/x.html"), pageAt("http://b/y.html")]);
    expect([...names.values()]).toEqual(["The live page 1", "The live page 2"]);
  });

  it("does not read a port number as the width of a live page", () => {
    // This produced "The live page - 3135px wide" on the real inbox.
    const pageAt = (url: string) => ({ id: url, label: url, kind: "page" as const, url });
    const names = describeSources([
      pageAt("http://100.64.218.108:3135/viral_queue_accounts.html"),
      pageAt("http://100.64.218.108:3140/other.html"),
    ]);
    expect([...names.values()]).toEqual(["The live page 1", "The live page 2"]);
  });
});

describe("framingBlocked", () => {
  it("catches a plain-http preview inside the https inbox", () => {
    // This is the phone case: the same preview frames fine at the desk over
    // http://localhost and is blocked over the https address.
    expect(framingBlocked("http://100.64.218.108:3135/x.html", "https://a9drew.ts.net")).toBe(true);
  });

  it("leaves everything else alone", () => {
    expect(framingBlocked("http://100.64.218.108:3135/x.html", "http://localhost:3000")).toBe(false);
    expect(framingBlocked("https://example.com/x.html", "https://a9drew.ts.net")).toBe(false);
    expect(framingBlocked("http://x/y", undefined)).toBe(false);
  });
});

describe("sandboxFor", () => {
  // The frame may never navigate the tab it sits in, whatever it is showing:
  // the address it loads came out of text an agent wrote.
  it("never lets a preview move the page it is embedded in", () => {
    for (const url of ["http://10.0.0.1/x.html", "/api/attachments/abc/download"]) {
      expect(sandboxFor(url, "https://inbox.example")).not.toMatch(/allow-top-navigation/);
    }
  });

  it("withholds same-origin from an attached preview, which the app serves itself", () => {
    // Same-origin AND allow-scripts together would let the frame reach out of
    // the sandbox and into the inbox.
    const attrs = sandboxFor("/api/attachments/abc/download", "https://inbox.example");
    expect(attrs).not.toMatch(/allow-same-origin/);
    expect(attrs).toMatch(/allow-scripts/);
  });

  it("leaves a live page on another address its own origin", () => {
    // It cannot reach us either way, and it needs its origin to use its storage.
    expect(sandboxFor("http://100.64.218.108:3135/x.html", "https://inbox.example")).toMatch(
      /allow-same-origin/,
    );
  });
});

describe("attachmentPreviewId", () => {
  const id = "117a9167-c091-49f8-b03f-1637756141bb";

  it("recognises a canonical attached-page preview on this app", () => {
    expect(
      attachmentPreviewId(
        `/local-candidate-samples/attachments/${id}/preview`,
        "http://127.0.0.1:13004",
      ),
    ).toBe(id);
  });

  it("does not trust a look-alike preview path from another origin", () => {
    expect(
      attachmentPreviewId(
        `https://example.com/attachments/${id}/preview`,
        "https://multica.example",
      ),
    ).toBeNull();
  });

  it("rejects malformed attachment preview paths", () => {
    expect(
      attachmentPreviewId("/attachments/not-an-id/preview", "https://multica.example"),
    ).toBeNull();
  });
});
