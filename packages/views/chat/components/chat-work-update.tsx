"use client";

import { useQuery } from "@tanstack/react-query";
import { useState } from "react";
import { ChevronDown, ChevronRight, CircleDot, CircleHelp, FileText, MessageCircle, MessageSquareReply, OctagonAlert } from "lucide-react";
import { useWorkspaceId } from "@multica/core/hooks";
import { chatWorkOptions } from "@multica/core/chat/queries";
import { chatWorkResponseKey } from "@multica/core/chat/work";
import { useChatStore } from "@multica/core/chat";
import type { ChatWorkIssue } from "@multica/core/chat/work";
import type { ChatMessage } from "@multica/core/types";
import type { InboxItem } from "@multica/core/types";
import { Button } from "@multica/ui/components/ui/button";
import { Tooltip, TooltipContent, TooltipTrigger } from "@multica/ui/components/ui/tooltip";
import { useT } from "../../i18n";
import { ChatReviewArtifact } from "./chat-review-artifact";
import { ChatQuestionReplies } from "./chat-question-replies";

// Persisted evidence stays readable after a question is answered. Its action
// always uses today's canonical request/target, never the historical snapshot.
export function ChatWorkUpdateMessage({ message, onDiscuss, onReply, onReplyWork }: {
  message: ChatMessage;
  onDiscuss?: (issue: ChatWorkIssue, draft?: string) => void;
  onReply?: (message: ChatMessage, subtarget?: { block_id?: string; artifact_id?: string }) => void;
  onReplyWork?: (message: ChatMessage, issue: ChatWorkIssue, draft?: string) => void;
}) {
  const { t } = useT("chat");
  const [expanded, setExpanded] = useState(false);
  const wsId = useWorkspaceId();
  const work = useQuery(chatWorkOptions(wsId, message.chat_session_id));
  const update = message.work_update;
  if (!update) return null;
  const issue = !work.isError ? work.data?.issues.find((item) => item.id === update.issue_id) : undefined;
  const request = issue?.pending_requests?.find((item) => item.id === update.request_id);
  const requestItem = work.data?.requests?.find((item) => item.id === update.request_id) as InboxItem | undefined;
  const question = update.kind === "question";
  const target = request && issue ? { ...issue, target: request.target } : issue;
  const canDiscuss = !!target && (!question || !!request) && !!onDiscuss;
  const showIssueContext = !!issue && ((work.data?.issues.length ?? 0) > 1 || (issue.pending_requests?.length ?? 0) > 1);
  // The delivery preamble is a status explanation, not the worker's result.
  // Retain it in details but lead with the actual reported result when present.
  const [firstParagraph = "", ...rest] = update.body.split("\n\n");
  const resultPreamble = "The worker finished this run. The issue is not automatically approved or complete.";
  const resultBody = update.kind === "result" && firstParagraph === resultPreamble && rest.length > 0
    ? rest.join("\n\n") : update.body;
  const labels = {
    started: t(($) => $.updates.started), result: t(($) => $.updates.result),
    stopped: t(($) => $.updates.stopped), question: t(($) => $.updates.question),
  };
  const Icon = { started: CircleDot, result: FileText, stopped: OctagonAlert, question: CircleHelp }[update.kind];
  const showQuestion = question && (!!request || !work.data || work.isError);
  if ((update.kind === "result" || update.kind === "stopped") && update.review) {
    return (
      <div className="min-w-0 max-w-2xl text-body" data-work-update={update.kind}>
        {update.kind === "stopped" && <p className="mb-1 flex items-center gap-2 text-caption text-muted-foreground"><OctagonAlert className="size-3.5" />{labels.stopped}</p>}
        {resultBody && (
          <p className="whitespace-pre-wrap break-words leading-relaxed">{resultBody}</p>
        )}
        <ChatReviewArtifact
          update={update}
          onUseFeedback={target && onDiscuss ? (feedback) => onDiscuss(target, feedback) : undefined}
          onReplyArtifact={onReply ? (artifactId) => onReply(message, { block_id: update.request_id || update.source_id, artifact_id: artifactId }) : undefined}
        />
        {onReply && !update.review.artifacts.length && <ReplyToUpdateButton onClick={() => onReply(message, { block_id: update.request_id || update.source_id })} />}
      </div>
    );
  }
  if (showQuestion) {
    return <ChatQuestionMessage
      message={message}
      issue={issue}
      request={request}
      requestItem={requestItem}
      showIssueContext={showIssueContext}
      onDiscuss={onDiscuss}
      onReplyWork={onReplyWork}
    />;
  }
  return (
    <div className="min-w-0 text-body" data-work-update={update.kind}>
      <button type="button" className="flex w-full min-w-0 items-center gap-2 text-left text-caption text-muted-foreground hover:text-foreground" aria-expanded={expanded} onClick={() => setExpanded(value => !value)}>
        <Icon className="size-3.5 shrink-0" />
        <span className="min-w-0 truncate" title={update.title}>{update.title}</span>
        <span className="shrink-0">{question && issue && !request ? t(($) => $.updates.no_longer_pending) : labels[update.kind]}</span>
        {expanded ? <ChevronDown className="size-3 shrink-0" /> : <ChevronRight className="size-3 shrink-0" />}
      </button>
      {!expanded && update.kind === "result" && resultBody !== resultPreamble && <p className="mt-1.5 line-clamp-2 whitespace-pre-wrap break-words text-muted-foreground">{resultBody}</p>}
      {expanded && <div className="mt-2 space-y-2 pl-5">
        <time className="text-caption text-muted-foreground" dateTime={update.occurred_at}>{new Date(update.occurred_at).toLocaleString()}</time>
        {!showQuestion && <p className="max-h-60 overflow-y-auto whitespace-pre-wrap break-words text-muted-foreground">{update.body}</p>}
      </div>}
      {canDiscuss && expanded && <Button variant="ghost" size="sm" className="mt-1 -ml-2 h-7 gap-1.5 text-caption" onClick={() => onDiscuss!(target!)}>
        <MessageCircle className="size-3.5" />{t(($) => $.updates.discuss)}
      </Button>}
      {onReply && expanded && <ReplyToUpdateButton onClick={() => onReply(message, { block_id: update.request_id || update.source_id })} />}
    </div>
  );
}

function ChatQuestionMessage({ message, issue, request, requestItem, showIssueContext, onDiscuss, onReplyWork }: {
  message: ChatMessage;
  issue?: ChatWorkIssue;
  request?: NonNullable<ChatWorkIssue["pending_requests"]>[number];
  requestItem?: InboxItem;
  showIssueContext: boolean;
  onDiscuss?: (issue: ChatWorkIssue, draft?: string) => void;
  onReplyWork?: (message: ChatMessage, issue: ChatWorkIssue, draft?: string) => void;
}) {
  const replyIssue = request && issue ? { ...issue, target: request.target } : undefined;
  const selectedDraft = useChatStore((state) => {
    if (!replyIssue) return undefined;
    return state.workResponses?.[message.chat_session_id]?.[chatWorkResponseKey(replyIssue.target)]?.answer;
  });
  return (
    <div className="min-w-0 max-w-2xl border-l-2 border-border/70 py-0.5 pl-3.5 text-body" data-work-update="question">
      {showIssueContext && issue ? <p className="mb-1 text-caption font-medium text-muted-foreground">{issue.title}</p> : null}
      <p className="whitespace-pre-wrap break-words leading-relaxed text-foreground">{message.work_update?.body}</p>
      {replyIssue && (onReplyWork || onDiscuss) ? (
        <ChatQuestionReplies
          item={requestItem}
          issue={replyIssue}
          selectedDraft={selectedDraft}
          onSelect={(selection, draft) => onReplyWork
            ? onReplyWork(message, selection, draft)
            : onDiscuss!(selection, draft)}
        />
      ) : null}
    </div>
  );
}

function ReplyToUpdateButton({ onClick }: { onClick: () => void }) {
  const { t } = useT("chat");
  const label = t(($) => $.message_list.reply_action);
  return (
    <Tooltip>
      <TooltipTrigger render={<Button variant="ghost" size="icon-xs" className="text-muted-foreground hover:text-foreground" aria-label={label} onClick={onClick} />}>
        <MessageSquareReply />
      </TooltipTrigger>
      <TooltipContent side="top">{label}</TooltipContent>
    </Tooltip>
  );
}
