"use client";

import { useEffect, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { ArrowLeft } from "lucide-react";
import {
  ResizablePanelGroup,
  ResizablePanel,
  ResizableHandle,
} from "@multica/ui/components/ui/resizable";
import { Skeleton } from "@multica/ui/components/ui/skeleton";
import { Button } from "@multica/ui/components/ui/button";
import {
  knowledgeHubTreeOptions,
  knowledgeHubFileOptions,
} from "@multica/core/knowledge/queries";
import { FileTree } from "../../skills/components/file-tree";
import { RichContent } from "../../rich-content";
import { useT } from "../../i18n";

/**
 * Read-only browser over every markdown file in the hub — same FileTree
 * component Skills uses (omitting `actions` is what makes it read-only) and
 * the same RichContent renderer chat/issues/skills already use. Demoted to
 * a secondary "browse all files" escape hatch behind the project-card
 * landing view (KnowledgeHubProjects) — the owner rejected this as the
 * primary experience: hundreds of dated filenames with no way to tell what
 * matters. Kept for the rare case someone needs a file this project's
 * curated card doesn't surface.
 */
export function KnowledgeHubBrowser({ onBack }: { onBack: () => void }) {
  const { t } = useT("knowledge");
  const { data: tree, isLoading: treeLoading } = useQuery(knowledgeHubTreeOptions());
  const [selectedPath, setSelectedPath] = useState("");

  const files = tree?.files ?? [];
  useEffect(() => {
    if (!selectedPath && files.length > 0) {
      setSelectedPath(files[0]!.path);
    }
  }, [files, selectedPath]);

  const { data: file, isLoading: fileLoading, isError: fileError } = useQuery(
    knowledgeHubFileOptions(selectedPath),
  );

  return (
    <ResizablePanelGroup orientation="horizontal" className="h-full">
      <ResizablePanel id="knowledge-tree" defaultSize={280} minSize={220} maxSize={420}>
        <div className="flex h-full flex-col border-r">
          <div className="shrink-0 border-b p-2">
            <Button variant="ghost" size="sm" className="gap-1.5" onClick={onBack}>
              <ArrowLeft className="size-3.5" />
              {t(($) => $.hub.back_to_projects)}
            </Button>
          </div>
          <div className="flex-1 overflow-y-auto px-2 py-3">
            {treeLoading ? (
              <div className="space-y-2 px-2">
                {Array.from({ length: 8 }).map((_, i) => (
                  <Skeleton key={i} className="h-6 w-full" />
                ))}
              </div>
            ) : (
              <FileTree
                filePaths={files.map((f) => f.path)}
                selectedPath={selectedPath}
                onSelect={setSelectedPath}
              />
            )}
          </div>
        </div>
      </ResizablePanel>
      <ResizableHandle />
      <ResizablePanel id="knowledge-file" minSize="40%">
        <div className="h-full overflow-y-auto">
          {!selectedPath ? (
            <div className="flex h-full items-center justify-center text-muted-foreground">
              <p className="text-body">{t(($) => $.hub.select_prompt)}</p>
            </div>
          ) : fileLoading ? (
            <div className="mx-auto max-w-[68ch] space-y-3 px-6 pt-7 sm:px-8">
              <Skeleton className="h-7 w-2/3" />
              <Skeleton className="h-4 w-full" />
              <Skeleton className="h-4 w-full" />
              <Skeleton className="h-4 w-5/6" />
            </div>
          ) : fileError || !file ? (
            <div className="flex h-full items-center justify-center text-muted-foreground">
              <p className="text-body">{t(($) => $.hub.load_failed)}</p>
            </div>
          ) : (
            <div className="mx-auto max-w-[68ch] px-6 pb-24 pt-7 sm:px-8">
              <RichContent content={file.content} density="document" phase="settled" />
            </div>
          )}
        </div>
      </ResizablePanel>
    </ResizablePanelGroup>
  );
}
