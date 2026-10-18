"use client";

import { useEffect, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { Checkbox } from "@multica/ui/components/ui/checkbox";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@multica/ui/components/ui/select";
import { agentListOptions, squadListOptions } from "@multica/core/workspace/queries";
import { runtimeListOptions } from "@multica/core/runtimes/queries";
import type { OwnerRuntimeRouteRequest } from "@multica/core/types";
import { useT } from "../../i18n";

const roles = ["implementation", "review", "release", "verification"] as const;
type Role = (typeof roles)[number];

// undefined opts out; null means an explicit route is incomplete and must
// never silently fall back to starting the issue's ordinary assignee.
export function OwnerRuntimeRouteFields({ workspaceId, onChange }: {
  workspaceId: string;
  onChange: (route: OwnerRuntimeRouteRequest | null | undefined) => void;
}) {
  const { t } = useT("inbox");
  const [enabled, setEnabled] = useState(false);
  const [squadId, setSquadId] = useState("");
  const [selected, setSelected] = useState<Record<Role, string>>({ implementation: "", review: "", release: "", verification: "" });
  const [interruption, setInterruption] = useState(false);
  const { data: agents = [] } = useQuery({ ...agentListOptions(workspaceId), enabled });
  const { data: squads = [] } = useQuery({ ...squadListOptions(workspaceId), enabled });
  const { data: runtimes = [] } = useQuery({ ...runtimeListOptions(workspaceId, "me"), enabled });
  const available = agents.filter((agent) => !agent.archived_at && runtimes.some((runtime) => runtime.id === agent.runtime_id && runtime.status === "online" && runtime.daemon_id));
  const chosen = roles.map((role) => available.find((agent) => agent.id === selected[role]));
  const hosts = chosen.map((agent) => runtimes.find((runtime) => runtime.id === agent?.runtime_id)?.daemon_id);
  const valid = !!squadId && chosen.every(Boolean) && !!hosts[0] && hosts.every((host) => host === hosts[0]) && selected.implementation !== selected.review && selected.release !== selected.verification;
  const bindingKey = chosen.map((agent) => `${agent?.id ?? ""}:${agent?.runtime_id ?? ""}`).join("|");
  const host = hosts[0] ?? "";
  useEffect(() => {
    if (!enabled) { onChange(undefined); return; }
    if (!valid) { onChange(null); return; }
    const bindings = Object.fromEntries(roles.map((role, index) => [role, { agent_id: selected[role], runtime_id: chosen[index]!.runtime_id }])) as Pick<OwnerRuntimeRouteRequest, Role>;
    onChange({ squad_id: squadId, daemon_id: host, ...bindings, allow_brief_deployment_interruption: interruption });
    // bindingKey captures exact selected runtime changes, including rebinds.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [enabled, valid, squadId, host, bindingKey, interruption, onChange]);
  const label = (role: Role) => {
    switch (role) {
      case "implementation": return t(($) => $.owner_route.implementation);
      case "review": return t(($) => $.owner_route.review);
      case "release": return t(($) => $.owner_route.release);
      case "verification": return t(($) => $.owner_route.verification);
    }
  };
  const agentItems = available.map((agent) => {
    const runtime = runtimes.find((entry) => entry.id === agent.runtime_id)!;
    return { value: agent.id, label: `${agent.name} · ${runtime.custom_name || runtime.name}` };
  });
  const squadItems = squads.filter((squad) => !squad.archived_at).map((squad) => ({ value: squad.id, label: squad.name }));
  return <section className="space-y-3 rounded-lg border p-4">
    <label className="flex items-center gap-2 text-body"><Checkbox checked={enabled} onCheckedChange={setEnabled} />{t(($) => $.owner_route.enable)}</label>
    {enabled && <>
      <p className="text-caption text-muted-foreground">{t(($) => $.owner_route.help)}</p>
      <div className="space-y-1"><span className="text-caption">{t(($) => $.owner_route.squad)}</span>
        <Select items={squadItems} value={squadId || null} onValueChange={(value) => setSquadId(value ?? "")}><SelectTrigger aria-label={t(($) => $.owner_route.squad)}><SelectValue placeholder={t(($) => $.owner_route.choose)} /></SelectTrigger><SelectContent>{squadItems.map((item) => <SelectItem key={item.value} value={item.value}>{item.label}</SelectItem>)}</SelectContent></Select>
      </div>
      {roles.map((role) => <div className="space-y-1" key={role}><span className="text-caption">{label(role)}</span>
        <Select items={agentItems} value={selected[role] || null} onValueChange={(value) => setSelected((current) => ({ ...current, [role]: value ?? "" }))}><SelectTrigger aria-label={label(role)}><SelectValue placeholder={t(($) => $.owner_route.choose)} /></SelectTrigger><SelectContent>{agentItems.map((item) => <SelectItem key={item.value} value={item.value}>{item.label}</SelectItem>)}</SelectContent></Select>
      </div>)}
      <label className="flex items-start gap-2 text-caption"><Checkbox checked={interruption} onCheckedChange={setInterruption} />{t(($) => $.owner_route.interruption)}</label>
      {!valid && <p className="text-caption text-muted-foreground">{t(($) => $.owner_route.incomplete)}</p>}
    </>}
  </section>;
}
