"use client";

import { useState } from "react";
import { Button } from "@multica/ui/components/ui/button";
import { cn } from "@multica/ui/lib/utils";
import { useT } from "../../i18n";
import type { Choice } from "./owner-inbox-card";

export function OwnerDecisionSelector({
  choices,
  descriptions,
  disabled,
  note,
  onNoteChange,
  onContinue,
}: {
  choices: readonly Choice[];
  descriptions: readonly string[];
  disabled: boolean;
  note: string;
  onNoteChange: (note: string) => void;
  onContinue: (choice: Choice) => void;
}) {
  const { t } = useT("inbox");
  const [selectedLabel, setSelectedLabel] = useState<string | null>(null);
  const [commentOpen, setCommentOpen] = useState(false);
  const selected = choices.find((choice) => choice.label === selectedLabel) ?? null;

  return (
    <div>
      <div
        role="radiogroup"
        aria-label={t(($) => $.owner_detail.next_step)}
        className="divide-y overflow-hidden rounded-lg border"
      >
        {choices.map((choice, index) => {
          const checked = choice.label === selectedLabel;
          return (
            <button
              key={choice.label}
              type="button"
              role="radio"
              aria-checked={checked}
              disabled={disabled}
              className={cn(
                "flex w-full items-start gap-3 bg-background px-3 py-3 text-left outline-none transition-colors hover:bg-accent/50 focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-ring disabled:pointer-events-none disabled:opacity-50",
                checked && "bg-brand/5 ring-1 ring-inset ring-brand/40",
                choice.danger && "text-destructive",
                choice.danger && checked && "bg-destructive/5 ring-destructive/40",
              )}
              onClick={() => setSelectedLabel(choice.label)}
            >
              <span
                aria-hidden="true"
                className={cn(
                  "mt-0.5 flex size-4 shrink-0 items-center justify-center rounded-full border-2 border-muted-foreground/50",
                  checked && "border-brand",
                  choice.danger && checked && "border-destructive",
                )}
              >
                {checked && (
                  <span
                    className={cn(
                      "size-2 rounded-full bg-brand",
                      choice.danger && "bg-destructive",
                    )}
                  />
                )}
              </span>
              <span className="min-w-0 flex-1">
                <span className="flex flex-wrap items-center gap-2 text-body font-medium leading-snug">
                  {choice.label}
                  {choice.primary && (
                    <span className="rounded bg-brand/10 px-1.5 py-0.5 text-[10px] font-semibold uppercase tracking-wide text-brand">
                      {t(($) => $.owner_detail.recommended_badge)}
                    </span>
                  )}
                </span>
                {descriptions[index] && (
                  <span className="mt-0.5 block text-caption leading-relaxed text-muted-foreground">
                    {descriptions[index]}
                  </span>
                )}
              </span>
            </button>
          );
        })}
      </div>

      {selected && (commentOpen || note.trim()) ? (
        <label className="mt-3 block rounded-lg border bg-muted/15 p-3">
          <span className="text-caption font-medium text-muted-foreground">
            {t(($) => $.owner_detail.add_comment_optional)}
          </span>
          <textarea
            autoFocus={commentOpen && !note}
            aria-label={t(($) => $.owner_detail.add_comment_optional)}
            value={note}
            disabled={disabled}
            onChange={(event) => onNoteChange(event.target.value)}
            placeholder={t(($) => $.owner_detail.comment_placeholder)}
            className="mt-2 min-h-20 w-full resize-y rounded-md border bg-background px-3 py-2 text-body outline-none placeholder:text-muted-foreground focus:border-primary disabled:opacity-50"
          />
        </label>
      ) : null}

      <div className="mt-3 flex flex-col-reverse items-stretch gap-2 sm:flex-row sm:items-center">
        <Button
          className="w-full sm:w-auto"
          disabled={disabled || !selected}
          onClick={() => selected && onContinue(selected)}
        >
          {t(($) => $.owner_detail.continue)}
        </Button>
        {selected && !commentOpen && !note.trim() && (
          <button
            type="button"
            disabled={disabled}
            onClick={() => setCommentOpen(true)}
            className="min-h-8 rounded-md px-2 text-left text-caption font-medium text-muted-foreground outline-none hover:bg-accent/50 hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring disabled:pointer-events-none disabled:opacity-50"
          >
            {t(($) => $.owner_detail.add_comment_optional)}
          </button>
        )}
      </div>
    </div>
  );
}
