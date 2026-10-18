"use client";

import { useMemo, useState } from "react";
import { useQueries, useQuery } from "@tanstack/react-query";
import {
  AlertTriangle,
  ArrowLeft,
  Bot,
  ChevronRight,
  CircleDot,
  Clock3,
  GitBranch,
  Layers3,
  Search,
  Server,
  Sparkles,
} from "lucide-react";
import { Button } from "@multica/ui/components/ui/button";
import { cn } from "@multica/ui/lib/utils";
import { useWorkspaceId } from "@multica/core/hooks";
import { useWorkspacePaths } from "@multica/core/paths";
import { useHqStore } from "@multica/core/issues/stores/hq-store";
import { issueTasksOptions, workforceOverviewIssuesOptions } from "@multica/core/issues/queries";
import { projectListOptions } from "@multica/core/projects/queries";
import { agentListOptions, memberListOptions } from "@multica/core/workspace";
import { agentTaskSnapshotOptions, workspaceWorkingAgentsOptions } from "@multica/core/agents/queries";
import {
  companyOperatingPictureOptions,
  companySystemGraphOptions,
} from "@multica/core/company-operations/queries";
import { AppLink } from "../../navigation";
import { useT } from "../../i18n";
import {
  buildHqMap,
  outcomeMatches,
  visibleHqOutcomes,
  type HqArea,
  type HqLane,
  type HqOutcome,
  type HqSharedIncident,
  type HqUrgency,
} from "./company-hq-model";

const COMPANY_OUTCOME_LIMIT = 8;

function timeAgo(value: string): string {
  const elapsed = Date.now() - Date.parse(value);
  if (!Number.isFinite(elapsed) || elapsed < 0) return "Unknown";
  const minutes = Math.floor(elapsed / 60_000);
  if (minutes < 1) return "Now";
  if (minutes < 60) return `${minutes}m ago`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}h ago`;
  return `${Math.floor(hours / 24)}d ago`;
}

function StateDot({ lane }: { lane: HqLane }) {
  return (
    <span
      aria-hidden="true"
      className={cn(
        "size-2 shrink-0 rounded-full",
        lane === "needs_you" && "bg-destructive",
        lane === "moving" && "bg-brand",
        lane === "waiting" && "bg-muted-foreground",
      )}
    />
  );
}

function urgencyLabel(urgency: HqUrgency): string {
  if (urgency === "critical") return "Critical";
  if (urgency === "high") return "High impact";
  if (urgency === "background") return "Background";
  return "Normal";
}

function stateLabel(lane: HqLane): string {
  if (lane === "needs_you") return "Needs you";
  if (lane === "moving") return "Working now";
  return "Waiting / unclear";
}

function taskEvidence(work: HqOutcome["work"][number]): string | null {
  const task = work.latestTask;
  if (!task) return null;
  if (task.status === "failed") return `Failed${task.error ? ` · ${task.error}` : task.failure_reason ? ` · ${task.failure_reason.replaceAll("_", " ")}` : ""}`;
  if (task.status === "cancelled") return "Cancelled";
  if (task.status === "queued") return "Queued";
  if (task.status === "dispatched") return "Starting";
  if (task.status === "waiting_local_directory") return "Waiting for its work area";
  if (task.status === "running") return "Working now";
  if (task.status === "completed") return "Last run completed";
  return null;
}

function latestOutcomeEvidence(outcome: HqOutcome, fallback: string): string {
  const workWithTask = outcome.work.find((entry) => !!entry.latestTask);
  return (workWithTask ? taskEvidence(workWithTask) : null) ?? outcome.work[0]?.item.latest ?? outcome.description ?? fallback;
}

function nextOutcomeStep(outcome: HqOutcome, fallback: string): string {
  const failed = outcome.work.find((entry) => entry.latestTask?.status === "failed");
  if (failed) return "Fix the failed run, then resume or reassign this work.";
  const cancelled = outcome.work.find((entry) => entry.latestTask?.status === "cancelled");
  if (cancelled) return "Decide whether this work should restart or stay stopped.";
  return outcome.work[0]?.item.next ?? outcome.description ?? fallback;
}

function UrgencyBadge({ urgency }: { urgency: HqUrgency }) {
  return (
    <span
      className={cn(
        "rounded-full border px-2 py-0.5 text-[11px] font-medium",
        urgency === "critical" && "border-destructive/40 bg-destructive/10 text-destructive",
        urgency === "high" && "border-warning/40 bg-warning/10 text-warning",
        urgency === "normal" && "border-surface-border bg-muted/40 text-muted-foreground",
        urgency === "background" && "border-surface-border text-faint-foreground",
      )}
    >
      {urgencyLabel(urgency)}
    </span>
  );
}

function OutcomeNode({
  outcome,
  selected,
  dimmed,
  onSelect,
}: {
  outcome: HqOutcome;
  selected: boolean;
  dimmed: boolean;
  onSelect: () => void;
}) {
  const { t } = useT("issues");
  const latest = latestOutcomeEvidence(outcome, t(($) => $.hq.no_work_recorded));
  const openIncidents = outcome.incidents.filter((incident) => incident.lifecycle_state !== "resolved");

  return (
    <button
      type="button"
      onClick={onSelect}
      aria-pressed={selected}
      className={cn(
        "group relative w-full min-w-0 rounded-xl border bg-surface/95 p-3 text-left shadow-sm transition-[border-color,background-color,opacity,transform]",
        "hover:border-strong hover:bg-surface-raised focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring",
        selected ? "border-brand bg-surface-raised ring-1 ring-brand/30" : "border-surface-border",
        outcome.urgency === "critical" && "border-destructive/50",
        dimmed && "opacity-30",
      )}
    >
      <div className="flex min-w-0 items-start gap-2">
        <StateDot lane={outcome.lane} />
        <h3 className="min-w-0 flex-1 text-body font-semibold leading-5 text-foreground">{outcome.title}</h3>
        <ChevronRight className="mt-0.5 size-4 shrink-0 text-faint-foreground transition-transform group-hover:translate-x-0.5" />
      </div>
      <p className="mt-2 line-clamp-2 text-caption leading-5 text-muted-foreground">{latest}</p>
      <div className="mt-3 flex flex-wrap items-center gap-2 text-[11px] text-muted-foreground">
        <span className={cn(
          "rounded-full border px-2 py-0.5 font-medium",
          outcome.lane === "needs_you" && "border-destructive/40 bg-destructive/10 text-destructive",
          outcome.lane === "moving" && "border-brand/40 bg-brand/10 text-brand",
          outcome.lane === "waiting" && "border-surface-border bg-muted/30 text-muted-foreground",
        )}>{stateLabel(outcome.lane)}</span>
        <UrgencyBadge urgency={outcome.urgency} />
        <span>{t(($) => $.hq.work_count, { count: outcome.work.length })}</span>
        {outcome.workers.length > 0 && <span>· {t(($) => $.hq.active_workers_count, { count: outcome.workers.length })}</span>}
        {openIncidents.length > 0 && (
          <span className="inline-flex items-center gap-1 text-destructive">
            <AlertTriangle className="size-3" />{openIncidents.length}
          </span>
        )}
        <span className={cn("ml-auto inline-flex items-center gap-1", outcome.isStale && "text-warning")}>
          <Clock3 className="size-3" />{outcome.isStale ? "No update " : "Updated "}{timeAgo(outcome.lastUpdatedAt)}
        </span>
      </div>
    </button>
  );
}

function AreaGroup({
  area,
  outcomes,
  query,
  selectedOutcomeId,
  onSelect,
}: {
  area: HqArea;
  outcomes: HqOutcome[];
  query: string;
  selectedOutcomeId: string | null;
  onSelect: (id: string) => void;
}) {
  const { t } = useT("issues");
  return (
    <section className="relative min-w-0 rounded-2xl border border-surface-border bg-surface/30 p-3.5">
      <div className="flex items-center gap-2 px-0.5">
        <span className="flex size-6 items-center justify-center rounded-md border border-surface-border bg-background text-muted-foreground">
          <Layers3 className="size-3.5" />
        </span>
        <h2 className="min-w-0 flex-1 truncate text-label font-medium text-foreground">{area.name}</h2>
        <span className="text-caption text-muted-foreground">{t(($) => $.hq.outcome_count, { count: outcomes.length })}</span>
      </div>
      <div className="relative mt-3 pl-4 before:absolute before:bottom-4 before:left-3 before:top-0 before:w-px before:bg-surface-border">
        <div className="grid grid-cols-[repeat(auto-fit,minmax(min(100%,260px),1fr))] gap-2.5">
        {outcomes.map((outcome) => (
          <div key={outcome.id} className="relative before:absolute before:-left-1 before:top-6 before:h-px before:w-1 before:bg-surface-border">
            <OutcomeNode
              outcome={outcome}
              selected={outcome.id === selectedOutcomeId}
              dimmed={!!query.trim() && !outcomeMatches(outcome, query)}
              onSelect={() => onSelect(outcome.id)}
            />
          </div>
        ))}
        </div>
      </div>
    </section>
  );
}

function SharedImpactRail({
  incidents,
  outcomes,
  onSelect,
}: {
  incidents: HqSharedIncident[];
  outcomes: HqOutcome[];
  onSelect: (id: string) => void;
}) {
  const { t } = useT("issues");
  if (incidents.length === 0) return null;
  const names = new Map(outcomes.map((outcome) => [outcome.id, outcome.title]));
  return (
    <section className="mb-4 rounded-xl border border-warning/30 bg-warning/5 px-3.5 py-3">
      <div className="flex items-center gap-2 text-label font-medium text-foreground">
        <GitBranch className="size-4 text-warning" />
        {t(($) => $.hq.cross_company_impact)}
      </div>
      <div className="mt-2 grid gap-2 lg:grid-cols-2">
        {incidents.slice(0, 4).map(({ incident, outcomeIds }) => (
          <div key={incident.id} className="flex min-w-0 items-start gap-2 rounded-lg border border-surface-border bg-background/55 p-2.5">
            <AlertTriangle className="mt-0.5 size-3.5 shrink-0 text-warning" />
            <div className="min-w-0 flex-1">
              <p className="truncate text-caption font-medium text-foreground">{incident.title}</p>
              <div className="mt-1 flex flex-wrap gap-1">
                {outcomeIds.map((id) => (
                  <button key={id} type="button" onClick={() => onSelect(id)} className="rounded bg-muted px-1.5 py-0.5 text-[10px] text-muted-foreground hover:text-foreground">
                    {names.get(id) ?? id}
                  </button>
                ))}
              </div>
            </div>
          </div>
        ))}
      </div>
    </section>
  );
}

function WorkRow({ work }: { work: HqOutcome["work"][number] }) {
  const { t } = useT("issues");
  const paths = useWorkspacePaths();
  const evidence = taskEvidence(work);
  const failed = work.latestTask?.status === "failed";
  return (
    <AppLink
      href={paths.issueDetail(work.issue.id)}
      className="group relative flex min-w-0 items-start gap-3 rounded-lg border border-surface-border bg-background/55 px-3 py-2.5 transition-colors hover:border-strong hover:bg-muted/30"
    >
      <span className={cn("mt-1.5 size-1.5 shrink-0 rounded-full", work.item.state === "working" ? "bg-brand" : "bg-muted-foreground")} />
      <div className="min-w-0 flex-1">
        <p className="line-clamp-2 text-body text-foreground">{work.issue.title}</p>
        <div className="mt-1 flex flex-wrap gap-x-2 text-caption text-muted-foreground">
          <span>{work.issue.identifier}</span>
          <span className={cn(failed && "text-destructive")}>· {evidence ?? work.item.label}</span>
          {work.assigneeName && <span>· {t(($) => $.hq.assigned_to, { name: work.assigneeName })}</span>}
          {work.workers.map((worker) => <span key={worker.id}>· {worker.name}</span>)}
        </div>
      </div>
      <ChevronRight className="mt-1 size-4 shrink-0 text-faint-foreground transition-transform group-hover:translate-x-0.5" />
    </AppLink>
  );
}

function LocalRelationshipView({ outcome }: { outcome: HqOutcome }) {
  const { t } = useT("issues");
  const nodeNames = new Map(outcome.systems.map((system) => [system.id, system.name]));
  return (
    <section className="mt-5 rounded-xl border border-surface-border bg-background/25 p-3.5">
      <div className="flex items-center gap-2">
        <GitBranch className="size-4 text-muted-foreground" />
        <h3 className="text-body font-medium text-foreground">{t(($) => $.hq.relationship_path)}</h3>
      </div>
      <p className="mt-1 text-caption leading-5 text-muted-foreground">{t(($) => $.hq.relationship_path_hint)}</p>

      <div className="relative mt-4 rounded-lg border border-brand/25 bg-brand/5 px-3 py-2.5">
        <span className="text-[10px] font-medium uppercase tracking-[0.14em] text-brand">{t(($) => $.hq.outcome_label)}</span>
        <p className="mt-0.5 text-body font-medium text-foreground">{outcome.title}</p>
      </div>

      <div className="grid gap-4 pt-3 sm:grid-cols-2 lg:grid-cols-1 xl:grid-cols-2">
        <div className="relative border-l border-surface-border pl-3">
          <p className="mb-2 text-[10px] font-medium uppercase tracking-[0.14em] text-muted-foreground">{t(($) => $.hq.work_contributing)}</p>
          <div className="space-y-2">
            {outcome.work.length === 0 ? (
              <p className="text-caption text-muted-foreground">{t(($) => $.hq.no_work_recorded)}</p>
            ) : outcome.work.slice(0, 6).map((work) => <WorkRow key={work.issue.id} work={work} />)}
          </div>
        </div>

        <div className="relative border-l border-surface-border pl-3">
          <p className="mb-2 text-[10px] font-medium uppercase tracking-[0.14em] text-muted-foreground">{t(($) => $.hq.systems_affected)}</p>
          <div className="space-y-2">
            {outcome.systems.length === 0 ? (
              <p className="text-caption text-muted-foreground">{t(($) => $.hq.no_system_evidence)}</p>
            ) : outcome.systems.slice(0, 6).map((system) => (
              <div key={system.id} className="rounded-lg border border-surface-border bg-background/55 px-3 py-2.5">
                <div className="flex items-center gap-2">
                  <Server className="size-3.5 text-muted-foreground" />
                  <span className="min-w-0 flex-1 truncate text-caption font-medium text-foreground">{system.name}</span>
                  <span className={cn("text-[10px]", system.coverage === "complete" ? "text-success" : "text-warning")}>{system.coverage}</span>
                </div>
                <p className="mt-1 text-[10px] text-muted-foreground">{system.kind} · {system.freshness}</p>
              </div>
            ))}
          </div>
        </div>
      </div>

      {outcome.systemEdges.length > 0 && (
        <div className="mt-4 border-t border-surface-border pt-3">
          <p className="text-[10px] font-medium uppercase tracking-[0.14em] text-muted-foreground">{t(($) => $.hq.known_system_links)}</p>
          <div className="mt-2 flex flex-wrap gap-1.5">
            {outcome.systemEdges.map((edge) => (
              <span key={edge.id} className="rounded-md border border-surface-border bg-muted/25 px-2 py-1 text-[10px] text-muted-foreground">
                {nodeNames.get(edge.from) ?? edge.from} → {edge.kind.replaceAll("_", " ")} → {nodeNames.get(edge.to) ?? edge.to}
              </span>
            ))}
          </div>
        </div>
      )}
    </section>
  );
}

function OutcomeDetail({ outcome, onClose }: { outcome: HqOutcome; onClose: () => void }) {
  const { t } = useT("issues");
  const paths = useWorkspacePaths();
  const next = nextOutcomeStep(outcome, t(($) => $.hq.no_work_recorded));
  const openIncidents = outcome.incidents.filter((incident) => incident.lifecycle_state !== "resolved");
  const recordHref = outcome.work[0]
    ? paths.issueDetail(outcome.work[0].issue.id)
    : outcome.project
      ? paths.projectDetail(outcome.project.id)
      : null;

  return (
    <aside className="min-w-0 border-t border-surface-border bg-surface lg:sticky lg:top-0 lg:h-full lg:border-l lg:border-t-0">
      <div className="flex h-full min-h-0 flex-col">
        <div className="flex items-center gap-2 border-b border-surface-border px-4 py-3">
          <Button variant="ghost" size="sm" onClick={onClose} className="-ml-2 lg:hidden">
            <ArrowLeft />{t(($) => $.hq.all_outcomes)}
          </Button>
          <span className="min-w-0 flex-1 truncate text-caption font-medium uppercase tracking-wide text-muted-foreground">
            {t(($) => $.hq.outcome_details)}
          </span>
          <UrgencyBadge urgency={outcome.urgency} />
          <Button variant="ghost" size="icon-sm" onClick={onClose} className="hidden lg:inline-flex" aria-label={t(($) => $.hq.close_details)}>
            <ArrowLeft />
          </Button>
        </div>

        <div className="min-h-0 flex-1 overflow-auto p-4">
          <div className="flex items-start gap-2.5">
            <StateDot lane={outcome.lane} />
            <div className="min-w-0 flex-1">
              <h2 className="text-title-sm font-semibold text-foreground">{outcome.title}</h2>
              {outcome.description && <p className="mt-1 text-caption leading-5 text-muted-foreground">{outcome.description}</p>}
            </div>
          </div>

          <section className="mt-4 rounded-xl border border-brand/25 bg-brand/5 p-3.5">
            <div className="flex items-center gap-2 text-label font-medium text-foreground"><Sparkles className="size-4 text-brand" />{t(($) => $.hq.what_happens_next)}</div>
            <p className="mt-2 text-body leading-6 text-muted-foreground">{next}</p>
          </section>

          <LocalRelationshipView outcome={outcome} />

          <div className="mt-4 grid gap-3 sm:grid-cols-2 lg:grid-cols-1 xl:grid-cols-2">
            <section className="rounded-xl border border-surface-border bg-background/35 p-3.5">
              <div className="flex items-center gap-2"><Bot className="size-4 text-muted-foreground" /><h3 className="text-body font-medium">{t(($) => $.hq.workers)}</h3></div>
              <div className="mt-2 space-y-1.5">
                {outcome.workers.map((worker) => <p key={worker.id} className="text-caption text-foreground"><span className="text-brand">{t(($) => $.hq.working_now)}</span> · {worker.name}</p>)}
                {outcome.assignees.filter((name) => !outcome.workers.some((worker) => worker.name === name)).map((name) => (
                  <p key={name} className="text-caption text-foreground"><span className="text-muted-foreground">{t(($) => $.hq.assigned)}</span> · {name}</p>
                ))}
                {outcome.workers.length === 0 && outcome.assignees.length === 0 && (
                  <p className="text-caption text-muted-foreground">{t(($) => $.hq.no_worker_assigned)}</p>
                )}
              </div>
            </section>

            <section className="rounded-xl border border-surface-border bg-background/35 p-3.5">
              <div className="flex items-center gap-2"><CircleDot className="size-4 text-muted-foreground" /><h3 className="text-body font-medium">{t(($) => $.hq.current_state)}</h3></div>
              <p className="mt-2 text-caption text-foreground">{outcome.lane === "moving" ? t(($) => $.hq.lanes.moving) : outcome.lane === "needs_you" ? t(($) => $.hq.lanes.needs_you) : t(($) => $.hq.lanes.waiting)}</p>
              <p className={cn("mt-1 text-caption", outcome.isStale ? "text-warning" : "text-muted-foreground")}>{timeAgo(outcome.lastUpdatedAt)}</p>
            </section>
          </div>

          {openIncidents.length > 0 && (
            <section className="mt-3 rounded-xl border border-destructive/35 bg-destructive/5 p-3.5">
              <div className="flex items-center gap-2 text-destructive"><AlertTriangle className="size-4" /><h3 className="text-body font-medium">{t(($) => $.hq.operational_impact)}</h3></div>
              <div className="mt-2 space-y-3">
                {openIncidents.map((incident) => (
                  <div key={incident.id}>
                    <p className="text-body text-foreground">{incident.title}</p>
                    <p className="mt-1 text-caption leading-5 text-muted-foreground">{incident.next_step}</p>
                  </div>
                ))}
              </div>
            </section>
          )}

          {recordHref && (
            <AppLink href={recordHref} className="mt-5 inline-flex items-center gap-1 text-label text-muted-foreground hover:text-foreground">
              {t(($) => $.hq.open_full_record)}<ChevronRight className="size-3" />
            </AppLink>
          )}
        </div>
      </div>
    </aside>
  );
}

function CompanySummary({ outcomes }: { outcomes: HqOutcome[] }) {
  const { t } = useT("issues");
  const active = outcomes.filter((outcome) => outcome.lane === "moving").length;
  const needOwner = outcomes.filter((outcome) => outcome.lane === "needs_you").length;
  const incidents = new Set(outcomes.flatMap((outcome) => outcome.incidents.filter((incident) => incident.lifecycle_state !== "resolved").map((incident) => incident.id))).size;
  const stale = outcomes.filter((outcome) => outcome.isStale).length;
  return (
    <div className="flex flex-wrap items-center gap-x-4 gap-y-1 text-caption text-muted-foreground">
      <span><strong className="font-medium text-foreground">{active}</strong> {t(($) => $.hq.moving_count)}</span>
      <span><strong className={cn("font-medium", needOwner > 0 ? "text-destructive" : "text-foreground")}>{needOwner}</strong> {t(($) => $.hq.need_you_count)}</span>
      <span><strong className={cn("font-medium", incidents > 0 ? "text-destructive" : "text-foreground")}>{incidents}</strong> {t(($) => $.hq.incidents_summary)}</span>
      <span><strong className="font-medium text-foreground">{stale}</strong> {t(($) => $.hq.stale_count)}</span>
    </div>
  );
}

export function CompanyHq() {
  const { t } = useT("issues");
  const wsId = useWorkspaceId();
  const selectedOutcomeId = useHqStore((state) => state.selectedOutcomeId);
  const selectOutcome = useHqStore((state) => state.selectOutcome);
  const [query, setQuery] = useState("");
  const { data: issues = [], isPending: issuesPending } = useQuery(workforceOverviewIssuesOptions(wsId));
  const { data: projects = [] } = useQuery(projectListOptions(wsId));
  const { data: agents = [] } = useQuery(agentListOptions(wsId));
  const { data: members = [] } = useQuery(memberListOptions(wsId));
  const { data: tasks = [] } = useQuery(agentTaskSnapshotOptions(wsId));
  const { data: workingAgents = [] } = useQuery(workspaceWorkingAgentsOptions(wsId, "issue"));
  const operatingPicture = useQuery(companyOperatingPictureOptions(wsId));
  const systemGraph = useQuery(companySystemGraphOptions(wsId));

  const map = useMemo(() => buildHqMap({
    issues,
    projects,
    agents,
    members,
    tasks,
    workingAgents,
    services: operatingPicture.data?.services ?? [],
    incidents: operatingPicture.data?.incidents ?? [],
    systemGraph: systemGraph.data ?? { version: "", generated_at: "", nodes: [], edges: [], journeys: [], regressions: [] },
  }), [issues, projects, agents, members, tasks, workingAgents, operatingPicture.data, systemGraph.data]);

  const ranked = useMemo(() => {
    if (!query.trim()) return map.outcomes;
    return [...map.outcomes].sort((left, right) => Number(outcomeMatches(right, query)) - Number(outcomeMatches(left, query)));
  }, [map.outcomes, query]);
  const { visible, overflow } = visibleHqOutcomes(ranked, COMPANY_OUTCOME_LIMIT);
  const visibleIds = new Set(visible.map((outcome) => outcome.id));
  const visibleRank = new Map(visible.map((outcome, index) => [outcome.id, index]));
  const visibleAreas = map.areas.map((area) => ({
    ...area,
    outcomeIds: area.outcomeIds.filter((id) => visibleIds.has(id)),
  })).filter((area) => area.outcomeIds.length > 0)
    .sort((left, right) =>
      Math.min(...left.outcomeIds.map((id) => visibleRank.get(id) ?? Number.MAX_SAFE_INTEGER)) -
      Math.min(...right.outcomeIds.map((id) => visibleRank.get(id) ?? Number.MAX_SAFE_INTEGER))
    );
  const outcomesById = new Map(map.outcomes.map((outcome) => [outcome.id, outcome]));
  const selected = map.outcomes.find((outcome) => outcome.id === selectedOutcomeId) ?? null;
  const selectedTaskQueries = useQueries({
    queries: (selected?.work ?? []).map((entry) => issueTasksOptions(entry.issue.id)),
  });
  const selectedWithDetailedTasks = useMemo(() => {
    if (!selected) return null;
    return {
      ...selected,
      work: selected.work.map((entry, index) => {
        const detailedTasks = selectedTaskQueries[index]?.data ?? [];
        const latestTask = [...detailedTasks]
          .sort((left, right) => Date.parse(right.created_at) - Date.parse(left.created_at))[0] ?? entry.latestTask;
        return { ...entry, latestTask };
      }),
    };
  }, [selected, selectedTaskQueries]);

  return (
    <div className="flex min-h-0 flex-1 flex-col bg-background">
      <div className="shrink-0 border-b border-surface-border bg-surface/70 px-4 py-2.5">
        <div className="flex flex-wrap items-center gap-3">
          <label className="flex h-8 min-w-[220px] flex-1 items-center gap-2 rounded-lg border border-input bg-background px-2.5 text-muted-foreground focus-within:border-ring focus-within:ring-2 focus-within:ring-ring/30 sm:max-w-md">
            <Search className="size-4" />
            <span className="sr-only">{t(($) => $.hq.search)}</span>
            <input value={query} onChange={(event) => setQuery(event.target.value)} placeholder={t(($) => $.hq.search)} className="min-w-0 flex-1 bg-transparent text-body text-foreground outline-none placeholder:text-muted-foreground" />
          </label>
          <CompanySummary outcomes={map.outcomes} />
        </div>
      </div>

      <div className={cn("grid min-h-0 flex-1", selected && "lg:grid-cols-[minmax(0,1fr)_minmax(380px,44%)]")}>
        <main className={cn("min-h-0 overflow-auto", selected && "hidden lg:block")}>
          <div className="mx-auto w-full max-w-[1500px] p-4 sm:p-5">
            <div className="mb-4 flex items-end justify-between gap-4">
              <div>
                <p className="text-label font-medium uppercase tracking-[0.16em] text-muted-foreground">{t(($) => $.hq.company_picture)}</p>
                <p className="mt-1 max-w-2xl text-caption leading-5 text-muted-foreground">{t(($) => $.hq.company_map_hint)}</p>
              </div>
              <span className="shrink-0 text-caption text-muted-foreground">{t(($) => $.hq.shown_count, { count: visible.length })}</span>
            </div>

            {issuesPending ? (
              <div className="flex min-h-[320px] items-center justify-center text-body text-muted-foreground">{t(($) => $.hq.loading)}</div>
            ) : map.outcomes.length === 0 ? (
              <div className="flex min-h-[320px] items-center justify-center text-body text-muted-foreground">{t(($) => $.hq.empty)}</div>
            ) : (
              <>
                <SharedImpactRail incidents={map.sharedIncidents} outcomes={map.outcomes} onSelect={selectOutcome} />
                <div className="grid items-start gap-3">
                  {visibleAreas.map((area) => (
                    <AreaGroup
                      key={area.id}
                      area={area}
                      outcomes={area.outcomeIds.map((id) => outcomesById.get(id)).filter((outcome): outcome is HqOutcome => !!outcome)}
                      query={query}
                      selectedOutcomeId={selectedOutcomeId}
                      onSelect={selectOutcome}
                    />
                  ))}
                </div>
                {overflow > 0 && (
                  <div className="mt-3 rounded-xl border border-dashed border-surface-border px-3 py-2.5 text-caption text-muted-foreground">
                    {t(($) => $.hq.more_outcomes, { count: overflow })}
                  </div>
                )}
              </>
            )}
          </div>
        </main>

        {selectedWithDetailedTasks && <OutcomeDetail outcome={selectedWithDetailedTasks} onClose={() => selectOutcome(null)} />}
      </div>
    </div>
  );
}
