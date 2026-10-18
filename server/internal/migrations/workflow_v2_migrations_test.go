package migrations

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestWorkflowV2RecordsStayFKFreeAndIndexFree(t *testing.T) {
	path := filepath.Join(realMigrationsDir(t), "466_issue_workflow_v2_records.up.sql")
	body := readWorkflowV2File(t, path)
	sql := stripSQLLineComments(body)

	forbidden := regexp.MustCompile(`(?i)\b(REFERENCES|FOREIGN\s+KEY|CASCADE|PRIMARY\s+KEY|CREATE\s+(UNIQUE\s+)?INDEX)\b`)
	if match := forbidden.FindString(sql); match != "" {
		t.Fatalf("workflow v2 records migration must use application relationships and separate concurrent indexes; found %q", match)
	}

	for _, table := range []string{
		"issue_workflow_outcome",
		"issue_workflow_scope",
		"issue_workflow_authority",
		"issue_workflow_execution",
		"issue_workflow_decision",
		"issue_workflow_decision_response",
		"issue_workflow_candidate",
		"issue_workflow_review",
		"issue_workflow_release",
		"issue_workflow_evidence",
		"issue_workflow_incident",
		"issue_workflow_command",
		"issue_workflow_action_receipt",
		"issue_workflow_budget",
		"issue_workflow_attention",
		"issue_workflow_target_head",
		"issue_current_picture",
	} {
		if !strings.Contains(sql, "CREATE TABLE "+table+" ") {
			t.Errorf("workflow v2 records migration is missing %s", table)
		}
	}

	if !strings.Contains(body, "grandfathered") || !strings.Contains(body, "squad_id primary key") {
		t.Error("migration 466 must document the immutable migration 465 primary-key exception")
	}

	downPath := filepath.Join(realMigrationsDir(t), "466_issue_workflow_v2_records.down.sql")
	down := readWorkflowV2File(t, downPath)
	if !strings.Contains(down, "RAISE EXCEPTION 'workflow v2 contains durable records") {
		t.Error("workflow v2 down migration must refuse destructive rollback while durable records exist")
	}

	for _, constraint := range []string{
		"squad_workflow_template_squad_id_fkey",
		"squad_workflow_template_workspace_id_fkey",
		"squad_workflow_template_implementer_id_fkey",
		"squad_workflow_template_reviewer_id_fkey",
		"squad_workflow_template_release_agent_id_fkey",
		"squad_workflow_template_verifier_id_fkey",
	} {
		if !strings.Contains(sql, "DROP CONSTRAINT IF EXISTS "+constraint) {
			t.Errorf("migration 466 must remove applied migration 465 constraint %s", constraint)
		}
	}
}

func TestWorkflowV2ReleaseAuthoritySnapshotMigrationIsCompleteAndGuarded(t *testing.T) {
	up := readWorkflowV2File(t, filepath.Join(realMigrationsDir(t), "516_issue_workflow_release_authority_snapshot.up.sql"))
	for _, required := range []string{"authority_envelope_digest", "owner_evidence_revision", "exposure_policy_digest", "rollback_target_release_id", "irreversible_effects_digest", "reviewed_review_id", "reviewed_review_revision", "attempt_contract_version < 3"} {
		if !strings.Contains(up, required) {
			t.Fatalf("516 up missing %q", required)
		}
	}
	down := readWorkflowV2File(t, filepath.Join(realMigrationsDir(t), "516_issue_workflow_release_authority_snapshot.down.sql"))
	if !strings.Contains(down, "refusing destructive rollback") || !strings.Contains(down, "attempt_contract_version = 3") {
		t.Fatal("516 down must fail closed while durable release authority snapshots exist")
	}
}

func TestWorkflowV2ReleaseEffectDueIndexIsConcurrentAndComplete(t *testing.T) {
	up := strings.TrimSpace(stripSQLLineComments(readWorkflowV2File(t, filepath.Join(realMigrationsDir(t), "517_issue_workflow_release_effect_due_idx.up.sql"))))
	if strings.Count(up, ";") != 1 || !strings.Contains(strings.ToUpper(up), "INDEX CONCURRENTLY") {
		t.Fatal("517 up must be one concurrent index statement")
	}
	for _, actionType := range []string{"deploy_exact_release", "expose_exact_release", "verify_runtime_exact_release", "verify_business_exact_release"} {
		if !strings.Contains(up, actionType) {
			t.Fatalf("517 up omits %s", actionType)
		}
	}
	down := strings.TrimSpace(stripSQLLineComments(readWorkflowV2File(t, filepath.Join(realMigrationsDir(t), "517_issue_workflow_release_effect_due_idx.down.sql"))))
	if strings.Count(down, ";") != 1 || !strings.Contains(strings.ToUpper(down), "DROP INDEX CONCURRENTLY") {
		t.Fatal("517 down must be one concurrent drop statement")
	}
}

func TestWorkflowV2RecoveryIdentityAndIndexesAreGuarded(t *testing.T) {
	identity := readWorkflowV2File(t, filepath.Join(realMigrationsDir(t), "518_issue_workflow_incident_recovery_identity.up.sql"))
	for _, required := range []string{"regression_fingerprint", "recovery_release_id", "rollback_safe", "containment_required", "irreversible"} {
		if !strings.Contains(identity, required) {
			t.Fatalf("518 up missing %q", required)
		}
	}
	down := readWorkflowV2File(t, filepath.Join(realMigrationsDir(t), "518_issue_workflow_incident_recovery_identity.down.sql"))
	if !strings.Contains(down, "refusing destructive rollback") {
		t.Fatal("518 down must fail closed while recovery identity is populated")
	}
	guard := readWorkflowV2File(t, filepath.Join(realMigrationsDir(t), "519_issue_workflow_active_regression_guard.up.sql"))
	if !strings.Contains(guard, "HAVING count(*) > 1") || !strings.Contains(guard, "RAISE EXCEPTION") {
		t.Fatal("519 must fail closed before installing active-regression uniqueness")
	}
	for _, tc := range []struct {
		file string
		want []string
	}{
		{"520_issue_workflow_active_regression_unique_idx.up.sql", []string{"CREATE UNIQUE INDEX CONCURRENTLY", "regression_fingerprint", "superseded_at IS NULL"}},
		{"521_issue_workflow_recovery_effect_due_idx.up.sql", []string{"CREATE INDEX CONCURRENTLY", "monitor_exact_release", "rollback_exact_release", "verify_restoration_exact_release"}},
	} {
		sql := strings.TrimSpace(stripSQLLineComments(readWorkflowV2File(t, filepath.Join(realMigrationsDir(t), tc.file))))
		if strings.Count(sql, ";") != 1 {
			t.Fatalf("%s must contain exactly one statement", tc.file)
		}
		for _, want := range tc.want {
			if !strings.Contains(sql, want) {
				t.Fatalf("%s missing %q", tc.file, want)
			}
		}
	}
}

func TestWorkflowV2IndexMigrationsAreConcurrentSingleStatements(t *testing.T) {
	files := append(migrationFilesForLint(t, "4??_*.sql"), migrationFilesForLint(t, "50[12567]_*.sql")...)
	found := 0
	for _, path := range files {
		base := filepath.Base(path)
		if base < "467_" || base > "507_zzzz" {
			continue
		}
		if base >= "499_" && base < "501_" {
			continue
		}
		// 496 creates/drops the FK-free concern-head table. The adjacent 497
		// and 498 migrations keep its indexes concurrent and single-statement.
		if strings.HasPrefix(base, "496_") {
			continue
		}
		found++
		sql := strings.TrimSpace(stripSQLLineComments(readWorkflowV2File(t, path)))
		if strings.Count(sql, ";") != 1 || !strings.HasSuffix(sql, ";") {
			t.Errorf("%s must contain exactly one SQL statement", base)
		}
		upper := strings.ToUpper(sql)
		if strings.Contains(upper, "CREATE INDEX") || strings.Contains(upper, "CREATE UNIQUE INDEX") {
			if !strings.Contains(upper, "INDEX CONCURRENTLY") {
				t.Errorf("%s creates an index without CONCURRENTLY", base)
			}
		}
		if strings.Contains(upper, "DROP INDEX") && !strings.Contains(upper, "DROP INDEX CONCURRENTLY") {
			t.Errorf("%s drops an index without CONCURRENTLY", base)
		}
	}
	if found != 72 {
		t.Fatalf("checked %d workflow v2 index migration directions, want 72", found)
	}
}

func TestWorkflowConcernHeadMigrationIsFKFreeAndExact(t *testing.T) {
	body := readWorkflowV2File(t, filepath.Join(realMigrationsDir(t), "496_issue_workflow_concern_head.up.sql"))
	plain := stripSQLLineComments(body)
	forbidden := regexp.MustCompile(`(?i)\b(REFERENCES|FOREIGN\s+KEY|CASCADE|PRIMARY\s+KEY|CREATE\s+(UNIQUE\s+)?INDEX)\b`)
	if match := forbidden.FindString(plain); match != "" {
		t.Fatalf("concern head migration must remain FK/index free; found %q", match)
	}
	for _, concern := range []string{"outcome", "scope", "authority", "execution", "review", "release", "attention", "evidence", "budget"} {
		if !strings.Contains(body, "'"+concern+"'") {
			t.Errorf("concern head CHECK omits %s", concern)
		}
	}
	for _, check := range []string{"revision > 0", "source_record_revision > 0", "state <> ''", "state = btrim(state)"} {
		if !strings.Contains(body, check) {
			t.Errorf("concern head migration omits CHECK %q", check)
		}
	}
	for _, typedState := range []string{
		"concern_type = 'outcome' AND state IN ('open', 'satisfied', 'cancelled', 'superseded')",
		"concern_type = 'execution' AND state IN ('received', 'queued', 'working'",
		"concern_type = 'release' AND state IN ('not_required', 'pending', 'built'",
		"concern_type = 'evidence' AND state IN ('unknown', 'current', 'stale', 'contradicted')",
	} {
		if !strings.Contains(body, typedState) {
			t.Errorf("concern head migration omits typed state constraint %q", typedState)
		}
	}
}

func TestWorkflowDecisionSuspendedStateMigrationIsExplicit(t *testing.T) {
	body := readWorkflowV2File(t, filepath.Join(realMigrationsDir(t), "499_issue_workflow_decision_suspended_state.up.sql"))
	for _, required := range []string{"suspended_execution_state TEXT NOT NULL", "'working'", "'correction'", "'blocked_external'", "'cancelled'"} {
		if !strings.Contains(body, required) {
			t.Errorf("decision suspended-state migration omits %q", required)
		}
	}
}

func TestWorkflowReviewCorrectionMetadataMigrationIsExplicit(t *testing.T) {
	body := readWorkflowV2File(t, filepath.Join(realMigrationsDir(t), "500_issue_workflow_review_correction_metadata.up.sql"))
	for _, required := range []string{"correction_count INTEGER NOT NULL", "max_corrections INTEGER NOT NULL", "correction_count <= max_corrections"} {
		if !strings.Contains(body, required) {
			t.Errorf("review correction migration omits %q", required)
		}
	}
}

func TestWorkflowReleaseAttemptMigrationsAreTypedAndRollbackSafe(t *testing.T) {
	up := readWorkflowV2File(t, filepath.Join(realMigrationsDir(t), "513_issue_workflow_release_attempt_metadata.up.sql"))
	for _, required := range []string{
		"attempt_contract_version SMALLINT", "issue_workflow_release_attempt_contract_complete",
		"expected_target_release_id UUID", "expected_target_generation BIGINT",
		"release_kind TEXT", "recovery_of_release_id UUID", "head_observed_at TIMESTAMPTZ",
		"target_generation > expected_target_generation", "release_kind = 'recovery'",
		"workflow release metadata cannot be backfilled from exact authoritative scope and target head",
	} {
		if !strings.Contains(up, required) {
			t.Errorf("release-attempt migration omits %q", required)
		}
	}
	down := readWorkflowV2File(t, filepath.Join(realMigrationsDir(t), "513_issue_workflow_release_attempt_metadata.down.sql"))
	if !strings.Contains(down, "refusing destructive rollback") || !strings.Contains(down, "head_observed_at IS NOT NULL") {
		t.Error("release-attempt down migration must fail closed on populated durable metadata")
	}

	incident := readWorkflowV2File(t, filepath.Join(realMigrationsDir(t), "514_issue_workflow_incident_release_binding.up.sql"))
	for _, required := range []string{"release_revision BIGINT", "candidate_revision BIGINT", "target_generation BIGINT", "detection_evidence_id UUID", "compatibility_state", "issue_workflow_incident_exact_binding_coherent"} {
		if !strings.Contains(incident, required) {
			t.Errorf("incident binding migration omits %q", required)
		}
	}

	index := strings.TrimSpace(stripSQLLineComments(readWorkflowV2File(t, filepath.Join(realMigrationsDir(t), "515_issue_workflow_active_incident_release_idx.up.sql"))))
	if strings.Count(index, ";") != 1 || !strings.Contains(index, "INDEX CONCURRENTLY") || !strings.Contains(index, "target_generation") {
		t.Errorf("active release incident index is not a one-statement concurrent exact-binding index: %s", index)
	}
}

func TestWorkflowActionReconciliationDueIndexCoversSelector(t *testing.T) {
	index := readWorkflowV2File(t, filepath.Join(realMigrationsDir(t), "507_issue_workflow_action_reconciliation_due_idx.up.sql"))
	query := readWorkflowV2File(t, filepath.Join(realMigrationsDir(t), "..", "pkg", "db", "queries", "workflow_action.sql"))
	for _, required := range []string{"INDEX CONCURRENTLY", "action_type = 'review_correction'", "'reconciling'", "not_before", "lease_expires_at"} {
		if !strings.Contains(index, required) {
			t.Errorf("reconciliation due index omits %q", required)
		}
	}
	for _, required := range []string{"action_type = 'review_correction'", "state IN ('admitted', 'retry_wait', 'reconciling')", "state = 'leased'"} {
		if !strings.Contains(query, required) {
			t.Errorf("due selector omits indexed predicate %q", required)
		}
	}
}

func TestWorkflowV2QueriesCoverAuthoritativeRecordsAndCAS(t *testing.T) {
	path := filepath.Join(realMigrationsDir(t), "..", "pkg", "db", "queries", "workflow_v2.sql")
	body := readWorkflowV2File(t, path)
	actionBody := readWorkflowV2File(t, filepath.Join(realMigrationsDir(t), "..", "pkg", "db", "queries", "workflow_action.sql"))

	for _, query := range []string{
		"LockIssueWorkflowV2Admission",
		"InitializeIssueWorkflowConcernHead",
		"ListIssueWorkflowConcernHeads",
		"GetIssueWorkflowConcernHead",
		"AdvanceIssueWorkflowConcernHeadCAS",
		"CreateIssueWorkflowOutcome",
		"CreateIssueWorkflowScope",
		"GetLatestIssueWorkflowAuthority",
		"UpdateIssueWorkflowExecutionCAS",
		"CreateIssueWorkflowDecisionResponse",
		"ListIssueWorkflowReviewsForCandidate",
		"GetIssueWorkflowReleaseByTargetGeneration",
		"CreateIssueWorkflowReleaseAttempt",
		"CreateIssueWorkflowReleaseIncident",
		"LockIssueWorkflowTarget",
		"LockIssueWorkflowTargetHead",
		"ListIssueWorkflowEvidence",
		"CreateIssueWorkflowCommand",
		"CreateIssueWorkflowAttention",
		"AdvanceIssueWorkflowTargetHeadCAS",
		"UpsertIssueCurrentPicture",
		"ListIssueWorkflowProjectionInputs",
		"DeleteIssueWorkflowV2RecordsByIssue",
		"DeleteIssueWorkflowV2RecordsByWorkspace",
		"DeleteSquadWorkflowTemplateScoped",
	} {
		if !strings.Contains(body, "-- name: "+query+" ") {
			t.Errorf("workflow_v2.sql is missing query %s", query)
		}
	}
	for _, query := range []string{"ListDueIssueWorkflowActions", "ClaimIssueWorkflowActionExact", "DeferIssueWorkflowActionExact", "BeginIssueWorkflowActionAttempt", "RenewIssueWorkflowActionLease", "RecordIssueWorkflowActionObservationFailure", "SettleIssueWorkflowAction", "DeadLetterIssueWorkflowActionExact", "CreateWorkflowCorrectionTask", "BindWorkflowActionTaskExact"} {
		if !strings.Contains(actionBody, "-- name: "+query+" ") {
			t.Errorf("workflow_action.sql is missing query %s", query)
		}
	}
	if !strings.Contains(body, "@suspended_execution_state") {
		t.Error("decision persistence must retain the exact suspended execution state")
	}

	for _, table := range []string{
		"issue_workflow_outcome",
		"issue_workflow_scope",
		"issue_workflow_authority",
		"issue_workflow_execution",
		"issue_workflow_decision",
		"issue_workflow_candidate",
		"issue_workflow_review",
		"issue_workflow_release",
		"issue_workflow_evidence",
		"issue_workflow_action_receipt",
		"issue_workflow_attention",
		"issue_workflow_target_head",
		"issue_current_picture",
	} {
		if !strings.Contains(body, table) {
			t.Errorf("workflow_v2.sql does not address %s", table)
		}
	}

	for name, fragment := range map[string]string{
		"execution terminal vocabulary": "state NOT IN ('completed', 'failed', 'cancelled', 'superseded')",
		"decision state vocabulary":     "state = 'unanswered' OR resolution_state = 'resuming'",
		"lower projection refusal":      "EXCLUDED.revision > issue_current_picture.revision",
		"idempotent replay":             "WHERE issue_workflow_command.request_hash = EXCLUDED.request_hash",
	} {
		if !strings.Contains(body, fragment) {
			t.Errorf("workflow_v2.sql is missing %s contract", name)
		}
	}
	for name, fragment := range map[string]string{
		"expired lease reclaim":       "state = 'leased' AND lease_expires_at <= @now",
		"lease count":                 "lease_count = lease_count + 1",
		"effect attempt count":        "effect_attempt_count = effect_attempt_count + 1",
		"future claim lease":          "@lease_expires_at > CAST(@now AS timestamptz)",
		"monotonic lease renewal":     "@lease_expires_at > lease_expires_at",
		"exact task effect identity":  "WHERE id = @id AND workflow_action_id = @workflow_action_id",
		"bound task context equality": "task.context = @context",
	} {
		if !strings.Contains(actionBody, fragment) {
			t.Errorf("workflow_action.sql is missing %s contract", name)
		}
	}

	issueDelete := readWorkflowV2File(t, filepath.Join(realMigrationsDir(t), "..", "pkg", "db", "queries", "issue.sql"))
	workspaceDelete := readWorkflowV2File(t, filepath.Join(realMigrationsDir(t), "..", "pkg", "db", "queries", "workspace_delete.sql"))
	squadQueries := readWorkflowV2File(t, filepath.Join(realMigrationsDir(t), "..", "pkg", "db", "queries", "squad.sql"))
	agentQueries := readWorkflowV2File(t, filepath.Join(realMigrationsDir(t), "..", "pkg", "db", "queries", "agent.sql"))
	runtimeQueries := readWorkflowV2File(t, filepath.Join(realMigrationsDir(t), "..", "pkg", "db", "queries", "runtime.sql"))
	workflowHandler := readWorkflowV2File(t, filepath.Join(realMigrationsDir(t), "..", "internal", "handler", "workflow_control.go"))
	squadHandler := readWorkflowV2File(t, filepath.Join(realMigrationsDir(t), "..", "internal", "handler", "squad.go"))
	runtimeTeardown := readWorkflowV2File(t, filepath.Join(realMigrationsDir(t), "..", "internal", "service", "runtime_teardown.go"))
	for _, table := range []string{"issue_workflow_outcome", "issue_workflow_attention", "issue_workflow_target_head", "issue_workflow_action_receipt", "issue_workflow_concern_head", "issue_current_picture"} {
		if !strings.Contains(issueDelete, "DELETE FROM "+table) {
			t.Errorf("DeleteIssue cleanup is missing %s", table)
		}
		if !strings.Contains(workspaceDelete, "DELETE FROM "+table) {
			t.Errorf("workspace cleanup is missing %s", table)
		}
	}
	if !strings.Contains(workspaceDelete, "DELETE FROM squad_workflow_template") {
		t.Error("workspace squad cleanup must explicitly remove FK-free workflow templates")
	}
	if !strings.Contains(issueDelete, "FOR UPDATE") || !strings.Contains(body, "FOR KEY SHARE OF workspace, issue") {
		t.Error("workflow admission and issue deletion must use conflicting owner-row locks")
	}
	if !strings.Contains(squadQueries, "DeleteSquadWorkflowTemplateForArchive") || !strings.Contains(squadHandler, "LockWorkflowOwnerWorkspace") || !strings.Contains(squadHandler, "LockSquadForUpdate") {
		t.Error("squad archive must lock workspace and squad before removing its workflow template")
	}
	if !strings.Contains(agentQueries, "DeleteWorkflowTemplatesByAgent") {
		t.Error("chat system-agent hard delete must remove role-bound workflow templates")
	}
	if !strings.Contains(runtimeQueries, "DeleteWorkflowTemplatesBySystemRuntimeAgents") || !strings.Contains(runtimeTeardown, "LockSystemAgentsByRuntimeForDelete") {
		t.Error("runtime system-agent teardown must lock agents then remove role-bound workflow templates")
	}
	for _, required := range []string{"LockWorkflowOwnerWorkspace", "LockSquadWorkflowTemplateSquad", "LockSquadWorkflowTemplateAgents", "UpsertSquadWorkflowTemplate"} {
		if !strings.Contains(workflowHandler, required) {
			t.Errorf("template writer transaction is missing %s", required)
		}
	}
}

func readWorkflowV2File(t *testing.T, path string) string {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

func stripSQLLineComments(body string) string {
	lines := strings.Split(body, "\n")
	for i, line := range lines {
		if at := strings.Index(line, "--"); at >= 0 {
			lines[i] = line[:at]
		}
	}
	return strings.Join(lines, "\n")
}
