/**
 * Inbox row content — the visual half, no gesture wrapping. Pulled out of
 * (tabs)/inbox.tsx so swipeable-inbox-row.tsx can wrap it with the gesture
 * recognizer without duplicating layout. Keep this file purely presentational
 * — the swipe and the press behaviour live in the wrapper.
 *
 * The compact owner-control row intentionally omits actor avatars and status
 * icons. That space belongs to a readable title, meaningful detail, category,
 * and timestamp; tapping opens the bounded control screen.
 */
import { Pressable, View } from "react-native";
import type { InboxItem } from "@multica/core/types";
import { Text } from "@/components/ui/text";
import { getInboxDisplayTitle, ownerInboxCategory } from "@/lib/inbox-display";
import { timeAgo } from "@/lib/time-ago";
import { cn } from "@/lib/utils";

interface Props {
  item: InboxItem;
  onPress: () => void;
}

export function InboxRow({ item, onPress }: Props) {
  const isUnread = !item.read;
  const displayTitle = getInboxDisplayTitle(item);
  const category = ownerInboxCategory(item);
  const categoryLabel = category === "needs_you" ? "Needs you"
    : category === "incident" ? "Incident"
    : category === "completion" ? "Verified"
    : "Update";
  const detail = item.details?.meaningful_summary ?? item.body ?? item.title;

  return (
    <Pressable onPress={onPress} className="bg-background active:bg-secondary px-4 py-3">
      <View className="flex-1 min-w-0">
          <View className="flex-row items-start gap-2">
            <View className="flex-row items-start gap-1.5 flex-1 min-w-0">
              {isUnread ? (
                <View className="size-1.5 rounded-full bg-brand shrink-0" />
              ) : null}
              <Text
                className={cn(
                  "flex-1 text-sm",
                  isUnread
                    ? "font-medium text-foreground"
                    : "text-muted-foreground",
                )}
                numberOfLines={2}
              >
                {displayTitle}
              </Text>
            </View>
          </View>
          <View className="flex-row items-end gap-2 mt-1">
            <View className="flex-1 min-w-0">
              <Text className={cn("text-xs leading-4", isUnread ? "text-muted-foreground" : "text-muted-foreground/60")} numberOfLines={2}>
                {detail}
              </Text>
            </View>
          </View>
          <View className="mt-2 flex-row items-center justify-between">
            <View className="rounded bg-secondary px-2 py-1">
              <Text className="text-[11px] font-semibold uppercase text-muted-foreground">{categoryLabel}</Text>
            </View>
            <Text className="text-xs text-muted-foreground">{timeAgo(item.created_at)}</Text>
          </View>
      </View>
    </Pressable>
  );
}
