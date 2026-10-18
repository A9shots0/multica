"use client";

import { useT } from "../../i18n";

import { useCallback, useEffect, useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { MoreHorizontal } from "lucide-react";
import { Button } from "@multica/ui/components/ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@multica/ui/components/ui/dropdown-menu";
import { cn } from "@multica/ui/lib/utils";
import { useWorkspaceId } from "@multica/core/hooks";
import { useWorkspacePaths } from "@multica/core/paths";
import { issueAttachmentsOptions, issueDetailOptions } from "@multica/core/issues/queries";
import { ownerReviewFeedbackOptions, workflowDecisionOptions } from "@multica/core/inbox/queries";
import {
  useAnswerWorkflowOwnerDecision,
  useCreateOwnerDirective,
  useRespondWorkflowDecision,
} from "@multica/core/inbox/mutations";
import type { OwnerInboxBatch, OwnerInboxGroup } from "@multica/core/inbox/owner-inbox-model";
import type { WorkflowDecision } from "@multica/core/types/workflow";
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
import { toast } from "sonner";
import { describeMarks, type Mark } from "./screenshot-annotator";
import { PreviewDeck } from "./preview-deck";
import { PreviewViewer, attachmentSources, extractPageLinks } from "./preview-viewer";
import { useOwnerReviewFeedback } from "./use-owner-review-feedback";

/**
 * One thing to answer, answered where it is read.
 *
 * Everything on a card runs top to bottom in the order the decision is made:
 * what is being asked, the thing itself at a size worth looking at, then the
 * answers. Nothing is captioned, because the title says what it is about and
 * the buttons say what can be done.
 *
 * The answers were briefly a column down the right-hand side on some cards and
 * a stack underneath on others, depending on how long an agent's wording was,
 * and a third size again in the full-screen view. One action wearing three
 * shapes on one screen gives the eye nowhere to learn, so there is now one
 * shape: a full-width stack, under the evidence, everywhere.
 *
 * Under the evidence matters on its own. Approve sitting top-right, above the
 * pictures, made the cheapest gesture on the card approving something nobody
 * had looked at yet.
 */

/** The wiring for sending an answer. Shared by a card, by one line of a
 *  batch, and by the split-pane detail view — the same act with different
 *  chrome around it. */
export function useAnswer(group: OwnerInboxGroup) {
  const { t } = useT("inbox");
  const wsId = useWorkspaceId();
  const item = group.lead;
  const issueId = item.issue_id ?? "";
  const { data: issue } = useQuery(issueDetailOptions(wsId, issueId));
  const decisionId = item.details?.decision_id ?? "";
  const { data: packet } = useQuery({ ...workflowDecisionOptions(issueId, decisionId), refetchInterval: 5000 });
  const createDirective = useCreateOwnerDirective();
  const respond = useRespondWorkflowDecision();
  const answerWorkflowQuestion = useAnswerWorkflowOwnerDecision();
  const [sent, setSent] = useState<string | null>(null);

  const [reviewedDecision, setReviewedDecision] = useState<WorkflowDecision | null>(null);
  const currentDecision = packet?.decision;
  if (currentDecision && currentDecision.id !== reviewedDecision?.id) setReviewedDecision(currentDecision);
  const decision = reviewedDecision?.id === decisionId ? reviewedDecision : currentDecision;
  const reviewChanged = !!decision && !!currentDecision && (decision.revision !== currentDecision.revision || decision.state !== currentDecision.state);
  const workflowEventId = item.details?.workflow_event_id;
  const isFailureInvestigationQuestion = !!item.details?.investigation_id;
  const isWorkflowQuestion =
    item.type === "workflow_owner_decision" && !decisionId && !!workflowEventId;
  const pending =
    createDirective.isPending || respond.isPending || answerWorkflowQuestion.isPending;

  /**
   * Sends down whichever path this row is parked on. The three are not
   * interchangeable — a directive on a row waiting for a workflow event
   * records the words and leaves the run parked forever — so the choice is
   * made once, from the row's own shape, rather than at each button.
   */
  const respondAsync = respond.mutateAsync;
  const directiveAsync = createDirective.mutateAsync;
  const workflowAsync = answerWorkflowQuestion.mutateAsync;
  const revision = issue?.revision;
  const send = useCallback(
    async (directive: OwnerDirectiveType, label: string, extra = "", expectedIssueRevision?: number) => {
      if (!issueId) return;
      const content = [label, extra].filter(Boolean).join("\n\n");
      // An investigation question is asking for direction after the failure
      // has already been investigated. Every answer is new input, not another
      // declaration that the prior AI action failed, so it must not recursively
      // launch a second investigation through request_changes.
      const effectiveDirective = isFailureInvestigationQuestion ? "correct" : directive;
      try {
        if (decisionId && (!decision || !currentDecision || reviewChanged || decision.state !== "unanswered")) throw new Error(t(($) => $.chat_response.changed));
        if (decision && decision.state === "unanswered") {
          await respondAsync({
            issueId,
            decisionId: decision.id,
            request: {
              expected_revision: decision.revision,
              // An atomic authorization packet takes only yes or no; the words
              // the owner typed ride along as the comment.
              outcome: effectiveDirective === "correct" ? "approve" : "reject",
              comment: content || undefined,
            },
          });
        } else if (isWorkflowQuestion) {
          await workflowAsync({ item, answer: content });
        } else {
          const directiveRevision = expectedIssueRevision ?? revision;
          if (!directiveRevision) throw new Error(t(($) => $.chat_response.loading));
          await directiveAsync({
            issueId,
            request: {
              expected_issue_revision: directiveRevision,
              directive: item.details?.question_scope === "ordinary_task" ? "correct" : effectiveDirective,
              content,
              inbox_item_id: item.id,
              ...((item.type === "chat_intake" || item.details?.question_scope === "ordinary_task") ? { intake_id: item.id } : {}),
            },
          });
        }
        setSent(label || "Sent");
        return true;
      } catch (error) {
        toast.error(error instanceof Error ? error.message : t(($) => $.chat_response.failed));
        return false;
      }
    },
    [
      issueId,
      decisionId,
      decision,
      currentDecision,
      reviewChanged,
      isWorkflowQuestion,
      isFailureInvestigationQuestion,
      revision,
      item,
      respondAsync,
      directiveAsync,
      workflowAsync,
      t,
    ],
  );

  return { send, sent, pending, ready: !!issueId && !!revision && !reviewChanged && (!decisionId || decision?.state === "unanswered"), issue, decision, reviewChanged, refreshReview: () => { if (currentDecision) setReviewedDecision(currentDecision); } };
}

/** How the board hands a card its slice of the full-screen view. */
export interface ViewerProps {
  viewer?: { sourceId?: string; position: { index: number; total: number } };
  onOpenViewer: (sourceId?: string) => void;
  onCloseViewer: () => void;
  onStepViewer: (delta: number) => void;
}

/** One answer the owner can give. */
export interface Choice {
  label: string;
  directive: OwnerDirectiveType;
  send: string;
  /** The straight yes. It sends on the first press; everything else asks what
   *  the owner means before it goes anywhere. */
  primary?: boolean;
  danger?: boolean;
}

/** An instruction to answer on somebody else's behalf, carrying which answer.
 *  A batch offers three, so a bare token cannot say which one was pressed. */
export interface FireOrder {
  token: number;
  directive: OwnerDirectiveType;
  send: string;
}

/** The answers on offer for a stalled run, in the order the sentence above
 *  them names them. Shared, so the batch card cannot drift from the card. */
function stoppedChoices(): Choice[] {
  return [
    { label: "Carry on", directive: "correct", send: "Carry on with this.", primary: true },
    {
      label: "Change what I asked for",
      directive: "change_scope",
      send: "I want to change what this should do.",
    },
    { label: "Drop it", directive: "cancel", send: "Stop work on this.", danger: true },
  ];
}

/** The answers on offer, by what the thing actually is. An agent that wrote
 *  its own options gets its own words; nothing invents choices never offered. */
export function choicesFor(group: OwnerInboxGroup): Choice[] {
  if (group.lead.details?.decision_id) return [
    { label: "Approve", directive: "correct", send: "Approved.", primary: true },
    { label: "Ask for a change", directive: "request_changes", send: "Please change this." },
    { label: "Decline", directive: "cancel", send: "I do not approve this request." },
  ];
  const options = parseOptions(group.lead.details?.options);
  if (options.length > 0) {
    return options.map((option, index) => ({
      label: option,
      directive: index === 0 ? "correct" : "request_changes",
      send: option,
      primary: index === 0,
    }));
  }
  switch (group.ask.kind) {
    case "decision":
      return [
        { label: "Approve", directive: "correct", send: "Approved.", primary: true },
        { label: "Ask for a change", directive: "request_changes", send: "Please change this." },
      ];
    case "stopped":
      return stoppedChoices();
    case "broke":
      return [
        { label: "Start it again", directive: "correct", send: "Start this again.", primary: true },
      ];
    default:
      return [
        { label: "Ask for a change", directive: "request_changes", send: "Please change this." },
      ];
  }
}

/** One shape for an answer, on the card and in the full-screen view alike.
 *  44 pixels tall, because these are pressed with a thumb as often as clicked. */
const ANSWER_BUTTON =
  "h-auto min-h-11 w-full justify-start whitespace-normal break-words px-3 py-2 text-left leading-snug";

/** The column answers live in: wide enough for a sentence, capped so that
 *  "Approve" is not a slab running the width of a 4K screen. */
const ANSWER_COLUMN = "flex w-full max-w-[620px] flex-col gap-2";

/** Which button skin an answer gets. A real destructive style for "Drop it" —
 *  not an outline button with its text recoloured, which reads as a styling
 *  bug rather than a warning. */
export function variantFor(choice: Choice): "brand" | "outline" | "destructive" {
  if (choice.danger) return "destructive";
  // The one real accent color on the page, spent on the one control that
  // actually moves the work forward — not the flat near-white "default"
  // fill every other button on the page also wears.
  return choice.primary ? "brand" : "outline";
}

/** A quiet control that is still a real target under a thumb. */
const QUIET_BUTTON =
  "inline-flex min-h-11 items-center rounded-md px-1 text-caption text-muted-foreground underline-offset-2 outline-none hover:text-foreground hover:underline focus-visible:ring-2 focus-visible:ring-ring sm:min-h-8";

export function OwnerInboxCard({
  group,
  onClear,
  onLater,
  highlighted,
  viewer,
  onOpenViewer,
  onCloseViewer,
  onStepViewer,
  fireOrder = null,
}: {
  group: OwnerInboxGroup;
  onClear: () => void;
  /** Seen, but not being decided now. Unread here means "not yet dealt with",
   *  never "not yet scrolled past" — a card that marked itself read on sight
   *  would make both the sidebar count and this button meaningless. */
  onLater: () => void;
  highlighted?: boolean;
  /** Set while the full-screen view is showing THIS card. The board owns it so
   *  the owner can walk the whole queue without dropping back to the list. */
  viewer?: { sourceId?: string; position: { index: number; total: number } };
  onOpenViewer: (sourceId?: string) => void;
  onCloseViewer: () => void;
  onStepViewer: (delta: number) => void;
  /** Set when a batch answers for all of its members at once. The card still
   *  sends down its own path, so one failure is one failure. */
  fireOrder?: FireOrder | null;
}) {
  const { t } = useT("inbox");
  const paths = useWorkspacePaths();
  const { send, sent, pending, ready, issue, decision, reviewChanged, refreshReview } = useAnswer(group);
  const [marks, setMarks] = useState<Mark[]>([]);
  const [note, setNote] = useState("");
  const [typing, setTyping] = useState(false);
  /** The answer being written, when it is one that needs the owner's words. */
  const [changing, setChanging] = useState<Choice | null>(null);
  /** Whether the reasoning behind the recommendation is expanded. */
  const [why, setWhy] = useState(false);

  const issueId = group.issueId ?? "";
  // Fetched for every piece of work, not just the ones already classed as a
  // visual approval. Halted work routinely carries the very preview that would
  // let the owner unblock it, and gating this by kind hid four screenshots and
  // a preview page behind a card that looked like it had nothing.
  const { data: attachments = [] } = useQuery({
    ...issueAttachmentsOptions(issueId),
    enabled: !!issueId,
  });
  const { data: reviewFeedback = [] } = useQuery(ownerReviewFeedbackOptions(issueId));
  const markedEvidenceIds = useMemo(
    () => new Set(reviewFeedback.map((entry) => entry.marked_screenshot_attachment_id)),
    [reviewFeedback],
  );
  // Read across every event on this work, not just the one leading the card.
  // The address of a running preview is routinely written into a follow-up
  // ("the candidate shown at http://…"), and the owner should not have to open
  // the issue to find the link they are being asked to look at.
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
  // Pictures first, whatever order they arrived in. The biggest preview on the
  // card is whichever source leads, and a running page has no thumbnail to
  // show — leading with one put a blank grey panel where the screenshots
  // should have been and pushed three real captures into the small rail.
  const sources = useMemo(() => {
    const all = [...attachmentSources(attachments, markedEvidenceIds), ...pages];
    return [...all.filter((s) => s.kind === "image"), ...all.filter((s) => s.kind !== "image")];
  }, [attachments, markedEvidenceIds, pages]);
  // Plain English everywhere a picture is named: on the card, in the viewer,
  // and in the note an agent reads back. "R-390" and "100.64.218.108:3135"
  // told the owner nothing about which of six things he was looking at.
  const sourceNames = useMemo(() => describeSources(sources), [sources]);

  const asked = useMemo(() => splitQuestion(decision?.question ?? group.ask.detail), [decision?.question, group.ask.detail]);
  const advice = useMemo(
    () => firstSentence(decision?.recommendation ?? group.lead.details?.recommendation),
    [decision?.recommendation, group.lead.details?.recommendation],
  );
  const outcomes = useMemo(
    () => splitOptionConsequences(meaningfulConsequence(decision?.consequence ?? group.lead.details?.consequence)),
    [decision?.consequence, group.lead.details?.consequence],
  );
  const sharedConsequence = outcomes.rest;
  const choices = choicesFor(group);
  /** True when the agent wrote the choices itself, so they are numbered and
   *  anything referring to "Option 2" has something on screen to point at. */
  const numbered = !group.lead.details?.decision_id && parseOptions(group.lead.details?.options).length > 0;
  const { persistMarkedEvidence, savingFeedback } = useOwnerReviewFeedback({
    issueId,
    inboxItemId: group.lead.id,
    sources,
    attachments,
    marks,
    note,
  });

  const answer = async (label: string, directive: OwnerDirectiveType) => {
    const drawn = describeMarks(marks, sourceNames);
    try {
      const evidence = await persistMarkedEvidence();
      const extra = [note.trim(), drawn && `What to change:\n${drawn}`, evidence.summary].filter(Boolean).join("\n\n");
      const succeeded = await send(directive, label, extra, evidence.issueRevision);
      if (succeeded) onCloseViewer();
      return succeeded;
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t(($) => $.owner_detail.review_save_failed));
      return false;
    }
  };

  /**
   * Yes goes straight through; anything else asks what you mean first.
   *
   * "Ask for a change" used to fire the words "Please change this." — an
   * instruction with no instruction in it, which sends the work back round for
   * nothing. A change needs the owner's own sentence, and the box opens with
   * the thing they are judging still in front of them.
   */
  const choose = (choice: Choice) => {
    if (choice.primary) {
      answer(choice.send, choice.directive);
      return;
    }
    setChanging(choice);
  };

  // Waits for this issue to finish loading before acting, so a batch answer
  // reaches every member instead of skipping the ones whose revision had not
  // arrived yet.
  const [fired, setFired] = useState(0);
  useEffect(() => {
    if (!fireOrder || fireOrder.token === fired || sent || !ready) return;
    setFired(fireOrder.token);
    void send(fireOrder.directive, fireOrder.send);
  }, [fireOrder, fired, sent, ready, send]);

  const sendChange = async () => {
    if (!changing) return;
    if (await answer(changing.send, changing.directive)) setChanging(null);
  };

  const cancelTyping = () => {
    setChanging(null);
    setTyping(false);
    setNote("");
  };

  if (sent) {
    return (
      <Shell highlighted={false} muted>
        <p className="text-caption text-muted-foreground">
          {t(($) => $.owner_detail.sent_summary, { title: group.title })}
        </p>
      </Shell>
    );
  }

  const answerBlock = (
    <div className={ANSWER_COLUMN}>
      {choices.map((choice, index) => {
        const outcome = numbered ? outcomes.byOption.get(index + 1) : undefined;
        return (
          <div key={choice.label} className="flex flex-col gap-1">
            <Button
              size="sm"
              variant={variantFor(choice)}
              disabled={!ready || pending || savingFeedback}
              className={ANSWER_BUTTON}
              onClick={() => choose(choice)}
            >
              {numbered && (
                <span className="mr-2 shrink-0 tabular-nums opacity-60">{index + 1}</span>
              )}
              <span className="min-w-0">{choice.label}</span>
            </Button>
            {/* The sentence that used to float above the buttons naming a
                number that appeared nowhere. It belongs here. */}
            {outcome && (
              <p
                className={cn(
                  "text-caption leading-relaxed text-muted-foreground",
                  numbered && "pl-7",
                )}
              >
                {outcome}
              </p>
            )}
            {/* When the agent wrote its own options, its recommendation IS one
                of them. Printing it again above the list as "We'd send the
                preview now" put the same advice on screen twice, in two
                different forms of words. */}
            {numbered && choice.primary && advice.head && (
              <p className="pl-7 text-caption text-muted-foreground">
                {t(($) => $.owner_detail.recommended_choice)}
                {advice.tail && (
                  <button
                    type="button"
                    onClick={() => setWhy((open) => !open)}
                    className="ml-1.5 underline-offset-2 outline-none hover:text-foreground hover:underline focus-visible:ring-2 focus-visible:ring-ring"
                  >
                    {why ? t(($) => $.owner_detail.less) : t(($) => $.owner_detail.why)}
                  </button>
                )}
              </p>
            )}
            {numbered && choice.primary && why && advice.tail && (
              <p className="pl-7 text-caption leading-relaxed text-muted-foreground">
                {advice.tail}
              </p>
            )}
          </div>
        );
      })}
    </div>
  );

  return (
    <Shell highlighted={highlighted}>
      {reviewChanged && <div role="alert" className="text-caption text-muted-foreground">
        {t(($) => $.chat_response.changed)}
        <Button variant="ghost" size="sm" onClick={refreshReview}>{t(($) => $.chat_response.review_latest)}</Button>
      </div>}
      <div className="flex items-start justify-between gap-3">
        <h3 className="min-w-0 text-body font-semibold leading-snug">
          {/* The issue's own number, so this can be talked about, searched for
              and matched to the board without opening anything. */}
          {issue?.identifier && (
            <span className="mr-2 font-mono text-caption text-muted-foreground">
              {issue.identifier}
            </span>
          )}
          {group.title}
        </h3>
        <span className="shrink-0 text-[11px] tabular-nums text-muted-foreground">
          {waitingFor(group.updatedAt)}
        </span>
      </div>

      {asked.parts.length > 0 ? (
        <div className="mt-1.5">
          {asked.lead && (
            <p className="text-body leading-relaxed text-foreground">{asked.lead}</p>
          )}
          <ul className="mt-2 space-y-1.5">
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
        </div>
      ) : (
        <p className="mt-1.5 whitespace-pre-wrap text-body leading-relaxed text-foreground">
          {group.ask.detail}
        </p>
      )}

      {/* One line, and the reasoning behind a toggle. Only when the agent did
          not write numbered options — otherwise it rides on the option it is
          recommending rather than repeating it in different words. */}
      {!numbered && advice.head && (
        <p className="mt-2 text-caption">
          <span className="font-medium text-muted-foreground">{t(($) => $.owner_detail.recommendation_prefix)}</span>{" "}
          <span className="text-muted-foreground">{lowerFirst(advice.head)}</span>
          {advice.tail && (
            <button
              type="button"
              onClick={() => setWhy((open) => !open)}
              className="ml-1.5 text-muted-foreground underline-offset-2 outline-none hover:text-foreground hover:underline focus-visible:ring-2 focus-visible:ring-ring"
            >
              {why ? t(($) => $.owner_detail.less) : t(($) => $.owner_detail.why)}
            </button>
          )}
        </p>
      )}
      {!numbered && why && advice.tail && (
        <p className="mt-1 text-caption text-muted-foreground">{advice.tail}</p>
      )}
      {/* Only what was left over after each option took its own sentence. */}
      {sharedConsequence && (
        <p className="mt-1.5 text-caption text-muted-foreground">{sharedConsequence}</p>
      )}
      {group.failureCount > 0 && group.ask.kind !== "broke" && (
        <p className="mt-1.5 text-caption text-muted-foreground">
          {t(($) => $.owner_detail.failure_count, { count: group.failureCount })}
        </p>
      )}

      {/* The evidence, above the answers, at a size it can be judged at. */}
      {sources.length > 0 && (
        <div className="mt-3">
          <PreviewDeck
            sources={sources}
            names={sourceNames}
            onOpen={(id) => onOpenViewer(id || sources[0]!.id)}
          />
        </div>
      )}

      {(changing || typing) && (
        <div className="mt-3 max-w-[620px]">
          <textarea
            autoFocus
            value={note}
            onChange={(event) => setNote(event.target.value)}
            onKeyDown={(event) => {
              // Sending was a small dim button and nothing else: Enter made a
              // newline and no key finished the job.
              if (event.key === "Enter" && (event.metaKey || event.ctrlKey)) {
                event.preventDefault();
                if (changing) sendChange();
                else if (note.trim()) answer("", "request_changes");
                return;
              }
              if (event.key === "Escape") {
                event.preventDefault();
                cancelTyping();
              }
            }}
            placeholder={
              changing && !changing.primary
                ? `${changing.label} — what should be different?`
                : "Type it the way you'd say it."
            }
            className="min-h-[72px] w-full resize-y rounded-lg border bg-muted/40 p-3 text-body outline-none focus:border-primary"
          />
          <div className="mt-2 flex flex-wrap items-center gap-3">
            <Button
              size="sm"
              disabled={!ready || pending || savingFeedback || (!note.trim() && marks.length === 0)}
              onClick={() => (changing ? sendChange() : answer("", "request_changes"))}
            >
              {t(($) => $.owner_detail.send)}
            </Button>
            <button type="button" onClick={cancelTyping} className={QUIET_BUTTON}>
              {t(($) => $.owner_detail.never_mind)}
            </button>
            <span className="text-[11px] text-muted-foreground">{t(($) => $.owner_detail.send_shortcut)}</span>
          </div>
        </div>
      )}

      <div className="mt-3">{answerBlock}</div>

      <div className="mt-2.5 flex flex-wrap items-center gap-3">
        {/* Always available, whatever the buttons offer: the way out of any
            shape this page failed to anticipate. */}
        <button type="button" onClick={() => setTyping(true)} className={QUIET_BUTTON}>
          {t(($) => $.owner_detail.say_something_else)}
        </button>
        {/* Seen, not decided. Without it the only ways past a card were to
            answer it or to file it away, so "I am not ready" had to be spelled
            as one of two things the owner did not mean. */}
        {group.unread && (
          <button type="button" onClick={onLater} className={QUIET_BUTTON}>
            {t(($) => $.owner_detail.not_now)}
          </button>
        )}
        {/* Opening the work and clearing it are both rare, and one of them
            takes the card away. As two more grey links in a row of four they
            sat a thumb's width from each other. They live behind this. */}
        <CardMenu issueHref={issueId ? paths.issueDetail(issueId) : undefined} onClear={onClear} />
      </div>

      {viewer && (
        <PreviewViewer
          sources={sources}
          startId={viewer.sourceId}
          marks={marks}
          onChangeMarks={setMarks}
          onClose={onCloseViewer}
          title={issue?.identifier ? `${issue.identifier} · ${group.title}` : group.title}
          question={group.ask.detail}
          position={viewer.position}
          onStep={onStepViewer}

        />
      )}
    </Shell>
  );
}

/** The rarely-wanted things, off the surface but never more than one press
 *  away. Nothing in here can be hit while aiming at something else, which is
 *  the whole reason it exists. */
function CardMenu({ issueHref, onClear }: { issueHref?: string; onClear: () => void }) {
  const { t } = useT("inbox");
  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        render={
          <button
            type="button"
            aria-label={t(($) => $.owner_detail.more_work)}
            className="ml-auto inline-flex size-11 items-center justify-center rounded-md text-muted-foreground outline-none transition-colors hover:bg-accent hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring sm:size-8"
          >
            <MoreHorizontal className="size-4" />
          </button>
        }
      />
      <DropdownMenuContent align="end" className="w-auto">
        {issueHref && (
          <DropdownMenuItem render={<a href={issueHref} />}>{t(($) => $.owner_detail.open_work)}</DropdownMenuItem>
        )}
        <DropdownMenuItem onClick={onClear}>{t(($) => $.owner_detail.clear_here)}</DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

/**
 * Several pieces of work stuck for the same reason, answered once.
 *
 * Six issues halted by the same machine crash used to be six entries reading
 * word for word the same — the owner read that as the page repeating itself,
 * and he was right. It is one situation, so it gets one card, and the six
 * subjects are a list underneath for anyone who wants to treat one of them
 * differently.
 */
export function OwnerInboxBatchCard({
  batch,
  onClear,
  onLater,
  viewerProps,
}: {
  batch: OwnerInboxBatch;
  onClear: (group: OwnerInboxGroup) => void;
  onLater: (group: OwnerInboxGroup) => void;
  viewerProps: (group: OwnerInboxGroup) => ViewerProps;
}) {
  const { t } = useT("inbox");
  const [open, setOpen] = useState(false);
  // Set once when the owner answers the whole batch. Each member watches it and
  // sends down its OWN path — the six issues can be parked on different
  // mechanisms, and one failing must not read as all six failing.
  const [order, setOrder] = useState<FireOrder | null>(null);
  const [asking, setAsking] = useState<Choice | null>(null);
  const [note, setNote] = useState("");
  const count = batch.groups.length;
  const newest = batch.groups.reduce(
    (latest, group) => (group.updatedAt > latest ? group.updatedAt : latest),
    batch.groups[0]!.updatedAt,
  );

  /**
   * The same three answers the sentence above them offers.
   *
   * This card said "tell the team to carry on, change what you asked for, or
   * drop them" and then showed one button. The other two existed only inside
   * the six member cards, behind a text link — so the copy promised three
   * choices and the surface gave one.
   */
  const choices: Choice[] =
    batch.kind === "broke"
      ? [
          {
            label: `Start all ${count} again`,
            directive: "correct",
            send: "Start this again.",
            primary: true,
          },
        ]
      : stoppedChoices().map((choice) => ({
          ...choice,
          label:
            choice.directive === "correct"
              ? `Carry on with all ${count}`
              : choice.directive === "cancel"
                ? `Drop all ${count}`
                : choice.label,
        }));

  const fire = (choice: Choice, extra = "") => {
    setOrder({
      token: Date.now(),
      directive: choice.directive,
      send: extra ? `${choice.send}\n\n${extra}` : choice.send,
    });
    setAsking(null);
    setNote("");
  };

  return (
    <Shell>
      <div className="flex items-start justify-between gap-3">
        <h3 className="text-body font-semibold leading-snug">
          {batch.kind === "broke"
            ? t(($) => $.owner_detail.batch_broke, { count })
            : t(($) => $.owner_detail.batch_stopped, { count })}
        </h3>
        <span className="shrink-0 text-[11px] tabular-nums text-muted-foreground">
          {waitingFor(newest)}
        </span>
      </div>
      <p className="mt-1.5 text-body leading-relaxed text-foreground">
        {batch.kind === "stopped"
          ? "Nothing is happening on any of them. Tell the team to carry on, change what you asked for, or drop them."
          : batch.shared}
      </p>

      {asking ? (
        <div className="mt-3 max-w-[620px]">
          <textarea
            autoFocus
            value={note}
            onChange={(event) => setNote(event.target.value)}
            onKeyDown={(event) => {
              if (event.key === "Enter" && (event.metaKey || event.ctrlKey) && note.trim()) {
                event.preventDefault();
                fire(asking, note.trim());
                return;
              }
              if (event.key === "Escape") {
                event.preventDefault();
                setAsking(null);
                setNote("");
              }
            }}
            placeholder={
              asking.danger
                ? `Dropping all ${count}. What should the team know?`
                : `${asking.label} — what should be different?`
            }
            className="min-h-[72px] w-full resize-y rounded-lg border bg-muted/40 p-3 text-body outline-none focus:border-primary"
          />
          <div className="mt-2 flex flex-wrap items-center gap-3">
            <Button size="sm" disabled={!note.trim()} onClick={() => fire(asking, note.trim())}>
              {t(($) => $.owner_detail.send_all, { count })}
            </Button>
            <button
              type="button"
              onClick={() => {
                setAsking(null);
                setNote("");
              }}
              className={QUIET_BUTTON}
            >
              {t(($) => $.owner_detail.never_mind)}
            </button>
          </div>
        </div>
      ) : (
        <div className={cn("mt-3", ANSWER_COLUMN)}>
          {choices.map((choice) => (
            <Button
              key={choice.label}
              size="sm"
              variant={variantFor(choice)}
              disabled={!!order}
              className={ANSWER_BUTTON}
              onClick={() => (choice.primary ? fire(choice) : setAsking(choice))}
            >
              {choice.label}
            </Button>
          ))}
        </div>
      )}

      <div className="mt-2.5">
        <button type="button" onClick={() => setOpen((shown) => !shown)} className={QUIET_BUTTON}>
          {open ? "Hide them" : `Show the ${count} on their own`}
        </button>
      </div>

      {/* Each member is a whole card, not a title and a button. Six pieces of
          work stopped for one reason are still six pieces of work, and one of
          them was carrying four screenshots and a preview page that a summary
          row gave the owner no way to reach.

          Mounted whether or not the list is open, because each card owns the
          hook that can answer for it and the batch buttons need them all
          listening. Collapsed only hides them. */}
      <div className={cn("mt-3 space-y-2 border-t pt-3", !open && "hidden")}>
        {batch.groups.map((group) => (
          <OwnerInboxCard
            key={group.key}
            group={group}
            fireOrder={order}
            onClear={() => onClear(group)}
            onLater={() => onLater(group)}
            {...viewerProps(group)}
          />
        ))}
      </div>
    </Shell>
  );
}

/** A note with nothing to answer. One line, and a way to be rid of it. */
export function OwnerInboxNote({
  group,
  onClear,
}: {
  group: OwnerInboxGroup;
  onClear: () => void;
}) {
  const { t } = useT("inbox");
  return (
    <div className="flex items-start gap-3 rounded-lg px-4 py-2.5 hover:bg-accent/40">
      <div className="min-w-0 flex-1">
        <p className="truncate text-caption font-medium">{group.title}</p>
        <p className="truncate text-caption text-muted-foreground">{group.ask.detail}</p>
      </div>
      <span className="shrink-0 text-[11px] tabular-nums text-muted-foreground">
        {waitingFor(group.updatedAt)}
      </span>
      <button type="button" onClick={onClear} className={cn(QUIET_BUTTON, "shrink-0")}>
        {t(($) => $.owner_detail.clear)}
      </button>
    </div>
  );
}

/**
 * The card itself.
 *
 * Raised off the page rather than level with it. At `--surface` on a
 * `--page-canvas` background the two differed by a contrast ratio of 1.06, so
 * the only thing separating one piece of work from the next was a border at 6%
 * white, and ten of them read as a single wall of text.
 */
function Shell({
  children,
  highlighted,
  muted,
}: {
  children: React.ReactNode;
  highlighted?: boolean;
  muted?: boolean;
}) {
  return (
    <article
      data-testid="owner-inbox-card"
      className={cn(
        "rounded-xl border border-surface-border bg-surface-raised p-4 shadow-[var(--surface-shadow)] transition-colors",
        highlighted && "ring-2 ring-primary/40",
        muted && "bg-muted/30 shadow-none",
      )}
    >
      {children}
    </article>
  );
}

/**
 * A notice with no piece of work behind it — an automation that ran out of its
 * allowance, a quick-create that failed. There is nothing to direct a team to
 * do, but these carry their own repair controls, so the page hands them
 * through rather than dropping them where the detail pane used to be.
 */
export function OwnerInboxNoticeCard({
  group,
  onClear,
  extras,
}: {
  group: OwnerInboxGroup;
  onClear: () => void;
  extras?: React.ReactNode;
}) {
  const { t } = useT("inbox");
  return (
    <Shell>
      <div className="flex items-start justify-between gap-3">
        <h3 className="text-body font-semibold leading-snug">{group.title}</h3>
        <span className="mt-0.5 shrink-0 text-[11px] tabular-nums text-muted-foreground">
          {waitingFor(group.updatedAt)}
        </span>
      </div>
      <p className="mt-1.5 whitespace-pre-wrap text-body leading-relaxed text-foreground">
        {group.ask.detail}
      </p>
      <div className="mt-3 flex flex-wrap items-center gap-2">
        {extras}
        <button type="button" onClick={onClear} className={cn(QUIET_BUTTON, "ml-auto")}>
          {t(($) => $.owner_detail.clear)}
        </button>
      </div>
    </Shell>
  );
}

function lowerFirst(text: string): string {
  return text.charAt(0).toLowerCase() + text.slice(1);
}
