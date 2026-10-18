import { useState } from "react";
import { ActivityIndicator, Alert, KeyboardAvoidingView, Linking, Platform, ScrollView, TextInput, View } from "react-native";
import { router, useLocalSearchParams } from "expo-router";
import { useQuery } from "@tanstack/react-query";
import {
  resolveBillingRecovery,
  type BillingRecoveryKind,
} from "@multica/core/billing/recovery";
import { BILLING_WORKSPACE_SUBSCRIPTIONS_FLAG } from "@multica/core/feature-flags";
import type { OwnerDirectiveType } from "@multica/core/types";
import { Text } from "@/components/ui/text";
import { Button } from "@/components/ui/button";
import { IconButton } from "@/components/ui/icon-button";
import { inboxListOptions, workflowDecisionOptions } from "@/data/queries/inbox";
import { issueDetailOptions } from "@/data/queries/issues";
import {
  appConfigOptions,
  workspaceSubscriptionSummaryOptions,
} from "@/data/queries/billing";
import { useAcknowledgeWorkflowAttention, useArchiveInbox, useCreateOwnerDirective, useRespondWorkflowDecision } from "@/data/mutations/inbox";
import { useCreateComment } from "@/data/mutations/issues";
import { useWorkspaceStore } from "@/data/workspace-store";
import { getAutopilotQuotaBody, getInboxDisplayTitle, ownerInboxCategory } from "@/lib/inbox-display";

const directiveLabels: Record<OwnerDirectiveType, string> = {
  correct: "Correct this", request_changes: "Request revisions",
  partial_acceptance: "Partially accept", change_scope: "Change scope", cancel: "Cancel work",
};
function readable(value: unknown): string | null {
  if (typeof value === "string" && value.trim()) return value.replaceAll("_", " ");
  if (typeof value === "number" || typeof value === "boolean") return String(value);
  return null;
}

function BillingRecovery({
  recovery,
  billingUrl,
}: {
  recovery: BillingRecoveryKind;
  billingUrl: string | null;
}) {
  if (recovery === "checking") return <ActivityIndicator />;
  if (recovery === "billing_disabled") {
    return <Text className="text-sm text-muted-foreground">Billing changes are unavailable for this workspace. Contact its administrator.</Text>;
  }
  if (recovery === "contact_admin") {
    return <Text className="text-sm text-muted-foreground">Ask a workspace owner or admin to review the billing options.</Text>;
  }
  return billingUrl ? (
    <Button onPress={() => void Linking.openURL(billingUrl)}><Text>Review billing options</Text></Button>
  ) : (
    <Text className="text-sm text-muted-foreground">Open Multica on the web to review billing options.</Text>
  );
}

export default function InboxControlDetail() {
  const { id } = useLocalSearchParams<{ id: string }>();
  const wsId = useWorkspaceStore((s) => s.currentWorkspaceId);
  const wsSlug = useWorkspaceStore((s) => s.currentWorkspaceSlug);
  const { data: items, isLoading } = useQuery(inboxListOptions(wsId));
  const item = items?.find((candidate) => candidate.id === id && candidate.workspace_id === wsId);
  const issueId = item?.issue_id ?? "";
  const decisionId = item?.details?.decision_id ?? "";
  const { data: issue, isError: issueError } = useQuery(issueDetailOptions(wsId, issueId));
  const { data: packet, isLoading: decisionLoading, isError: decisionError } = useQuery(workflowDecisionOptions(issueId, decisionId));
  const respond = useRespondWorkflowDecision();
  const createDirective = useCreateOwnerDirective();
  const acknowledge = useAcknowledgeWorkflowAttention();
  const archive = useArchiveInbox();
  const createComment = useCreateComment(issueId);
  const [note, setNote] = useState("");
  const [directive, setDirective] = useState<OwnerDirectiveType | null>(null);
  const category = item ? ownerInboxCategory(item) : "history";
  const summary = issue?.current_picture?.owner_summary;
  const evidence = Object.entries(issue?.current_picture?.evidence_summary ?? {})
    .flatMap(([key, value]) => { const display = readable(value); return display ? [[key, display] as const] : []; })
    .slice(0, 6);
  const isSystemNotice = item?.type === "autopilot_quota_exceeded" || item?.type === "autopilot_paused";
  const isQuotaNotice = item?.type === "autopilot_quota_exceeded";
  const configQuery = useQuery({ ...appConfigOptions(), enabled: isQuotaNotice });
  const checkingConfig = isQuotaNotice && !configQuery.data && configQuery.isFetching;
  const billingEnabled = configQuery.data?.feature_flags?.[BILLING_WORKSPACE_SUBSCRIPTIONS_FLAG] === true;
  const summaryQuery = useQuery(workspaceSubscriptionSummaryOptions(wsId, isQuotaNotice && billingEnabled));
  const recovery = resolveBillingRecovery({
    actions: summaryQuery.data?.availableActions,
    billingEnabled: checkingConfig || billingEnabled,
    loading: checkingConfig || (billingEnabled && !summaryQuery.data?.availableActions && summaryQuery.isFetching),
  });
  const webUrl = process.env.EXPO_PUBLIC_WEB_URL?.replace(/\/+$/, "");
  const billingUrl = webUrl && wsSlug ? `${webUrl}/${wsSlug}/settings?tab=billing` : null;
  const busy = respond.isPending || createDirective.isPending || acknowledge.isPending || archive.isPending || createComment.isPending;
  const showError = (error: unknown) => Alert.alert(
    "That action did not complete",
    error instanceof Error ? error.message : "Nothing was changed. Please try again.",
  );
  const finish = () => {
    if (item) archive.mutate(item.id, { onSuccess: () => router.back(), onError: showError });
  };
  const submitDecision = (outcome: "approve" | "reject") => {
    if (!packet?.decision || !issueId) return;
    const run = () => respond.mutate({ issueId, decisionId: packet.decision.id, request: {
      expected_revision: packet.decision.revision, outcome, comment: note.trim() || undefined,
    } }, { onSuccess: finish, onError: showError });
    const action = outcome === "approve" ? "Approve" : "Reject";
    const noteSummary = note.trim() ? `\n\nYour note: ${note.trim()}` : "";
    Alert.alert(
      `${action} this decision?`,
      `${packet.decision.question}${noteSummary}`,
      [
        { text: "Go back", style: "cancel" },
        { text: action, style: outcome === "reject" ? "destructive" : "default", onPress: run },
      ],
    );
  };
  const submitDirective = () => {
    if (!directive || !issueId || !issue?.revision || !note.trim()) return;
    const run = () => createDirective.mutate({ issueId, request: {
      expected_issue_revision: issue.revision!, directive, content: note.trim(),
    } }, {
      onSuccess: (receipt) => receipt.state === "failed"
        ? Alert.alert("That direction was not applied", "Nothing was changed. Inspect the issue for its current state.")
        : finish(),
      onError: showError,
    });
    if (directive === "cancel") Alert.alert("Cancel this work?", "The current work will stop and the issue will be cancelled.", [
      { text: "Keep working", style: "cancel" }, { text: "Cancel work", style: "destructive", onPress: run },
    ]); else run();
  };
  const acknowledgeItem = () => {
    if (packet?.attention && issueId && !packet.attention.acknowledged) {
      acknowledge.mutate({ issueId, attentionId: packet.attention.id }, { onSuccess: finish, onError: showError });
    } else finish();
  };

  return <KeyboardAvoidingView className="flex-1 bg-background" behavior={Platform.OS === "ios" ? "padding" : undefined}>
    <View className="flex-row items-center border-b border-border px-4 py-3">
      <View className="flex-1 pr-3">
        <Text className="text-xs font-semibold uppercase text-muted-foreground">
          {category === "needs_you" ? "Needs you" : category === "incident" ? "Important incident" : category === "completion" ? "Verified completion" : category === "update" ? "Meaningful update" : "Notice"}
        </Text>
        <Text className="mt-1 text-lg font-semibold text-foreground" numberOfLines={2}>{item ? getInboxDisplayTitle(item) : "Inbox item"}</Text>
      </View>
      <IconButton name="close" variant="secondary" className="size-8 rounded-full" onPress={() => router.back()} accessibilityLabel="Close" />
    </View>
    {isLoading ? <View className="flex-1 items-center justify-center"><ActivityIndicator /></View>
    : !item ? <View className="px-5 py-10"><Text className="text-center text-muted-foreground">This inbox item is no longer available.</Text></View>
    : <ScrollView className="flex-1" contentContainerClassName="gap-5 px-4 py-5" keyboardShouldPersistTaps="handled">
      <View><Text className="text-xs font-semibold uppercase text-muted-foreground">What happened</Text>
        <Text className="mt-2 text-base leading-6 text-foreground">{packet?.decision.question ?? (isSystemNotice ? getAutopilotQuotaBody(item) : item.body) ?? item.title}</Text></View>
      {isQuotaNotice ? <BillingRecovery recovery={recovery} billingUrl={billingUrl} /> : null}
      {decisionId && decisionLoading ? <View className="rounded-xl border border-border p-4"><Text className="text-sm text-muted-foreground">Loading the exact decision and its current revision…</Text></View> : null}
      {decisionId && decisionError ? <View className="rounded-xl border border-red-500/40 bg-red-500/10 p-4"><Text className="text-sm text-foreground">The current decision could not be verified. Approval controls are disabled so stale information cannot authorize work.</Text></View> : null}
      {decisionId && !decisionLoading && !decisionError && !packet ? <View className="rounded-xl border border-amber-500/40 bg-amber-500/10 p-4"><Text className="text-sm text-foreground">This decision is no longer current. It cannot authorize work; inspect the issue for the latest state.</Text></View> : null}
      {issueId && issueError ? <View className="rounded-xl border border-red-500/40 bg-red-500/10 p-4"><Text className="text-sm text-foreground">Current issue evidence is unavailable. You can inspect the issue, but direct-work controls remain disabled until its revision is verified.</Text></View> : null}
      {summary ? <View className="rounded-xl border border-border bg-secondary/30 p-4 gap-3">
        <Text className="text-xs font-semibold uppercase text-muted-foreground">Current state</Text>
        <Text className="text-base font-medium text-foreground">{summary.stage.replaceAll("_", " ")}</Text>
        <Text className="text-sm text-muted-foreground">Latest: {summary.latest_meaningful_change ? `${summary.latest_meaningful_change.code.replaceAll("_", " ")} · ${summary.latest_meaningful_change.result_code}` : "No verified progress recorded"}</Text>
        <Text className="text-sm text-muted-foreground">Next: {summary.next_action ? `${summary.next_action.kind.replaceAll("_", " ")}${summary.next_action.automatic ? " automatically" : ""}` : "No next step recorded"}</Text>
      </View> : null}
      {packet?.decision.recommendation ? <View className="rounded-xl border border-border p-4">
        <Text className="text-sm font-semibold text-foreground">Recommended</Text><Text className="mt-1 text-sm leading-5 text-foreground">{packet.decision.recommendation}</Text>
        <Text className="mt-2 text-sm text-muted-foreground">Consequence: {packet.decision.consequence}</Text>
      </View> : null}
      {evidence.length ? <View className="rounded-xl border border-border p-4 gap-2"><Text className="text-xs font-semibold uppercase text-muted-foreground">Evidence</Text>
        {evidence.map(([key, value]) => <Button key={key} variant="ghost" className="justify-start px-0" onPress={/^https?:\/\//.test(value) ? () => void Linking.openURL(value) : undefined}>
          <Text className="text-sm text-foreground">{key.replaceAll("_", " ")}: {value}</Text></Button>)}
      </View> : null}
      {packet?.decision.state === "unanswered" && packet.decision.type === "authorization" ? <View className="gap-2">
        <Text className="text-xs font-semibold uppercase text-muted-foreground">Your decision</Text><View className="flex-row gap-2">
          <Button className="flex-1" disabled={busy} onPress={() => submitDecision("approve")}><Text>Approve</Text></Button>
          <Button className="flex-1" variant="outline" disabled={busy} onPress={() => submitDecision("reject")}><Text>Reject</Text></Button>
        </View></View> : null}
      {issueId && issue ? <View className="gap-3"><Text className="text-xs font-semibold uppercase text-muted-foreground">Direct the work</Text>
        <View className="flex-row flex-wrap gap-2">{(Object.keys(directiveLabels) as OwnerDirectiveType[]).map((value) => <Button key={value} size="sm" variant={directive === value ? "secondary" : "outline"} onPress={() => setDirective(value)}><Text>{directiveLabels[value]}</Text></Button>)}</View>
        <TextInput value={note} onChangeText={setNote} multiline textAlignVertical="top" placeholder={directive ? `Explain: ${directiveLabels[directive]}` : "Add a comment or explain your direction"} placeholderTextColor="#888" className="min-h-28 rounded-xl border border-border bg-background px-3 py-3 text-base text-foreground" />
        {directive ? <Button disabled={busy || !note.trim()} onPress={submitDirective}><Text>Send {directiveLabels[directive].toLowerCase()}</Text></Button>
        : <Button variant="outline" disabled={busy || !note.trim()} onPress={() => createComment.mutate({ content: note.trim() }, { onSuccess: () => setNote(""), onError: showError })}><Text>Add comment</Text></Button>}
      </View> : null}
      <View className="gap-2 border-t border-border pt-4">
        {issueId && wsSlug ? <Button variant="outline" onPress={() => router.push(`/${wsSlug}/issue/${issueId}`)}><Text>Inspect full issue</Text></Button> : null}
        {category !== "needs_you" ? <Button variant="ghost" disabled={busy} onPress={acknowledgeItem}><Text>Acknowledge</Text></Button> : null}
      </View>
      {!decisionId && category === "needs_you" ? <View className="rounded-xl border border-amber-500/40 bg-amber-500/10 p-3"><Text className="text-sm text-foreground">This older request has no atomic decision receipt. It cannot approve work from this screen; add a correction or inspect the issue.</Text></View> : null}
    </ScrollView>}
  </KeyboardAvoidingView>;
}
