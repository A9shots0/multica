import type { Label } from "./label";
import type { IssuePropertyValues } from "./property";

/**
 * A status CATEGORY — the behavior equivalence class an issue's status belongs
 * to. There are exactly 7, and each is also the key of the built-in status that
 * defines it, which is why this stayed a closed union while `Issue.status`
 * became open. Board columns, filters and the presentation config are all keyed
 * off categories, so their shape is fixed no matter how many custom statuses a
 * workspace defines. (MUL-6243)
 */
export type IssueStatusCategory =
  | "backlog"
  | "todo"
  | "in_progress"
  | "in_review"
  | "done"
  | "blocked"
  | "cancelled";

/**
 * A status KEY as stored on the issue: one of the 7 built-ins, or a custom key
 * an admin defined for this workspace.
 *
 * OPEN by design. `(string & {})` keeps editor autocomplete for the 7 built-ins
 * while accepting any catalog key, which is what the server has always been
 * able to send. Anything that needs presentation (label, colour, board column)
 * must resolve the key to its CATEGORY first — `useIssueStatuses(wsId)` in a
 * component, `statusCategoryOfKey` in a pure path. (MUL-6243)
 */
export type IssueStatus = IssueStatusCategory | (string & {});

export type IssuePriority = "urgent" | "high" | "medium" | "low" | "none";

export type IssueAssigneeType = "member" | "agent" | "squad";

export interface IssueReaction {
  id: string;
  issue_id: string;
  actor_type: string;
  actor_id: string;
  emoji: string;
  created_at: string;
  issue_revision?: number;
}

/**
 * Per-issue metadata is a flat KV map agents use to record pipeline state
 * (PR number, pipeline_status, waiting_on, ...). Values are primitives only —
 * string / number / bool — enforced by both the API and the DB. Always
 * present in responses (empty object when unset) so reads don't need a
 * nil guard on the parent field.
 */
export type IssueMetadataValue = string | number | boolean;
export type IssueMetadata = Record<string, IssueMetadataValue>;

export interface SourceContextAttachment {
  id: string;
  source_attachment_id?: string;
  owner_type: "issue" | "comment" | (string & {});
  owner_id: string;
  filename: string;
  content_type: string;
  size_bytes: number;
  created_at: string;
}

export interface SourceContextAuthor {
  type: "member" | "agent" | (string & {});
  id: string;
  name: string;
}

export interface SourceContextIssueSnapshot {
  id: string;
  identifier: string;
  number: number;
  title: string;
  description: string | null;
  created_at: string;
  updated_at: string;
  revision: number;
  attachments: SourceContextAttachment[];
}

export interface SourceContextCommentSnapshot {
  id: string;
  parent_id: string | null;
  type: string;
  content: string;
  author: SourceContextAuthor;
  created_at: string;
  updated_at: string;
  revision: number;
  attachments: SourceContextAttachment[];
}

export interface SourceContextSnapshot {
  /** Capture metadata is present on a persisted detail snapshot and omitted
   * from the pre-submit preview payload. */
  version?: number;
  captured_by_user_id?: string;
  captured_at?: string;
  source_issue: SourceContextIssueSnapshot;
  comment_thread: SourceContextCommentSnapshot[];
  anchor_comment_id: string;
}

export interface SourceContextLimitUsage {
  comment_count: number;
  text_bytes: number;
  attachment_count: number;
  attachment_bytes: number;
}

export interface SourceContextPreview extends SourceContextSnapshot {
  capture_token: string;
  limits: SourceContextLimitUsage;
}

export interface SourceContextAuthorState {
  type: string;
  id: string;
  captured_name: string;
  current_name?: string;
  state: string;
}

export interface SourceContextDescriptionAttachmentChange {
  kind: "added" | "removed" | "replaced" | (string & {});
  attachment_id: string;
  filename: string;
  previous_filename?: string;
}

export interface SourceContextChangeDetails {
  changed_comment_ids: string[];
  added_comments?: SourceContextCommentSnapshot[];
  removed_comment_ids?: string[];
  description_attachment_changes: SourceContextDescriptionAttachmentChange[];
}

export interface IssueSourceContext {
  id: string;
  version: number;
  usage: "read_only_historical_background" | (string & {});
  captured_at: string;
  display_state: "unchanged" | "changed" | "deleted" | "unavailable" | (string & {});
  source_issue_state: "unchanged" | "changed" | "deleted" | "unavailable" | (string & {});
  comment_thread_state: "unchanged" | "changed" | "unavailable" | (string & {});
  anchor_comment_state: "available" | "deleted" | "unavailable" | (string & {});
  can_open_current_source: boolean;
  change_reasons?: string[];
  change_details?: SourceContextChangeDetails;
  current_source?: { issue_id: string; identifier: string; anchor_comment_id: string };
  source_author_state?: SourceContextAuthorState[];
  snapshot: SourceContextSnapshot;
}

export type IssueCurrentPictureObject = Record<string, unknown>;

export interface IssueCurrentPictureSourceRevision {
  type: string;
  id: string;
  revision: number;
}

export type OwnerIssueStage =
  | "needs_owner"
  | "recovering"
  | "technical_stop"
  | "blocked"
  | "verifying"
  | "releasing"
  | "in_review"
  | "active"
  | "incoming"
  | "complete"
  | "status_unavailable";

export interface OwnerIssueReference {
  type: string;
  id: string;
  revision: number;
}

export interface OwnerIssueSignal {
  code:
    | "projection_unavailable"
    | "owner_attention_required"
    | "current_incident"
    | "external_dependency"
    | "recovery_active"
    | "technical_stop"
    | "work_blocked"
    | "verification_pending"
    | "release_in_progress"
    | "independent_review"
    | "automatic_continuation"
    | "work_active"
    | "waiting_automatic"
    | "outcome_complete"
    | "outcome_done"
    | "verified_live"
    | "current_regression"
    | "rollback_in_progress"
    | "service_restored_outcome_open";
  references: OwnerIssueReference[];
}

export interface OwnerIssueNextAction {
  kind: "answer_decision" | "continue_workflow" | "inspect_incident";
  automatic: boolean;
  actor_type?: string;
  actor_id?: string;
}

export interface OwnerMeaningfulChange {
  code:
    | "outcome_changed" | "scope_changed" | "authority_changed" | "execution_changed"
    | "decision_requested" | "decision_answered" | "candidate_changed" | "review_recorded"
    | "release_changed" | "evidence_recorded" | "incident_changed" | "action_changed"
    | "budget_changed" | "target_changed";
  result_code: "recorded" | "started" | "succeeded" | "failed" | "resolved" | "superseded" | "changed";
  occurred_at: string;
  reference: OwnerIssueReference;
}

export interface IssueMeaningfulActivityPage {
  schema_version: 1;
  issue_id: string;
  current_picture_revision: number;
  items: OwnerMeaningfulChange[];
  next_cursor: string | null;
}

export interface ExternalWorkUpdateEvidence {
  type: "commit" | "pr" | "test" | "deployment" | "runtime" | "artifact" | "owner_ui" | "owner_report" | "controller";
  reference: string;
}

export interface ExternalWorkResourceUsage {
  source: "provider_reported" | "measured" | "unavailable";
  attempts: number | null;
  wall_clock_seconds: number | null;
  active_compute_seconds: number | null;
  input_tokens: number | null;
  output_tokens: number | null;
  cost_usd: number | null;
  capacity_window_ref: string | null;
}

export interface ExternalWorkUpdate {
  id: string;
  provider: "codex" | "claude" | "chatgpt" | "copilot" | "evo" | "other";
  reported_state: "working" | "waiting_external" | "waiting_owner" | "blocked" | "complete" | "stopped";
  observed_at: string;
  recorded_at: string;
  receipt_revision: number;
  current_milestone: string;
  summary: string;
  evidence: ExternalWorkUpdateEvidence[];
  evidence_total: number;
  blockers: string[];
  blocker_total: number;
  next_action: string;
  acceptance_stage: "local" | "source_ready" | "merged" | "release_ready" | "deployed" | "production_verified";
  acceptance_status: "not_tested" | "partial" | "accepted" | "rejected";
  resource_usage?: ExternalWorkResourceUsage;
  provider_report_only: true;
}

export interface ExternalWorkUpdatesResponse {
  schema_version: 1;
  issue_id: string;
  items: ExternalWorkUpdate[];
}

export interface OwnerIssueSummary {
  schema_version: 1;
  current_picture_revision: number;
  stage: OwnerIssueStage;
  signals: OwnerIssueSignal[];
  next_action: OwnerIssueNextAction | null;
  latest_meaningful_change: OwnerMeaningfulChange | null;
}

export type OwnerReleasePhase =
  | "reviewed"
  | "releasing"
  | "deployed_checking"
  | "verified_live"
  | "regressed"
  | "rolling_back"
  | "rolled_back"
  | "technical_stop";

export type IssueReleaseEvidenceGateState =
  | "missing"
  | "running"
  | "passed"
  | "failed"
  | "stale"
  | "contradicted";

export interface IssueReleaseEvidenceGate {
  state: IssueReleaseEvidenceGateState;
  refs: string[];
  observed_at: string | null;
  limitations: string[];
}

export interface IssueReleaseGates {
  review: IssueReleaseEvidenceGate;
  build: IssueReleaseEvidenceGate;
  deployment: IssueReleaseEvidenceGate;
  exposure: IssueReleaseEvidenceGate;
  runtime: IssueReleaseEvidenceGate;
  business: IssueReleaseEvidenceGate;
  regression: IssueReleaseEvidenceGate;
  rollback: IssueReleaseEvidenceGate;
  restoration: IssueReleaseEvidenceGate;
}

export interface IssueReleaseBinding {
  release_id: string;
  release_revision: number;
  candidate_id: string;
  candidate_revision: number;
  source_revision: string;
  artifact_digest: string;
  build_manifest_digest: string;
  scope_revision: number;
  acceptance_version: number;
  authority_revision: number;
  target_key: string;
  target_generation: number;
  deployed_revision: string | null;
}

export interface IssueReleaseNextAction {
  kind: string;
  automatic: boolean;
  actor_type: string | null;
  action_id: string | null;
  due_at: string | null;
}

export interface IssueReleaseTechnicalStop {
  code: string;
  reason: string;
  retryable: boolean;
}

export interface IssueReleaseHistory {
  was_verified_live: boolean;
  last_verified_live_at: string | null;
}

export interface IssueReleaseStatus {
  schema_version: 1;
  phase: OwnerReleasePhase;
  binding: IssueReleaseBinding;
  gates: IssueReleaseGates;
  historical: IssueReleaseHistory;
  next_action: IssueReleaseNextAction | null;
  stop: IssueReleaseTechnicalStop | null;
}

export interface IssueCurrentPictureDocumentV2 {
  contract_version: 2;
  original_business_request: IssueCurrentPictureObject;
  outcome: IssueCurrentPictureObject;
  historical_completion: IssueCurrentPictureObject;
  scope: IssueCurrentPictureObject;
  authority: IssueCurrentPictureObject;
  execution: IssueCurrentPictureObject;
  active_branches: IssueCurrentPictureObject[];
  primary_branch: IssueCurrentPictureObject;
  release: IssueCurrentPictureObject;
  health: string;
  attention: IssueCurrentPictureObject;
  current_milestone: string;
  latest_meaningful_change: IssueCurrentPictureObject;
  next_automatic_action: IssueCurrentPictureObject;
  active_actor: IssueCurrentPictureObject;
  wait_reason: IssueCurrentPictureObject;
  candidate: IssueCurrentPictureObject;
  target: IssueCurrentPictureObject;
  evidence_summary: IssueCurrentPictureObject;
  authority_summary: IssueCurrentPictureObject;
  unresolved_decisions: IssueCurrentPictureObject[];
  unresolved_incidents: IssueCurrentPictureObject[];
  unresolved_dependencies: IssueCurrentPictureObject[];
  budgets: IssueCurrentPictureObject[];
  supersession: IssueCurrentPictureObject;
  terminal_reason: string;
  next_actor: IssueCurrentPictureObject;
}

export interface IssueCurrentPictureDocumentV3 extends Omit<IssueCurrentPictureDocumentV2, "contract_version"> {
  contract_version: 3;
  release_status: IssueReleaseStatus;
}

/** Version 2 compact workflow projection embedded in issue list surfaces. */
export interface IssueCurrentPictureCompactV2 {
  contract_version: 2;
  owner_summary: OwnerIssueSummary;
  revision: number;
  scope_revision: number | null;
  authority_revision: number | null;
  outcome_state: string;
  outcome_open: boolean;
  execution_state: string;
  release_state: string;
  health_state: string;
  attention_state: string;
  current_milestone: string;
  latest_meaningful_change: IssueCurrentPictureObject;
  next_automatic_action: IssueCurrentPictureObject;
  active_actor: IssueCurrentPictureObject;
  wait_reason: IssueCurrentPictureObject;
  candidate: IssueCurrentPictureObject;
  target: IssueCurrentPictureObject;
  evidence_summary: IssueCurrentPictureObject;
  unresolved_decisions: IssueCurrentPictureObject[];
  unresolved_incidents: IssueCurrentPictureObject[];
  unresolved_dependencies: IssueCurrentPictureObject[];
  terminal_reason: string;
  next_actor: IssueCurrentPictureObject;
  generated_at: string;
}

/** Version 3 adds the normalized, fail-closed owner release projection. */
export interface IssueCurrentPictureCompactV3 extends Omit<IssueCurrentPictureCompactV2, "contract_version"> {
  contract_version: 3;
  release_status: IssueReleaseStatus;
}

export type IssueCurrentPictureCompact = IssueCurrentPictureCompactV2 | IssueCurrentPictureCompactV3;

/** Complete version 2 Current Picture returned by the detail endpoint. */
interface IssueCurrentPictureDetailFields {
  workspace_id: string;
  issue_id: string;
  authority_summary: IssueCurrentPictureObject;
  budget_summary: IssueCurrentPictureObject[];
  supersession_summary: IssueCurrentPictureObject;
  source_revisions: IssueCurrentPictureSourceRevision[];
  updated_at: string;
}

export interface IssueCurrentPictureV2 extends IssueCurrentPictureCompactV2, IssueCurrentPictureDetailFields {
  document: IssueCurrentPictureDocumentV2;
}

export interface IssueCurrentPictureV3 extends IssueCurrentPictureCompactV3, IssueCurrentPictureDetailFields {
  document: IssueCurrentPictureDocumentV3;
}

export type IssueCurrentPicture = IssueCurrentPictureV2 | IssueCurrentPictureV3;

export interface Issue {
  owner_question?: { id: string; task_id: string; question: string; consequence: string };
  id: string;
  workspace_id: string;
  number: number;
  identifier: string;
  title: string;
  description: string | null;
  status: IssueStatus;
  /**
   * The category `status` belongs to, when the endpoint resolved it. Optional
   * because a BUILT-IN status is its own category and needs no resolution —
   * use `issueStatusCategory(issue)` rather than reading this directly.
   * (MUL-6243)
   */
  status_category?: IssueStatusCategory;
  /**
   * A CUSTOM status's display name, carried beside the key. Empty for the 7
   * built-ins, which are localized from the key — prefer `useStatusLabel`,
   * which handles both and stays correct when an admin renames a status.
   *
   * Optional only for compatibility with a server that predates it; a current
   * server always sends the field. (MUL-6749)
   */
  status_name?: string;
  priority: IssuePriority;
  assignee_type: IssueAssigneeType | null;
  assignee_id: string | null;
  creator_type: IssueAssigneeType;
  creator_id: string;
  parent_issue_id: string | null;
  project_id: string | null;
  position: number;
  // Ordered barrier group among sibling sub-issues (null = unstaged). The
  // parent assignee is notified/woken only when every sub-issue in a stage
  // finishes; see server/internal/handler/issue_child_done.go.
  stage: number | null;
  // Calendar days as date-only "YYYY-MM-DD" (no time, no timezone). Use the
  // helpers in @multica/core/issues/date to format/compare — never `new Date()`
  // + local formatting, which shifts the day by the viewer's offset.
  start_date: string | null;
  due_date: string | null;
  metadata: IssueMetadata;
  // Custom property values keyed by property definition id. Always present
  // in responses (empty object when unset), mirroring `metadata`.
  properties: IssuePropertyValues;
  reactions?: IssueReaction[];
  labels?: Label[];
  created_at: string;
  updated_at: string;
  /** Monotonic server revision; absent when connected to an older backend. */
  revision?: number;
  /**
   * Null until the server's historical activity backfill reaches this row.
   * This RFC3339 timestamp may include sub-second precision while legacy
   * created_at/updated_at values are second-precision; parse before comparing.
   */
  last_activity_at?: string | null;
  /** Present only on issue detail responses for issues created from a comment. */
  source_context?: IssueSourceContext;
  /** Optional compact workflow-v2 projection on issue list surfaces. */
  current_picture?: IssueCurrentPictureCompact;
}
