"use client";

import { usePersistOwnerReviewFeedback } from "@multica/core/inbox/mutations";
import type { Mark } from "./screenshot-annotator";
import type { PreviewSource } from "./preview-viewer";
import { renderMarkedScreenshot } from "./marked-screenshot";

/** Shared fail-closed persistence for both Inbox review layouts. */
export function useOwnerReviewFeedback({
  issueId,
  inboxItemId,
  sources,
  attachments,
  marks,
  note,
}: {
  issueId: string;
  inboxItemId?: string;
  sources: PreviewSource[];
  attachments: readonly { id: string; filename: string }[];
  marks: Mark[];
  note: string;
}) {
  const persist = usePersistOwnerReviewFeedback();

  const persistMarkedEvidence = async (ownerNote = note): Promise<{ summary: string; issueRevision?: number }> => {
    if (marks.length === 0) return { summary: "" };
    if (!issueId) throw new Error("This review is not linked to a piece of work.");
    if (marks.some((mark) => !mark.note.trim())) {
      throw new Error("Add a short note to every marked area before sending it.");
    }

    const receipts: string[] = [];
    let issueRevision: number | undefined;
    for (const sourceID of new Set(marks.map((mark) => mark.shot))) {
      const source = sources.find((candidate) => candidate.id === sourceID);
      const attachment = attachments.find((candidate) => candidate.id === sourceID);
      if (!source || source.kind !== "image" || !attachment) {
        throw new Error("Only saved screenshots can be marked. Capture the page as an image first.");
      }
      const sourceMarks = marks.filter((mark) => mark.shot === sourceID);
      const file = await renderMarkedScreenshot(source.url, attachment.filename, sourceMarks);
      const { feedback, marked } = await persist.mutateAsync({
        issueId,
        file,
        request: {
          ...(inboxItemId ? { inbox_item_id: inboxItemId } : {}),
          source_attachment_id: sourceID,
          marks: sourceMarks,
          note: ownerNote.trim(),
        },
      });
      issueRevision = feedback.issue_revision;
      receipts.push(`${feedback.source_filename}: marked screenshot attachment ${marked.id}`);
    }
    return {
      summary: `Saved review evidence:\n${receipts.map((receipt) => `- ${receipt}`).join("\n")}`,
      issueRevision,
    };
  };

  return { persistMarkedEvidence, savingFeedback: persist.isPending };
}
