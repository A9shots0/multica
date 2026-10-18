"use client";

import { useMemo, useState } from "react";
import { ExternalLink, FileDown, Files, Maximize2, MessageSquareReply } from "lucide-react";
import { toast } from "sonner";
import type { ChatWorkUpdate } from "@multica/core/types";
import {
  attachmentPreviewId,
  describeSources,
  framingBlocked,
  sandboxFor,
} from "@multica/core/inbox/owner-inbox-model";
import { PreviewViewer, type PreviewSource } from "../../inbox/components/preview-viewer";
import { describeMarks, type Mark } from "../../inbox/components/screenshot-annotator";
import { useOwnerReviewFeedback } from "../../inbox/components/use-owner-review-feedback";
import { useT } from "../../i18n";

/**
 * A visual result Mika is showing inside the conversation. Chat owns the
 * compact attachment; the shared full-screen viewer owns inspection and
 * annotation so Inbox and Chat never develop two different review gestures.
 */
export function ChatReviewArtifact({
  update,
  onUseFeedback,
  onReplyArtifact,
}: {
  update: ChatWorkUpdate;
  onUseFeedback?: (feedback: string) => void;
  onReplyArtifact?: (artifactId: string) => void;
}) {
  const { t } = useT("chat");
  const [fullScreen, setFullScreen] = useState(false);
  const [startId, setStartId] = useState<string | undefined>();
  const [marks, setMarks] = useState<Mark[]>([]);
  const artifacts = useMemo(() => update.review?.artifacts ?? [], [update.review]);
  const sources = useMemo<PreviewSource[]>(
    () => artifacts.flatMap((artifact) => artifact.kind === "page" || artifact.kind === "image"
      ? [{ id: artifact.id, label: artifact.label, kind: artifact.kind, url: artifact.url }]
      : []),
    [artifacts],
  );
  const sourceNames = useMemo(() => describeSources(sources), [sources]);
  const active = sources[0];
  const files = artifacts.filter((artifact) => artifact.kind === "file");
  const multiple = artifacts.length > 1;
  const secondarySources = sources.slice(1);
  const hasRail = secondarySources.length > 0 || files.length > 0;
  const attachmentRefs = useMemo(
    () => artifacts
      .filter((artifact) => artifact.kind === "image")
      .map((artifact) => ({ id: artifact.id, filename: artifact.label })),
    [artifacts],
  );
  const { persistMarkedEvidence, savingFeedback } = useOwnerReviewFeedback({
    issueId: update.issue_id,
    sources,
    attachments: attachmentRefs,
    marks,
    note: "",
  });
  if (!update.review || artifacts.length === 0) return null;

  const origin = typeof window === "undefined" ? undefined : window.location.origin;
  const reviewLabel = active
    ? t(($) => $.updates.review_and_annotate_named, { name: active.label })
    : t(($) => $.updates.review_and_annotate);

  const openReview = (id?: string) => {
    setStartId(id ?? active?.id);
    setFullScreen(true);
  };

  const preview = (source: PreviewSource) => {
    const sourceBlocked = source.kind === "page" && framingBlocked(source.url, origin);
    const attachedPage = source.kind === "page" && attachmentPreviewId(source.url, origin);
    return (
      <>
        {source.kind === "image" ? (
          <img src={source.url} alt={source.label} className="h-full w-full bg-black/10 object-contain transition-transform duration-200 group-hover:scale-[1.01]" />
        ) : sourceBlocked || attachedPage ? (
          <span className="flex h-full flex-col items-center justify-center gap-2 px-4 text-center text-caption text-muted-foreground">
            <ExternalLink className="size-5" />
            <span className="line-clamp-2">{source.label}</span>
          </span>
        ) : (
          <iframe src={source.url} title={source.label} sandbox={sandboxFor(source.url, origin)} referrerPolicy="no-referrer" tabIndex={-1} className="pointer-events-none h-full w-full bg-white" />
        )}
      </>
    );
  };

  const railSource = (source: PreviewSource) => (
    <button
      key={source.id}
      type="button"
      onClick={() => openReview(source.id)}
      className="flex h-10 max-w-48 shrink-0 items-center gap-2 rounded-lg bg-muted/70 pr-2.5 text-left text-caption font-medium text-muted-foreground transition-colors hover:bg-muted hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
      aria-label={t(($) => $.updates.open_deliverable_named, { name: source.label })}
    >
      <span className="flex size-10 shrink-0 items-center justify-center overflow-hidden rounded-l-lg bg-black/10">
        {source.kind === "image"
          ? <img src={source.url} alt="" className="h-full w-full object-cover object-top" />
          : <ExternalLink className="size-4" />}
      </span>
      <span className="truncate">
        {source.kind === "page"
          ? t(($) => $.updates.page_preview)
          : (sourceNames.get(source.id) ?? source.label)}
      </span>
    </button>
  );

  const useFeedback = async () => {
    const drawn = describeMarks(marks, sourceNames);
    if (!drawn || !onUseFeedback) return;
    try {
      const evidence = await persistMarkedEvidence();
      onUseFeedback([
        t(($) => $.updates.feedback_intro),
        drawn,
        evidence.summary,
      ].filter(Boolean).join("\n\n"));
      setFullScreen(false);
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t(($) => $.updates.review_save_failed));
    }
  };

  return (
    <section className="mt-2.5 w-full max-w-lg" data-chat-review-artifact>
      <div className="overflow-hidden rounded-xl border bg-muted/10">
        {active && (
          <div className="group relative h-52 overflow-hidden bg-muted/20">
            <button
              type="button"
              onClick={() => openReview(active.id)}
              className="absolute inset-0 w-full text-left focus-visible:z-10 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-ring"
              aria-label={reviewLabel}
            >
              {preview(active)}
              <span className="pointer-events-none absolute inset-x-0 bottom-0 flex items-end justify-between gap-2 bg-gradient-to-t from-black/80 via-black/45 to-transparent px-3 pb-2.5 pt-10 text-caption font-medium text-white">
                <span className="truncate">{active.label}</span>
                <Maximize2 className="size-3.5 shrink-0" />
              </span>
              {(multiple || marks.length > 0) && (
                <span className="pointer-events-none absolute left-2.5 top-2.5 flex gap-1.5">
                  {multiple && <span className="inline-flex h-6 items-center gap-1 rounded-md bg-black/65 px-2 text-caption font-medium text-white backdrop-blur-sm"><Files className="size-3.5" />{t(($) => $.updates.files, { count: artifacts.length })}</span>}
                  {marks.length > 0 && <span className="inline-flex h-6 items-center rounded-md bg-black/65 px-2 text-caption font-medium text-white backdrop-blur-sm">{t(($) => $.updates.note_count, { count: marks.length })}</span>}
                </span>
              )}
            </button>
            {onReplyArtifact && (
              <button
                type="button"
                className="absolute right-2.5 top-2.5 z-20 flex size-8 items-center justify-center rounded-lg bg-black/65 text-white opacity-80 backdrop-blur-sm transition-opacity hover:opacity-100 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-white sm:opacity-0 sm:group-hover:opacity-80 sm:focus:opacity-100"
                onClick={() => onReplyArtifact(active.id)}
                aria-label={t(($) => $.message_list.reply_action)}
                title={active.label}
              >
                <MessageSquareReply className="size-4" />
              </button>
            )}
          </div>
        )}

        {hasRail && (
          <div
            data-chat-review-files={files.length > 0 || undefined}
            className={`flex items-center gap-1.5 overflow-x-auto p-2 [scrollbar-width:none] ${active ? "border-t" : ""}`}
          >
            {!active && multiple && <span className="inline-flex h-10 shrink-0 items-center gap-1.5 px-1 text-caption font-medium text-muted-foreground"><Files className="size-4" />{t(($) => $.updates.files, { count: artifacts.length })}</span>}
            {secondarySources.map(railSource)}
            {files.map((file) => (
              <a key={file.id} href={file.url} aria-label={file.label} className="flex h-10 max-w-56 shrink-0 items-center gap-2 rounded-lg bg-muted/70 pr-2.5 text-caption font-medium text-muted-foreground transition-colors hover:bg-muted hover:text-foreground">
                <span className="flex size-10 shrink-0 items-center justify-center rounded-l-lg bg-muted"><FileDown className="size-4" /></span>
                <span className="truncate">{compactFileLabel(file.label)}</span>
              </a>
            ))}
          </div>
        )}
      </div>

      {fullScreen && sources.length > 0 && (
        <PreviewViewer
          sources={sources}
          startId={startId}
          marks={marks}
          onChangeMarks={setMarks}
          allowMarking
          onClose={() => setFullScreen(false)}
          title={update.title}
          primaryAction={marks.length > 0 && onUseFeedback ? {
            label: t(($) => $.updates.add_notes_to_chat, { count: marks.length }),
            disabled: marks.some((mark) => !mark.note.trim()),
            pending: savingFeedback,
            onClick: useFeedback,
          } : undefined}
        />
      )}
    </section>
  );
}

function compactFileLabel(label: string): string {
  const match = label.trim().match(/^(.*?)(?:\.([a-z0-9]{1,5}))?$/i);
  if (!match) return label;
  const base = (match[1] ?? label).trim();
  const extension = match[2]?.toUpperCase();
  const words = base.split(/\s+/).filter(Boolean);
  const shortBase = words.slice(-2).join(" ") || base;
  return extension ? `${shortBase} · ${extension}` : shortBase;
}
