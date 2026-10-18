package brain

import (
	"context"
	"net/http"
	"testing"
)

// A versioned envelope alone is not a durable event acknowledgement.
func TestOwnerReviewHandoffRequiresRecordedReceipt(t *testing.T) {
	c := testClient(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"schemaVersion":"silvermediums.brain.v2","servedAt":"2026-09-24T06:00:00Z"}`))
	})
	if err := c.Send(context.Background(), []byte(`{"idempotencyKey":"owner-review"}`)); err == nil {
		t.Fatal("handoff reported delivered without a recorded event receipt")
	}
}
