package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/multica-ai/multica/server/internal/events"
	"github.com/multica-ai/multica/server/internal/util"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
	"github.com/multica-ai/multica/server/pkg/dbid"
	"github.com/multica-ai/multica/server/pkg/protocol"
	"github.com/multica-ai/multica/server/pkg/taskfailure"
)

// Provider authentication/model access is an operational recovery condition. Its terminal
// failure remains recorded and projected to Chat, but does not establish an AI
// reasoning/design failure requiring six-domain research before work resumes.
// Reclassify the raw error as well for older daemons that report unknown.
func shouldInvestigateTerminalAIFailure(reason, rawError string) bool {
	for _, candidate := range []taskfailure.Reason{taskfailure.Reason(reason), taskfailure.Classify(rawError)} {
		if candidate == taskfailure.ReasonAgentProviderAuthOrAccess || candidate == taskfailure.ReasonAgentModelNotFoundOrUnavailable {
			return false
		}
	}
	return true
}

var failureInvestigationScope = []string{
	"company_and_product_context",
	"model_behavior_and_capability_limits",
	"prompt_instructions_memory_and_context",
	"tools_runtime_data_and_evidence",
	"human_factors_psychology_and_decision_design",
	"workflow_review_qa_and_prevention_systems",
}

var validFailureInvestigationTriggers = map[string]bool{
	"task_failed":     true,
	"owner_rejection": true,
	"qa_rejection":    true,
}

const maxFailureInvestigationSummaryRunes = 4000

const (
	failureInvestigationOpenTag  = "<ai-failure-investigation>"
	failureInvestigationCloseTag = "</ai-failure-investigation>"
)

const failureInvestigationBrief = `Internal AI failure investigation.

Do not jump to a replacement solution. First reconstruct what happened from the issue history, task evidence, attachments, owner feedback, and prior related work. Investigate all of these domains even when the first cause looks obvious:
1. company and product context;
2. model behavior, capability limits, and known failure modes;
3. prompts, instructions, memory, retrieved context, and examples;
4. tools, runtime, data, evidence, and handoffs;
5. human factors, psychology, attention, comprehension, and decision design;
6. workflow, reviewer, QA, incentives, and prevention systems.

Use current authoritative external research when a material claim about AI behavior or human psychology cannot be established from local evidence, and cite it. Separate observed facts, hypotheses, and unknowns. Compare this failure with prior similar failures and with successful work. Produce: a causal timeline; a root-cause tree; contributing conditions; why existing gates missed it; prevention changes by system layer; a verification plan that could falsify the proposed fix; and concise reusable learning candidates. Do not silently accept those candidates into durable memory. Do not resume or propose replacement implementation until the investigation is complete. Ask the owner only when missing input would materially change the conclusion.

Your final answer must include exactly one <ai-failure-investigation> JSON block. Prose cannot substitute for this evidence contract. Use this shape:
{"schema_version":1,"investigation_id":"the supplied investigation UUID","summary":"plain-English conclusion","causal_timeline":["ordered event"],"root_causes":[{"cause":"...","confidence":"confirmed|probable|possible","evidence_refs":["durable local evidence"]}],"contributing_conditions":["..."],"gate_misses":["..."],"domains":[{"id":"one required research domain","findings":["..."],"evidence_refs":["..."],"external_sources":[{"title":"authoritative source","url":"https://..."}]}],"facts":["..."],"hypotheses":[],"unknowns":[],"prevention_changes":[{"layer":"prompt|memory|tool|runtime|workflow|review|product","change":"...","verification":"..."}],"verification_plan":[{"test":"...","falsifies_if":"..."}],"learning_candidates":["..."],"replacement_work_status":"blocked|ready_for_review","next_action":"...","owner_input":{"required":false,"question":"","options":[]}}. Include every one of the six supplied research-domain IDs exactly once. The model-behavior and human-factors domains each require at least one authoritative HTTPS source.`

type failureInvestigationReport struct {
	SchemaVersion   int      `json:"schema_version"`
	InvestigationID string   `json:"investigation_id"`
	Summary         string   `json:"summary"`
	CausalTimeline  []string `json:"causal_timeline"`
	RootCauses      []struct {
		Cause        string   `json:"cause"`
		Confidence   string   `json:"confidence"`
		EvidenceRefs []string `json:"evidence_refs"`
	} `json:"root_causes"`
	ContributingConditions []string `json:"contributing_conditions"`
	GateMisses             []string `json:"gate_misses"`
	Domains                []struct {
		ID              string   `json:"id"`
		Findings        []string `json:"findings"`
		EvidenceRefs    []string `json:"evidence_refs"`
		ExternalSources []struct {
			Title string `json:"title"`
			URL   string `json:"url"`
		} `json:"external_sources"`
	} `json:"domains"`
	Facts             []string `json:"facts"`
	Hypotheses        []string `json:"hypotheses"`
	Unknowns          []string `json:"unknowns"`
	PreventionChanges []struct {
		Layer        string `json:"layer"`
		Change       string `json:"change"`
		Verification string `json:"verification"`
	} `json:"prevention_changes"`
	VerificationPlan []struct {
		Test        string `json:"test"`
		FalsifiesIf string `json:"falsifies_if"`
	} `json:"verification_plan"`
	LearningCandidates    []string `json:"learning_candidates"`
	ReplacementWorkStatus string   `json:"replacement_work_status"`
	NextAction            string   `json:"next_action"`
	OwnerInput            struct {
		Required bool     `json:"required"`
		Question string   `json:"question"`
		Options  []string `json:"options"`
	} `json:"owner_input"`
}

// LaunchAIFailureInvestigation creates one idempotent investigation for a
// concrete failure signal and dispatches Mika as the coordinator when that
// workspace has one. It never creates an Inbox chore: the investigation only
// asks the owner later if its evidence proves a real decision is required.
func (s *TaskService) LaunchAIFailureInvestigation(
	ctx context.Context,
	issue db.Issue,
	triggerKind string,
	triggerID pgtype.UUID,
	sourceTaskID pgtype.UUID,
	sourceAgentID pgtype.UUID,
	accountableUserID pgtype.UUID,
	summary string,
) (db.AiFailureInvestigation, error) {
	if !validFailureInvestigationTriggers[triggerKind] || !triggerID.Valid {
		return db.AiFailureInvestigation{}, errors.New("invalid AI failure investigation trigger")
	}
	cleanSummary := normalizeFailureInvestigationSummary(summary)
	scope, _ := json.Marshal(failureInvestigationScope)
	row, err := s.Queries.CreateAIFailureInvestigation(ctx, db.CreateAIFailureInvestigationParams{
		ID: dbid.NewV7(), WorkspaceID: issue.WorkspaceID, IssueID: issue.ID,
		TriggerKind: triggerKind, TriggerID: triggerID,
		SourceTaskID: sourceTaskID, SourceAgentID: sourceAgentID,
		AccountableUserID: accountableUserID,
		Summary:           cleanSummary, ResearchScope: scope,
	})
	if err != nil {
		return db.AiFailureInvestigation{}, fmt.Errorf("record AI failure investigation: %w", err)
	}
	if row.Status == "completed" || row.Status == "running" || row.Status == "investigator_failed" {
		return row, nil
	}
	return s.dispatchAIFailureInvestigation(ctx, issue, row)
}

func normalizeFailureInvestigationSummary(summary string) string {
	cleanSummary := strings.TrimSpace(util.SanitizeTextForPostgres(summary))
	if cleanSummary == "" {
		cleanSummary = "The failure signal did not include a plain-language summary. Reconstruct it from durable task and issue evidence."
	}
	if runes := []rune(cleanSummary); len(runes) > maxFailureInvestigationSummaryRunes {
		cleanSummary = string(runes[:maxFailureInvestigationSummaryRunes]) + "…"
	}
	return cleanSummary
}

func (s *TaskService) dispatchAIFailureInvestigation(ctx context.Context, issue db.Issue, row db.AiFailureInvestigation) (db.AiFailureInvestigation, error) {
	commentID := row.TriggerCommentID
	if !commentID.Valid {
		content := failureInvestigationBrief + "\n\nInvestigation ID: " + util.UUIDToString(row.ID) +
			"\nFailure signal: " + row.TriggerKind +
			"\nObserved summary: " + row.Summary
		created, createErr := s.Queries.CreateComment(ctx, db.CreateCommentParams{
			ID: dbid.NewV7(), IssueID: issue.ID, WorkspaceID: issue.WorkspaceID,
			AuthorType: "system", AuthorID: pgtype.UUID{Valid: true}, Content: content, Type: "system",
			SourceTaskID: row.SourceTaskID,
		})
		if createErr != nil {
			return row, fmt.Errorf("create AI failure investigation brief: %w", createErr)
		}
		commentID = created.ID
		updated, updateErr := s.Queries.SetAIFailureInvestigationComment(ctx, db.SetAIFailureInvestigationCommentParams{
			ID: row.ID, TriggerCommentID: commentID,
		})
		if updateErr != nil && !errors.Is(updateErr, pgx.ErrNoRows) {
			return row, fmt.Errorf("bind AI failure investigation brief: %w", updateErr)
		}
		if updateErr == nil {
			row = updated
		}
	}

	mika, err := s.Queries.GetAgentBySystemKey(ctx, db.GetAgentBySystemKeyParams{
		WorkspaceID: issue.WorkspaceID,
		SystemKey:   pgtype.Text{String: MikaSystemKey, Valid: true},
	})
	if err != nil || !mika.RuntimeID.Valid {
		if waitingErr := s.Queries.SetAIFailureInvestigationAwaiting(ctx, row.ID); waitingErr != nil {
			return row, fmt.Errorf("record AI failure investigation awaiting state: %w", waitingErr)
		}
		row.Status = "awaiting_investigator"
		return row, nil
	}

	task, err := s.EnqueueTaskForMention(ctx, issue, mika.ID, commentID)
	if err != nil {
		if errors.Is(err, ErrDuplicatePendingTask) {
			return row, nil
		}
		return row, fmt.Errorf("dispatch AI failure investigation: %w", err)
	}
	row, err = s.Queries.SetAIFailureInvestigationDispatch(ctx, db.SetAIFailureInvestigationDispatchParams{
		ID: row.ID, InvestigatorAgentID: mika.ID, InvestigatorTaskID: task.ID,
	})
	if err != nil {
		return row, fmt.Errorf("record AI failure investigation dispatch: %w", err)
	}
	return row, nil
}

func (s *TaskService) completeAIFailureInvestigation(ctx context.Context, task db.AgentTaskQueue, result []byte) {
	investigation, err := s.Queries.GetAIFailureInvestigationByTask(ctx, task.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		return
	}
	if err != nil {
		slog.Warn("load AI failure investigation completion failed", "task_id", util.UUIDToString(task.ID), "error", err)
		return
	}
	var payload protocol.TaskCompletedPayload
	if err := json.Unmarshal(result, &payload); err != nil {
		s.failInvalidAIFailureInvestigation(ctx, task, fmt.Errorf("decode task completion: %w", err))
		return
	}
	findings, err := validatedFailureInvestigationFindings(payload.Output, util.UUIDToString(investigation.ID))
	if err != nil {
		s.failInvalidAIFailureInvestigation(ctx, task, err)
		return
	}
	var report failureInvestigationReport
	if err := json.Unmarshal(findings, &report); err != nil {
		s.failInvalidAIFailureInvestigation(ctx, task, fmt.Errorf("decode validated investigation: %w", err))
		return
	}
	var ownerItem *db.InboxItem
	var changed int64
	err = s.runInTx(ctx, func(qtx *db.Queries) error {
		completed, completeErr := qtx.CompleteAIFailureInvestigationByTask(ctx, db.CompleteAIFailureInvestigationByTaskParams{
			InvestigatorTaskID: task.ID, Findings: findings,
		})
		if completeErr != nil {
			return completeErr
		}
		changed = completed
		if completed == 0 || !report.OwnerInput.Required {
			return nil
		}
		item, itemErr := createFailureInvestigationOwnerQuestion(ctx, qtx, investigation, report)
		if itemErr != nil {
			return itemErr
		}
		ownerItem = &item
		return nil
	})
	if err != nil {
		slog.Warn("complete AI failure investigation failed", "task_id", util.UUIDToString(task.ID), "error", err)
		return
	}
	if changed > 0 {
		slog.Info("AI failure investigation completed", "task_id", util.UUIDToString(task.ID), "issue_id", util.UUIDToString(task.IssueID))
	}
	if ownerItem != nil && s.Bus != nil {
		s.Bus.Publish(events.Event{
			Type:        protocol.EventInboxNew,
			WorkspaceID: util.UUIDToString(investigation.WorkspaceID),
			Payload:     map[string]any{"item": *ownerItem},
		})
	}
}

func createFailureInvestigationOwnerQuestion(
	ctx context.Context,
	q *db.Queries,
	investigation db.AiFailureInvestigation,
	report failureInvestigationReport,
) (db.InboxItem, error) {
	recipient := investigation.AccountableUserID
	if !recipient.Valid {
		managers, err := q.ListWorkspaceManagerUserIDs(ctx, investigation.WorkspaceID)
		if err != nil {
			return db.InboxItem{}, fmt.Errorf("find investigation owner: %w", err)
		}
		if len(managers) == 0 {
			return db.InboxItem{}, errors.New("failure investigation needs owner input but the workspace has no manager")
		}
		recipient = managers[0]
	}
	issue, err := q.GetIssue(ctx, investigation.IssueID)
	if err != nil {
		return db.InboxItem{}, fmt.Errorf("load investigated issue: %w", err)
	}
	details, err := json.Marshal(map[string]string{
		"owner_relevant":   "true",
		"owner_category":   "needs_you",
		"required_action":  "answer_failure_investigation",
		"question":         report.OwnerInput.Question,
		"options":          strings.Join(report.OwnerInput.Options, "\n"),
		"consequence":      "Replacement work remains paused until you decide.",
		"investigation_id": util.UUIDToString(investigation.ID),
	})
	if err != nil {
		return db.InboxItem{}, fmt.Errorf("encode investigation owner question: %w", err)
	}
	item, err := q.CreateInboxItem(ctx, db.CreateInboxItemParams{
		ID: dbid.NewV7(), WorkspaceID: investigation.WorkspaceID,
		RecipientType: "member", RecipientID: recipient,
		Type: "workflow_owner_decision", Severity: "action_required", IssueID: investigation.IssueID,
		Title:     issue.Title,
		Body:      pgtype.Text{String: report.Summary + "\n\n" + report.OwnerInput.Question, Valid: true},
		ActorType: pgtype.Text{String: "system", Valid: true}, ActorID: pgtype.UUID{}, Details: details,
	})
	if err != nil {
		return db.InboxItem{}, fmt.Errorf("create investigation owner question: %w", err)
	}
	return item, nil
}

func (s *TaskService) failInvalidAIFailureInvestigation(ctx context.Context, task db.AgentTaskQueue, validationErr error) {
	changed, err := s.Queries.FailAIFailureInvestigationByTask(ctx, task.ID)
	if err != nil {
		slog.Warn("record invalid AI failure investigation failed", "task_id", util.UUIDToString(task.ID), "validation_error", validationErr, "error", err)
		return
	}
	if changed > 0 {
		slog.Warn("AI failure investigation rejected by evidence contract", "task_id", util.UUIDToString(task.ID), "error", validationErr)
	}
}

func validatedFailureInvestigationFindings(output, expectedID string) ([]byte, error) {
	raw, err := extractFailureInvestigationBlock(output)
	if err != nil {
		return nil, err
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var report failureInvestigationReport
	if err := decoder.Decode(&report); err != nil {
		return nil, fmt.Errorf("investigation JSON does not match the required contract: %w", err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return nil, errors.New("investigation JSON contains trailing content")
	}
	if report.SchemaVersion != 1 || report.InvestigationID != expectedID || strings.TrimSpace(report.Summary) == "" {
		return nil, errors.New("investigation identity, schema, or summary is invalid")
	}
	if !nonEmptyInvestigationStrings(report.CausalTimeline, 2) || !nonEmptyInvestigationStrings(report.ContributingConditions, 1) ||
		!nonEmptyInvestigationStrings(report.GateMisses, 1) || !nonEmptyInvestigationStrings(report.Facts, 1) ||
		report.Hypotheses == nil || report.Unknowns == nil || !nonEmptyInvestigationStrings(report.LearningCandidates, 1) {
		return nil, errors.New("investigation is missing required causal, evidence, or learning sections")
	}
	if len(report.RootCauses) == 0 {
		return nil, errors.New("investigation requires at least one evidence-backed root cause")
	}
	for _, cause := range report.RootCauses {
		if strings.TrimSpace(cause.Cause) == "" || !map[string]bool{"confirmed": true, "probable": true, "possible": true}[cause.Confidence] ||
			!nonEmptyInvestigationStrings(cause.EvidenceRefs, 1) {
			return nil, errors.New("investigation root cause lacks confidence or evidence")
		}
	}

	requiredDomains := make(map[string]bool, len(failureInvestigationScope))
	for _, domain := range failureInvestigationScope {
		requiredDomains[domain] = true
	}
	seenDomains := make(map[string]bool, len(report.Domains))
	for _, domain := range report.Domains {
		if !requiredDomains[domain.ID] || seenDomains[domain.ID] || !nonEmptyInvestigationStrings(domain.Findings, 1) || !nonEmptyInvestigationStrings(domain.EvidenceRefs, 1) {
			return nil, fmt.Errorf("investigation domain %q is missing, duplicated, or lacks evidence", domain.ID)
		}
		seenDomains[domain.ID] = true
		if domain.ID == "model_behavior_and_capability_limits" || domain.ID == "human_factors_psychology_and_decision_design" {
			if len(domain.ExternalSources) == 0 {
				return nil, fmt.Errorf("investigation domain %q requires authoritative external research", domain.ID)
			}
			for _, source := range domain.ExternalSources {
				if strings.TrimSpace(source.Title) == "" || !strings.HasPrefix(strings.ToLower(strings.TrimSpace(source.URL)), "https://") {
					return nil, fmt.Errorf("investigation domain %q contains an invalid external source", domain.ID)
				}
			}
		}
	}
	if len(seenDomains) != len(requiredDomains) {
		return nil, errors.New("investigation does not cover all required research domains")
	}
	if len(report.PreventionChanges) == 0 || len(report.VerificationPlan) == 0 {
		return nil, errors.New("investigation requires prevention changes and falsifiable verification")
	}
	for _, change := range report.PreventionChanges {
		if strings.TrimSpace(change.Layer) == "" || strings.TrimSpace(change.Change) == "" || strings.TrimSpace(change.Verification) == "" {
			return nil, errors.New("investigation contains an incomplete prevention change")
		}
	}
	for _, check := range report.VerificationPlan {
		if strings.TrimSpace(check.Test) == "" || strings.TrimSpace(check.FalsifiesIf) == "" {
			return nil, errors.New("investigation contains a non-falsifiable verification step")
		}
	}
	if report.ReplacementWorkStatus != "blocked" && report.ReplacementWorkStatus != "ready_for_review" {
		return nil, errors.New("investigation replacement work status is invalid")
	}
	if strings.TrimSpace(report.NextAction) == "" {
		return nil, errors.New("investigation next action is required")
	}
	if report.OwnerInput.Required && (strings.TrimSpace(report.OwnerInput.Question) == "" || !nonEmptyInvestigationStrings(report.OwnerInput.Options, 2)) {
		return nil, errors.New("investigation requests owner input without a clear multiple-choice question")
	}
	if !report.OwnerInput.Required && (strings.TrimSpace(report.OwnerInput.Question) != "" || len(report.OwnerInput.Options) != 0) {
		return nil, errors.New("investigation includes an owner question without declaring that input is required")
	}
	return json.Marshal(report)
}

func extractFailureInvestigationBlock(output string) ([]byte, error) {
	start := strings.Index(output, failureInvestigationOpenTag)
	if start < 0 || strings.Index(output[start+len(failureInvestigationOpenTag):], failureInvestigationOpenTag) >= 0 {
		return nil, errors.New("investigation must contain exactly one opening evidence block")
	}
	contentStart := start + len(failureInvestigationOpenTag)
	endOffset := strings.Index(output[contentStart:], failureInvestigationCloseTag)
	if endOffset < 0 {
		return nil, errors.New("investigation evidence block is not closed")
	}
	end := contentStart + endOffset
	if strings.Contains(output[end+len(failureInvestigationCloseTag):], failureInvestigationCloseTag) {
		return nil, errors.New("investigation must contain exactly one closing evidence block")
	}
	return []byte(strings.TrimSpace(output[contentStart:end])), nil
}

func nonEmptyInvestigationStrings(values []string, minimum int) bool {
	if len(values) < minimum {
		return false
	}
	for _, value := range values {
		if strings.TrimSpace(value) == "" {
			return false
		}
	}
	return true
}

// failAIFailureInvestigation returns true when the failed task was itself an
// investigator. The caller must not recursively launch an investigation of
// that failure; the durable investigator_failed state is the bounded stop.
func (s *TaskService) failAIFailureInvestigation(ctx context.Context, task db.AgentTaskQueue) bool {
	changed, err := s.Queries.FailAIFailureInvestigationByTask(ctx, task.ID)
	if err != nil {
		slog.Warn("mark AI failure investigation failed", "task_id", util.UUIDToString(task.ID), "error", err)
		return false
	}
	return changed > 0
}

func (s *TaskService) dispatchPendingAIFailureInvestigations(ctx context.Context, issue db.Issue) {
	rows, err := s.Queries.ListDispatchableAIFailureInvestigationsForIssue(ctx, issue.ID)
	if err != nil {
		slog.Warn("list pending AI failure investigations failed", "issue_id", util.UUIDToString(issue.ID), "error", err)
		return
	}
	for _, row := range rows {
		if _, err := s.dispatchAIFailureInvestigation(ctx, issue, row); err != nil {
			slog.Warn("dispatch pending AI failure investigation failed", "investigation_id", util.UUIDToString(row.ID), "error", err)
			return
		}
	}
}
