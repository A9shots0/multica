"use client";

import { useEffect, useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { Brain, RotateCcw } from "lucide-react";
import { toast } from "sonner";
import { clientErrorMessage } from "@multica/core/api";
import type { DevelopmentMemoryDraft, DevelopmentMemoryProposal } from "@multica/core/types";
import { issueDevelopmentMemoryOptions } from "@multica/core/issues/queries";
import {
  useApproveIssueDevelopmentMemory,
  useRejectIssueDevelopmentMemory,
  useRetryIssueDevelopmentMemory,
} from "@multica/core/issues/mutations";
import { Button } from "@multica/ui/components/ui/button";
import { Textarea } from "@multica/ui/components/ui/textarea";
import { cn } from "@multica/ui/lib/utils";
import { useT } from "../../i18n";

export function DevelopmentMemorySection({ wsId, issueId }: { wsId: string; issueId: string }) {
  const { data: proposals = [] } = useQuery(issueDevelopmentMemoryOptions(wsId, issueId));
  return <DevelopmentMemoryReviewList wsId={wsId} proposals={proposals} />;
}

export function DevelopmentMemoryReviewList({ wsId, proposals, className }: {
  wsId: string;
  proposals: DevelopmentMemoryProposal[];
  className?: string;
}) {
  const { t } = useT("issues");
  const attention = useMemo(
    () => proposals.filter((proposal) =>
      proposal.reviewStatus === "pending" ||
      (proposal.reviewStatus === "approved" && proposal.deliveredAt == null && proposal.deadLetteredAt != null)),
    [proposals],
  );
  if (attention.length === 0) return null;

  return (
    <section className={cn("space-y-2", className)} aria-label={t(($) => $.development_memory.review_aria)}>
      {attention.map((proposal) => (
        <DevelopmentMemoryReviewCard key={proposal.id} wsId={wsId} proposal={proposal} />
      ))}
    </section>
  );
}

function DevelopmentMemoryReviewCard({ wsId, proposal }: { wsId: string; proposal: DevelopmentMemoryProposal }) {
  const { t } = useT("issues");
  const approve = useApproveIssueDevelopmentMemory(wsId, proposal.issueId);
  const reject = useRejectIssueDevelopmentMemory(wsId, proposal.issueId);
  const retry = useRetryIssueDevelopmentMemory(wsId, proposal.issueId);
  const sourceStatements = useMemo(
    () => proposal.candidate.memoryItems.map((item) => item.statement),
    [proposal.candidate.memoryItems],
  );
  const [editing, setEditing] = useState(false);
  const [expanded, setExpanded] = useState(false);
  const [statements, setStatements] = useState(sourceStatements);
  const busy = approve.isPending || reject.isPending || retry.isPending;

  useEffect(() => {
    setStatements(sourceStatements);
    setEditing(false);
    setExpanded(false);
  }, [proposal.id, proposal.updatedAt, sourceStatements]);

  if (proposal.reviewStatus === "approved") {
    return (
      <div className="rounded-lg border border-destructive/30 bg-destructive/5 p-3">
        <div className="flex items-start gap-2.5">
          <Brain className="mt-0.5 size-4 shrink-0 text-destructive" />
          <div className="min-w-0 flex-1">
            <p className="text-body font-medium">{t(($) => $.development_memory.delivery_problem)}</p>
            <p className="mt-0.5 text-caption text-muted-foreground">
              {t(($) => $.development_memory.delivery_problem_description)}
            </p>
          </div>
          <Button type="button" variant="outline" size="sm" disabled={busy} onClick={() => retry.mutate(proposal.id, {
            onSuccess: () => toast.success(t(($) => $.development_memory.retry_queued)),
            onError: (error) => toast.error(clientErrorMessage(error) ?? t(($) => $.development_memory.retry_failed)),
          })}>
            <RotateCcw className="size-3.5" />
            {t(($) => $.development_memory.retry)}
          </Button>
        </div>
      </div>
    );
  }

  const remember = (payload: DevelopmentMemoryDraft) => approve.mutate({
    proposalId: proposal.id,
    payload,
    note: editing ? "Owner edited and remembered this context." : "Owner chose to remember this context.",
  }, {
    onSuccess: () => toast.success(t(($) => $.development_memory.remembered_toast)),
    onError: (error) => toast.error(clientErrorMessage(error) ?? t(($) => $.development_memory.approval_failed)),
  });

  const editedPayload = (): DevelopmentMemoryDraft => ({
    ...proposal.candidate,
    summary: statements.join(" ").trim(),
    memoryItems: proposal.candidate.memoryItems.map((item, index) => ({
      ...item,
      statement: statements[index]?.trim() ?? "",
    })).filter((item) => item.statement.length > 0),
  });

  return (
    <div className="rounded-lg border bg-card p-3 shadow-xs">
      <div className="flex items-start gap-2.5">
        <Brain className="mt-0.5 size-4 shrink-0 text-brand" />
        <div className="min-w-0 flex-1">
          <p className="text-body font-semibold">{t(($) => $.development_memory.owner_title)}</p>
          {proposal.issueTitle && (
            <p className="mt-0.5 truncate text-caption text-muted-foreground">
              {t(($) => $.development_memory.from_work, { title: proposal.issueTitle })}
            </p>
          )}
          <p className="mt-1 text-caption text-muted-foreground">
            {t(($) => $.development_memory.owner_description)}
          </p>
        </div>
      </div>

      <div className="mt-3 space-y-2">
        {editing ? statements.map((statement, index) => (
          <Textarea key={index} aria-label={t(($) => $.development_memory.edit_item_aria, { number: index + 1 })}
            value={statement} rows={2} maxLength={4000}
            onChange={(event) => setStatements((current) => current.map((value, itemIndex) => itemIndex === index ? event.target.value : value))} />
        )) : sourceStatements.slice(0, expanded ? sourceStatements.length : 2).map((statement, index) => (
          <p key={index} className={cn("rounded-md bg-muted/45 px-3 py-2 text-body leading-relaxed", !expanded && statement.length > 220 && "line-clamp-3")}>{statement}</p>
        ))}
        {!editing && (sourceStatements.length > 2 || sourceStatements.some((statement) => statement.length > 220)) && (
          <button type="button" className="text-caption text-muted-foreground underline-offset-2 hover:text-foreground hover:underline" onClick={() => setExpanded((value) => !value)}>
            {t(($) => expanded ? $.development_memory.show_less : $.development_memory.show_full)}
          </button>
        )}
      </div>

      <div className="mt-3 flex flex-wrap gap-2">
        {editing ? (
          <>
            <Button type="button" size="sm" disabled={busy || editedPayload().memoryItems.length === 0} onClick={() => remember(editedPayload())}>
              {t(($) => $.development_memory.save_remember)}
            </Button>
            <Button type="button" variant="ghost" size="sm" disabled={busy} onClick={() => {
              setStatements(sourceStatements);
              setEditing(false);
            }}>
              {t(($) => $.development_memory.cancel)}
            </Button>
          </>
        ) : (
          <>
            <Button type="button" size="sm" disabled={busy} onClick={() => remember(proposal.candidate)}>
              {t(($) => $.development_memory.remember)}
            </Button>
            <Button type="button" variant="outline" size="sm" disabled={busy} onClick={() => setEditing(true)}>
              {t(($) => $.development_memory.edit)}
            </Button>
            <Button type="button" variant="ghost" size="sm" disabled={busy} onClick={() => reject.mutate({ proposalId: proposal.id, note: "Not useful for future work." }, {
              onSuccess: () => toast.success(t(($) => $.development_memory.ignored_toast)),
              onError: (error) => toast.error(clientErrorMessage(error) ?? t(($) => $.development_memory.rejection_failed)),
            })}>
              {t(($) => $.development_memory.ignore)}
            </Button>
          </>
        )}
      </div>
    </div>
  );
}
