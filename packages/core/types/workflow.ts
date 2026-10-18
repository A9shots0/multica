/**
 * Normalized workflow-v2 owner decision contract.
 *
 * State strings are deliberately open so an installed client can carry a
 * newer server state without treating it as approval. Consumers must switch
 * on the known values and handle the default branch as non-authoritative.
 */
export type WorkflowDecisionType =
  | "authorization"
  | "approval"
  | "direction"
  | "factual_input"
  | "budget"
  | "cancellation"
  | "external_step"
  | "authentication"
  | (string & {});

export type WorkflowDecisionState =
  | "unanswered"
  | "answered"
  | "expired"
  | "cancelled"
  | "superseded"
  | (string & {});

export type WorkflowDecisionResolutionState =
  | "open"
  | "resuming"
  | "resolved"
  | "superseded"
  | (string & {});

export type WorkflowAttentionReadState =
  | "unread"
  | "read"
  | (string & {});

export type WorkflowDecisionResponseType =
  | "approve"
  | "reject"
  | "request_changes"
  | "partial_acceptance"
  | "provide_fact"
  | "ask_question"
  | "hold"
  | "cancel"
  | "change_scope"
  | "change_budget"
  | "external_step_complete"
  | "authentication_complete"
  | (string & {});

/** Outcomes the accepted v2 command service can currently settle atomically. */
export type AtomicWorkflowDecisionResponseType = "approve" | "reject";

export interface WorkflowDecision {
  id: string;
  revision: number;
  type: WorkflowDecisionType;
  state: WorkflowDecisionState;
  resolution_state: WorkflowDecisionResolutionState;
  question: string;
  recommendation: string;
  consequence: string;
  options: string[];
  scope_revision: number;
  authority_revision: number;
  candidate_id: string;
  operation_id: string;
  branch_id: string;
}

export interface WorkflowDecisionAttention {
  id: string;
  revision: number;
  read_state: WorkflowAttentionReadState;
  acknowledged: boolean;
  resolution_state: WorkflowDecisionResolutionState;
}

export interface WorkflowDecisionContinuation {
  action_receipt_id: string;
  state: string;
}

export interface WorkflowDecisionPacket {
  contract_version: 1;
  issue_id: string;
  decision: WorkflowDecision;
  attention: WorkflowDecisionAttention;
  continuation: WorkflowDecisionContinuation | null;
  /** Reserved for a server-provided replacement summary. Never inferred. */
  replacement: Record<string, unknown> | null;
}

export interface RespondWorkflowDecisionRequest {
  expected_revision: number;
  outcome: AtomicWorkflowDecisionResponseType;
  comment?: string;
}

export interface WorkflowDecisionResponseReceipt {
  decision_id: string;
  decision_revision: number;
  response_id: string;
  response_type: WorkflowDecisionResponseType;
  resolution_state: WorkflowDecisionResolutionState;
  action_receipt_id: string;
  action_state: string;
  command_id: string;
  current_picture_revision: number;
}

export type OwnerDirectiveType =
  | "correct"
  | "request_changes"
  | "partial_acceptance"
  | "change_scope"
  | "cancel";

export interface CreateOwnerDirectiveRequest {
  owner_runtime_route?: OwnerRuntimeRouteRequest;
  intake_id?: string;
  inbox_item_id?: string;
  expected_issue_revision: number;
  directive: OwnerDirectiveType;
  content: string;
}

export interface OwnerRuntimeRouteRequest {
  squad_id: string;
  daemon_id: string;
  implementation: { agent_id: string; runtime_id: string };
  review: { agent_id: string; runtime_id: string };
  release: { agent_id: string; runtime_id: string };
  verification: { agent_id: string; runtime_id: string };
  allow_brief_deployment_interruption: boolean;
}

export interface OwnerDirectiveReceipt {
  id: string;
  issue_id: string;
  revision: number;
  directive: OwnerDirectiveType;
  state: "recorded" | "applied" | "cancelled" | "failed";
  comment_id: string;
  issue_revision: number;
}

export interface OwnerNotificationReceipt {
  id: string;
  event_type: "owner_decision" | "important_incident" | "production_regression" | "rollback_started" | "verified_completion" | (string & {});
  delivery_mode: "dry_run" | "live_test" | "live" | (string & {});
  state: "admitted" | "claimed" | "provider_accepted" | "delivered" | "ambiguous" | "retryable" | "rejected" | "failed" | "acknowledged" | "resolved" | (string & {});
  provider_state: string | null;
  admitted_at: string;
  delivered_at: string | null;
}

interface WorkflowDecisionStaleConflictBase {
  code: "decision_stale";
  error: string;
  submitted_decision_id: string;
  submitted_revision: number;
}

export type WorkflowDecisionStaleConflict = WorkflowDecisionStaleConflictBase & (
  | { current_decision: WorkflowDecisionPacket; terminal_state?: string }
  | { current_decision: null; terminal_state: string }
);
