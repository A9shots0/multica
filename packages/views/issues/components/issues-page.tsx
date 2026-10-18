"use client";

import { Building2, ListTodo } from "lucide-react";
import type {
  Issue,
  IssueTableFacetSpec,
  IssueTableFacetsResponse,
  WorkingAgentSummary,
} from "@multica/core/types";
import { useIssuesScope } from "@multica/core/issues/stores/issues-scope-store";
import { useHqStore } from "@multica/core/issues/stores/hq-store";
import { useViewStore } from "@multica/core/issues/stores/view-store-context";
import { Button } from "@multica/ui/components/ui/button";
import { PageHeader } from "../../layout/page-header";
import { useT } from "../../i18n";
import { IssueSurface } from "../surface/issue-surface";
import { IssuesHeader } from "./issues-header";
import { CompanyHq } from "./company-hq";

function IssuesSurfaceHeader({
  issues,
  workingAgents,
  isRefreshing,
  facetCountsExact,
  tableFacetCounts,
  onTableFacetChange,
}: {
  issues: Issue[];
  workingAgents: WorkingAgentSummary[] | undefined;
  isRefreshing: boolean;
  facetCountsExact: boolean;
  tableFacetCounts?: IssueTableFacetsResponse;
  onTableFacetChange: (facet: IssueTableFacetSpec | null) => void;
}) {
  const dateFilter = useViewStore((s) => s.dateFilter);
  const setDateFilter = useViewStore((s) => s.setDateFilter);

  return (
    <IssuesHeader
      scopedIssues={issues}
      workingAgents={workingAgents}
      dateFilter={dateFilter}
      onDateFilterChange={setDateFilter}
      isRefreshing={isRefreshing}
      facetCountsExact={facetCountsExact}
      tableFacetCounts={tableFacetCounts}
      onTableFacetChange={onTableFacetChange}
    />
  );
}

export function IssuesPage() {
  const { t } = useT("issues");
  const scope = useIssuesScope("issues");
  const mode = useHqStore((state) => state.mode);
  const setMode = useHqStore((state) => state.setMode);

  return (
    <div className="flex flex-1 min-h-0 flex-col">
      <PageHeader>
        {mode === "hq" ? <Building2 className="h-4 w-4 text-muted-foreground" /> : <ListTodo className="h-4 w-4 text-muted-foreground" />}
        <h1 className="flex-1 text-body font-medium">{mode === "hq" ? t(($) => $.hq.title) : t(($) => $.page.breadcrumb_title)}</h1>
        <Button variant="ghost" size="sm" onClick={() => setMode(mode === "hq" ? "classic" : "hq")}>
          {mode === "hq" ? t(($) => $.hq.classic_views) : t(($) => $.hq.back_to_hq)}
        </Button>
      </PageHeader>

      {mode === "hq" ? (
        <CompanyHq />
      ) : (
        <IssueSurface
          scope={{ type: "workspace", actorKind: scope }}
          modes={["board", "list", "table", "swimlane"]}
          batchToolbar="list"
          renderHeader={({ controller }) => (
            <IssuesSurfaceHeader
              issues={controller.surfaceIssues}
              workingAgents={controller.workingAgents}
              isRefreshing={controller.isRefreshing}
              facetCountsExact={controller.facetCountsExact}
              tableFacetCounts={controller.tableFacetCounts}
              onTableFacetChange={controller.setActiveTableFacet}
            />
          )}
          renderEmpty={() => (
            <div className="flex flex-1 min-h-0 flex-col items-center justify-center gap-2 text-muted-foreground">
              <ListTodo className="h-10 w-10 text-faint-foreground" />
              <p className="text-body">{t(($) => $.page.empty_title)}</p>
              <p className="text-caption">{t(($) => $.page.empty_hint)}</p>
            </div>
          )}
        />
      )}
    </div>
  );
}
