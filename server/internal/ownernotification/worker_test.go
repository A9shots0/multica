package ownernotification

import "testing"

func TestClassifySendTransitionNeverRetriesAmbiguousOutcome(t *testing.T) {
	for _, disposition := range []string{"", "ambiguous", "unexpected"} {
		if got := classifySendTransition(disposition, 1); got != sendTransitionAmbiguous {
			t.Fatalf("disposition %q = %q, want ambiguous", disposition, got)
		}
	}
}

func TestClassifySendTransitionHasBoundedRetryCeiling(t *testing.T) {
	if got := classifySendTransition("retryable", 2); got != sendTransitionRetry {
		t.Fatalf("attempt 2 = %q, want retry", got)
	}
	if got := classifySendTransition("retryable", 3); got != sendTransitionFailed {
		t.Fatalf("attempt 3 = %q, want failed", got)
	}
}

func TestClassifyDeliveryTransitionIsBoundedAndTruthful(t *testing.T) {
	tests := []struct {
		name        string
		disposition string
		checks      int32
		want        deliveryTransition
	}{
		{name: "delivered", disposition: "delivered", checks: 12, want: deliveryTransitionDelivered},
		{name: "provider failure", disposition: "failed", checks: 1, want: deliveryTransitionFailed},
		{name: "provider unconfirmed", disposition: "unconfirmed", checks: 1, want: deliveryTransitionUnconfirmed},
		{name: "pending below ceiling", disposition: "pending", checks: 11, want: deliveryTransitionDefer},
		{name: "pending at ceiling", disposition: "pending", checks: 12, want: deliveryTransitionStop},
		{name: "ambiguous at ceiling", disposition: "ambiguous", checks: 12, want: deliveryTransitionStop},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := classifyDeliveryTransition(tt.disposition, tt.checks, 12); got != tt.want {
				t.Fatalf("transition = %q, want %q", got, tt.want)
			}
		})
	}
}
