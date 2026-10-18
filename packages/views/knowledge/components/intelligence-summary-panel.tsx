"use client";

import { useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { ChevronDown, CircleAlert, CircleCheck, DatabaseZap } from "lucide-react";
import { Skeleton } from "@multica/ui/components/ui/skeleton";
import { Badge } from "@multica/ui/components/ui/badge";
import { Card, CardHeader } from "@multica/ui/components/ui/card";
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from "@multica/ui/components/ui/collapsible";
import {
  Table,
  TableHeader,
  TableBody,
  TableRow,
  TableHead,
  TableCell,
} from "@multica/ui/components/ui/table";
import {
  intelligenceSummaryOptions,
  intelligenceBusinessSummaryOptions,
} from "@multica/core/knowledge/queries";
import type {
  IntelligenceProducerEvent,
  IntelligenceStatusBreakdown,
  IntelligenceCollectorHealth,
} from "@multica/core/types/api";
import { useT } from "../../i18n";

function formatRelative(iso: string): string {
  if (!iso) return "";
  const then = new Date(iso).getTime();
  if (Number.isNaN(then)) return iso;
  const diffMs = Date.now() - then;
  const minutes = Math.round(diffMs / 60_000);
  if (minutes < 1) return "just now";
  if (minutes < 60) return `${minutes}m ago`;
  const hours = Math.round(minutes / 60);
  if (hours < 24) return `${hours}h ago`;
  const days = Math.round(hours / 24);
  return `${days}d ago`;
}

function breakdownSubtext(breakdown: IntelligenceStatusBreakdown[], primaryStatus: string): string {
  const rest = breakdown.filter((b) => b.status !== primaryStatus && b.count > 0);
  if (rest.length === 0) return "";
  return rest.map((b) => `${b.count.toLocaleString()} ${b.status}`).join(" · ");
}

function statusCount(breakdown: IntelligenceStatusBreakdown[], status: string): number {
  return breakdown.find((b) => b.status === status)?.count ?? 0;
}

interface ProducerGroup {
  producer: string;
  events: IntelligenceProducerEvent[];
  total: number;
  latest: string;
}

function groupByProducer(events: IntelligenceProducerEvent[]): ProducerGroup[] {
  const byProducer = new Map<string, IntelligenceProducerEvent[]>();
  for (const e of events) {
    const list = byProducer.get(e.producer);
    if (list) list.push(e);
    else byProducer.set(e.producer, [e]);
  }
  return [...byProducer.entries()]
    .map(([producer, list]) => ({
      producer,
      events: [...list].sort((a, b) => b.count - a.count),
      total: list.reduce((sum, e) => sum + e.count, 0),
      latest: list.reduce((latest, e) => (e.latest > latest ? e.latest : latest), ""),
    }))
    .sort((a, b) => b.total - a.total);
}

function CollectorHealthRow({ health }: { health: IntelligenceCollectorHealth }) {
  const { t } = useT("knowledge");
  const healthy = health.rejected === 0 && health.quarantined === 0 && !health.has_errors;
  return (
    <div className="flex items-center justify-between gap-3 py-2">
      <div className="flex items-center gap-2">
        {healthy ? (
          <CircleCheck className="size-4 shrink-0 text-emerald-500" />
        ) : (
          <CircleAlert className="size-4 shrink-0 text-amber-500" />
        )}
        <Badge variant="secondary" className="text-micro">
          {health.producer}
        </Badge>
        <span className="text-caption text-muted-foreground">{health.collector}</span>
      </div>
      <div className="flex items-center gap-3 text-caption text-muted-foreground">
        {!healthy && (
          <span className="text-amber-600 dark:text-amber-400">
            {health.rejected > 0 && `${health.rejected} rejected`}
            {health.rejected > 0 && health.quarantined > 0 && " · "}
            {health.quarantined > 0 && `${health.quarantined} quarantined`}
          </span>
        )}
        <span className="tabular-nums">{health.accepted.toLocaleString()}{" "}{t(($) => $.operational_copy.accepted)}</span>
        <span>{formatRelative(health.observed_at)}</span>
      </div>
    </div>
  );
}

/**
 * Data & Metrics tab. Two layers, not one raw ledger dump:
 *  - Business summary (default, primary): owner-relevant derived figures —
 *    current total followers, active client/account counts, recent
 *    subscriber growth, whether each producer's last sync actually
 *    succeeded. Computed server-side from the latest observation per
 *    entity, not a count of how many rows the ledger has recorded — a raw
 *    event count (e.g. "47,733 follower.snapshot rows") answers "how much
 *    data has been ingested," not "how many followers do we have," and the
 *    owner correctly called that out as meaningless to him.
 *  - Raw event ledger (collapsed by default): the (producer, event_type)
 *    count/freshness breakdown this tab used to show as the only view. Kept
 *    because Intelligence's ledger is the real source of truth an agent may
 *    need to inspect — just not the first thing a human sees.
 */
export function IntelligenceSummaryPanel() {
  const { t } = useT("knowledge");
  const { data: business, isLoading: businessLoading } = useQuery(intelligenceBusinessSummaryOptions());
  const { data, isLoading } = useQuery(intelligenceSummaryOptions());
  const [ledgerOpen, setLedgerOpen] = useState(false);
  const groups = useMemo(() => groupByProducer(data?.events ?? []), [data]);

  if (businessLoading || isLoading) {
    return (
      <div className="mx-auto max-w-5xl space-y-3 px-6 py-5 sm:px-8">
        {Array.from({ length: 4 }).map((_, i) => (
          <Skeleton key={i} className="h-28 w-full rounded-xl" />
        ))}
      </div>
    );
  }

  if (!business?.configured) {
    return (
      <div className="flex h-full flex-col items-center justify-center gap-2 text-muted-foreground">
        <DatabaseZap className="size-6 text-faint-foreground" />
        <p className="text-body">{t(($) => $.intelligence.not_configured)}</p>
      </div>
    );
  }

  if (!business.available) {
    return (
      <div className="flex h-full flex-col items-center justify-center gap-2 text-muted-foreground">
        <DatabaseZap className="size-6 text-faint-foreground" />
        <p className="text-body">{t(($) => $.intelligence.unavailable)}</p>
      </div>
    );
  }

  const activeClients = statusCount(business.clients_by_status, "active");
  const activeAccounts = statusCount(business.social_accounts_by_status, "active");

  return (
    <div className="h-full overflow-y-auto px-6 py-5 sm:px-8">
      <div className="mx-auto max-w-5xl space-y-6">
        <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-4">
          <Card className="bg-surface-raised">
            <CardHeader className="gap-1">
              <p className="text-caption text-muted-foreground">{t(($) => $.intelligence.total_followers)}</p>
              <p className="text-title-lg font-semibold tabular-nums">
                {business.total_followers.toLocaleString()}
              </p>
              <p className="text-micro text-muted-foreground">
                {t(($) => $.intelligence.across_accounts).replace(
                  "{count}",
                  business.tracked_social_accounts.toLocaleString(),
                )}
              </p>
            </CardHeader>
          </Card>
          <Card className="bg-surface-raised">
            <CardHeader className="gap-1">
              <p className="text-caption text-muted-foreground">{t(($) => $.intelligence.active_clients)}</p>
              <p className="text-title-lg font-semibold tabular-nums">{activeClients.toLocaleString()}</p>
              <p className="text-micro text-muted-foreground">
                {breakdownSubtext(business.clients_by_status, "active") || " "}
              </p>
            </CardHeader>
          </Card>
          <Card className="bg-surface-raised">
            <CardHeader className="gap-1">
              <p className="text-caption text-muted-foreground">{t(($) => $.intelligence.active_accounts)}</p>
              <p className="text-title-lg font-semibold tabular-nums">{activeAccounts.toLocaleString()}</p>
              <p className="text-micro text-muted-foreground">
                {breakdownSubtext(business.social_accounts_by_status, "active") || " "}
              </p>
            </CardHeader>
          </Card>
          <Card className="bg-surface-raised">
            <CardHeader className="gap-1">
              <p className="text-caption text-muted-foreground">{t(($) => $.intelligence.new_subscribers_7d)}</p>
              <p className="text-title-lg font-semibold tabular-nums">
                {business.new_subscribers_7d.toLocaleString()}
              </p>
              <p className="text-micro text-muted-foreground">&nbsp;</p>
            </CardHeader>
          </Card>
        </div>

        {business.collection_health.length > 0 && (
          <Card className="bg-surface-raised">
            <CardHeader className="gap-1">
              <p className="text-caption font-medium">{t(($) => $.intelligence.sync_health)}</p>
            </CardHeader>
            <div className="divide-y px-4">
              {business.collection_health.map((h) => (
                <CollectorHealthRow key={h.producer} health={h} />
              ))}
            </div>
          </Card>
        )}

        {data?.configured && data.available && groups.length > 0 && (
          <Collapsible open={ledgerOpen} onOpenChange={setLedgerOpen}>
            <CollapsibleTrigger className="flex w-full items-center gap-1.5 text-caption text-muted-foreground hover:text-foreground">
              <ChevronDown
                className={`size-3.5 transition-transform ${ledgerOpen ? "" : "-rotate-90"}`}
              />
              {t(($) => $.intelligence.raw_ledger)} ({data.total_count.toLocaleString()}{" "}
              {t(($) => $.intelligence.total_events).toLowerCase()})
            </CollapsibleTrigger>
            <CollapsibleContent>
              <div className="mt-3 space-y-3">
                {groups.map((g) => (
                  <Card key={g.producer} className="bg-surface-raised">
                    <div className="flex items-center justify-between gap-2 px-4">
                      <div className="flex items-center gap-2">
                        <Badge variant="secondary" className="text-micro">
                          {g.producer}
                        </Badge>
                        <span className="text-caption text-muted-foreground">
                          {g.events.length} {t(($) => $.intelligence.event_type_count)}
                        </span>
                      </div>
                      <span className="text-micro text-muted-foreground">
                        {t(($) => $.intelligence.table_latest)}: {formatRelative(g.latest)}
                      </span>
                    </div>
                    <div className="px-2">
                      <Table>
                        <TableHeader>
                          <TableRow>
                            <TableHead>{t(($) => $.intelligence.table_event_type)}</TableHead>
                            <TableHead className="text-right">{t(($) => $.intelligence.table_count)}</TableHead>
                            <TableHead className="text-right">{t(($) => $.intelligence.table_latest)}</TableHead>
                            <TableHead className="text-right">{t(($) => $.intelligence.table_earliest)}</TableHead>
                          </TableRow>
                        </TableHeader>
                        <TableBody>
                          {g.events.map((e) => (
                            <TableRow key={e.event_type}>
                              <TableCell className="font-mono text-caption">{e.event_type}</TableCell>
                              <TableCell className="text-right tabular-nums">
                                {e.count.toLocaleString()}
                              </TableCell>
                              <TableCell className="text-right text-caption text-muted-foreground">
                                {formatRelative(e.latest)}
                              </TableCell>
                              <TableCell className="text-right text-caption text-muted-foreground">
                                {formatRelative(e.earliest)}
                              </TableCell>
                            </TableRow>
                          ))}
                        </TableBody>
                      </Table>
                    </div>
                  </Card>
                ))}
              </div>
            </CollapsibleContent>
          </Collapsible>
        )}
      </div>
    </div>
  );
}
