"use client";
import { useT } from "../../i18n";

import { CheckCircle2, ArrowRightLeft } from "lucide-react";
import { Badge } from "@multica/ui/components/ui/badge";

interface SubscriptionProvider {
  id: string;
  name: string;
  role: "primary" | "standby";
  model: string;
  status: "healthy" | "standby" | "rate_limited";
  quotaUsedPercent: number;
  rateLimitStatus: string;
  resetInfo: string;
}

const PROVIDERS: SubscriptionProvider[] = [
  {
    id: "codex-1",
    name: "OpenAI Codex #1 (Primary Pro)",
    role: "primary",
    model: "codex-5.3 / gpt-5.3-codex",
    status: "healthy",
    quotaUsedPercent: 68,
    rateLimitStatus: "32% Headroom",
    resetInfo: "Rolling window",
  },
  {
    id: "codex-2",
    name: "OpenAI Codex #2 (Secondary)",
    role: "standby",
    model: "codex-5.3 / gpt-5.3-codex",
    status: "standby",
    quotaUsedPercent: 24,
    rateLimitStatus: "76% Headroom",
    resetInfo: "Active fallback queue",
  },
  {
    id: "codex-3",
    name: "OpenAI Codex #3 (Overflow)",
    role: "standby",
    model: "codex-5.3 / gpt-5.3-codex",
    status: "standby",
    quotaUsedPercent: 8,
    rateLimitStatus: "92% Headroom",
    resetInfo: "Batch workflows",
  },
  {
    id: "claude-max",
    name: "Anthropic Claude Max",
    role: "primary",
    model: "claude-3-7-sonnet / opus-4.8",
    status: "healthy",
    quotaUsedPercent: 82,
    rateLimitStatus: "18% Headroom (Throttling near)",
    resetInfo: "5-hour rolling reset",
  },
  {
    id: "copilot-max",
    name: "GitHub Copilot Max",
    role: "primary",
    model: "GPT-4o / Claude 3.5 Sonnet",
    status: "healthy",
    quotaUsedPercent: 41,
    rateLimitStatus: "59% Headroom",
    resetInfo: "Monthly allowance",
  },
];

export function SubscriptionsTab() {
  const { t: tc } = useT("common");
  return (
    <div className="space-y-4">
      {/* Compact table view for subscriptions and quota */}
      <div className="overflow-hidden rounded-lg border bg-card shadow-xs">
        <div className="border-b bg-muted/40 px-4 py-3">
          <div className="flex items-center justify-between">
            <div>
              <h3 className="text-body font-semibold text-foreground">{tc(($) => $.operational_copy.active_subscriptions_title)}</h3>
              <p className="text-caption text-muted-foreground">{tc(($) => $.operational_copy.capacity_tracking_description)}</p>
            </div>
            <div className="flex items-center gap-2">
              <span className="inline-flex items-center gap-1.5 rounded-full bg-emerald-500/10 px-2.5 py-0.5 text-caption font-medium text-emerald-600 dark:text-emerald-400">
                <span className="size-1.5 rounded-full bg-emerald-500 animate-pulse" />{tc(($) => $.operational_copy.all_5_accounts_connected)}</span>
            </div>
          </div>
        </div>

        <div className="divide-y divide-border/60">
          {PROVIDERS.map((provider) => {
            const isPrimary = provider.role === "primary";
            const isHighUsage = provider.quotaUsedPercent > 80;
            const isModerate = provider.quotaUsedPercent > 50;

            return (
              <div
                key={provider.id}
                className="flex flex-col sm:flex-row sm:items-center justify-between gap-3 px-4 py-3 hover:bg-muted/20 transition-colors"
              >
                {/* Account info */}
                <div className="min-w-[240px] space-y-0.5">
                  <div className="flex items-center gap-2">
                    <span className="text-body font-medium text-foreground">{provider.name}</span>
                    <Badge
                      variant="secondary"
                      className={`text-micro capitalize py-0 px-1.5 h-4.5 ${
                        isPrimary
                          ? "bg-primary/10 text-primary border-primary/20"
                          : "bg-muted text-muted-foreground"
                      }`}
                    >
                      {provider.role}
                    </Badge>
                  </div>
                  <p className="text-caption font-mono text-muted-foreground text-xs">{provider.model}</p>
                </div>

                {/* Progress bar and % */}
                <div className="flex-1 max-w-xs space-y-1">
                  <div className="flex items-center justify-between text-caption">
                    <span className="text-xs font-semibold text-foreground">{provider.quotaUsedPercent}{tc(($) => $.operational_copy.used)}</span>
                    <span className="text-xs text-muted-foreground">{provider.rateLimitStatus}</span>
                  </div>
                  <div className="h-1.5 w-full overflow-hidden rounded-full bg-muted">
                    <div
                      className={`h-full transition-all duration-300 ${
                        isHighUsage
                          ? "bg-destructive"
                          : isModerate
                            ? "bg-amber-500"
                            : "bg-primary"
                      }`}
                      style={{ width: `${provider.quotaUsedPercent}%` }}
                    />
                  </div>
                </div>

                {/* Status and reset timing */}
                <div className="flex items-center justify-between sm:justify-end gap-4 min-w-[180px] text-right">
                  <span className="text-caption text-muted-foreground text-xs">{provider.resetInfo}</span>
                  {provider.status === "healthy" ? (
                    <span className="inline-flex items-center gap-1 text-xs font-medium text-emerald-500">
                      <CheckCircle2 className="size-3.5" />{tc(($) => $.operational_copy.active)}</span>
                  ) : (
                    <span className="inline-flex items-center gap-1 text-xs font-medium text-muted-foreground">
                      <ArrowRightLeft className="size-3.5" />{tc(($) => $.operational_copy.standby)}</span>
                  )}
                </div>
              </div>
            );
          })}
        </div>
      </div>
    </div>
  );
}
