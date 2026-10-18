"use client";

import { memo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { agentTaskSnapshotOptions } from "@multica/core/agents";
import { useWorkspaceId } from "@multica/core/hooks";
import { AppLink } from "../../navigation";
import type {
  Issue,
  AgentTask,
  ExternalWorkUpdate,
  OwnerIssueStage,
  OwnerIssueSummary,
  OwnerMeaningfulChange,
} from "@multica/core/types";
import { useWorkspacePaths } from "@multica/core/paths";
import { issueStatusCategory } from "@multica/core/issues";
import { useT, useTimeAgo } from "../../i18n";
import type { ChildProgress } from "./list-row";
import {
  ExternalWorkHistory,
  externalWorkProviderName,
  externalWorkSummaryFromPicture,
} from "./external-work-history";

export type OperationalLane =
  | "not_started"
  | "needs_owner"
  | "active"
  | "waiting_automatic"
  | "in_review"
  | "releasing"
  | "deployed_checking"
  | "verified_live"
  | "recovering"
  | "technical_stop"
  | "done"
  | "needs_next_step";

export const OPERATIONAL_LANES: readonly OperationalLane[] = [
  "not_started",
  "needs_owner",
  "active",
  "waiting_automatic",
  "in_review",
  "releasing",
  "deployed_checking",
  "verified_live",
  "recovering",
  "technical_stop",
  "done",
  "needs_next_step",
];

/**
 * The columns the OWNER sees (SILV owner feedback, 2026-09-21).
 *
 * The twelve operational lanes describe what the controller is doing;
 * Releasing, Deployed-checking, Verified Live and Regression/rollback are all
 * states of "an agent is working on it" that the owner cannot act on
 * differently. Twelve columns of that is not a board you can read at a glance,
 * which is the only thing the owner uses it for.
 *
 * Six columns answer the questions actually being asked: is it mine, has it
 * begun, is it moving, is it being checked, is it finished, is it broken. The
 * lanes are kept intact underneath — this is a presentation grouping, so
 * per-lane wording on the cards, the Current Picture and every test that
 * asserts a lane all keep working.
 *
 * Each column reuses an existing lane's translation key rather than adding new
 * strings, so all four shipped locales stay complete.
 */
export type OwnerBoardColumn = "needs_owner" | "not_started" | "active" | "in_review" | "done" | "technical_stop";

export const OWNER_BOARD_COLUMNS: readonly OwnerBoardColumn[] = [
  "needs_owner",
  "not_started",
  "active",
  "in_review",
  "done",
  "technical_stop",
];

const LANE_TO_OWNER_COLUMN: Record<OperationalLane, OwnerBoardColumn> = {
  needs_owner: "needs_owner",
  not_started: "not_started",
  // Everything an agent is currently carrying reads as one thing to the owner.
  active: "active",
  waiting_automatic: "active",
  releasing: "active",
  deployed_checking: "active",
  recovering: "active",
  in_review: "in_review",
  // A verified-live issue is done; the distinction is release detail.
  verified_live: "done",
  done: "done",
  technical_stop: "technical_stop",
  // Work whose next step nobody can name is stuck, not quietly in progress.
  needs_next_step: "technical_stop",
};

export function ownerBoardColumnForIssue(issue: Issue, tasks?: readonly AgentTask[]): OwnerBoardColumn {
  return LANE_TO_OWNER_COLUMN[operationalLaneForIssue(issue, tasks)];
}

function hasSignal(summary: OwnerIssueSummary, code: string): boolean {
  return summary.signals.some((signal) => signal.code === code);
}

/** True only for work that is still waiting to be picked up.
 *
 * A missing Current Picture is not proof that work has never started. The
 * durable issue status remains the safe fallback for legacy and agent-routed
 * issues while the controller projection is being rebuilt. */
export function issueHasNotStarted(issue: Issue, tasks?: readonly AgentTask[]): boolean {
  if (issue.owner_question) return false;
  if (issue.current_picture) return false;
  if (tasks?.some((task) => task.issue_id === issue.id)) return false;
  const category = issueStatusCategory(issue);
  return category === "backlog" || category === "todo";
}

/** Map durable issue status into an owner lane when no controller projection
 * has been published yet. Unverified execution requires a next-step decision. */
function durableStatusFallbackLane(issue: Issue): OperationalLane {
  switch (issueStatusCategory(issue)) {
    case "backlog":
    case "todo":
      return "not_started";
    case "in_progress":
      return "needs_next_step";
    case "in_review":
      return "in_review";
    case "blocked":
      return "technical_stop";
    case "done":
      return "done";
    case "cancelled":
      return "technical_stop";
    default:
      return "needs_next_step";
  }
}

export function operationalLaneForIssue(issue: Issue, tasks?: readonly AgentTask[]): OperationalLane {
  if (!issue.current_picture && issue.owner_question && !["done", "cancelled"].includes(issueStatusCategory(issue) ?? "")) return "needs_owner";
  if (!issue.current_picture && tasks && ["backlog", "todo", "in_progress"].includes(issueStatusCategory(issue) ?? "")) {
    const own = tasks.filter((task) => task.issue_id === issue.id);
    if (own.some((task) => task.status === "running")) return "active";
    if (own.some((task) => ["queued", "dispatched", "waiting_local_directory"].includes(task.status))) return "waiting_automatic";
    if (own.length > 0 || issueStatusCategory(issue) === "in_progress") return "needs_next_step";
  }
  if (issueHasNotStarted(issue, tasks)) return "not_started";
  if (!issue.current_picture) return durableStatusFallbackLane(issue);
  if (!ownerSummaryIsRevisionAligned(issue)) return "needs_next_step";
  const summary = issue.current_picture?.owner_summary;
  if (!summary || summary.stage === "status_unavailable") return "needs_next_step";
  if (summary.stage === "needs_owner") return "needs_owner";
  if (summary.stage === "recovering") return "recovering";
  if (summary.stage === "technical_stop" || summary.stage === "blocked") return "technical_stop";
  if (summary.stage === "verifying") return "deployed_checking";
  if (summary.stage === "releasing") return "releasing";
  if (summary.stage === "in_review") return "in_review";
  if (summary.stage === "complete") {
    return hasSignal(summary, "verified_live") ? "verified_live" : "done";
  }
  if (
    hasSignal(summary, "waiting_automatic") ||
    hasSignal(summary, "automatic_continuation") ||
    summary.next_action?.automatic === true
  ) {
    return "waiting_automatic";
  }
  return "active";
}

export function newestMeaningfulChange(
  summary: OwnerIssueSummary | undefined,
  activityRevision: number | undefined,
  activity: OwnerMeaningfulChange | undefined,
): OwnerMeaningfulChange | null {
  if (!summary) return null;
  if (activity && activityRevision === summary.current_picture_revision) return activity;
  return summary.latest_meaningful_change;
}

export function ownerSummaryIsRevisionAligned(issue: Issue): boolean {
  const picture = issue.current_picture;
  const summary = picture?.owner_summary;
  return !!summary && summary.current_picture_revision === picture.revision;
}

function waitText(value: unknown): string | undefined {
  return typeof value === "string" && value.trim() ? value.trim() : undefined;
}

/** Recovery metadata explains a parked legacy issue; it never outranks a
 * Current Picture or makes a running/resolved issue look parked. */
function operationalWaitDetails(issue: Issue) {
  const picture = issue.current_picture;
  if (picture) {
    if (!ownerSummaryIsRevisionAligned(issue) || picture.owner_summary.stage !== "needs_owner") return undefined;
    const question = waitText(picture.wait_reason?.question);
    return question ? {
      reason: question,
      nextAction: waitText(picture.wait_reason?.consequence),
      ownerQuestion: question,
    } : undefined;
  }
  if (issue.owner_question && !["done", "cancelled"].includes(issueStatusCategory(issue) ?? "")) {
    return { reason: issue.owner_question.question, ownerQuestion: issue.owner_question.question };
  }
  if (issueStatusCategory(issue) !== "blocked") return undefined;
  const kind = waitText(issue.metadata?.["recovery.wait_kind"]);
  const reason = waitText(issue.metadata?.["recovery.wait_reason"]);
  if (!kind || !reason) return undefined;
  return {
    reason,
    nextAction: waitText(issue.metadata?.["recovery.next_action"]),
    ownerQuestion: kind === "owner" || kind.startsWith("owner_") ? reason : undefined,
  };
}

function completedOrdinaryRun(issue: Issue, tasks?: readonly AgentTask[]): boolean {
  if (issue.current_picture || !tasks || ["done", "cancelled"].includes(issueStatusCategory(issue) ?? "")) return false;
  const own = tasks.filter((task) => task.issue_id === issue.id);
  if (own.some((task) => ["running", "queued", "dispatched", "waiting_local_directory"].includes(task.status))) return false;
  return own.toSorted((a, b) => (b.created_at ?? "").localeCompare(a.created_at ?? ""))[0]?.status === "completed";
}

export interface OperationalIssueTreeNode {
  issue: Issue;
  children: OperationalIssueTreeNode[];
}

function issueOrder(a: Issue, b: Issue): number {
  const aStage = a.stage ?? Number.MAX_SAFE_INTEGER;
  const bStage = b.stage ?? Number.MAX_SAFE_INTEGER;
  return aStage - bStage || a.position - b.position || a.number - b.number;
}

/** Build the parent tree for the owner board without inventing parents that
 * were filtered out of the current surface. A child whose parent is absent
 * remains a top-level card so filters never make work unreachable. */
export function buildOperationalIssueTree(issues: readonly Issue[]): OperationalIssueTreeNode[] {
  const byId = new Map(issues.map((issue) => [issue.id, issue]));
  const childrenByParent = new Map<string, Issue[]>();
  const roots: Issue[] = [];
  for (const issue of issues) {
    if (issue.parent_issue_id && byId.has(issue.parent_issue_id)) {
      const children = childrenByParent.get(issue.parent_issue_id) ?? [];
      children.push(issue);
      childrenByParent.set(issue.parent_issue_id, children);
    } else {
      roots.push(issue);
    }
  }
  const makeNode = (issue: Issue, visiting: Set<string>): OperationalIssueTreeNode => {
    // A malformed cycle must not make the board recurse forever; show the
    // cyclic edge as a normal leaf instead.
    if (visiting.has(issue.id)) return { issue, children: [] };
    const next = new Set(visiting).add(issue.id);
    return {
      issue,
      children: (childrenByParent.get(issue.id) ?? [])
        .toSorted(issueOrder)
        .map((child) => makeNode(child, next)),
    };
  };
  return roots.toSorted(issueOrder).map((issue) => makeNode(issue, new Set()));
}

function stageCopyKey(stage: OwnerIssueStage): string {
  switch (stage) {
    case "needs_owner": return "needs_owner";
    case "recovering": return "recovering";
    case "technical_stop": return "technical_stop";
    case "blocked": return "blocked";
    case "verifying": return "deployed_checking";
    case "releasing": return "releasing";
    case "in_review": return "in_review";
    case "active": return "active";
    case "incoming": return "incoming";
    case "complete": return "done";
    case "status_unavailable":
    default:
      return "needs_next_step";
  }
}

function summaryStateKey(summary: OwnerIssueSummary): string {
  if (hasSignal(summary, "current_regression")) return "current_regression";
  if (hasSignal(summary, "rollback_in_progress")) return "rollback_in_progress";
  if (hasSignal(summary, "service_restored_outcome_open")) return "service_restored_outcome_open";
  return stageCopyKey(summary.stage);
}

function useMeaningfulChangeText(
  change: OwnerMeaningfulChange | null | undefined,
  notStarted = false,
): string {
  const { t } = useT("issues");
  if (!change) {
    return notStarted
      ? t(($) => $.workflow.not_started_progress)
      : t(($) => $.workflow.no_verified_progress);
  }
  return t(($) => $.workflow.meaningful_change, {
    change: t(($) => $.workflow.change[change.code]),
    result: t(($) => $.workflow.result[change.result_code]),
  });
}

function useNextActionText(
  summary: OwnerIssueSummary | undefined,
  notStarted = false,
  hasAssignee = false,
): string {
  const { t } = useT("issues");
  // Telling the owner to assign an issue that already has an owner is the same
  // class of untruth as claiming its status is unavailable.
  if (notStarted) {
    return hasAssignee
      ? t(($) => $.workflow.next.start_when_ready)
      : t(($) => $.workflow.next.assign_or_route);
  }
  if (summary?.next_action?.kind === "answer_decision") return t(($) => $.workflow.next.answer_decision);
  if (summary?.next_action?.kind === "inspect_incident") return t(($) => $.workflow.next.inspect_incident);
  if (summary?.next_action?.automatic === true) return t(($) => $.workflow.next.continues_automatically);
  return t(($) => $.workflow.next.none_recorded);
}

function SummaryState({
  summary,
  notStarted = false,
  fallbackLane,
}: {
  summary: OwnerIssueSummary | undefined;
  notStarted?: boolean;
  fallbackLane?: OperationalLane;
}) {
  const { t } = useT("issues");
  const lane = summary?.stage === "complete" && hasSignal(summary, "verified_live")
    ? "verified_live"
    : summary?.stage === "complete"
      ? "done"
      : fallbackLane ?? "needs_next_step";
  const stageKey = summary
    ? summaryStateKey(summary)
    : notStarted
      ? "not_started"
      : fallbackLane ?? "needs_next_step";
  const label = t(($) => $.workflow.stage[stageKey as keyof typeof $.workflow.stage]);
  const laneLabel = t(($) => $.workflow.lane[lane]);
  return (
    <span className="inline-flex min-w-0 items-center gap-1.5">
      <span className="size-2 shrink-0 rounded-full bg-current opacity-70" aria-hidden="true" />
      <span className="truncate font-medium">{summary
        ? (summary.stage === "complete" ? laneLabel : label)
        : fallbackLane === "not_started" ? label : laneLabel}</span>
    </span>
  );
}

export const OperationalBoardCard = memo(function OperationalBoardCard({
  issue,
  tasks,
  nestedChildren = [],
  childProgress,
}: {
  issue: Issue;
  tasks?: readonly AgentTask[];
  nestedChildren?: OperationalIssueTreeNode[];
  childProgress?: ChildProgress;
}) {
  const { t } = useT("issues");
  const paths = useWorkspacePaths();
  const timeAgo = useTimeAgo();
  const [expanded, setExpanded] = useState(false);
  const aligned = ownerSummaryIsRevisionAligned(issue);
  const summary = aligned ? issue.current_picture?.owner_summary : undefined;
  const meaningful = aligned ? summary?.latest_meaningful_change : null;
  const notStarted = issueHasNotStarted(issue, tasks);
  const fallbackLane = operationalLaneForIssue(issue, tasks);
  const defaultMeaningfulText = useMeaningfulChangeText(meaningful, notStarted);
  const completedRun = completedOrdinaryRun(issue, tasks);
  const meaningfulText = completedRun ? t(($) => $.workflow.execution.completed) : defaultMeaningfulText;
  const defaultNextAction = useNextActionText(summary, notStarted, Boolean(issue.assignee_id));
  const nextActionText = fallbackLane === "needs_next_step" ? t(($) => $.workflow.execution.idle) : !issue.current_picture && issue.owner_question ? t(($) => $.workflow.next.answer_in_inbox) : !issue.current_picture && tasks && ["todo", "backlog", "in_progress"].includes(issueStatusCategory(issue) ?? "") && !notStarted
    ? fallbackLane === "needs_owner" ? t(($) => $.workflow.next.answer_in_inbox) : t(($) => $.workflow.execution[fallbackLane === "active" ? "running" : fallbackLane === "waiting_automatic" ? "queued" : "idle"])
    : defaultNextAction;
  const wait = operationalWaitDetails(issue);
  const external = externalWorkSummaryFromPicture(issue.current_picture);
  const externalMeaningfulText = external
    ? t(($) => $.workflow.external.latest, {
        provider: externalWorkProviderName(external.provider),
        state: t(($) => $.workflow.external.state[external.reportedState]),
        summary: external.summary,
      })
    : null;
  const externalNextAction = external?.nextAction;

  return (
    <div className="space-y-1" data-operational-card-group="">
      <div className="relative">
        <AppLink
          href={paths.issueDetail(issue.id)}
          newTabTitle={issue.identifier}
          className="group block rounded-lg border-[0.5px] border-surface-border bg-surface px-3 py-3 shadow-[var(--surface-shadow)] transition-colors hover:border-foreground/15 hover:bg-surface-hover focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
          data-operational-card=""
        >
          <div className="flex items-center justify-between gap-2 text-caption text-muted-foreground">
            <span className="truncate">{issue.identifier}</span>
            <span className="shrink-0">{timeAgo(issue.updated_at)}</span>
          </div>
          <p className="mt-1 line-clamp-2 text-body font-medium leading-snug">{issue.title}</p>
          <div className="mt-2 flex items-center justify-between gap-2 text-caption text-foreground">
            <SummaryState summary={summary} notStarted={notStarted} fallbackLane={fallbackLane} />
            {childProgress && (
              <span className="shrink-0 tabular-nums text-muted-foreground" aria-label={t(($) => $.workflow.subissues_done, { done: childProgress.done, total: childProgress.total })}>
                {childProgress.done}/{childProgress.total}
              </span>
            )}
          </div>
          <dl className="mt-2 space-y-1 text-caption text-muted-foreground">
            <div>
              <dt className="sr-only">{t(($) => $.workflow.latest_progress)}</dt>
              <dd className="line-clamp-2">{externalMeaningfulText ?? (meaningful || completedRun ? meaningfulText : wait?.reason ?? meaningfulText)}</dd>
            </div>
            {(meaningful || completedRun) && wait?.reason && <div><dd className="line-clamp-2">{wait.reason}</dd></div>}
            <div>
              <dt className="sr-only">{t(($) => $.workflow.next_step)}</dt>
              <dd className="line-clamp-2 text-muted-foreground">{wait?.nextAction ?? externalNextAction ?? nextActionText}</dd>
            </div>
          </dl>
        </AppLink>
        {nestedChildren.length > 0 && (
          <button
            type="button"
            className="absolute bottom-2 right-2 rounded px-1.5 py-0.5 text-micro font-medium text-muted-foreground hover:bg-muted hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
            aria-expanded={expanded}
            aria-label={expanded ? t(($) => $.workflow.collapse_subissues) : t(($) => $.workflow.expand_subissues)}
            onClick={() => setExpanded((value) => !value)}
          >
            {expanded ? "▾" : "▸"} {nestedChildren.length}
          </button>
        )}
      </div>
      {expanded && nestedChildren.length > 0 && (
        <div className="ml-3 space-y-1 border-l-2 border-border/60 pl-2" data-operational-subissues="">
          {nestedChildren.map((child) => (
            <OperationalBoardCard
              key={child.issue.id}
              issue={child.issue}
              tasks={tasks}
              nestedChildren={child.children}
            />
          ))}
        </div>
      )}
    </div>
  );
});

export function IssueNow({
  issue,
  tasks,
  activityRevision,
  newestActivity,
  externalUpdates = [],
  externalUpdatesLoading = false,
  externalUpdatesFailed = false,
}: {
  issue: Issue;
  tasks?: readonly AgentTask[];
  activityRevision?: number;
  newestActivity?: OwnerMeaningfulChange;
  externalUpdates?: ExternalWorkUpdate[];
  externalUpdatesLoading?: boolean;
  externalUpdatesFailed?: boolean;
}) {
  const { t } = useT("issues");
  const summary = issue.current_picture?.owner_summary;
  const aligned = ownerSummaryIsRevisionAligned(issue);
  const safeSummary = aligned ? summary : undefined;
  const meaningful = newestMeaningfulChange(safeSummary, activityRevision, newestActivity);
  const release = issue.current_picture?.contract_version === 3 && aligned
    ? issue.current_picture.release_status
    : null;
  const needsOwner = safeSummary?.stage === "needs_owner" ||
    safeSummary?.signals.some((signal) => signal.code === "owner_attention_required") === true ||
    (safeSummary?.next_action?.automatic === false && safeSummary.next_action.actor_type === "owner");
  const notStarted = issueHasNotStarted(issue, tasks);
  const fallbackLane = operationalLaneForIssue(issue, tasks);
  const defaultMeaningfulText = useMeaningfulChangeText(meaningful, notStarted);
  const completedRun = completedOrdinaryRun(issue, tasks);
  const meaningfulText = completedRun ? t(($) => $.workflow.execution.completed) : defaultMeaningfulText;
  const defaultNextAction = useNextActionText(safeSummary, notStarted, Boolean(issue.assignee_id));
  const nextActionText = fallbackLane === "needs_next_step" ? t(($) => $.workflow.execution.idle) : !issue.current_picture && issue.owner_question ? t(($) => $.workflow.next.answer_in_inbox) : !issue.current_picture && tasks && ["todo", "backlog", "in_progress"].includes(issueStatusCategory(issue) ?? "") && !notStarted
    ? fallbackLane === "needs_owner" ? t(($) => $.workflow.next.answer_in_inbox) : t(($) => $.workflow.execution[fallbackLane === "active" ? "running" : fallbackLane === "waiting_automatic" ? "queued" : "idle"])
    : defaultNextAction;
  const wait = operationalWaitDetails(issue);
  const external = externalWorkSummaryFromPicture(issue.current_picture);
  const externalMeaningfulText = external
    ? t(($) => $.workflow.external.latest, {
        provider: externalWorkProviderName(external.provider),
        state: t(($) => $.workflow.external.state[external.reportedState]),
        summary: external.summary,
      })
    : null;

  return (
    <section id="issue-now" aria-labelledby="issue-now-title" className="mt-6 rounded-xl border bg-muted/20 p-4 md:p-5">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <h2 id="issue-now-title" className="text-title-sm font-semibold">{t(($) => $.workflow.now.title)}</h2>
        <span className="text-caption text-muted-foreground"><SummaryState summary={safeSummary} notStarted={notStarted} fallbackLane={fallbackLane} /></span>
      </div>
      <dl className="mt-4 grid gap-4 sm:grid-cols-2 xl:grid-cols-3">
        <NowField label={t(($) => $.workflow.now.goal)} value={`${issue.identifier}: ${issue.title}`} />
        <NowField
          label={t(($) => $.workflow.now.latest_progress)}
          value={externalMeaningfulText ?? (meaningful || completedRun ? meaningfulText : wait?.reason ?? meaningfulText)}
        />
        <NowField
          label={t(($) => $.workflow.now.next_step)}
          value={wait?.nextAction ?? external?.nextAction ?? nextActionText}
        />
        <NowField
          label={t(($) => $.workflow.now.owner_need)}
          value={wait?.ownerQuestion ?? (needsOwner
            ? t(($) => $.workflow.owner.action_required)
            : t(($) => $.workflow.owner.nothing_needed))}
          emphasis={needsOwner || Boolean(wait?.ownerQuestion)}
        />
        <NowField
          label={t(($) => $.workflow.now.release_state)}
          value={release
            ? t(($) => $.workflow.release[release.phase])
            : t(($) => $.workflow.release.not_applicable)}
        />
      </dl>
      <ExternalWorkHistory updates={externalUpdates} loading={externalUpdatesLoading} failed={externalUpdatesFailed} />
    </section>
  );
}

export function IssueNowWithTaskStatus(props: React.ComponentProps<typeof IssueNow>) {
  const wsId = useWorkspaceId();
  const { data: tasks } = useQuery(agentTaskSnapshotOptions(wsId));
  return <IssueNow {...props} tasks={tasks} />;
}

function NowField({ label, value, emphasis = false }: { label: string; value: string; emphasis?: boolean }) {
  return (
    <div className="min-w-0">
      <dt className="text-caption font-medium text-muted-foreground">{label}</dt>
      <dd className={`mt-1 break-words text-body ${emphasis ? "font-semibold text-foreground" : "text-foreground"}`}>{value}</dd>
    </div>
  );
}
