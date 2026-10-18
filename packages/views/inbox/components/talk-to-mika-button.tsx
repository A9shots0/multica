"use client";

import { useQuery } from "@tanstack/react-query";
import { MessageSquareText } from "lucide-react";
import { toast } from "sonner";
import { Button } from "@multica/ui/components/ui/button";
import { useWorkspaceId } from "@multica/core/hooks";
import { useWorkspacePaths } from "@multica/core/paths";
import { agentListOptions } from "@multica/core/workspace/queries";
import { chatSessionsOptions } from "@multica/core/chat/queries";
import { useCreateChatSession, useLinkChatIssue } from "@multica/core/chat/mutations";
import { useChatStore } from "@multica/core/chat";
import { isMikaAgent } from "@multica/core/onboarding";
import type { IssueChatLink } from "@multica/core/types";
import { useNavigation } from "../../navigation";
import { useT } from "../../i18n";

/**
 * Opens a real Mika conversation with the Inbox decision already in the
 * editable composer. The issue is linked before navigation, so Mika receives
 * the same durable work context as the Inbox rather than a pasted summary with
 * no identity. Nothing is sent until the owner presses Send in Chat.
 */
export function TalkToMikaButton({
  issueId,
  issueIdentifier,
  title,
  question,
  linkedChats,
  promptKind = "decision",
}: {
  issueId: string | null;
  issueIdentifier?: string | null;
  title: string;
  question: string;
  linkedChats: IssueChatLink[];
  promptKind?: "decision" | "status";
}) {
  const { t } = useT("inbox");
  const wsId = useWorkspaceId();
  const paths = useWorkspacePaths();
  const navigation = useNavigation();
  const { data: agents = [] } = useQuery(agentListOptions(wsId));
  const { data: sessions = [] } = useQuery(chatSessionsOptions(wsId));
  const createSession = useCreateChatSession();
  const linkIssue = useLinkChatIssue();

  const open = async () => {
    const mika = agents.find((agent) => !agent.archived_at && isMikaAgent(agent));
    if (!mika) {
      toast.error(t(($) => $.owner_detail.mika_unavailable));
      return;
    }

    const linkedIds = new Set(linkedChats.map((chat) => chat.chat_session_id));
    let session = sessions.find(
      (candidate) =>
        candidate.status === "active" &&
        candidate.agent_id === mika.id &&
        linkedIds.has(candidate.id),
    );

    try {
      if (!session) {
        session = await createSession.mutateAsync({
          agent_id: mika.id,
          title: issueIdentifier
            ? t(($) => $.owner_detail.mika_chat_title_issue, { identifier: issueIdentifier })
            : t(($) => $.owner_detail.mika_chat_title, { title }),
        });
      }
      if (issueId && !linkedIds.has(session.id)) {
        await linkIssue.mutateAsync({ sessionId: session.id, issueId });
      }
      useChatStore.getState().setInputDraft(
        session.id,
        [
          t(($) => $.owner_detail.mika_draft_intro, {
            subject: [issueIdentifier, title].filter(Boolean).join(" · "),
          }),
          promptKind === "decision"
            ? t(($) => $.owner_detail.mika_draft_question, { question })
            : t(($) => $.owner_detail.mika_draft_status, { status: question }),
          promptKind === "decision"
            ? t(($) => $.owner_detail.mika_draft_help)
            : t(($) => $.owner_detail.mika_draft_status_help),
        ].join("\n\n"),
      );
      navigation.push(paths.chatSession(session.id));
    } catch {
      toast.error(t(($) => $.owner_detail.mika_open_failed));
    }
  };

  return (
    <Button
      type="button"
      variant="outline"
      className="h-auto min-h-10 w-full justify-start gap-2 whitespace-normal text-left sm:w-auto"
      disabled={createSession.isPending || linkIssue.isPending}
      onClick={() => void open()}
    >
      <MessageSquareText className="size-4" />
      {t(($) => $.owner_detail.talk_to_mika)}
    </Button>
  );
}
