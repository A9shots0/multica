import { useMemo, useRef } from "react";
import { useChatStore } from "@multica/core/chat";
import { getCurrentSlug } from "@multica/core/platform";
import type { DiagramReviewActions, DiagramReviewReceiver } from "../../editor/diagram-review-context";

export function useChatDiagramReview(sessionId: string | null, enabled: boolean) {
  const receiver = useRef<DiagramReviewReceiver | null>(null);
  const workspace = getCurrentSlug();
  const actions = useMemo<DiagramReviewActions | null>(() => {
    if (!sessionId || !enabled) return null;
    const key = (chart: string) => JSON.stringify([sessionId, chart]);
    const current = () => getCurrentSlug() === workspace
      && useChatStore.getState().activeSessionId === sessionId;
    return {
      readDraft: (chart) => current() ? useChatStore.getState().diagramReviews?.[key(chart)] : undefined,
      saveDraft: (chart, draft) => { if (current()) useChatStore.getState().setDiagramReview(key(chart), draft); },
      addToDraft: (attachment) => current() && !!receiver.current?.(attachment, sessionId),
    };
  }, [sessionId, enabled, workspace]);
  return { actions, receiver };
}
