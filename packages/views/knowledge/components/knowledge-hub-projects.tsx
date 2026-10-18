"use client";

import { useQuery } from "@tanstack/react-query";
import { FolderOpen, Library } from "lucide-react";
import {
  Card,
  CardHeader,
  CardTitle,
  CardDescription,
} from "@multica/ui/components/ui/card";
import { Button } from "@multica/ui/components/ui/button";
import { Skeleton } from "@multica/ui/components/ui/skeleton";
import { knowledgeHubProjectsOptions } from "@multica/core/knowledge/queries";
import type { KnowledgeHubProjectSummary } from "@multica/core/types/api";
import { useT } from "../../i18n";

/**
 * Default landing view for the Company Knowledge tab: one card per
 * registered project (registry.json), each showing the plain-English
 * `- Role:` line already written into that project's CONTEXT.md — the
 * owner's own words for what a given company repo/system does, not a raw
 * file listing. Replaces the file tree as the first thing a human sees;
 * the tree survives as "Browse all files" for the rare case a project
 * card doesn't surface the specific doc someone needs.
 */
export function KnowledgeHubProjects({
  onSelectProject,
  onBrowseAll,
}: {
  onSelectProject: (project: KnowledgeHubProjectSummary) => void;
  onBrowseAll: () => void;
}) {
  const { t } = useT("knowledge");
  const { data, isLoading } = useQuery(knowledgeHubProjectsOptions());

  if (!isLoading && data && !data.configured) {
    return (
      <div className="flex h-full flex-col items-center justify-center gap-2 text-muted-foreground">
        <FolderOpen className="size-6 text-faint-foreground" />
        <p className="text-body">{t(($) => $.hub.not_configured)}</p>
      </div>
    );
  }

  const projects = data?.projects ?? [];

  return (
    <div className="h-full overflow-y-auto px-6 py-5 sm:px-8">
      <div className="mx-auto max-w-5xl">
        <div className="flex items-center justify-end pb-4">
          <Button variant="ghost" size="sm" className="gap-1.5" onClick={onBrowseAll}>
            <Library className="size-3.5" />
            {t(($) => $.hub.browse_all_files)}
          </Button>
        </div>

        {isLoading ? (
          <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3">
            {Array.from({ length: 6 }).map((_, i) => (
              <Skeleton key={i} className="h-28 w-full rounded-xl" />
            ))}
          </div>
        ) : projects.length === 0 ? (
          <div className="flex flex-col items-center justify-center gap-2 py-16 text-muted-foreground">
            <FolderOpen className="size-6 text-faint-foreground" />
            <p className="text-body">{t(($) => $.hub.no_projects)}</p>
          </div>
        ) : (
          <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3">
            {projects.map((project) => (
              <Card
                key={project.id}
                role="button"
                tabIndex={0}
                onClick={() => onSelectProject(project)}
                onKeyDown={(e) => {
                  if (e.key === "Enter" || e.key === " ") {
                    e.preventDefault();
                    onSelectProject(project);
                  }
                }}
                className="cursor-pointer bg-surface-raised transition-colors hover:bg-muted/50 focus-visible:ring-3 focus-visible:ring-ring/50 focus-visible:outline-none"
              >
                <CardHeader>
                  <CardTitle>{project.name}</CardTitle>
                  <CardDescription className="line-clamp-3 text-caption">
                    {project.summary || t(($) => $.hub.no_summary)}
                  </CardDescription>
                </CardHeader>
              </Card>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
