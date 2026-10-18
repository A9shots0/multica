import { View } from "react-native";
import type { Issue } from "@multica/core/types";
import { Text } from "@/components/ui/text";

const stageLabels: Record<string, string> = {
  active_work: "Active work",
  waiting_automatic: "Waiting automatically",
  needs_owner: "Needs you",
  independent_review: "In independent review",
  releasing: "Releasing",
  deployed_checking: "Deployed, not verified yet",
  verified_live: "Verified Live",
  current_regression: "Regression detected",
  rollback_in_progress: "Rolling back",
  service_restored_outcome_open: "Service restored; outcome still open",
  outcome_done: "Done",
  outcome_complete: "Done",
  technical_stop: "Technical stop",
};

function label(value: string | undefined | null): string {
  if (!value) return "Not recorded";
  return stageLabels[value] ?? value.replaceAll("_", " ");
}

export function CurrentPictureCard({ issue }: { issue: Issue }) {
  const summary = issue.current_picture?.owner_summary;
  if (!summary) {
    return <View className="rounded-xl border border-border bg-secondary/30 p-4 gap-2">
      <Text className="text-sm font-semibold text-foreground">Now</Text>
      <Text className="text-sm text-muted-foreground">Status unavailable. This issue has no validated Current Picture yet.</Text>
    </View>;
  }
  return <View className="rounded-xl border border-border bg-secondary/30 p-4 gap-3">
    <View className="flex-row items-center justify-between gap-3">
      <Text className="text-sm font-semibold text-foreground">Now</Text>
      <View className="rounded-full bg-background px-2.5 py-1"><Text className="text-xs font-semibold text-foreground">{label(summary.stage)}</Text></View>
    </View>
    <View><Text className="text-xs uppercase text-muted-foreground">Latest meaningful progress</Text>
      <Text className="mt-1 text-sm text-foreground">{summary.latest_meaningful_change ? `${label(summary.latest_meaningful_change.code)} · ${label(summary.latest_meaningful_change.result_code)}` : "No verified progress yet"}</Text></View>
    <View><Text className="text-xs uppercase text-muted-foreground">Next step</Text>
      <Text className="mt-1 text-sm text-foreground">{summary.next_action ? `${label(summary.next_action.kind)}${summary.next_action.automatic ? " automatically" : ""}` : "No next step is recorded"}</Text></View>
  </View>;
}
