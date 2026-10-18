"use client";

import { MessageSquare, X } from "lucide-react";
import type { ChatWorkSelection } from "@multica/core/chat/work";
import { useT } from "../../i18n";

/**
 * A temporary, visible reply scope. It only appears after the owner chooses
 * "Discuss" beside a specific work item; ordinary messages remain General
 * chat. There is deliberately no destination menu in the composer.
 */
export function ChatReplyContext({ selected, onClear }: {
  selected: ChatWorkSelection;
  onClear: () => void;
}) {
  const { t } = useT("chat");
  const question = selected.pending_requests?.find(
    (request) => request.id === selected.target.request_id,
  )?.question;
  const label = question || selected.title;

  return (
    <div className="flex min-h-8 min-w-0 max-w-full items-center gap-2 border-l-2 border-primary/60 bg-muted/45 px-2.5 py-1 text-caption text-foreground" title={label}>
      <MessageSquare className="size-3.5 shrink-0 text-primary" />
      <span className="shrink-0 text-muted-foreground">{t(($) => $.work.replying_to)}</span>
      <span className="min-w-0 truncate font-medium">{label}</span>
      <button
        type="button"
        className="ml-auto shrink-0 rounded-sm p-0.5 text-muted-foreground hover:bg-background/70 hover:text-foreground"
        aria-label={t(($) => $.work.clear_target)}
        onClick={onClear}
      >
        <X className="size-3.5" />
      </button>
    </div>
  );
}
