import type { ExternalWorkUpdate, IssueCurrentPictureCompact } from "../types";

export interface ExternalWorkCardSummary {
  provider: ExternalWorkUpdate["provider"];
  reportedState: ExternalWorkUpdate["reported_state"];
  summary: string;
  nextAction: string;
}

/**
 * Reads only the bounded provider report embedded in Current Picture. A report
 * describes what an outside tool said; it never upgrades the native issue
 * outcome or release state.
 */
export function externalWorkSummaryFromPicture(
  picture: IssueCurrentPictureCompact | undefined,
): ExternalWorkCardSummary | null {
  const raw = picture?.latest_meaningful_change?.external_work;
  if (!raw || typeof raw !== "object" || Array.isArray(raw)) return null;
  const record = raw as Record<string, unknown>;
  const provider = record.provider;
  const reportedState = record.reported_state;
  const summary = record.change_since_last_receipt;
  const nextAction = record.next_action;
  if (
    !(
      provider === "codex" ||
      provider === "claude" ||
      provider === "chatgpt" ||
      provider === "copilot" ||
      provider === "evo" ||
      provider === "other"
    ) ||
    !(
      reportedState === "working" ||
      reportedState === "waiting_external" ||
      reportedState === "waiting_owner" ||
      reportedState === "blocked" ||
      reportedState === "complete" ||
      reportedState === "stopped"
    ) ||
    typeof summary !== "string" ||
    !summary.trim() ||
    typeof nextAction !== "string" ||
    !nextAction.trim()
  ) {
    return null;
  }
  return {
    provider,
    reportedState,
    summary: summary.trim(),
    nextAction: nextAction.trim(),
  };
}
