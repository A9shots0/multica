import { describe, expect, it, vi, beforeEach } from "vitest";
import { render, screen, fireEvent } from "@testing-library/react";
import { I18nProvider } from "@multica/core/i18n/react";
import type { ChatMessage } from "@multica/core/types";
import enChat from "../../locales/en/chat.json";
import enInbox from "../../locales/en/inbox.json";
import { ChatWorkUpdateMessage } from "./chat-work-update";
import { ChatWorkUpdateGroup } from "./chat-work-update-group";

const state = vi.hoisted(() => ({ data: undefined as unknown, isError: false, isPending: false }));
const acknowledge = vi.hoisted(() => vi.fn());
vi.mock("@tanstack/react-query", () => ({ useQuery: () => state }));
vi.mock("@multica/core/hooks", () => ({ useWorkspaceId: () => "workspace" }));
vi.mock("@multica/core/chat/queries", () => ({
  chatWorkOptions: () => ({}),
  useAcknowledgeChatWork: () => ({ mutate: acknowledge, isPending: false, isError: false }),
}));
vi.mock("@multica/core/chat", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@multica/core/chat")>();
  return {
    ...actual,
    useChatStore: (selector: (value: { workTopics: Record<string, never>; inputDrafts: Record<string, string> }) => unknown) => selector({ workTopics: {}, inputDrafts: {} }),
  };
});
vi.mock("../../inbox/components/use-owner-review-feedback", () => ({
  useOwnerReviewFeedback: () => ({ persistMarkedEvidence: vi.fn(), savingFeedback: false }),
}));

const target = { issue_id: "issue", issue_revision: 9, owner_id: "owner", request_id: "question" };
const issue = { id: "issue", title: "Review mobile", target: { ...target, request_id: undefined }, pending_requests: [{ id: "question", question: "Keep the compact layout?", target }] };
const message: ChatMessage = {
  id: "update", chat_session_id: "chat", role: "assistant", task_id: null,
  content: "Work update", created_at: "2026-09-24T10:00:00Z", message_kind: "work_update",
  work_update: { kind: "question", issue_id: "issue", title: "Review mobile", source_id: "question", request_id: "question", occurred_at: "2026-09-24T10:00:00Z", body: "Keep the compact layout?" },
};
function view(onDiscuss = vi.fn(), value = message) {
  return <I18nProvider locale="en" resources={{ en: { chat: enChat, inbox: enInbox } }}><ChatWorkUpdateMessage message={value} onDiscuss={onDiscuss} /></I18nProvider>;
}
beforeEach(() => { state.data = { issues: [issue], requests: [{ id: "question", details: { options: "Keep it compact\nMake it roomier" } }] }; state.isError = false; state.isPending = false; acknowledge.mockClear(); });
describe("automatic work updates", () => {
  it("shows a stopped run's evidence without calling it a ready result", () => {
    const stopped = { ...message, work_update: { ...message.work_update!, kind: "stopped" as const, body: "Execution is blocked. Logs are attached.", review: { schema_version: 1 as const, source_task_id: "worker", artifacts: [{ id: "logs", label: "Execution logs", kind: "file" as const, url: "/logs.txt", original: true }] } } };
    render(view(vi.fn(), stopped));
    expect(screen.getByText("Execution is blocked. Logs are attached.")).toBeVisible();
    expect(screen.getByRole("link", { name: "Execution logs" })).toHaveAttribute("href", "/logs.txt");
    expect(screen.queryByText("Result ready")).toBeNull();
  });
  it("collapses routine updates and renders a pending question as ordinary conversation", () => {
    const routine = ["started", "result", "stopped"].map((kind, index) => ({ ...message, id: `event-${index}`, work_update: { ...message.work_update!, kind: kind as "started" | "result" | "stopped", body: `Evidence ${index}` } }));
    render(<I18nProvider locale="en" resources={{ en: { chat: enChat, inbox: enInbox } }}><ChatWorkUpdateGroup messages={[...routine, message]} onDiscuss={vi.fn()} /></I18nProvider>);
    expect(screen.queryByText("Evidence 1")).toBeNull();
    expect(screen.getByText("Keep the compact layout?")).toBeVisible();
    expect(screen.queryByRole("button", { name: "Reply to Mika" })).toBeNull();
    expect(screen.queryByText("Review mobile")).toBeNull();
    fireEvent.click(screen.getByRole("button", { name: /3 background updates/ }));
    expect(screen.getByText("Evidence 1")).toBeVisible();
  });
  it("routes a suggested answer through the normal composer instead of submitting an Inbox form", () => {
    const onDiscuss = vi.fn(); render(view(onDiscuss));
    fireEvent.click(screen.getByRole("button", { name: "Keep it compact" }));
    expect(onDiscuss).toHaveBeenCalledExactlyOnceWith({ ...issue, target }, "Keep it compact\n\n");
    expect(screen.queryByRole("radio")).toBeNull();
    expect(screen.queryByRole("button", { name: "Continue" })).toBeNull();
  });

  it("keeps simultaneous questions from different issues distinct and replyable", () => {
    const secondTarget = { issue_id: "issue-2", issue_revision: 3, owner_id: "owner", request_id: "question-2" };
    const secondIssue = { ...issue, id: "issue-2", number: 6, title: "Homepage review", target: { ...secondTarget, request_id: undefined }, pending_requests: [{ id: "question-2", question: "Ship the new hierarchy?", target: secondTarget }] };
    const secondMessage = { ...message, id: "update-2", work_update: { ...message.work_update!, issue_id: "issue-2", title: "Homepage review", source_id: "question-2", request_id: "question-2", body: "Ship the new hierarchy?" } };
    state.data = { issues: [issue, secondIssue], requests: [{ id: "question", details: { options: "Keep it compact" } }, { id: "question-2", details: { options: "Ship it" } }] };
    const onReply = vi.fn();
    render(<I18nProvider locale="en" resources={{ en: { chat: enChat, inbox: enInbox } }}><ChatWorkUpdateGroup messages={[message, secondMessage]} onDiscuss={onReply} /></I18nProvider>);
    expect(screen.getByText("Review mobile")).toBeVisible();
    expect(screen.getByText("Homepage review")).toBeVisible();
    expect(screen.getByText("Keep the compact layout?")).toBeVisible();
    expect(screen.getByText("Ship the new hierarchy?")).toBeVisible();
    fireEvent.click(screen.getByRole("button", { name: "Ship it" }));
    expect(onReply).toHaveBeenCalledExactlyOnceWith({ ...secondIssue, target: secondTarget }, "Ship it\n\n");
  });
  it("retains historical evidence but removes the answer action after resolution elsewhere", () => {
    const { rerender } = render(view());
    state.data = { issues: [{ ...issue, pending_requests: [] }] }; rerender(view());
    expect(screen.getByText("No longer pending")).toBeVisible();
    expect(screen.queryByRole("button", { name: "Reply to Mika" })).toBeNull();
    fireEvent.click(screen.getByRole("button", { name: /Review mobile/ }));
    expect(screen.getByText("Keep the compact layout?")).toBeVisible();
  });
  it("fails closed when the latest request cannot be loaded", () => {
    state.isError = true; render(view());
    expect(screen.queryByRole("button", { name: "Reply to Mika" })).toBeNull();
    expect(screen.queryByText("No longer pending")).toBeNull();
  });
  it("leads with the result and keeps supporting detail and discussion folded", () => {
    const preamble = "The worker finished this run. The issue is not automatically approved or complete.";
    render(view(vi.fn(), { ...message, work_update: { ...message.work_update!, kind: "result", body: `${preamble}\n\nA detailed worker report.` } }));
    expect(screen.getByText("A detailed worker report.")).toBeVisible();
    expect(screen.queryByText(preamble, { exact: false })).toBeNull();
    expect(screen.queryByRole("button", { name: "Discuss with Mika" })).toBeNull();
    fireEvent.click(screen.getByRole("button", { name: /Review mobile/ }));
    expect(screen.getByText(preamble, { exact: false })).toBeVisible();
    expect(screen.getByRole("button", { name: "Discuss with Mika" })).toBeVisible();
  });
  it("shows a delivered visual result inline instead of hiding it in routine updates", () => {
    const review = {
      schema_version: 1 as const,
      source_task_id: "task",
      artifacts: [{ id: "preview", label: "Clients page", kind: "image" as const, url: "/preview.png", original: true }],
    };
    const result = { ...message, id: "result", work_update: { ...message.work_update!, kind: "result" as const, body: "Review the result.", review } };
    const started = { ...message, id: "started", work_update: { ...message.work_update!, kind: "started" as const, body: "Started." } };
    render(<I18nProvider locale="en" resources={{ en: { chat: enChat, inbox: enInbox } }}><ChatWorkUpdateGroup messages={[started, result]} onDiscuss={vi.fn()} /></I18nProvider>);
    expect(screen.queryByText("Ready to review")).toBeNull();
    expect(screen.queryByText("Result ready")).toBeNull();
    expect(screen.getByRole("button", { name: "Review and annotate Clients page" })).toBeVisible();
    expect(screen.getByAltText("Clients page")).toBeVisible();
    expect(screen.queryByTitle("Clients page")).toBeNull();
    expect(screen.queryByText("Started.")).toBeNull();
    fireEvent.click(screen.getByRole("button", { name: "Review and annotate Clients page" }));
    expect(screen.getByRole("dialog", { name: "Review mobile" })).toBeVisible();
    expect(screen.getByRole("button", { name: "Annotate" })).toBeVisible();
  });

  it("presents a mixed set as one conversational preview plus a compact file rail", () => {
    const review = {
      schema_version: 1 as const,
      source_task_id: "task",
      artifacts: [
        { id: "desktop", label: "Desktop checkout", kind: "image" as const, url: "/desktop.png", original: true },
        { id: "mobile", label: "Mobile checkout", kind: "image" as const, url: "/mobile.png", original: true },
        { id: "page", label: "Full-page flow", kind: "page" as const, url: "/flow.html", original: true },
        { id: "details", label: "Details panel", kind: "image" as const, url: "/details.png", original: true },
        { id: "notes", label: "Checkout handoff.pdf", kind: "file" as const, url: "/handoff.pdf", original: true },
      ],
    };
    const result = { ...message, id: "bundle", work_update: { ...message.work_update!, kind: "result" as const, body: "Five related deliverables.", review } };
    render(view(vi.fn(), result));

    expect(screen.getByText("5 files")).toBeVisible();
    expect(screen.queryByText("5 deliverables")).toBeNull();
    expect(screen.getByText("On a phone")).toBeVisible();
    expect(screen.getByText("Full page")).toBeVisible();
    expect(screen.getByRole("button", { name: "Review and annotate Desktop checkout" })).toBeVisible();
    expect(screen.getByRole("button", { name: "Open Mobile checkout" })).toBeVisible();
    expect(screen.getByRole("button", { name: "Open Full-page flow" })).toBeVisible();
    expect(screen.getByRole("button", { name: "Open Details panel" })).toBeVisible();
    expect(screen.getByRole("link", { name: "Checkout handoff.pdf" })).toHaveAttribute("href", "/handoff.pdf");
    expect(screen.queryByRole("button", { name: "Review 5 deliverables" })).toBeNull();
    expect(screen.queryByText("Reply")).toBeNull();

    fireEvent.click(screen.getByRole("button", { name: "Open Mobile checkout" }));
    expect(screen.getByRole("dialog", { name: "Review mobile" })).toBeVisible();
    expect(screen.getByAltText("On a phone")).toBeVisible();
  });

  it("keeps a large file-only set in one horizontal rail without hiding downloads", () => {
    const artifacts = Array.from({ length: 6 }, (_, index) => ({
      id: `file-${index + 1}`,
      label: `Handoff ${index + 1}.pdf`,
      kind: "file" as const,
      url: `/handoff-${index + 1}.pdf`,
      original: true,
    }));
    const result = {
      ...message,
      id: "file-bundle",
      work_update: {
        ...message.work_update!,
        kind: "result" as const,
        body: "Six handoff files.",
        review: { schema_version: 1 as const, source_task_id: "task", artifacts },
      },
    };
    render(view(vi.fn(), result));

    const links = screen.getAllByRole("link", { name: /Handoff \d+\.pdf/ });
    expect(links).toHaveLength(6);
    expect(links[0]!.closest("[data-chat-review-files]")).toHaveClass("overflow-x-auto");
    expect(screen.getByText("6 files")).toBeVisible();
  });

  it("renders simultaneous result bundles for separate issues independently", () => {
    const review = (id: string, label: string) => ({
      schema_version: 1 as const,
      source_task_id: `task-${id}`,
      artifacts: [{ id, label, kind: "image" as const, url: `/${id}.png`, original: true }],
    });
    const checkout = { ...message, id: "checkout-result", work_update: { ...message.work_update!, kind: "result" as const, issue_id: "checkout", body: "Checkout set.", review: review("checkout", "Checkout set") } };
    const profile = { ...message, id: "profile-result", work_update: { ...message.work_update!, kind: "result" as const, issue_id: "profile", body: "Profile set.", review: review("profile", "Profile set") } };
    render(<I18nProvider locale="en" resources={{ en: { chat: enChat, inbox: enInbox } }}><ChatWorkUpdateGroup messages={[checkout, profile]} onDiscuss={vi.fn()} /></I18nProvider>);

    expect(screen.getByRole("button", { name: "Review and annotate Checkout set" })).toBeVisible();
    expect(screen.getByRole("button", { name: "Review and annotate Profile set" })).toBeVisible();
    expect(screen.queryByRole("button", { name: /background update/ })).toBeNull();
  });

  it("keeps only the newest visual result prominent for each issue", () => {
    const review = (id: string) => ({
      schema_version: 1 as const,
      source_task_id: `task-${id}`,
      artifacts: [{ id, label: `Result ${id}`, kind: "image" as const, url: `/${id}.png`, original: true }],
    });
    const first = { ...message, id: "first", work_update: { ...message.work_update!, kind: "result" as const, body: "First result.", review: review("one") } };
    const latest = { ...message, id: "latest", work_update: { ...message.work_update!, kind: "result" as const, body: "Latest result.", review: review("two") } };
    render(<I18nProvider locale="en" resources={{ en: { chat: enChat, inbox: enInbox } }}><ChatWorkUpdateGroup messages={[first, latest]} onDiscuss={vi.fn()} /></I18nProvider>);
    expect(screen.queryByRole("button", { name: "Review and annotate Result one" })).toBeNull();
    expect(screen.getByRole("button", { name: "Review and annotate Result two" })).toBeVisible();
    expect(screen.getByRole("button", { name: /1 background update/ })).toBeVisible();
  });

  it("leads with Mika's result and places background activity after the conversation", () => {
    const review = {
      schema_version: 1 as const,
      source_task_id: "task",
      artifacts: [{ id: "preview", label: "Clients page", kind: "image" as const, url: "/preview.png", original: true }],
    };
    const result = { ...message, id: "result", work_update: { ...message.work_update!, kind: "result" as const, body: "I have the review ready.", review } };
    const started = { ...message, id: "started", work_update: { ...message.work_update!, kind: "started" as const, body: "Started." } };
    render(<I18nProvider locale="en" resources={{ en: { chat: enChat, inbox: enInbox } }}><ChatWorkUpdateGroup messages={[started, result]} onDiscuss={vi.fn()} /></I18nProvider>);
    const resultText = screen.getByText("I have the review ready.");
    const activity = screen.getByRole("button", { name: /1 background update/ });
    expect(resultText.compareDocumentPosition(activity) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
  });

  it("marks only visible timeline updates seen", () => {
    state.data = {
      issues: [issue], requests: [{ id: "question" }],
      catch_up: [
        { message_id: "update", issue_id: "issue" },
        { message_id: "not-loaded", issue_id: "issue" },
      ],
    };
    const routine = { ...message, work_update: { ...message.work_update!, kind: "started" as const, body: "Started." } };
    render(<I18nProvider locale="en" resources={{ en: { chat: enChat, inbox: enInbox } }}><ChatWorkUpdateGroup messages={[routine]} onDiscuss={vi.fn()} /></I18nProvider>);
    const updates = screen.getByRole("button", { name: /1 background update/ });
    expect(updates).toBeVisible();
    fireEvent.click(updates);
    expect(acknowledge).toHaveBeenCalledExactlyOnceWith(["update"]);
  });
});
