"use client";

import React, { useMemo, useState } from "react";
import { Plus } from "lucide-react";
import { Button } from "@multica/ui/components/ui/button";
import { Tooltip, TooltipTrigger, TooltipContent } from "@multica/ui/components/ui/tooltip";
import { ActorAvatar } from "../../common/actor-avatar";
import {
  PickerEmpty,
  PickerItem,
  PickerSection,
  PropertyPicker,
} from "../../issues/components/pickers/property-picker";
import { matchesPinyin } from "../../editor/extensions/pinyin-match";
import type { Agent } from "@multica/core/types";
import { isAgentRuntimeBound } from "@multica/core/agents";
import { isMikaAgent } from "@multica/core/onboarding";
import { toast } from "sonner";
import { useT } from "../../i18n";

/**
 * Agent picker: a searchable, grouped (My agents / Others) list of agents in a
 * PropertyPicker. The caller supplies the trigger. `currentAgentId` marks one
 * agent with a check — omit it (as "new chat" does) when there is no current
 * selection to highlight.
 */
export function AgentPicker({
  agents,
  userId,
  currentAgentId,
  onSelect,
  trigger,
  triggerRender,
  side = "bottom",
  align = "start",
}: {
  agents: Agent[];
  userId: string | undefined;
  currentAgentId?: string;
  onSelect: (agent: Agent) => void;
  trigger: React.ReactNode;
  triggerRender: React.ReactElement;
  side?: "top" | "bottom";
  align?: "start" | "center" | "end";
}) {
  const { t } = useT("chat");
  const [open, setOpen] = useState(false);
  const [filter, setFilter] = useState("");
  const { mine, others } = useMemo(() => {
    const mine: Agent[] = [];
    const others: Agent[] = [];
    for (const a of agents) {
      if (a.owner_id === userId) mine.push(a);
      else others.push(a);
    }
    return { mine, others };
  }, [agents, userId]);

  const query = filter.trim().toLowerCase();
  const matches = (name: string) =>
    !query || name.toLowerCase().includes(query) || matchesPinyin(name, query);
  const filteredMine = mine.filter((agent) => matches(agent.name));
  const filteredOthers = others.filter((agent) => matches(agent.name));

  const handlePick = (agent: Agent) => {
    onSelect(agent);
    setOpen(false);
  };

  return (
    <PropertyPicker
      open={open}
      onOpenChange={setOpen}
      width="w-64"
      align={align}
      side={side}
      searchable
      searchPlaceholder={t(($) => $.window.agent_filter_placeholder)}
      onSearchChange={setFilter}
      triggerRender={triggerRender}
      trigger={trigger}
    >
      {filteredMine.length === 0 && filteredOthers.length === 0 ? (
        <PickerEmpty />
      ) : (
        <>
          {filteredMine.length > 0 && (
            <PickerSection label={t(($) => $.window.my_agents)}>
              {filteredMine.map((agent) => (
                <AgentPickerItem
                  key={agent.id}
                  agent={agent}
                  isCurrent={agent.id === currentAgentId}
                  onSelect={handlePick}
                />
              ))}
            </PickerSection>
          )}
          {filteredOthers.length > 0 && (
            <PickerSection label={t(($) => $.window.others)}>
              {filteredOthers.map((agent) => (
                <AgentPickerItem
                  key={agent.id}
                  agent={agent}
                  isCurrent={agent.id === currentAgentId}
                  onSelect={handlePick}
                />
              ))}
            </PickerSection>
          )}
        </>
      )}
    </PropertyPicker>
  );
}

function AgentPickerItem({
  agent,
  isCurrent,
  onSelect,
}: {
  agent: Agent;
  isCurrent: boolean;
  onSelect: (agent: Agent) => void;
}) {
  const { t } = useT("chat");
  const runtimeBound = isAgentRuntimeBound(agent);
  return (
    <PickerItem
      selected={isCurrent}
      disabled={!runtimeBound}
      tooltip={
        runtimeBound ? undefined : t(($) => $.window.agent_needs_runtime_hint)
      }
      onClick={() => onSelect(agent)}
    >
      <ActorAvatar
        actorType="agent"
        actorId={agent.id}
        size="md"
        enableHoverCard
        showStatusDot
      />
      <span className="truncate flex-1">{agent.name}</span>
      {!runtimeBound && (
        <span className="shrink-0 text-micro text-amber-600 dark:text-amber-400">
          {t(($) => $.window.agent_needs_runtime)}
        </span>
      )}
    </PickerItem>
  );
}

/**
 * A "New chat" trigger that starts immediately with a fixed agent — no
 * picker. Shared by the single-agent shortcut and the Mika default below,
 * which differ only in which agent that is and how it's found.
 */
function QuickStartButton({
  agent,
  label,
  side,
  onStart,
}: {
  agent: Agent | null;
  label: string;
  side: "top" | "bottom";
  onStart: (agent: Agent | null) => void;
}) {
  const { t } = useT("chat");
  return (
    <Tooltip>
      <TooltipTrigger
        render={
          <Button
            variant="ghost"
            size="icon-sm"
            className="rounded-full text-muted-foreground"
            aria-label={label}
            onClick={() => {
              if (agent && !isAgentRuntimeBound(agent)) {
                toast.error(t(($) => $.input.runtime_required_toast));
                return;
              }
              onStart(agent);
            }}
          />
        }
      >
        <Plus />
      </TooltipTrigger>
      <TooltipContent side={side === "top" ? "top" : "bottom"}>{label}</TooltipContent>
    </Tooltip>
  );
}

/**
 * "New chat" ⊕ button. Starts a chat with Mika directly, with no agent
 * picker: Mika is the workspace's Chief of Staff and the coordinator every
 * other agent works through, so the owner should not have to answer "who do
 * you want to talk to?" on every new thread. Going straight to a specialist
 * is still one tap away, just not through this button — pin it in the
 * `QuickAgentBar` above the list and tap its avatar to start that chat.
 * Falls back to the old shortcuts (single agent starts immediately; zero or
 * multiple non-Mika agents show the full picker) for a workspace with no
 * Mika.
 */
export function NewChatButton({
  agents,
  userId,
  onStart,
  side = "bottom",
}: {
  agents: Agent[];
  userId: string | undefined;
  onStart: (agent: Agent | null) => void;
  side?: "top" | "bottom";
}) {
  const { t } = useT("chat");
  const label = t(($) => $.window.new_chat_tooltip);

  const mika = agents.find(isMikaAgent);
  if (mika) {
    return <QuickStartButton agent={mika} label={label} side={side} onStart={onStart} />;
  }

  if (agents.length <= 1) {
    return (
      <QuickStartButton agent={agents[0] ?? null} label={label} side={side} onStart={onStart} />
    );
  }

  return (
    <AgentPicker
      agents={agents}
      userId={userId}
      onSelect={(agent) => onStart(agent)}
      side={side}
      align="start"
      triggerRender={
        <Button
          variant="ghost"
          size="icon-sm"
          className="rounded-full text-muted-foreground"
          aria-label={label}
        />
      }
      trigger={<Plus />}
    />
  );
}
