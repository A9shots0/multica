"use client";

import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { ChevronDown, ChevronRight } from "lucide-react";
import { useWorkspaceId } from "@multica/core/hooks";
import { chatWorkOptions, useAcknowledgeChatWork } from "@multica/core/chat/queries";
import type { ChatWorkIssue } from "@multica/core/chat/work";
import type { ChatMessage } from "@multica/core/types";
import { ChatWorkUpdateMessage } from "./chat-work-update";
import { useT } from "../../i18n";

export function ChatWorkUpdateGroup({ messages, onDiscuss, onReply, onReplyWork, promotedReviewMessageIds, promotedQuestionMessageIds }: {
  messages: ChatMessage[];
  onDiscuss?: (issue: ChatWorkIssue, draft?: string) => void;
  onReply?: (message: ChatMessage, subtarget?: { block_id?: string; artifact_id?: string }) => void;
  onReplyWork?: (message: ChatMessage, issue: ChatWorkIssue, draft?: string) => void;
  promotedReviewMessageIds?: ReadonlySet<string>;
  promotedQuestionMessageIds?: ReadonlySet<string>;
}) {
  const { t } = useT("chat");
  const [expanded, setExpanded] = useState(false);
  const wsId = useWorkspaceId();
  const work = useQuery(chatWorkOptions(wsId, messages[0]?.chat_session_id ?? ""));
  const sessionId = messages[0]?.chat_session_id ?? "";
  const acknowledge = useAcknowledgeChatWork(wsId, sessionId);
  const messageIds = new Set(messages.map((message) => message.id));
  const unseenIds = (work.data?.catch_up ?? [])
    .filter((update) => messageIds.has(update.message_id))
    .map((update) => update.message_id);
  // Keep actionable questions outside the routine disclosure. If refreshing
  // fails, retain the question text without inventing an enabled reply action.
  const questions = messages.filter(message => {
    const update = message.work_update;
    if (update?.kind !== "question") return false;
    if (promotedQuestionMessageIds && !promotedQuestionMessageIds.has(message.id)) return false;
    if (!work.data || work.isError) return true;
    return work.data.issues.some(issue => issue.id === update.issue_id && issue.pending_requests?.some(request => request.id === update.request_id));
  });
  const questionIDs = new Set(questions.map(message => message.id));
  // A visual result is the thing the owner asked to see, not routine activity.
  // Keep it in the conversation even when started/stopped noise is collapsed.
  const latestReviewByIssue = new Map<string, ChatMessage>();
  for (const message of messages) {
    const update = message.work_update;
    if ((update?.kind === "result" || update?.kind === "stopped") && (update.review?.artifacts.length ?? 0) > 0) {
      latestReviewByIssue.set(update.issue_id, message);
    }
  }
  const reviews = [...latestReviewByIssue.values()].filter(
    message => !promotedReviewMessageIds || promotedReviewMessageIds.has(message.id),
  );
  const reviewIDs = new Set(reviews.map(message => message.id));
  const routine = messages.filter(message => !questionIDs.has(message.id) && !reviewIDs.has(message.id));
  const toggleRoutine = () => {
    const next = !expanded;
    setExpanded(next);
    if (next && unseenIds.length > 0 && !acknowledge.isPending) acknowledge.mutate(unseenIds);
  };
  return <div className="min-w-0 space-y-3" data-work-update-group>
    {reviews.map(message => <ChatWorkUpdateMessage key={message.id} message={message} onDiscuss={onDiscuss} onReply={onReply} />)}
    {questions.length > 0 && <div className="divide-y divide-border/50 border-y border-border/50" data-chat-decisions>
      {questions.map(message => <div key={message.id} className="py-4 first:pt-3 last:pb-3"><ChatWorkUpdateMessage message={message} onDiscuss={onDiscuss} onReply={onReply} onReplyWork={onReplyWork} /></div>)}
    </div>}
    {routine.length > 0 && <div>
      <button type="button" className="flex max-w-full items-center gap-1.5 py-1 text-caption text-muted-foreground hover:text-foreground" aria-expanded={expanded} onClick={toggleRoutine}>
        {unseenIds.length > 0 ? <span className="size-1.5 rounded-full bg-primary" aria-hidden="true" /> : null}
        <span>{t(($) => $.updates.group, { count: routine.length })}</span>
        {expanded ? <ChevronDown className="size-3 shrink-0" /> : <ChevronRight className="size-3 shrink-0" />}
      </button>
      {expanded && <div className="mt-2 max-h-72 space-y-3 overflow-y-auto border-l pl-3">{routine.map(message => <ChatWorkUpdateMessage key={message.id} message={message} onDiscuss={onDiscuss} onReply={onReply} />)}</div>}
    </div>}
  </div>;
}
