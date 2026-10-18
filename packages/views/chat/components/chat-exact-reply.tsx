"use client";

import { MessageSquareReply, X } from "lucide-react";
import type { ChatMessage, ChatReplyContext, ChatReplySelection } from "@multica/core/types";
import { useT } from "../../i18n";

function compactPreview(value: string, fallback: string) {
  const preview = value.replace(/\s+/g, " ").trim();
  return preview || fallback;
}

export function replySelectionFromMessage(
  message: ChatMessage,
  subtarget: { block_id?: string; artifact_id?: string } = {},
): ChatReplySelection {
  const artifact = subtarget.artifact_id
    ? message.work_update?.review?.artifacts.find((item) => item.id === subtarget.artifact_id)
    : undefined;
  return {
    target: { event_id: message.id, ...subtarget },
    actor: message.role,
    message_kind: message.message_kind ?? "message",
    preview: artifact?.label || message.work_update?.body || message.content || message.attachments?.[0]?.filename || "",
  };
}

export function ChatExactReplyContext({ selected, onClear }: {
  selected: ChatReplySelection;
  onClear: () => void;
}) {
  const { t } = useT("chat");
  const who = selected.actor === "user"
    ? t(($) => $.message_list.reply_to_you)
    : t(($) => $.message_list.reply_to_mika);
  const preview = compactPreview(selected.preview, t(($) => $.message_list.reply_attachment));

  return (
    <div className="flex min-h-8 min-w-0 max-w-full items-center gap-2 border-l-2 border-primary/60 bg-muted/45 px-2.5 py-1 text-caption text-foreground" title={preview}>
      <MessageSquareReply className="size-3.5 shrink-0 text-primary" />
      <span className="shrink-0 text-muted-foreground">{who}</span>
      <span className="min-w-0 truncate font-medium">{preview}</span>
      <button
        type="button"
        className="ml-auto shrink-0 rounded-sm p-0.5 text-muted-foreground hover:bg-background/70 hover:text-foreground"
        aria-label={t(($) => $.message_list.clear_reply)}
        onClick={onClear}
      >
        <X className="size-3.5" />
      </button>
    </div>
  );
}

/** Compact quoted reference persisted on a sent user turn. */
export function ChatReplyReference({ context }: { context?: ChatReplyContext | null }) {
  const { t } = useT("chat");
  if (!context) return null;
  const who = context.actor === "user"
    ? t(($) => $.message_list.reply_to_you)
    : t(($) => $.message_list.reply_to_mika);
  const artifact = context.artifact_id
    ? context.artifacts.find((item) => item.id === context.artifact_id)
    : undefined;
  const preview = compactPreview(
    artifact?.label || context.content,
    context.attachments[0]?.filename || t(($) => $.message_list.reply_attachment),
  );
  return (
    <div className="mb-1.5 min-w-0 border-l-2 border-foreground/20 pl-2 text-caption text-muted-foreground">
      <div className="font-medium text-muted-foreground">{who}</div>
      <div className="max-w-96 truncate" title={preview}>{preview}</div>
    </div>
  );
}
