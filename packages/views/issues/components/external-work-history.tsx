"use client";

import { externalWorkSummaryFromPicture } from "@multica/core/issues";
import type { ExternalWorkUpdate } from "@multica/core/types";
import { useT, useTimeAgo } from "../../i18n";

const providers: Record<ExternalWorkUpdate["provider"], string> = {
  codex: "Codex",
  claude: "Claude",
  chatgpt: "ChatGPT",
  copilot: "GitHub Copilot",
  evo: "EVO",
  other: "Outside agent",
};

export function externalWorkProviderName(provider: ExternalWorkUpdate["provider"]): string {
  return providers[provider] ?? providers.other;
}

export function formatExternalWorkDuration(seconds: number): string {
  if (seconds < 60) return `${seconds}s`;
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return `${minutes}m`;
  const hours = Math.floor(minutes / 60);
  const remainder = minutes % 60;
  return remainder === 0 ? `${hours}h` : `${hours}h ${remainder}m`;
}

export { externalWorkSummaryFromPicture };

function ExternalWorkUpdateCard({ update }: { update: ExternalWorkUpdate }) {
  const { t } = useT("issues");
  const timeAgo = useTimeAgo();
  const hiddenEvidence = Math.max(0, update.evidence_total - update.evidence.length);
  const hiddenBlockers = Math.max(0, update.blocker_total - update.blockers.length);
  const usage = update.resource_usage;
  const usageParts: string[] = [];
  if (usage?.attempts !== null && usage?.attempts !== undefined) {
    usageParts.push(t(($) => $.workflow.external.usage_attempts, { count: usage.attempts }));
  }
  if (usage?.active_compute_seconds !== null && usage?.active_compute_seconds !== undefined) {
    usageParts.push(t(($) => $.workflow.external.usage_active, { duration: formatExternalWorkDuration(usage.active_compute_seconds) }));
  } else if (usage?.wall_clock_seconds !== null && usage?.wall_clock_seconds !== undefined) {
    usageParts.push(t(($) => $.workflow.external.usage_elapsed, { duration: formatExternalWorkDuration(usage.wall_clock_seconds) }));
  }
  if (usage?.input_tokens !== null && usage?.input_tokens !== undefined && usage.output_tokens !== null) {
    usageParts.push(t(($) => $.workflow.external.usage_tokens, { tokens: new Intl.NumberFormat().format(usage.input_tokens + usage.output_tokens) }));
  }
  if (usage?.cost_usd !== null && usage?.cost_usd !== undefined) {
    usageParts.push(t(($) => $.workflow.external.usage_cost, {
      cost: new Intl.NumberFormat(undefined, { style: "currency", currency: "USD", maximumFractionDigits: 6 }).format(usage.cost_usd),
    }));
  }
  return (
    <article className="rounded-lg border bg-background px-3 py-3" data-external-work-update="">
      <div className="flex flex-wrap items-center justify-between gap-x-3 gap-y-1">
        <p className="text-body font-medium">
          {t(($) => $.workflow.external.reported_by, { provider: externalWorkProviderName(update.provider) })}
        </p>
        <div className="flex items-center gap-2 text-caption text-muted-foreground">
          <span className="rounded-full bg-muted px-2 py-0.5 text-foreground">
            {t(($) => $.workflow.external.state[update.reported_state])}
          </span>
          <time dateTime={update.observed_at}>{timeAgo(update.observed_at)}</time>
        </div>
      </div>
      <p className="mt-2 whitespace-pre-wrap break-words text-body text-foreground">{update.summary}</p>
      {update.blockers.length > 0 && (
        <p className="mt-2 break-words text-caption text-destructive">
          {t(($) => $.workflow.external.blocked_by, { blocker: update.blockers[0] })}
        </p>
      )}
      <details className="mt-2 text-caption text-muted-foreground">
        <summary className="w-fit cursor-pointer rounded-sm font-medium text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring">
          {t(($) => $.workflow.external.details)}
        </summary>
        <dl className="mt-2 grid gap-2 sm:grid-cols-2">
          <div>
            <dt className="font-medium text-foreground">{t(($) => $.workflow.external.milestone)}</dt>
            <dd className="mt-0.5 whitespace-pre-wrap break-words">{update.current_milestone}</dd>
          </div>
          <div>
            <dt className="font-medium text-foreground">{t(($) => $.workflow.external.next)}</dt>
            <dd className="mt-0.5 whitespace-pre-wrap break-words">{update.next_action}</dd>
          </div>
          <div>
            <dt className="font-medium text-foreground">{t(($) => $.workflow.external.usage)}</dt>
            <dd className="mt-0.5 break-words">
              {!usage || usage.source === "unavailable"
                ? t(($) => $.workflow.external.usage_unavailable)
                : `${usage.source === "measured" ? t(($) => $.workflow.external.usage_measured) : t(($) => $.workflow.external.usage_reported)}${usageParts.length > 0 ? `: ${usageParts.join(" · ")}` : ""}`}
            </dd>
          </div>
          {update.blockers.length > 0 && (
            <div>
              <dt className="font-medium text-foreground">{t(($) => $.workflow.external.blockers)}</dt>
              <dd className="mt-0.5">
                <ul className="list-disc space-y-1 pl-4">
                  {update.blockers.map((blocker, index) => <li className="break-words" key={`${update.id}-blocker-${index}`}>{blocker}</li>)}
                  {hiddenBlockers > 0 && <li>{t(($) => $.workflow.external.more_items, { count: hiddenBlockers })}</li>}
                </ul>
              </dd>
            </div>
          )}
          <div>
            <dt className="font-medium text-foreground">{t(($) => $.workflow.external.evidence)}</dt>
            <dd className="mt-0.5">
              <ul className="list-disc space-y-1 pl-4">
                {update.evidence.map((item, index) => (
                  <li className="break-all" key={`${update.id}-evidence-${index}`}>
                    <span className="font-medium">{item.type}:</span> {item.reference}
                  </li>
                ))}
                {hiddenEvidence > 0 && <li>{t(($) => $.workflow.external.more_items, { count: hiddenEvidence })}</li>}
              </ul>
            </dd>
          </div>
        </dl>
      </details>
    </article>
  );
}

export function ExternalWorkHistory({
  updates,
  loading = false,
  failed = false,
}: {
  updates: ExternalWorkUpdate[];
  loading?: boolean;
  failed?: boolean;
}) {
  const { t } = useT("issues");
  if (loading) return null;
  if (failed) {
    return <p className="mt-4 text-caption text-muted-foreground">{t(($) => $.workflow.external.unavailable)}</p>;
  }
  if (updates.length === 0) return null;
  const recent = updates.slice(0, 3);
  const older = updates.slice(3);
  return (
    <section className="mt-5 border-t pt-4" aria-labelledby="external-work-history-title">
      <details>
        <summary id="external-work-history-title" aria-label={t(($) => $.workflow.external.title)} className="w-fit cursor-pointer rounded-sm text-body font-semibold text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring">
          {t(($) => $.workflow.external.title)} <span className="ml-1 text-caption font-medium text-muted-foreground">{updates.length}</span>
        </summary>
        <p className="mt-1 text-caption text-muted-foreground">{t(($) => $.workflow.external.explanation)}</p>
        <div className="mt-3 space-y-2">
          {recent.map((update) => <ExternalWorkUpdateCard key={update.id} update={update} />)}
        </div>
        {older.length > 0 && (
          <details className="mt-2">
            <summary className="w-fit cursor-pointer rounded-sm text-caption font-medium text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring">
              {t(($) => $.workflow.external.older, { count: older.length })}
            </summary>
            <div className="mt-2 space-y-2">
              {older.map((update) => <ExternalWorkUpdateCard key={update.id} update={update} />)}
            </div>
          </details>
        )}
      </details>
    </section>
  );
}
