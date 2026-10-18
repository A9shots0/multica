import { useMemo, useState } from "react";
import {
  ActionSheetIOS,
  Alert,
  FlatList,
  Pressable,
  View,
} from "react-native";
import { useQuery } from "@tanstack/react-query";
import { router } from "expo-router";
import { Ionicons } from "@expo/vector-icons";
import type { InboxItem, OwnerInboxCategory } from "@multica/core/types";
import { Text } from "@/components/ui/text";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import { Header } from "@/components/ui/header";
import { IconButton } from "@/components/ui/icon-button";
import { HeaderActions } from "@/components/ui/app-header-actions";
import { SwipeableInboxRow } from "@/components/inbox/swipeable-inbox-row";
import { inboxListOptions } from "@/data/queries/inbox";
import {
  useArchiveAllInbox,
  useArchiveAllReadInbox,
  useArchiveCompletedInbox,
  useArchiveInbox,
  useMarkAllInboxRead,
  useMarkInboxRead,
} from "@/data/mutations/inbox";
import { useWorkspaceStore } from "@/data/workspace-store";
import { useColorScheme } from "@/lib/use-color-scheme";
import { THEME } from "@/lib/theme";
import { cn } from "@/lib/utils";
import {
  deduplicateInboxItems,
  getInboxNavigationTarget,
  ownerInboxCategory,
} from "@/lib/inbox-display";

export default function Inbox() {
  const [filter, setFilter] = useState<OwnerInboxCategory | null>(null);
  const wsId = useWorkspaceStore((s) => s.currentWorkspaceId);
  const wsSlug = useWorkspaceStore((s) => s.currentWorkspaceSlug);
  const { colorScheme } = useColorScheme();
  const { data: rawItems, isLoading, error, refetch, isRefetching } = useQuery(
    inboxListOptions(wsId),
  );
  // Dedup + drop archived to match web/desktop. See CLAUDE.md
  // "Behavioral parity" → inbox dedup incident.
  const ownerItems = useMemo(
    () => deduplicateInboxItems(rawItems ?? []),
    [rawItems],
  );
  const counts = useMemo(() => ({
    needs_you: ownerItems.filter((item) => ownerInboxCategory(item) === "needs_you").length,
    incident: ownerItems.filter((item) => ownerInboxCategory(item) === "incident").length,
    update: ownerItems.filter((item) => ownerInboxCategory(item) === "update").length,
    completion: ownerItems.filter((item) => ownerInboxCategory(item) === "completion").length,
  }), [ownerItems]);
  const data = useMemo(
    () => filter ? ownerItems.filter((item) => ownerInboxCategory(item) === filter) : ownerItems,
    [filter, ownerItems],
  );
  const markRead = useMarkInboxRead();
  const markAllRead = useMarkAllInboxRead();
  const archive = useArchiveInbox();
  const archiveAll = useArchiveAllInbox();
  const archiveAllRead = useArchiveAllReadInbox();
  const archiveCompleted = useArchiveCompletedInbox();

  const onPressItem = (item: InboxItem) => {
    if (!item.read) {
      // Optimistic read flip lives in useMarkInboxRead.onMutate — fires
      // setQueryData synchronously before the cancelQueries await, so the
      // row is already styled "read" by the time iOS captures the source
      // snapshot for the native stack push transition.
      markRead.mutate(item.id);
    }
    const target = getInboxNavigationTarget(item, wsSlug, String(Date.now()));
    if (target) router.push(target);
  };

  // Trailing batch menu — mirrors web's dropdown
  // (packages/views/inbox/components/inbox-page.tsx). "Mark all read" is
  // first (most common batch op); "Archive all" is destructive so it gets
  // the iOS red treatment + Alert confirm.
  const onPressMenu = () => {
    const options = [
      "Cancel",
      "Mark all read",
      "Archive all read",
      "Archive completed",
      "Archive all",
    ];
    ActionSheetIOS.showActionSheetWithOptions(
      {
        options,
        cancelButtonIndex: 0,
        destructiveButtonIndex: 4,
        title: "Inbox",
      },
      (i) => {
        if (i === 1) markAllRead.mutate();
        else if (i === 2) archiveAllRead.mutate();
        else if (i === 3) archiveCompleted.mutate();
        else if (i === 4) {
          Alert.alert(
            "Archive all?",
            "This archives every inbox item, read or unread. You can still find them via the issue pages.",
            [
              { text: "Cancel", style: "cancel" },
              {
                text: "Archive all",
                style: "destructive",
                onPress: () => archiveAll.mutate(),
              },
            ],
          );
        }
      },
    );
  };

  return (
    <View className="flex-1 bg-background">
      <Header
        title="Inbox"
        right={
          <>
            <IconButton
              name="ellipsis-horizontal"
              onPress={onPressMenu}
              accessibilityLabel="Inbox actions"
            />
            <HeaderActions />
          </>
        }
      />
      <View className="flex-row flex-wrap gap-2 border-b border-border px-4 py-3">
        {([
          ["needs_you", "Needs you"],
          ["incident", "Incidents"],
          ["update", "Updates"],
          ["completion", "Verified"],
        ] as const).map(([category, label]) => (
          <Pressable
            key={category}
            accessibilityRole="button"
            accessibilityState={{ selected: filter === category }}
            onPress={() => setFilter((current) => current === category ? null : category)}
            className={cn(
              "w-[48%] flex-row items-center justify-between rounded-lg border px-3 py-2.5",
              filter === category ? "border-brand bg-brand/10" : "border-border bg-secondary/40",
            )}
          >
            <Text className="text-sm font-medium text-foreground">{label}</Text>
            <Text className="text-sm font-semibold text-muted-foreground">{counts[category]}</Text>
          </Pressable>
        ))}
      </View>
      {isLoading ? (
        <InboxLoading />
      ) : error ? (
        <View className="px-4 gap-3 pt-4">
          <Text className="text-sm text-destructive">
            Failed to load inbox:{" "}
            {error instanceof Error ? error.message : "unknown error"}
          </Text>
          <Button variant="outline" onPress={() => refetch()}>
            <Text>Retry</Text>
          </Button>
        </View>
      ) : !data || data.length === 0 ? (
        <InboxEmpty iconColor={THEME[colorScheme].mutedForeground} />
      ) : (
        <FlatList
          data={data}
          keyExtractor={(item) => item.id}
          ItemSeparatorComponent={() => (
            <View className="h-px bg-border ml-4" />
          )}
          contentContainerClassName="pb-6"
          renderItem={({ item }) => (
            <SwipeableInboxRow
              item={item}
              onPress={() => onPressItem(item)}
              onArchive={() => archive.mutate(item.id)}
            />
          )}
          refreshing={isRefetching}
          onRefresh={refetch}
        />
      )}
    </View>
  );
}

// Loading state — 6 row-shaped Skeletons matching InboxRow's layout.
// spinner because the eye immediately sees the list-like structure.
function InboxLoading() {
  return (
    <View className="px-4 pt-4 gap-4">
      {Array.from({ length: 6 }).map((_, i) => (
        <View key={i} className="flex-row gap-3 px-4">
          <View className="flex-1 gap-2 pt-1">
            <Skeleton className="h-3.5 w-3/4" />
            <Skeleton className="h-3 w-1/2" />
          </View>
        </View>
      ))}
    </View>
  );
}

function InboxEmpty({ iconColor }: { iconColor: string }) {
  return (
    <View className="flex-1 items-center justify-center px-8 gap-3">
      <Ionicons name="mail-open-outline" size={42} color={iconColor} />
      <Text className="text-base font-medium text-foreground text-center">
        Inbox zero
      </Text>
      <Text className="text-sm text-muted-foreground text-center">
        Decisions that need you, important incidents, meaningful progress, and
        independently verified completions appear here.
      </Text>
    </View>
  );
}
