"use client";

import { useEffect, useMemo, useState, type FormEvent } from "react";
import { useQuery } from "@tanstack/react-query";
import { useDefaultLayout } from "react-resizable-panels";
import { ArrowLeft, BriefcaseBusiness, Inbox, Search, X } from "lucide-react";
import type { Issue } from "@multica/core/types";
import { issueChatLinksOptions } from "@multica/core/chat/queries";
import { useWorkspaceId } from "@multica/core/hooks";
import { useWorkspacePaths } from "@multica/core/paths";
import type { WorkHistoryGroup, WorkHistoryMatch } from "@multica/core/work-history";
import { workHistorySearchOptions } from "@multica/core/work-history/queries";
import { Button } from "@multica/ui/components/ui/button";
import { Input } from "@multica/ui/components/ui/input";
import {
  ResizableHandle,
  ResizablePanel,
  ResizablePanelGroup,
} from "@multica/ui/components/ui/resizable";
import { Skeleton } from "@multica/ui/components/ui/skeleton";
import { useIsCompact } from "@multica/ui/hooks/use-mobile";
import { cn } from "@multica/ui/lib/utils";
import { PAGE_GUTTER, PageHeader } from "../../layout/page-header";
import { useT } from "../../i18n";
import { TalkToMikaButton } from "./talk-to-mika-button";
import { useTimeAgo } from "./inbox-list-item";
import { workforceItems, type WorkforceItem, type WorkforceState } from "./workforce-overview-model";

export function InboxModeTabs({
  mode,
  needsCount,
  workCount,
  onNeedsYou,
  onAllWork,
}: {
  mode: "needs" | "work";
  needsCount: number;
  workCount: number;
  onNeedsYou: () => void;
  onAllWork: () => void;
}) {
  const { t } = useT("inbox");
  return (
    <div className="flex h-10 shrink-0 items-end gap-4 border-b px-4" aria-label={t(($) => $.workforce.view_label)}>
      <ModeTab active={mode === "needs"} onClick={onNeedsYou} label={t(($) => $.workforce.needs_you)} count={needsCount} />
      <ModeTab active={mode === "work"} onClick={onAllWork} label={t(($) => $.workforce.all_work)} count={workCount} />
    </div>
  );
}
function ModeTab({ active, onClick, label, count }: { active: boolean; onClick: () => void; label: string; count: number }) {
  return (
    <button
      type="button"
      onClick={onClick}
      aria-current={active ? "page" : undefined}
      className={cn(
        "relative flex h-full items-center gap-1.5 text-caption font-medium outline-none transition-colors focus-visible:ring-1 focus-visible:ring-ring",
        active ? "text-foreground after:absolute after:inset-x-0 after:bottom-0 after:h-0.5 after:bg-foreground" : "text-muted-foreground hover:text-foreground",
      )}
    >
      {label}
      <span className="tabular-nums text-[10px] text-muted-foreground">{count}</span>
    </button>
  );
}

export function WorkforceOverview({
  issues,
  loading,
  selectedId,
  needsCount,
  onNeedsYou,
  onSelect,
  onClearSelection,
}: {
  issues: readonly Issue[];
  loading: boolean;
  selectedId: string;
  needsCount: number;
  onNeedsYou: () => void;
  onSelect: (issueId: string) => void;
  onClearSelection: () => void;
}) {
  const { t } = useT("inbox");
  const wsId = useWorkspaceId();
  const items = useMemo(() => workforceItems(issues), [issues]);
  const selected = items.find((item) => item.issue.id === selectedId) ?? null;
  const isCompact = useIsCompact();
  const { defaultLayout, onLayoutChanged } = useDefaultLayout({ id: "multica_workforce_overview_layout" });
  const [searchDraft, setSearchDraft] = useState("");
  const [searchQuery, setSearchQuery] = useState("");
  const [selectedHistoryKey, setSelectedHistoryKey] = useState("");
  const historyQuery = useQuery(workHistorySearchOptions(wsId, searchQuery));
  const historyGroups = useMemo(
    () => historyQuery.data?.history.groups ?? [],
    [historyQuery.data],
  );
  const historyMatches = useMemo(
    () => historyGroups.flatMap((group) => group.matches),
    [historyGroups],
  );
  const selectedHistory = historyMatches.find((match) => historyMatchKey(match) === selectedHistoryKey) ?? null;
  const searchingHistory = searchQuery.length > 0;

  const submitSearch = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const next = searchDraft.trim();
    if (!next) return;
    setSearchQuery(next);
    setSelectedHistoryKey("");
  };
  const clearSearch = () => {
    setSearchDraft("");
    setSearchQuery("");
    setSelectedHistoryKey("");
  };

  // Desktop has a permanent detail pane, so leaving it empty wastes most of
  // the screen and makes the owner click before learning anything. Phones keep
  // the list first because opening a detail there replaces the whole screen.
  useEffect(() => {
    if (!loading && !isCompact && !selected && items[0]) onSelect(items[0].issue.id);
  }, [isCompact, items, loading, onSelect, selected]);

  useEffect(() => {
    if (!isCompact && searchingHistory && !selectedHistory && historyMatches[0]) {
      setSelectedHistoryKey(historyMatchKey(historyMatches[0]));
    }
  }, [historyMatches, isCompact, searchingHistory, selectedHistory]);

  const header = (
    <>
      <PageHeader>
        <div className="flex flex-1 items-center gap-2">
          <h1 className="text-body font-semibold">{t(($) => $.page.title)}</h1>
        </div>
      </PageHeader>
      <InboxModeTabs mode="work" needsCount={needsCount} workCount={items.length} onNeedsYou={onNeedsYou} onAllWork={() => undefined} />
      <WorkHistorySearch
        draft={searchDraft}
        activeQuery={searchQuery}
        onDraftChange={setSearchDraft}
        onSubmit={submitSearch}
        onClear={clearSearch}
      />
    </>
  );

  const list = searchingHistory ? (
    <WorkHistoryResults
      groups={historyGroups}
      loading={historyQuery.isLoading}
      error={historyQuery.isError}
      selectedKey={selectedHistoryKey}
      onSelect={setSelectedHistoryKey}
      onRetry={() => void historyQuery.refetch()}
      query={searchQuery}
    />
  ) : loading ? <WorkforceLoading /> : (
    <WorkforceList items={items} selectedId={selectedId} onSelect={onSelect} />
  );

  if (isCompact) {
    if (selectedHistory) {
      return (
        <div className="flex min-h-0 flex-1 flex-col">
          <div className={cn("flex h-12 shrink-0 items-center border-b", PAGE_GUTTER)}>
            <Button variant="ghost" size="sm" onClick={() => setSelectedHistoryKey("")} className="-ml-2 gap-1.5 text-muted-foreground">
              <ArrowLeft className="size-4" /> {t(($) => $.workforce.history.back_to_results)}
            </Button>
          </div>
          <div className="min-h-0 flex-1 overflow-y-auto"><WorkHistoryDetail match={selectedHistory} /></div>
        </div>
      );
    }
    if (searchingHistory) {
      return <div className="flex min-h-0 flex-1 flex-col">{header}{list}</div>;
    }
    if (selected) {
      return (
        <div className="flex min-h-0 flex-1 flex-col">
          <div className={cn("flex h-12 shrink-0 items-center border-b", PAGE_GUTTER)}>
            <Button variant="ghost" size="sm" onClick={onClearSelection} className="-ml-2 gap-1.5 text-muted-foreground">
              <ArrowLeft className="size-4" /> {t(($) => $.workforce.all_work)}
            </Button>
          </div>
          <div className="min-h-0 flex-1 overflow-y-auto"><WorkforceDetail item={selected} /></div>
        </div>
      );
    }
    return <div className="flex min-h-0 flex-1 flex-col">{header}{list}</div>;
  }

  return (
    <ResizablePanelGroup orientation="horizontal" className="min-h-0 flex-1" defaultLayout={defaultLayout} onLayoutChanged={onLayoutChanged}>
      <ResizablePanel id="work-list" defaultSize={360} minSize={320} maxSize={560} groupResizeBehavior="preserve-pixel-size">
        <div className="flex h-full flex-col border-r">{header}{list}</div>
      </ResizablePanel>
      <ResizableHandle />
      <ResizablePanel id="work-detail" minSize="40%">
        {searchingHistory
          ? selectedHistory
            ? <WorkHistoryDetail match={selectedHistory} />
            : <WorkHistoryEmpty hasResults={historyMatches.length > 0} />
          : selected
            ? <WorkforceDetail item={selected} />
            : <WorkforceEmpty hasItems={items.length > 0} />}
      </ResizablePanel>
    </ResizablePanelGroup>
  );
}

function WorkHistorySearch({
  draft,
  activeQuery,
  onDraftChange,
  onSubmit,
  onClear,
}: {
  draft: string;
  activeQuery: string;
  onDraftChange: (value: string) => void;
  onSubmit: (event: FormEvent<HTMLFormElement>) => void;
  onClear: () => void;
}) {
  const { t } = useT("inbox");
  return (
    <form onSubmit={onSubmit} className="flex shrink-0 items-center gap-2 border-b px-3 py-2" role="search">
      <div className="relative min-w-0 flex-1">
        <Search className="pointer-events-none absolute left-2.5 top-1/2 size-3.5 -translate-y-1/2 text-muted-foreground" />
        <Input
          type="search"
          value={draft}
          onChange={(event) => onDraftChange(event.target.value)}
          placeholder={t(($) => $.workforce.history.placeholder)}
          aria-label={t(($) => $.workforce.history.search_label)}
          className="h-8 pl-8 pr-8"
        />
        {(draft || activeQuery) && (
          <button
            type="button"
            onClick={onClear}
            className="absolute right-1.5 top-1/2 flex size-6 -translate-y-1/2 items-center justify-center rounded text-muted-foreground hover:bg-accent hover:text-foreground"
            aria-label={t(($) => $.workforce.history.clear)}
          >
            <X className="size-3.5" />
          </button>
        )}
      </div>
      <Button type="submit" size="sm" disabled={!draft.trim() || draft.trim() === activeQuery} className="h-8">
        {t(($) => $.workforce.history.search)}
      </Button>
    </form>
  );
}

function historyMatchKey(match: WorkHistoryMatch): string {
  return match.id || match.sourceReference || `${match.provider}:${match.occurredAt}:${match.sourceOrdinal}`;
}

function providerLabel(provider: string): string {
  const normalized = provider.trim().toLowerCase();
  if (normalized === "chatgpt") return "ChatGPT";
  if (normalized === "codex") return "Codex";
  if (normalized === "claude") return "Claude";
  return provider ? provider[0]!.toUpperCase() + provider.slice(1) : "Outside AI";
}

function projectLabel(project: string): string {
  if (!project) return "Company";
  const known: Record<string, string> = {
    brain: "Brain",
    company: "Company",
    linkhub: "LinkHub",
    multica: "Multica",
    phonefarm: "PhoneFarm",
  };
  const normalized = project.trim().toLowerCase();
  if (known[normalized]) return known[normalized];
  return project
    .split(/[-_\s]+/)
    .filter(Boolean)
    .map((part) => part[0]!.toUpperCase() + part.slice(1))
    .join(" ");
}

function historyDate(value: string, includeTime = false): string {
  const dateOnly = /^\d{4}-\d{2}-\d{2}$/.test(value);
  const date = new Date(dateOnly ? `${value}T00:00:00Z` : value);
  if (Number.isNaN(date.getTime())) return value || "Unknown date";
  return new Intl.DateTimeFormat(
    undefined,
    includeTime
      ? { dateStyle: "medium", timeStyle: "short" }
      : { dateStyle: "medium", ...(dateOnly ? { timeZone: "UTC" } : {}) },
  ).format(date);
}

function HistorySource({ match }: { match: WorkHistoryMatch }) {
  const { t } = useT("inbox");
  const provider = providerLabel(match.provider);
  if (match.role.toLowerCase() === "user") {
    return <>{t(($) => $.workforce.history.from_you, { provider })}</>;
  }
  if (match.role.toLowerCase() === "assistant") {
    return <>{t(($) => $.workforce.history.from_ai, { provider })}</>;
  }
  return <>{t(($) => $.workforce.history.from_record, { provider })}</>;
}

function WorkHistoryResults({
  groups,
  loading,
  error,
  selectedKey,
  onSelect,
  onRetry,
  query,
}: {
  groups: readonly WorkHistoryGroup[];
  loading: boolean;
  error: boolean;
  selectedKey: string;
  onSelect: (key: string) => void;
  onRetry: () => void;
  query: string;
}) {
  const { t } = useT("inbox");
  if (loading) return <WorkforceLoading />;
  if (error) {
    return (
      <div className="flex min-h-0 flex-1 flex-col items-center justify-center gap-3 px-6 text-center">
        <p className="text-body font-medium">{t(($) => $.workforce.history.error_title)}</p>
        <p className="max-w-xs text-caption text-muted-foreground">{t(($) => $.workforce.history.error_body)}</p>
        <Button variant="outline" size="sm" onClick={onRetry}>{t(($) => $.workforce.history.retry)}</Button>
      </div>
    );
  }
  if (groups.every((group) => group.matches.length === 0)) {
    return (
      <div className="flex min-h-0 flex-1 flex-col items-center justify-center px-6 text-center text-muted-foreground">
        <Search className="mb-3 size-8 text-faint-foreground" />
        <p className="text-body text-foreground">{t(($) => $.workforce.history.no_results_title)}</p>
        <p className="mt-1 max-w-xs text-caption">{t(($) => $.workforce.history.no_results_body, { query })}</p>
      </div>
    );
  }
  return (
    <div className="min-h-0 flex-1 overflow-y-auto p-2">
      <p className="px-3.5 pb-2 pt-1 text-caption text-muted-foreground">
        {t(($) => $.workforce.history.unverified_short)}
      </p>
      {groups.map((group) => (
        <section key={`${group.day}:${group.project}`} className="mb-3">
          <h3 className="flex items-center justify-between gap-3 px-3.5 py-1.5 text-caption font-medium text-muted-foreground">
            <span>{projectLabel(group.project)}</span>
            <span>{historyDate(group.day)}</span>
          </h3>
          {group.matches.map((match) => {
            const key = historyMatchKey(match);
            return (
              <button
                key={key}
                type="button"
                onClick={() => onSelect(key)}
                aria-current={key === selectedKey}
                className={cn(
                  "flex w-full flex-col gap-1 rounded-md px-3.5 py-2.5 text-left outline-none transition-colors focus-visible:ring-1 focus-visible:ring-ring",
                  key === selectedKey ? "bg-accent" : "hover:bg-accent/50",
                )}
              >
                <div className="flex w-full items-center justify-between gap-3 text-caption text-muted-foreground">
                  <span className="font-medium text-foreground"><HistorySource match={match} /></span>
                  <span>{historyDate(match.occurredAt, true)}</span>
                </div>
                <p className="line-clamp-3 text-body leading-snug">{match.excerpt}</p>
              </button>
            );
          })}
        </section>
      ))}
    </div>
  );
}

function WorkHistoryEmpty({ hasResults }: { hasResults: boolean }) {
  const { t } = useT("inbox");
  return (
    <div className="flex h-full flex-col items-center justify-center px-6 text-center text-muted-foreground">
      <Search className="mb-3 size-10 text-faint-foreground" />
      <p className="text-body">{hasResults ? t(($) => $.workforce.history.choose_result) : t(($) => $.workforce.history.no_results_title)}</p>
    </div>
  );
}

function WorkHistoryDetail({ match }: { match: WorkHistoryMatch }) {
  const { t } = useT("inbox");
  return (
    <article className="mx-auto w-full max-w-3xl px-5 py-5 sm:px-8 sm:py-7">
      <div className="flex flex-wrap items-center gap-2 text-caption text-muted-foreground">
        <span className="size-2 rounded-full bg-slate-400" />
        <span className="font-medium text-foreground"><HistorySource match={match} /></span>
        <span aria-hidden="true">·</span>
        <span>{projectLabel(match.project)}</span>
        <span aria-hidden="true">·</span>
        <span>{historyDate(match.occurredAt, true)}</span>
      </div>
      <h2 className="mt-2 text-title font-semibold leading-tight">{t(($) => $.workforce.history.detail_title)}</h2>
      <div className="mt-4 rounded-xl border bg-surface-raised px-4 py-4 shadow-[var(--surface-shadow)] sm:px-5">
        <p className="whitespace-pre-wrap break-words text-body leading-relaxed">{match.excerpt}</p>
        {match.excerptTruncated && (
          <p className="mt-3 text-caption text-muted-foreground">{t(($) => $.workforce.history.excerpt_truncated)}</p>
        )}
      </div>
      <div className="mt-4 rounded-lg border border-dashed px-4 py-3">
        <p className="text-caption font-medium">{t(($) => $.workforce.history.unverified_title)}</p>
        <p className="mt-1 text-caption leading-relaxed text-muted-foreground">{t(($) => $.workforce.history.unverified_body)}</p>
      </div>
    </article>
  );
}

function WorkforceLoading() {
  return (
    <div className="min-h-0 flex-1 space-y-1 overflow-y-auto p-2">
      {Array.from({ length: 5 }).map((_, index) => (
        <div key={index} className="space-y-2 px-3.5 py-3">
          <Skeleton className="h-4 w-3/4" />
          <Skeleton className="h-3 w-1/2" />
        </div>
      ))}
    </div>
  );
}

const STATE_TONE: Record<WorkforceState, string> = {
  needs_you: "bg-amber-500",
  stopped: "bg-red-500",
  working: "bg-emerald-500",
  waiting: "bg-blue-500",
  reported_done: "bg-violet-500",
  no_update: "bg-muted-foreground/45",
};

function WorkforceList({ items, selectedId, onSelect }: { items: readonly WorkforceItem[]; selectedId: string; onSelect: (id: string) => void }) {
  const { t } = useT("inbox");
  if (items.length === 0) {
    return (
      <div className="flex min-h-0 flex-1 flex-col items-center justify-center px-6 text-center text-muted-foreground">
        <BriefcaseBusiness className="mb-3 size-8 text-faint-foreground" />
        <p className="text-body">{t(($) => $.workforce.no_open_work)}</p>
      </div>
    );
  }
  return (
    <div className="min-h-0 flex-1 overflow-y-auto p-2">
      {items.map((item) => (
        <button
          key={item.issue.id}
          type="button"
          onClick={() => onSelect(item.issue.id)}
          aria-current={item.issue.id === selectedId}
          className={cn(
            "flex w-full flex-col gap-1 rounded-md px-3.5 py-2.5 text-left outline-none transition-colors focus-visible:ring-1 focus-visible:ring-ring",
            item.issue.id === selectedId ? "bg-accent" : "hover:bg-accent/50",
          )}
        >
          <div className="flex w-full items-start justify-between gap-3">
            <span className="line-clamp-2 text-body font-medium leading-snug">{item.issue.title}</span>
            {(item.issue.priority === "urgent" || item.issue.priority === "high") && (
              <span className="shrink-0 rounded bg-muted px-1.5 py-0.5 text-[10px] font-semibold uppercase tracking-wide text-muted-foreground">
                {item.issue.priority}
              </span>
            )}
          </div>
          <div className="flex w-full items-center gap-2 text-caption text-muted-foreground">
            <span className={cn("size-1.5 shrink-0 rounded-full", STATE_TONE[item.state])} />
            <span>{item.label}</span>
            <span aria-hidden="true">·</span>
            <span className="truncate">{item.issue.identifier}</span>
          </div>
          <p className="line-clamp-1 text-caption text-muted-foreground">{item.latest}</p>
        </button>
      ))}
    </div>
  );
}

function WorkforceEmpty({ hasItems }: { hasItems: boolean }) {
  const { t } = useT("inbox");
  return (
    <div className="flex h-full flex-col items-center justify-center text-muted-foreground">
      <Inbox className="mb-3 size-10 text-faint-foreground" />
      <p className="text-body">{hasItems ? t(($) => $.workforce.choose_work) : t(($) => $.workforce.no_open_work)}</p>
    </div>
  );
}

function WorkforceDetail({ item }: { item: WorkforceItem }) {
  const { t } = useT("inbox");
  const paths = useWorkspacePaths();
  const timeAgo = useTimeAgo();
  const { data: linkedChats = [] } = useQuery(issueChatLinksOptions(item.issue.id));
  return (
    <div className="mx-auto w-full max-w-3xl px-5 py-5 sm:px-8 sm:py-7">
      <div className="flex flex-wrap items-center gap-2 text-caption text-muted-foreground">
        <span className={cn("size-2 rounded-full", STATE_TONE[item.state])} />
        <span className="font-medium text-foreground">{item.label}</span>
        <span aria-hidden="true">·</span>
        <span>{item.issue.identifier}</span>
        <span aria-hidden="true">·</span>
        <span>{timeAgo(item.updatedAt)}</span>
      </div>
      <h2 className="mt-2 text-title font-semibold leading-tight">{item.issue.title}</h2>

      <div className="mt-6 divide-y rounded-xl border bg-surface-raised px-4 shadow-[var(--surface-shadow)] sm:px-5">
        <PlainSection label="Latest progress" value={item.latest} />
        <PlainSection label="What happens next" value={item.next} />
        {item.context && <PlainSection label="Also worth knowing" value={item.context} />}
        {item.source && <PlainSection label="Where this update came from" value={item.source} />}
      </div>

      <div className="mt-4 flex flex-wrap items-center gap-2">
        <TalkToMikaButton
          issueId={item.issue.id}
          issueIdentifier={item.issue.identifier}
          title={item.issue.title}
          question={item.next}
          linkedChats={linkedChats}
          promptKind={item.state === "needs_you" ? "decision" : "status"}
        />
        <Button
          variant="ghost"
          size="sm"
          nativeButton={false}
          render={<a href={paths.issueDetail(item.issue.id)} />}
        >
          {t(($) => $.owner_detail.full_history)}
        </Button>
      </div>
    </div>
  );
}

function PlainSection({ label, value }: { label: string; value: string }) {
  return (
    <section className="py-4">
      <h3 className="text-caption font-medium text-muted-foreground">{label}</h3>
      <p className="mt-1 text-body leading-relaxed">{value}</p>
    </section>
  );
}
