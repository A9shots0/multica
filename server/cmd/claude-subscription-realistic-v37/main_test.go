package main

import (
	"testing"

	"github.com/multica-ai/multica/server/internal/agentqualification"
)

func TestV37ExecutionFreezeIsReviewedAndValid(t *testing.T) {
	if err := agentqualification.RequireClaudeSubscriptionRealisticV37Freeze(); err != nil {
		t.Fatalf("v37 reviewed freeze rejected: %v", err)
	}
}
