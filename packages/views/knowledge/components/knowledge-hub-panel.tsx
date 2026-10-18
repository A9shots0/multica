"use client";

import { useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { ArrowLeft, FileText, History } from "lucide-react";
import { Button } from "@multica/ui/components/ui/button";
import { Skeleton } from "@multica/ui/components/ui/skeleton";
import { knowledgeHubFileOptions } from "@multica/core/knowledge/queries";
import type { KnowledgeHubProjectSummary } from "@multica/core/types/api";
import { RichContent } from "../../rich-content";
import { useT } from "../../i18n";
import { KnowledgeHubProjects } from "./knowledge-hub-projects";
import { KnowledgeHubBrowser } from "./knowledge-hub-browser";
import { HubChangelogTimeline } from "./hub-changelog-timeline";
import { parseContextSections, parseChangelogEntries, type HubDocSection } from "../hub-doc-sections";

type View =
  | { kind: "projects" }
  | { kind: "project"; project: KnowledgeHubProjectSummary }
  | { kind: "browse" };

/**
 * Company Knowledge tab. Three views, not one raw file tree:
 *  - "projects" (default): curated cards, one per registered project.
 *  - "project": a chosen project's briefing — role + recent activity
 *    headlines by default, with the full technical documentation one click
 *    away, not forced on the reader (see ProjectDetail).
 *  - "browse": the full markdown tree, demoted to an escape hatch.
 * See KnowledgeHubProjects/KnowledgeHubBrowser for why the tree isn't the
 * default: hundreds of dated filenames, no way to tell what matters without
 * opening each one.
 */
export function KnowledgeHubPanel() {
  const [view, setView] = useState<View>({ kind: "projects" });

  if (view.kind === "browse") {
    return <KnowledgeHubBrowser onBack={() => setView({ kind: "projects" })} />;
  }

  if (view.kind === "project") {
    return (
      <ProjectDetail project={view.project} onBack={() => setView({ kind: "projects" })} />
    );
  }

  return (
    <KnowledgeHubProjects
      onSelectProject={(project) => setView({ kind: "project", project })}
      onBrowseAll={() => setView({ kind: "browse" })}
    />
  );
}

type ProjectTab = "overview" | "activity" | "technical";

/**
 * A project's briefing has three layers, matched to who's reading:
 *  - "overview" (default): the plain-English role line plus the 5 most
 *    recent changelog headlines — what this is and what's happened lately,
 *    scannable in a few seconds.
 *  - "activity": every changelog entry, same headline-first treatment,
 *    for when the short list isn't enough.
 *  - "technical": the full CONTEXT.md, split into chapters with a table of
 *    contents (see hub-doc-sections.ts) — the dense, citation-heavy
 *    engineering record an agent needs to do real work here. Real and
 *    worth keeping, just not what a non-technical reader opens this page
 *    for, so it's a deliberate second click, not the landing view.
 */
function ProjectDetail({
  project,
  onBack,
}: {
  project: KnowledgeHubProjectSummary;
  onBack: () => void;
}) {
  const { t } = useT("knowledge");
  const [tab, setTab] = useState<ProjectTab>("overview");

  const changelogNeeded = tab === "overview" || tab === "activity";
  const path = changelogNeeded ? project.changelog : project.context;
  const { data: file, isLoading, isError } = useQuery(knowledgeHubFileOptions(path));

  const changelogEntries = useMemo<HubDocSection[]>(() => {
    if (!changelogNeeded || !file?.content) return [];
    return parseChangelogEntries(file.content);
  }, [changelogNeeded, file]);

  const technicalSections = useMemo<HubDocSection[]>(() => {
    if (changelogNeeded || !file?.content) return [];
    return parseContextSections(file.content);
  }, [changelogNeeded, file]);

  const scrollToSection = (id: string) => {
    document.getElementById(`hub-section-${id}`)?.scrollIntoView({
      behavior: "smooth",
      block: "start",
    });
  };

  return (
    <div className="flex h-full flex-col">
      <div className="flex shrink-0 items-center gap-1 border-b px-4 py-2 sm:px-6">
        <Button variant="ghost" size="sm" className="gap-1.5" onClick={onBack}>
          <ArrowLeft className="size-3.5" />
          {t(($) => $.hub.back_to_projects)}
        </Button>
        <span className="mx-1 text-muted-foreground">/</span>
        <span className="text-body font-medium">{project.name}</span>
        <div className="ml-auto flex items-center gap-1">
          <Button
            variant={tab === "overview" ? "secondary" : "ghost"}
            size="sm"
            onClick={() => setTab("overview")}
          >
            {t(($) => $.hub.tab_overview)}
          </Button>
          <Button
            variant={tab === "activity" ? "secondary" : "ghost"}
            size="sm"
            disabled={!project.changelog}
            onClick={() => setTab("activity")}
          >
            {t(($) => $.hub.tab_changelog)}
          </Button>
          <Button
            variant={tab === "technical" ? "secondary" : "ghost"}
            size="sm"
            className="gap-1.5 text-muted-foreground"
            disabled={!project.context}
            onClick={() => setTab("technical")}
          >
            <FileText className="size-3.5" />
            {t(($) => $.hub.tab_technical)}
          </Button>
        </div>
      </div>

      {!path ? (
        <div className="flex flex-1 items-center justify-center text-muted-foreground">
          <p className="text-body">{t(($) => $.hub.no_changelog)}</p>
        </div>
      ) : isLoading ? (
        <div className="mx-auto w-full max-w-[68ch] flex-1 space-y-3 overflow-y-auto px-6 pt-7 sm:px-8">
          <Skeleton className="h-7 w-2/3" />
          <Skeleton className="h-4 w-full" />
          <Skeleton className="h-4 w-full" />
          <Skeleton className="h-4 w-5/6" />
        </div>
      ) : isError || !file ? (
        <div className="flex flex-1 items-center justify-center text-muted-foreground">
          <p className="text-body">{t(($) => $.hub.load_failed)}</p>
        </div>
      ) : tab === "overview" ? (
        <div className="flex-1 overflow-y-auto">
          <div className="mx-auto max-w-[68ch] space-y-6 px-6 pb-24 pt-7 sm:px-8">
            {project.summary && (
              <p className="text-title-sm leading-relaxed">{project.summary}</p>
            )}
            <div>
              <div className="mb-1 flex items-center justify-between">
                <h2 className="text-caption font-medium text-muted-foreground">
                  {t(($) => $.hub.recent_activity)}
                </h2>
                {changelogEntries.length > 5 && (
                  <Button variant="ghost" size="sm" className="gap-1" onClick={() => setTab("activity")}>
                    <History className="size-3.5" />
                    {t(($) => $.hub.view_all_activity)}
                  </Button>
                )}
              </div>
              {changelogEntries.length === 0 ? (
                <p className="text-body text-muted-foreground">{t(($) => $.hub.no_changelog)}</p>
              ) : (
                <HubChangelogTimeline entries={changelogEntries} limit={5} />
              )}
            </div>
            <Button
              variant="outline"
              size="sm"
              className="gap-1.5 text-muted-foreground"
              onClick={() => setTab("technical")}
            >
              <FileText className="size-3.5" />
              {t(($) => $.hub.tab_technical)}
            </Button>
          </div>
        </div>
      ) : tab === "activity" ? (
        <div className="flex-1 overflow-y-auto">
          <div className="mx-auto max-w-[68ch] px-6 pb-24 pt-7 sm:px-8">
            {changelogEntries.length === 0 ? (
              <p className="text-body text-muted-foreground">{t(($) => $.hub.no_changelog)}</p>
            ) : (
              <HubChangelogTimeline entries={changelogEntries} />
            )}
          </div>
        </div>
      ) : (
        <div className="flex min-h-0 flex-1">
          <nav className="hidden w-56 shrink-0 overflow-y-auto border-r py-4 pl-4 pr-2 md:block">
            <ul className="space-y-0.5">
              {technicalSections.map((section) => (
                <li key={section.id}>
                  <button
                    type="button"
                    onClick={() => scrollToSection(section.id)}
                    className="block w-full truncate rounded-md px-2 py-1.5 text-left text-caption text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
                    title={section.label}
                  >
                    {section.label}
                  </button>
                </li>
              ))}
            </ul>
          </nav>
          <div className="flex-1 overflow-y-auto">
            <div className="mx-auto max-w-[68ch] px-6 pb-24 pt-7 sm:px-8">
              {technicalSections.length === 0 ? (
                <RichContent content={file.content} density="document" phase="settled" />
              ) : (
                technicalSections.map((section, i) => (
                  <section
                    key={section.id}
                    id={`hub-section-${section.id}`}
                    className={i > 0 ? "mt-8 scroll-mt-4 border-t pt-8" : "scroll-mt-4"}
                  >
                    <h2 className="mb-3 font-heading text-title-sm font-semibold">
                      {section.label}
                    </h2>
                    <RichContent content={section.content} density="document" phase="settled" />
                  </section>
                ))
              )}
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
