import { describe, expect, it } from "vitest";
import {
  AppConfigSchema,
  WecomInstallationSchema,
  ListWecomInstallationsResponseSchema,
  RedeemWecomBindingTokenResponseSchema,
  EMPTY_WECOM_INSTALLATION,
  EMPTY_LIST_WECOM_INSTALLATIONS_RESPONSE,
  EMPTY_REDEEM_WECOM_BINDING_TOKEN_RESPONSE,
  TelegramInstallationSchema,
  ListTelegramInstallationsResponseSchema,
  RedeemTelegramBindingTokenResponseSchema,
  EMPTY_TELEGRAM_INSTALLATION,
  EMPTY_LIST_TELEGRAM_INSTALLATIONS_RESPONSE,
  EMPTY_REDEEM_TELEGRAM_BINDING_TOKEN_RESPONSE,
  AgentTaskListSchema,
  AutopilotQuotaUsageSchema,
  AutopilotRunSchema,
  FALLBACK_AUTOPILOT_RUN,
  CommentTriggerPreviewSchema,
  DashboardAgentRunTimeListSchema,
  DashboardRunTimeDailyListSchema,
  DashboardFailureByAgentListSchema,
  DashboardFailureDailyListSchema,
  DashboardUsageByAgentListSchema,
  DashboardUsageDailyListSchema,
  ChatDraftRestoresResponseSchema,
  ChatPendingTaskSchema,
  ChatSessionListSchema,
  ChatSessionSchema,
  PrioritizeQueuedChatTaskResponseSchema,
  CreateFeedbackResponseSchema,
  DuplicateIssueErrorBodySchema,
  EMPTY_CHAT_DRAFT_RESTORES,
  EMPTY_CHAT_PENDING_TASK,
  EMPTY_CHAT_SESSION,
  EMPTY_PRIORITIZE_QUEUED_CHAT_TASK_RESPONSE,
  EMPTY_CREATE_FEEDBACK_RESPONSE,
  EMPTY_INBOX_ITEMS,
  EMPTY_INBOX_UNREAD_SUMMARY,
  EMPTY_SEARCH_PROJECTS_RESPONSE,
  EMPTY_USER,
  InboxItemListSchema,
  InboxUnreadSummarySchema,
  IssueTriggerPreviewSchema,
  ListIssuesResponseSchema,
  EMPTY_LIST_ISSUES_RESPONSE,
  IssueCurrentPictureSchema,
  OwnerIssueSummarySchema,
  OwnerDirectiveReceiptSchema,
  WorkflowDecisionPacketSchema,
  WorkflowDecisionResponseReceiptSchema,
  WorkflowDecisionStaleConflictSchema,
  ListPropertiesResponseSchema,
  MALFORMED_RUNTIME_MODEL_LIST_REQUEST,
  RuntimeModelListRequestSchema,
  SearchProjectsResponseSchema,
  RuntimeHourlyActivityListSchema,
  RuntimeUsageByAgentListSchema,
  RuntimeUsageByHourListSchema,
  RuntimeUsageListSchema,
  SendChatMessageResponseSchema,
  SquadListSchema,
  SquadSchema,
  SourceContextPreviewSchema,
  TimelineEntriesSchema,
  UserSchema,
  PluginInstallationSchema,
  PluginInstallationListResponseSchema,
  PluginMCPToolListSchema,
  PluginPreviewSchema,
  EMPTY_PLUGIN_INSTALLATION_LIST,
  EMPTY_PLUGIN_PREVIEW,
} from "./schemas";
import { IssueViewSchema, IssueViewListSchema } from "./schemas";
import {
  ListIssueStatusesResponseSchema,
  IssueStatusEntrySchema,
  EMPTY_LIST_ISSUE_STATUSES_RESPONSE,
  EMPTY_ISSUE_STATUS_ENTRY,
} from "./schemas";
import { parseWithFallback } from "./schema";

const baseIssue = {
  id: "11111111-1111-1111-1111-111111111111",
  workspace_id: "ws-1",
  number: 1,
  identifier: "MUL-1",
  title: "Test",
  description: null,
  status: "todo",
  priority: "medium",
  assignee_type: null,
  assignee_id: null,
  creator_type: "member",
  creator_id: "user-1",
  parent_issue_id: null,
  project_id: null,
  position: 0,
  stage: null,
  start_date: null,
  due_date: null,
  metadata: {},
  created_at: "2026-01-01T00:00:00Z",
  updated_at: "2026-01-01T00:00:00Z",
};

const workflowDecisionPacket = {
  contract_version: 1 as const,
  issue_id: "issue-1",
  decision: {
    id: "decision-1",
    revision: 1,
    type: "authorization",
    state: "unanswered",
    resolution_state: "open",
    question: "Deploy the reviewed candidate?",
    recommendation: "Approve the exact reviewed candidate.",
    consequence: "Production remains unchanged until this is answered.",
    options: ["approve", "reject"],
    scope_revision: 3,
    authority_revision: 2,
    candidate_id: "candidate-1",
    operation_id: "operation-1",
    branch_id: "main",
  },
  attention: {
    id: "attention-1",
    revision: 1,
    read_state: "unread",
    acknowledged: false,
    resolution_state: "open",
  },
  continuation: null,
  replacement: null,
};

describe("normalized workflow decision schemas", () => {
  it("accepts atomic authorization without custom prose options", () => {
    expect(WorkflowDecisionPacketSchema.safeParse({
      ...workflowDecisionPacket,
      decision: { ...workflowDecisionPacket.decision, options: [] },
    }).success).toBe(true);
  });
  it("parses a complete decision packet and preserves future fields and states", () => {
    const parsed = WorkflowDecisionPacketSchema.parse({
      ...workflowDecisionPacket,
      future_packet_field: "keep",
      decision: {
        ...workflowDecisionPacket.decision,
        state: "future_state",
        future_decision_field: "keep",
      },
    });

    expect(parsed.decision.state).toBe("future_state");
    expect((parsed as unknown as { future_packet_field: string }).future_packet_field).toBe("keep");
    expect((parsed.decision as unknown as { future_decision_field: string }).future_decision_field).toBe("keep");
  });

  it.each([
    ["decision id", { ...workflowDecisionPacket, decision: { ...workflowDecisionPacket.decision, id: "" } }],
    ["decision revision", { ...workflowDecisionPacket, decision: { ...workflowDecisionPacket.decision, revision: 0 } }],
    ["authority binding", { ...workflowDecisionPacket, decision: { ...workflowDecisionPacket.decision, candidate_id: undefined } }],
    ["attention identity", { ...workflowDecisionPacket, attention: { ...workflowDecisionPacket.attention, id: "" } }],
    ["contract version", { ...workflowDecisionPacket, contract_version: 2 }],
  ])("rejects a packet with a missing or invalid critical %s", (_name, value) => {
    expect(WorkflowDecisionPacketSchema.safeParse(value).success).toBe(false);
  });

  it("requires every durable response receipt binding", () => {
    const receipt = {
      decision_id: "decision-1",
      decision_revision: 2,
      response_id: "response-1",
      response_type: "approve",
      resolution_state: "resuming",
      action_receipt_id: "action-1",
      action_state: "admitted",
      command_id: "command-1",
      current_picture_revision: 14,
      future_receipt_field: "keep",
    };

    expect(WorkflowDecisionResponseReceiptSchema.parse(receipt)).toMatchObject(receipt);
    expect(WorkflowDecisionResponseReceiptSchema.parse({
      ...receipt,
      response_type: "future_outcome",
    }).response_type).toBe("future_outcome");
    expect(
      WorkflowDecisionResponseReceiptSchema.safeParse({
        ...receipt,
        action_receipt_id: undefined,
      }).success,
    ).toBe(false);
  });

  it("validates every owner directive receipt binding", () => {
    const receipt = {
      id: "directive-1",
      issue_id: "issue-1",
      revision: 2,
      directive: "change_scope",
      state: "applied",
      comment_id: "comment-1",
      issue_revision: 3,
    };

    expect(OwnerDirectiveReceiptSchema.parse(receipt)).toMatchObject(receipt);
    expect(OwnerDirectiveReceiptSchema.safeParse({ ...receipt, issue_revision: 0 }).success).toBe(false);
    expect(OwnerDirectiveReceiptSchema.safeParse({ ...receipt, directive: "approve" }).success).toBe(false);
  });

  it("parses a structured stale conflict with its authoritative replacement", () => {
    const conflict = WorkflowDecisionStaleConflictSchema.parse({
      code: "decision_stale",
      error: "This decision is no longer current.",
      submitted_decision_id: "decision-old",
      submitted_revision: 1,
      current_decision: workflowDecisionPacket,
      future_conflict_field: true,
    });

    expect(conflict.current_decision?.decision.id).toBe("decision-1");
    expect((conflict as unknown as { future_conflict_field: boolean }).future_conflict_field).toBe(true);
    expect(WorkflowDecisionStaleConflictSchema.safeParse({ ...conflict, code: "conflict" }).success).toBe(false);
    expect(WorkflowDecisionStaleConflictSchema.safeParse({
      ...conflict,
      current_decision: null,
      terminal_state: undefined,
    }).success).toBe(false);
    expect(WorkflowDecisionStaleConflictSchema.safeParse({
      ...conflict,
      current_decision: null,
      terminal_state: "resolved",
    }).success).toBe(true);
  });
});

describe("ChatSessionSchema", () => {
  const baseSession = {
    id: "chat-1",
    workspace_id: "ws-1",
    agent_id: "agent-1",
    creator_id: "user-1",
    title: "Channel chat",
    status: "active",
    has_unread: false,
    created_at: "2026-08-18T00:00:00Z",
    updated_at: "2026-08-18T00:00:00Z",
  };

  it("accepts channel route metadata and leaves it absent for first-party Chats", () => {
    expect(ChatSessionSchema.parse(baseSession).channel_source).toBeUndefined();

    const parsed = ChatSessionSchema.parse({
      ...baseSession,
      channel_source: {
        channel_type: "slack",
        installation_id: "installation-1",
        route_revision: 3,
      },
      is_current_channel_route: false,
    });
    expect(parsed.channel_source).toEqual({
      channel_type: "slack",
      installation_id: "installation-1",
      route_revision: 3,
    });
    expect(parsed.is_current_channel_route).toBe(false);
  });

  it("degrades malformed session metadata without dropping the Chat", () => {
    const parsed = ChatSessionSchema.parse({
      ...baseSession,
      channel_source: { channel_type: 42 },
      is_current_channel_route: "yes",
    });
    expect(parsed.channel_source).toBeUndefined();
    expect(parsed.is_current_channel_route).toBeUndefined();
    expect(parsed.id).toBe("chat-1");

    expect(parseWithFallback({ id: 42 }, ChatSessionSchema, EMPTY_CHAT_SESSION, {
      endpoint: "GET /api/chat/sessions/:id",
    })).toEqual(EMPTY_CHAT_SESSION);
  });

  it("drops one malformed list item without hiding the other Chats", () => {
    const parsed = ChatSessionListSchema.parse([
      baseSession,
      { ...baseSession, id: 42 },
      {
        ...baseSession,
        id: "onboarding-chat",
        last_message: {
          content: "Welcome",
          role: "assistant",
          created_at: "2026-08-18T00:00:00Z",
          message_kind: "onboarding_opening",
        },
      },
    ]);

    expect(parsed.map((session) => session.id)).toEqual(["chat-1", "onboarding-chat"]);
    expect(parsed[1]?.last_message?.message_kind).toBe("onboarding_opening");
  });
});
describe("OwnerIssueSummarySchema", () => {
  const ref = { type: "decision", id: "decision-1", revision: 2 };
  const change = { code: "decision_requested" as const, result_code: "started" as const, occurred_at: "2026-09-10T00:00:00Z", reference: ref };
  const fixtures = [
    {
      name: "minimum",
      value: { schema_version: 1, current_picture_revision: 1, stage: "incoming", signals: [], next_action: null, latest_meaningful_change: null },
    },
    {
      name: "typical",
      value: {
        schema_version: 1, current_picture_revision: 7, stage: "needs_owner",
        signals: [{ code: "owner_attention_required", references: [ref] }],
        next_action: { kind: "answer_decision", automatic: false, actor_type: "owner" }, latest_meaningful_change: change,
      },
    },
    {
      name: "maximum",
      value: {
        schema_version: 1, current_picture_revision: 99, stage: "recovering",
        signals: [
          { code: "current_incident", references: [{ type: "incident", id: "incident-1", revision: 4 }] },
          { code: "owner_attention_required", references: [ref] },
          { code: "recovery_active", references: [{ type: "release", id: "release-2", revision: 8 }] },
          { code: "current_regression", references: [{ type: "release", id: "release-2", revision: 8 }] },
        ],
        next_action: { kind: "answer_decision", automatic: false, actor_type: "owner" },
        latest_meaningful_change: { code: "incident_changed", result_code: "started", occurred_at: "2026-09-10T00:00:00Z", reference: { type: "incident", id: "incident-1", revision: 4 } },
      },
    },
    {
      name: "technical stop",
      value: {
        schema_version: 1, current_picture_revision: 8, stage: "technical_stop",
        signals: [
          { code: "technical_stop", references: [{ type: "execution", id: "task-1", revision: 1 }] },
          { code: "current_incident", references: [{ type: "execution", id: "task-1", revision: 1 }] },
          { code: "owner_attention_required", references: [{ type: "execution", id: "task-1", revision: 1 }] },
        ],
        next_action: { kind: "inspect_incident", automatic: false, actor_type: "owner" },
        latest_meaningful_change: { code: "execution_changed", result_code: "failed", occurred_at: "2026-09-10T00:00:00Z", reference: { type: "execution", id: "task-1", revision: 1 } },
      },
    },
  ] as const;

  it.each(fixtures)("accepts the $name fixture", ({ value }) => {
    expect(OwnerIssueSummarySchema.parse(value)).toEqual(value);
  });

  it.each([
    ["unknown stage", { ...fixtures[0].value, stage: "future" }],
    ["unknown signal", { ...fixtures[0].value, signals: [{ code: "future", references: [] }] }],
    ["unknown field", { ...fixtures[0].value, client_stage: "active" }],
    ["complete without proof signal", { ...fixtures[0].value, stage: "complete" }],
    ["unavailable with promised action", { ...fixtures[0].value, stage: "status_unavailable", signals: [{ code: "projection_unavailable", references: [] }], next_action: { kind: "retry", automatic: true } }],
    ["complete with technical stop", { ...fixtures[0].value, stage: "complete", signals: [{ code: "outcome_complete", references: [] }, { code: "outcome_done", references: [] }, { code: "technical_stop", references: [] }] }],
    ["active with owner attention", { ...fixtures[0].value, stage: "active", signals: [{ code: "work_active", references: [] }, { code: "owner_attention_required", references: [] }] }],
    ["technical stop without inspection action", { ...fixtures[3].value, next_action: null }],
    ["technical stop with automatic inspection", { ...fixtures[3].value, next_action: { kind: "inspect_incident", automatic: true, actor_type: "owner" } }],
    ["duplicate signal", { ...fixtures[0].value, stage: "active", signals: [{ code: "work_active", references: [] }, { code: "work_active", references: [] }] }],
    ["unknown meaningful change code", { ...fixtures[0].value, latest_meaningful_change: { ...change, code: "future_change" } }],
    ["invalid meaningful change time", { ...fixtures[0].value, latest_meaningful_change: { ...change, occurred_at: "not-a-time" } }],
  ])("rejects %s", (_name, value) => {
    expect(OwnerIssueSummarySchema.safeParse(value).success).toBe(false);
  });
});

describe("IssueSchema (via ListIssuesResponseSchema)", () => {
  const compactPicture = {
    contract_version: 2 as const,
    revision: 7,
    owner_summary: {
      schema_version: 1 as const,
      current_picture_revision: 7,
      stage: "status_unavailable" as const,
      signals: [{ code: "projection_unavailable" as const, references: [] }],
      next_action: null,
      latest_meaningful_change: null,
    },
    scope_revision: 2,
    authority_revision: 3,
    outcome_state: "future_outcome",
    outcome_open: false,
    execution_state: "future_execution",
    release_state: "future_release",
    health_state: "future_health",
    attention_state: "future_attention",
    current_milestone: "reviewing",
    latest_meaningful_change: { kind: "review" },
    next_automatic_action: { kind: "dispatch" },
    active_actor: { id: "agent-1" },
    wait_reason: {},
    candidate: {},
    target: {},
    evidence_summary: {},
    unresolved_decisions: [],
    unresolved_incidents: [],
    unresolved_dependencies: [],
    terminal_reason: "",
    next_actor: {},
    generated_at: "2026-09-10T00:00:00Z",
  };

  const detailPicture = {
    ...compactPicture,
    workspace_id: "ws-1",
    issue_id: "issue-1",
    authority_summary: {},
    budget_summary: [],
    supersession_summary: {},
    source_revisions: [],
    document: {
      contract_version: 2 as const,
      original_business_request: {},
      outcome: { state: "future_outcome" },
      historical_completion: {},
      scope: { revision: 2 },
      authority: { revision: 3 },
      execution: { state: "future_execution" },
      active_branches: [],
      primary_branch: {},
      release: { state: "future_release" },
      health: "future_health",
      attention: { resolution_state: "future_attention" },
      current_milestone: "reviewing",
      latest_meaningful_change: { kind: "review" },
      next_automatic_action: { kind: "dispatch" },
      active_actor: { id: "agent-1" },
      wait_reason: {},
      candidate: {},
      target: {},
      evidence_summary: {},
      authority_summary: {},
      unresolved_decisions: [],
      unresolved_incidents: [],
      unresolved_dependencies: [],
      budgets: [],
      supersession: {},
      terminal_reason: "",
      next_actor: {},
    },
    updated_at: "2026-09-10T00:00:00Z",
  };

  const passedGate = {
    state: "passed" as const,
    refs: ["receipt-1"],
    observed_at: "2026-09-10T00:00:00Z",
    limitations: [],
  };
  const missingGate = {
    state: "missing" as const,
    refs: [],
    observed_at: null,
    limitations: [],
  };
  const verifiedLiveStatus = {
    schema_version: 1 as const,
    phase: "verified_live" as const,
    binding: {
      release_id: "release-1",
      release_revision: 7,
      candidate_id: "candidate-1",
      candidate_revision: 2,
      source_revision: "source-sha",
      artifact_digest: "sha256:artifact",
      build_manifest_digest: "sha256:build-contract",
      scope_revision: 2,
      acceptance_version: 4,
      authority_revision: 3,
      target_key: "production",
      target_generation: 9,
      deployed_revision: "deployed-sha",
    },
    gates: {
      review: passedGate,
      build: passedGate,
      deployment: passedGate,
      exposure: passedGate,
      runtime: passedGate,
      business: passedGate,
      regression: passedGate,
      rollback: missingGate,
      restoration: missingGate,
    },
    historical: {
      was_verified_live: true,
      last_verified_live_at: "2026-09-10T00:00:00Z",
    },
    next_action: null,
    stop: null,
  };
  const compactPictureV3 = {
    ...compactPicture,
    contract_version: 3 as const,
    outcome_state: "satisfied",
    outcome_open: false,
    release_state: "live",
    health_state: "healthy",
    current_milestone: "verified_live",
    release_status: verifiedLiveStatus,
    owner_summary: {
      schema_version: 1 as const,
      current_picture_revision: 7,
      stage: "complete" as const,
      signals: [
        { code: "outcome_complete" as const, references: [{ type: "outcome", id: "outcome-1", revision: 1 }] },
        { code: "verified_live" as const, references: [{ type: "release", id: "release-1", revision: 7 }] },
      ],
      next_action: null,
      latest_meaningful_change: { code: "evidence_recorded", result_code: "succeeded", occurred_at: "2026-09-10T00:00:00Z", reference: { type: "evidence", id: "receipt-1", revision: 1 } },
    },
  };
  const detailPictureV3 = {
    ...detailPicture,
    ...compactPictureV3,
    document: {
      ...detailPicture.document,
      contract_version: 3 as const,
      outcome: { state: "satisfied" },
      scope: { revision: 2, requires_deployment: true },
      release: { state: "live" },
      health: "healthy",
      current_milestone: "verified_live",
      release_status: verifiedLiveStatus,
    },
  };

  it("preserves unknown future Current Picture states", () => {
    const parsed = ListIssuesResponseSchema.parse({
      issues: [{ ...baseIssue, current_picture: compactPicture }],
      total: 1,
    });
    expect(parsed.issues[0]?.current_picture?.outcome_state).toBe("future_outcome");
    expect(parsed.issues[0]?.current_picture?.execution_state).toBe("future_execution");
  });

  it("drops only malformed optional Current Picture enrichment", () => {
    const parsed = ListIssuesResponseSchema.parse({
      issues: [{ ...baseIssue, current_picture: { ...compactPicture, unresolved_decisions: {} } }],
      total: 1,
    });
    expect(parsed.issues).toHaveLength(1);
    expect(parsed.issues[0]?.id).toBe(baseIssue.id);
    expect(parsed.issues[0]?.current_picture).toBeUndefined();
  });

  it("requires the complete versioned Current Picture detail contract", () => {
    expect(IssueCurrentPictureSchema.safeParse(detailPicture).success).toBe(true);
    expect(IssueCurrentPictureSchema.safeParse({ ...compactPicture, workspace_id: "ws-1" }).success).toBe(false);
  });

  it("accepts v3 release truth while v2 never invents a release phase", () => {
    const v2 = IssueCurrentPictureSchema.parse(detailPicture);
    expect("release_status" in v2).toBe(false);

    const v3 = IssueCurrentPictureSchema.parse({
      ...detailPictureV3,
      release_status: { ...verifiedLiveStatus, future_server_field: true },
      document: {
        ...detailPictureV3.document,
        release_status: { ...verifiedLiveStatus, future_server_field: true },
      },
    });
    expect(v3.contract_version).toBe(3);
    if (v3.contract_version === 3) {
      expect(v3.release_status.phase).toBe("verified_live");
      expect(v3.release_status.future_server_field).toBe(true);
    }
  });

  it.each(["review", "build", "deployment", "exposure", "runtime", "business", "regression"] as const)(
    "rejects verified_live when the %s gate is not passed",
    (gate) => {
      const releaseStatus = {
        ...verifiedLiveStatus,
        gates: { ...verifiedLiveStatus.gates, [gate]: missingGate },
      };
      expect(IssueCurrentPictureSchema.safeParse({
        ...detailPictureV3,
        release_status: releaseStatus,
        document: { ...detailPictureV3.document, release_status: releaseStatus },
      }).success).toBe(false);
    },
  );

  it("rejects verified_live without an exact deployed revision", () => {
    const releaseStatus = {
      ...verifiedLiveStatus,
      binding: { ...verifiedLiveStatus.binding, deployed_revision: null },
    };
    expect(IssueCurrentPictureSchema.safeParse({
      ...detailPictureV3,
      release_status: releaseStatus,
      document: { ...detailPictureV3.document, release_status: releaseStatus },
    }).success).toBe(false);
  });

  it("rejects disagreement between top-level and document release truth", () => {
    expect(IssueCurrentPictureSchema.safeParse({
      ...detailPictureV3,
      document: {
        ...detailPictureV3.document,
        release_status: { ...verifiedLiveStatus, phase: "deployed_checking" },
      },
    }).success).toBe(false);
  });

  it.each([
    ["technical_stop", {
      ...verifiedLiveStatus,
      phase: "technical_stop",
      stop: { code: "runtime_unavailable", reason: "No usable runtime", retryable: true },
    }],
    ["rolled_back", {
      ...verifiedLiveStatus,
      phase: "rolled_back",
      gates: { ...verifiedLiveStatus.gates, rollback: passedGate, restoration: passedGate },
    }],
  ])("rejects %s when legacy release_state still claims live", (phase, releaseStatus) => {
    expect(IssueCurrentPictureSchema.safeParse({
      ...detailPictureV3,
      current_milestone: phase,
      release_status: releaseStatus,
      document: {
        ...detailPictureV3.document,
        current_milestone: phase,
        release_status: releaseStatus,
      },
    }).success).toBe(false);
  });

  it("rejects a v3 milestone that disagrees with the normalized release phase", () => {
    expect(IssueCurrentPictureSchema.safeParse({
      ...detailPictureV3,
      current_milestone: "live",
      document: { ...detailPictureV3.document, current_milestone: "live" },
    }).success).toBe(false);
  });

  it("rejects verified_live without coherent verified history", () => {
    const releaseStatus = {
      ...verifiedLiveStatus,
      historical: { was_verified_live: false, last_verified_live_at: null },
    };
    expect(IssueCurrentPictureSchema.safeParse({
      ...detailPictureV3,
      release_status: releaseStatus,
      document: { ...detailPictureV3.document, release_status: releaseStatus },
    }).success).toBe(false);
  });

  it("rejects verified_live backed only by an older historical timestamp", () => {
    const releaseStatus = {
      ...verifiedLiveStatus,
      historical: { ...verifiedLiveStatus.historical, last_verified_live_at: "2026-09-09T00:00:00Z" },
    };
    expect(IssueCurrentPictureSchema.safeParse({
      ...detailPictureV3,
      release_status: releaseStatus,
      document: { ...detailPictureV3.document, release_status: releaseStatus },
    }).success).toBe(false);
  });

  it("rejects a technical stop payload on a non-technical phase", () => {
    const releaseStatus = {
      ...verifiedLiveStatus,
      stop: { code: "contradiction", reason: "Conflicting evidence", retryable: false },
    };
    expect(IssueCurrentPictureSchema.safeParse({
      ...detailPictureV3,
      release_status: releaseStatus,
      document: { ...detailPictureV3.document, release_status: releaseStatus },
    }).success).toBe(false);
  });

  it("rejects untrusted v3 release truth smuggled into a v2 detail", () => {
    expect(IssueCurrentPictureSchema.safeParse({
      ...detailPicture,
      release_status: verifiedLiveStatus,
    }).success).toBe(false);
    expect(IssueCurrentPictureSchema.safeParse({
      ...detailPicture,
      document: { ...detailPicture.document, release_status: verifiedLiveStatus },
    }).success).toBe(false);
  });

  it("keeps an authoritative issue row when optional v3 release truth is malformed", () => {
    const parsed = ListIssuesResponseSchema.parse({
      issues: [{
        ...baseIssue,
        current_picture: {
          ...compactPictureV3,
          release_status: { ...verifiedLiveStatus, phase: "future_phase" },
        },
      }],
      total: 1,
    });
    expect(parsed.issues).toHaveLength(1);
    expect(parsed.issues[0]?.id).toBe(baseIssue.id);
    expect(parsed.issues[0]?.current_picture).toBeUndefined();
  });

  it("drops contradictory compact v3 release truth without hiding the issue row", () => {
    const parsed = ListIssuesResponseSchema.parse({
      issues: [{
        ...baseIssue,
        current_picture: {
          ...compactPictureV3,
          // This status is internally valid, but it cannot coexist with the
          // compact projection's legacy live/verified_live claims.
          release_status: {
            ...verifiedLiveStatus,
            phase: "technical_stop",
            stop: { code: "runtime_unavailable", reason: "No usable runtime", retryable: true },
          },
        },
      }],
      total: 1,
    });
    expect(parsed.issues).toHaveLength(1);
    expect(parsed.issues[0]?.id).toBe(baseIssue.id);
    expect(parsed.issues[0]?.current_picture).toBeUndefined();
  });

  it("keeps an authoritative issue row when v2 enrichment smuggles release truth", () => {
    const parsed = ListIssuesResponseSchema.parse({
      issues: [{
        ...baseIssue,
        current_picture: { ...compactPicture, release_status: verifiedLiveStatus },
      }],
      total: 1,
    });
    expect(parsed.issues).toHaveLength(1);
    expect(parsed.issues[0]?.id).toBe(baseIssue.id);
    expect(parsed.issues[0]?.current_picture).toBeUndefined();
  });

  it.each([
    ["evidence array", { ...detailPicture, evidence_summary: [] }],
    ["source revisions null", { ...detailPicture, source_revisions: null }],
    ["minimal document", { ...detailPicture, document: { contract_version: 2 } }],
    ["contradictory denormalized state", {
      ...detailPicture,
      document: { ...detailPicture.document, outcome: { state: "completed" } },
    }],
  ])("rejects semantic Current Picture malformation: %s", (_name, value) => {
    expect(IssueCurrentPictureSchema.safeParse(value).success).toBe(false);
  });

  // A custom status key can be derived rather than readable — "客户确认" becomes
  // `in_review_2` — so the display name travels with it. The field has to
  // survive a server that predates it, since an issue that fails validation
  // degrades to a stub rather than losing one field. (MUL-6749)
  it("carries a custom status's display name", () => {
    const parsed = ListIssuesResponseSchema.parse({
      issues: [{ ...baseIssue, status: "in_review_2", status_name: "客户确认" }],
      total: 1,
    });
    expect(parsed.issues[0]?.status_name).toBe("客户确认");
  });
  it("drops only a malformed status_name, keeping the issue and the list", () => {
    for (const bad of [42, { name: "x" }, ["x"], true]) {
      const parsed = ListIssuesResponseSchema.parse({
        issues: [{ ...baseIssue, status: "in_review_2", status_name: bad }],
        total: 1,
      });
      expect(parsed.issues).toHaveLength(1);
      expect(parsed.issues[0]?.id).toBe(baseIssue.id);
      expect(parsed.issues[0]?.status).toBe("in_review_2");
      expect(parsed.issues[0]?.status_name).toBeUndefined();
    }
  });
  it("still parses an issue from a server that does not send status_name", () => {
    const { status_name: _omitted, ...withoutName } = { ...baseIssue, status_name: "x" };
    const parsed = ListIssuesResponseSchema.parse({ issues: [withoutName], total: 1 });
    expect(parsed.issues[0]?.id).toBe(baseIssue.id);
    expect(parsed.issues[0]?.status_name).toBeUndefined();
  });
  it("keeps the issue while independently dropping a malformed source context", () => {
    const parsed = ListIssuesResponseSchema.parse({
      issues: [{ ...baseIssue, source_context: { snapshot: "bad" } }],
      total: 1,
    });
    expect(parsed.issues[0]?.id).toBe(baseIssue.id);
    expect(parsed.issues[0]?.source_context).toBeUndefined();
  });
  it("parses source-context change reasons without requiring them from older servers", () => {
    const sourceContext = {
      id: "context-1",
      version: 1,
      usage: "read_only_historical_background",
      captured_at: "2026-08-21T12:00:00Z",
      display_state: "changed",
      source_issue_state: "changed",
      comment_thread_state: "unchanged",
      anchor_comment_state: "available",
      can_open_current_source: true,
      change_reasons: ["issue_description_attachments"],
      change_details: {
        changed_comment_ids: ["comment-1"],
        added_comments: [{
          id: "comment-2", parent_id: "comment-1", type: "comment", content: "new reply",
          author: { type: "member", id: "user-2", name: "Bob" },
          created_at: "later", updated_at: "later", revision: 1, attachments: [],
        }],
        removed_comment_ids: ["comment-3"],
        description_attachment_changes: [{
          kind: "removed", attachment_id: "attachment-1", filename: "old.txt",
        }],
      },
      snapshot: {
        source_issue: {
          id: "issue-1", identifier: "MUL-1", number: 1, title: "Source",
          description: null, created_at: "now", updated_at: "now", revision: 1,
          attachments: [],
        },
        comment_thread: [{
          id: "comment-1", parent_id: null, type: "comment", content: "history",
          author: { type: "member", id: "user-1", name: "Alice" },
          created_at: "now", updated_at: "now", revision: 1, attachments: [],
        }],
        anchor_comment_id: "comment-1",
      },
    };
    const parsed = ListIssuesResponseSchema.parse({
      issues: [{ ...baseIssue, source_context: sourceContext }],
      total: 1,
    });
    expect(parsed.issues[0]?.source_context?.change_reasons).toEqual(["issue_description_attachments"]);
    expect(parsed.issues[0]?.source_context?.change_details).toEqual(sourceContext.change_details);

    const { change_reasons: _reasonsOmitted, change_details: _detailsOmitted, ...legacyContext } = sourceContext;
    const legacy = ListIssuesResponseSchema.parse({
      issues: [{ ...baseIssue, source_context: legacyContext }],
      total: 1,
    });
    expect(legacy.issues[0]?.source_context?.change_reasons).toBeUndefined();
    expect(legacy.issues[0]?.source_context?.change_details).toBeUndefined();

    const malformed = ListIssuesResponseSchema.parse({
      issues: [{
        ...baseIssue,
        source_context: {
          ...sourceContext,
          change_details: { ...sourceContext.change_details, added_comments: [{ id: 42 }] },
        },
      }],
      total: 1,
    });
    expect(malformed.issues[0]?.source_context).toBeUndefined();
  });
  it("accepts null activity during backfill and rejects malformed activity", () => {
    const parsed = ListIssuesResponseSchema.parse({
      issues: [{ ...baseIssue, last_activity_at: null }],
      total: 1,
    });
    expect(parsed.issues[0]?.last_activity_at).toBeNull();
    expect(() =>
      ListIssuesResponseSchema.parse({
        issues: [{ ...baseIssue, last_activity_at: 42 }],
        total: 1,
      }),
    ).toThrow();
  });

  it("preserves durable owner questions and all issue rows across optional projection versions", () => {
    const question = { id: "question-1", task_id: "task-1", question: "Which contract?", consequence: "The answer determines the next step." };
    const parsed = ListIssuesResponseSchema.parse({ issues: [
      { ...baseIssue, owner_question: question },
      { ...baseIssue, id: "other", owner_question: { id: 4 } },
      { ...baseIssue, id: "older" },
    ], total: 3 });
    expect(parsed.issues).toHaveLength(3);
    expect(parsed.issues[0]?.owner_question).toEqual(question);
    expect(parsed.issues[1]?.owner_question).toBeUndefined();
    expect(parsed.issues[2]?.owner_question).toBeUndefined();
  });

  it("accepts a primitive metadata KV map", () => {
    const payload = {
      issues: [
        {
          ...baseIssue,
          metadata: { pipeline_status: "waiting", pr_number: 3, is_blocked: true },
        },
      ],
      total: 1,
    };
    const parsed = ListIssuesResponseSchema.parse(payload);
    expect(parsed.issues[0]?.metadata).toEqual({
      pipeline_status: "waiting",
      pr_number: 3,
      is_blocked: true,
    });
  });

  it("defaults metadata to {} when the server omits it (older backend)", () => {
    const { metadata: _omit, ...issueWithoutMetadata } = baseIssue;
    const payload = { issues: [issueWithoutMetadata], total: 1 };
    const parsed = ListIssuesResponseSchema.parse(payload);
    expect(parsed.issues[0]?.metadata).toEqual({});
  });

  it("preserves every issue and supported owner-wait metadata when additive structured metadata arrives", () => {
    const supported = {
      "recovery.wait_kind": "owner_approval",
      "recovery.wait_reason": "Approve the existing candidate?",
      "recovery.next_action": "Resume the same issue after approval.",
      pr_number: 7,
      is_blocked: true,
    };
    const payload = {
      issues: [baseIssue, {
        ...baseIssue,
        id: "22222222-2222-2222-2222-222222222222",
        status: "blocked",
        metadata: {
          ...supported,
          terminal_task_failure_wait: { task_id: "task-1", reason: "worker stopped" },
          future_list: ["new bookkeeping"],
          future_null: null,
        },
      }],
      total: 2,
    };
    const parsed = parseWithFallback(payload, ListIssuesResponseSchema, EMPTY_LIST_ISSUES_RESPONSE, { endpoint: "/api/issues" });
    expect(parsed.issues.map((item) => item.id)).toEqual(payload.issues.map((item) => item.id));
    expect(parsed.total).toBe(2);
    expect(parsed.issues[1]?.metadata).toEqual(supported);
    expect(parsed.issues[1]?.status).toBe("blocked");
  });

  it("accepts a numeric stage", () => {
    const payload = { issues: [{ ...baseIssue, stage: 2 }], total: 1 };
    const parsed = ListIssuesResponseSchema.parse(payload);
    expect(parsed.issues[0]?.stage).toBe(2);
  });

  it("defaults stage to null when the server omits it (older backend)", () => {
    const { stage: _omit, ...issueWithoutStage } = baseIssue;
    const payload = { issues: [issueWithoutStage], total: 1 };
    const parsed = ListIssuesResponseSchema.parse(payload);
    expect(parsed.issues[0]?.stage).toBeNull();
  });

  it("accepts custom property values including multi_select arrays", () => {
    const payload = {
      issues: [
        {
          ...baseIssue,
          properties: { "def-1": "opt-a", "def-2": ["opt-x", "opt-y"], "def-3": 3.5, "def-4": true },
        },
      ],
      total: 1,
    };
    const parsed = ListIssuesResponseSchema.parse(payload);
    expect(parsed.issues[0]?.properties).toEqual({
      "def-1": "opt-a",
      "def-2": ["opt-x", "opt-y"],
      "def-3": 3.5,
      "def-4": true,
    });
  });

  it("defaults properties to {} when the server omits it (older backend)", () => {
    const parsed = ListIssuesResponseSchema.parse({ issues: [baseIssue], total: 1 });
    expect(parsed.issues[0]?.properties).toEqual({});
  });

  it("drops unknown-shaped property values instead of failing the issue parse", () => {
    // Forward compat: a future server type (actor/relation) may ship object
    // values. That one entry must disappear; the issue and its other
    // properties must survive — a full parse failure would blank the whole
    // list through parseWithFallback on installed desktop builds.
    const payload = {
      issues: [
        {
          ...baseIssue,
          properties: { "def-1": { nested: 1 }, "def-2": "opt-a" },
        },
      ],
      total: 1,
    };
    const parsed = ListIssuesResponseSchema.parse(payload);
    expect(parsed.issues[0]?.properties).toEqual({ "def-2": "opt-a" });
  });
});

describe("SourceContextPreviewSchema", () => {
  it("parses a thread-history preview and rejects a missing token", () => {
    const preview = {
      source_issue: {
        id: "issue-1", identifier: "MUL-1", number: 1, title: "Source",
        description: null, created_at: "now", updated_at: "now", revision: 1,
        attachments: [],
      },
      comment_thread: [{
        id: "comment-1", parent_id: null, type: "comment", content: "history",
        author: { type: "member", id: "user-1", name: "Alice" },
        created_at: "now", updated_at: "now", revision: 1, attachments: [],
      }],
      anchor_comment_id: "comment-1",
      capture_token: "sha256:token",
      limits: { comment_count: 1, text_bytes: 7, attachment_count: 0, attachment_bytes: 0 },
    };
    expect(SourceContextPreviewSchema.parse(preview).comment_thread).toHaveLength(1);
    expect(() => SourceContextPreviewSchema.parse({ ...preview, capture_token: "" })).toThrow();
  });

  it("normalizes null attachment lists from early source-context servers", () => {
    const preview = {
      source_issue: {
        id: "issue-1", identifier: "MUL-1", number: 1, title: "Source",
        description: null, created_at: "now", updated_at: "now", revision: 1,
        attachments: null,
      },
      comment_thread: [{
        id: "comment-1", parent_id: null, type: "comment", content: "history",
        author: { type: "member", id: "user-1", name: "Alice" },
        created_at: "now", updated_at: "now", revision: 1, attachments: null,
      }],
      anchor_comment_id: "comment-1",
      capture_token: "sha256:token",
      limits: { comment_count: 1, text_bytes: 7, attachment_count: 0, attachment_bytes: 0 },
    };

    const parsed = SourceContextPreviewSchema.parse(preview);
    expect(parsed.source_issue.attachments).toEqual([]);
    expect(parsed.comment_thread[0]?.attachments).toEqual([]);
  });
});

describe("IssuePropertySchema (via ListPropertiesResponseSchema)", () => {
  const baseProperty = {
    id: "22222222-2222-2222-2222-222222222222",
    workspace_id: "ws-1",
    name: "Severity",
    type: "select",
    description: "",
    icon: "flag",
    config: { options: [{ id: "opt-1", name: "Critical", color: "#ef4444" }] },
    position: 1,
    archived: false,
    archived_at: null,
    usage_count: 2,
    created_at: "2026-01-01T00:00:00Z",
    updated_at: "2026-01-01T00:00:00Z",
  };

  it("parses a full definition", () => {
    const parsed = ListPropertiesResponseSchema.parse({ properties: [baseProperty], total: 1 });
    expect(parsed.properties[0]?.config.options?.[0]?.name).toBe("Critical");
    expect(parsed.properties[0]?.icon).toBe("flag");
  });

  it("survives a malformed response by defaulting the list", () => {
    const parsed = ListPropertiesResponseSchema.parse({});
    expect(parsed.properties).toEqual([]);
    expect(parsed.total).toBe(0);
  });

  it("keeps unknown property types as strings (forward compat)", () => {
    const parsed = ListPropertiesResponseSchema.parse({
      properties: [{ ...baseProperty, type: "relation", config: {} }],
      total: 1,
    });
    expect(parsed.properties[0]?.type).toBe("relation");
  });

  it("defaults config when the server sends none", () => {
    const { config: _omit, ...withoutConfig } = baseProperty;
    const parsed = ListPropertiesResponseSchema.parse({ properties: [withoutConfig], total: 1 });
    expect(parsed.properties[0]?.config).toEqual({});
  });

  it("defaults icon for an older server response", () => {
    const { icon: _omit, ...withoutIcon } = baseProperty;
    const parsed = ListPropertiesResponseSchema.parse({ properties: [withoutIcon], total: 1 });
    expect(parsed.properties[0]?.icon).toBe("");
  });
});

// POST /api/issues/preview-trigger feeds this schema through parseWithFallback
// in client.previewIssueTrigger with fallback { triggers: [], total_count: 0 }
// (MUL-3375). The four entry points read it to decide "will this start a run",
// so malformed / missing / null drift must degrade to "nothing will start"
// rather than throw into the picker/modal.
const PREVIEW_FALLBACK = { triggers: [], total_count: 0 };
const PREVIEW_ENDPOINT = { endpoint: "POST /api/issues/preview-trigger" };

describe("IssueTriggerPreviewSchema", () => {
  it("parses a well-formed response", () => {
    const parsed = IssueTriggerPreviewSchema.parse({
      triggers: [
        { issue_id: "i1", agent_id: "a1", source: "assign" },
        { issue_id: "i2", agent_id: "a2", source: "status" },
      ],
      total_count: 2,
    });
    expect(parsed.total_count).toBe(2);
    expect(parsed.triggers).toHaveLength(2);
    expect(parsed.triggers[0]).toMatchObject({ issue_id: "i1", agent_id: "a1", source: "assign" });
  });

  it("defaults missing top-level fields (empty / older backend)", () => {
    const parsed = IssueTriggerPreviewSchema.parse({});
    expect(parsed.triggers).toEqual([]);
    expect(parsed.total_count).toBe(0);
  });

  it("defaults missing optional item fields, keeping required issue_id", () => {
    const parsed = IssueTriggerPreviewSchema.parse({ triggers: [{ issue_id: "i1" }], total_count: 1 });
    expect(parsed.triggers[0]).toEqual({
      issue_id: "i1",
      agent_id: "",
      source: "",
    });
  });

  it("parseWithFallback returns the fallback for a malformed shape (triggers not an array)", () => {
    const parsed = parseWithFallback(
      { triggers: "nope", total_count: 1 },
      IssueTriggerPreviewSchema,
      PREVIEW_FALLBACK,
      PREVIEW_ENDPOINT,
    );
    expect(parsed).toEqual(PREVIEW_FALLBACK);
  });

  it("parseWithFallback returns the fallback when an item drops the required issue_id", () => {
    const parsed = parseWithFallback(
      { triggers: [{ agent_id: "a1", source: "assign" }], total_count: 1 },
      IssueTriggerPreviewSchema,
      PREVIEW_FALLBACK,
      PREVIEW_ENDPOINT,
    );
    expect(parsed).toEqual(PREVIEW_FALLBACK);
  });

  it("parseWithFallback returns the fallback for a wrong-typed total_count", () => {
    const parsed = parseWithFallback(
      { triggers: [], total_count: "5" },
      IssueTriggerPreviewSchema,
      PREVIEW_FALLBACK,
      PREVIEW_ENDPOINT,
    );
    expect(parsed).toEqual(PREVIEW_FALLBACK);
  });

  it("parseWithFallback returns the fallback for null / non-object bodies", () => {
    expect(parseWithFallback(null, IssueTriggerPreviewSchema, PREVIEW_FALLBACK, PREVIEW_ENDPOINT)).toEqual(PREVIEW_FALLBACK);
    expect(parseWithFallback("oops", IssueTriggerPreviewSchema, PREVIEW_FALLBACK, PREVIEW_ENDPOINT)).toEqual(PREVIEW_FALLBACK);
  });
});

describe("TimelineEntriesSchema", () => {
  it("preserves source_task_id for agent failure comments", () => {
    const parsed = TimelineEntriesSchema.parse([
      {
        type: "comment",
        id: "comment-1",
        actor_type: "agent",
        actor_id: "agent-1",
        created_at: "2026-01-01T00:00:00Z",
        content: "API Error: 500 Internal server error",
        comment_type: "system",
        source_task_id: "task-1",
      },
    ]);

    expect(parsed[0]?.source_task_id).toBe("task-1");
  });

  it("preserves server-hydrated historical actor identity", () => {
    const parsed = TimelineEntriesSchema.parse([
      {
        type: "comment",
        id: "comment-1",
        actor_type: "member",
        actor_id: "former-user-1",
        actor_name: "Former Member",
        actor_avatar_url: "https://profiles.example.com/former.png",
        created_at: "2026-01-01T00:00:00Z",
        content: "Authored before leaving",
      },
    ]);

    expect(parsed[0]?.actor_name).toBe("Former Member");
    expect(parsed[0]?.actor_avatar_url).toBe(
      "https://profiles.example.com/former.png",
    );
  });
});

describe("AgentTaskListSchema", () => {
  it.each([true, false, undefined, null, "true", 1])("safely parses comment cancellation metadata: %s", (value) => {
    const parsed = AgentTaskListSchema.parse([{ id: "run", cancelled_by_comment_change: value }]);
    expect(parsed).toHaveLength(1);
    expect(parsed[0]?.cancelled_by_comment_change).toBe(typeof value === "boolean" ? value : undefined);
  });
  const task = {
    id: "task-1",
    agent_id: "agent-1",
    runtime_id: "runtime-1",
    issue_id: "issue-1",
    status: "queued",
    priority: 0,
    dispatched_at: null,
    started_at: null,
    completed_at: null,
    result: null,
    error: null,
    created_at: "2026-07-10T00:00:00Z",
    trigger_comment_id: "comment-3",
  };

  it("preserves planned and delivered comment IDs for a task run", () => {
    const parsed = AgentTaskListSchema.parse([
      {
        ...task,
        coalesced_comment_ids: ["comment-1", "comment-2"],
        delivered_comment_ids: ["comment-1", "comment-2", "comment-3"],
      },
    ]);

    expect(parsed[0]?.trigger_comment_id).toBe("comment-3");
    expect(parsed[0]?.coalesced_comment_ids).toEqual([
      "comment-1",
      "comment-2",
    ]);
    expect(parsed[0]?.delivered_comment_ids).toEqual([
      "comment-1",
      "comment-2",
      "comment-3",
    ]);
  });

  it("accepts task payloads from older backends without comment coverage", () => {
    const parsed = AgentTaskListSchema.parse([task]);
    expect(parsed[0]?.coalesced_comment_ids).toBeUndefined();
    expect(parsed[0]?.delivered_comment_ids).toBeUndefined();
  });

  it("degrades malformed optional coverage without dropping task rows", () => {
    const parsed = AgentTaskListSchema.parse([
      {
        ...task,
        coalesced_comment_ids: ["comment-1", 2],
        delivered_comment_ids: "not-an-array",
      },
      {
        ...task,
        id: "task-2",
        delivered_comment_ids: ["comment-2", "comment-3"],
      },
    ]);

    expect(parsed).toHaveLength(2);
    expect(parsed[0]?.coalesced_comment_ids).toBeUndefined();
    expect(parsed[0]?.delivered_comment_ids).toBeUndefined();
    expect(parsed[1]?.delivered_comment_ids).toEqual([
      "comment-2",
      "comment-3",
    ]);
  });

  it("accepts durable workdir metadata from newer backends", () => {
    const parsed = AgentTaskListSchema.parse([
      {
        ...task,
        status: "completed",
        work_dir: "/managed/task/worktree",
        durable_work_dir: "/Users/dev/project",
        relative_durable_work_dir: "project",
        branch_name: "agent/j/abc12345",
      },
    ]);

    expect(parsed[0]).toMatchObject({
      durable_work_dir: "/Users/dev/project",
      relative_durable_work_dir: "project",
      branch_name: "agent/j/abc12345",
    });
  });

  it("degrades malformed optional path metadata without dropping task rows", () => {
    const parsed = AgentTaskListSchema.parse([
      {
        ...task,
        work_dir: 1,
        durable_work_dir: { path: "/project" },
        relative_durable_work_dir: false,
        branch_name: ["agent/j/abc12345"],
      },
    ]);

    expect(parsed).toHaveLength(1);
    expect(parsed[0]?.work_dir).toBeUndefined();
    expect(parsed[0]?.durable_work_dir).toBeUndefined();
    expect(parsed[0]?.relative_durable_work_dir).toBeUndefined();
    expect(parsed[0]?.branch_name).toBeUndefined();
  });
});

describe("ChatDraftRestoresResponseSchema", () => {
  it("parses a well-formed response with attachments", () => {
    const parsed = parseWithFallback(
      {
        restores: [
          {
            id: "msg-1",
            chat_session_id: "s-1",
            task_id: "t-1",
            content: "run the thing",
            attachments: [{ id: "att-1", filename: "notes.txt" }],
            created_at: "2026-07-01T00:00:00Z",
          },
        ],
      },
      ChatDraftRestoresResponseSchema,
      EMPTY_CHAT_DRAFT_RESTORES,
      { endpoint: "test" },
    );
    expect(parsed.restores).toHaveLength(1);
    expect(parsed.restores[0]?.content).toBe("run the thing");
    expect(parsed.restores[0]?.attachments?.[0]?.id).toBe("att-1");
  });

  it("defaults a missing restores array instead of crashing the composer", () => {
    const parsed = parseWithFallback(
      {},
      ChatDraftRestoresResponseSchema,
      EMPTY_CHAT_DRAFT_RESTORES,
      { endpoint: "test" },
    );
    expect(parsed.restores).toEqual([]);
  });

  it("falls back to the empty response on a malformed row", () => {
    // A row without the consume key (id) is unusable — the whole response
    // falls back and the durable rows simply stay pending server-side.
    const parsed = parseWithFallback(
      { restores: [{ chat_session_id: "s-1", content: 42 }] },
      ChatDraftRestoresResponseSchema,
      EMPTY_CHAT_DRAFT_RESTORES,
      { endpoint: "test" },
    );
    expect(parsed).toEqual(EMPTY_CHAT_DRAFT_RESTORES);
  });
});

describe("ChatPendingTaskSchema", () => {
  const ENDPOINT = { endpoint: "GET /api/chat/sessions/:id/pending-task" };

  it("keeps legacy responses compatible when queued_tasks is absent", () => {
    const parsed = parseWithFallback(
      {
        task_id: "task-active",
        status: "running",
        created_at: "2026-07-01T00:00:00Z",
      },
      ChatPendingTaskSchema,
      EMPTY_CHAT_PENDING_TASK,
      ENDPOINT,
    );

    expect(parsed).toMatchObject({
      task_id: "task-active",
      status: "running",
    });
    expect(parsed.queued_tasks).toBeUndefined();
  });

  it("parses queued task summaries", () => {
    const parsed = ChatPendingTaskSchema.parse({
      task_id: "task-active",
      status: "running",
      queued_tasks: [
        {
          task_id: "task-queued",
          status: "queued",
          content: "Follow up after the current task",
          created_at: "2026-07-01T00:01:00Z",
        },
      ],
    });

    expect(parsed.queued_tasks).toEqual([
      expect.objectContaining({
        task_id: "task-queued",
        content: "Follow up after the current task",
      }),
    ]);
  });

  it("keeps a valid head and ignores only malformed queued rows", () => {
    const parsed = parseWithFallback(
      {
        task_id: "task-active",
        queued_tasks: [{ task_id: 42, status: "queued" }],
      },
      ChatPendingTaskSchema,
      EMPTY_CHAT_PENDING_TASK,
      ENDPOINT,
    );

    expect(parsed).toEqual({
      task_id: "task-active",
      queued_tasks: [],
    });
  });
});

describe("SendChatMessageResponseSchema", () => {
  const base = {
    message_id: "message-1",
    task_id: "task-1",
    created_at: "2026-08-05T00:00:00Z",
  };

  it("parses the server-authoritative queue position", () => {
    expect(SendChatMessageResponseSchema.parse({ ...base, queued: false }).queued).toBe(false);
  });

  it("ignores a malformed additive queue position without losing the accepted send", () => {
    expect(SendChatMessageResponseSchema.parse({ ...base, queued: "no" }).queued).toBeUndefined();
  });
});

describe("PrioritizeQueuedChatTaskResponseSchema", () => {
  const ENDPOINT = {
    endpoint: "POST /api/chat/sessions/:id/queued-tasks/:taskId/prioritize",
  };

  it("parses the prioritized task id", () => {
    expect(
      PrioritizeQueuedChatTaskResponseSchema.parse({
        task_id: "task-queued",
        active_task_id: "task-active",
      }),
    ).toEqual({
      task_id: "task-queued",
      active_task_id: "task-active",
    });
  });

  it("falls back when task_id is malformed", () => {
    expect(
      parseWithFallback(
        { task_id: 42 },
        PrioritizeQueuedChatTaskResponseSchema,
        EMPTY_PRIORITIZE_QUEUED_CHAT_TASK_RESPONSE,
        ENDPOINT,
      ),
    ).toBe(EMPTY_PRIORITIZE_QUEUED_CHAT_TASK_RESPONSE);
  });
});

describe("CreateFeedbackResponseSchema", () => {
  const ENDPOINT = { endpoint: "POST /api/feedback" };

  it("parses a well-formed response and preserves extra fields", () => {
    const parsed = parseWithFallback(
      { id: "feedback-1", created_at: "2026-06-26T00:00:00Z", future_field: true },
      CreateFeedbackResponseSchema,
      EMPTY_CREATE_FEEDBACK_RESPONSE,
      ENDPOINT,
    );
    expect(parsed).toMatchObject({
      id: "feedback-1",
      created_at: "2026-06-26T00:00:00Z",
      future_field: true,
    });
  });

  it("returns the empty fallback for malformed feedback responses", () => {
    expect(
      parseWithFallback(
        { id: 123, created_at: "2026-06-26T00:00:00Z" },
        CreateFeedbackResponseSchema,
        EMPTY_CREATE_FEEDBACK_RESPONSE,
        ENDPOINT,
      ),
    ).toBe(EMPTY_CREATE_FEEDBACK_RESPONSE);
    expect(
      parseWithFallback(null, CreateFeedbackResponseSchema, EMPTY_CREATE_FEEDBACK_RESPONSE, ENDPOINT),
    ).toBe(EMPTY_CREATE_FEEDBACK_RESPONSE);
  });
});

// The duplicate-issue branch in create-issue.tsx feeds ApiError.body
// (typed as `unknown`) through this schema. Any future server drift that
// loses the contract MUST fail the parse so the UI falls back to a normal
// error toast instead of rendering an empty / partial duplicate card.
describe("DuplicateIssueErrorBodySchema", () => {
  const valid = {
    code: "active_duplicate_issue",
    error: "An active issue with this title already exists: MUL-12 – Login bug",
    issue: {
      id: "11111111-1111-1111-1111-111111111111",
      identifier: "MUL-12",
      title: "Login bug",
    },
  };

  it("accepts a well-formed body", () => {
    expect(DuplicateIssueErrorBodySchema.safeParse(valid).success).toBe(true);
  });

  it("accepts unknown extra fields via .loose()", () => {
    const forwardCompat = {
      ...valid,
      hint: "Try a different title",
      issue: { ...valid.issue, workspace_id: "ws-1", status: "todo" },
    };
    expect(DuplicateIssueErrorBodySchema.safeParse(forwardCompat).success).toBe(true);
  });

  it("rejects a renamed code (so renames degrade to the generic toast)", () => {
    const renamed = { ...valid, code: "duplicate_issue" };
    expect(DuplicateIssueErrorBodySchema.safeParse(renamed).success).toBe(false);
  });

  it("rejects a missing issue object", () => {
    const { issue: _omit, ...without } = valid;
    expect(DuplicateIssueErrorBodySchema.safeParse(without).success).toBe(false);
  });

  it("rejects a non-string issue.id", () => {
    const broken = { ...valid, issue: { ...valid.issue, id: 42 } };
    expect(DuplicateIssueErrorBodySchema.safeParse(broken).success).toBe(false);
  });

  it("accepts a missing error field (it is optional)", () => {
    const { error: _omit, ...without } = valid;
    expect(DuplicateIssueErrorBodySchema.safeParse(without).success).toBe(true);
  });
});

// `user.timezone` (Viewing tz) was added in the timezone-architecture RFC.
// A desktop build older than the server — or a server predating the
// `user.timezone` migration — will return a `/api/me` body with no
// `timezone` key. The schema must not fail closed on that: the field
// defaults to `null`, which the frontend resolves to the browser-detected
// tz at render time.
describe("UserSchema timezone drift", () => {
  const base = {
    id: "11111111-1111-1111-1111-111111111111",
    name: "Ada",
    email: "ada@example.com",
  };

  it("defaults timezone to null when the field is absent", () => {
    const parsed = UserSchema.parse(base);
    expect(parsed.timezone).toBe(null);
  });

  it("preserves an explicit IANA timezone", () => {
    const parsed = UserSchema.parse({ ...base, timezone: "Asia/Tokyo" });
    expect(parsed.timezone).toBe("Asia/Tokyo");
  });

  it("accepts an explicit null timezone", () => {
    const parsed = UserSchema.parse({ ...base, timezone: null });
    expect(parsed.timezone).toBe(null);
  });

  // Wrong-type drift: a future server bug sending `timezone` as a number
  // must not throw into the UI. parseWithFallback degrades the whole user
  // object to the explicit fallback (EMPTY_USER) so /api/me callers keep a
  // valid shape instead of white-screening.
  it("falls back to EMPTY_USER when timezone is the wrong type", () => {
    const parsed = parseWithFallback(
      { ...base, timezone: 42 },
      UserSchema,
      EMPTY_USER,
      { endpoint: "GET /api/me" },
    );
    expect(parsed).toBe(EMPTY_USER);
  });
});

describe("SquadListSchema member preview drift", () => {
  const baseSquad = {
    id: "squad-1",
    workspace_id: "ws-1",
    name: "Frontend Squad",
    description: "",
    instructions: "",
    avatar_url: null,
    leader_id: "agent-1",
    creator_id: "user-1",
    created_at: "2026-05-01T00:00:00Z",
    updated_at: "2026-05-01T00:00:00Z",
    archived_at: null,
    archived_by: null,
  };

  it("defaults preview fields when an older backend omits them", () => {
    const parsed = SquadListSchema.parse([baseSquad]);
    expect(parsed[0]?.member_count).toBe(0);
    expect(parsed[0]?.member_preview).toEqual([]);
  });

  it("defaults preview fields on a single squad response", () => {
    const parsed = SquadSchema.parse(baseSquad);
    expect(parsed.member_count).toBe(0);
    expect(parsed.member_preview).toEqual([]);
  });

  it("preserves lightweight member preview rows", () => {
    const parsed = SquadListSchema.parse([
      {
        ...baseSquad,
        member_count: 2,
        member_preview: [
          { member_type: "agent", member_id: "agent-1", role: "leader" },
          { member_type: "member", member_id: "user-2", role: "member" },
        ],
      },
    ]);
    expect(parsed[0]?.member_count).toBe(2);
    expect(parsed[0]?.member_preview).toHaveLength(2);
    expect(parsed[0]?.member_preview?.[0]?.role).toBe("leader");
  });
});

// The workspace dashboard and runtime-detail pages were re-pointed at the
// unified `task_usage_hourly` rollup. Every numeric field drives chart /
// KPI math, and string keys (date / agent_id / model) bucket the series.
// The contract these schemas must hold: a row missing a field degrades
// that field to a sane default rather than dropping the WHOLE array to
// the `[]` fallback — one drifted row must not blank the entire chart.
describe("dashboard + runtime usage schema drift", () => {
  it("coerces a missing numeric field to 0 instead of dropping the array", () => {
    const parsed = DashboardUsageDailyListSchema.parse([
      { date: "2026-05-19", model: "claude-opus-4-7", input_tokens: 100 },
    ]);
    expect(parsed).toHaveLength(1);
    expect(parsed[0]?.output_tokens).toBe(0);
    expect(parsed[0]?.cache_read_tokens).toBe(0);
    expect(parsed[0]?.cache_write_tokens).toBe(0);
  });

  it("coerces a missing date key to \"\" so the rest of the series survives", () => {
    const parsed = DashboardUsageDailyListSchema.parse([
      { model: "claude-opus-4-7", input_tokens: 5 },
    ]);
    expect(parsed).toHaveLength(1);
    expect(parsed[0]?.date).toBe("");
  });

  it("coerces a missing agent_id key to \"\" for the agent-runtime panel", () => {
    const parsed = DashboardAgentRunTimeListSchema.parse([
      { total_seconds: 42, task_count: 3, failed_count: 0 },
    ]);
    expect(parsed).toHaveLength(1);
    expect(parsed[0]?.agent_id).toBe("");
  });

  it("defaults a missing cancelled_count to 0 so a pre-cancelled-count server still renders", () => {
    // cancelled_count was added when the run-time rollups started counting
    // runs the user stopped mid-flight. A backend predating it omits the
    // field; the row must survive with a 0 segment rather than drop the
    // whole series (installed desktop clients hit older backends).
    expect(
      DashboardAgentRunTimeListSchema.parse([
        { agent_id: "a", total_seconds: 42, task_count: 3, failed_count: 0 },
      ])[0]?.cancelled_count,
    ).toBe(0);
    expect(
      DashboardRunTimeDailyListSchema.parse([
        { date: "2026-05-19", total_seconds: 42, task_count: 3, failed_count: 0 },
      ])[0]?.cancelled_count,
    ).toBe(0);
  });

  it("coerces a missing agent_id key to \"\" for the usage-by-agent panel", () => {
    const parsed = DashboardUsageByAgentListSchema.parse([
      { model: "claude-opus-4-7", input_tokens: 7 },
    ]);
    expect(parsed[0]?.agent_id).toBe("");
  });

  it("coerces missing fields on every runtime usage schema", () => {
    expect(RuntimeUsageListSchema.parse([{ date: "2026-05-19" }])[0]?.input_tokens).toBe(0);
    expect(RuntimeHourlyActivityListSchema.parse([{ hour: 9 }])[0]?.count).toBe(0);
    expect(RuntimeUsageByAgentListSchema.parse([{ model: "x" }])[0]?.agent_id).toBe("");
    expect(RuntimeUsageByHourListSchema.parse([{ hour: 9 }])[0]?.model).toBe("");
  });

  it("defaults a missing provider to \"\" so an older server's rows still price by bare model", () => {
    // provider was added for cross-provider model disambiguation; a server
    // predating it omits the field. The schema must fill "" (→ bare-model
    // pricing lookup) rather than drop the row.
    expect(
      DashboardUsageDailyListSchema.parse([{ date: "2026-05-19", model: "claude-opus-4-7" }])[0]
        ?.provider,
    ).toBe("");
    expect(
      DashboardUsageByAgentListSchema.parse([{ model: "claude-opus-4-7" }])[0]?.provider,
    ).toBe("");
    expect(RuntimeUsageByAgentListSchema.parse([{ model: "x" }])[0]?.provider).toBe("");
  });

  it("rejects a non-array body so parseWithFallback can return its fallback", () => {
    expect(DashboardUsageDailyListSchema.safeParse(null).success).toBe(false);
    expect(DashboardFailureDailyListSchema.safeParse(null).success).toBe(false);
    expect(DashboardFailureByAgentListSchema.safeParse({ rows: [] }).success).toBe(
      false,
    );
    expect(RuntimeUsageListSchema.safeParse({ rows: [] }).success).toBe(false);
  });

  it("keeps a failure_reason the client build has never heard of", () => {
    // failure_reason is an open string, not an enum: the backend taxonomy
    // grows, and an installed desktop client must still count a reason its
    // build predates rather than dropping the row (and with it the day's
    // error total).
    const parsed = DashboardFailureDailyListSchema.parse([
      { date: "2026-05-19", failure_reason: "agent_error.brand_new", task_count: 3 },
    ]);
    expect(parsed).toHaveLength(1);
    expect(parsed[0]?.failure_reason).toBe("agent_error.brand_new");
    expect(parsed[0]?.task_count).toBe(3);
  });

  it("coerces a missing failure row field without dropping the array", () => {
    const daily = DashboardFailureDailyListSchema.parse([{ date: "2026-05-19" }]);
    expect(daily).toHaveLength(1);
    // "" is the succeeded bucket, so a reason-less row lands in the
    // denominator instead of inventing a failure that never happened.
    //
    // Defaulting to a failure bucket instead was considered and rejected: the
    // realistic drift here is someone adding `omitempty` to the Go struct
    // tag, which would strip the field from exactly the SUCCESS rows and turn
    // every window into a 100% error rate. Deflating a rate under drift is
    // the milder failure. TestDashboardFailureWireContractKeepsEmptyReason
    // (server/internal/handler/dashboard_test.go) guards the other side by
    // pinning that the server always emits the field.
    expect(daily[0]?.failure_reason).toBe("");
    expect(daily[0]?.task_count).toBe(0);

    const byAgent = DashboardFailureByAgentListSchema.parse([
      { failure_reason: "timeout", task_count: 2 },
    ]);
    expect(byAgent[0]?.agent_id).toBe("");
  });

  it("keeps unknown server-side fields via .loose()", () => {
    const parsed = RuntimeUsageListSchema.parse([
      { date: "2026-05-19", region: "us-east" },
    ]);
    expect((parsed[0] as Record<string, unknown>).region).toBe("us-east");
  });
});

// A server that never heard of worktree mode also never sends this flag, and
// it does not reject the mode either — it drops execution_mode and answers 201,
// leaving the task to run in the user's working copy (#7113). So the absent
// case has to parse as false, not as "unknown, probably fine".
describe("AppConfigSchema local_worktree_supported drift", () => {
  it("defaults to false when the server predates the signal", () => {
    const parsed = AppConfigSchema.parse({ cdn_domain: "cdn.example.com" });
    expect(parsed.local_worktree_supported).toBe(false);
  });

  it("coerces a malformed value to false rather than trusting it", () => {
    const parsed = AppConfigSchema.parse({
      cdn_domain: "cdn.example.com",
      local_worktree_supported: "yes",
    });
    expect(parsed.local_worktree_supported).toBe(false);
  });

  it("carries a genuine true through", () => {
    const parsed = AppConfigSchema.parse({
      cdn_domain: "cdn.example.com",
      local_worktree_supported: true,
    });
    expect(parsed.local_worktree_supported).toBe(true);
  });
});

describe("AppConfigSchema agent_conversation_starters_supported drift", () => {
  it("defaults to false when the server predates the persistence contract", () => {
    expect(AppConfigSchema.parse({}).agent_conversation_starters_supported).toBe(false);
  });

  it("coerces a malformed declaration to false", () => {
    expect(
      AppConfigSchema.parse({ agent_conversation_starters_supported: "yes" })
        .agent_conversation_starters_supported,
    ).toBe(false);
  });

  it("carries a genuine declaration through", () => {
    expect(
      AppConfigSchema.parse({ agent_conversation_starters_supported: true })
        .agent_conversation_starters_supported,
    ).toBe(true);
  });
});

describe("AppConfigSchema cdn_signed drift", () => {
  it("defaults cdn_signed to false when the server omits it (pre-MUL-3254 servers)", () => {
    const parsed = AppConfigSchema.parse({ cdn_domain: "cdn.example.com" });
    expect(parsed.cdn_signed).toBe(false);
  });

  it("coerces a malformed cdn_signed to false instead of failing the whole config", () => {
    const parsed = AppConfigSchema.parse({
      cdn_domain: "cdn.example.com",
      cdn_signed: "yes",
    });
    expect(parsed.cdn_signed).toBe(false);
    expect(parsed.cdn_domain).toBe("cdn.example.com");
  });

  it("keeps cdn_signed=true from a signing-enabled server", () => {
    const parsed = AppConfigSchema.parse({ cdn_signed: true });
    expect(parsed.cdn_signed).toBe(true);
  });

  it("parses frontend feature flag decisions", () => {
    const parsed = AppConfigSchema.parse({
      feature_flags: {
        composio_mcp_apps: true,
        malformed_future_flag: "yes",
      },
    });
    expect(parsed.feature_flags).toEqual({
      composio_mcp_apps: true,
      malformed_future_flag: false,
    });
  });

  it("defaults malformed feature_flags to an empty object", () => {
    const parsed = AppConfigSchema.parse({ feature_flags: ["not", "an", "object"] });
    expect(parsed.feature_flags).toEqual({});
  });

  it("parses server_version and leaves it undefined when the server omits it", () => {
    expect(AppConfigSchema.parse({ server_version: "1.2.3" }).server_version).toBe("1.2.3");
    expect(AppConfigSchema.parse({}).server_version).toBeUndefined();
  });
});

describe("InboxUnreadSummarySchema", () => {
  const ENDPOINT = { endpoint: "GET /api/inbox/unread-summary" };

  it("parses a well-formed summary and tolerates extra fields", () => {
    const parsed = parseWithFallback(
      [
        { workspace_id: "ws-1", count: 2 },
        { workspace_id: "ws-2", count: 0, future_field: "ignored" },
      ],
      InboxUnreadSummarySchema,
      EMPTY_INBOX_UNREAD_SUMMARY,
      ENDPOINT,
    );
    expect(parsed).toEqual([
      { workspace_id: "ws-1", count: 2 },
      { workspace_id: "ws-2", count: 0, future_field: "ignored" },
    ]);
  });

  it("returns the empty fallback (dot hidden) for a non-array body", () => {
    expect(
      parseWithFallback({ rows: [] }, InboxUnreadSummarySchema, EMPTY_INBOX_UNREAD_SUMMARY, ENDPOINT),
    ).toBe(EMPTY_INBOX_UNREAD_SUMMARY);
    expect(
      parseWithFallback(null, InboxUnreadSummarySchema, EMPTY_INBOX_UNREAD_SUMMARY, ENDPOINT),
    ).toBe(EMPTY_INBOX_UNREAD_SUMMARY);
  });

  it("returns the empty fallback when an entry has a wrong-typed count", () => {
    expect(
      parseWithFallback(
        [{ workspace_id: "ws-1", count: "lots" }],
        InboxUnreadSummarySchema,
        EMPTY_INBOX_UNREAD_SUMMARY,
        ENDPOINT,
      ),
    ).toBe(EMPTY_INBOX_UNREAD_SUMMARY);
  });
});

describe("InboxItemListSchema", () => {
  const ENDPOINT = { endpoint: "GET /api/inbox/archived" };

  const row = (overrides: Record<string, unknown> = {}) => ({
    id: "inbox-1",
    workspace_id: "ws-1",
    recipient_type: "member",
    recipient_id: "member-1",
    type: "new_comment",
    severity: "info",
    issue_id: "issue-1",
    title: "Issue title",
    body: null,
    read: false,
    archived: true,
    created_at: "2026-06-15T08:00:00Z",
    ...overrides,
  });

  it("preserves a Chat intake continuation and the rest of the actual Inbox response contract", () => {
    const details = { owner_relevant: "true", owner_category: "needs_you", required_action: "continue_intake", intake_state: "pending", source_task_id: "chat-task", chat_session_id: "chat-session", request: "Verify the existing issue." };
    const parsed = parseWithFallback([row({ type: "chat_intake", severity: "action_required", archived: false, details }), row({ id: "ordinary" })], InboxItemListSchema, EMPTY_INBOX_ITEMS, { endpoint: "GET /api/inbox" });
    expect(parsed).toHaveLength(2);
    expect(parsed[0]).toMatchObject({ type: "chat_intake", details });
  });

  it("parses a well-formed archived list and tolerates extra fields", () => {
    const parsed = parseWithFallback(
      [row({
        issue_status: "in_progress",
        issue_priority: "high",
        details: { comment_id: "c-1" },
        future_field: 1,
      })],
      InboxItemListSchema,
      EMPTY_INBOX_ITEMS,
      ENDPOINT,
    );
    expect(parsed).toHaveLength(1);
    expect(parsed[0]).toMatchObject({
      id: "inbox-1",
      archived: true,
      issue_status: "in_progress",
      issue_priority: "high",
    });
  });

  it("keeps a notification type this client doesn't know yet", () => {
    // Enums stay lenient on purpose: a backend that ships a new inbox type
    // must not blank the whole archived list on older clients.
    const parsed = parseWithFallback(
      [row({ type: "some_future_type", severity: "future_severity" })],
      InboxItemListSchema,
      EMPTY_INBOX_ITEMS,
      ENDPOINT,
    );
    expect(parsed).toHaveLength(1);
  });

  it("accepts rows that omit the nullable optional fields", () => {
    const { body, issue_id, ...withoutOptionals } = row();
    void body;
    void issue_id;
    expect(
      parseWithFallback([withoutOptionals], InboxItemListSchema, EMPTY_INBOX_ITEMS, ENDPOINT),
    ).toHaveLength(1);
  });

  it("returns the empty fallback when an issue projection is wrong-typed", () => {
    expect(
      parseWithFallback(
        [row({ issue_priority: 3 })],
        InboxItemListSchema,
        EMPTY_INBOX_ITEMS,
        ENDPOINT,
      ),
    ).toBe(EMPTY_INBOX_ITEMS);
  });

  it("returns the empty fallback for a non-array body", () => {
    expect(
      parseWithFallback({ items: [] }, InboxItemListSchema, EMPTY_INBOX_ITEMS, ENDPOINT),
    ).toBe(EMPTY_INBOX_ITEMS);
    expect(
      parseWithFallback(null, InboxItemListSchema, EMPTY_INBOX_ITEMS, ENDPOINT),
    ).toBe(EMPTY_INBOX_ITEMS);
  });

  it("returns the empty fallback when a row is missing a required field", () => {
    const { id, ...withoutId } = row();
    void id;
    expect(
      parseWithFallback([withoutId], InboxItemListSchema, EMPTY_INBOX_ITEMS, ENDPOINT),
    ).toBe(EMPTY_INBOX_ITEMS);
  });

  it("returns the empty fallback when `archived` is wrong-typed", () => {
    expect(
      parseWithFallback(
        [row({ archived: "yes" })],
        InboxItemListSchema,
        EMPTY_INBOX_ITEMS,
        ENDPOINT,
      ),
    ).toBe(EMPTY_INBOX_ITEMS);
  });
});

describe("SearchProjectsResponseSchema date drift", () => {
  const ENDPOINT = { endpoint: "GET /api/projects/search" };

  const baseProject = {
    id: "p-1",
    workspace_id: "ws-1",
    title: "Launch",
    description: null,
    icon: null,
    status: "in_progress",
    priority: "high",
    lead_type: null,
    lead_id: null,
    created_at: "2026-01-01T00:00:00Z",
    updated_at: "2026-01-01T00:00:00Z",
    issue_count: 0,
    done_count: 0,
    resource_count: 0,
    match_source: "title",
  };

  it("parses start_date / due_date when the backend returns them", () => {
    const parsed = parseWithFallback(
      { projects: [{ ...baseProject, start_date: "2026-03-01", due_date: "2026-03-31" }], total: 1 },
      SearchProjectsResponseSchema,
      EMPTY_SEARCH_PROJECTS_RESPONSE,
      ENDPOINT,
    );
    expect(parsed.projects[0]?.start_date).toBe("2026-03-01");
    expect(parsed.projects[0]?.due_date).toBe("2026-03-31");
  });

  // Frontend deploys before backend: an older backend omits the new keys. The
  // .default(null) must keep the whole batch parseable (→ null), not degrade
  // it to the empty fallback and blank the search results.
  it("defaults missing start_date / due_date to null without dropping results", () => {
    const parsed = parseWithFallback(
      { projects: [baseProject], total: 1 },
      SearchProjectsResponseSchema,
      EMPTY_SEARCH_PROJECTS_RESPONSE,
      ENDPOINT,
    );
    expect(parsed).not.toBe(EMPTY_SEARCH_PROJECTS_RESPONSE);
    expect(parsed.projects).toHaveLength(1);
    expect(parsed.projects[0]?.start_date).toBeNull();
    expect(parsed.projects[0]?.due_date).toBeNull();
  });
});

// The "run now" flow branches on run.status/reason_code to avoid a false-success
// toast (MUL-4525), so the trigger response must survive backend drift.
describe("AutopilotRunSchema", () => {
  const ENDPOINT = { endpoint: "POST /api/autopilots/:id/trigger" };
  const baseRun = {
    id: "run-1",
    autopilot_id: "ap-1",
    trigger_id: null,
    source: "manual",
    status: "issue_created",
    issue_id: "issue-1",
    task_id: null,
    triggered_at: "2026-07-14T00:00:00Z",
    completed_at: null,
    failure_reason: null,
    trigger_payload: null,
    result: null,
    created_at: "2026-07-14T00:00:00Z",
  };

  it("preserves a blocked run's status and reason_code", () => {
    const parsed = parseWithFallback(
      { ...baseRun, status: "skipped", failure_reason: "you are not allowed to trigger this autopilot's assignee agent", reason_code: "invocation_not_allowed" },
      AutopilotRunSchema,
      FALLBACK_AUTOPILOT_RUN,
      ENDPOINT,
    );
    expect(parsed.status).toBe("skipped");
    expect(parsed.reason_code).toBe("invocation_not_allowed");
  });

  it("tolerates an older server omitting reason_code", () => {
    const parsed = parseWithFallback(baseRun, AutopilotRunSchema, FALLBACK_AUTOPILOT_RUN, ENDPOINT);
    expect(parsed.status).toBe("issue_created");
    expect(parsed.reason_code).toBeUndefined();
  });

  it("degrades a malformed response to a non-success fallback (never a false success)", () => {
    const parsed = parseWithFallback("not-an-object", AutopilotRunSchema, FALLBACK_AUTOPILOT_RUN, ENDPOINT);
    expect(parsed).toBe(FALLBACK_AUTOPILOT_RUN);
    expect(parsed.status).toBe("failed");
  });
});

describe("AutopilotQuotaUsageSchema", () => {
  const baseUsage = {
    action: "enforce",
    used: 12,
    reserved: 2,
    limit: 100,
    period_start: "2026-08-01T00:00:00Z",
    period_end: "2026-09-01T00:00:00Z",
    reset_at: "2026-09-01T00:00:00Z",
  };

  it("preserves durable blocked counts by execution source", () => {
    const parsed = AutopilotQuotaUsageSchema.parse({
      ...baseUsage,
      blocked_counts: { schedule: 3, webhook: 7 },
    });
    expect(parsed.blocked_counts).toEqual({ schedule: 3, webhook: 7 });
  });

  it("defaults blocked_counts to null for an older server", () => {
    expect(AutopilotQuotaUsageSchema.parse(baseUsage).blocked_counts).toBeNull();
  });

  it("isolates a malformed blocked_counts field", () => {
    const parsed = AutopilotQuotaUsageSchema.parse({
      ...baseUsage,
      blocked_counts: { webhook: "many" },
    });
    expect(parsed.used).toBe(12);
    expect(parsed.blocked_counts).toBeNull();
  });
});

// The comment composer branches on preview.blocked to warn before sending
// (MUL-4525 §2), so the additive field must parse and degrade gracefully.
describe("CommentTriggerPreviewSchema.blocked", () => {
  it("parses blocked mention outcomes alongside agents", () => {
    const parsed = CommentTriggerPreviewSchema.parse({
      agents: [{ id: "a1", source: "mention_agent", reason: "" }],
      blocked: [
        { target_type: "squad", target_id: "s1", status: "blocked", reason_code: "invocation_not_allowed" },
      ],
    });
    expect(parsed.agents).toHaveLength(1);
    expect(parsed.blocked).toEqual([
      { target_type: "squad", target_id: "s1", status: "blocked", reason_code: "invocation_not_allowed" },
    ]);
  });

  it("defaults blocked to [] when an older server omits it", () => {
    const parsed = CommentTriggerPreviewSchema.parse({ agents: [] });
    expect(parsed.blocked).toEqual([]);
  });

  it("degrades a malformed blocked field to [] without dropping agents", () => {
    const parsed = CommentTriggerPreviewSchema.parse({
      agents: [{ id: "a1", source: "mention_agent", reason: "" }],
      blocked: "nope",
    });
    expect(parsed.agents).toHaveLength(1);
    expect(parsed.blocked).toEqual([]);
  });

  it("drops a single malformed blocked entry without discarding the valid ones", () => {
    const parsed = CommentTriggerPreviewSchema.parse({
      agents: [],
      blocked: [
        { target_type: "squad", target_id: "s1", status: "blocked", reason_code: "invocation_not_allowed" },
        { status: "blocked" }, // missing target_id → dropped individually
        { target_type: "agent", target_id: "a1", status: "blocked", reason_code: "runtime_offline" },
      ],
    });
    expect(parsed.blocked.map((b) => b.target_id)).toEqual(["s1", "a1"]);
  });
});

describe("RuntimeModelListRequestSchema", () => {
  const completed = {
    id: "req-1",
    runtime_id: "rt-1",
    status: "completed",
    supported: true,
    created_at: "2026-07-29T00:00:00Z",
    updated_at: "2026-07-29T00:00:01Z",
    models: [
      {
        id: "gpt-5.6-sol",
        label: "GPT-5.6-Sol",
        provider: "openai",
        default: true,
        thinking: {
          supported_levels: [{ value: "high", label: "High" }],
          default_level: "low",
        },
        service_tiers: [{ id: "fast", name: "Fast" }],
        supports_explicit_standard_service_tier: true,
      },
    ],
  };

  it("parses a live completed discovery, keeping the fields the UI branches on", () => {
    const parsed = parseWithFallback(
      completed,
      RuntimeModelListRequestSchema,
      MALFORMED_RUNTIME_MODEL_LIST_REQUEST,
      { endpoint: "test" },
    );
    expect(parsed.status).toBe("completed");
    expect(parsed.supported).toBe(true);
    expect(parsed.models?.[0]?.default).toBe(true);
    expect(parsed.models?.[0]?.thinking?.supported_levels).toEqual([
      { value: "high", label: "High" },
    ]);
    expect(parsed.models?.[0]?.service_tiers).toEqual([{ id: "fast", name: "Fast" }]);
    expect(
      parsed.models?.[0]?.supports_explicit_standard_service_tier,
    ).toBe(true);
    expect(parsed.cached).toBeUndefined();
  });

  it("keeps the additive cache markers when the server serves a snapshot", () => {
    const parsed = parseWithFallback(
      { ...completed, cached: true, cached_at: "2026-07-29T00:00:00Z" },
      RuntimeModelListRequestSchema,
      MALFORMED_RUNTIME_MODEL_LIST_REQUEST,
      { endpoint: "test" },
    );
    expect(parsed.cached).toBe(true);
    expect(parsed.cached_at).toBe("2026-07-29T00:00:00Z");
  });

  // A backend that predates MUL-5444 sends neither marker; an even older one
  // may omit `supported`. Both must stay usable rather than reading as
  // "runtime manages the model itself" off an undefined.
  it("defaults supported to true on an older backend that omits it", () => {
    const { supported: _omitted, ...withoutSupported } = completed;
    const parsed = parseWithFallback(
      withoutSupported,
      RuntimeModelListRequestSchema,
      MALFORMED_RUNTIME_MODEL_LIST_REQUEST,
      { endpoint: "test" },
    );
    expect(parsed.supported).toBe(true);
    expect(parsed.cached).toBeUndefined();
  });

  // MUL-6961: models the runtime cannot run arrive in their OWN list. The
  // separation is the compatibility contract — a client that never learned the
  // field reads `models` and therefore cannot offer one — so the schema must
  // keep them apart rather than folding them together.
  it("parses unavailable models without letting them into the selectable list", () => {
    const parsed = parseWithFallback(
      {
        ...completed,
        unavailable_models: [
          {
            id: "cc-update-required-1",
            label: "Fable 5.1 (disabled)",
            reason: "Update to 2.1.255+ to use Fable 5.1",
          },
        ],
      },
      RuntimeModelListRequestSchema,
      MALFORMED_RUNTIME_MODEL_LIST_REQUEST,
      { endpoint: "test" },
    );
    expect(parsed.unavailable_models?.[0]?.id).toBe("cc-update-required-1");
    expect(parsed.unavailable_models?.[0]?.reason).toBe(
      "Update to 2.1.255+ to use Fable 5.1",
    );
    expect(parsed.models?.map((m) => m.id)).not.toContain(
      "cc-update-required-1",
    );
  });

  // An older daemon or server sends no such key. The picker then shows no
  // unavailable section, which is exactly the pre-MUL-6961 behaviour.
  it("tolerates a backend that omits unavailable models entirely", () => {
    const parsed = parseWithFallback(
      completed,
      RuntimeModelListRequestSchema,
      MALFORMED_RUNTIME_MODEL_LIST_REQUEST,
      { endpoint: "test" },
    );
    expect(parsed.unavailable_models).toBeUndefined();
    expect(parsed.models?.length).toBe(1);
  });

  it("treats an older daemon that omits explicit-standard support as unsupported", () => {
    const model = completed.models[0]!;
    const {
      supports_explicit_standard_service_tier: _omitted,
      ...oldDaemonModel
    } = model;
    const parsed = parseWithFallback(
      { ...completed, models: [oldDaemonModel] },
      RuntimeModelListRequestSchema,
      MALFORMED_RUNTIME_MODEL_LIST_REQUEST,
      { endpoint: "test" },
    );

    expect(
      parsed.models?.[0]?.supports_explicit_standard_service_tier,
    ).toBeUndefined();
  });

  it("passes an unknown status through instead of failing the whole response", () => {
    const parsed = parseWithFallback(
      { ...completed, status: "superseded" },
      RuntimeModelListRequestSchema,
      MALFORMED_RUNTIME_MODEL_LIST_REQUEST,
      { endpoint: "test" },
    );
    expect(parsed.status).toBe("superseded");
  });

  // Malformed bodies must land on the "failed" fallback: `completed` would
  // fabricate an empty catalog and `pending` would spin the picker until the
  // client-side poll timeout.
  it("falls back to an explicit failure on a malformed body", () => {
    for (const malformed of [
      null,
      "nope",
      42,
      {},
      { status: 7 },
      { ...completed, status: undefined },
      { ...completed, supported: "yes" },
      { ...completed, models: "nope" },
      { ...completed, models: [{ label: "no id" }] },
      {
        ...completed,
        models: [
          {
            ...completed.models[0],
            supports_explicit_standard_service_tier: "yes",
          },
        ],
      },
    ]) {
      const parsed = parseWithFallback(
        malformed,
        RuntimeModelListRequestSchema,
        MALFORMED_RUNTIME_MODEL_LIST_REQUEST,
        { endpoint: "test" },
      );
      expect(parsed.status).toBe("failed");
      expect(parsed.supported).toBe(true);
      expect(parsed.error).toBe("invalid model discovery response");
    }
  });

  it("keeps unknown server fields instead of stripping them", () => {
    const parsed = parseWithFallback(
      { ...completed, future_field: "keep me" },
      RuntimeModelListRequestSchema,
      MALFORMED_RUNTIME_MODEL_LIST_REQUEST,
      { endpoint: "test" },
    );
    expect((parsed as unknown as { future_field?: string }).future_field).toBe(
      "keep me",
    );
  });
});

describe("IssueViewSchema", () => {
  const valid = {
    id: "v1",
    workspace_id: "ws1",
    owner_id: "u1",
    name: "Needs review",
    scope_type: "workspace",
    scope_id: null,
    scope_variant: null,
    visibility: "workspace",
    definition_version: 1,
    query: { statusFilters: ["in_review"] },
    display: { viewMode: "board" },
    revision: 3,
    created_at: "2026-08-06T00:00:00Z",
    updated_at: "2026-08-06T00:00:00Z",
  };

  it("parses a well-formed view and keeps unknown future fields", () => {
    const parsed = IssueViewSchema.parse({ ...valid, future_field: "keep me" });
    expect(parsed.name).toBe("Needs review");
    expect(parsed.query).toEqual({ statusFilters: ["in_review"] });
    expect((parsed as unknown as { future_field?: string }).future_field).toBe("keep me");
  });

  it("defaults missing definition blobs instead of failing", () => {
    const parsed = IssueViewSchema.parse({ id: "v2" });
    expect(parsed.query).toEqual({});
    expect(parsed.display).toEqual({});
    expect(parsed.revision).toBe(1);
  });

  it("degrades a malformed list response to [] via parseWithFallback", () => {
    expect(
      parseWithFallback({ nonsense: true }, IssueViewListSchema, [], {
        endpoint: "GET /api/issue-views",
      }),
    ).toEqual([]);
    expect(
      parseWithFallback(null, IssueViewListSchema, [], {
        endpoint: "GET /api/issue-views",
      }),
    ).toEqual([]);
  });

  it("degrades a malformed detail response to null — NOT an error", () => {
    // The sidebar's pinned view rows hinge on this distinction: a parse
    // fallback (null, no error) hides the row, while only a REAL 404
    // error may ever unpin. A malformed body must never destroy a pin.
    expect(
      parseWithFallback({ nonsense: true }, IssueViewSchema.nullable(), null, {
        endpoint: "GET /api/issue-views/{id}",
      }),
    ).toBeNull();
  });
});

// WeCom smart-bot installation schemas. These gate UI affordances (the Connect
// dialog, the "ask your operator" state, the revoked-vs-active badge), so a
// malformed response must degrade to the safe state rather than a broken one.
describe("WeCom installation schemas", () => {
  it("parses a well-formed installation", () => {
    const parsed = WecomInstallationSchema.parse({
      id: "i1",
      workspace_id: "w1",
      agent_id: "a1",
      bot_id: "aibot_xyz",
      installer_user_id: "u1",
      status: "active",
    });
    expect(parsed.bot_id).toBe("aibot_xyz");
    expect(parsed.status).toBe("active");
  });

  it("defaults a missing status to 'revoked', never 'active'", () => {
    // A broken read must not render a bot as connected when it may not be.
    const parsed = WecomInstallationSchema.parse({ id: "i1" });
    expect(parsed.status).toBe("revoked");
    expect(parsed.bot_id).toBe("");
  });

  it("keeps unknown forward-compat fields (loose) instead of failing the parse", () => {
    const parsed = WecomInstallationSchema.parse({ id: "i1", future_field: "keep" });
    expect((parsed as unknown as { future_field?: string }).future_field).toBe("keep");
  });

  it("defaults 'configured' to false so a malformed list renders the operator state", () => {
    const parsed = ListWecomInstallationsResponseSchema.parse({});
    expect(parsed.configured).toBe(false);
    expect(parsed.installations).toEqual([]);
  });

  it("falls back to the empty list when the response is not an object", () => {
    const parsed = parseWithFallback(
      "not json",
      ListWecomInstallationsResponseSchema,
      EMPTY_LIST_WECOM_INSTALLATIONS_RESPONSE,
      { endpoint: "GET /api/workspaces/:id/wecom/installations" },
    );
    expect(parsed).toEqual(EMPTY_LIST_WECOM_INSTALLATIONS_RESPONSE);
    expect(parsed.configured).toBe(false);
  });

  it("falls back on a malformed installation and redeem response", () => {
    const inst = parseWithFallback(42, WecomInstallationSchema, EMPTY_WECOM_INSTALLATION, {
      endpoint: "POST /api/workspaces/:id/wecom/install/byo",
    });
    expect(inst).toEqual(EMPTY_WECOM_INSTALLATION);

    const redeem = parseWithFallback(
      null,
      RedeemWecomBindingTokenResponseSchema,
      EMPTY_REDEEM_WECOM_BINDING_TOKEN_RESPONSE,
      { endpoint: "POST /api/wecom/binding/redeem" },
    );
    expect(redeem).toEqual(EMPTY_REDEEM_WECOM_BINDING_TOKEN_RESPONSE);
  });
});

// Telegram drives the same connect/disabled/revoked UI decisions as WeCom.
// Keep its wire-contract fallbacks explicit so malformed or older responses
// cannot render a bot as connected or report a binding success.
describe("Telegram installation schemas", () => {
  it("parses a well-formed installation", () => {
    const parsed = TelegramInstallationSchema.parse({
      id: "i1",
      workspace_id: "w1",
      agent_id: "a1",
      bot_id: "12345",
      bot_username: "multica_test_bot",
      installer_user_id: "u1",
      status: "active",
    });
    expect(parsed.bot_username).toBe("multica_test_bot");
    expect(parsed.status).toBe("active");
  });

  it("defaults incomplete data to the disconnected state", () => {
    const parsed = TelegramInstallationSchema.parse({ id: "i1" });
    expect(parsed.status).toBe("revoked");
    expect(parsed.bot_id).toBe("");
    expect(parsed.bot_username).toBe("");

    const list = ListTelegramInstallationsResponseSchema.parse({});
    expect(list).toEqual({ installations: [], configured: false });
  });

  it("keeps unknown forward-compatible installation fields", () => {
    const parsed = TelegramInstallationSchema.parse({ id: "i1", future_field: "keep" });
    expect((parsed as unknown as { future_field?: string }).future_field).toBe("keep");
  });

  it("falls back safely for malformed list, install, and redeem responses", () => {
    expect(
      parseWithFallback(
        "not json",
        ListTelegramInstallationsResponseSchema,
        EMPTY_LIST_TELEGRAM_INSTALLATIONS_RESPONSE,
        { endpoint: "GET /api/workspaces/:id/telegram/installations" },
      ),
    ).toEqual(EMPTY_LIST_TELEGRAM_INSTALLATIONS_RESPONSE);

    expect(
      parseWithFallback(42, TelegramInstallationSchema, EMPTY_TELEGRAM_INSTALLATION, {
        endpoint: "POST /api/workspaces/:id/telegram/install",
      }),
    ).toEqual(EMPTY_TELEGRAM_INSTALLATION);

    expect(
      parseWithFallback(
        null,
        RedeemTelegramBindingTokenResponseSchema,
        EMPTY_REDEEM_TELEGRAM_BINDING_TOKEN_RESPONSE,
        { endpoint: "POST /api/telegram/binding/redeem" },
      ),
    ).toEqual(EMPTY_REDEEM_TELEGRAM_BINDING_TOKEN_RESPONSE);
  });
});

describe("Plugin schemas", () => {
  it("defaults every missing installation field to an inert, disabled shape", () => {
    const parsed = PluginInstallationSchema.parse({ id: "installation-1" });
    expect(parsed.enabled).toBe(false);
    expect(parsed.granted_scopes).toEqual([]);
    expect(parsed.config_schema).toEqual([]);
    expect(parsed.configured_secrets).toEqual([]);
    expect(parsed.surfaces).toEqual([]);
    expect(parsed.hooks).toEqual([]);
    expect(parsed.resources).toEqual([]);
  });

  it("does not model a secret value even when the server sends one", () => {
    // The API contract is that a secret is write-only. If a future response
    // ever regressed and echoed one, the client must not carry it into typed
    // state where a component could render it.
    const parsed = PluginInstallationSchema.parse({
      id: "installation-1",
      config: { repo: "multica-ai/multica" },
      configured_secrets: ["token"],
    });
    expect(parsed.config).toEqual({ repo: "multica-ai/multica" });
    expect(parsed.configured_secrets).toEqual(["token"]);
    expect(Object.keys(parsed)).not.toContain("secrets");
  });

  it("keeps config field declaration order so the generated form is stable", () => {
    const parsed = PluginInstallationSchema.parse({
      id: "installation-1",
      config_schema: [
        { key: "repo", type: "string", label: "Repo", required: true },
        { key: "token", type: "secret", label: "Token" },
      ],
    });
    expect(parsed.config_schema.map((field) => field.key)).toEqual(["repo", "token"]);
    expect(parsed.config_schema[1]?.required).toBe(false);
  });

  it("degrades a malformed installation list to empty rather than a partial list", () => {
    const parsed = parseWithFallback("not-json", PluginInstallationListResponseSchema, EMPTY_PLUGIN_INSTALLATION_LIST, {
      endpoint: "GET /api/workspaces/{id}/plugins",
    });
    expect(parsed).toEqual(EMPTY_PLUGIN_INSTALLATION_LIST);
  });

  it("degrades a malformed preview so the consent screen cannot show a blank scope list as approval", () => {
    const parsed = parseWithFallback({ scopes: "issues:read" }, PluginPreviewSchema, EMPTY_PLUGIN_PREVIEW, {
      endpoint: "POST /api/workspaces/{id}/plugins/preview",
    });
    expect(parsed).toEqual(EMPTY_PLUGIN_PREVIEW);
    expect(parsed.scopes).toEqual([]);
  });

  it("degrades a malformed MCP tool list to empty rather than showing tools as approved", () => {
    const parsed = parseWithFallback({ tools: "search" }, PluginMCPToolListSchema, { tools: [] }, {
      endpoint: "GET /api/workspaces/{id}/plugins/{installationId}/mcp/{hookKey}/tools",
    });
    expect(parsed.tools).toEqual([]);
  });

  // The dangerous direction is one-sided: a response missing `approved` must
  // read as NOT approved. The opposite default would render an unpinned tool
  // with a checked box, and the administrator's next save would pin it.
  it("treats a tool with no approval field as unapproved", () => {
    const parsed = PluginMCPToolListSchema.parse({ tools: [{ name: "search" }] });
    expect(parsed.tools[0]?.approved).toBe(false);
    expect(parsed.tools[0]?.drifted).toBe(false);
  });

  it("parses a preview that reports an upgrade adding new scopes", () => {
    const parsed = PluginPreviewSchema.parse({
      manifest: { key: "com.example.hello", name: "Hello", version: "2.0.0", author: { name: "example" } },
      scopes: ["issues:read", "comments:write"],
      installed: true,
      installed_version: "1.0.0",
      added_scopes: ["comments:write"],
    });
    expect(parsed.installed).toBe(true);
    expect(parsed.installed_version).toBe("1.0.0");
    expect(parsed.added_scopes).toEqual(["comments:write"]);
  });

  it("preserves automatic schedules on both consent and installed Plugin payloads", () => {
    const schedule = { cron: "*/5 * * * *", timezone: "Asia/Shanghai" };
    const preview = PluginPreviewSchema.parse({
      manifest: {
        key: "com.example.digest",
        name: "Digest",
        version: "1.0.0",
        author: { name: "example" },
        contributes: {
          hooks: [{ key: "digest", name: "Digest", triggers: ["schedule"], schedule }],
        },
      },
    });
    expect(preview.manifest.contributes?.hooks?.[0]?.schedule).toEqual(schedule);

    const installation = PluginInstallationSchema.parse({
      id: "installation-1",
      hooks: [{
        key: "digest",
        name: "Digest",
        triggers: ["schedule"],
        schedule: { ...schedule, next_run_at: "2026-08-23T10:15:00Z" },
      }],
    });
    expect(installation.hooks[0]?.schedule?.next_run_at).toBe("2026-08-23T10:15:00Z");
  });
});

// Issue status catalog (MUL-6243). The catalog drives how every status renders,
// so a drifting or malformed response must degrade to the built-ins rather than
// leaving the UI with no statuses at all.
describe("issue status catalog schemas", () => {
  const baseStatus = {
    id: "status-1",
    workspace_id: "ws-1",
    key: "human_review",
    name: "Human Review",
    description: "Waiting on a person",
    category: "in_review",
    color: "#22c55e",
    is_system: false,
    position: 1,
    archived_at: null,
    created_at: "2026-01-01T00:00:00Z",
    updated_at: "2026-01-01T00:00:00Z",
  };

  it("parses a full catalog response", () => {
    const parsed = ListIssueStatusesResponseSchema.parse({
      statuses: [baseStatus],
      categories: ["backlog", "todo", "in_progress", "in_review", "done", "blocked", "cancelled"],
      total: 1,
    });
    expect(parsed.statuses[0]?.key).toBe("human_review");
    expect(parsed.statuses[0]?.category).toBe("in_review");
    expect(parsed.categories).toHaveLength(7);
  });

  it("falls back to the built-in categories on a malformed response", () => {
    const parsed = parseWithFallback(
      { statuses: "not-an-array", categories: 7 },
      ListIssueStatusesResponseSchema,
      EMPTY_LIST_ISSUE_STATUSES_RESPONSE,
      { endpoint: "GET /api/issue-statuses" },
    );
    expect(parsed).toEqual(EMPTY_LIST_ISSUE_STATUSES_RESPONSE);
    // The fallback still names all 7 categories, so a client talking to a
    // server that predates this endpoint can still render every built-in.
    expect(parsed.categories).toHaveLength(7);
    expect(parsed.statuses).toEqual([]);
  });

  it("defaults the optional presentation fields the server may omit", () => {
    const { color: _c, is_system: _s, position: _p, description: _d, archived_at: _a, ...minimal } = baseStatus;
    const parsed = IssueStatusEntrySchema.parse(minimal);
    expect(parsed.color).toBe("#6b7280");
    expect(parsed.is_system).toBe(false);
    expect(parsed.position).toBe(0);
    expect(parsed.archived_at).toBeNull();
  });

  // PATCH /api/issue-statuses/reorder returns the same catalog shape as the
  // list endpoint, so a malformed reorder response degrades the same way rather
  // than leaving the settings page holding an unparsed blob. (MUL-6243)
  it("falls back on a malformed reorder response", () => {
    const parsed = parseWithFallback(
      { statuses: [{ id: 1 }], total: "many" },
      ListIssueStatusesResponseSchema,
      EMPTY_LIST_ISSUE_STATUSES_RESPONSE,
      { endpoint: "PATCH /api/issue-statuses/reorder" },
    );
    expect(parsed).toEqual(EMPTY_LIST_ISSUE_STATUSES_RESPONSE);
  });

  it("keeps an unknown category as a string instead of failing the whole catalog", () => {
    // A newer server could report a category this build does not know. Dropping
    // the entry (or throwing) would leave the board unable to render issues
    // already sitting on it, so the value is carried through verbatim.
    const parsed = ListIssueStatusesResponseSchema.parse({
      statuses: [{ ...baseStatus, category: "started" }],
      categories: ["started"],
      total: 1,
    });
    expect(parsed.statuses[0]?.category).toBe("started");
  });

  it("falls back to an empty entry on a malformed single status", () => {
    const parsed = parseWithFallback(
      { id: 12345 },
      IssueStatusEntrySchema,
      EMPTY_ISSUE_STATUS_ENTRY,
      { endpoint: "POST /api/issue-statuses" },
    );
    expect(parsed).toEqual(EMPTY_ISSUE_STATUS_ENTRY);
  });
});
