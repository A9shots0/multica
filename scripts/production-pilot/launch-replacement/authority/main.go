// This host-admin helper runs inside the existing backend container. It uses
// the container's existing DATABASE_URL without exporting credential values.
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	aq "github.com/multica-ai/multica/server/internal/agentqualification"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

const controllerID = "c43dc900-4554-4cbb-a664-759067b81118"
const workspaceID = "4e3cdf12-463c-4834-9291-c5ff2e422663"
const issueID = "01a0a5fc-a49d-766b-b483-cf85a129c38c"
const ownerID = "6315deec-17ef-4f66-8885-ddc278c52bba"

type role struct {
	ID    string   `json:"id"`
	Name  string   `json:"name"`
	Kinds []string `json:"kinds"`
	Label string   `json:"label"`
}
type contract struct {
	RuntimeID      string    `json:"runtime_id"`
	SquadID        string    `json:"squad_id"`
	BrokerPath     string    `json:"broker_path"`
	BrokerSHA256   string    `json:"broker_sha256"`
	ProviderPath   string    `json:"provider_path"`
	ProviderSHA256 string    `json:"provider_sha256"`
	ProfilePath    string    `json:"provider_profile_path"`
	EvidenceID     string    `json:"evidence_id"`
	IssuedAt       time.Time `json:"issued_at"`
	ExpiresAt      time.Time `json:"expires_at"`
	Roles          []role    `json:"roles"`
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "SILV-38 authority:", err)
		os.Exit(1)
	}
}
func run() error {
	if len(os.Args) != 2 {
		return fmt.Errorf("mode is required")
	}
	mode := os.Args[1]
	if mode != "prepare" && mode != "issue" && mode != "revoke" {
		return fmt.Errorf("unsupported mode")
	}
	if os.Getenv("MULTICA_WORKFLOW_CONTROLLER_ID") != controllerID || os.Getenv("APP_ENV") != "production" {
		return fmt.Errorf("wrong production controller")
	}
	var c contract
	decoder := json.NewDecoder(os.Stdin)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&c); err != nil {
		return err
	}
	if err := validateContract(c); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		return fmt.Errorf("database configuration unavailable")
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("database unavailable")
	}
	store := aq.NewPermissionReceiptStore(db.New(pool))
	if mode == "revoke" {
		for _, r := range c.Roles {
			if err := store.RevokeQualifiedRuntimeBinding(ctx, controllerID, workspaceID, aq.WorkflowBrokerRuntimeReceiptIDV1(issueID, r.ID), "SILV-38 controlled pilot closed", time.Now().UTC()); err != nil {
				return err
			}
		}
		fmt.Println("SILV-38 runtime receipts revoked: 3")
		return nil
	}
	if err := validateLive(ctx, pool, c); err != nil {
		return err
	}
	receipts, err := buildReceipts(c)
	if err != nil {
		return err
	}
	if mode == "prepare" {
		fmt.Println("SILV-38 exact runtime authority prepared: 3; stored=0; provider_started=false")
		return nil
	}
	// Store the three immutable bindings atomically, before any issue dispatch.
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	store = aq.NewPermissionReceiptStore(db.New(tx))
	for _, r := range receipts {
		if _, err := store.StoreQualifiedRuntimeBinding(ctx, controllerID, r, c.IssuedAt); err != nil {
			return err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	fmt.Println("SILV-38 exact runtime receipts issued: 3; provider_started=false")
	return nil
}

func validateContract(c contract) error {
	now := time.Now().UTC()
	if c.RuntimeID == "" || c.SquadID == "" || len(c.Roles) != 3 || !strings.HasPrefix(c.EvidenceID, "silv38-provider-free-review-sha256.") || len(c.EvidenceID) != len("silv38-provider-free-review-sha256.")+64 || len(c.BrokerSHA256) != 64 || len(c.ProviderSHA256) != 64 {
		return fmt.Errorf("incomplete frozen contract or missing review evidence")
	}
	if c.BrokerPath != `C:\Users\a9sho\.codex\worktrees\3228\SMK-multica-selfhost-pilot\server\bin\silv38-launch-replacement-broker.exe` || c.ProviderPath != `C:\Users\a9sho\AppData\Roaming\npm\node_modules\@anthropic-ai\claude-code\bin\claude.exe` || c.ProfilePath != `C:\Users\a9sho\.codex\worktrees\3228\SMK-multica-selfhost-pilot\.qualification-evidence\production-pilot-SILV-38-launch-replacement\provider-home` {
		return fmt.Errorf("executable or profile path drift")
	}
	if c.ExpiresAt.Sub(c.IssuedAt) > 4*time.Hour || !c.ExpiresAt.After(c.IssuedAt) || c.IssuedAt.After(now.Add(time.Minute)) {
		return fmt.Errorf("invalid bounded authority window")
	}
	expected := []role{{Name: "CRM Handoff Planner SILV-38 LaunchReplacement", Kinds: []string{"leader"}, Label: "planner"}, {Name: "CRM Handoff Writer SILV-38 LaunchReplacement", Kinds: []string{"implementation", "correction"}, Label: "writer"}, {Name: "CRM Handoff Reviewer SILV-38 LaunchReplacement", Kinds: []string{"review"}, Label: "reviewer"}}
	seen := map[string]bool{}
	for i, r := range c.Roles {
		a, _ := json.Marshal(r.Kinds)
		b, _ := json.Marshal(expected[i].Kinds)
		if len(r.ID) != 36 || seen[r.ID] || r.Name != expected[i].Name || r.Label != expected[i].Label || string(a) != string(b) {
			return fmt.Errorf("role binding drift")
		}
		seen[r.ID] = true
	}
	return nil
}

func validateLive(ctx context.Context, pool *pgxpool.Pool, c contract) error {
	if !c.ExpiresAt.After(time.Now().UTC()) {
		return fmt.Errorf("authority expired")
	}
	var title, status, assigneeType, assigneeID string
	if err := pool.QueryRow(ctx, `SELECT title,status,coalesce(assignee_type,''),coalesce(assignee_id::text,'') FROM issue WHERE id=$1 AND workspace_id=$2`, issueID, workspaceID).Scan(&title, &status, &assigneeType, &assigneeID); err != nil {
		return err
	}
	if title != "Prepare the current CRM engineering handoff" || assigneeType != "squad" || assigneeID != c.SquadID || status != "backlog" {
		return fmt.Errorf("issue must be parked in its exact squad before receipts")
	}
	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM agent_task_queue WHERE issue_id=$1 AND id NOT IN ('01a0a634-ff16-700a-b631-0726ac8e2229','01a0a642-564f-7946-a7a2-90bb8183ed47','01a0a647-e6c4-7b38-acf3-23930226f396','01a0a64d-27e3-73c5-9c56-02f90bc84fc3','01a0a658-4e03-7ce3-b651-d2be994b0c87')`, issueID).Scan(&count); err != nil || count != 0 {
		return fmt.Errorf("issue already has tasks")
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM agent_runtime WHERE id=$1 AND workspace_id=$2 AND provider='claude' AND daemon_id='silv38-production-pilot-launch-replacement-20260915' AND status='online' AND visibility='private' AND owner_id=$3`, c.RuntimeID, workspaceID, ownerID).Scan(&count); err != nil || count != 1 {
		return fmt.Errorf("exact private Claude runtime is not online")
	}
	for _, r := range c.Roles {
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM agent WHERE id=$1 AND workspace_id=$2 AND runtime_id=$3 AND owner_id=$4 AND name=$5 AND model='claude-sonnet-4-6' AND thinking_level='medium' AND permission_mode='private' AND max_concurrent_tasks=1 AND coalesce(instructions,'')='' AND coalesce(custom_env,'{}'::jsonb)='{}'::jsonb AND coalesce(custom_args,'[]'::jsonb)='[]'::jsonb AND (mcp_config IS NULL OR mcp_config='{}'::jsonb) AND (runtime_config IS NULL OR runtime_config='{}'::jsonb) AND archived_at IS NULL`, r.ID, workspaceID, c.RuntimeID, ownerID, r.Name).Scan(&count); err != nil || count != 1 {
			return fmt.Errorf("exact agent configuration mismatch: %s", r.Label)
		}
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM squad_member WHERE squad_id=$1`, c.SquadID).Scan(&count); err != nil || count != 3 {
		return fmt.Errorf("squad membership differs")
	}
	for _, r := range c.Roles {
		expectedRole := r.Label
		if expectedRole == "planner" {
			expectedRole = "leader"
		}
		if expectedRole == "writer" {
			expectedRole = "implementer"
		}
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM squad_member WHERE squad_id=$1 AND member_type='agent' AND member_id=$2 AND role=$3`, c.SquadID, r.ID, expectedRole).Scan(&count); err != nil || count != 1 {
			return fmt.Errorf("squad role mismatch")
		}
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM issue_workflow_plan WHERE issue_id=$1 AND workspace_id=$2 AND squad_id=$3 AND implementer_id=$4 AND reviewer_id=$5 AND max_corrections=1 AND NOT requires_owner AND NOT requires_deployment`, issueID, workspaceID, c.SquadID, c.Roles[1].ID, c.Roles[2].ID).Scan(&count); err != nil || count != 1 {
		return fmt.Errorf("workflow plan mismatch")
	}
	return nil
}

func buildReceipts(c contract) ([]aq.QualifiedRuntimeBindingReceiptV2, error) {
	artifact, err := aq.ValidatedRoleArtifactBindingV2("silver-mediums.role-artifact.v2.data_sync_integrity")
	if err != nil {
		return nil, err
	}
	raw, _ := json.Marshal(artifact)
	hash := sha256.Sum256(raw)
	keys, err := aq.GuardedWorkflowBrokerEnvironmentKeysV1("windows")
	if err != nil {
		return nil, err
	}
	envHash, err := aq.GuardedEnvironmentAllowlistSHA256V1(keys)
	if err != nil {
		return nil, err
	}
	var result []aq.QualifiedRuntimeBindingReceiptV2
	for _, r := range c.Roles {
		result = append(result, aq.QualifiedRuntimeBindingReceiptV2{
			Version: aq.QualifiedRuntimeBindingVersionV2, ID: aq.WorkflowBrokerRuntimeReceiptIDV1(issueID, r.ID), WorkspaceID: workspaceID, IssueID: issueID, ExecutorAgentID: r.ID, ConfigurationID: "silv38.crm-handoff.v1." + r.Label, RoleID: "data_sync_integrity", RuntimeID: c.RuntimeID, RuntimeProvider: "claude", Model: "claude-sonnet-4-6", ReasoningEffort: "medium",
			ChildExecution:        aq.GuardedChildExecutionBoundaryV1{Version: aq.GuardedChildExecutionBoundaryVersionV1, HostOS: "windows", Adapter: aq.GuardedClaudeWorkflowBrokerAdapterV1, EnvironmentPolicy: aq.GuardedEnvironmentPolicyExactV1, EnvironmentAllowlistSHA256: envHash, ToolPolicy: aq.GuardedToolPolicyNoneV1, HostAttestationCustody: aq.GuardedHostAttestationParentOnlyV1, ExecutableSHA256: c.BrokerSHA256, ProviderExecutableSHA256: c.ProviderSHA256},
			WorkflowBroker:        &aq.GuardedWorkflowBrokerTemplateV1{Version: aq.GuardedWorkflowBrokerTemplateVersionV1, AgentName: r.Name, BrokerExecutablePath: c.BrokerPath, ProviderExecutablePath: c.ProviderPath, ProviderExecutableSHA256: c.ProviderSHA256, ProviderProfileKind: aq.GuardedWorkflowBrokerProfileClaudeV1, ProviderProfilePath: c.ProfilePath, HostAttestation: "alienware_windows", AllowedTaskKinds: r.Kinds},
			ArtifactBindingSHA256: hex.EncodeToString(hash[:]), QualificationStatus: aq.StatusQualifiedBounded, EvidenceReceiptIDs: []string{c.EvidenceID}, Source: "controller_record", Immutable: true, IssuedAt: c.IssuedAt, ExpiresAt: c.ExpiresAt,
		})
	}
	return result, nil
}
