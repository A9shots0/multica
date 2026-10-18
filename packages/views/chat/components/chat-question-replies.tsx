"use client";

import { Check } from "lucide-react";
import type { InboxItem } from "@multica/core/types";
import type { ChatWorkIssue } from "@multica/core/chat/work";
import { parseOptions } from "@multica/core/inbox/owner-inbox-model";
import { selectChatWorkOption } from "@multica/core/chat/work";
import { Button } from "@multica/ui/components/ui/button";

/**
 * Suggested answers compose independent guarded responses. Multiple questions
 * can stay selected and leave together in one normal Chat turn.
 */
export function ChatQuestionReplies({ item, issue, selectedDraft, onSelect }: {
  item?: InboxItem;
  issue: ChatWorkIssue;
  selectedDraft?: string;
  onSelect: (issue: ChatWorkIssue, draft: string) => void;
}) {
  const options = parseOptions(item?.details?.options);
  const draft = selectedDraft?.trim() ?? "";
  const leadingChoice = draft.split(/\r?\n/, 1)[0]?.trim() ?? "";
  const selectedOption = options.find((option) => leadingChoice === option.trim());

  return (
    <div className="mt-3">
      <div className="flex flex-wrap items-center gap-1.5" role="group" aria-label={issue.title}>
        {options.map((option) => {
          const selected = selectedOption === option;
          return (
            <Button
              key={option}
              variant={selected ? "brandSubtle" : "outline"}
              size="sm"
              aria-pressed={selected}
              className="h-auto min-h-8 max-w-full gap-1.5 whitespace-normal rounded-lg px-2.5 py-1 text-left text-caption font-medium"
              onClick={() => onSelect(issue, selectChatWorkOption(selectedDraft, options, option))}
            >
              {selected ? <Check className="size-3.5 shrink-0" aria-hidden="true" /> : null}
              <span>{option}</span>
            </Button>
          );
        })}
      </div>
    </div>
  );
}
