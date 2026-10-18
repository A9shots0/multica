"use client";

import { Check, X } from "lucide-react";
import type { ChatWorkResponseDraft } from "@multica/core/chat/work";
import { Button } from "@multica/ui/components/ui/button";
import { useT } from "../../i18n";

/**
 * A compact summary of independent answers queued for the next conversational
 * turn. It deliberately does not look like an exact-message reply: the checked
 * suggestion remains visible beside its question, while this strip only makes
 * the cross-question batch reviewable and removable before Send.
 */
export function ChatWorkResponseTray({ responses, onRemove, onClear }: {
  responses: ChatWorkResponseDraft[];
  onRemove: (response: ChatWorkResponseDraft) => void;
  onClear: () => void;
}) {
  const { t } = useT("chat");
  if (!responses.length) return null;
  return (
    <div className="flex min-w-0 items-center gap-2 border-b border-surface-border px-3 py-2" data-chat-work-responses>
      <span className="inline-flex shrink-0 items-center gap-1 text-caption font-medium text-foreground">
        <Check className="size-3.5 text-brand" aria-hidden="true" />
        {t(($) => $.updates.work_response_count, { count: responses.length })}
      </span>
      <div className="flex min-w-0 flex-1 gap-1.5 overflow-x-auto [scrollbar-width:none]">
        {responses.map((response) => (
          <span key={response.issue.target.request_id ?? response.issue.id} className="inline-flex h-6 max-w-72 shrink-0 items-center gap-1 rounded-md bg-surface-raised px-2 text-caption text-muted-foreground">
            <span className="max-w-28 truncate text-muted-foreground">{response.issue.title}</span>
            <span aria-hidden="true">·</span>
            <span className="max-w-36 truncate">{response.answer.trim()}</span>
            <button type="button" className="ml-0.5 rounded-sm hover:text-foreground" aria-label={t(($) => $.updates.remove_work_response, { title: response.issue.title })} onClick={() => onRemove(response)}>
              <X className="size-3" />
            </button>
          </span>
        ))}
      </div>
      {responses.length > 1 ? <Button type="button" variant="ghost" size="xs" className="h-6 shrink-0 px-1.5 text-caption text-muted-foreground" onClick={onClear}>{t(($) => $.updates.clear_work_responses)}</Button> : null}
    </div>
  );
}
