// Command implementation-lane-review-authority issues and revokes the one
// workflow-broker grant that lets the lane review broker run the reviewer turn
// of one issue-started lane workflow.
//
// It follows the accepted #40 authority helper
// (scripts/production-pilot/completion-acceptance-v3/authority), with two
// deliberate differences. It serves only the disposable loopback proof instance:
// it refuses a production environment and any database that is not on
// 127.0.0.1. And it issues exactly one receipt, for the reviewer's "review"
// task kind; the leader and implementer stay ordinary tasks with no grant.
//
// The receipt reuses the qualified role artifact the #40 grant was issued under.
// It does not add a role, a task kind or a capability.
//
//	implementation-lane-review-authority prepare|issue|revoke < contract.json
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	aq "github.com/multica-ai/multica/server/internal/agentqualification"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

const (
	roleArtifactID       = "silver-mediums.role-artifact.v2.data_sync_integrity"
	roleID               = "data_sync_integrity"
	configurationID      = "implementation-lane.review-broker.v1"
	exactModel           = "claude-sonnet-4-6"
	exactReasoningEffort = "medium"
	hostAttestation      = "alienware_windows"
	evidencePrefix       = "implementation-lane-review-broker-sha256."
	maxAuthorityWindow   = 4 * time.Hour
)

type contract struct {
	ControllerID      string    `json:"controller_id"`
	WorkspaceID       string    `json:"workspace_id"`
	OwnerID           string    `json:"owner_id"`
	IssueID           string    `json:"issue_id"`
	SquadID           string    `json:"squad_id"`
	RuntimeID         string    `json:"runtime_id"`
	ImplementerID     string    `json:"implementer_id"`
	ReviewerID        string    `json:"reviewer_id"`
	ReviewerName      string    `json:"reviewer_name"`
	BrokerPath        string    `json:"broker_path"`
	BrokerSHA256      string    `json:"broker_sha256"`
	ProviderPath      string    `json:"provider_path"`
	ProviderSHA256    string    `json:"provider_sha256"`
	ProviderProfile   string    `json:"provider_profile_path"`
	EvidenceReceiptID string    `json:"evidence_receipt_id"`
	IssuedAt          time.Time `json:"issued_at"`
	ExpiresAt         time.Time `json:"expires_at"`
}

var sha256Pattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

func main() {
	if err := run(os.Args[1:], os.Getenv); err != nil {
		fmt.Fprintln(os.Stderr, "lane review authority:", err)
		os.Exit(1)
	}
}

func run(args []string, getenv func(string) string) error {
	if len(args) != 1 || (args[0] != "prepare" && args[0] != "issue" && args[0] != "revoke") {
		return errors.New("mode must be prepare, issue or revoke")
	}
	mode := args[0]
	decoder := json.NewDecoder(os.Stdin)
	decoder.DisallowUnknownFields()
	var c contract
	if err := decoder.Decode(&c); err != nil {
		return fmt.Errorf("contract: %w", err)
	}
	if err := checkEnvironment(getenv, c); err != nil {
		return err
	}
	if err := validateContract(c, time.Now().UTC()); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, getenv("DATABASE_URL"))
	if err != nil {
		return errors.New("database configuration unavailable")
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		return errors.New("database unavailable")
	}
	store := aq.NewPermissionReceiptStore(db.New(pool))
	receiptID := aq.WorkflowBrokerRuntimeReceiptIDV1(c.IssueID, c.ReviewerID)
	if mode == "revoke" {
		if err := store.RevokeQualifiedRuntimeBinding(ctx, c.ControllerID, c.WorkspaceID, receiptID,
			"implementation-lane issue proof closed", time.Now().UTC()); err != nil {
			return err
		}
		fmt.Println("lane review grant revoked: 1")
		return nil
	}
	if err := validateLive(ctx, pool, c); err != nil {
		return err
	}
	receipt, err := buildReceipt(c)
	if err != nil {
		return err
	}
	if mode == "prepare" {
		fmt.Println("lane review grant prepared: 1; stored=0; provider_started=false")
		return nil
	}
	if _, err := store.StoreQualifiedRuntimeBinding(ctx, c.ControllerID, receipt, c.IssuedAt); err != nil {
		return err
	}
	fmt.Println("lane review grant issued: 1; provider_started=false")
	return nil
}

// checkEnvironment keeps the helper on the disposable loopback proof instance.
func checkEnvironment(getenv func(string) string, c contract) error {
	if getenv("APP_ENV") == "production" {
		return errors.New("refusing a production environment")
	}
	if getenv("MULTICA_WORKFLOW_CONTROLLER_ID") != c.ControllerID {
		return errors.New("the contract names another workflow controller")
	}
	parsed, err := url.Parse(getenv("DATABASE_URL"))
	if err != nil || parsed.Scheme != "postgres" || parsed.Hostname() != "127.0.0.1" {
		return errors.New("the grant helper only serves a loopback proof database")
	}
	return nil
}

func canonicalUUID(value string) bool {
	parsed, err := uuid.Parse(value)
	return err == nil && parsed != uuid.Nil && parsed.String() == value
}

func validateContract(c contract, now time.Time) error {
	for name, value := range map[string]string{"controller": c.ControllerID, "workspace": c.WorkspaceID, "owner": c.OwnerID,
		"issue": c.IssueID, "squad": c.SquadID, "runtime": c.RuntimeID, "implementer": c.ImplementerID, "reviewer": c.ReviewerID} {
		if !canonicalUUID(value) {
			return fmt.Errorf("the %s id is not canonical", name)
		}
	}
	if c.ImplementerID == c.ReviewerID {
		return errors.New("the reviewer must be independent of the implementer")
	}
	if strings.TrimSpace(c.ReviewerName) == "" {
		return errors.New("the reviewer name is missing")
	}
	if !sha256Pattern.MatchString(c.BrokerSHA256) || !sha256Pattern.MatchString(c.ProviderSHA256) {
		return errors.New("executable digests must be exact")
	}
	if c.EvidenceReceiptID != evidencePrefix+c.BrokerSHA256 {
		return errors.New("the evidence receipt must name the reviewed broker digest")
	}
	for name, path := range map[string]string{"broker": c.BrokerPath, "provider": c.ProviderPath, "profile": c.ProviderProfile} {
		if !canonicalWindowsAbsolutePath(path) {
			return fmt.Errorf("the %s path must be an exact absolute Windows path", name)
		}
	}
	if !c.ExpiresAt.After(c.IssuedAt) || c.ExpiresAt.Sub(c.IssuedAt) > maxAuthorityWindow ||
		c.IssuedAt.After(now.Add(time.Minute)) || !c.ExpiresAt.After(now) {
		return errors.New("invalid bounded authority window")
	}
	return nil
}

func validateLive(ctx context.Context, pool *pgxpool.Pool, c contract) error {
	var assigneeType, assigneeID string
	if err := pool.QueryRow(ctx, `SELECT coalesce(assignee_type,''), coalesce(assignee_id::text,'') FROM issue WHERE id=$1 AND workspace_id=$2`,
		c.IssueID, c.WorkspaceID).Scan(&assigneeType, &assigneeID); err != nil {
		return errors.New("the issue is not in the workspace")
	}
	if assigneeType != "squad" || assigneeID != c.SquadID {
		return errors.New("the issue must be assigned to its lane squad")
	}
	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM agent_task_queue WHERE issue_id=$1`, c.IssueID).Scan(&count); err != nil || count != 0 {
		return errors.New("the grant must be issued before the issue has any task")
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM agent_runtime WHERE id=$1 AND workspace_id=$2 AND provider='claude' AND status='online' AND owner_id=$3`,
		c.RuntimeID, c.WorkspaceID, c.OwnerID).Scan(&count); err != nil || count != 1 {
		return errors.New("the reviewer's Claude runtime is not online for the owner")
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM agent WHERE id=$1 AND workspace_id=$2 AND runtime_id=$3 AND owner_id=$4 AND name=$5
		AND model=$6 AND thinking_level=$7 AND max_concurrent_tasks=1 AND coalesce(instructions,'')=''
		AND coalesce(custom_env,'{}'::jsonb)='{}'::jsonb AND coalesce(custom_args,'[]'::jsonb)='[]'::jsonb
		AND (mcp_config IS NULL OR mcp_config='{}'::jsonb) AND (runtime_config IS NULL OR runtime_config='{}'::jsonb) AND archived_at IS NULL`,
		c.ReviewerID, c.WorkspaceID, c.RuntimeID, c.OwnerID, c.ReviewerName, exactModel, exactReasoningEffort).Scan(&count); err != nil || count != 1 {
		return errors.New("the reviewer agent configuration is not exact")
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM squad_member WHERE squad_id=$1 AND member_type='agent' AND member_id=$2 AND role='reviewer'`,
		c.SquadID, c.ReviewerID).Scan(&count); err != nil || count != 1 {
		return errors.New("the reviewer is not the squad's reviewer")
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM issue_workflow_plan WHERE issue_id=$1 AND workspace_id=$2 AND squad_id=$3 AND implementer_id=$4 AND reviewer_id=$5 AND NOT requires_deployment`,
		c.IssueID, c.WorkspaceID, c.SquadID, c.ImplementerID, c.ReviewerID).Scan(&count); err != nil || count != 1 {
		return errors.New("the workflow plan does not name this implementer and reviewer")
	}
	return nil
}

func buildReceipt(c contract) (aq.QualifiedRuntimeBindingReceiptV2, error) {
	artifact, err := aq.ValidatedRoleArtifactBindingV2(roleArtifactID)
	if err != nil {
		return aq.QualifiedRuntimeBindingReceiptV2{}, err
	}
	raw, _ := json.Marshal(artifact)
	artifactHash := sha256.Sum256(raw)
	keys, err := aq.GuardedWorkflowBrokerEnvironmentKeysV1("windows")
	if err != nil {
		return aq.QualifiedRuntimeBindingReceiptV2{}, err
	}
	envHash, err := aq.GuardedEnvironmentAllowlistSHA256V1(keys)
	if err != nil {
		return aq.QualifiedRuntimeBindingReceiptV2{}, err
	}
	return aq.QualifiedRuntimeBindingReceiptV2{
		Version: aq.QualifiedRuntimeBindingVersionV2, ID: aq.WorkflowBrokerRuntimeReceiptIDV1(c.IssueID, c.ReviewerID),
		WorkspaceID: c.WorkspaceID, IssueID: c.IssueID, ExecutorAgentID: c.ReviewerID, ConfigurationID: configurationID,
		RoleID: roleID, RuntimeID: c.RuntimeID, RuntimeProvider: "claude", Model: exactModel, ReasoningEffort: exactReasoningEffort,
		ChildExecution: aq.GuardedChildExecutionBoundaryV1{Version: aq.GuardedChildExecutionBoundaryVersionV1, HostOS: "windows",
			Adapter: aq.GuardedClaudeWorkflowBrokerAdapterV1, EnvironmentPolicy: aq.GuardedEnvironmentPolicyExactV1,
			EnvironmentAllowlistSHA256: envHash, ToolPolicy: aq.GuardedToolPolicyNoneV1,
			HostAttestationCustody: aq.GuardedHostAttestationParentOnlyV1, ExecutableSHA256: c.BrokerSHA256,
			ProviderExecutableSHA256: c.ProviderSHA256},
		WorkflowBroker: &aq.GuardedWorkflowBrokerTemplateV1{Version: aq.GuardedWorkflowBrokerTemplateVersionV1, AgentName: c.ReviewerName,
			BrokerExecutablePath: c.BrokerPath, ProviderExecutablePath: c.ProviderPath, ProviderExecutableSHA256: c.ProviderSHA256,
			ProviderProfileKind: aq.GuardedWorkflowBrokerProfileClaudeV1, ProviderProfilePath: c.ProviderProfile,
			HostAttestation: hostAttestation, AllowedTaskKinds: []string{"review"}},
		ArtifactBindingSHA256: hex.EncodeToString(artifactHash[:]), QualificationStatus: aq.StatusQualifiedBounded,
		EvidenceReceiptIDs: []string{c.EvidenceReceiptID}, Source: "controller_record", Immutable: true,
		IssuedAt: c.IssuedAt, ExpiresAt: c.ExpiresAt,
	}, nil
}

// Contracts carry Windows paths even when validated on a Linux CI runner.
func canonicalWindowsAbsolutePath(value string) bool {
	if len(value) < 4 || !((value[0] >= 'A' && value[0] <= 'Z') || (value[0] >= 'a' && value[0] <= 'z')) || value[1:3] != `:\` || strings.ContainsAny(value[3:], `/:<>"|?*`) {
		return false
	}
	for _, part := range strings.Split(value[3:], `\`) {
		if part == "" || part == "." || part == ".." || strings.TrimSpace(part) != part || strings.HasSuffix(part, ".") {
			return false
		}
		base := strings.ToUpper(strings.SplitN(part, ".", 2)[0])
		if base == "CON" || base == "PRN" || base == "AUX" || base == "NUL" || (len(base) == 4 && (base[:3] == "COM" || base[:3] == "LPT") && base[3] >= '1' && base[3] <= '9') {
			return false
		}
	}
	return true
}
