/** @vitest-environment jsdom */
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { TalkToMikaButton } from "./talk-to-mika-button";

const state = vi.hoisted(() => ({
  agents: [{ id: "mika-agent", system_key: "mika", archived_at: null }],
  sessions: [{ id: "mika-chat", agent_id: "mika-agent", status: "active" }],
  create: vi.fn(),
  link: vi.fn(),
  setDraft: vi.fn(),
  push: vi.fn(),
}));

vi.mock("@tanstack/react-query", () => ({
  useQuery: (options: { kind: string }) => ({
    data: options.kind === "agents" ? state.agents : state.sessions,
  }),
}));
vi.mock("@multica/core/workspace/queries", () => ({
  agentListOptions: () => ({ kind: "agents" }),
}));
vi.mock("@multica/core/chat/queries", () => ({
  chatSessionsOptions: () => ({ kind: "sessions" }),
}));
vi.mock("@multica/core/chat/mutations", () => ({
  useCreateChatSession: () => ({ mutateAsync: state.create, isPending: false }),
  useLinkChatIssue: () => ({ mutateAsync: state.link, isPending: false }),
}));
vi.mock("@multica/core/chat", () => ({
  useChatStore: { getState: () => ({ setInputDraft: state.setDraft }) },
}));
vi.mock("@multica/core/hooks", () => ({ useWorkspaceId: () => "workspace" }));
vi.mock("@multica/core/paths", () => ({
  useWorkspacePaths: () => ({ chatSession: (id: string) => `/acme/chat?session=${id}` }),
}));
vi.mock("../../navigation", () => ({ useNavigation: () => ({ push: state.push }) }));
vi.mock("sonner", () => ({ toast: { error: vi.fn() } }));
vi.mock("../../i18n", async () => {
  const resource = (await import("../../locales/en/inbox.json")).default;
  return {
    useT: () => ({
      t: (selector: (value: typeof resource) => string, vars?: Record<string, unknown>) => {
        let value = selector(resource);
        for (const [key, replacement] of Object.entries(vars ?? {})) {
          value = value.replace(`{{${key}}}`, String(replacement));
        }
        return value;
      },
    }),
  };
});

describe("TalkToMikaButton", () => {
  beforeEach(() => {
    state.create.mockReset().mockResolvedValue({ id: "new-mika-chat", agent_id: "mika-agent", status: "active" });
    state.link.mockReset().mockResolvedValue({});
    state.setDraft.mockReset();
    state.push.mockReset();
    state.sessions = [{ id: "mika-chat", agent_id: "mika-agent", status: "active" }];
  });

  it("creates a dedicated linked conversation instead of adding work to an unrelated Mika chat", async () => {
    render(
      <TalkToMikaButton
        issueId="issue-1"
        issueIdentifier="SILV-83"
        title="Review the Clients mockup"
        question="Should the team keep this layout?"
        linkedChats={[]}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Talk to Mika about this" }));

    await waitFor(() => expect(state.create).toHaveBeenCalledWith({
      agent_id: "mika-agent",
      title: "Discuss SILV-83",
    }));
    await waitFor(() => expect(state.link).toHaveBeenCalledWith({
      sessionId: "new-mika-chat",
      issueId: "issue-1",
    }));
    expect(state.setDraft).toHaveBeenCalledWith(
      "new-mika-chat",
      expect.stringContaining("The decision waiting on me is: Should the team keep this layout?"),
    );
    expect(state.push).toHaveBeenCalledWith("/acme/chat?session=new-mika-chat");
  });

  it("reuses the exact linked Mika conversation when one exists", async () => {
    render(
      <TalkToMikaButton
        issueId="issue-1"
        issueIdentifier="SILV-83"
        title="Review the Clients mockup"
        question="Should the team keep this layout?"
        linkedChats={[{
          chat_session_id: "mika-chat",
          title: "Existing issue discussion",
          status: "active",
          source: "origin",
          created_at: "2026-10-03T00:00:00Z",
          updated_at: "2026-10-03T00:00:00Z",
        }]}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Talk to Mika about this" }));

    await waitFor(() => expect(state.setDraft).toHaveBeenCalledWith(
      "mika-chat",
      expect.stringContaining("Should the team keep this layout?"),
    ));
    expect(state.create).not.toHaveBeenCalled();
    expect(state.link).not.toHaveBeenCalled();
    expect(state.push).toHaveBeenCalledWith("/acme/chat?session=mika-chat");
  });

  it("asks Mika for status without inventing an owner decision", async () => {
    render(
      <TalkToMikaButton
        issueId="issue-1"
        issueIdentifier="SILV-80"
        title="Verify durable memory"
        question="Multica needs a fresh update before it can say what happens next."
        linkedChats={[]}
        promptKind="status"
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Talk to Mika about this" }));

    await waitFor(() => expect(state.setDraft).toHaveBeenCalledWith(
      "new-mika-chat",
      expect.stringContaining("The latest status says: Multica needs a fresh update"),
    ));
    expect(state.setDraft.mock.calls[0]?.[1]).not.toContain("decision waiting on me");
  });
});
