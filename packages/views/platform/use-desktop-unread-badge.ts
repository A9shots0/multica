import { useEffect } from "react";
import { useOwnerInboxWaitingCount } from "@multica/core/inbox/queries";

type BadgeCapableAPI = {
  setUnreadBadge?: (count: number) => void;
};

function getDesktopAPI(): BadgeCapableAPI | undefined {
  if (typeof window === "undefined") return undefined;
  return (window as unknown as { desktopAPI?: BadgeCapableAPI }).desktopAPI;
}

/**
 * Mirror the count of pieces of work waiting on the owner onto the OS
 * dock/taskbar badge — not the raw unread-event count. The whole point of
 * that badge is "you have this many real things to decide"; badging it with
 * every unread crash log a stalled run produced trains the owner to stop
 * trusting the number on his own taskbar. No-op on web (no `desktopAPI`) and
 * on the login screen (no workspace ⇒ count defaults to 0, which clears any
 * stale badge from a previous session).
 */
export function useDesktopUnreadBadge(wsId: string | null | undefined): void {
  const count = useOwnerInboxWaitingCount(wsId);
  useEffect(() => {
    getDesktopAPI()?.setUnreadBadge?.(count);
  }, [count]);
}
