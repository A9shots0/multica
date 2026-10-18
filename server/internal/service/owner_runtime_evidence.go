package service

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/multica-ai/multica/server/internal/workflowprojection"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

type OwnerRuntimeStageEvidence struct {
	CandidateSHA256     string                                            `json:"candidate_sha256"`
	BuildManifestSHA256 string                                            `json:"build_manifest_sha256"`
	TargetKey           string                                            `json:"target_key"`
	DeployedRevision    string                                            `json:"deployed_revision,omitempty"`
	Gates               map[string]workflowprojection.ReleaseEvidenceGate `json:"gates"`
}

func ValidateOwnerRuntimeStageEvidence(ctx context.Context, q *db.Queries, plan db.IssueWorkflowPlan, task db.AgentTaskQueue, event, candidate string, packet *OwnerRuntimeStageEvidence) error {
	required := map[string][]string{
		"review_passed":        {"review", "build"},
		"deployment_succeeded": {"deployment", "exposure"},
		"production_verified":  {"runtime", "business", "regression"},
		"rollback_succeeded":   {"rollback", "restoration"},
	}
	gates, needs := required[event]
	if !needs {
		if packet != nil {
			return errors.New("stage evidence is not valid for this event")
		}
		return nil
	}
	if packet == nil {
		return errors.New("this owner workflow stage requires --stage-evidence-file and --candidate")
	}
	manifest, err := q.GetLatestWorkflowManifest(ctx, db.GetLatestWorkflowManifestParams{IssueID: plan.IssueID, WorkspaceID: plan.WorkspaceID})
	if err != nil {
		return err
	}
	digest, err := ValidateFileManifest(manifest.Payload)
	if err != nil {
		return err
	}
	if candidate != digest || packet.CandidateSHA256 != digest {
		return errors.New("stage evidence does not match the admitted candidate")
	}
	decoded, err := hex.DecodeString(packet.BuildManifestSHA256)
	if err != nil || len(decoded) != 32 || strings.ToLower(packet.BuildManifestSHA256) != packet.BuildManifestSHA256 || strings.TrimSpace(packet.TargetKey) == "" || len(packet.TargetKey) > 256 {
		return errors.New("stage evidence requires the exact build manifest and deployment target")
	}
	if len(packet.Gates) != len(gates) {
		return errors.New("stage evidence may report only its assigned gates")
	}
	for _, key := range gates {
		gate, ok := packet.Gates[key]
		if !ok || gate.State != "passed" || gate.ObservedAt == nil || len(gate.Refs) < 1 || len(gate.Refs) > 16 || gate.Limitations == nil {
			return errors.New("each passed stage gate requires actual observed evidence references")
		}
		when, err := time.Parse(time.RFC3339Nano, *gate.ObservedAt)
		if err != nil || !task.StartedAt.Valid || when.Before(task.StartedAt.Time) || when.After(time.Now().Add(time.Minute)) {
			return errors.New("stage evidence must be observed by this running task")
		}
		for _, ref := range gate.Refs {
			if strings.TrimSpace(ref) == "" || len(ref) > 2048 {
				return errors.New("invalid stage evidence reference")
			}
		}
	}
	events, err := q.ListIssueWorkflowEvents(ctx, db.ListIssueWorkflowEventsParams{IssueID: plan.IssueID, WorkspaceID: plan.WorkspaceID})
	if err != nil {
		return err
	}
	var review, deployment *OwnerRuntimeStageEvidence
	for _, entry := range events {
		var prior OwnerRuntimeStageEvidence
		if json.Unmarshal(entry.Payload, &prior) != nil || prior.CandidateSHA256 != digest {
			continue
		}
		if entry.EventType == "review_passed" {
			copy := prior
			review = &copy
		}
		if entry.EventType == "deployment_succeeded" {
			copy := prior
			deployment = &copy
		}
	}
	if event != "review_passed" && (review == nil || review.BuildManifestSHA256 != packet.BuildManifestSHA256 || review.TargetKey != packet.TargetKey) {
		return errors.New("release evidence does not match independent review")
	}
	if event == "deployment_succeeded" && strings.TrimSpace(packet.DeployedRevision) == "" {
		return errors.New("deployment requires its real receipt revision")
	}
	if event == "production_verified" && (deployment == nil || packet.DeployedRevision != deployment.DeployedRevision || packet.DeployedRevision == "") {
		return errors.New("verification must identify the exact deployed receipt")
	}
	if event == "rollback_succeeded" && strings.TrimSpace(packet.DeployedRevision) == "" {
		return errors.New("rollback requires its actual restored revision")
	}
	return nil
}
