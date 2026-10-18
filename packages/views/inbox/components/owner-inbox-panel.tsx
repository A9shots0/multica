"use client";

import { useState, useMemo } from "react";
import { useQuery } from "@tanstack/react-query";
import { ExternalLink, MessageSquareText, Send, ShieldAlert, CheckCircle2, ChevronDown, ChevronRight, Eye } from "lucide-react";
import { Button } from "@multica/ui/components/ui/button";
import { Textarea } from "@multica/ui/components/ui/textarea";
import { RichContent } from "../../rich-content/rich-content";
import { useWorkspaceId } from "@multica/core/hooks";
import { useWorkspacePaths } from "@multica/core/paths";
import { issueDetailOptions } from "@multica/core/issues/queries";
import { useCreateComment } from "@multica/core/issues/mutations";
import { ownerNotificationOptions, workflowDecisionOptions } from "@multica/core/inbox/queries";
import { ownerInboxCategory } from "@multica/core/inbox/owner-control";
import {
  useAcknowledgeWorkflowAttention,
  useAnswerWorkflowOwnerDecision,
  useCreateOwnerDirective,
  useRespondWorkflowDecision,
} from "@multica/core/inbox/mutations";
import type {
  AtomicWorkflowDecisionResponseType,
  InboxItem,
  OwnerDirectiveType,
  WorkflowDecisionResponseType,
} from "@multica/core/types";
import { toast } from "sonner";
import { useT } from "../../i18n";
import type { OwnerRuntimeRouteRequest } from "@multica/core/types";
import { OwnerRuntimeRouteFields } from "./owner-runtime-route-fields";

function allowedResponses(type: string): AtomicWorkflowDecisionResponseType[] {
  return type === "authorization" ? ["approve", "reject"] : [];
}

export function OwnerInboxPanel({ item, onAcknowledge }: { item: InboxItem; onAcknowledge: () => void }) {
  const { t } = useT("inbox");
  const wsId = useWorkspaceId();
  const paths = useWorkspacePaths();
  const category = ownerInboxCategory(item);
  const isTaskFailure = item.type === "task_failed";
  const isTaskQuestion = item.type === "workflow_owner_decision" && item.details?.question_scope === "ordinary_task";
  const isWorkflowQuestion = item.type === "workflow_owner_decision" && !isTaskQuestion && !item.details?.decision_id && !!item.details?.workflow_event_id;
  const isIntake = item.type === "chat_intake" || isTaskQuestion;
  const categoryCopy = (() => {
    switch (category) {
      case "needs_you": return { label: t(($) => $.owner_control.categories.needs_you_label), why: t(($) => $.owner_control.categories.needs_you_why) };
      case "incident": return { label: t(($) => $.owner_control.categories.incident_label), why: t(($) => $.owner_control.categories.incident_why) };
      case "update": return { label: t(($) => $.owner_control.categories.update_label), why: t(($) => $.owner_control.categories.update_why) };
      case "completion": return { label: t(($) => $.owner_control.categories.completion_label), why: t(($) => $.owner_control.categories.completion_why) };
      default: return { label: t(($) => $.owner_control.categories.history_label), why: t(($) => $.owner_control.categories.history_why) };
    }
  })();
  const actionLabel = (action: WorkflowDecisionResponseType) => {
    switch (action) {
      case "approve": return t(($) => $.owner_control.actions.approve);
      case "reject": return t(($) => $.owner_control.actions.reject);
      case "request_changes": return t(($) => $.owner_control.actions.request_changes);
      case "partial_acceptance": return t(($) => $.owner_control.actions.partial_acceptance);
      case "provide_fact": return t(($) => $.owner_control.actions.provide_fact);
      case "ask_question": return t(($) => $.owner_control.actions.ask_question);
      case "hold": return t(($) => $.owner_control.actions.hold);
      case "cancel": return t(($) => $.owner_control.actions.cancel);
      case "change_scope": return t(($) => $.owner_control.actions.change_scope);
      case "change_budget": return t(($) => $.owner_control.actions.change_budget);
      case "external_step_complete": return t(($) => $.owner_control.actions.external_step_complete);
      case "authentication_complete": return t(($) => $.owner_control.actions.authentication_complete);
      default: return action;
    }
  };
  const decisionId = item.details?.decision_id ?? "";
  const { data: issue } = useQuery(issueDetailOptions(wsId, item.issue_id ?? ""));
  const { data: packet } = useQuery(workflowDecisionOptions(item.issue_id ?? "", decisionId));
  const { data: notificationReceipts = [] } = useQuery(ownerNotificationOptions(item.issue_id ?? ""));
  const createComment = useCreateComment(item.issue_id ?? "");
  const createDirective = useCreateOwnerDirective();
  const respond = useRespondWorkflowDecision();
  const acknowledge = useAcknowledgeWorkflowAttention();
  const answerWorkflowQuestion = useAnswerWorkflowOwnerDecision();
  const [showIssuePreview, setShowIssuePreview] = useState(false);
  const [note, setNote] = useState("");
  const [ownerRoute, setOwnerRoute] = useState<OwnerRuntimeRouteRequest | null | undefined>(undefined);
  const [selectedAction, setSelectedAction] = useState<AtomicWorkflowDecisionResponseType | null>(null);
  const [selectedDirective, setSelectedDirective] = useState<OwnerDirectiveType | null>(null);
  const summary = issue?.current_picture?.owner_summary;
  const evidenceEntries = Object.entries(issue?.current_picture?.evidence_summary ?? {})
    .filter(([, value]) => typeof value === "string" || typeof value === "number" || typeof value === "boolean")
    .slice(0, 6);
  const decision = packet?.decision;
  const attention = packet?.attention;
  const recommendation = decision?.recommendation ?? (isTaskQuestion || isWorkflowQuestion ? item.details?.recommendation : undefined);
  const consequence = decision?.consequence ?? (isTaskQuestion || isWorkflowQuestion ? item.details?.consequence : undefined);
  const needsDetail = selectedAction && !["approve", "reject", "hold", "cancel"].includes(selectedAction);
  const directiveLabel = (directive: OwnerDirectiveType) => {
    switch (directive) {
      case "correct": return t(($) => $.owner_control.directives.correct);
      case "request_changes": return t(($) => $.owner_control.directives.request_changes);
      case "partial_acceptance": return t(($) => $.owner_control.directives.partial_acceptance);
      case "change_scope": return t(($) => $.owner_control.directives.change_scope);
      case "cancel": return t(($) => $.owner_control.directives.cancel);
    }
  };

  const submitComment = async () => {
    if (!item.issue_id || !note.trim()) return;
    await createComment.mutateAsync({ content: note.trim() });
    setNote("");
    toast.success(t(($) => $.owner_control.comment_success));
  };

  const submitDecision = async (outcome: AtomicWorkflowDecisionResponseType) => {
    if (!item.issue_id || !decision) return;
    if (!["approve", "reject", "hold", "cancel"].includes(outcome) && !note.trim()) {
      setSelectedAction(outcome);
      return;
    }
    await respond.mutateAsync({
      issueId: item.issue_id,
      decisionId: decision.id,
      request: { expected_revision: decision.revision, outcome, comment: note.trim() || undefined },
    });
    setNote("");
    setSelectedAction(null);
    toast.success(t(($) => $.owner_control.response_success));
  };

  const submitDirective = async (directive: OwnerDirectiveType) => {
    if (!item.issue_id) return;
    if (selectedDirective !== directive || !note.trim()) {
      setSelectedDirective(directive);
      return;
    }
    if (!issue?.revision) throw new Error(t(($) => $.owner_control.directive_failed));
    await createDirective.mutateAsync({
      issueId: item.issue_id,
      request: { expected_issue_revision: issue.revision, directive, content: note.trim() },
    });
    setNote("");
    setSelectedDirective(null);
    toast.success(t(($) => $.owner_control.directive_success));
  };

  const continueIntake = async () => {
    if (!item.issue_id || !issue?.revision || ownerRoute === null) return;
    await createDirective.mutateAsync({ issueId: item.issue_id, request: {
      expected_issue_revision: issue.revision, directive: "correct",
      content: note.trim() || (!isTaskQuestion ? item.details?.request : "") || "", intake_id: item.id,
      owner_runtime_route: ownerRoute,
    } });
    setNote("");
    toast.success(t(($) => $.owner_control.directive_success));
  };

  const parsedOptions = useMemo(() => {
    if (!item.details?.options) return [];
    if (Array.isArray(item.details.options)) return (item.details.options as unknown[]).map(String).filter(Boolean);
    return item.details.options
      .split(/\r?\n/)
      .map((s) => s.replace(/^\d+[.)]\s*/, "").trim())
      .filter(Boolean);
  }, [item.details?.options]);

  const handleSelectOption = async (option: string) => {
    try {
      if (isWorkflowQuestion) {
        await answerWorkflowQuestion.mutateAsync({ item, answer: option });
        toast.success(t(($) => $.owner_control.response_success));
      } else if (isIntake && item.issue_id && issue?.revision) {
        await createDirective.mutateAsync({
          issueId: item.issue_id,
          request: {
            expected_issue_revision: issue.revision,
            directive: "correct",
            content: option,
            intake_id: item.id,
            owner_runtime_route: ownerRoute ?? undefined,
          },
        });
        toast.success(t(($) => $.owner_control.directive_success));
      } else if (item.issue_id && issue?.revision) {
        await createDirective.mutateAsync({
          issueId: item.issue_id,
          request: {
            expected_issue_revision: issue.revision,
            directive: "correct",
            content: option,
          },
        });
        toast.success(t(($) => $.owner_control.directive_success));
      } else {
        setNote(option);
      }
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t(($) => $.operational_copy.submit_option_failed));
    }
  };

  return (
    <div className="flex min-h-0 flex-1 flex-col overflow-y-auto">
      <div className="border-b px-6 py-5">
        <div className="flex flex-wrap items-center gap-2">
          <span className="rounded-full bg-muted px-2.5 py-1 text-caption font-semibold">{categoryCopy.label}</span>
          {summary?.stage && <span className="text-caption text-muted-foreground">{t(($) => $.owner_control.current_state, { state: summary.stage.replaceAll("_", " ") })}</span>}
        </div>
        <h2 className="mt-3 text-title font-semibold">{issue?.identifier ? `${issue.identifier} · ` : ""}{issue?.title ?? item.title}</h2>
        <p className="mt-2 text-body text-muted-foreground">{categoryCopy.why}</p>

        {item.issue_id && (
          <div className="mt-4 space-y-2">
            <div className="flex items-center justify-between rounded-lg border bg-muted/40 px-3.5 py-2.5">
              <div className="flex items-center gap-2 text-caption">
                <span className="font-semibold text-foreground">{issue?.identifier ?? "Issue"}</span>
                <span className="text-muted-foreground truncate max-w-[280px]">{issue?.title}</span>
              </div>
              <div className="flex items-center gap-2">
                {issue?.description && (
                  <Button
                    size="sm"
                    variant="ghost"
                    onClick={() => setShowIssuePreview(!showIssuePreview)}
                    className="h-7 gap-1 px-2 text-caption text-muted-foreground hover:text-foreground"
                  >
                    <Eye className="size-3.5" />
                    <span>{showIssuePreview ? "Hide preview" : "Preview issue context"}</span>
                    {showIssuePreview ? <ChevronDown className="size-3.5" /> : <ChevronRight className="size-3.5" />}
                  </Button>
                )}
                <a
                  href={paths.issueDetail(item.issue_id)}
                  className="inline-flex items-center gap-1.5 rounded-md bg-background px-2.5 py-1 text-caption font-medium shadow-xs border hover:bg-accent hover:text-accent-foreground transition-colors"
                >
                  <span>{t(($) => $.operational_copy.open_full_issue)}</span>
                  <ExternalLink className="size-3.5" />
                </a>
              </div>
            </div>

            {showIssuePreview && issue?.description && (
              <div className="rounded-lg border bg-card p-4 shadow-inner max-h-60 overflow-y-auto">
                <h4 className="text-micro font-semibold uppercase tracking-wider text-muted-foreground mb-2">{t(($) => $.operational_copy.issue_description_preview)}</h4>
                <div className="text-caption text-foreground">
                  <RichContent content={issue.description} />
                </div>
              </div>
            )}
          </div>
        )}
      </div>

      <div className="space-y-4 px-6 py-5">
        {/* Core question / issue without repetitive headings */}
        <div>
          <p className="whitespace-pre-wrap text-body font-medium leading-relaxed text-foreground">
            {decision?.question ?? (isWorkflowQuestion ? item.details?.question : undefined) ?? item.body ?? (isTaskFailure ? "An agent run failed. No reply is required; inspect the incident and recover the run when the provider/runtime is available." : item.title)}
          </p>
        </div>

        {isTaskFailure && (
          <section className="rounded-lg border border-red-500/30 bg-red-500/5 p-4">
            <h3 className="text-caption font-semibold uppercase tracking-wide text-muted-foreground">{categoryCopy.label}</h3>
            <p className="mt-2 text-body">{t(($) => $.operational_copy.incident_alert_description)}</p>
          </section>
        )}

        {recommendation && (
          <section className="rounded-lg border bg-muted/30 p-4">
            <h3 className="text-caption font-semibold text-foreground">{t(($) => $.owner_control.recommended)}</h3>
            <p className="mt-1 text-body">{recommendation}</p>

            {parsedOptions.length > 0 ? (
              <div className="mt-4 pt-3 border-t border-border/60">
                <span className="block text-caption font-medium text-muted-foreground mb-2">{t(($) => $.operational_copy.select_an_action)}</span>
                <div className="flex flex-wrap gap-2">
                  {parsedOptions.map((opt, idx) => (
                    <Button
                      key={idx}
                      size="sm"
                      variant={idx === 0 ? "default" : "outline"}
                      disabled={answerWorkflowQuestion.isPending || createDirective.isPending}
                      onClick={() => void handleSelectOption(opt)}
                      className="flex items-center gap-1.5 font-medium"
                    >
                      <CheckCircle2 className="size-3.5" />
                      {opt}
                    </Button>
                  ))}
                </div>
              </div>
            ) : (
              (isTaskQuestion || isWorkflowQuestion) && item.details?.options && (
                <p className="mt-2 whitespace-pre-wrap text-body">{item.details.options}</p>
              )
            )}

            {consequence && (
              <details className="mt-3 group text-caption text-muted-foreground cursor-pointer">
                <summary className="hover:text-foreground font-medium select-none">{t(($) => $.operational_copy.impact_consequence_details)}</summary>
                <p className="mt-1.5 pl-2 border-l border-border/80">{consequence}</p>
              </details>
            )}
          </section>
        )}

        {summary?.latest_meaningful_change && (
          <section>
            <h3 className="text-caption font-semibold uppercase tracking-wide text-muted-foreground">{t(($) => $.owner_control.latest_progress)}</h3>
            <p className="mt-2 text-body">{summary.latest_meaningful_change.code.replaceAll("_", " ")} · {summary.latest_meaningful_change.result_code}</p>
          </section>
        )}

        {evidenceEntries.length > 0 && (
          <section className="rounded-lg border p-4">
            <h3 className="text-caption font-semibold uppercase tracking-wide text-muted-foreground">{t(($) => $.owner_control.evidence)}</h3>
            <dl className="mt-2 space-y-2">
              {evidenceEntries.map(([key, value]) => (
                <div key={key} className="grid grid-cols-[minmax(7rem,auto)_1fr] gap-3 text-caption">
                  <dt className="font-medium">{key.replaceAll("_", " ")}</dt>
                  <dd className="min-w-0 break-words text-muted-foreground">
                    {typeof value === "string" && /^https?:\/\//.test(value)
                      ? <a className="text-primary underline" href={value} target="_blank" rel="noreferrer">{t(($) => $.owner_control.open_evidence)}</a>
                      : String(value)}
                  </dd>
                </div>
              ))}
            </dl>
          </section>
        )}

        {notificationReceipts.length > 0 && (
          <section className="rounded-lg border p-4">
            <div className="flex items-center gap-2">
              <MessageSquareText className="size-4 text-muted-foreground" />
              <h3 className="text-caption font-semibold uppercase tracking-wide text-muted-foreground">{t(($) => $.owner_control.sms_delivery)}</h3>
            </div>
            <div className="mt-3 space-y-2">
              {notificationReceipts.slice(0, 3).map((receipt) => (
                <div key={receipt.id} className="flex flex-wrap items-center justify-between gap-2 text-caption">
                  <span>{receipt.event_type.replaceAll("_", " ")}</span>
                  <span className="rounded-full bg-muted px-2 py-1 font-medium">
                    {receipt.delivery_mode === "dry_run"
                      ? t(($) => $.owner_control.sms_dry_run)
                      : receipt.delivery_mode === "live_test"
                        ? t(($) => $.owner_control.sms_live_test, { state: receipt.state.replaceAll("_", " ") })
                        : receipt.state.replaceAll("_", " ")}
                  </span>
                </div>
              ))}
            </div>
            <p className="mt-3 text-caption text-muted-foreground">{t(($) => $.owner_control.sms_authority)}</p>
          </section>
        )}

        {decision && decision.state === "unanswered" && (
          <section className="space-y-3">
            <h3 className="text-caption font-semibold uppercase tracking-wide text-muted-foreground">{t(($) => $.owner_control.respond)}</h3>
            <div className="flex flex-wrap gap-2">
              {allowedResponses(decision.type).map((action) => (
                <Button
                  key={action}
                  size="sm"
                  variant={action === "approve" ? "default" : "outline"}
                  disabled={respond.isPending}
                  onClick={() => void submitDecision(action).catch((error) => toast.error(error instanceof Error ? error.message : t(($) => $.owner_control.response_failed)))}
                >{actionLabel(action)}</Button>
              ))}
            </div>
            {needsDetail && <p className="text-caption text-muted-foreground">{t(($) => $.owner_control.detail_instruction)}</p>}
          </section>
        )}

        {item.type === "chat_intake" && item.details?.intake_state === "pending" && <OwnerRuntimeRouteFields key={item.id} workspaceId={wsId} onChange={setOwnerRoute} />}
        {isIntake && item.details?.intake_state === "pending" && (
          <Button disabled={createDirective.isPending || !issue?.revision || ownerRoute === null || (isTaskQuestion && !note.trim())}
            onClick={() => void continueIntake().catch((error) => toast.error(error instanceof Error ? error.message : t(($) => $.owner_control.directive_failed)))}>
            {isTaskQuestion ? t(($) => $.owner_control.answer_and_continue) : t(($) => $.owner_control.continue_work)}
          </Button>
        )}

        {isWorkflowQuestion && !item.archived && (
          <Button disabled={!note.trim() || answerWorkflowQuestion.isPending}
            onClick={() => void answerWorkflowQuestion.mutateAsync({ item, answer: note.trim() }).then(() => {
              setNote("");
              toast.success(t(($) => $.owner_control.response_success));
            }).catch((error) => toast.error(error instanceof Error ? error.message : t(($) => $.owner_control.response_failed)))}>
            {t(($) => $.owner_control.answer_and_continue)}
          </Button>
        )}

        {!isTaskFailure && !isIntake && !isWorkflowQuestion && item.issue_id && (
          <section className="space-y-3">
            <div>
              <h3 className="text-caption font-semibold uppercase tracking-wide text-muted-foreground">{t(($) => $.owner_control.direct_work)}</h3>
              <p className="mt-1 text-caption text-muted-foreground">{t(($) => $.owner_control.direct_work_help)}</p>
            </div>
            <div className="flex flex-wrap gap-2">
              {(["correct", "request_changes", "partial_acceptance", "change_scope", "cancel"] as OwnerDirectiveType[]).map((directive) => (
                <Button
                  key={directive}
                  size="sm"
                  variant={selectedDirective === directive ? "secondary" : "outline"}
                  disabled={createComment.isPending || createDirective.isPending}
                  onClick={() => void submitDirective(directive).catch((error) => toast.error(error instanceof Error ? error.message : t(($) => $.owner_control.directive_failed)))}
                >{directiveLabel(directive)}</Button>
              ))}
            </div>
            {selectedDirective && <p className="text-caption text-muted-foreground">{t(($) => $.owner_control.directive_instruction, { action: directiveLabel(selectedDirective) })}</p>}
          </section>
        )}

        {!isTaskFailure && <section className="space-y-2">
          <h3 className="text-caption font-semibold uppercase tracking-wide text-muted-foreground">{t(($) => $.owner_control.comment_heading)}</h3>
          <Textarea value={note} onChange={(event) => setNote(event.target.value)} placeholder={t(($) => $.owner_control.comment_placeholder)} rows={4} />
          {!isIntake && !isWorkflowQuestion && <Button size="sm" variant="outline" disabled={!note.trim() || createComment.isPending} onClick={() => void submitComment().catch((error) => toast.error(error instanceof Error ? error.message : t(($) => $.owner_control.comment_failed)))}>
            <Send className="mr-1.5 size-3.5" /> {t(($) => $.owner_control.add_comment)}
          </Button>}
        </section>}

        <div className="flex flex-wrap gap-2 border-t pt-4">
          {item.issue_id && <a className="inline-flex h-8 items-center rounded-md border bg-background px-3 text-caption font-medium hover:bg-accent" href={paths.issueDetail(item.issue_id)}><ExternalLink className="mr-1.5 size-3.5" />{t(($) => $.owner_control.inspect)}</a>}
          {attention && !attention.acknowledged ? (
            <Button size="sm" variant="ghost" disabled={acknowledge.isPending} onClick={() => void acknowledge.mutateAsync({ issueId: item.issue_id!, attentionId: attention.id }).then(onAcknowledge).catch((error) => toast.error(error instanceof Error ? error.message : t(($) => $.owner_control.ack_failed)))}>{t(($) => $.owner_control.acknowledge)}</Button>
          ) : category !== "needs_you" ? (
            <Button size="sm" variant="ghost" onClick={onAcknowledge}>{t(($) => $.owner_control.acknowledge)}</Button>
          ) : null}
        </div>

        {!isIntake && !isWorkflowQuestion && !decisionId && category === "needs_you" && (
          <div className="flex gap-2 rounded-lg border border-amber-500/40 bg-amber-500/10 p-3 text-caption">
            <ShieldAlert className="mt-0.5 size-4 shrink-0" />
            <p>{t(($) => $.owner_control.legacy_warning)}</p>
          </div>
        )}
      </div>
    </div>
  );
}
