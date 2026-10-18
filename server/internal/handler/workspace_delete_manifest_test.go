package handler

import (
	"context"
	"sort"
	"testing"
)

type workspaceDeleteAction string

const (
	workspaceDelete       workspaceDeleteAction = "delete"
	workspaceDeleteDetach workspaceDeleteAction = "detach"
	workspaceDeleteKeep   workspaceDeleteAction = "keep"
	workspaceDeleteSettle workspaceDeleteAction = "settle"
	// Mutable operational evidence survives, with pending delivery settled.
	workspaceDeleteArchive workspaceDeleteAction = "archive"
	// Retained tables are append-only evidence: a database trigger rejects
	// DELETE, so workspace teardown leaves their rows in place by design.
	workspaceDeleteRetain workspaceDeleteAction = "retain"
)

// workspaceDeletionManifest is the schema coverage contract for workspace
// teardown. Adding a table requires an explicit ownership decision here; the
// handler deletion graph must then implement that decision before CI passes.
var workspaceDeletionManifest = map[string]workspaceDeleteAction{
	"ai_failure_investigation":           workspaceDeleteArchive,
	"brain_event_outbox":                 workspaceDeleteArchive,
	"brain_task_link":                    workspaceDelete,
	"chat_library":                       workspaceDelete,
	"development_memory_proposal":        workspaceDeleteArchive,
	"owner_review_feedback":              workspaceDeleteArchive,
	"activity_log":                       workspaceDelete,
	"agent":                              workspaceDelete,
	"agent_builder_draft":                workspaceDelete,
	"agent_invocation_target":            workspaceDelete,
	"agent_runtime":                      workspaceDelete,
	"agent_skill":                        workspaceDelete,
	"agent_task_queue":                   workspaceDelete,
	"agent_to_label":                     workspaceDelete,
	"attachment":                         workspaceDelete,
	"autopilot":                          workspaceDelete,
	"autopilot_collaborator":             workspaceDelete,
	"autopilot_quota_period":             workspaceDelete,
	"autopilot_quota_reservation":        workspaceDelete,
	"autopilot_rule_version":             workspaceDelete,
	"autopilot_run":                      workspaceDelete,
	"autopilot_subscriber":               workspaceDelete,
	"autopilot_trigger":                  workspaceDelete,
	"channel_binding_token":              workspaceDelete,
	"channel_chat_context_generation":    workspaceDelete,
	"channel_chat_session_binding":       workspaceDelete,
	"channel_inbound_audit":              workspaceDelete,
	"channel_inbound_message_dedup":      workspaceDelete,
	"channel_installation":               workspaceDelete,
	"channel_media_pending_object":       workspaceDeleteSettle,
	"channel_outbound_card_message":      workspaceDelete,
	"channel_outbound_message":           workspaceDelete,
	"channel_task_delivery":              workspaceDelete,
	"channel_user_binding":               workspaceDelete,
	"chat_draft_restore":                 workspaceDelete,
	"chat_message":                       workspaceDelete,
	"chat_pinned_agent":                  workspaceDelete,
	"chat_session":                       workspaceDelete,
	"chat_issue_link":                    workspaceDelete,
	"client_usage_daily":                 workspaceDeleteDetach,
	"company_incident":                   workspaceDelete,
	"company_service":                    workspaceDelete,
	"company_signal_event":               workspaceDelete,
	"company_signal_quarantine":          workspaceDelete,
	"company_work_topology_binding":      workspaceDelete,
	"comment":                            workspaceDelete,
	"comment_reaction":                   workspaceDelete,
	"contact_sales_inquiry":              workspaceDeleteKeep,
	"daemon_connection":                  workspaceDelete,
	"daemon_token":                       workspaceDelete,
	"dingtalk_group_presence":            workspaceDelete,
	"dingtalk_bot_identity":              workspaceDelete,
	"dingtalk_group_route":               workspaceDelete,
	"feedback":                           workspaceDeleteDetach,
	"external_incident_episode":          workspaceDelete,
	"external_incident_occurrence":       workspaceDelete,
	"external_incident_quarantine":       workspaceDelete,
	"external_incident_source_binding":   workspaceDelete,
	"github_installation":                workspaceDelete,
	"github_pending_check_suite":         workspaceDelete,
	"github_pending_installation":        workspaceDeleteKeep,
	"github_pull_request":                workspaceDelete,
	"github_pull_request_check_run":      workspaceDelete,
	"github_pull_request_check_suite":    workspaceDelete,
	"inbox_item":                         workspaceDelete,
	"issue":                              workspaceDelete,
	"issue_current_picture":              workspaceDelete,
	"issue_external_work_receipt":        workspaceDelete,
	"issue_owner_directive":              workspaceDelete,
	"issue_workflow_concern_head":        workspaceDelete,
	"issue_view":                         workspaceDelete,
	"issue_view_preference":              workspaceDelete,
	"issue_dependency":                   workspaceDelete,
	"issue_label":                        workspaceDelete,
	"issue_property":                     workspaceDelete,
	"issue_preview_publication":          workspaceDelete,
	"issue_pull_request":                 workspaceDelete,
	"issue_reaction":                     workspaceDelete,
	"issue_source_context":               workspaceDelete,
	"issue_source_context_object_intent": workspaceDeleteSettle,
	"issue_status":                       workspaceDelete,
	"issue_subscriber":                   workspaceDelete,
	"issue_to_label":                     workspaceDelete,
	"issue_vcs_pull_request":             workspaceDelete,
	"issue_workflow_action_receipt":      workspaceDelete,
	"issue_workflow_attention":           workspaceDelete,
	"issue_workflow_authority":           workspaceDelete,
	"issue_workflow_budget":              workspaceDelete,
	"issue_workflow_candidate":           workspaceDelete,
	"issue_workflow_command":             workspaceDelete,
	"issue_workflow_decision":            workspaceDelete,
	"issue_workflow_decision_response":   workspaceDelete,
	"issue_workflow_evidence":            workspaceDelete,
	"issue_workflow_event":               workspaceDelete,
	"issue_workflow_execution":           workspaceDelete,
	"issue_workflow_incident":            workspaceDelete,
	"issue_workflow_outcome":             workspaceDelete,
	"issue_workflow_plan":                workspaceDelete,
	"issue_workflow_release":             workspaceDelete,
	"issue_workflow_review":              workspaceDelete,
	"issue_workflow_scope":               workspaceDelete,
	"issue_workflow_target_head":         workspaceDelete,
	"lark_binding_token":                 workspaceDelete,
	"lark_chat_session_binding":          workspaceDelete,
	"lark_inbound_audit":                 workspaceDelete,
	"lark_inbound_message_dedup":         workspaceDelete,
	"lark_installation":                  workspaceDelete,
	"lark_outbound_card_message":         workspaceDelete,
	"lark_user_binding":                  workspaceDelete,
	"member":                             workspaceDelete,
	"agent_mcp_server":                   workspaceDelete,
	"workspace_mcp_server":               workspaceDelete,
	"notification_preference":            workspaceDelete,
	"personal_access_token":              workspaceDeleteKeep,
	"pinned_item":                        workspaceDelete,
	"plugin_installation":                workspaceDelete,
	"plugin_hook_schedule":               workspaceDelete,
	"plugin_invocation":                  workspaceDelete,
	"plugin_storage":                     workspaceDelete,
	"plugin_secret":                      workspaceDelete,
	"plugin_package":                     workspaceDelete,
	"plugin_package_version":             workspaceDelete,
	"plugin_package_file":                workspaceDelete,
	"project":                            workspaceDelete,
	"project_resource":                   workspaceDelete,
	"quick_action":                       workspaceDelete,
	"runtime_profile":                    workspaceDelete,
	"schema_migrations":                  workspaceDeleteKeep,
	"seat_capacity_outbox":               workspaceDeleteSettle,
	"skill":                              workspaceDelete,
	"skill_file":                         workspaceDelete,
	"skill_to_label":                     workspaceDelete,
	"squad":                              workspaceDelete,
	"squad_member":                       workspaceDelete,
	"squad_workflow_template":            workspaceDelete,
	"sys_cron_executions":                workspaceDeleteKeep,
	"task_message":                       workspaceDelete,
	"task_token":                         workspaceDelete,
	"task_usage":                         workspaceDelete,
	"task_usage_hourly":                  workspaceDelete,
	"task_usage_hourly_dirty":            workspaceDelete,
	"task_usage_hourly_rollup_state":     workspaceDeleteKeep,
	"user":                               workspaceDeleteKeep,
	"user_composio_connection":           workspaceDeleteKeep,
	"vcs_commit_status":                  workspaceDelete,
	"vcs_connection":                     workspaceDelete,
	"vcs_pull_request":                   workspaceDelete,
	"verification_code":                  workspaceDeleteKeep,
	"webhook_delivery":                   workspaceDelete,
	"workspace":                          workspaceDelete,
	"workspace_invitation":               workspaceDelete,
	"workspace_share_link":               workspaceDelete,

	"agent_task_context_manifest":                  workspaceDelete,
	"agent_task_continuation_receipt":              workspaceDelete,
	"agent_task_loop_observation":                  workspaceDelete,
	"agent_task_usage_sample":                      workspaceDelete,
	"issue_workflow_execution_checkpoint_snapshot": workspaceDelete,

	"owner_notification":         workspaceDelete,
	"owner_notification_attempt": workspaceDelete,
	"owner_notification_binding": workspaceDelete,

	"guarded_execution_launch_consumption":       workspaceDeleteRetain,
	"guarded_task_capability_grant":              workspaceDeleteRetain,
	"guarded_task_capability_revocation":         workspaceDeleteRetain,
	"issue_workflow_consequence_receipt":         workspaceDeleteRetain,
	"issue_workflow_regression_change_record":    workspaceDeleteRetain,
	"issue_workflow_regression_rejected_attempt": workspaceDeleteRetain,
	"qualified_runtime_binding_receipt":          workspaceDeleteRetain,
	"qualified_runtime_binding_revocation":       workspaceDeleteRetain,
}

func TestWorkspaceDeletionManifestClassifiesWorkflowTablesForExplicitDelete(t *testing.T) {
	for _, table := range []string{
		"company_incident",
		"company_service",
		"company_signal_event",
		"company_signal_quarantine",
		"company_work_topology_binding",
		"issue_current_picture",
		"issue_external_work_receipt",
		"issue_owner_directive",
		"issue_workflow_concern_head",
		"issue_workflow_action_receipt",
		"issue_workflow_attention",
		"issue_workflow_authority",
		"issue_workflow_budget",
		"issue_workflow_candidate",
		"issue_workflow_command",
		"issue_workflow_decision",
		"issue_workflow_decision_response",
		"issue_workflow_evidence",
		"issue_workflow_event",
		"issue_workflow_execution",
		"issue_workflow_incident",
		"issue_workflow_outcome",
		"issue_workflow_plan",
		"issue_workflow_release",
		"issue_workflow_review",
		"issue_workflow_scope",
		"issue_workflow_target_head",
		"squad_workflow_template",
	} {
		if got := workspaceDeletionManifest[table]; got != workspaceDelete {
			t.Errorf("workspace deletion manifest classifies %s as %q, want explicit delete", table, got)
		}
	}
}

func TestWorkspaceDeletionManifestCoversPublicSchema(t *testing.T) {
	if testPool == nil {
		t.Skip("database not available")
	}

	rows, err := testPool.Query(context.Background(), `
SELECT tablename
FROM pg_tables
WHERE schemaname = 'public'
ORDER BY tablename
`)
	if err != nil {
		t.Fatalf("list public tables: %v", err)
	}
	defer rows.Close()

	actual := make(map[string]struct{}, len(workspaceDeletionManifest))
	for rows.Next() {
		var table string
		if err := rows.Scan(&table); err != nil {
			t.Fatalf("scan public table: %v", err)
		}
		actual[table] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate public tables: %v", err)
	}

	var unclassified, missing []string
	for table := range actual {
		if _, ok := workspaceDeletionManifest[table]; !ok {
			unclassified = append(unclassified, table)
		}
	}
	for table := range workspaceDeletionManifest {
		if _, ok := actual[table]; !ok {
			missing = append(missing, table)
		}
	}
	sort.Strings(unclassified)
	sort.Strings(missing)
	if len(unclassified) > 0 || len(missing) > 0 {
		t.Fatalf("workspace deletion manifest drift: unclassified=%v missing=%v", unclassified, missing)
	}

	workspaceColumns, err := testPool.Query(context.Background(), `
SELECT table_name
FROM information_schema.columns
WHERE table_schema = 'public'
  AND column_name = 'workspace_id'
`)
	if err != nil {
		t.Fatalf("list workspace_id columns: %v", err)
	}
	defer workspaceColumns.Close()

	withWorkspaceID := make(map[string]struct{})
	for workspaceColumns.Next() {
		var table string
		if err := workspaceColumns.Scan(&table); err != nil {
			t.Fatalf("scan workspace_id table: %v", err)
		}
		withWorkspaceID[table] = struct{}{}
	}
	if err := workspaceColumns.Err(); err != nil {
		t.Fatalf("iterate workspace_id tables: %v", err)
	}

	for table, action := range workspaceDeletionManifest {
		_, hasWorkspaceID := withWorkspaceID[table]
		switch action {
		case workspaceDeleteKeep:
			if hasWorkspaceID {
				t.Errorf("KEEP table %s gained workspace_id; classify its teardown behavior", table)
			}
		case workspaceDeleteDetach, workspaceDeleteSettle, workspaceDeleteArchive:
			if !hasWorkspaceID {
				t.Errorf("%s table %s lost workspace_id; update its teardown selector", action, table)
			}
		case workspaceDeleteRetain:
			// tgtype bits: 2 = BEFORE, 8 = DELETE.
			var appendOnly bool
			if err := testPool.QueryRow(context.Background(), `
SELECT EXISTS (
    SELECT 1 FROM pg_trigger t JOIN pg_class c ON c.oid = t.tgrelid
    WHERE c.relname = $1 AND NOT t.tgisinternal AND (t.tgtype & 10) = 10
)`, table).Scan(&appendOnly); err != nil {
				t.Fatalf("check append-only trigger on %s: %v", table, err)
			}
			if !appendOnly {
				t.Errorf("RETAIN table %s has no BEFORE DELETE trigger; classify its teardown behavior", table)
			}
		}
	}
}
