"use client";

import { useCallback, memo } from "react";
import { AppLink } from "../../navigation";
import { useSortable, defaultAnimateLayoutChanges } from "@dnd-kit/sortable";
import type { AnimateLayoutChanges } from "@dnd-kit/sortable";
import { CSS } from "@dnd-kit/utilities";
import type { Issue, IssueProperty, Project, UpdateIssueRequest } from "@multica/core/types";
import { useQuery } from "@tanstack/react-query";
import { useWorkspaceId } from "@multica/core/hooks";
import { propertyListOptions } from "@multica/core/properties";
import { CustomPropertyValueDisplay } from "./pickers/custom-property-picker";
import { descriptionPreview } from "./description-preview";
import { formatDateOnly, isPastDateOnly } from "@multica/core/issues/date";
import { CalendarClock, CalendarDays } from "lucide-react";
import { ActorAvatar } from "../../common/actor-avatar";
import { PropertyIcon } from "../../common/property-icon";
import { useWorkspacePaths } from "@multica/core/paths";
import { useActorName } from "@multica/core/workspace/hooks";
import { useLocale, useT, useTimeAgo } from "../../i18n";
import { ProjectIcon } from "../../projects/components/project-icon";
import { PriorityIcon } from "./priority-icon";
import { PriorityPicker, AssigneePicker, StartDatePicker, DueDatePicker } from "./pickers";
import { useViewStore } from "@multica/core/issues/stores/view-store-context";
import { ProgressRing } from "./progress-ring";
import type { ChildProgress } from "./list-row";
import { IssueActionsContextMenu } from "../actions";
import { LabelChip } from "../../labels/label-chip";
import { IssueAgentActivityIndicator } from "./issue-agent-activity-indicator";
import { CustomStatusChip, useIsCustomStatus } from "./custom-status-chip";
import { useIssueSurfaceActionsOptional } from "../surface/actions-context";
import { issueColumnCategory } from "@multica/core/issues";
import { PRIORITY_CONFIG } from "@multica/core/issues/config/priority";
function formatDate(date: string, locale: string): string {
  return formatDateOnly(date, { month: "short", day: "numeric" }, locale);
}

/** Stops event from bubbling to Link/drag handlers */
function PickerWrapper({ children, className }: { children: React.ReactNode; className?: string }) {
  const stop = (e: React.SyntheticEvent) => {
    e.stopPropagation();
    e.preventDefault();
  };
  return (
    <div onClick={stop} onMouseDown={stop} onPointerDown={stop} className={className}>
      {children}
    </div>
  );
}

export const BoardCardContent = memo(function BoardCardContent({
  issue,
  editable = false,
  childProgress,
  project,
}: {
  issue: Issue;
  editable?: boolean;
  childProgress?: ChildProgress;
  project?: Project;
}) {
  const { t } = useT("issues");
  const locale = useLocale();
  const timeAgo = useTimeAgo();
  const storeProperties = useViewStore((s) => s.cardProperties);
  const cardPropertyIds = useViewStore((s) => s.cardPropertyIds);
  const cardWsId = useWorkspaceId();
  const { data: workspaceProperties = [] } = useQuery(propertyListOptions(cardWsId));
  // Custom properties toggled on in Display options, in toggle order, only
  // when this issue actually carries a value.
  const cardCustomProperties = cardPropertyIds
    .map((id) => workspaceProperties.find((p) => p.id === id))
    .filter((p): p is IssueProperty => !!p && issue.properties?.[p.id] !== undefined);
  const labels = issue.labels ?? [];

  const surfaceActions = useIssueSurfaceActionsOptional();
  const handleUpdate = useCallback(
    (updates: Partial<UpdateIssueRequest>) => {
      surfaceActions?.updateIssue(issue.id, updates, {
        errorMessage: t(($) => $.card.update_failed),
      });
    },
    [issue.id, surfaceActions, t],
  );
  const canEdit = editable && !!surfaceActions;

  const showPriority = storeProperties.priority;
  const showDescription = storeProperties.description && issue.description;
  const showAssigneeSection = storeProperties.assignee;
  const hasAssignee = !!issue.assignee_type && !!issue.assignee_id;
  const showStartDate = storeProperties.startDate && issue.start_date;
  const showDueDate = storeProperties.dueDate && issue.due_date;
  const showProject = storeProperties.project && project;
  const showChildProgress = storeProperties.childProgress && childProgress;
  const showLabels = storeProperties.labels && labels.length > 0;
  // Keeps the chip row from rendering an empty flex container when the status
  // chip is the only thing in it and it decides to render nothing.
  const showCustomStatus = useIsCustomStatus(issue.status);

  const showAssigneeName = showAssigneeSection && hasAssignee && !showStartDate && !showDueDate;
  const showUpdatedHint = showAssigneeName && !showChildProgress;
  const { getActorName } = useActorName();
  const assigneeName =
    showAssigneeName && issue.assignee_type && issue.assignee_id
      ? getActorName(issue.assignee_type, issue.assignee_id)
      : null;

  const priorityLabel = t(($) => $.priority[issue.priority]);
  const priorityCfg = issue.priority in PRIORITY_CONFIG ? PRIORITY_CONFIG[issue.priority] : null;
  const priorityBadgeStyle = priorityCfg && issue.priority !== "none"
    ? `${priorityCfg.badgeBg} ${priorityCfg.badgeText}`
    : "";

  const priorityIconNode = showPriority ? (
    canEdit ? (
      <PickerWrapper className="flex">
        <PriorityPicker
          priority={issue.priority}
          onUpdate={handleUpdate}
          triggerRender={
            <button
              type="button"
              aria-label={priorityLabel}
              className={`inline-flex items-center gap-1 h-5 px-1.5 shrink-0 rounded text-micro font-medium hover:opacity-80 transition-opacity ${priorityBadgeStyle || "hover:bg-muted/60"}`}
            >
              <PriorityIcon priority={issue.priority} inheritColor={!!priorityBadgeStyle} />
              {issue.priority !== "none" && (
                <span className="capitalize">{priorityCfg?.label}</span>
              )}
            </button>
          }
        />
      </PickerWrapper>
    ) : (
      <span
        aria-label={priorityLabel}
        className={`inline-flex items-center gap-1 h-5 px-1.5 shrink-0 rounded text-micro font-medium ${priorityBadgeStyle}`}
      >
        <PriorityIcon priority={issue.priority} inheritColor={!!priorityBadgeStyle} />
        {issue.priority !== "none" && (
          <span className="capitalize">{priorityCfg?.label}</span>
        )}
      </span>
    )
  ) : null;

  // The parent row gives this container the leftover space; min-w-0 and
  // max-w-full make the nested picker trigger respect that limit.
  const assigneeContainerClass = assigneeName
    ? "flex min-w-0 max-w-full items-center"
    : "inline-flex items-center";

  const assigneeInner = hasAssignee ? (
    <span className="flex min-w-0 max-w-full items-center gap-1.5">
      <ActorAvatar
        actorType={issue.assignee_type!}
        actorId={issue.assignee_id!}
        size="sm"
        enableHoverCard
        profileLink={false}
        className="shrink-0"
      />
      {assigneeName && (
        <span className="min-w-0 truncate text-caption text-foreground">{assigneeName}</span>
      )}
    </span>
  ) : (
    <span className="text-caption text-muted-foreground">{t(($) => $.pickers.assignee.trigger_unassigned)}</span>
  );

  const assigneeNode = showAssigneeSection ? (
    canEdit ? (
      <PickerWrapper className={assigneeContainerClass}>
        <AssigneePicker
          assigneeType={issue.assignee_type}
          assigneeId={issue.assignee_id}
          onUpdate={handleUpdate}
          trigger={assigneeInner}
        />
      </PickerWrapper>
    ) : (
      <span className={assigneeContainerClass}>{assigneeInner}</span>
    )
  ) : null;

  const showMetaRow = showAssigneeSection || showStartDate || showDueDate || showChildProgress;
  const showRightMeta = !!showStartDate || !!showDueDate || !!showChildProgress || showUpdatedHint;

  const category = issueColumnCategory(issue);
  const statusBorderColor =
    category === "blocked"
      ? "border-l-rose-500 bg-rose-500/[0.04]"
      : category === "in_progress"
        ? "border-l-amber-500 bg-amber-500/[0.04]"
        : category === "in_review"
          ? "border-l-indigo-500 bg-indigo-500/[0.04]"
          : category === "done"
            ? "border-l-emerald-500 bg-emerald-500/[0.04]"
            : category === "todo"
              ? "border-l-sky-400 bg-sky-400/[0.02]"
              : category === "backlog"
                ? "border-l-slate-400 bg-slate-400/[0.02]"
                : "border-l-muted-foreground/40";

  return (
    <div className={`rounded-lg border-[0.5px] border-surface-border border-l-4 ${statusBorderColor} py-3 px-2.5 shadow-[var(--surface-shadow)] transition-colors group-hover/card:border-foreground/15 group-hover/card:bg-surface-hover group-data-[popup-open]/card:border-foreground/15 group-data-[popup-open]/card:bg-surface-hover`}>
      {/* Row 1: priority + identifier (left), agent activity + assignee (right) */}
      <div className="flex items-center justify-between gap-2">
        <div className="flex items-center gap-1.5 min-w-0">
          {priorityIconNode}
          <span className="font-mono text-xs font-semibold text-muted-foreground">{issue.identifier}</span>
          {category === "blocked" && (
            <span className="inline-flex items-center px-2 py-0.5 rounded-full text-[10px] font-semibold bg-rose-500/20 text-rose-500 border border-rose-500/30">{t(($) => $.operational_copy.blocked)}</span>
          )}
          {category === "in_progress" && (
            <span className="inline-flex items-center px-2 py-0.5 rounded-full text-[10px] font-semibold bg-amber-500/20 text-amber-500 border border-amber-500/30">{t(($) => $.operational_copy.active)}</span>
          )}
          {category === "in_review" && (
            <span className="inline-flex items-center px-2 py-0.5 rounded-full text-[10px] font-semibold bg-indigo-500/20 text-indigo-400 border border-indigo-500/30">{t(($) => $.operational_copy.review)}</span>
          )}
          {category === "done" && (
            <span className="inline-flex items-center px-2 py-0.5 rounded-full text-[10px] font-semibold bg-emerald-500/20 text-emerald-500 border border-emerald-500/30">{t(($) => $.operational_copy.done)}</span>
          )}
        </div>
        <IssueAgentActivityIndicator issueId={issue.id} />
      </div>

      {/* Row 2: Title */}
      <p className="mt-1 text-body font-medium leading-snug line-clamp-2">
        {issue.title}
      </p>

      {showDescription && (() => {
        const preview = descriptionPreview(issue.description!);
        if (!preview) return null;
        return (
          <p className="mt-1 text-caption text-muted-foreground line-clamp-1">
            {preview}
          </p>
        );
      })()}

      {/* Chip row: status + project + labels + custom property values.
          The status chip renders only for a CUSTOM status — the column header
          already names the category. (MUL-6243) */}
      {(showCustomStatus || showProject || showLabels || cardCustomProperties.length > 0) && (
        <div className="mt-1.5 flex items-center gap-1.5 flex-wrap">
          <CustomStatusChip status={issue.status} />
          {showProject && (
            <span className="inline-flex items-center gap-1 rounded-full bg-muted/60 px-1.5 py-0.5 text-micro text-muted-foreground max-w-[160px]">
              <ProjectIcon project={project} size="sm" />
              <span className="truncate">{project!.title}</span>
            </span>
          )}
          {showLabels && labels.map((label) => (
            <LabelChip key={label.id} label={label} />
          ))}
          {cardCustomProperties.map((property) => (
            <span
              key={property.id}
              className="inline-flex max-w-[160px] items-center gap-1 rounded-full bg-muted/60 px-1.5 py-0.5 text-micro text-muted-foreground"
            >
              <PropertyIcon property={property} className="size-3 text-micro" />
              <CustomPropertyValueDisplay property={property} value={issue.properties?.[property.id]} />
            </span>
          ))}
        </div>
      )}

      {/* Meta row: assignee (left), start date, due date, child progress (right) */}
      {showMetaRow && (
        <div className="mt-2 flex items-center justify-between gap-2">
          {showAssigneeSection && (
            <div className="min-w-0 flex-1">
              {assigneeNode}
            </div>
          )}
          {showRightMeta && (
            <div className="ml-auto flex shrink-0 items-center gap-2">
              {showStartDate && (
                canEdit ? (
                  <PickerWrapper className="flex shrink-0">
                    <StartDatePicker
                      startDate={issue.start_date}
                      onUpdate={handleUpdate}
                      trigger={
                        <span className="flex items-center gap-1 text-caption text-muted-foreground">
                          <CalendarClock className="size-3" />
                          {formatDate(issue.start_date!, locale)}
                        </span>
                      }
                    />
                  </PickerWrapper>
                ) : (
                  <span className="flex shrink-0 items-center gap-1 text-caption text-muted-foreground">
                    <CalendarClock className="size-3" />
                    {formatDate(issue.start_date!, locale)}
                  </span>
                )
              )}
              {showDueDate && (
                canEdit ? (
                  <PickerWrapper className="flex shrink-0">
                    <DueDatePicker
                      dueDate={issue.due_date}
                      onUpdate={handleUpdate}
                      trigger={
                        <span
                          className={`flex items-center gap-1 text-caption ${
                            isPastDateOnly(issue.due_date)
                              ? "text-destructive"
                              : "text-muted-foreground"
                          }`}
                        >
                          <CalendarDays className="size-3" />
                          {formatDate(issue.due_date!, locale)}
                        </span>
                      }
                    />
                  </PickerWrapper>
                ) : (
                  <span
                    className={`flex shrink-0 items-center gap-1 text-caption ${
                      isPastDateOnly(issue.due_date)
                        ? "text-destructive"
                        : "text-muted-foreground"
                    }`}
                  >
                    <CalendarDays className="size-3" />
                    {formatDate(issue.due_date!, locale)}
                  </span>
                )
              )}
              {showChildProgress && (
                <div className="inline-flex shrink-0 items-center gap-1">
                  <ProgressRing done={childProgress!.done} total={childProgress!.total} size={14} />
                  <span className="text-micro text-muted-foreground tabular-nums font-medium">
                    {childProgress!.done}/{childProgress!.total}
                  </span>
                </div>
              )}
              {showUpdatedHint && (
                <span className="shrink-0 text-caption text-muted-foreground">
                  {t(($) => $.card.updated_ago, { time: timeAgo(issue.updated_at) })}
                </span>
              )}
            </div>
          )}
        </div>
      )}
    </div>
  );
});

const animateLayoutChanges: AnimateLayoutChanges = (args) => {
  const { isSorting, wasDragging } = args;
  if (isSorting || wasDragging) return false;
  return defaultAnimateLayoutChanges(args);
};

export const DraggableBoardCard = memo(function DraggableBoardCard({
  issue,
  childProgress,
  project,
  disableSorting,
}: {
  issue: Issue;
  childProgress?: ChildProgress;
  project?: Project;
  disableSorting?: boolean;
}) {
  const p = useWorkspacePaths();
  const {
    attributes,
    listeners,
    setNodeRef,
    transform,
    transition,
    isDragging,
  } = useSortable({
    id: issue.id,
    data: { status: issue.status },
    animateLayoutChanges,
    disabled: disableSorting ? { droppable: true } : undefined,
  });

  const style = {
    transform: CSS.Transform.toString(transform),
    transition,
  };

  return (
    <IssueActionsContextMenu issue={issue}>
      <div
        ref={setNodeRef}
        style={style}
        data-board-card=""
        {...attributes}
        {...listeners}
        className={`group/card ${isDragging ? "opacity-30" : ""}`}
      >
        <AppLink
          href={p.issueDetail(issue.id)}
          newTabTitle={issue.identifier}
          className={`group block transition-colors ${isDragging ? "pointer-events-none" : ""}`}
        >
          <BoardCardContent
            issue={issue}
            editable
            childProgress={childProgress}
            project={project}
          />
        </AppLink>
      </div>
    </IssueActionsContextMenu>
  );
});
