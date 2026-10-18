"use client";

import { createContext, useContext } from "react";
import type { DiagramReviewDraft } from "@multica/core/chat/diagram-review";

export interface DiagramReviewAttachment { file: File; notes: string }
export type DiagramReviewReceiver = (attachment: DiagramReviewAttachment, draftKey: string) => boolean;
export interface DiagramReviewActions {
  readDraft: (chart: string) => DiagramReviewDraft | undefined;
  saveDraft: (chart: string, draft: DiagramReviewDraft) => void;
  addToDraft: (attachment: DiagramReviewAttachment) => boolean;
}

// Rendering/composer capability plumbing only. Draft ownership and persistence
// belong to the host surface and its Core store, never to this context.
const DiagramReviewContext = createContext<DiagramReviewActions | null>(null);
export const DiagramReviewProvider = DiagramReviewContext.Provider;
export const useDiagramReviewActions = () => useContext(DiagramReviewContext);
