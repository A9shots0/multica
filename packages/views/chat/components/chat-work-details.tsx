"use client";

import { useQuery } from "@tanstack/react-query";
import { agentListOptions, memberListOptions } from "@multica/core/workspace/queries";
import { chatWorkDetails } from "@multica/core/chat/work-details";
import type { ChatWorkIssue } from "@multica/core/chat/work";
import { useT } from "../../i18n";

export function ChatWorkDetails({ issue, wsId }: { issue: ChatWorkIssue; wsId: string }) {
  const { t } = useT("chat");
  const details = chatWorkDetails(issue);
  const agents = useQuery({ ...agentListOptions(wsId), enabled: !!details });
  const members = useQuery({ ...memberListOptions(wsId), enabled: !!details });
  if (!details) return null;
  const actorName = (type?: string, id?: string) => {
    if (!id) return t(($) => $.work.actor_unknown);
    if (type === "agent") return agents.data?.find(agent => agent.id === id)?.name ?? t(($) => $.work.actor_unknown);
    return members.data?.find(member => member.user_id === id)?.name ?? t(($) => $.work.actor_unknown);
  };
  const steps: Record<string, string> = {
    received: t(($) => $.work.steps.received), ready: t(($) => $.work.steps.ready),
    working: t(($) => $.work.steps.working), needs_attention: t(($) => $.work.steps.needs_attention),
    blocked_external: t(($) => $.work.steps.blocked_external), waiting_automatic: t(($) => $.work.steps.waiting_automatic),
    suspended: t(($) => $.work.steps.suspended), cancelled: t(($) => $.work.steps.cancelled),
    superseded: t(($) => $.work.steps.superseded), failed: t(($) => $.work.steps.failed),
    succeeded: t(($) => $.work.steps.succeeded), satisfied: t(($) => $.work.steps.satisfied),
    reviewed: t(($) => $.work.steps.reviewed), releasing: t(($) => $.work.steps.releasing),
    deployed: t(($) => $.work.steps.deployed), deployed_checking: t(($) => $.work.steps.deployed),
    production_verifying: t(($) => $.work.steps.deployed), live: t(($) => $.work.steps.verified_live),
    verified_live: t(($) => $.work.steps.verified_live), degraded: t(($) => $.work.steps.regressed),
    regressed: t(($) => $.work.steps.regressed), rolling_back: t(($) => $.work.steps.rolling_back),
    rolled_back: t(($) => $.work.steps.rolled_back), restored: t(($) => $.work.steps.restored),
    technical_stop: t(($) => $.work.steps.failed),
  };
  const next: Record<string, string> = {
    provider_readback: t(($) => $.work.next.provider_readback),
    "task.completed": t(($) => $.work.next.task_completed),
    deploy_exact_release: t(($) => $.work.next.deploy),
  };
  return <div className="grid gap-2 text-caption sm:grid-cols-2">
    {details.milestone && <div className="rounded-md border bg-background/70 px-2.5 py-2 font-medium">{t(($) => $.work.current_step, { step: steps[details.milestone] ?? t(($) => $.work.details_unavailable) })}</div>}
    {details.actor?.actor_id && <div className="rounded-md border bg-background/70 px-2.5 py-2 text-muted-foreground">{t(($) => $.work.handled_by, { name: actorName(details.actor.actor_type, details.actor.actor_id) })}</div>}
    {details.waitReason && details.waitReason !== issue.wait_reason && !details.dependencies.some(dependency => dependency.reason === details.waitReason) && <div className="rounded-md border bg-background/70 px-2.5 py-2 whitespace-pre-wrap break-words text-muted-foreground">{details.waitReason}</div>}
    {details.nextEvent && <div className="rounded-md border bg-background/70 px-2.5 py-2 text-muted-foreground">{t(($) => $.work.next_step, { step: next[details.nextEvent] ?? t(($) => $.work.details_unavailable) })}</div>}
    {details.dependencies.length > 0 && <div className="rounded-md border bg-background/70 px-2.5 py-2 sm:col-span-2">
      <p className="font-medium text-foreground">{t(($) => $.work.dependencies)}</p>
      <ul className="mt-2 divide-y">
        {details.dependencies.map(dependency => <li key={dependency.id} className="break-words text-muted-foreground">
          <p className="py-2 first:pt-0 last:pb-0">{dependency.reason ?? t(($) => $.work.external_wait)}</p>
          {dependency.actorId && (dependency.actorId !== details.actor?.actor_id || dependency.actorType !== details.actor?.actor_type) && <p>{t(($) => $.work.handled_by, { name: actorName(dependency.actorType, dependency.actorId) })}</p>}
        </li>)}
      </ul>
    </div>}
  </div>;
}
