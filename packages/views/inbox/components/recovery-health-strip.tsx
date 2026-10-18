"use client";
import { useT } from "../../i18n";

import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { AlertTriangle, DatabaseBackup, ShieldCheck } from "lucide-react";
import { api } from "@multica/core/api";
import { cn } from "@multica/ui/lib/utils";

/**
 * Said in terms of the thing at risk — the owner's data — rather than in terms
 * of the mechanism. "The latest bounded backup did not complete" and "inspect
 * the latest recovery receipt" name internals the reader cannot act on; the
 * only two facts that matter to them are whether their work is safe and
 * whether a person has to do something about it.
 */
const presentation = {
  protected: {
    label: "Your data is safe",
    detail: "There is a recent copy, and it has been proved to restore.",
    next: "",
    tone: "border-emerald-500/30 bg-emerald-500/5 text-emerald-800 dark:text-emerald-200",
    icon: ShieldCheck,
  },
  backup_due: {
    label: "No recent copy of your data",
    detail: "The last backup is older than it should be.",
    next: "A new one is due. Nothing for you to do unless this is still here tomorrow.",
    tone: "border-amber-500/30 bg-amber-500/5 text-amber-800 dark:text-amber-200",
    icon: DatabaseBackup,
  },
  recovery_unproven: {
    label: "The backup has not been tested",
    detail: "A copy of your data exists, but nothing has confirmed it can be restored from.",
    next: "A restore test runs by itself. Nothing for you to do yet.",
    tone: "border-amber-500/30 bg-amber-500/5 text-amber-800 dark:text-amber-200",
    icon: DatabaseBackup,
  },
  secondary_copy_needed: {
    label: "The only copy is on this machine",
    detail: "Your data restores fine, but if this machine is lost the backup goes with it.",
    next: "A second copy somewhere else needs setting up.",
    tone: "border-amber-500/30 bg-amber-500/5 text-amber-800 dark:text-amber-200",
    icon: DatabaseBackup,
  },
  backup_failed: {
    label: "Your data did not get backed up",
    detail: "The last attempt to copy it did not finish.",
    next: "It retries by itself. If this is still here tomorrow, somebody needs to look at it.",
    tone: "border-red-500/30 bg-red-500/5 text-red-800 dark:text-red-200",
    icon: AlertTriangle,
  },
  platform_unavailable: {
    label: "Multica is failing its own health checks",
    detail: "The watchdog watching this system has stopped getting an answer.",
    next: "This one does need a person.",
    tone: "border-red-500/30 bg-red-500/5 text-red-800 dark:text-red-200",
    icon: AlertTriangle,
  },
} as const;

export function RecoveryHealthStrip({ workspaceId }: { workspaceId: string }) {
  const { t: tc } = useT("inbox");
  const [dismissed, setDismissed] = useState(false);
  const { data, isError } = useQuery({
    queryKey: ["recovery-status", workspaceId],
    queryFn: () => api.getRecoveryStatus(workspaceId),
    staleTime: 30_000,
    refetchInterval: 60_000,
  });

  if (dismissed) return null;
  if (!data && !isError) return null;
  if (isError) return null;
  if (!data) return null;
  // If recovery is already protected and healthy, don't clutter the inbox with a banner
  if (data.status === "protected") return null;

  const view = presentation[data.status] ?? presentation.recovery_unproven;
  const Icon = view.icon;
  const age = typeof data.backup_age_hours === "number"
    ? data.backup_age_hours < 1
      ? "less than 1h old"
      : `${Math.floor(data.backup_age_hours)}h old`
    : null;

  return (
    <div className="border-b p-3" data-testid="recovery-health-strip">
      <div className={cn("relative rounded-lg border px-3 py-2.5", view.tone)}>
        <div className="flex items-start gap-2.5">
          <Icon className="mt-0.5 size-4 shrink-0" aria-hidden="true" />
          <div className="min-w-0 flex-1 pr-6">
            <div className="flex flex-wrap items-baseline justify-between gap-x-3 gap-y-0.5">
              <p className="text-xs font-semibold">{view.label}</p>
              {age && <span className="text-[11px] opacity-75">{age}</span>}
            </div>
            <p className="mt-0.5 text-[11px] leading-4 opacity-80">{view.detail}</p>
            {view.next && (
              <p className="mt-1 text-[11px] leading-4 text-current">{view.next}</p>
            )}
          </div>
          <button
            type="button"
            onClick={() => setDismissed(true)}
            aria-label={tc(($) => $.operational_copy.dismiss_banner)}
            className="absolute right-2 top-2 rounded p-1 opacity-70 hover:opacity-100 hover:bg-black/5 dark:hover:bg-white/10"
          >
            <span className="text-xs font-bold leading-none">{"✕"}</span>
          </button>
        </div>
      </div>
    </div>
  );
}
