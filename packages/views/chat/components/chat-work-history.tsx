"use client";

import { useInfiniteQuery } from "@tanstack/react-query";
import { chatWorkHistoryOptions } from "@multica/core/chat/queries";
import type { ChatWorkIssue } from "@multica/core/chat/work";
import { Button } from "@multica/ui/components/ui/button";
import { useT } from "../../i18n";

export function ChatWorkHistory({ wsId, sessionId, issues, onSelect }: { wsId: string; sessionId: string; issues: ChatWorkIssue[]; onSelect: (id: string) => void }) {
  const { t } = useT("chat");
  const history = useInfiniteQuery(chatWorkHistoryOptions(wsId, sessionId));
  const linked = new Map(issues.map(issue => [issue.id, issue.title]));
  const seen = new Set<string>();
  const events = (history.data?.pages.flatMap(page => page.activity) ?? []).filter(event => {
    if (!linked.has(event.issue_id) || seen.has(event.id)) return false;
    seen.add(event.id);
    return true;
  });
  return <div className="space-y-2">
    {history.isPending && <p role="status" className="text-caption text-muted-foreground">{t(($) => $.work.history_loading)}</p>}
    {history.isError && <div role="alert" className="text-caption text-muted-foreground">
      {t(($) => $.work.history_failed)}
      <Button variant="ghost" size="sm" disabled={history.isFetching} onClick={() => void (history.isFetchNextPageError ? history.fetchNextPage() : history.refetch())}>{t(($) => $.library.retry)}</Button>
    </div>}
    {events.map(event => <button type="button" key={event.id} className="block w-full py-2 text-left" onClick={() => onSelect(event.issue_id)}>
      <span className="block truncate text-caption font-medium">{linked.get(event.issue_id)}</span>
      <span className="block whitespace-pre-wrap break-words text-caption text-muted-foreground">{event.reason || t(($) => $.catch_up.changed)}</span>
      <time dateTime={event.created_at} className="text-caption text-muted-foreground">{new Date(event.created_at).toLocaleString()}</time>
    </button>)}
    {!history.isPending && !history.isError && events.length === 0 && <p className="text-caption text-muted-foreground">{t(($) => $.work.history_empty)}</p>}
    {history.hasNextPage && !history.isError && <Button variant="ghost" size="sm" disabled={history.isFetching} onClick={() => void history.fetchNextPage()}>{t(($) => history.isFetchingNextPage ? $.work.history_loading : $.work.more)}</Button>}
  </div>;
}
