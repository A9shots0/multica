package workflowstore

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// This guard makes the admission API architectural rather than optional:
// application code cannot call a generated v2 mutator directly. Migration DB
// tests are intentionally exempt because they verify the SQL contract itself.
func TestApplicationV2WritesUseAdmissionTransaction(t *testing.T) {
	mutators := make([]string, 0)
	for _, queryFile := range []string{"workflow_v2.sql", "workflow_action.sql", "owner_notification.sql", "external_work_receipt.sql"} {
		body, err := os.ReadFile(filepath.Join("..", "..", "pkg", "db", "queries", queryFile))
		if err != nil {
			t.Fatal(err)
		}
		for _, block := range strings.Split(string(body), "\n-- name: ")[1:] {
			header, sql, found := strings.Cut(block, "\n")
			if !found {
				continue
			}
			name, _, found := strings.Cut(header, " ")
			if !found {
				continue
			}
			sql = strings.TrimSpace(sql)
			withMutation := strings.HasPrefix(sql, "WITH ") &&
				(strings.Contains(sql, "\nINSERT ") || strings.Contains(sql, "\nUPDATE ") || strings.Contains(sql, "\nDELETE "))
			if strings.HasPrefix(sql, "INSERT ") || strings.HasPrefix(sql, "UPDATE ") ||
				strings.HasPrefix(sql, "DELETE ") || withMutation {
				mutators = append(mutators, name)
			}
		}
	}

	for _, root := range []string{filepath.Join("..", "..", "internal"), filepath.Join("..", "..", "cmd")} {
		err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() {
				normalized := filepath.ToSlash(path)
				if strings.Contains(normalized, "/internal/migrations") {
					return filepath.SkipDir
				}
				return nil
			}
			if filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			source, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			for _, name := range mutators {
				if strings.Contains(string(source), "."+name+"(") {
					// Delivery-state mutations are intentionally owned by the
					// restart-safe notification worker, not by workflow truth. Only
					// intent admission belongs inside IssueWrite.
					if strings.Contains(filepath.ToSlash(path), "/internal/ownernotification/") && name != "AdmitOwnerNotification" {
						continue
					}
					// The local correction effect owns a separate transaction with
					// the established task lock order (workspace, agent, issue,
					// runtime), so these two mutations cannot route through the
					// issue-first capability without inverting locks.
					if (filepath.Base(path) == "local_task_effect.go" || filepath.Base(path) == "review_task_effect.go" || filepath.Base(path) == "release_verification_task_effect.go") && (name == "CreateWorkflowCorrectionTask" || name == "BindWorkflowActionTaskExact") {
						continue
					}
					// Workflow task completion has its own archive-safe
					// workspace -> agent -> issue -> runtime admission in TaskService;
					// routing it through the issue-first capability would invert locks.
					if filepath.Base(path) == "task.go" && name == "CompleteWorkflowTaskExact" {
						continue
					}
					if filepath.Base(path) == "writer.go" {
						scoped := map[string]bool{
							"UpsertIssueCurrentPicture":          true,
							"InitializeIssueWorkflowConcernHead": true,
							"AdvanceIssueWorkflowConcernHeadCAS": true,
							"CreateIssueWorkflowCommand":         true, "SetIssueWorkflowCommandResult": true,
							"CreateIssueWorkflowActionReceipt": true, "UpdateIssueWorkflowExecutionCAS": true,
							"CreateIssueWorkflowExecution": true,
							"CreateIssueWorkflowScope":     true,
							"CreateIssueWorkflowReview":    true, "CreateIssueWorkflowDecision": true,
							"UpdateIssueWorkflowDecisionStateCAS": true, "CreateIssueWorkflowDecisionResponse": true,
							"CreateIssueWorkflowEvidence": true, "UpdateIssueWorkflowAttentionCAS": true,
							"CreateIssueWorkflowAttention":                       true,
							"CreateIssueWorkflowCandidate":                       true,
							"SupersedeIssueWorkflowCandidateCAS":                 true,
							"CreateIssueWorkflowRelease":                         true,
							"CreateIssueWorkflowReleaseAttempt":                  true,
							"CreateIssueWorkflowRecoveryReleaseAttempt":          true,
							"AdmitPendingIssueWorkflowReleaseAttemptCAS":         true,
							"LockWorkflowReleaseBuildAuthority":                  true,
							"CreateIssueWorkflowReleaseIncident":                 true,
							"UpdateIssueWorkflowIncidentCAS":                     true,
							"LockIssueWorkflowTarget":                            true,
							"UpdateIssueWorkflowReleaseCAS":                      true,
							"RebindPendingIssueWorkflowReleaseCandidateCAS":      true,
							"MarkWorkflowTaskResultReconciliationFailed":         true,
							"UpdateIssueWorkflowExecutionCorrectionCandidateCAS": true,
							"LockWorkflowReviewerCommandAuthority":               true,
							"LockWorkflowReviewResultAuthority":                  true,
							"LockWorkflowReleaseVerificationResultAuthority":     true,
							"LockWorkflowFinalizerCommandAuthority":              true,
							"UpdateIssueWorkflowOutcomeCAS":                      true,
							"CreateIssueWorkflowOutcome":                         true,
							"ClaimIssueWorkflowActionExact":                      true,
							"AdvanceIssueWorkflowTargetHeadCAS":                  true,
							"UpdateIssueWorkflowTargetHeadStateCAS":              true,
							"DeferIssueWorkflowActionExact":                      true,
							"BeginIssueWorkflowActionAttempt":                    true,
							"RenewIssueWorkflowActionLease":                      true,
							"RecordIssueWorkflowActionObservationFailure":        true,
							"SettleIssueWorkflowAction":                          true,
							"DeadLetterIssueWorkflowActionExact":                 true,
							"AdmitOwnerNotification":                             true,
							"CreateExternalWorkReceipt":                          true,
							"SetExternalWorkReceiptAdmissionResult":              true,
							"PutIssueWorkflowExecutionCheckpointSnapshot":        true,
						}
						if scoped[name] {
							continue
						}
					}
					t.Errorf("%s calls generated v2 mutator %s directly; use InIssueWriteTransaction", path, name)
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	writerSource, err := os.ReadFile("writer.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{
		"type IssueWrite struct {", "state *issueWriteState", "type issueWriteState struct {", "queries       *db.Queries", "workspaceID   pgtype.UUID", "issueID       pgtype.UUID", "mu            sync.RWMutex", "active        bool", "releaseActors map[pgtype.UUID]struct{}",
		"capability := &IssueWrite{state: &issueWriteState{queries: qtx, workspaceID: workspaceID, issueID: issueID, active: true}}", "capability.state.mu.Lock()", "capability.state.active = false",
		"func (write *IssueWrite) MatchesIssue",
		"func (write *IssueWrite) WithIssueOperation",
		"func (write *IssueWrite) UpsertCurrentPictureCAS",
		"func (write *IssueWrite) InitializeConcernHead",
		"func (write *IssueWrite) AdvanceConcernHeadCAS",
	} {
		if !strings.Contains(string(writerSource), required) {
			t.Errorf("admission capability is missing %q", required)
		}
	}
	if strings.Contains(string(writerSource), "func (write *IssueWrite) Queries()") {
		t.Error("IssueWrite exposes unrestricted raw generated queries")
	}
	if strings.Count(string(writerSource), ".UpsertIssueCurrentPicture(") != 1 {
		t.Errorf("writer.go generated Current Picture mutator calls = %d, want exactly 1 scoped call", strings.Count(string(writerSource), ".UpsertIssueCurrentPicture("))
	}
}
