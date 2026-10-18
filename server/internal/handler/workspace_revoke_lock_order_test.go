package handler

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/multica-ai/multica/server/internal/testutil"
	db "github.com/multica-ai/multica/server/pkg/db/generated"
)

func TestMemberRevocationUsesRuntimeBeforeAgentLockOrder(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	workspaceID := dbfx.Workspace(t, "Revocation lock order", "revoke-lock-"+uuid.NewString())
	dbfx.Member(t, workspaceID, testUserID, "owner")
	leaverID := dbfx.User(t, "Revocation leaver", "revoke-lock-"+uuid.NewString()+"@multica.test")
	memberID := dbfx.Member(t, workspaceID, leaverID, "member")
	fx := testutil.New(testPool, workspaceID, leaverID)
	runtimeID := fx.Runtime(t, "Revocation runtime")
	agentID := fx.Agent(t, "Revocation agent", runtimeID)

	// Authority admission takes runtime then agent. Hold its runtime share lock
	// so member revocation must demonstrate that it waits there before touching
	// the agent row.
	authorityConn, err := testPool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer authorityConn.Release()
	authorityTx, err := authorityConn.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer authorityTx.Rollback(context.Background())
	authorityQ := db.New(authorityTx)
	if _, err := authorityQ.LockAgentRuntimeForProjectExecutionAuthority(ctx, db.LockAgentRuntimeForProjectExecutionAuthorityParams{
		ID: parseUUID(runtimeID), WorkspaceID: parseUUID(workspaceID),
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := authorityTx.Exec(ctx, `SET LOCAL lock_timeout = '750ms'`); err != nil {
		t.Fatal(err)
	}

	// A one-connection pool gives the revocation transaction a stable backend
	// PID, letting the test prove exactly where it is blocked.
	config := testPool.Config()
	config.MaxConns = 1
	config.MinConns = 1
	revokePool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(revokePool.Close)
	probe, err := revokePool.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var revokePID int32
	if err := probe.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&revokePID); err != nil {
		probe.Release()
		t.Fatal(err)
	}
	probe.Release()

	revokeHandler := *testHandler
	revokeHandler.Queries = db.New(revokePool)
	revokeHandler.DB = revokePool
	revokeHandler.TxStarter = revokePool
	type revokeResult struct {
		result revocationResult
		err    error
	}
	done := make(chan revokeResult, 1)
	go func() {
		result, revokeErr := revokeHandler.revokeAndRemoveMember(
			context.Background(), parseUUID(workspaceID), parseUUID(leaverID), parseUUID(memberID), parseUUID(testUserID),
		)
		done <- revokeResult{result: result, err: revokeErr}
	}()

	waitForBlockedWriter(t, ctx, revokePID)
	// With runtime-first ordering, revocation owns no agent lock yet and this
	// succeeds immediately. The old agent-then-runtime path timed out here (or
	// PostgreSQL detected a deadlock) because each transaction waited on the
	// other's second lock.
	if _, err := authorityQ.LockAgentForAutopilotAssignment(ctx, db.LockAgentForAutopilotAssignmentParams{
		ID: parseUUID(agentID), WorkspaceID: parseUUID(workspaceID),
	}); err != nil {
		t.Fatalf("authority could not take agent after runtime: %v", err)
	}
	if err := authorityTx.Commit(ctx); err != nil {
		t.Fatal(err)
	}

	select {
	case outcome := <-done:
		if outcome.err != nil {
			t.Fatalf("member revocation after authority commit: %v", outcome.err)
		}
		if len(outcome.result.Runtimes) != 1 || len(outcome.result.ArchivedAgents) != 1 {
			t.Fatalf("revocation result = %+v", outcome.result)
		}
	case <-ctx.Done():
		t.Fatal(fmt.Errorf("member revocation did not finish: %w", ctx.Err()))
	}
}
