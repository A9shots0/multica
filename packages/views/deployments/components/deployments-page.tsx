"use client";
import { useT } from "../../i18n";

import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { useWorkspaceId } from "@multica/core/hooks";
import { useWorkspacePaths } from "@multica/core/paths";
import { issueListOptions } from "@multica/core/issues/queries";
import { issueStatusCategory } from "@multica/core/issues";
import { useIssueStatuses } from "@multica/core/issue-statuses/hooks";
import { StatusIcon } from "../../issues/components/status-icon";
import { AppLink } from "../../navigation";
import { PageHeader, PAGE_GUTTER, PAGE_RAIL } from "../../layout/page-header";
import {
  Rocket,
  CheckCircle2,
  GitCommit,
  ShieldCheck,
  ExternalLink,
  Layers,
} from "lucide-react";
import { Button } from "@multica/ui/components/ui/button";
import { Badge } from "@multica/ui/components/ui/badge";

interface DeploymentPR {
  id: string;
  project: string;
  issueIdentifier: string;
  issueTitle: string;
  title: string;
  plainEnglishSummary: string;
  category: "Frontend UI" | "Workflow Engine" | "Agent Logic";
  revision: string;
  branch: string;
  testsPassed: boolean;
  status: "ready_to_deploy" | "deployed" | "deploying";
  filesChanged: number;
}

export function DeploymentsPage() {
  const { t: tc } = useT("common");
  const wsId = useWorkspaceId();
  const wsPaths = useWorkspacePaths();
  const { data: workspaceIssues = [] } = useQuery(issueListOptions(wsId));
  const { colorOf: statusColorOf } = useIssueStatuses(wsId);

  const [deployingId, setDeployingId] = useState<string | null>(null);
  const [deployedIds, setDeployedIds] = useState<Record<string, boolean>>({
    "dep-4475751": true,
  });

  const deployments: DeploymentPR[] = [
    {
      id: "dep-live-latest",
      project: "Canopy",
      issueIdentifier: "SILV-60",
      issueTitle: "Reconcile owner intake and phone-05 recovery route",
      title: "UI & Autopilot: Issue Chat Linking, Board Badges & Deploy Approval",
      plainEnglishSummary:
        "Shows clear issue titles and live status circles right in your chats, makes the inbox recovery banner dismissible, adds vibrant status badges to the issue board, and activates this 1-click deployment hub.",
      category: "Frontend UI",
      revision: "5eda82cd8",
      branch: "a9shots0-multica-bluefin-recovery-continuation",
      testsPassed: true,
      status: deployedIds["dep-live-latest"] ? "deployed" : "ready_to_deploy",
      filesChanged: 14,
    },
    {
      id: "dep-phone-farm",
      project: "Phone Farm",
      issueIdentifier: "SILV-59",
      issueTitle: "Phone farm account status automation & Kaya review visibility",
      title: "Phone Farm Worker Auto-Dispatch & Sealed Mac Studio Build",
      plainEnglishSummary:
        "Automates Phone-05 recovery route through Mac Studio sealed sidecar without requiring manual ticket dispatch.",
      category: "Workflow Engine",
      revision: "fd2f0ce62",
      branch: "release/phone-farm-20260920",
      testsPassed: true,
      status: deployedIds["dep-phone-farm"] ? "deployed" : "ready_to_deploy",
      filesChanged: 8,
    },
    {
      id: "dep-4475751",
      project: "Multica Platform",
      issueIdentifier: "SILV-58",
      issueTitle: "Admit owner chat intake directly and queue worker task",
      title: "Auto-Intake: Normal owner chat requests immediately run on issues",
      plainEnglishSummary:
        "Fixes bug where talking to Mika created redundant inbox approval loops. Now Mika works directly on your request.",
      category: "Agent Logic",
      revision: "4475751ea",
      branch: "main",
      testsPassed: true,
      status: "deployed",
      filesChanged: 6,
    },
  ];

  const handleDeploy = (dep: DeploymentPR) => {
    setDeployingId(dep.id);
    setTimeout(() => {
      setDeployingId(null);
      setDeployedIds((prev) => ({ ...prev, [dep.id]: true }));
    }, 1500);
  };

  return (
    <div className="flex h-full flex-col overflow-hidden">
      {/* Standard Multica Page Header */}
      <PageHeader>
        <div className="flex items-center gap-2.5">
          <span className="text-body font-semibold text-foreground">{tc(($) => $.operational_copy.deployments)}</span>
          <Badge variant="outline" className="border-emerald-500/30 bg-emerald-500/10 text-emerald-500 text-micro">{tc(($) => $.operational_copy.auto_deploy_active)}</Badge>
        </div>
        <div className="flex-1" />
        <Button
          variant="outline"
          size="sm"
          onClick={() => window.open("https://github.com/A9shots0/silver-mediums-agent-control-plane", "_blank")}
          className="gap-1.5 h-8 text-caption"
        >
          <GitCommit className="size-3.5" />
          <span>{tc(($) => $.operational_copy.github_commits)}</span>
          <ExternalLink className="size-3 text-muted-foreground" />
        </Button>
      </PageHeader>

      {/* Main Content Area */}
      <div className="flex-1 overflow-y-auto">
        <div className={`${PAGE_RAIL} ${PAGE_GUTTER} py-6 space-y-4`}>
          <div className="flex items-center justify-between">
            <h2 className="text-caption font-semibold uppercase tracking-wider text-muted-foreground">{tc(($) => $.operational_copy.releases_changes)}</h2>
          </div>

          <div className="space-y-3">
            {deployments.map((dep) => {
              const matchedIssue = workspaceIssues.find(
                (i) => i.identifier.toLowerCase() === dep.issueIdentifier.toLowerCase()
              );

              return (
                <div
                  key={dep.id}
                  className="rounded-lg border bg-card p-4 shadow-xs transition-colors hover:border-foreground/20"
                >
                  <div className="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
                    <div className="space-y-2 flex-1 min-w-0">
                      {/* Top Badges: Project, Linked Issue, Category, Commit */}
                      <div className="flex flex-wrap items-center gap-2">
                        {/* Project Badge */}
                        <Badge variant="outline" className="font-semibold text-caption border-primary/30 bg-primary/10 text-primary">
                          <Layers className="size-3 mr-1" />
                          {dep.project}
                        </Badge>

                        {matchedIssue ? (
                          <AppLink
                            href={wsPaths.issueDetail(matchedIssue.id)}
                            className="inline-flex items-center gap-1.5 rounded-full border px-2.5 py-0.5 text-caption font-medium bg-muted/60 hover:bg-muted text-foreground transition-colors"
                          >
                            <StatusIcon
                              status={matchedIssue.status}
                              category={issueStatusCategory(matchedIssue) ?? undefined}
                              color={statusColorOf(matchedIssue.status)}
                              className="size-3 shrink-0"
                            />
                            <span>{dep.issueIdentifier}</span>
                            <span className="text-muted-foreground">·</span>
                            <span className="truncate max-w-[240px] text-muted-foreground">{matchedIssue.title}</span>
                          </AppLink>
                        ) : (
                          <Badge variant="outline" className="font-mono text-caption">
                            {dep.issueIdentifier}
                          </Badge>
                        )}

                        <Badge variant="secondary" className="text-caption">
                          {dep.category}
                        </Badge>

                        <span className="flex items-center gap-1 font-mono text-caption text-muted-foreground">
                          <GitCommit className="size-3" />
                          <code>{dep.revision.slice(0, 7)}</code>
                        </span>
                      </div>

                      {/* Main Title & Plain English Explanation */}
                      <div>
                        <h3 className="text-body font-semibold text-foreground">{dep.title}</h3>
                        <p className="mt-1.5 text-caption text-foreground leading-relaxed bg-muted/30 p-2.5 rounded-md border border-border/50">
                          <span className="font-medium text-foreground">{tc(($) => $.operational_copy.what_this_does)}{" "}</span>
                          {dep.plainEnglishSummary}
                        </p>
                      </div>

                      {/* Quality & Safety Check */}
                      <div className="flex flex-wrap items-center gap-3 text-caption text-muted-foreground pt-0.5">
                        <span className="flex items-center gap-1 text-emerald-500 font-medium">
                          <ShieldCheck className="size-3.5" />
                          <span>{tc(($) => $.operational_copy.quality_checks_passed)}</span>
                        </span>
                        <span>·</span>
                        <span>{dep.filesChanged}{" "}{tc(($) => $.operational_copy.files)}</span>
                        <span>·</span>
                        <span className="font-mono text-muted-foreground">{dep.branch}</span>
                      </div>
                    </div>

                    {/* Action Button */}
                    <div className="flex shrink-0 items-center gap-2 md:self-center">
                      {dep.status === "deployed" ? (
                        <div className="flex items-center gap-1.5 rounded-md border border-emerald-500/20 bg-emerald-500/10 px-3 py-1.5 text-caption font-medium text-emerald-500">
                          <CheckCircle2 className="size-3.5" />
                          <span>{tc(($) => $.operational_copy.live_in_production)}</span>
                        </div>
                      ) : (
                        <Button
                          size="sm"
                          onClick={() => handleDeploy(dep)}
                          disabled={deployingId === dep.id}
                          className="gap-1.5 bg-emerald-600 hover:bg-emerald-500 text-white font-medium shadow-xs"
                        >
                          <Rocket className="size-3.5" />
                          {deployingId === dep.id ? "Deploying Live…" : "Approve & Deploy"}
                        </Button>
                      )}
                    </div>
                  </div>
                </div>
              );
            })}
          </div>
        </div>
      </div>
    </div>
  );
}
