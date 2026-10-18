package main

import (
	"context"
	"errors"
	"github.com/multica-ai/multica/server/internal/agentqualification"
	"os"
	"path/filepath"
	"testing"
)

func TestV38CommandHasExecutableReviewedFreeze(t *testing.T) {
	if err := agentqualification.RequireClaudeSubscriptionRealisticV38Freeze(); err != nil {
		t.Fatalf("v38 reviewed execution freeze rejected: %v", err)
	}
}

func TestV38LifecycleGateRunsBeforeArtifactsOrProvider(t *testing.T) {
	root := filepath.Join(t.TempDir(), "locked")
	calls := 0
	_, err := executeV38(context.Background(), root, v38ProviderFunc(func(context.Context, agentqualification.ClaudeScreeningPlan, agentqualification.RealisticProviderRequest) (agentqualification.ClaudeRealisticV37Plan, []byte, error) {
		calls++
		return agentqualification.ClaudeRealisticV37Plan{}, nil, nil
	}), func() error { return errors.New("review locked") })
	if err == nil || calls != 0 {
		t.Fatalf("gate failed: calls=%d err=%v", calls, err)
	}
	if _, statErr := os.Stat(root); !os.IsNotExist(statErr) {
		t.Fatalf("locked command created artifacts: %v", statErr)
	}
}
