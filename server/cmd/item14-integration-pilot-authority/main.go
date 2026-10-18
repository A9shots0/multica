package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/multica-ai/multica/server/internal/agentqualification"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

const (
	replayHost               = "alienware_windows"
	replayModel              = "claude-sonnet-4-6"
	replayEffort             = "medium"
	replayRoleID             = "data_sync_integrity"
	replayBrokerPath         = `A:\Silver-Mediums-Workspace\worktrees\multica\integration-preservation-pilot-20260914T232814Z\server\bin\item14-integration-pilot-broker-v3.exe`
	replayBrokerSHA256       = "366718905d210c833d10a1116e4ef4033613be65046fa647625be88e09b4d4da"
	replayCLIPath            = `A:\Silver-Mediums-Workspace\worktrees\multica\integration-preservation-pilot-20260914T232814Z\server\bin\multica-item14-replay-v3.exe`
	replayCLISHA256          = "6abe339f33fdecb774f0a92c5d67e09aa74afba1540ce4ff1024594e7b1c3835"
	replayControllerImageID  = "sha256:53ce0736da07fbc00c0fc7c7be8c2b8414e5afea63100aa921fee881b5f37080"
	replayReportPath         = `C:\Dev\Silver-Mediums-Knowledge\initiatives\company-engineering-foundation\multica-integration-pilot-replay-20260914`
	replayFixtureDescription = "Temporary bounded integration replay fixture; not a permanent role or model selection"
	replaySkillDescription   = "Temporary frozen integration replay boundary; not a permanent role skill"
	replaySkillContent       = "Execute only the controller-bound Item 14 replay task kind. The provider child is tool-free and repository-free. The deterministic broker alone performs the frozen local inventory, exact report writes, and task-scoped workflow handoffs. No production, outbound action, Phone Farm technical access, model selection, or retries."
)

// The default values preserve the first replay authority helper. A fresh
// replay binary overrides only these fixed strings at link time, producing a
// non-configurable executable for that one isolated fixture while keeping the
// original helper source and binary reproducible.
var (
	replayControllerID        = "00000000-0000-4000-8000-000000001401"
	replayWorkspaceID         = "4d89b87d-6c66-48ba-b671-f7c6ed176dc6"
	replayIssueID             = "01a0a390-d524-79a0-a3bf-5781d57f72ee"
	replayRuntimeID           = "dc924e46-34a9-4916-9fc2-847f13a8a6d0"
	replaySquadID             = "fecb50de-9ea3-4700-afb1-68b344096f68"
	replayLeaderID            = "7d622153-c9a9-4b57-912c-bebef8ce06b0"
	replayExecutorID          = "eabbf54b-06e9-4709-8c4d-9ab4cbff1246"
	replayReviewerID          = "c59782cf-15fb-4293-9110-08d89184994f"
	replaySkillID             = "795a1b83-f97c-4cb9-8bda-88e5bf37d34f"
	replayProviderProfilePath = `A:\Silver-Mediums-Workspace\worktrees\multica\integration-preservation-pilot-20260914T232814Z\.qualification-evidence\item14-integration-replay-20260915\provider-home`
	replayProviderPath        = `C:\Users\a9sho\AppData\Roaming\npm\node_modules\@anthropic-ai\claude-code\bin\claude.exe`
	replayProviderSHA256      = "cd8d8d33e549ba5973428bcf18310164227d68ca756925242c47d3b0d48bf4dc"
	replayDatabasePort        = "15434"
	replayBackendPort         = "18081"
	replayWebPort             = "13001"
	replayWorkspaceSlug       = "company-engineering-foundation-replay"
	replayDaemonID            = "item14-integration-replay-20260915"
	replayLaunchContractPath  = ""
	replayLaunchContractSHA   = ""
)

type options struct {
	mode         string
	databaseURL  string
	workspaceID  string
	issueID      string
	runtimeID    string
	squadID      string
	leaderID     string
	executorID   string
	reviewerID   string
	skillID      string
	brokerPath   string
	providerPath string
	profilePath  string
	launchPath   string
	issuedAt     time.Time
	expiresAt    time.Time
	revokedAt    time.Time
}

type roleBinding struct {
	agentID     string
	agentName   string
	taskKinds   []string
	configLabel string
}

type replayLaunchContractV1 struct {
	Version                     string `json:"version"`
	ControllerImageID           string `json:"controller_image_id"`
	DatabaseContainer           string `json:"database_container"`
	ControllerContainer         string `json:"controller_container"`
	FrontendContainer           string `json:"frontend_container"`
	DatabasePort                string `json:"database_port"`
	BackendPort                 string `json:"backend_port"`
	WebPort                     string `json:"web_port"`
	ServerURL                   string `json:"server_url"`
	WorkspaceID                 string `json:"workspace_id"`
	WorkflowControllerID        string `json:"workflow_controller_id"`
	DaemonID                    string `json:"daemon_id"`
	DaemonAuthMode              string `json:"daemon_auth_mode"`
	DaemonAuthExpiresAt         string `json:"daemon_auth_expires_at"`
	HostAttestation             string `json:"host_attestation"`
	CLIPath                     string `json:"cli_path"`
	CLISHA256                   string `json:"cli_sha256"`
	BrokerPath                  string `json:"broker_path"`
	BrokerSHA256                string `json:"broker_sha256"`
	CLIProfile                  string `json:"cli_profile"`
	CLIProfilePath              string `json:"cli_profile_path"`
	DeviceName                  string `json:"device_name"`
	RuntimeName                 string `json:"runtime_name"`
	WorkspacesRoot              string `json:"workspaces_root"`
	AgentTimeout                string `json:"agent_timeout"`
	MaxConcurrentTasks          int    `json:"max_concurrent_tasks"`
	AutoUpdateEnabled           bool   `json:"auto_update_enabled"`
	AutoReloadEnabled           bool   `json:"auto_reload_enabled"`
	RuntimeProvider             string `json:"runtime_provider"`
	Model                       string `json:"model"`
	ReasoningEffort             string `json:"reasoning_effort"`
	ProviderProfilePath         string `json:"provider_profile_path"`
	ProviderCallLimit           int    `json:"provider_call_limit"`
	ProviderRetryLimit          int    `json:"provider_retry_limit"`
	MaxTurns                    int    `json:"max_turns"`
	ToolPolicy                  string `json:"tool_policy"`
	ReportPath                  string `json:"report_path"`
	NonClaudeProviderSentinel   string `json:"non_claude_provider_sentinel"`
	WorkflowProviderTaskCeiling int    `json:"workflow_provider_task_ceiling"`
	TaskMaxAttempts             int    `json:"task_max_attempts"`
	GuardedPackageRequired      bool   `json:"guarded_package_required"`
}

func main() {
	if err := run(context.Background(), os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "item14 replay authority:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	opts, err := parseOptions(args)
	if err != nil {
		return err
	}
	if err := validateLoopbackDatabaseURL(opts.databaseURL); err != nil {
		return err
	}
	pool, err := pgxpool.New(ctx, opts.databaseURL)
	if err != nil {
		return fmt.Errorf("connect replay database: %w", err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("ping replay database: %w", err)
	}
	if err := validateLiveReplayScope(ctx, pool, opts); err != nil {
		return err
	}

	store := agentqualification.NewPermissionReceiptStore(db.New(pool))
	switch opts.mode {
	case "prepare":
		receipts, err := buildReceipts(opts)
		if err != nil {
			return err
		}
		fmt.Printf("ITEM14_REPLAY_AUTHORITY prepared=%d stored=0 provider_started=false\n", len(receipts))
		return nil
	case "issue":
		receipts, err := buildReceipts(opts)
		if err != nil {
			return err
		}
		for _, receipt := range receipts {
			if _, err := store.StoreQualifiedRuntimeBinding(ctx, replayControllerID, receipt, opts.issuedAt); err != nil {
				return fmt.Errorf("store receipt %s: %w", receipt.ID, err)
			}
		}
		fmt.Printf("ITEM14_REPLAY_AUTHORITY issued=%d active=%d provider_started=false\n", len(receipts), len(receipts))
		return nil
	case "status":
		var total, active, revoked int
		err := pool.QueryRow(ctx, `
			SELECT count(*),
			       count(*) FILTER (WHERE expires_at > now() AND NOT EXISTS (
			         SELECT 1 FROM qualified_runtime_binding_revocation r WHERE r.receipt_id = q.id
			       )),
			       count(*) FILTER (WHERE EXISTS (
			         SELECT 1 FROM qualified_runtime_binding_revocation r WHERE r.receipt_id = q.id
			       ))
			FROM qualified_runtime_binding_receipt q
			WHERE workspace_id = $1 AND issue_id = $2 AND issued_by_controller_id = $3`,
			opts.workspaceID, opts.issueID, replayControllerID,
		).Scan(&total, &active, &revoked)
		if err != nil {
			return fmt.Errorf("read replay authority status: %w", err)
		}
		fmt.Printf("ITEM14_REPLAY_AUTHORITY total=%d active=%d revoked=%d provider_started=false\n", total, active, revoked)
		return nil
	case "revoke":
		for _, role := range replayRoles(opts) {
			receiptID := agentqualification.WorkflowBrokerRuntimeReceiptIDV1(opts.issueID, role.agentID)
			if err := store.RevokeQualifiedRuntimeBinding(ctx, replayControllerID, opts.workspaceID, receiptID, "item14 bounded replay terminal cleanup", opts.revokedAt); err != nil {
				return fmt.Errorf("revoke receipt %s: %w", receiptID, err)
			}
		}
		fmt.Printf("ITEM14_REPLAY_AUTHORITY revoked=3 provider_started=false\n")
		return nil
	default:
		return fmt.Errorf("unsupported mode %q", opts.mode)
	}
}

func parseOptions(args []string) (options, error) {
	fs := flag.NewFlagSet("item14-integration-pilot-authority", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var opts options
	var issuedRaw, expiresRaw, revokedRaw string
	fs.StringVar(&opts.mode, "mode", "", "prepare, issue, status, or revoke")
	fs.StringVar(&opts.databaseURL, "database-url", "", "fresh loopback replay database")
	fs.StringVar(&opts.workspaceID, "workspace-id", "", "replay workspace UUID")
	fs.StringVar(&opts.issueID, "issue-id", "", "replay issue UUID")
	fs.StringVar(&opts.runtimeID, "runtime-id", "", "replay runtime UUID")
	fs.StringVar(&opts.squadID, "squad-id", "", "replay squad UUID")
	fs.StringVar(&opts.leaderID, "leader-id", "", "temporary leader UUID")
	fs.StringVar(&opts.executorID, "executor-id", "", "temporary executor UUID")
	fs.StringVar(&opts.reviewerID, "reviewer-id", "", "temporary reviewer UUID")
	fs.StringVar(&opts.skillID, "skill-id", "", "temporary skill UUID")
	fs.StringVar(&opts.brokerPath, "broker-path", "", "exact frozen broker path")
	fs.StringVar(&opts.providerPath, "provider-path", "", "exact Claude executable path")
	fs.StringVar(&opts.profilePath, "profile-path", "", "task-local Claude profile home")
	fs.StringVar(&opts.launchPath, "launch-contract-path", "", "exact shared daemon launch contract")
	fs.StringVar(&issuedRaw, "issued-at", "", "fixed RFC3339 issuance time")
	fs.StringVar(&expiresRaw, "expires-at", "", "fixed RFC3339 expiry time")
	fs.StringVar(&revokedRaw, "revoked-at", "", "fixed RFC3339 revocation time")
	if err := fs.Parse(args); err != nil || fs.NArg() != 0 {
		return opts, fmt.Errorf("invalid authority arguments")
	}
	if opts.mode != "prepare" && opts.mode != "issue" && opts.mode != "status" && opts.mode != "revoke" {
		return opts, fmt.Errorf("mode must be prepare, issue, status, or revoke")
	}
	for name, value := range map[string]string{
		"mode": opts.mode, "database-url": opts.databaseURL, "workspace-id": opts.workspaceID,
		"issue-id": opts.issueID, "runtime-id": opts.runtimeID, "squad-id": opts.squadID,
		"leader-id": opts.leaderID, "executor-id": opts.executorID, "reviewer-id": opts.reviewerID,
		"skill-id": opts.skillID, "broker-path": opts.brokerPath, "provider-path": opts.providerPath,
		"profile-path": opts.profilePath,
	} {
		if strings.TrimSpace(value) == "" {
			return opts, fmt.Errorf("%s is required", name)
		}
	}
	if opts.mode == "prepare" || opts.mode == "issue" {
		var err error
		if opts.issuedAt, err = time.Parse(time.RFC3339, issuedRaw); err != nil {
			return opts, fmt.Errorf("issued-at must be RFC3339")
		}
		if opts.expiresAt, err = time.Parse(time.RFC3339, expiresRaw); err != nil || !opts.issuedAt.Before(opts.expiresAt) {
			return opts, fmt.Errorf("expires-at must be later RFC3339 time")
		}
	}
	if replayLaunchContractPath == "" {
		if strings.TrimSpace(opts.launchPath) != "" {
			return opts, fmt.Errorf("launch-contract-path is unavailable for this frozen authority")
		}
	} else if strings.TrimSpace(opts.launchPath) == "" {
		return opts, fmt.Errorf("launch-contract-path is required")
	}
	if opts.mode == "revoke" {
		var err error
		if opts.revokedAt, err = time.Parse(time.RFC3339, revokedRaw); err != nil {
			return opts, fmt.Errorf("revoked-at must be RFC3339")
		}
	}
	return opts, nil
}

func validateLoopbackDatabaseURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") || u.Hostname() != "127.0.0.1" || u.Port() != replayDatabasePort || u.Path != "/multica" {
		return fmt.Errorf("database must be the exact fresh loopback replay database")
	}
	return nil
}

func validateLiveReplayScope(ctx context.Context, pool *pgxpool.Pool, opts options) error {
	if opts.workspaceID != replayWorkspaceID || opts.issueID != replayIssueID || opts.runtimeID != replayRuntimeID ||
		opts.squadID != replaySquadID || opts.leaderID != replayLeaderID || opts.executorID != replayExecutorID ||
		opts.reviewerID != replayReviewerID || opts.skillID != replaySkillID {
		return fmt.Errorf("authority identifiers do not match the frozen replay")
	}
	if filepath.Clean(opts.brokerPath) != opts.brokerPath || !filepath.IsAbs(opts.brokerPath) ||
		filepath.Clean(opts.providerPath) != opts.providerPath || !filepath.IsAbs(opts.providerPath) ||
		filepath.Clean(opts.profilePath) != opts.profilePath || !filepath.IsAbs(opts.profilePath) {
		return fmt.Errorf("broker, provider, and profile paths must be exact and absolute")
	}
	if !strings.EqualFold(opts.brokerPath, replayBrokerPath) || !strings.EqualFold(opts.providerPath, replayProviderPath) ||
		!strings.EqualFold(opts.profilePath, replayProviderProfilePath) {
		return fmt.Errorf("authority paths do not match the frozen replay")
	}
	if err := validateReplayLaunchContract(opts.launchPath); err != nil {
		return err
	}
	for path, wantHash := range map[string]string{opts.brokerPath: replayBrokerSHA256, opts.providerPath: replayProviderSHA256} {
		info, err := os.Lstat(path)
		if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return fmt.Errorf("bound executable is not a non-symlink regular file")
		}
		gotHash, err := fileSHA256(path)
		if err != nil || gotHash != wantHash {
			return fmt.Errorf("bound executable digest does not match the frozen replay")
		}
	}
	profileInfo, err := os.Lstat(opts.profilePath)
	if err != nil || profileInfo.Mode()&os.ModeSymlink != 0 || !profileInfo.IsDir() {
		return fmt.Errorf("task-local profile home is unavailable")
	}
	credentialInfo, err := os.Lstat(filepath.Join(opts.profilePath, ".claude", ".credentials.json"))
	if err != nil || credentialInfo.Mode()&os.ModeSymlink != 0 || !credentialInfo.Mode().IsRegular() {
		return fmt.Errorf("task-local subscription credential is unavailable")
	}
	if opts.mode == "prepare" || opts.mode == "issue" {
		if _, err := os.Lstat(replayReportPath); !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("additive replay report path must remain absent before issuance")
		}
	}

	var ok bool
	err = pool.QueryRow(ctx, `
		SELECT w.slug = $7
		   AND i.workspace_id = $1 AND i.title = 'Windows repository/worktree inventory replay'
		   AND i.assignee_type = 'squad' AND i.assignee_id = $3
		   AND p.workspace_id = $1 AND p.squad_id = $3 AND p.implementer_id = $4 AND p.reviewer_id = $5
		   AND p.max_corrections = 1 AND NOT p.requires_owner AND NOT p.requires_deployment
		   AND s.workspace_id = $1 AND s.name = 'CEF Inventory Replay Squad' AND s.leader_id = $6
		   AND s.archived_at IS NULL
		FROM workspace w
		JOIN issue i ON i.id = $2 AND i.workspace_id = w.id
		JOIN issue_workflow_plan p ON p.issue_id = i.id
		JOIN squad s ON s.id = $3
		WHERE w.id = $1`, opts.workspaceID, opts.issueID, opts.squadID, opts.executorID, opts.reviewerID, opts.leaderID, replayWorkspaceSlug,
	).Scan(&ok)
	if err != nil || !ok {
		return fmt.Errorf("live issue/workflow scope does not match frozen replay")
	}
	var membersOK bool
	err = pool.QueryRow(ctx, `
		SELECT count(*) = 3
		   AND count(*) FILTER (WHERE member_type = 'agent' AND member_id = $2 AND role = 'leader') = 1
		   AND count(*) FILTER (WHERE member_type = 'agent' AND member_id = $3 AND role = 'implementer') = 1
		   AND count(*) FILTER (WHERE member_type = 'agent' AND member_id = $4 AND role = 'reviewer') = 1
		FROM squad_member WHERE squad_id = $1`, opts.squadID, opts.leaderID, opts.executorID, opts.reviewerID,
	).Scan(&membersOK)
	if err != nil || !membersOK {
		return fmt.Errorf("live squad membership does not match frozen replay")
	}
	var skillOK bool
	err = pool.QueryRow(ctx, `
		SELECT workspace_id = $1 AND name = 'Item 14 Replay Boundary'
		   AND description = $3 AND content = $4 AND config = '{}'::jsonb
		FROM skill WHERE id = $2`, opts.workspaceID, opts.skillID, replaySkillDescription, replaySkillContent,
	).Scan(&skillOK)
	if err != nil || !skillOK {
		return fmt.Errorf("temporary skill does not match frozen replay")
	}
	var runtimeOK bool
	err = pool.QueryRow(ctx, `
		SELECT workspace_id = $1 AND provider = 'claude' AND status = 'online'
		   AND visibility = 'private' AND daemon_id = $3
		FROM agent_runtime WHERE id = $2`, opts.workspaceID, opts.runtimeID, replayDaemonID).Scan(&runtimeOK)
	if err != nil || !runtimeOK {
		return fmt.Errorf("live runtime does not match frozen replay")
	}
	for _, role := range replayRoles(opts) {
		var agentOK bool
		err = pool.QueryRow(ctx, `
			SELECT workspace_id = $1 AND name = $3 AND runtime_id = $4
			   AND model = $5 AND thinking_level = $6 AND visibility = 'private'
			   AND permission_mode = 'private' AND max_concurrent_tasks = 1
			   AND description = $8 AND instructions = ''
			   AND archived_at IS NULL AND custom_env = '{}'::jsonb AND custom_args = '[]'::jsonb
			   AND mcp_config = '{}'::jsonb AND runtime_config = '{}'::jsonb
			   AND (SELECT count(*) FROM agent_skill s WHERE s.agent_id = agent.id) = 1
			   AND EXISTS (SELECT 1 FROM agent_skill s WHERE s.agent_id = agent.id AND s.skill_id = $7)
			FROM agent WHERE id = $2`, opts.workspaceID, role.agentID, role.agentName,
			opts.runtimeID, replayModel, replayEffort, opts.skillID, replayFixtureDescription,
		).Scan(&agentOK)
		if err != nil || !agentOK {
			return fmt.Errorf("temporary agent %s does not match frozen replay", role.agentName)
		}
	}
	if opts.mode == "prepare" || opts.mode == "issue" {
		var maxAttemptsDefault string
		if err := pool.QueryRow(ctx, `
			SELECT COALESCE(column_default, '')
			FROM information_schema.columns
			WHERE table_schema = 'public'
			  AND table_name = 'agent_task_queue'
			  AND column_name = 'max_attempts'`).Scan(&maxAttemptsDefault); err != nil || !isExactSingleAttemptDefault(maxAttemptsDefault) {
			return fmt.Errorf("replay task retry default must be exactly one attempt")
		}
		var prelaunchOK bool
		if err := pool.QueryRow(ctx, `
			SELECT i.status = 'in_progress' AND i.acceptance_criteria = '[]'::jsonb
			   AND i.context_refs = '[]'::jsonb AND p.current_state = 'working'
			FROM issue i JOIN issue_workflow_plan p ON p.issue_id = i.id
			WHERE i.id = $1`, opts.issueID).Scan(&prelaunchOK); err != nil || !prelaunchOK {
			return fmt.Errorf("replay issue is not in the frozen pre-launch state")
		}
		var tasks int
		if err := pool.QueryRow(ctx, `SELECT count(*) FROM agent_task_queue WHERE issue_id = $1`, opts.issueID).Scan(&tasks); err != nil || tasks != 0 {
			return fmt.Errorf("replay issue must have zero task rows before authority issuance")
		}
	}
	return nil
}

func validateReplayLaunchContract(path string) error {
	if replayLaunchContractPath == "" {
		return nil
	}
	if filepath.Clean(path) != path || !filepath.IsAbs(path) || !strings.EqualFold(path, replayLaunchContractPath) {
		return fmt.Errorf("launch contract path does not match the frozen replay")
	}
	info, err := os.Lstat(path)
	if err != nil || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return fmt.Errorf("launch contract is not a non-symlink regular file")
	}
	digest, err := fileSHA256(path)
	if err != nil || digest != replayLaunchContractSHA {
		return fmt.Errorf("launch contract digest does not match the frozen replay")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read launch contract: %w", err)
	}
	var contract replayLaunchContractV1
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&contract); err != nil {
		return fmt.Errorf("decode launch contract: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return fmt.Errorf("launch contract has trailing data")
	}
	if contract != expectedReplayLaunchContractV1() {
		return fmt.Errorf("launch contract values do not match the frozen replay")
	}
	return nil
}

func expectedReplayLaunchContractV1() replayLaunchContractV1 {
	return replayLaunchContractV1{
		Version:             "item14.integration-replay-exact-launch.v2",
		ControllerImageID:   replayControllerImageID,
		DatabaseContainer:   "smk-integration-replay-20260915-v3-postgres-1",
		ControllerContainer: "smk-integration-replay-20260915-v3-backend-1",
		FrontendContainer:   "smk-integration-replay-20260915-v3-frontend-1",
		DatabasePort:        replayDatabasePort, BackendPort: replayBackendPort, WebPort: replayWebPort,
		ServerURL: "http://127.0.0.1:" + replayBackendPort, WorkspaceID: replayWorkspaceID,
		WorkflowControllerID: replayControllerID,
		DaemonID:             replayDaemonID, DaemonAuthMode: "workspace_scoped_daemon_token_v1",
		DaemonAuthExpiresAt: "2026-09-16T14:00:00Z", HostAttestation: replayHost,
		CLIPath: replayCLIPath, CLISHA256: replayCLISHA256,
		BrokerPath: replayBrokerPath, BrokerSHA256: replayBrokerSHA256,
		CLIProfile:     "item14-integration-replay-v3-daemon-20260915",
		CLIProfilePath: `C:\Users\a9sho\.multica\profiles\item14-integration-replay-v3-daemon-20260915`,
		DeviceName:     "Item 14 Replay V3 Isolated", RuntimeName: "Item 14 Replay V2 Claude",
		WorkspacesRoot: `A:\Silver-Mediums-Workspace\worktrees\multica\integration-preservation-pilot-20260914T232814Z\.qualification-evidence\item14-integration-replay-20260915-v3\runtime-workspaces`,
		AgentTimeout:   "20m0s", MaxConcurrentTasks: 1,
		AutoUpdateEnabled: false, AutoReloadEnabled: false,
		RuntimeProvider: "claude", Model: replayModel, ReasoningEffort: replayEffort,
		ProviderProfilePath: replayProviderProfilePath,
		ProviderCallLimit:   1, ProviderRetryLimit: 0, MaxTurns: 1,
		ToolPolicy:                  agentqualification.GuardedToolPolicyNoneV1,
		ReportPath:                  replayReportPath,
		NonClaudeProviderSentinel:   `A:\Silver-Mediums-Workspace\worktrees\multica\integration-preservation-pilot-20260914T232814Z\.qualification-evidence\item14-integration-replay-20260915-v3\absent-provider.exe`,
		WorkflowProviderTaskCeiling: 5, TaskMaxAttempts: 1, GuardedPackageRequired: true,
	}
}

func isExactSingleAttemptDefault(value string) bool {
	return strings.TrimSpace(value) == "1"
}

func replayRoles(opts options) []roleBinding {
	return []roleBinding{
		{agentID: opts.leaderID, agentName: "Item 14 Replay Intake", taskKinds: []string{"leader"}, configLabel: "leader"},
		{agentID: opts.executorID, agentName: "Item 14 Replay Executor", taskKinds: []string{"implementation", "correction"}, configLabel: "executor"},
		{agentID: opts.reviewerID, agentName: "Item 14 Replay Reviewer", taskKinds: []string{"review"}, configLabel: "reviewer"},
	}
}

func buildReceipts(opts options) ([]agentqualification.QualifiedRuntimeBindingReceiptV2, error) {
	artifact, err := agentqualification.ValidatedRoleArtifactBindingV2("silver-mediums.role-artifact.v2." + replayRoleID)
	if err != nil {
		return nil, err
	}
	artifactRaw, err := json.Marshal(artifact)
	if err != nil {
		return nil, err
	}
	artifactSum := sha256.Sum256(artifactRaw)
	keys, err := agentqualification.GuardedWorkflowBrokerEnvironmentKeysV1(runtime.GOOS)
	if err != nil {
		return nil, err
	}
	environmentHash, err := agentqualification.GuardedEnvironmentAllowlistSHA256V1(keys)
	if err != nil {
		return nil, err
	}
	brokerHash, err := fileSHA256(opts.brokerPath)
	if err != nil {
		return nil, err
	}
	providerHash, err := fileSHA256(opts.providerPath)
	if err != nil {
		return nil, err
	}
	receipts := make([]agentqualification.QualifiedRuntimeBindingReceiptV2, 0, 3)
	for _, role := range replayRoles(opts) {
		receipts = append(receipts, agentqualification.QualifiedRuntimeBindingReceiptV2{
			Version:     agentqualification.QualifiedRuntimeBindingVersionV2,
			ID:          agentqualification.WorkflowBrokerRuntimeReceiptIDV1(opts.issueID, role.agentID),
			WorkspaceID: opts.workspaceID, IssueID: opts.issueID, ExecutorAgentID: role.agentID,
			ConfigurationID: "item14.workflow-broker.real-replay.v1." + role.configLabel,
			RoleID:          replayRoleID, RuntimeID: opts.runtimeID, RuntimeProvider: "claude",
			Model: replayModel, ReasoningEffort: replayEffort,
			ChildExecution: agentqualification.GuardedChildExecutionBoundaryV1{
				Version: agentqualification.GuardedChildExecutionBoundaryVersionV1,
				HostOS:  runtime.GOOS, Adapter: agentqualification.GuardedClaudeWorkflowBrokerAdapterV1,
				EnvironmentPolicy:          agentqualification.GuardedEnvironmentPolicyExactV1,
				EnvironmentAllowlistSHA256: environmentHash, ToolPolicy: agentqualification.GuardedToolPolicyNoneV1,
				HostAttestationCustody: agentqualification.GuardedHostAttestationParentOnlyV1,
				ExecutableSHA256:       brokerHash, ProviderExecutableSHA256: providerHash,
			},
			WorkflowBroker: &agentqualification.GuardedWorkflowBrokerTemplateV1{
				Version:   agentqualification.GuardedWorkflowBrokerTemplateVersionV1,
				AgentName: role.agentName, BrokerExecutablePath: opts.brokerPath,
				ProviderExecutablePath: opts.providerPath, ProviderExecutableSHA256: providerHash,
				ProviderProfileKind: agentqualification.GuardedWorkflowBrokerProfileClaudeV1,
				ProviderProfilePath: opts.profilePath, HostAttestation: replayHost,
				AllowedTaskKinds: append([]string(nil), role.taskKinds...),
			},
			ArtifactBindingSHA256: hex.EncodeToString(artifactSum[:]),
			QualificationStatus:   agentqualification.StatusQualifiedBounded,
			EvidenceReceiptIDs:    []string{"item14-integration-replay-provider-free-freeze-20260915"},
			Source:                "controller_record", Immutable: true,
			IssuedAt: opts.issuedAt, ExpiresAt: opts.expiresAt,
		})
	}
	return receipts, nil
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
