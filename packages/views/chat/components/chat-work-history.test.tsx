/** @vitest-environment jsdom */
import { render, screen, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi } from "vitest";
import { ChatWorkHistory } from "./chat-work-history";
import { I18nProvider } from "@multica/core/i18n/react";
import enChat from "../../locales/en/chat.json";
import type { ChatWorkIssue } from "@multica/core/chat/work";
const state = vi.hoisted(() => ({ next: vi.fn(), retry: vi.fn(), error: false }));
vi.mock("@tanstack/react-query", () => ({ useInfiniteQuery: () => ({ data: { pages: [{ activity: [{ id: "one", issue_id: "linked", reason: "First review", created_at: "2026-09-24T12:00:00Z" }, { id: "gone", issue_id: "unlinked", reason: "Private old issue", created_at: "2026-09-24T12:00:00Z" }] }, { activity: [{ id: "one", issue_id: "linked", reason: "First review", created_at: "2026-09-24T12:00:00Z" }] }] }, hasNextPage: true, isPending: false, isError: state.error, fetchNextPage: state.next, refetch: state.retry }) }));
vi.mock("@multica/core/chat/queries", () => ({ chatWorkHistoryOptions: vi.fn() }));
const issues = [{ id: "linked", title: "Preview review" }] as ChatWorkIssue[];
describe("compact work history", () => {
  it("deduplicates overlapping pages and hides unlinked issue history", () => {
    state.error = false;
    const select = vi.fn();
    render(<I18nProvider locale="en" resources={{ en: { chat: enChat } }}><ChatWorkHistory wsId="ws" sessionId="session" issues={issues} onSelect={select}/></I18nProvider>);
    expect(screen.getAllByText("First review")).toHaveLength(1);
    expect(screen.queryByText("Private old issue")).toBeNull();
    fireEvent.click(screen.getByRole("button", { name: /Preview review/ }));
    expect(select).toHaveBeenCalledWith("linked");
    fireEvent.click(screen.getByRole("button", { name: "Load older updates" }));
    expect(state.next).toHaveBeenCalled();
  });
  it("offers retry without silently claiming the end of history", () => {
    state.error = true;
    render(<I18nProvider locale="en" resources={{ en: { chat: enChat } }}><ChatWorkHistory wsId="ws" sessionId="session" issues={issues} onSelect={vi.fn()}/></I18nProvider>);
    expect(screen.getByRole("alert")).toHaveTextContent("Could not load work history.");
    fireEvent.click(screen.getByRole("button", { name: "Retry" }));
    expect(state.retry).toHaveBeenCalled();
  });
});
