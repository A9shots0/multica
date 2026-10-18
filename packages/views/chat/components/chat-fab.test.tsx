import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { I18nProvider } from "@multica/core/i18n/react";
import enChat from "../../locales/en/chat.json";

const state = vi.hoisted(() => ({ status: "queued" }));
vi.mock("@multica/core/hooks", () => ({ useWorkspaceId: () => "preview" }));
vi.mock("@multica/core/chat", () => ({
  useChatStore: (selector: (value: { isOpen: boolean; toggle: () => void }) => unknown) => selector({ isOpen: false, toggle: vi.fn() }),
}));
vi.mock("@multica/core/shortcuts", () => ({ useShortcut: () => null }));
vi.mock("@multica/core/chat/queries", () => ({
  chatSessionsOptions: () => ({ queryKey: ["sessions"] }),
  pendingChatTasksOptions: () => ({ queryKey: ["pending"] }),
  countUnreadChatSessions: () => 0,
}));
vi.mock("@tanstack/react-query", () => ({
  useQuery: ({ queryKey }: { queryKey: string[] }) => ({
    data: queryKey[0] === "sessions" ? [] : { tasks: [{ task_id: "preview-task", status: state.status, chat_session_id: "preview-chat" }] },
  }),
}));
vi.mock("@multica/ui/components/ui/tooltip", () => ({
  Tooltip: ({ children }: React.PropsWithChildren) => <>{children}</>,
  TooltipTrigger: (props: React.ComponentProps<"button">) => <button {...props} />,
  TooltipContent: ({ children }: React.PropsWithChildren) => <span>{children}</span>,
}));

import { ChatFab } from "./chat-fab";

describe("Chat pending activity labels", () => {
  it.each(["queued", "dispatched", "deferred", "waiting_local_directory", "waiting_resource"])("does not claim execution for %s", (status) => {
    state.status = status;
    render(<I18nProvider locale="en" resources={{ en: { chat: enChat } }}><ChatFab /></I18nProvider>);
    expect(screen.getByRole("button", { name: "Queued" })).not.toHaveClass("animate-chat-impulse");
    expect(screen.queryByRole("button", { name: "Multica is working..." })).not.toBeInTheDocument();
  });

  it("shows working only for the server running state", () => {
    state.status = "running";
    render(<I18nProvider locale="en" resources={{ en: { chat: enChat } }}><ChatFab /></I18nProvider>);
    expect(screen.getByRole("button", { name: "Multica is working..." })).toHaveClass("animate-chat-impulse");
  });
});
