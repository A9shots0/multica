"use client";

import { useEffect, useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { ChevronLeft, ChevronRight, ExternalLink, MoreHorizontal } from "lucide-react";
import { toast } from "sonner";
import { Button } from "@multica/ui/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@multica/ui/components/ui/dropdown-menu";
import { cn } from "@multica/ui/lib/utils";
import { useWorkspacePaths } from "@multica/core/paths";
import { issueAttachmentsOptions } from "@multica/core/issues/queries";
import { issueChatLinksOptions } from "@multica/core/chat/queries";
import { ownerReviewFeedbackOptions } from "@multica/core/inbox/queries";
import type { OwnerInboxBatch, OwnerInboxGroup } from "@multica/core/inbox/owner-inbox-model";
import {
  describeSources,
  firstSentence,
  meaningfulConsequence,
  parseOptions,
  splitOptionConsequences,
  splitQuestion,
  waitingFor,
} from "@multica/core/inbox/owner-inbox-model";
import type { OwnerDirectiveType } from "@multica/core/types";
import { describeMarks, type Mark } from "./screenshot-annotator";
import { useT } from "../../i18n";
import { PreviewViewer, attachmentSources, extractPageLinks } from "./preview-viewer";
import { choicesFor, useAnswer, variantFor, type Choice, type FireOrder } from "./owner-inbox-card";
import { OwnerDecisionSelector } from "./owner-decision-selector";
import { TalkToMikaButton } from "./talk-to-mika-button";
import { useOwnerReviewFeedback } from "./use-owner-review-feedback";

/**
 * The right panel: the one thing on the left you clicked, read in full and
 * answered here.
 *
 * This is the list-beside-detail shape from the reference — a "NEEDS YOU"
 * strip, the question as a headline, the evidence behind a set of tabs named
 * for the screen it was captured at (the plain-English naming already built
 * for exactly this — "On your monitor" reads as a tab label as naturally as
 * it reads as an image caption), a plain-English readout of what each answer
 * actually does, then the answer itself.
 */
export function OwnerInboxDetail({
  group,
  onClear,
  onLater,
  extras,
}: {
  group: OwnerInboxGroup;
  /** Archive this group off the list — the same action the row's context
   *  menu and the old board's "Clear" link both did. */
  onClear: () => void;
  /** Seen, but not decided now — marks the group's unread events read
   *  without answering or filing it away, so it sinks under what is still
   *  genuinely new but stays reachable in the list. */
  onLater?: () => void;
  /** Repair controls for a notice with no piece of work behind it — a
   *  quick-create retry, quota recovery. The page wires these through
   *  because it owns what a notice's type actually needs. */
  extras?: React.ReactNode;
}) {
  const { t } = useT("inbox");
  const paths = useWorkspacePaths();
  const { send, sent, pending, ready, issue, decision, reviewChanged, refreshReview } = useAnswer(group);
  const [choiceNote, setChoiceNote] = useState("");
  const [customDirection, setCustomDirection] = useState("");
  const [marks, setMarks] = useState<Mark[]>([]);
  const [activeTab, setActiveTab] = useState<string | null>(null);
  const [fullScreen, setFullScreen] = useState(false);
  /** A different direction is an escape hatch, not a peer to the proposed
   *  choices. The ordinary path is choice + optional context + Continue. */
  const [writing, setWriting] = useState(false);

  const issueId = group.issueId ?? "";
  const { data: attachments = [] } = useQuery({
    ...issueAttachmentsOptions(issueId),
    enabled: !!issueId,
  });
  // Only rendered when non-empty (see the link row below) — an "own" chat
  // discussed this issue and tagged it, so the owner can jump back into that
  // conversation instead of reconstructing context from the issue alone.
  const { data: linkedChats = [] } = useQuery({
    ...issueChatLinksOptions(issueId),
    enabled: !!issueId,
  });
  const { data: reviewFeedback = [] } = useQuery(ownerReviewFeedbackOptions(issueId));
  const markedEvidenceIds = useMemo(
    () => new Set(reviewFeedback.map((entry) => entry.marked_screenshot_attachment_id)),
    [reviewFeedback],
  );
  const pages = useMemo(
    () =>
      extractPageLinks(
        group.ask.detail,
        ...group.items.flatMap((entry) => [
          entry.details?.question,
          entry.details?.recommendation,
          entry.details?.consequence,
          entry.body,
        ]),
      ),
    [group.ask.detail, group.items],
  );
  const sources = useMemo(
    () => [...attachmentSources(attachments, markedEvidenceIds), ...pages],
    [attachments, markedEvidenceIds, pages],
  );
  const sourceNames = useMemo(() => describeSources(sources), [sources]);
  // A picture by default, not a "running now" placeholder: a live page with
  // nothing to draw shown first is a mostly-empty box before the reader has
  // even chosen anything, and a captured screenshot is more often the thing
  // actually being judged.
  const tabId =
    activeTab ?? sources.find((s) => s.kind === "image")?.id ?? sources[0]?.id ?? null;
  const activeSource = sources.find((s) => s.id === tabId) ?? sources[0];
  const activeSourceIndex = activeSource
    ? Math.max(0, sources.findIndex((source) => source.id === activeSource.id))
    : 0;
  const stepSource = (delta: number) => {
    if (sources.length < 2) return;
    const next = sources[(activeSourceIndex + delta + sources.length) % sources.length];
    if (next) setActiveTab(next.id);
  };

  const asked = useMemo(() => splitQuestion(decision?.question ?? group.ask.detail), [decision?.question, group.ask.detail]);
  const advice = useMemo(
    () => firstSentence(decision?.recommendation ?? group.lead.details?.recommendation),
    [decision?.recommendation, group.lead.details?.recommendation],
  );
  const outcomes = useMemo(
    () => splitOptionConsequences(meaningfulConsequence(decision?.consequence ?? group.lead.details?.consequence)),
    [decision?.consequence, group.lead.details?.consequence],
  );
  const numbered = !group.lead.details?.decision_id && parseOptions(group.lead.details?.options).length > 0;
  const choices = choicesFor(group);
  const isExternalWork = group.lead.type === "external_work_update";

  const { persistMarkedEvidence, savingFeedback } = useOwnerReviewFeedback({
    issueId,
    inboxItemId: group.lead.id,
    sources,
    attachments,
    marks,
    note: choiceNote,
  });

  const answer = async (label: string, directive: OwnerDirectiveType, ownerNote = choiceNote) => {
    const drawn = describeMarks(marks, sourceNames);
    try {
      const evidence = await persistMarkedEvidence(ownerNote);
      const extra = [ownerNote.trim(), drawn && `What to change:\n${drawn}`, evidence.summary].filter(Boolean).join("\n\n");
      return await send(directive, label, extra, evidence.issueRevision);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t(($) => $.owner_detail.review_save_failed));
      return false;
    }
  };
  const outcomeFor = (index: number) =>
    (numbered && outcomes.byOption.get(index + 1)) || "";
  const hasTechnicalDetails =
    Boolean(group.ask.technical) && group.ask.technical?.trim() !== group.ask.detail.trim();

  // One decision surface. The old detail repeated consequences, a separate
  // recommendation card, giant buttons, freeform direction, Mika, history,
  // and chat links as unrelated blocks. Here the recommendation and choices
  // share one hierarchy, with the escape routes kept in one quiet footer.
  const answerArea = (
    <div className="w-full rounded-xl border bg-surface-raised p-3.5 shadow-[var(--surface-shadow)] sm:p-4">
      {reviewChanged && <div role="alert" className="text-caption text-muted-foreground">
        {t(($) => $.chat_response.changed)}
        <Button variant="ghost" size="sm" onClick={refreshReview}>{t(($) => $.chat_response.review_latest)}</Button>
      </div>}
      <div className="mb-3">
        <h3 className="text-body font-semibold">{t(($) => $.owner_detail.next_step)}</h3>
        {advice.head && (
          <p className="mt-1 text-caption leading-relaxed text-muted-foreground">
            <span className="font-medium text-muted-foreground">{t(($) => $.owner_detail.recommended)}:</span>{" "}
            {advice.head}
          </p>
        )}
        {advice.tail && (
          <details className="mt-1">
            <summary className="cursor-pointer text-caption text-muted-foreground">{t(($) => $.owner_detail.why)}</summary>
            <p className="mt-1 text-caption leading-relaxed text-muted-foreground">{advice.tail}</p>
          </details>
        )}
      </div>

      {writing ? (
        <div className="rounded-lg border bg-muted/15 p-3">
          <label className="block">
            <span className="text-body font-medium">{t(($) => $.owner_detail.give_different_direction)}</span>
            <textarea
              autoFocus
              value={customDirection}
              onChange={(event) => setCustomDirection(event.target.value)}
              placeholder={t(($) => $.owner_detail.change_placeholder)}
              className="mt-2 min-h-[88px] w-full resize-y rounded-lg border bg-background p-3 text-body outline-none focus:border-primary"
            />
          </label>
          <div className="flex items-center gap-3">
            <Button
              size="sm"
              disabled={!ready || pending || savingFeedback || !customDirection.trim()}
              onClick={() => { void answer("", "request_changes", customDirection); }}
            >
              {t(($) => $.owner_detail.send)}
            </Button>
            <button
              type="button"
              onClick={() => {
                setWriting(false);
                setCustomDirection("");
              }}
              className="text-caption text-muted-foreground underline-offset-2 hover:text-foreground hover:underline"
            >
              {t(($) => $.owner_detail.never_mind)}
            </button>
          </div>
        </div>
      ) : (
        <>
          <OwnerDecisionSelector
            choices={choices}
            descriptions={choices.map((_, index) => outcomeFor(index))}
            disabled={!ready || pending || savingFeedback}
            note={choiceNote}
            onNoteChange={setChoiceNote}
            onContinue={(choice: Choice) => { void answer(choice.send, choice.directive); }}
          />

          <div className="mt-3 flex flex-wrap items-center gap-2 border-t pt-3">
            <TalkToMikaButton issueId={group.issueId} issueIdentifier={issue?.identifier} title={group.title} question={asked.lead || group.ask.detail} linkedChats={linkedChats} />
            <DropdownMenu>
              <DropdownMenuTrigger
                render={
                  <Button variant="ghost" size="sm">
                    <MoreHorizontal className="size-4" />
                    {t(($) => $.owner_detail.more_options)}
                  </Button>
                }
              />
              <DropdownMenuContent align="start" className="w-auto min-w-52">
                <DropdownMenuItem onClick={() => setWriting(true)}>
                  {t(($) => $.owner_detail.give_different_direction)}
                </DropdownMenuItem>
                {issueId && (
                  <DropdownMenuItem render={<a href={paths.issueDetail(issueId)} />}>
                    {t(($) => $.owner_detail.full_history)}
                  </DropdownMenuItem>
                )}
                {linkedChats.length > 0 && (
                  <DropdownMenuItem render={<a href={`${paths.chat()}?session=${encodeURIComponent(linkedChats[0]!.chat_session_id)}`} />}>
                    {t(($) => $.owner_detail.continue_in_chat, { title: linkedChats[0]!.title || "chat" })}
                  </DropdownMenuItem>
                )}
              </DropdownMenuContent>
            </DropdownMenu>
          </div>
        </>
      )}
    </div>
  );

  if (sent) {
    return (
      <div className="flex flex-1 flex-col items-center justify-center px-6 text-center">
        <p className="text-body text-muted-foreground">
          {t(($) => $.owner_detail.sent_summary, { title: group.title })}
        </p>
      </div>
    );
  }

  return (
    <div className="flex min-h-0 flex-1 flex-col overflow-y-auto">
      <div className="border-b px-4 py-4 sm:px-6 sm:py-5">
        <div className="flex flex-wrap items-center gap-2">
          <span
            className={cn(
              "rounded px-2 py-0.5 text-[10px] font-semibold uppercase tracking-wide",
              group.ask.kind === "decision" && "bg-brand/15 text-brand",
              group.ask.kind === "stopped" && "bg-rose-500/15 text-rose-500",
              group.ask.kind === "broke" && "bg-amber-500/15 text-amber-500",
              group.ask.kind === "news" && "bg-muted text-muted-foreground",
            )}
          >
            {isExternalWork && group.lead.details?.reported_state === "complete"
              ? "Reported done"
              : isExternalWork && group.lead.details?.reported_state === "waiting_owner"
                ? "Needs you"
                : group.ask.kind === "broke"
                  ? "Stopped"
                  : group.ask.kind === "news"
                    ? "Update"
                    : "Needs you"}
          </span>
          {issue?.identifier && (
            <span className="text-caption text-muted-foreground">
              {issue.identifier} · {group.title}
            </span>
          )}
          <span className="ml-auto shrink-0 text-caption text-muted-foreground">
            {waitingFor(group.updatedAt)}
          </span>
          {group.unread && onLater && (
            <button
              type="button"
              onClick={onLater}
              className="text-caption text-muted-foreground underline-offset-2 hover:text-foreground hover:underline"
            >
              {t(($) => $.owner_detail.not_now)}
            </button>
          )}
          <button
            type="button"
            onClick={onClear}
            className="text-caption text-muted-foreground underline-offset-2 hover:text-foreground hover:underline"
          >
            {t(($) => $.owner_detail.clear)}
          </button>
        </div>

        <h2 className="mt-3 break-words text-xl font-semibold leading-snug sm:text-title">
          {asked.lead || group.ask.detail}
        </h2>
        {decision && <p className="mt-2 text-caption text-muted-foreground">{t(($) => $.chat_response.version, { revision: decision.revision })}</p>}

        {asked.parts.length > 0 && (
          <ul className="mt-3 space-y-1.5">
            {asked.parts.map((part) => (
              <li key={part.label} className="flex gap-2.5">
                <span className="mt-0.5 flex size-5 shrink-0 items-center justify-center rounded-full bg-muted text-[11px] font-semibold uppercase">
                  {part.label}
                </span>
                <span className="min-w-0 text-body leading-relaxed text-foreground">
                  {part.text}
                </span>
              </li>
            ))}
          </ul>
        )}
      </div>

      <div className="space-y-4 px-4 py-4 sm:space-y-5 sm:px-6 sm:py-5">
        {sources.length > 0 && activeSource && (
          <section>
            <div className="flex items-center gap-2">
              {sources.length > 1 && (
                <div className="flex items-center gap-1 rounded-lg border bg-muted/20 p-1">
                  <Button size="icon-sm" variant="ghost" onClick={() => stepSource(-1)} aria-label={t(($) => $.preview.previous_picture)}>
                    <ChevronLeft className="size-4" />
                  </Button>
                  <span className="min-w-24 text-center text-caption tabular-nums text-muted-foreground">
                    {t(($) => $.preview.result_position, { current: activeSourceIndex + 1, total: sources.length })}
                  </span>
                  <Button size="icon-sm" variant="ghost" onClick={() => stepSource(1)} aria-label={t(($) => $.preview.next_picture)}>
                    <ChevronRight className="size-4" />
                  </Button>
                </div>
              )}
              <button
                type="button"
                onClick={() => setFullScreen(true)}
                className="ml-auto inline-flex shrink-0 items-center gap-1 text-caption font-medium text-brand hover:underline"
              >
                {t(($) => $.owner_detail.open_full_screen)}
                <ExternalLink className="size-3.5" />
              </button>
            </div>

            <button type="button" onClick={() => setFullScreen(true)} aria-label={t(($) => $.preview.open)} className="mt-3 block w-full cursor-zoom-in overflow-hidden rounded-lg border bg-muted/30 text-left focus-visible:ring-2 focus-visible:ring-ring">
              {activeSource.kind === "image" ? (
                <img
                  src={activeSource.url}
                  alt={sourceNames.get(activeSource.id) ?? activeSource.label}
                  className="mx-auto block max-h-[360px] w-auto max-w-full object-contain"
                />
              ) : (
                <div className="flex h-20 items-center justify-center gap-1.5 text-caption font-medium sm:h-[120px]">
                  <ExternalLink className="size-3.5" />
                  {t(($) => $.preview.open)}
                </div>
              )}
            </button>
          </section>
        )}

        {hasTechnicalDetails && (
          <details className="rounded-lg border bg-muted/15 px-3.5 py-3">
            <summary className="cursor-pointer text-caption text-muted-foreground">
              {t(($) => $.owner_detail.technical_details)}
            </summary>
            <pre className="mt-2 max-h-48 overflow-auto whitespace-pre-wrap break-words font-mono text-[11px] leading-relaxed text-muted-foreground">
              {group.ask.technical}
            </pre>
          </details>
        )}

        {group.ask.kind === "news" && extras && (
          <section className="flex flex-wrap gap-2">{extras}</section>
        )}

        {!isExternalWork && group.ask.kind !== "news" && choices.length > 0 && <section>{answerArea}</section>}

        {(isExternalWork || group.ask.kind === "news" || choices.length === 0) && (
          <section className="flex flex-wrap items-center gap-3">
            <TalkToMikaButton issueId={group.issueId} issueIdentifier={issue?.identifier} title={group.title} question={asked.lead || group.ask.detail} linkedChats={linkedChats} />
            {issueId && <a href={paths.issueDetail(issueId)} className="text-caption font-medium text-muted-foreground hover:text-foreground">{t(($) => $.owner_detail.full_history)} →</a>}
          </section>
        )}
      </div>

      {/* The SAME answer area, not a second copy of it: the full-screen
          viewer's actions panel used to just re-fire choose() and rely on the
          resulting textarea rendering in the ordinary body below — which is
          exactly where the portal covering the screen hides it. Picking
          "Ask for a change" while full-screen looked like it did nothing. */}
      {fullScreen && sources.length > 0 && (
        <PreviewViewer
          sources={sources}
          startId={activeSource?.id}
          marks={marks}
          onChangeMarks={setMarks}
          onClose={() => setFullScreen(false)}
          title={issue?.identifier ? `${issue.identifier} · ${group.title}` : group.title}
          question={group.ask.detail}
        />
      )}
    </div>
  );
}

/**
 * Several pieces of work stalled for the identical reason, answered once.
 *
 * Reuses the same `useAnswer`/`FireOrder` mechanism the card build already
 * had: each member mounts its own hook (rules of hooks forbid calling one N
 * times in a loop), and a bumped token tells every member "send this," each
 * down its own path, so one member failing reads as one failure, not six.
 */
export function OwnerInboxBatchDetail({ batch }: { batch: OwnerInboxBatch }) {
  const { t } = useT("inbox");
  const [open, setOpen] = useState(false);
  const [order, setOrder] = useState<FireOrder | null>(null);
  const [asking, setAsking] = useState<Choice | null>(null);
  const [note, setNote] = useState("");
  const count = batch.groups.length;
  const newest = batch.groups.reduce(
    (latest, group) => (group.updatedAt > latest ? group.updatedAt : latest),
    batch.groups[0]!.updatedAt,
  );

  const choices: Choice[] =
    batch.kind === "broke"
      ? [{ label: `Start all ${count} again`, directive: "correct", send: "Start this again.", primary: true }]
      : [
          { label: `Carry on with all ${count}`, directive: "correct", send: "Carry on with this.", primary: true },
          {
            label: "Change what I asked for",
            directive: "change_scope",
            send: "I want to change what this should do.",
          },
          { label: `Drop all ${count}`, directive: "cancel", send: "Stop work on this.", danger: true },
        ];

  const fire = (choice: Choice, extra = "") => {
    setOrder({ token: Date.now(), directive: choice.directive, send: extra ? `${choice.send}\n\n${extra}` : choice.send });
    setAsking(null);
    setNote("");
  };

  return (
    <div className="flex min-h-0 flex-1 flex-col overflow-y-auto">
      <div className="border-b px-6 py-5">
        <div className="flex flex-wrap items-center gap-2">
          <span
            className={cn(
              "rounded px-2 py-0.5 text-[10px] font-semibold uppercase tracking-wide",
              batch.kind === "broke" ? "bg-amber-500/15 text-amber-500" : "bg-rose-500/15 text-rose-500",
            )}
          >
            {batch.kind === "broke" ? "Broke" : "Needs you"}
          </span>
          <span className="ml-auto shrink-0 text-caption text-muted-foreground">{waitingFor(newest)}</span>
        </div>
        <h2 className="mt-3 text-title font-semibold leading-snug">
          {batch.kind === "broke"
            ? t(($) => $.owner_detail.batch_broke, { count })
            : t(($) => $.owner_detail.batch_stopped, { count })}
        </h2>
        <p className="mt-2 text-body leading-relaxed text-foreground">{batch.shared}</p>
      </div>

      <div className="space-y-5 px-6 py-5">
        {asking ? (
          <div className="max-w-[620px] space-y-2">
            <textarea
              autoFocus
              value={note}
              onChange={(event) => setNote(event.target.value)}
              placeholder={
                asking.danger
                  ? `Dropping all ${count}. What should the team know?`
                  : `${asking.label} — what should be different?`
              }
              className="min-h-[88px] w-full resize-y rounded-lg border bg-muted/40 p-3 text-body outline-none focus:border-primary"
            />
            <div className="flex items-center gap-3">
              <Button size="sm" disabled={!note.trim()} onClick={() => fire(asking, note.trim())}>
                {t(($) => $.owner_detail.send_all, { count })}
              </Button>
              <button
                type="button"
                onClick={() => {
                  setAsking(null);
                  setNote("");
                }}
                className="text-caption text-muted-foreground underline-offset-2 hover:text-foreground hover:underline"
              >
                {t(($) => $.owner_detail.never_mind)}
              </button>
            </div>
          </div>
        ) : (
          <div className="flex w-full flex-col gap-2">
            {choices.map((choice) => (
              <Button
                key={choice.label}
                variant={variantFor(choice)}
                disabled={!!order}
                className="h-auto min-h-11 w-full whitespace-normal px-4 py-2 text-left leading-snug"
                onClick={() => (choice.primary ? fire(choice) : setAsking(choice))}
              >
                {choice.label}
              </Button>
            ))}
          </div>
        )}

        <button
          type="button"
          onClick={() => setOpen((shown) => !shown)}
          className="text-caption text-muted-foreground underline-offset-2 hover:text-foreground hover:underline"
        >
          {open ? "Hide them" : `Show the ${count}`}
        </button>

        {open && (
          <ul className="space-y-1.5">
            {batch.groups.map((group) => (
              <BatchMemberRow key={group.key} group={group} fireOrder={order} />
            ))}
          </ul>
        )}
      </div>
    </div>
  );
}

/** One member of a batch, in the expanded list. Mounts its own `useAnswer` so
 *  it can send on the shared `fireOrder` down its own path — the same
 *  mechanism `OwnerInboxCard`'s own `fireOrder` prop already uses. */
function BatchMemberRow({ group, fireOrder }: { group: OwnerInboxGroup; fireOrder: FireOrder | null }) {
  const { send, sent, ready } = useAnswer(group);
  const [fired, setFired] = useState(0);
  useEffect(() => {
    if (!fireOrder || fireOrder.token === fired || sent || !ready) return;
    setFired(fireOrder.token);
    void send(fireOrder.directive, fireOrder.send);
  }, [fireOrder, fired, sent, ready, send]);

  return (
    <li
      className={cn(
        "rounded-md border px-3 py-2 text-caption",
        sent ? "text-muted-foreground line-through" : "text-foreground",
      )}
    >
      {group.title}
    </li>
  );
}
