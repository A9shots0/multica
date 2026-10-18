package handler

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/multica-ai/multica/server/internal/workflowcommand"
	workflowstatev2 "github.com/multica-ai/multica/server/internal/workflowstate/v2"
)

func TestDecodeWorkflowDecisionResponseRequiresExactlyOneJSONValue(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		body    string
		wantErr bool
	}{
		{name: "one value", body: `{"expected_revision":1,"outcome":"approve"}`},
		{name: "trailing whitespace", body: "{\"expected_revision\":1,\"outcome\":\"reject\"}\n\t"},
		{name: "second object", body: `{"expected_revision":1,"outcome":"approve"}{}`, wantErr: true},
		{name: "second scalar", body: `{"expected_revision":1,"outcome":"approve"} true`, wantErr: true},
		{name: "trailing garbage", body: `{"expected_revision":1,"outcome":"approve"} nope`, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var got workflowDecisionResponseRequest
			err := decodeWorkflowDecisionResponse(strings.NewReader(tc.body), &got)
			if (err != nil) != tc.wantErr {
				t.Fatalf("decode error=%v wantErr=%v", err, tc.wantErr)
			}
			if err == nil && (got.ExpectedRevision != 1 || (got.Outcome != workflowstatev2.DecisionApprove && got.Outcome != workflowstatev2.DecisionReject)) {
				t.Fatalf("decoded response=%+v", got)
			}
		})
	}
}

func TestOwnerDecisionMutationStatusNeverReclassifiesForbiddenAsStale(t *testing.T) {
	t.Parallel()
	if got := ownerDecisionMutationStatus(workflowcommand.ErrForbidden); got != http.StatusForbidden {
		t.Fatalf("revoked or downgraded owner status=%d, want 403", got)
	}
	if got := ownerDecisionMutationStatus(errors.Join(errors.New("wrapped"), workflowcommand.ErrForbidden)); got != http.StatusForbidden {
		t.Fatalf("wrapped forbidden status=%d, want 403", got)
	}
	if got := ownerDecisionMutationStatus(workflowcommand.ErrDecisionStale); got != http.StatusConflict {
		t.Fatalf("stale status=%d, want 409", got)
	}
}
