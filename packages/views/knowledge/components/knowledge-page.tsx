"use client";

import { Tabs, TabsList, TabsTrigger, TabsContent } from "@multica/ui/components/ui/tabs";
import { PageHeader, PAGE_GUTTER } from "../../layout/page-header";
import { KnowledgeHubPanel } from "./knowledge-hub-panel";
import { IntelligenceSummaryPanel } from "./intelligence-summary-panel";
import { useT } from "../../i18n";

/**
 * One page, two tabs, per the owner's own framing: the knowledge hub
 * (narrative/decisions, one project per source repo — see
 * C:\Dev\Silver-Mediums-Knowledge) and Silver Mediums Intelligence (the
 * machine-readable data ledger on a separate host) are "literally all of my
 * company's knowledge and data truths" and belong read together, not as two
 * separate nav items the owner has to mentally connect himself. Two tabs
 * under one page header, not two pages — see the PR/commit description for
 * the fuller reasoning.
 */
export function KnowledgePage() {
  const { t } = useT("knowledge");

  return (
    <div className="flex h-full min-h-0 flex-col">
      <PageHeader>
        <h1 className="flex-1 text-body font-semibold">{t(($) => $.page.title)}</h1>
      </PageHeader>
      <Tabs defaultValue="hub" className="flex min-h-0 flex-1 flex-col">
        <TabsList className={`${PAGE_GUTTER} shrink-0 border-b bg-transparent pt-2`}>
          <TabsTrigger value="hub">{t(($) => $.page.tab_hub)}</TabsTrigger>
          <TabsTrigger value="intelligence">{t(($) => $.page.tab_intelligence)}</TabsTrigger>
        </TabsList>
        <TabsContent value="hub" className="min-h-0 flex-1">
          <KnowledgeHubPanel />
        </TabsContent>
        <TabsContent value="intelligence" className="min-h-0 flex-1">
          <IntelligenceSummaryPanel />
        </TabsContent>
      </Tabs>
    </div>
  );
}
